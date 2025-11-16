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

package common_filters

import (
	"fmt"

	"github.com/weaviate/weaviate/entities/searchparams"
)

// ExtractTimeDecay extracts time decay parameters from GraphQL arguments
func ExtractTimeDecay(source map[string]interface{}) (*searchparams.TimeDecay, error) {
	timeDecayRaw, ok := source["timeDecay"]
	if !ok {
		return nil, nil
	}

	timeDecayMap, ok := timeDecayRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("timeDecay must be an object")
	}

	timeDecay := &searchparams.TimeDecay{}

	// Extract property (required)
	if prop, ok := timeDecayMap["property"].(string); ok {
		timeDecay.Property = prop
	} else {
		return nil, fmt.Errorf("timeDecay.property is required and must be a string")
	}

	// Extract decayFunction (required)
	if fn, ok := timeDecayMap["decayFunction"].(string); ok {
		timeDecay.DecayFunction = fn
	} else {
		return nil, fmt.Errorf("timeDecay.decayFunction is required and must be a string")
	}

	// Extract optional parameters based on decay function
	if halfLife, ok := timeDecayMap["halfLife"].(string); ok {
		timeDecay.HalfLife = halfLife
	}

	if maxAge, ok := timeDecayMap["maxAge"].(string); ok {
		timeDecay.MaxAge = maxAge
	}

	// Extract step thresholds if provided
	if stepsRaw, ok := timeDecayMap["stepThresholds"]; ok {
		steps, ok := stepsRaw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("timeDecay.stepThresholds must be an array")
		}

		timeDecay.StepThresholds = make([]searchparams.TimeDecayStepThreshold, len(steps))
		for i, stepRaw := range steps {
			stepMap, ok := stepRaw.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("each step threshold must be an object")
			}

			maxAge, ok := stepMap["maxAge"].(string)
			if !ok {
				return nil, fmt.Errorf("step threshold maxAge is required and must be a string")
			}

			weight, ok := stepMap["weight"].(float64)
			if !ok {
				return nil, fmt.Errorf("step threshold weight is required and must be a number")
			}

			timeDecay.StepThresholds[i] = searchparams.TimeDecayStepThreshold{
				MaxAge: maxAge,
				Weight: float32(weight),
			}
		}
	}

	return timeDecay, nil
}
