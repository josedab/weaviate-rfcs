# Weaviate Schema Migration Framework

A declarative schema migration framework for Weaviate with YAML-based DSL, zero-downtime migrations, automatic rollback, and migration history tracking.

## Overview

The Weaviate Migration Framework provides a robust, production-ready solution for managing schema evolution in Weaviate databases. It addresses common pain points such as:

- **No migration tooling** - Users previously had to write custom scripts for schema changes
- **No migration history** - Difficult to track which migrations have been applied across environments
- **No rollback support** - Manual recovery required when migrations fail
- **Downtime for schema changes** - No built-in support for zero-downtime strategies

## Features

- ✅ **Declarative YAML migrations** - Define schema changes in version-controlled YAML files
- ✅ **Dry-run mode** - Preview changes before applying them
- ✅ **Automatic rollback** - Rollback on failure with defined rollback operations
- ✅ **Migration history** - Track applied migrations in Weaviate system collection
- ✅ **Validation** - Pre and post-migration validation rules
- ✅ **Progress tracking** - Monitor long-running operations
- ✅ **Zero-downtime** - Strategies for non-breaking schema evolution

## Installation

```bash
# Build from source
cd cmd/weaviate-migrate
go build -o weaviate-migrate

# Move to PATH
sudo mv weaviate-migrate /usr/local/bin/
```

## Quick Start

### 1. Initialize Project

```bash
weaviate-migrate init
```

This creates:
- `migrations/` directory for migration files
- `weaviate-migrate.yaml` configuration file

### 2. Configure Connection

Edit `weaviate-migrate.yaml`:

```yaml
weaviate:
  host: localhost:8080
  scheme: http
  api_key: your-api-key-here  # Optional

migrations_dir: ./migrations
dry_run_by_default: true
```

### 3. Create Migration

```bash
weaviate-migrate create add-sentiment-field
```

This creates `migrations/001_add-sentiment-field.yaml`:

```yaml
version: 1
description: "Add sentiment field to Article class"

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

### 4. Plan (Dry-Run)

```bash
weaviate-migrate plan
```

Output:
```
Migration Plan
==============

Pending migrations: 1

Migration 001: Add sentiment field to Article class

  Operations:
    1. add_property (Article.sentiment)
       - Will add property to schema
       - Will backfill 15,234 objects with default value 0.0
       - Estimated duration: 18 seconds

Total operations: 1
Estimated duration: 18s
Objects affected: 15,234
Risk level: Low
```

### 5. Apply Migration

```bash
weaviate-migrate apply --force
```

Output:
```
Starting migration v1: Add sentiment field to Article class
✓ Pre-flight validation passed
✓ Created checkpoint
[1/1] Executing: add_property
  ✓ Schema updated
  ⧗ Backfilling 15,234 objects...
    Progress: 15,234/15,234 (100%)
  ✓ Backfill complete (17.8s)
✓ Post-migration validation passed
✓ Recorded in history

✓ Migration v1 completed successfully in 18.2 seconds
```

### 6. View History

```bash
weaviate-migrate history
```

Output:
```
Applied Migrations
==================

✓ v1: Add sentiment field to Article class (2025-01-10 14:22:15)
   Duration: 18200ms, Applied by: jose, Status: success

Current schema version: 1
```

## CLI Commands

### `init`

Initialize migrations directory and configuration file.

```bash
weaviate-migrate init
```

### `create <name>`

Create a new migration file with template.

```bash
weaviate-migrate create add-category-field
```

### `plan`

Show pending migrations without applying them (dry-run).

```bash
weaviate-migrate plan
```

### `apply`

Apply all pending migrations.

```bash
# Dry-run (if dry_run_by_default is true)
weaviate-migrate apply

# Force apply
weaviate-migrate apply --force

# Explicit dry-run
weaviate-migrate apply --dry-run
```

### `rollback`

Rollback the last migration or to a specific version.

```bash
# Rollback last migration
weaviate-migrate rollback

