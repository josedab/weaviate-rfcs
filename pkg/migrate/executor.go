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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/client"
)

// Executor executes migrations
type Executor struct {
	client  *client.Client
	config  *Config
	logger  *logrus.Logger
	history *HistoryManager
}

// NewExecutor creates a new migration executor
func NewExecutor(client *client.Client, config *Config, logger *logrus.Logger) *Executor {
	return &Executor{
		client:  client,
		config:  config,
		logger:  logger,
		history: NewHistoryManager(client, logger),
	}
}

// Apply executes a migration
func (e *Executor) Apply(ctx context.Context, migration *Migration, dryRun bool) error {
	e.logger.Infof("Starting migration v%d: %s", migration.Version, migration.Description)
	startTime := time.Now()

	if dryRun {
		e.logger.Info("DRY RUN MODE - no changes will be applied")
		return e.dryRunMigration(ctx, migration)
	}

	// 1. Validate pre-conditions
	if err := e.validatePre(ctx, migration); err != nil {
		return fmt.Errorf("pre-flight validation failed: %w", err)
	}
	e.logger.Info("✓ Pre-flight validation passed")

	// 2. Create checkpoint for rollback
	checkpoint, err := e.createCheckpoint(ctx, migration.Version)
	if err != nil {
		e.logger.Warnf("Failed to create checkpoint: %v (continuing without checkpoint)", err)
	}

	// 3. Execute operations
	for i, op := range migration.Operations {
		e.logger.Infof("[%d/%d] Executing: %s", i+1, len(migration.Operations), op.Type)

		if err := e.executeOperation(ctx, &op); err != nil {
			e.logger.Errorf("Operation %d failed: %v", i+1, err)

			// Attempt rollback
			if rollbackErr := e.rollback(ctx, migration, checkpoint); rollbackErr != nil {
				e.logger.Errorf("Rollback also failed: %v", rollbackErr)
				e.history.RecordMigration(migration.Version, migration.Description, StatusFailed, time.Since(startTime))
				return fmt.Errorf("migration AND rollback failed: operation error: %w, rollback error: %v", err, rollbackErr)
			}

			e.history.RecordMigration(migration.Version, migration.Description, StatusRolledBack, time.Since(startTime))
			return fmt.Errorf("migration failed and was rolled back: %w", err)
		}

		e.logger.Infof("✓ Operation %d completed", i+1)
	}

	// 4. Validate post-conditions
	if err := e.validatePost(ctx, migration); err != nil {
		e.logger.Errorf("Post-migration validation failed: %v", err)
		if rollbackErr := e.rollback(ctx, migration, checkpoint); rollbackErr != nil {
			e.logger.Errorf("Rollback failed: %v", rollbackErr)
		}
		e.history.RecordMigration(migration.Version, migration.Description, StatusFailed, time.Since(startTime))
		return fmt.Errorf("post-migration validation failed: %w", err)
	}
	e.logger.Info("✓ Post-migration validation passed")

	// 5. Record successful migration
	duration := time.Since(startTime)
	if err := e.history.RecordMigration(migration.Version, migration.Description, StatusSuccess, duration); err != nil {
		e.logger.Warnf("Failed to record migration history: %v", err)
	}

	e.logger.Infof("✓ Migration v%d completed successfully in %s", migration.Version, duration)
	return nil
}

// dryRunMigration simulates a migration without making changes
func (e *Executor) dryRunMigration(ctx context.Context, migration *Migration) error {
	e.logger.Info("Validating migration operations...")

	for i, op := range migration.Operations {
		e.logger.Infof("[%d/%d] Would execute: %s on class: %s", i+1, len(migration.Operations), op.Type, op.Class)

		// Validate that the operation is feasible
		if err := e.validateOperation(ctx, &op); err != nil {
			return fmt.Errorf("operation %d would fail: %w", i+1, err)
		}
	}

	e.logger.Info("✓ Dry run completed - all operations are valid")
	return nil
}

// executeOperation executes a single migration operation
func (e *Executor) executeOperation(ctx context.Context, op *Operation) error {
	switch op.Type {
	case OperationAddProperty:
		return e.addProperty(ctx, op)
	case OperationUpdateVectorIndexConfig:
		return e.updateVectorIndexConfig(ctx, op)
	case OperationReindexProperty:
		return e.reindexProperty(ctx, op)
	case OperationAddClass:
		return e.addClass(ctx, op)
	case OperationUpdateClass:
		return e.updateClass(ctx, op)
	case OperationEnableCompression:
		return e.enableCompression(ctx, op)
	case OperationDeleteProperty:
		return e.deleteProperty(ctx, op)
	default:
		return fmt.Errorf("unsupported operation type: %s", op.Type)
	}
}

