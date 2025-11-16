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
	"sync"
	"time"
)

// RefreshType defines how a materialized view is refreshed
type RefreshType string

const (
	RefreshManual      RefreshType = "manual"
	RefreshPeriodic    RefreshType = "periodic"
	RefreshIncremental RefreshType = "incremental"
)

// RefreshPolicy defines when and how a materialized view should be refreshed
type RefreshPolicy struct {
	Type     RefreshType
	Interval time.Duration // For periodic refresh
	OnWrite  bool          // For incremental refresh
}

// Query represents a query that can be materialized
type Query struct {
	ClassName    string
	Filters      []Filter
	Aggregations []Aggregation
	Joins        []Join
	Properties   []string
}

// Filter represents a query filter
type Filter struct {
	Property string
	Operator string
	Value    interface{}
}

// Aggregation represents an aggregation operation
type Aggregation struct {
	Type     string // count, sum, avg, min, max
	Property string
}

// Join represents a join operation
type Join struct {
	ClassName  string
	Property   string
	JoinType   string
	Conditions []Filter
}

// QueryResult represents the result of a query execution
type QueryResult struct {
	Rows      []map[string]interface{}
	Columns   []string
	RowCount  int64
	ExecTime  time.Duration
}

// Serialize converts query result to bytes
func (qr *QueryResult) Serialize() []byte {
	data, _ := json.Marshal(qr)
	return data
}

// DeserializeQueryResult converts bytes back to query result
func DeserializeQueryResult(data []byte) (*QueryResult, error) {
	var qr QueryResult
	err := json.Unmarshal(data, &qr)
	return &qr, err
}

// MaterializedView represents a cached query result
type MaterializedView struct {
	Name          string
	Query         *Query
	RefreshPolicy RefreshPolicy
	LastRefresh   time.Time
	NextRefresh   time.Time

	// Storage
	Data        []byte
	Cardinality int64
	Size        int64

	// Metadata
	Created     time.Time
	Accessed    time.Time
	AccessCount int64

	mu sync.RWMutex
}

// GetData retrieves the materialized data
func (mv *MaterializedView) GetData() (*QueryResult, error) {
	mv.mu.RLock()
	defer mv.mu.RUnlock()

	mv.Accessed = time.Now()
	mv.AccessCount++

	return DeserializeQueryResult(mv.Data)
}

// SetData updates the materialized data
func (mv *MaterializedView) SetData(result *QueryResult) {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	mv.Data = result.Serialize()
	mv.Cardinality = result.RowCount
	mv.Size = int64(len(mv.Data))
	mv.LastRefresh = time.Now()

	// Update next refresh time for periodic views
	if mv.RefreshPolicy.Type == RefreshPeriodic {
		mv.NextRefresh = time.Now().Add(mv.RefreshPolicy.Interval)
	}
}

// NeedsRefresh checks if the view needs to be refreshed
func (mv *MaterializedView) NeedsRefresh() bool {
	mv.mu.RLock()
	defer mv.mu.RUnlock()

	switch mv.RefreshPolicy.Type {
	case RefreshManual:
		return false
	case RefreshPeriodic:
		return time.Now().After(mv.NextRefresh)
	case RefreshIncremental:
		// Would check for data changes
		return false
	default:
		return false
	}
}

// QueryExecutor interface for executing queries
type QueryExecutor interface {
	Execute(ctx context.Context, query *Query) (*QueryResult, error)
}

// MaterializedViewManager manages materialized views
type MaterializedViewManager struct {
	views     map[string]*MaterializedView
	executor  QueryExecutor
	refresher *Refresher
	optimizer *ViewOptimizer

	mu sync.RWMutex
}

// NewMaterializedViewManager creates a new materialized view manager
func NewMaterializedViewManager(executor QueryExecutor) *MaterializedViewManager {
	mgr := &MaterializedViewManager{
		views:    make(map[string]*MaterializedView),
		executor: executor,
	}

	mgr.refresher = NewRefresher(mgr)
	mgr.optimizer = NewViewOptimizer(mgr)

	return mgr
}

// CreateView creates a new materialized view
func (m *MaterializedViewManager) CreateView(ctx context.Context, name string, query *Query, policy RefreshPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if view already exists
	if _, exists := m.views[name]; exists {
		return fmt.Errorf("materialized view '%s' already exists", name)
	}

	// Execute query to get initial data
	result, err := m.executor.Execute(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute query for materialized view: %w", err)
	}

	// Create the view
	view := &MaterializedView{
		Name:          name,
		Query:         query,
		RefreshPolicy: policy,
		LastRefresh:   time.Now(),
		Created:       time.Now(),
		Accessed:      time.Now(),
	}

	view.SetData(result)

	m.views[name] = view

	// Schedule refresh if periodic
	if policy.Type == RefreshPeriodic {
		m.refresher.Schedule(view)
	}

	return nil
}

// GetView retrieves a materialized view by name
func (m *MaterializedViewManager) GetView(name string) (*MaterializedView, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	view, exists := m.views[name]
	if !exists {
		return nil, fmt.Errorf("materialized view '%s' not found", name)
	}

	return view, nil
}

// DropView removes a materialized view
func (m *MaterializedViewManager) DropView(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	view, exists := m.views[name]
	if !exists {
		return fmt.Errorf("materialized view '%s' not found", name)
	}

	// Unschedule refresh
	m.refresher.Unschedule(view)

	delete(m.views, name)
	return nil
}

