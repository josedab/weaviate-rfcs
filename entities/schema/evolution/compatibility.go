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

package evolution

import (
	"fmt"

	"github.com/weaviate/weaviate/entities/models"
)

// CompatibilityRule defines an interface for checking schema compatibility
type CompatibilityRule interface {
	// Check validates compatibility between old and new schemas
	Check(old, new *models.Schema) (CompatibilityLevel, []CompatibilityIssue, error)

	// Name returns the name of this rule
	Name() string

	// Description returns a description of what this rule checks
	Description() string
}

// CompatibilityIssue describes a specific compatibility problem
type CompatibilityIssue struct {
	// Level indicates the severity (breaking, warning, info)
	Level CompatibilityLevel `json:"level"`

	// Rule that detected this issue
	Rule string `json:"rule"`

	// Path identifies where the issue occurred
	Path string `json:"path"`

	// Message describes the issue
	Message string `json:"message"`

	// Suggestion provides guidance on how to fix the issue
	Suggestion string `json:"suggestion,omitempty"`

	// Change that caused this issue
	Change *SchemaChange `json:"change,omitempty"`
}

// CompatibilityResult contains the results of compatibility checking
type CompatibilityResult struct {
	// Compatible indicates if schemas are compatible
	Compatible bool `json:"compatible"`

	// Level is the overall compatibility level
	Level CompatibilityLevel `json:"level"`

	// Issues contains all detected compatibility issues
	Issues []CompatibilityIssue `json:"issues"`

	// Summary provides a human-readable summary
	Summary string `json:"summary"`
}

// CompatibilityConfig configures compatibility checking behavior
type CompatibilityConfig struct {
	// Level is the minimum required compatibility level
	Level CompatibilityLevel `json:"level"`

	// EnforceOnWrite blocks writes if compatibility check fails
	EnforceOnWrite bool `json:"enforceOnWrite"`

	// AllowBreakingChanges allows breaking changes with explicit confirmation
	AllowBreakingChanges bool `json:"allowBreakingChanges"`

	// IgnoreRules specifies rules to ignore
	IgnoreRules []string `json:"ignoreRules,omitempty"`

	// CustomRules specifies additional custom rules
	CustomRules []CompatibilityRule `json:"-"`
}

// DefaultCompatibilityConfig returns the default compatibility configuration
func DefaultCompatibilityConfig() CompatibilityConfig {
	return CompatibilityConfig{
		Level:                BackwardCompatible,
		EnforceOnWrite:       true,
		AllowBreakingChanges: false,
		IgnoreRules:          []string{},
		CustomRules:          []CompatibilityRule{},
	}
}

// IsBreaking returns true if the compatibility level is breaking
func (c CompatibilityLevel) IsBreaking() bool {
	return c == Breaking
}

// IsCompatible returns true if the change is at least backward compatible
func (c CompatibilityLevel) IsCompatible() bool {
	return c == BackwardCompatible || c == ForwardCompatible || c == FullyCompatible
}

// String returns the string representation
func (c CompatibilityLevel) String() string {
	return string(c)
}

// Validate checks if the compatibility level is valid
func (c CompatibilityLevel) Validate() error {
	switch c {
	case BackwardCompatible, ForwardCompatible, FullyCompatible, Breaking, Unknown:
		return nil
	default:
		return NewCompatibilityError("invalid compatibility level: %s", c)
	}
}

// MeetRequirement checks if this level meets the required level
func (c CompatibilityLevel) MeetRequirement(required CompatibilityLevel) bool {
	if required == FullyCompatible {
		return c == FullyCompatible
	}
	if required == BackwardCompatible {
		return c == BackwardCompatible || c == FullyCompatible
	}
	if required == ForwardCompatible {
		return c == ForwardCompatible || c == FullyCompatible
	}
	return !c.IsBreaking()
}

// CompatibilityError represents a compatibility validation error
type CompatibilityError struct {
	Message string
	Issues  []CompatibilityIssue
}

func (e *CompatibilityError) Error() string {
	return e.Message
}

// NewCompatibilityError creates a new compatibility error
func NewCompatibilityError(format string, args ...interface{}) *CompatibilityError {
	return &CompatibilityError{
		Message: fmt.Sprintf(format, args...),
		Issues:  []CompatibilityIssue{},
	}
}

// WithIssues adds issues to the error
func (e *CompatibilityError) WithIssues(issues ...CompatibilityIssue) *CompatibilityError {
	e.Issues = append(e.Issues, issues...)
	return e
}