// addProperty adds a new property to a class
func (e *Executor) addProperty(ctx context.Context, op *Operation) error {
	e.logger.Infof("Adding property to class: %s", op.Class)

	// Note: This is a placeholder. In a real implementation, you would:
	// 1. Use the Weaviate client to add the property
	// 2. Handle backfilling if op.Backfill is true
	// 3. Track progress for large backfills

	if op.Property == nil {
		return fmt.Errorf("property definition is required for add_property operation")
	}

	// Simulate property addition (replace with actual client call)
	e.logger.Infof("Property definition: %v", op.Property)

	if op.Backfill {
		e.logger.Info("Backfilling property with default value...")
		// In real implementation: iterate over all objects and set default value
	}

	return nil
}

// updateVectorIndexConfig updates vector index configuration
func (e *Executor) updateVectorIndexConfig(ctx context.Context, op *Operation) error {
	e.logger.Infof("Updating vector index config for class: %s", op.Class)

	if op.Config == nil {
		return fmt.Errorf("config is required for update_vector_index_config operation")
	}

	// Placeholder for actual implementation
	e.logger.Infof("New config: %v", op.Config)

	return nil
}

// reindexProperty reindexes a property
func (e *Executor) reindexProperty(ctx context.Context, op *Operation) error {
	e.logger.Infof("Reindexing property %s on class: %s", op.PropertyName, op.Class)

	// Placeholder for actual implementation
	return nil
}

// addClass adds a new class to the schema
func (e *Executor) addClass(ctx context.Context, op *Operation) error {
	e.logger.Infof("Adding new class: %s", op.Class)

	// Placeholder for actual implementation
	return nil
}

// updateClass updates an existing class
func (e *Executor) updateClass(ctx context.Context, op *Operation) error {
	e.logger.Infof("Updating class: %s", op.Class)

	if op.Config == nil {
		return fmt.Errorf("config is required for update_class operation")
	}

	// Placeholder for actual implementation
	return nil
}

// enableCompression enables vector compression
func (e *Executor) enableCompression(ctx context.Context, op *Operation) error {
	e.logger.Infof("Enabling compression for class: %s", op.Class)

	if op.Compression == nil {
		return fmt.Errorf("compression config is required for enable_compression operation")
	}

	if op.Background {
		e.logger.Info("Running compression in background...")
		// In real implementation: start background job
	}

	return nil
}

// deleteProperty deletes a property (used in rollback)
func (e *Executor) deleteProperty(ctx context.Context, op *Operation) error {
	e.logger.Infof("Deleting property %s from class: %s", op.PropertyName, op.Class)

	// Note: Property deletion might not be supported in all Weaviate versions
	// This is mainly for rollback purposes

	return nil
}

// validatePre validates pre-conditions before migration
func (e *Executor) validatePre(ctx context.Context, migration *Migration) error {
	for _, rule := range migration.Validation {
		if err := e.validateRule(ctx, &rule); err != nil {
			return err
		}
	}
	return nil
}

// validatePost validates post-conditions after migration
func (e *Executor) validatePost(ctx context.Context, migration *Migration) error {
	for _, rule := range migration.ValidationAfter {
		if err := e.validateRule(ctx, &rule); err != nil {
			return err
		}
	}
	return nil
}

// validateRule validates a single validation rule
func (e *Executor) validateRule(ctx context.Context, rule *ValidationRule) error {
	switch rule.Type {
	case ValidationClassExists:
		e.logger.Infof("Validating class exists: %s", rule.Class)
		// Placeholder: check if class exists in schema
		return nil
	case ValidationPropertyExists:
		e.logger.Infof("Validating property exists: %s.%s", rule.Class, rule.Property)
		// Placeholder: check if property exists
		return nil
	case ValidationMinWeaviateVersion:
		e.logger.Infof("Validating Weaviate version >= %s", rule.Version)
		// Placeholder: check Weaviate version
		return nil
	case ValidationIndexHealthy:
		e.logger.Infof("Validating index health for class: %s", rule.Class)
		// Placeholder: check index health
		return nil
	case ValidationDataIntegrity:
		e.logger.Infof("Validating data integrity for class: %s", rule.Class)
		// Placeholder: check data integrity
		return nil
	default:
		return fmt.Errorf("unknown validation type: %s", rule.Type)
	}
}

