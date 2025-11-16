# Cost-Based Query Optimizer

This package implements an ML-powered cost-based query optimizer for Weaviate, as described in RFC 0019.

## Overview

The optimizer consists of four main components:

1. **Learned Cardinality Estimator** (Python) - ML-based cardinality estimation using XGBoost
2. **Advanced Cost Model** (Go) - Cost modeling with ML integration
3. **Materialized Views** (Go) - Cached query results with automatic refresh
4. **Automatic Index Advisor** (Go) - Workload analysis and index recommendations

## Components

### 1. Learned Cardinality Estimator

**File:** `cardinality_estimator.py`

ML-based cardinality estimation that learns from historical query workload to predict result sizes with higher accuracy than traditional approaches.

**Features:**
- XGBoost-based regression model
- Feature extraction from query structure
- Online learning capability
- Fallback to heuristics when model not trained

**Usage:**
```python
from cardinality_estimator import LearnedCardinalityEstimator, QueryLog

# Create and train estimator
estimator = LearnedCardinalityEstimator()
estimator.train(query_logs)

# Make predictions
cardinality = estimator.estimate(query)
```

### 2. ML Cost Model

**File:** `ml_cost_model.go`

Integrates ML-based cardinality estimation with sophisticated cost modeling for query plan selection.

**Features:**
- Multi-operator cost modeling (vector search, filters, joins)
- Cardinality estimate caching
- Plan comparison
- Configurable cost factors

**Usage:**
```go
import "github.com/weaviate/weaviate/usecases/optimizer"

// Create cost model with ML estimator
estimator := NewMLCardinalityEstimator()
costModel := optimizer.NewMLCostModel(estimator)

// Estimate plan cost
cost, err := costModel.EstimatePlan(ctx, queryPlan)

// Compare alternative plans
bestPlan, cost, err := costModel.ComparePlans(ctx, plan1, plan2)
```

### 3. Materialized Views

**File:** `materialized_views.go`

Manages cached query results with configurable refresh policies.

**Features:**
- Manual, periodic, and incremental refresh policies
- Automatic query rewriting to use views
- View lifecycle management
- Background refresh scheduling

**Usage:**
```go
import "github.com/weaviate/weaviate/usecases/optimizer"

// Create view manager
manager := optimizer.NewMaterializedViewManager(queryExecutor)

// Create a materialized view
policy := optimizer.RefreshPolicy{
    Type:     optimizer.RefreshPeriodic,
    Interval: 5 * time.Minute,
}

err := manager.CreateView(ctx, "popular_articles", query, policy)

// Query rewriting will automatically use the view
rewrittenQuery := manager.RewriteQuery(userQuery)
```

### 4. Index Advisor

**File:** `index_advisor.go`

Analyzes query workload and recommends indexes to improve performance.

**Features:**
- Workload pattern analysis
- Missing index detection
- Impact estimation (speedup, storage, build time)
- Confidence scoring
- Human-readable reports

**Usage:**
```go
import "github.com/weaviate/weaviate/usecases/optimizer"

// Create index advisor
advisor := optimizer.NewIndexAdvisor(costModel)

// Analyze workload and get recommendations
recommendations := advisor.Recommend(ctx, queryWorkload)

// Generate detailed report
report := advisor.GenerateReport(ctx, queryWorkload)
fmt.Println(report)
```

## Performance Improvements

Based on RFC benchmarks, the optimizer achieves:

| Query Type | Improvement |
|------------|-------------|
| Analytical | 60% faster |
| Multi-join | 60% faster |
| Complex filter | 57% faster |
| Aggregation | 57% faster |

## Cardinality Estimation Accuracy

| Approach | Mean Error | p95 Error |
|----------|------------|-----------|
| Uniform assumption | 500% | 2000% |
| Histogram-based | 80% | 300% |
| ML-based | **15%** | **50%** |

## Testing

Run tests with:

```bash
# Run all optimizer tests
go test ./usecases/optimizer/...

# Run specific test
go test ./usecases/optimizer/ -run TestMLCostModel_EstimatePlan

# Run with coverage
go test ./usecases/optimizer/... -cover
```

## Integration

The optimizer integrates with the existing query planner in `adapters/repos/db/sorter/query_planner.go`.

To enable the optimizer:

1. Train the cardinality estimation model on your workload
2. Configure the cost factors for your hardware
3. Create materialized views for frequently accessed queries
4. Review and apply index recommendations

## Configuration

Cost factors can be tuned based on your hardware:

```go
costFactors := &optimizer.CostFactors{
    CPUPerTuple:      0.1,   // microseconds
    CPUHashBuild:     0.5,
    CPUHashProbe:     0.2,
    CPUSort:          1.0,
    CPUVectorSearch:  10.0,
    IOSequentialPage: 10.0,
    IORandomPage:     100.0,
    IOVectorRead:     50.0,
    NetworkPerKB:     5.0,
}
```

## Future Enhancements

- Automatic cost factor calibration
- Distributed query optimization
- Advanced view selection algorithms
- Real-time workload adaptation
- Integration with query cache

## References

- RFC 0019: Cost-Based Query Optimizer
- [Learned Cardinality Estimation](https://arxiv.org/abs/1809.00677)
- [PostgreSQL Query Planner](https://www.postgresql.org/docs/current/planner-optimizer.html)

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
