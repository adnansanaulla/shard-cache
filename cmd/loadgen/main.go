package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/shard-cache/internal/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Get node addresses from environment
	node1Addr := getEnv("NODE1_ADDR", "localhost:8080")
	node2Addr := getEnv("NODE2_ADDR", "localhost:8082")
	node3Addr := getEnv("NODE3_ADDR", "localhost:8084")

	// Create client
	clientConfig := &client.Config{
		ReadQuorum:   2,
		WriteQuorum:  2,
		HedgeTimeout: 100 * time.Millisecond,
		HedgeRatio:   0.1,
	}

	c, err := client.NewClient(clientConfig)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Add nodes
	if err := c.AddNode("node1", node1Addr); err != nil {
		log.Printf("Failed to add node1: %v", err)
	}
	if err := c.AddNode("node2", node2Addr); err != nil {
		log.Printf("Failed to add node2: %v", err)
	}
	if err := c.AddNode("node3", node3Addr); err != nil {
		log.Printf("Failed to add node3: %v", err)
	}

	log.Printf("Load generator started with nodes: %s, %s, %s", node1Addr, node2Addr, node3Addr)

	// Run load test
	runLoadTest(c)
}

func runLoadTest(c *client.Client) {
	const (
		numGoroutines = 10
		duration      = 60 * time.Second
		keys          = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	start := time.Now()

	// Start worker goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker(ctx, c, workerID, keys)
		}(i)
	}

	// Wait for completion
	wg.Wait()
	elapsed := time.Since(start)

	log.Printf("Load test completed in %v", elapsed)
}

func worker(ctx context.Context, c *client.Client, workerID, numKeys int) {
	operations := 0
	errors := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d completed: %d operations, %d errors", workerID, operations, errors)
			return
		default:
			// Generate random key
			key := fmt.Sprintf("key-%d-%d", workerID, rand.Intn(numKeys))
			value := []byte(fmt.Sprintf("value-%d-%d", workerID, operations))

			// Random operation: 80% reads, 20% writes
			if rand.Float64() < 0.8 {
				// Read operation
				_, err := c.Get(ctx, key)
				if err != nil {
					errors++
				}
			} else {
				// Write operation
				err := c.Set(ctx, key, value, 0)
				if err != nil {
					errors++
				}
			}

			operations++

			// Small delay to avoid overwhelming the system
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
} 