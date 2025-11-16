package zerocopy

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/google/uuid"
)

// VectorIndex provides zero-copy vector indexing using memory-mapped storage
type VectorIndex struct {
	// Vector storage
	vectorFile *os.File
	vectorMap  []byte
	vectorSize int // Dimension of vectors
	vectorCount int

	// Index metadata
	idToOffset map[uuid.UUID]int64
	offsetToID map[int64]uuid.UUID

	mu sync.RWMutex
}

// NewVectorIndex creates a new zero-copy vector index
func NewVectorIndex(vectorDim int) *VectorIndex {
	return &VectorIndex{
		vectorSize: vectorDim,
		idToOffset: make(map[uuid.UUID]int64),
		offsetToID: make(map[int64]uuid.UUID),
	}
}

// AddVector adds a vector to the index using zero-copy semantics
func (v *VectorIndex) AddVector(id uuid.UUID, vector []float32) error {
	if len(vector) != v.vectorSize {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d",
			v.vectorSize, len(vector))
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Check if already exists
	if _, exists := v.idToOffset[id]; exists {
		return fmt.Errorf("vector already exists: %s", id)
	}

	// Calculate offset for new vector
	offset := int64(v.vectorCount * v.vectorSize * 4) // 4 bytes per float32

	// Store in-memory for now (production would use mmap file)
	if v.vectorMap == nil {
		v.vectorMap = make([]byte, 0, 1024*1024) // Start with 1MB
	}

	// Ensure capacity
	needed := int(offset) + v.vectorSize*4
	if cap(v.vectorMap) < needed {
		newMap := make([]byte, needed, needed*2)
		copy(newMap, v.vectorMap)
		v.vectorMap = newMap
	}

	if len(v.vectorMap) < needed {
		v.vectorMap = v.vectorMap[:needed]
	}

	// Write vector data directly to memory (zero-copy)
	dst := unsafe.Slice(
		(*float32)(unsafe.Pointer(&v.vectorMap[offset])),
		v.vectorSize,
	)
	copy(dst, vector)

	// Update index
	v.idToOffset[id] = offset
	v.offsetToID[offset] = id
	v.vectorCount++

	return nil
}

// GetVector retrieves a vector using zero-copy access
// Returns a slice backed by the mmap region
func (v *VectorIndex) GetVector(id uuid.UUID) ([]float32, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	offset, exists := v.idToOffset[id]
	if !exists {
		return nil, fmt.Errorf("vector not found: %s", id)
	}

	return v.getVectorDirect(offset), nil
}

// getVectorDirect retrieves a vector by offset (internal, no locking)
func (v *VectorIndex) getVectorDirect(offset int64) []float32 {
	// Create zero-copy slice backed by mmap
	return unsafe.Slice(
		(*float32)(unsafe.Pointer(&v.vectorMap[offset])),
		v.vectorSize,
	)
}

// Search performs a vector similarity search
// This is a simplified implementation; production would use HNSW algorithm
func (v *VectorIndex) Search(query []float32, k int) ([]uuid.UUID, []float32, error) {
	if len(query) != v.vectorSize {
		return nil, nil, fmt.Errorf("query dimension mismatch: expected %d, got %d",
			v.vectorSize, len(query))
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	if k > v.vectorCount {
		k = v.vectorCount
	}

	if k == 0 {
		return []uuid.UUID{}, []float32{}, nil
	}

	// Simple linear scan with dot product
	// Production would use HNSW with SIMD optimizations
	type result struct {
		id       uuid.UUID
		distance float32
	}

	results := make([]result, 0, v.vectorCount)

	for id, offset := range v.idToOffset {
		vector := v.getVectorDirect(offset)
		distance := dotProduct(query, vector)
		results = append(results, result{id: id, distance: distance})
	}

	// Sort by distance (descending for dot product)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].distance > results[i].distance {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Take top k
	if len(results) > k {
		results = results[:k]
	}

	ids := make([]uuid.UUID, len(results))
	distances := make([]float32, len(results))
	for i, r := range results {
		ids[i] = r.id
		distances[i] = r.distance
	}

	return ids, distances, nil
}

// Delete removes a vector from the index
func (v *VectorIndex) Delete(id uuid.UUID) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	offset, exists := v.idToOffset[id]
	if !exists {
		return fmt.Errorf("vector not found: %s", id)
	}

	delete(v.idToOffset, id)
	delete(v.offsetToID, offset)

	return nil
}

// Size returns the number of vectors in the index
func (v *VectorIndex) Size() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.vectorCount
}

// Dimension returns the vector dimension
func (v *VectorIndex) Dimension() int {
	return v.vectorSize
}

// Close closes the vector index and releases resources
func (v *VectorIndex) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vectorFile != nil {
		if err := v.vectorFile.Close(); err != nil {
			return err
		}
	}

	v.vectorMap = nil
	return nil
}

// dotProduct computes the dot product of two vectors
// This is a basic implementation; production would use SIMD
func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
