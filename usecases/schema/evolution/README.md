# Schema Evolution System

This package implements RFC 0011: Automated Schema Evolution and Versioning for Weaviate.

## Overview

The schema evolution system provides:

- **Versioning**: Track all schema changes with metadata and history
- **Compatibility Checking**: Validate that schema changes don't break existing functionality
- **Zero-Downtime Migrations**: Execute schema changes without service interruption
- **Rollback Support**: Revert to previous schema versions if needed
- **Diff and Merge**: Compare schema versions and merge concurrent changes

## Architecture

### Components

1. **Entity Layer** (`entities/schema/evolution/`)
   - `version.go`: Core data types (SchemaVersion, SchemaChange, MigrationPlan)
   - `diff.go`: Schema diff and merge types
   - `compatibility.go`: Compatibility level and validation types

2. **Use Case Layer** (`usecases/schema/evolution/`)
   - `registry.go`: SchemaRegistry for managing versions
   - `validator.go`: CompatibilityValidator with rule engine
   - `differ.go`: SchemaDiffer for computing diffs and merges
   - `migrator.go`: SchemaMigrator for executing migrations
   - `planner.go`: MigrationPlanner for generating migration plans

3. **Storage Layer** (`adapters/repos/schema/evolution/`)
   - `version_store.go`: Persistence for schema versions (BoltDB and in-memory)

## Usage

### Basic Example

```go
// Create components
store := evolution.NewInMemoryVersionStore()
validator := evolution.NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())
planner := evolution.NewMigrationPlanner(evolution.DefaultMigrationConfig())
migrator := evolution.NewSchemaMigrator(executor, planner, evolution.DefaultMigrationConfig())
differ := evolution.NewSchemaDiffer(evolution.DefaultDiffOptions())

// Create registry
config := evolution.DefaultRegistryConfig()
registry := evolution.NewSchemaRegistry(store, validator, migrator, differ, config)

// Register a new schema version
newSchema := &models.Schema{
    Classes: []*models.Class{
        {
            Class: "Article",
            Properties: []*models.Property{
                {Name: "title", DataType: []string{"text"}},
                {Name: "content", DataType: []string{"text"}},
            },
        },
    },
}

version, err := registry.RegisterSchema(newSchema, "alice", "Initial schema")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Registered schema version %d\n", version.ID)
fmt.Printf("Compatibility: %s\n", version.Compatibility)
fmt.Printf("Migration status: %s\n", version.MigrationStatus)
```

### Checking Compatibility

```go
oldSchema := &models.Schema{...}
newSchema := &models.Schema{...}

result, err := validator.Validate(oldSchema, newSchema)
if err != nil {
    log.Fatal(err)
}

if result.Compatible {
    fmt.Printf("Schema is %s compatible\n", result.Level)
} else {
    fmt.Println("Breaking changes detected:")
    for _, issue := range result.Issues {
        fmt.Printf("  - %s: %s\n", issue.Path, issue.Message)
    }
}
```

### Computing Schema Diff

```go
diff, err := registry.Diff(versionID1, versionID2)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Changes from version %d to %d:\n", diff.FromVersion, diff.ToVersion)
fmt.Printf("  Added: %d elements\n", len(diff.Added))
fmt.Printf("  Removed: %d elements\n", len(diff.Removed))
fmt.Printf("  Modified: %d elements\n", len(diff.Modified))

for _, added := range diff.Added {
    fmt.Printf("  + %s\n", added.Path)
}
```

### Executing Migrations

```go
// Plan migration
changes := []evolution.SchemaChange{
    {
        Type:     evolution.ChangeTypeAddProperty,
        Class:    "Article",
        Property: "author",
        After:    &models.Property{Name: "author", DataType: []string{"text"}},
    },
}

plan, err := migrator.Plan(changes)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Migration plan: %d steps\n", len(plan.Steps))
fmt.Printf("Strategy: %s\n", plan.Strategy)
fmt.Printf("Estimated duration: %s\n", plan.Estimate)
fmt.Printf("Risk level: %s\n", plan.Impact.RiskLevel)

// Execute migration
ctx := context.Background()
if err := migrator.Execute(ctx, plan); err != nil {
    log.Fatal(err)
}
```

## Compatibility Rules

The system includes built-in compatibility rules:

| Change | Compatibility | Notes |
|--------|--------------|-------|
| Add optional property | Backward | Old data doesn't have the property |
| Add class | Backward | Old code doesn't know about new class |
| Remove property | Breaking | Existing queries may fail |
| Remove class | Breaking | Existing data and queries affected |
| Change property type | Breaking | Requires data conversion |
| Modify vector config | Backward | Usually non-breaking |
| Add index | Backward | Applied in background |

