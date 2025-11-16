# RFC: Enhanced Observability Suite for Weaviate

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-10  
**Updated:** 2025-01-10  

---

## Summary

Implement comprehensive observability suite including query explain visualizer, HNSW health metrics, enhanced slow query logging, and OpenTelemetry tracing integration.

**Current state:** Basic Prometheus metrics, limited query debugging  
**Proposed state:** Full observability with explain plans, distributed tracing, health checks

---

## Motivation

### Problem Statement

**Debugging production issues is difficult:**

1. **No query execution visibility**
   - Why is this query slow?
   - Which part takes time (vector search vs keyword vs fusion)?
   - What path did HNSW traversal take?

2. **Limited HNSW index health metrics**
   - Cannot detect graph connectivity issues
   - No unreachable node detection
   - Missing layer distribution visibility

3. **Slow query logging lacks context**
   - Only duration logged
   - No query plan captured
   - Hard to reproduce issues

4. **No distributed tracing**
   - Multi-shard queries opaque
   - Cannot trace cross-service calls (e.g., vectorizer APIs)
   - Missing request correlation

### User Impact

**Production debugging scenarios:**

**Scenario 1: "Why is this specific query slow?"**
- Current: Guess based on duration, try changing parameters randomly
- Proposed: View explain plan → see HNSW traversed 2000 nodes (high ef) → reduce ef

**Scenario 2: "Index performance degraded after 1M deletes"**
- Current: Restart index, hope it improves
- Proposed: Check unreachable nodes metric → 15,000 unreachable → run cleanup

**Scenario 3: "Some queries fast, some slow, can't find pattern"**
- Current: Manual log analysis
- Proposed: Distributed trace shows some queries wait for vectorizer API (rate limited)

---

## Component 1: Query Explain Visualizer

### Explain Plan JSON Schema

```json
{
  "query_id": "uuid-...",
  "timestamp": "2025-01-10T14:30:00Z",
  "total_duration_ms": 23.5,
  
  "query": {
    "type": "hybrid",
    "class": "Article",
    "limit": 10,
    "filters": {...},
    "hybrid": {"alpha": 0.7, "query": "machine learning"}
  },
  
  "execution": {
    "phases": [
      {
        "phase": "filter",
        "duration_ms": 2.1,
        "selectivity": 0.15,
        "strategy": "bitmap_and",
        "candidates_before": 1000000,
        "candidates_after": 150000
      },
      {
        "phase": "vector_search",
        "algorithm": "hnsw",
        "duration_ms": 12.4,
        "entry_point": 12345,
        "layers_traversed": [3, 2, 1, 0],
        "nodes_evaluated": 427,
        "ef_used": 128,
        "traversal_path": [
          {"layer": 3, "nodes": [12345, 23456]},
          {"layer": 2, "nodes": [23456, 34567, 45678]},
          {"layer": 1, "nodes": [...]},
          {"layer": 0, "nodes": [...]}
        ],
        "result_count": 100
      },
      {
        "phase": "keyword_search",
        "algorithm": "bm25f_blockmax_wand",
        "duration_ms": 8.7,
        "query_terms": ["machine", "learning"],
        "properties": ["title", "content"],
        "blocks_scanned": 45,
        "blocks_skipped": 312,
        "skip_rate": 0.874,
        "result_count": 150
      },
      {
        "phase": "fusion",
        "algorithm": "ranked_fusion",
        "duration_ms": 0.3,
        "weights": [0.7, 0.3],
        "results_before": 250,
        "results_after": 10
      }
    ]
  },
  
  "results": [
    {
      "id": "uuid-...",
      "score": 0.924,
      "vector_score": 0.87,
      "keyword_score": 0.68
    }
  ]
}
```

### Visualization Outputs

**1. HTML Interactive Visualization**

Features:
- HNSW graph with traversal path highlighted
- BM25F score breakdown (stacked bar chart)
- Timeline of phases
- Filterable/zoomable

**2. CLI Text Output**

```
Query Execution Plan
====================

Total: 23.5ms

Filter (2.1ms)
  Strategy: bitmap_and
  Selectivity: 15% (1M → 150k candidates)

Vector Search (12.4ms)
  Algorithm: HNSW
  Entry: 12345
  Layers: 3 → 0
  Nodes: 427 evaluated
  ef: 128
  
  Path:
    L3: 12345 → 23456
    L2: 23456 → 34567 → 45678
    L1: [15 nodes]
    L0: [397 nodes]

Keyword Search (8.7ms, parallel)
  Algorithm: BM25F + BlockMaxWAND
  Terms: machine (IDF: 4.2), learning (IDF: 3.8)
  Blocks: 45 scanned, 312 skipped (87%)

Fusion (0.3ms)
  Algorithm: Ranked Fusion
  Weights: [70%, 30%]
  Merged: 250 → 10

Top Result:
  Score: 0.924 = 0.7*0.87(vector) + 0.3*0.68(keyword)
```

