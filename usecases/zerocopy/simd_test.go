package zerocopy

import (
	"math"
	"testing"
)

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "simple",
			a:        []float32{1, 2, 3},
			b:        []float32{4, 5, 6},
			expected: 32.0, // 1*4 + 2*5 + 3*6 = 32
		},
		{
			name:     "orthogonal",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "same vector",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 14.0, // 1 + 4 + 9 = 14
		},
		{
			name:     "longer vector",
			a:        []float32{1, 2, 3, 4, 5, 6, 7, 8},
			b:        []float32{8, 7, 6, 5, 4, 3, 2, 1},
			expected: 120.0, // 8 + 14 + 18 + 20 + 20 + 18 + 14 + 8 = 120
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DotProduct(tt.a, tt.b)
			if math.Abs(float64(result-tt.expected)) > 0.0001 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestDotProductScalar(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5}
	b := []float32{5, 4, 3, 2, 1}

	result := dotProductScalar(a, b)
	expected := float32(35.0) // 5 + 8 + 9 + 8 + 5 = 35

	if math.Abs(float64(result-expected)) > 0.0001 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestL2Distance(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "same point",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 0.0,
		},
		{
			name:     "unit distance",
			a:        []float32{0, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "pythagorean",
			a:        []float32{0, 0},
			b:        []float32{3, 4},
			expected: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := L2Distance(tt.a, tt.b)
			if math.Abs(float64(result-tt.expected)) > 0.0001 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestL2DistanceScalar(t *testing.T) {
	a := []float32{0, 0}
	b := []float32{3, 4}

	result := l2DistanceScalar(a, b)
	expected := float32(5.0) // sqrt(9 + 16) = 5

	if math.Abs(float64(result-expected)) > 0.0001 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestDotProductPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched lengths")
		}
	}()

	a := []float32{1, 2, 3}
	b := []float32{1, 2}

	DotProduct(a, b)
}

func TestL2DistancePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched lengths")
		}
	}()

	a := []float32{1, 2, 3}
	b := []float32{1, 2}

	L2Distance(a, b)
}

func BenchmarkDotProduct_Small(b *testing.B) {
	a := make([]float32, 128)
	vec := make([]float32, 128)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DotProduct(a, vec)
	}
}

func BenchmarkDotProduct_Medium(b *testing.B) {
	a := make([]float32, 768)
	vec := make([]float32, 768)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DotProduct(a, vec)
	}
}

func BenchmarkDotProduct_Large(b *testing.B) {
	a := make([]float32, 4096)
	vec := make([]float32, 4096)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DotProduct(a, vec)
	}
}

func BenchmarkL2Distance_Small(b *testing.B) {
	a := make([]float32, 128)
	vec := make([]float32, 128)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = L2Distance(a, vec)
	}
}

func BenchmarkL2Distance_Medium(b *testing.B) {
	a := make([]float32, 768)
	vec := make([]float32, 768)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = L2Distance(a, vec)
	}
}

func BenchmarkL2Distance_Large(b *testing.B) {
	a := make([]float32, 4096)
	vec := make([]float32, 4096)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = L2Distance(a, vec)
	}
}

func BenchmarkDotProductScalar(b *testing.B) {
	a := make([]float32, 768)
	vec := make([]float32, 768)
	for i := range a {
		a[i] = float32(i) * 0.1
		vec[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dotProductScalar(a, vec)
	}
}
