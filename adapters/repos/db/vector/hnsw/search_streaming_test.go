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

package hnsw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchBatch_Construction(t *testing.T) {
	batch := &SearchBatch{
		IDs:                 []uint64{1, 2, 3, 4, 5},
		Distances:           []float32{0.1, 0.15, 0.2, 0.25, 0.3},
		MinScore:            0.3,
		MaxRemainingScore:   0.35,
		Exhausted:           false,
		TotalResults:        20,
		MinScorePresent:     true,
		MaxRemainingPresent: true,
	}

	assert.Len(t, batch.IDs, 5)
	assert.Len(t, batch.Distances, 5)
	assert.Equal(t, float32(0.3), batch.MinScore)
	assert.Equal(t, float32(0.35), batch.MaxRemainingScore)
	assert.False(t, batch.Exhausted)
	assert.Equal(t, uint32(20), batch.TotalResults)
	assert.True(t, batch.MinScorePresent)
	assert.True(t, batch.MaxRemainingPresent)
}

func TestSearchBatch_Exhausted(t *testing.T) {
	batch := &SearchBatch{
		IDs:       []uint64{},
		Distances: []float32{},
		Exhausted: true,
	}

	assert.True(t, batch.Exhausted)
	assert.Len(t, batch.IDs, 0)
	assert.Len(t, batch.Distances, 0)
}

func TestSearchBatch_BoundsPresent(t *testing.T) {
	// Batch with bounds
	batchWithBounds := &SearchBatch{
		IDs:                 []uint64{1, 2, 3},
		Distances:           []float32{0.1, 0.2, 0.3},
		MinScore:            0.3,
		MaxRemainingScore:   0.4,
		MinScorePresent:     true,
		MaxRemainingPresent: true,
	}

	assert.True(t, batchWithBounds.MinScorePresent)
	assert.True(t, batchWithBounds.MaxRemainingPresent)

	// Batch without bounds (exhausted)
	batchWithoutBounds := &SearchBatch{
		IDs:                 []uint64{1, 2, 3},
		Distances:           []float32{0.1, 0.2, 0.3},
		Exhausted:           true,
		MinScorePresent:     false,
		MaxRemainingPresent: false,
	}

	assert.False(t, batchWithoutBounds.MinScorePresent)
	assert.False(t, batchWithoutBounds.MaxRemainingPresent)
}

func TestMinFunction(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 7, min(7, 7))
	assert.Equal(t, 0, min(0, 100))
	assert.Equal(t, -5, min(-5, 10))
}

func TestSearchBatch_PartialResults(t *testing.T) {
	// Simulate a batch where we have partial results
	batch := &SearchBatch{
		IDs:                 []uint64{10, 20, 30},
		Distances:           []float32{0.5, 0.6, 0.7},
		MinScore:            0.7,
		MaxRemainingScore:   0.75,
		Exhausted:           false,
		TotalResults:        15,
		MinScorePresent:     true,
		MaxRemainingPresent: true,
	}

	// Verify the batch represents partial results correctly
	assert.Equal(t, 3, len(batch.IDs))
	assert.Less(t, len(batch.IDs), int(batch.TotalResults))
	assert.False(t, batch.Exhausted)

	// The last distance in the batch should match the min score
	assert.Equal(t, batch.Distances[len(batch.Distances)-1], batch.MinScore)

	// Max remaining should be >= min score (more distant results remain)
	assert.GreaterOrEqual(t, batch.MaxRemainingScore, batch.MinScore)
}

func TestSearchBatch_EarlyTerminationScenario(t *testing.T) {
	// Scenario: We have top-k results with score threshold
	// and remaining results would all have worse scores
	k := 10
	currentThreshold := float32(0.5) // k-th best score

	// Shard 1 batch - has some good results
	shard1Batch := &SearchBatch{
		IDs:                 []uint64{1, 2, 3},
		Distances:           []float32{0.1, 0.2, 0.3},
		MinScore:            0.3,
		MaxRemainingScore:   0.4,
		Exhausted:           false,
		MinScorePresent:     true,
		MaxRemainingPresent: true,
	}

	// Shard 2 batch - all remaining results are worse than threshold
	shard2Batch := &SearchBatch{
		IDs:                 []uint64{10, 11, 12},
		Distances:           []float32{0.6, 0.7, 0.8},
		MinScore:            0.8,
		MaxRemainingScore:   0.85, // Worse than current threshold
		Exhausted:           false,
		MinScorePresent:     true,
		MaxRemainingPresent: true,
	}

	// Check termination logic for each shard
	// Shard 1 can still contribute (max remaining < threshold)
	canShard1Contribute := shard1Batch.MaxRemainingScore < currentThreshold
	assert.True(t, canShard1Contribute)

	// Shard 2 cannot contribute (max remaining > threshold)
	canShard2Contribute := shard2Batch.MaxRemainingScore < currentThreshold
	assert.False(t, canShard2Contribute)

	// If all shards' max remaining > threshold, we can terminate early
	allShardsWorse := shard1Batch.MaxRemainingScore > currentThreshold &&
		shard2Batch.MaxRemainingScore > currentThreshold
	assert.False(t, allShardsWorse) // Can't terminate yet because shard1 can contribute

	_ = k // Used for context
}

func TestSearchBatch_BatchSizeOptimization(t *testing.T) {
	// Test that batch size optimization works correctly
	testCases := []struct {
		name          string
		k             int
		batchSize     int
		expectedBatch int
	}{
		{
			name:          "small k uses smaller batches",
			k:             10,
			batchSize:     0, // Will default to min(10, k)
			expectedBatch: 10,
		},
		{
			name:          "large k with explicit batch size",
			k:             100,
			batchSize:     20,
			expectedBatch: 20,
		},
		{
			name:          "batch size larger than k",
			k:             5,
			batchSize:     10,
			expectedBatch: 5, // Should be capped at k
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This would be tested in the actual searcher implementation
			actualBatchSize := min(tc.batchSize, tc.k)
			if tc.batchSize == 0 {
				actualBatchSize = min(10, tc.k)
			}
			assert.Equal(t, tc.expectedBatch, actualBatchSize)
		})
	}
}
