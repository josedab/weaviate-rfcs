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

package dataloader

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchFunc is a function that loads multiple items by their keys
type BatchFunc func(ctx context.Context, keys []string) ([]interface{}, []error)

// Result holds the result of a single load operation
type Result struct {
	Data  interface{}
	Error error
}

// Loader implements the DataLoader pattern for batching and caching requests
type Loader struct {
	batchFn   BatchFunc
	cache     *sync.Map
	batchSize int
	wait      time.Duration
	mu        sync.Mutex
	batch     []string
	results   map[string]chan *Result
}

// Config holds configuration for the DataLoader
type Config struct {
	BatchSize int
	Wait      time.Duration
	Cache     *sync.Map
}

// NewLoader creates a new DataLoader
func NewLoader(batchFn BatchFunc, config Config) *Loader {
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.Wait == 0 {
		config.Wait = 16 * time.Millisecond
	}
	if config.Cache == nil {
		config.Cache = &sync.Map{}
	}

	return &Loader{
		batchFn:   batchFn,
		cache:     config.Cache,
		batchSize: config.BatchSize,
		wait:      config.Wait,
		results:   make(map[string]chan *Result),
	}
}

// Load loads a single item by key, batching requests automatically
func (l *Loader) Load(ctx context.Context, key string) (interface{}, error) {
	// Check cache first
	if cached, ok := l.cache.Load(key); ok {
		return cached, nil
	}

	// Create result channel for this key
	l.mu.Lock()
	resultChan, exists := l.results[key]
	if !exists {
		resultChan = make(chan *Result, 1)
		l.results[key] = resultChan
		l.batch = append(l.batch, key)
	}

	// If batch is full, execute immediately
	if len(l.batch) >= l.batchSize {
		l.mu.Unlock()
		go l.executeBatch()
	} else {
		// Otherwise, schedule batch execution
		if len(l.batch) == 1 {
			go func() {
				time.Sleep(l.wait)
				l.executeBatch()
			}()
		}
		l.mu.Unlock()
	}

	// Wait for result
	select {
	case result := <-resultChan:
		if result.Error != nil {
			return nil, result.Error
		}
		return result.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// LoadMany loads multiple items by keys
func (l *Loader) LoadMany(ctx context.Context, keys []string) ([]interface{}, []error) {
	results := make([]interface{}, len(keys))
	errors := make([]error, len(keys))

	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func(idx int, k string) {
			defer wg.Done()
			data, err := l.Load(ctx, k)
			results[idx] = data
			errors[idx] = err
		}(i, key)
	}

	wg.Wait()
	return results, errors
}

// executeBatch executes the current batch of requests
func (l *Loader) executeBatch() {
	l.mu.Lock()
	if len(l.batch) == 0 {
		l.mu.Unlock()
		return
	}

	// Get current batch
	batch := l.batch
	results := l.results

	// Reset for next batch
	l.batch = []string{}
	l.results = make(map[string]chan *Result)
	l.mu.Unlock()

	// Execute batch function
	ctx := context.Background()
	data, errors := l.batchFn(ctx, batch)

	// Distribute results
	for i, key := range batch {
		var result *Result
		if i < len(errors) && errors[i] != nil {
			result = &Result{Error: errors[i]}
		} else if i < len(data) {
			result = &Result{Data: data[i]}
			// Cache successful results
			l.cache.Store(key, data[i])
		} else {
			result = &Result{Error: fmt.Errorf("no result for key %s", key)}
		}

		// Send result to waiting goroutine
		if ch, ok := results[key]; ok {
			ch <- result
			close(ch)
		}
	}
}

// Clear clears the cache
func (l *Loader) Clear(key string) {
	l.cache.Delete(key)
}

// ClearAll clears the entire cache
func (l *Loader) ClearAll() {
	l.cache = &sync.Map{}
}

// Prime primes the cache with a value
func (l *Loader) Prime(key string, value interface{}) {
	l.cache.Store(key, value)
}
