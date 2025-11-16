package zerocopy

import (
	"sort"
	"sync"
	"sync/atomic"
)

// BufferPool manages a pool of reusable buffers to reduce allocations.
// Buffers are organized by size buckets for efficient allocation.
type BufferPool struct {
	pools map[int]*sync.Pool
	sizes []int
	mu    sync.RWMutex
}

// NewBufferPool creates a new buffer pool with predefined size buckets
func NewBufferPool() *BufferPool {
	return NewBufferPoolWithSizes([]int{
		1024,      // 1 KB
		4096,      // 4 KB
		16384,     // 16 KB
		65536,     // 64 KB
		262144,    // 256 KB
		1048576,   // 1 MB
		4194304,   // 4 MB
		16777216,  // 16 MB
	})
}

// NewBufferPoolWithSizes creates a buffer pool with custom size buckets
func NewBufferPoolWithSizes(sizes []int) *BufferPool {
	// Sort sizes for efficient bucket lookup
	sortedSizes := make([]int, len(sizes))
	copy(sortedSizes, sizes)
	sort.Ints(sortedSizes)

	pool := &BufferPool{
		pools: make(map[int]*sync.Pool),
		sizes: sortedSizes,
	}

	// Initialize a pool for each size bucket
	for _, size := range sortedSizes {
		bucketSize := size
		pool.pools[bucketSize] = &sync.Pool{
			New: func() interface{} {
				buf := &HeapBuffer{
					data:   make([]byte, bucketSize),
					length: 0,
					refs:   atomic.Int32{},
				}
				buf.pool = pool
				return buf
			},
		}
	}

	return pool
}

// Get retrieves a buffer from the pool with at least the requested size
func (p *BufferPool) Get(size int64) Buffer {
	bucket := p.findBucket(int(size))

	p.mu.RLock()
	pool, ok := p.pools[bucket]
	p.mu.RUnlock()

	if !ok {
		// Size not in predefined buckets, allocate directly
		buf := &HeapBuffer{
			data:   make([]byte, size),
			length: int(size),
			refs:   atomic.Int32{},
			pool:   p,
		}
		buf.refs.Store(1)
		return buf
	}

	buf := pool.Get().(*HeapBuffer)
	buf.length = int(size)
	buf.refs.Store(1)

	return buf
}

// Put returns a buffer to the pool for reuse
func (p *BufferPool) Put(buf Buffer) {
	heap, ok := buf.(*HeapBuffer)
	if !ok {
		return
	}

	// Only return to pool if ref count is zero
	if heap.RefCount() != 0 {
		return
	}

	// Reset the buffer
	heap.Reset()

	// Find the appropriate pool
	bucket := cap(heap.data)
	p.mu.RLock()
	pool, ok := p.pools[bucket]
	p.mu.RUnlock()

	if ok {
		pool.Put(heap)
	}
	// If no matching pool, let GC collect it
}

// findBucket finds the smallest bucket that fits the requested size
func (p *BufferPool) findBucket(size int) int {
	// Binary search for the smallest size >= requested
	idx := sort.SearchInts(p.sizes, size)

	if idx < len(p.sizes) {
		return p.sizes[idx]
	}

	// If larger than all buckets, return the largest
	if len(p.sizes) > 0 {
		return p.sizes[len(p.sizes)-1] * ((size / p.sizes[len(p.sizes)-1]) + 1)
	}

	return size
}

// Stats returns statistics about buffer pool usage
type PoolStats struct {
	Buckets []BucketStats
}

type BucketStats struct {
	Size      int
	Allocated int
	InUse     int
}

// Stats returns current pool statistics
func (p *BufferPool) Stats() PoolStats {
	stats := PoolStats{
		Buckets: make([]BucketStats, 0, len(p.sizes)),
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, size := range p.sizes {
		stats.Buckets = append(stats.Buckets, BucketStats{
			Size: size,
			// Note: sync.Pool doesn't expose size metrics
			// These would need custom tracking if needed
		})
	}

	return stats
}
