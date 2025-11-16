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
	"github.com/weaviate/weaviate/entities/schema/evolution"
)

// CompatibilityValidator validates schema compatibility using a set of rules
type CompatibilityValidator struct {
	rules  []evolution.CompatibilityRule
	config evolution.CompatibilityConfig
}

// NewCompatibilityValidator creates a new compatibility validator
func NewCompatibilityValidator(config evolution.CompatibilityConfig) *CompatibilityValidator {
	v := &CompatibilityValidator{
		rules:  make([]evolution.CompatibilityRule, 0),
		config: config,
	}

	// Register default rules
	v.RegisterDefaultRules()

	// Register custom rules from config
	for _, rule := range config.CustomRules {
		v.AddRule(rule)
	}

	return v
}

// RegisterDefaultRules registers the standard compatibility rules
func (v *CompatibilityValidator) RegisterDefaultRules() {
	v.AddRule(&AddPropertyRule{})
	v.AddRule(&RemovePropertyRule{})
	v.AddRule(&ChangePropertyTypeRule{})
	v.AddRule(&RemoveClassRule{})
	v.AddRule(&AddClassRule{})
	v.AddRule(&ModifyVectorConfigRule{})
	v.AddRule(&ModifyInvertedIndexRule{})
}

// AddRule adds a compatibility rule to the validator
func (v *CompatibilityValidator) AddRule(rule evolution.CompatibilityRule) {
	// Skip if rule is in ignore list
	for _, ignoredRule := range v.config.IgnoreRules {
		if rule.Name() == ignoredRule {
			return
		}
	}
	v.rules = append(v.rules, rule)
}

// Validate checks compatibility between old and new schemas
func (v *CompatibilityValidator) Validate(old, new *models.Schema) (*evolution.CompatibilityResult, error) {
	allIssues := make([]evolution.CompatibilityIssue, 0)
	overallLevel := evolution.FullyCompatible

	// Run all rules
	for _, rule := range v.rules {
		level, issues, err := rule.Check(old, new)
		if err != nil {
			return nil, fmt.Errorf("rule %s failed: %w", rule.Name(), err)
		}

		// Collect issues
		allIssues = append(allIssues, issues...)

		// Update overall compatibility level (most restrictive wins)
		if level == evolution.Breaking {
			overallLevel = evolution.Breaking
		} else if level == evolution.BackwardCompatible && overallLevel != evolution.Breaking {
			overallLevel = evolution.BackwardCompatible
		} else if level == evolution.ForwardCompatible && overallLevel == evolution.FullyCompatible {
			overallLevel = evolution.ForwardCompatible
		}
	}

	// Determine if schemas are compatible based on required level
	compatible := overallLevel.MeetRequirement(v.config.Level)

	result := &evolution.CompatibilityResult{
		Compatible: compatible,
		Level:      overallLevel,
		Issues:     allIssues,
		Summary:    v.generateSummary(overallLevel, allIssues),
	}

	return result, nil
}

// generateSummary creates a human-readable summary
func (v *CompatibilityValidator) generateSummary(level evolution.CompatibilityLevel, issues []evolution.CompatibilityIssue) string {
	breakingCount := 0
	warningCount := 0

	for _, issue := range issues {
		if issue.Level == evolution.Breaking {
			breakingCount++
		} else {
			warningCount++
		}
	}

	if breakingCount > 0 {
		return fmt.Sprintf("Schema has %d breaking change(s) and %d warning(s)", breakingCount, warningCount)
	} else if warningCount > 0 {
		return fmt.Sprintf("Schema is %s compatible with %d warning(s)", level, warningCount)
	}

	return fmt.Sprintf("Schema is %s compatible", level)
}

// AddPropertyRule checks if adding properties is compatible
type AddPropertyRule struct{}

func (r *AddPropertyRule) Name() string {
	return "add_property"
}

func (r *AddPropertyRule) Description() string {
	return "Checks compatibility when adding properties to a class"
}

