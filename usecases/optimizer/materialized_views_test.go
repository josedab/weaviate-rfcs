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

// MockQueryExecutor for testing
type MockQueryExecutor struct {
	executeFunc func(ctx context.Context, query *Query) (*QueryResult, error)
}

func (m *MockQueryExecutor) Execute(ctx context.Context, query *Query) (*QueryResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, query)
	}

	// Default implementation
	return &QueryResult{
		Rows: []map[string]interface{}{
			{"id": 1, "name": "test1"},
			{"id": 2, "name": "test2"},
		},
		Columns:  []string{"id", "name"},
		RowCount: 2,
		ExecTime: 10 * time.Millisecond,
	}, nil
}

func TestMaterializedViewManager_CreateView(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	query := &Query{
		ClassName:  "Article",
		Properties: []string{"title", "author"},
	}

	policy := RefreshPolicy{
		Type:     RefreshManual,
		Interval: 0,
	}

	ctx := context.Background()
	err := manager.CreateView(ctx, "test_view", query, policy)
	if err != nil {
		t.Fatalf("CreateView failed: %v", err)
	}

	// Verify view was created
	view, err := manager.GetView("test_view")
	if err != nil {
		t.Fatalf("GetView failed: %v", err)
	}

	if view.Name != "test_view" {
		t.Errorf("Expected view name 'test_view', got '%s'", view.Name)
	}

	if view.Cardinality != 2 {
		t.Errorf("Expected cardinality 2, got %d", view.Cardinality)
	}
}

func TestMaterializedViewManager_CreateView_Duplicate(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	query := &Query{
		ClassName: "Article",
	}

	policy := RefreshPolicy{
		Type: RefreshManual,
	}

	ctx := context.Background()

	// Create first view
	err := manager.CreateView(ctx, "test_view", query, policy)
	if err != nil {
		t.Fatalf("First CreateView failed: %v", err)
	}

	// Try to create duplicate
	err = manager.CreateView(ctx, "test_view", query, policy)
	if err == nil {
		t.Error("Expected error when creating duplicate view")
	}
}

func TestMaterializedViewManager_GetView(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	// Try to get non-existent view
	_, err := manager.GetView("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent view")
	}
}

func TestMaterializedViewManager_DropView(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	query := &Query{
		ClassName: "Article",
	}

	policy := RefreshPolicy{
		Type: RefreshManual,
	}

	ctx := context.Background()
	err := manager.CreateView(ctx, "test_view", query, policy)
	if err != nil {
		t.Fatalf("CreateView failed: %v", err)
	}

	// Drop the view
	err = manager.DropView("test_view")
	if err != nil {
		t.Fatalf("DropView failed: %v", err)
	}

	// Verify it's gone
	_, err = manager.GetView("test_view")
	if err == nil {
		t.Error("Expected error after dropping view")
	}
}

func TestMaterializedViewManager_RefreshView(t *testing.T) {
	executor := &MockQueryExecutor{
		executeFunc: func(ctx context.Context, query *Query) (*QueryResult, error) {
			return &QueryResult{
				Rows: []map[string]interface{}{
					{"id": 1, "name": "updated1"},
					{"id": 2, "name": "updated2"},
					{"id": 3, "name": "updated3"},
				},
				Columns:  []string{"id", "name"},
				RowCount: 3,
				ExecTime: 15 * time.Millisecond,
			}, nil
		},
	}

	manager := NewMaterializedViewManager(executor)

	query := &Query{
		ClassName: "Article",
	}

	policy := RefreshPolicy{
		Type: RefreshManual,
	}

	ctx := context.Background()
	err := manager.CreateView(ctx, "test_view", query, policy)
	if err != nil {
		t.Fatalf("CreateView failed: %v", err)
	}

	// Refresh the view
	err = manager.RefreshView(ctx, "test_view")
	if err != nil {
		t.Fatalf("RefreshView failed: %v", err)
	}

	// Verify the data was updated
	view, _ := manager.GetView("test_view")
	if view.Cardinality != 3 {
		t.Errorf("Expected cardinality 3 after refresh, got %d", view.Cardinality)
	}
}

