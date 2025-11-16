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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/schema/evolution"
)

// VersionStore defines the interface for persisting schema versions
type VersionStore interface {
	// Save stores a schema version
	Save(version *evolution.SchemaVersion) error

	// Get retrieves a schema version by ID
	Get(id uint64) (*evolution.SchemaVersion, error)

	// GetLatest retrieves the latest schema version
	GetLatest() (*evolution.SchemaVersion, error)

	// List returns a range of schema versions
	List(offset, limit int) ([]*evolution.SchemaVersion, error)

	// GetByHash retrieves a version by its hash
	GetByHash(hash string) (*evolution.SchemaVersion, error)

	// Delete removes a schema version (for cleanup)
	Delete(id uint64) error

	// NextID returns the next available version ID
	NextID() (uint64, error)
}

// SchemaRegistry manages schema versions and orchestrates evolution
type SchemaRegistry struct {
	mu sync.RWMutex

	// store persists schema versions
	store VersionStore

	// validator checks compatibility
	validator *CompatibilityValidator

	// migrator executes migrations
	migrator *SchemaMigrator

	// differ computes diffs
	differ *SchemaDiffer

	// current is the currently active schema version
	current *evolution.SchemaVersion

	// history contains recent versions (cache)
	history []*evolution.SchemaVersion

	// config contains registry configuration
	config RegistryConfig
}

// RegistryConfig configures the schema registry
type RegistryConfig struct {
	// MaxHistorySize is the maximum number of versions to keep in history
	MaxHistorySize int

	// CompatibilityConfig for validation
	CompatibilityConfig evolution.CompatibilityConfig

	// AutoMigrate enables automatic migration execution
	AutoMigrate bool

	// MigrationConfig for migration behavior
	MigrationConfig MigrationConfig
}

// DefaultRegistryConfig returns default registry configuration
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		MaxHistorySize:      100,
		CompatibilityConfig: evolution.DefaultCompatibilityConfig(),
		AutoMigrate:         false,
		MigrationConfig:     DefaultMigrationConfig(),
	}
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry(
	store VersionStore,
	validator *CompatibilityValidator,
	migrator *SchemaMigrator,
	differ *SchemaDiffer,
	config RegistryConfig,
) *SchemaRegistry {
	return &SchemaRegistry{
		store:     store,
		validator: validator,
		migrator:  migrator,
		differ:    differ,
		history:   make([]*evolution.SchemaVersion, 0),
		config:    config,
	}
}

// RegisterSchema registers a new schema version
func (r *SchemaRegistry) RegisterSchema(schema *models.Schema, author, description string) (*evolution.SchemaVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate version metadata
	versionID, err := r.store.NextID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate version ID: %w", err)
	}

	hash := r.hashSchema(schema)

	// Check if this exact schema already exists
	existing, err := r.store.GetByHash(hash)
	if err == nil && existing != nil {
		return existing, nil
	}

	version := &evolution.SchemaVersion{
		ID:              versionID,
		Timestamp:       time.Now(),
		Author:          author,
		Description:     description,
		Hash:            hash,
		Compatibility:   evolution.Unknown,
		MigrationStatus: evolution.MigrationNotRequired,
	}

	// If there's a current version, detect changes and validate compatibility
	if r.current != nil {
		version.PreviousVersion = r.current.ID

		// Detect changes
		changes, err := r.detectChanges(r.current, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to detect changes: %w", err)
		}
		version.Changes = changes

		// Validate compatibility if there are changes
		if len(changes) > 0 {
			// Get the old schema for comparison
			oldSchema, err := r.getSchemaForVersion(r.current)
			if err != nil {
				return nil, fmt.Errorf("failed to get previous schema: %w", err)
			}

			result, err := r.validator.Validate(oldSchema, schema)
			if err != nil {
				return nil, fmt.Errorf("compatibility validation failed: %w", err)
			}

			version.Compatibility = result.Level

			// Check if breaking changes are allowed
			if result.Level.IsBreaking() && !r.config.CompatibilityConfig.AllowBreakingChanges {
				return nil, &evolution.CompatibilityError{
					Message: "breaking changes not allowed",
					Issues:  result.Issues,
				}
			}

			// Generate migration plan if changes require migration
			if r.requiresMigration(changes) {
				plan, err := r.migrator.Plan(changes)
				if err != nil {
					return nil, fmt.Errorf("failed to generate migration plan: %w", err)
				}

				// Attach migration plan to changes
				for i := range version.Changes {
					if version.Changes[i].Migration == nil {
						version.Changes[i].Migration = plan
						break
					}
				}

				version.MigrationStatus = evolution.MigrationPending
			}
		}
	} else {
		// First version
		version.PreviousVersion = 0
		version.Compatibility = evolution.FullyCompatible
		version.Changes = []evolution.SchemaChange{}
	}

	// Store version
	if err := r.store.Save(version); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Update current version
	r.current = version

	// Add to history cache
	r.addToHistory(version)

	return version, nil
}

