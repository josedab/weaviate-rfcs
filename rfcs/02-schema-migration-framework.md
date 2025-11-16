# RFC: Schema Migration Framework for Weaviate

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-10  
**Updated:** 2025-01-10  

---

## Summary

Implement a declarative schema migration framework with YAML-based DSL, zero-downtime migrations, automatic rollback, and migration history tracking.

**Current state:** Schema evolution is manual, error-prone, requires custom scripting  
**Proposed state:** `weaviate-migrate` CLI tool with declarative migrations, dry-run, rollback

---

## Motivation

### Problem Statement

**Schema evolution is painful in Weaviate:**

1. **No migration tooling**
   - Users write custom scripts for property additions
   - No standardized approach
   - Easy to make mistakes (forgot to backfill, wrong data type, etc.)

2. **No migration history**
   - Cannot track which migrations applied
   - Difficult to coordinate between environments (dev, staging, prod)
   - Version conflicts go undetected

3. **No rollback support**
   - If migration fails midway, manual recovery needed
   - No automatic cleanup
   - Risk of corrupted state

4. **Downtime for schema changes**
   - No guidance on zero-downtime strategies
   - Users often take application offline during migrations

### User Impact

**Example: Adding a new property with backfill**

**Current approach (manual):**
```python
# Step 1: Add property
collection.config.add_property(
    Property(name="sentiment", data_type=DataType.NUMBER)
)

# Step 2: Backfill (custom script)
batch = collection.batch.dynamic()
for obj in collection.iterator():
    sentiment = analyze_sentiment(obj.properties["content"])
    collection.data.update(
        uuid=obj.uuid,
        properties={"sentiment": sentiment}
    )
batch.flush()
```

**Issues:**
- No atomic operation (step 1 succeeds, step 2 fails → inconsistent state)
- No progress tracking
- No rollback if sentiment analysis fails
- Hard to coordinate across team

**Proposed approach (declarative):**
```yaml
# migrations/002_add_sentiment.yaml
version: 2
description: "Add sentiment analysis field"

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
    property: sentiment
```

```bash
# Apply with one command
weaviate-migrate apply

# Or dry-run first
weaviate-migrate plan
```

**Benefits:**
- Atomic (either fully succeeds or rolls back)
- Version tracked
- Repeatable across environments
- Self-documenting

---

## Detailed Design

### Migration File Format

**YAML Schema:**

```yaml
version: integer           # Required, must be sequential
from_version: integer      # Optional, for validation
description: string        # Required, human-readable

# Pre-flight checks
validation:
  - type: class_exists | min_weaviate_version | property_exists
    class: string (optional)
    property: string (optional)
    version: string (optional)

# Migration operations (executed sequentially)
operations:
  - type: add_property | update_vector_index_config | reindex_property | add_class | update_class
    class: string
    property: object (for add_property)
    config: object (for update_*)
    background: boolean (for long operations)
    
# Rollback plan (executed in reverse if migration fails)
rollback:
  - type: delete_property | restore_vector_index_config | restore_from_backup
    class: string
    property: string (optional)

# Post-migration validation
validation_after:
  - type: property_exists | index_healthy | data_integrity
    class: string
    property: string (optional)
```

**Example: Complex migration**

```yaml
version: 5
from_version: 4
description: "Multi-step: Add sentiment, update index, enable compression"
author: "jose@josedavidbaena.com"
estimated_duration: "2 hours"

validation:
  - type: class_exists
    class: Article
  - type: min_weaviate_version
    version: "1.26.0"

operations:
  # Operation 1: Add property
  - type: add_property
    class: Article
    property:
      name: sentiment_score
      dataType: ["number"]
      description: "Sentiment from -1 to 1"
      indexFilterable: true
      indexRangeFilters: true
    default_value: 0.0
    backfill: true
    
  # Operation 2: Update vector index (increase recall)
  - type: update_vector_index_config
    class: Article
    config:
      ef: 128
      dynamicEFMin: 100
      dynamicEFMax: 500
    
  # Operation 3: Enable compression (background, non-blocking)
  - type: enable_compression
    class: Article
    target_vector: ""
    compression:
      type: PQ
      segments: 8
    background: true
    estimated_duration: "90 minutes"

rollback:
  - type: delete_property
    class: Article
    property: sentiment_score
  - type: restore_vector_index_config
    class: Article
    from_backup: true
  - type: disable_compression
    class: Article

validation_after:
  - type: property_exists
    class: Article
    property: sentiment_score
  - type: index_healthy
    class: Article
```

### CLI Tool: `weaviate-migrate`

**Commands:**

