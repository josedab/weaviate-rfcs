# GraphQL API v2 Implementation

This document describes the implementation of RFC 0007: GraphQL API v2 Design.

## Implementation Status

### Phase 1: Beta Release ✅ COMPLETED

- ✅ Core v2 schema types (Connection, Edge, PageInfo)
- ✅ Schema builder for dynamic type generation
- ✅ Resolver implementation with standard patterns
- ✅ DataLoader pattern for batching and caching
- ✅ Query complexity analyzer
- ✅ Standardized error handling
- ✅ v1 to v2 query translation layer
- ✅ Versioned endpoint routing
- ✅ Parallel execution utilities
- ✅ Basic test coverage
- ✅ Documentation

### Phase 2: Client Updates ⏳ PENDING

- [ ] Update Python client
- [ ] Update JavaScript/TypeScript client
- [ ] Update Go client
- [ ] Update Java client

### Phase 3: Migration Tools ⏳ PENDING

- [ ] Query migration CLI tool
- [ ] Automated code migration scripts
- [ ] Compatibility testing suite

### Phase 4: General Availability ⏳ PENDING

- [ ] Comprehensive performance testing
- [ ] Security audit
- [ ] Production deployment guide
- [ ] Migration documentation
- [ ] Deprecation timeline for v1

## What Was Implemented

### 1. Core Types (`types/`)

