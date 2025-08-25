package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/shard-cache/internal/client"
	"github.com/shard-cache/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestE2EQuorumLogic tests the complete distributed cache with quorum logic
func TestE2EQuorumLogic(t *testing.T) {
	// Start 3 embedded servers
	servers := make([]*Server, 3)
	ports := []int{8080, 8082, 8084}
	
	for i, port := range ports {
		config := &Config{
			GRPCPort:      port,
			HTTPPort:      port + 1,
			CacheCapacity: 1000,
			MaxConcurrent: 100,
			CPUThreshold:  0.9,
			CPUWindow:     10 * time.Second,
		}
		
		server, err := NewServer(config)
		if err != nil {
			t.Fatalf("Failed to create server %d: %v", i, err)
		}
		
		// Start server in background
		go func(s *Server) {
			if err := s.Start(); err != nil {
				t.Errorf("Server failed: %v", err)
			}
		}(server)
		
		servers[i] = server
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
	}
	
	// Create client
	clientConfig := &client.Config{
		ReadQuorum:   2,
		WriteQuorum:  2,
		HedgeTimeout: 100 * time.Millisecond,
		HedgeRatio:   0.1,
	}
	
	c, err := client.NewClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()
	
	// Add nodes to client
	for i, port := range ports {
		nodeID := fmt.Sprintf("node%d", i)
		addr := fmt.Sprintf("localhost:%d", port)
		if err := c.AddNode(nodeID, addr); err != nil {
			t.Fatalf("Failed to add node %s: %v", nodeID, err)
		}
	}
	
	// Test basic operations
	t.Run("BasicOperations", func(t *testing.T) {
		ctx := context.Background()
		
		// Test Set
		key := "test-key"
		value := []byte("test-value")
		
		if err := c.Set(ctx, key, value, 0); err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
		
		// Test Get
		retrieved, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get key: %v", err)
		}
		
		if string(retrieved) != string(value) {
			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
		}
		
		// Test Delete
		if err := c.Delete(ctx, key); err != nil {
			t.Fatalf("Failed to delete key: %v", err)
		}
		
		// Verify deletion
		_, err = c.Get(ctx, key)
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})
	
	// Test quorum behavior with node failure
	t.Run("QuorumWithNodeFailure", func(t *testing.T) {
		ctx := context.Background()
		
		// Set a key
		key := "quorum-test"
		value := []byte("quorum-value")
		
		if err := c.Set(ctx, key, value, 0); err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
		
		// Kill one node (simulate failure)
		t.Log("Killing node 0...")
		if servers[0].grpcServer != nil {
			servers[0].grpcServer.Stop()
		}
		c.RemoveNode("node0")
		
		// Wait a bit for failure to propagate
		time.Sleep(200 * time.Millisecond)
		
		// Should still be able to read (quorum of 2 with 2 nodes remaining)
		retrieved, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get key after node failure: %v", err)
		}
		
		if string(retrieved) != string(value) {
			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
		}
		
		// Should be able to write (quorum of 2 with 2 nodes remaining)
		newValue := []byte("new-quorum-value")
		if err := c.Set(ctx, key, newValue, 0); err != nil {
			t.Fatalf("Failed to set key after node failure: %v", err)
		}
		
		// Verify the new value
		retrieved, err = c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get updated key: %v", err)
		}
		
		if string(retrieved) != string(newValue) {
			t.Errorf("Expected %s, got %s", string(newValue), string(retrieved))
		}
	})
	
	// Test TTL functionality
	t.Run("TTLFunctionality", func(t *testing.T) {
		ctx := context.Background()
		
		key := "ttl-test"
		value := []byte("ttl-value")
		
		// Set with short TTL
		if err := c.Set(ctx, key, value, 100*time.Millisecond); err != nil {
			t.Fatalf("Failed to set key with TTL: %v", err)
		}
		
		// Should exist immediately
		retrieved, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get key immediately: %v", err)
		}
		
		if string(retrieved) != string(value) {
			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
		}
		
		// Wait for expiry
		time.Sleep(200 * time.Millisecond)
		
		// Should not exist after expiry
		_, err = c.Get(ctx, key)
		if err == nil {
			t.Error("Expected error after TTL expiry")
		}
	})
	
	// Test concurrent operations
	t.Run("ConcurrentOperations", func(t *testing.T) {
		ctx := context.Background()
		const numGoroutines = 10
		const operationsPerGoroutine = 10
		
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*operationsPerGoroutine)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					key := fmt.Sprintf("concurrent-%d-%d", id, j)
					value := []byte(fmt.Sprintf("value-%d-%d", id, j))
					
					// Set
					if err := c.Set(ctx, key, value, 0); err != nil {
						errors <- fmt.Errorf("set failed for %s: %v", key, err)
						return
					}
					
					// Get
					retrieved, err := c.Get(ctx, key)
					if err != nil {
						errors <- fmt.Errorf("get failed for %s: %v", key, err)
						return
					}
					
					if string(retrieved) != string(value) {
						errors <- fmt.Errorf("value mismatch for %s", key)
						return
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent operation error: %v", err)
		}
	})
	
	// Cleanup
	for _, server := range servers {
		if server.grpcServer != nil {
			server.grpcServer.Stop()
		}
	}
}

// TestE2ESingleNode tests single node behavior
func TestE2ESingleNode(t *testing.T) {
	// Start single server
	config := &Config{
		GRPCPort:      8080,
		HTTPPort:      8081,
		CacheCapacity: 1000,
		MaxConcurrent: 100,
		CPUThreshold:  0.9,
		CPUWindow:     10 * time.Second,
	}
	
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	
	// Start server in background
	go func() {
		if err := server.Start(); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Create direct gRPC client for testing
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	client := proto.NewCacheServiceClient(conn)
	ctx := context.Background()
	
	// Test basic operations
	key := "single-test"
	value := []byte("single-value")
	
	// Set
	setResp, err := client.Set(ctx, &proto.SetRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	
	if !setResp.Success {
		t.Error("Set response indicates failure")
	}
	
	// Get
	getResp, err := client.Get(ctx, &proto.GetRequest{Key: key})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	
	if !getResp.Found {
		t.Error("Get response indicates key not found")
	}
	
	if string(getResp.Value) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(getResp.Value))
	}
	
	// Delete
	deleteResp, err := client.Delete(ctx, &proto.DeleteRequest{Key: key})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	if !deleteResp.Deleted {
		t.Error("Delete response indicates failure")
	}
	
	// Verify deletion
	getResp, err = client.Get(ctx, &proto.GetRequest{Key: key})
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	
	if getResp.Found {
		t.Error("Key should not be found after deletion")
	}
	
	// Cleanup
	if server.grpcServer != nil {
		server.grpcServer.Stop()
	}
} 