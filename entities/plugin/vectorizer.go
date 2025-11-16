package plugin

import "context"

// VectorizerPlugin extends Plugin interface with vectorization capabilities
type VectorizerPlugin interface {
	Plugin

	// VectorizeText converts a single text into a vector
	VectorizeText(ctx context.Context, text string) ([]float32, error)

	// VectorizeBatch converts multiple texts into vectors
	VectorizeBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimensionality of vectors produced by this plugin
	Dimensions() int

	// Model returns the model identifier being used
	Model() string
}

// TransformerPlugin extends Plugin interface with transformation capabilities
type TransformerPlugin interface {
	Plugin

	// Transform applies transformation to input data
	Transform(ctx context.Context, input interface{}) (interface{}, error)

	// TransformBatch applies transformation to multiple inputs
	TransformBatch(ctx context.Context, inputs []interface{}) ([]interface{}, error)
}

// RerankerPlugin extends Plugin interface with reranking capabilities
type RerankerPlugin interface {
	Plugin

	// Rerank reorders search results based on relevance
	Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error)
}

// RankResult represents a reranked document with its score
type RankResult struct {
	Index int
	Score float64
}

// GeneratorPlugin extends Plugin interface with generation capabilities
type GeneratorPlugin interface {
	Plugin

	// Generate produces text based on input
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateStream produces text as a stream
	GenerateStream(ctx context.Context, prompt string, callback func(string) error) error
}

// StoragePlugin extends Plugin interface with storage capabilities
type StoragePlugin interface {
	Plugin

	// Store saves data to storage
	Store(ctx context.Context, key string, data []byte) error

	// Retrieve gets data from storage
	Retrieve(ctx context.Context, key string) ([]byte, error)

	// Delete removes data from storage
	Delete(ctx context.Context, key string) error

	// List lists all keys with given prefix
	List(ctx context.Context, prefix string) ([]string, error)
}

// AuthPlugin extends Plugin interface with authentication capabilities
type AuthPlugin interface {
	Plugin

	// Authenticate validates credentials and returns user identity
	Authenticate(ctx context.Context, credentials map[string]string) (*Identity, error)

	// Authorize checks if user has permission for action
	Authorize(ctx context.Context, identity *Identity, resource, action string) (bool, error)
}

// Identity represents an authenticated user
type Identity struct {
	ID       string
	Username string
	Email    string
	Roles    []string
	Metadata map[string]string
}
