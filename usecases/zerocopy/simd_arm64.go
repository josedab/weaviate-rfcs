//go:build arm64
// +build arm64

package zerocopy

import (
	"math"
)

// dotProductNEON computes dot product using ARM NEON instructions
// This is a simplified implementation; production would use assembly
func dotProductNEON(a, b []float32) float32 {
	var sum float32
	n := len(a)

	// Process 4 floats at a time (NEON can handle 4x float32 in 128-bit register)
	i := 0
	for ; i+3 < n; i += 4 {
		// In a real implementation, this would use NEON intrinsics
		// For now, we unroll the loop manually
		sum += a[i] * b[i]
		sum += a[i+1] * b[i+1]
		sum += a[i+2] * b[i+2]
		sum += a[i+3] * b[i+3]
	}

	// Handle remaining elements
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// l2DistanceNEON computes L2 distance using ARM NEON instructions
func l2DistanceNEON(a, b []float32) float32 {
	var sum float32
	n := len(a)

	// Process 4 floats at a time
	i := 0
	for ; i+3 < n; i += 4 {
		// Unrolled loop
		diff0 := a[i] - b[i]
		diff1 := a[i+1] - b[i+1]
		diff2 := a[i+2] - b[i+2]
		diff3 := a[i+3] - b[i+3]

		sum += diff0 * diff0
		sum += diff1 * diff1
		sum += diff2 * diff2
		sum += diff3 * diff3
	}

	// Handle remaining elements
	for ; i < n; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum)))
}
