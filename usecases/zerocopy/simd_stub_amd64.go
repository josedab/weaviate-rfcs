//go:build amd64
// +build amd64

package zerocopy

// Stub implementations for ARM64 functions on AMD64

func dotProductNEON(a, b []float32) float32 {
	return dotProductScalar(a, b)
}

func l2DistanceNEON(a, b []float32) float32 {
	return l2DistanceScalar(a, b)
}
