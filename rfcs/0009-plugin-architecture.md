# RFC 0009: Plugin Architecture for Custom Modules

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-16  
**Updated:** 2025-01-16  

---

## Summary

Introduce a standardized plugin architecture enabling developers to extend Weaviate functionality through custom modules without modifying core codebase, supporting vectorizers, transformers, and custom processing pipelines.

**Current state:** Hard-coded module integration; requires core modifications  
**Proposed state:** Dynamic plugin system with hot-reload, sandboxing, and marketplace distribution

---

## Motivation

### Current Limitations

1. **Hard-coded modules:**
   - All modules compiled into binary
   - No custom modules without forking
   - 80+ modules in codebase increase complexity

2. **Development friction:**
   - Core code changes required
   - Long PR review cycles
   - Deployment requires full rebuild

3. **Limited extensibility:**
   - Cannot add custom vectorizers easily
   - No support for proprietary models
   - Missing domain-specific processors

### Use Cases

**Enterprise customers:**
- Proprietary embedding models
- Custom data transformations
- Industry-specific processors

**Research teams:**
- Experimental vectorizers
- Novel search algorithms
- A/B testing new approaches

**Community developers:**
- Domain-specific modules
- Integration with niche services
- Specialized preprocessing

---

## Detailed Design

### Plugin Architecture

```go
// Plugin interface
type Plugin interface {
    // Metadata
    Name() string
    Version() string
    Type() PluginType
    
    // Lifecycle
    Init(ctx context.Context, config Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // Health
    Health() HealthStatus
}

// Plugin types
type PluginType string

const (
    PluginTypeVectorizer   PluginType = "vectorizer"
    PluginTypeTransformer  PluginType = "transformer"
    PluginTypeReranker     PluginType = "reranker"
    PluginTypeGenerator    PluginType = "generator"
    PluginTypeStorage      PluginType = "storage"
    PluginTypeAuth         PluginType = "auth"
)

// Vectorizer plugin interface
type VectorizerPlugin interface {
    Plugin
    
    // Vectorization
    VectorizeText(ctx context.Context, text string) ([]float32, error)
    VectorizeBatch(ctx context.Context, texts []string) ([][]float32, error)
    
    // Configuration
    Dimensions() int
    Model() string
}
```

### Plugin Manifest

```yaml
# plugin.yaml
apiVersion: weaviate.io/v1
kind: Plugin
metadata:
  name: custom-embedder
  version: 1.0.0
  author: "Custom Corp"
  description: "Proprietary embedding model"
  
spec:
  type: vectorizer
  
  # Runtime
  runtime: wasm  # wasm | grpc | native
  
  # Binary/WASM file
  binary: ./custom-embedder.wasm
  
  # Dependencies
  dependencies:
    - name: onnx-runtime
      version: "^1.15.0"
  
  # Resource limits
  resources:
    memory: "2Gi"
    cpu: "1000m"
    
  # Capabilities
  capabilities:
    dimensions: 768
    maxBatchSize: 100
    
  # Configuration schema
  config:
    apiKey:
      type: string
      required: true
      secret: true
    modelPath:
      type: string
      default: "/models/default"
```

### WebAssembly (WASM) Plugin Example

```rust
// Rust plugin compiled to WASM
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
    
    fn vectorize_batch(&self, texts: Vec<&str>) -> Result<Vec<Vec<f32>>> {
        texts.into_iter()
            .map(|text| self.vectorize_text(text))
            .collect()
    }
    
    fn dimensions(&self) -> u32 {
        768
    }
}
```

### gRPC Plugin Example

```protobuf
// plugin.proto
syntax = "proto3";

package weaviate.plugin.v1;

service VectorizerPlugin {
  rpc VectorizeText(VectorizeRequest) returns (VectorizeResponse);
  rpc VectorizeBatch(BatchVectorizeRequest) returns (BatchVectorizeResponse);
  rpc GetInfo(InfoRequest) returns (InfoResponse);
}

message VectorizeRequest {
  string text = 1;
  map<string, string> metadata = 2;
}

message VectorizeResponse {
  repeated float vector = 1;
}
```

```python
# Python gRPC plugin server
from weaviate_plugin_sdk import VectorizerPlugin
import grpc
from concurrent import futures

class CustomVectorizer(VectorizerPlugin):
    def __init__(self):
        self.model = load_model("custom-model")
    
    def VectorizeText(self, request, context):
        vector = self.model.encode(request.text)
        return VectorizeResponse(vector=vector.tolist())
    
    def VectorizeBatch(self, request, context):
        vectors = self.model.encode_batch(request.texts)
        return BatchVectorizeResponse(
            vectors=[VectorizeResponse(vector=v.tolist()) for v in vectors]
        )

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    add_VectorizerPluginServicer_to_server(CustomVectorizer(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()
```

### Plugin Manager

