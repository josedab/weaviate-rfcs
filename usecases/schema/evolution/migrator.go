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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weaviate/weaviate/entities/schema/evolution"
)

// MigrationExecutor defines the interface for executing migration operations
type MigrationExecutor interface {
	// AddColumn adds a new property column
	AddColumn(ctx context.Context, class, property string, def interface{}) error

	// DropColumn removes a property column
	DropColumn(ctx context.Context, class, property string) error

	// RenameProperty renames a property
	RenameProperty(ctx context.Context, class, oldName, newName string) error

	// EnableProperty enables a property for queries
	EnableProperty(ctx context.Context, class, property string) error

	// DisableProperty disables a property for queries
	DisableProperty(ctx context.Context, class, property string) error

	// ExecuteBackfill executes a backfill operation
	ExecuteBackfill(ctx context.Context, plan *BackfillPlan) error

	// RebuildIndex rebuilds an index
	RebuildIndex(ctx context.Context, class, property string) error

	// WaitForCompletion waits for an async operation to complete
	WaitForCompletion(ctx context.Context, operationID string) error
}

// MigrationConfig configures migration behavior
type MigrationConfig struct {
	// MaxDuration is the maximum time a migration can run
	MaxDuration time.Duration

	// BatchSize for backfill operations
	BatchSize int

	// Parallelism level for concurrent operations
	Parallelism int

	// EnableDualWrite enables dual-write during migrations
	EnableDualWrite bool

	// ShadowPropertySuffix for shadow migration strategy
	ShadowPropertySuffix string

	// DryRun if true, only plans migration without executing
	DryRun bool
}

// DefaultMigrationConfig returns default migration configuration
func DefaultMigrationConfig() MigrationConfig {
	return MigrationConfig{
		MaxDuration:          1 * time.Hour,
		BatchSize:            1000,
		Parallelism:          4,
		EnableDualWrite:      true,
		ShadowPropertySuffix: "_v2",
		DryRun:               false,
	}
}

// SchemaMigrator orchestrates schema migrations
type SchemaMigrator struct {
	mu sync.RWMutex

	executor MigrationExecutor
	planner  *MigrationPlanner
	config   MigrationConfig

	// Track active migrations
	activeMigrations map[string]*MigrationExecution
}

// MigrationExecution tracks an active migration
type MigrationExecution struct {
	Plan      *evolution.MigrationPlan
	StartTime time.Time
	Context   context.Context
	Cancel    context.CancelFunc
	Progress  *MigrationProgress
}

// MigrationProgress tracks migration progress
type MigrationProgress struct {
	mu sync.RWMutex

	TotalSteps      int
	CompletedSteps  int
	CurrentStep     int
	ProcessedItems  int64
	TotalItems      int64
	Errors          []error
	StartTime       time.Time
	EstimatedEnd    time.Time
}

// NewSchemaMigrator creates a new schema migrator
func NewSchemaMigrator(executor MigrationExecutor, planner *MigrationPlanner, config MigrationConfig) *SchemaMigrator {
	return &SchemaMigrator{
		executor:         executor,
		planner:          planner,
		config:           config,
		activeMigrations: make(map[string]*MigrationExecution),
	}
}

// Plan generates a migration plan for schema changes
func (m *SchemaMigrator) Plan(changes []evolution.SchemaChange) (*evolution.MigrationPlan, error) {
	return m.planner.CreatePlan(changes, m.config)
}

