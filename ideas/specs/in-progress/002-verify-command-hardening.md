# Spec 002: Verify Command Hardening

> **Status:** In Progress
> **Priority:** P0 - Foundation
> **Effort:** 3-5 days
> **Dependencies:** None
> **Blocks:** 012 (Drift Detection)

---

## Problem Statement

The `git vendor verify` command is the trust anchor for all downstream features (SBOMs, compliance reports, CVE scans). If verify says "all good," everything else can be trusted. Currently, verify checks file hashes but may miss:
- Files added to vendor directories but not in lockfile (unauthorized additions)
- Files in lockfile but missing from disk (deletions)
- Incomplete or inconsistent output formats

## Solution Overview

Harden the verify command to be bulletproof:
- Detect all three discrepancy types (modified, added, deleted)
- Provide machine-readable JSON output for CI integration
- Use consistent exit codes for automation
- Clear human-readable output with pass/fail per file

## Detailed Design

### 1. Discrepancy Types

| Type | Detection Method | Severity |
|------|------------------|----------|
| **Modified** | Hash mismatch: lockfile SHA ≠ disk SHA | ERROR |
| **Added** | File on disk in vendor path but not in lockfile | WARNING |
| **Deleted** | File in lockfile but missing from disk | ERROR |

### 2. Output Formats

#### Table Format (default)

```
Verifying vendored dependencies...

✓ some-lib/utils.go                     [OK]
✗ some-lib/config.go                    [MODIFIED]
? other-lib/extra.go                    [ADDED]
✗ other-lib/missing.go                  [DELETED]

Summary: 4 files checked
  ✓ 1 verified
  ✗ 2 errors (1 modified, 1 deleted)
  ? 1 warning (1 added)

Result: FAIL
```

#### JSON Format (`--format json`)

```json
{
  "schema_version": "1.0",
  "timestamp": "2026-02-04T12:00:00Z",
  "summary": {
    "total_files": 4,
    "verified": 1,
    "modified": 1,
    "added": 1,
    "deleted": 1,
    "result": "FAIL"
  },
  "files": [
    {
      "path": "lib/some-lib/utils.go",
      "vendor": "some-lib",
      "status": "verified",
      "expected_hash": "abc123...",
      "actual_hash": "abc123..."
    },
    {
      "path": "lib/some-lib/config.go",
      "vendor": "some-lib",
      "status": "modified",
      "expected_hash": "abc123...",
      "actual_hash": "def456..."
    },
    {
      "path": "lib/other-lib/extra.go",
      "vendor": null,
      "status": "added",
      "expected_hash": null,
      "actual_hash": "ghi789..."
    },
    {
      "path": "lib/other-lib/missing.go",
      "vendor": "other-lib",
      "status": "deleted",
      "expected_hash": "jkl012...",
      "actual_hash": null
    }
  ]
}
```

### 3. Exit Codes

| Code | Meaning | When |
|------|---------|------|
| 0 | PASS | All files verified, no discrepancies |
| 1 | FAIL | Modified or deleted files detected |
| 2 | WARN | Only added files detected (no modifications) |

### 4. Implementation

#### New Types

```go
// VerifyResult represents the outcome of verification
type VerifyResult struct {
    SchemaVersion string       `json:"schema_version"`
    Timestamp     time.Time    `json:"timestamp"`
    Summary       VerifySummary `json:"summary"`
    Files         []FileStatus  `json:"files"`
}

type VerifySummary struct {
    TotalFiles int    `json:"total_files"`
    Verified   int    `json:"verified"`
    Modified   int    `json:"modified"`
    Added      int    `json:"added"`
    Deleted    int    `json:"deleted"`
    Result     string `json:"result"` // PASS, FAIL, WARN
}

type FileStatus struct {
    Path         string  `json:"path"`
    Vendor       *string `json:"vendor"`
    Status       string  `json:"status"` // verified, modified, added, deleted
    ExpectedHash *string `json:"expected_hash,omitempty"`
    ActualHash   *string `json:"actual_hash,omitempty"`
}
```

#### Core Algorithm

```go
func (m *Manager) Verify(format string) (*VerifyResult, error) {
    lock, err := m.lockStore.Load(m.lockPath)
    if err != nil {
        return nil, fmt.Errorf("load lockfile: %w", err)
    }

    result := &VerifyResult{
        SchemaVersion: "1.0",
        Timestamp:     time.Now().UTC(),
    }

    // Build map of expected files from lockfile
    expectedFiles := make(map[string]string) // path -> hash
    for _, vendor := range lock.Vendors {
        for path, hash := range vendor.FileHashes {
            expectedFiles[path] = hash
        }
    }

    // Check all expected files
    for path, expectedHash := range expectedFiles {
        actualHash, err := m.fs.HashFile(path)
        if err != nil {
            if os.IsNotExist(err) {
                result.Files = append(result.Files, FileStatus{
                    Path:         path,
                    Status:       "deleted",
                    ExpectedHash: &expectedHash,
                })
                result.Summary.Deleted++
                continue
            }
            return nil, fmt.Errorf("hash file %s: %w", path, err)
        }

        if actualHash == expectedHash {
            result.Files = append(result.Files, FileStatus{
                Path:         path,
                Status:       "verified",
                ExpectedHash: &expectedHash,
                ActualHash:   &actualHash,
            })
            result.Summary.Verified++
        } else {
            result.Files = append(result.Files, FileStatus{
                Path:         path,
                Status:       "modified",
                ExpectedHash: &expectedHash,
                ActualHash:   &actualHash,
            })
            result.Summary.Modified++
        }
    }

    // Scan for added files (not in lockfile)
    addedFiles, err := m.findAddedFiles(lock)
    if err != nil {
        return nil, fmt.Errorf("scan for added files: %w", err)
    }
    for _, af := range addedFiles {
        result.Files = append(result.Files, af)
        result.Summary.Added++
    }

    // Compute result
    result.Summary.TotalFiles = len(result.Files)
    if result.Summary.Modified > 0 || result.Summary.Deleted > 0 {
        result.Summary.Result = "FAIL"
    } else if result.Summary.Added > 0 {
        result.Summary.Result = "WARN"
    } else {
        result.Summary.Result = "PASS"
    }

    return result, nil
}
```

