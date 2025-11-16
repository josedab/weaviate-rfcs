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

package resolvers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/tailor-inc/graphql"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/dataloader"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/types"
)

// Repository defines the interface for data access
type Repository interface {
	GetByID(ctx context.Context, className string, id string) (interface{}, error)
	GetByIDs(ctx context.Context, className string, ids []string) ([]interface{}, []error)
	Search(ctx context.Context, params SearchParams) (*SearchResults, error)
	Aggregate(ctx context.Context, params AggregateParams) (interface{}, error)
}

// SearchParams defines parameters for search operations
type SearchParams struct {
	ClassName string
	Near      *VectorParams
	Hybrid    *HybridParams
	Where     *FilterParams
	Limit     int
	Offset    int
	Sort      []SortParams
}

// VectorParams defines parameters for vector search
type VectorParams struct {
	Vector    []float32
	Text      string
	Certainty *float64
	Distance  *float64
}

// HybridParams defines parameters for hybrid search
type HybridParams struct {
	Query      string
	Vector     []float32
	Alpha      *float64
	FusionType string
}

// FilterParams defines parameters for filtering
type FilterParams struct {
	Field        string
	Operator     string
	Value        interface{}
	And          []*FilterParams
	Or           []*FilterParams
}

// SortParams defines parameters for sorting
type SortParams struct {
	Field     string
	Direction string
}

// AggregateParams defines parameters for aggregation
type AggregateParams struct {
	ClassName string
	Where     *FilterParams
	GroupBy   []string
}

// SearchResults contains search results with metadata
type SearchResults struct {
	Objects    []interface{}
	Scores     []float64
	Distances  []float64
	TotalCount int
}

// Resolver implements GraphQL resolvers for v2 API
type Resolver struct {
	repo    Repository
	loaders map[string]*dataloader.Loader
}

// NewResolver creates a new resolver
func NewResolver(repo Repository) *Resolver {
	return &Resolver{
		repo:    repo,
		loaders: make(map[string]*dataloader.Loader),
	}
}

// GetLoader gets or creates a DataLoader for a specific class
func (r *Resolver) GetLoader(className string) *dataloader.Loader {
	if loader, ok := r.loaders[className]; ok {
		return loader
	}

	// Create batch function for this class
	batchFn := func(ctx context.Context, keys []string) ([]interface{}, []error) {
		return r.repo.GetByIDs(ctx, className, keys)
	}

	loader := dataloader.NewLoader(batchFn, dataloader.Config{})
	r.loaders[className] = loader
	return loader
}

// ResolveObject resolves a single object by ID
func (r *Resolver) ResolveObject(params graphql.ResolveParams) (interface{}, error) {
	className := params.Info.FieldName
	id, ok := params.Args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("id is required")
	}

	loader := r.GetLoader(className)
	return loader.Load(params.Context, id)
}

// ResolveObjects resolves a list of objects with pagination
func (r *Resolver) ResolveObjects(params graphql.ResolveParams) (interface{}, error) {
	className := strings.TrimSuffix(params.Info.FieldName, "s")

	// Extract parameters
	searchParams := SearchParams{
		ClassName: className,
		Limit:     10, // default
	}

	if limit, ok := params.Args["limit"].(int); ok {
		searchParams.Limit = limit
	}
	if offset, ok := params.Args["offset"].(int); ok {
		searchParams.Offset = offset
	}

	// Parse where clause
	if whereArg, ok := params.Args["where"]; ok && whereArg != nil {
		searchParams.Where = parseFilter(whereArg)
	}

	// Parse sort
	if sortArg, ok := params.Args["sort"].([]interface{}); ok {
		searchParams.Sort = parseSort(sortArg)
	}

	// Execute search
	results, err := r.repo.Search(params.Context, searchParams)
	if err != nil {
		return nil, err
	}

	// Build connection
	return r.buildConnection(results, searchParams.Offset, searchParams.Limit), nil
}

// ResolveSearch resolves vector/hybrid search queries
func (r *Resolver) ResolveSearch(params graphql.ResolveParams) (interface{}, error) {
	className := strings.TrimPrefix(params.Info.FieldName, "search")
	className = strings.TrimSuffix(className, "s")

	searchParams := SearchParams{
		ClassName: className,
		Limit:     10, // default
	}

	if limit, ok := params.Args["limit"].(int); ok {
		searchParams.Limit = limit
	}

	// Parse near vector
	if nearArg, ok := params.Args["near"]; ok && nearArg != nil {
		searchParams.Near = parseVectorParams(nearArg)
	}

	// Parse hybrid
	if hybridArg, ok := params.Args["hybrid"]; ok && hybridArg != nil {
		searchParams.Hybrid = parseHybridParams(hybridArg)
	}

	// Execute search
	results, err := r.repo.Search(params.Context, searchParams)
	if err != nil {
		return nil, err
	}

	// Build connection with scores
	return r.buildSearchConnection(results), nil
}

// ResolveAggregate resolves aggregation queries
func (r *Resolver) ResolveAggregate(params graphql.ResolveParams) (interface{}, error) {
	className := strings.TrimPrefix(params.Info.FieldName, "aggregate")
	className = strings.TrimSuffix(className, "s")

	aggParams := AggregateParams{
		ClassName: className,
	}

	if whereArg, ok := params.Args["where"]; ok && whereArg != nil {
		aggParams.Where = parseFilter(whereArg)
	}

	if groupByArg, ok := params.Args["groupBy"].([]interface{}); ok {
		for _, g := range groupByArg {
			if field, ok := g.(string); ok {
				aggParams.GroupBy = append(aggParams.GroupBy, field)
			}
		}
	}

	return r.repo.Aggregate(params.Context, aggParams)
}

