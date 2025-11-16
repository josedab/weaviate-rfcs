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

package timedecay

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

// DecayFunction represents the type of decay function to apply
type DecayFunction string

const (
	// DecayFunctionExponential applies exponential decay: exp(-age / halfLife)
	DecayFunctionExponential DecayFunction = "EXPONENTIAL"
	// DecayFunctionLinear applies linear decay: max(0, 1 - age / maxAge)
	DecayFunctionLinear DecayFunction = "LINEAR"
	// DecayFunctionStep applies step decay with predefined thresholds
	DecayFunctionStep DecayFunction = "STEP"
)

// Config holds the configuration for time decay scoring
type Config struct {
	// Property is the name of the datetime property to use for decay calculation
	Property string
	// HalfLife is the duration after which the decay factor is 0.5 (for exponential)
	HalfLife time.Duration
	// MaxAge is the maximum age beyond which the score is 0 (for linear)
	MaxAge time.Duration
	// DecayFunction specifies which decay function to apply
	DecayFunction DecayFunction
	// StepThresholds defines the step decay thresholds (optional, for STEP function)
	StepThresholds []StepThreshold
	// OverFetchMultiplier specifies how many more candidates to fetch before applying decay
	// Default is calculated based on half-life if not specified
	OverFetchMultiplier float32
}

// StepThreshold defines a threshold for step decay
type StepThreshold struct {
	MaxAge time.Duration
	Weight float32
}

// Validate checks if the time decay configuration is valid
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}

	if c.Property == "" {
		return fmt.Errorf("time decay property cannot be empty")
	}

	switch c.DecayFunction {
	case DecayFunctionExponential:
		if c.HalfLife <= 0 {
			return fmt.Errorf("half-life must be positive for exponential decay")
		}
	case DecayFunctionLinear:
		if c.MaxAge <= 0 {
			return fmt.Errorf("max age must be positive for linear decay")
		}
	case DecayFunctionStep:
		if len(c.StepThresholds) == 0 {
			return fmt.Errorf("step thresholds must be provided for step decay")
		}
	default:
		return fmt.Errorf("unknown decay function: %s", c.DecayFunction)
	}

	return nil
}

// CalculateDecay computes the decay factor for a given age
func (c *Config) CalculateDecay(age time.Duration) float32 {
	if c == nil {
		return 1.0
	}

	switch c.DecayFunction {
	case DecayFunctionExponential:
		return c.exponentialDecay(age)
	case DecayFunctionLinear:
		return c.linearDecay(age)
	case DecayFunctionStep:
		return c.stepDecay(age)
	default:
		return 1.0
	}
}

// exponentialDecay calculates: exp(-age / halfLife)
func (c *Config) exponentialDecay(age time.Duration) float32 {
	if c.HalfLife <= 0 {
		return 1.0
	}
	exponent := -float64(age) / float64(c.HalfLife)
	return float32(math.Exp(exponent))
}

// linearDecay calculates: max(0, 1 - age / maxAge)
func (c *Config) linearDecay(age time.Duration) float32 {
	if c.MaxAge <= 0 {
		return 1.0
	}
	if age >= c.MaxAge {
		return 0.0
	}
	return 1.0 - float32(age)/float32(c.MaxAge)
}

// stepDecay applies step function based on thresholds
func (c *Config) stepDecay(age time.Duration) float32 {
	for _, threshold := range c.StepThresholds {
		if age < threshold.MaxAge {
			return threshold.Weight
		}
	}
	// Age exceeds all thresholds
	return 0.0
}

// GetOverFetchMultiplier returns the over-fetch multiplier
// If not explicitly set, calculates it based on the decay parameters
func (c *Config) GetOverFetchMultiplier() float32 {
	if c == nil {
		return 1.0
	}

	if c.OverFetchMultiplier > 0 {
		return c.OverFetchMultiplier
	}

	// Calculate default multiplier based on decay function
	switch c.DecayFunction {
	case DecayFunctionExponential:
		// For exponential decay, use empirical values based on half-life
		// These are recommendations from the RFC
		if c.HalfLife <= 24*time.Hour {
			return 5.0
		} else if c.HalfLife <= 7*24*time.Hour {
			return 3.0
		} else if c.HalfLife <= 30*24*time.Hour {
			return 2.0
		}
		return 1.5
	case DecayFunctionLinear, DecayFunctionStep:
		return 3.0
	default:
		return 1.0
	}
}

// ParseDuration parses duration strings like "7d", "30d", "1h"
func ParseDuration(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([smhdw])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s (expected format: 7d, 30h, etc.)", s)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}

	var multiplier time.Duration
	switch matches[2] {
	case "s":
		multiplier = time.Second
	case "m":
		multiplier = time.Minute
	case "h":
		multiplier = time.Hour
	case "d":
		multiplier = 24 * time.Hour
	case "w":
		multiplier = 7 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", matches[2])
	}

	return time.Duration(value) * multiplier, nil
}