```go
type PluginManager struct {
    registry  *PluginRegistry
    loader    *PluginLoader
    sandbox   *Sandbox
    lifecycle *LifecycleManager
}

func (pm *PluginManager) Load(manifest *Manifest) error {
    // Validate manifest
    if err := pm.validateManifest(manifest); err != nil {
        return err
    }
    
    // Create sandbox environment
    sandbox := pm.sandbox.Create(manifest.Name, manifest.Spec.Resources)
    
    // Load plugin based on runtime
    var plugin Plugin
    switch manifest.Spec.Runtime {
    case "wasm":
        plugin, err = pm.loader.LoadWASM(manifest.Spec.Binary, sandbox)
    case "grpc":
        plugin, err = pm.loader.LoadGRPC(manifest.Spec.Binary, sandbox)
    case "native":
        plugin, err = pm.loader.LoadNative(manifest.Spec.Binary, sandbox)
    default:
        return ErrUnsupportedRuntime
    }
    
    if err != nil {
        return err
    }
    
    // Initialize plugin
    if err := plugin.Init(context.Background(), manifest.Spec.Config); err != nil {
        return err
    }
    
    // Register plugin
    pm.registry.Register(manifest.Name, plugin)
    
    return nil
}

// Hot reload
func (pm *PluginManager) Reload(name string, newManifest *Manifest) error {
    // Load new plugin
    newPlugin, err := pm.Load(newManifest)
    if err != nil {
        return err
    }
    
    // Get current plugin
    oldPlugin := pm.registry.Get(name)
    
    // Start new plugin
    if err := newPlugin.Start(context.Background()); err != nil {
        return err
    }
    
    // Wait for in-flight requests
    pm.lifecycle.DrainRequests(oldPlugin)
    
    // Swap plugins atomically
    pm.registry.Swap(name, oldPlugin, newPlugin)
    
    // Stop old plugin
    oldPlugin.Stop(context.Background())
    
    return nil
}
```

### Sandboxing and Security

```go
// WASM sandbox
type WASMSandbox struct {
    instance *wazero.Runtime
    limits   *ResourceLimits
}

type ResourceLimits struct {
    MaxMemory      uint64
    MaxCPUTime     time.Duration
    MaxFileSize    uint64
    AllowedSyscalls []string
}

func (s *WASMSandbox) Execute(fn string, args []interface{}) (interface{}, error) {
    // Set resource limits
    ctx := context.WithValue(context.Background(), "limits", s.limits)
    
    // Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, s.limits.MaxCPUTime)
    defer cancel()
    
    // Call WASM function
    result, err := s.instance.Call(ctx, fn, args...)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil, ErrCPULimitExceeded
        }
        return nil, err
    }
    
    return result, nil
}
```

### Plugin Marketplace

```yaml
# Centralized plugin registry
registry:
  url: https://plugins.weaviate.io
  
  # Featured plugins
  featured:
    - name: openai-embeddings
      downloads: 50000
      rating: 4.8
    
    - name: cohere-reranker
      downloads: 25000
      rating: 4.7

# CLI commands
$ weaviate plugin search "embeddings"
$ weaviate plugin install openai-embeddings@1.2.0
$ weaviate plugin list
$ weaviate plugin update openai-embeddings
```

---

## Performance Impact

### WASM vs Native Performance

| Operation | Native | WASM | Overhead |
|-----------|--------|------|----------|
| Vectorize single | 1.2ms | 1.8ms | +50% |
| Vectorize batch (100) | 45ms | 52ms | +15% |
| Plugin init | 10ms | 25ms | +150% |
| Memory usage | 100MB | 120MB | +20% |

### gRPC Plugin Performance

| Metric | Local | Remote |
|--------|-------|--------|
| Latency | 2-3ms | 10-50ms |
| Throughput | 10k req/s | 2k req/s |
| Resource usage | Low | Medium |

---

## Implementation Plan

### Phase 1: Core Framework (6 weeks)
- [ ] Plugin interface design
- [ ] WASM runtime integration
- [ ] Plugin manager
- [ ] Resource sandboxing

### Phase 2: Plugin Types (4 weeks)
- [ ] Vectorizer plugins
- [ ] Transformer plugins
- [ ] Reranker plugins
- [ ] Storage plugins

### Phase 3: Distribution (3 weeks)
- [ ] Plugin marketplace
- [ ] CLI tools
- [ ] Documentation
- [ ] Example plugins

### Phase 4: Security & Testing (3 weeks)
- [ ] Security audit
- [ ] Performance testing
- [ ] Plugin certification
- [ ] Production rollout

**Total: 16 weeks**

---

## Success Criteria

- ✅ 10+ community plugins in first 6 months
- ✅ <25% performance overhead for WASM
- ✅ Zero security incidents
- ✅ Hot-reload without downtime
- ✅ Plugin SDK for 3+ languages

---

## References

- WebAssembly: https://webassembly.org/
- HashiCorp go-plugin: https://github.com/hashicorp/go-plugin
- Envoy WASM: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/wasm_filter
- Kubernetes Operators: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-16*