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

func TestWorkloadAnalyzer_Analyze(t *testing.T) {
	analyzer := NewWorkloadAnalyzer(100 * time.Millisecond)

	workload := []Query{
		{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "John"},
			},
		},
		{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "Jane"},
			},
		},
		{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "published_date", Operator: ">", Value: "2024-01-01"},
			},
		},
	}

	result := analyzer.Analyze(workload)

	if result.TotalQueries != 3 {
		t.Errorf("Expected 3 total queries, got %d", result.TotalQueries)
	}

	if len(result.Patterns) == 0 {
		t.Error("Expected to find patterns")
	}
}

func TestWorkloadAnalyzer_FindMissingIndexes(t *testing.T) {
	analyzer := NewWorkloadAnalyzer(100 * time.Millisecond)

	// Create workload with repeated filter on same property
	workload := []Query{}
	for i := 0; i < 5; i++ {
		workload = append(workload, Query{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "John"},
			},
		})
	}

	result := analyzer.Analyze(workload)

	if len(result.MissingIndexes) == 0 {
		t.Error("Expected to find missing indexes")
	}

	// Verify the missing index is for 'author' property
	found := false
	for _, missing := range result.MissingIndexes {
		if missing.Class == "Article" && len(missing.Properties) == 1 && missing.Properties[0] == "author" {
			found = true
			if missing.Frequency < 5 {
				t.Errorf("Expected frequency >= 5, got %d", missing.Frequency)
			}
		}
	}

	if !found {
		t.Error("Expected to find missing index for Article.author")
	}
}

func TestWorkloadAnalyzer_RecommendIndexType(t *testing.T) {
	analyzer := NewWorkloadAnalyzer(100 * time.Millisecond)

	tests := []struct {
		operator string
		expected IndexType
	}{
		{"=", IndexTypeHash},
		{">", IndexTypeBTree},
		{"<", IndexTypeBTree},
		{"LIKE", IndexTypeInverted},
		{"contains", IndexTypeInverted},
	}

	for _, tt := range tests {
		filter := Filter{Operator: tt.operator}
		result := analyzer.recommendIndexType(filter)
		if result != tt.expected {
			t.Errorf("For operator %s, expected %s, got %s", tt.operator, tt.expected, result)
		}
	}
}

func TestIndexAdvisor_Recommend(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	// Create workload with frequent filters
	workload := []Query{}
	for i := 0; i < 10; i++ {
		workload = append(workload, Query{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "John"},
			},
		})
	}

	ctx := context.Background()
	recommendations := advisor.Recommend(ctx, workload)

	if len(recommendations) == 0 {
		t.Error("Expected to get index recommendations")
	}

	// Verify recommendation details
	for _, rec := range recommendations {
		if rec.Class != "Article" {
			t.Errorf("Expected class 'Article', got '%s'", rec.Class)
		}
		if rec.EstimatedSpeedup <= 1.0 {
			t.Errorf("Expected speedup > 1.0, got %f", rec.EstimatedSpeedup)
		}
		if rec.Confidence <= 0 || rec.Confidence > 1 {
			t.Errorf("Expected confidence in (0, 1], got %f", rec.Confidence)
		}
		if rec.Reason == "" {
			t.Error("Expected non-empty reason")
		}
	}
}

func TestIndexAdvisor_EstimateImpact(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	pattern := MissingIndexPattern{
		Class:      "Article",
		Properties: []string{"author"},
		Type:       IndexTypeHash,
		Frequency:  10,
	}

	workload := []Query{
		{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "John"},
			},
		},
		{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "title", Operator: "LIKE", Value: "test"},
			},
		},
	}

	impact := advisor.estimateImpact(pattern, workload)

	if impact.QueriesAffected == 0 {
		t.Error("Expected some queries to be affected")
	}
	if impact.Speedup <= 1.0 {
		t.Errorf("Expected speedup > 1.0, got %f", impact.Speedup)
	}
	if impact.StorageBytes <= 0 {
		t.Errorf("Expected positive storage overhead, got %d", impact.StorageBytes)
	}
	if impact.BuildTime <= 0 {
		t.Errorf("Expected positive build time, got %s", impact.BuildTime)
	}
}

func TestIndexAdvisor_EstimateStorage(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	tests := []struct {
		indexType IndexType
		minSize   int64
	}{
		{IndexTypeHash, 1000},
		{IndexTypeBTree, 1000},
		{IndexTypeInverted, 1000},
		{IndexTypeVector, 1000},
	}

	for _, tt := range tests {
		pattern := MissingIndexPattern{
			Type: tt.indexType,
		}
		size := advisor.estimateStorage(pattern)
		if size < tt.minSize {
			t.Errorf("For %s, expected size >= %d, got %d", tt.indexType, tt.minSize, size)
		}
	}
}

func TestIndexAdvisor_EstimateBuildTime(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	pattern := MissingIndexPattern{
		Type: IndexTypeHash,
	}

	buildTime := advisor.estimateBuildTime(pattern)
	if buildTime <= 0 {
		t.Errorf("Expected positive build time, got %s", buildTime)
	}
}

func TestIndexAdvisor_GenerateReport(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	workload := []Query{}
	for i := 0; i < 10; i++ {
		workload = append(workload, Query{
			ClassName: "Article",
			Filters: []Filter{
				{Property: "author", Operator: "=", Value: "John"},
			},
		})
	}

	ctx := context.Background()
	report := advisor.GenerateReport(ctx, workload)

	if report == "" {
		t.Error("Expected non-empty report")
	}

	// Check report contains key sections
	if len(report) < 100 {
		t.Error("Report seems too short")
	}
}

func TestIndexAdvisor_CalculateConfidence(t *testing.T) {
	estimator := &MockCardinalityEstimator{}
	costModel := NewMLCostModel(estimator)
	advisor := NewIndexAdvisor(costModel)

	pattern := MissingIndexPattern{
		Frequency: 15,
	}

	impact := ImpactEstimate{
		Speedup:         6.0,
		QueriesAffected: 10,
	}

	confidence := advisor.calculateConfidence(pattern, impact)

	if confidence <= 0 || confidence > 1 {
		t.Errorf("Expected confidence in (0, 1], got %f", confidence)
	}

	// High frequency + high speedup + many affected queries should give high confidence
	if confidence < 0.7 {
		t.Errorf("Expected high confidence (>0.7), got %f", confidence)
	}
}
