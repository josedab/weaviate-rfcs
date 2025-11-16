package zerocopy

import (
	"sync"
	"testing"
)

func TestBufferPool_Get(t *testing.T) {
	pool := NewBufferPool()

	buf := pool.Get(1024)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}

	if buf.Len() != 1024 {
		t.Errorf("expected length 1024, got %d", buf.Len())
	}

	// Should get a buffer with capacity >= requested size
	if buf.Cap() < 1024 {
		t.Errorf("expected capacity >= 1024, got %d", buf.Cap())
	}

	buf.Release()
}

func TestBufferPool_GetMultipleSizes(t *testing.T) {
	pool := NewBufferPool()

	sizes := []int64{100, 1024, 4096, 16384, 65536, 262144, 1048576}

	for _, size := range sizes {
		buf := pool.Get(size)
		if buf.Len() != int(size) {
			t.Errorf("size %d: expected length %d, got %d", size, size, buf.Len())
		}
		buf.Release()
	}
}

func TestBufferPool_PutAndReuse(t *testing.T) {
	pool := NewBufferPool()

	// Get a buffer
	buf1 := pool.Get(4096)
	if buf1.RefCount() != 1 {
		t.Errorf("expected ref count 1, got %d", buf1.RefCount())
	}

	// Write some data
	copy(buf1.Bytes(), []byte("test data"))

	// Release it
	buf1.Release()

	// Get another buffer of the same size
	buf2 := pool.Get(4096)

	// Should be reused (same underlying capacity)
	// Note: We can't reliably test pointer equality due to pool behavior
	// but we can verify it works correctly
	if buf2.Len() != 4096 {
		t.Errorf("expected length 4096, got %d", buf2.Len())
	}

	buf2.Release()
}

func TestBufferPool_LargerThanBuckets(t *testing.T) {
	pool := NewBufferPool()

	// Request a size larger than largest bucket
	buf := pool.Get(20 * 1024 * 1024) // 20 MB

	if buf.Len() != 20*1024*1024 {
		t.Errorf("expected length 20MB, got %d", buf.Len())
	}

	buf.Release()
}

func TestBufferPool_ConcurrentGetPut(t *testing.T) {
	pool := NewBufferPool()

	var wg sync.WaitGroup
	numGoroutines := 100
	numIterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				buf := pool.Get(4096)
				// Do some work
				copy(buf.Bytes(), []byte("concurrent test"))
				buf.Release()
			}
		}()
	}

	wg.Wait()
}

func TestBufferPool_RefCountPreventsPut(t *testing.T) {
	pool := NewBufferPool()

	buf := pool.Get(4096)
	buf.Retain() // Increase ref count

	// Release once - should not return to pool
	buf.Release()

	if buf.RefCount() != 1 {
		t.Errorf("expected ref count 1, got %d", buf.RefCount())
	}

	// Final release
	buf.Release()

	if buf.RefCount() != 0 {
		t.Errorf("expected ref count 0, got %d", buf.RefCount())
	}
}

func TestBufferPool_CustomSizes(t *testing.T) {
	customSizes := []int{512, 2048, 8192}
	pool := NewBufferPoolWithSizes(customSizes)

	// Test each custom size
	for _, size := range customSizes {
		buf := pool.Get(int64(size))
		if buf.Len() != size {
			t.Errorf("expected length %d, got %d", size, buf.Len())
		}
		buf.Release()
	}

	// Test in-between size
	buf := pool.Get(1024)
	if buf.Len() != 1024 {
		t.Errorf("expected length 1024, got %d", buf.Len())
	}
	// Should get bucket >= 1024 (which is 2048)
	if buf.Cap() < 1024 {
		t.Errorf("expected capacity >= 1024, got %d", buf.Cap())
	}
	buf.Release()
}

func TestBufferPool_Stats(t *testing.T) {
	pool := NewBufferPool()

	stats := pool.Stats()
	if len(stats.Buckets) == 0 {
		t.Error("expected non-zero buckets in stats")
	}

	// Verify bucket sizes match
	expectedSizes := []int{1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216}
	if len(stats.Buckets) != len(expectedSizes) {
		t.Errorf("expected %d buckets, got %d", len(expectedSizes), len(stats.Buckets))
	}

	for i, bucket := range stats.Buckets {
		if bucket.Size != expectedSizes[i] {
			t.Errorf("bucket %d: expected size %d, got %d", i, expectedSizes[i], bucket.Size)
		}
	}
}

func TestBufferPool_FindBucket(t *testing.T) {
	pool := NewBufferPool()

	tests := []struct {
		requestSize  int
		minBucketSize int
	}{
		{100, 1024},
		{1024, 1024},
		{1025, 4096},
		{4096, 4096},
		{5000, 16384},
		{65536, 65536},
		{100000, 262144},
	}

	for _, tt := range tests {
		bucket := pool.findBucket(tt.requestSize)
		if bucket < tt.minBucketSize {
			t.Errorf("size %d: expected bucket >= %d, got %d",
				tt.requestSize, tt.minBucketSize, bucket)
		}
	}
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(4096)
		buf.Release()
	}
}

func BenchmarkBufferPool_GetPutParallel(b *testing.B) {
	pool := NewBufferPool()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(4096)
			buf.Release()
		}
	})
}

func BenchmarkBufferPool_VariableSizes(b *testing.B) {
	pool := NewBufferPool()
	sizes := []int64{1024, 4096, 16384, 65536}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		buf := pool.Get(size)
		buf.Release()
	}
}

func BenchmarkDirectAllocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 4096)
		_ = buf
	}
}
