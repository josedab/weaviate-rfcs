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
	"context"
	"math"

	"github.com/pkg/errors"
	"github.com/weaviate/weaviate/adapters/repos/db/helpers"
	"github.com/weaviate/weaviate/adapters/repos/db/priorityqueue"
	"github.com/weaviate/weaviate/adapters/repos/db/vector/compressionhelpers"
)

// SearchBatch represents a batch of search results with bounds for early termination
type SearchBatch struct {
	IDs                 []uint64
	Distances           []float32
	MinScore            float32 // Score of the last result in this batch
	MaxRemainingScore   float32 // Upper bound on unseen results
	Exhausted           bool    // No more results available
	TotalResults        uint32  // Total results available
	MinScorePresent     bool
	MaxRemainingPresent bool
}

// StreamingSearcher provides streaming search capabilities with bounds estimation
type StreamingSearcher struct {
	hnsw      *hnsw
	ctx       context.Context
	searchVec []float32
	k         int
	batchSize int
	allowList helpers.AllowList

	// Internal state for iterative search
	allResults        *priorityqueue.Queue[any]
	currentOffset     int
	totalResultsFound int
	searchComplete    bool
	compressorDist    compressionhelpers.CompressorDistancer
	returnFn          compressionhelpers.ReturnDistancerFn
}

// NewStreamingSearcher creates a new streaming searcher
func (h *hnsw) NewStreamingSearcher(ctx context.Context, vector []float32, k int, batchSize int, allowList helpers.AllowList) (*StreamingSearcher, error) {
	if batchSize <= 0 {
		batchSize = min(10, k) // Default batch size
	}

	vector = h.normalizeVec(vector)

	var compressorDist compressionhelpers.CompressorDistancer
	var returnFn compressionhelpers.ReturnDistancerFn
	if h.compressed.Load() {
		compressorDist, returnFn = h.compressor.NewDistancer(vector)
	}

	return &StreamingSearcher{
		hnsw:           h,
		ctx:            ctx,
		searchVec:      vector,
		k:              k,
		batchSize:      batchSize,
		allowList:      allowList,
		compressorDist: compressorDist,
		returnFn:       returnFn,
	}, nil
}

// Close releases resources
func (s *StreamingSearcher) Close() {
	if s.returnFn != nil {
		s.returnFn()
	}
	if s.allResults != nil {
		s.hnsw.pools.pqResults.Put(s.allResults)
	}
}

// NextBatch returns the next batch of results with bounds
func (s *StreamingSearcher) NextBatch() (*SearchBatch, error) {
	if s.searchComplete {
		return &SearchBatch{
			Exhausted: true,
		}, nil
	}

	// First call: perform the full search
	if s.allResults == nil {
		if err := s.performFullSearch(); err != nil {
			return nil, err
		}
	}

	// Calculate how many results to return in this batch
	remainingResults := s.totalResultsFound - s.currentOffset
	if remainingResults <= 0 {
		s.searchComplete = true
		return &SearchBatch{
			Exhausted: true,
		}, nil
	}

	batchCount := min(s.batchSize, remainingResults)
	batch := &SearchBatch{
		IDs:          make([]uint64, batchCount),
		Distances:    make([]float32, batchCount),
		TotalResults: uint32(s.totalResultsFound),
	}

	// Extract results from the priority queue
	// We need to convert the queue to a slice, take a batch, and keep the rest
	tempResults := make([]priorityqueue.Item[any], s.allResults.Len())
	idx := 0
	for s.allResults.Len() > 0 {
		tempResults[idx] = s.allResults.Pop()
		idx++
	}

	// Reverse to get correct order (closest first)
	for i := 0; i < len(tempResults)/2; i++ {
		tempResults[i], tempResults[len(tempResults)-1-i] = tempResults[len(tempResults)-1-i], tempResults[i]
	}

	// Take batch from current offset
	for i := 0; i < batchCount; i++ {
		resultIdx := s.currentOffset + i
		batch.IDs[i] = tempResults[resultIdx].ID
		batch.Distances[i] = tempResults[resultIdx].Dist
	}

	// Put remaining results back in queue
	for i := s.currentOffset + batchCount; i < len(tempResults); i++ {
		s.allResults.Insert(tempResults[i].ID, tempResults[i].Dist)
	}

	// Set bounds
	if batchCount > 0 {
		batch.MinScore = batch.Distances[batchCount-1]
		batch.MinScorePresent = true
	}

	// Estimate max remaining score
	if s.currentOffset+batchCount < s.totalResultsFound {
		// There are more results, estimate the max remaining
		batch.MaxRemainingScore = s.estimateMaxRemaining(tempResults, s.currentOffset+batchCount)
		batch.MaxRemainingPresent = true
	} else {
		// No more results
		batch.Exhausted = true
		s.searchComplete = true
		batch.MaxRemainingScore = math.MaxFloat32
		batch.MaxRemainingPresent = false
	}

	s.currentOffset += batchCount

	return batch, nil
}

// performFullSearch executes the full HNSW search
func (s *StreamingSearcher) performFullSearch() error {
	s.hnsw.compressActionLock.RLock()
	defer s.hnsw.compressActionLock.RUnlock()

	// Use a larger ef to get more candidates than k
	ef := s.hnsw.searchTimeEF(s.k)

	// Perform the search (reusing existing logic)
	ids, dists, err := s.hnsw.knnSearchByVector(s.ctx, s.searchVec, s.k, ef, s.allowList)
	if err != nil {
		return errors.Wrap(err, "streaming search: knn search failed")
	}

	// Store results in priority queue
	s.allResults = priorityqueue.NewMin[any](len(ids))
	for i := range ids {
		s.allResults.Insert(ids[i], dists[i])
	}

	s.totalResultsFound = len(ids)
	s.currentOffset = 0

	return nil
}

// estimateMaxRemaining estimates the maximum score for remaining unseen results
// This is a conservative estimate based on the distance distribution
func (s *StreamingSearcher) estimateMaxRemaining(allResults []priorityqueue.Item[any], fromIndex int) float32 {
	if fromIndex >= len(allResults) {
		return math.MaxFloat32 // No remaining results
	}

	// Conservative estimate: return the best distance in the remaining results
	// This ensures we don't prematurely terminate
	bestRemaining := allResults[fromIndex].Dist
	for i := fromIndex + 1; i < len(allResults); i++ {
		if allResults[i].Dist < bestRemaining {
			bestRemaining = allResults[i].Dist
		}
	}

	return bestRemaining
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
