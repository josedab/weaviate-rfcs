# Learned Filter Optimization

## Overview

The learned filter optimization feature uses machine learning to predict filter selectivity and automatically select the optimal strategy (pre-filtering vs post-filtering) for filtered vector searches. This can achieve **20-50% improvement** in filtered query latency compared to using a fixed threshold.

## Background

### Current Approach (ACORN with Fixed Threshold)

Weaviate's ACORN (Adaptive Cluster Optimization for Refined Neighbors) strategy uses a fixed threshold (default: 0.1 or 10%) to decide between pre-filtering and post-filtering:

- **Selectivity < 10%**: Use pre-filtering (ACORN strategy)
- **Selectivity ≥ 10%**: Use post-filtering (SWEEPING/RRE strategy)

**Problems:**
- One-size-fits-all threshold doesn't work for all workloads
- Cannot adapt to changing query patterns
- Same threshold applied to all filters regardless of characteristics

### Machine Learning Approach

The learned filter optimization uses an XGBoost model to predict selectivity based on:

- Property name and operator type
- Historical selectivity for the property
- Query vector characteristics
- Time-based patterns
- Corpus size and filter complexity

## Architecture

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
│   Input: Feature vector (30 dimensions)                     │
│   Output: Selectivity prediction (0.0-1.0)                  │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Strategy Selector                                           │
│   If predicted_selectivity < 0.08:                          │
│     return PreFilter (ACORN)                                │
│   Else:                                                     │
│     return PostFilter (SWEEPING/RRE)                        │
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

## Configuration

### Vector Index Configuration

```json
{
  "class": "Article",
  "vectorIndexConfig": {
    "filterStrategy": "acorn",

    // Learned filter optimization
    "learnedFilterEnabled": true,
    "learnedFilterModelPath": "/var/lib/weaviate/models/filter_selectivity_model.json",

    // Optional: Enable logging for training data collection
    "learnedFilterLogEnabled": true,
    "learnedFilterLogPath": "/var/lib/weaviate/logs/filter_queries.log"
  }
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `learnedFilterEnabled` | boolean | `false` | Enable learned filter optimization |
| `learnedFilterModelPath` | string | `""` | Path to XGBoost model JSON file |
| `learnedFilterLogEnabled` | boolean | `false` | Enable logging of filter queries for training |
| `learnedFilterLogPath` | string | `""` | Path to filter query log file |

## Usage

### Phase 1: Data Collection

1. **Enable query logging:**

```json
{
  "vectorIndexConfig": {
    "learnedFilterLogEnabled": true,
    "learnedFilterLogPath": "/var/lib/weaviate/filter_queries.log"
  }
}
```

2. **Run production workload** for at least 1-2 weeks to collect sufficient training data (recommended: 10,000+ queries)

### Phase 2: Model Training

1. **Install dependencies:**

```bash
pip install pandas numpy xgboost scikit-learn
```

2. **Train the model:**

```bash
cd tools/learned_filter

python train_model.py \
  --log-file /var/lib/weaviate/filter_queries.log \
  --output-model filter_selectivity_model.json
```

3. **Verify model performance:**

The script will output metrics:
- **MAE (Mean Absolute Error)**: Target < 0.05
- **RMSE**: Root mean squared error
- **R² Score**: Model fit quality

### Phase 3: Deployment

1. **Copy model to Weaviate data directory:**

```bash
cp filter_selectivity_model.json /var/lib/weaviate/models/
```

2. **Update vector index configuration:**

```json
{
  "vectorIndexConfig": {
    "learnedFilterEnabled": true,
    "learnedFilterModelPath": "/var/lib/weaviate/models/filter_selectivity_model.json"
  }
}
```

3. **Restart Weaviate or recreate the index**

The model will be loaded and used for all subsequent filtered queries.

## Features Used by the Model

### Categorical Features (one-hot encoded)
- **Property name**: The property being filtered (e.g., "category", "publishedAt")
- **Operator**: Filter operator (Equal, GreaterThan, LessThan, etc.)

### Numerical Features
- **Property cardinality**: Estimated unique values in property
- **Corpus size**: Total documents in index
- **Historical selectivity (p50)**: Median selectivity from past queries
- **Historical selectivity (p95)**: 95th percentile selectivity
- **Time of day hour**: 0-23
- **Day of week**: 0-6 (Sunday = 0)
- **Query vector norm**: L2 norm of query vector
- **Vector dimensions**: Dimensionality of vectors
- **Filter complexity**: Number of AND/OR/NOT operations

## Performance Impact

### Expected Improvements

Based on RFC simulations:

| Workload | Current (ACORN 0.1) | Learned (adaptive) | Improvement |
|----------|---------------------|--------------------| ------------|
| Mixed queries | 18.5ms p95 | 12.3ms p95 | 34% faster |
| Time-series (recent bias) | 22.1ms p95 | 15.7ms p95 | 29% faster |
| User-specific (high selectivity) | 8.2ms p95 | 5.1ms p95 | 38% faster |

### Model Inference Overhead

- **Feature extraction**: 10-20µs
- **XGBoost inference**: 50-100µs
- **Total**: 60-120µs (0.06-0.12ms)
- **Relative overhead**: ~0.4% of typical query latency (negligible)

## Model Retraining

### Recommended Schedule

- **Initial**: Weekly retraining
- **Stable workloads**: Bi-weekly or monthly
- **Changing workloads**: Daily or more frequent

### Automated Retraining (Cron Job Example)

```bash
# Retrain every Sunday at 2 AM
0 2 * * 0 python /opt/weaviate/tools/learned_filter/train_model.py \
  --log-file /var/lib/weaviate/filter_queries.log \
  --output-model /var/lib/weaviate/models/filter_selectivity_model.json
