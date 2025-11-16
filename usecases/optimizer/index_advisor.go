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
	"fmt"
	"sort"
	"time"
)

// IndexType represents the type of index
type IndexType string

const (
	IndexTypeInverted IndexType = "inverted"
	IndexTypeVector   IndexType = "vector"
	IndexTypeHash     IndexType = "hash"
	IndexTypeBTree    IndexType = "btree"
)

// IndexRecommendation represents a suggested index
type IndexRecommendation struct {
	Class      string
	Properties []string
	Type       IndexType

	// Impact analysis
	QueriesImproved  int
	EstimatedSpeedup float64
	StorageOverhead  int64

	// Creation cost
	BuildTime time.Duration
	BuildCost float64

	// Confidence score (0.0 to 1.0)
	Confidence float64

	// Explanation
	Reason string
}

// WorkloadPattern represents a pattern in query workload
type WorkloadPattern struct {
	Pattern         string
	Frequency       int
	AvgExecutionTime time.Duration
	Properties      []string
	Class           string
}

// MissingIndexPattern represents a pattern that would benefit from an index
type MissingIndexPattern struct {
	Class      string
	Properties []string
	Type       IndexType
	Queries    []Query
	Frequency  int
}

// WorkloadAnalysisResult contains the results of workload analysis
type WorkloadAnalysisResult struct {
	TotalQueries    int
	SlowQueries     int
	MissingIndexes  []MissingIndexPattern
	Patterns        []WorkloadPattern
}

// WorkloadAnalyzer analyzes query workload patterns
type WorkloadAnalyzer struct {
	slowQueryThreshold time.Duration
}

// NewWorkloadAnalyzer creates a new workload analyzer
func NewWorkloadAnalyzer(slowQueryThreshold time.Duration) *WorkloadAnalyzer {
	return &WorkloadAnalyzer{
		slowQueryThreshold: slowQueryThreshold,
	}
}

// Analyze analyzes a workload and identifies patterns
func (wa *WorkloadAnalyzer) Analyze(workload []Query) *WorkloadAnalysisResult {
	result := &WorkloadAnalysisResult{
		TotalQueries:   len(workload),
		MissingIndexes: []MissingIndexPattern{},
		Patterns:       []WorkloadPattern{},
	}

	// Group queries by pattern
	patternMap := make(map[string]*WorkloadPattern)

	for _, query := range workload {
		pattern := wa.extractPattern(query)
		key := pattern.Pattern

		if existing, ok := patternMap[key]; ok {
			existing.Frequency++
		} else {
			patternMap[key] = &pattern
		}
	}

	// Convert map to slice
	for _, pattern := range patternMap {
		result.Patterns = append(result.Patterns, *pattern)
	}

	// Identify missing indexes
	result.MissingIndexes = wa.findMissingIndexes(workload)

	return result
}

// extractPattern extracts a pattern from a query
func (wa *WorkloadAnalyzer) extractPattern(query Query) WorkloadPattern {
	// Extract properties used in filters
	props := []string{}
	for _, filter := range query.Filters {
		props = append(props, filter.Property)
	}

	pattern := fmt.Sprintf("class=%s,filters=%v", query.ClassName, props)

	return WorkloadPattern{
		Pattern:    pattern,
		Frequency:  1,
		Properties: props,
		Class:      query.ClassName,
	}
}

// findMissingIndexes identifies filters on unindexed properties
func (wa *WorkloadAnalyzer) findMissingIndexes(workload []Query) []MissingIndexPattern {
	// Track property usage
	propertyUsage := make(map[string]*MissingIndexPattern)

	for _, query := range workload {
		for _, filter := range query.Filters {
			key := fmt.Sprintf("%s.%s", query.ClassName, filter.Property)

			if pattern, exists := propertyUsage[key]; exists {
				pattern.Frequency++
				pattern.Queries = append(pattern.Queries, query)
			} else {
				propertyUsage[key] = &MissingIndexPattern{
					Class:      query.ClassName,
					Properties: []string{filter.Property},
					Type:       wa.recommendIndexType(filter),
					Queries:    []Query{query},
					Frequency:  1,
				}
			}
		}
	}

	// Convert to slice
	patterns := []MissingIndexPattern{}
	for _, pattern := range propertyUsage {
		// Only include if used frequently
		if pattern.Frequency >= 3 {
			patterns = append(patterns, *pattern)
		}
	}

	return patterns
}

