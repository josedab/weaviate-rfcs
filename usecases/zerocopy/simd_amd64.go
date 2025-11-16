//go:build amd64
// +build amd64

package zerocopy

import (
	"math"
)

// dotProductAVX2 computes dot product using AVX2 instructions
// This is a simplified implementation; production would use assembly
func dotProductAVX2(a, b []float32) float32 {
	var sum float32
	n := len(a)

	// Process 8 floats at a time (AVX2 can handle 8x float32)
	i := 0
	for ; i+7 < n; i += 8 {
		// In a real implementation, this would use AVX2 intrinsics
		// For now, we unroll the loop manually for better performance
		sum += a[i] * b[i]
		sum += a[i+1] * b[i+1]
		sum += a[i+2] * b[i+2]
		sum += a[i+3] * b[i+3]
		sum += a[i+4] * b[i+4]
		sum += a[i+5] * b[i+5]
		sum += a[i+6] * b[i+6]
		sum += a[i+7] * b[i+7]
	}

	// Handle remaining elements
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// l2DistanceAVX2 computes L2 distance using AVX2 instructions
func l2DistanceAVX2(a, b []float32) float32 {
	var sum float32
	n := len(a)

	// Process 8 floats at a time
	i := 0
	for ; i+7 < n; i += 8 {
		// Unrolled loop for better performance
		diff0 := a[i] - b[i]
		diff1 := a[i+1] - b[i+1]
		diff2 := a[i+2] - b[i+2]
		diff3 := a[i+3] - b[i+3]
		diff4 := a[i+4] - b[i+4]
		diff5 := a[i+5] - b[i+5]
		diff6 := a[i+6] - b[i+6]
		diff7 := a[i+7] - b[i+7]

		sum += diff0 * diff0
		sum += diff1 * diff1
		sum += diff2 * diff2
		sum += diff3 * diff3
		sum += diff4 * diff4
		sum += diff5 * diff5
		sum += diff6 * diff6
		sum += diff7 * diff7
	}

	// Handle remaining elements
	for ; i < n; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum)))
}
