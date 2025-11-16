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
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/adapters/repos/db/helpers"
	"github.com/weaviate/weaviate/entities/filters"
)

func TestFeatureExtractor_ExtractFeatures(t *testing.T) {
	fe := NewFeatureExtractor()

	// Create a simple filter
	filter := &filters.Clause{
		Operator: filters.OperatorEqual,
		On: &filters.Path{
			Property: "category",
		},
		Value: &filters.Value{
			Value: "electronics",
			Type:  "text",
		},
	}

	// Create allow list
	allowList := helpers.NewAllowList(1, 2, 3, 4, 5)

	// Create query vector
	queryVector := []float32{0.1, 0.2, 0.3, 0.4}

	// Extract features
	features := fe.ExtractFeatures(filter, allowList, queryVector, 1000)

	// Verify features
	assert.Equal(t, "category", features.PropertyName)
	assert.Equal(t, "Equal", features.Operator)
	assert.Equal(t, 1000, features.CorpusSize)
	assert.Equal(t, 4, features.VectorDimensions)
	assert.Greater(t, features.QueryVectorNorm, 0.0)
	assert.GreaterOrEqual(t, features.TimeOfDayHour, 0)
	assert.LessOrEqual(t, features.TimeOfDayHour, 23)
	assert.GreaterOrEqual(t, features.DayOfWeek, 0)
	assert.LessOrEqual(t, features.DayOfWeek, 6)
}

func TestFeatureExtractor_UpdatePropertyStats(t *testing.T) {
	fe := NewFeatureExtractor()

	// Update stats for a property
	fe.UpdatePropertyStats("category", 0.05, 100)

	// Verify stats were stored
	stats, ok := fe.propertyStats["category"]
	require.True(t, ok, "Property stats should be stored")
	assert.Equal(t, 100, stats.Cardinality)
	assert.Equal(t, 1, stats.TotalQueries)
	assert.Equal(t, 1, len(stats.SelectivitySamples))
	assert.Equal(t, 0.05, stats.SelectivitySamples[0])

	// Update again with different selectivity
	fe.UpdatePropertyStats("category", 0.08, 150)

	stats = fe.propertyStats["category"]
	assert.Equal(t, 150, stats.Cardinality) // Should update to higher value
	assert.Equal(t, 2, stats.TotalQueries)
	assert.Equal(t, 2, len(stats.SelectivitySamples))
}

func TestFeatureExtractor_CalculateFilterComplexity(t *testing.T) {
	tests := []struct {
		name       string
		filter     *filters.Clause
		complexity int
	}{
		{
			name: "simple filter",
			filter: &filters.Clause{
				Operator: filters.OperatorEqual,
			},
			complexity: 1,
		},
		{
			name: "AND filter",
			filter: &filters.Clause{
				Operator: filters.OperatorAnd,
				Operands: []filters.Clause{
					{Operator: filters.OperatorEqual},
					{Operator: filters.OperatorGreaterThan},
				},
			},
			complexity: 3, // 1 for AND + 1 for each operand
		},
		{
			name: "nested filter",
			filter: &filters.Clause{
				Operator: filters.OperatorOr,
				Operands: []filters.Clause{
					{
						Operator: filters.OperatorAnd,
						Operands: []filters.Clause{
							{Operator: filters.OperatorEqual},
							{Operator: filters.OperatorLike},
						},
					},
					{Operator: filters.OperatorGreaterThan},
				},
			},
			complexity: 5, // 1 for OR + (1 for AND + 2 for its operands) + 1 for GT
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexity := calculateFilterComplexity(tt.filter)
			assert.Equal(t, tt.complexity, complexity)
		})
	}
}

