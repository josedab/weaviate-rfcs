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

package complexity

import (
	"errors"
	"fmt"

	"github.com/tailor-inc/graphql/language/ast"
)

var (
	// ErrQueryTooComplex is returned when a query exceeds the maximum complexity
	ErrQueryTooComplex = errors.New("query complexity exceeds maximum allowed")
)

// Config holds configuration for complexity analysis
type Config struct {
	MaxComplexity int
	CostMap       map[string]int
	DefaultCost   int
}

// Analyzer calculates and validates query complexity
type Analyzer struct {
	maxComplexity int
	costMap       map[string]int
	defaultCost   int
}

// NewAnalyzer creates a new complexity analyzer
func NewAnalyzer(config Config) *Analyzer {
	if config.MaxComplexity == 0 {
		config.MaxComplexity = 10000
	}
	if config.DefaultCost == 0 {
		config.DefaultCost = 1
	}
	if config.CostMap == nil {
		config.CostMap = getDefaultCostMap()
	}

	return &Analyzer{
		maxComplexity: config.MaxComplexity,
		costMap:       config.CostMap,
		defaultCost:   config.DefaultCost,
	}
}

// Calculate calculates the complexity of a GraphQL query
func (a *Analyzer) Calculate(doc *ast.Document) (int, error) {
	if doc == nil {
		return 0, nil
	}

	complexity := 0
	for _, def := range doc.Definitions {
		if op, ok := def.(*ast.OperationDefinition); ok {
			c := a.calculateSelectionSet(op.SelectionSet, 1)
			complexity += c
		}
	}

	if complexity > a.maxComplexity {
		return complexity, fmt.Errorf("%w: %d > %d", ErrQueryTooComplex, complexity, a.maxComplexity)
	}

	return complexity, nil
}

// calculateSelectionSet calculates complexity for a selection set
func (a *Analyzer) calculateSelectionSet(selectionSet *ast.SelectionSet, multiplier int) int {
	if selectionSet == nil {
		return 0
	}

	complexity := 0
	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			complexity += a.calculateField(sel, multiplier)
		case *ast.InlineFragment:
			complexity += a.calculateSelectionSet(sel.SelectionSet, multiplier)
		case *ast.FragmentSpread:
			// Fragment spreads would need to be resolved from the document
			// For now, apply a fixed cost
			complexity += a.defaultCost * multiplier
		}
	}

	return complexity
}

// calculateField calculates complexity for a field
func (a *Analyzer) calculateField(field *ast.Field, multiplier int) int {
	// Get base cost for this field
	fieldName := field.Name.Value
	cost, ok := a.costMap[fieldName]
	if !ok {
		cost = a.defaultCost
	}

	// Apply multiplier from parent
	totalCost := cost * multiplier

	// Check for list multipliers (limit, first, last arguments)
	listMultiplier := a.getListMultiplier(field)
	if listMultiplier > 1 {
		totalCost *= listMultiplier
	}

	// Recursively calculate nested fields
	if field.SelectionSet != nil {
		nestedMultiplier := multiplier
		if listMultiplier > 1 {
			nestedMultiplier = listMultiplier
		}
		totalCost += a.calculateSelectionSet(field.SelectionSet, nestedMultiplier)
	}

	return totalCost
}

// getListMultiplier extracts the list size from limit/first/last arguments
func (a *Analyzer) getListMultiplier(field *ast.Field) int {
	multiplier := 1

	for _, arg := range field.Arguments {
		argName := arg.Name.Value
		if argName == "limit" || argName == "first" || argName == "last" {
			if value, ok := arg.Value.(*ast.IntValue); ok {
				// Parse the integer value
				var intVal int
				fmt.Sscanf(value.Value, "%d", &intVal)
				if intVal > multiplier {
					multiplier = intVal
				}
			}
		}
	}

	// Cap multiplier at a reasonable value to prevent overflow
	if multiplier > 1000 {
		multiplier = 1000
	}

	return multiplier
}

// getDefaultCostMap returns default costs for common operations
func getDefaultCostMap() map[string]int {
	return map[string]int{
		// Query operations
		"article":           1,
		"articles":          2,
		"searchArticles":    5,
		"aggregateArticles": 10,

		// Field costs
		"id":          0,
		"title":       0,
		"content":     1,
		"_vector":     2,
		"_metadata":   1,
		"edges":       1,
		"node":        1,
		"pageInfo":    0,
		"totalCount":  1,

		// Vector operations
		"near":        5,
		"hybrid":      8,
		"where":       2,
		"sort":        2,
		"groupBy":     5,

		// Reference fields (more expensive)
		"author":      3,
		"categories":  3,
		"references":  3,
	}
}

// ValidateQuery validates a query's complexity
func (a *Analyzer) ValidateQuery(doc *ast.Document) error {
	_, err := a.Calculate(doc)
	return err
}
