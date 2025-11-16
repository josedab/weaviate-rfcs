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

// PageInfo represents pagination information following Relay cursor connections specification
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
}

// PageInfoType is the GraphQL type for PageInfo
var PageInfoType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "PageInfo",
	Description: "Information about pagination in a connection",
	Fields: graphql.Fields{
		"hasNextPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "When paginating forwards, are there more items?",
		},
		"hasPreviousPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "When paginating backwards, are there more items?",
		},
		"startCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "When paginating backwards, the cursor to continue",
		},
		"endCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "When paginating forwards, the cursor to continue",
		},
	},
})

// Edge represents a single item in a connection with cursor and search metadata
type Edge struct {
	Node     interface{} `json:"node"`
	Cursor   string      `json:"cursor"`
	Score    *float64    `json:"score,omitempty"`
	Distance *float64    `json:"distance,omitempty"`
}

// CreateEdgeType creates a GraphQL edge type for a specific node type
func CreateEdgeType(name string, nodeType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:        name + "Edge",
		Description: "An edge in a " + name + " connection",
		Fields: graphql.Fields{
			"node": &graphql.Field{
				Type:        graphql.NewNonNull(nodeType),
				Description: "The item at the end of the edge",
			},
			"cursor": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "A cursor for use in pagination",
			},
			"score": &graphql.Field{
				Type:        graphql.Float,
				Description: "Relevance score for search results (higher is better)",
			},
			"distance": &graphql.Field{
				Type:        graphql.Float,
				Description: "Distance from query vector (lower is better)",
			},
		},
	})
}

// Connection represents a paginated collection following Relay cursor connections
type Connection struct {
	Edges      []Edge   `json:"edges"`
	PageInfo   PageInfo `json:"pageInfo"`
	TotalCount int      `json:"totalCount"`
}

// CreateConnectionType creates a GraphQL connection type for a specific edge type
func CreateConnectionType(name string, edgeType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:        name + "Connection",
		Description: "A connection to a list of " + name + " items",
		Fields: graphql.Fields{
			"edges": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(edgeType))),
				Description: "A list of edges",
			},
			"pageInfo": &graphql.Field{
				Type:        graphql.NewNonNull(PageInfoType),
				Description: "Information to aid in pagination",
			},
			"totalCount": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Int),
				Description: "Total number of items in the connection",
			},
		},
	})
}

// VectorData represents vector embeddings and metadata
type VectorData struct {
	Vector []float32 `json:"vector,omitempty"`
}

// VectorDataType is the GraphQL type for vector data
var VectorDataType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "VectorData",
	Description: "Vector embedding data",
	Fields: graphql.Fields{
		"vector": &graphql.Field{
			Type:        graphql.NewList(graphql.Float),
			Description: "The vector embedding",
		},
	},
})

// ObjectMetadata represents metadata for Weaviate objects
type ObjectMetadata struct {
	CreationTimeUnix int64  `json:"creationTimeUnix,omitempty"`
	LastUpdateTimeUnix int64  `json:"lastUpdateTimeUnix,omitempty"`
	ExplainScore     string `json:"explainScore,omitempty"`
}

// ObjectMetadataType is the GraphQL type for object metadata
var ObjectMetadataType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "ObjectMetadata",
	Description: "Metadata about a Weaviate object",
	Fields: graphql.Fields{
		"creationTimeUnix": &graphql.Field{
			Type:        graphql.Int,
			Description: "Unix timestamp of object creation",
		},
		"lastUpdateTimeUnix": &graphql.Field{
			Type:        graphql.Int,
			Description: "Unix timestamp of last update",
		},
		"explainScore": &graphql.Field{
			Type:        graphql.String,
			Description: "Explanation of relevance score",
		},
	},
})