```

### Hot Reload

The model can be reloaded without restarting Weaviate:

```bash
# Copy new model
cp new_model.json /var/lib/weaviate/models/filter_selectivity_model.json

# The model will be automatically reloaded on the next query
```

## Monitoring

### Success Metrics

**Model Performance:**
- MAE < 0.05 (5% selectivity prediction error)
- R² Score > 0.8

**Query Performance:**
- 20%+ latency improvement on filtered queries
- < 1% inference overhead

### Logging

Filter query logs are in JSONL format:

```json
{
  "timestamp": "2025-01-15T10:30:45Z",
  "features": {
    "PropertyName": "category",
    "Operator": "Equal",
    "PropertyCardinality": 150,
    "CorpusSize": 100000,
    "HistoricalSelectivityP50": 0.08,
    "VectorDimensions": 384
  },
  "predicted_selectivity": 0.075,
  "predicted_strategy": "pre_filter",
  "actual_selectivity": 0.082,
  "actual_latency": 15000000,
  "filtered_count": 8200,
  "total_count": 100000
}
```

## Troubleshooting

### Model Not Loading

**Symptoms**: Queries still use fixed threshold

**Check:**
1. Model file path is correct
2. Model file is valid JSON
3. File permissions allow Weaviate to read
4. Check logs for errors

**Solution:**
```bash
# Verify file exists and is readable
ls -la /var/lib/weaviate/models/filter_selectivity_model.json

# Check Weaviate logs
grep "learned filter" /var/log/weaviate/weaviate.log
```

### Poor Model Performance

**Symptoms**: High MAE (> 0.05), no latency improvement

**Possible causes:**
- Insufficient training data
- Training data not representative of production
- Workload has changed since training

**Solution:**
1. Collect more diverse training data
2. Ensure training data matches production workload
3. Retrain model more frequently
4. Consider per-class models for varied workloads

### Logging Not Working

**Check:**
1. `learnedFilterLogEnabled` is true
2. Log path is writable by Weaviate
3. Disk space available

**Solution:**
```bash
# Create log directory if needed
mkdir -p /var/lib/weaviate/logs
chown weaviate:weaviate /var/lib/weaviate/logs

# Check disk space
df -h /var/lib/weaviate
```

## Rollout Best Practices

### Phase 1: Shadow Mode (2 weeks)
- Enable logging only
- Model makes predictions but doesn't affect decisions
- Monitor prediction accuracy vs actual selectivity

### Phase 2: Canary (4 weeks)
- Enable for one shard or class
- Monitor performance metrics
- Gradually expand to more shards/classes

### Phase 3: Full Deployment
- Enable for all classes
- Keep feature flag to disable if needed
- Continue monitoring and retraining

## Implementation Details

### Code Structure

```
adapters/repos/db/vector/hnsw/
├── learned_filter_features.go      # Feature extraction
├── learned_filter_optimizer.go     # ML model integration
├── learned_filter_logger.go        # Query logging
├── learned_filter_optimizer_test.go # Unit tests
└── search.go                       # Integration point

tools/learned_filter/
├── train_model.py                  # Training script
└── README.md                       # Usage guide

entities/vectorindex/hnsw/
└── config.go                       # Configuration options
```

### Key Functions

- `acornEnabledWithVector()`: Decision point for strategy selection
- `PredictSelectivity()`: ML model prediction
- `ExtractFeatures()`: Feature extraction from queries
- `LogQuery()`: Training data collection

## References

- **RFC**: `rfcs/04-learned-filter-optimization.md`
- **Training Tools**: `tools/learned_filter/`
- **Implementation**: `adapters/repos/db/vector/hnsw/learned_filter_*.go`
- **Configuration**: `entities/vectorindex/hnsw/config.go`
- **XGBoost Documentation**: https://xgboost.readthedocs.io