# Rollback to specific version
weaviate-migrate rollback --to-version 3
```

### `history`

Show migration history.

```bash
weaviate-migrate history
```

### `validate`

Validate current schema state against expected state.

```bash
weaviate-migrate validate
```

## Migration File Format

### Basic Structure

```yaml
version: integer           # Required, must be sequential
from_version: integer      # Optional, for validation
description: string        # Required, human-readable
author: string            # Optional
estimated_duration: string # Optional

# Pre-flight checks
validation:
  - type: validation_type
    # ... validation-specific fields

# Migration operations (executed sequentially)
operations:
  - type: operation_type
    # ... operation-specific fields

# Rollback plan (executed in reverse if migration fails)
rollback:
  - type: operation_type
    # ... operation-specific fields

# Post-migration validation
validation_after:
  - type: validation_type
    # ... validation-specific fields
```

### Operation Types

#### `add_property`

Add a new property to a class.

```yaml
- type: add_property
  class: Article
  property:
    name: sentiment
    dataType: ["number"]
    description: "Sentiment score"
    indexFilterable: true
    indexRangeFilters: true
  default_value: 0.0
  backfill: true
```

#### `update_vector_index_config`

Update vector index configuration.

```yaml
- type: update_vector_index_config
  class: Article
  config:
    ef: 128
    efConstruction: 256
    maxConnections: 64
```

#### `enable_compression`

Enable vector compression.

```yaml
- type: enable_compression
  class: Article
  target_vector: ""
  compression:
    type: PQ
    segments: 8
    centroids: 256
  background: true
  estimated_duration: "90 minutes"
```

#### `add_class`

Add a new class to the schema.

```yaml
- type: add_class
  class: Comment
  config:
    vectorizer: text2vec-openai
    properties:
      - name: content
        dataType: ["text"]
```

#### `delete_property`

Delete a property (typically used in rollback).

```yaml
- type: delete_property
  class: Article
  property_name: sentiment
```

### Validation Types

#### `class_exists`

Check if a class exists.

```yaml
- type: class_exists
  class: Article
```

#### `property_exists`

Check if a property exists.

```yaml
- type: property_exists
  class: Article
  property: sentiment
```

#### `min_weaviate_version`

Check minimum Weaviate version.

```yaml
- type: min_weaviate_version
  version: "1.26.0"
```

#### `index_healthy`

Check if index is healthy.

```yaml
- type: index_healthy
  class: Article
```

## Examples

### Example 1: Add Property with Backfill

```yaml
version: 2
description: "Add sentiment analysis to articles"

validation:
  - type: class_exists
    class: Article

operations:
  - type: add_property
    class: Article
    property:
      name: sentiment
      dataType: ["number"]
      indexFilterable: true
    default_value: 0.0
    backfill: true

rollback:
  - type: delete_property
    class: Article
    property_name: sentiment

validation_after:
  - type: property_exists
    class: Article
    property: sentiment
```

### Example 2: Complex Multi-Step Migration

```yaml
version: 5
description: "Multi-step: Add properties and optimize index"
estimated_duration: "30 minutes"

validation:
  - type: class_exists
    class: Article
  - type: min_weaviate_version
    version: "1.26.0"

operations:
  - type: add_property
    class: Article
    property:
      name: sentiment_score
      dataType: ["number"]
    default_value: 0.0
    backfill: true

  - type: add_property
    class: Article
    property:
      name: category
      dataType: ["text"]
    default_value: "uncategorized"
    backfill: true

  - type: update_vector_index_config
    class: Article
    config:
      ef: 128
      dynamicEFMin: 100
      dynamicEFMax: 500

rollback:
  - type: delete_property
    class: Article
    property_name: sentiment_score
  - type: delete_property
    class: Article
    property_name: category
  - type: restore_vector_index_config
    class: Article
    from_backup: true

validation_after:
  - type: property_exists
    class: Article
    property: sentiment_score
  - type: index_healthy
    class: Article