### API Integration

**GraphQL:**

```graphql
{
  Get {
    Article(hybrid: {query: "AI"}, limit: 10) {
      title
      _additional {
        explainPlan  # NEW: Full execution plan
      }
    }
  }
}
```

**REST:**

```bash
POST /v1/graphql?explain=true
# Returns explain plan in response headers or separate field
```

---

## Component 2: HNSW Health Metrics

### New Prometheus Metrics

```go
// Graph connectivity
vector_index_unreachable_nodes_total{class, shard}
// Nodes not reachable from entry point

vector_index_isolated_components{class, shard}
// Number of disconnected subgraphs

vector_index_avg_degree{class, shard, layer}
// Average connections per layer

vector_index_max_degree{class, shard, layer}
// Maximum connections (should be <= M or M0)

// Layer distribution
vector_index_layer_node_count{class, shard, layer}
// Nodes per layer (histogram)

vector_index_max_layer{class, shard}
// Current maximum layer

// Entry point
vector_index_entrypoint_id{class, shard}
// Current entry point doc ID

vector_index_entrypoint_degree{class, shard}
// Connections of entry point

vector_index_entrypoint_changes_total{class, shard}
// How often entry point changed (should be rare)
```

### Health Check API

**New REST endpoint:**

```bash
GET /v1/indices/{class}/{shard}/health

Response:
{
  "status": "healthy",  # healthy | degraded | unhealthy
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
      "status": "warning",  # >5% of total
      "count": 15000,
      "percentage": 1.5,
      "threshold": 10.0
    }
  }
}
```

---

## Component 3: Enhanced Slow Query Logging

### Current Implementation

**File:** `adapters/repos/db/helpers/slow_queries.go`

```go
// Current (limited)
type SlowQueryLog struct {
    Timestamp time.Time
    Duration time.Duration
    QueryType string
}
```

### Proposed Enhancement

```go
type SlowQueryLog struct {
    // Existing fields
    Timestamp time.Time
    Duration time.Duration
    QueryType string
    
    // NEW: Structured context
    ClassName string
    ShardName string
    Limit int
    FiltersApplied bool
    FilterSummary string
    
    // NEW: Performance breakdown
    Breakdown PerformanceBreakdown
    
    // NEW: Full explain plan
    ExplainPlan *QueryExplainPlan  // Detailed execution plan
}

type PerformanceBreakdown struct {
    FilterMS float64
    VectorSearchMS float64
    KeywordSearchMS float64
    FusionMS float64
    SerializationMS float64
}
```

### Structured JSON Logging

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
  "filter_summary": "category=tech AND publishedAt>2023",
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

**Benefits:**
- Structured logging enables easy querying (ELK, Splunk, etc.)
- Performance breakdown pinpoints bottleneck
- Explain plan enables reproduction

---

## Component 4: OpenTelemetry Tracing

### Trace Span Hierarchy

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
          │   │   ├─ Tokenize (0.3ms)
          │   │   ├─ Apply Filter (1.8ms, same as vector)
          │   │   ├─ BM25F.WAND (5.2ms)
          │   │   └─ Fetch Objects (1.4ms)
          │   └─ Fusion (0.4ms)
          └─ Format Response (0.3ms)
```

### Implementation

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("weaviate")

func (h *hnsw) SearchByVector(ctx context.Context, vector []float32, k int) ([]Result, error) {
    ctx, span := tracer.Start(ctx, "hnsw.SearchByVector")
    defer span.End()
    
    span.SetAttributes(
        attribute.Int("k", k),
        attribute.Int("ef", int(h.ef)),
        attribute.Int("dimensions", len(vector)),
        attribute.String("class", h.className),
        attribute.String("shard", h.shardName),
    )
    
    // Layer-by-layer spans
    for level := h.currentMaximumLayer; level >= 0; level-- {
        layerCtx, layerSpan := tracer.Start(ctx, "hnsw.searchLayer")
        layerSpan.SetAttributes(attribute.Int("layer", level))
        
        results := h.searchLayer(layerCtx, vector, entryPoint, ef, level)
        
        layerSpan.SetAttributes(
            attribute.Int("nodes_evaluated", results.NodesEvaluated),
            attribute.Int("candidates_found", len(results.Candidates)),
        )
        layerSpan.End()
    }
    
    span.SetAttributes(attribute.Int("total_nodes_evaluated", totalNodes))
    
    return results, nil
}
```

