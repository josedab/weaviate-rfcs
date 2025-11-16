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
	"context"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/adapters/repos/db/vector/common"
	"github.com/weaviate/weaviate/usecases/memwatch"
)

// TieredCache implements a three-tier cache system (L1/L2/L3)
// L1: Hot cache (LRU, uncompressed)
// L2: Warm cache (LFU, optionally compressed)
// L3: Cold cache (all vectors, delegates to storage)
type TieredCache struct {
	l1 *L1Cache
	l2 *L2Cache
	l3 *L3Cache

	config          *TieredCacheConfig
	stats           *CacheStats
	accessCounter   *CountMinSketch
	maxSize         int64
	count           int64
	logger          logrus.FieldLogger
	allocChecker    memwatch.AllocChecker
	normalizeOnRead bool

	prefetcher *Prefetcher
}

// NewTieredCache creates a new tiered cache
func NewTieredCache(
	vectorForID common.VectorForID[float32],
	maxSize int,
	config *TieredCacheConfig,
	logger logrus.FieldLogger,
	normalizeOnRead bool,
	allocChecker memwatch.AllocChecker,
) Cache[float32] {
	if config == nil {
		config = DefaultTieredCacheConfig()
	}

	// Calculate tier capacities
	l1Capacity := int(float64(maxSize) * config.L1Ratio)
	l2Capacity := int(float64(maxSize) * config.L2Ratio)

	// Ensure minimum capacities
	if l1Capacity < 100 {
		l1Capacity = 100
	}
	if l2Capacity < 300 {
		l2Capacity = 300
	}

	// Create tiers
	l1 := NewL1Cache(l1Capacity)
	l2 := NewL2Cache(l2Capacity, config.L2Compressed)
	l3 := NewL3Cache(vectorForID)

	// Create access counter for promotion decisions
	accessCounter := NewCountMinSketch(1000, 4)

	tc := &TieredCache{
		l1:              l1,
		l2:              l2,
		l3:              l3,
		config:          config,
		stats:           NewCacheStats(),
		accessCounter:   accessCounter,
		maxSize:         int64(maxSize),
		count:           0,
		logger:          logger,
		allocChecker:    allocChecker,
		normalizeOnRead: normalizeOnRead,
	}

	// Initialize prefetcher if enabled
	if config.Prefetching != nil && config.Prefetching.Enabled {
		tc.prefetcher = NewPrefetcher(tc, config.Prefetching, logger)
		tc.prefetcher.Start(context.Background())
	}

	return tc
}

// Get retrieves a vector from the cache
func (tc *TieredCache) Get(ctx context.Context, id uint64) ([]float32, error) {
	// Try L1 first (hot cache)
	if vec, found := tc.l1.Get(id); found {
		tc.stats.L1Hits.Add(1)
		return vec, nil
	}

	// Try L2 (warm cache)
	if vec, found := tc.l2.Get(id); found {
		tc.stats.L2Hits.Add(1)

		// Track access and consider promotion to L1
		tc.accessCounter.Increment(id)
		if tc.accessCounter.Count(id) >= tc.config.PromotionThreshold {
			tc.l1.Set(id, vec)
			tc.stats.Promotions.Add(1)
		}

		return vec, nil
	}

	// Try L3 (cold cache - fetches from storage)
	vec, err := tc.l3.Get(ctx, id)
	if err != nil {
		tc.stats.Misses.Add(1)
		return nil, err
	}

	tc.stats.L3Hits.Add(1)

	// Add to L2 cache
	tc.l2.Set(id, vec)
	atomic.AddInt64(&tc.count, 1)

	return vec, nil
}

// MultiGet retrieves multiple vectors from the cache
func (tc *TieredCache) MultiGet(ctx context.Context, ids []uint64) ([][]float32, []error) {
	out := make([][]float32, len(ids))
	errs := make([]error, len(ids))

	for i, id := range ids {
		vec, err := tc.Get(ctx, id)
		out[i] = vec
		errs[i] = err
	}

	return out, errs
}

// Preload adds a vector to the cache without fetching from storage
func (tc *TieredCache) Preload(id uint64, vec []float32) {
	tc.l2.Set(id, vec)
	atomic.AddInt64(&tc.count, 1)
}

