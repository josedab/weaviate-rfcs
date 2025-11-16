package zerocopy

import (
	"testing"
)

func TestObjectWriter_WriteHeader(t *testing.T) {
	writer := NewObjectWriter(1024)
	writer.WriteHeader(5)

	if len(writer.Bytes()) != HeaderSize {
		t.Errorf("expected header size %d, got %d", HeaderSize, len(writer.Bytes()))
	}

	// Verify magic number
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	header := reader.GetHeader()
	if header.Magic != ObjectMagic {
		t.Errorf("expected magic 0x%08X, got 0x%08X", ObjectMagic, header.Magic)
	}
	if header.Version != 1 {
		t.Errorf("expected version 1, got %d", header.Version)
	}
	if header.PropertyCount != 5 {
		t.Errorf("expected property count 5, got %d", header.PropertyCount)
	}
}

func TestObjectWriter_WriteVector(t *testing.T) {
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)

	vector := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	writer.WriteVector(vector)

	// Create reader
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

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

func TestObjectWriter_WriteString(t *testing.T) {
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)

	testStr := "Hello, zero-copy world!"
	offset := writer.WriteString(testStr)

	// Create reader
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	// Read string
	readStr, err := reader.GetString(offset)
	if err != nil {
		t.Fatalf("failed to read string: %v", err)
	}

	if readStr != testStr {
		t.Errorf("expected string '%s', got '%s'", testStr, readStr)
	}
}

func TestObjectReader_InvalidMagic(t *testing.T) {
	buf := NewHeapBuffer(HeaderSize)
	buf.Retain()
	defer buf.Release()

	// Write invalid magic
	copy(buf.Bytes(), []byte{0x00, 0x00, 0x00, 0x00})

	_, err := NewObjectReader(buf)
	if err == nil {
		t.Error("expected error for invalid magic number")
	}
}

func TestObjectReader_BufferTooSmall(t *testing.T) {
	buf := NewHeapBuffer(8) // Less than HeaderSize
	buf.Retain()
	defer buf.Release()

	_, err := NewObjectReader(buf)
	if err == nil {
		t.Error("expected error for buffer too small")
	}
}

func TestObjectReader_VectorAlignment(t *testing.T) {
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)

	// Write some data to offset the vector
	writer.WriteString("padding")

	vector := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}
	writer.WriteVector(vector)

	// Create reader
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	header := reader.GetHeader()

	// Vector offset should be 16-byte aligned
	if header.VectorOffset%VectorAlignment != 0 {
		t.Errorf("vector offset %d is not %d-byte aligned",
			header.VectorOffset, VectorAlignment)
	}

	// Verify we can read the vector
	readVector, err := reader.GetVector()
	if err != nil {
		t.Fatalf("failed to read vector: %v", err)
	}

	if len(readVector) != len(vector) {
		t.Fatalf("expected vector length %d, got %d", len(vector), len(readVector))
	}
}

func TestObjectWriter_CompleteObject(t *testing.T) {
	writer := NewObjectWriter(2048)
	writer.WriteHeader(2)

	// Write properties
	offset1 := writer.WriteString("property1")
	offset2 := writer.WriteString("value123")

	// Write vector
	vector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	writer.WriteVector(vector)

	// Create reader
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	// Verify header
	header := reader.GetHeader()
	if header.PropertyCount != 2 {
		t.Errorf("expected property count 2, got %d", header.PropertyCount)
	}

	// Verify properties
	prop1, err := reader.GetString(offset1)
	if err != nil {
		t.Fatalf("failed to read property 1: %v", err)
	}
	if prop1 != "property1" {
		t.Errorf("expected 'property1', got '%s'", prop1)
	}

	prop2, err := reader.GetString(offset2)
	if err != nil {
		t.Fatalf("failed to read property 2: %v", err)
	}
	if prop2 != "value123" {
		t.Errorf("expected 'value123', got '%s'", prop2)
	}

	// Verify vector
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

func TestObjectReader_EmptyVector(t *testing.T) {
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)

	// Don't write a vector (VectorOffset remains 0)

	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	vector, err := reader.GetVector()
	if err != nil {
		t.Fatalf("unexpected error reading empty vector: %v", err)
	}

	if vector != nil {
		t.Errorf("expected nil vector, got %v", vector)
	}
}

func TestObjectReader_LargeVector(t *testing.T) {
	writer := NewObjectWriter(100000)
	writer.WriteHeader(0)

	// Create a large vector
	vector := make([]float32, 10000)
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}

	writer.WriteVector(vector)

	// Create reader
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	defer buf.Release()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, err := NewObjectReader(buf)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	readVector, err := reader.GetVector()
	if err != nil {
		t.Fatalf("failed to read vector: %v", err)
	}

	if len(readVector) != len(vector) {
		t.Fatalf("expected vector length %d, got %d", len(vector), len(readVector))
	}

	// Spot check some values
	for i := 0; i < 100; i++ {
		idx := i * 100
		if readVector[idx] != vector[idx] {
			t.Errorf("vector[%d]: expected %f, got %f", idx, vector[idx], readVector[idx])
		}
	}
}

func BenchmarkObjectWriter_WriteVector(b *testing.B) {
	vector := make([]float32, 768) // Typical embedding size
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer := NewObjectWriter(4096)
		writer.WriteHeader(0)
		writer.WriteVector(vector)
	}
}

func BenchmarkObjectReader_GetVector(b *testing.B) {
	// Create test object
	writer := NewObjectWriter(4096)
	writer.WriteHeader(0)

	vector := make([]float32, 768)
	for i := range vector {
		vector[i] = float32(i) * 0.1
	}
	writer.WriteVector(vector)

	// Create buffer
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, _ := NewObjectReader(buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.GetVector()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObjectWriter_WriteString(b *testing.B) {
	testStr := "This is a test string for benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer := NewObjectWriter(1024)
		writer.WriteHeader(0)
		writer.WriteString(testStr)
	}
}

func BenchmarkObjectReader_GetString(b *testing.B) {
	// Create test object
	writer := NewObjectWriter(1024)
	writer.WriteHeader(0)
	offset := writer.WriteString("This is a test string for benchmarking")

	// Create buffer
	buf := NewHeapBuffer(len(writer.Bytes()))
	buf.Retain()
	copy(buf.Bytes(), writer.Bytes())
	buf.SetLength(len(writer.Bytes()))

	reader, _ := NewObjectReader(buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.GetString(offset)
		if err != nil {
			b.Fatal(err)
		}
	}
}
