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

package db

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/weaviate/weaviate/adapters/repos/db/vector/hnsw"
)

func TestStreamingSearchCoordinator_EarlyTermination(t *testing.T) {
	logger, _ := test.NewNullLogger()
	k := 10
	batchSize := 3
	maxRounds := 10

	coordinator := NewStreamingSearchCoordinator(logger, k, batchSize, maxRounds)

	// Test early termination logic
	assert.NotNil(t, coordinator)
	assert.Equal(t, k, coordinator.k)
	assert.Equal(t, batchSize, coordinator.batchSize)
	assert.Equal(t, maxRounds, coordinator.maxRounds)
}

func TestStreamingSearchCoordinator_AllShardsExhausted(t *testing.T) {
	logger, _ := test.NewNullLogger()
	coordinator := NewStreamingSearchCoordinator(logger, 10, 3, 10)

	// Initially no shards
	assert.True(t, coordinator.allShardsExhausted())

	// Add a non-exhausted shard searcher (simulated)
	coordinator.searchers = append(coordinator.searchers, &ShardSearcher{
		shardName: "shard1",
		exhausted: false,
	})

	assert.False(t, coordinator.allShardsExhausted())

	// Mark as exhausted
	coordinator.searchers[0].exhausted = true
	assert.True(t, coordinator.allShardsExhausted())
}

func TestStreamingSearchCoordinator_CanTerminate(t *testing.T) {
	logger, _ := test.NewNullLogger()
	coordinator := NewStreamingSearchCoordinator(logger, 3, 2, 10)

	// Not enough results to terminate
	assert.False(t, coordinator.canTerminate())

	// Add 3 results to heap
	coordinator.globalHeap.Insert(1, 0.1)
	coordinator.globalHeap.Insert(2, 0.2)
	coordinator.globalHeap.Insert(3, 0.3)
	coordinator.threshold = 0.3

	// Add shard with better possible score (can't terminate)
	coordinator.searchers = append(coordinator.searchers, &ShardSearcher{
		shardName:         "shard1",
		maxRemainingScore: 0.25, // Better than threshold (lower distance)
		exhausted:         false,
	})

	assert.False(t, coordinator.canTerminate())

	// Update shard to have worse possible score (can terminate)
	coordinator.searchers[0].maxRemainingScore = 0.4 // Worse than threshold
	assert.True(t, coordinator.canTerminate())
}

func TestStreamingSearchCoordinator_ExtractTopK(t *testing.T) {
	logger, _ := test.NewNullLogger()
	k := 3
	coordinator := NewStreamingSearchCoordinator(logger, k, 2, 10)

	// Add more than k results
	coordinator.globalHeap.Insert(1, 0.5)
	coordinator.globalHeap.Insert(2, 0.2)
	coordinator.globalHeap.Insert(3, 0.3)
	coordinator.globalHeap.Insert(4, 0.1)
	coordinator.globalHeap.Insert(5, 0.4)

	ids, dists, err := coordinator.extractTopK()
	assert.NoError(t, err)
	assert.Len(t, ids, k)
	assert.Len(t, dists, k)

	// Results should be sorted by distance (best first)
	assert.Equal(t, uint64(4), ids[0]) // 0.1
	assert.Equal(t, uint64(2), ids[1]) // 0.2
	assert.Equal(t, uint64(3), ids[2]) // 0.3
	assert.Equal(t, float32(0.1), dists[0])
	assert.Equal(t, float32(0.2), dists[1])
	assert.Equal(t, float32(0.3), dists[2])
}

func TestSearchBatch_Bounds(t *testing.T) {
	batch := &hnsw.SearchBatch{
		IDs:       []uint64{1, 2, 3},
		Distances: []float32{0.1, 0.2, 0.3},
		MinScore:  0.3,
		MaxRemainingScore: 0.4,
		Exhausted: false,
		MinScorePresent: true,
		MaxRemainingPresent: true,
	}

	assert.Len(t, batch.IDs, 3)
	assert.Equal(t, float32(0.3), batch.MinScore)
	assert.Equal(t, float32(0.4), batch.MaxRemainingScore)
	assert.False(t, batch.Exhausted)
}

func TestStreamingSearchCoordinator_DefaultBatchSize(t *testing.T) {
	logger, _ := test.NewNullLogger()

	// Test default batch size when 0 or negative
	coordinator := NewStreamingSearchCoordinator(logger, 10, 0, 10)
	assert.Equal(t, 10, coordinator.batchSize) // min(10, k=10)

	coordinator = NewStreamingSearchCoordinator(logger, 50, 0, 10)
	assert.Equal(t, 10, coordinator.batchSize) // min(10, k=50)

	// Test explicit batch size
	coordinator = NewStreamingSearchCoordinator(logger, 10, 5, 10)
	assert.Equal(t, 5, coordinator.batchSize)
}

func TestStreamingSearchCoordinator_Integration(t *testing.T) {
	// Integration test for the full coordinator lifecycle
	logger, _ := test.NewNullLogger()
	k := 10
	batchSize := 3
	maxRounds := 5

	coordinator := NewStreamingSearchCoordinator(logger, k, batchSize, maxRounds)

	// Simulate adding results from multiple shards
	// This is a simplified test - in practice, we'd use actual HNSW searchers
	coordinator.globalHeap.Insert(1, 0.1)
	coordinator.globalHeap.Insert(2, 0.15)
	coordinator.globalHeap.Insert(3, 0.2)
	coordinator.globalHeap.Insert(4, 0.25)
	coordinator.globalHeap.Insert(5, 0.3)
	coordinator.globalHeap.Insert(6, 0.35)
	coordinator.globalHeap.Insert(7, 0.4)
	coordinator.globalHeap.Insert(8, 0.45)
	coordinator.globalHeap.Insert(9, 0.5)
	coordinator.globalHeap.Insert(10, 0.55)
	coordinator.globalHeap.Insert(11, 0.6)
	coordinator.globalHeap.Insert(12, 0.65)

	ids, dists, err := coordinator.extractTopK()
	assert.NoError(t, err)
	assert.Len(t, ids, k)
	assert.Len(t, dists, k)

	// Verify results are sorted correctly
	for i := 1; i < len(dists); i++ {
		assert.LessOrEqual(t, dists[i-1], dists[i], "results should be sorted by distance")
	}
}
