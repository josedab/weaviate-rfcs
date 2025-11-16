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

package hnsw

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FilterQueryLog represents a logged filter query with ground truth data
// This data is used for training the ML model
type FilterQueryLog struct {
	Timestamp time.Time `json:"timestamp"`

	// Features
	Features *FilterQueryFeatures `json:"features"`

	// Predictions (from model)
	PredictedSelectivity float64 `json:"predicted_selectivity"`
	PredictedStrategy    string  `json:"predicted_strategy"`
	ConfidenceScore      float64 `json:"confidence_score"`

	// Ground truth (from execution)
	ActualSelectivity float64       `json:"actual_selectivity"`
	ActualLatency     time.Duration `json:"actual_latency"`
	OptimalStrategy   string        `json:"optimal_strategy"`

	// Additional metadata
	FilteredCount int `json:"filtered_count"` // Number of records after filter
	TotalCount    int `json:"total_count"`    // Total records in corpus
}

// FilterQueryLogger manages logging of filter queries for ML training
type FilterQueryLogger struct {
	enabled    bool
	logFile    string
	mu         sync.Mutex
	writer     *os.File
	logCount   int
	maxLogs    int // Maximum logs before rotation
	rotateSize int64
}

// NewFilterQueryLogger creates a new logger for filter queries
func NewFilterQueryLogger(logFile string, enabled bool) (*FilterQueryLogger, error) {
	logger := &FilterQueryLogger{
		enabled:    enabled,
		logFile:    logFile,
		maxLogs:    100000, // Rotate after 100k logs
		rotateSize: 100 * 1024 * 1024, // 100MB
	}

	if enabled && logFile != "" {
		if err := logger.openLogFile(); err != nil {
			return nil, err
		}
	}

	return logger, nil
}

func (l *FilterQueryLogger) openLogFile() error {
	var err error
	l.writer, err = os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	return nil
}

// LogQuery logs a filter query with its features and ground truth
func (l *FilterQueryLogger) LogQuery(log *FilterQueryLog) error {
	if !l.enabled || l.writer == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if we need to rotate the log file
	if l.logCount >= l.maxLogs {
		if err := l.rotateLogFile(); err != nil {
			return err
		}
	}

	// Marshal to JSON
	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	// Write to file with newline
	if _, err := l.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	l.logCount++
	return nil
}

func (l *FilterQueryLogger) rotateLogFile() error {
	// Close current file
	if l.writer != nil {
		l.writer.Close()
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedFile := fmt.Sprintf("%s.%s", l.logFile, timestamp)
	if err := os.Rename(l.logFile, rotatedFile); err != nil {
		// If rename fails, just continue with new file
		// This might happen if the file doesn't exist yet
	}

	// Open new file
	l.logCount = 0
	return l.openLogFile()
}

// Close closes the log file
func (l *FilterQueryLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// IsEnabled returns whether logging is enabled
func (l *FilterQueryLogger) IsEnabled() bool {
	return l.enabled
}

// CalculateOptimalStrategy determines which strategy would have been optimal
// based on actual execution results
func CalculateOptimalStrategy(
	actualSelectivity float64,
	preFilterLatency time.Duration,
	postFilterLatency time.Duration,
) string {
	// If we measured both, return the faster one
	if preFilterLatency > 0 && postFilterLatency > 0 {
		if preFilterLatency < postFilterLatency {
			return "pre_filter"
		}
		return "post_filter"
	}

	// Otherwise, use heuristic based on selectivity
	// Low selectivity (< 10%) typically benefits from pre-filtering
	if actualSelectivity < 0.1 {
		return "pre_filter"
	}
	return "post_filter"
}
