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
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// CostFactors defines the cost parameters for different operations
// All costs are in microseconds
type CostFactors struct {
	// CPU costs
	CPUPerTuple     float64
	CPUHashBuild    float64
	CPUHashProbe    float64
	CPUSort         float64
	CPUVectorSearch float64

	// I/O costs
	IOSequentialPage float64
	IORandomPage     float64
	IOVectorRead     float64

	// Network costs
	NetworkPerKB float64
}

// DefaultCostFactors returns typical cost factors for modern NVMe storage
func DefaultCostFactors() *CostFactors {
	return &CostFactors{
		CPUPerTuple:      0.1,   // 0.1 microseconds per tuple
		CPUHashBuild:     0.5,   // Hash table build
		CPUHashProbe:     0.2,   // Hash table probe
		CPUSort:          1.0,   // Sort operation
		CPUVectorSearch:  10.0,  // Vector distance computation
		IOSequentialPage: 10.0,  // Sequential page read
		IORandomPage:     100.0, // Random page read
		IOVectorRead:     50.0,  // Vector read from disk
		NetworkPerKB:     5.0,   // Network transfer per KB
	}
}

// EstimateCache caches cardinality estimates to avoid redundant ML predictions
type EstimateCache struct {
	mu      sync.RWMutex
	cache   map[string]*CacheEntry
	maxSize int
	ttl     time.Duration
}

type CacheEntry struct {
	Cardinality int64
	Cost        float64
	Timestamp   time.Time
}

// NewEstimateCache creates a new estimate cache
func NewEstimateCache(maxSize int, ttl time.Duration) *EstimateCache {
	return &EstimateCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached estimate
func (c *EstimateCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.Timestamp) > c.ttl {
		return nil, false
	}

	return entry, true
}

// Put stores an estimate in cache
func (c *EstimateCache) Put(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction: remove oldest if at capacity
	if len(c.cache) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		delete(c.cache, oldestKey)
	}

	c.cache[key] = entry
}

// CardinalityEstimator interface for ML-based cardinality estimation
type CardinalityEstimator interface {
	EstimateCardinality(ctx context.Context, op Operator) (int64, error)
}

// MLCostModel integrates ML-based cardinality estimation with cost modeling
type MLCostModel struct {
	estimator   CardinalityEstimator
	costFactors *CostFactors
	cache       *EstimateCache
}

// NewMLCostModel creates a new ML-powered cost model
func NewMLCostModel(estimator CardinalityEstimator) *MLCostModel {
	return &MLCostModel{
		estimator:   estimator,
		costFactors: DefaultCostFactors(),
		cache:       NewEstimateCache(1000, 5*time.Minute),
	}
}

// Operator represents a query plan operator
type Operator interface {
	Type() string
	Children() []Operator
	EstimatedCardinality() int64
	SetEstimatedCardinality(card int64)
	GetProperties() map[string]interface{}
}

// VectorSearchOp represents a vector similarity search operation
type VectorSearchOp struct {
	IndexSize              int64
	IndexConfig            VectorIndexConfig
	estimatedCardinality   int64
	properties             map[string]interface{}
}

func (v *VectorSearchOp) Type() string                          { return "VectorSearch" }
func (v *VectorSearchOp) Children() []Operator                  { return nil }
func (v *VectorSearchOp) EstimatedCardinality() int64           { return v.estimatedCardinality }
func (v *VectorSearchOp) SetEstimatedCardinality(card int64)    { v.estimatedCardinality = card }
func (v *VectorSearchOp) GetProperties() map[string]interface{} { return v.properties }

// FilterOp represents a filter operation
type FilterOp struct {
	Selectivity          float64
	InputCardinality     int64
	estimatedCardinality int64
	Child                Operator
	properties           map[string]interface{}
}

func (f *FilterOp) Type() string                          { return "Filter" }
func (f *FilterOp) Children() []Operator                  { return []Operator{f.Child} }
func (f *FilterOp) EstimatedCardinality() int64           { return f.estimatedCardinality }
func (f *FilterOp) SetEstimatedCardinality(card int64)    { f.estimatedCardinality = card }
func (f *FilterOp) GetProperties() map[string]interface{} { return f.properties }

// JoinOp represents a join operation
type JoinOp struct {
	JoinType             string
	LeftChild            Operator
	RightChild           Operator
	estimatedCardinality int64
	properties           map[string]interface{}
}

func (j *JoinOp) Type() string                          { return "Join" }
func (j *JoinOp) Children() []Operator                  { return []Operator{j.LeftChild, j.RightChild} }
func (j *JoinOp) EstimatedCardinality() int64           { return j.estimatedCardinality }
func (j *JoinOp) SetEstimatedCardinality(card int64)    { j.estimatedCardinality = card }
func (j *JoinOp) GetProperties() map[string]interface{} { return j.properties }

// VectorIndexConfig represents HNSW index configuration
type VectorIndexConfig struct {
	EF int // Exploration factor for HNSW
}

