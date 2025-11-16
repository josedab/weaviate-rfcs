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
	"fmt"

	"github.com/tailor-inc/graphql"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/resolvers"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/types"
	"github.com/weaviate/weaviate/entities/schema"
)

// SchemaBuilder builds GraphQL v2 schema from Weaviate schema
type SchemaBuilder struct {
	resolver      *resolvers.Resolver
	classTypes    map[string]*graphql.Object
	connectionTypes map[string]*graphql.Object
	edgeTypes     map[string]*graphql.Object
}

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder(resolver *resolvers.Resolver) *SchemaBuilder {
	return &SchemaBuilder{
		resolver:      resolver,
		classTypes:    make(map[string]*graphql.Object),
		connectionTypes: make(map[string]*graphql.Object),
		edgeTypes:     make(map[string]*graphql.Object),
	}
}

// Build builds the complete GraphQL schema
func (b *SchemaBuilder) Build(weaviateSchema *schema.Schema) (graphql.Schema, error) {
	// Build types for each class
	for _, class := range weaviateSchema.Objects.Classes {
		if err := b.buildClassTypes(class); err != nil {
			return graphql.Schema{}, fmt.Errorf("failed to build types for class %s: %w", class.Class, err)
		}
	}

	// Build root query type
	queryType := b.buildQueryType(weaviateSchema)

	// Build schema
	schemaConfig := graphql.SchemaConfig{
		Query: queryType,
	}

	return graphql.NewSchema(schemaConfig)
}

// buildClassTypes builds GraphQL types for a Weaviate class
func (b *SchemaBuilder) buildClassTypes(class *schema.Class) error {
	className := class.Class

	// Build object type
	objectType := b.buildObjectType(class)
	b.classTypes[className] = objectType

	// Build edge type
	edgeType := types.CreateEdgeType(className, objectType)
	b.edgeTypes[className] = edgeType

	// Build connection type
	connectionType := types.CreateConnectionType(className, edgeType)
	b.connectionTypes[className] = connectionType

	return nil
}

// buildObjectType builds a GraphQL object type for a Weaviate class
func (b *SchemaBuilder) buildObjectType(class *schema.Class) *graphql.Object {
	className := class.Class

	fields := graphql.Fields{
		"id": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.ID),
			Description: "Unique identifier",
		},
	}

	// Add properties
	for _, prop := range class.Properties {
		fieldType := b.getGraphQLType(prop)
		fields[prop.Name] = &graphql.Field{
			Type:        fieldType,
			Description: prop.Description,
		}
	}

	// Add vector field
	fields["_vector"] = &graphql.Field{
		Type:        types.VectorDataType,
		Description: "Vector embedding data",
	}

	// Add metadata field
	fields["_metadata"] = &graphql.Field{
		Type:        types.ObjectMetadataType,
		Description: "Object metadata",
	}

	return graphql.NewObject(graphql.ObjectConfig{
		Name:        className,
		Description: class.Description,
		Fields:      fields,
	})
}

// buildQueryType builds the root Query type
func (b *SchemaBuilder) buildQueryType(weaviateSchema *schema.Schema) *graphql.Object {
	fields := graphql.Fields{}

	for _, class := range weaviateSchema.Objects.Classes {
		className := class.Class

		// Single object query: article(id: ID!): Article
		fields[b.toLowerFirst(className)] = &graphql.Field{
			Type:        b.classTypes[className],
			Description: fmt.Sprintf("Get a single %s by ID", className),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.ID),
					Description: "Object ID",
				},
			},
			Resolve: b.resolver.ResolveObject,
		}

		// List query: articles(...): ArticleConnection!
		pluralName := b.pluralize(className)
		filterType := types.CreateFilterInputType(className)
		sortType := types.CreateSortInputType(className)

		fields[b.toLowerFirst(pluralName)] = &graphql.Field{
			Type:        graphql.NewNonNull(b.connectionTypes[className]),
			Description: fmt.Sprintf("Get a list of %s", pluralName),
			Args: graphql.FieldConfigArgument{
				"where": &graphql.ArgumentConfig{
					Type:        filterType,
					Description: "Filter criteria",
				},
				"limit": &graphql.ArgumentConfig{
					Type:        graphql.Int,
					Description: "Maximum number of results",
				},
				"offset": &graphql.ArgumentConfig{
					Type:        graphql.Int,
					Description: "Number of results to skip",
				},
				"sort": &graphql.ArgumentConfig{
					Type:        graphql.NewList(graphql.NewNonNull(sortType)),
					Description: "Sort order",
				},
			},
			Resolve: b.resolver.ResolveObjects,
		}

		// Search query: searchArticles(...): ArticleConnection!
		fields["search"+pluralName] = &graphql.Field{
			Type:        graphql.NewNonNull(b.connectionTypes[className]),
			Description: fmt.Sprintf("Search %s using vector/hybrid search", pluralName),
			Args: graphql.FieldConfigArgument{
				"near": &graphql.ArgumentConfig{
					Type:        types.VectorInputType,
					Description: "Vector search parameters",
				},
				"hybrid": &graphql.ArgumentConfig{
					Type:        types.HybridInputType,
					Description: "Hybrid search parameters",
				},
				"limit": &graphql.ArgumentConfig{
					Type:        graphql.Int,
					Description: "Maximum number of results",
					DefaultValue: 10,
				},
			},
			Resolve: b.resolver.ResolveSearch,
		}

		// Aggregate query: aggregateArticles(...): ArticleAggregation!
		fields["aggregate"+pluralName] = &graphql.Field{
			Type:        graphql.String, // Simplified for now
			Description: fmt.Sprintf("Aggregate %s", pluralName),
			Args: graphql.FieldConfigArgument{
				"where": &graphql.ArgumentConfig{
					Type:        filterType,
					Description: "Filter criteria",
				},
				"groupBy": &graphql.ArgumentConfig{
					Type:        graphql.NewList(graphql.NewNonNull(graphql.String)),
					Description: "Fields to group by",
				},
			},
			Resolve: b.resolver.ResolveAggregate,
		}
	}

	return graphql.NewObject(graphql.ObjectConfig{
		Name:        "Query",
		Description: "Root query type",
		Fields:      fields,
	})
}

// getGraphQLType maps Weaviate property types to GraphQL types
func (b *SchemaBuilder) getGraphQLType(prop *schema.Property) graphql.Output {
	// Handle arrays
	if prop.DataType[0] == "[]" {
		// This is a simplified check - real implementation would need proper parsing
		return graphql.NewList(graphql.String)
	}

	// Map data types
	switch prop.DataType[0] {
	case "string", "text":
		return graphql.String
	case "int":
		return graphql.Int
	case "number", "float":
		return graphql.Float
	case "boolean":
		return graphql.Boolean
	case "date":
		return graphql.String // DateTime as string in ISO format
	case "uuid":
		return graphql.ID
	default:
		// Check if it's a cross-reference
		if b.classTypes[prop.DataType[0]] != nil {
			return b.classTypes[prop.DataType[0]]
		}
		return graphql.String
	}
}

// Helper functions

func (b *SchemaBuilder) toLowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]|32) + s[1:]
}

func (b *SchemaBuilder) pluralize(s string) string {
	// Simple pluralization - real implementation would use proper rules
	if len(s) == 0 {
		return s
	}

	// Common special cases
	switch s {
	case "Person":
		return "People"
	case "Category":
		return "Categories"
	}

	// Default: add 's'
	if s[len(s)-1] == 'y' {
		return s[:len(s)-1] + "ies"
	}
	return s + "s"
}