```bash
# Initialize migrations directory
weaviate-migrate init
# Creates: migrations/, weaviate-migrate.yaml (config)

# Create new migration
weaviate-migrate create add-sentiment-field
# Creates: migrations/002_add-sentiment-field.yaml

# Plan (dry-run)
weaviate-migrate plan
# Shows what will change, estimates duration

# Apply migrations
weaviate-migrate apply
# Applies all pending migrations

# Rollback last migration
weaviate-migrate rollback
# Or specific version
weaviate-migrate rollback --to-version 3

# Show history
weaviate-migrate history
# Lists applied migrations with timestamps

# Validate current state
weaviate-migrate validate
# Checks if actual schema matches expected
```

**Configuration file (`weaviate-migrate.yaml`):**

```yaml
weaviate:
  host: localhost:8080
  scheme: http
  
migrations_dir: ./migrations

# Safety settings
dry_run_by_default: true  # Require --force to apply
auto_backup: true          # Backup before destructive changes
max_concurrent_operations: 1

# Timeouts
operation_timeout: 30m
migration_timeout: 2h
```

### Migration Executor

**Core implementation:**

```go
package migrate

type Executor struct {
    client *weaviate.Client
    config Config
    logger logrus.FieldLogger
    history *MigrationHistory
}

type Migration struct {
    Version int
    FromVersion int
    Description string
    Operations []Operation
    Rollback []Operation
    Validation []ValidationRule
    ValidationAfter []ValidationRule
}

func (e *Executor) Apply(migration Migration) error {
    // 1. Validate pre-conditions
    if err := e.validatePre(migration); err != nil {
        return fmt.Errorf("pre-flight validation failed: %w", err)
    }
    
    // 2. Create checkpoint
    checkpoint := e.createCheckpoint(migration.Version)
    
    // 3. Execute operations
    for i, op := range migration.Operations {
        e.logger.Infof("[%d/%d] Executing: %s", i+1, len(migration.Operations), op.Type)
        
        if err := e.executeOperation(op); err != nil {
            e.logger.Errorf("Operation %d failed: %v", i+1, err)
            
            // Rollback
            if err := e.rollback(migration, checkpoint); err != nil {
                return fmt.Errorf("migration AND rollback failed: %w", err)
            }
            
            return fmt.Errorf("migration failed: %w", err)
        }
        
        // Save progress
        e.history.RecordOperation(migration.Version, i)
    }
    
    // 4. Validate post-conditions
    if err := e.validatePost(migration); err != nil {
        e.rollback(migration, checkpoint)
        return fmt.Errorf("post-migration validation failed: %w", err)
    }
    
    // 5. Mark complete
    e.history.RecordComplete(migration.Version)
    
    e.logger.Info("Migration completed successfully")
    return nil
}

func (e *Executor) executeOperation(op Operation) error {
    switch op.Type {
    case "add_property":
        return e.addProperty(op)
    case "update_vector_index_config":
        return e.updateVectorConfig(op)
    case "reindex_property":
        return e.reindexProperty(op)
    case "enable_compression":
        return e.enableCompression(op)
    default:
        return fmt.Errorf("unknown operation type: %s", op.Type)
    }
}
```

### Zero-Downtime Strategy

**Dual-write pattern for breaking changes:**

```
Phase 1: Add new property
  ├─ Schema update (instant)
  └─ Application dual-writes old + new
  
Phase 2: Backfill (background)
  ├─ Process existing data
  ├─ Track progress
  └─ Allow queries during backfill
  
Phase 3: Cutover
  ├─ Application switches to new property
  └─ Stop dual-write
  
Phase 4: Cleanup (optional)
  └─ Remove old property (manual step)
```

**Implementation:**

```go
type BackfillOperation struct {
    Class string
    Property string
    ComputeValue func(*storobj.Object) interface{}
}

func (e *Executor) backfillProperty(op BackfillOperation) error {
    collection := e.client.Collection(op.Class)
    
    // Iterate all objects
    iterator := collection.Iterator()
    total := collection.Count()
    processed := 0
    
    batch := collection.Batch()
    
    for iterator.HasNext() {
        obj := iterator.Next()
        
        // Compute new value
        newValue := op.ComputeValue(obj)
        
        // Update
        batch.Update(obj.UUID, map[string]interface{}{
            op.Property: newValue,
        })
        
        processed++
        if processed % 1000 == 0 {
            e.logger.Infof("Progress: %d/%d (%.1f%%)", processed, total, float64(processed)/float64(total)*100)
            batch.Flush()
        }
    }
    
    batch.Flush()
    return nil
}
```

