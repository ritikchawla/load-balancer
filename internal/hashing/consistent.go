package hashing

import (
	"hash/crc32"
	"sort"
	"sync"
)

const replicationFactor = 100

type ConsistentHasher struct {
	mu      sync.RWMutex
	hash    map[uint32]string
	nodes   []uint32
	weights map[string]int
}

// New creates a new ConsistentHasher instance
func New() *ConsistentHasher {
	return &ConsistentHasher{
		hash:    make(map[uint32]string),
		nodes:   make([]uint32, 0),
		weights: make(map[string]int),
	}
}

// Add adds a node to the hash ring with optional weight
func (c *ConsistentHasher) Add(node string, weight int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.weights[node] = weight
	for i := 0; i < replicationFactor*weight; i++ {
		hash := c.hashKey(node + string(rune(i)))
		c.hash[hash] = node
		c.nodes = append(c.nodes, hash)
	}
	sort.Slice(c.nodes, func(i, j int) bool {
		return c.nodes[i] < c.nodes[j]
	})
}

// Remove removes a node from the hash ring
func (c *ConsistentHasher) Remove(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	weight := c.weights[node]
	delete(c.weights, node)

	for i := 0; i < replicationFactor*weight; i++ {
		hash := c.hashKey(node + string(rune(i)))
		delete(c.hash, hash)
	}

	nodes := make([]uint32, 0)
	for _, v := range c.nodes {
		if c.hash[v] != node {
			nodes = append(nodes, v)
		}
	}
	c.nodes = nodes
}

// Get returns the node that a key hashes to
func (c *ConsistentHasher) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.nodes) == 0 {
		return ""
	}

	hash := c.hashKey(key)
	idx := sort.Search(len(c.nodes), func(i int) bool {
		return c.nodes[i] >= hash
	})

	if idx == len(c.nodes) {
		idx = 0
	}

	return c.hash[c.nodes[idx]]
}

// hashKey generates a hash for a key
func (c *ConsistentHasher) hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}
