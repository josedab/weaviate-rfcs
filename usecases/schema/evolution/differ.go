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

// SchemaDiffer computes differences and merges between schemas
type SchemaDiffer struct {
	options evolution.DiffOptions
}

// NewSchemaDiffer creates a new schema differ
func NewSchemaDiffer(options evolution.DiffOptions) *SchemaDiffer {
	return &SchemaDiffer{
		options: options,
	}
}

// Diff computes the difference between two schemas
func (d *SchemaDiffer) Diff(old, new *models.Schema, fromVersion, toVersion uint64) (*evolution.SchemaDiff, error) {
	diff := &evolution.SchemaDiff{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Added:       make([]evolution.SchemaElement, 0),
		Removed:     make([]evolution.SchemaElement, 0),
		Modified:    make([]evolution.SchemaModification, 0),
		Conflicts:   make([]evolution.MergeConflict, 0),
	}

	// Build maps for comparison
	oldClasses := d.buildClassMap(old)
	newClasses := d.buildClassMap(new)

	// Find added classes
	for className, newClass := range newClasses {
		if _, exists := oldClasses[className]; !exists {
			diff.Added = append(diff.Added, evolution.SchemaElement{
				Type:  evolution.ElementClass,
				Path:  className,
				Value: newClass,
			})
		}
	}

	// Find removed and modified classes
	for className, oldClass := range oldClasses {
		newClass, exists := newClasses[className]
		if !exists {
			// Class removed
			diff.Removed = append(diff.Removed, evolution.SchemaElement{
				Type:  evolution.ElementClass,
				Path:  className,
				Value: oldClass,
			})
		} else {
			// Class exists in both - check for property changes
			d.diffProperties(className, oldClass, newClass, diff)
			d.diffVectorConfig(className, oldClass, newClass, diff)
			d.diffInvertedIndex(className, oldClass, newClass, diff)
		}
	}

	return diff, nil
}

// diffProperties compares properties between two class versions
func (d *SchemaDiffer) diffProperties(className string, oldClass, newClass *models.Class, diff *evolution.SchemaDiff) {
	oldProps := d.buildPropertyMap(oldClass)
	newProps := d.buildPropertyMap(newClass)

	// Find added properties
	for propName, newProp := range newProps {
		if _, exists := oldProps[propName]; !exists {
			diff.Added = append(diff.Added, evolution.SchemaElement{
				Type:  evolution.ElementProperty,
				Path:  fmt.Sprintf("%s.%s", className, propName),
				Value: newProp,
			})
		}
	}

	// Find removed and modified properties
	for propName, oldProp := range oldProps {
		newProp, exists := newProps[propName]
		if !exists {
			// Property removed
			diff.Removed = append(diff.Removed, evolution.SchemaElement{
				Type:  evolution.ElementProperty,
				Path:  fmt.Sprintf("%s.%s", className, propName),
				Value: oldProp,
			})
		} else {
			// Property exists - check for modifications
			if fieldChanges := d.diffProperty(oldProp, newProp); len(fieldChanges) > 0 {
				diff.Modified = append(diff.Modified, evolution.SchemaModification{
					Path:         fmt.Sprintf("%s.%s", className, propName),
					Type:         evolution.ElementProperty,
					Before:       oldProp,
					After:        newProp,
					FieldChanges: fieldChanges,
				})
			}
		}
	}
}

// diffProperty compares two property definitions
func (d *SchemaDiffer) diffProperty(old, new *models.Property) []evolution.FieldChange {
	changes := make([]evolution.FieldChange, 0)

	// Compare data types
	if !dataTypesEqual(old.DataType, new.DataType) {
		changes = append(changes, evolution.FieldChange{
			Field:  "dataType",
			Before: old.DataType,
			After:  new.DataType,
		})
	}

	// Compare description
	if old.Description != new.Description && !d.options.IgnoreMetadata {
		changes = append(changes, evolution.FieldChange{
			Field:  "description",
			Before: old.Description,
			After:  new.Description,
		})
	}

	// Compare indexing
	if old.IndexInverted != new.IndexInverted {
		changes = append(changes, evolution.FieldChange{
			Field:  "indexInverted",
			Before: old.IndexInverted,
			After:  new.IndexInverted,
		})
	}

	// Compare tokenization
	if old.Tokenization != new.Tokenization {
		changes = append(changes, evolution.FieldChange{
			Field:  "tokenization",
			Before: old.Tokenization,
			After:  new.Tokenization,
		})
	}

	return changes
}

