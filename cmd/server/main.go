package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/shard-cache/internal/server"
)

func main() {
	var (
		grpcPort      = flag.Int("grpc-port", 8080, "gRPC server port")
		httpPort      = flag.Int("http-port", 8081, "HTTP server port")
		cacheCapacity = flag.Int("cache-capacity", 10000, "Cache capacity")
		maxConcurrent = flag.Int64("max-concurrent", 1000, "Maximum concurrent requests")
		cpuThreshold  = flag.Float64("cpu-threshold", 0.9, "CPU threshold for load shedding")
		cpuWindow     = flag.Duration("cpu-window", 10*time.Second, "CPU monitoring window")
	)
	flag.Parse()
	
	config := &server.Config{
		GRPCPort:      *grpcPort,
		HTTPPort:      *httpPort,
		CacheCapacity: *cacheCapacity,
		MaxConcurrent: *maxConcurrent,
		CPUThreshold:  *cpuThreshold,
		CPUWindow:     *cpuWindow,
	}
	
	srv, err := server.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	
	fmt.Printf("Starting cache server on gRPC port %d, HTTP port %d\n", *grpcPort, *httpPort)
	
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
} 