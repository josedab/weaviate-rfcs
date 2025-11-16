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

package errors

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// TraceFrame represents a single frame in a stack trace
type TraceFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// EnhancedError provides detailed context and helpful suggestions for errors
// This implements the error interface and provides structured error information
// as described in RFC 0015 for improved developer experience
type EnhancedError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
	DocsLink   string                 `json:"docs,omitempty"`
	Trace      []TraceFrame           `json:"trace,omitempty"`
	Err        error                  `json:"-"` // Original error if wrapping
}

// Error implements the error interface
func (e *EnhancedError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (wrapped: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error for errors.Is and errors.As
func (e *EnhancedError) Unwrap() error {
	return e.Err
}

// MarshalJSON provides custom JSON serialization
func (e *EnhancedError) MarshalJSON() ([]byte, error) {
	type Alias EnhancedError
	return json.Marshal(&struct {
		*Alias
		WrappedError string `json:"wrapped_error,omitempty"`
	}{
		Alias:        (*Alias)(e),
		WrappedError: errorToString(e.Err),
	})
}

func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// NewEnhancedError creates a new EnhancedError with the given parameters
func NewEnhancedError(code, message string) *EnhancedError {
	return &EnhancedError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Trace:   captureTrace(3), // Skip 3 frames: runtime.Callers, captureTrace, NewEnhancedError
	}
}

// WithDetails adds contextual details to the error
func (e *EnhancedError) WithDetails(details map[string]interface{}) *EnhancedError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithDetail adds a single detail to the error
func (e *EnhancedError) WithDetail(key string, value interface{}) *EnhancedError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithSuggestion adds a helpful suggestion to the error
func (e *EnhancedError) WithSuggestion(suggestion string) *EnhancedError {
	e.Suggestion = suggestion
	return e
}

// WithDocsLink adds a documentation link to the error
func (e *EnhancedError) WithDocsLink(link string) *EnhancedError {
	e.DocsLink = link
	return e
}

// WithWrapped wraps an existing error
func (e *EnhancedError) WithWrapped(err error) *EnhancedError {
	e.Err = err
	return e
}

// captureTrace captures the current stack trace
func captureTrace(skip int) []TraceFrame {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip, pcs[:])

	frames := make([]TraceFrame, 0, n)
	for i := 0; i < n; i++ {
		pc := pcs[i]
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		file, line := fn.FileLine(pc)

		// Clean up file path for readability
		if idx := strings.LastIndex(file, "/weaviate/"); idx >= 0 {
			file = file[idx+1:]
		}

		frames = append(frames, TraceFrame{
			Function: fn.Name(),
			File:     file,
			Line:     line,
		})
	}

	return frames
}

// Common error constructors for typical scenarios

// NewValidationError creates an error for validation failures
func NewValidationError(field string, expected, got interface{}) *EnhancedError {
	return NewEnhancedError("VALIDATION_ERROR", fmt.Sprintf("Invalid value for field '%s'", field)).
		WithDetails(map[string]interface{}{
			"field":    field,
			"expected": expected,
			"got":      got,
		}).
		WithSuggestion(fmt.Sprintf("Expected %T but got %T. Convert the value or check schema definition.", expected, got)).
		WithDocsLink("https://weaviate.io/docs/errors/validation")
}

// NewVectorDimensionMismatchError creates an error for vector dimension mismatches
func NewVectorDimensionMismatchError(className string, expectedDim, gotDim int, vectorizer string) *EnhancedError {
	return NewEnhancedError("VECTOR_DIMENSION_MISMATCH", "Vector dimension does not match schema").
		WithDetails(map[string]interface{}{
			"class":               className,
			"expected_dimensions": expectedDim,
			"got_dimensions":      gotDim,
			"vectorizer":          vectorizer,
		}).
		WithSuggestion(fmt.Sprintf(
			"Check that you're using the correct model. The schema expects %d dimensions but the vector has %d dimensions.",
			expectedDim, gotDim,
		)).
		WithDocsLink("https://weaviate.io/docs/errors/vector-dimension-mismatch")
}

// NewSchemaNotFoundError creates an error when a schema class is not found
func NewSchemaNotFoundError(className string) *EnhancedError {
	return NewEnhancedError("SCHEMA_NOT_FOUND", fmt.Sprintf("Class '%s' not found in schema", className)).
		WithDetails(map[string]interface{}{
			"class": className,
		}).
		WithSuggestion("Ensure the class name is spelled correctly and that the schema has been created.").
		WithDocsLink("https://weaviate.io/docs/errors/schema-not-found")
}

// NewPropertyNotFoundError creates an error when a property is not found
func NewPropertyNotFoundError(className, propertyName string) *EnhancedError {
	return NewEnhancedError("PROPERTY_NOT_FOUND", fmt.Sprintf("Property '%s' not found in class '%s'", propertyName, className)).
		WithDetails(map[string]interface{}{
			"class":    className,
			"property": propertyName,
		}).
		WithSuggestion("Check the property name spelling and ensure it exists in the class schema.").
		WithDocsLink("https://weaviate.io/docs/errors/property-not-found")
}

// NewInsufficientResourcesError creates an error for resource limitations
func NewInsufficientResourcesError(resource string, required, available interface{}) *EnhancedError {
	return NewEnhancedError("INSUFFICIENT_RESOURCES", fmt.Sprintf("Insufficient %s to complete operation", resource)).
		WithDetails(map[string]interface{}{
			"resource":  resource,
			"required":  required,
			"available": available,
		}).
		WithSuggestion(fmt.Sprintf("Increase available %s or reduce the size of your operation.", resource)).
		WithDocsLink("https://weaviate.io/docs/errors/insufficient-resources")
}

// NewAuthenticationError creates an error for authentication failures
func NewAuthenticationError(reason string) *EnhancedError {
	return NewEnhancedError("AUTHENTICATION_ERROR", "Authentication failed").
		WithDetail("reason", reason).
		WithSuggestion("Check your API key or authentication credentials.").
		WithDocsLink("https://weaviate.io/docs/configuration/authentication")
}

// NewAuthorizationError creates an error for authorization failures
func NewAuthorizationError(action, resource string) *EnhancedError {
	return NewEnhancedError("AUTHORIZATION_ERROR", fmt.Sprintf("Not authorized to perform '%s' on '%s'", action, resource)).
		WithDetails(map[string]interface{}{
			"action":   action,
			"resource": resource,
		}).
		WithSuggestion("Check your permissions and role assignments.").
		WithDocsLink("https://weaviate.io/docs/configuration/authorization")
}

// NewTimeoutError creates an error for timeout scenarios
func NewTimeoutError(operation string, timeout interface{}) *EnhancedError {
	return NewEnhancedError("TIMEOUT_ERROR", fmt.Sprintf("Operation '%s' timed out", operation)).
		WithDetails(map[string]interface{}{
			"operation": operation,
			"timeout":   timeout,
		}).
		WithSuggestion("Increase the timeout value or optimize the operation to complete faster.").
		WithDocsLink("https://weaviate.io/docs/errors/timeout")
}

// NewConnectionError creates an error for connection failures
func NewConnectionError(target string, err error) *EnhancedError {
	return NewEnhancedError("CONNECTION_ERROR", fmt.Sprintf("Failed to connect to %s", target)).
		WithDetail("target", target).
		WithWrapped(err).
		WithSuggestion("Check that the target service is running and network connectivity is available.").
		WithDocsLink("https://weaviate.io/docs/errors/connection")
}
