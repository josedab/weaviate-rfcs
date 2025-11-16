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
	"math"
	"testing"
	"time"
)

func TestExponentialDecay(t *testing.T) {
	cfg := &Config{
		Property:      "publishedAt",
		HalfLife:      7 * 24 * time.Hour,
		DecayFunction: DecayFunctionExponential,
	}

	tests := []struct {
		age      time.Duration
		expected float32
		name     string
	}{
		{0, 1.0, "zero age"},
		{7 * 24 * time.Hour, float32(math.Exp(-1)), "one half-life"},
		{14 * 24 * time.Hour, float32(math.Exp(-2)), "two half-lives"},
		{30 * 24 * time.Hour, float32(math.Exp(-30.0 / 7.0)), "30 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.CalculateDecay(tt.age)
			if math.Abs(float64(result-tt.expected)) > 0.01 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestLinearDecay(t *testing.T) {
	cfg := &Config{
		Property:      "publishedAt",
		MaxAge:        30 * 24 * time.Hour,
		DecayFunction: DecayFunctionLinear,
	}

	tests := []struct {
		age      time.Duration
		expected float32
		name     string
	}{
		{0, 1.0, "zero age"},
		{15 * 24 * time.Hour, 0.5, "half max age"},
		{30 * 24 * time.Hour, 0.0, "max age"},
		{60 * 24 * time.Hour, 0.0, "beyond max age"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.CalculateDecay(tt.age)
			if math.Abs(float64(result-tt.expected)) > 0.01 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestStepDecay(t *testing.T) {
	cfg := &Config{
		Property:      "publishedAt",
		DecayFunction: DecayFunctionStep,
		StepThresholds: []StepThreshold{
			{MaxAge: 7 * 24 * time.Hour, Weight: 1.0},
			{MaxAge: 30 * 24 * time.Hour, Weight: 0.5},
			{MaxAge: 90 * 24 * time.Hour, Weight: 0.2},
		},
	}

	tests := []struct {
		age      time.Duration
		expected float32
		name     string
	}{
		{3 * 24 * time.Hour, 1.0, "within first threshold"},
		{15 * 24 * time.Hour, 0.5, "within second threshold"},
		{60 * 24 * time.Hour, 0.2, "within third threshold"},
		{100 * 24 * time.Hour, 0.0, "beyond all thresholds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.CalculateDecay(tt.age)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
		name     string
	}{
		{"7d", 7 * 24 * time.Hour, false, "7 days"},
		{"30d", 30 * 24 * time.Hour, false, "30 days"},
		{"1h", time.Hour, false, "1 hour"},
		{"5m", 5 * time.Minute, false, "5 minutes"},
		{"10s", 10 * time.Second, false, "10 seconds"},
		{"2w", 14 * 24 * time.Hour, false, "2 weeks"},
		{"invalid", 0, true, "invalid format"},
		{"7x", 0, true, "invalid unit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		cfg      *Config
		hasError bool
		name     string
	}{
		{
			cfg:      nil,
			hasError: false,
			name:     "nil config",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				HalfLife:      7 * 24 * time.Hour,
				DecayFunction: DecayFunctionExponential,
			},
			hasError: false,
			name:     "valid exponential",
		},
		{
			cfg: &Config{
				Property:      "",
				HalfLife:      7 * 24 * time.Hour,
				DecayFunction: DecayFunctionExponential,
			},
			hasError: true,
			name:     "empty property",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				HalfLife:      0,
				DecayFunction: DecayFunctionExponential,
			},
			hasError: true,
			name:     "zero half-life",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				DecayFunction: "INVALID",
			},
			hasError: true,
			name:     "invalid decay function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.hasError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetOverFetchMultiplier(t *testing.T) {
	tests := []struct {
		cfg      *Config
		expected float32
		name     string
	}{
		{
			cfg:      nil,
			expected: 1.0,
			name:     "nil config",
		},
		{
			cfg: &Config{
				Property:            "publishedAt",
				HalfLife:            24 * time.Hour,
				DecayFunction:       DecayFunctionExponential,
				OverFetchMultiplier: 10.0,
			},
			expected: 10.0,
			name:     "explicit multiplier",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				HalfLife:      12 * time.Hour,
				DecayFunction: DecayFunctionExponential,
			},
			expected: 5.0,
			name:     "short half-life",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				HalfLife:      7 * 24 * time.Hour,
				DecayFunction: DecayFunctionExponential,
			},
			expected: 3.0,
			name:     "week half-life",
		},
		{
			cfg: &Config{
				Property:      "publishedAt",
				MaxAge:        30 * 24 * time.Hour,
				DecayFunction: DecayFunctionLinear,
			},
			expected: 3.0,
			name:     "linear decay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetOverFetchMultiplier()
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}
