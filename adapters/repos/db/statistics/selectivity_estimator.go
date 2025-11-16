package statistics

import (
	"math"
)

// SelectivityEstimator estimates the selectivity of query predicates
// using statistics and histograms.
type SelectivityEstimator struct {
	statistics *StatisticsStore
}

// NewSelectivityEstimator creates a new selectivity estimator.
func NewSelectivityEstimator(statistics *StatisticsStore) *SelectivityEstimator {
	return &SelectivityEstimator{
		statistics: statistics,
	}
}

// EstimateFilterSelectivity estimates the selectivity of a filter predicate.
// Returns a value between 0.0 and 1.0 representing the fraction of rows that match.
func (se *SelectivityEstimator) EstimateFilterSelectivity(
	tableName, columnName string,
	op FilterOperator,
	value interface{},
) float64 {
	// Get column statistics
	colStats := se.statistics.GetColumnStats(tableName, columnName)
	if colStats == nil {
		// No statistics available, use default estimates
		return se.defaultSelectivity(op)
	}

	return colStats.EstimateSelectivity(op, value)
}

// EstimateConjunctionSelectivity estimates the selectivity of multiple filters combined with AND.
// Uses independence assumption: P(A AND B) = P(A) * P(B)
func (se *SelectivityEstimator) EstimateConjunctionSelectivity(selectivities []float64) float64 {
	if len(selectivities) == 0 {
		return 1.0
	}

	result := 1.0
	for _, sel := range selectivities {
		result *= sel
	}

	return result
}

// EstimateDisjunctionSelectivity estimates the selectivity of multiple filters combined with OR.
// Uses inclusion-exclusion principle: P(A OR B) = P(A) + P(B) - P(A AND B)
func (se *SelectivityEstimator) EstimateDisjunctionSelectivity(selectivities []float64) float64 {
	if len(selectivities) == 0 {
		return 0.0
	}

	if len(selectivities) == 1 {
		return selectivities[0]
	}

	// For multiple predicates, use approximation: P(A OR B OR C) ≈ 1 - (1-A)(1-B)(1-C)
	result := 1.0
	for _, sel := range selectivities {
		result *= (1.0 - sel)
	}

	return 1.0 - result
}

// EstimateJoinSelectivity estimates the selectivity of a join operation.
func (se *SelectivityEstimator) EstimateJoinSelectivity(
	leftTable, leftColumn string,
	rightTable, rightColumn string,
) float64 {
	leftStats := se.statistics.GetColumnStats(leftTable, leftColumn)
	rightStats := se.statistics.GetColumnStats(rightTable, rightColumn)

	// If no statistics, use default
	if leftStats == nil || rightStats == nil {
		return 0.1
	}

	// Join selectivity estimate: 1 / max(NDV_left, NDV_right)
	// This assumes foreign key relationship
	maxNDV := int64(math.Max(float64(leftStats.NDV), float64(rightStats.NDV)))
	if maxNDV == 0 {
		return 0.1
	}

	return 1.0 / float64(maxNDV)
}

// EstimateCardinality estimates the output cardinality of an operation.
func (se *SelectivityEstimator) EstimateCardinality(
	tableName string,
	selectivity float64,
) int64 {
	tableStats := se.statistics.GetTableStats(tableName)
	if tableStats == nil {
		return 1000 // Default estimate
	}

	return int64(float64(tableStats.Tuples) * selectivity)
}

// defaultSelectivity returns default selectivity estimates when no statistics are available.
func (se *SelectivityEstimator) defaultSelectivity(op FilterOperator) float64 {
	switch op {
	case OpEqual:
		return 0.1
	case OpNotEqual:
		return 0.9
	case OpLess, OpLessOrEqual, OpGreater, OpGreaterOrEqual:
		return 0.33
	case OpBetween:
		return 0.1
	case OpIn:
		return 0.1
	case OpIsNull:
		return 0.01
	case OpIsNotNull:
		return 0.99
	default:
		return 0.1
	}
}
