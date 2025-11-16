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

package types

import (
	"github.com/tailor-inc/graphql"
)

// VectorInputType is the GraphQL input type for vector search
var VectorInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "VectorInput",
	Description: "Input for vector-based search",
	Fields: graphql.InputObjectConfigFieldMap{
		"vector": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewList(graphql.Float),
			Description: "The query vector",
		},
		"text": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "Text to vectorize for search",
		},
		"certainty": &graphql.InputObjectFieldConfig{
			Type:        graphql.Float,
			Description: "Minimum certainty threshold (0-1)",
		},
		"distance": &graphql.InputObjectFieldConfig{
			Type:        graphql.Float,
			Description: "Maximum distance threshold",
		},
	},
})

// HybridInputType is the GraphQL input type for hybrid search
var HybridInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name:        "HybridInput",
	Description: "Input for hybrid (vector + keyword) search",
	Fields: graphql.InputObjectConfigFieldMap{
		"query": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "Search query text",
		},
		"vector": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewList(graphql.Float),
			Description: "Optional query vector",
		},
		"alpha": &graphql.InputObjectFieldConfig{
			Type:        graphql.Float,
			Description: "Balance between vector (1.0) and keyword (0.0) search",
		},
		"fusionType": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "Fusion algorithm: rankedFusion or relativeScoreFusion",
		},
	},
})

// SortDirectionEnum defines sort order
var SortDirectionEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "SortDirection",
	Description: "Sort direction",
	Values: graphql.EnumValueConfigMap{
		"ASC": &graphql.EnumValueConfig{
			Value:       "asc",
			Description: "Ascending order",
		},
		"DESC": &graphql.EnumValueConfig{
			Value:       "desc",
			Description: "Descending order",
		},
	},
})

// CreateSortInputType creates a sort input type for a specific class
func CreateSortInputType(className string) *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        className + "Sort",
		Description: "Sort options for " + className,
		Fields: graphql.InputObjectConfigFieldMap{
			"field": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Field to sort by",
			},
			"direction": &graphql.InputObjectFieldConfig{
				Type:        SortDirectionEnum,
				Description: "Sort direction",
			},
		},
	})
}

// OperatorEnum defines filter operators
var OperatorEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "Operator",
	Description: "Filter operator",
	Values: graphql.EnumValueConfigMap{
		"EQUAL": &graphql.EnumValueConfig{
			Value:       "Equal",
			Description: "Equal to",
		},
		"NOT_EQUAL": &graphql.EnumValueConfig{
			Value:       "NotEqual",
			Description: "Not equal to",
		},
		"GREATER_THAN": &graphql.EnumValueConfig{
			Value:       "GreaterThan",
			Description: "Greater than",
		},
		"GREATER_THAN_EQUAL": &graphql.EnumValueConfig{
			Value:       "GreaterThanEqual",
			Description: "Greater than or equal to",
		},
		"LESS_THAN": &graphql.EnumValueConfig{
			Value:       "LessThan",
			Description: "Less than",
		},
		"LESS_THAN_EQUAL": &graphql.EnumValueConfig{
			Value:       "LessThanEqual",
			Description: "Less than or equal to",
		},
		"LIKE": &graphql.EnumValueConfig{
			Value:       "Like",
			Description: "Pattern matching",
		},
		"CONTAINS_ANY": &graphql.EnumValueConfig{
			Value:       "ContainsAny",
			Description: "Contains any of the values",
		},
		"CONTAINS_ALL": &graphql.EnumValueConfig{
			Value:       "ContainsAll",
			Description: "Contains all of the values",
		},
	},
})

// CreateFilterInputType creates a filter input type for a specific class
func CreateFilterInputType(className string) *graphql.InputObject {
	filterType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        className + "Filter",
		Description: "Filter options for " + className,
		Fields: graphql.InputObjectConfigFieldMap{
			"field": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Field to filter on",
			},
			"operator": &graphql.InputObjectFieldConfig{
				Type:        OperatorEnum,
				Description: "Filter operator",
			},
			"value": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Value to compare against",
			},
			"valueInt": &graphql.InputObjectFieldConfig{
				Type:        graphql.Int,
				Description: "Integer value to compare against",
			},
			"valueFloat": &graphql.InputObjectFieldConfig{
				Type:        graphql.Float,
				Description: "Float value to compare against",
			},
			"valueBoolean": &graphql.InputObjectFieldConfig{
				Type:        graphql.Boolean,
				Description: "Boolean value to compare against",
			},
		},
	})

	// Add self-reference for nested AND/OR
	filterType.AddFieldConfig("and", &graphql.InputObjectFieldConfig{
		Type:        graphql.NewList(filterType),
		Description: "Logical AND of filters",
	})
	filterType.AddFieldConfig("or", &graphql.InputObjectFieldConfig{
		Type:        graphql.NewList(filterType),
		Description: "Logical OR of filters",
	})

	return filterType
}