// diffVectorConfig compares vector configurations
func (d *SchemaDiffer) diffVectorConfig(className string, oldClass, newClass *models.Class, diff *evolution.SchemaDiff) {
	// Simplified comparison - in production would do deep comparison
	if oldClass.VectorConfig != nil || newClass.VectorConfig != nil {
		// Check if vector config changed
		if !d.vectorConfigsEqual(oldClass.VectorConfig, newClass.VectorConfig) {
			diff.Modified = append(diff.Modified, evolution.SchemaModification{
				Path:   fmt.Sprintf("%s.vectorConfig", className),
				Type:   evolution.ElementVectorConfig,
				Before: oldClass.VectorConfig,
				After:  newClass.VectorConfig,
			})
		}
	}
}

// diffInvertedIndex compares inverted index configurations
func (d *SchemaDiffer) diffInvertedIndex(className string, oldClass, newClass *models.Class, diff *evolution.SchemaDiff) {
	// Simplified comparison - in production would do deep comparison
	if oldClass.InvertedIndexConfig != nil || newClass.InvertedIndexConfig != nil {
		if !d.invertedIndexConfigsEqual(oldClass.InvertedIndexConfig, newClass.InvertedIndexConfig) {
			diff.Modified = append(diff.Modified, evolution.SchemaModification{
				Path:   fmt.Sprintf("%s.invertedIndexConfig", className),
				Type:   evolution.ElementIndex,
				Before: oldClass.InvertedIndexConfig,
				After:  newClass.InvertedIndexConfig,
			})
		}
	}
}

// Merge performs a three-way merge of schemas
func (d *SchemaDiffer) Merge(base, v1, v2 *models.Schema) (*models.Schema, error) {
	// Compute diffs from base to each version
	diff1, err := d.Diff(base, v1, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to compute diff to v1: %w", err)
	}

	diff2, err := d.Diff(base, v2, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to compute diff to v2: %w", err)
	}

	// Detect conflicts
	conflicts := d.detectConflicts(diff1, diff2)
	if len(conflicts) > 0 {
		return nil, &MergeConflictError{
			Message:   "schema merge conflicts detected",
			Conflicts: conflicts,
		}
	}

	// Create merged schema starting from base
	merged := d.cloneSchema(base)

	// Apply changes from both diffs
	if err := d.applyDiff(merged, diff1); err != nil {
		return nil, fmt.Errorf("failed to apply diff1: %w", err)
	}

	if err := d.applyDiff(merged, diff2); err != nil {
		return nil, fmt.Errorf("failed to apply diff2: %w", err)
	}

	return merged, nil
}

// detectConflicts finds conflicting changes between two diffs
func (d *SchemaDiffer) detectConflicts(diff1, diff2 *evolution.SchemaDiff) []evolution.MergeConflict {
	conflicts := make([]evolution.MergeConflict, 0)

	// Build maps of changes by path
	added1 := d.buildPathMap(diff1.Added)
	added2 := d.buildPathMap(diff2.Added)
	removed1 := d.buildPathMap(diff1.Removed)
	removed2 := d.buildPathMap(diff2.Removed)
	modified1 := d.buildModificationMap(diff1.Modified)
	modified2 := d.buildModificationMap(diff2.Modified)

	// Check for add-add conflicts (same path, different values)
	for path, elem1 := range added1 {
		if elem2, exists := added2[path]; exists {
			// Both added same element - check if values match
			// Simplified - in production would do deep comparison
			conflicts = append(conflicts, evolution.MergeConflict{
				Path:          path,
				Type:          evolution.ConflictAddAdd,
				Version1Value: elem1,
				Version2Value: elem2,
				Description:   fmt.Sprintf("Both versions added '%s' with potentially different definitions", path),
			})
		}
	}

	// Check for modify-delete conflicts
	for path, mod := range modified1 {
		if elem, exists := removed2[path]; exists {
			conflicts = append(conflicts, evolution.MergeConflict{
				Path:          path,
				Type:          evolution.ConflictModifyDelete,
				Version1Value: mod,
				Version2Value: elem,
				Description:   fmt.Sprintf("Version 1 modified '%s' while version 2 deleted it", path),
			})
		}
	}

	for path, mod := range modified2 {
		if elem, exists := removed1[path]; exists {
			conflicts = append(conflicts, evolution.MergeConflict{
				Path:          path,
				Type:          evolution.ConflictModifyDelete,
				Version1Value: elem,
				Version2Value: mod,
				Description:   fmt.Sprintf("Version 1 deleted '%s' while version 2 modified it", path),
			})
		}
	}

	// Check for modify-modify conflicts (different modifications to same element)
	for path, mod1 := range modified1 {
		if mod2, exists := modified2[path]; exists {
			// Both modified same element - this is a conflict
			conflicts = append(conflicts, evolution.MergeConflict{
				Path:          path,
				Type:          evolution.ConflictModifyModify,
				Version1Value: mod1,
				Version2Value: mod2,
				Description:   fmt.Sprintf("Both versions modified '%s' differently", path),
			})
		}
	}

	return conflicts
}