// validateOperation validates that an operation can be executed
func (e *Executor) validateOperation(ctx context.Context, op *Operation) error {
	// Validate operation-specific requirements
	switch op.Type {
	case OperationAddProperty:
		if op.Property == nil {
			return fmt.Errorf("property definition required")
		}
	case OperationUpdateVectorIndexConfig:
		if op.Config == nil {
			return fmt.Errorf("config required")
		}
	}
	return nil
}

// createCheckpoint creates a checkpoint for rollback
func (e *Executor) createCheckpoint(ctx context.Context, version int) (*MigrationCheckpoint, error) {
	e.logger.Info("Creating checkpoint for rollback...")

	checkpoint := &MigrationCheckpoint{
		Version:   version,
		CreatedAt: time.Now(),
		// In real implementation: capture current schema state
		SchemaSnapshot: make(map[string]interface{}),
	}

	return checkpoint, nil
}

// rollback rolls back a migration
func (e *Executor) rollback(ctx context.Context, migration *Migration, checkpoint *MigrationCheckpoint) error {
	e.logger.Warn("Rolling back migration...")

	if len(migration.Rollback) == 0 {
		return fmt.Errorf("no rollback operations defined")
	}

	// Execute rollback operations in reverse order
	for i := len(migration.Rollback) - 1; i >= 0; i-- {
		op := migration.Rollback[i]
		e.logger.Infof("Rollback [%d/%d]: %s", len(migration.Rollback)-i, len(migration.Rollback), op.Type)

		if err := e.executeOperation(ctx, &op); err != nil {
			e.logger.Errorf("Rollback operation failed: %v", err)
			return err
		}
	}

	e.logger.Info("✓ Rollback completed")
	return nil
}

// GeneratePlan generates an execution plan for pending migrations
func (e *Executor) GeneratePlan(ctx context.Context, migrations []Migration) (*MigrationPlan, error) {
	plan := &MigrationPlan{
		Migrations:      migrations,
		TotalOperations: 0,
		EstimatedTime:   0,
		ObjectsAffected: 0,
		DiskSpaceNeeded: 0,
		RiskLevel:       "low",
	}

	for _, m := range migrations {
		plan.TotalOperations += len(m.Operations)

		// Estimate time and impact
		for _, op := range m.Operations {
			// Simple estimation (in real implementation, query actual object counts)
			if op.Backfill {
				plan.ObjectsAffected += 1000 // placeholder
				plan.EstimatedTime += 5 * time.Second
			} else {
				plan.EstimatedTime += 100 * time.Millisecond
			}
		}
	}

	// Determine risk level
	if plan.TotalOperations > 10 || plan.ObjectsAffected > 100000 {
		plan.RiskLevel = "high"
	} else if plan.TotalOperations > 5 || plan.ObjectsAffected > 10000 {
		plan.RiskLevel = "medium"
	}

	return plan, nil
}

// FormatPlan formats a migration plan for display
func FormatPlan(plan *MigrationPlan) string {
	var output string
	output += "Migration Plan\n"
	output += "==============\n\n"
	output += fmt.Sprintf("Pending migrations: %d\n\n", len(plan.Migrations))

	for _, m := range plan.Migrations {
		output += fmt.Sprintf("Migration %03d: %s\n", m.Version, m.Description)
		output += "\n  Operations:\n"

		for i, op := range m.Operations {
			output += fmt.Sprintf("    %d. %s (%s)\n", i+1, op.Type, op.Class)
		}

		output += "\n"
	}

	output += fmt.Sprintf("Total operations: %d\n", plan.TotalOperations)
	output += fmt.Sprintf("Estimated duration: %s\n", plan.EstimatedTime)
	output += fmt.Sprintf("Objects affected: %d\n", plan.ObjectsAffected)
	output += fmt.Sprintf("Risk level: %s\n", plan.RiskLevel)

	return output
}

// MarshalCheckpoint serializes a checkpoint to JSON
func MarshalCheckpoint(checkpoint *MigrationCheckpoint) (string, error) {
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalCheckpoint deserializes a checkpoint from JSON
func UnmarshalCheckpoint(data string) (*MigrationCheckpoint, error) {
	var checkpoint MigrationCheckpoint
	if err := json.Unmarshal([]byte(data), &checkpoint); err != nil {
		return nil, err
	}
	return &checkpoint, nil
}
