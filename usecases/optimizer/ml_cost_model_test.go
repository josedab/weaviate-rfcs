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

package optimizer

import (
	"context"
	"testing"
	"time"
)

// MockCardinalityEstimator for testing
type MockCardinalityEstimator struct {
	estimateFunc func(ctx context.Context, op Operator) (int64, error)
}

func (m *MockCardinalityEstimator) EstimateCardinality(ctx context.Context, op Operator) (int64, error) {
	if m.estimateFunc != nil {
		return m.estimateFunc(ctx, op)
	}
	return 1000, nil // Default estimate
}

func TestMLCostModel_EstimateVectorSearch(t *testing.T) {
	estimator := &MockCardinalityEstimator{
		estimateFunc: func(ctx context.Context, op Operator) (int64, error) {
			return 100, nil
		},
	}

	model := NewMLCostModel(estimator)

	op := &VectorSearchOp{
		IndexSize: 100000,
		IndexConfig: VectorIndexConfig{
			EF: 100,
		},
		properties: make(map[string]interface{}),
	}

	ctx := context.Background()
	plan := &QueryPlan{
		Root:      op,
		Operators: []Operator{op},
	}

	cost, err := model.EstimatePlan(ctx, plan)
	if err != nil {
		t.Fatalf("EstimatePlan failed: %v", err)
	}

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}

	// Verify cardinality was estimated
	if op.EstimatedCardinality() != 100 {
		t.Errorf("Expected cardinality 100, got %d", op.EstimatedCardinality())
	}
}

func TestMLCostModel_EstimateFilter(t *testing.T) {
	estimator := &MockCardinalityEstimator{
		estimateFunc: func(ctx context.Context, op Operator) (int64, error) {
			switch op.Type() {
			case "Filter":
				return 50, nil
			default:
				return 1000, nil
			}
		},
	}

	model := NewMLCostModel(estimator)

	filterOp := &FilterOp{
		Selectivity:      0.5,
		InputCardinality: 1000,
		properties:       make(map[string]interface{}),
	}

	ctx := context.Background()
	plan := &QueryPlan{
		Root:      filterOp,
		Operators: []Operator{filterOp},
	}

	cost, err := model.EstimatePlan(ctx, plan)
	if err != nil {
		t.Fatalf("EstimatePlan failed: %v", err)
	}

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
}

func TestMLCostModel_EstimateJoin(t *testing.T) {
	estimator := &MockCardinalityEstimator{
		estimateFunc: func(ctx context.Context, op Operator) (int64, error) {
			return 100, nil
		},
	}

	model := NewMLCostModel(estimator)

	leftOp := &FilterOp{
		Selectivity:      1.0,
		InputCardinality: 100,
		properties:       make(map[string]interface{}),
	}
	leftOp.SetEstimatedCardinality(100)

	rightOp := &FilterOp{
		Selectivity:      1.0,
		InputCardinality: 50,
		properties:       make(map[string]interface{}),
	}
	rightOp.SetEstimatedCardinality(50)

	joinOp := &JoinOp{
		JoinType:   "hash",
		LeftChild:  leftOp,
		RightChild: rightOp,
		properties: make(map[string]interface{}),
	}

	ctx := context.Background()
	plan := &QueryPlan{
		Root:      joinOp,
		Operators: []Operator{joinOp, leftOp, rightOp},
	}

	cost, err := model.EstimatePlan(ctx, plan)
	if err != nil {
		t.Fatalf("EstimatePlan failed: %v", err)
	}

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
}

func TestMLCostModel_ComparePlans(t *testing.T) {
	estimator := &MockCardinalityEstimator{
		estimateFunc: func(ctx context.Context, op Operator) (int64, error) {
			return 100, nil
		},
	}

	model := NewMLCostModel(estimator)

	// Plan 1: Simple filter
	plan1Op := &FilterOp{
		Selectivity:      0.1,
		InputCardinality: 1000,
		properties:       make(map[string]interface{}),
	}
	plan1 := &QueryPlan{
		Root:      plan1Op,
		Operators: []Operator{plan1Op},
	}

	// Plan 2: More expensive join
	leftOp := &FilterOp{
		Selectivity:      1.0,
		InputCardinality: 1000,
		properties:       make(map[string]interface{}),
	}
	leftOp.SetEstimatedCardinality(1000)

	rightOp := &FilterOp{
		Selectivity:      1.0,
		InputCardinality: 1000,
		properties:       make(map[string]interface{}),
	}
	rightOp.SetEstimatedCardinality(1000)

	plan2Op := &JoinOp{
		JoinType:   "nested_loop",
		LeftChild:  leftOp,
		RightChild: rightOp,
		properties: make(map[string]interface{}),
	}
	plan2 := &QueryPlan{
		Root:      plan2Op,
		Operators: []Operator{plan2Op, leftOp, rightOp},
	}

	ctx := context.Background()
	chosen, cost, err := model.ComparePlans(ctx, plan1, plan2)
	if err != nil {
		t.Fatalf("ComparePlans failed: %v", err)
	}

	if chosen != plan1 {
		t.Errorf("Expected plan1 to be cheaper")
	}

	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}
}

func TestEstimateCache(t *testing.T) {
	cache := NewEstimateCache(2, 1*time.Second)

	// Test Put and Get
	entry1 := &CacheEntry{
		Cardinality: 100,
		Cost:        1000.0,
		Timestamp:   time.Now(),
	}
	cache.Put("key1", entry1)

	retrieved, ok := cache.Get("key1")
	if !ok {
		t.Error("Expected to find key1 in cache")
	}
	if retrieved.Cardinality != 100 {
		t.Errorf("Expected cardinality 100, got %d", retrieved.Cardinality)
	}

	// Test eviction
	entry2 := &CacheEntry{
		Cardinality: 200,
		Cost:        2000.0,
		Timestamp:   time.Now(),
	}
	cache.Put("key2", entry2)

	entry3 := &CacheEntry{
		Cardinality: 300,
		Cost:        3000.0,
		Timestamp:   time.Now(),
	}
	cache.Put("key3", entry3) // Should evict oldest (key1)

	_, ok = cache.Get("key1")
	if ok {
		t.Error("Expected key1 to be evicted")
	}

	// Test TTL
	oldEntry := &CacheEntry{
		Cardinality: 400,
		Cost:        4000.0,
		Timestamp:   time.Now().Add(-2 * time.Second),
	}
	cache.Put("old", oldEntry)

	_, ok = cache.Get("old")
	if ok {
		t.Error("Expected old entry to be expired")
	}
}

func TestCostFactors(t *testing.T) {
	factors := DefaultCostFactors()

	if factors.CPUPerTuple <= 0 {
		t.Error("CPUPerTuple should be positive")
	}
	if factors.IOSequentialPage <= 0 {
		t.Error("IOSequentialPage should be positive")
	}
	if factors.IORandomPage <= factors.IOSequentialPage {
		t.Error("IORandomPage should be more expensive than IOSequentialPage")
	}
}