## Migration Strategies

The planner automatically selects the best migration strategy:

### 1. Immediate Strategy
For simple, non-breaking changes that can be applied instantly.

### 2. Blue-Green Strategy
For adding new properties with backfill:
1. Add property to schema (non-blocking)
2. Backfill data in background
3. Validate backfill
4. Enable property for queries

### 3. Shadow Strategy
For breaking changes like type conversions:
1. Add shadow property (e.g., `field_v2`)
2. Enable dual-write (write to both)
3. Backfill and convert data
4. Validate conversion
5. Switch reads to shadow property
6. Rename shadow property
7. Disable dual-write

### 4. Expand/Contract Strategy
For removing properties:
1. Disable property (stop using in code)
2. Remove from schema
3. Lazy cleanup of data

### 5. Background Strategy
For long-running operations like index rebuilds:
- Execute asynchronously
- Don't block reads or writes
- Track progress

## Configuration

```go
config := evolution.RegistryConfig{
    MaxHistorySize: 100,
    CompatibilityConfig: evolution.CompatibilityConfig{
        Level:                evolution.BackwardCompatible,
        EnforceOnWrite:       true,
        AllowBreakingChanges: false,
    },
    AutoMigrate: false,
    MigrationConfig: evolution.MigrationConfig{
        MaxDuration:          1 * time.Hour,
        BatchSize:            1000,
        Parallelism:          4,
        EnableDualWrite:      true,
        ShadowPropertySuffix: "_v2",
    },
}
```

## Testing

Run tests:

```bash
# Storage layer tests
go test ./adapters/repos/schema/evolution/...

# Use case layer tests
go test ./usecases/schema/evolution/...

# All schema evolution tests
go test ./...schema/evolution/... -v
```

## Implementation Status

### ✅ Completed (Phase 1)

- [x] Entity types (SchemaVersion, SchemaChange, CompatibilityLevel, etc.)
- [x] SchemaRegistry implementation
- [x] CompatibilityValidator with 7 built-in rules
- [x] SchemaDiffer for diff and merge operations
- [x] SchemaMigrator with migration execution
- [x] MigrationPlanner with strategy selection
- [x] BoltDB and in-memory version stores
- [x] Unit tests for all components

### 🚧 TODO (Future Phases)

- [ ] REST API endpoints for version management
- [ ] CLI tool for schema operations
- [ ] Integration with existing schema manager
- [ ] Schema reconstruction from version history
- [ ] Migration execution engine (actual database operations)
- [ ] Progress tracking and monitoring
- [ ] Distributed coordination for cluster environments
- [ ] Integration tests
- [ ] Performance benchmarks
- [ ] Documentation and examples

## Migration Examples

### Example 1: Add Property

```go
// Before
type Article struct {
    Title   string
    Content string
}

// After
type Article struct {
    Title   string
    Content string
    Author  string // New field
}

// Migration steps:
// 1. Add property to schema
// 2. Backfill with default value
// 3. Enable for queries
```

### Example 2: Change Property Type

```go
// Before
type Article struct {
    Published bool // boolean
}

// After
type Article struct {
    Published time.Time // timestamp
}

// Migration steps (shadow strategy):
// 1. Add Published_v2 (timestamp)
// 2. Dual-write to both fields
// 3. Backfill Published_v2 from Published
// 4. Switch reads to Published_v2
// 5. Rename Published_v2 → Published
```

### Example 3: Remove Property

```go
// Before
type Article struct {
    Title    string
    Category string // To be removed
    Tags     []string
}

// After
type Article struct {
    Title string
    Tags  []string
}

// Migration steps:
// 1. Disable Category in code
// 2. Remove from schema
// 3. Lazy cleanup (background)
```

## Performance Characteristics

Based on RFC impact analysis:

| Operation | Objects | Duration | Downtime |
|-----------|---------|----------|----------|
| Add property | 10M | ~12 min | 0s (background) |
| Add required property | 10M | ~18 min | 0s (with default) |
| Change property type | 10M | ~25 min | 0s (shadow mode) |
| Remove property | 10M | ~8 min | 0s (lazy deletion) |
| Add index | 10M | ~35 min | 0s (background) |

## References

- [RFC 0011: Automated Schema Evolution](../../../rfcs/0011-automated-schema-evolution.md)
- [RFC 02: Schema Migration Framework](../../../rfcs/02-schema-migration-framework.md)
- [Expand/Contract Pattern](https://www.martinfowler.com/bliki/ParallelChange.html)

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
