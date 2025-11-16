package zerocopy

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// Buffer provides a zero-copy interface for accessing memory regions.
// It supports reference counting to manage buffer lifecycle and prevents
// premature release while slices are still in use.
type Buffer interface {
	// Bytes returns the underlying byte slice
	Bytes() []byte

	// Ptr returns an unsafe pointer to the buffer's data
	Ptr() unsafe.Pointer

	// Len returns the current length of the buffer
	Len() int

	// Cap returns the capacity of the buffer
	Cap() int

	// Slice creates a view into the buffer without copying.
	// The returned buffer shares the same underlying data.
	Slice(start, end int) Buffer

	// Retain increments the reference count and returns the buffer.
	// This prevents the buffer from being released while in use.
	Retain() Buffer

	// Release decrements the reference count.
	// When the count reaches zero, the buffer is returned to the pool.
	Release()

	// RefCount returns the current reference count
	RefCount() int32
}

// HeapBuffer is a buffer allocated on the Go heap, suitable for pooling
type HeapBuffer struct {
	data   []byte
	length int
	refs   atomic.Int32
	pool   *BufferPool
}

// NewHeapBuffer creates a new heap-allocated buffer
func NewHeapBuffer(size int) *HeapBuffer {
	return &HeapBuffer{
		data:   make([]byte, size),
		length: size,
		refs:   atomic.Int32{},
	}
}

func (b *HeapBuffer) Bytes() []byte {
	return b.data[:b.length]
}

func (b *HeapBuffer) Ptr() unsafe.Pointer {
	if len(b.data) == 0 {
		return nil
	}
	return unsafe.Pointer(&b.data[0])
}

func (b *HeapBuffer) Len() int {
	return b.length
}

func (b *HeapBuffer) Cap() int {
	return cap(b.data)
}

func (b *HeapBuffer) Slice(start, end int) Buffer {
	if start < 0 || end > b.length || start > end {
		panic(fmt.Sprintf("slice bounds out of range [%d:%d] with length %d",
			start, end, b.length))
	}

	// Create a new view that shares the same underlying data
	view := &HeapBuffer{
		data:   b.data[start:end],
		length: end - start,
		refs:   b.refs,
		pool:   b.pool,
	}

	// Increment reference count since we now have another reference
	b.Retain()

	return view
}

func (b *HeapBuffer) Retain() Buffer {
	b.refs.Add(1)
	return b
}

func (b *HeapBuffer) Release() {
	newCount := b.refs.Add(-1)
	if newCount == 0 {
		// Return to pool if available
		if b.pool != nil {
			b.pool.Put(b)
		}
	} else if newCount < 0 {
		panic(fmt.Sprintf("buffer released too many times: ref count = %d", newCount))
	}
}

func (b *HeapBuffer) RefCount() int32 {
	return b.refs.Load()
}

// Reset resets the buffer for reuse
func (b *HeapBuffer) Reset() {
	b.length = 0
	b.refs.Store(0)
}

// SetLength sets the logical length of the buffer
func (b *HeapBuffer) SetLength(length int) {
	if length > cap(b.data) {
		panic(fmt.Sprintf("length %d exceeds capacity %d", length, cap(b.data)))
	}
	b.length = length
}
