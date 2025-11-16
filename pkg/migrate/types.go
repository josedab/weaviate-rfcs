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

package migrate

import (
	"time"
)

// Migration represents a single migration file
type Migration struct {
	Version          int                `yaml:"version"`
	FromVersion      int                `yaml:"from_version,omitempty"`
	Description      string             `yaml:"description"`
	Author           string             `yaml:"author,omitempty"`
	EstimatedDuration string            `yaml:"estimated_duration,omitempty"`
	Validation       []ValidationRule   `yaml:"validation,omitempty"`
	Operations       []Operation        `yaml:"operations"`
	Rollback         []Operation        `yaml:"rollback,omitempty"`
	ValidationAfter  []ValidationRule   `yaml:"validation_after,omitempty"`
}

// Operation represents a migration operation
type Operation struct {
	Type              string                 `yaml:"type"`
	Class             string                 `yaml:"class,omitempty"`
	Classes           []string               `yaml:"classes,omitempty"`
	Property          map[string]interface{} `yaml:"property,omitempty"`
	PropertyName      string                 `yaml:"property_name,omitempty"`
	Config            map[string]interface{} `yaml:"config,omitempty"`
	TargetVector      string                 `yaml:"target_vector,omitempty"`
	Compression       map[string]interface{} `yaml:"compression,omitempty"`
	DefaultValue      interface{}            `yaml:"default_value,omitempty"`
	Backfill          bool                   `yaml:"backfill,omitempty"`
	Background        bool                   `yaml:"background,omitempty"`
	EstimatedDuration string                 `yaml:"estimated_duration,omitempty"`
	FromBackup        bool                   `yaml:"from_backup,omitempty"`
}

// ValidationRule represents a pre or post-migration validation
type ValidationRule struct {
	Type     string `yaml:"type"`
	Class    string `yaml:"class,omitempty"`
	Property string `yaml:"property,omitempty"`
	Version  string `yaml:"version,omitempty"`
}

// Config represents the weaviate-migrate configuration
type Config struct {
	Weaviate                  WeaviateConfig `yaml:"weaviate"`
	MigrationsDir             string         `yaml:"migrations_dir"`
	DryRunByDefault           bool           `yaml:"dry_run_by_default"`
	AutoBackup                bool           `yaml:"auto_backup"`
	MaxConcurrentOperations   int            `yaml:"max_concurrent_operations"`
	OperationTimeout          string         `yaml:"operation_timeout"`
	MigrationTimeout          string         `yaml:"migration_timeout"`
}

// WeaviateConfig represents Weaviate connection settings
type WeaviateConfig struct {
	Host   string `yaml:"host"`
	Scheme string `yaml:"scheme"`
	APIKey string `yaml:"api_key,omitempty"`
}

// MigrationHistory represents a record of applied migration
type MigrationHistory struct {
	Version     int       `json:"version"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at"`
	DurationMs  int64     `json:"duration_ms"`
	Status      string    `json:"status"` // success, failed, rolled_back
	AppliedBy   string    `json:"applied_by"`
}

// OperationProgress tracks progress of a single operation
type OperationProgress struct {
	OperationIndex int
	TotalObjects   int64
	ProcessedObjects int64
	StartTime      time.Time
}

// MigrationCheckpoint stores state for rollback
type MigrationCheckpoint struct {
	Version        int
	CreatedAt      time.Time
	SchemaSnapshot map[string]interface{}
}

// MigrationPlan represents the execution plan for pending migrations
type MigrationPlan struct {
	Migrations      []Migration
	TotalOperations int
	EstimatedTime   time.Duration
	ObjectsAffected int64
	DiskSpaceNeeded int64
	RiskLevel       string
}

// OperationType constants
const (
	OperationAddProperty             = "add_property"
	OperationUpdateVectorIndexConfig = "update_vector_index_config"
	OperationReindexProperty         = "reindex_property"
	OperationAddClass                = "add_class"
	OperationUpdateClass             = "update_class"
	OperationDeleteProperty          = "delete_property"
	OperationRestoreVectorIndexConfig = "restore_vector_index_config"
	OperationEnableCompression       = "enable_compression"
	OperationDisableCompression      = "disable_compression"
	OperationRestoreFromBackup       = "restore_from_backup"
)

// ValidationType constants
const (
	ValidationClassExists         = "class_exists"
	ValidationPropertyExists      = "property_exists"
	ValidationMinWeaviateVersion  = "min_weaviate_version"
	ValidationIndexHealthy        = "index_healthy"
	ValidationDataIntegrity       = "data_integrity"
)

// MigrationStatus constants
const (
	StatusSuccess     = "success"
	StatusFailed      = "failed"
	StatusRolledBack  = "rolled_back"
	StatusInProgress  = "in_progress"
)