// GetVersion retrieves a specific schema version
func (r *SchemaRegistry) GetVersion(id uint64) (*evolution.SchemaVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check cache first
	for _, v := range r.history {
		if v.ID == id {
			return v, nil
		}
	}

	// Fetch from store
	return r.store.Get(id)
}

// GetLatestVersion returns the current schema version
func (r *SchemaRegistry) GetLatestVersion() (*evolution.SchemaVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.current != nil {
		return r.current, nil
	}

	return r.store.GetLatest()
}

// ListVersions returns a list of schema versions
func (r *SchemaRegistry) ListVersions(offset, limit int) ([]*evolution.SchemaVersion, error) {
	return r.store.List(offset, limit)
}

// Diff computes the difference between two versions
func (r *SchemaRegistry) Diff(fromID, toID uint64) (*evolution.SchemaDiff, error) {
	from, err := r.GetVersion(fromID)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", fromID, err)
	}

	to, err := r.GetVersion(toID)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", toID, err)
	}

	fromSchema, err := r.getSchemaForVersion(from)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for version %d: %w", fromID, err)
	}

	toSchema, err := r.getSchemaForVersion(to)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for version %d: %w", toID, err)
	}

	return r.differ.Diff(fromSchema, toSchema, fromID, toID)
}

// Merge performs a three-way merge of schema versions
func (r *SchemaRegistry) Merge(baseID, v1ID, v2ID uint64) (*models.Schema, error) {
	base, err := r.GetVersion(baseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base version %d: %w", baseID, err)
	}

	v1, err := r.GetVersion(v1ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v1ID, err)
	}

	v2, err := r.GetVersion(v2ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", v2ID, err)
	}

	baseSchema, err := r.getSchemaForVersion(base)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for base version: %w", err)
	}

	v1Schema, err := r.getSchemaForVersion(v1)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for version 1: %w", err)
	}

	v2Schema, err := r.getSchemaForVersion(v2)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for version 2: %w", err)
	}

	return r.differ.Merge(baseSchema, v1Schema, v2Schema)
}

// Rollback rolls back to a previous schema version
func (r *SchemaRegistry) Rollback(targetID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	target, err := r.store.Get(targetID)
	if err != nil {
		return fmt.Errorf("failed to get target version %d: %w", targetID, err)
	}

	if target.ID >= r.current.ID {
		return fmt.Errorf("cannot rollback to version %d (current is %d)", targetID, r.current.ID)
	}

	// Create a new version that restores the target schema
	schema, err := r.getSchemaForVersion(target)
	if err != nil {
		return fmt.Errorf("failed to get schema for target version: %w", err)
	}

	newVersion, err := r.RegisterSchema(
		schema,
		"system",
		fmt.Sprintf("Rollback to version %d", targetID),
	)
	if err != nil {
		return fmt.Errorf("failed to register rollback version: %w", err)
	}

	newVersion.MigrationStatus = evolution.MigrationRolledBack

	return nil
}

