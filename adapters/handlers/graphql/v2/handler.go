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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/tailor-inc/graphql"
	"github.com/tailor-inc/graphql/gqlerrors"
	"github.com/tailor-inc/graphql/language/parser"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/complexity"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/middleware"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/resolvers"
	"github.com/weaviate/weaviate/adapters/handlers/graphql/v2/translation"
	"github.com/weaviate/weaviate/entities/schema"
)

// Config holds configuration for the v2 GraphQL API
type Config struct {
	MaxComplexity  int
	EnableV1Compat bool // Enable automatic v1 to v2 translation
	Logger         logrus.FieldLogger
}

// Handler implements the GraphQL v2 API
type Handler struct {
	schema       graphql.Schema
	translator   *translation.Translator
	analyzer     *complexity.Analyzer
	config       Config
	logger       logrus.FieldLogger
}

// NewHandler creates a new v2 GraphQL handler
func NewHandler(weaviateSchema *schema.Schema, repo resolvers.Repository, config Config) (*Handler, error) {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	// Extract class names for translator
	classNames := make([]string, 0, len(weaviateSchema.Objects.Classes))
	for _, class := range weaviateSchema.Objects.Classes {
		classNames = append(classNames, class.Class)
	}

	// Build GraphQL schema
	resolver := resolvers.NewResolver(repo)
	builder := NewSchemaBuilder(resolver)
	gqlSchema, err := builder.Build(weaviateSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to build GraphQL schema: %w", err)
	}

	// Create translator
	translator := translation.NewTranslator(classNames)

	// Create complexity analyzer
	analyzer := complexity.NewAnalyzer(complexity.Config{
		MaxComplexity: config.MaxComplexity,
	})

	return &Handler{
		schema:     gqlSchema,
		translator: translator,
		analyzer:   analyzer,
		config:     config,
		logger:     config.Logger,
	}, nil
}

// GraphQLRequest represents a GraphQL HTTP request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL HTTP response
type GraphQLResponse struct {
	Data   interface{}   `json:"data,omitempty"`
	Errors []interface{} `json:"errors,omitempty"`
}

// ServeHTTP handles GraphQL HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Check for v1 compatibility mode
	if h.config.EnableV1Compat && r.Header.Get("X-API-Version") == "1" {
		h.logger.Debug("Translating v1 query to v2")
		translatedQuery, err := h.translator.TranslateV1ToV2(req.Query)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "Failed to translate v1 query", err)
			return
		}
		req.Query = translatedQuery
	}

	// Execute query
	result := h.Execute(ctx, req.Query, req.OperationName, req.Variables)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if len(result.Errors) > 0 {
		w.WriteHeader(http.StatusOK) // GraphQL always returns 200 with errors in response
	}
	json.NewEncoder(w).Encode(result)
}

// Execute executes a GraphQL query
func (h *Handler) Execute(ctx context.Context, query string, operationName string, variables map[string]interface{}) *graphql.Result {
	// Parse query
	doc, err := parser.Parse(parser.ParseParams{
		Source: query,
	})
	if err != nil {
		return &graphql.Result{
			Errors: []gqlerrors.FormattedError{
				*middleware.WrapError(err, middleware.ErrorCodeInvalidArgument, nil),
			},
		}
	}

	// Validate complexity
	if err := h.analyzer.ValidateQuery(doc); err != nil {
		complexity, _ := h.analyzer.Calculate(doc)
		return &graphql.Result{
			Errors: []gqlerrors.FormattedError{
				*middleware.QueryTooComplexError(complexity, h.config.MaxComplexity, nil),
			},
		}
	}

	// Execute query
	result := graphql.Do(graphql.Params{
		Schema:         h.schema,
		RequestString:  query,
		OperationName:  operationName,
		VariableValues: variables,
		Context:        ctx,
	})

	// Add trace ID to errors if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		for i := range result.Errors {
			if result.Errors[i].Extensions == nil {
				result.Errors[i].Extensions = make(map[string]interface{})
			}
			result.Errors[i].Extensions["traceId"] = traceID
		}
	}

	return result
}

// writeError writes an error response
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string, err error) {
	h.logger.WithError(err).Error(message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := GraphQLResponse{
		Errors: []interface{}{
			map[string]interface{}{
				"message": message,
				"extensions": map[string]interface{}{
					"code": middleware.ErrorCodeInternal,
				},
			},
		},
	}

	json.NewEncoder(w).Encode(response)
}

// GetSchema returns the GraphQL schema
func (h *Handler) GetSchema() graphql.Schema {
	return h.schema
}
