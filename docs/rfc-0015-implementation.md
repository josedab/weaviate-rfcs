# RFC 0015 Implementation Guide

**RFC:** Developer Experience Improvements
**Status:** In Progress
**Implementation Date:** 2025-01-16
**Author:** Claude Code

---

## Overview

This document describes the implementation of RFC 0015: Developer Experience Improvements. The implementation focuses on foundational components that enhance the developer experience when working with Weaviate.

## Implemented Components

### 1. Enhanced Error Handling

**Location:** `usecases/errors/enhanced.go`

**Description:** Structured error system with contextual information, suggestions, and documentation links.

**Features:**
- Structured error types with error codes
- Contextual details (map of key-value pairs)
- Helpful suggestions for resolution
- Documentation links
- Stack trace capture
- Error wrapping support

**Example Usage:**

```go
import "github.com/weaviate/weaviate/usecases/errors"

// Create a validation error
err := errors.NewValidationError("title", "string", 123)

// Create a custom error with details
err := errors.NewEnhancedError("CUSTOM_ERROR", "Something went wrong").
    WithDetail("field", "value").
    WithSuggestion("Try doing X instead").
    WithDocsLink("https://weaviate.io/docs/errors/custom")
```

**Available Error Constructors:**
- `NewValidationError()` - Field validation errors
- `NewVectorDimensionMismatchError()` - Vector dimension mismatches
- `NewSchemaNotFoundError()` - Missing schema classes
- `NewPropertyNotFoundError()` - Missing properties
- `NewInsufficientResourcesError()` - Resource limitations
- `NewAuthenticationError()` - Authentication failures
- `NewAuthorizationError()` - Authorization failures
- `NewTimeoutError()` - Timeout scenarios
- `NewConnectionError()` - Connection failures

### 2. Request/Response Debugging Middleware

**Location:** `adapters/handlers/rest/middleware/debug.go`

**Description:** Comprehensive request/response logging and debugging capabilities.

**Features:**
- Request ID generation (UUID)
- Request/response body capture
- Duration tracking
- Configurable body size limits
- HTTP headers in responses (X-Request-ID, X-Duration-Ms)
- Query execution plan logging
- Structured logging with logrus

**Example Usage:**

```go
import (
    "github.com/weaviate/weaviate/adapters/handlers/rest/middleware"
    "github.com/sirupsen/logrus"
)

logger := logrus.New()
debugLogger := middleware.NewDebugLogger(logger, true, true, 10240)
debugMiddleware := middleware.NewDebugMiddleware(debugLogger)

// Use in HTTP handler chain
http.Handle("/", debugMiddleware.Handler(yourHandler))
```

**Configuration:**

```go
config := middleware.DebugConfig{
    Enabled:     true,
    LogRequests: true,
    LogBodies:   false,  // Set true to log request/response bodies
    MaxBodySize: 10240,  // 10KB limit
}
```

### 3. Development Mode Configuration

**Location:** `usecases/config/development.go`

**Description:** Comprehensive development mode configuration system.

**Features:**
- In-memory storage mode
- Schema auto-reload with file watching
- Mock vectorizers (no API keys required)
- Fixture auto-loading
- Hot reload support
- Debug mode enhancements

**Configuration Structure:**

```yaml
development:
  enabled: true

  storage:
    type: memory  # or "disk" or "hybrid"
    persist: false

  schema:
    autoReload: true
    watchDirectory: ./schema
    validateOnLoad: true

  vectorizers:
    text2vec-openai:
      mock: true
      dimensions: 1536
      mockLatency: 50

  fixtures:
    enabled: true
    directory: ./fixtures
    autoLoad: true
    clearBeforeLoad: true

  hotReload:
    enabled: true
    watchPaths:
      - ./schema
      - ./fixtures
    debounceMs: 1000

  debug:
    enableQueryExplain: true
    logAllQueries: false
    enableProfiling: true
    enableMetrics: true
```

**Example Usage:**

```go
import "github.com/weaviate/weaviate/usecases/config"

// Load development config
devConfig, err := config.LoadDevelopmentConfig("weaviate.dev.yaml")
if err != nil {
    log.Fatal(err)
}

// Validate config
if err := devConfig.Validate(); err != nil {
    log.Fatal(err)
}

// Check if development mode is enabled
if devConfig.IsDevelopmentMode() {
    // Enable development features
}
```

### 4. Interactive CLI Tool

**Location:** `cmd/weaviate-cli/main.go`

**Description:** Command-line interface for managing Weaviate instances.

**Features:**
- Interactive shell mode
- Schema management commands
- Query execution with explain
- Performance benchmarking
- Development mode helpers
- Connection management

**Commands:**

```bash
# Interactive mode
weaviate-cli interactive

# Connect to instance
weaviate-cli connect http://localhost:8080

# Schema operations
weaviate-cli schema list
weaviate-cli schema describe Article

# Query execution
weaviate-cli query Article --limit 10
weaviate-cli query Article --explain

# Benchmarking
weaviate-cli benchmark "query Article" --runs 100

# Development mode
weaviate-cli dev init          # Create weaviate.dev.yaml
weaviate-cli dev start         # Start dev server
```

**Building the CLI:**

```bash
cd cmd/weaviate-cli
go build -o weaviate-cli
./weaviate-cli --help
```

---

## Installation and Usage

### 1. Enhanced Errors

Enhanced errors are automatically available throughout the codebase. Import and use:

