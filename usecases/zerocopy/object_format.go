package zerocopy

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

const (
	// Magic number to identify zero-copy object format
	ObjectMagic uint32 = 0x5A45524F // "ZERO" in ASCII

	// Header size in bytes
	HeaderSize = 16

	// Alignment for vectors
	VectorAlignment = 16
)

/*
Object Layout (FlatBuffer-style):
  [Header][Properties][Vectors][References]

  Header (16 bytes):
    - Magic (4 bytes): 0x5A45524F
    - Version (2 bytes)
    - Flags (2 bytes)
    - Property count (4 bytes)
    - Vector offset (4 bytes)

  Properties (variable):
    - Offset table (4 bytes per property)
    - String pool
    - Inline values

  Vectors (16-byte aligned):
    - Dimension count (4 bytes)
    - Float32 array

  References (variable):
    - Reference count (4 bytes)
    - UUID array (16 bytes per UUID)
*/

// ObjectHeader represents the object header structure
type ObjectHeader struct {
	Magic         uint32
	Version       uint16
	Flags         uint16
	PropertyCount uint32
	VectorOffset  uint32
}

// ObjectWriter writes objects in zero-copy format
type ObjectWriter struct {
	buf    []byte
	offset int
}

// NewObjectWriter creates a new object writer
func NewObjectWriter(capacity int) *ObjectWriter {
	return &ObjectWriter{
		buf:    make([]byte, 0, capacity),
		offset: 0,
	}
}

// WriteHeader writes the object header
func (w *ObjectWriter) WriteHeader(propertyCount uint32) {
	header := ObjectHeader{
		Magic:         ObjectMagic,
		Version:       1,
		Flags:         0,
		PropertyCount: propertyCount,
		VectorOffset:  0, // Will be set later
	}

	// Ensure buffer has space for header
	w.ensureCapacity(HeaderSize)

	// Expand buffer if needed
	if len(w.buf) < HeaderSize {
		w.buf = w.buf[:HeaderSize]
	}

	// Write header fields
	binary.LittleEndian.PutUint32(w.buf[0:4], header.Magic)
	binary.LittleEndian.PutUint16(w.buf[4:6], header.Version)
	binary.LittleEndian.PutUint16(w.buf[6:8], header.Flags)
	binary.LittleEndian.PutUint32(w.buf[8:12], header.PropertyCount)
	binary.LittleEndian.PutUint32(w.buf[12:16], header.VectorOffset)

	w.offset = HeaderSize
}

// WriteVector writes a vector with proper alignment
func (w *ObjectWriter) WriteVector(vector []float32) {
	// Align to 16-byte boundary
	padding := (VectorAlignment - (w.offset % VectorAlignment)) % VectorAlignment
	totalSize := w.offset + padding + 4 + len(vector)*4
	w.ensureCapacity(totalSize)

	// Add padding
	for i := 0; i < padding; i++ {
		if w.offset >= len(w.buf) {
			w.buf = w.buf[:w.offset+1]
		}
		w.buf[w.offset] = 0
		w.offset++
	}

	// Update vector offset in header
	vectorOffset := uint32(w.offset)
	binary.LittleEndian.PutUint32(w.buf[12:16], vectorOffset)

	// Ensure buffer is large enough
	if len(w.buf) < totalSize {
		w.buf = w.buf[:totalSize]
	}

	// Write dimension count
	binary.LittleEndian.PutUint32(w.buf[w.offset:w.offset+4], uint32(len(vector)))
	w.offset += 4

	// Write vector data
	for i, v := range vector {
		offset := w.offset + i*4
		binary.LittleEndian.PutUint32(w.buf[offset:offset+4], math.Float32bits(v))
	}
	w.offset += len(vector) * 4
}

// WriteString writes a string value
func (w *ObjectWriter) WriteString(s string) int {
	offset := w.offset

	// Calculate total size needed
	totalSize := w.offset + 4 + len(s)
	w.ensureCapacity(totalSize)

	// Ensure buffer is large enough
	if len(w.buf) < totalSize {
		w.buf = w.buf[:totalSize]
	}

	// Write length
	binary.LittleEndian.PutUint32(w.buf[w.offset:w.offset+4], uint32(len(s)))
	w.offset += 4

	// Write string data
	copy(w.buf[w.offset:], []byte(s))
	w.offset += len(s)

	return offset
}

// Bytes returns the serialized object
func (w *ObjectWriter) Bytes() []byte {
	return w.buf[:w.offset]
}

func (w *ObjectWriter) ensureCapacity(needed int) {
	if cap(w.buf) < needed {
		newBuf := make([]byte, len(w.buf), needed*2)
		copy(newBuf, w.buf)
		w.buf = newBuf
	}
}

// ObjectReader reads objects in zero-copy format
type ObjectReader struct {
	buf Buffer
}

// NewObjectReader creates a new object reader
func NewObjectReader(buf Buffer) (*ObjectReader, error) {
	if buf.Len() < HeaderSize {
		return nil, fmt.Errorf("buffer too small for header: %d bytes", buf.Len())
	}

	// Verify magic number
	magic := binary.LittleEndian.Uint32(buf.Bytes()[0:4])
	if magic != ObjectMagic {
		return nil, fmt.Errorf("invalid magic number: 0x%08X (expected 0x%08X)", magic, ObjectMagic)
	}

	return &ObjectReader{buf: buf}, nil
}

// GetHeader reads the object header
func (r *ObjectReader) GetHeader() ObjectHeader {
	data := r.buf.Bytes()
	return ObjectHeader{
		Magic:         binary.LittleEndian.Uint32(data[0:4]),
		Version:       binary.LittleEndian.Uint16(data[4:6]),
		Flags:         binary.LittleEndian.Uint16(data[6:8]),
		PropertyCount: binary.LittleEndian.Uint32(data[8:12]),
		VectorOffset:  binary.LittleEndian.Uint32(data[12:16]),
	}
}

// GetVector returns the vector using zero-copy access
func (r *ObjectReader) GetVector() ([]float32, error) {
	header := r.GetHeader()
	if header.VectorOffset == 0 {
		return nil, nil
	}

	offset := int(header.VectorOffset)
	if offset+4 > r.buf.Len() {
		return nil, fmt.Errorf("vector offset out of bounds: %d", offset)
	}

	// Read dimension count
	data := r.buf.Bytes()
	dimCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	if offset+int(dimCount)*4 > r.buf.Len() {
		return nil, fmt.Errorf("vector data out of bounds")
	}

	// Create zero-copy slice backed by the buffer
	vectorData := data[offset : offset+int(dimCount)*4]

	// Convert byte slice to float32 slice (zero-copy)
	vector := unsafe.Slice(
		(*float32)(unsafe.Pointer(&vectorData[0])),
		dimCount,
	)

	return vector, nil
}

// GetString reads a string value from the given offset
func (r *ObjectReader) GetString(offset int) (string, error) {
	if offset+4 > r.buf.Len() {
		return "", fmt.Errorf("string offset out of bounds: %d", offset)
	}

	data := r.buf.Bytes()

	// Read length
	length := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	if offset+int(length) > r.buf.Len() {
		return "", fmt.Errorf("string data out of bounds")
	}

	// Return string (this creates a copy, but the data is read from buffer)
	return string(data[offset : offset+int(length)]), nil
}

// Buffer returns the underlying buffer
func (r *ObjectReader) Buffer() Buffer {
	return r.buf
}
