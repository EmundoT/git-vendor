---
description: Plan and verify vendor.lock schema migrations with backward compatibility analysis, drift classification, and migration prompt generation
allowed-tools: Bash, Read, Glob, Grep, Write, Edit, Task
---

# Migration Planner Workflow

**Role:** You are a lockfile schema migration planner working in a concurrent multi-agent Git environment. Your goal is to plan and verify vendor.lock schema changes, ensure backward compatibility, generate migration prompts for other Claude instances, and verify migrations are safe through iterative review cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your migration branch (already created for you)

**Key Principle:** Lockfile migrations must maintain backward compatibility. Old lockfiles must work with new CLI versions. Breaking changes should never ship.

## Phase 1: Sync & Schema Analysis

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Review Current Schema:**
  ```bash
  # Check current lockfile structure
  cat vendor/vendor.lock

  # Review type definitions
  grep -A 30 "type VendorLock" internal/types/types.go
  grep -A 20 "type LockDetails" internal/types/types.go

  # Check lock_store implementation
  cat internal/core/lock_store.go
  ```

- **Check Roadmap Requirements:**
  Review `docs/ROADMAP.md` for planned schema changes:
  - Feature 1.1: Schema Versioning
  - Feature 1.3: Metadata Enrichment
  - Phase 2 features requiring new lockfile fields

- **Identify Proposed Changes:**
  For each planned schema modification:
  - New fields to add
  - Field type changes
  - Field deprecations
  - Structural reorganizations

## Phase 2: Drift Classification

Categorize proposed schema changes:

| Type | Risk | Examples |
|------|------|----------|
| **Additive** | LOW | New optional field, new section |
| **Default Change** | LOW | New field with default value |
| **Structural** | MEDIUM | Nesting changes, field renames with migration |
| **Type Change** | HIGH | int to string, nested to flat |
| **Removal** | HIGH | Removing a field (use deprecation instead) |

### Change Impact Analysis

For each change, assess:

| Factor | Question |
|--------|----------|
| **Old Lockfiles** | Will old lockfiles parse correctly with new CLI? |
| **New Lockfiles** | Will new lockfiles work with old CLI? |
| **Data Loss** | Will any data be lost during migration? |
| **User Action** | Does user need to do anything? |

## Phase 3: Migration Planning

### Migration Strategy Options

| Strategy | When to Use | Example |
|----------|-------------|---------|
| **Additive Only** | New optional fields | Add `license_spdx` to LockDetails |
| **Default Fill** | Required fields with safe defaults | Add `schema_version: "1.0"` |
| **On-Read Migration** | Transform old format while reading | Parse old format, output new format |
| **Explicit Command** | Large structural changes | `git vendor migrate-lockfile` |

### Schema Version Guidelines

Per ROADMAP.md Feature 1.1:
- Add `schema_version: "1.0"` as first field
- Missing version = treat as `v1.0`
- Higher minor version than CLI understands = warn but proceed
- Higher major version = error with upgrade instructions

### Migration Implementation Template

For new field additions:

```go
// In types.go - Add optional field with yaml omitempty
type LockDetails struct {
    Name        string    `yaml:"name"`
    Ref         string    `yaml:"ref"`
    CommitHash  string    `yaml:"commit"`
    // New field - omitempty for backward compatibility
    LicenseSPDX string    `yaml:"license_spdx,omitempty"`
}
```

For on-read migration:

```go
// In lock_store.go - Handle missing fields gracefully
func (s *YAMLLockStore) Read() (*VendorLock, error) {
    // Parse YAML
    var lock VendorLock
    if err := yaml.Unmarshal(data, &lock); err != nil {
        return nil, err
    }

    // Apply migrations
    if lock.SchemaVersion == "" {
        lock.SchemaVersion = "1.0"
    }

    return &lock, nil
}
```

### Rollback Considerations

For each migration, document:
- Can the change be reverted?
- What data would be lost on rollback?
- Is the rollback automatic or requires user action?

## Phase 4: Migration Prompt Generation

### Prompt Template

```text
TASK: Implement lockfile schema change for [Feature ID]

SCHEMA CHANGE:
- Type: [Additive/Default Fill/On-Read/Explicit]
- Risk Level: [LOW/MEDIUM/HIGH]
- Backward Compatible: [Yes/No]

CURRENT SCHEMA:
[Show current type definition]

PROPOSED SCHEMA:
[Show updated type definition]

MIGRATION STRATEGY:
1. [Step-by-step migration approach]
2. [How old lockfiles will be handled]
3. [How new lockfiles will be written]

FILES TO MODIFY:
1. internal/types/types.go - Add new field to [struct]
2. internal/core/lock_store.go - Handle migration on read
3. internal/core/vendor_syncer.go - Populate new field on sync
4. [Additional files]

TESTS TO ADD:
1. Test parsing old lockfile format
2. Test parsing new lockfile format
3. Test migration from old to new
4. Test round-trip (read/write/read)

DOCUMENTATION:
1. Update CLAUDE.md with new lockfile fields
2. Create/update docs/LOCKFILE_SCHEMA.md

VERIFICATION:
- [ ] Old lockfiles parse without error
- [ ] New lockfiles include new fields
- [ ] `go test ./...` passes
- [ ] No breaking changes to existing behavior
- [ ] Schema version updated if required

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

## Phase 5: Migration Report

```markdown
## Lockfile Schema Migration Report