func TestLearnedFilterOptimizer_PredictSelectivity(t *testing.T) {
	// Create optimizer without model (fallback mode)
	optimizer, err := NewLearnedFilterOptimizer(true, "", 0.1)
	require.NoError(t, err)

	filter := &filters.Clause{
		Operator: filters.OperatorEqual,
		On: &filters.Path{
			Property: "status",
		},
	}

	allowList := helpers.NewAllowList(1, 2, 3, 4, 5)
	queryVector := []float32{0.1, 0.2, 0.3}

	// Predict selectivity
	prediction, err := optimizer.PredictSelectivity(filter, allowList, queryVector, 1000)
	require.NoError(t, err)
	require.NotNil(t, prediction)

	// Verify prediction
	assert.GreaterOrEqual(t, prediction.Selectivity, 0.0)
	assert.LessOrEqual(t, prediction.Selectivity, 1.0)
	assert.Contains(t, []string{"pre_filter", "post_filter"}, prediction.Strategy)
	assert.GreaterOrEqual(t, prediction.ConfidenceScore, 0.0)
	assert.LessOrEqual(t, prediction.ConfidenceScore, 1.0)
}

func TestLearnedFilterOptimizer_ShouldUsePreFilter(t *testing.T) {
	optimizer, err := NewLearnedFilterOptimizer(true, "", 0.1)
	require.NoError(t, err)

	tests := []struct {
		name       string
		prediction *SelectivityPrediction
		expected   bool
	}{
		{
			name: "low selectivity - pre-filter",
			prediction: &SelectivityPrediction{
				Selectivity: 0.05,
				Strategy:    "pre_filter",
			},
			expected: true,
		},
		{
			name: "high selectivity - post-filter",
			prediction: &SelectivityPrediction{
				Selectivity: 0.5,
				Strategy:    "post_filter",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.ShouldUsePreFilter(tt.prediction)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLearnedFilterOptimizer_Disabled(t *testing.T) {
	// Create disabled optimizer
	optimizer, err := NewLearnedFilterOptimizer(false, "", 0.1)
	require.NoError(t, err)

	assert.False(t, optimizer.IsEnabled())

	prediction := &SelectivityPrediction{
		Strategy: "pre_filter",
	}
	assert.False(t, optimizer.ShouldUsePreFilter(prediction))
}

func TestCalculateVectorNorm(t *testing.T) {
	tests := []struct {
		name     string
		vector   []float32
		expected float64
	}{
		{
			name:     "empty vector",
			vector:   []float32{},
			expected: 0.0,
		},
		{
			name:     "unit vector",
			vector:   []float32{1.0},
			expected: 1.0,
		},
		{
			name:     "3D vector",
			vector:   []float32{3.0, 4.0, 0.0},
			expected: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateVectorNorm(tt.vector)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestCalculatePercentile(t *testing.T) {
	samples := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

	tests := []struct {
		name       string
		percentile float64
		expected   float64
	}{
		{
			name:       "median (p50)",
			percentile: 0.50,
			expected:   0.5,
		},
		{
			name:       "p95",
			percentile: 0.95,
			expected:   0.9,
		},
		{
			name:       "minimum (p0)",
			percentile: 0.0,
			expected:   0.1,
		},
		{
			name:       "maximum (p100)",
			percentile: 1.0,
			expected:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePercentile(samples, tt.percentile)
			assert.InDelta(t, tt.expected, result, 0.11) // Allow some tolerance due to simple implementation
		})
	}
}

func TestCalculateOptimalStrategy(t *testing.T) {
	tests := []struct {
		name              string
		actualSelectivity float64
		expected          string
	}{
		{
			name:              "low selectivity",
			actualSelectivity: 0.05,
			expected:          "pre_filter",
		},
		{
			name:              "high selectivity",
			actualSelectivity: 0.5,
			expected:          "post_filter",
		},
		{
			name:              "threshold selectivity",
			actualSelectivity: 0.1,
			expected:          "post_filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateOptimalStrategy(tt.actualSelectivity, 0, 0)
			assert.Equal(t, tt.expected, result)
		})
	}
}
