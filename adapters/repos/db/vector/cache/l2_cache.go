//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package cache

import (
	"container/heap"
	"sync"
)

// L2Cache implements an LFU (Least Frequently Used) cache for warm vectors
// Can store vectors in compressed or uncompressed form
type L2Cache struct {
	mu          sync.RWMutex
	capacity    int
	items       map[uint64]*l2Entry
	freqHeap    *frequencyHeap
	compressed  bool
	minFreq     int
	currentTime int64
}

// l2Entry represents an entry in the L2 cache
type l2Entry struct {
	id        uint64
	vector    []float32  // Uncompressed vector
	frequency int        // Access frequency
	heapIndex int        // Index in the heap
	timestamp int64      // For tie-breaking in LFU
}

// frequencyHeap implements a min-heap for LFU eviction
type frequencyHeap []*l2Entry

func (h frequencyHeap) Len() int { return len(h) }

func (h frequencyHeap) Less(i, j int) bool {
	// First compare by frequency
	if h[i].frequency != h[j].frequency {
		return h[i].frequency < h[j].frequency
	}
	// If frequencies are equal, use timestamp (older first)
	return h[i].timestamp < h[j].timestamp
}

func (h frequencyHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *frequencyHeap) Push(x interface{}) {
	entry := x.(*l2Entry)
	entry.heapIndex = len(*h)
	*h = append(*h, entry)
}

func (h *frequencyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.heapIndex = -1
	*h = old[0 : n-1]
	return entry
}

// NewL2Cache creates a new L2 cache with the specified capacity
func NewL2Cache(capacity int, compressed bool) *L2Cache {
	h := make(frequencyHeap, 0)
	heap.Init(&h)

	return &L2Cache{
		capacity:    capacity,
		items:       make(map[uint64]*l2Entry),
		freqHeap:    &h,
		compressed:  compressed,
		currentTime: 0,
	}
}

// Get retrieves a vector from the L2 cache
// Returns the vector and true if found, nil and false otherwise
func (c *L2Cache) Get(id uint64) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.items[id]; exists {
		// Increment frequency
		entry.frequency++
		entry.timestamp = c.currentTime
		c.currentTime++

		// Update heap
		heap.Fix(c.freqHeap, entry.heapIndex)

		return entry.vector, true
	}
	return nil, false
}

// Set adds or updates a vector in the L2 cache
// Evicts the least frequently used item if at capacity
func (c *L2Cache) Set(id uint64, vector []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If item already exists, update it
	if entry, exists := c.items[id]; exists {
		entry.vector = vector
		entry.frequency++
		entry.timestamp = c.currentTime
		c.currentTime++
		heap.Fix(c.freqHeap, entry.heapIndex)
		return
	}

	// Add new item
	entry := &l2Entry{
		id:        id,
		vector:    vector,
		frequency: 1,
		timestamp: c.currentTime,
	}
	c.currentTime++

	c.items[id] = entry
	heap.Push(c.freqHeap, entry)

	// Evict least frequently used if over capacity
	if len(c.items) > c.capacity {
		c.evictLeastFrequent()
	}
}

// Delete removes a vector from the L2 cache
func (c *L2Cache) Delete(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.items[id]; exists {
		heap.Remove(c.freqHeap, entry.heapIndex)
		delete(c.items, id)
	}
}

// evictLeastFrequent removes the least frequently used item
// Must be called with lock held
func (c *L2Cache) evictLeastFrequent() {
	if c.freqHeap.Len() > 0 {
		entry := heap.Pop(c.freqHeap).(*l2Entry)
		delete(c.items, entry.id)
	}
}

// Len returns the current number of items in the cache
func (c *L2Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear removes all items from the cache
func (c *L2Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[uint64]*l2Entry)
	h := make(frequencyHeap, 0)
	heap.Init(&h)
	c.freqHeap = &h
	c.currentTime = 0
}

// Contains checks if a vector ID exists in the cache
func (c *L2Cache) Contains(id uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.items[id]
	return exists
}

// GetFrequency returns the access frequency of a vector
func (c *L2Cache) GetFrequency(id uint64) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, exists := c.items[id]; exists {
		return entry.frequency
	}
	return 0
}
