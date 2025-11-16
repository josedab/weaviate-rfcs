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
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestL1Cache(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		cache := NewL1Cache(3)

		// Test Set and Get
		vec1 := []float32{1.0, 2.0, 3.0}
		cache.Set(1, vec1)

		retrieved, found := cache.Get(1)
		assert.True(t, found)
		assert.Equal(t, vec1, retrieved)

		// Test miss
		_, found = cache.Get(999)
		assert.False(t, found)
	})

	t.Run("LRU eviction", func(t *testing.T) {
		cache := NewL1Cache(2)

		vec1 := []float32{1.0}
		vec2 := []float32{2.0}
		vec3 := []float32{3.0}

		cache.Set(1, vec1)
		cache.Set(2, vec2)
		assert.Equal(t, 2, cache.Len())

		// Add third item, should evict first
		cache.Set(3, vec3)
		assert.Equal(t, 2, cache.Len())

		// First item should be evicted
		_, found := cache.Get(1)
		assert.False(t, found)

		// Second and third should still be present
		_, found = cache.Get(2)
		assert.True(t, found)
		_, found = cache.Get(3)
		assert.True(t, found)
	})

	t.Run("LRU update on access", func(t *testing.T) {
		cache := NewL1Cache(2)

		vec1 := []float32{1.0}
		vec2 := []float32{2.0}
		vec3 := []float32{3.0}

		cache.Set(1, vec1)
		cache.Set(2, vec2)

		// Access first item to make it recent
		cache.Get(1)

		// Add third item, should evict second (not first)
		cache.Set(3, vec3)

		// First should still be present
		_, found := cache.Get(1)
		assert.True(t, found)

		// Second should be evicted
		_, found = cache.Get(2)
		assert.False(t, found)
	})

	t.Run("delete", func(t *testing.T) {
		cache := NewL1Cache(3)

		vec1 := []float32{1.0}
		cache.Set(1, vec1)

		assert.True(t, cache.Contains(1))
		cache.Delete(1)
		assert.False(t, cache.Contains(1))
	})

	t.Run("clear", func(t *testing.T) {
		cache := NewL1Cache(3)

		cache.Set(1, []float32{1.0})
		cache.Set(2, []float32{2.0})

		assert.Equal(t, 2, cache.Len())
		cache.Clear()
		assert.Equal(t, 0, cache.Len())
	})
}

func TestL2Cache(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		cache := NewL2Cache(3, false)

		// Test Set and Get
		vec1 := []float32{1.0, 2.0, 3.0}
		cache.Set(1, vec1)

		retrieved, found := cache.Get(1)
		assert.True(t, found)
		assert.Equal(t, vec1, retrieved)

		// Test miss
		_, found = cache.Get(999)
		assert.False(t, found)
	})

	t.Run("LFU eviction", func(t *testing.T) {
		cache := NewL2Cache(2, false)

		vec1 := []float32{1.0}
		vec2 := []float32{2.0}
		vec3 := []float32{3.0}

		cache.Set(1, vec1)
		cache.Set(2, vec2)

		// Access item 2 multiple times to increase frequency
		cache.Get(2)
		cache.Get(2)

		// Add third item, should evict item 1 (lowest frequency)
		cache.Set(3, vec3)

		// Item 1 should be evicted
		_, found := cache.Get(1)
		assert.False(t, found)

		// Items 2 and 3 should still be present
		_, found = cache.Get(2)
		assert.True(t, found)
		_, found = cache.Get(3)
		assert.True(t, found)
	})

	t.Run("frequency tracking", func(t *testing.T) {
		cache := NewL2Cache(5, false)

		vec1 := []float32{1.0}
		cache.Set(1, vec1)

		assert.Equal(t, 1, cache.GetFrequency(1))

		cache.Get(1)
		assert.Equal(t, 2, cache.GetFrequency(1))

		cache.Get(1)
		assert.Equal(t, 3, cache.GetFrequency(1))
	})

	t.Run("delete", func(t *testing.T) {
		cache := NewL2Cache(3, false)

		vec1 := []float32{1.0}
		cache.Set(1, vec1)

		assert.True(t, cache.Contains(1))
		cache.Delete(1)
		assert.False(t, cache.Contains(1))
	})
}

