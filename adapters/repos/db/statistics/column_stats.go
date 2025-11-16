package statistics

// ColumnStats contains statistical information about a single column.
type ColumnStats struct {
	NDV      int64        // Number of distinct values
	NullFrac float64      // Fraction of null values (0.0 to 1.0)
	AvgWidth int          // Average column width in bytes
	Histogram *Histogram  // Value distribution histogram
	MCV      []MostCommon // Most common values
}

// MostCommon represents a frequently occurring value and its frequency.
type MostCommon struct {
	Value     interface{} // The value itself
	Frequency float64     // Fraction of rows with this value (0.0 to 1.0)
}

// NewColumnStats creates a new ColumnStats instance with default values.
func NewColumnStats() *ColumnStats {
	return &ColumnStats{
		NDV:       0,
		NullFrac:  0.0,
		AvgWidth:  0,
		Histogram: NewHistogram(),
		MCV:       make([]MostCommon, 0),
	}
}

// EstimateSelectivity estimates the selectivity of a predicate on this column.
// Selectivity is the fraction of rows that satisfy the predicate (0.0 to 1.0).
func (cs *ColumnStats) EstimateSelectivity(op FilterOperator, value interface{}) float64 {
	// Handle null checks
	if op == OpIsNull {
		return cs.NullFrac
	}
	if op == OpIsNotNull {
		return 1.0 - cs.NullFrac
	}

	// Check most common values first
	if cs.MCV != nil {
		for _, mcv := range cs.MCV {
			if op == OpEqual && mcv.Value == value {
				return mcv.Frequency
			}
		}
	}

	// Use histogram if available
	if cs.Histogram != nil {
		return cs.Histogram.EstimateSelectivity(op, value)
	}

	// Default selectivity estimates when no statistics available
	switch op {
	case OpEqual:
		if cs.NDV > 0 {
			return 1.0 / float64(cs.NDV)
		}
		return 0.1 // Default assumption
	case OpNotEqual:
		if cs.NDV > 0 {
			return 1.0 - (1.0 / float64(cs.NDV))
		}
		return 0.9
	case OpLess, OpLessOrEqual, OpGreater, OpGreaterOrEqual:
		return 0.33 // Default range selectivity
	case OpBetween:
		return 0.1 // Default range selectivity
	case OpIn:
		// Estimate based on number of values in IN clause
		if cs.NDV > 0 {
			// Assume average case
			return 0.1
		}
		return 0.1
	default:
		return 0.1 // Conservative default
	}
}

// FilterOperator represents different filter operation types.
type FilterOperator int

const (
	OpEqual FilterOperator = iota
	OpNotEqual
	OpLess
	OpLessOrEqual
	OpGreater
	OpGreaterOrEqual
	OpBetween
	OpIn
	OpIsNull
	OpIsNotNull
)
