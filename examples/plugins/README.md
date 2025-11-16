# Weaviate Plugin Examples

This directory contains example plugin implementations demonstrating the Weaviate Plugin Architecture.

## Example Plugins

### custom-vectorizer

An example vectorizer plugin demonstrating:
- Plugin manifest structure (plugin.yaml)
- WASM runtime configuration
- Resource limits and capabilities
- Configuration schema with required and optional fields

## Plugin Structure

Each plugin directory should contain:

```
plugin-name/
├── plugin.yaml           # Plugin manifest
├── src/                  # Source code (Rust, Go, Python, etc.)
├── build.sh             # Build script
└── README.md            # Plugin documentation
```

## Plugin Manifest Format

```yaml
apiVersion: weaviate.io/v1
kind: Plugin
metadata:
  name: plugin-name
  version: 1.0.0
  author: "Your Name"
  description: "Plugin description"

spec:
  type: vectorizer  # or transformer, reranker, generator, storage, auth
  runtime: wasm     # or grpc, native
  binary: ./plugin-binary.wasm

  dependencies:
    - name: dependency-name
      version: "^1.0.0"

  resources:
    memory: "2Gi"
    cpu: "1000m"

  capabilities:
    dimensions: 768  # for vectorizers
    maxBatchSize: 100

  config:
    paramName:
      type: string
      required: true
      secret: false
      default: "default-value"
```

## Runtime Types

### WASM (WebAssembly)
- **Pros**: Sandboxed, secure, portable
- **Cons**: ~50% performance overhead for single operations
- **Best for**: Untrusted plugins, community plugins
- **Languages**: Rust, C/C++ (via emscripten), AssemblyScript

### gRPC
- **Pros**: Language-agnostic, mature ecosystem
- **Cons**: Network overhead, process management
- **Best for**: Existing services, Python/Node.js plugins
- **Languages**: Any language with gRPC support

### Native
- **Pros**: Best performance, full Go integration
- **Cons**: No sandboxing, security risks, same process
- **Best for**: Trusted internal plugins
- **Languages**: Go only (.so shared libraries)

## Plugin Types

### Vectorizer
Converts text/images to vectors:
```go
VectorizeText(ctx, text) -> []float32
VectorizeBatch(ctx, texts) -> [][]float32
```

### Transformer
Transforms data:
```go
Transform(ctx, input) -> output
TransformBatch(ctx, inputs) -> []output
```

### Reranker
Reorders search results:
```go
Rerank(ctx, query, documents) -> []RankResult
```

### Generator
Generates text (LLMs):
```go
Generate(ctx, prompt) -> string
GenerateStream(ctx, prompt, callback) -> error
```

### Storage
Custom storage backends:
```go
Store(ctx, key, data) -> error
Retrieve(ctx, key) -> []byte
```

### Auth
Custom authentication:
```go
Authenticate(ctx, credentials) -> *Identity
Authorize(ctx, identity, resource, action) -> bool
```

## Loading Plugins

### Using Plugin Manager

```go
import "github.com/weaviate/weaviate/usecases/plugin"

// Create manager
manager := plugin.NewManager()

// Load single plugin
err := manager.Load(ctx, "./plugins/custom-vectorizer/plugin.yaml")

// Load all plugins from directory
err := manager.LoadDirectory(ctx, "./plugins")

// Get and use plugin
vectorizer, err := manager.GetVectorizer("custom-embedder")
vector, err := vectorizer.VectorizeText(ctx, "hello world")

// Hot-reload plugin
err := manager.Reload(ctx, "custom-embedder", "./plugins/custom-vectorizer-v2/plugin.yaml")
```

## Building Plugins

### WASM Plugin (Rust)

```rust
use weaviate_plugin_sdk::*;

#[plugin_export]
pub struct CustomVectorizer {
    model: Model,
}

#[plugin_impl]
impl VectorizerPlugin for CustomVectorizer {
    fn init(&mut self, config: Config) -> Result<()> {
        let model_path = config.get_string("modelPath")?;
        self.model = Model::load(model_path)?;
        Ok(())
    }

    fn vectorize_text(&self, text: &str) -> Result<Vec<f32>> {
        self.model.encode(text)
    }
}
```

Build:
```bash
cargo build --release --target wasm32-wasi
```

### gRPC Plugin (Python)

```python
from weaviate_plugin_sdk import VectorizerPlugin
import grpc

class CustomVectorizer(VectorizerPlugin):
    def __init__(self):
        self.model = load_model()

    def VectorizeText(self, request, context):
        vector = self.model.encode(request.text)
        return VectorizeResponse(vector=vector.tolist())
```

Run:
```bash
python plugin_server.py
```

## Security Considerations

1. **WASM plugins**: Sandboxed by default, safe for untrusted code
2. **gRPC plugins**: Run in separate process, network isolation possible
3. **Native plugins**: No sandboxing, only use trusted code
4. **Resource limits**: Set appropriate memory and CPU limits
5. **Secret management**: Use `secret: true` for sensitive config values

## Performance Guidelines

- **WASM**: 1.5-2x overhead for single operations, ~15% for batch operations
- **gRPC**: 2-5ms latency overhead for local connections
- **Native**: Minimal overhead, near-native performance

For best performance:
- Use batch operations when possible
- Set appropriate batch sizes in capabilities
- Monitor resource usage and adjust limits
- Consider native runtime for performance-critical plugins

## Testing Plugins

```go
func TestCustomVectorizer(t *testing.T) {
    manager := plugin.NewManager()
    err := manager.Load(context.Background(), "./plugin.yaml")
    require.NoError(t, err)

    vectorizer, err := manager.GetVectorizer("custom-embedder")
    require.NoError(t, err)

    vector, err := vectorizer.VectorizeText(context.Background(), "test")
    require.NoError(t, err)
    assert.Equal(t, 768, len(vector))
}
```

## Resources

- RFC 0009: Plugin Architecture
- Plugin SDK Documentation
- Example Plugins Repository
- Community Plugin Registry
