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
	"sync/atomic"
)

// CacheStats tracks statistics for the tiered cache
type CacheStats struct {
	L1Hits     atomic.Int64
	L2Hits     atomic.Int64
	L3Hits     atomic.Int64
	Misses     atomic.Int64
	Promotions atomic.Int64
	Evictions  atomic.Int64

	// Prefetching stats
	PrefetchRequests atomic.Int64
	PrefetchHits     atomic.Int64
}

// NewCacheStats creates a new CacheStats instance
func NewCacheStats() *CacheStats {
	return &CacheStats{}
}

// TotalHits returns the total number of cache hits across all tiers
func (s *CacheStats) TotalHits() int64 {
	return s.L1Hits.Load() + s.L2Hits.Load() + s.L3Hits.Load()
}

// TotalRequests returns the total number of cache requests
func (s *CacheStats) TotalRequests() int64 {
	return s.TotalHits() + s.Misses.Load()
}

// HitRate returns the overall cache hit rate (0.0 to 1.0)
func (s *CacheStats) HitRate() float64 {
	total := s.TotalRequests()
	if total == 0 {
		return 0.0
	}
	return float64(s.TotalHits()) / float64(total)
}

// L1HitRate returns the L1 cache hit rate relative to total requests
func (s *CacheStats) L1HitRate() float64 {
	total := s.TotalRequests()
	if total == 0 {
		return 0.0
	}
	return float64(s.L1Hits.Load()) / float64(total)
}

// L2HitRate returns the L2 cache hit rate relative to total requests
func (s *CacheStats) L2HitRate() float64 {
	total := s.TotalRequests()
	if total == 0 {
		return 0.0
	}
	return float64(s.L2Hits.Load()) / float64(total)
}

// L3HitRate returns the L3 cache hit rate relative to total requests
func (s *CacheStats) L3HitRate() float64 {
	total := s.TotalRequests()
	if total == 0 {
		return 0.0
	}
	return float64(s.L3Hits.Load()) / float64(total)
}

// PrefetchAccuracy returns the accuracy of prefetching (0.0 to 1.0)
func (s *CacheStats) PrefetchAccuracy() float64 {
	requests := s.PrefetchRequests.Load()
	if requests == 0 {
		return 0.0
	}
	return float64(s.PrefetchHits.Load()) / float64(requests)
}

// Reset clears all statistics
func (s *CacheStats) Reset() {
	s.L1Hits.Store(0)
	s.L2Hits.Store(0)
	s.L3Hits.Store(0)
	s.Misses.Store(0)
	s.Promotions.Store(0)
	s.Evictions.Store(0)
	s.PrefetchRequests.Store(0)
	s.PrefetchHits.Store(0)
}