func TestCountMinSketch(t *testing.T) {
	t.Run("basic counting", func(t *testing.T) {
		cms := NewCountMinSketch(100, 4)

		// Initially zero
		assert.Equal(t, 0, cms.Count(1))

		// Increment and check
		cms.Increment(1)
		assert.Equal(t, 1, cms.Count(1))

		cms.Increment(1)
		assert.Equal(t, 2, cms.Count(1))
	})

	t.Run("multiple items", func(t *testing.T) {
		cms := NewCountMinSketch(100, 4)

		cms.Increment(1)
		cms.Increment(2)
		cms.Increment(1)

		assert.Equal(t, 2, cms.Count(1))
		assert.Equal(t, 1, cms.Count(2))
		assert.Equal(t, 0, cms.Count(3))
	})

	t.Run("reset", func(t *testing.T) {
		cms := NewCountMinSketch(100, 4)

		cms.Increment(1)
		cms.Increment(2)

		cms.Reset()

		assert.Equal(t, 0, cms.Count(1))
		assert.Equal(t, 0, cms.Count(2))
	})
}

func TestTieredCache(t *testing.T) {
	t.Run("L1 hit", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		// Mock vector storage
		storage := make(map[uint64][]float32)
		storage[1] = []float32{1.0, 2.0, 3.0}

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return storage[id], nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false // Disable prefetching for test

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Preload into L1
		vec1 := []float32{1.0, 2.0, 3.0}
		tc.l1.Set(1, vec1)

		// Get should hit L1
		retrieved, err := cache.Get(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, vec1, retrieved)
		assert.Equal(t, int64(1), tc.stats.L1Hits.Load())
		assert.Equal(t, int64(0), tc.stats.L2Hits.Load())
	})

	t.Run("L2 hit and promotion to L1", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		storage := make(map[uint64][]float32)
		storage[1] = []float32{1.0, 2.0, 3.0}

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return storage[id], nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false
		config.PromotionThreshold = 2

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Preload into L2
		vec1 := []float32{1.0, 2.0, 3.0}
		tc.l2.Set(1, vec1)

		// First access - should hit L2 but not promote
		_, err := cache.Get(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), tc.stats.L2Hits.Load())
		assert.False(t, tc.l1.Contains(1))

		// Second access - should promote to L1
		_, err = cache.Get(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(2), tc.stats.L2Hits.Load())
		assert.True(t, tc.l1.Contains(1))
		assert.Equal(t, int64(1), tc.stats.Promotions.Load())
	})

	t.Run("L3 hit and cache in L2", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		storage := make(map[uint64][]float32)
		storage[1] = []float32{1.0, 2.0, 3.0}

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return storage[id], nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Get should fetch from L3 and cache in L2
		retrieved, err := cache.Get(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, storage[1], retrieved)
		assert.Equal(t, int64(1), tc.stats.L3Hits.Load())
		assert.True(t, tc.l2.Contains(1))
	})

	t.Run("cache miss", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return nil, assert.AnError
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Get non-existent vector
		_, err := cache.Get(context.Background(), 999)
		assert.Error(t, err)
		assert.Equal(t, int64(1), tc.stats.Misses.Load())
	})

	t.Run("multi-get", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		storage := make(map[uint64][]float32)
		storage[1] = []float32{1.0}
		storage[2] = []float32{2.0}
		storage[3] = []float32{3.0}

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return storage[id], nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)

		vecs, errs := cache.MultiGet(context.Background(), []uint64{1, 2, 3})
		assert.Len(t, vecs, 3)
		assert.Len(t, errs, 3)

		for i, vec := range vecs {
			assert.NoError(t, errs[i])
			assert.Equal(t, storage[uint64(i+1)], vec)
		}
	})

	t.Run("delete", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return []float32{1.0}, nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Add to caches
		vec1 := []float32{1.0}
		tc.l1.Set(1, vec1)
		tc.l2.Set(1, vec1)

		// Delete
		cache.Delete(context.Background(), 1)

		assert.False(t, tc.l1.Contains(1))
		assert.False(t, tc.l2.Contains(1))
	})

	t.Run("drop", func(t *testing.T) {
		logger, _ := test.NewNullLogger()

		vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
			return []float32{1.0}, nil
		}

		config := DefaultTieredCacheConfig()
		config.Prefetching.Enabled = false

		cache := NewTieredCache(vecForID, 1000, config, logger, false, nil)
		tc := cache.(*TieredCache)

		// Add vectors
		tc.l1.Set(1, []float32{1.0})
		tc.l2.Set(2, []float32{2.0})

		// Drop
		cache.Drop()

		assert.Equal(t, 0, tc.l1.Len())
		assert.Equal(t, 0, tc.l2.Len())
		assert.Equal(t, int64(0), cache.CountVectors())
	})
}