**connection.go**
- `PageInfo` - Pagination metadata following Relay spec
- `Edge` - Individual item with cursor and search metadata
- `Connection` - Paginated collection with edges and page info
- `VectorData` - Vector embedding representation
- `ObjectMetadata` - Object metadata (replaces v1's `_additional`)

**inputs.go**
- `VectorInput` - Vector search parameters
- `HybridInput` - Hybrid search (vector + keyword) parameters
- `FilterInput` - Filtering with nested AND/OR support
- `SortInput` - Sorting configuration
- Operator enum for filter operations

### 2. DataLoader (`dataloader/`)

**loader.go**
- Batching: Collects multiple requests and executes them together
- Caching: Stores results to prevent duplicate requests
- Configurable batch size and wait time
- Context-aware execution
- Thread-safe implementation

### 3. Query Complexity (`complexity/`)

**analyzer.go**
- AST-based complexity calculation
- Considers field costs and list multipliers
- Configurable maximum complexity
- Default cost map for common operations
- Validation before query execution

### 4. Resolvers (`resolvers/`)

**resolver.go**
- `ResolveObject` - Single object by ID
- `ResolveObjects` - List with filtering and pagination
- `ResolveSearch` - Vector/hybrid search
- `ResolveAggregate` - Aggregation queries
- Filter, sort, and pagination parsing
- Connection building with metadata

### 5. Translation Layer (`translation/`)

**translator.go**
- Converts v1 queries to v2 format
- Removes `Get { }` wrapper
- Converts class names to lowercase plural
- Wraps results in `edges { node { } }`
- Converts `_additional` to `_metadata`
- Bidirectional translation for validation

### 6. Middleware (`middleware/`)

**errors.go**
- Standardized error codes
- Rich error extensions with metadata
- Helper functions for common errors
- Timestamp and trace ID support

**parallel.go**
- Parallel task execution with concurrency limits
- Error group for coordinated execution
- Batch processing support
- Map and slice result types

### 7. Schema Builder (`schema_builder.go`)

- Dynamically builds GraphQL schema from Weaviate schema
- Creates object types for each class
- Generates connection and edge types
- Builds root Query type with standard patterns
- Maps Weaviate types to GraphQL types
- Handles cross-references

### 8. Handler (`handler.go`)

- HTTP request handling
- Request parsing and validation
- Complexity analysis before execution
- v1 compatibility mode with header detection
- Trace ID propagation
- Error response formatting

### 9. Versioned Router (`router.go`)

- Routes requests to v1 or v2 based on path
- Deprecation warnings for v1 API
- Version detection via headers
- Configurable default version
- Rate limiting middleware
- Version info endpoint

## Architecture Decisions

### 1. Repository Pattern

We chose to define a `Repository` interface rather than coupling directly to Weaviate's internal types. This provides:

- **Testability**: Easy to mock for unit tests
- **Flexibility**: Can swap implementations
- **Separation of concerns**: Clear boundary between GraphQL and data layer

### 2. DataLoader Implementation

Custom implementation rather than using a third-party library because:

- **Control**: Full control over batching strategy
- **Integration**: Better integration with Weaviate's context
- **Simplicity**: No external dependencies
- **Performance**: Optimized for our use case

### 3. AST-Based Complexity

Using GraphQL AST for complexity analysis provides:

- **Accuracy**: Precise calculation of query cost
- **Flexibility**: Easy to add custom cost rules
- **Performance**: Analyzed before execution
- **Security**: Prevents resource exhaustion

### 4. Translation Layer

Regex-based translation for v1 compatibility:

- **Simplicity**: Easier to implement and understand
- **Performance**: Fast for common patterns
- **Limitations**: May not handle all edge cases
- **Future**: Could be replaced with AST-based translation

## Performance Characteristics

### DataLoader Batching

**Without DataLoader:**
```
Request 1: getAuthor(id1) → DB query
Request 2: getAuthor(id2) → DB query
Request 3: getAuthor(id3) → DB query
...
Total: N DB queries
```

**With DataLoader:**
```
Requests 1-N collected
Batch execution: getAuthors([id1, id2, id3, ...]) → 1 DB query
Total: 1 DB query
```

**Improvement: 79% faster for reference queries** (as per RFC benchmarks)

### Complexity Analysis

**Cost calculation:**
```
Base cost:     Field-specific cost
Multiplier:    From limit/first/last arguments
Nested cost:   Recursive calculation for sub-selections

Example:
articles(limit: 100) {           # 2 * 100 = 200
  edges {                        # 1 * 100 = 100
    node {                       # 1 * 100 = 100
      title                      # 0 * 100 = 0
      author {                   # 3 * 100 = 300
        name                     # 0 * 100 = 0
      }
    }
  }
}
Total complexity: 700
```

**Protection:**
- Prevents expensive queries before execution
- Configurable limits per deployment
- Clear error messages with actual vs. allowed complexity

## Testing Strategy

### Unit Tests

- Type constructors and helper functions
- DataLoader batching and caching
- Complexity calculation
- Filter and sort parsing
- Error formatting

### Integration Tests

- End-to-end query execution
- v1 to v2 translation
- Versioned routing
- Error handling

### Performance Tests

- DataLoader batching efficiency
- Complexity analysis overhead
- Concurrent request handling
- Memory usage under load

## Migration Guide

### For Weaviate Developers

1. **Update imports:**
   ```go
   import v2 "github.com/weaviate/weaviate/adapters/handlers/graphql/v2"
   ```

2. **Implement Repository interface:**
   ```go
   type MyRepository struct { ... }

   func (r *MyRepository) GetByID(...) { ... }
   func (r *MyRepository) GetByIDs(...) { ... }
   func (r *MyRepository) Search(...) { ... }
   func (r *MyRepository) Aggregate(...) { ... }
   ```

3. **Create handler:**
   ```go
   handler, err := v2.NewHandler(schema, repo, v2.Config{
       MaxComplexity: 10000,
   })
   ```

4. **Set up routing:**
   ```go
   http.Handle("/v2/graphql", handler)
   ```

### For Client Developers

1. **Update endpoint:**
   ```
   Old: POST /v1/graphql
   New: POST /v2/graphql
   ```

2. **Update query structure:**
   ```graphql
   # Old
   { Get { Article { title } } }

   # New
   { articles { edges { node { title } } } }
   ```

3. **Update field names:**
   ```graphql
   # Old
   _additional { id }

   # New
   id (top-level) or _metadata { }
   ```

## Future Work

### Short Term (Next 3 months)

- Complete client library updates
- Build migration CLI tool
- Comprehensive benchmarking
- Security audit

### Medium Term (3-6 months)

- GraphQL subscriptions for real-time updates
- Persisted queries for performance
- Enhanced aggregation types
- Batch mutation support

### Long Term (6-12 months)

- Apollo Federation support
- Custom scalar types
- Advanced caching strategies
- Query optimization hints

## Known Limitations

1. **Translation layer** - Regex-based, may not handle all v1 query patterns
2. **Aggregations** - Simplified implementation, needs full aggregation types
3. **Mutations** - Not yet implemented in v2
4. **Subscriptions** - Planned for future release
5. **Custom scalars** - Using strings for DateTime, UUID, etc.

## Dependencies

- `github.com/tailor-inc/graphql` - GraphQL implementation
- `golang.org/x/sync/errgroup` - Parallel execution
- `github.com/sirupsen/logrus` - Logging

## Contributing

To contribute to the v2 implementation:

1. Review the RFC: `rfcs/0007-graphql-api-v2.md`
2. Check existing issues and PRs
3. Write tests for new features
4. Update documentation
5. Follow Go best practices
6. Ensure backward compatibility

## Support

For questions or issues:

- GitHub Issues: https://github.com/weaviate/weaviate/issues
- Slack: #contributors channel
- Email: hello@weaviate.io

---

**Implementation Date:** 2025-01-16
**Author:** Jose David Baena (@josedab)
**RFC:** 0007-graphql-api-v2.md
**Status:** Phase 1 Complete (Beta Release)