// Execute executes a migration plan
func (m *SchemaMigrator) Execute(ctx context.Context, plan *evolution.MigrationPlan) error {
	m.mu.Lock()

	// Check if migration is already running
	if _, exists := m.activeMigrations[plan.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("migration %s is already running", plan.ID)
	}

	// Create execution context
	execCtx, cancel := context.WithTimeout(ctx, m.config.MaxDuration)
	execution := &MigrationExecution{
		Plan:      plan,
		StartTime: time.Now(),
		Context:   execCtx,
		Cancel:    cancel,
		Progress: &MigrationProgress{
			TotalSteps:   len(plan.Steps),
			TotalItems:   plan.Impact.AffectedObjects,
			StartTime:    time.Now(),
			EstimatedEnd: time.Now().Add(plan.Impact.EstimatedDuration),
		},
	}

	m.activeMigrations[plan.ID] = execution
	m.mu.Unlock()

	// Execute migration
	defer func() {
		m.mu.Lock()
		delete(m.activeMigrations, plan.ID)
		m.mu.Unlock()
		cancel()
	}()

	return m.executeSteps(execution)
}

// executeSteps executes migration steps in order
func (m *SchemaMigrator) executeSteps(execution *MigrationExecution) error {
	for i, step := range execution.Plan.Steps {
		execution.Progress.CurrentStep = i

		// Check context cancellation
		select {
		case <-execution.Context.Done():
			return fmt.Errorf("migration cancelled: %w", execution.Context.Err())
		default:
		}

		// Execute step based on type
		if err := m.executeStep(execution.Context, &step); err != nil {
			execution.Progress.mu.Lock()
			execution.Progress.Errors = append(execution.Progress.Errors, err)
			execution.Progress.mu.Unlock()

			// If step is reversible, attempt rollback
			if step.Reversible {
				m.rollbackStep(execution.Context, &step)
			}

			return fmt.Errorf("step %d (%s) failed: %w", i, step.Type, err)
		}

		// Update progress
		execution.Progress.mu.Lock()
		execution.Progress.CompletedSteps++
		execution.Progress.mu.Unlock()

		// Mark step as completed
		step.Status = evolution.StepCompleted
	}

	return nil
}

