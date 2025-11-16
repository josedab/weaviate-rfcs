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
	"time"
)

// SchemaVersion represents a specific version of the schema with metadata
type SchemaVersion struct {
	// ID is the monotonically increasing version number
	ID uint64 `json:"id"`

	// Timestamp when this version was created
	Timestamp time.Time `json:"timestamp"`

	// Author who created this version
	Author string `json:"author"`

	// Description of what changed in this version
	Description string `json:"description"`

	// Hash is the cryptographic hash of the schema content
	Hash string `json:"hash"`

	// Changes contains the list of schema modifications in this version
	Changes []SchemaChange `json:"changes"`

	// Compatibility indicates the compatibility level with the previous version
	Compatibility CompatibilityLevel `json:"compatibility"`

	// MigrationStatus tracks the migration progress for this version
	MigrationStatus MigrationStatus `json:"migrationStatus"`

	// PreviousVersion is the ID of the previous version (0 if this is the first version)
	PreviousVersion uint64 `json:"previousVersion"`
}

// CompatibilityLevel indicates how compatible a schema change is
type CompatibilityLevel string

const (
	// BackwardCompatible means old readers can read new data
	BackwardCompatible CompatibilityLevel = "backward"

	// ForwardCompatible means new readers can read old data
	ForwardCompatible CompatibilityLevel = "forward"

	// FullyCompatible means both backward and forward compatible
	FullyCompatible CompatibilityLevel = "full"

	// Breaking means the change is not compatible
	Breaking CompatibilityLevel = "breaking"

	// Unknown indicates compatibility hasn't been determined yet
	Unknown CompatibilityLevel = "unknown"
)

// MigrationStatus tracks the state of schema migration
type MigrationStatus string

const (
	// MigrationPending means migration hasn't started yet
	MigrationPending MigrationStatus = "pending"

	// MigrationInProgress means migration is currently running
	MigrationInProgress MigrationStatus = "in_progress"

	// MigrationCompleted means migration finished successfully
	MigrationCompleted MigrationStatus = "completed"

	// MigrationFailed means migration encountered an error
	MigrationFailed MigrationStatus = "failed"

	// MigrationRolledBack means migration was rolled back
	MigrationRolledBack MigrationStatus = "rolled_back"

	// MigrationNotRequired means no migration is needed
	MigrationNotRequired MigrationStatus = "not_required"
)

// SchemaChange represents a single change in the schema
type SchemaChange struct {
	// Type of the change (add_class, remove_class, add_property, etc.)
	Type ChangeType `json:"type"`

	// Class name affected by this change
	Class string `json:"class"`

	// Property name affected by this change (empty for class-level changes)
	Property string `json:"property,omitempty"`

	// Before contains the state before the change
	Before interface{} `json:"before,omitempty"`

	// After contains the state after the change
	After interface{} `json:"after,omitempty"`

	// Migration contains the plan to execute this change
	Migration *MigrationPlan `json:"migration,omitempty"`

	// Timestamp when this change was detected
	Timestamp time.Time `json:"timestamp"`
}

// ChangeType identifies the type of schema change
type ChangeType string

const (
	// Class-level changes
	ChangeTypeAddClass    ChangeType = "add_class"
	ChangeTypeRemoveClass ChangeType = "remove_class"
	ChangeTypeModifyClass ChangeType = "modify_class"

	// Property-level changes
	ChangeTypeAddProperty        ChangeType = "add_property"
	ChangeTypeRemoveProperty     ChangeType = "remove_property"
	ChangeTypeModifyProperty     ChangeType = "modify_property"
	ChangeTypeChangePropertyType ChangeType = "change_property_type"

	// Index changes
	ChangeTypeAddIndex     ChangeType = "add_index"
	ChangeTypeRemoveIndex  ChangeType = "remove_index"
	ChangeTypeModifyIndex  ChangeType = "modify_index"

	// Vector configuration changes
	ChangeTypeModifyVectorConfig ChangeType = "modify_vector_config"

	// Replication changes
	ChangeTypeModifyReplication ChangeType = "modify_replication"

	// Multi-tenancy changes
	ChangeTypeEnableMultiTenancy  ChangeType = "enable_multi_tenancy"
	ChangeTypeDisableMultiTenancy ChangeType = "disable_multi_tenancy"
)

