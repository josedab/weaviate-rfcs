package executor

import (
	"sync"
	"time"
)

// RuntimeStatistics collects and manages runtime execution statistics.
type RuntimeStatistics struct {
	executions     []*OperatorStats
	mu             sync.RWMutex
	totalExecutions int64
	totalTimeMs     int64
}

// NewRuntimeStatistics creates a new runtime statistics collector.
func NewRuntimeStatistics() *RuntimeStatistics {
	return &RuntimeStatistics{
		executions: make([]*OperatorStats, 0),
	}
}

// RecordExecution records statistics for a completed operator execution.
func (rs *RuntimeStatistics) RecordExecution(stats *OperatorStats) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.executions = append(rs.executions, stats)
	rs.totalExecutions++
	rs.totalTimeMs += stats.ExecutionTimeMs
}

// GetRecentExecutions returns the N most recent executions.
func (rs *RuntimeStatistics) GetRecentExecutions(n int) []*OperatorStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if n > len(rs.executions) {
		n = len(rs.executions)
	}

	start := len(rs.executions) - n
	result := make([]*OperatorStats, n)
	copy(result, rs.executions[start:])

	return result
}

// GetExecutionCount returns the total number of recorded executions.
func (rs *RuntimeStatistics) GetExecutionCount() int64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.totalExecutions
}

// GetAverageExecutionTime returns the average execution time in milliseconds.
func (rs *RuntimeStatistics) GetAverageExecutionTime() float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.totalExecutions == 0 {
		return 0.0
	}

	return float64(rs.totalTimeMs) / float64(rs.totalExecutions)
}

// GetCardinalityErrorStats returns statistics about cardinality estimation errors.
func (rs *RuntimeStatistics) GetCardinalityErrorStats() CardinalityErrorStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if len(rs.executions) == 0 {
		return CardinalityErrorStats{}
	}

	var sumError float64
	var maxError float64
	var minError float64 = 1000000.0
	count := 0

	for _, exec := range rs.executions {
		if exec.EstimatedCardinality > 0 {
			error := exec.CardinalityError()
			sumError += error
			count++

			if error > maxError {
				maxError = error
			}
			if error < minError {
				minError = error
			}
		}
	}

	avgError := 0.0
	if count > 0 {
		avgError = sumError / float64(count)
	}

	return CardinalityErrorStats{
		AverageError: avgError,
		MaxError:     maxError,
		MinError:     minError,
		SampleCount:  count,
	}
}

// Clear removes all recorded statistics.
func (rs *RuntimeStatistics) Clear() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.executions = make([]*OperatorStats, 0)
	rs.totalExecutions = 0
	rs.totalTimeMs = 0
}

// Prune removes statistics older than the specified duration.
func (rs *RuntimeStatistics) Prune(olderThan time.Duration) int {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	pruned := 0
	newExecutions := make([]*OperatorStats, 0)

	for _, exec := range rs.executions {
		if exec.EndTime.After(cutoff) {
			newExecutions = append(newExecutions, exec)
		} else {
			pruned++
			rs.totalTimeMs -= exec.ExecutionTimeMs
		}
	}

	rs.executions = newExecutions
	rs.totalExecutions = int64(len(newExecutions))

	return pruned
}

// GetOperatorTypeStats returns statistics grouped by operator type.
func (rs *RuntimeStatistics) GetOperatorTypeStats() map[string]OperatorTypeStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	stats := make(map[string]OperatorTypeStats)

	for _, exec := range rs.executions {
		opType := exec.OperatorType
		typeStats := stats[opType]

		typeStats.Count++
		typeStats.TotalTimeMs += exec.ExecutionTimeMs
		typeStats.TotalCardinality += exec.ActualCardinality

		if exec.EstimatedCardinality > 0 {
			typeStats.TotalError += exec.CardinalityError()
			typeStats.ErrorSamples++
		}

		stats[opType] = typeStats
	}

	// Calculate averages
	for opType, typeStats := range stats {
		if typeStats.Count > 0 {
			typeStats.AverageTimeMs = float64(typeStats.TotalTimeMs) / float64(typeStats.Count)
			typeStats.AverageCardinality = float64(typeStats.TotalCardinality) / float64(typeStats.Count)
		}
		if typeStats.ErrorSamples > 0 {
			typeStats.AverageError = typeStats.TotalError / float64(typeStats.ErrorSamples)
		}
		stats[opType] = typeStats
	}

	return stats
}

// CardinalityErrorStats contains statistics about cardinality estimation errors.
type CardinalityErrorStats struct {
	AverageError float64 // Average error ratio
	MaxError     float64 // Maximum error ratio
	MinError     float64 // Minimum error ratio
	SampleCount  int     // Number of samples
}

// OperatorTypeStats contains statistics for a specific operator type.
type OperatorTypeStats struct {
	Count              int     // Number of executions
	TotalTimeMs        int64   // Total execution time
	AverageTimeMs      float64 // Average execution time
	TotalCardinality   int64   // Total output cardinality
	AverageCardinality float64 // Average output cardinality
	TotalError         float64 // Sum of cardinality errors
	AverageError       float64 // Average cardinality error
	ErrorSamples       int     // Number of error samples
}
