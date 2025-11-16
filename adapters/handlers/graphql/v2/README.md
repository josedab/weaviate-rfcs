# GraphQL API v2

This package implements the GraphQL API v2 for Weaviate, as described in RFC 0007.

## Overview

The v2 API provides a redesigned GraphQL interface with:

- **Standard GraphQL patterns** - Following Relay cursor connections specification
- **Improved performance** - DataLoader pattern for batching, query complexity analysis
- **Better developer experience** - Intuitive naming, better IDE support
- **Backward compatibility** - Automatic translation from v1 to v2

## Key Features

### 1. Connection-based Pagination

All list queries return a `Connection` type following the Relay specification:

```graphql
{
  articles(limit: 10) {
    edges {
      node {
        id
        title
      }
      cursor
    }
    pageInfo {
      hasNextPage
      hasPreviousPage
      startCursor
      endCursor
    }
    totalCount
  }
}
```

### 2. Vector Search

Dedicated search endpoints with integrated scoring:

```graphql
{
  searchArticles(
    near: {
      vector: [0.1, 0.2, 0.3]
    }
    limit: 10
  ) {
    edges {
      node {
        title
      }
      distance
      score
    }
  }
}
```

### 3. DataLoader Pattern

Automatic batching and caching of reference queries to prevent N+1 problems:

- Batches multiple ID lookups into single query
- Caches results within request context
- Configurable batch size and wait time

### 4. Query Complexity Analysis

Prevents expensive queries from overloading the system:

- Calculates complexity before execution
- Configurable maximum complexity limit
- Considers list multipliers (limit, first, last)

### 5. Standardized Errors

Consistent error format with helpful metadata:

```json
{
  "errors": [{
    "message": "Vector dimension mismatch",
    "extensions": {
      "code": "VECTOR_DIMENSION_ERROR",
      "expected": 384,
      "received": 512,
      "timestamp": "2025-01-16T10:30:00Z",
      "traceId": "abc123"
    },
    "path": ["searchArticles", 0, "node"]
  }]
}
```

## Architecture

### Package Structure

```
v2/
├── handler.go              # Main HTTP handler
├── schema_builder.go       # Builds GraphQL schema from Weaviate schema
├── types/
│   ├── connection.go       # Connection, Edge, PageInfo types
│   └── inputs.go           # Input types (filters, sorts, etc.)
├── resolvers/
│   └── resolver.go         # GraphQL field resolvers
├── dataloader/
│   └── loader.go           # DataLoader implementation
├── complexity/
│   └── analyzer.go         # Query complexity analyzer
├── translation/
│   └── translator.go       # v1 to v2 query translation
├── middleware/
│   ├── errors.go           # Standardized error handling
│   └── parallel.go         # Parallel execution utilities
└── README.md               # This file
```

### Data Flow

```
HTTP Request
    ↓
Handler.ServeHTTP
    ↓
[Translation Layer] (if v1 query)
    ↓
Complexity Analysis
    ↓
GraphQL Execution
    ↓
Resolvers
    ↓
[DataLoader] (for batching)
    ↓
Repository
    ↓
Response
```

## Usage

### Creating a Handler

```go
import (
    v2 "github.com/weaviate/weaviate/adapters/handlers/graphql/v2"
    "github.com/weaviate/weaviate/adapters/handlers/graphql/v2/resolvers"
)

// Create repository (implements resolvers.Repository interface)
repo := NewMyRepository()

// Create handler
handler, err := v2.NewHandler(weaviateSchema, repo, v2.Config{
    MaxComplexity:  10000,
    EnableV1Compat: true,
    Logger:         logger,
})

// Serve HTTP
http.Handle("/v2/graphql", handler)
```

### Implementing a Repository

The repository interface provides data access:

