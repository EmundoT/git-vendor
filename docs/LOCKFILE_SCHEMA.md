# Lockfile Schema Reference

The `vendor.lock` file tracks the exact state of vendored dependencies, ensuring reproducible builds across environments.

## Current Version: 1.0

## File Location

```text
vendor/vendor.lock
```

## Example Lockfile

```yaml
schema_version: "1.0"
vendors:
  - name: frontend-lib
    ref: main
    commit_hash: abc123def456789abc123def456789abc123def4
    license_path: vendor/licenses/frontend-lib.txt
    updated: "2026-02-04T10:30:00Z"
  - name: shared-utils
    ref: v2.1.0
    commit_hash: 789def456abc123def456789abc123def456789a
    license_path: vendor/licenses/shared-utils.txt
    updated: "2026-02-04T10:30:00Z"
```

## Schema Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02 | Initial versioned schema |

## Field Reference

### Top-Level Fields

| Field | Type | Since | Required | Description |
|-------|------|-------|----------|-------------|
| `schema_version` | string | 1.0 | No | Schema version in `major.minor` format. Defaults to `"1.0"` if omitted. |
| `vendors` | array | 1.0 | Yes | List of locked vendor entries. |

### Vendor Entry Fields

| Field | Type | Since | Required | Description |
|-------|------|-------|----------|-------------|
| `name` | string | 1.0 | Yes | Vendor display name (must match `vendor.yml`). |
| `ref` | string | 1.0 | Yes | Git ref (branch name, tag, or commit SHA). |
| `commit_hash` | string | 1.0 | Yes | Full 40-character commit SHA at time of sync. |
| `license_path` | string | 1.0 | No | Path to cached license file (e.g., `vendor/licenses/name.txt`). |
| `updated` | string | 1.0 | Yes | ISO 8601 timestamp of when this entry was last updated. |

## Version Compatibility

### Versioning Semantics

The schema version uses `major.minor` format:

- **Major version**: Incremented for breaking changes requiring CLI upgrade
- **Minor version**: Incremented when new optional fields are added

### CLI Compatibility Matrix

| Lockfile Version | CLI Understanding | Behavior |
|------------------|-------------------|----------|
| Missing | Any | Treated as 1.0, no warning |
| 1.0 | 1.0+ | Normal operation |
| 1.x (x > 0) | 1.0 | Warning: "Some metadata fields may be ignored" |
| 2.0+ | 1.x | Error: "Lockfile requires newer git-vendor version" |

### Backward Compatibility

- Lockfiles created before schema versioning (without `schema_version`) are treated as version 1.0
- Older CLIs will ignore unknown fields from newer minor versions
- All future 1.x versions will remain backward-compatible with 1.0 CLIs

### Forward Compatibility

When encountering a lockfile with:

1. **Unknown minor version** (e.g., CLI v1.0 reads lockfile v1.5):
   - Warning is displayed to stderr
   - Operation proceeds normally
   - Unknown fields are preserved during read/write cycles

2. **Unknown major version** (e.g., CLI v1.x reads lockfile v2.0):
   - Error is displayed with upgrade instructions
   - Operation aborts to prevent data corruption

## Automatic Version Management

- `git vendor update` always writes `schema_version: "1.0"` (current version)
- `git vendor sync` preserves existing schema version unless lockfile is regenerated
- Migration to new schema versions is automatic when running `git vendor update`

## Relationship to vendor.yml

The lockfile (`vendor.lock`) is derived from the configuration file (`vendor.yml`):

```text
vendor.yml (configuration)    vendor.lock (state)
-------------------------    -------------------
vendors:                     schema_version: "1.0"
  - name: lib                vendors:
    url: https://...           - name: lib
    specs:                       ref: main
      - ref: main                commit_hash: abc123...
        mapping: [...]           updated: 2026-02-04T...
```

- `vendor.yml` declares what to vendor (repositories, refs, paths)
- `vendor.lock` records what was vendored (exact commit SHAs, timestamps)
- Both files should be committed to version control

## Security Considerations

The lockfile ensures supply chain security by:

1. **Pinning exact commits**: Prevents unexpected code changes between syncs
2. **Recording provenance**: Timestamps and license paths provide audit trail
3. **Enabling verification**: `git vendor status` can verify local files match lockfile
