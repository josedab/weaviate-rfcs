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
	"fmt"
	"math"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/adapters/repos/db/priorityqueue"
	"github.com/weaviate/weaviate/adapters/repos/db/vector/hnsw"
	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/storobj"
)

// StreamingSearchCoordinator coordinates streaming searches across multiple shards
// with early termination based on bounds
type StreamingSearchCoordinator struct {
	logger    logrus.FieldLogger
	k         int
	batchSize int

	// Shard searchers
	searchers []*ShardSearcher
	mu        sync.Mutex

	// Global state
	globalHeap        *priorityqueue.Queue[any]
	threshold         float32
	round             int
	maxRounds         int
	totalResultsFetch int
}

// ShardSearcher manages streaming search for a single shard
type ShardSearcher struct {
	shardName         string
	searcher          *hnsw.StreamingSearcher
	maxRemainingScore float32
	exhausted         bool
	mu                sync.Mutex
}

// NewStreamingSearchCoordinator creates a new streaming search coordinator
func NewStreamingSearchCoordinator(logger logrus.FieldLogger, k int, batchSize int, maxRounds int) *StreamingSearchCoordinator {
	if maxRounds <= 0 {
		maxRounds = 10 // Default max rounds to prevent infinite loops
	}
	if batchSize <= 0 {
		batchSize = min(10, k) // Default batch size
	}

	return &StreamingSearchCoordinator{
		logger:     logger,
		k:          k,
		batchSize:  batchSize,
		maxRounds:  maxRounds,
		globalHeap: priorityqueue.NewMin[any](k),
		threshold:  0,
	}
}

// AddShardSearcher adds a shard searcher to the coordinator
func (c *StreamingSearchCoordinator) AddShardSearcher(shardName string, searcher *hnsw.StreamingSearcher) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.searchers = append(c.searchers, &ShardSearcher{
		shardName:         shardName,
		searcher:          searcher,
		maxRemainingScore: 0,
		exhausted:         false,
	})
}

// Search performs the streaming search with early termination
func (c *StreamingSearchCoordinator) Search(ctx context.Context) ([]uint64, []float32, error) {
	if len(c.searchers) == 0 {
		return nil, nil, nil
	}

	// Iterative rounds of fetching
	for c.round = 0; c.round < c.maxRounds; c.round++ {
		// Fetch next batch from each non-exhausted shard
		if err := c.fetchRound(ctx); err != nil {
			return nil, nil, fmt.Errorf("round %d: %w", c.round, err)
		}

		// Check early termination condition
		if c.canTerminate() {
			c.logger.WithFields(logrus.Fields{
				"round":               c.round + 1,
				"total_results_fetch": c.totalResultsFetch,
				"k":                   c.k,
			}).Debug("early termination triggered")
			break
		}

		// Check if all shards are exhausted
		if c.allShardsExhausted() {
			c.logger.WithFields(logrus.Fields{
				"round":               c.round + 1,
				"total_results_fetch": c.totalResultsFetch,
			}).Debug("all shards exhausted")
			break
		}
	}

	// Extract top-k results
	return c.extractTopK()
}

// fetchRound fetches the next batch from each shard
func (c *StreamingSearchCoordinator) fetchRound(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(c.searchers))

	for _, shardSearcher := range c.searchers {
		if shardSearcher.exhausted {
			continue
		}

		wg.Add(1)
		go func(ss *ShardSearcher) {
			defer wg.Done()

			batch, err := ss.searcher.NextBatch()
			if err != nil {
				errChan <- fmt.Errorf("shard %s: %w", ss.shardName, err)
				return
			}

			ss.mu.Lock()
			defer ss.mu.Unlock()

			// Update shard state
			ss.exhausted = batch.Exhausted
			if batch.MaxRemainingPresent {
				ss.maxRemainingScore = batch.MaxRemainingScore
			} else {
				ss.maxRemainingScore = math.MaxFloat32
			}

			// Merge results into global heap
			c.mu.Lock()
			for i := range batch.IDs {
				c.globalHeap.Insert(batch.IDs[i], batch.Distances[i])
				c.totalResultsFetch++

				// Update threshold (k-th best score)
				if c.globalHeap.Len() >= c.k {
					c.threshold = c.globalHeap.Top().Dist
				}
			}
			c.mu.Unlock()
		}(shardSearcher)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// canTerminate checks if early termination is possible
func (c *StreamingSearchCoordinator) canTerminate() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Need at least k results to consider termination
	if c.globalHeap.Len() < c.k {
		return false
	}

	// Find the maximum possible score from any shard
	maxPossible := float32(0)
	for _, ss := range c.searchers {
		ss.mu.Lock()
		if !ss.exhausted && ss.maxRemainingScore > maxPossible {
			maxPossible = ss.maxRemainingScore
		}
		ss.mu.Unlock()
	}

	// For distance metrics (lower is better), if all remaining scores
	// are worse (higher) than our threshold, we can terminate
	return maxPossible > c.threshold
}

// allShardsExhausted checks if all shards are exhausted
func (c *StreamingSearchCoordinator) allShardsExhausted() bool {
	for _, ss := range c.searchers {
		ss.mu.Lock()
		exhausted := ss.exhausted
		ss.mu.Unlock()
		if !exhausted {
			return false
		}
	}
	return true
}

// extractTopK extracts the top-k results from the global heap
func (c *StreamingSearchCoordinator) extractTopK() ([]uint64, []float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove excess results
	for c.globalHeap.Len() > c.k {
		c.globalHeap.Pop()
	}

	resultCount := c.globalHeap.Len()
	ids := make([]uint64, resultCount)
	dists := make([]float32, resultCount)

	// Extract in reverse order (best first)
	for i := resultCount - 1; i >= 0; i-- {
		item := c.globalHeap.Pop()
		ids[i] = item.ID
		dists[i] = item.Dist
	}

	return ids, dists, nil
}

// Close releases resources for all shard searchers
func (c *StreamingSearchCoordinator) Close() {
	for _, ss := range c.searchers {
		if ss.searcher != nil {
			ss.searcher.Close()
		}
	}
}

// objectVectorSearchStreaming performs streaming cross-shard vector search with early termination
// This is the streaming equivalent of objectVectorSearch
func (i *Index) objectVectorSearchStreaming(ctx context.Context, searchVector []float32,
	k int, batchSize int,
) ([]*storobj.Object, []float32, error) {
	// For now, this is a placeholder for the integration point
	// In a full implementation, this would:
	// 1. Create StreamingSearchCoordinator
	// 2. For each shard, create a streaming searcher and add to coordinator
	// 3. Run coordinator.Search()
	// 4. Convert IDs to objects
	// 5. Return results

	// This will be integrated with the full objectVectorSearch function
	// with feature flag support

	return nil, nil, fmt.Errorf("streaming search not yet fully integrated")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