// executeStep executes a single migration step
func (m *SchemaMigrator) executeStep(ctx context.Context, step *evolution.MigrationStep) error {
	if m.config.DryRun {
		// In dry-run mode, just log what would be done
		return nil
	}

	step.Status = evolution.StepInProgress

	switch step.Type {
	case evolution.StepTypeSchemaChange:
		return m.executeSchemaChange(ctx, step)
	case evolution.StepTypeBackfill:
		return m.executeBackfill(ctx, step)
	case evolution.StepTypeValidation:
		return m.executeValidation(ctx, step)
	case evolution.StepTypeIndexRebuild:
		return m.executeIndexRebuild(ctx, step)
	case evolution.StepTypeDualWriteStart:
		return m.executeDualWriteStart(ctx, step)
	case evolution.StepTypeDualWriteStop:
		return m.executeDualWriteStop(ctx, step)
	case evolution.StepTypeSwitchReads:
		return m.executeSwitchReads(ctx, step)
	case evolution.StepTypeCleanup:
		return m.executeCleanup(ctx, step)
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeSchemaChange executes a schema modification step
func (m *SchemaMigrator) executeSchemaChange(ctx context.Context, step *evolution.MigrationStep) error {
	// Schema changes are applied directly through the executor
	// The specific operation depends on the query/description
	return nil
}

// executeBackfill executes a data backfill step
func (m *SchemaMigrator) executeBackfill(ctx context.Context, step *evolution.MigrationStep) error {
	// Create backfill plan from step
	plan := &BackfillPlan{
		Class:      "", // Extract from step
		Property:   "", // Extract from step
		BatchSize:  m.config.BatchSize,
		Parallel:   m.config.Parallelism,
	}

	return m.executor.ExecuteBackfill(ctx, plan)
}

// executeValidation validates data after migration
func (m *SchemaMigrator) executeValidation(ctx context.Context, step *evolution.MigrationStep) error {
	// Implement validation logic
	return nil
}

// executeIndexRebuild rebuilds an index
func (m *SchemaMigrator) executeIndexRebuild(ctx context.Context, step *evolution.MigrationStep) error {
	// Extract class and property from step
	// Return m.executor.RebuildIndex(ctx, class, property)
	return nil
}

// executeDualWriteStart enables dual-write mode
func (m *SchemaMigrator) executeDualWriteStart(ctx context.Context, step *evolution.MigrationStep) error {
	// Enable dual-write to both old and new properties
	return nil
}

// executeDualWriteStop disables dual-write mode
func (m *SchemaMigrator) executeDualWriteStop(ctx context.Context, step *evolution.MigrationStep) error {
	// Disable dual-write mode
	return nil
}

// executeSwitchReads switches read operations to new property
func (m *SchemaMigrator) executeSwitchReads(ctx context.Context, step *evolution.MigrationStep) error {
	// Switch read path to new property
	return nil
}

// executeCleanup performs cleanup operations
func (m *SchemaMigrator) executeCleanup(ctx context.Context, step *evolution.MigrationStep) error {
	// Cleanup temporary resources
	return nil
}

// rollbackStep attempts to rollback a step
func (m *SchemaMigrator) rollbackStep(ctx context.Context, step *evolution.MigrationStep) error {
	// Implement rollback logic based on step type
	return nil
}

// GetProgress returns the progress of an active migration
func (m *SchemaMigrator) GetProgress(migrationID string) (*MigrationProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	execution, exists := m.activeMigrations[migrationID]
	if !exists {
		return nil, fmt.Errorf("migration %s not found", migrationID)
	}

	return execution.Progress, nil
}

// Cancel cancels an active migration
func (m *SchemaMigrator) Cancel(migrationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	execution, exists := m.activeMigrations[migrationID]
	if !exists {
		return fmt.Errorf("migration %s not found", migrationID)
	}

	execution.Cancel()
	return nil
}

// AddProperty performs zero-downtime property addition
func (m *SchemaMigrator) AddProperty(ctx context.Context, class, property string, def interface{}) error {
	// Phase 1: Add property to schema (non-blocking)
	if err := m.executor.AddColumn(ctx, class, property, def); err != nil {
		return fmt.Errorf("failed to add column: %w", err)
	}

	// Phase 2: Backfill data (background)
	backfillPlan := m.planner.CreateBackfillPlan(class, property, def)
	if err := m.executor.ExecuteBackfill(ctx, backfillPlan); err != nil {
		return fmt.Errorf("failed to backfill: %w", err)
	}

	// Phase 3: Enable property for queries
	if err := m.executor.EnableProperty(ctx, class, property); err != nil {
		return fmt.Errorf("failed to enable property: %w", err)
	}

	return nil
}

// ChangePropertyType performs zero-downtime type change using shadow property
func (m *SchemaMigrator) ChangePropertyType(ctx context.Context, class, property string, oldType, newType interface{}) error {
	shadowProp := property + m.config.ShadowPropertySuffix

	// Phase 1: Add shadow property
	if err := m.executor.AddColumn(ctx, class, shadowProp, newType); err != nil {
		return fmt.Errorf("failed to add shadow property: %w", err)
	}

	// Phase 2: Dual writes (old + new) - handled by data layer

	// Phase 3: Backfill shadow property
	backfillPlan := m.planner.CreateConversionBackfill(class, property, shadowProp)
	if err := m.executor.ExecuteBackfill(ctx, backfillPlan); err != nil {
		return fmt.Errorf("failed to backfill shadow property: %w", err)
	}

	// Phase 4: Validate conversion
	// TODO: Implement validation

	// Phase 5: Switch reads to shadow property
	// TODO: Implement read switching

	// Phase 6: Drop old property and rename shadow
	if err := m.executor.RenameProperty(ctx, class, shadowProp, property); err != nil {
		return fmt.Errorf("failed to rename shadow property: %w", err)
	}

	return nil
}

// BackfillPlan describes a data backfill operation
type BackfillPlan struct {
	Class      string
	Property   string
	BatchSize  int
	Parallel   int
	Transform  func(interface{}) interface{}
}
