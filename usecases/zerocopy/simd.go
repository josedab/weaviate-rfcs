package zerocopy

import (
	"math"

	"golang.org/x/sys/cpu"
)

// DotProduct computes the dot product of two float32 vectors
// Automatically selects the best implementation based on CPU features
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("vector length mismatch")
	}

	// Select implementation based on CPU features
	if cpu.X86.HasAVX2 {
		return dotProductAVX2(a, b)
	} else if cpu.ARM64.HasASIMD {
		return dotProductNEON(a, b)
	}

	return dotProductScalar(a, b)
}

// L2Distance computes the L2 (Euclidean) distance between two vectors
func L2Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		panic("vector length mismatch")
	}

	if cpu.X86.HasAVX2 {
		return l2DistanceAVX2(a, b)
	} else if cpu.ARM64.HasASIMD {
		return l2DistanceNEON(a, b)
	}

	return l2DistanceScalar(a, b)
}

// dotProductScalar is the scalar implementation
func dotProductScalar(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// l2DistanceScalar is the scalar implementation for L2 distance
func l2DistanceScalar(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

// Prefetch hints for cache optimization
// These are no-ops in pure Go but document intent for assembly implementations
func prefetchT0(addr uintptr) {
	// Hardware prefetch into all cache levels
	// In assembly: PREFETCHT0
}

func prefetchT1(addr uintptr) {
	// Hardware prefetch into L2 and L3 cache
	// In assembly: PREFETCHT1
}

func prefetchT2(addr uintptr) {
	// Hardware prefetch into L3 cache
	// In assembly: PREFETCHT2
}

// PrefetchVector prefetches a vector for reading
func PrefetchVector(vector []float32) {
	if len(vector) == 0 {
		return
	}
	// Prefetch first cache line
	prefetchT0(uintptr(len(vector)))
}