#### Added File Detection

```go
func (m *Manager) findAddedFiles(lock *VendorLock) ([]FileStatus, error) {
    var added []FileStatus

    // Get all vendor destination paths from config
    cfg, err := m.configStore.Load(m.configPath)
    if err != nil {
        return nil, err
    }

    // Build set of expected paths
    expectedPaths := make(map[string]bool)
    for _, vendor := range lock.Vendors {
        for path := range vendor.FileHashes {
            expectedPaths[path] = true
        }
    }

    // Walk each vendor destination directory
    for _, vendor := range cfg.Vendors {
        for _, spec := range vendor.Specs {
            for _, mapping := range spec.Mapping {
                destPath := mapping.To
                err := filepath.WalkDir(destPath, func(path string, d fs.DirEntry, err error) error {
                    if err != nil || d.IsDir() {
                        return err
                    }
                    if !expectedPaths[path] {
                        hash, _ := m.fs.HashFile(path)
                        added = append(added, FileStatus{
                            Path:       path,
                            Status:     "added",
                            ActualHash: &hash,
                        })
                    }
                    return nil
                })
                if err != nil && !os.IsNotExist(err) {
                    return nil, err
                }
            }
        }
    }

    return added, nil
}
```

### 5. CLI Integration

```go
func runVerify(format string, output string) {
    result, err := manager.Verify(format)
    if err != nil {
        tui.PrintError("Verification failed", err.Error())
        os.Exit(1)
    }

    // Output results
    var out io.Writer = os.Stdout
    if output != "" {
        f, err := os.Create(output)
        if err != nil {
            tui.PrintError("Cannot create output file", err.Error())
            os.Exit(1)
        }
        defer f.Close()
        out = f
    }

    switch format {
    case "json":
        enc := json.NewEncoder(out)
        enc.SetIndent("", "  ")
        enc.Encode(result)
    default:
        printVerifyTable(out, result)
    }

    // Exit code based on result
    switch result.Summary.Result {
    case "PASS":
        os.Exit(0)
    case "WARN":
        os.Exit(2)
    default:
        os.Exit(1)
    }
}
```

## Test Plan

### Unit Tests

```go
func TestVerify_ModifiedFile(t *testing.T) {
    // Setup: lockfile says hash A, file on disk has hash B
    // Expect: status "modified", result FAIL
}

func TestVerify_DeletedFile(t *testing.T) {
    // Setup: lockfile references file that doesn't exist
    // Expect: status "deleted", result FAIL
}

func TestVerify_AddedFile(t *testing.T) {
    // Setup: file exists in vendor dir but not in lockfile
    // Expect: status "added", result WARN
}

func TestVerify_AllPass(t *testing.T) {
    // Setup: all files match lockfile
    // Expect: all status "verified", result PASS
}

func TestVerify_JSONOutput(t *testing.T) {
    // Verify JSON output is valid and complete
}
```

### Integration Tests

1. Create temp repo with vendor.yml and vendor.lock
2. Run sync to create files
3. Verify passes
4. Modify a file → verify fails (modified)
5. Delete a file → verify fails (deleted)
6. Add a file → verify warns (added)
7. Verify JSON output is parseable

## Acceptance Criteria

- [ ] Detects modified files (hash mismatch)
- [ ] Detects added files (in vendor dir but not in lockfile)
- [ ] Detects deleted files (in lockfile but not on disk)
- [ ] `--format json` produces machine-parseable output
- [ ] Exit codes: 0=PASS, 1=FAIL, 2=WARN
- [ ] Exit codes are documented in help text
- [ ] Integration test covers all three discrepancy types
- [ ] Table output is clear and easy to read

## Rollout Plan

1. Implement new verify logic with backward compatibility
2. Add --format flag (default: table)
3. Add --output flag for file output
4. Update help text with exit code documentation
5. Release in v1.1.0

## Open Questions

1. **Should added files be ERROR or WARN?** Current spec uses WARN to avoid breaking existing workflows where users intentionally add files to vendor directories. Could be configurable via `--strict` flag.

2. **What about empty directories?** Current spec only tracks files. Empty directories in lockfile but missing on disk won't be detected.

## References

- ROADMAP.md Feature 1.2
- CLAUDE.md verify command docs
