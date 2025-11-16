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

	"github.com/tailor-inc/graphql"
)

// TimeDecayArgument creates the GraphQL argument configuration for time decay
func TimeDecayArgument(prefix string, className string) *graphql.ArgumentConfig {
	decayFunctionEnum := graphql.NewEnum(graphql.EnumConfig{
		Name: fmt.Sprintf("%s%sTimeDecayDecayFunctionEnum", prefix, className),
		Values: graphql.EnumValueConfigMap{
			"EXPONENTIAL": &graphql.EnumValueConfig{
				Value:       "EXPONENTIAL",
				Description: "Exponential decay: exp(-age / halfLife)",
			},
			"LINEAR": &graphql.EnumValueConfig{
				Value:       "LINEAR",
				Description: "Linear decay: max(0, 1 - age / maxAge)",
			},
			"STEP": &graphql.EnumValueConfig{
				Value:       "STEP",
				Description: "Step decay with configurable thresholds",
			},
		},
	})

	stepThresholdInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: fmt.Sprintf("%s%sTimeDecayStepThreshold", prefix, className),
		Fields: graphql.InputObjectConfigFieldMap{
			"maxAge": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Maximum age for this threshold (e.g., '7d', '30d')",
			},
			"weight": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(graphql.Float),
				Description: "Weight to apply for this threshold (0.0 to 1.0)",
			},
		},
	})

	timeDecayInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: fmt.Sprintf("%s%sTimeDecayInpObj", prefix, className),
		Fields: graphql.InputObjectConfigFieldMap{
			"property": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "The datetime property to use for time decay calculation",
			},
			"halfLife": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Half-life duration for exponential decay (e.g., '7d', '30d')",
			},
			"maxAge": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Maximum age for linear decay (e.g., '30d', '90d')",
			},
			"decayFunction": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(decayFunctionEnum),
				Description: "The decay function to apply (EXPONENTIAL, LINEAR, or STEP)",
			},
			"stepThresholds": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewList(stepThresholdInput),
				Description: "Step thresholds for STEP decay function",
			},
		},
		Description: "Time decay configuration for temporal vector search",
	})

	return &graphql.ArgumentConfig{
		Type: timeDecayInput,
		Description: "Time decay scoring to boost recent content. " +
			"Combines vector similarity with temporal relevance. " +
			"Use with vector search (nearVector, nearText, etc.) to rank recent results higher.",
	}
}
