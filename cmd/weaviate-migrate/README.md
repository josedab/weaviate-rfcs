# weaviate-migrate CLI

Command-line tool for managing Weaviate schema migrations with declarative YAML files.

## Overview

`weaviate-migrate` is a production-ready CLI tool that provides:

- Declarative schema migrations using YAML
- Migration history tracking
- Automatic rollback on failure
- Dry-run mode for safe previewing
- Zero-downtime migration strategies

## Installation

### Build from Source

```bash
cd cmd/weaviate-migrate
go build -o weaviate-migrate
sudo mv weaviate-migrate /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/weaviate/weaviate/cmd/weaviate-migrate@latest
```

## Quick Start

```bash
# Initialize new project
weaviate-migrate init

# Create migration
weaviate-migrate create add-sentiment-field

# Preview changes
weaviate-migrate plan

# Apply migrations
weaviate-migrate apply --force

# View history
weaviate-migrate history
```

## Commands

- `init` - Initialize migrations directory and config
- `create <name>` - Create new migration file
- `plan` - Show pending migrations (dry-run)
- `apply` - Apply pending migrations
- `rollback` - Rollback last migration
- `history` - Show migration history
- `validate` - Validate current schema state

## Global Flags

- `-c, --config` - Config file path (default: weaviate-migrate.yaml)
- `-v, --verbose` - Verbose output

## Example Workflow

1. **Initialize project:**
```bash
$ weaviate-migrate init
✓ Created: migrations/
✓ Created: weaviate-migrate.yaml
```

2. **Create migration:**
```bash
$ weaviate-migrate create add-sentiment
✓ Created: migrations/001_add-sentiment.yaml
```

3. **Edit migration file:**
```yaml
version: 1
description: "Add sentiment field"
operations:
  - type: add_property
    class: Article
    property:
      name: sentiment
      dataType: ["number"]
```

4. **Dry-run:**
```bash
$ weaviate-migrate plan
Pending migrations: 1
Migration 001: Add sentiment field
  Operations: 1
  Estimated duration: 5s
```

5. **Apply:**
```bash
$ weaviate-migrate apply --force
Starting migration v1: Add sentiment field
✓ Migration v1 completed successfully in 4.8s
```

## Configuration

Edit `weaviate-migrate.yaml`:

```yaml
weaviate:
  host: localhost:8080
  scheme: http

migrations_dir: ./migrations
dry_run_by_default: true
```

## Documentation

For detailed documentation, see:
- [Migration Framework README](../../pkg/migrate/README.md)
- [RFC 02: Schema Migration Framework](../../rfcs/02-schema-migration-framework.md)
- [Example Migrations](../../examples/migrations/)

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
