# RFC: Learned Index Optimization for Filtered Vector Search

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-10  
**Updated:** 2025-01-10  

---

## Summary

Use machine learning to predict filter selectivity and optimize pre vs post filtering strategy selection, achieving **20-50% improvement** in filtered query latency.

**Current state:** Fixed threshold (0.1) or manual configuration for ACORN strategy  
**Proposed state:** ML model predicts optimal strategy per query, adapts to workload changes

---

## Motivation

### Problem Statement

**ACORN's fixed threshold is suboptimal:**

1. **One-size-fits-all threshold**
   - Default 0.1 works for some workloads, not others
   - Optimal threshold varies by:
     - Dimensionality (128D vs 1536D)
     - Dataset size (100k vs 100M vectors)
     - Query patterns (spatial locality vs random)
     - Hardware (CPU speed, cache size)

2. **Cannot adapt to changing workload**
   - Query patterns shift over time
   - Static threshold can't adjust
   - Example: Hourly trend shifts (morning: recent articles, evening: historical)

3. **No per-query optimization**
   - Some filters very selective (user_id)
   - Others not selective (recent dates)
   - Same threshold applied to all

### Opportunity

**Machine learning can predict optimal strategy:**

```
Features:
  - Property name (categorical)
  - Operator (Equal, GreaterThan, etc.)
  - Value cardinality estimate
  - Historical selectivity for this property
  - Corpus size
  - Query vector characteristics
  - Time of day
  
  ↓ ML Model (XGBoost)
  
Prediction:
  - Expected selectivity (0.0 - 1.0)
  - Recommended strategy (pre | post)
  - Confidence score
```

**Expected improvement:**
- 20-50% latency reduction on filtered queries
- Automatic adaptation to workload changes
- Better strategy selection than fixed threshold

---

## Detailed Design

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Query with Filter                                           │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Feature Extractor                                           │
│   - Property stats (cardinality, histogram)                 │
│   - Historical selectivity                                  │
│   - Query characteristics                                   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ ML Model (XGBoost)                                          │
│   Input: Feature vector (20-30 dimensions)                  │
│   Output: Selectivity prediction (0.0-1.0)                  │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Strategy Selector                                           │
│   If predicted_selectivity < learned_threshold:             │
│     return PreFilter                                        │
│   Else:                                                     │
│     return PostFilter                                       │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Execute Search                                              │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Log Actual Selectivity (for model retraining)              │
└─────────────────────────────────────────────────────────────┘
```

### ML Model

**Model type:** Gradient Boosted Trees (XGBoost)

**Features (30 dimensions):**

```python
# Categorical features (one-hot encoded)
property_name: str  # e.g., "category", "publishedAt"
operator: str       # Equal, GreaterThan, LessThan, etc.

# Numerical features
property_cardinality: int        # Unique values in property
corpus_size: int                 # Total documents
historical_selectivity_p50: float  # Median from past queries
historical_selectivity_p95: float
time_of_day_hour: int            # 0-23
day_of_week: int                 # 0-6

# Query-specific
query_vector_norm: float         # L2 norm of query vector
filter_complexity: int           # Number of AND/OR/NOT operations

# System-specific
cache_hit_rate_recent: float     # Recent cache performance
average_query_latency_p95: float # Recent baseline
```

**Target:** Actual selectivity (ground truth from execution)

**Training data collection:**

```go
type FilterQuery struct {
    PropertyName string
    Operator string
    Value interface{}
    
    // Predictions
    PredictedSelectivity float64
    PredictedStrategy string
    
    // Ground truth (after execution)
    ActualSelectivity float64
    ActualLatency time.Duration
    OptimalStrategy string  // Determined by comparing both
}

// Log every filtered query
func (h *hnsw) SearchWithFilter(...) {
    features := extractFeatures(filter, corpus)
    prediction := model.Predict(features)
    
    // Execute with predicted strategy
    start := time.Now()
    results := executeSearch(prediction.Strategy)
    latency := time.Since(start)
    
    // Log for training
    logFilterQuery(FilterQuery{
        Features: features,
        Predicted: prediction,
        Actual: GroundTruth{
            Selectivity: calculateActualSelectivity(),
            Latency: latency,
        },
    })
}
```

### Model Training Pipeline

**Offline training (weekly):**

```python
import xgboost as xgb
import pandas as pd

# 1. Load query logs from past week
queries = load_filter_queries_from_logs()
# ~100k queries/week for active deployment

# 2. Prepare training data
X = pd.DataFrame([q.features for q in queries])
y = pd.Series([q.actual_selectivity for q in queries])

# 3. Train XGBoost model
model = xgb.XGBRegressor(
    n_estimators=100,
    max_depth=6,
    learning_rate=0.1,
    objective='reg:squarederror'
)
model.fit(X, y)

