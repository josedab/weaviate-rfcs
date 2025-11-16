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

package middleware

import (
	"fmt"
	"time"

	"github.com/tailor-inc/graphql/gqlerrors"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	ErrorCodeInternal          ErrorCode = "INTERNAL_ERROR"
	ErrorCodeInvalidArgument   ErrorCode = "INVALID_ARGUMENT"
	ErrorCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrorCodePermissionDenied  ErrorCode = "PERMISSION_DENIED"
	ErrorCodeUnauthenticated   ErrorCode = "UNAUTHENTICATED"
	ErrorCodeQueryTooComplex   ErrorCode = "QUERY_TOO_COMPLEX"
	ErrorCodeVectorDimension   ErrorCode = "VECTOR_DIMENSION_ERROR"
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"
	ErrorCodeRateLimit         ErrorCode = "RATE_LIMIT_EXCEEDED"
)

// ErrorExtensions contains additional error metadata
type ErrorExtensions struct {
	Code      ErrorCode              `json:"code"`
	Timestamp string                 `json:"timestamp"`
	TraceID   string                 `json:"traceId,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// NewGraphQLError creates a standardized GraphQL error
func NewGraphQLError(message string, code ErrorCode, path []interface{}, details map[string]interface{}) *gqlerrors.FormattedError {
	extensions := ErrorExtensions{
		Code:      code,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Details:   details,
	}

	return &gqlerrors.FormattedError{
		Message: message,
		Path:    path,
		Extensions: map[string]interface{}{
			"code":      extensions.Code,
			"timestamp": extensions.Timestamp,
			"details":   extensions.Details,
		},
	}
}

// WrapError wraps a Go error into a standardized GraphQL error
func WrapError(err error, code ErrorCode, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(err.Error(), code, path, nil)
}

// VectorDimensionError creates a vector dimension mismatch error
func VectorDimensionError(expected, received int, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		"Vector dimension mismatch",
		ErrorCodeVectorDimension,
		path,
		map[string]interface{}{
			"expected": expected,
			"received": received,
		},
	)
}

// NotFoundError creates a not found error
func NotFoundError(resourceType, id string, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		fmt.Sprintf("%s with id '%s' not found", resourceType, id),
		ErrorCodeNotFound,
		path,
		map[string]interface{}{
			"resourceType": resourceType,
			"id":           id,
		},
	)
}

// InvalidArgumentError creates an invalid argument error
func InvalidArgumentError(argument, reason string, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		fmt.Sprintf("Invalid argument '%s': %s", argument, reason),
		ErrorCodeInvalidArgument,
		path,
		map[string]interface{}{
			"argument": argument,
			"reason":   reason,
		},
	)
}

// QueryTooComplexError creates a query complexity error
func QueryTooComplexError(complexity, maxComplexity int, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		fmt.Sprintf("Query complexity %d exceeds maximum allowed %d", complexity, maxComplexity),
		ErrorCodeQueryTooComplex,
		path,
		map[string]interface{}{
			"complexity":    complexity,
			"maxComplexity": maxComplexity,
		},
	)
}

// TimeoutError creates a timeout error
func TimeoutError(operation string, duration time.Duration, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		fmt.Sprintf("Operation '%s' timed out after %v", operation, duration),
		ErrorCodeTimeout,
		path,
		map[string]interface{}{
			"operation": operation,
			"duration":  duration.String(),
		},
	)
}

// RateLimitError creates a rate limit error
func RateLimitError(limit int, window time.Duration, path []interface{}) *gqlerrors.FormattedError {
	return NewGraphQLError(
		fmt.Sprintf("Rate limit exceeded: %d requests per %v", limit, window),
		ErrorCodeRateLimit,
		path,
		map[string]interface{}{
			"limit":  limit,
			"window": window.String(),
		},
	)
}
