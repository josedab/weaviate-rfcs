package zerocopy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// ZeroCopyHandler handles HTTP requests with zero-copy semantics
type ZeroCopyHandler struct {
	store      *ObjectStore
	vectorIdx  *VectorIndex
	bufferPool *BufferPool
}

// NewZeroCopyHandler creates a new zero-copy HTTP handler
func NewZeroCopyHandler(vectorDim int) *ZeroCopyHandler {
	return &ZeroCopyHandler{
		store:      NewObjectStore(),
		vectorIdx:  NewVectorIndex(vectorDim),
		bufferPool: NewBufferPool(),
	}
}

// ObjectRequest represents an object creation request
type ObjectRequest struct {
	ID         string             `json:"id"`
	Properties map[string]string  `json:"properties"`
	Vector     []float32          `json:"vector"`
}

// ObjectResponse represents an object response
type ObjectResponse struct {
	ID         string             `json:"id"`
	Properties map[string]string  `json:"properties,omitempty"`
	Vector     []float32          `json:"vector,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Vector []float32 `json:"vector"`
	Limit  int       `json:"limit"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID       string  `json:"id"`
	Distance float32 `json:"distance"`
}

// HandleCreate handles object creation with zero-copy
func (h *ZeroCopyHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	// Get buffer from pool
	buf := h.bufferPool.Get(r.ContentLength)
	defer buf.Release()

	// Read directly into buffer (no intermediate allocation)
	if _, err := io.ReadFull(r.Body, buf.Bytes()); err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
		return
	}

	// Parse request
	var req ObjectRequest
	if err := json.Unmarshal(buf.Bytes(), &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Parse ID
	id, err := uuid.Parse(req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid UUID: %v", err), http.StatusBadRequest)
		return
	}

	// Create object in zero-copy format
	writer := NewObjectWriter(4096)
	writer.WriteHeader(uint32(len(req.Properties)))

	// Write properties
	for key, value := range req.Properties {
		writer.WriteString(key)
		writer.WriteString(value)
	}

	// Write vector
	if len(req.Vector) > 0 {
		writer.WriteVector(req.Vector)
	}

	// Store object
	objBuf := h.bufferPool.Get(int64(len(writer.Bytes())))
	defer objBuf.Release()
	copy(objBuf.Bytes(), writer.Bytes())

	if err := h.store.Put(id, objBuf); err != nil {
		http.Error(w, fmt.Sprintf("failed to store object: %v", err), http.StatusInternalServerError)
		return
	}

	// Add to vector index
	if len(req.Vector) > 0 {
		if err := h.vectorIdx.AddVector(id, req.Vector); err != nil {
			http.Error(w, fmt.Sprintf("failed to index vector: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id.String()})
}

// HandleGet retrieves an object with zero-copy
func (h *ZeroCopyHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path (simplified)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid UUID: %v", err), http.StatusBadRequest)
		return
	}

	// Get object reader (zero-copy)
	reader, err := h.store.GetReader(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("object not found: %v", err), http.StatusNotFound)
		return
	}
	defer reader.Buffer().Release()

	// Read vector (zero-copy)
	vector, err := reader.GetVector()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read vector: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	resp := ObjectResponse{
		ID:     id.String(),
		Vector: vector,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSearch performs vector search with zero-copy
func (h *ZeroCopyHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	// Get buffer from pool
	buf := h.bufferPool.Get(r.ContentLength)
	defer buf.Release()

	// Read directly into buffer
	if _, err := io.ReadFull(r.Body, buf.Bytes()); err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
		return
	}

	// Parse request
	var req SearchRequest
	if err := json.Unmarshal(buf.Bytes(), &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	// Perform search (zero-copy vector access)
	ids, distances, err := h.vectorIdx.Search(req.Vector, req.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	results := make([]SearchResult, len(ids))
	for i, id := range ids {
		results[i] = SearchResult{
			ID:       id.String(),
			Distance: distances[i],
		}
	}

	resp := SearchResponse{Results: results}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleDelete deletes an object
func (h *ZeroCopyHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid UUID: %v", err), http.StatusBadRequest)
		return
	}

	// Delete from store
	if err := h.store.Delete(id); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete: %v", err), http.StatusNotFound)
		return
	}

	// Delete from index
	h.vectorIdx.Delete(id)

	w.WriteHeader(http.StatusNoContent)
}

// Stats returns handler statistics
func (h *ZeroCopyHandler) Stats() map[string]interface{} {
	return map[string]interface{}{
		"object_store":  h.store.Stats(),
		"vector_index":  map[string]int{
			"size":      h.vectorIdx.Size(),
			"dimension": h.vectorIdx.Dimension(),
		},
		"buffer_pool": h.bufferPool.Stats(),
	}
}
