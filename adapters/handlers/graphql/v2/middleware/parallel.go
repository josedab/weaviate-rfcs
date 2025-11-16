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

package middleware

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ParallelExecutor executes multiple operations in parallel
type ParallelExecutor struct {
	maxConcurrency int
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(maxConcurrency int) *ParallelExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // default
	}
	return &ParallelExecutor{
		maxConcurrency: maxConcurrency,
	}
}

// Task represents a task to execute
type Task func(ctx context.Context) (interface{}, error)

// Result represents the result of a task
type Result struct {
	Data  interface{}
	Error error
	Index int
}

// Execute executes multiple tasks in parallel
func (p *ParallelExecutor) Execute(ctx context.Context, tasks []Task) ([]Result, error) {
	results := make([]Result, len(tasks))
	g, ctx := errgroup.WithContext(ctx)

	// Limit concurrency using a semaphore
	sem := make(chan struct{}, p.maxConcurrency)

	for i, task := range tasks {
		i := i // capture loop variable
		task := task

		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// Execute task
			data, err := task(ctx)
			results[i] = Result{
				Data:  data,
				Error: err,
				Index: i,
			}
			return nil // Don't propagate individual task errors
		})
	}

	// Wait for all tasks to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// ExecuteMap executes multiple tasks and returns results as a map
func (p *ParallelExecutor) ExecuteMap(ctx context.Context, tasks map[string]Task) (map[string]Result, error) {
	results := make(map[string]Result)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, p.maxConcurrency)

	for key, task := range tasks {
		key := key
		task := task

		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// Execute task
			data, err := task(ctx)

			// Store result
			mu.Lock()
			results[key] = Result{
				Data:  data,
				Error: err,
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// Batch executes tasks in batches
func (p *ParallelExecutor) Batch(ctx context.Context, tasks []Task, batchSize int) ([]Result, error) {
	if batchSize <= 0 {
		batchSize = p.maxConcurrency
	}

	allResults := make([]Result, 0, len(tasks))

	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		batch := tasks[i:end]
		results, err := p.Execute(ctx, batch)
		if err != nil {
			return nil, err
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}
