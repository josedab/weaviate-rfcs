# RFC 0011: Automated Schema Evolution and Versioning

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-16  
**Updated:** 2025-01-16  

---

## Summary

Implement automated schema evolution with versioning, compatibility checking, and zero-downtime migrations to enable safe, continuous schema updates in production environments.

**Current state:** Manual schema changes with downtime risk  
**Proposed state:** Automated, versioned schema evolution with rollback capability

---

## Motivation

### Current Limitations

1. **No schema versioning:**
   - Schema changes are immediate and irreversible
   - No history tracking
   - Difficult to coordinate across environments

2. **Downtime required:**
   - Schema updates require service restart
   - Data migration blocks queries
   - Risk of data inconsistency

3. **Limited validation:**
   - No compatibility checking
   - Breaking changes not detected
   - No automated testing of schema changes

### Impact on Users

**Development teams:**
- Cannot iterate quickly on schema
- Fear of breaking production
- Manual coordination required

**Operations teams:**
- Scheduled maintenance windows
- Complex rollback procedures
- Data migration risks

**Business impact:**
- Downtime costs: $1000+/hour
- Delayed feature releases
- Reduced development velocity

---

## Detailed Design

### Schema Versioning System

```go
// Schema version metadata
type SchemaVersion struct {
    ID          uint64
    Timestamp   time.Time
    Author      string
    Description string
    Hash        string
    
    // Changes in this version
    Changes     []SchemaChange
    
    // Compatibility status
    Compatibility CompatibilityLevel
    
    // Migration status
    MigrationStatus MigrationStatus
}

type CompatibilityLevel string

const (
    BackwardCompatible  CompatibilityLevel = "backward"
    ForwardCompatible   CompatibilityLevel = "forward"
    FullyCompatible     CompatibilityLevel = "full"
    Breaking            CompatibilityLevel = "breaking"
)

type SchemaChange struct {
    Type        ChangeType
    Class       string
    Property    string
    Before      interface{}
    After       interface{}
    Migration   *MigrationPlan
}
```

### Schema Registry

```go
type SchemaRegistry struct {
    store       *VersionStore
    validator   *CompatibilityValidator
    migrator    *SchemaMigrator
    
    // Active schema version
    current     *SchemaVersion
    
    // Version history
    history     []SchemaVersion
}

func (r *SchemaRegistry) RegisterSchema(schema *Schema, author string) error {
    // Generate version
    version := &SchemaVersion{
        ID:          r.nextVersion(),
        Timestamp:   time.Now(),
        Author:      author,
        Hash:        r.hashSchema(schema),
        Changes:     r.detectChanges(r.current, schema),
    }
    
    // Validate compatibility
    compat, err := r.validator.Check(r.current, schema)
    if err != nil {
        return err
    }
    version.Compatibility = compat
    
    // Generate migration plan
    if len(version.Changes) > 0 {
        plan, err := r.migrator.Plan(version.Changes)
        if err != nil {
            return err
        }
        for i, change := range version.Changes {
            version.Changes[i].Migration = plan[i]
        }
    }
    
    // Store version
    if err := r.store.Save(version); err != nil {
        return err
    }
    
    return nil
}
```

### Compatibility Validation

```go
type CompatibilityValidator struct {
    rules []CompatibilityRule
}

type CompatibilityRule interface {
    Check(old, new *Schema) (CompatibilityLevel, error)
}

// Example: Adding property is backward compatible
type AddPropertyRule struct{}

func (r *AddPropertyRule) Check(old, new *Schema) (CompatibilityLevel, error) {
    for className, newClass := range new.Classes {
        oldClass, exists := old.Classes[className]
        if !exists {
            continue
        }
        
        for propName, newProp := range newClass.Properties {
            _, exists := oldClass.Properties[propName]
            if !exists {
                // New property added
                if newProp.Required {
                    return Breaking, fmt.Errorf(
                        "adding required property %s.%s is breaking",
                        className, propName,
                    )
                }
                return BackwardCompatible, nil
            }
        }
    }
    
    return FullyCompatible, nil
}

// Example: Removing property is breaking
type RemovePropertyRule struct{}

func (r *RemovePropertyRule) Check(old, new *Schema) (CompatibilityLevel, error) {
    for className, oldClass := range old.Classes {
        newClass, exists := new.Classes[className]
        if !exists {
            return Breaking, fmt.Errorf("removing class %s is breaking", className)
        }
        
        for propName := range oldClass.Properties {
            _, exists := newClass.Properties[propName]
            if !exists {
                return Breaking, fmt.Errorf(
                    "removing property %s.%s is breaking",
                    className, propName,
                )
            }
        }
    }
    
    return FullyCompatible, nil
}
```

