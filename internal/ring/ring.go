package ring

import (
	"crypto/md5"
	"encoding/binary"
	"sort"
	"sync"
)

// Node represents a cache node in the ring
type Node struct {
	ID   string
	Addr string
}

// Ring implements consistent hashing using rendezvous hashing
type Ring struct {
	mu    sync.RWMutex
	nodes map[string]*Node
}

// NewRing creates a new ring
func NewRing() *Ring {
	return &Ring{
		nodes: make(map[string]*Node),
	}
}

// AddNode adds a node to the ring
func (r *Ring) AddNode(id, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[id] = &Node{ID: id, Addr: addr}
}

// RemoveNode removes a node from the ring
func (r *Ring) RemoveNode(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, id)
}

// GetNodes returns all nodes in the ring
func (r *Ring) GetNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Owners returns the top N nodes responsible for a key using rendezvous hashing
func (r *Ring) Owners(key string, n int) []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if len(r.nodes) == 0 {
		return nil
	}
	
	if n > len(r.nodes) {
		n = len(r.nodes)
	}
	
	// Calculate hash scores for all nodes
	type nodeScore struct {
		node  *Node
		score uint64
	}
	
	scores := make([]nodeScore, 0, len(r.nodes))
	for _, node := range r.nodes {
		score := r.hash(key + node.ID)
		scores = append(scores, nodeScore{node: node, score: score})
	}
	
	// Sort by score (highest first for rendezvous hashing)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	
	// Return top N nodes
	result := make([]*Node, n)
	for i := 0; i < n; i++ {
		result[i] = scores[i].node
	}
	
	return result
}

// hash computes a hash for rendezvous hashing
func (r *Ring) hash(input string) uint64 {
	h := md5.Sum([]byte(input))
	return binary.BigEndian.Uint64(h[:8])
}

// NodeCount returns the number of nodes in the ring
func (r *Ring) NodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
} 