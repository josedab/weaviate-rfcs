package statistics

import (
	"fmt"
)

// Histogram represents the value distribution of a column using buckets.
// Each bucket covers a range of values and tracks counts and distinct values.
type Histogram struct {
	Buckets []HistogramBucket
}

// HistogramBucket represents a single bucket in a histogram.
type HistogramBucket struct {
	LowerBound    interface{} // Lower bound of the bucket (inclusive)
	UpperBound    interface{} // Upper bound of the bucket (inclusive)
	Count         int64       // Number of rows in this bucket
	DistinctCount int64       // Number of distinct values in this bucket
}

// NewHistogram creates a new empty histogram.
func NewHistogram() *Histogram {
	return &Histogram{
		Buckets: make([]HistogramBucket, 0),
	}
}

// EstimateSelectivity estimates the selectivity of a filter operation using the histogram.
func (h *Histogram) EstimateSelectivity(op FilterOperator, value interface{}) float64 {
	if len(h.Buckets) == 0 {
		return 0.1 // Default selectivity when no histogram
	}

	switch op {
	case OpEqual:
		return h.estimateEquality(value)
	case OpLess:
		return h.estimateRange(nil, value, false)
	case OpLessOrEqual:
		return h.estimateRange(nil, value, true)
	case OpGreater:
		return h.estimateRange(value, nil, false)
	case OpGreaterOrEqual:
		return h.estimateRange(value, nil, true)
	case OpBetween:
		// For BETWEEN, value should be a range struct
		if r, ok := value.(Range); ok {
			return h.estimateRange(r.Lower, r.Upper, true)
		}
		return 0.1
	default:
		return 0.1
	}
}

// estimateEquality estimates selectivity for equality predicates.
func (h *Histogram) estimateEquality(value interface{}) float64 {
	bucket := h.findBucket(value)
	if bucket == nil {
		return 0.0 // Value not in any bucket
	}

	if bucket.DistinctCount == 0 {
		return 0.0
	}

	// Assume uniform distribution within bucket
	return 1.0 / float64(bucket.DistinctCount)
}

// estimateRange estimates selectivity for range predicates.
func (h *Histogram) estimateRange(lower, upper interface{}, inclusive bool) float64 {
	totalCount := int64(0)
	selectedCount := int64(0)

	for _, bucket := range h.Buckets {
		totalCount += bucket.Count

		if h.bucketInRange(bucket, lower, upper, inclusive) {
			selectedCount += bucket.Count
		}
	}

	if totalCount == 0 {
		return 0.0
	}

	return float64(selectedCount) / float64(totalCount)
}

// bucketInRange checks if a bucket overlaps with the given range.
func (h *Histogram) bucketInRange(bucket HistogramBucket, lower, upper interface{}, inclusive bool) bool {
	// If no lower bound specified, check upper bound only
	if lower == nil && upper != nil {
		return h.compareValues(bucket.LowerBound, upper) <= 0
	}

	// If no upper bound specified, check lower bound only
	if upper == nil && lower != nil {
		return h.compareValues(bucket.UpperBound, lower) >= 0
	}

	// Both bounds specified
	if lower != nil && upper != nil {
		return h.compareValues(bucket.UpperBound, lower) >= 0 &&
			h.compareValues(bucket.LowerBound, upper) <= 0
	}

	// No bounds specified - include all
	return true
}

// findBucket finds the bucket containing the given value.
func (h *Histogram) findBucket(value interface{}) *HistogramBucket {
	for i := range h.Buckets {
		bucket := &h.Buckets[i]
		if h.compareValues(value, bucket.LowerBound) >= 0 &&
			h.compareValues(value, bucket.UpperBound) <= 0 {
			return bucket
		}
	}
	return nil
}

// compareValues compares two values for ordering.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func (h *Histogram) compareValues(a, b interface{}) int {
	// Type-specific comparison logic
	switch aVal := a.(type) {
	case int:
		if bVal, ok := b.(int); ok {
			if aVal < bVal {
				return -1
			} else if aVal > bVal {
				return 1
			}
			return 0
		}
	case int64:
		if bVal, ok := b.(int64); ok {
			if aVal < bVal {
				return -1
			} else if aVal > bVal {
				return 1
			}
			return 0
		}
	case float64:
		if bVal, ok := b.(float64); ok {
			if aVal < bVal {
				return -1
			} else if aVal > bVal {
				return 1
			}
			return 0
		}
	case string:
		if bVal, ok := b.(string); ok {
			if aVal < bVal {
				return -1
			} else if aVal > bVal {
				return 1
			}
			return 0
		}
	}

	// Default: cannot compare
	return 0
}

// AddBucket adds a new bucket to the histogram.
func (h *Histogram) AddBucket(bucket HistogramBucket) {
	h.Buckets = append(h.Buckets, bucket)
}

// TotalCount returns the total count across all buckets.
func (h *Histogram) TotalCount() int64 {
	total := int64(0)
	for _, bucket := range h.Buckets {
		total += bucket.Count
	}
	return total
}

// Range represents a value range for BETWEEN operations.
type Range struct {
	Lower interface{}
	Upper interface{}
}

// String returns a string representation of the histogram for debugging.
func (h *Histogram) String() string {
	return fmt.Sprintf("Histogram{buckets: %d, total: %d}", len(h.Buckets), h.TotalCount())
}
