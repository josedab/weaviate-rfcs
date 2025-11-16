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

// SchemaDiff represents differences between two schema versions
type SchemaDiff struct {
	// FromVersion is the starting version
	FromVersion uint64 `json:"fromVersion"`

	// ToVersion is the target version
	ToVersion uint64 `json:"toVersion"`

	// Added contains elements that were added
	Added []SchemaElement `json:"added"`

	// Removed contains elements that were removed
	Removed []SchemaElement `json:"removed"`

	// Modified contains elements that were changed
	Modified []SchemaModification `json:"modified"`

	// Conflicts contains any merge conflicts (for three-way merges)
	Conflicts []MergeConflict `json:"conflicts,omitempty"`
}

// SchemaElement represents a single schema element (class, property, etc.)
type SchemaElement struct {
	// Type of element (class, property, index, etc.)
	Type ElementType `json:"type"`

	// Path identifies the element (e.g., "Article", "Article.title")
	Path string `json:"path"`

	// Value contains the element definition
	Value interface{} `json:"value"`

	// Metadata contains additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ElementType identifies the type of schema element
type ElementType string

const (
	ElementClass          ElementType = "class"
	ElementProperty       ElementType = "property"
	ElementIndex          ElementType = "index"
	ElementVectorConfig   ElementType = "vector_config"
	ElementReplication    ElementType = "replication"
	ElementMultiTenancy   ElementType = "multi_tenancy"
	ElementShardingConfig ElementType = "sharding_config"
)

// SchemaModification represents a change to an existing element
type SchemaModification struct {
	// Path identifies the modified element
	Path string `json:"path"`

	// Type of element being modified
	Type ElementType `json:"type"`

	// Before contains the state before modification
	Before interface{} `json:"before"`

	// After contains the state after modification
	After interface{} `json:"after"`

	// FieldChanges contains specific field-level changes
	FieldChanges []FieldChange `json:"fieldChanges,omitempty"`
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	// Field name that changed
	Field string `json:"field"`

	// Before value
	Before interface{} `json:"before"`

	// After value
	After interface{} `json:"after"`
}

// MergeConflict represents a conflict when merging schema changes
type MergeConflict struct {
	// Path identifies where the conflict occurred
	Path string `json:"path"`

	// Type of conflict
	Type ConflictType `json:"type"`

	// BaseValue is the value in the common ancestor
	BaseValue interface{} `json:"baseValue,omitempty"`

	// Version1Value is the value in version 1
	Version1Value interface{} `json:"version1Value"`

	// Version2Value is the value in version 2
	Version2Value interface{} `json:"version2Value"`

	// Description explains the conflict
	Description string `json:"description"`

	// Resolution contains the resolved value (if manually resolved)
	Resolution interface{} `json:"resolution,omitempty"`
}

// ConflictType identifies the type of merge conflict
type ConflictType string

const (
	// ConflictModifyModify means both versions modified the same element differently
	ConflictModifyModify ConflictType = "modify_modify"

	// ConflictModifyDelete means one version modified and another deleted
	ConflictModifyDelete ConflictType = "modify_delete"

	// ConflictAddAdd means both versions added the same element with different definitions
	ConflictAddAdd ConflictType = "add_add"

	// ConflictTypeChange means both versions changed the type differently
	ConflictTypeChange ConflictType = "type_change"
)

// DiffOptions configures how schema diffs are computed
type DiffOptions struct {
	// IgnoreOrder ignores ordering differences in arrays
	IgnoreOrder bool

	// IgnoreDefaults ignores differences in default values
	IgnoreDefaults bool

	// IgnoreMetadata ignores metadata-only changes
	IgnoreMetadata bool

	// DetectRenames attempts to detect renamed elements
	DetectRenames bool

	// SimilarityThreshold for rename detection (0.0 - 1.0)
	SimilarityThreshold float64
}

// DefaultDiffOptions returns the default diff options
func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		IgnoreOrder:         false,
		IgnoreDefaults:      false,
		IgnoreMetadata:      false,
		DetectRenames:       true,
		SimilarityThreshold: 0.8,
	}
}
