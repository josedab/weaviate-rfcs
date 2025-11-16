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
	"time"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate/entities/schema/evolution"
)

// MigrationPlanner generates migration plans for schema changes
type MigrationPlanner struct {
	config MigrationConfig
}

// NewMigrationPlanner creates a new migration planner
func NewMigrationPlanner(config MigrationConfig) *MigrationPlanner {
	return &MigrationPlanner{
		config: config,
	}
}

// CreatePlan creates a migration plan for a list of schema changes
func (p *MigrationPlanner) CreatePlan(changes []evolution.SchemaChange, config MigrationConfig) (*evolution.MigrationPlan, error) {
	plan := &evolution.MigrationPlan{
		ID:        uuid.New().String(),
		Steps:     make([]evolution.MigrationStep, 0),
		CreatedAt: time.Now(),
	}

	// Analyze changes and determine strategy
	for _, change := range changes {
		steps, err := p.planChange(change, config)
		if err != nil {
			return nil, fmt.Errorf("failed to plan change %s: %w", change.Type, err)
		}
		plan.Steps = append(plan.Steps, steps...)
	}

	// Compute impact analysis
	plan.Impact = p.analyzeImpact(changes, plan.Steps)

	// Determine overall strategy
	plan.Strategy = p.determineStrategy(changes, plan.Impact)

	// Estimate duration
	plan.Estimate = p.estimateDuration(plan.Steps, plan.Impact)

	return plan, nil
}

// planChange creates migration steps for a single schema change
func (p *MigrationPlanner) planChange(change evolution.SchemaChange, config MigrationConfig) ([]evolution.MigrationStep, error) {
	switch change.Type {
	case evolution.ChangeTypeAddClass:
		return p.planAddClass(change), nil
	case evolution.ChangeTypeRemoveClass:
		return p.planRemoveClass(change), nil
	case evolution.ChangeTypeAddProperty:
		return p.planAddProperty(change, config), nil
	case evolution.ChangeTypeRemoveProperty:
		return p.planRemoveProperty(change), nil
	case evolution.ChangeTypeChangePropertyType:
		return p.planChangePropertyType(change, config), nil
	case evolution.ChangeTypeAddIndex:
		return p.planAddIndex(change), nil
	case evolution.ChangeTypeRemoveIndex:
		return p.planRemoveIndex(change), nil
	case evolution.ChangeTypeModifyVectorConfig:
		return p.planModifyVectorConfig(change), nil
	default:
		return nil, fmt.Errorf("unsupported change type: %s", change.Type)
	}
}

// planAddClass creates steps for adding a new class
func (p *MigrationPlanner) planAddClass(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Add class %s", change.Class),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Status:      evolution.StepPending,
		},
	}
}

// planRemoveClass creates steps for removing a class
func (p *MigrationPlanner) planRemoveClass(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeValidation,
			Description: fmt.Sprintf("Validate no references to class %s", change.Class),
			Reversible:  false,
			Blocking:    false,
			Estimate:    1 * time.Second,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Remove class %s", change.Class),
			Reversible:  false,
			Blocking:    true,
			Estimate:    100 * time.Millisecond,
			Status:      evolution.StepPending,
		},
	}
}

// planAddProperty creates steps for adding a property (blue-green strategy)
func (p *MigrationPlanner) planAddProperty(change evolution.SchemaChange, config MigrationConfig) []evolution.MigrationStep {
	steps := []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Add property %s.%s to schema", change.Class, change.Property),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       0,
			Status:      evolution.StepPending,
		},
	}

	// If backfill is needed
	if p.needsBackfill(change) {
		steps = append(steps, evolution.MigrationStep{
			Type:        evolution.StepTypeBackfill,
			Description: fmt.Sprintf("Backfill property %s.%s", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    15 * time.Minute, // Estimated based on data size
			Order:       1,
			Status:      evolution.StepPending,
		})

		steps = append(steps, evolution.MigrationStep{
			Type:        evolution.StepTypeValidation,
			Description: fmt.Sprintf("Validate backfill for %s.%s", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    1 * time.Minute,
			Order:       2,
			Status:      evolution.StepPending,
		})
	}

	steps = append(steps, evolution.MigrationStep{
		Type:        evolution.StepTypeSchemaChange,
		Description: fmt.Sprintf("Enable property %s.%s for queries", change.Class, change.Property),
		Reversible:  true,
		Blocking:    false,
		Estimate:    100 * time.Millisecond,
		Order:       3,
		Status:      evolution.StepPending,
	})

	return steps
}

// planRemoveProperty creates steps for removing a property
func (p *MigrationPlanner) planRemoveProperty(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Disable property %s.%s", change.Class, change.Property),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       0,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Remove property %s.%s from schema", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       1,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeCleanup,
			Description: fmt.Sprintf("Cleanup data for %s.%s (lazy deletion)", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    8 * time.Minute,
			Order:       2,
			Status:      evolution.StepPending,
		},
	}
}

