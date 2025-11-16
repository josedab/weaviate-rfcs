package statistics

import (
	"testing"
)

func TestNewHistogram(t *testing.T) {
	hist := NewHistogram()
	if hist == nil {
		t.Fatal("NewHistogram returned nil")
	}

	if len(hist.Buckets) != 0 {
		t.Errorf("Expected empty histogram, got %d buckets", len(hist.Buckets))
	}
}

func TestHistogram_AddBucket(t *testing.T) {
	hist := NewHistogram()

	bucket := HistogramBucket{
		LowerBound:    1,
		UpperBound:    10,
		Count:         100,
		DistinctCount: 10,
	}

	hist.AddBucket(bucket)

	if len(hist.Buckets) != 1 {
		t.Fatalf("Expected 1 bucket, got %d", len(hist.Buckets))
	}

	if hist.Buckets[0].Count != 100 {
		t.Errorf("Expected count=100, got %d", hist.Buckets[0].Count)
	}
}

func TestHistogram_TotalCount(t *testing.T) {
	hist := NewHistogram()

	hist.AddBucket(HistogramBucket{Count: 100})
	hist.AddBucket(HistogramBucket{Count: 200})
	hist.AddBucket(HistogramBucket{Count: 300})

	total := hist.TotalCount()
	if total != 600 {
		t.Errorf("Expected total=600, got %d", total)
	}
}

func TestHistogram_EstimateEquality(t *testing.T) {
	hist := NewHistogram()

	hist.AddBucket(HistogramBucket{
		LowerBound:    1,
		UpperBound:    10,
		Count:         100,
		DistinctCount: 10,
	})

	// Value within bucket
	selectivity := hist.EstimateSelectivity(OpEqual, 5)

	// Should be approximately 1/10 = 0.1
	if selectivity < 0.05 || selectivity > 0.15 {
		t.Errorf("Expected selectivity ~0.1, got %f", selectivity)
	}
}

func TestHistogram_EstimateRange(t *testing.T) {
	hist := NewHistogram()

	// Create buckets: 1-10 (100 rows), 11-20 (200 rows), 21-30 (300 rows)
	hist.AddBucket(HistogramBucket{
		LowerBound: 1,
		UpperBound: 10,
		Count:      100,
	})
	hist.AddBucket(HistogramBucket{
		LowerBound: 11,
		UpperBound: 20,
		Count:      200,
	})
	hist.AddBucket(HistogramBucket{
		LowerBound: 21,
		UpperBound: 30,
		Count:      300,
	})

	// Test less than predicate
	selectivity := hist.EstimateSelectivity(OpLess, 15)

	// Should select first bucket (100) + part of second bucket
	// Total = 600, selected should be > 100/600 = 0.166
	if selectivity < 0.1 || selectivity > 1.0 {
		t.Errorf("Expected selectivity between 0.1 and 1.0, got %f", selectivity)
	}
}

func TestHistogram_FindBucket(t *testing.T) {
	hist := NewHistogram()

	hist.AddBucket(HistogramBucket{
		LowerBound: 1,
		UpperBound: 10,
		Count:      100,
	})
	hist.AddBucket(HistogramBucket{
		LowerBound: 11,
		UpperBound: 20,
		Count:      200,
	})

	// Value in first bucket
	bucket := hist.findBucket(5)
	if bucket == nil {
		t.Fatal("Expected to find bucket for value 5")
	}
	if bucket.Count != 100 {
		t.Errorf("Found wrong bucket, expected count=100, got %d", bucket.Count)
	}

	// Value in second bucket
	bucket = hist.findBucket(15)
	if bucket == nil {
		t.Fatal("Expected to find bucket for value 15")
	}
	if bucket.Count != 200 {
		t.Errorf("Found wrong bucket, expected count=200, got %d", bucket.Count)
	}

	// Value not in any bucket
	bucket = hist.findBucket(100)
	if bucket != nil {
		t.Error("Expected nil for value outside buckets")
	}
}

func TestHistogram_CompareValues(t *testing.T) {
	hist := NewHistogram()

	tests := []struct {
		a        interface{}
		b        interface{}
		expected int
	}{
		{5, 10, -1},
		{10, 5, 1},
		{5, 5, 0},
		{int64(5), int64(10), -1},
		{3.14, 2.71, 1},
		{"apple", "banana", -1},
		{"banana", "apple", 1},
		{"same", "same", 0},
	}

	for _, tt := range tests {
		result := hist.compareValues(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("compareValues(%v, %v) = %d, expected %d",
				tt.a, tt.b, result, tt.expected)
		}
	}
}
