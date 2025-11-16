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
	"container/list"
	"sync"
)

// L1Cache implements an LRU (Least Recently Used) cache for hot vectors
// Stores uncompressed vectors for zero-overhead access
type L1Cache struct {
	mu       sync.RWMutex
	capacity int
	items    map[uint64]*list.Element
	lruList  *list.List
}

// l1Entry represents an entry in the L1 cache
type l1Entry struct {
	id     uint64
	vector []float32
}

// NewL1Cache creates a new L1 cache with the specified capacity
func NewL1Cache(capacity int) *L1Cache {
	return &L1Cache{
		capacity: capacity,
		items:    make(map[uint64]*list.Element),
		lruList:  list.New(),
	}
}

// Get retrieves a vector from the L1 cache
// Returns the vector and true if found, nil and false otherwise
func (c *L1Cache) Get(id uint64) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[id]; exists {
		// Move to front (most recently used)
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*l1Entry)
		return entry.vector, true
	}
	return nil, false
}

// Set adds or updates a vector in the L1 cache
// Evicts the least recently used item if at capacity
func (c *L1Cache) Set(id uint64, vector []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If item already exists, update it and move to front
	if elem, exists := c.items[id]; exists {
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*l1Entry)
		entry.vector = vector
		return
	}

	// Add new item
	entry := &l1Entry{
		id:     id,
		vector: vector,
	}
	elem := c.lruList.PushFront(entry)
	c.items[id] = elem

	// Evict least recently used if over capacity
	if c.lruList.Len() > c.capacity {
		c.evictOldest()
	}
}

// Delete removes a vector from the L1 cache
func (c *L1Cache) Delete(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[id]; exists {
		c.lruList.Remove(elem)
		delete(c.items, id)
	}
}

// evictOldest removes the least recently used item
// Must be called with lock held
func (c *L1Cache) evictOldest() {
	elem := c.lruList.Back()
	if elem != nil {
		c.lruList.Remove(elem)
		entry := elem.Value.(*l1Entry)
		delete(c.items, entry.id)
	}
}

// Len returns the current number of items in the cache
func (c *L1Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}

// Clear removes all items from the cache
func (c *L1Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[uint64]*list.Element)
	c.lruList = list.New()
}

// Contains checks if a vector ID exists in the cache
func (c *L1Cache) Contains(id uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.items[id]
	return exists
}
