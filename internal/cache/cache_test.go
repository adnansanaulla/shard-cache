package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCacheBasicOperations(t *testing.T) {
	cache := NewCache(100)
	
	// Test Set and Get
	key := "test-key"
	value := []byte("test-value")
	
	cache.Set(key, value, 0)
	
	retrieved, exists := cache.Get(key)
	if !exists {
		t.Error("Expected key to exist")
	}
	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(retrieved))
	}
	
	// Test non-existent key
	_, exists = cache.Get("non-existent")
	if exists {
		t.Error("Expected key to not exist")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cache := NewCache(100)
	
	key := "ttl-test"
	value := []byte("ttl-value")
	
	// Set with short TTL
	cache.Set(key, value, 10*time.Millisecond)
	
	// Should exist immediately
	_, exists := cache.Get(key)
	if !exists {
		t.Error("Expected key to exist immediately")
	}
	
	// Wait for expiry
	time.Sleep(20 * time.Millisecond)
	
	// Should not exist after expiry
	_, exists = cache.Get(key)
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewCache(3)
	
	// Add 4 items to trigger eviction
	cache.Set("key1", []byte("value1"), 0)
	cache.Set("key2", []byte("value2"), 0)
	cache.Set("key3", []byte("value3"), 0)
	cache.Set("key4", []byte("value4"), 0)
	
	// key1 should be evicted (LRU)
	_, exists := cache.Get("key1")
	if exists {
		t.Error("Expected key1 to be evicted")
	}
	
	// Other keys should still exist
	_, exists = cache.Get("key2")
	if !exists {
		t.Error("Expected key2 to exist")
	}
	
	_, exists = cache.Get("key3")
	if !exists {
		t.Error("Expected key3 to exist")
	}
	
	_, exists = cache.Get("key4")
	if !exists {
		t.Error("Expected key4 to exist")
	}
}

func TestCacheLRUOrder(t *testing.T) {
	cache := NewCache(3)
	
	// Add 3 items
	cache.Set("key1", []byte("value1"), 0)
	cache.Set("key2", []byte("value2"), 0)
	cache.Set("key3", []byte("value3"), 0)
	
	// Access key1 to make it most recently used
	cache.Get("key1")
	
	// Add a new key, should evict key2 (least recently used)
	cache.Set("key4", []byte("value4"), 0)
	
	// key2 should be evicted
	_, exists := cache.Get("key2")
	if exists {
		t.Error("Expected key2 to be evicted")
	}
	
	// key1, key3, key4 should exist
	_, exists = cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	
	_, exists = cache.Get("key3")
	if !exists {
		t.Error("Expected key3 to exist")
	}
	
	_, exists = cache.Get("key4")
	if !exists {
		t.Error("Expected key4 to exist")
	}
}

func TestCacheDelete(t *testing.T) {
	cache := NewCache(100)
	
	key := "delete-test"
	value := []byte("delete-value")
	
	cache.Set(key, value, 0)
	
	// Verify it exists
	_, exists := cache.Get(key)
	if !exists {
		t.Error("Expected key to exist before deletion")
	}
	
	// Delete it
	deleted := cache.Delete(key)
	if !deleted {
		t.Error("Expected deletion to succeed")
	}
	
	// Verify it's gone
	_, exists = cache.Get(key)
	if exists {
		t.Error("Expected key to not exist after deletion")
	}
	
	// Try to delete non-existent key
	deleted = cache.Delete("non-existent")
	if deleted {
		t.Error("Expected deletion of non-existent key to fail")
	}
}

func TestCacheUpdate(t *testing.T) {
	cache := NewCache(100)
	
	key := "update-test"
	value1 := []byte("value1")
	value2 := []byte("value2")
	
	// Set initial value
	cache.Set(key, value1, 0)
	
	// Update value
	cache.Set(key, value2, 0)
	
	// Verify updated value
	retrieved, exists := cache.Get(key)
	if !exists {
		t.Error("Expected key to exist after update")
	}
	if string(retrieved) != string(value2) {
		t.Errorf("Expected %s, got %s", string(value2), string(retrieved))
	}
	
	// Size should still be 1
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}
}

func TestCacheConcurrency(t *testing.T) {
	cache := NewCache(1000)
	
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100
	
	// Hammer test with concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := []byte(fmt.Sprintf("value-%d-%d", id, j))
				
				// Set
				cache.Set(key, value, 0)
				
				// Get
				cache.Get(key)
				
				// Occasionally delete
				if j%10 == 0 {
					cache.Delete(key)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Should not panic and should have reasonable size
	size := cache.Size()
	if size < 0 || size > cache.Capacity() {
		t.Errorf("Invalid cache size: %d", size)
	}
}

func TestCacheCleanup(t *testing.T) {
	cache := NewCache(100)
	
	// Add some entries with TTL
	cache.Set("expired1", []byte("value1"), 1*time.Millisecond)
	cache.Set("expired2", []byte("value2"), 1*time.Millisecond)
	cache.Set("valid", []byte("value3"), 0) // No TTL
	
	// Wait for expiry
	time.Sleep(10 * time.Millisecond)
	
	// Cleanup should remove expired entries
	removed := cache.Cleanup()
	if removed != 2 {
		t.Errorf("Expected 2 expired entries to be removed, got %d", removed)
	}
	
	// Valid entry should still exist
	_, exists := cache.Get("valid")
	if !exists {
		t.Error("Expected valid entry to still exist after cleanup")
	}
	
	// Expired entries should be gone
	_, exists = cache.Get("expired1")
	if exists {
		t.Error("Expected expired1 to be removed")
	}
	
	_, exists = cache.Get("expired2")
	if exists {
		t.Error("Expected expired2 to be removed")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(100)
	
	// Add some entries
	cache.Set("key1", []byte("value1"), 0)
	cache.Set("key2", []byte("value2"), 0)
	
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}
	
	// Clear cache
	cache.Clear()
	
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
	
	// Entries should not exist
	_, exists := cache.Get("key1")
	if exists {
		t.Error("Expected key1 to not exist after clear")
	}
	
	_, exists = cache.Get("key2")
	if exists {
		t.Error("Expected key2 to not exist after clear")
	}
}

func TestCacheStats(t *testing.T) {
	cache := NewCache(100)
	
	// Add some entries
	cache.Set("key1", []byte("value1"), 0)
	cache.Set("key2", []byte("value2"), 0)
	
	stats := cache.GetStats()
	
	if stats["size"] != 2 {
		t.Errorf("Expected size 2, got %v", stats["size"])
	}
	
	if stats["capacity"] != 100 {
		t.Errorf("Expected capacity 100, got %v", stats["capacity"])
	}
	
	load := stats["load"].(float64)
	expectedLoad := 2.0 / 100.0
	if load != expectedLoad {
		t.Errorf("Expected load %f, got %f", expectedLoad, load)
	}
} 