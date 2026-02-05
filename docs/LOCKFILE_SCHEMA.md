# Lockfile Schema Reference

The `vendor.lock` file tracks the exact state of vendored dependencies, ensuring reproducible builds across environments.

## Current Version: 1.1

## File Location

```text
vendor/vendor.lock
```

## Example Lockfile

```yaml
schema_version: "1.1"
vendors:
  - name: frontend-lib
    ref: main
    commit_hash: abc123def456789abc123def456789abc123def4
    license_path: vendor/licenses/frontend-lib.txt
    updated: "2026-02-04T10:30:00Z"
    license_spdx: MIT
    source_version_tag: v1.2.3
    vendored_at: "2026-01-15T09:00:00Z"
    vendored_by: "Developer <dev@example.com>"
    last_synced_at: "2026-02-04T10:30:00Z"
  - name: shared-utils
    ref: v2.1.0
    commit_hash: 789def456abc123def456789abc123def456789a
    license_path: vendor/licenses/shared-utils.txt
    updated: "2026-02-04T10:30:00Z"
    license_spdx: Apache-2.0
    source_version_tag: v2.1.0
    vendored_at: "2026-02-01T14:00:00Z"
    vendored_by: "Developer <dev@example.com>"
    last_synced_at: "2026-02-04T10:30:00Z"
```

## Schema Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.1 | 2026-02 | Added metadata fields: license_spdx, source_version_tag, vendored_at, vendored_by, last_synced_at |
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
| `license_spdx` | string | 1.1 | No | SPDX license identifier (e.g., `MIT`, `Apache-2.0`). Copied from `vendor.yml`. |
| `source_version_tag` | string | 1.1 | No | Git tag matching the commit hash, if any (e.g., `v1.2.3`). |
| `vendored_at` | string | 1.1 | No | ISO 8601 timestamp of when this dependency was first vendored. |
| `vendored_by` | string | 1.1 | No | Git user identity who vendored it (e.g., `Name <email>`). |
| `last_synced_at` | string | 1.1 | No | ISO 8601 timestamp of most recent sync. |

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
| 1.1 | 1.1+ | Normal operation with full metadata support |
| 1.x (x > 1) | 1.1 | Warning: "Some metadata fields may be ignored" |
| 2.0+ | 1.x | Error: "Lockfile requires newer git-vendor version" |

### Backward Compatibility

- Lockfiles created before schema versioning (without `schema_version`) are treated as version 1.0
- Older CLIs will ignore unknown fields from newer minor versions
- All future 1.x versions will remain backward-compatible with 1.0 CLIs
- Schema 1.1 lockfiles can be read by 1.0 CLIs (new fields ignored)

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

- `git vendor update` always writes `schema_version: "1.1"` (current version)
- `git vendor sync` preserves existing schema version unless lockfile is regenerated
- Migration to new schema versions is automatic when running `git vendor update`

## Migrating Existing Lockfiles

For lockfiles created before v1.1, run:

```bash
git vendor migrate
```

This command:
- Adds `license_spdx` from `vendor.yml` license field
- Sets `vendored_at` from the `updated` timestamp (best guess)
- Sets `vendored_by` to "unknown (migrated)"
- Sets `last_synced_at` from the `updated` timestamp
- Updates `schema_version` to "1.1"

Note: `source_version_tag` cannot be determined without network access. Run `git vendor update` to populate it.

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
2. **Recording provenance**: Timestamps, license info, and user identity provide audit trail
3. **Enabling verification**: `git vendor status` and `git vendor verify` can verify local files match lockfile
4. **SBOM support**: Metadata fields (`license_spdx`, `source_version_tag`) enable SBOM generation
5. **Compliance tracking**: `vendored_at`, `vendored_by`, and `last_synced_at` support compliance audits

## Use Cases for Metadata

The v1.1 metadata fields support various enterprise requirements:

- **SBOM Generation**: `license_spdx` and `source_version_tag` feed into CycloneDX/SPDX output
- **License Compliance**: `license_spdx` enables policy enforcement and auditing
- **Security Scanning**: `source_version_tag` helps correlate with CVE databases
- **Audit Trails**: `vendored_at`, `vendored_by`, `last_synced_at` track dependency lifecycle
- **Metrics**: `vendored_at` and `last_synced_at` enable dependency freshness tracking
