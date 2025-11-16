package zerocopy

import (
	"testing"
	"unsafe"
)

func TestHeapBuffer_Basic(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()

	if buf.Len() != 1024 {
		t.Errorf("expected length 1024, got %d", buf.Len())
	}

	if buf.Cap() != 1024 {
		t.Errorf("expected capacity 1024, got %d", buf.Cap())
	}

	if buf.RefCount() != 1 {
		t.Errorf("expected ref count 1, got %d", buf.RefCount())
	}

	// Write some data
	copy(buf.Bytes(), []byte("hello"))

	if string(buf.Bytes()[:5]) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(buf.Bytes()[:5]))
	}
}

func TestHeapBuffer_Slice(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()
	defer buf.Release()

	// Write test data
	copy(buf.Bytes(), []byte("hello world"))

	// Create a slice
	slice := buf.Slice(0, 5)
	defer slice.Release()

	if slice.Len() != 5 {
		t.Errorf("expected slice length 5, got %d", slice.Len())
	}

	if string(slice.Bytes()) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(slice.Bytes()))
	}

	// Original buffer ref count should increase
	if buf.RefCount() != 2 {
		t.Errorf("expected ref count 2 after slice, got %d", buf.RefCount())
	}
}

func TestHeapBuffer_SliceBounds(t *testing.T) {
	buf := NewHeapBuffer(100)
	buf.Retain()
	defer buf.Release()

	tests := []struct {
		name      string
		start     int
		end       int
		shouldPanic bool
	}{
		{"valid slice", 0, 50, false},
		{"negative start", -1, 50, true},
		{"end beyond length", 0, 101, true},
		{"start > end", 50, 25, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic && r == nil {
					t.Error("expected panic but didn't get one")
				}
				if !tt.shouldPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			slice := buf.Slice(tt.start, tt.end)
			if slice != nil {
				slice.Release()
			}
		})
	}
}

func TestHeapBuffer_RefCounting(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()

	if buf.RefCount() != 1 {
		t.Fatalf("expected initial ref count 1, got %d", buf.RefCount())
	}

	// Retain multiple times
	buf.Retain()
	buf.Retain()

	if buf.RefCount() != 3 {
		t.Errorf("expected ref count 3, got %d", buf.RefCount())
	}

	// Release
	buf.Release()
	if buf.RefCount() != 2 {
		t.Errorf("expected ref count 2 after release, got %d", buf.RefCount())
	}

	buf.Release()
	buf.Release()

	// After all releases, ref count should be 0
	if buf.RefCount() != 0 {
		t.Errorf("expected ref count 0 after all releases, got %d", buf.RefCount())
	}
}

func TestHeapBuffer_DoubleRelease(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on double release")
		}
	}()

	buf.Release()
	buf.Release() // Should panic
}

func TestHeapBuffer_Ptr(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()
	defer buf.Release()

	ptr := buf.Ptr()
	if ptr == nil {
		t.Error("expected non-nil pointer")
	}

	// Verify we can access data through pointer
	bytes := buf.Bytes()
	copy(bytes, []byte("test"))

	ptrBytes := (*[4]byte)(unsafe.Pointer(ptr))
	if string(ptrBytes[:]) != "test" {
		t.Errorf("expected 'test' through pointer, got '%s'", string(ptrBytes[:]))
	}
}

func TestHeapBuffer_SetLength(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()
	defer buf.Release()

	buf.SetLength(512)
	if buf.Len() != 512 {
		t.Errorf("expected length 512, got %d", buf.Len())
	}

	if buf.Cap() != 1024 {
		t.Errorf("expected capacity unchanged at 1024, got %d", buf.Cap())
	}
}

func TestHeapBuffer_SetLengthExceedsCap(t *testing.T) {
	buf := NewHeapBuffer(1024)
	buf.Retain()
	defer buf.Release()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when setting length beyond capacity")
		}
	}()

	buf.SetLength(2048)
}

func TestHeapBuffer_ZeroLength(t *testing.T) {
	buf := NewHeapBuffer(0)
	buf.Retain()
	defer buf.Release()

	if buf.Len() != 0 {
		t.Errorf("expected length 0, got %d", buf.Len())
	}

	ptr := buf.Ptr()
	if ptr != nil {
		t.Error("expected nil pointer for zero-length buffer")
	}
}

func BenchmarkHeapBuffer_Allocate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := NewHeapBuffer(4096)
		buf.Retain()
		buf.Release()
	}
}

func BenchmarkHeapBuffer_Slice(b *testing.B) {
	buf := NewHeapBuffer(4096)
	buf.Retain()
	defer buf.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := buf.Slice(0, 2048)
		slice.Release()
	}
}

func BenchmarkHeapBuffer_RefCounting(b *testing.B) {
	buf := NewHeapBuffer(4096)
	buf.Retain()
	defer buf.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Retain()
		buf.Release()
	}
}