// QueryPlan represents a complete query execution plan
type QueryPlan struct {
	Root      Operator
	Operators []Operator
}

// EstimatePlan estimates the cost of a complete query plan
func (m *MLCostModel) EstimatePlan(ctx context.Context, plan *QueryPlan) (float64, error) {
	// Use ML to estimate cardinalities for all operators
	for _, op := range plan.Operators {
		// Check cache first
		cacheKey := m.getCacheKey(op)
		if entry, ok := m.cache.Get(cacheKey); ok {
			op.SetEstimatedCardinality(entry.Cardinality)
			continue
		}

		// Estimate using ML
		card, err := m.estimator.EstimateCardinality(ctx, op)
		if err != nil {
			// Fall back to heuristic
			card = m.heuristicCardinality(op)
		}

		op.SetEstimatedCardinality(card)

		// Cache the result
		m.cache.Put(cacheKey, &CacheEntry{
			Cardinality: card,
			Timestamp:   time.Now(),
		})
	}

	// Calculate costs bottom-up
	cost := m.calculateCost(plan.Root)
	return cost, nil
}

// calculateCost calculates the cost of an operator and its subtree
func (m *MLCostModel) calculateCost(op Operator) float64 {
	switch typed := op.(type) {
	case *VectorSearchOp:
		return m.costVectorSearch(typed)
	case *FilterOp:
		childCost := 0.0
		if typed.Child != nil {
			childCost = m.calculateCost(typed.Child)
		}
		return childCost + m.costFilter(typed)
	case *JoinOp:
		leftCost := m.calculateCost(typed.LeftChild)
		rightCost := m.calculateCost(typed.RightChild)
		return leftCost + rightCost + m.costJoin(typed)
	default:
		return 0
	}
}

// costVectorSearch calculates the cost of a vector search operation
func (m *MLCostModel) costVectorSearch(op *VectorSearchOp) float64 {
	// HNSW search cost model
	ef := float64(op.IndexConfig.EF)
	if ef == 0 {
		ef = 100 // Default EF
	}

	// Cost = ef * log(N) * distance_computation
	logN := math.Log2(float64(op.IndexSize))
	distComputations := ef * logN

	// CPU cost for distance computations
	cpuCost := distComputations * m.costFactors.CPUVectorSearch

	// I/O cost for fetching vectors
	vectorReads := float64(op.EstimatedCardinality())
	ioCost := vectorReads * m.costFactors.IOVectorRead

	return cpuCost + ioCost
}

// costFilter calculates the cost of a filter operation
func (m *MLCostModel) costFilter(op *FilterOp) float64 {
	// Cost to evaluate filter predicate on each input tuple
	return float64(op.InputCardinality) * m.costFactors.CPUPerTuple
}

// costJoin calculates the cost of a join operation
func (m *MLCostModel) costJoin(op *JoinOp) float64 {
	leftCard := float64(op.LeftChild.EstimatedCardinality())
	rightCard := float64(op.RightChild.EstimatedCardinality())

	switch op.JoinType {
	case "hash":
		// Hash join: build hash table on right, probe with left
		buildCost := rightCard * m.costFactors.CPUHashBuild
		probeCost := leftCard * m.costFactors.CPUHashProbe
		return buildCost + probeCost

	case "nested_loop":
		// Nested loop join
		return leftCard * rightCard * m.costFactors.CPUPerTuple

	default:
		// Default to hash join
		buildCost := rightCard * m.costFactors.CPUHashBuild
		probeCost := leftCard * m.costFactors.CPUHashProbe
		return buildCost + probeCost
	}
}

// heuristicCardinality provides fallback cardinality estimation
func (m *MLCostModel) heuristicCardinality(op Operator) int64 {
	switch typed := op.(type) {
	case *FilterOp:
		// Apply selectivity
		return int64(float64(typed.InputCardinality) * typed.Selectivity)
	case *JoinOp:
		// Simple join cardinality: product of inputs with reduction
		left := typed.LeftChild.EstimatedCardinality()
		right := typed.RightChild.EstimatedCardinality()
		return int64(float64(left*right) * 0.1) // Assume 10% selectivity
	default:
		return 1000 // Default guess
	}
}

// getCacheKey generates a cache key for an operator
func (m *MLCostModel) getCacheKey(op Operator) string {
	// Simple key based on operator type and properties
	props, _ := json.Marshal(op.GetProperties())
	return fmt.Sprintf("%s:%s", op.Type(), string(props))
}

// ComparePlans compares two query plans and returns the cheaper one
func (m *MLCostModel) ComparePlans(ctx context.Context, plan1, plan2 *QueryPlan) (*QueryPlan, float64, error) {
	cost1, err1 := m.EstimatePlan(ctx, plan1)
	if err1 != nil {
		return nil, 0, err1
	}

	cost2, err2 := m.EstimatePlan(ctx, plan2)
	if err2 != nil {
		return nil, 0, err2
	}

	if cost1 <= cost2 {
		return plan1, cost1, nil
	}
	return plan2, cost2, nil
}