func (r *AddPropertyRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	issues := make([]evolution.CompatibilityIssue, 0)
	level := evolution.FullyCompatible

	// Build map of old classes for quick lookup
	oldClasses := make(map[string]*models.Class)
	if old != nil && old.Classes != nil {
		for _, class := range old.Classes {
			oldClasses[class.Class] = class
		}
	}

	// Check new classes for added properties
	if new != nil && new.Classes != nil {
		for _, newClass := range new.Classes {
			oldClass, exists := oldClasses[newClass.Class]
			if !exists {
				continue // New class, handled by AddClassRule
			}

			// Build map of old properties
			oldProps := make(map[string]*models.Property)
			if oldClass.Properties != nil {
				for _, prop := range oldClass.Properties {
					oldProps[prop.Name] = prop
				}
			}

			// Check for new properties
			if newClass.Properties != nil {
				for _, newProp := range newClass.Properties {
					if _, exists := oldProps[newProp.Name]; !exists {
						// Property was added
						// Adding an optional property is backward compatible
						// Adding a required property would be breaking
						// Note: Weaviate doesn't have "required" fields currently,
						// so adding properties is always backward compatible
						level = evolution.BackwardCompatible

						issues = append(issues, evolution.CompatibilityIssue{
							Level:      evolution.BackwardCompatible,
							Rule:       r.Name(),
							Path:       fmt.Sprintf("%s.%s", newClass.Class, newProp.Name),
							Message:    fmt.Sprintf("Property '%s' added to class '%s'", newProp.Name, newClass.Class),
							Suggestion: "Ensure existing data is backfilled with appropriate values if needed",
						})
					}
				}
			}
		}
	}

	return level, issues, nil
}

// RemovePropertyRule checks if removing properties is compatible
type RemovePropertyRule struct{}

func (r *RemovePropertyRule) Name() string {
	return "remove_property"
}

func (r *RemovePropertyRule) Description() string {
	return "Checks compatibility when removing properties from a class"
}

func (r *RemovePropertyRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	issues := make([]evolution.CompatibilityIssue, 0)
	level := evolution.FullyCompatible

	// Build map of new classes for quick lookup
	newClasses := make(map[string]*models.Class)
	if new != nil && new.Classes != nil {
		for _, class := range new.Classes {
			newClasses[class.Class] = class
		}
	}

	// Check old classes for removed properties
	if old != nil && old.Classes != nil {
		for _, oldClass := range old.Classes {
			newClass, exists := newClasses[oldClass.Class]
			if !exists {
				continue // Class removed, handled by RemoveClassRule
			}

			// Build map of new properties
			newProps := make(map[string]*models.Property)
			if newClass.Properties != nil {
				for _, prop := range newClass.Properties {
					newProps[prop.Name] = prop
				}
			}

			// Check for removed properties
			if oldClass.Properties != nil {
				for _, oldProp := range oldClass.Properties {
					if _, exists := newProps[oldProp.Name]; !exists {
						// Property was removed - this is a breaking change
						level = evolution.Breaking

						issues = append(issues, evolution.CompatibilityIssue{
							Level:      evolution.Breaking,
							Rule:       r.Name(),
							Path:       fmt.Sprintf("%s.%s", oldClass.Class, oldProp.Name),
							Message:    fmt.Sprintf("Property '%s' removed from class '%s'", oldProp.Name, oldClass.Class),
							Suggestion: "Consider deprecating the property instead of removing it, or use a migration strategy",
						})
					}
				}
			}
		}
	}

	return level, issues, nil
}

// ChangePropertyTypeRule checks if changing property types is compatible
type ChangePropertyTypeRule struct{}

func (r *ChangePropertyTypeRule) Name() string {
	return "change_property_type"
}

func (r *ChangePropertyTypeRule) Description() string {
	return "Checks compatibility when changing property data types"
}

func (r *ChangePropertyTypeRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	issues := make([]evolution.CompatibilityIssue, 0)
	level := evolution.FullyCompatible

	// Build map of old classes
	oldClasses := make(map[string]*models.Class)
	if old != nil && old.Classes != nil {
		for _, class := range old.Classes {
			oldClasses[class.Class] = class
		}
	}

	// Check for type changes
	if new != nil && new.Classes != nil {
		for _, newClass := range new.Classes {
			oldClass, exists := oldClasses[newClass.Class]
			if !exists {
				continue
			}

			// Build map of old properties
			oldProps := make(map[string]*models.Property)
			if oldClass.Properties != nil {
				for _, prop := range oldClass.Properties {
					oldProps[prop.Name] = prop
				}
			}

			// Check for type changes in properties
			if newClass.Properties != nil {
				for _, newProp := range newClass.Properties {
					oldProp, exists := oldProps[newProp.Name]
					if !exists {
						continue
					}

					// Compare data types
					if !dataTypesEqual(oldProp.DataType, newProp.DataType) {
						// Type changed - this is breaking
						level = evolution.Breaking

						issues = append(issues, evolution.CompatibilityIssue{
							Level: evolution.Breaking,
							Rule:  r.Name(),
							Path:  fmt.Sprintf("%s.%s", newClass.Class, newProp.Name),
							Message: fmt.Sprintf(
								"Property '%s' in class '%s' changed type from %v to %v",
								newProp.Name, newClass.Class, oldProp.DataType, newProp.DataType,
							),
							Suggestion: "Use shadow property migration strategy to change types without downtime",
						})
					}
				}
			}
		}
	}

	return level, issues, nil
}