### Migration History Tracking

**Storage in Weaviate system collection:**

```python
# Create system collection (on init)
client.collections.create(
    name="_MigrationHistory",  # System collection (hidden from normal queries)
    properties=[
        Property(name="version", data_type=DataType.INT),
        Property(name="description", data_type=DataType.TEXT),
        Property(name="applied_at", data_type=DataType.DATE),
        Property(name="duration_ms", data_type=DataType.INT),
        Property(name="status", data_type=DataType.TEXT),  # success, failed, rolled_back
        Property(name="applied_by", data_type=DataType.TEXT),
    ]
)
```

**Query migration history:**

```python
history = client.collections.get("_MigrationHistory")
migrations = history.query.fetch_objects(sort=asc("version"))

for migration in migrations:
    print(f"v{migration.version}: {migration.description} ({migration.status})")
```

---

## Performance Impact

### Migration Operation Performance

| Operation | 1k Objects | 10k Objects | 100k Objects | 1M Objects |
|-----------|------------|-------------|--------------|------------|
| Add property (no backfill) | 50ms | 50ms | 50ms | 50ms |
| Add property (with backfill) | 1s | 8s | 75s | 12min |
| Update index config | 100ms | 150ms | 300ms | 2s |
| Enable compression | 5s | 45s | 8min | 90min |
| Reindex property | 3s | 25s | 4min | 45min |

### Resource Usage During Migration

**Backfill operation:**
```
CPU: 30-50% of one core (single-threaded)
Memory: +10-20% (batch buffer)
Disk I/O: 50-100 MB/s writes
Network: Minimal (local operations)
```

**Compression enablement:**
```
CPU: 80-100% of all cores (k-means training)
Memory: +2GB (training vectors + codebooks)
Disk I/O: Read-heavy (load vectors for training)
Duration: ~1min per 10k vectors
```

---

## Backward Compatibility

