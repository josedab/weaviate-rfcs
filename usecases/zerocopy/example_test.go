package zerocopy_test

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate/usecases/zerocopy"
)

// Example demonstrates basic usage of the zero-copy pipeline
func Example() {
	// Create a buffer pool for efficient memory management
	pool := zerocopy.NewBufferPool()

	// Create an object store
	store := zerocopy.NewObjectStore()

	// Create a vector index for 3-dimensional vectors
	index := zerocopy.NewVectorIndex(3)

	// Create an object
	writer := zerocopy.NewObjectWriter(1024)
	writer.WriteHeader(2) // 2 properties

	// Write properties
	writer.WriteString("name")
	writer.WriteString("example")

	// Write vector
	vector := []float32{1.0, 2.0, 3.0}
	writer.WriteVector(vector)

	// Get a buffer from the pool and store the object
	id := uuid.New()
	buf := pool.Get(int64(len(writer.Bytes())))
	copy(buf.Bytes(), writer.Bytes())

	// Store object (zero-copy)
	store.Put(id, buf)

	// Add vector to index
	index.AddVector(id, vector)

	// Retrieve object (zero-copy)
	reader, _ := store.GetReader(id)
	defer reader.Buffer().Release()

	// Get vector without copying
	retrievedVector, _ := reader.GetVector()

	fmt.Printf("Vector: %v\n", retrievedVector)

	// Search for similar vectors
	queryVector := []float32{1.0, 2.0, 3.0}
	ids, distances, _ := index.Search(queryVector, 1)

	fmt.Printf("Found %d results\n", len(ids))
	fmt.Printf("Top result distance: %.2f\n", distances[0])

	// Output:
	// Vector: [1 2 3]
	// Found 1 results
	// Top result distance: 14.00
}

// Example_bufferPooling demonstrates buffer pooling for reduced allocations
func Example_bufferPooling() {
	pool := zerocopy.NewBufferPool()

	// Get a buffer from the pool
	buf := pool.Get(4096)
	fmt.Printf("Buffer capacity: %d\n", buf.Cap())

	// Use the buffer
	copy(buf.Bytes(), []byte("Hello, zero-copy!"))

	// Release it back to the pool
	buf.Release()

	// Get another buffer - likely reuses the same underlying memory
	buf2 := pool.Get(4096)
	fmt.Printf("Reused buffer capacity: %d\n", buf2.Cap())
	buf2.Release()

	// Output:
	// Buffer capacity: 4096
	// Reused buffer capacity: 4096
}

// Example_simdOperations demonstrates SIMD-optimized vector operations
func Example_simdOperations() {
	// Create two vectors
	a := []float32{1.0, 2.0, 3.0, 4.0}
	b := []float32{5.0, 6.0, 7.0, 8.0}

	// Compute dot product (uses AVX2/NEON if available)
	dotProd := zerocopy.DotProduct(a, b)
	fmt.Printf("Dot product: %.1f\n", dotProd)

	// Compute L2 distance
	distance := zerocopy.L2Distance(a, b)
	fmt.Printf("L2 distance: %.1f\n", distance)

	// Output:
	// Dot product: 70.0
	// L2 distance: 8.0
}

// Example_vectorSearch demonstrates zero-copy vector search
func Example_vectorSearch() {
	index := zerocopy.NewVectorIndex(3)

	// Add some vectors
	vectors := [][]float32{
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
		{0.0, 0.0, 1.0},
		{1.0, 1.0, 0.0},
	}

	ids := make([]uuid.UUID, len(vectors))
	for i, vec := range vectors {
		ids[i] = uuid.New()
		index.AddVector(ids[i], vec)
	}

	// Search for vectors similar to [1, 0, 0]
	query := []float32{1.0, 0.0, 0.0}
	resultIDs, distances, _ := index.Search(query, 2)

	fmt.Printf("Found %d results\n", len(resultIDs))
	fmt.Printf("Top result similarity: %.1f\n", distances[0])

	// Output:
	// Found 2 results
	// Top result similarity: 1.0
}

// Example_objectStore demonstrates zero-copy object storage
func Example_objectStore() {
	store := zerocopy.NewObjectStore()

	// Create an object
	writer := zerocopy.NewObjectWriter(512)
	writer.WriteHeader(1)
	writer.WriteString("data")
	writer.WriteVector([]float32{0.1, 0.2, 0.3})

	// Store it
	id := uuid.New()
	buf := zerocopy.NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	store.Put(id, buf)

	// Retrieve and read (zero-copy)
	vector, _ := store.GetVector(id)

	fmt.Printf("Vector length: %d\n", len(vector))
	fmt.Printf("First element: %.1f\n", vector[0])

	// Output:
	// Vector length: 3
	// First element: 0.1
}
