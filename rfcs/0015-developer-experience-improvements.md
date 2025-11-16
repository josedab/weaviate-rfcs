# RFC 0015: Developer Experience Improvements

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-16  
**Updated:** 2025-01-16  

---

## Summary

Enhance developer experience through improved SDKs, interactive CLI tools, local development mode, IDE integrations, better debugging tools, and comprehensive examples to reduce onboarding time by 50% and improve development velocity.

**Current state:** Basic SDKs, manual setup, limited debugging tools  
**Proposed state:** Rich developer ecosystem with modern tooling and excellent documentation

---

## Motivation

### Current Pain Points

1. **SDK limitations:**
   - Verbose API calls
   - No type safety in dynamic languages
   - Poor error messages
   - Limited IDE support

2. **Setup complexity:**
   - Manual configuration required
   - No quick-start mode
   - Sample data generation tedious
   - Local development cumbersome

3. **Debugging difficulties:**
   - Opaque errors
   - No query visualization
   - Limited introspection
   - Poor logging

### Developer Metrics

**Current onboarding:**
- Time to first query: 45 minutes
- Time to production: 2-3 weeks
- Support tickets: 150/month

**Target improvements:**
- 50% faster onboarding
- 60% fewer support tickets
- 2x development velocity

---

## Detailed Design

### Enhanced Python SDK

```python
from weaviate import Client
from weaviate.schema import Class, Property, VectorConfig
from weaviate.query import Query

# Type-safe schema builder
Article = Class(
    name="Article",
    description="News articles with embeddings",
    properties=[
        Property("title", "string", description="Article title"),
        Property("content", "text", description="Article body"),
        Property("publishedAt", "date", description="Publication date"),
        Property("author", "Author", description="Article author"),
    ],
    vectorConfig=VectorConfig(
        vectorizer="text2vec-openai",
        model="text-embedding-ada-002",
        dimensions=1536
    )
)

# Initialize with schema
client = Client("http://localhost:8080")
client.schema.create(Article)

# Fluent query API with type hints
from typing import List

def search_articles(query: str, limit: int = 10) -> List[Article]:
    return (
        client.query
        .get("Article", ["title", "content", "author { name }"])
        .with_near_text({"concepts": [query]})
        .with_limit(limit)
        .with_additional(["certainty", "distance"])
        .do()
    )

# Batch operations with progress
with client.batch.configure(batch_size=100) as batch:
    for article in articles:
        batch.add_data_object(
            data_object=article,
            class_name="Article"
        )
        # Automatic progress bar

# Error handling with context
try:
    client.data_object.create(article, "Article")
except weaviate.exceptions.ValidationError as e:
    print(f"Validation failed: {e.message}")
    print(f"Field: {e.field}")
    print(f"Expected: {e.expected_type}")
    print(f"Got: {e.actual_type}")
```

### Interactive CLI

```bash
$ weaviate-cli

Welcome to Weaviate CLI v2.0.0
Type 'help' for commands or 'tutorial' for interactive guide

weaviate> connect localhost:8080
✓ Connected to Weaviate v1.27.0
✓ Health: GREEN
✓ Nodes: 3
✓ Classes: 5

weaviate> show classes
┌──────────┬──────────┬────────────┬───────────┐
│ Class    │ Objects  │ Shards     │ Vectorizer│
├──────────┼──────────┼────────────┼───────────┤
│ Article  │ 10.2M    │ 3          │ openai    │
│ Author   │ 50k      │ 1          │ none      │
│ Category │ 120      │ 1          │ none      │
└──────────┴──────────┴────────────┴───────────┘

weaviate> describe Article
Class: Article
Description: News articles with embeddings
Vectorizer: text2vec-openai (1536D)

Properties:
  - title (string) - Article title
  - content (text) - Article body
  - publishedAt (date) - Publication date
  - author → Author - Article author

Indexes:
  - vector: HNSW (ef=100, M=32)
  - inverted: title, content
  - filterable: publishedAt

weaviate> query Article near:text["AI"] limit:5
Executing query... (12ms)

Results (5):
  1. "Introduction to AI" (certainty: 0.95)
     Published: 2025-01-15
     Author: Alice Johnson
     
  2. "Machine Learning Basics" (certainty: 0.89)
     Published: 2025-01-14
     Author: Bob Smith
     
  ... (3 more)

weaviate> explain query Article where:{category:"AI"}
Query Plan:
  1. IndexScan: inverted_category
     Cost: 1.2ms
     Rows: ~1200
     
  2. VectorIndex: HNSW
     Cost: 8.5ms
     Candidates: 1200
     
  3. Fetch: ObjectStore
     Cost: 2.1ms
     
Total estimated: 11.8ms
Actual: 12.3ms (104% of estimate)

weaviate> benchmark query Article near:text["AI"] --runs 100
Running benchmark (100 iterations)...
[████████████████████████] 100/100

Results:
  Mean: 12.4ms
  Median: 11.8ms
  p95: 18.2ms
  p99: 24.5ms
  Min: 10.1ms
  Max: 32.8ms
```

### Local Development Mode

```yaml
# weaviate.dev.yaml
development:
  enabled: true
  
  # In-memory storage (no persistence)
  storage:
    type: memory
    
  # Auto-reload schema
  schema:
    autoReload: true
    watchDirectory: ./schema
    
  # Mock vectorizers
  vectorizers:
    text2vec-openai:
      mock: true
      dimensions: 1536
      
  # Sample data generation
  fixtures:
    enabled: true
    directory: ./fixtures
    autoLoad: true
    
  # Hot reload
  hotReload:
    enabled: true
    watchPaths:
      - ./schema
      - ./fixtures
```

