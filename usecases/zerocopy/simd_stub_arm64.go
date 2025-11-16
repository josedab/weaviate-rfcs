//go:build arm64
// +build arm64

package zerocopy

// Stub implementations for AMD64 functions on ARM64

func dotProductAVX2(a, b []float32) float32 {
	return dotProductScalar(a, b)
}

func l2DistanceAVX2(a, b []float32) float32 {
	return l2DistanceScalar(a, b)
}
