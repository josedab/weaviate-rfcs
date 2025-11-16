# Schema Migration Framework - Implementation Guide

This document describes the implementation of RFC 02: Schema Migration Framework for Weaviate.

## Overview

The schema migration framework has been implemented as a CLI tool (`weaviate-migrate`) with supporting packages in the Weaviate codebase.

## Architecture

### Directory Structure

```
weaviate/
├── cmd/
│   └── weaviate-migrate/          # CLI tool
│       ├── main.go                 # CLI commands and entry point
│       └── README.md               # CLI documentation
├── pkg/
│   └── migrate/                    # Migration framework core
│       ├── types.go                # Type definitions
│       ├── parser.go               # YAML parser
│       ├── executor.go             # Migration executor
│       ├── history.go              # Migration history manager
│       └── README.md               # Framework documentation
└── examples/
    ├── migrations/                 # Example migration files
    │   ├── 001_initial_schema.yaml
    │   ├── 002_add_sentiment.yaml
    │   ├── 003_optimize_vector_index.yaml
    │   ├── 004_enable_compression.yaml
    │   └── 005_complex_multi_step.yaml
    └── weaviate-migrate.yaml       # Example config
```

## Components

### 1. Type Definitions (`pkg/migrate/types.go`)

Defines the core data structures:

- **Migration**: Represents a migration file with version, operations, and rollback
- **Operation**: Represents a single migration operation
- **ValidationRule**: Pre/post-migration validation rules
- **Config**: Configuration for weaviate-migrate tool
- **MigrationHistory**: Record of applied migrations
- **MigrationCheckpoint**: State snapshot for rollback
- **MigrationPlan**: Execution plan for pending migrations

### 2. YAML Parser (`pkg/migrate/parser.go`)

Handles parsing and validation of migration files:

- **ParseMigration()**: Parse single migration file
- **ParseAllMigrations()**: Parse all migrations in directory
- **validateMigration()**: Validate migration structure
- **ParseConfig()**: Parse configuration file
- **GetNextMigrationVersion()**: Determine next version number
- **GenerateMigrationFilename()**: Create standardized filename

### 3. Migration Executor (`pkg/migrate/executor.go`)

Core migration execution engine:

- **Apply()**: Execute a migration with rollback support
- **executeOperation()**: Execute single operation
- **Operation handlers**:
  - `addProperty()`: Add property to class
  - `updateVectorIndexConfig()`: Update index settings
  - `enableCompression()`: Enable vector compression
  - `addClass()`: Add new class
  - `deleteProperty()`: Delete property (for rollback)
- **Validation**:
  - `validatePre()`: Pre-flight validation
  - `validatePost()`: Post-migration validation
- **Rollback**:
  - `createCheckpoint()`: Snapshot for rollback
  - `rollback()`: Execute rollback operations
- **Planning**:
  - `GeneratePlan()`: Generate execution plan
  - `dryRunMigration()`: Simulate migration

### 4. History Manager (`pkg/migrate/history.go`)

Tracks migration history:

- **InitializeHistoryCollection()**: Create system collection
- **RecordMigration()**: Record applied migration
- **GetMigrationHistory()**: Retrieve history
- **GetAppliedVersions()**: List applied version numbers
- **GetCurrentVersion()**: Get current schema version
- **IsMigrationApplied()**: Check if version applied

### 5. CLI Tool (`cmd/weaviate-migrate/main.go`)

Command-line interface using Cobra:

Commands:
- **init**: Initialize project structure
- **create**: Create new migration file
- **plan**: Show pending migrations
- **apply**: Apply migrations
- **rollback**: Rollback migrations
- **history**: Show migration history
- **validate**: Validate schema state

## Migration File Format

### Structure

```yaml
version: int                    # Required, sequential
from_version: int              # Optional, for validation
description: string            # Required
author: string                 # Optional
estimated_duration: string     # Optional

validation:                    # Pre-flight checks
  - type: validation_type
    # ... fields

operations:                    # Migration steps
  - type: operation_type
    # ... fields

rollback:                      # Rollback steps
  - type: operation_type
    # ... fields

validation_after:              # Post-migration checks
  - type: validation_type
    # ... fields
```

### Supported Operations

1. **add_property**: Add property to class
2. **update_vector_index_config**: Update vector index
3. **reindex_property**: Reindex property
4. **add_class**: Add new class
5. **update_class**: Update class config
6. **delete_property**: Delete property
7. **enable_compression**: Enable compression
8. **disable_compression**: Disable compression
9. **restore_vector_index_config**: Restore index config
10. **restore_from_backup**: Restore from backup

### Supported Validations

1. **class_exists**: Check class exists
2. **property_exists**: Check property exists
3. **min_weaviate_version**: Check minimum version
4. **index_healthy**: Check index health
5. **data_integrity**: Check data integrity

## Migration Execution Flow

```
1. Load configuration
   ├─ Parse weaviate-migrate.yaml
   └─ Create Weaviate client

2. Parse migrations
   ├─ Read all YAML files from migrations/
   ├─ Validate structure
   └─ Sort by version

3. Generate plan
   ├─ Estimate duration
   ├─ Calculate impact
   └─ Assess risk level

4. Execute migration
   ├─ Pre-flight validation
   ├─ Create checkpoint
   ├─ Execute operations sequentially
   │  ├─ Operation 1
   │  ├─ Operation 2
   │  └─ ...
   ├─ Post-migration validation
   └─ Record in history

5. On failure
   ├─ Execute rollback operations
   ├─ Restore from checkpoint
   └─ Record failure in history
```

