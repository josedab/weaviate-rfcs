//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package hnsw

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/weaviate/weaviate/adapters/repos/db/helpers"
	"github.com/weaviate/weaviate/entities/filters"
)

// SelectivityPrediction represents the model's prediction
type SelectivityPrediction struct {
	Selectivity     float64 // Predicted selectivity (0.0 - 1.0)
	Strategy        string  // Recommended strategy ("pre_filter" or "post_filter")
	ConfidenceScore float64 // Confidence in prediction (0.0 - 1.0)
}

// LearnedFilterOptimizer uses ML to predict optimal filter strategy
type LearnedFilterOptimizer struct {
	enabled           bool
	model             MLModel
	featureExtractor  *FeatureExtractor
	fallbackThreshold float64
	learnedThreshold  float64
	mu                sync.RWMutex
}

// MLModel interface for different model implementations
type MLModel interface {
	Predict(features *FilterQueryFeatures) (*SelectivityPrediction, error)
	IsLoaded() bool
}

// NewLearnedFilterOptimizer creates a new learned filter optimizer
func NewLearnedFilterOptimizer(
	enabled bool,
	modelPath string,
	fallbackThreshold float64,
) (*LearnedFilterOptimizer, error) {
	optimizer := &LearnedFilterOptimizer{
		enabled:           enabled,
		featureExtractor:  NewFeatureExtractor(),
		fallbackThreshold: fallbackThreshold,
		learnedThreshold:  0.08, // Learned threshold (can be adjusted)
	}

	if enabled && modelPath != "" {
		// Try to load the model
		model, err := LoadXGBoostModel(modelPath)
		if err != nil {
			// Log error but continue with fallback
			fmt.Printf("Failed to load ML model from %s: %v. Using fallback threshold.\n", modelPath, err)
		} else {
			optimizer.model = model
		}
	}

	return optimizer, nil
}

// PredictSelectivity predicts the selectivity for a given filter
func (o *LearnedFilterOptimizer) PredictSelectivity(
	filter *filters.Clause,
	allowList helpers.AllowList,
	queryVector []float32,
	corpusSize int,
) (*SelectivityPrediction, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Extract features
	features := o.featureExtractor.ExtractFeatures(filter, allowList, queryVector, corpusSize)

	// If model is available, use it
	if o.model != nil && o.model.IsLoaded() {
		prediction, err := o.model.Predict(features)
		if err != nil {
			// Fall back to historical statistics
			return o.fallbackPrediction(features), nil
		}
		return prediction, nil
	}

	// Fallback to historical statistics or simple heuristic
	return o.fallbackPrediction(features), nil
}

func (o *LearnedFilterOptimizer) fallbackPrediction(features *FilterQueryFeatures) *SelectivityPrediction {
	// Use historical selectivity if available
	selectivity := features.HistoricalSelectivityP50
	if selectivity == 0.0 {
		// No historical data, use corpus-based heuristic
		if features.PropertyCardinality > 0 && features.CorpusSize > 0 {
			selectivity = float64(features.PropertyCardinality) / float64(features.CorpusSize)
		} else {
			// Default to conservative estimate
			selectivity = 0.5
		}
	}

	strategy := "post_filter"
	if selectivity < o.learnedThreshold {
		strategy = "pre_filter"
	}

	return &SelectivityPrediction{
		Selectivity:     selectivity,
		Strategy:        strategy,
		ConfidenceScore: 0.5, // Low confidence for fallback
	}
}

// ShouldUsePreFilter determines whether to use pre-filtering strategy
func (o *LearnedFilterOptimizer) ShouldUsePreFilter(
	prediction *SelectivityPrediction,
) bool {
	if !o.enabled {
		return false
	}
	return prediction.Strategy == "pre_filter"
}

// UpdateFeatureStats updates historical statistics after query execution
func (o *LearnedFilterOptimizer) UpdateFeatureStats(
	propertyName string,
	actualSelectivity float64,
	cardinality int,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.featureExtractor.UpdatePropertyStats(propertyName, actualSelectivity, cardinality)
}

// ReloadModel reloads the ML model from disk (for hot reload)
func (o *LearnedFilterOptimizer) ReloadModel(modelPath string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	model, err := LoadXGBoostModel(modelPath)
	if err != nil {
		return fmt.Errorf("failed to reload model: %w", err)
	}

	o.model = model
	return nil
}

// IsEnabled returns whether the optimizer is enabled
func (o *LearnedFilterOptimizer) IsEnabled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.enabled
}

// XGBoostModel implements MLModel interface for XGBoost models
type XGBoostModel struct {
	modelData map[string]interface{}
	loaded    bool
}

// LoadXGBoostModel loads an XGBoost model from JSON file
func LoadXGBoostModel(modelPath string) (*XGBoostModel, error) {
	data, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	var modelData map[string]interface{}
	if err := json.Unmarshal(data, &modelData); err != nil {
		return nil, fmt.Errorf("failed to parse model JSON: %w", err)
	}

	return &XGBoostModel{
		modelData: modelData,
		loaded:    true,
	}, nil
}

// Predict makes a prediction using the XGBoost model
func (m *XGBoostModel) Predict(features *FilterQueryFeatures) (*SelectivityPrediction, error) {
	if !m.loaded {
		return nil, fmt.Errorf("model not loaded")
	}

	// TODO: Implement actual XGBoost inference
	// For now, return a simple heuristic-based prediction
	// In production, this would use the actual XGBoost model

	// Simple heuristic: if historical selectivity is available, use it
	selectivity := features.HistoricalSelectivityP50
	if selectivity == 0.0 {
		// Estimate based on property cardinality
		if features.PropertyCardinality > 0 && features.CorpusSize > 0 {
			selectivity = float64(features.PropertyCardinality) / float64(features.CorpusSize)
		} else {
			selectivity = 0.5 // Default
		}
	}

	// Adjust based on operator type
	switch features.Operator {
	case "Equal":
		// Equal is typically very selective
		selectivity *= 0.5
	case "GreaterThan", "LessThan":
		// Range queries are typically less selective
		selectivity *= 1.5
	case "Like":
		// Like queries vary widely
		selectivity *= 1.0
	}

	// Clamp to [0, 1]
	if selectivity > 1.0 {
		selectivity = 1.0
	}
	if selectivity < 0.0 {
		selectivity = 0.0
	}

	strategy := "post_filter"
	if selectivity < 0.08 {
		strategy = "pre_filter"
	}

	return &SelectivityPrediction{
		Selectivity:     selectivity,
		Strategy:        strategy,
		ConfidenceScore: 0.7, // Moderate confidence
	}, nil
}

// IsLoaded returns whether the model is loaded
func (m *XGBoostModel) IsLoaded() bool {
	return m.loaded
}