// hashSchema computes a cryptographic hash of the schema
func (r *SchemaRegistry) hashSchema(schema *models.Schema) string {
	// Serialize schema to JSON
	data, err := json.Marshal(schema)
	if err != nil {
		// Fallback to simple hash if serialization fails
		return fmt.Sprintf("error-%d", time.Now().Unix())
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// detectChanges detects changes between two schema versions
func (r *SchemaRegistry) detectChanges(oldVersion *evolution.SchemaVersion, newSchema *models.Schema) ([]evolution.SchemaChange, error) {
	oldSchema, err := r.getSchemaForVersion(oldVersion)
	if err != nil {
		return nil, err
	}

	diff, err := r.differ.Diff(oldSchema, newSchema, oldVersion.ID, 0)
	if err != nil {
		return nil, err
	}

	// Convert diff to SchemaChange list
	changes := make([]evolution.SchemaChange, 0)

	// Process added elements
	for _, added := range diff.Added {
		change := evolution.SchemaChange{
			Type:      r.elementTypeToChangeType(added.Type, "add"),
			Class:     r.extractClassName(added.Path),
			Property:  r.extractPropertyName(added.Path),
			Before:    nil,
			After:     added.Value,
			Timestamp: time.Now(),
		}
		changes = append(changes, change)
	}

	// Process removed elements
	for _, removed := range diff.Removed {
		change := evolution.SchemaChange{
			Type:      r.elementTypeToChangeType(removed.Type, "remove"),
			Class:     r.extractClassName(removed.Path),
			Property:  r.extractPropertyName(removed.Path),
			Before:    removed.Value,
			After:     nil,
			Timestamp: time.Now(),
		}
		changes = append(changes, change)
	}

	// Process modified elements
	for _, modified := range diff.Modified {
		change := evolution.SchemaChange{
			Type:      r.elementTypeToChangeType(modified.Type, "modify"),
			Class:     r.extractClassName(modified.Path),
			Property:  r.extractPropertyName(modified.Path),
			Before:    modified.Before,
			After:     modified.After,
			Timestamp: time.Now(),
		}
		changes = append(changes, change)
	}

	return changes, nil
}

// requiresMigration checks if changes require data migration
func (r *SchemaRegistry) requiresMigration(changes []evolution.SchemaChange) bool {
	for _, change := range changes {
		switch change.Type {
		case evolution.ChangeTypeAddProperty,
			evolution.ChangeTypeRemoveProperty,
			evolution.ChangeTypeChangePropertyType,
			evolution.ChangeTypeAddIndex,
			evolution.ChangeTypeModifyIndex:
			return true
		}
	}
	return false
}

// getSchemaForVersion reconstructs the schema for a given version
// In a full implementation, this would either:
// 1. Store the full schema with each version, or
// 2. Replay changes from version 0 to reconstruct the schema
func (r *SchemaRegistry) getSchemaForVersion(version *evolution.SchemaVersion) (*models.Schema, error) {
	// TODO: Implement schema reconstruction
	// For now, return an error indicating this needs to be implemented
	return nil, fmt.Errorf("schema reconstruction not yet implemented")
}

// addToHistory adds a version to the history cache
func (r *SchemaRegistry) addToHistory(version *evolution.SchemaVersion) {
	r.history = append(r.history, version)

	// Trim history if it exceeds max size
	if len(r.history) > r.config.MaxHistorySize {
		r.history = r.history[1:]
	}
}

// elementTypeToChangeType converts element type to change type
func (r *SchemaRegistry) elementTypeToChangeType(elemType evolution.ElementType, operation string) evolution.ChangeType {
	switch elemType {
	case evolution.ElementClass:
		if operation == "add" {
			return evolution.ChangeTypeAddClass
		} else if operation == "remove" {
			return evolution.ChangeTypeRemoveClass
		}
		return evolution.ChangeTypeModifyClass
	case evolution.ElementProperty:
		if operation == "add" {
			return evolution.ChangeTypeAddProperty
		} else if operation == "remove" {
			return evolution.ChangeTypeRemoveProperty
		}
		return evolution.ChangeTypeModifyProperty
	case evolution.ElementIndex:
		if operation == "add" {
			return evolution.ChangeTypeAddIndex
		} else if operation == "remove" {
			return evolution.ChangeTypeRemoveIndex
		}
		return evolution.ChangeTypeModifyIndex
	case evolution.ElementVectorConfig:
		return evolution.ChangeTypeModifyVectorConfig
	case evolution.ElementReplication:
		return evolution.ChangeTypeModifyReplication
	default:
		return evolution.ChangeTypeModifyClass
	}
}

// extractClassName extracts class name from a path like "Article" or "Article.title"
func (r *SchemaRegistry) extractClassName(path string) string {
	// Simple implementation - in production would use proper path parsing
	if path == "" {
		return ""
	}
	// Split by dot and return first part
	for i, c := range path {
		if c == '.' {
			return path[:i]
		}
	}
	return path
}

// extractPropertyName extracts property name from a path like "Article.title"
func (r *SchemaRegistry) extractPropertyName(path string) string {
	// Simple implementation - in production would use proper path parsing
	if path == "" {
		return ""
	}
	// Split by dot and return second part if exists
	for i, c := range path {
		if c == '.' && i+1 < len(path) {
			return path[i+1:]
		}
	}
	return ""
}
