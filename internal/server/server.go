package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/shard-cache/internal/cache"
	"github.com/shard-cache/proto"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server represents a cache server
type Server struct {
	proto.UnimplementedCacheServiceServer
	
	config     *Config
	cache      *cache.Cache
	logger     *zap.Logger
	grpcServer *grpc.Server
	httpServer *http.Server
	
	// Backpressure control
	semaphore *semaphore.Weighted
	
	// Graceful shutdown
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	
	// Load shedding
	cpuThreshold float64
	cpuWindow    time.Duration
	cpuHistory   []float64
	cpuMutex     sync.RWMutex
}

// Config holds server configuration
type Config struct {
	GRPCPort      int
	HTTPPort      int
	CacheCapacity int
	MaxConcurrent int64
	CPUThreshold  float64
	CPUWindow     time.Duration
}

// NewServer creates a new cache server
func NewServer(config *Config) (*Server, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	
	server := &Server{
		config:       config,
		cache:        cache.NewCache(config.CacheCapacity),
		logger:       logger,
		semaphore:    semaphore.NewWeighted(config.MaxConcurrent),
		shutdownCh:   make(chan struct{}),
		cpuThreshold: config.CPUThreshold,
		cpuWindow:    config.CPUWindow,
		cpuHistory:   make([]float64, 0),
	}
	
	// Start CPU monitoring
	server.startCPUMonitoring()
	
	return server, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Start gRPC server
	if err := s.startGRPCServer(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}
	
	// Start HTTP server for metrics
	if err := s.startHTTPServer(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	
	s.logger.Info("Server started", 
		zap.Int("grpc_port", s.config.GRPCPort),
		zap.Int("http_port", s.config.HTTPPort))
	
	// Wait for shutdown signal
	s.waitForShutdown()
	
	return nil
}

// startGRPCServer starts the gRPC server
func (s *Server) startGRPCServer() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
	)
	proto.RegisterCacheServiceServer(s.grpcServer, s)
	
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()
	
	return nil
}

// startHTTPServer starts the HTTP server for metrics
func (s *Server) startHTTPServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/metrics", s.metricsHandler)
	
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.HTTPPort),
		Handler: mux,
	}
	
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server failed", zap.Error(err))
		}
	}()
	
	return nil
}

// unaryInterceptor provides backpressure and load shedding
func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Check for context cancellation early
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}
	
	// Load shedding based on CPU usage
	if s.shouldShedLoad() {
		return nil, status.Error(codes.Unavailable, "server overloaded")
	}
	
	// Backpressure control
	if !s.semaphore.TryAcquire(1) {
		return nil, status.Error(codes.Unavailable, "too many concurrent requests")
	}
	defer s.semaphore.Release(1)
	
	// Call the actual handler
	return handler(ctx, req)
}

// shouldShedLoad determines if we should shed load based on CPU usage
func (s *Server) shouldShedLoad() bool {
	s.cpuMutex.RLock()
	defer s.cpuMutex.RUnlock()
	
	if len(s.cpuHistory) == 0 {
		return false
	}
	
	// Calculate average CPU usage over the window
	var sum float64
	for _, usage := range s.cpuHistory {
		sum += usage
	}
	avgCPU := sum / float64(len(s.cpuHistory))
	
	return avgCPU > s.cpuThreshold
}

// startCPUMonitoring starts CPU usage monitoring
func (s *Server) startCPUMonitoring() {
	ticker := time.NewTicker(time.Second)
	s.wg.Add(1)
	
	go func() {
		defer s.wg.Done()
		defer ticker.Stop()
		
		for {
			select {
			case <-s.shutdownCh:
				return
			case <-ticker.C:
				s.updateCPUUsage()
			}
		}
	}()
}

// updateCPUUsage updates the CPU usage history
func (s *Server) updateCPUUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Simple CPU approximation based on goroutine count and memory usage
	// In a real implementation, you'd use proper CPU monitoring
	cpuUsage := float64(runtime.NumGoroutine()) / 1000.0 // Simplified
	
	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()
	
	s.cpuHistory = append(s.cpuHistory, cpuUsage)
	
	// Keep only the window size
	windowSize := int(s.cpuWindow.Seconds())
	if len(s.cpuHistory) > windowSize {
		s.cpuHistory = s.cpuHistory[len(s.cpuHistory)-windowSize:]
	}
}

// waitForShutdown waits for shutdown signal
func (s *Server) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigCh
	s.logger.Info("Shutdown signal received")
	
	// Start graceful shutdown
	close(s.shutdownCh)
	
	// Stop accepting new requests
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
	
	// Wait for all goroutines to finish
	s.wg.Wait()
	
	s.logger.Info("Server shutdown complete")
}

// healthHandler handles HTTP health checks
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// metricsHandler handles metrics endpoint
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.cache.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Simple metrics output
	fmt.Fprintf(w, `{
		"cache_size": %v,
		"cache_capacity": %v,
		"cache_load": %v,
		"goroutines": %d,
		"concurrent_requests": %d
	}`, 
		stats["size"], 
		stats["capacity"], 
		stats["load"],
		runtime.NumGoroutine(),
		s.config.MaxConcurrent-s.semaphore.Available())
}

// Get implements the Get RPC
func (s *Server) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}
	
	value, found := s.cache.Get(req.Key)
	
	return &proto.GetResponse{
		Value: value,
		Found: found,
	}, nil
}

// Set implements the Set RPC
func (s *Server) Set(ctx context.Context, req *proto.SetRequest) (*proto.SetResponse, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}
	
	var ttl time.Duration
	if req.Ttl != nil {
		ttl = req.Ttl.AsDuration()
	}
	
	s.cache.Set(req.Key, req.Value, ttl)
	
	return &proto.SetResponse{
		Success: true,
	}, nil
}

// Delete implements the Delete RPC
func (s *Server) Delete(ctx context.Context, req *proto.DeleteRequest) (*proto.DeleteResponse, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}
	
	deleted := s.cache.Delete(req.Key)
	
	return &proto.DeleteResponse{
		Deleted: deleted,
	}, nil
}

// Health implements the Health RPC
func (s *Server) Health(ctx context.Context, req *proto.HealthRequest) (*proto.HealthResponse, error) {
	if ctx.Err() != nil {
		return nil, status.Error(codes.Canceled, "request canceled")
	}
	
	return &proto.HealthResponse{
		Healthy: true,
		Status:  "healthy",
	}, nil
} 