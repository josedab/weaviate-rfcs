package zerocopy

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// ObjectStore provides zero-copy object storage using memory-mapped buffers
type ObjectStore struct {
	objects map[uuid.UUID]Buffer
	pool    *BufferPool
	mu      sync.RWMutex
}

// NewObjectStore creates a new zero-copy object store
func NewObjectStore() *ObjectStore {
	return &ObjectStore{
		objects: make(map[uuid.UUID]Buffer),
		pool:    NewBufferPool(),
	}
}

// Put stores an object with zero-copy semantics
// The buffer ownership is transferred to the store
func (s *ObjectStore) Put(id uuid.UUID, buf Buffer) error {
	if buf == nil {
		return fmt.Errorf("buffer cannot be nil")
	}

	// Validate it's a valid object format
	_, err := NewObjectReader(buf)
	if err != nil {
		return fmt.Errorf("invalid object format: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Release old buffer if exists
	if oldBuf, exists := s.objects[id]; exists {
		oldBuf.Release()
	}

	// Store the new buffer (retain to keep it alive)
	s.objects[id] = buf.Retain()

	return nil
}

// Get retrieves an object buffer without copying
// The returned buffer must be released when done
func (s *ObjectStore) Get(id uuid.UUID) (Buffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buf, exists := s.objects[id]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", id)
	}

	// Retain the buffer for the caller
	return buf.Retain(), nil
}

// GetReader retrieves an object reader without copying
// The reader's buffer must be released when done
func (s *ObjectStore) GetReader(id uuid.UUID) (*ObjectReader, error) {
	buf, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	reader, err := NewObjectReader(buf)
	if err != nil {
		buf.Release()
		return nil, err
	}

	return reader, nil
}

// Delete removes an object from the store
func (s *ObjectStore) Delete(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, exists := s.objects[id]
	if !exists {
		return fmt.Errorf("object not found: %s", id)
	}

	// Release the buffer
	buf.Release()
	delete(s.objects, id)

	return nil
}

// Exists checks if an object exists in the store
func (s *ObjectStore) Exists(id uuid.UUID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.objects[id]
	return exists
}

// Size returns the number of objects in the store
func (s *ObjectStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.objects)
}

// GetVector retrieves just the vector from an object (zero-copy)
func (s *ObjectStore) GetVector(id uuid.UUID) ([]float32, error) {
	reader, err := s.GetReader(id)
	if err != nil {
		return nil, err
	}
	defer reader.Buffer().Release()

	return reader.GetVector()
}

// Clear removes all objects from the store
func (s *ObjectStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Release all buffers
	for _, buf := range s.objects {
		buf.Release()
	}

	s.objects = make(map[uuid.UUID]Buffer)
}

// Pool returns the buffer pool used by the store
func (s *ObjectStore) Pool() *BufferPool {
	return s.pool
}

// ObjectStoreStats provides statistics about the object store
type ObjectStoreStats struct {
	ObjectCount   int
	TotalBytes    int64
	AverageSize   int64
	LargestObject int64
	SmallestObject int64
}

// Stats returns statistics about the object store
func (s *ObjectStore) Stats() ObjectStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ObjectStoreStats{
		ObjectCount: len(s.objects),
		SmallestObject: int64(^uint(0) >> 1), // Max int64
	}

	for _, buf := range s.objects {
		size := int64(buf.Len())
		stats.TotalBytes += size

		if size > stats.LargestObject {
			stats.LargestObject = size
		}
		if size < stats.SmallestObject {
			stats.SmallestObject = size
		}
	}

	if stats.ObjectCount > 0 {
		stats.AverageSize = stats.TotalBytes / int64(stats.ObjectCount)
	} else {
		stats.SmallestObject = 0
	}

	return stats
}