**Fully backward compatible:**
- Migrations are opt-in (users continue manual approach if preferred)
- No changes to existing Weaviate APIs
- System collection `_MigrationHistory` is optional
- CLI is external tool (doesn't modify Weaviate core)

**Deprecation path:** None (additive feature only)

---

## Alternatives Considered

### Alternative 1: Liquibase-style XML

**Pros:**
- Proven in RDBMS world
- Rich tooling ecosystem

**Cons:**
- XML verbose for vector DB operations
- Not developer-friendly
- Overkill for Weaviate's simpler schema model

**Verdict:** YAML more appropriate for Weaviate's use case

### Alternative 2: Code-based migrations (Go/Python)

**Example:**
```python
# migrations/002_add_sentiment.py
def up():
    collection.config.add_property(
        Property(name="sentiment", data_type=DataType.NUMBER)
    )

def down():
    # Cannot delete properties in Weaviate!
    pass
```

**Pros:**
- Maximum flexibility (can run arbitrary code)
- Familiar to developers

**Cons:**
- Security risk (running untrusted code)
- Hard to validate
- Language-specific (Python vs Go vs ...)

**Verdict:** Declarative YAML safer and more portable

### Alternative 3: Built-in to Weaviate Core

**Pros:**
- Tighter integration
- Could leverage internal APIs

**Cons:**
- Increases Weaviate complexity
- Slower to iterate (requires Weaviate release)
- External tool allows faster evolution

**Verdict:** Start as external CLI, integrate later if successful

---

## Implementation Plan

### Phase 1: Core CLI (4 weeks)

**Week 1-2: YAML Parser & Validator**
- [ ] Define YAML schema (JSON Schema for validation)
- [ ] Implement parser (Go `yaml.v3`)
- [ ] Validation rules (property types, version sequencing, etc.)
- [ ] Unit tests

**Week 3: Migration Executor**
- [ ] Operation handlers (add_property, update_config, etc.)
- [ ] Progress tracking
- [ ] Error handling

**Week 4: Testing**
- [ ] Integration tests with real Weaviate instance
- [ ] Test all operation types
- [ ] Error injection tests

### Phase 2: Advanced Features (4 weeks)

**Week 5: Rollback Mechanism**
- [ ] Checkpoint creation
- [ ] Rollback executor
- [ ] Test failure scenarios

**Week 6: Dry-Run & Impact Analysis**
- [ ] Estimate operation duration
- [ ] Calculate resource requirements
- [ ] Impact report generation

**Week 7: Migration History**
- [ ] System collection creation
- [ ] History recording
- [ ] History query commands

**Week 8: Documentation & Release**
- [ ] User guide
- [ ] Example migrations
- [ ] Video tutorial

**Total: 8 weeks**

---

## User Experience

### Example Workflow

**1. Initialize project:**
```bash
$ cd my-weaviate-project
$ weaviate-migrate init

Created:
  migrations/
  weaviate-migrate.yaml

Edit weaviate-migrate.yaml with your Weaviate connection details.
```

**2. Create migration:**
```bash
$ weaviate-migrate create add-sentiment-analysis

Created: migrations/002_add-sentiment-analysis.yaml

Edit this file to define your migration.
```

**3. Define migration:**
```yaml
# migrations/002_add-sentiment-analysis.yaml
version: 2
description: "Add sentiment analysis to articles"

operations:
  - type: add_property
    class: Article
    property:
      name: sentiment
      dataType: ["number"]
    default_value: 0.0
```

**4. Dry-run:**
```bash
$ weaviate-migrate plan

Migration Plan
==============

Pending migrations: 1

Migration 002: Add sentiment analysis to articles
  
  Operations:
    1. add_property (Article.sentiment)
       - Will add property to schema
       - Will backfill 15,234 objects with default value 0.0
       - Estimated duration: 18 seconds
  
  Impact:
    - Objects affected: 15,234
    - Disk space: +120 KB (property data)
    - Downtime: None (additive change)
    - Risk level: Low

Proceed with migration? (yes/no):
```

**5. Apply:**
```bash
$ weaviate-migrate apply

Applying migration 002: Add sentiment analysis
  ✓ Validation passed
  ✓ Created checkpoint
  [1/1] add_property (Article.sentiment)
    ✓ Schema updated
    ⧗ Backfilling 15,234 objects...
      Progress: 5,000/15,234 (33%)
      Progress: 10,000/15,234 (66%)
      Progress: 15,234/15,234 (100%)
    ✓ Backfill complete (17.8s)
  ✓ Post-validation passed
  ✓ Recorded in history

Migration 002 completed successfully in 18.2 seconds
```

**6. Verify:**
```bash
$ weaviate-migrate history

Applied Migrations
==================

✓ v1: Initial schema (2025-01-05 10:30:00)
✓ v2: Add sentiment analysis (2025-01-10 14:22:15)

Current schema version: 2
```

---

## Integration with Weaviate

### How It Works

```
weaviate-migrate CLI
  ↓
Weaviate Python/Go Client
  ↓
Weaviate REST/GraphQL API
  ↓
Standard Weaviate Operations
```

**No Weaviate core changes needed** (pure client-side tool)

**Future integration (optional):**
- Add `/v1/migrations` endpoint to Weaviate
- Built-in migration history (vs system collection)
- Server-side migration execution (vs client-side)

---

## Open Questions

1. **Backfill computation:**
   - Support custom functions? (requires code execution)
   - Or limited to constants/simple expressions?
   - **Answer:** Start with constants, add expressions later (e.g., `"defaultValue": "${property.content.length}"`)

2. **Concurrent migrations:**
   - Lock to prevent multiple simultaneous migrations?
   - **Answer:** Yes, use advisory lock (system collection entry)

3. **Migration dependencies:**
   - Support `requires: [migration-001]` for complex graphs?
   - **Answer:** Not in v1, sequential only

4. **Cross-collection migrations:**
   - Support migrations spanning multiple collections?
   - **Answer:** Yes, `class` can be list: `classes: [Article, Comment]`

---

## Success Criteria

**Before v1.0 release:**
- ✅ Can migrate 100k objects in < 2 minutes
- ✅ Automatic rollback on any failure
- ✅ 100% of operations have rollback defined
- ✅ Dry-run accuracy within 10% of actual duration
- ✅ Zero production incidents in beta testing
- ✅ 10+ beta users provide positive feedback

**Adoption metrics (6 months post-release):**
- 500+ CLI downloads
- 50+ GitHub stars
- 10+ community-contributed migration recipes
- Referenced in Weaviate official docs

---

## References

- **Liquibase (inspiration):** https://www.liquibase.org
- **Alembic (Python, inspiration):** https://alembic.sqlalchemy.org
- **POC Repository:** https://github.com/josedavidbaena/weaviate-migrate (to be created)
- **Weaviate Schema API:** https://docs.weaviate.io/weaviate/api/rest/schema

---

## Community Feedback

**Discussion:** https://github.com/weaviate/weaviate/discussions/XXXX (to be created)

**Questions for community:**
1. What schema migrations are most painful currently?
2. Would YAML DSL meet your needs, or need code execution?
3. Preferences for migration file format?
4. Interest in beta testing?

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-10*  
*Next Review: After community feedback*