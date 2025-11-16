# Learned Filter Optimization

This directory contains tools for training and deploying machine learning models to optimize filter strategy selection in Weaviate's HNSW vector index.

## Overview

The learned filter optimization uses XGBoost to predict filter selectivity and automatically choose between pre-filtering (ACORN) and post-filtering strategies, achieving 20-50% improvement in filtered query latency.

## Quick Start

### 1. Enable Query Logging

Configure your vector index to log filter queries:

```json
{
  "vectorIndexConfig": {
    "learnedFilterLogEnabled": true,
    "learnedFilterLogPath": "/var/lib/weaviate/filter_queries.log"
  }
}
```

### 2. Collect Training Data

Run Weaviate with filter queries for at least a week to collect sufficient training data (recommended: 10,000+ queries).

### 3. Train the Model

```bash
pip install pandas numpy xgboost scikit-learn

python train_model.py \
  --log-file /var/lib/weaviate/filter_queries.log \
  --output-model filter_selectivity_model.json
```

### 4. Deploy the Model

Copy the trained model to your Weaviate data directory and configure:

```json
{
  "vectorIndexConfig": {
    "learnedFilterEnabled": true,
    "learnedFilterModelPath": "/var/lib/weaviate/filter_selectivity_model.json"
  }
}
```

### 5. Monitor Performance

The model will automatically predict selectivity and choose optimal strategies. Monitor your query performance metrics to validate improvements.

## Training Script Options

```bash
python train_model.py --help

Options:
  --log-file PATH       Path to filter query log file (required)
  --output-model PATH   Output path for trained model (default: filter_selectivity_model.json)
  --test-size FLOAT     Fraction of data for testing (default: 0.2)
  --min-samples INT     Minimum samples required (default: 1000)
```

## Model Features

The model uses the following features to predict selectivity:

### Numerical Features
- `property_cardinality` - Unique values in property
- `corpus_size` - Total documents in index
- `historical_selectivity_p50` - Median selectivity from past queries
- `historical_selectivity_p95` - 95th percentile selectivity
- `time_of_day_hour` - Hour of day (0-23)
- `day_of_week` - Day of week (0-6)
- `query_vector_norm` - L2 norm of query vector
- `vector_dimensions` - Dimensionality of vectors
- `filter_complexity` - Number of AND/OR/NOT operations

### Categorical Features (one-hot encoded)
- `property_name` - Property being filtered
- `operator` - Filter operator (Equal, GreaterThan, etc.)

## Performance Targets

From RFC 04-learned-filter-optimization.md:

- **MAE (Mean Absolute Error)**: < 0.05 (5% selectivity prediction error)
- **Latency Improvement**: 20%+ on filtered queries
- **Inference Overhead**: < 1% (60-120µs per query)

## Model Retraining

Retrain the model weekly (or more frequently if workload changes rapidly):

```bash
# Automated retraining (cron job example)
0 2 * * 0 python /path/to/train_model.py \
  --log-file /var/lib/weaviate/filter_queries.log \
  --output-model /var/lib/weaviate/filter_selectivity_model.json
```

The model will be hot-reloaded automatically on the next query.

## Rollout Strategy

### Phase 1: Shadow Mode (2 weeks)
Model makes predictions but doesn't affect decisions. Monitor prediction accuracy.

### Phase 2: Canary (4 weeks)
Enable for 10% of queries, gradually increase to 50%, then 100%.

### Phase 3: Default
Learned model becomes default, with feature flag to disable if needed.

## Troubleshooting

### Not Enough Training Data
```
Error: Not enough training samples. Found 500, need at least 1000
```
**Solution**: Run Weaviate longer to collect more query logs.

### Model Performance Below Target
```
⚠ Model does not meet RFC target: MAE = 0.08 (target: < 0.05)
```
**Solution**:
- Collect more diverse training data
- Ensure queries represent production workload
- Consider per-class models for varied workloads

### Model Not Loading
Check logs for errors:
- Verify model file path is correct
- Ensure model file is valid JSON
- Check file permissions

## Architecture

```
┌─────────────────────┐
│ Filter Query        │
└──────────┬──────────┘
           ↓
┌─────────────────────┐
│ Feature Extractor   │
└──────────┬──────────┘
           ↓
┌─────────────────────┐
│ XGBoost Model       │
│ (Predict Selectivity)│
└──────────┬──────────┘
           ↓
┌─────────────────────┐
│ Strategy Selector   │
│ (Pre or Post Filter)│
└──────────┬──────────┘
           ↓
┌─────────────────────┐
│ Execute Search      │
└──────────┬──────────┘
           ↓
┌─────────────────────┐
│ Log Actual Results  │
└─────────────────────┘
```

## References

- RFC: `rfcs/04-learned-filter-optimization.md`
- XGBoost Documentation: https://xgboost.readthedocs.io
- Current ACORN Implementation: `adapters/repos/db/vector/hnsw/search.go`