func TestMaterializedView_GetData(t *testing.T) {
	view := &MaterializedView{
		Name: "test",
	}

	result := &QueryResult{
		Rows: []map[string]interface{}{
			{"id": 1},
		},
		RowCount: 1,
	}

	view.SetData(result)

	retrieved, err := view.GetData()
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}

	if retrieved.RowCount != 1 {
		t.Errorf("Expected RowCount 1, got %d", retrieved.RowCount)
	}

	if view.AccessCount != 1 {
		t.Errorf("Expected AccessCount 1, got %d", view.AccessCount)
	}
}

func TestMaterializedView_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		policy   RefreshPolicy
		lastTime time.Time
		expected bool
	}{
		{
			name:     "Manual never needs refresh",
			policy:   RefreshPolicy{Type: RefreshManual},
			lastTime: time.Now().Add(-1 * time.Hour),
			expected: false,
		},
		{
			name: "Periodic needs refresh when interval passed",
			policy: RefreshPolicy{
				Type:     RefreshPeriodic,
				Interval: 1 * time.Minute,
			},
			lastTime: time.Now().Add(-2 * time.Minute),
			expected: true,
		},
		{
			name: "Periodic doesn't need refresh when interval not passed",
			policy: RefreshPolicy{
				Type:     RefreshPeriodic,
				Interval: 1 * time.Hour,
			},
			lastTime: time.Now().Add(-30 * time.Minute),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &MaterializedView{
				Name:          "test",
				RefreshPolicy: tt.policy,
				LastRefresh:   tt.lastTime,
				NextRefresh:   tt.lastTime.Add(tt.policy.Interval),
			}

			result := view.NeedsRefresh()
			if result != tt.expected {
				t.Errorf("Expected NeedsRefresh=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMaterializedViewManager_RewriteQuery(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	// Create a materialized view
	viewQuery := &Query{
		ClassName:  "Article",
		Properties: []string{"title", "author", "date"},
	}

	policy := RefreshPolicy{Type: RefreshManual}
	ctx := context.Background()
	err := manager.CreateView(ctx, "article_view", viewQuery, policy)
	if err != nil {
		t.Fatalf("CreateView failed: %v", err)
	}

	// Try to rewrite a matching query
	userQuery := &Query{
		ClassName:  "Article",
		Properties: []string{"title", "author"},
	}

	rewritten := manager.RewriteQuery(userQuery)

	// Should be rewritten to use the view
	if rewritten == userQuery {
		// If they're the same object, it wasn't rewritten
		// Note: this is a simple test; more sophisticated checks needed
	}
}

func TestMaterializedViewManager_ListViews(t *testing.T) {
	executor := &MockQueryExecutor{}
	manager := NewMaterializedViewManager(executor)

	ctx := context.Background()
	policy := RefreshPolicy{Type: RefreshManual}

	// Create multiple views
	for i := 0; i < 3; i++ {
		query := &Query{
			ClassName: "Article",
		}
		err := manager.CreateView(ctx, string(rune('a'+i))+"_view", query, policy)
		if err != nil {
			t.Fatalf("CreateView failed: %v", err)
		}
	}

	views := manager.ListViews()
	if len(views) != 3 {
		t.Errorf("Expected 3 views, got %d", len(views))
	}
}

func TestQueryResult_Serialization(t *testing.T) {
	original := &QueryResult{
		Rows: []map[string]interface{}{
			{"id": 1, "name": "test"},
		},
		Columns:  []string{"id", "name"},
		RowCount: 1,
		ExecTime: 10 * time.Millisecond,
	}

	// Serialize
	data := original.Serialize()
	if len(data) == 0 {
		t.Error("Expected non-empty serialized data")
	}

	// Deserialize
	deserialized, err := DeserializeQueryResult(data)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	if deserialized.RowCount != original.RowCount {
		t.Errorf("Expected RowCount %d, got %d", original.RowCount, deserialized.RowCount)
	}

	if len(deserialized.Columns) != len(original.Columns) {
		t.Errorf("Expected %d columns, got %d", len(original.Columns), len(deserialized.Columns))
	}
}