// recommendIndexType recommends the best index type for a filter
func (wa *WorkloadAnalyzer) recommendIndexType(filter Filter) IndexType {
	switch filter.Operator {
	case "=":
		return IndexTypeHash
	case "<", ">", "<=", ">=":
		return IndexTypeBTree
	case "LIKE", "contains":
		return IndexTypeInverted
	default:
		return IndexTypeInverted
	}
}

// ImpactEstimate represents the estimated impact of creating an index
type ImpactEstimate struct {
	QueriesAffected int
	Speedup         float64
	StorageBytes    int64
	BuildTime       time.Duration
}

// IndexAdvisor provides index recommendations based on workload analysis
type IndexAdvisor struct {
	workloadAnalyzer *WorkloadAnalyzer
	costModel        *MLCostModel
}

// NewIndexAdvisor creates a new index advisor
func NewIndexAdvisor(costModel *MLCostModel) *IndexAdvisor {
	return &IndexAdvisor{
		workloadAnalyzer: NewWorkloadAnalyzer(100 * time.Millisecond),
		costModel:        costModel,
	}
}

// Recommend generates index recommendations for a workload
func (a *IndexAdvisor) Recommend(ctx context.Context, workload []Query) []IndexRecommendation {
	// Analyze query patterns
	analysis := a.workloadAnalyzer.Analyze(workload)

	recommendations := []IndexRecommendation{}

	// For each missing index pattern
	for _, pattern := range analysis.MissingIndexes {
		// Estimate impact
		impact := a.estimateImpact(pattern, workload)

		// Only recommend if significant improvement (50%+ speedup)
		if impact.Speedup > 1.5 {
			recommendation := IndexRecommendation{
				Class:            pattern.Class,
				Properties:       pattern.Properties,
				Type:             pattern.Type,
				QueriesImproved:  impact.QueriesAffected,
				EstimatedSpeedup: impact.Speedup,
				StorageOverhead:  impact.StorageBytes,
				BuildTime:        impact.BuildTime,
				BuildCost:        float64(impact.BuildTime.Microseconds()),
				Confidence:       a.calculateConfidence(pattern, impact),
				Reason:           a.generateReason(pattern, impact),
			}

			recommendations = append(recommendations, recommendation)
		}
	}

	// Sort by estimated speedup (descending)
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].EstimatedSpeedup > recommendations[j].EstimatedSpeedup
	})

	return recommendations
}

// estimateImpact estimates the impact of creating an index for a pattern
func (a *IndexAdvisor) estimateImpact(pattern MissingIndexPattern, workload []Query) ImpactEstimate {
	// Count affected queries
	affected := 0
	for _, query := range workload {
		if a.wouldBenefit(query, pattern) {
			affected++
		}
	}

	// Estimate speedup based on index type and cardinality
	speedup := a.estimateSpeedup(pattern)

	// Estimate storage overhead
	storageBytes := a.estimateStorage(pattern)

	// Estimate build time
	buildTime := a.estimateBuildTime(pattern)

	return ImpactEstimate{
		QueriesAffected: affected,
		Speedup:         speedup,
		StorageBytes:    storageBytes,
		BuildTime:       buildTime,
	}
}

// wouldBenefit checks if a query would benefit from an index
func (a *IndexAdvisor) wouldBenefit(query Query, pattern MissingIndexPattern) bool {
	if query.ClassName != pattern.Class {
		return false
	}

	// Check if query uses the indexed properties
	for _, filter := range query.Filters {
		for _, prop := range pattern.Properties {
			if filter.Property == prop {
				return true
			}
		}
	}

	return false
}

// estimateSpeedup estimates the speedup from creating an index
func (a *IndexAdvisor) estimateSpeedup(pattern MissingIndexPattern) float64 {
	// Base speedup depends on index type
	baseSpeedup := 1.0

	switch pattern.Type {
	case IndexTypeHash:
		baseSpeedup = 10.0 // Hash lookup is very fast
	case IndexTypeBTree:
		baseSpeedup = 5.0 // B-tree is good for ranges
	case IndexTypeInverted:
		baseSpeedup = 3.0 // Inverted index for text search
	case IndexTypeVector:
		baseSpeedup = 20.0 // Vector index provides huge speedup
	}

	// Adjust based on query frequency
	frequencyMultiplier := 1.0 + float64(pattern.Frequency)/100.0

	return baseSpeedup * frequencyMultiplier
}

