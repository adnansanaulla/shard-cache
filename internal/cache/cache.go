package cache

import (
	"sync"
	"time"
)

// Entry represents a cache entry
type Entry struct {
	Key       string
	Value     []byte
	ExpiresAt time.Time
	Prev      *Entry
	Next      *Entry
}

// Cache implements an LRU cache with TTL support
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]*Entry
	head     *Entry // Most recently used
	tail     *Entry // Least recently used
	capacity int
	size     int
}

// NewCache creates a new cache with the specified capacity
func NewCache(capacity int) *Cache {
	cache := &Cache{
		entries:  make(map[string]*Entry),
		capacity: capacity,
	}
	return cache
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		c.removeEntry(entry)
		return nil, false
	}
	
	// Move to front (most recently used)
	c.moveToFront(entry)
	
	return entry.Value, true
}

// Set stores a value in the cache
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if key already exists
	if existing, exists := c.entries[key]; exists {
		// Update existing entry
		existing.Value = value
		if ttl > 0 {
			existing.ExpiresAt = time.Now().Add(ttl)
		} else {
			existing.ExpiresAt = time.Time{}
		}
		c.moveToFront(existing)
		return
	}
	
	// Create new entry
	entry := &Entry{
		Key:   key,
		Value: value,
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}
	
	// Add to map
	c.entries[key] = entry
	
	// Add to front of list
	c.addToFront(entry)
	c.size++
	
	// Evict if necessary
	if c.size > c.capacity {
		c.evictLRU()
	}
}

// Delete removes a key from the cache
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	entry, exists := c.entries[key]
	if !exists {
		return false
	}
	
	c.removeEntry(entry)
	return true
}

// Size returns the current number of entries
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

// Capacity returns the cache capacity
func (c *Cache) Capacity() int {
	return c.capacity
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*Entry)
	c.head = nil
	c.tail = nil
	c.size = 0
}

// Cleanup removes expired entries
func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	removed := 0
	now := time.Now()
	
	// Iterate through entries and remove expired ones
	for key, entry := range c.entries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			c.removeEntry(entry)
			removed++
		}
	}
	
	return removed
}

// moveToFront moves an entry to the front of the LRU list
func (c *Cache) moveToFront(entry *Entry) {
	if entry == c.head {
		return // Already at front
	}
	
	// Remove from current position
	if entry.Prev != nil {
		entry.Prev.Next = entry.Next
	}
	if entry.Next != nil {
		entry.Next.Prev = entry.Prev
	}
	if entry == c.tail {
		c.tail = entry.Prev
	}
	
	// Add to front
	c.addToFront(entry)
}

// addToFront adds an entry to the front of the LRU list
func (c *Cache) addToFront(entry *Entry) {
	entry.Prev = nil
	entry.Next = c.head
	
	if c.head != nil {
		c.head.Prev = entry
	}
	c.head = entry
	
	if c.tail == nil {
		c.tail = entry
	}
}

// removeEntry removes an entry from the cache
func (c *Cache) removeEntry(entry *Entry) {
	// Remove from map
	delete(c.entries, entry.Key)
	
	// Remove from list
	if entry.Prev != nil {
		entry.Prev.Next = entry.Next
	} else {
		c.head = entry.Next
	}
	
	if entry.Next != nil {
		entry.Next.Prev = entry.Prev
	} else {
		c.tail = entry.Prev
	}
	
	c.size--
}

// evictLRU removes the least recently used entry
func (c *Cache) evictLRU() {
	if c.tail != nil {
		c.removeEntry(c.tail)
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]interface{}{
		"size":     c.size,
		"capacity": c.capacity,
		"load":     float64(c.size) / float64(c.capacity),
	}
} 