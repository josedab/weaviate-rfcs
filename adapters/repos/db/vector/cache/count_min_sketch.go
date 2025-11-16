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
	"hash/fnv"
	"sync"
)

// CountMinSketch is a probabilistic data structure for frequency counting
// Uses multiple hash functions to provide approximate counts with small memory footprint
type CountMinSketch struct {
	mu      sync.RWMutex
	width   int     // Number of buckets per row
	depth   int     // Number of hash functions (rows)
	counts  [][]int // 2D array of counters
	hashSeeds []uint64
}

// NewCountMinSketch creates a new Count-Min Sketch
// width: number of buckets per row (larger = more accurate)
// depth: number of hash functions (larger = more accurate)
// Typical values: width=1000, depth=4 provides good accuracy for most use cases
func NewCountMinSketch(width, depth int) *CountMinSketch {
	counts := make([][]int, depth)
	for i := 0; i < depth; i++ {
		counts[i] = make([]int, width)
	}

	// Generate different seeds for each hash function
	seeds := make([]uint64, depth)
	goldenRatio := uint64(0x9e3779b97f4a7c15)
	for i := 0; i < depth; i++ {
		seeds[i] = uint64(i) * goldenRatio // Golden ratio-based seeds
	}

	return &CountMinSketch{
		width:     width,
		depth:     depth,
		counts:    counts,
		hashSeeds: seeds,
	}
}

// Increment increases the count for the given ID
func (cms *CountMinSketch) Increment(id uint64) {
	cms.mu.Lock()
	defer cms.mu.Unlock()

	for i := 0; i < cms.depth; i++ {
		hash := cms.hash(id, i)
		index := int(hash % uint64(cms.width))
		cms.counts[i][index]++
	}
}

// Count returns the approximate count for the given ID
// Returns the minimum count across all hash functions (conservative estimate)
func (cms *CountMinSketch) Count(id uint64) int {
	cms.mu.RLock()
	defer cms.mu.RUnlock()

	minCount := int(^uint(0) >> 1) // Max int
	for i := 0; i < cms.depth; i++ {
		hash := cms.hash(id, i)
		index := int(hash % uint64(cms.width))
		if cms.counts[i][index] < minCount {
			minCount = cms.counts[i][index]
		}
	}
	return minCount
}

// Reset clears all counts
func (cms *CountMinSketch) Reset() {
	cms.mu.Lock()
	defer cms.mu.Unlock()

	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			cms.counts[i][j] = 0
		}
	}
}

// hash computes a hash value for the given ID using the specified hash function index
func (cms *CountMinSketch) hash(id uint64, hashIndex int) uint64 {
	h := fnv.New64a()

	// Combine ID with seed
	value := id ^ cms.hashSeeds[hashIndex]

	// Write bytes to hash
	bytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		bytes[i] = byte(value >> (i * 8))
	}
	h.Write(bytes)

	return h.Sum64()
}
