# Enhanced Observability Suite

This document describes the enhanced observability features implemented in Weaviate as per RFC 03.

## Overview

The Enhanced Observability Suite provides comprehensive monitoring, debugging, and performance analysis capabilities for Weaviate, including:

1. **Query Explain Visualizer** - Detailed execution plans for queries
2. **HNSW Health Metrics** - Graph connectivity and health monitoring
3. **Enhanced Slow Query Logging** - Structured logging with performance breakdowns
4. **OpenTelemetry Tracing** - Distributed tracing support

## 1. Query Explain Visualizer

### Overview

The Query Explain Visualizer captures detailed execution plans for queries, showing exactly how Weaviate processes each query including HNSW traversal paths, BM25F scoring, and fusion operations.

### Usage

#### Programmatic Access

```go
import "github.com/weaviate/weaviate/entities/observability"

// Create an explain plan builder
builder := observability.NewExplainPlanBuilder("hybrid", "Article", 10)

// Add phases as the query executes
builder.AddVectorSearchPhase(
    duration,
    "hnsw",
    entryPoint,
    []int{3, 2, 1, 0}, // layers traversed
    427,  // nodes evaluated
    128,  // ef used
    100,  // result count
    traversalPath,
)

builder.AddKeywordSearchPhase(
    duration,
    "bm25f_blockmax_wand",
    []string{"machine", "learning"},
    []string{"title", "content"},
    45,   // blocks scanned
    312,  // blocks skipped
    150,  // result count
)

builder.AddFusionPhase(
    duration,
    "ranked_fusion",
    []float64{0.7, 0.3},
    250,  // results before
    10,   // results after
)

// Build the final plan
plan := builder.Build()
```

### Explain Plan Schema

The explain plan includes:

- **Query Information**: Type, class, limit, filters
- **Execution Phases**: Filter, vector search, keyword search, fusion
- **Performance Metrics**: Duration per phase, selectivity, nodes evaluated
- **HNSW Traversal**: Layer-by-layer path through the graph
- **BM25F Details**: Block skipping statistics, query terms
- **Results**: Score breakdown (vector + keyword components)

### Integration with Slow Query Logging

Explain plans are automatically included in slow query logs when a query exceeds the threshold:

```go
helpers.AnnotateExplainPlan(ctx, plan)
```

## 2. HNSW Health Metrics

### Overview

HNSW Health Metrics provide visibility into the health and structure of HNSW vector indices, enabling proactive detection of issues like graph fragmentation or connectivity problems.

### Prometheus Metrics

The following metrics are exposed:

#### Graph Connectivity
- `vector_index_unreachable_nodes_total{class, shard}` - Nodes not reachable from entry point
- `vector_index_isolated_components{class, shard}` - Number of disconnected subgraphs

#### Layer Distribution
- `vector_index_layer_node_count{class, shard, layer}` - Nodes per layer
- `vector_index_max_layer{class, shard}` - Current maximum layer
- `vector_index_avg_degree{class, shard, layer}` - Average connections per layer
- `vector_index_max_degree{class, shard, layer}` - Maximum connections (should be ≤ M or M0)

#### Entry Point
- `vector_index_entrypoint_id{class, shard}` - Current entry point doc ID
- `vector_index_entrypoint_degree{class, shard}` - Connections of entry point
- `vector_index_entrypoint_changes_total{class, shard}` - How often entry point changed

### Health Check API

**Endpoint:** `GET /v1/indices/{class}/{shard}/health`

**Response:**
```json
{
  "status": "healthy",
  "checks": {
    "connectivity": {
      "status": "healthy",
      "unreachable_nodes": 0,
      "isolated_components": 0
    },
    "layer_distribution": {
      "status": "healthy",
      "layers": {
        "0": 93700,
        "1": 5890,
        "2": 408,
        "3": 2
      },
      "expected_distribution": "normal"
    },
    "entry_point": {
      "status": "healthy",
      "id": 12345,
      "degree": 16,
      "layer": 3
    },
    "tombstones": {
      "status": "healthy",
      "count": 150,
      "percentage": 0.15,
      "threshold": 10.0
    }
  }
}
```

### Computing Health Metrics

Health metrics are computed periodically:

```go
// Compute and update health metrics
hnsw.ComputeHealthMetrics()

// Get health check status
health := hnsw.GetHealthCheck(totalObjects)
```

### Health Status Levels

- **healthy** - All checks pass
- **degraded** - Some checks show warnings (e.g., >5% tombstones)
- **unhealthy** - Critical issues detected (e.g., >10% tombstones, isolated components)

## 3. Enhanced Slow Query Logging

### Overview

Enhanced slow query logging provides structured, detailed information about slow queries, including performance breakdowns and explain plans.

### Configuration

Slow query logging is configured via runtime dynamic values:

```go
// Enable slow query logging
slowQueryReporter := helpers.NewSlowQueryReporter(
    enabledFlag,
    thresholdDuration, // e.g., 5 * time.Second
    logger,
)
```

