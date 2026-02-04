# Spec 003: Lockfile Metadata Enrichment

> **Status:** In Progress
> **Priority:** P0 - Foundation
> **Effort:** 5-7 days
> **Dependencies:** 001 (Schema Versioning)
> **Blocks:** 010 (SBOM), 011 (CVE Scanning), 013 (License Policy), 021 (Graph), 024 (Metrics)

---

## Problem Statement

SBOM generation, compliance reports, and license scanning all need metadata that currently isn't captured in the lockfile:
- License information (SPDX identifier)
- Version tag if the commit matches a release
- When the dependency was vendored and by whom
- When it was last synced

Collecting this at `add`/`sync` time (when we have the source repo cloned) is trivial. Reconstructing it later is painful or impossible.

## Solution Overview

Extend `vendor.lock` entries with:
- `license_spdx` - SPDX identifier for the detected license
- `source_version_tag` - Git tag matching the vendored commit (if any)
- `vendored_at` - ISO 8601 timestamp of initial vendoring
- `vendored_by` - Git user identity who vendored it
- `last_synced_at` - ISO 8601 timestamp of most recent sync

## Detailed Design

### 1. Extended LockDetails Structure

```go
type LockDetails struct {
    // Existing fields
    Name        string `yaml:"name"`
    Ref         string `yaml:"ref"`
    CommitHash  string `yaml:"commit_hash"`
    LicensePath string `yaml:"license_path,omitempty"`
    Updated     string `yaml:"updated"`

    // New fields (schema v1.1)
    LicenseSPDX       string `yaml:"license_spdx,omitempty"`
    SourceVersionTag  string `yaml:"source_version_tag,omitempty"`
    VendoredAt        string `yaml:"vendored_at,omitempty"`
    VendoredBy        string `yaml:"vendored_by,omitempty"`
    LastSyncedAt      string `yaml:"last_synced_at,omitempty"`
}
```

### 2. SPDX License Detection

#### Detection Sources (in priority order)

1. **GitHub/GitLab API** - Already implemented, returns SPDX identifier
2. **LICENSE file analysis** - Parse LICENSE, LICENSE.md, LICENSE.txt, COPYING
3. **Package manifest** - Check package.json, Cargo.toml, go.mod for license field

#### Using go-license-detector

```go
import "github.com/go-enry/go-license-detector/v4/licensedb"

func detectLicense(repoPath string) (string, error) {
    licenses, err := licensedb.Detect(repoPath)
    if err != nil {
        return "", err
    }

    // Return highest confidence match
    var best string
    var bestConf float32
    for id, match := range licenses {
        if match.Confidence > bestConf {
            best = id
            bestConf = match.Confidence
        }
    }

    if bestConf < 0.85 {
        return "", nil // Confidence too low
    }

    return best, nil
}
```

#### Supported Licenses (minimum set)

| SPDX ID | License Name |
|---------|--------------|
| MIT | MIT License |
| Apache-2.0 | Apache License 2.0 |
| GPL-2.0-only | GNU GPL v2 |
| GPL-3.0-only | GNU GPL v3 |
| BSD-2-Clause | BSD 2-Clause |
| BSD-3-Clause | BSD 3-Clause |
| ISC | ISC License |
| MPL-2.0 | Mozilla Public License 2.0 |
| LGPL-2.1-only | GNU LGPL v2.1 |
| Unlicense | The Unlicense |

### 3. Version Tag Detection

```go
func (g *SystemGitClient) GetTagForCommit(repoPath, commitHash string) (string, error) {
    // Get tags pointing to this exact commit
    out, err := g.run(repoPath, "tag", "--points-at", commitHash)
    if err != nil {
        return "", nil // No tags, not an error
    }

    tags := strings.Split(strings.TrimSpace(out), "\n")
    if len(tags) == 0 || tags[0] == "" {
        return "", nil
    }

    // Prefer semver-looking tags (v1.0.0, 1.0.0)
    for _, tag := range tags {
        if isSemverTag(tag) {
            return tag, nil
        }
    }

    // Fall back to first tag
    return tags[0], nil
}

func isSemverTag(tag string) bool {
    tag = strings.TrimPrefix(tag, "v")
    matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+`, tag)
    return matched
}
```

### 4. User Identity Capture

```go
func getGitUserIdentity() (string, error) {
    name, err := exec.Command("git", "config", "user.name").Output()
    if err != nil {
        return "", err
    }

    email, err := exec.Command("git", "config", "user.email").Output()
    if err != nil {
        return "", err
    }

    return fmt.Sprintf("%s <%s>",
        strings.TrimSpace(string(name)),
        strings.TrimSpace(string(email))), nil
}
```

### 5. Integration with Existing Flow

#### During `git vendor add`

```go
func (vs *VendorSyncer) addVendor(vendor *VendorSpec) error {
    // ... existing clone logic ...

    // Collect metadata
    now := time.Now().UTC().Format(time.RFC3339)
    user, _ := getGitUserIdentity()
    license := vs.detectLicense(tempDir)
    versionTag, _ := vs.git.GetTagForCommit(tempDir, commitHash)

    lockEntry := LockDetails{
        // ... existing fields ...
        LicenseSPDX:      license,
        SourceVersionTag: versionTag,
        VendoredAt:       now,
        VendoredBy:       user,
        LastSyncedAt:     now,
    }

    // ... save lockfile ...
}
```

#### During `git vendor sync`

```go
func (vs *VendorSyncer) syncVendor(vendor *VendorSpec, existing *LockDetails) error {
    // ... existing sync logic ...

    // Preserve original vendored_at, update last_synced_at
    now := time.Now().UTC().Format(time.RFC3339)

    lockEntry := LockDetails{
        // ... existing fields ...
        VendoredAt:   existing.VendoredAt, // Preserve original
        VendoredBy:   existing.VendoredBy, // Preserve original
        LastSyncedAt: now,                 // Update to now
    }

    // Re-detect license and version tag (may have changed)
    lockEntry.LicenseSPDX = vs.detectLicense(tempDir)
    lockEntry.SourceVersionTag, _ = vs.git.GetTagForCommit(tempDir, commitHash)

    // ... save lockfile ...
}
```

### 6. List Command Enhancement

Update `git vendor list` to show new metadata:

```
Vendored Dependencies:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  some-lib
    URL:      https://github.com/owner/some-lib
    Ref:      main @ abc1234
    Version:  v1.2.3
    License:  MIT
    Vendored: 2026-01-15 by User <user@example.com>
    Synced:   2026-02-04

  other-lib
    URL:      https://github.com/owner/other-lib
    Ref:      v2.0.0 @ def5678
    Version:  v2.0.0
    License:  Apache-2.0
    Vendored: 2026-02-01 by User <user@example.com>
    Synced:   2026-02-01