// buildConnection builds a connection from search results
func (r *Resolver) buildConnection(results *SearchResults, offset, limit int) types.Connection {
	edges := make([]types.Edge, len(results.Objects))
	for i, obj := range results.Objects {
		cursor := encodeCursor(offset + i)
		edges[i] = types.Edge{
			Node:   obj,
			Cursor: cursor,
		}
	}

	hasNextPage := results.TotalCount > offset+limit
	hasPrevPage := offset > 0

	startCursor := ""
	endCursor := ""
	if len(edges) > 0 {
		startCursor = edges[0].Cursor
		endCursor = edges[len(edges)-1].Cursor
	}

	return types.Connection{
		Edges: edges,
		PageInfo: types.PageInfo{
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPrevPage,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
		TotalCount: results.TotalCount,
	}
}

// buildSearchConnection builds a connection with scores and distances
func (r *Resolver) buildSearchConnection(results *SearchResults) types.Connection {
	edges := make([]types.Edge, len(results.Objects))
	for i, obj := range results.Objects {
		cursor := encodeCursor(i)
		edge := types.Edge{
			Node:   obj,
			Cursor: cursor,
		}

		if i < len(results.Scores) && results.Scores[i] > 0 {
			score := results.Scores[i]
			edge.Score = &score
		}

		if i < len(results.Distances) && results.Distances[i] > 0 {
			distance := results.Distances[i]
			edge.Distance = &distance
		}

		edges[i] = edge
	}

	return types.Connection{
		Edges: edges,
		PageInfo: types.PageInfo{
			HasNextPage:     false,
			HasPreviousPage: false,
			StartCursor:     "",
			EndCursor:       "",
		},
		TotalCount: len(edges),
	}
}

// Helper functions

func encodeCursor(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("cursor:%d", offset)))
}

func decodeCursor(cursor string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid cursor format")
	}
	return strconv.Atoi(parts[1])
}

func parseFilter(whereArg interface{}) *FilterParams {
	whereMap, ok := whereArg.(map[string]interface{})
	if !ok {
		return nil
	}

	filter := &FilterParams{}

	if field, ok := whereMap["field"].(string); ok {
		filter.Field = field
	}
	if op, ok := whereMap["operator"].(string); ok {
		filter.Operator = op
	}
	if value, ok := whereMap["value"]; ok {
		filter.Value = value
	}
	if valueInt, ok := whereMap["valueInt"]; ok {
		filter.Value = valueInt
	}
	if valueFloat, ok := whereMap["valueFloat"]; ok {
		filter.Value = valueFloat
	}
	if valueBool, ok := whereMap["valueBoolean"]; ok {
		filter.Value = valueBool
	}

	if andClauses, ok := whereMap["and"].([]interface{}); ok {
		for _, clause := range andClauses {
			if f := parseFilter(clause); f != nil {
				filter.And = append(filter.And, f)
			}
		}
	}

	if orClauses, ok := whereMap["or"].([]interface{}); ok {
		for _, clause := range orClauses {
			if f := parseFilter(clause); f != nil {
				filter.Or = append(filter.Or, f)
			}
		}
	}

	return filter
}

func parseSort(sortArg []interface{}) []SortParams {
	var sorts []SortParams
	for _, s := range sortArg {
		sortMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		sort := SortParams{
			Direction: "asc", // default
		}

		if field, ok := sortMap["field"].(string); ok {
			sort.Field = field
		}
		if direction, ok := sortMap["direction"].(string); ok {
			sort.Direction = direction
		}

		sorts = append(sorts, sort)
	}
	return sorts
}

func parseVectorParams(nearArg interface{}) *VectorParams {
	nearMap, ok := nearArg.(map[string]interface{})
	if !ok {
		return nil
	}

	params := &VectorParams{}

	if vectorArg, ok := nearMap["vector"].([]interface{}); ok {
		vector := make([]float32, len(vectorArg))
		for i, v := range vectorArg {
			if f, ok := v.(float64); ok {
				vector[i] = float32(f)
			}
		}
		params.Vector = vector
	}

	if text, ok := nearMap["text"].(string); ok {
		params.Text = text
	}

	if certainty, ok := nearMap["certainty"].(float64); ok {
		params.Certainty = &certainty
	}

	if distance, ok := nearMap["distance"].(float64); ok {
		params.Distance = &distance
	}

	return params
}

func parseHybridParams(hybridArg interface{}) *HybridParams {
	hybridMap, ok := hybridArg.(map[string]interface{})
	if !ok {
		return nil
	}

	params := &HybridParams{}

	if query, ok := hybridMap["query"].(string); ok {
		params.Query = query
	}

	if vectorArg, ok := hybridMap["vector"].([]interface{}); ok {
		vector := make([]float32, len(vectorArg))
		for i, v := range vectorArg {
			if f, ok := v.(float64); ok {
				vector[i] = float32(f)
			}
		}
		params.Vector = vector
	}

	if alpha, ok := hybridMap["alpha"].(float64); ok {
		params.Alpha = &alpha
	}

	if fusionType, ok := hybridMap["fusionType"].(string); ok {
		params.FusionType = fusionType
	}

	return params
}
