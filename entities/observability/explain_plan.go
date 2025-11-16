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
	"time"
)

// QueryExplainPlan represents the complete execution plan for a query
type QueryExplainPlan struct {
	QueryID       string        `json:"query_id"`
	Timestamp     time.Time     `json:"timestamp"`
	TotalDuration float64       `json:"total_duration_ms"`
	Query         QueryInfo     `json:"query"`
	Execution     Execution     `json:"execution"`
	Results       []ResultScore `json:"results,omitempty"`
}

// QueryInfo contains information about the query itself
type QueryInfo struct {
	Type    string                 `json:"type"` // "vector", "keyword", "hybrid", etc.
	Class   string                 `json:"class"`
	Limit   int                    `json:"limit"`
	Filters map[string]interface{} `json:"filters,omitempty"`
	Hybrid  *HybridQueryInfo       `json:"hybrid,omitempty"`
}

// HybridQueryInfo contains hybrid search specific information
type HybridQueryInfo struct {
	Alpha float64 `json:"alpha"`
	Query string  `json:"query"`
}

// Execution contains the execution plan phases
type Execution struct {
	Phases []ExecutionPhase `json:"phases"`
}

// ExecutionPhase represents a single phase in query execution
type ExecutionPhase struct {
	Phase    string  `json:"phase"` // "filter", "vector_search", "keyword_search", "fusion"
	Duration float64 `json:"duration_ms"`

	// Filter phase specific
	Selectivity      *float64 `json:"selectivity,omitempty"`
	Strategy         string   `json:"strategy,omitempty"`
	CandidatesBefore *int     `json:"candidates_before,omitempty"`
	CandidatesAfter  *int     `json:"candidates_after,omitempty"`

	// Vector search phase specific
	Algorithm      string            `json:"algorithm,omitempty"`
	EntryPoint     *uint64           `json:"entry_point,omitempty"`
	LayersTraversed []int            `json:"layers_traversed,omitempty"`
	NodesEvaluated *int              `json:"nodes_evaluated,omitempty"`
	EfUsed         *int              `json:"ef_used,omitempty"`
	TraversalPath  []TraversalLayer  `json:"traversal_path,omitempty"`
	ResultCount    *int              `json:"result_count,omitempty"`

	// Keyword search phase specific
	QueryTerms    []string  `json:"query_terms,omitempty"`
	Properties    []string  `json:"properties,omitempty"`
	BlocksScanned *int      `json:"blocks_scanned,omitempty"`
	BlocksSkipped *int      `json:"blocks_skipped,omitempty"`
	SkipRate      *float64  `json:"skip_rate,omitempty"`

	// Fusion phase specific
	Weights       []float64 `json:"weights,omitempty"`
	ResultsBefore *int      `json:"results_before,omitempty"`
	ResultsAfter  *int      `json:"results_after,omitempty"`
}

// TraversalLayer represents nodes visited in a specific HNSW layer
type TraversalLayer struct {
	Layer int      `json:"layer"`
	Nodes []uint64 `json:"nodes"`
}

// ResultScore contains score breakdown for a result
type ResultScore struct {
	ID           string   `json:"id"`
	Score        float64  `json:"score"`
	VectorScore  *float64 `json:"vector_score,omitempty"`
	KeywordScore *float64 `json:"keyword_score,omitempty"`
}

// PerformanceBreakdown provides timing breakdown for slow query logging
type PerformanceBreakdown struct {
	FilterMS        float64 `json:"filter_ms"`
	VectorSearchMS  float64 `json:"vector_search_ms"`
	KeywordSearchMS float64 `json:"keyword_search_ms"`
	FusionMS        float64 `json:"fusion_ms"`
	SerializationMS float64 `json:"serialization_ms"`
}

// SlowQueryLog represents enhanced slow query log entry
type SlowQueryLog struct {
	Timestamp time.Time `json:"timestamp"`
	Duration  float64   `json:"duration_ms"`
	QueryType string    `json:"query_type"`

	// Context
	ClassName      string `json:"class"`
	ShardName      string `json:"shard"`
	Limit          int    `json:"limit"`
	FiltersApplied bool   `json:"filters_applied"`
	FilterSummary  string `json:"filter_summary,omitempty"`

	// Performance breakdown
	Breakdown PerformanceBreakdown `json:"breakdown"`

	// Full explain plan
	ExplainPlan *QueryExplainPlan `json:"explain_plan,omitempty"`
}
