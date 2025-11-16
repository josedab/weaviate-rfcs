#!/usr/bin/env python3
"""
Training script for learned filter optimization model.

This script trains an XGBoost model to predict filter selectivity
for optimal pre-filter vs post-filter strategy selection.

Usage:
    python train_model.py --log-file /path/to/filter_queries.log --output-model model.json
"""

import argparse
import json
import sys
from pathlib import Path
from typing import List, Dict, Any

try:
    import pandas as pd
    import numpy as np
    import xgboost as xgb
    from sklearn.model_selection import train_test_split
    from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score
except ImportError as e:
    print(f"Error: Required dependencies not found: {e}")
    print("Please install required packages:")
    print("  pip install pandas numpy xgboost scikit-learn")
    sys.exit(1)


def load_query_logs(log_file: Path) -> List[Dict[str, Any]]:
    """Load filter query logs from JSONL file."""
    queries = []
    with open(log_file, 'r') as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    queries.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
    return queries


def extract_features(queries: List[Dict[str, Any]]) -> pd.DataFrame:
    """Extract features from query logs into a DataFrame."""
    features_list = []

    for query in queries:
        features = query.get('features', {})

        # Extract numerical features
        feature_dict = {
            'property_cardinality': features.get('PropertyCardinality', 0),
            'corpus_size': features.get('CorpusSize', 0),
            'historical_selectivity_p50': features.get('HistoricalSelectivityP50', 0.0),
            'historical_selectivity_p95': features.get('HistoricalSelectivityP95', 0.0),
            'time_of_day_hour': features.get('TimeOfDayHour', 0),
            'day_of_week': features.get('DayOfWeek', 0),
            'query_vector_norm': features.get('QueryVectorNorm', 0.0),
            'vector_dimensions': features.get('VectorDimensions', 0),
            'filter_complexity': features.get('FilterComplexity', 1),
            'cache_hit_rate_recent': features.get('CacheHitRateRecent', 0.0),
            'average_query_latency_p95': features.get('AverageQueryLatencyP95', 0),

            # Target variable
            'actual_selectivity': query.get('actual_selectivity', 0.0),
        }

        # One-hot encode categorical features
        property_name = features.get('PropertyName', 'unknown')
        operator = features.get('Operator', 'unknown')

        feature_dict[f'property_{property_name}'] = 1
        feature_dict[f'operator_{operator}'] = 1

        features_list.append(feature_dict)

    df = pd.DataFrame(features_list)

    # Fill missing categorical features with 0
    df = df.fillna(0)

    return df


def train_model(X_train, y_train, X_test, y_test) -> xgb.XGBRegressor:
    """Train XGBoost model to predict selectivity."""
    print("Training XGBoost model...")

    model = xgb.XGBRegressor(
        n_estimators=100,
        max_depth=6,
        learning_rate=0.1,
        objective='reg:squarederror',
        random_state=42,
        verbosity=1,
    )

    model.fit(X_train, y_train)

    # Evaluate on test set
    y_pred = model.predict(X_test)

    mae = mean_absolute_error(y_test, y_pred)
    mse = mean_squared_error(y_test, y_pred)
    rmse = np.sqrt(mse)
    r2 = r2_score(y_test, y_pred)

    print(f"\nModel Performance on Test Set:")
    print(f"  MAE (Mean Absolute Error): {mae:.4f}")
    print(f"  RMSE (Root Mean Squared Error): {rmse:.4f}")
    print(f"  R² Score: {r2:.4f}")

    # Check if MAE meets target from RFC (< 0.05)
    if mae < 0.05:
        print("✓ Model meets RFC target: MAE < 0.05")
    else:
        print(f"⚠ Model does not meet RFC target: MAE = {mae:.4f} (target: < 0.05)")

    return model


def feature_importance_analysis(model, feature_names):
    """Analyze and display feature importance."""
    print("\nFeature Importance (top 10):")

    importance = model.feature_importances_
    feature_importance = sorted(
        zip(feature_names, importance),
        key=lambda x: x[1],
        reverse=True
    )

    for i, (feature, importance) in enumerate(feature_importance[:10], 1):
        print(f"  {i:2d}. {feature:40s} {importance:.4f}")


def main():
    parser = argparse.ArgumentParser(
        description="Train XGBoost model for filter selectivity prediction"
    )
    parser.add_argument(
        '--log-file',
        type=Path,
        required=True,
        help='Path to filter query log file (JSONL format)'
    )
    parser.add_argument(
        '--output-model',
        type=Path,
        default='filter_selectivity_model.json',
        help='Output path for trained model (JSON format)'
    )
    parser.add_argument(
        '--test-size',
        type=float,
        default=0.2,
        help='Fraction of data to use for testing (default: 0.2)'
    )
    parser.add_argument(
        '--min-samples',
        type=int,
        default=1000,
        help='Minimum number of samples required for training (default: 1000)'
    )

    args = parser.parse_args()

    # Check if log file exists
    if not args.log_file.exists():
        print(f"Error: Log file not found: {args.log_file}")
        sys.exit(1)

    # Load and process data
    print(f"Loading query logs from {args.log_file}...")
    queries = load_query_logs(args.log_file)

    if len(queries) < args.min_samples:
        print(f"Error: Not enough training samples. Found {len(queries)}, need at least {args.min_samples}")
        print("Collect more query logs before training.")
        sys.exit(1)

    print(f"Loaded {len(queries)} query logs")

    # Extract features
    print("Extracting features...")
    df = extract_features(queries)

    # Separate features and target
    target_col = 'actual_selectivity'
    feature_cols = [col for col in df.columns if col != target_col]

    X = df[feature_cols]
    y = df[target_col]

    # Split data
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=args.test_size, random_state=42
    )

    print(f"Training set: {len(X_train)} samples")
    print(f"Test set: {len(X_test)} samples")

    # Train model
    model = train_model(X_train, y_train, X_test, y_test)

    # Feature importance
    feature_importance_analysis(model, feature_cols)

    # Save model
    print(f"\nSaving model to {args.output_model}...")
    model.save_model(str(args.output_model))

    print("\n✓ Model training complete!")
    print(f"\nTo deploy the model:")
    print(f"  1. Copy {args.output_model} to your Weaviate data directory")
    print(f"  2. Configure the vector index with:")
    print(f"     \"learnedFilterEnabled\": true,")
    print(f"     \"learnedFilterModelPath\": \"/path/to/{args.output_model.name}\"")


if __name__ == '__main__':
    main()
