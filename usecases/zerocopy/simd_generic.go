//go:build !amd64 && !arm64
// +build !amd64,!arm64

package zerocopy

// Fallback implementations for platforms without SIMD support

func dotProductAVX2(a, b []float32) float32 {
	return dotProductScalar(a, b)
}

func dotProductNEON(a, b []float32) float32 {
	return dotProductScalar(a, b)
}

func l2DistanceAVX2(a, b []float32) float32 {
	return l2DistanceScalar(a, b)
}

func l2DistanceNEON(a, b []float32) float32 {
	return l2DistanceScalar(a, b)
}