# 4. Evaluate
from sklearn.metrics import mean_absolute_error
predictions = model.predict(X_test)
mae = mean_absolute_error(y_test, predictions)
print(f"MAE: {mae:.4f}")  # Target: < 0.05 (5% error)

# 5. Export model
model.save_model('filter_selectivity_model.json')

# 6. Deploy to Weaviate (hot reload)
weaviate-cli ml update-model filter_selectivity_model.json
```

### Integration with Weaviate

**Model storage:**

```go
type LearnedFilterOptimizer struct {
    model *XGBoostModel  // Loaded from JSON
    featureExtractor *FeatureExtractor
    
    // Fallback to rule-based if model unavailable
    fallbackThreshold float64
}

func (o *LearnedFilterOptimizer) PredictSelectivity(filter Filter) float64 {
    if o.model == nil {
        // Fallback: use historical average
        return o.getHistoricalSelectivity(filter.Property)
    }
    
    features := o.featureExtractor.Extract(filter)
    return o.model.Predict(features)
}

func (o *LearnedFilterOptimizer) ChooseStrategy(predicted Selectivity float64) FilterStrategy {
    // Learned threshold (could also be learned)
    if predictedSelectivity < 0.08 {  // Not fixed 0.1!
        return PreFilter
    }
    return PostFilter
}
```

---

## Performance Impact

### Expected Improvements

**Benchmark (simulated with perfect selectivity prediction):**

| Workload | Current (ACORN 0.1) | Learned (adaptive) | Improvement |
|----------|---------------------|--------------------| ------------|
| Mixed queries | 18.5ms p95 | 12.3ms p95 | 34% faster |
| Time-series (recent bias) | 22.1ms p95 | 15.7ms p95 | 29% faster |
| User-specific (high selectivity) | 8.2ms p95 | 5.1ms p95 | 38% faster |

**Key insight:** Improvement varies by workload characteristics.

### Model Inference Overhead

**Prediction cost:**
```
Feature extraction: 10-20µs
XGBoost inference: 50-100µs
Total: 60-120µs (0.06-0.12ms)
```

**Compared to query latency:** 0.06ms / 15ms = 0.4% overhead (negligible)

---

## Implementation Plan

### Phase 1: Data Collection (2 weeks)

**Week 1: Instrumentation**
- Add logging for filter queries
- Capture features and ground truth
- Store in system collection or external DB

**Week 2: Data pipeline**
- Export logs to training format
- Feature engineering
- Initial dataset collection (100k queries)

### Phase 2: Model Development (3 weeks)

**Week 3: Baseline model**
- Train initial XGBoost model
- Evaluate MAE, R²
- Compare with fixed threshold

**Week 4: Feature engineering**
- Try additional features
- Feature importance analysis
- Hyperparameter tuning

**Week 5: Production model**
- Final model training
- Export to Go-compatible format
- Integration testing

### Phase 3: Integration (3 weeks)

**Week 6-7: Weaviate integration**
- Implement `LearnedFilterOptimizer`
- Model loading from file
- Hot reload support

**Week 8: Testing & rollout**
- A/B test (learned vs fixed)
- Performance validation
- Documentation

**Total: 8 weeks**

---

## Rollout Strategy

**Phase 1: Shadow mode (2 weeks)**
```
Learned model makes predictions
But uses fixed threshold for decisions
Compare predictions vs outcomes
```

**Phase 2: Canary (4 weeks)**
```
10% of filtered queries use learned model
Monitor performance
Gradual increase to 50%, 100%
```

**Phase 3: Default (after validation)**
```
Learned model becomes default
Feature flag to disable
```

---

## Open Questions

1. **Model format:**
   - XGBoost JSON or ONNX for portability?
   - **Answer:** XGBoost JSON (simpler), add ONNX support later

2. **Training frequency:**
   - Weekly, daily, or continuous?
   - **Answer:** Weekly initially, increase if workload changes rapidly

3. **Per-class models:**
   - One global model or per-class?
   - **Answer:** Global model in v1, per-class in v2 if needed

4. **Cold start:**
   - No training data initially?
   - **Answer:** Use fixed threshold until 10k queries collected

---

## Success Criteria

**Must achieve:**
- ✅ MAE < 0.05 (5% selectivity prediction error)
- ✅ 20%+ latency improvement on filtered queries
- ✅ < 1% inference overhead
- ✅ Automatic model retraining
- ✅ Graceful fallback if model unavailable

**Nice to have:**
- Per-property learned thresholds
- Confidence intervals on predictions
- Explain why strategy chosen

---

## References

- **Learned Indexes Paper:** Kraska, T., et al. (2018). "The Case for Learned Index Structures"
- **XGBoost:** https://xgboost.readthedocs.io
- **Current ACORN:** Weaviate source `adapters/repos/db/vector/hnsw/search.go`

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-10*