// PreloadNoLock is provided for compatibility but delegates to Preload
func (tc *TieredCache) PreloadNoLock(id uint64, vec []float32) {
	tc.Preload(id, vec)
}

// Delete removes a vector from all cache tiers
func (tc *TieredCache) Delete(ctx context.Context, id uint64) {
	tc.l1.Delete(id)
	tc.l2.Delete(id)
	atomic.AddInt64(&tc.count, -1)
}

// Len returns the total capacity of the cache
func (tc *TieredCache) Len() int32 {
	// Return total capacity across all tiers
	return int32(tc.l1.Len() + tc.l2.Len())
}

// CountVectors returns the number of vectors currently cached
func (tc *TieredCache) CountVectors() int64 {
	return atomic.LoadInt64(&tc.count)
}

// Prefetch hints that a vector will be needed soon
func (tc *TieredCache) Prefetch(id uint64) {
	// Track prefetch request
	tc.stats.PrefetchRequests.Add(1)

	// Check if already in cache
	if tc.l1.Contains(id) || tc.l2.Contains(id) {
		tc.stats.PrefetchHits.Add(1)
		return
	}

	// For now, prefetch is a hint only
	// Future: could trigger async load
}

// Drop clears all cache tiers
func (tc *TieredCache) Drop() {
	tc.l1.Clear()
	tc.l2.Clear()
	atomic.StoreInt64(&tc.count, 0)

	if tc.prefetcher != nil {
		tc.prefetcher.Stop()
	}
}

// UpdateMaxSize updates the maximum cache size
func (tc *TieredCache) UpdateMaxSize(size int64) {
	atomic.StoreInt64(&tc.maxSize, size)
	// Note: This doesn't resize the tiers dynamically
	// A full implementation would recalculate tier capacities
}

// CopyMaxSize returns the current maximum size
func (tc *TieredCache) CopyMaxSize() int64 {
	return atomic.LoadInt64(&tc.maxSize)
}

// Grow is a no-op for tiered cache (tiers manage their own size)
func (tc *TieredCache) Grow(size uint64) {
	// No-op: Tiered cache tiers are fixed size
}

// SetSizeAndGrowNoLock is a no-op for tiered cache
func (tc *TieredCache) SetSizeAndGrowNoLock(id uint64) {
	// No-op for tiered cache
}

// All returns all cached vectors (L1 + L2)
// Note: This is for compatibility and may not be efficient
func (tc *TieredCache) All() [][]float32 {
	// Not implemented for tiered cache
	// This method is not commonly used in the codebase
	return nil
}

// LockAll is a no-op for tiered cache (individual tiers have their own locks)
func (tc *TieredCache) LockAll() {
	// No-op: Tiered cache uses per-tier locks
}

// UnlockAll is a no-op for tiered cache
func (tc *TieredCache) UnlockAll() {
	// No-op: Tiered cache uses per-tier locks
}

// PageSize returns the page size (not applicable for tiered cache)
func (tc *TieredCache) PageSize() uint64 {
	return 1 // Default page size
}

// GetAllInCurrentLock retrieves all vectors in the current lock page
// Not applicable for tiered cache, returns empty results
func (tc *TieredCache) GetAllInCurrentLock(ctx context.Context, id uint64, out [][]float32, errs []error) ([][]float32, []error, uint64, uint64) {
	// Not implemented for tiered cache
	return out, errs, 0, 0
}

// Methods for multi-vector support (not implemented for tiered cache)

func (tc *TieredCache) GetDoc(ctx context.Context, docID uint64) ([][]float32, error) {
	panic("not implemented")
}

func (tc *TieredCache) GetKeys(id uint64) (uint64, uint64) {
	panic("not implemented")
}

func (tc *TieredCache) SetKeys(id uint64, docID uint64, relativeID uint64) {
	panic("not implemented")
}

func (tc *TieredCache) PreloadMulti(docID uint64, ids []uint64, vecs [][]float32) {
	panic("not implemented")
}

func (tc *TieredCache) PreloadPassage(id uint64, docID uint64, relativeID uint64, vec []float32) {
	panic("not implemented")
}

// Stats returns the cache statistics
func (tc *TieredCache) Stats() *CacheStats {
	return tc.stats
}