### Trace Backends

**Supported:**
- Jaeger (open-source)
- Datadog APM
- New Relic
- Honeycomb
- Any OpenTelemetry-compatible backend

**Configuration:**

```yaml
OTEL_EXPORTER_OTLP_ENDPOINT: http://jaeger:4318
OTEL_SERVICE_NAME: weaviate
OTEL_TRACES_SAMPLER: parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG: 0.1  # Sample 10% of requests
```

---

## Implementation Plan

### Phase 1: Metrics & Health (4 weeks)

**Week 1-2: HNSW Health Metrics**
- Implement unreachable node detection
- Layer distribution metrics
- Entry point health tracking
- Unit tests

**Week 3-4: Health Check API**
- REST endpoint implementation
- Health check logic
- Grafana dashboard panels

### Phase 2: Query Explain (4 weeks)

**Week 5-6: Explain Plan Generation**
- Instrument HNSW search (capture traversal)
- Instrument BM25F search (capture scoring)
- Instrument fusion (capture weights)
- JSON schema definition

**Week 7: Visualization**
- HTML visualizer (D3.js)
- CLI text formatter
- GraphQL integration (`_additional.explainPlan`)

**Week 8: Testing & Documentation**
- Integration tests
- User guide
- Example queries

### Phase 3: Enhanced Logging (2 weeks)

**Week 9: Slow Query Logger**
- Add structured fields
- Integrate explain plan
- Performance breakdown

**Week 10: Testing & Rollout**
- Test log volume impact
- Configuration options
- Documentation

### Phase 4: Distributed Tracing (4 weeks)

**Week 11-12: OpenTelemetry Integration**
- Add spans to critical paths
- Context propagation
- Sampling strategy

**Week 13: External Call Tracing**
- Vectorizer API calls
- Cross-shard calls
- Replication operations

**Week 14: Documentation & Examples**
- Jaeger setup guide
- Example traces
- Performance impact analysis

**Total: 14 weeks**

---

## Performance Impact

### Overhead Analysis

**Explain plan generation:**
- Capture mode: +5-10% latency (instrumentation overhead)
- Enabled by default: +0.1% (flag check only)
- Recommendation: Opt-in per query

**Prometheus metrics:**
- Existing: ~50 metrics, 0.5% CPU overhead
- Proposed: +20 metrics (health), +1% CPU
- Acceptable for production

**OpenTelemetry tracing:**
- Sampling disabled: 0% overhead
- 10% sampling: +2% latency (span creation/export)
- 100% sampling: +8% latency (not recommended)

**Total observability overhead (recommended config):**
- Metrics: +1% CPU
- Tracing (10% sample): +0.2% latency (averaged)
- Explain (opt-in): +0% (unless requested)
- **Total: Negligible (<2%) for production**

---

## Success Criteria

**Must achieve:**
- ✅ Explain plan captures 100% of execution details
- ✅ HNSW health metrics detect all known issues
- ✅ Slow query logging includes explain plan
- ✅ OpenTelemetry traces correlate cross-service calls
- ✅ Performance overhead < 3% with recommended config
- ✅ Documentation for all features

**Nice to have:**
- Automatic anomaly detection
- Query optimization suggestions
- Historical query analysis

---

## Open Questions

1. **Explain plan storage:**
   - Store in database for historical analysis?
   - Or ephemeral (returned in response only)?
   - **Answer:** Ephemeral in v1, optional storage in v2

2. **Tracing sampling strategy:**
   - Fixed 10% or adaptive based on error rate?
   - **Answer:** Fixed 10% default, configurable

3. **Performance regression detection:**
   - Automatically detect slow queries compared to baseline?
   - **Answer:** Out of scope for v1, consider for v2

---

## References

- **OpenTelemetry:** https://opentelemetry.io
- **Jaeger:** https://www.jaegertracing.io
- **Current Metrics:** [`docs/metrics.md`](https://github.com/weaviate/weaviate/blob/main/docs/metrics.md)
- **POC Repository:** https://github.com/josedavidbaena/weaviate-explain (to be created)

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-10*