```

### 7. Migration Command

For existing lockfiles without metadata:

```bash
git vendor migrate
```

```go
func (vs *VendorSyncer) MigrateLockfile() error {
    lock, err := vs.lockStore.Load(vs.lockPath)
    if err != nil {
        return err
    }

    for i, entry := range lock.Vendors {
        // Skip if already has metadata
        if entry.VendoredAt != "" {
            continue
        }

        // Set defaults for non-computable fields
        lock.Vendors[i].VendoredAt = entry.Updated // Best guess
        lock.Vendors[i].VendoredBy = "unknown"
        lock.Vendors[i].LastSyncedAt = entry.Updated

        // Try to fetch computable metadata (requires network)
        // This is optional - skip if offline
        if vs.canFetchMetadata(entry) {
            license, tag := vs.fetchMetadata(entry)
            lock.Vendors[i].LicenseSPDX = license
            lock.Vendors[i].SourceVersionTag = tag
        }
    }

    // Update schema version
    lock.SchemaVersion = "1.1"

    return vs.lockStore.Save(vs.lockPath, lock)
}
```

## Test Plan

### Unit Tests

```go
func TestDetectLicense_MIT(t *testing.T) {
    // Create temp dir with MIT LICENSE file
    // Verify detection returns "MIT"
}

func TestDetectLicense_Apache(t *testing.T) {
    // Create temp dir with Apache LICENSE file
    // Verify detection returns "Apache-2.0"
}

func TestGetTagForCommit_WithTag(t *testing.T) {
    // Create temp repo with tagged commit
    // Verify tag is returned
}

func TestGetTagForCommit_NoTag(t *testing.T) {
    // Create temp repo without tags
    // Verify empty string returned
}

func TestGetTagForCommit_PrefersSemver(t *testing.T) {
    // Create commit with multiple tags (release, v1.0.0)
    // Verify v1.0.0 is returned
}
```

### Integration Tests

1. `git vendor add` captures all metadata fields
2. `git vendor sync` preserves vendored_at, updates last_synced_at
3. `git vendor list` displays metadata correctly
4. `git vendor migrate` updates old lockfiles
5. License detection works for top 10 licenses

## Acceptance Criteria

- [ ] `git vendor add` populates all new metadata fields
- [ ] `git vendor sync` updates metadata, preserving vendored_at
- [ ] License detection correctly identifies: MIT, Apache-2.0, GPL-2.0, GPL-3.0, BSD-2-Clause, BSD-3-Clause, ISC, MPL-2.0, LGPL-2.1, Unlicense
- [ ] `git vendor list` displays license and version tag when available
- [ ] `git vendor migrate` updates existing lockfiles with computable metadata
- [ ] Schema version incremented to 1.1
- [ ] All fields documented in LOCKFILE_SCHEMA.md

## Dependencies

### New Dependencies

```go
// go.mod
require github.com/go-enry/go-license-detector/v4 v4.3.0
```

Note: This adds ~2MB to binary size. Alternative: use simpler regex-based detection for common licenses only.

## Rollout Plan

1. Implement schema version 1.1 support (from spec 001)
2. Add new fields to LockDetails struct
3. Implement license detection
4. Implement version tag detection
5. Update add/sync flows to capture metadata
6. Update list command to display metadata
7. Implement migrate command
8. Update documentation
9. Release in v1.1.0

## Open Questions

1. **License detection library size:** go-license-detector adds ~2MB. Worth it for accuracy, or use simpler approach?

2. **Offline behavior:** When offline during migrate, should we skip metadata or error? Current spec: skip with warning.

3. **Multiple licenses:** Some repos have dual licenses. Should we capture all or just primary? Current spec: capture highest-confidence match only.

## References

- ROADMAP.md Feature 1.3
- SPDX License List: https://spdx.org/licenses/
- go-license-detector: https://github.com/go-enry/go-license-detector
