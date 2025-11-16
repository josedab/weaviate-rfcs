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
	"sync"
	"time"

	"github.com/google/uuid"
)

// ExplainPlanBuilder helps build a query explain plan during execution
type ExplainPlanBuilder struct {
	mu    sync.Mutex
	plan  *QueryExplainPlan
	start time.Time
}

// NewExplainPlanBuilder creates a new explain plan builder
func NewExplainPlanBuilder(queryType, className string, limit int) *ExplainPlanBuilder {
	return &ExplainPlanBuilder{
		plan: &QueryExplainPlan{
			QueryID:   uuid.New().String(),
			Timestamp: time.Now(),
			Query: QueryInfo{
				Type:  queryType,
				Class: className,
				Limit: limit,
			},
			Execution: Execution{
				Phases: []ExecutionPhase{},
			},
		},
		start: time.Now(),
	}
}

// AddFilterPhase adds a filter phase to the explain plan
func (b *ExplainPlanBuilder) AddFilterPhase(duration time.Duration, selectivity float64,
	strategy string, candidatesBefore, candidatesAfter int,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	durationMS := float64(duration.Microseconds()) / 1000.0
	b.plan.Execution.Phases = append(b.plan.Execution.Phases, ExecutionPhase{
		Phase:            "filter",
		Duration:         durationMS,
		Selectivity:      &selectivity,
		Strategy:         strategy,
		CandidatesBefore: &candidatesBefore,
		CandidatesAfter:  &candidatesAfter,
	})
}

// AddVectorSearchPhase adds a vector search phase to the explain plan
func (b *ExplainPlanBuilder) AddVectorSearchPhase(duration time.Duration, algorithm string,
	entryPoint uint64, layersTraversed []int, nodesEvaluated, efUsed, resultCount int,
	traversalPath []TraversalLayer,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	durationMS := float64(duration.Microseconds()) / 1000.0
	b.plan.Execution.Phases = append(b.plan.Execution.Phases, ExecutionPhase{
		Phase:           "vector_search",
		Duration:        durationMS,
		Algorithm:       algorithm,
		EntryPoint:      &entryPoint,
		LayersTraversed: layersTraversed,
		NodesEvaluated:  &nodesEvaluated,
		EfUsed:          &efUsed,
		TraversalPath:   traversalPath,
		ResultCount:     &resultCount,
	})
}

// AddKeywordSearchPhase adds a keyword search phase to the explain plan
func (b *ExplainPlanBuilder) AddKeywordSearchPhase(duration time.Duration, algorithm string,
	queryTerms, properties []string, blocksScanned, blocksSkipped, resultCount int,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	durationMS := float64(duration.Microseconds()) / 1000.0
	skipRate := 0.0
	totalBlocks := blocksScanned + blocksSkipped
	if totalBlocks > 0 {
		skipRate = float64(blocksSkipped) / float64(totalBlocks)
	}

	b.plan.Execution.Phases = append(b.plan.Execution.Phases, ExecutionPhase{
		Phase:         "keyword_search",
		Duration:      durationMS,
		Algorithm:     algorithm,
		QueryTerms:    queryTerms,
		Properties:    properties,
		BlocksScanned: &blocksScanned,
		BlocksSkipped: &blocksSkipped,
		SkipRate:      &skipRate,
		ResultCount:   &resultCount,
	})
}

// AddFusionPhase adds a fusion phase to the explain plan
func (b *ExplainPlanBuilder) AddFusionPhase(duration time.Duration, algorithm string,
	weights []float64, resultsBefore, resultsAfter int,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	durationMS := float64(duration.Microseconds()) / 1000.0
	b.plan.Execution.Phases = append(b.plan.Execution.Phases, ExecutionPhase{
		Phase:         "fusion",
		Duration:      durationMS,
		Algorithm:     algorithm,
		Weights:       weights,
		ResultsBefore: &resultsBefore,
		ResultsAfter:  &resultsAfter,
	})
}

// AddResults adds result scores to the explain plan
func (b *ExplainPlanBuilder) AddResults(results []ResultScore) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.plan.Results = results
}

// SetHybridInfo sets hybrid query specific information
func (b *ExplainPlanBuilder) SetHybridInfo(alpha float64, query string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.plan.Query.Hybrid = &HybridQueryInfo{
		Alpha: alpha,
		Query: query,
	}
}

// Build finalizes and returns the explain plan
func (b *ExplainPlanBuilder) Build() *QueryExplainPlan {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.plan.TotalDuration = float64(time.Since(b.start).Microseconds()) / 1000.0
	return b.plan
}

// GetPerformanceBreakdown extracts performance breakdown from the plan
func (b *ExplainPlanBuilder) GetPerformanceBreakdown() PerformanceBreakdown {
	b.mu.Lock()
	defer b.mu.Unlock()

	breakdown := PerformanceBreakdown{}

	for _, phase := range b.plan.Execution.Phases {
		switch phase.Phase {
		case "filter":
			breakdown.FilterMS = phase.Duration
		case "vector_search":
			breakdown.VectorSearchMS = phase.Duration
		case "keyword_search":
			breakdown.KeywordSearchMS = phase.Duration
		case "fusion":
			breakdown.FusionMS = phase.Duration
		}
	}

	return breakdown
}