// MigrationPlan describes how to execute a schema change
type MigrationPlan struct {
	// ID is the unique identifier for this migration
	ID string `json:"id"`

	// Steps are the individual operations to execute
	Steps []MigrationStep `json:"steps"`

	// Estimate is the estimated duration for the migration
	Estimate time.Duration `json:"estimate"`

	// Impact describes the impact on the system
	Impact ImpactAnalysis `json:"impact"`

	// Strategy defines the migration strategy to use
	Strategy MigrationStrategy `json:"strategy"`

	// CreatedAt is when this plan was created
	CreatedAt time.Time `json:"createdAt"`
}

// MigrationStep represents a single step in a migration
type MigrationStep struct {
	// Type of step (schema_change, backfill, validation, etc.)
	Type StepType `json:"type"`

	// Description of what this step does
	Description string `json:"description"`

	// Query or operation to execute (optional)
	Query string `json:"query,omitempty"`

	// Reversible indicates if this step can be undone
	Reversible bool `json:"reversible"`

	// Blocking indicates if this step blocks read/write operations
	Blocking bool `json:"blocking"`

	// Estimate is the estimated duration for this step
	Estimate time.Duration `json:"estimate"`

	// Order is the execution order within the plan
	Order int `json:"order"`

	// Status tracks the execution status
	Status MigrationStepStatus `json:"status"`

	// Error contains error details if the step failed
	Error string `json:"error,omitempty"`
}

// StepType identifies the type of migration step
type StepType string

const (
	StepTypeSchemaChange   StepType = "schema_change"
	StepTypeBackfill       StepType = "backfill"
	StepTypeValidation     StepType = "validation"
	StepTypeIndexRebuild   StepType = "index_rebuild"
	StepTypeDualWriteStart StepType = "dual_write_start"
	StepTypeDualWriteStop  StepType = "dual_write_stop"
	StepTypeSwitchReads    StepType = "switch_reads"
	StepTypeCleanup        StepType = "cleanup"
)

// MigrationStepStatus tracks the status of a migration step
type MigrationStepStatus string

const (
	StepPending    MigrationStepStatus = "pending"
	StepInProgress MigrationStepStatus = "in_progress"
	StepCompleted  MigrationStepStatus = "completed"
	StepFailed     MigrationStepStatus = "failed"
	StepSkipped    MigrationStepStatus = "skipped"
)

// MigrationStrategy defines how a migration should be executed
type MigrationStrategy string

const (
	// StrategyImmediate applies changes immediately (may cause downtime)
	StrategyImmediate MigrationStrategy = "immediate"

	// StrategyBlueGreen uses blue-green deployment pattern
	StrategyBlueGreen MigrationStrategy = "blue_green"

	// StrategyShadow uses shadow properties for breaking changes
	StrategyShadow MigrationStrategy = "shadow"

	// StrategyBackground runs migration in background
	StrategyBackground MigrationStrategy = "background"

	// StrategyExpandContract uses expand/contract pattern
	StrategyExpandContract MigrationStrategy = "expand_contract"
)

// ImpactAnalysis describes the impact of a migration
type ImpactAnalysis struct {
	// AffectedObjects is the number of objects that will be modified
	AffectedObjects int64 `json:"affectedObjects"`

	// AffectedShards is the number of shards that will be modified
	AffectedShards int `json:"affectedShards"`

	// EstimatedDuration is the estimated time to complete
	EstimatedDuration time.Duration `json:"estimatedDuration"`

	// RequiresDowntime indicates if downtime is required
	RequiresDowntime bool `json:"requiresDowntime"`

	// BlocksWrites indicates if writes will be blocked
	BlocksWrites bool `json:"blocksWrites"`

	// BlocksReads indicates if reads will be blocked
	BlocksReads bool `json:"blocksReads"`

	// MemoryImpact estimates additional memory usage (in bytes)
	MemoryImpact int64 `json:"memoryImpact"`

	// DiskImpact estimates additional disk usage (in bytes)
	DiskImpact int64 `json:"diskImpact"`

	// RiskLevel indicates the risk level of the migration
	RiskLevel RiskLevel `json:"riskLevel"`
}

// RiskLevel indicates the risk level of a migration
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)
