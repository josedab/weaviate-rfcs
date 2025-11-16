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

package observability

import (
	"testing"
	"time"
)

func TestExplainPlanBuilder(t *testing.T) {
	// Create a builder for a hybrid query
	builder := NewExplainPlanBuilder("hybrid", "Article", 10)

	// Add a filter phase
	builder.AddFilterPhase(
		2*time.Millisecond,
		0.15, // selectivity
		"bitmap_and",
		1000000, // candidates before
		150000,  // candidates after
	)

	// Add vector search phase
	traversalPath := []TraversalLayer{
		{Layer: 3, Nodes: []uint64{12345, 23456}},
		{Layer: 2, Nodes: []uint64{23456, 34567, 45678}},
		{Layer: 1, Nodes: []uint64{45678, 56789}},
		{Layer: 0, Nodes: []uint64{56789, 67890, 78901}},
	}

	builder.AddVectorSearchPhase(
		12*time.Millisecond,
		"hnsw",
		12345,                  // entry point
		[]int{3, 2, 1, 0},     // layers traversed
		427,                    // nodes evaluated
		128,                    // ef used
		100,                    // result count
		traversalPath,
	)

	// Add keyword search phase
	builder.AddKeywordSearchPhase(
		8*time.Millisecond,
		"bm25f_blockmax_wand",
		[]string{"machine", "learning"},
		[]string{"title", "content"},
		45,  // blocks scanned
		312, // blocks skipped
		150, // result count
	)

	// Add fusion phase
	builder.AddFusionPhase(
		300*time.Microsecond,
		"ranked_fusion",
		[]float64{0.7, 0.3},
		250, // results before
		10,  // results after
	)

	// Set hybrid info
	builder.SetHybridInfo(0.7, "machine learning")

	// Add results
	vectorScore := 0.87
	keywordScore := 0.68
	builder.AddResults([]ResultScore{
		{
			ID:           "uuid-123",
			Score:        0.924,
			VectorScore:  &vectorScore,
			KeywordScore: &keywordScore,
		},
	})

	// Build the plan
	plan := builder.Build()

	// Verify the plan
	if plan.Query.Type != "hybrid" {
		t.Errorf("Expected query type 'hybrid', got '%s'", plan.Query.Type)
	}

	if plan.Query.Class != "Article" {
		t.Errorf("Expected class 'Article', got '%s'", plan.Query.Class)
	}

	if plan.Query.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", plan.Query.Limit)
	}

	if plan.Query.Hybrid == nil {
		t.Error("Expected hybrid info to be set")
	} else {
		if plan.Query.Hybrid.Alpha != 0.7 {
			t.Errorf("Expected alpha 0.7, got %f", plan.Query.Hybrid.Alpha)
		}
		if plan.Query.Hybrid.Query != "machine learning" {
			t.Errorf("Expected query 'machine learning', got '%s'", plan.Query.Hybrid.Query)
		}
	}

	// Verify phases
	if len(plan.Execution.Phases) != 4 {
		t.Errorf("Expected 4 phases, got %d", len(plan.Execution.Phases))
	}

	// Verify filter phase
	filterPhase := plan.Execution.Phases[0]
	if filterPhase.Phase != "filter" {
		t.Errorf("Expected phase 'filter', got '%s'", filterPhase.Phase)
	}
	if filterPhase.Selectivity == nil || *filterPhase.Selectivity != 0.15 {
		t.Error("Expected selectivity 0.15")
	}

	// Verify vector search phase
	vectorPhase := plan.Execution.Phases[1]
	if vectorPhase.Phase != "vector_search" {
		t.Errorf("Expected phase 'vector_search', got '%s'", vectorPhase.Phase)
	}
	if vectorPhase.Algorithm != "hnsw" {
		t.Errorf("Expected algorithm 'hnsw', got '%s'", vectorPhase.Algorithm)
	}
	if vectorPhase.NodesEvaluated == nil || *vectorPhase.NodesEvaluated != 427 {
		t.Error("Expected 427 nodes evaluated")
	}

	// Verify keyword search phase
	keywordPhase := plan.Execution.Phases[2]
	if keywordPhase.Phase != "keyword_search" {
		t.Errorf("Expected phase 'keyword_search', got '%s'", keywordPhase.Phase)
	}
	if len(keywordPhase.QueryTerms) != 2 {
		t.Errorf("Expected 2 query terms, got %d", len(keywordPhase.QueryTerms))
	}

	// Verify fusion phase
	fusionPhase := plan.Execution.Phases[3]
	if fusionPhase.Phase != "fusion" {
		t.Errorf("Expected phase 'fusion', got '%s'", fusionPhase.Phase)
	}
	if len(fusionPhase.Weights) != 2 {
		t.Errorf("Expected 2 weights, got %d", len(fusionPhase.Weights))
	}

	// Verify results
	if len(plan.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(plan.Results))
	}

	// Get performance breakdown
	breakdown := builder.GetPerformanceBreakdown()
	if breakdown.FilterMS == 0 {
		t.Error("Expected non-zero filter duration")
	}
	if breakdown.VectorSearchMS == 0 {
		t.Error("Expected non-zero vector search duration")
	}
	if breakdown.KeywordSearchMS == 0 {
		t.Error("Expected non-zero keyword search duration")
	}
	if breakdown.FusionMS == 0 {
		t.Error("Expected non-zero fusion duration")
	}
}

func TestPerformanceBreakdown(t *testing.T) {
	builder := NewExplainPlanBuilder("vector", "Document", 20)

	builder.AddVectorSearchPhase(
		15*time.Millisecond,
		"hnsw",
		54321,
		[]int{2, 1, 0},
		250,
		64,
		20,
		nil,
	)

	breakdown := builder.GetPerformanceBreakdown()

	if breakdown.VectorSearchMS == 0 {
		t.Error("Expected non-zero vector search duration")
	}

	if breakdown.KeywordSearchMS != 0 {
		t.Error("Expected zero keyword search duration for vector-only query")
	}
}