### Zero-Downtime Migration

```go
type SchemaMigrator struct {
    executor *MigrationExecutor
    planner  *MigrationPlanner
}

type MigrationPlan struct {
    Steps    []MigrationStep
    Estimate time.Duration
    Impact   ImpactAnalysis
}

type MigrationStep struct {
    Type        StepType
    Query       string
    Reversible  bool
    Blocking    bool
    Estimate    time.Duration
}

// Blue-green migration for property addition
func (m *SchemaMigrator) AddProperty(class, property string, def *PropertyDef) error {
    // Phase 1: Add property to schema (non-blocking)
    if err := m.executor.AddColumn(class, property, def); err != nil {
        return err
    }
    
    // Phase 2: Backfill data (background)
    backfill := m.planner.CreateBackfillPlan(class, property, def)
    if err := m.executor.ExecuteAsync(backfill); err != nil {
        return err
    }
    
    // Phase 3: Wait for backfill completion
    if err := m.executor.WaitForCompletion(backfill.ID); err != nil {
        return err
    }
    
    // Phase 4: Enable property for queries
    if err := m.executor.EnableProperty(class, property); err != nil {
        return err
    }
    
    return nil
}

// Shadow migration for breaking changes
func (m *SchemaMigrator) ChangePropertyType(
    class, property string,
    oldType, newType DataType,
) error {
    shadowProp := property + "_v2"
    
    // Phase 1: Add shadow property
    if err := m.executor.AddColumn(class, shadowProp, &PropertyDef{
        Type: newType,
    }); err != nil {
        return err
    }
    
    // Phase 2: Dual writes (old + new)
    if err := m.executor.EnableDualWrite(class, property, shadowProp); err != nil {
        return err
    }
    
    // Phase 3: Backfill shadow property
    backfill := m.planner.CreateConversionBackfill(class, property, shadowProp)
    if err := m.executor.ExecuteAsync(backfill); err != nil {
        return err
    }
    
    // Phase 4: Wait for backfill + validation
    if err := m.executor.WaitAndValidate(backfill.ID); err != nil {
        return err
    }
    
    // Phase 5: Switch reads to shadow property
    if err := m.executor.SwitchReads(class, property, shadowProp); err != nil {
        return err
    }
    
    // Phase 6: Drop old property, rename shadow
    if err := m.executor.RenameProperty(class, shadowProp, property); err != nil {
        return err
    }
    
    return nil
}
```

### Schema Diff and Merge

```go
type SchemaDiff struct {
    Added    []SchemaElement
    Removed  []SchemaElement
    Modified []SchemaModification
}

type SchemaElement struct {
    Type  ElementType
    Path  string
    Value interface{}
}

func (r *SchemaRegistry) Diff(v1, v2 uint64) (*SchemaDiff, error) {
    schema1, err := r.GetVersion(v1)
    if err != nil {
        return nil, err
    }
    
    schema2, err := r.GetVersion(v2)
    if err != nil {
        return nil, err
    }
    
    diff := &SchemaDiff{}
    
    // Compare classes
    for name, class2 := range schema2.Classes {
        class1, exists := schema1.Classes[name]
        if !exists {
            diff.Added = append(diff.Added, SchemaElement{
                Type:  ElementClass,
                Path:  name,
                Value: class2,
            })
            continue
        }
        
        // Compare properties
        for propName, prop2 := range class2.Properties {
            prop1, exists := class1.Properties[propName]
            if !exists {
                diff.Added = append(diff.Added, SchemaElement{
                    Type:  ElementProperty,
                    Path:  fmt.Sprintf("%s.%s", name, propName),
                    Value: prop2,
                })
            } else if !reflect.DeepEqual(prop1, prop2) {
                diff.Modified = append(diff.Modified, SchemaModification{
                    Path:   fmt.Sprintf("%s.%s", name, propName),
                    Before: prop1,
                    After:  prop2,
                })
            }
        }
    }
    
    return diff, nil
}

// Three-way merge for concurrent schema changes
func (r *SchemaRegistry) Merge(base, v1, v2 uint64) (*Schema, error) {
    schemaBase, _ := r.GetVersion(base)
    schema1, _ := r.GetVersion(v1)
    schema2, _ := r.GetVersion(v2)
    
    diff1, _ := r.Diff(base, v1)
    diff2, _ := r.Diff(base, v2)
    
    // Detect conflicts
    conflicts := r.detectConflicts(diff1, diff2)
    if len(conflicts) > 0 {
        return nil, ErrMergeConflict{Conflicts: conflicts}
    }
    
    // Merge changes
    merged := schemaBase.Clone()
    r.applyDiff(merged, diff1)
    r.applyDiff(merged, diff2)
    
    return merged, nil
}
```

