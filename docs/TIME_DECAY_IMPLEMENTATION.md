# Temporal Vector Support - Implementation Details

## Architecture Overview

The temporal vector support implementation consists of several key components:

```
GraphQL/REST API
       ↓
   Parameter Extraction (common_filters/time_decay.go)
       ↓
   Search Parameters (entities/searchparams/retrieval.go)
       ↓
   GetParams (entities/dto/dto.go)
       ↓
   Database Search Layer (adapters/repos/db/search.go)
       ↓
   Time Decay Application (adapters/repos/db/time_decay.go)
       ↓
   Results Re-ranking
```

## Core Components

### 1. Time Decay Engine (`entities/timedecay/decay.go`)

**Purpose**: Core time decay calculation logic

**Key Types**:
```go
type Config struct {
    Property            string
    HalfLife            time.Duration
    MaxAge              time.Duration
    DecayFunction       DecayFunction
    StepThresholds      []StepThreshold
    OverFetchMultiplier float32
}

type DecayFunction string
const (
    DecayFunctionExponential DecayFunction = "EXPONENTIAL"
    DecayFunctionLinear      DecayFunction = "LINEAR"
    DecayFunctionStep        DecayFunction = "STEP"
)
```

**Key Methods**:
- `CalculateDecay(age time.Duration) float32`: Computes decay factor
- `Validate() error`: Validates configuration
- `GetOverFetchMultiplier() float32`: Returns optimal over-fetch multiplier

### 2. Search Parameters (`entities/searchparams/retrieval.go`)

**Purpose**: GraphQL/API parameter representation

**Key Types**:
```go
type TimeDecay struct {
    Property       string
    HalfLife       string  // e.g., "7d"
    MaxAge         string  // e.g., "30d"
    DecayFunction  string
    StepThresholds []TimeDecayStepThreshold
}

func (td *TimeDecay) ToConfig() (*timedecay.Config, error)
```

### 3. GraphQL Schema (`adapters/handlers/graphql/local/common_filters/`)

**Files**:
- `time_decay_argument.go`: GraphQL type definitions
- `time_decay.go`: Parameter extraction

**GraphQL Schema**:
```graphql
input TimeDecayInpObj {
  property: String!
  halfLife: String
  maxAge: String
  decayFunction: DecayFunctionEnum!
  stepThresholds: [StepThreshold]
}

enum DecayFunctionEnum {
  EXPONENTIAL
  LINEAR
  STEP
}
```

### 4. Database Search Layer (`adapters/repos/db/`)

**Files**:
- `search.go`: Main search orchestration
- `time_decay.go`: Time decay application logic

**Key Functions**:
```go
func (db *DB) VectorSearch(ctx context.Context,
    params dto.GetParams,
    targetVectors []string,
    searchVectors []models.Vector,
) ([]search.Result, error)

func applyTimeDecay(
    objects []*storobj.Object,
    distances []float32,
    timeDecayConfig *timedecay.Config,
    originalLimit int,
) ([]*storobj.Object, []float32, error)

func calculateOverFetchLimit(
    originalLimit int,
    timeDecayConfig *timedecay.Config,
) int
```

## Implementation Flow

### Search Execution Flow

1. **GraphQL Query Received**
   ```graphql
   nearText: {concepts: ["AI"]}
   timeDecay: {property: "publishedAt", halfLife: "7d", decayFunction: EXPONENTIAL}
   limit: 10
   ```

2. **Parameter Extraction** (`class_builder_fields.go`)
   - Extract `timeDecay` from GraphQL args
   - Convert to `searchparams.TimeDecay`
   - Add to `dto.GetParams`

3. **Search Preparation** (`search.go: VectorSearch`)
   - Convert `searchparams.TimeDecay` to `timedecay.Config`
   - Calculate over-fetch limit: `10 * 3 = 30` (for 7d half-life)
   - Pass modified limit to vector search

4. **Vector Search** (`index.go: objectVectorSearch`)
   - Perform HNSW search with limit=30
   - Return 30 candidates with distances

5. **Time Decay Application** (`time_decay.go: applyTimeDecay`)
   - For each object:
     - Extract timestamp from property
     - Calculate age: `now - timestamp`
     - Compute decay: `exp(-age / halfLife)`
     - Convert distance to similarity: `sim = 1 - distance`
     - Apply decay: `decayed_sim = sim * decay`
     - Convert back: `score = -decayed_sim` (for sorting)
   - Sort by combined score
   - Truncate to original limit (10)