// planChangePropertyType creates steps for changing property type (shadow strategy)
func (p *MigrationPlanner) planChangePropertyType(change evolution.SchemaChange, config MigrationConfig) []evolution.MigrationStep {
	shadowProp := change.Property + config.ShadowPropertySuffix

	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Add shadow property %s.%s", change.Class, shadowProp),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       0,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeDualWriteStart,
			Description: fmt.Sprintf("Enable dual-write for %s.%s", change.Class, change.Property),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       1,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeBackfill,
			Description: fmt.Sprintf("Backfill and convert %s.%s to %s", change.Class, change.Property, shadowProp),
			Reversible:  false,
			Blocking:    false,
			Estimate:    25 * time.Minute,
			Order:       2,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeValidation,
			Description: fmt.Sprintf("Validate conversion for %s.%s", change.Class, shadowProp),
			Reversible:  false,
			Blocking:    false,
			Estimate:    2 * time.Minute,
			Order:       3,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeSwitchReads,
			Description: fmt.Sprintf("Switch reads to %s.%s", change.Class, shadowProp),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       4,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Rename %s.%s to %s.%s", change.Class, shadowProp, change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       5,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeDualWriteStop,
			Description: fmt.Sprintf("Disable dual-write for %s.%s", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       6,
			Status:      evolution.StepPending,
		},
	}
}

// planAddIndex creates steps for adding an index
func (p *MigrationPlanner) planAddIndex(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Add index configuration for %s.%s", change.Class, change.Property),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Order:       0,
			Status:      evolution.StepPending,
		},
		{
			Type:        evolution.StepTypeIndexRebuild,
			Description: fmt.Sprintf("Build index for %s.%s in background", change.Class, change.Property),
			Reversible:  false,
			Blocking:    false,
			Estimate:    35 * time.Minute,
			Order:       1,
			Status:      evolution.StepPending,
		},
	}
}

// planRemoveIndex creates steps for removing an index
func (p *MigrationPlanner) planRemoveIndex(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Remove index configuration for %s.%s", change.Class, change.Property),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Status:      evolution.StepPending,
		},
	}
}

// planModifyVectorConfig creates steps for modifying vector configuration
func (p *MigrationPlanner) planModifyVectorConfig(change evolution.SchemaChange) []evolution.MigrationStep {
	return []evolution.MigrationStep{
		{
			Type:        evolution.StepTypeSchemaChange,
			Description: fmt.Sprintf("Update vector configuration for %s", change.Class),
			Reversible:  true,
			Blocking:    false,
			Estimate:    100 * time.Millisecond,
			Status:      evolution.StepPending,
		},
	}
}

// analyzeImpact analyzes the impact of changes
func (p *MigrationPlanner) analyzeImpact(changes []evolution.SchemaChange, steps []evolution.MigrationStep) evolution.ImpactAnalysis {
	impact := evolution.ImpactAnalysis{
		AffectedObjects:   10000000, // TODO: Get actual count from database
		AffectedShards:    1,
		RequiresDowntime:  false,
		BlocksWrites:      false,
		BlocksReads:       false,
		MemoryImpact:      0,
		DiskImpact:        0,
		RiskLevel:         evolution.RiskLow,
	}

	// Analyze each step
	for _, step := range steps {
		if step.Blocking {
			impact.RequiresDowntime = true
			impact.RiskLevel = evolution.RiskHigh
		}
	}

	// Estimate based on affected objects
	if impact.AffectedObjects > 1000000 {
		impact.RiskLevel = evolution.RiskMedium
	}

	return impact
}

// determineStrategy selects the best migration strategy
func (p *MigrationPlanner) determineStrategy(changes []evolution.SchemaChange, impact evolution.ImpactAnalysis) evolution.MigrationStrategy {
	// Check for breaking changes
	for _, change := range changes {
		if change.Type == evolution.ChangeTypeChangePropertyType {
			return evolution.StrategyShadow
		}
		if change.Type == evolution.ChangeTypeRemoveProperty || change.Type == evolution.ChangeTypeRemoveClass {
			return evolution.StrategyExpandContract
		}
	}

	// Check if backfill is needed
	needsBackfill := false
	for _, change := range changes {
		if change.Type == evolution.ChangeTypeAddProperty {
			needsBackfill = true
			break
		}
	}

	if needsBackfill {
		return evolution.StrategyBlueGreen
	}

	// For simple changes
	if impact.RequiresDowntime {
		return evolution.StrategyImmediate
	}

	return evolution.StrategyBackground
}

// estimateDuration estimates how long the migration will take
func (p *MigrationPlanner) estimateDuration(steps []evolution.MigrationStep, impact evolution.ImpactAnalysis) time.Duration {
	var total time.Duration
	for _, step := range steps {
		total += step.Estimate
	}
	return total
}

// needsBackfill determines if a property addition needs backfilling
func (p *MigrationPlanner) needsBackfill(change evolution.SchemaChange) bool {
	// If there's a default value or transformation, backfill is needed
	return change.After != nil
}

// CreateBackfillPlan creates a backfill plan for a property
func (p *MigrationPlanner) CreateBackfillPlan(class, property string, def interface{}) *BackfillPlan {
	return &BackfillPlan{
		Class:     class,
		Property:  property,
		BatchSize: p.config.BatchSize,
		Parallel:  p.config.Parallelism,
	}
}

// CreateConversionBackfill creates a backfill plan for type conversion
func (p *MigrationPlanner) CreateConversionBackfill(class, oldProp, newProp string) *BackfillPlan {
	return &BackfillPlan{
		Class:     class,
		Property:  newProp,
		BatchSize: p.config.BatchSize,
		Parallel:  p.config.Parallelism,
		Transform: func(value interface{}) interface{} {
			// Type conversion logic here
			return value
		},
	}
}