```bash
# Quick start development server
$ weaviate dev
Starting Weaviate in development mode...

✓ In-memory storage initialized
✓ Schema loaded from ./schema/
✓ Fixtures loaded: 1000 objects
✓ Mock vectorizers enabled
✓ Hot reload watching ./schema

Weaviate ready at http://localhost:8080
GraphQL Playground: http://localhost:8080/v1/graphql
Documentation: http://localhost:8080/docs

Press Ctrl+C to stop
```

### IDE Integration

**VSCode Extension:**
```json
// .vscode/settings.json
{
  "weaviate.endpoint": "http://localhost:8080",
  "weaviate.apiKey": "${env:WEAVIATE_API_KEY}",
  "weaviate.schemaPath": "./schema/*.yaml",
  "weaviate.validateOnSave": true,
  "weaviate.autoComplete": true
}
```

**Features:**
- Schema validation on save
- Autocomplete for GraphQL queries
- Inline documentation
- Query performance hints
- Error highlighting

### Request/Response Debugging

```go
// Debug middleware
type DebugMiddleware struct {
    logger *DebugLogger
}

func (m *DebugMiddleware) Handle(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Capture request
        reqID := generateRequestID()
        reqBody := m.captureBody(r.Body)
        
        // Log request
        m.logger.LogRequest(reqID, &RequestLog{
            Method:    r.Method,
            URL:       r.URL.String(),
            Headers:   r.Header,
            Body:      reqBody,
            Timestamp: time.Now(),
        })
        
        // Wrap response writer
        rw := &ResponseCapture{ResponseWriter: w}
        
        // Execute
        start := time.Now()
        next.ServeHTTP(rw, r)
        duration := time.Since(start)
        
        // Log response
        m.logger.LogResponse(reqID, &ResponseLog{
            StatusCode: rw.statusCode,
            Headers:    rw.Header(),
            Body:       rw.body.String(),
            Duration:   duration,
        })
        
        // Add debug headers
        w.Header().Set("X-Request-ID", reqID)
        w.Header().Set("X-Duration-Ms", fmt.Sprintf("%.2f", duration.Seconds()*1000))
    })
}
```

### Enhanced Error Messages

```go
// Structured error with context
type EnhancedError struct {
    Code       string
    Message    string
    Details    map[string]interface{}
    Suggestion string
    DocsLink   string
    Trace      []TraceFrame
}

func NewValidationError(field string, expected, got interface{}) *EnhancedError {
    return &EnhancedError{
        Code:    "VALIDATION_ERROR",
        Message: fmt.Sprintf("Invalid value for field '%s'", field),
        Details: map[string]interface{}{
            "field":    field,
            "expected": expected,
            "got":      got,
        },
        Suggestion: fmt.Sprintf(
            "Expected %T but got %T. Convert the value or check schema definition.",
            expected, got,
        ),
        DocsLink: "https://weaviate.io/docs/errors/validation",
    }
}

// Example error output
{
  "error": {
    "code": "VECTOR_DIMENSION_MISMATCH",
    "message": "Vector dimension does not match schema",
    "details": {
      "class": "Article",
      "expected_dimensions": 1536,
      "got_dimensions": 384,
      "vectorizer": "text2vec-openai"
    },
    "suggestion": "Check that you're using the correct model. The schema expects ada-002 (1536D) but the vector appears to be from a smaller model (384D).",
    "docs": "https://weaviate.io/docs/errors/vector-dimension-mismatch",
    "trace": [
      "VectorIndex.Add() at index.go:234",
      "Shard.AddObject() at shard.go:156",
      "BatchWriter.Write() at batch.go:89"
    ]
  }
}
```

---

## Implementation Plan

### Phase 1: SDK Enhancements (3 weeks)
- [ ] Python SDK improvements
- [ ] TypeScript SDK improvements
- [ ] Go SDK improvements
- [ ] Type generation

### Phase 2: CLI Tools (3 weeks)
- [ ] Interactive CLI
- [ ] Query builder
- [ ] Benchmark tools
- [ ] Schema validator

### Phase 3: Local Development (2 weeks)
- [ ] Development mode
- [ ] Hot reload
- [ ] Mock vectorizers
- [ ] Fixture generation

### Phase 4: IDE Integration (2 weeks)
- [ ] VSCode extension
- [ ] Schema validation
- [ ] Autocomplete
- [ ] Documentation

**Total: 10 weeks** (revised from 8 weeks)

---

## Success Criteria

- ✅ 50% faster onboarding
- ✅ 60% fewer support tickets
- ✅ 90%+ developer satisfaction
- ✅ IDE plugins for VSCode and IntelliJ
- ✅ <5 minute quick start

---

## Alternatives Considered

### Alternative 1: Focus on Documentation Only
**Verdict:** Insufficient, tooling is essential

### Alternative 2: External Tools
**Verdict:** Better to have official first-party tools

### Alternative 3: Community-Driven Tools
**Verdict:** Too slow, fragmented experience

---

## References

- Create React App: https://create-react-app.dev/
- Rails Generators: https://guides.rubyonrails.org/command_line.html
- Django Admin: https://docs.djangoproject.com/en/stable/ref/contrib/admin/

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-16*