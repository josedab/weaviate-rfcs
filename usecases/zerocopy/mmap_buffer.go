package zerocopy

import (
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"

	"golang.org/x/exp/mmap"
)

// MMapBuffer is a buffer backed by a memory-mapped file.
// Multiple slices can reference the same underlying mmap region
// with proper reference counting to prevent premature unmapping.
type MMapBuffer struct {
	data   []byte
	file   *os.File
	offset int64
	length int
	refs   *atomic.Int32
	reader *mmap.ReaderAt
}

// NewMMapBuffer creates a new memory-mapped buffer from a file
func NewMMapBuffer(filename string, offset int64, length int) (*MMapBuffer, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	reader, err := mmap.Open(filename)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap file: %w", err)
	}

	// Get the file size to validate offset and length
	info, err := file.Stat()
	if err != nil {
		reader.Close()
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if offset+int64(length) > info.Size() {
		reader.Close()
		file.Close()
		return nil, fmt.Errorf("offset %d + length %d exceeds file size %d",
			offset, length, info.Size())
	}

	// Create a byte slice view into the mmap region
	data := make([]byte, length)
	if _, err := reader.ReadAt(data, offset); err != nil {
		reader.Close()
		file.Close()
		return nil, fmt.Errorf("failed to read mmap data: %w", err)
	}

	refs := &atomic.Int32{}
	refs.Store(1)

	return &MMapBuffer{
		data:   data,
		file:   file,
		offset: offset,
		length: length,
		refs:   refs,
		reader: reader,
	}, nil
}

func (b *MMapBuffer) Bytes() []byte {
	return b.data
}

func (b *MMapBuffer) Ptr() unsafe.Pointer {
	if len(b.data) == 0 {
		return nil
	}
	return unsafe.Pointer(&b.data[0])
}

func (b *MMapBuffer) Len() int {
	return b.length
}

func (b *MMapBuffer) Cap() int {
	return len(b.data)
}

func (b *MMapBuffer) Slice(start, end int) Buffer {
	if start < 0 || end > b.length || start > end {
		panic(fmt.Sprintf("slice bounds out of range [%d:%d] with length %d",
			start, end, b.length))
	}

	// Create a view that shares the same underlying mmap
	return &MMapBuffer{
		data:   b.data[start:end],
		file:   b.file,
		offset: b.offset + int64(start),
		length: end - start,
		refs:   b.refs, // Share the same reference counter
		reader: b.reader,
	}
}

func (b *MMapBuffer) Retain() Buffer {
	b.refs.Add(1)
	return b
}

func (b *MMapBuffer) Release() {
	newCount := b.refs.Add(-1)
	if newCount == 0 {
		// Close the mmap and file when no more references exist
		if b.reader != nil {
			b.reader.Close()
		}
		if b.file != nil {
			b.file.Close()
		}
	} else if newCount < 0 {
		panic(fmt.Sprintf("mmap buffer released too many times: ref count = %d", newCount))
	}
}

func (b *MMapBuffer) RefCount() int32 {
	return b.refs.Load()
}
