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
	"math"
	"time"

	"github.com/weaviate/weaviate/adapters/repos/db/helpers"
	"github.com/weaviate/weaviate/entities/filters"
)

// FilterQueryFeatures represents features extracted from a filter query
// for ML model prediction of optimal filter strategy
type FilterQueryFeatures struct {
	// Filter characteristics
	PropertyName     string
	Operator         string
	FilterComplexity int // Number of AND/OR/NOT operations

	// Statistical features
	PropertyCardinality      int     // Unique values in property (estimated)
	CorpusSize               int     // Total documents in index
	HistoricalSelectivityP50 float64 // Median selectivity from past queries
	HistoricalSelectivityP95 float64 // 95th percentile selectivity

	// Temporal features
	TimeOfDayHour int // 0-23
	DayOfWeek     int // 0-6 (Sunday = 0)

	// Query-specific features
	QueryVectorNorm  float64 // L2 norm of query vector
	VectorDimensions int     // Dimensionality of vectors

	// System-specific features
	CacheHitRateRecent     float64       // Recent cache performance
	AverageQueryLatencyP95 time.Duration // Recent baseline latency
}

// FeatureExtractor extracts features from filter queries for ML prediction
type FeatureExtractor struct {
	// Historical statistics per property
	propertyStats map[string]*PropertyStats
}

// PropertyStats tracks historical statistics for a property
type PropertyStats struct {
	Cardinality          int       // Estimated unique values
	SelectivitySamples   []float64 // Recent selectivity samples
	LastUpdated          time.Time
	TotalQueries         int
	TotalFilteredRecords int
}

// NewFeatureExtractor creates a new feature extractor
func NewFeatureExtractor() *FeatureExtractor {
	return &FeatureExtractor{
		propertyStats: make(map[string]*PropertyStats),
	}
}

// ExtractFeatures extracts features from a filter query
func (fe *FeatureExtractor) ExtractFeatures(
	filter *filters.Clause,
	allowList helpers.AllowList,
	queryVector []float32,
	corpusSize int,
) *FilterQueryFeatures {
	features := &FilterQueryFeatures{
		CorpusSize:       corpusSize,
		VectorDimensions: len(queryVector),
		QueryVectorNorm:  calculateVectorNorm(queryVector),
	}

	// Extract filter-specific features
	if filter != nil {
		features.PropertyName = extractPropertyName(filter)
		features.Operator = filter.Operator.Name()
		features.FilterComplexity = calculateFilterComplexity(filter)

		// Get historical statistics for this property
		if stats, ok := fe.propertyStats[features.PropertyName]; ok {
			features.PropertyCardinality = stats.Cardinality
			features.HistoricalSelectivityP50 = calculatePercentile(stats.SelectivitySamples, 0.50)
			features.HistoricalSelectivityP95 = calculatePercentile(stats.SelectivitySamples, 0.95)
		}
	}

	// Extract temporal features
	now := time.Now()
	features.TimeOfDayHour = now.Hour()
	features.DayOfWeek = int(now.Weekday())

	// System-specific features would be populated from metrics
	// These are placeholders for now
	features.CacheHitRateRecent = 0.0
	features.AverageQueryLatencyP95 = 0

	return features
}

// UpdatePropertyStats updates historical statistics for a property after query execution
func (fe *FeatureExtractor) UpdatePropertyStats(
	propertyName string,
	actualSelectivity float64,
	cardinality int,
) {
	stats, ok := fe.propertyStats[propertyName]
	if !ok {
		stats = &PropertyStats{
			Cardinality:          cardinality,
			SelectivitySamples:   make([]float64, 0, 100),
			LastUpdated:          time.Now(),
			TotalQueries:         0,
			TotalFilteredRecords: 0,
		}
		fe.propertyStats[propertyName] = stats
	}

	// Update selectivity samples (keep last 100 samples)
	stats.SelectivitySamples = append(stats.SelectivitySamples, actualSelectivity)
	if len(stats.SelectivitySamples) > 100 {
		stats.SelectivitySamples = stats.SelectivitySamples[1:]
	}

	stats.LastUpdated = time.Now()
	stats.TotalQueries++

	// Update cardinality estimate if provided
	if cardinality > stats.Cardinality {
		stats.Cardinality = cardinality
	}
}

// Helper functions

func extractPropertyName(filter *filters.Clause) string {
	if filter == nil || filter.On == nil {
		return ""
	}
	if len(filter.On.Property) > 0 {
		return string(filter.On.Property)
	}
	return ""
}

func calculateFilterComplexity(filter *filters.Clause) int {
	if filter == nil {
		return 0
	}

	complexity := 0
	switch filter.Operator {
	case filters.OperatorAnd, filters.OperatorOr, filters.OperatorNot:
		complexity = 1
		for _, operand := range filter.Operands {
			complexity += calculateFilterComplexity(&operand)
		}
	default:
		complexity = 1
	}

	return complexity
}

func calculateVectorNorm(vector []float32) float64 {
	if len(vector) == 0 {
		return 0.0
	}

	var sum float64
	for _, v := range vector {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum)
}

func calculatePercentile(samples []float64, percentile float64) float64 {
	if len(samples) == 0 {
		return 0.0
	}

	// Simple percentile calculation (could be optimized)
	sorted := make([]float64, len(samples))
	copy(sorted, samples)

	// Simple bubble sort for small arrays
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)-1) * percentile)
	return sorted[index]
}
