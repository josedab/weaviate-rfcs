package zerocopy

import (
	"testing"

	"github.com/google/uuid"
)

func TestObjectStore_PutGet(t *testing.T) {
	store := NewObjectStore()
	id := uuid.New()

	// Create an object
	writer := NewObjectWriter(1024)
	writer.WriteHeader(1)
	writer.WriteString("test")
	vector := []float32{1.0, 2.0, 3.0}
	writer.WriteVector(vector)

	// Create buffer
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	// Store it
	if err := store.Put(id, buf); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Retrieve it
	retrieved, err := store.Get(id)
	if err != nil {
		t.Fatalf("failed to get object: %v", err)
	}
	defer retrieved.Release()

	if retrieved.Len() != buf.Len() {
		t.Errorf("expected length %d, got %d", buf.Len(), retrieved.Len())
	}
}

func TestObjectStore_GetReader(t *testing.T) {
	store := NewObjectStore()
	id := uuid.New()

	// Create an object with vector
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)
	vector := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	writer.WriteVector(vector)

	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	if err := store.Put(id, buf); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Get reader
	reader, err := store.GetReader(id)
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}
	defer reader.Buffer().Release()

	// Read vector
	readVector, err := reader.GetVector()
	if err != nil {
		t.Fatalf("failed to read vector: %v", err)
	}

	if len(readVector) != len(vector) {
		t.Fatalf("expected vector length %d, got %d", len(vector), len(readVector))
	}

	for i, v := range vector {
		if readVector[i] != v {
			t.Errorf("vector[%d]: expected %f, got %f", i, v, readVector[i])
		}
	}
}

func TestObjectStore_Delete(t *testing.T) {
	store := NewObjectStore()
	id := uuid.New()

	// Create and store an object
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)

	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	if err := store.Put(id, buf); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Verify it exists
	if !store.Exists(id) {
		t.Error("object should exist")
	}

	// Delete it
	if err := store.Delete(id); err != nil {
		t.Fatalf("failed to delete object: %v", err)
	}

	// Verify it's gone
	if store.Exists(id) {
		t.Error("object should not exist after delete")
	}

	// Try to get it (should fail)
	_, err := store.Get(id)
	if err == nil {
		t.Error("expected error getting deleted object")
	}
}

func TestObjectStore_GetVector(t *testing.T) {
	store := NewObjectStore()
	id := uuid.New()

	// Create object with vector
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)
	vector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	writer.WriteVector(vector)

	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	if err := store.Put(id, buf); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Get vector directly
	readVector, err := store.GetVector(id)
	if err != nil {
		t.Fatalf("failed to get vector: %v", err)
	}

	if len(readVector) != len(vector) {
		t.Fatalf("expected vector length %d, got %d", len(vector), len(readVector))
	}

	for i, v := range vector {
		if readVector[i] != v {
			t.Errorf("vector[%d]: expected %f, got %f", i, v, readVector[i])
		}
	}
}

func TestObjectStore_Size(t *testing.T) {
	store := NewObjectStore()

	if store.Size() != 0 {
		t.Errorf("expected size 0, got %d", store.Size())
	}

	// Add some objects
	for i := 0; i < 5; i++ {
		id := uuid.New()
		writer := NewObjectWriter(1024)
		writer.WriteHeader(0)

		buf := NewHeapBuffer(len(writer.Bytes()))
		buf.Retain()
		copy(buf.Bytes(), writer.Bytes())
		buf.SetLength(len(writer.Bytes()))

		store.Put(id, buf)
	}

	if store.Size() != 5 {
		t.Errorf("expected size 5, got %d", store.Size())
	}
}

func TestObjectStore_Clear(t *testing.T) {
	store := NewObjectStore()

	// Add objects
	for i := 0; i < 10; i++ {
		id := uuid.New()
		writer := NewObjectWriter(1024)
		writer.WriteHeader(0)

		buf := NewHeapBuffer(len(writer.Bytes()))
		buf.Retain()
		copy(buf.Bytes(), writer.Bytes())
		buf.SetLength(len(writer.Bytes()))

		store.Put(id, buf)
	}

	if store.Size() != 10 {
		t.Fatalf("expected size 10, got %d", store.Size())
	}

	// Clear
	store.Clear()

	if store.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", store.Size())
	}
}

func TestObjectStore_Stats(t *testing.T) {
	store := NewObjectStore()

	// Add objects of varying sizes
	sizes := []int{100, 500, 1000, 2000}
	for _, size := range sizes {
		id := uuid.New()
		writer := NewObjectWriter(size)
		writer.WriteHeader(0)

		// Fill with data
		for i := 0; i < (size-HeaderSize)/8; i++ {
			writer.WriteString("x")
		}

		buf := NewHeapBuffer(len(writer.Bytes()))
		buf.Retain()
		copy(buf.Bytes(), writer.Bytes())
		buf.SetLength(len(writer.Bytes()))

		store.Put(id, buf)
	}

	stats := store.Stats()

	if stats.ObjectCount != len(sizes) {
		t.Errorf("expected object count %d, got %d", len(sizes), stats.ObjectCount)
	}

	if stats.TotalBytes == 0 {
		t.Error("expected non-zero total bytes")
	}

	if stats.AverageSize == 0 {
		t.Error("expected non-zero average size")
	}
}

func BenchmarkObjectStore_Put(b *testing.B) {
	store := NewObjectStore()

	// Create a test object
	writer := NewObjectWriter(4096)
	writer.WriteHeader(0)
	vector := make([]float32, 768)
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}
	writer.WriteVector(vector)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uuid.New()
		buf := NewHeapBuffer(len(writer.Bytes()))
		buf.Retain()
		copy(buf.Bytes(), writer.Bytes())
		buf.SetLength(len(writer.Bytes()))

		store.Put(id, buf)
		buf.Release()
	}
}

func BenchmarkObjectStore_Get(b *testing.B) {
	store := NewObjectStore()
	id := uuid.New()

	// Create and store an object
	writer := NewObjectWriter(4096)
	writer.WriteHeader(0)
	vector := make([]float32, 768)
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}
	writer.WriteVector(vector)

	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))
	store.Put(id, buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retrieved, _ := store.Get(id)
		retrieved.Release()
	}
}