// applyDiff applies a diff to a schema
func (d *SchemaDiffer) applyDiff(schema *models.Schema, diff *evolution.SchemaDiff) error {
	// Build class map
	classMap := d.buildClassMap(schema)

	// Apply additions
	for _, added := range diff.Added {
		switch added.Type {
		case evolution.ElementClass:
			if class, ok := added.Value.(*models.Class); ok {
				schema.Classes = append(schema.Classes, class)
			}
		case evolution.ElementProperty:
			className := d.extractClassName(added.Path)
			if class, exists := classMap[className]; exists {
				if prop, ok := added.Value.(*models.Property); ok {
					class.Properties = append(class.Properties, prop)
				}
			}
		}
	}

	// Apply modifications
	for _, mod := range diff.Modified {
		switch mod.Type {
		case evolution.ElementProperty:
			className := d.extractClassName(mod.Path)
			propName := d.extractPropertyName(mod.Path)
			if class, exists := classMap[className]; exists {
				for i, prop := range class.Properties {
					if prop.Name == propName {
						if newProp, ok := mod.After.(*models.Property); ok {
							class.Properties[i] = newProp
						}
						break
					}
				}
			}
		}
	}

	// Apply removals (do this last to avoid index issues)
	for _, removed := range diff.Removed {
		switch removed.Type {
		case evolution.ElementClass:
			className := removed.Path
			for i, class := range schema.Classes {
				if class.Class == className {
					schema.Classes = append(schema.Classes[:i], schema.Classes[i+1:]...)
					break
				}
			}
		case evolution.ElementProperty:
			className := d.extractClassName(removed.Path)
			propName := d.extractPropertyName(removed.Path)
			if class, exists := classMap[className]; exists {
				for i, prop := range class.Properties {
					if prop.Name == propName {
						class.Properties = append(class.Properties[:i], class.Properties[i+1:]...)
						break
					}
				}
			}
		}
	}

	return nil
}

// Helper methods

func (d *SchemaDiffer) buildClassMap(schema *models.Schema) map[string]*models.Class {
	classMap := make(map[string]*models.Class)
	if schema != nil && schema.Classes != nil {
		for _, class := range schema.Classes {
			classMap[class.Class] = class
		}
	}
	return classMap
}

func (d *SchemaDiffer) buildPropertyMap(class *models.Class) map[string]*models.Property {
	propMap := make(map[string]*models.Property)
	if class != nil && class.Properties != nil {
		for _, prop := range class.Properties {
			propMap[prop.Name] = prop
		}
	}
	return propMap
}

func (d *SchemaDiffer) buildPathMap(elements []evolution.SchemaElement) map[string]evolution.SchemaElement {
	pathMap := make(map[string]evolution.SchemaElement)
	for _, elem := range elements {
		pathMap[elem.Path] = elem
	}
	return pathMap
}

func (d *SchemaDiffer) buildModificationMap(mods []evolution.SchemaModification) map[string]evolution.SchemaModification {
	modMap := make(map[string]evolution.SchemaModification)
	for _, mod := range mods {
		modMap[mod.Path] = mod
	}
	return modMap
}

func (d *SchemaDiffer) cloneSchema(schema *models.Schema) *models.Schema {
	// Deep clone schema
	// In production, use proper deep copy implementation
	cloned := &models.Schema{
		Classes: make([]*models.Class, len(schema.Classes)),
	}
	copy(cloned.Classes, schema.Classes)
	return cloned
}

func (d *SchemaDiffer) vectorConfigsEqual(v1, v2 interface{}) bool {
	// Simplified comparison
	// In production, implement proper deep comparison
	return true
}

func (d *SchemaDiffer) invertedIndexConfigsEqual(i1, i2 *models.InvertedIndexConfig) bool {
	// Simplified comparison
	// In production, implement proper deep comparison
	if i1 == nil && i2 == nil {
		return true
	}
	if i1 == nil || i2 == nil {
		return false
	}
	return true
}

func (d *SchemaDiffer) extractClassName(path string) string {
	for i, c := range path {
		if c == '.' {
			return path[:i]
		}
	}
	return path
}

func (d *SchemaDiffer) extractPropertyName(path string) string {
	for i, c := range path {
		if c == '.' && i+1 < len(path) {
			return path[i+1:]
		}
	}
	return ""
}

// MergeConflictError represents a merge conflict
type MergeConflictError struct {
	Message   string
	Conflicts []evolution.MergeConflict
}

func (e *MergeConflictError) Error() string {
	return fmt.Sprintf("%s: %d conflict(s)", e.Message, len(e.Conflicts))
}