```go
type Repository interface {
    GetByID(ctx context.Context, className string, id string) (interface{}, error)
    GetByIDs(ctx context.Context, className string, ids []string) ([]interface{}, []error)
    Search(ctx context.Context, params SearchParams) (*SearchResults, error)
    Aggregate(ctx context.Context, params AggregateParams) (interface{}, error)
}
```

### Versioned Routing

Use the versioned router for dual v1/v2 support:

```go
import "github.com/weaviate/weaviate/adapters/handlers/graphql"

config := graphql.DefaultVersionedRouterConfig()
config.V2Default = true // Make v2 the default

router, err := graphql.NewVersionedRouter(
    weaviateSchema,
    v1Traverser,
    v2Repository,
    config,
)

http.Handle("/graphql", router)
```

## Query Examples

### Simple Fetch

```graphql
{
  article(id: "abc-123") {
    id
    title
    content
  }
}
```

### Filtered List

```graphql
{
  articles(
    where: {
      field: "publishedAt"
      operator: GREATER_THAN
      value: "2025-01-01"
    }
    limit: 20
    sort: [{
      field: "publishedAt"
      direction: DESC
    }]
  ) {
    edges {
      node {
        id
        title
      }
    }
    totalCount
  }
}
```

### Vector Search

```graphql
{
  searchArticles(
    near: {
      text: "artificial intelligence"
      certainty: 0.7
    }
    limit: 10
  ) {
    edges {
      node {
        id
        title
      }
      score
      distance
    }
  }
}
```

### Hybrid Search

```graphql
{
  searchArticles(
    hybrid: {
      query: "machine learning"
      alpha: 0.5
    }
    limit: 10
  ) {
    edges {
      node {
        id
        title
      }
      score
    }
  }
}
```

## Performance

### DataLoader Batching

The DataLoader automatically batches requests:

```go
// Without DataLoader: 100 queries
for _, article := range articles {
    author := getAuthor(article.authorId) // 100 DB queries
}

// With DataLoader: 1 query
for _, article := range articles {
    author := loader.Load(article.authorId) // Batched into 1 query
}
```

### Complexity Analysis

Query complexity is calculated as:

```
complexity = base_cost * multiplier * nested_complexity

Example:
articles(limit: 100) {     # 2 * 100 = 200
  edges {                  # 1 * 100 = 100
    node {                 # 1 * 100 = 100
      author {             # 3 * 100 = 300 (reference cost)
        name               # 0
      }
    }
  }
}
Total: 700
```

## Migration from v1

### Automatic Translation

Enable v1 compatibility mode for automatic translation:

```go
config := v2.Config{
    EnableV1Compat: true,
}
```

Clients can specify API version via header:

```
X-API-Version: 1
```

### Manual Migration

Use the translator directly:

```go
translator := translation.NewTranslator(classNames)
v2Query, err := translator.TranslateV1ToV2(v1Query)
```

### Common Patterns

| v1 Pattern | v2 Pattern |
|------------|------------|
| `Get { Article { } }` | `articles { edges { node { } } }` |
| `_additional { id }` | `_metadata { }` or `id` (top-level) |
| `nearVector: { vector: [...] }` | `near: { vector: [...] }` |
| `Article(limit: 10)` | `articles(limit: 10)` |

## Testing

Run tests:

```bash
go test ./adapters/handlers/graphql/v2/...
```

Run with coverage:

```bash
go test -cover ./adapters/handlers/graphql/v2/...
```

## Future Enhancements

- [ ] GraphQL subscriptions for real-time updates
- [ ] Persisted queries for performance
- [ ] Apollo Federation support
- [ ] Custom scalar types
- [ ] Batch mutations
- [ ] File uploads

## References

- [RFC 0007: GraphQL API v2 Design](../../../../rfcs/0007-graphql-api-v2.md)
- [GraphQL Best Practices](https://graphql.org/learn/best-practices/)
- [Relay Cursor Connections](https://relay.dev/graphql/connections.htm)
- [DataLoader Pattern](https://github.com/graphql/dataloader)
