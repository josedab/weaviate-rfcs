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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// RequestLog captures detailed request information for debugging
type RequestLog struct {
	RequestID string              `json:"request_id"`
	Method    string              `json:"method"`
	URL       string              `json:"url"`
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      string              `json:"body,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
}

// ResponseLog captures detailed response information for debugging
type ResponseLog struct {
	RequestID  string              `json:"request_id"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
	Duration   time.Duration       `json:"duration"`
	DurationMs float64             `json:"duration_ms"`
}

// DebugLogger handles logging of requests and responses
type DebugLogger struct {
	logger      *logrus.Logger
	logRequests bool
	logBodies   bool
	maxBodySize int
}

// NewDebugLogger creates a new debug logger
func NewDebugLogger(logger *logrus.Logger, logRequests, logBodies bool, maxBodySize int) *DebugLogger {
	if maxBodySize <= 0 {
		maxBodySize = 10240 // 10KB default
	}
	return &DebugLogger{
		logger:      logger,
		logRequests: logRequests,
		logBodies:   logBodies,
		maxBodySize: maxBodySize,
	}
}

// LogRequest logs a request
func (d *DebugLogger) LogRequest(reqLog *RequestLog) {
	if !d.logRequests {
		return
	}

	fields := logrus.Fields{
		"request_id": reqLog.RequestID,
		"method":     reqLog.Method,
		"url":        reqLog.URL,
		"timestamp":  reqLog.Timestamp,
	}

	if d.logBodies && reqLog.Body != "" {
		fields["body"] = reqLog.Body
	}

	d.logger.WithFields(fields).Debug("HTTP Request")
}

// LogResponse logs a response
func (d *DebugLogger) LogResponse(respLog *ResponseLog) {
	if !d.logRequests {
		return
	}

	fields := logrus.Fields{
		"request_id":  respLog.RequestID,
		"status_code": respLog.StatusCode,
		"duration_ms": respLog.DurationMs,
	}

	if d.logBodies && respLog.Body != "" {
		fields["body"] = respLog.Body
	}

	level := logrus.InfoLevel
	if respLog.StatusCode >= 500 {
		level = logrus.ErrorLevel
	} else if respLog.StatusCode >= 400 {
		level = logrus.WarnLevel
	}

	d.logger.WithFields(fields).Log(level, "HTTP Response")
}

// ResponseCapture wraps http.ResponseWriter to capture response data
type ResponseCapture struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// NewResponseCapture creates a new response capture wrapper
func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}
}

// WriteHeader captures the status code
func (rc *ResponseCapture) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (rc *ResponseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// DebugMiddleware provides request/response debugging and logging
type DebugMiddleware struct {
	logger *DebugLogger
}

// NewDebugMiddleware creates a new debug middleware
func NewDebugMiddleware(logger *DebugLogger) *DebugMiddleware {
	return &DebugMiddleware{
		logger: logger,
	}
}

// Handler returns the HTTP handler
func (m *DebugMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate request ID
		reqID := uuid.New().String()

		// Capture request body if enabled
		var reqBody string
		if m.logger.logBodies && r.Body != nil {
			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, int64(m.logger.maxBodySize)))
			if err == nil {
				reqBody = string(bodyBytes)
				// Restore body for downstream handlers
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Log request
		reqLog := &RequestLog{
			RequestID: reqID,
			Method:    r.Method,
			URL:       r.URL.String(),
			Headers:   r.Header,
			Body:      reqBody,
			Timestamp: time.Now(),
		}
		m.logger.LogRequest(reqLog)

		// Wrap response writer
		rc := NewResponseCapture(w)

		// Add debug headers
		rc.Header().Set("X-Request-ID", reqID)

		// Execute handler
		start := time.Now()
		next.ServeHTTP(rc, r)
		duration := time.Since(start)

		// Add duration header
		rc.Header().Set("X-Duration-Ms", fmt.Sprintf("%.2f", duration.Seconds()*1000))

		// Log response
		respBody := ""
		if m.logger.logBodies && rc.body.Len() > 0 {
			bodyBytes := rc.body.Bytes()
			if len(bodyBytes) > m.logger.maxBodySize {
				bodyBytes = bodyBytes[:m.logger.maxBodySize]
			}
			respBody = string(bodyBytes)
		}

		respLog := &ResponseLog{
			RequestID:  reqID,
			StatusCode: rc.statusCode,
			Headers:    rc.Header(),
			Body:       respBody,
			Duration:   duration,
			DurationMs: duration.Seconds() * 1000,
		}
		m.logger.LogResponse(respLog)
	})
}

// DebugConfig holds configuration for debug middleware
type DebugConfig struct {
	Enabled     bool `json:"enabled" yaml:"enabled"`
	LogRequests bool `json:"log_requests" yaml:"log_requests"`
	LogBodies   bool `json:"log_bodies" yaml:"log_bodies"`
	MaxBodySize int  `json:"max_body_size" yaml:"max_body_size"`
}

// DefaultDebugConfig returns the default debug configuration
func DefaultDebugConfig() DebugConfig {
	return DebugConfig{
		Enabled:     false,
		LogRequests: true,
		LogBodies:   false,
		MaxBodySize: 10240, // 10KB
	}
}

// QueryExplainLog captures query execution plan information
type QueryExplainLog struct {
	RequestID      string                 `json:"request_id"`
	Query          string                 `json:"query"`
	ExecutionPlan  []ExecutionStep        `json:"execution_plan"`
	EstimatedCost  float64                `json:"estimated_cost_ms"`
	ActualDuration float64                `json:"actual_duration_ms"`
	Efficiency     float64                `json:"efficiency_percent"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionStep represents a single step in query execution
type ExecutionStep struct {
	Operation   string                 `json:"operation"`
	Description string                 `json:"description,omitempty"`
	Cost        float64                `json:"cost_ms"`
	Rows        int                    `json:"rows,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// FormatQueryExplain formats a query explanation as JSON
func FormatQueryExplain(explain *QueryExplainLog) (string, error) {
	data, err := json.MarshalIndent(explain, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatQueryExplainText formats a query explanation as human-readable text
func FormatQueryExplainText(explain *QueryExplainLog) string {
	var buf bytes.Buffer

	buf.WriteString("Query Plan:\n")
	for i, step := range explain.ExecutionPlan {
		buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.Operation))
		if step.Description != "" {
			buf.WriteString(fmt.Sprintf("     %s\n", step.Description))
		}
		buf.WriteString(fmt.Sprintf("     Cost: %.1fms\n", step.Cost))
		if step.Rows > 0 {
			buf.WriteString(fmt.Sprintf("     Rows: ~%d\n", step.Rows))
		}
		buf.WriteString("\n")
	}

	buf.WriteString(fmt.Sprintf("Total estimated: %.1fms\n", explain.EstimatedCost))
	buf.WriteString(fmt.Sprintf("Actual: %.1fms (%.0f%% of estimate)\n",
		explain.ActualDuration,
		(explain.ActualDuration/explain.EstimatedCost)*100))

	return buf.String()
}