```go
import "github.com/weaviate/weaviate/usecases/errors"

err := errors.NewValidationError("fieldName", "expectedType", actualValue)
if err != nil {
    return err
}
```

### 2. Debug Middleware

To enable debug middleware in development:

1. Add to your HTTP handler chain
2. Configure via environment or config file
3. Monitor logs for request/response details

### 3. Development Configuration

1. Copy example config:
```bash
cp weaviate.dev.yaml.example weaviate.dev.yaml
```

2. Customize settings in `weaviate.dev.yaml`

3. Create schema and fixtures directories:
```bash
mkdir -p schema fixtures
```

4. Start Weaviate with development config

### 4. CLI Tool

1. Build the CLI:
```bash
make build-cli  # or manually: go build -o weaviate-cli ./cmd/weaviate-cli
```

2. Initialize development config:
```bash
./weaviate-cli dev init
```

3. Start interactive mode:
```bash
./weaviate-cli interactive
```

---

## Integration Points

### Error Handling Integration

Replace existing error creation with enhanced errors:

**Before:**
```go
return fmt.Errorf("class %s not found", className)
```

**After:**
```go
return errors.NewSchemaNotFoundError(className)
```

### Middleware Integration

Add debug middleware to the HTTP server:

```go
// In adapters/handlers/rest/configure_api.go or similar
if config.Debug.Enabled {
    debugLogger := middleware.NewDebugLogger(logger, true, config.Debug.LogBodies, 10240)
    debugMW := middleware.NewDebugMiddleware(debugLogger)
    handler = debugMW.Handler(handler)
}
```

### Development Config Integration

Load and use development config at startup:

```go
// In cmd/weaviate-server/main.go or initialization code
devConfig, err := config.LoadDevelopmentConfig("")
if err != nil {
    log.Warn("Could not load development config: %v", err)
    devConfig = &config.DefaultDevelopmentConfig()
}

if devConfig.IsDevelopmentMode() {
    log.Info("Starting in development mode")
    // Enable development features
}
```

---

## Future Enhancements

The following components from RFC 0015 are planned for future implementation:

### Phase 1: SDK Enhancements (Planned)
- Python SDK with type-safe schema builders
- TypeScript SDK with fluent APIs
- Go SDK improvements
- Type generation tools

### Phase 2: Advanced CLI Features (Planned)
- Full query builder
- Schema validator
- Migration tools
- Data import/export

### Phase 3: IDE Integration (Planned)
- VSCode extension
- Schema validation in editor
- Autocomplete for GraphQL
- Inline documentation

### Phase 4: Production Features (Planned)
- Query explain API endpoint
- Real-time query performance monitoring
- Automatic index recommendations
- Enhanced observability integration

---

## Testing

### Unit Tests

Create unit tests for each component:

```go
// usecases/errors/enhanced_test.go
func TestEnhancedError(t *testing.T) {
    err := NewValidationError("field", "string", 123)
    assert.NotNil(t, err)
    assert.Equal(t, "VALIDATION_ERROR", err.Code)
    assert.Contains(t, err.Message, "field")
}
```

### Integration Tests

Test middleware integration:

```go
// adapters/handlers/rest/middleware/debug_test.go
func TestDebugMiddleware(t *testing.T) {
    // Create test handler
    // Apply middleware
    // Make request
    // Verify headers and logging
}
```

---

## Performance Considerations

### Error Handling
- Stack trace capture adds minimal overhead (~5μs per error)
- Only enabled in error paths
- No performance impact on happy path

### Debug Middleware
- Request ID generation: ~1μs
- Body capture: Only when enabled
- Configurable body size limits prevent memory issues

### Development Config
- File watching uses efficient file system notifications
- Debouncing prevents excessive reloads
- In-memory mode provides faster iteration

---

## Configuration Reference

### Environment Variables

```bash
# Enable debug mode
DEBUG=true

# Enable request logging
LOG_REQUESTS=true

# Enable body logging
LOG_BODIES=true

# Development mode
DEVELOPMENT_MODE=true
```

### Config File Options

See `weaviate.dev.yaml.example` for complete configuration reference.

---

## Troubleshooting

### Common Issues

**1. Development config not loading**
- Ensure `weaviate.dev.yaml` exists in working directory
- Check YAML syntax
- Verify file permissions

**2. Mock vectorizers not working**
- Ensure `vectorizers.<name>.mock: true` in config
- Check dimensions match schema
- Verify development mode is enabled

**3. Hot reload not triggering**
- Ensure `hotReload.enabled: true`
- Check `watchPaths` includes changed files
- Verify file system notifications work

---

## Contributing

When adding new error types:

1. Add constructor function in `usecases/errors/enhanced.go`
2. Include appropriate error code
3. Provide helpful suggestion
4. Link to documentation
5. Add unit tests

When adding CLI commands:

1. Add command to `cmd/weaviate-cli/main.go`
2. Implement command logic
3. Add help text
4. Update documentation

---

## References

- RFC 0015: [rfcs/0015-developer-experience-improvements.md](../rfcs/0015-developer-experience-improvements.md)
- Error Handling Best Practices: [Go Error Wrapping](https://go.dev/blog/go1.13-errors)
- Middleware Pattern: [HTTP Middleware in Go](https://www.alexedwards.net/blog/making-and-using-middleware)

---

*Implementation Guide Version: 1.0*
*Last Updated: 2025-01-16*