### Usage

```go
// Initialize slow query details in context
ctx = helpers.InitSlowQueryDetails(ctx)

// Track query execution
startTime := time.Now()
defer slowQueryReporter.LogIfSlow(ctx, startTime, map[string]any{
    "query_type": "hybrid",
    "class_name": "Article",
})

// Annotate with structured details
helpers.AnnotateSlowQueryLog(ctx, "shard_name", "main")
helpers.AnnotateSlowQueryLog(ctx, "limit", 10)

// Add performance breakdown
breakdown := observability.PerformanceBreakdown{
    FilterMS:        2.1,
    VectorSearchMS:  12.4,
    KeywordSearchMS: 8.7,
    FusionMS:        0.3,
    SerializationMS: 0.1,
}
helpers.AnnotatePerformanceBreakdown(ctx, breakdown)

// Add explain plan
helpers.AnnotateExplainPlan(ctx, plan)
```

### Log Format

Slow queries are logged in structured JSON format:

```json
{
  "timestamp": "2025-01-10T14:30:45Z",
  "level": "warn",
  "msg": "slow_query",
  "duration_ms": 523,
  "query_type": "hybrid",
  "class": "Article",
  "shard": "main",
  "limit": 10,
  "filters_applied": true,
  "breakdown": {
    "filter_ms": 2.1,
    "vector_search_ms": 312,
    "keyword_search_ms": 98,
    "fusion_ms": 45,
    "serialization_ms": 66
  },
  "explain_plan": { ... }
}
```

### Benefits

- **Structured format** enables easy querying in log aggregation systems (ELK, Splunk, Datadog)
- **Performance breakdown** pinpoints bottlenecks
- **Explain plan** enables query reproduction and analysis

## 4. OpenTelemetry Tracing

### Overview

OpenTelemetry tracing provides distributed tracing capabilities for tracking requests across services and components.

### Configuration

```go
import "github.com/weaviate/weaviate/usecases/monitoring"

// Configure tracing
tracingConfig := monitoring.TracingConfig{
    Enabled:       true,
    ServiceName:   "weaviate",
    OTLPEndpoint:  "http://jaeger:4318",
    SamplingRatio: 0.1, // 10% sampling
}

tracer, err := monitoring.NewTracer(tracingConfig)
```

### Usage

```go
// Start a span
ctx, span := tracer.StartSpan(ctx, "hnsw.SearchByVector",
    monitoring.IntAttr("k", k),
    monitoring.IntAttr("ef", ef),
    monitoring.StringAttr("class", className),
    monitoring.StringAttr("shard", shardName),
)
defer span.End()

// Add attributes during execution
span.SetAttributes(
    monitoring.IntAttr("nodes_evaluated", nodesEvaluated),
    monitoring.IntAttr("result_count", len(results)),
)

// Record errors
if err != nil {
    span.RecordError(err)
    return nil, err
}
```

### Span Hierarchy Example

```
HTTP Request: POST /v1/graphql
  ├─ GraphQL Parse (1.2ms)
  ├─ GraphQL Validation (0.5ms)
  └─ GraphQL Execution (23.1ms)
      └─ Get.Article (23.1ms)
          ├─ Extract Arguments (0.3ms)
          ├─ Explorer.Hybrid (22.5ms)
          │   ├─ Vector Search (12.4ms)
          │   │   ├─ Vectorize Query [external: text2vec-openai] (2.1ms)
          │   │   ├─ Apply Filter (1.8ms)
          │   │   └─ HNSW.SearchByVector (8.5ms)
          │   │       ├─ Search Layer 3 (0.5ms)
          │   │       ├─ Search Layer 2 (1.2ms)
          │   │       ├─ Search Layer 1 (2.3ms)
          │   │       └─ Search Layer 0 (4.5ms)
          │   ├─ Keyword Search (8.7ms, parallel)
          │   └─ Fusion (0.4ms)
          └─ Format Response (0.3ms)
```

### Supported Backends

- Jaeger (open-source)
- Datadog APM
- New Relic
- Honeycomb
- Any OpenTelemetry-compatible backend

