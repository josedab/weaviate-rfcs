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

package db

import (
	"fmt"
	"sort"
	"time"

	"github.com/weaviate/weaviate/entities/schema"
	"github.com/weaviate/weaviate/entities/storobj"
	"github.com/weaviate/weaviate/entities/timedecay"
)

// applyTimeDecay applies time decay scoring to search results
// It extracts timestamps from the specified property, calculates decay factors,
// and re-ranks results based on combined vector similarity and temporal relevance
func applyTimeDecay(objects []*storobj.Object, distances []float32,
	timeDecayConfig *timedecay.Config, originalLimit int) ([]*storobj.Object, []float32, error) {
	if timeDecayConfig == nil || len(objects) == 0 {
		return objects, distances, nil
	}

	now := time.Now()
	type scoredResult struct {
		obj             *storobj.Object
		originalDist    float32
		decayFactor     float32
		combinedScore   float32
		originalRank    int
		timestampExists bool
	}

	scored := make([]scoredResult, len(objects))

	// Calculate decay factors for all results
	for i := range objects {
		result := scoredResult{
			obj:          objects[i],
			originalDist: distances[i],
			originalRank: i,
			decayFactor:  1.0, // default to no decay
		}

		// Extract timestamp from property
		if objects[i].Object.Properties != nil {
			props, ok := objects[i].Object.Properties.(map[string]interface{})
			if ok {
				if timestampVal, exists := props[timeDecayConfig.Property]; exists {
					result.timestampExists = true
					timestamp, err := parseTimestamp(timestampVal)
					if err == nil {
						age := now.Sub(timestamp)
						result.decayFactor = timeDecayConfig.CalculateDecay(age)
					}
				}
			}
		}

		// Convert distance to similarity score (assuming cosine distance)
		// For cosine distance: similarity = 1 - distance
		// For other distance metrics, this might need adjustment
		similarity := float32(1.0) - result.originalDist
		if similarity < 0 {
			similarity = 0
		}

		// Apply time decay to similarity
		decayedSimilarity := similarity * result.decayFactor

		// Convert back to distance-like metric (lower is better)
		// We use negative decayed similarity so that sorting by score (ascending)
		// gives us the best results first
		result.combinedScore = -decayedSimilarity

		scored[i] = result
	}

	// Sort by combined score (ascending, since we negated the similarity)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].combinedScore < scored[j].combinedScore
	})

	// Truncate to original limit
	limit := originalLimit
	if limit > len(scored) {
		limit = len(scored)
	}

	// Extract re-ranked objects and distances
	rerankedObjects := make([]*storobj.Object, limit)
	rerankedDistances := make([]float32, limit)

	for i := 0; i < limit; i++ {
		rerankedObjects[i] = scored[i].obj
		// Store the original distance for now
		// In the future, we might want to expose the combined score
		rerankedDistances[i] = scored[i].originalDist
	}

	return rerankedObjects, rerankedDistances, nil
}

// parseTimestamp attempts to parse a timestamp from various formats
func parseTimestamp(val interface{}) (time.Time, error) {
	switch v := val.(type) {
	case string:
		// Try RFC3339 format (ISO 8601)
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		// Try RFC3339 with nanoseconds
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t, nil
		}
		// Try other common formats
		if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
			return t, nil
		}
		if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
			return t, nil
		}
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", v)
	case time.Time:
		return v, nil
	case int64:
		// Assume Unix timestamp in milliseconds
		return time.Unix(0, v*int64(time.Millisecond)), nil
	case float64:
		// Assume Unix timestamp in seconds
		return time.Unix(int64(v), 0), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type: %T", val)
	}
}

// calculateOverFetchLimit calculates the limit with over-fetch multiplier applied
func calculateOverFetchLimit(originalLimit int, timeDecayConfig *timedecay.Config) int {
	if timeDecayConfig == nil {
		return originalLimit
	}

	multiplier := timeDecayConfig.GetOverFetchMultiplier()
	overFetchLimit := int(float32(originalLimit) * multiplier)

	// Cap at a reasonable maximum to avoid excessive memory usage
	maxOverFetch := 10000
	if overFetchLimit > maxOverFetch {
		overFetchLimit = maxOverFetch
	}

	return overFetchLimit
}

// isDateTimeProperty checks if a property is a datetime type
func isDateTimeProperty(class *schema.Schema, className, propertyName string) bool {
	if class == nil {
		return false
	}

	classSchema := class.FindClassByName(schema.ClassName(className))
	if classSchema == nil {
		return false
	}

	prop, err := schema.GetPropertyByName(classSchema, propertyName)
	if err != nil {
		return false
	}

	// Check if the property is a date type
	dataType, err := schema.AsPrimitive(prop.DataType)
	if err != nil {
		return false
	}

	return dataType == schema.DataTypeDate
}
