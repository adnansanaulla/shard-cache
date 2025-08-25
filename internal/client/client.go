package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shard-cache/internal/ring"
	"github.com/shard-cache/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Client represents a distributed cache client
type Client struct {
	ring       *ring.Ring
	logger     *zap.Logger
	connections map[string]*grpc.ClientConn
	connMutex   sync.RWMutex
	
	// Quorum settings
	readQuorum  int
	writeQuorum int
	
	// Hedging settings
	hedgeTimeout time.Duration
	hedgeRatio   float64
}

// Config holds client configuration
type Config struct {
	ReadQuorum   int
	WriteQuorum  int
	HedgeTimeout time.Duration
	HedgeRatio   float64
}

// NewClient creates a new distributed cache client
func NewClient(config *Config) (*Client, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	
	client := &Client{
		ring:         ring.NewRing(),
		logger:       logger,
		connections:  make(map[string]*grpc.ClientConn),
		readQuorum:   config.ReadQuorum,
		writeQuorum:  config.WriteQuorum,
		hedgeTimeout: config.HedgeTimeout,
		hedgeRatio:   config.HedgeRatio,
	}
	
	return client, nil
}

// AddNode adds a node to the client's ring
func (c *Client) AddNode(id, addr string) error {
	c.ring.AddNode(id, addr)
	
	// Create connection
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	
	c.connMutex.Lock()
	c.connections[id] = conn
	c.connMutex.Unlock()
	
	c.logger.Info("Added node", zap.String("id", id), zap.String("addr", addr))
	return nil
}

// RemoveNode removes a node from the client's ring
func (c *Client) RemoveNode(id string) {
	c.ring.RemoveNode(id)
	
	c.connMutex.Lock()
	if conn, exists := c.connections[id]; exists {
		conn.Close()
		delete(c.connections, id)
	}
	c.connMutex.Unlock()
	
	c.logger.Info("Removed node", zap.String("id", id))
}

// Get retrieves a value using quorum reads
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	owners := c.ring.Owners(key, c.readQuorum)
	if len(owners) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}
	
	// Try to get from primary owner first
	primary := owners[0]
	value, err := c.getFromNode(ctx, primary.ID, key)
	if err == nil {
		return value, nil
	}
	
	// If primary fails, try other owners
	for i := 1; i < len(owners); i++ {
		value, err := c.getFromNode(ctx, owners[i].ID, key)
		if err == nil {
			return value, nil
		}
	}
	
	return nil, fmt.Errorf("failed to get key from any node")
}

// Set stores a value using quorum writes
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	owners := c.ring.Owners(key, c.writeQuorum)
	if len(owners) == 0 {
		return fmt.Errorf("no nodes available")
	}
	
	// Send to all owners concurrently
	results := make(chan error, len(owners))
	for _, owner := range owners {
		go func(owner *ring.Node) {
			results <- c.setToNode(ctx, owner.ID, key, value, ttl)
		}(owner)
	}
	
	// Wait for quorum
	successes := 0
	for i := 0; i < len(owners); i++ {
		if err := <-results; err == nil {
			successes++
		}
	}
	
	if successes >= c.writeQuorum {
		return nil
	}
	
	return fmt.Errorf("failed to write to quorum of nodes")
}

// Delete removes a key using quorum writes
func (c *Client) Delete(ctx context.Context, key string) error {
	owners := c.ring.Owners(key, c.writeQuorum)
	if len(owners) == 0 {
		return fmt.Errorf("no nodes available")
	}
	
	// Send to all owners concurrently
	results := make(chan error, len(owners))
	for _, owner := range owners {
		go func(owner *ring.Node) {
			results <- c.deleteFromNode(ctx, owner.ID, key)
		}(owner)
	}
	
	// Wait for quorum
	successes := 0
	for i := 0; i < len(owners); i++ {
		if err := <-results; err == nil {
			successes++
		}
	}
	
	if successes >= c.writeQuorum {
		return nil
	}
	
	return fmt.Errorf("failed to delete from quorum of nodes")
}

// getFromNode gets a value from a specific node
func (c *Client) getFromNode(ctx context.Context, nodeID, key string) ([]byte, error) {
	conn, err := c.getConnection(nodeID)
	if err != nil {
		return nil, err
	}
	
	client := proto.NewCacheServiceClient(conn)
	
	// Apply hedging if configured
	if c.hedgeTimeout > 0 {
		ctx, cancel := context.WithTimeout(ctx, c.hedgeTimeout)
		defer cancel()
		
		// Start hedge request after a delay
		hedgeCh := make(chan []byte, 1)
		go func() {
			time.Sleep(c.hedgeTimeout / 2)
			if value, err := c.getFromNodeWithRetry(ctx, client, key); err == nil {
				hedgeCh <- value
			}
		}()
		
		// Try primary request
		if value, err := c.getFromNodeWithRetry(ctx, client, key); err == nil {
			return value, nil
		}
		
		// Try hedge request
		select {
		case value := <-hedgeCh:
			return value, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return c.getFromNodeWithRetry(ctx, client, key)
}

// getFromNodeWithRetry gets a value with retry logic
func (c *Client) getFromNodeWithRetry(ctx context.Context, client proto.CacheServiceClient, key string) ([]byte, error) {
	resp, err := client.Get(ctx, &proto.GetRequest{Key: key})
	if err != nil {
		return nil, err
	}
	
	if !resp.Found {
		return nil, fmt.Errorf("key not found")
	}
	
	return resp.Value, nil
}

// setToNode sets a value to a specific node
func (c *Client) setToNode(ctx context.Context, nodeID, key string, value []byte, ttl time.Duration) error {
	conn, err := c.getConnection(nodeID)
	if err != nil {
		return err
	}
	
	client := proto.NewCacheServiceClient(conn)
	
	var protoTTL *durationpb.Duration
	if ttl > 0 {
		protoTTL = durationpb.New(ttl)
	}
	
	resp, err := client.Set(ctx, &proto.SetRequest{
		Key:   key,
		Value: value,
		Ttl:   protoTTL,
	})
	if err != nil {
		return err
	}
	
	if !resp.Success {
		return fmt.Errorf("set operation failed")
	}
	
	return nil
}

// deleteFromNode deletes a key from a specific node
func (c *Client) deleteFromNode(ctx context.Context, nodeID, key string) error {
	conn, err := c.getConnection(nodeID)
	if err != nil {
		return err
	}
	
	client := proto.NewCacheServiceClient(conn)
	
	resp, err := client.Delete(ctx, &proto.DeleteRequest{Key: key})
	if err != nil {
		return err
	}
	
	if !resp.Deleted {
		return fmt.Errorf("delete operation failed")
	}
	
	return nil
}

// getConnection gets or creates a connection to a node
func (c *Client) getConnection(nodeID string) (*grpc.ClientConn, error) {
	c.connMutex.RLock()
	conn, exists := c.connections[nodeID]
	c.connMutex.RUnlock()
	
	if exists {
		return conn, nil
	}
	
	return nil, fmt.Errorf("no connection to node %s", nodeID)
}

// Close closes all connections
func (c *Client) Close() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	
	for id, conn := range c.connections {
		conn.Close()
		delete(c.connections, id)
	}
	
	return nil
}

// GetStats returns client statistics
func (c *Client) GetStats() map[string]interface{} {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	
	return map[string]interface{}{
		"nodes":         c.ring.NodeCount(),
		"connections":   len(c.connections),
		"read_quorum":   c.readQuorum,
		"write_quorum":  c.writeQuorum,
		"hedge_timeout": c.hedgeTimeout,
		"hedge_ratio":   c.hedgeRatio,
	}
} 