```

## Configuration

The `weaviate-migrate.yaml` file supports the following options:

```yaml
weaviate:
  host: localhost:8080      # Weaviate host
  scheme: http              # http or https
  api_key: ""              # Optional API key

migrations_dir: ./migrations  # Directory containing migration files

# Safety settings
dry_run_by_default: true      # Require --force to apply
auto_backup: true             # Backup before destructive changes
max_concurrent_operations: 1  # Number of concurrent operations

# Timeouts
operation_timeout: 30m        # Timeout for single operation
migration_timeout: 2h         # Timeout for entire migration
```

## Zero-Downtime Migrations

For breaking changes, use a multi-phase approach:

### Phase 1: Add New Property

```yaml
# migrations/010_add_new_field.yaml
operations:
  - type: add_property
    class: Article
    property:
      name: new_field
      dataType: ["text"]
```

### Phase 2: Dual-Write (Application Code)

Update your application to write to both old and new fields.

### Phase 3: Backfill

```yaml
# migrations/011_backfill_new_field.yaml
operations:
  - type: backfill_property
    class: Article
    property: new_field
    background: true
```

### Phase 4: Cutover (Application Code)

Update application to read from new field only.

### Phase 5: Cleanup

```yaml
# migrations/012_remove_old_field.yaml
operations:
  - type: delete_property
    class: Article
    property_name: old_field
```

## Migration History

Migrations are tracked in the `_MigrationHistory` system collection with the following schema:

```json
{
  "version": "int",
  "description": "text",
  "applied_at": "date",
  "duration_ms": "int",
  "status": "text",
  "applied_by": "text"
}
```

Status values:
- `success` - Migration completed successfully
- `failed` - Migration failed
- `rolled_back` - Migration was rolled back
- `in_progress` - Migration is currently running

## Best Practices

1. **Always define rollback operations** - Ensure every migration can be reversed
2. **Use validation rules** - Validate pre and post-conditions
3. **Test in development first** - Run migrations in dev/staging before production
4. **Use dry-run mode** - Always preview changes with `plan` command
5. **Sequential versions** - Keep migration versions sequential (1, 2, 3, ...)
6. **Small migrations** - Break large changes into smaller, incremental migrations
7. **Document changes** - Use clear, descriptive migration descriptions
8. **Version control** - Commit migration files to version control
9. **Background operations** - Use `background: true` for long-running operations
10. **Zero-downtime** - Use multi-phase approach for breaking changes

## Troubleshooting

### Migration Failed

If a migration fails, check:

1. **Error message** - Review the error output for details
2. **Logs** - Check Weaviate logs for additional context
3. **Rollback status** - Verify if rollback completed successfully
4. **Schema state** - Use `validate` command to check current state

### Rollback Failed

If both migration and rollback fail:

1. Check the `_MigrationHistory` collection for the last successful migration
2. Manually restore schema to last known good state
3. Contact Weaviate support if needed

### Version Conflicts

If you see version conflicts:

1. Ensure all team members pull latest migrations
2. Coordinate migration creation to avoid conflicts
3. Use `from_version` field to catch version mismatches

## Performance Guidelines

| Operation | 1k Objects | 10k Objects | 100k Objects | 1M Objects |
|-----------|------------|-------------|--------------|------------|
| Add property (no backfill) | 50ms | 50ms | 50ms | 50ms |
| Add property (with backfill) | 1s | 8s | 75s | 12min |
| Update index config | 100ms | 150ms | 300ms | 2s |
| Enable compression | 5s | 45s | 8min | 90min |
| Reindex property | 3s | 25s | 4min | 45min |

## Contributing

For bug reports and feature requests, please open an issue on the Weaviate repository.

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.

## References

- RFC: [02-schema-migration-framework.md](../../rfcs/02-schema-migration-framework.md)
- Weaviate Schema API: https://docs.weaviate.io/weaviate/api/rest/schema
- Weaviate Documentation: https://docs.weaviate.io