func TestCacheStats(t *testing.T) {
	stats := NewCacheStats()

	stats.L1Hits.Add(100)
	stats.L2Hits.Add(50)
	stats.L3Hits.Add(30)
	stats.Misses.Add(20)

	assert.Equal(t, int64(180), stats.TotalHits())
	assert.Equal(t, int64(200), stats.TotalRequests())
	assert.InDelta(t, 0.9, stats.HitRate(), 0.01)
	assert.InDelta(t, 0.5, stats.L1HitRate(), 0.01)

	stats.Reset()
	assert.Equal(t, int64(0), stats.TotalHits())
}

func TestQueryPatternDetector(t *testing.T) {
	t.Run("temporal pattern detection", func(t *testing.T) {
		detector := NewQueryPatternDetector(true, false)

		// Record some accesses
		for i := 0; i < 10; i++ {
			detector.RecordAccess(1)
		}
		for i := 0; i < 5; i++ {
			detector.RecordAccess(2)
		}

		// Predict should return vectors based on current hour
		predictions := detector.PredictNext(10)
		assert.NotEmpty(t, predictions)
	})

	t.Run("spatial pattern detection", func(t *testing.T) {
		detector := NewQueryPatternDetector(false, true)

		// Record batch accesses (simulating neighbor co-access)
		detector.RecordBatchAccess([]uint64{1, 2, 3})
		detector.RecordBatchAccess([]uint64{1, 2, 4})
		detector.RecordBatchAccess([]uint64{1, 5, 6})

		// Predict should find co-accessed neighbors
		predictions := detector.PredictNext(10)
		assert.NotEmpty(t, predictions)
	})
}

// Benchmark tests
func BenchmarkL1Cache(b *testing.B) {
	cache := NewL1Cache(10000)

	// Preload
	for i := 0; i < 10000; i++ {
		cache.Set(uint64(i), []float32{float32(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(uint64(i % 10000))
	}
}

func BenchmarkL2Cache(b *testing.B) {
	cache := NewL2Cache(10000, false)

	// Preload
	for i := 0; i < 10000; i++ {
		cache.Set(uint64(i), []float32{float32(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(uint64(i % 10000))
	}
}

func BenchmarkTieredCache(b *testing.B) {
	logger, _ := test.NewNullLogger()

	storage := make(map[uint64][]float32)
	for i := 0; i < 100000; i++ {
		storage[uint64(i)] = []float32{float32(i)}
	}

	vecForID := func(ctx context.Context, id uint64) ([]float32, error) {
		return storage[id], nil
	}

	config := DefaultTieredCacheConfig()
	config.Prefetching.Enabled = false

	cache := NewTieredCache(vecForID, 10000, config, logger, false, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(context.Background(), uint64(i%100000))
	}
}