### CLI Tool

```bash
# Schema version management
$ weaviate schema version list
VERSION  TIMESTAMP            AUTHOR       COMPATIBILITY  STATUS
1        2025-01-15 10:00:00  alice        full           deployed
2        2025-01-15 14:30:00  bob          backward       deployed
3        2025-01-16 09:15:00  alice        breaking       pending

# Show diff between versions
$ weaviate schema diff 2 3
+ Article.tags (string[])
- Article.category (string)
~ Article.publishedAt (datetime → timestamp)

# Validate compatibility
$ weaviate schema validate schema.yaml
✓ Backward compatible with version 3
⚠ Breaking change: removing Article.category
  Migration required: 1.2M objects to update
  Estimated time: 15 minutes

# Apply schema with automatic migration
$ weaviate schema apply schema.yaml --auto-migrate
Planning migration...
  ✓ Add Article.tags (non-blocking)
  ✓ Backfill Article.tags (background, 15min)
  ⚠ Remove Article.category (requires confirmation)
  
Proceed? [y/N]: y

Applying migration:
  [████████████████████████] 100% (1.2M/1.2M objects)
  
✓ Migration completed in 14m 32s
✓ Schema version 4 deployed

# Rollback to previous version
$ weaviate schema rollback 3
Rolling back to version 3...
✓ Rolled back successfully
```

---

## Configuration

```yaml
# weaviate.conf.yaml
schema:
  versioning:
    enabled: true
    
    # Compatibility enforcement
    compatibility:
      level: "backward"  # backward | forward | full | none
      enforceOnWrite: true
      
    # Migration settings
    migration:
      maxDuration: "1h"
      batchSize: 1000
      parallelism: 4
      
      # Zero-downtime options
      enableDualWrite: true
      shadowPropertySuffix: "_v2"
      
    # History retention
    history:
      maxVersions: 100
      retentionDays: 365
      
    # Registry storage
    registry:
      backend: "etcd"  # etcd | consul | database
      endpoint: "localhost:2379"
```

---

## Performance Impact

### Migration Performance

| Operation | Objects | Duration | Downtime |
|-----------|---------|----------|----------|
| Add property | 10M | 12 min | 0s (background) |
| Add required property | 10M | 18 min | 0s (with default) |
| Change property type | 10M | 25 min | 0s (shadow mode) |
| Remove property | 10M | 8 min | 0s (lazy deletion) |
| Add index | 10M | 35 min | 0s (background) |

### Overhead

- **Version tracking:** ~1KB per version
- **Compatibility check:** <100ms per schema change
- **Diff computation:** <50ms for typical schemas

---

## Implementation Plan

### Phase 1: Versioning Core (4 weeks)
- [ ] Schema version data model
- [ ] Version registry implementation
- [ ] Diff and merge algorithms
- [ ] Unit tests

### Phase 2: Compatibility (3 weeks)
- [ ] Compatibility rules engine
- [ ] Validation framework
- [ ] Breaking change detection
- [ ] Integration tests

### Phase 3: Migration (4 weeks)
- [ ] Migration planner
- [ ] Background migration executor
- [ ] Dual-write coordination
- [ ] Shadow migration support

### Phase 4: CLI & Tooling (2 weeks)
- [ ] CLI commands
- [ ] Schema validation
- [ ] Migration dry-run
- [ ] Documentation

**Total: 13 weeks** (revised from 10 weeks)

---

## Success Criteria

- ✅ Zero-downtime for 95% of schema changes
- ✅ <1% migration failure rate
- ✅ Automatic rollback on errors
- ✅ <5 minute rollback time
- ✅ 100% compatibility detection accuracy

---

## Alternatives Considered

### Alternative 1: Manual Versioning
**Verdict:** Too error-prone for production

### Alternative 2: External Schema Registry (Confluent)
**Verdict:** Over-engineered for embedded use case

### Alternative 3: Database-style ALTER TABLE
**Verdict:** Doesn't support complex migrations

---

## References

- Liquibase: https://www.liquibase.org/
- Flyway: https://flywaydb.org/
- Confluent Schema Registry: https://docs.confluent.io/platform/current/schema-registry/
- Expand/Contract Pattern: https://www.martinfowler.com/bliki/ParallelChange.html

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-16*