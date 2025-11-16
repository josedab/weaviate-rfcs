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

package v2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/resolvers"
	"github.com/weaviate/weaviate/entities/schema"
)

// MockRepository implements resolvers.Repository for testing
type MockRepository struct {
	objects map[string]map[string]interface{}
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		objects: make(map[string]map[string]interface{}),
	}
}

func (m *MockRepository) GetByID(ctx context.Context, className string, id string) (interface{}, error) {
	if classObjs, ok := m.objects[className]; ok {
		if obj, ok := classObjs[id]; ok {
			return obj, nil
		}
	}
	return nil, nil
}

func (m *MockRepository) GetByIDs(ctx context.Context, className string, ids []string) ([]interface{}, []error) {
	results := make([]interface{}, len(ids))
	errors := make([]error, len(ids))

	for i, id := range ids {
		results[i], errors[i] = m.GetByID(ctx, className, id)
	}

	return results, errors
}

func (m *MockRepository) Search(ctx context.Context, params resolvers.SearchParams) (*resolvers.SearchResults, error) {
	results := &resolvers.SearchResults{
		Objects:    []interface{}{},
		Scores:     []float64{},
		Distances:  []float64{},
		TotalCount: 0,
	}

	if classObjs, ok := m.objects[params.ClassName]; ok {
		for _, obj := range classObjs {
			results.Objects = append(results.Objects, obj)
			results.TotalCount++
		}
	}

	return results, nil
}

func (m *MockRepository) Aggregate(ctx context.Context, params resolvers.AggregateParams) (interface{}, error) {
	return map[string]interface{}{
		"count": 0,
	}, nil
}

func TestNewHandler(t *testing.T) {
	// Create test schema
	testSchema := &schema.Schema{
		Objects: &schema.SemanticSchema{
			Classes: []*schema.Class{
				{
					Class:       "Article",
					Description: "A news article",
					Properties: []*schema.Property{
						{
							Name:     "title",
							DataType: []string{"string"},
						},
						{
							Name:     "content",
							DataType: []string{"text"},
						},
					},
				},
			},
		},
	}

	repo := NewMockRepository()
	config := Config{
		MaxComplexity: 10000,
	}

	handler, err := NewHandler(testSchema, repo, config)
	require.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.schema)
}

func TestExecuteSimpleQuery(t *testing.T) {
	testSchema := &schema.Schema{
		Objects: &schema.SemanticSchema{
			Classes: []*schema.Class{
				{
					Class:       "Article",
					Description: "A news article",
					Properties: []*schema.Property{
						{
							Name:     "title",
							DataType: []string{"string"},
						},
					},
				},
			},
		},
	}

	repo := NewMockRepository()
	repo.objects["Article"] = map[string]interface{}{
		"1": map[string]interface{}{
			"id":    "1",
			"title": "Test Article",
		},
	}

	config := Config{
		MaxComplexity: 10000,
	}

	handler, err := NewHandler(testSchema, repo, config)
	require.NoError(t, err)

	query := `
		{
			articles(limit: 10) {
				edges {
					node {
						id
						title
					}
				}
			}
		}
	`

	ctx := context.Background()
	result := handler.Execute(ctx, query, "", nil)

	assert.NotNil(t, result)
	assert.Len(t, result.Errors, 0)
}

func TestComplexityValidation(t *testing.T) {
	testSchema := &schema.Schema{
		Objects: &schema.SemanticSchema{
			Classes: []*schema.Class{
				{
					Class: "Article",
					Properties: []*schema.Property{
						{
							Name:     "title",
							DataType: []string{"string"},
						},
					},
				},
			},
		},
	}

	repo := NewMockRepository()
	config := Config{
		MaxComplexity: 10, // Very low limit for testing
	}

	handler, err := NewHandler(testSchema, repo, config)
	require.NoError(t, err)

	// Query that exceeds complexity
	query := `
		{
			articles(limit: 1000) {
				edges {
					node {
						id
						title
					}
				}
			}
		}
	`

	ctx := context.Background()
	result := handler.Execute(ctx, query, "", nil)

	assert.NotNil(t, result)
	assert.Greater(t, len(result.Errors), 0)
}