6. **Results Returned**
   - Top 10 results re-ranked by temporal relevance

## Over-Fetch Strategy

### Why Over-Fetch?

Time decay changes ranking, so the top-k results *after* decay may not be the same as the top-k results *before* decay.

**Example**:
```
Before Decay (by vector similarity):
1. Article A (distance: 0.1, age: 30 days) → similarity: 0.9
2. Article B (distance: 0.11, age: 1 day) → similarity: 0.89
3. Article C (distance: 0.12, age: 7 days) → similarity: 0.88

After Decay (halfLife = 7 days):
1. Article B (0.89 * 1.0 = 0.89) ← Now ranked #1!
2. Article C (0.88 * 0.37 = 0.33)
3. Article A (0.9 * 0.01 = 0.009)
```

### Over-Fetch Multipliers

**Empirical values** (from RFC):
```go
func (c *Config) GetOverFetchMultiplier() float32 {
    switch c.DecayFunction {
    case DecayFunctionExponential:
        if c.HalfLife <= 24*time.Hour:
            return 5.0  // Very short half-life
        } else if c.HalfLife <= 7*24*time.Hour {
            return 3.0  // Week half-life
        } else if c.HalfLife <= 30*24*time.Hour {
            return 2.0  // Month half-life
        }
        return 1.5
    case DecayFunctionLinear, DecayFunctionStep:
        return 3.0
    default:
        return 1.0
    }
}
```

## Scoring Algorithm

### Distance to Score Conversion

Vector search returns **distances** (lower is better), but we need **scores** (higher is better) to apply multiplicative decay.

**Conversion** (for cosine distance):
```go
similarity := 1.0 - distance
```

### Time Decay Application

```go
decayFactor := timeDecayConfig.CalculateDecay(age)
decayedSimilarity := similarity * decayFactor
```

### Re-ranking

```go
combinedScore := -decayedSimilarity  // Negate so lower is better
sort.Slice(results, func(i, j int) bool {
    return results[i].combinedScore < results[j].combinedScore
})
```

## Timestamp Parsing

The `parseTimestamp` function supports multiple formats:

```go
func parseTimestamp(val interface{}) (time.Time, error) {
    switch v := val.(type) {
    case string:
        // Try multiple formats:
        // - RFC3339: "2024-01-15T10:30:00Z"
        // - RFC3339Nano: "2024-01-15T10:30:00.123Z"
        // - ISO 8601 variants
        // - Date only: "2024-01-15"
    case time.Time:
        return v, nil
    case int64:
        // Unix timestamp in milliseconds
        return time.Unix(0, v*int64(time.Millisecond)), nil
    case float64:
        // Unix timestamp in seconds
        return time.Unix(int64(v), 0), nil
    }
}
```

## Testing Strategy

### Unit Tests

1. **Decay Functions** (`entities/timedecay/decay_test.go`)
   - Test exponential, linear, step decay
   - Validate duration parsing
   - Test over-fetch multiplier calculation

### Integration Tests

2. **Time Decay Application** (`test/acceptance/timedecay/time_decay_test.go`)
   - Create test data with different timestamps
   - Verify re-ranking with different decay functions
   - Test GraphQL query integration

### Test Data Setup

```go
now := time.Now()
objects := []*models.Object{
    {
        Properties: map[string]interface{}{
            "title": "Recent",
            "publishedAt": now.Add(-1 * time.Hour).Format(time.RFC3339),
        },
        Vector: []float32{0.1, 0.2, 0.3},
    },
    {
        Properties: map[string]interface{}{
            "title": "Old",
            "publishedAt": now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
        },
        Vector: []float32{0.11, 0.21, 0.31}, // Very similar vector
    },
}
```

## Performance Considerations

### Memory Usage

Over-fetching increases memory usage:
- `limit: 10` with `3x` multiplier → 30 objects in memory
- Each object: ~1-10 KB (depending on properties)
- Re-ranking: O(n log n) where n = over-fetched count

### Latency Breakdown

1. **Vector search**: +30% (3x more candidates)
2. **Timestamp extraction**: ~0.1ms per object
3. **Decay calculation**: ~0.01ms per object
4. **Re-sorting**: ~0.3ms for 30 objects
5. **Total overhead**: ~35%

### Optimizations