### Schema Change Summary

| Field | Change Type | Risk | Backward Compatible |
|-------|-------------|------|---------------------|
| schema_version | NEW | LOW | Yes |
| license_spdx | NEW | LOW | Yes |

### Dependency Analysis

Check `docs/ROADMAP.md` Section 12 (Dependency Map):
- Which features depend on this schema change?
- What must be completed first?

### Migration Testing Matrix

| Scenario | Test |
|----------|------|
| Old lockfile -> New CLI | Parse succeeds, migration applied |
| New lockfile -> Old CLI | Parse succeeds (unknown fields ignored) |
| Round-trip | Read -> Write -> Read preserves data |
| Missing optional fields | Defaults applied correctly |

### Prompts Generated

---

## PROMPT 1: Add schema_version field
[Full prompt]

---
```

## Phase 6: Verification Cycle

After migration prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Verify Schema Changes:**
  ```bash
  # Check type definitions updated
  grep -A 30 "type VendorLock" internal/types/types.go

  # Check lock_store handles migration
  grep -B5 -A10 "SchemaVersion" internal/core/lock_store.go
  ```

- **Test Migration:**
  ```bash
  # Run all tests
  go test ./...

  # Test with a sample old lockfile (if available)
  # Create test fixture in testdata/old-lockfile.yml
  ```

- **Verify Backward Compatibility:**
  ```bash
  # Create a lockfile with old format
  # Run new CLI against it
  # Verify it parses and migrates correctly
  ```

- **Grade Each Migration:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Schema updated, tests pass, backward compatible | Complete |
  | **NEEDS-TESTING** | Code done but tests incomplete | Add test follow-up |
  | **INCOMPLETE** | Missing migration handling | Generate follow-up |
  | **BREAKING** | Old lockfiles don't parse | Urgent fix, revert |

## Phase 7: Migration Approval

When migrations are verified:

- **Final Checklist:**
  - [ ] Schema version updated
  - [ ] All new fields documented
  - [ ] Old lockfiles still parse
  - [ ] Tests cover migration paths
  - [ ] CLAUDE.md updated
  - [ ] LOCKFILE_SCHEMA.md created/updated

- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```

- **Migration Report:**

  ```markdown
  ## Lockfile Schema Migration Complete

  ### Schema Version
  | Before | After |
  |--------|-------|
  | 1.0 | 1.1 |

  ### Changes Applied
  | Field | Type | Default |
  |-------|------|---------|
  | schema_version | string | "1.0" |
  | license_spdx | string | "" (omitempty) |

  ### Compatibility Verified
  - Old lockfiles: Parse correctly
  - New lockfiles: Backward compatible
  - Round-trip: Data preserved

  ### Documentation Updated
  - [x] CLAUDE.md
  - [x] LOCKFILE_SCHEMA.md
  - [x] Test coverage for migrations
  ```

---

## Schema Migration Quick Reference

### Check Current Schema

```bash
# View lockfile structure
cat vendor/vendor.lock

# View type definitions
grep -A 50 "type VendorLock\|type LockDetails\|type VendorConfig\|type VendorSpec" internal/types/types.go
```

### Common Migration Patterns

**Add Optional Field (Safe):**
```go
// Add with omitempty for backward compat
NewField string `yaml:"new_field,omitempty"`
```

**Add Required Field with Default:**
```go
// Handle in lock_store.go on read
if lock.SchemaVersion == "" {
    lock.SchemaVersion = "1.0"
}
```

**Deprecate Field:**
```go
// Keep field but mark deprecated in docs
// Never remove fields - just stop writing them
OldField string `yaml:"old_field,omitempty"` // Deprecated: use NewField
```

### Test Fixtures

Create test lockfiles in `testdata/`:
```text
testdata/
├── lockfile-v1.0.yml     # Minimal v1.0 lockfile
├── lockfile-v1.1.yml     # With new fields
└── lockfile-legacy.yml   # Pre-versioning format
```

---

## Integration Points

- **internal/types/types.go** - Schema definitions
- **internal/core/lock_store.go** - Lockfile I/O and migration
- **vendor/vendor.lock** - Actual lockfile
- **docs/LOCKFILE_SCHEMA.md** - Schema documentation
- **docs/ROADMAP.md** - Planned schema changes
- **CLAUDE.md** - Developer documentation