// estimateStorage estimates storage overhead for an index
func (a *IndexAdvisor) estimateStorage(pattern MissingIndexPattern) int64 {
	// Rough estimates based on index type
	// Assumes 1M rows for estimation
	const estimatedRows = 1_000_000

	switch pattern.Type {
	case IndexTypeHash:
		// Hash index: ~16 bytes per entry
		return estimatedRows * 16
	case IndexTypeBTree:
		// B-tree: ~32 bytes per entry
		return estimatedRows * 32
	case IndexTypeInverted:
		// Inverted index: ~64 bytes per entry (varies with text)
		return estimatedRows * 64
	case IndexTypeVector:
		// Vector index: ~512 bytes per vector (HNSW overhead)
		return estimatedRows * 512
	default:
		return estimatedRows * 32
	}
}

// estimateBuildTime estimates how long it will take to build the index
func (a *IndexAdvisor) estimateBuildTime(pattern MissingIndexPattern) time.Duration {
	// Rough estimates
	const estimatedRows = 1_000_000

	switch pattern.Type {
	case IndexTypeHash:
		// ~1000 rows/ms
		return time.Duration(estimatedRows/1000) * time.Millisecond
	case IndexTypeBTree:
		// ~500 rows/ms (more complex)
		return time.Duration(estimatedRows/500) * time.Millisecond
	case IndexTypeInverted:
		// ~300 rows/ms (text processing)
		return time.Duration(estimatedRows/300) * time.Millisecond
	case IndexTypeVector:
		// ~100 rows/ms (vector indexing is expensive)
		return time.Duration(estimatedRows/100) * time.Millisecond
	default:
		return time.Duration(estimatedRows/500) * time.Millisecond
	}
}

// calculateConfidence calculates confidence score for a recommendation
func (a *IndexAdvisor) calculateConfidence(pattern MissingIndexPattern, impact ImpactEstimate) float64 {
	confidence := 0.5 // Base confidence

	// Increase confidence with frequency
	if pattern.Frequency > 10 {
		confidence += 0.2
	}

	// Increase confidence with high speedup
	if impact.Speedup > 5.0 {
		confidence += 0.2
	}

	// Increase confidence with many affected queries
	if impact.QueriesAffected > 5 {
		confidence += 0.1
	}

	return min(confidence, 1.0)
}

// generateReason generates a human-readable reason for the recommendation
func (a *IndexAdvisor) generateReason(pattern MissingIndexPattern, impact ImpactEstimate) string {
	return fmt.Sprintf(
		"Property '%s' in class '%s' is frequently filtered (%d times) but not indexed. "+
			"Creating a %s index would improve %d queries by approximately %.1fx.",
		pattern.Properties[0],
		pattern.Class,
		pattern.Frequency,
		pattern.Type,
		impact.QueriesAffected,
		impact.Speedup,
	)
}

// GenerateReport generates a detailed report of index recommendations
func (a *IndexAdvisor) GenerateReport(ctx context.Context, workload []Query) string {
	recommendations := a.Recommend(ctx, workload)

	report := "=== INDEX RECOMMENDATIONS ===\n\n"
	report += fmt.Sprintf("Analyzed %d queries\n", len(workload))
	report += fmt.Sprintf("Found %d recommendations\n\n", len(recommendations))

	for i, rec := range recommendations {
		report += fmt.Sprintf("Recommendation %d:\n", i+1)
		report += fmt.Sprintf("  Class: %s\n", rec.Class)
		report += fmt.Sprintf("  Properties: %v\n", rec.Properties)
		report += fmt.Sprintf("  Type: %s\n", rec.Type)
		report += fmt.Sprintf("  Estimated Speedup: %.1fx\n", rec.EstimatedSpeedup)
		report += fmt.Sprintf("  Queries Improved: %d\n", rec.QueriesImproved)
		report += fmt.Sprintf("  Storage Overhead: %d MB\n", rec.StorageOverhead/1024/1024)
		report += fmt.Sprintf("  Build Time: %s\n", rec.BuildTime)
		report += fmt.Sprintf("  Confidence: %.0f%%\n", rec.Confidence*100)
		report += fmt.Sprintf("  Reason: %s\n", rec.Reason)
		report += "\n"
	}

	return report
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