1. **Adaptive over-fetch**: Learn optimal multiplier from query patterns
2. **Caching**: Cache decay factors for common age ranges
3. **Parallel processing**: Compute decay in parallel (future work)

## Migration Guide

### Backward Compatibility

✅ **Fully backward compatible**:
- `timeDecay` parameter is optional
- Existing queries work unchanged
- No schema changes required

### Adding Time Decay to Existing Queries

**Before**:
```graphql
{
  Get {
    Article(nearText: {concepts: ["AI"]}, limit: 10) {
      title
    }
  }
}
```

**After**:
```graphql
{
  Get {
    Article(
      nearText: {concepts: ["AI"]}
      timeDecay: {
        property: "publishedAt"
        halfLife: "7d"
        decayFunction: EXPONENTIAL
      }
      limit: 10
    ) {
      title
      publishedAt
    }
  }
}
```

### Schema Requirements

Add a `date` property if not present:

```json
{
  "class": "Article",
  "properties": [
    {
      "name": "publishedAt",
      "dataType": ["date"],
      "description": "Publication timestamp"
    }
  ]
}
```

## Future Enhancements

### Phase 2 Features (Potential)

1. **Adaptive Over-Fetch**
   - Learn optimal multiplier based on query patterns
   - Reduce latency for stable datasets

2. **Additional Metrics**
   - Expose `timeDecayScore` in `_additional`
   - Return `vectorScore` and `combinedScore` separately

3. **Batch Optimization**
   - Parallel decay calculation
   - SIMD-optimized timestamp parsing

4. **Advanced Decay Functions**
   - Gaussian decay
   - Custom decay curves via expression language

5. **Multi-Property Decay**
   - Combine multiple timestamps (e.g., `publishedAt` + `updatedAt`)
   - Weighted decay across properties

## Code References

### Key Files

- **Core Logic**: `entities/timedecay/decay.go`
- **Search Integration**: `adapters/repos/db/search.go`, `adapters/repos/db/time_decay.go`
- **GraphQL**: `adapters/handlers/graphql/local/common_filters/time_decay*.go`
- **Parameters**: `entities/searchparams/retrieval.go`
- **Tests**: `entities/timedecay/decay_test.go`, `test/acceptance/timedecay/time_decay_test.go`

### Related Components

- HNSW Search: `adapters/repos/db/vector/hnsw/search.go`
- Distance Calculation: `adapters/repos/db/vector/hnsw/distancer/`
- Result Sorting: `adapters/repos/db/sorter/`

## Debugging

### Enable Debug Logging

```go
// In adapters/repos/db/search.go
db.logger.WithFields(logrus.Fields{
    "time_decay_config": timeDecayConfig,
    "original_limit":    originalLimit,
    "over_fetch_limit":  totalLimit,
}).Debug("Applying time decay to vector search")
```

### Inspect Results

Add temporary logging in `applyTimeDecay`:

```go
for i, result := range scored {
    logger.WithFields(logrus.Fields{
        "rank":          i,
        "id":            result.obj.ID(),
        "original_dist": result.originalDist,
        "decay_factor":  result.decayFactor,
        "combined":      result.combinedScore,
    }).Debug("Time decay result")
}
```

### Common Issues

1. **No re-ranking observed**
   - Check property name matches schema
   - Verify all objects have the timestamp
   - Ensure half-life is appropriate for data age range

2. **Unexpected order**
   - Verify decay function choice
   - Check timestamp values are correct
   - Inspect combined scores in debug logs

3. **Performance degradation**
   - Check over-fetch multiplier
   - Verify dataset size (time decay works best with < 1M objects per shard)
   - Consider adding pre-filters to reduce search space

## Contributing

To extend or modify time decay support:

1. **Add new decay function**:
   - Update `DecayFunction` enum in `entities/timedecay/decay.go`
   - Implement calculation in `CalculateDecay`
   - Add tests in `decay_test.go`
   - Update GraphQL enum in `time_decay_argument.go`

2. **Optimize performance**:
   - Profile with `pprof` to identify bottlenecks
   - Consider caching strategies
   - Benchmark with realistic datasets

3. **Add metrics**:
   - Expose time decay scores via `_additional`
   - Add telemetry for over-fetch effectiveness

## References

- RFC: `rfcs/05-temporal-vector-support.md`
- User Guide: `docs/TIME_DECAY_GUIDE.md`
- Weaviate Architecture: `https://weaviate.io/developers/weaviate/more-resources/architecture`
