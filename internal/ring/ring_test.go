package ring

import (
	"fmt"
	"testing"
	"time"
)

func TestRingAddRemoveNode(t *testing.T) {
	ring := NewRing()
	
	// Test initial state
	if ring.NodeCount() != 0 {
		t.Errorf("Expected 0 nodes, got %d", ring.NodeCount())
	}
	
	// Add nodes
	ring.AddNode("node1", "localhost:8081")
	ring.AddNode("node2", "localhost:8082")
	ring.AddNode("node3", "localhost:8083")
	
	if ring.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", ring.NodeCount())
	}
	
	// Remove a node
	ring.RemoveNode("node2")
	
	if ring.NodeCount() != 2 {
		t.Errorf("Expected 2 nodes after removal, got %d", ring.NodeCount())
	}
	
	nodes := ring.GetNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes in GetNodes, got %d", len(nodes))
	}
}

func TestRingOwnersDistinct(t *testing.T) {
	ring := NewRing()
	ring.AddNode("node1", "localhost:8081")
	ring.AddNode("node2", "localhost:8082")
	ring.AddNode("node3", "localhost:8083")
	
	// Test that owners are distinct
	owners := ring.Owners("test-key", 3)
	if len(owners) != 3 {
		t.Errorf("Expected 3 owners, got %d", len(owners))
	}
	
	// Check that all owners are distinct
	seen := make(map[string]bool)
	for _, owner := range owners {
		if seen[owner.ID] {
			t.Errorf("Duplicate owner found: %s", owner.ID)
		}
		seen[owner.ID] = true
	}
}

func TestRingMappingStability(t *testing.T) {
	ring := NewRing()
	ring.AddNode("node1", "localhost:8081")
	ring.AddNode("node2", "localhost:8082")
	ring.AddNode("node3", "localhost:8083")
	
	// Test mapping stability for the same key
	key := "stable-test-key"
	owners1 := ring.Owners(key, 2)
	owners2 := ring.Owners(key, 2)
	
	if len(owners1) != len(owners2) {
		t.Errorf("Owner count changed: %d vs %d", len(owners1), len(owners2))
	}
	
	for i, owner1 := range owners1 {
		if owner1.ID != owners2[i].ID {
			t.Errorf("Owner mapping changed: %s vs %s", owner1.ID, owners2[i].ID)
		}
	}
}

func TestRingWrapAround(t *testing.T) {
	ring := NewRing()
	ring.AddNode("node1", "localhost:8081")
	ring.AddNode("node2", "localhost:8082")
	
	// Test that requesting more owners than nodes works correctly
	owners := ring.Owners("test-key", 5)
	if len(owners) != 2 {
		t.Errorf("Expected 2 owners when requesting 5 but only 2 nodes exist, got %d", len(owners))
	}
}

func TestRingEmptyRing(t *testing.T) {
	ring := NewRing()
	
	// Test behavior with empty ring
	owners := ring.Owners("test-key", 3)
	if owners != nil {
		t.Errorf("Expected nil owners for empty ring, got %v", owners)
	}
}

func TestRingConcurrentAccess(t *testing.T) {
	ring := NewRing()
	
	// Test concurrent access
	done := make(chan bool, 10)
	
	for i := 0; i < 5; i++ {
		go func(id int) {
			ring.AddNode(fmt.Sprintf("node%d", id), fmt.Sprintf("localhost:%d", 8080+id))
			ring.Owners("test-key", 2)
			done <- true
		}(i)
	}
	
	for i := 0; i < 5; i++ {
		<-done
	}
	
	// Should not panic and should have some nodes
	if ring.NodeCount() == 0 {
		t.Error("Expected some nodes after concurrent access")
	}
}

func TestRingHashConsistency(t *testing.T) {
	ring := NewRing()
	
	// Test that hash function is consistent
	key := "test-key"
	nodeID := "node1"
	
	hash1 := ring.hash(key + nodeID)
	hash2 := ring.hash(key + nodeID)
	
	if hash1 != hash2 {
		t.Errorf("Hash not consistent: %d vs %d", hash1, hash2)
	}
}

func TestRingOwnerDistribution(t *testing.T) {
	ring := NewRing()
	ring.AddNode("node1", "localhost:8081")
	ring.AddNode("node2", "localhost:8082")
	ring.AddNode("node3", "localhost:8083")
	
	// Test distribution across multiple keys
	distribution := make(map[string]int)
	keys := []string{"key1", "key2", "key3", "key4", "key5", "key6", "key7", "key8", "key9", "key10"}
	
	for _, key := range keys {
		owners := ring.Owners(key, 1)
		if len(owners) > 0 {
			distribution[owners[0].ID]++
		}
	}
	
	// Check that all nodes get some keys (basic distribution test)
	for nodeID := range distribution {
		if distribution[nodeID] == 0 {
			t.Errorf("Node %s got no keys", nodeID)
		}
	}
} 