# Spec 001: Lockfile Schema Versioning

> **Status:** In Progress
> **Priority:** P0 - Foundation
> **Effort:** 2-3 days
> **Dependencies:** None
> **Blocks:** 003 (Metadata Enrichment), all future lockfile extensions

---

## Problem Statement

The `vendor.lock` file currently has no schema version field. As new features add metadata fields (license, timestamps, provenance), older CLI versions encountering unknown fields may behave unpredictably. Additionally, there's no way to communicate to users that they need to upgrade when breaking changes occur.

## Solution Overview

Add a `schema_version` field to the lockfile as the foundation for all future lockfile extensions. Implement version negotiation logic that:
- Treats missing version as v1.0 (backward compatibility)
- Warns on unknown minor versions but proceeds
- Errors on unknown major versions with upgrade instructions

## Detailed Design

### 1. Schema Version Format

```yaml
schema_version: "1.0"
vendors:
  - name: some-lib
    # ... existing fields
```

- **Format:** Semantic versioning major.minor (no patch)
- **Major version:** Incremented for breaking changes (should never happen)
- **Minor version:** Incremented when new fields are added

### 2. Version Compatibility Matrix

| Lockfile Version | CLI Understanding | Behavior |
|------------------|-------------------|----------|
| Missing | Any | Treat as 1.0, no warning |
| 1.0 | 1.0+ | Normal operation |
| 1.1 | 1.0 | Warn: "Lockfile has unknown fields, some features may not work" |
| 1.1 | 1.1+ | Normal operation |
| 2.0 | 1.x | Error: "This lockfile requires git-vendor v2.x. Run `git vendor version` to check your version." |

### 3. Implementation Changes

#### lock_store.go

```go
// VendorLock represents the lockfile structure
type VendorLock struct {
    SchemaVersion string        `yaml:"schema_version,omitempty"`
    Vendors       []LockDetails `yaml:"vendors"`
}

const (
    CurrentSchemaVersion = "1.0"
    MaxSupportedMajor    = 1
    MaxSupportedMinor    = 0
)
```

#### Version Parsing

```go
func parseSchemaVersion(version string) (major, minor int, err error) {
    if version == "" {
        return 1, 0, nil // Default to 1.0
    }
    parts := strings.Split(version, ".")
    if len(parts) != 2 {
        return 0, 0, fmt.Errorf("invalid schema version format: %s", version)
    }
    major, err = strconv.Atoi(parts[0])
    if err != nil {
        return 0, 0, fmt.Errorf("invalid major version: %s", parts[0])
    }
    minor, err = strconv.Atoi(parts[1])
    if err != nil {
        return 0, 0, fmt.Errorf("invalid minor version: %s", parts[1])
    }
    return major, minor, nil
}
```

#### Load Function Updates

```go
func (s *YAMLLockStore) Load(path string) (*VendorLock, error) {
    // ... existing load logic ...

    major, minor, err := parseSchemaVersion(lock.SchemaVersion)
    if err != nil {
        return nil, fmt.Errorf("parse lockfile: %w", err)
    }

    if major > MaxSupportedMajor {
        return nil, fmt.Errorf(
            "lockfile schema version %s requires a newer git-vendor version\n"+
            "  Context: Your CLI supports schema v%d.x, lockfile is v%d.x\n"+
            "  Fix: Update git-vendor to the latest version",
            lock.SchemaVersion, MaxSupportedMajor, major)
    }

    if minor > MaxSupportedMinor {
        fmt.Fprintf(os.Stderr,
            "Warning: Lockfile schema version %s is newer than expected (%d.%d)\n"+
            "  Some metadata fields may be ignored. Consider updating git-vendor.\n",
            lock.SchemaVersion, MaxSupportedMajor, MaxSupportedMinor)
    }

    return lock, nil
}
```

#### Save Function Updates

```go
func (s *YAMLLockStore) Save(path string, lock *VendorLock) error {
    lock.SchemaVersion = CurrentSchemaVersion
    // ... existing save logic ...
}
```

### 4. LOCKFILE_SCHEMA.md Documentation

Create `docs/LOCKFILE_SCHEMA.md`:

```markdown
# Lockfile Schema Reference

## Current Version: 1.0

The `vendor.lock` file tracks the exact state of vendored dependencies.

## Schema Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02 | Initial versioned schema |

## Field Reference

### Top-Level Fields

| Field | Type | Since | Required | Description |
|-------|------|-------|----------|-------------|
| schema_version | string | 1.0 | No | Schema version (default: "1.0") |
| vendors | array | 1.0 | Yes | List of locked vendor entries |

### Vendor Entry Fields

| Field | Type | Since | Required | Description |
|-------|------|-------|----------|-------------|
| name | string | 1.0 | Yes | Vendor display name |
| ref | string | 1.0 | Yes | Git ref (branch/tag) |
| commit_hash | string | 1.0 | Yes | Full commit SHA |
| license_path | string | 1.0 | No | Path to cached license file |
| updated | string | 1.0 | Yes | ISO 8601 timestamp |

## Compatibility

- Older CLIs (pre-versioning) treat missing schema_version as 1.0
- Unknown fields in newer minor versions are ignored
- Major version mismatch produces an error with upgrade instructions
```

## Test Plan

### Unit Tests

```go
func TestParseSchemaVersion(t *testing.T) {
    tests := []struct {
        input        string
        wantMajor    int
        wantMinor    int
        wantErr      bool
    }{
        {"", 1, 0, false},          // Default
        {"1.0", 1, 0, false},
        {"1.1", 1, 1, false},
        {"2.0", 2, 0, false},
        {"1", 0, 0, true},          // Invalid format
        {"1.0.0", 0, 0, true},      // Invalid format
        {"a.b", 0, 0, true},        // Non-numeric
    }
    // ...
}

func TestLoadLockfile_VersionCompatibility(t *testing.T) {
    tests := []struct {
        name        string
        version     string
        wantErr     bool
        wantWarning bool
    }{
        {"missing version", "", false, false},
        {"current version", "1.0", false, false},
        {"newer minor", "1.5", false, true},
        {"newer major", "2.0", true, false},
    }
    // ...
}
```

### Integration Tests

1. **Backward compatibility:** Load lockfile without schema_version, verify it works
2. **Forward compatibility:** Load lockfile with unknown fields, verify warning but no error
3. **Major version block:** Load lockfile with schema_version: "2.0", verify error message
4. **Save always writes version:** Save lockfile, verify schema_version is present

## Acceptance Criteria

- [ ] `vendor.lock` includes `schema_version: "1.0"` on new `git vendor add`
- [ ] Old lockfiles without `schema_version` still parse correctly
- [ ] CLI warns on unknown minor version (e.g., "1.5")
- [ ] CLI errors on unknown major version (e.g., "2.0") with actionable message
- [ ] `docs/LOCKFILE_SCHEMA.md` documents every field, its type, when added, what it means
- [ ] All existing tests pass unchanged
- [ ] New tests cover version parsing and compatibility logic

## Rollout Plan

1. Add schema_version field as optional (write always, read with default)
2. Update all lockfile writes to include schema_version
3. Create LOCKFILE_SCHEMA.md documentation
4. Release in v1.1.0

## Open Questions

*None currently.*

## References

- ROADMAP.md Feature 1.1
- Semantic versioning: https://semver.org/
- YAML spec: https://yaml.org/spec/1.2.2/