// RemoveClassRule checks if removing classes is compatible
type RemoveClassRule struct{}

func (r *RemoveClassRule) Name() string {
	return "remove_class"
}

func (r *RemoveClassRule) Description() string {
	return "Checks compatibility when removing classes"
}

func (r *RemoveClassRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	issues := make([]evolution.CompatibilityIssue, 0)
	level := evolution.FullyCompatible

	// Build map of new classes
	newClasses := make(map[string]*models.Class)
	if new != nil && new.Classes != nil {
		for _, class := range new.Classes {
			newClasses[class.Class] = class
		}
	}

	// Check for removed classes
	if old != nil && old.Classes != nil {
		for _, oldClass := range old.Classes {
			if _, exists := newClasses[oldClass.Class]; !exists {
				// Class removed - this is breaking
				level = evolution.Breaking

				issues = append(issues, evolution.CompatibilityIssue{
					Level:      evolution.Breaking,
					Rule:       r.Name(),
					Path:       oldClass.Class,
					Message:    fmt.Sprintf("Class '%s' removed from schema", oldClass.Class),
					Suggestion: "Consider archiving the class data before removal",
				})
			}
		}
	}

	return level, issues, nil
}

// AddClassRule checks if adding classes is compatible
type AddClassRule struct{}

func (r *AddClassRule) Name() string {
	return "add_class"
}

func (r *AddClassRule) Description() string {
	return "Checks compatibility when adding classes"
}

func (r *AddClassRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	issues := make([]evolution.CompatibilityIssue, 0)
	level := evolution.FullyCompatible

	// Build map of old classes
	oldClasses := make(map[string]*models.Class)
	if old != nil && old.Classes != nil {
		for _, class := range old.Classes {
			oldClasses[class.Class] = class
		}
	}

	// Check for new classes
	if new != nil && new.Classes != nil {
		for _, newClass := range new.Classes {
			if _, exists := oldClasses[newClass.Class]; !exists {
				// Class added - this is fully compatible
				issues = append(issues, evolution.CompatibilityIssue{
					Level:      evolution.FullyCompatible,
					Rule:       r.Name(),
					Path:       newClass.Class,
					Message:    fmt.Sprintf("Class '%s' added to schema", newClass.Class),
					Suggestion: "",
				})
			}
		}
	}

	return level, issues, nil
}

// ModifyVectorConfigRule checks vector configuration changes
type ModifyVectorConfigRule struct{}

func (r *ModifyVectorConfigRule) Name() string {
	return "modify_vector_config"
}

func (r *ModifyVectorConfigRule) Description() string {
	return "Checks compatibility when modifying vector configurations"
}

func (r *ModifyVectorConfigRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	// Vector config changes are typically backward compatible if dimensions stay the same
	// This is a simplified implementation
	return evolution.BackwardCompatible, []evolution.CompatibilityIssue{}, nil
}

// ModifyInvertedIndexRule checks inverted index changes
type ModifyInvertedIndexRule struct{}

func (r *ModifyInvertedIndexRule) Name() string {
	return "modify_inverted_index"
}

func (r *ModifyInvertedIndexRule) Description() string {
	return "Checks compatibility when modifying inverted index configurations"
}

func (r *ModifyInvertedIndexRule) Check(old, new *models.Schema) (evolution.CompatibilityLevel, []evolution.CompatibilityIssue, error) {
	// Index changes are typically backward compatible and applied in background
	return evolution.BackwardCompatible, []evolution.CompatibilityIssue{}, nil
}

// Helper function to compare data types
func dataTypesEqual(dt1, dt2 []string) bool {
	if len(dt1) != len(dt2) {
		return false
	}

	for i := range dt1 {
		if dt1[i] != dt2[i] {
			return false
		}
	}

	return true
}