## Configuration

### weaviate-migrate.yaml

```yaml
weaviate:
  host: localhost:8080
  scheme: http
  api_key: ""

migrations_dir: ./migrations

# Safety
dry_run_by_default: true
auto_backup: true
max_concurrent_operations: 1

# Timeouts
operation_timeout: 30m
migration_timeout: 2h
```

## Migration History Storage

Migrations are tracked in a system collection `_MigrationHistory`:

```json
{
  "class": "_MigrationHistory",
  "properties": [
    {"name": "version", "dataType": ["int"]},
    {"name": "description", "dataType": ["text"]},
    {"name": "applied_at", "dataType": ["date"]},
    {"name": "duration_ms", "dataType": ["int"]},
    {"name": "status", "dataType": ["text"]},
    {"name": "applied_by", "dataType": ["text"]}
  ]
}
```

Status values:
- `success`: Migration completed
- `failed`: Migration failed
- `rolled_back`: Migration rolled back
- `in_progress`: Currently running

## Example Usage

### 1. Initialize Project

```bash
$ weaviate-migrate init
✓ Created: migrations/
✓ Created: weaviate-migrate.yaml
```

### 2. Create Migration

```bash
$ weaviate-migrate create add-sentiment-field
✓ Created: migrations/001_add-sentiment-field.yaml
```

### 3. Edit Migration

```yaml
version: 1
description: "Add sentiment analysis"

operations:
  - type: add_property
    class: Article
    property:
      name: sentiment
      dataType: ["number"]
    default_value: 0.0
    backfill: true

rollback:
  - type: delete_property
    class: Article
    property_name: sentiment
```

### 4. Plan Migration

```bash
$ weaviate-migrate plan

Migration Plan
==============
Pending migrations: 1

Migration 001: Add sentiment analysis
  Operations:
    1. add_property (Article.sentiment)

Total operations: 1
Estimated duration: 5s
Risk level: low
```

### 5. Apply Migration

```bash
$ weaviate-migrate apply --force

Starting migration v1: Add sentiment analysis
✓ Pre-flight validation passed
✓ Created checkpoint
[1/1] Executing: add_property
✓ Operation 1 completed
✓ Post-migration validation passed
✓ Recorded in history

✓ Migration v1 completed successfully in 4.8s
```

### 6. View History

```bash
$ weaviate-migrate history

Applied Migrations
==================

✓ v1: Add sentiment analysis (2025-01-10 14:22:15)
   Duration: 4800ms, Applied by: jose, Status: success

Current schema version: 1
```

## Implementation Status

### Phase 1: Core CLI ✅

- [x] YAML parser and validator
- [x] Migration executor
- [x] Basic operation handlers (placeholders)
- [x] CLI commands (init, create, plan, apply, rollback, history, validate)

### Phase 2: Advanced Features (Partial)

- [x] Rollback mechanism
- [x] Dry-run mode
- [x] Migration history tracking
- [ ] Full Weaviate client integration (placeholders present)
- [ ] Actual operation implementations (currently placeholders)
- [ ] Background operation support
- [ ] Progress tracking for long operations

## Next Steps

To make this production-ready, the following needs to be completed:

1. **Weaviate Client Integration**
   - Replace placeholder client code with actual Weaviate client calls
   - Implement schema operations (add property, update config, etc.)
   - Add authentication support

2. **Operation Implementations**
   - Implement actual property addition with Weaviate API
   - Implement backfilling logic
   - Implement vector index updates
   - Implement compression operations

3. **Testing**
   - Unit tests for parser, executor, history manager
   - Integration tests with real Weaviate instance
   - Error injection tests
   - Rollback scenario tests

4. **Documentation**
   - API documentation (GoDoc)
   - User guide with more examples
   - Video tutorial
   - Migration recipes for common scenarios

5. **Production Features**
   - Locking mechanism to prevent concurrent migrations
   - Better error messages and diagnostics
   - Progress bar for long operations
   - Metrics and observability
   - Backup integration

## Dependencies

The implementation uses the following Go packages:

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/spf13/cobra` - CLI framework
- `github.com/sirupsen/logrus` - Logging
- `github.com/weaviate/weaviate/client` - Weaviate client

## Testing

### Unit Tests

```bash
cd pkg/migrate
go test -v
```

### Integration Tests

```bash
# Start Weaviate instance
docker-compose up -d

# Run integration tests
cd cmd/weaviate-migrate
go test -v -tags=integration
```

## Contributing

When contributing to the migration framework:

1. Follow existing code patterns
2. Add tests for new features
3. Update documentation
4. Run linters: `make lint`
5. Test with real Weaviate instance

## References

- [RFC 02: Schema Migration Framework](rfcs/02-schema-migration-framework.md)
- [Migration Framework README](pkg/migrate/README.md)
- [CLI Tool README](cmd/weaviate-migrate/README.md)
- [Example Migrations](examples/migrations/)

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