// RefreshView manually refreshes a materialized view
func (m *MaterializedViewManager) RefreshView(ctx context.Context, name string) error {
	view, err := m.GetView(name)
	if err != nil {
		return err
	}

	// Execute query to get fresh data
	result, err := m.executor.Execute(ctx, view.Query)
	if err != nil {
		return fmt.Errorf("failed to refresh materialized view: %w", err)
	}

	view.SetData(result)
	return nil
}

// RewriteQuery attempts to rewrite a query to use materialized views
func (m *MaterializedViewManager) RewriteQuery(query *Query) *Query {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find matching views
	matches := m.findMatchingViews(query)
	if len(matches) == 0 {
		return query
	}

	// Select best view
	best := m.selectBestView(matches, query)
	if best == nil {
		return query
	}

	// Rewrite query to use view
	return m.rewriteWithView(query, best)
}

// findMatchingViews finds materialized views that could satisfy the query
func (m *MaterializedViewManager) findMatchingViews(query *Query) []*MaterializedView {
	var matches []*MaterializedView

	for _, view := range m.views {
		if m.canUseView(query, view) {
			matches = append(matches, view)
		}
	}

	return matches
}

// canUseView checks if a materialized view can be used for a query
func (m *MaterializedViewManager) canUseView(query *Query, view *MaterializedView) bool {
	// Simple matching: check if class name matches
	if query.ClassName != view.Query.ClassName {
		return false
	}

	// Check if all required properties are available
	viewProps := make(map[string]bool)
	for _, prop := range view.Query.Properties {
		viewProps[prop] = true
	}

	for _, prop := range query.Properties {
		if !viewProps[prop] {
			return false
		}
	}

	// More sophisticated matching would check filters, aggregations, etc.
	return true
}

// selectBestView selects the best materialized view for a query
func (m *MaterializedViewManager) selectBestView(matches []*MaterializedView, query *Query) *MaterializedView {
	if len(matches) == 0 {
		return nil
	}

	// Simple heuristic: select most recently accessed view
	best := matches[0]
	for _, view := range matches[1:] {
		if view.AccessCount > best.AccessCount {
			best = view
		}
	}

	return best
}

// rewriteWithView rewrites a query to use a materialized view
func (m *MaterializedViewManager) rewriteWithView(query *Query, view *MaterializedView) *Query {
	// Create a new query that reads from the materialized view
	// This is a simplified version
	rewritten := &Query{
		ClassName:  fmt.Sprintf("__materialized__%s", view.Name),
		Properties: query.Properties,
		Filters:    query.Filters,
	}

	return rewritten
}

// ListViews returns all materialized views
func (m *MaterializedViewManager) ListViews() []*MaterializedView {
	m.mu.RLock()
	defer m.mu.RUnlock()

	views := make([]*MaterializedView, 0, len(m.views))
	for _, view := range m.views {
		views = append(views, view)
	}

	return views
}

// Refresher handles periodic refresh of materialized views
type Refresher struct {
	manager   *MaterializedViewManager
	scheduled map[string]*time.Timer
	mu        sync.Mutex
}

// NewRefresher creates a new refresher
func NewRefresher(manager *MaterializedViewManager) *Refresher {
	return &Refresher{
		manager:   manager,
		scheduled: make(map[string]*time.Timer),
	}
}

// Schedule schedules a view for periodic refresh
func (r *Refresher) Schedule(view *MaterializedView) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if view.RefreshPolicy.Type != RefreshPeriodic {
		return
	}

	// Cancel existing timer if any
	if timer, exists := r.scheduled[view.Name]; exists {
		timer.Stop()
	}

	// Schedule new timer
	timer := time.AfterFunc(view.RefreshPolicy.Interval, func() {
		r.refresh(view)
	})

	r.scheduled[view.Name] = timer
}

// Unschedule removes a view from the refresh schedule
func (r *Refresher) Unschedule(view *MaterializedView) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timer, exists := r.scheduled[view.Name]; exists {
		timer.Stop()
		delete(r.scheduled, view.Name)
	}
}

// refresh refreshes a single view
func (r *Refresher) refresh(view *MaterializedView) {
	ctx := context.Background()
	err := r.manager.RefreshView(ctx, view.Name)
	if err != nil {
		// Log error (in production, use proper logging)
		fmt.Printf("Error refreshing view %s: %v\n", view.Name, err)
	}

	// Reschedule
	r.Schedule(view)
}

// ViewOptimizer analyzes view usage and suggests optimizations
type ViewOptimizer struct {
	manager *MaterializedViewManager
}

// NewViewOptimizer creates a new view optimizer
func NewViewOptimizer(manager *MaterializedViewManager) *ViewOptimizer {
	return &ViewOptimizer{
		manager: manager,
	}
}

// SuggestViews suggests new materialized views based on query patterns
func (vo *ViewOptimizer) SuggestViews(queryLogs []Query) []ViewSuggestion {
	// Analyze query patterns and suggest views
	// This would use ML or heuristics to identify common query patterns
	return nil
}

// ViewSuggestion represents a suggested materialized view
type ViewSuggestion struct {
	Query             *Query
	EstimatedBenefit  float64
	EstimatedSize     int64
	AffectedQueries   int
}