### Environment Variables

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
OTEL_SERVICE_NAME=weaviate
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # Sample 10% of requests
```

## Performance Impact

### Recommended Configuration

- **Metrics**: ~1% CPU overhead (acceptable for production)
- **Tracing (10% sampling)**: ~0.2% average latency increase
- **Explain Plan (opt-in)**: ~5-10% latency when enabled
- **Total**: <2% overhead with recommended settings

### Production Recommendations

1. **Always enable** HNSW health metrics (negligible overhead)
2. **Always enable** enhanced slow query logging (only activates on slow queries)
3. **Enable tracing** with 10% sampling for production environments
4. **Enable explain plans** only when debugging specific queries (opt-in per query)

## Troubleshooting Guide

### Scenario 1: "Why is this specific query slow?"

1. Check slow query log for performance breakdown
2. Examine explain plan to see execution details
3. Look for:
   - High `nodes_evaluated` in HNSW → Consider reducing `ef`
   - Low `skip_rate` in BM25F → Filter selectivity issues
   - Large `results_before` in fusion → Too many candidates

### Scenario 2: "Index performance degraded after deletions"

1. Check health endpoint: `GET /v1/indices/{class}/{shard}/health`
2. Look at tombstones metrics:
   - If `percentage > 10%` → Run compaction
   - If `unreachable_nodes > 0` → Graph fragmentation detected
3. Check Prometheus metrics for layer distribution anomalies

### Scenario 3: "Some queries fast, some slow"

1. Enable distributed tracing
2. Compare traces between fast and slow queries
3. Look for external call patterns (vectorizer rate limiting, network issues)

## API Reference

### Explain Plan Builder

```go
type ExplainPlanBuilder
    func NewExplainPlanBuilder(queryType, className string, limit int) *ExplainPlanBuilder
    func (b *ExplainPlanBuilder) AddFilterPhase(...)
    func (b *ExplainPlanBuilder) AddVectorSearchPhase(...)
    func (b *ExplainPlanBuilder) AddKeywordSearchPhase(...)
    func (b *ExplainPlanBuilder) AddFusionPhase(...)
    func (b *ExplainPlanBuilder) SetHybridInfo(alpha float64, query string)
    func (b *ExplainPlanBuilder) Build() *QueryExplainPlan
    func (b *ExplainPlanBuilder) GetPerformanceBreakdown() PerformanceBreakdown
```

### HNSW Health

```go
// Metrics methods
func (m *Metrics) SetUnreachableNodes(count int)
func (m *Metrics) SetIsolatedComponents(count int)
func (m *Metrics) SetAvgDegree(layer int, avgDegree float64)
func (m *Metrics) SetMaxDegree(layer int, maxDegree int)
func (m *Metrics) SetLayerNodeCount(layer int, count int)
func (m *Metrics) SetMaxLayer(layer int)
func (m *Metrics) SetEntrypointID(id uint64)
func (m *Metrics) SetEntrypointDegree(degree int)
func (m *Metrics) IncrementEntrypointChanges()

// Health check
func (h *hnsw) ComputeHealthMetrics()
func (h *hnsw) GetHealthCheck(totalObjects int) observability.IndexHealthResponse
```

### Slow Query Logging

```go
func InitSlowQueryDetails(ctx context.Context) context.Context
func AnnotateSlowQueryLog(ctx context.Context, key string, value any)
func AnnotatePerformanceBreakdown(ctx context.Context, breakdown observability.PerformanceBreakdown)
func AnnotateExplainPlan(ctx context.Context, plan *observability.QueryExplainPlan)
func ExtractSlowQueryDetails(ctx context.Context) map[string]any
```

### Tracing

```go
type Tracer
    func NewTracer(cfg TracingConfig) (*Tracer, error)
    func (t *Tracer) StartSpan(ctx context.Context, name string, attributes ...Attribute) (context.Context, SpanFinisher)

func StringAttr(key, value string) Attribute
func IntAttr(key string, value int) Attribute
func Float64Attr(key string, value float64) Attribute
func BoolAttr(key string, value bool) Attribute
```

## Migration Guide

### From Basic Metrics to Enhanced Observability

1. **Update imports**:
   ```go
   import "github.com/weaviate/weaviate/entities/observability"
   import "github.com/weaviate/weaviate/adapters/repos/db/helpers"
   ```

2. **Initialize context for slow query tracking**:
   ```go
   ctx = helpers.InitSlowQueryDetails(ctx)
   ```

3. **Add explain plan building** (optional, for debugging):
   ```go
   builder := observability.NewExplainPlanBuilder("hybrid", className, limit)
   // ... add phases ...
   plan := builder.Build()
   helpers.AnnotateExplainPlan(ctx, plan)
   ```

4. **Enable health metrics computation** (periodic background task):
   ```go
   // In HNSW maintenance routine
   hnsw.ComputeHealthMetrics()
   ```

5. **Configure tracing** (optional):
   ```go
   tracer, _ := monitoring.NewTracer(tracingConfig)
   ctx, span := tracer.StartSpan(ctx, "operation_name")
   defer span.End()
   ```

## Best Practices

1. **Always use context annotation** for slow query logging
2. **Compute health metrics periodically** (e.g., every 5 minutes) not on every query
3. **Use sampling for tracing** (10% is recommended for production)
4. **Enable explain plans on-demand** for specific queries, not all queries
5. **Monitor metrics dashboards** for health status trends
6. **Alert on health status changes** (e.g., when status becomes "degraded")

## Future Enhancements

- Automatic query optimization suggestions based on explain plans
- Historical query performance analysis
- Adaptive sampling based on error rates
- Interactive visualizations for explain plans (HTML/D3.js)
- CLI text formatter for explain plans
