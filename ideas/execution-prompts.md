# Execution Plan: git-vendor Feature Implementation

> Generated: 2026-02-04
> Based on: ideas/queue.md, ideas/code_quality.md, ideas/security.md

---

## Queue Health Summary

| Queue | HIGH | MEDIUM | LOW | Spec Coverage | Stale Claims |
|-------|------|--------|-----|---------------|--------------|
| queue.md (P0 Foundation) | 5 | 0 | 0 | 60% (3/5) | 0 items |
| queue.md (P0 Supply Chain) | 4 | 0 | 0 | 50% (2/4) | 0 items |
| code_quality.md | 4 | 6 | 6 | 0% | 0 items |
| security.md | 1 CRITICAL, 4 HIGH | 4 | 3 | 0% | 0 items |

## Dependency Analysis

```
PARALLEL GROUP A (No dependencies - run simultaneously):
├── 001 Lockfile Schema Versioning [spec ready]
└── 002 Verify Command Hardening [spec ready]

SEQUENTIAL (After Group A completes):
└── 003 Lockfile Metadata Enrichment [spec ready, depends on 001]

PARALLEL GROUP B (After 003 completes - run simultaneously):
├── 010 SBOM Generation [spec ready]
└── 011 CVE Vulnerability Scanning [spec ready]

INDEPENDENT CODE QUALITY (Can run any time):
├── CQ-002 Error Wrapping Consistency
├── CQ-003 Context Propagation
└── CQ-004 Godoc Coverage

INDEPENDENT SECURITY (Can run any time):
└── SEC-001 Path Traversal Audit [CRITICAL]
```

---

## Execution Order

**Phase 1 - Parallel Group A (run simultaneously):**
- PROMPT 1: Lockfile Schema Versioning (001)
- PROMPT 2: Verify Command Hardening (002)

**Phase 2 - Sequential (after Phase 1):**
- PROMPT 3: Lockfile Metadata Enrichment (003)

**Phase 3 - Parallel Group B (after Phase 2, run simultaneously):**
- PROMPT 4: SBOM Generation (010)
- PROMPT 5: CVE Vulnerability Scanning (011)

**Independent Tasks (can run in parallel with any phase):**
- PROMPT 6: Error Wrapping Consistency (CQ-002)
- PROMPT 7: Context Propagation (CQ-003)
- PROMPT 8: Path Traversal Audit (SEC-001) [CRITICAL]

---

## PROMPT 1: Lockfile Schema Versioning (001)

```
TASK: Implement 001: Lockfile Schema Versioning

CONTEXT: This is the foundation for all future lockfile extensions. SBOM generation, CVE scanning, and metadata enrichment all depend on having schema versioning in place first. This is a P0 priority item.

SPEC: ideas/specs/in-progress/001-lockfile-schema-versioning.md

SCOPE:
- Files to modify:
  - internal/types/types.go (add SchemaVersion to VendorLock)
  - internal/core/lock_store.go (add version parsing, compatibility checks)
- Files to create:
  - docs/LOCKFILE_SCHEMA.md
- Tests to add:
  - internal/core/lock_store_test.go (TestParseSchemaVersion, TestLoadLockfile_VersionCompatibility)

IMPLEMENTATION GUIDANCE:
1. Add `SchemaVersion string` field to VendorLock struct in types.go
2. Create constants: CurrentSchemaVersion = "1.0", MaxSupportedMajor = 1, MaxSupportedMinor = 0
3. Implement parseSchemaVersion(version string) (major, minor int, err error)
4. Update Load() to validate schema version:
   - Missing version → treat as 1.0
   - Unknown minor → warn but proceed
   - Unknown major → error with upgrade instructions
5. Update Save() to always write CurrentSchemaVersion
6. Create docs/LOCKFILE_SCHEMA.md documenting all fields

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change 001 status to "completed"
2. Move row to ideas/completed.md under "Feature Ideas" section
3. Move spec from ideas/specs/in-progress/ to ideas/specs/complete/
4. Update CLAUDE.md if any user-facing behavior changes

ACCEPTANCE CRITERIA:
- [ ] `vendor.lock` includes `schema_version: "1.0"` on new `git vendor add`
- [ ] Old lockfiles without `schema_version` still parse correctly
- [ ] CLI warns on unknown minor version (e.g., "1.5")
- [ ] CLI errors on unknown major version (e.g., "2.0") with actionable message
- [ ] docs/LOCKFILE_SCHEMA.md documents every field
- [ ] All existing tests pass
- [ ] New unit tests cover version parsing and compatibility logic
- [ ] Run `go test ./...` and ensure all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 2: Verify Command Hardening (002)

```
TASK: Implement 002: Verify Command Hardening

CONTEXT: The verify command is the trust anchor for all downstream features (SBOMs, compliance reports, CVE scans). If verify says "all good," everything else can be trusted. Currently it may miss added or deleted files. This is a P0 priority item.

SPEC: ideas/specs/in-progress/002-verify-command-hardening.md

SCOPE:
- Files to modify:
  - internal/core/vendor_syncer.go (add/enhance Verify method)
  - internal/types/types.go (add VerifyResult, FileStatus types)
  - main.go (add --format flag, update exit codes)
- Tests to add:
  - internal/core/vendor_syncer_test.go (TestVerify_ModifiedFile, TestVerify_DeletedFile, TestVerify_AddedFile, TestVerify_AllPass, TestVerify_JSONOutput)

IMPLEMENTATION GUIDANCE:
1. Add new types to types.go:
   - VerifyResult (SchemaVersion, Timestamp, Summary, Files)
   - VerifySummary (TotalFiles, Verified, Modified, Added, Deleted, Result)
   - FileStatus (Path, Vendor, Status, ExpectedHash, ActualHash)
2. Implement Verify() method that:
   - Builds map of expected files from lockfile
   - Checks each expected file (hash match, missing = deleted)
   - Scans vendor directories for added files not in lockfile
   - Returns VerifyResult with summary
3. Add --format flag (table|json) to verify command
4. Implement exit codes: 0=PASS, 1=FAIL, 2=WARN
5. Update help text to document exit codes

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change 002 status to "completed"
2. Move row to ideas/completed.md under "Feature Ideas" section
3. Move spec from ideas/specs/in-progress/ to ideas/specs/complete/
4. Update CLAUDE.md verify command documentation

ACCEPTANCE CRITERIA:
- [ ] Detects modified files (hash mismatch)
- [ ] Detects added files (in vendor dir but not in lockfile)
- [ ] Detects deleted files (in lockfile but not on disk)
- [ ] `--format json` produces machine-parseable output
- [ ] Exit codes: 0=PASS, 1=FAIL, 2=WARN
- [ ] Exit codes documented in help text
- [ ] Table output is clear with symbols (✓ ✗ ?)
- [ ] All existing tests pass
- [ ] Run `go test ./...` and ensure all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 3: Lockfile Metadata Enrichment (003)

```
TASK: Implement 003: Lockfile Metadata Enrichment

CONTEXT: SBOM generation, compliance reports, and license scanning need metadata that isn't currently captured. This builds on 001 (schema versioning) and is required by 010 (SBOM), 011 (CVE scanning), and other features. P0 priority.

SPEC: ideas/specs/in-progress/003-lockfile-metadata-enrichment.md

DEPENDENCIES: Requires 001 (Lockfile Schema Versioning) to be completed first.

SCOPE:
- Files to modify:
  - internal/types/types.go (extend LockDetails with new fields)
  - internal/core/vendor_syncer.go (capture metadata during add/sync)
  - internal/core/git_operations.go (add GetTagForCommit method)
  - main.go (add migrate command, update list output)
- Files to create:
  - None (uses existing license detection)
- Tests to add:
  - internal/core/git_operations_test.go (TestGetTagForCommit_*)
  - internal/core/vendor_syncer_test.go (TestAddVendor_CapturesMetadata, TestSyncVendor_PreservesVendoredAt)

IMPLEMENTATION GUIDANCE:
1. Add new fields to LockDetails in types.go:
   - LicenseSPDX string `yaml:"license_spdx,omitempty"`
   - SourceVersionTag string `yaml:"source_version_tag,omitempty"`
   - VendoredAt string `yaml:"vendored_at,omitempty"`
   - VendoredBy string `yaml:"vendored_by,omitempty"`
   - LastSyncedAt string `yaml:"last_synced_at,omitempty"`
2. Implement GetTagForCommit(repoPath, commitHash) in git_operations.go
3. Implement getGitUserIdentity() helper
4. Update addVendor() to populate all metadata fields
5. Update syncVendor() to preserve VendoredAt/VendoredBy, update LastSyncedAt
6. Update list command to display new metadata
7. Add migrate command for existing lockfiles
8. Update schema version to "1.1"

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change 003 status to "completed"
2. Move row to ideas/completed.md under "Feature Ideas" section
3. Move spec from ideas/specs/in-progress/ to ideas/specs/complete/
4. Update docs/LOCKFILE_SCHEMA.md with new fields
5. Update CLAUDE.md with new metadata in lockfile documentation

ACCEPTANCE CRITERIA:
- [ ] `git vendor add` populates all new metadata fields
- [ ] `git vendor sync` updates metadata, preserving vendored_at/vendored_by
- [ ] `git vendor list` displays license and version tag when available
- [ ] `git vendor migrate` updates existing lockfiles
- [ ] Schema version is "1.1"
- [ ] All fields documented in docs/LOCKFILE_SCHEMA.md
- [ ] All existing tests pass
- [ ] Run `go test ./...` and ensure all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 4: SBOM Generation (010)

```
TASK: Implement 010: SBOM Generation

CONTEXT: SBOM requirements are proliferating (EO 14028, DORA, CRA). Teams need SBOMs for vendored code that doesn't appear in package manifests. This is a P0 regulatory requirement.

SPEC: ideas/specs/in-progress/010-sbom-generation.md

DEPENDENCIES: Requires 003 (Metadata Enrichment) to be completed first.

SCOPE:
- Files to modify:
  - main.go (add sbom command)
  - go.mod (add CycloneDX and SPDX dependencies)
- Files to create:
  - internal/core/sbom_generator.go (SBOMGenerator implementation)
  - internal/core/sbom_generator_test.go
- Tests to add:
  - TestGenerateCycloneDX_SingleVendor
  - TestGenerateCycloneDX_MultipleVendors
  - TestGenerateSPDX_ValidOutput
  - TestGetPURL_GitHub, TestGetPURL_GitLab

IMPLEMENTATION GUIDANCE:
1. Add dependencies:
   - github.com/CycloneDX/cyclonedx-go v0.8.0
   - github.com/spdx/tools-golang v0.5.3
2. Create SBOMGenerator struct with lockStore and fs dependencies
3. Implement Generate(format string) method
4. Implement generateCycloneDX(lock) producing valid CycloneDX 1.5 JSON
5. Implement generateSPDX(lock) producing valid SPDX 2.3 JSON
6. Implement getPURL(vendor) for github/gitlab/bitbucket/generic
7. Add `git vendor sbom` command with:
   - --format cyclonedx|spdx (default: cyclonedx)
   - --output file (default: stdout)
8. Map lockfile fields to SBOM fields per spec

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change 010 status to "completed"
2. Move row to ideas/completed.md under "Feature Ideas" section
3. Move spec from ideas/specs/in-progress/ to ideas/specs/complete/
4. Update CLAUDE.md with new sbom command documentation
5. Add to Available Commands in Quick Reference section

ACCEPTANCE CRITERIA:
- [ ] CycloneDX JSON validates against CycloneDX 1.5 schema
- [ ] SPDX JSON validates against SPDX 2.3 schema
- [ ] Every vendored dependency appears as component with name, version, license, hashes
- [ ] PURLs follow Package URL specification
- [ ] --output flag writes to file; without it, writes to stdout
- [ ] All existing tests pass
- [ ] Run `go test ./...` and ensure all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 5: CVE Vulnerability Scanning (011)

```
TASK: Implement 011: CVE/Vulnerability Scanning

CONTEXT: Snyk, Grype, and Trivy miss vendored code that doesn't appear in manifests. git-vendor fills this gap by scanning vendored dependencies against OSV.dev. This is a P0 security requirement.

SPEC: ideas/specs/in-progress/011-cve-vulnerability-scanning.md

DEPENDENCIES: Requires 003 (Metadata Enrichment) to be completed first.

SCOPE:
- Files to modify:
  - main.go (add scan command)
- Files to create:
  - internal/core/vuln_scanner.go (VulnScanner, OSV client, caching)
  - internal/core/vuln_scanner_test.go
- Tests to add:
  - TestQueryOSV_WithKnownCVE
  - TestQueryOSV_NoCVEs
  - TestCVSSToSeverity
  - TestCaching

IMPLEMENTATION GUIDANCE:
1. Create VulnScanner struct with http.Client, cacheDir, cacheTTL
2. Create types: ScanResult, ScanSummary, DependencyScan, Vulnerability
3. Implement queryOSV(dep) using OSV.dev API:
   - POST to https://api.osv.dev/v1/query
   - Try PURL query first, fall back to commit query
4. Implement caching in .git-vendor-cache/osv/ with 24h TTL
5. Implement cvssToSeverity(score) mapping
6. Implement batchQuery for efficiency
7. Add `git vendor scan` command with:
   - --format table|json (default: table)
   - --fail-on critical|high|medium|low
8. Exit codes: 0=PASS, 1=FAIL, 2=WARN

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change 011 status to "completed"
2. Move row to ideas/completed.md under "Feature Ideas" section
3. Move spec from ideas/specs/in-progress/ to ideas/specs/complete/
4. Update CLAUDE.md with new scan command documentation
5. Add to Available Commands in Quick Reference section

ACCEPTANCE CRITERIA:
- [ ] Correctly identifies known CVEs from OSV.dev
- [ ] --fail-on exits with code 1 when threshold exceeded
- [ ] JSON output includes full CVE details
- [ ] Gracefully handles: no network, rate limits, unresolvable packages
- [ ] Results cached for 24 hours
- [ ] Limitations documented in help text
- [ ] All existing tests pass
- [ ] Run `go test ./...` and ensure all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 6: Error Wrapping Consistency (CQ-002)

```
TASK: Fix CQ-002: Error Wrapping Consistency

PROBLEM: Error returns throughout the codebase don't consistently use fmt.Errorf with %w for wrapping. This breaks error chain inspection with errors.Is() and errors.As().

AFFECTED FILES:
Search all .go files in internal/core/ and main.go for patterns like:
- `return err` (bare returns without context)
- `return fmt.Errorf("...", err)` without %w
- `return errors.New(...)` where wrapping would be better

REQUIRED CHANGES:
Transform bare error returns to wrapped errors with context:

Before:
```go
return err
```

After:
```go
return fmt.Errorf("sync vendor %s: %w", name, err)
```

Before:
```go
return fmt.Errorf("failed to load: %v", err)
```

After:
```go
return fmt.Errorf("load config: %w", err)
```

Guidelines:
- Use lowercase error messages (Go convention)
- Include relevant context (operation name, file path, etc.)
- Always use %w for the wrapped error
- Don't wrap if creating a new error (errors.New is fine for new errors)

MANDATORY TRACKING UPDATES:
1. Update ideas/code_quality.md - change CQ-002 status to "completed"
2. Add completion notes under "## Completed Issue Details":
   ```
   ### CQ-002: Error Wrapping Consistency
   - Completed: 2026-02-XX
   - Files updated: [list files]
   - Pattern: All error returns now use fmt.Errorf with %w
   ```

ACCEPTANCE CRITERIA:
- [ ] Zero instances of bare `return err` in core business logic
- [ ] All error wrapping uses %w format verb
- [ ] Error messages follow Go conventions (lowercase, no punctuation)
- [ ] Run `go build ./...` - no compilation errors
- [ ] Run `go test ./...` - all tests pass

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 7: Context Propagation (CQ-003)

```
TASK: Fix CQ-003: Context Propagation

PROBLEM: Long-running operations (git clone, API calls, file operations) don't accept context.Context, preventing proper cancellation and timeout handling.

AFFECTED FILES:
- internal/core/vendor_syncer.go (syncVendor, UpdateAll, etc.)
- internal/core/git_operations.go (Clone, Fetch, ListTree, etc.)
- internal/core/github_client.go (CheckLicense)

REQUIRED CHANGES:
Add context.Context as first parameter to all long-running operations:

Before:
```go
func (g *SystemGitClient) Clone(url, dest string) error {
    return g.run(dest, "clone", url, dest)
}
```

After:
```go
func (g *SystemGitClient) Clone(ctx context.Context, url, dest string) error {
    return g.runWithContext(ctx, dest, "clone", url, dest)
}
```

Implementation steps:
1. Add runWithContext method that uses exec.CommandContext
2. Update GitClient interface with context parameters
3. Update all implementations
4. Update VendorSyncer methods to accept and propagate context
5. Update call sites in main.go to create contexts with timeouts
6. Update mock implementations for testing

MANDATORY TRACKING UPDATES:
1. Update ideas/code_quality.md - change CQ-003 status to "completed"
2. Add completion notes under "## Completed Issue Details":
   ```
   ### CQ-003: Context Propagation
   - Completed: 2026-02-XX
   - Files updated: [list files]
   - Pattern: All long-running operations accept context.Context
   ```

ACCEPTANCE CRITERIA:
- [ ] All GitClient methods accept context.Context as first parameter
- [ ] All VendorSyncer public methods accept context.Context
- [ ] Git commands use exec.CommandContext for cancellation
- [ ] HTTP clients use request.WithContext for API calls
- [ ] Run `go build ./...` - no compilation errors
- [ ] Run `go test ./...` - all tests pass
- [ ] Update mock implementations to match new signatures

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## PROMPT 8: Path Traversal Audit (SEC-001) [CRITICAL]

```
TASK: Audit SEC-001: Path Traversal Security Audit

PRIORITY: CRITICAL - Complete within 24 hours

PROBLEM: ValidateDestPath is the security boundary preventing path traversal attacks. We need to verify it's called before ALL file operations with no bypasses.

AFFECTED FILES:
- internal/core/filesystem.go (ValidateDestPath implementation)
- internal/core/vendor_syncer.go (all file copy operations)
- Any other file that writes to user-specified paths

AUDIT CHECKLIST:
1. Review ValidateDestPath implementation:
   - Rejects absolute paths (/etc/passwd, C:\Windows)
   - Rejects parent directory traversal (../)
   - Only allows relative paths within project
   - Handles edge cases (unicode, encoded chars, symlinks)

2. Find all file write operations:
   ```
   grep -rn "CopyFile\|CopyDir\|WriteFile\|os.Create\|os.OpenFile" internal/
   ```

3. For each write operation, verify ValidateDestPath is called BEFORE the operation

4. Check for bypasses:
   - Direct os.* calls that skip validation
   - Path manipulation after validation
   - Symlink following that escapes project root

REQUIRED ACTIONS:
1. Document every file write location and its validation status
2. Add ValidateDestPath calls where missing
3. Add tests for edge cases if not covered
4. Document findings in security.md

MANDATORY TRACKING UPDATES:
1. Update ideas/security.md - change SEC-001 status to "completed"
2. Add remediation notes under "## Completed Issue Details":
   ```
   ### SEC-001: Path Traversal Audit
   - Completed: 2026-02-XX
   - Audit findings: [summary]
   - Files reviewed: [list]
   - Fixes applied: [list if any]
   - Tests added: [list if any]
   ```

ACCEPTANCE CRITERIA:
- [ ] Every file write operation is preceded by ValidateDestPath
- [ ] No bypasses exist in the codebase
- [ ] Edge cases tested: absolute paths, ../, unicode, symlinks
- [ ] Audit documented with specific file:line references
- [ ] Run `go test ./...` - all tests pass
- [ ] If vulnerabilities found, document and fix immediately

GIT WORKFLOW (MANDATORY - do these steps at the end):
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   git fetch private main
   git merge private/main
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   git push -u private <your-branch-name>
```

---

## Recommended Execution Strategy

### For Maximum Parallelism

**Wave 1 (run 4 agents simultaneously):**
- PROMPT 1: Lockfile Schema Versioning (001)
- PROMPT 2: Verify Command Hardening (002)
- PROMPT 6: Error Wrapping Consistency (CQ-002)
- PROMPT 8: Path Traversal Audit (SEC-001) [CRITICAL]

**Wave 2 (after PROMPT 1 completes):**
- PROMPT 3: Lockfile Metadata Enrichment (003)
- PROMPT 7: Context Propagation (CQ-003) [can run parallel with 003]

**Wave 3 (after PROMPT 3 completes, run 2 agents simultaneously):**
- PROMPT 4: SBOM Generation (010)
- PROMPT 5: CVE Vulnerability Scanning (011)

### Branch Naming Convention

Each agent should work on a unique branch:
- `claude/001-lockfile-schema-<session-id>`
- `claude/002-verify-hardening-<session-id>`
- `claude/003-metadata-enrichment-<session-id>`
- `claude/010-sbom-generation-<session-id>`
- `claude/011-cve-scanning-<session-id>`
- `claude/cq-002-error-wrapping-<session-id>`
- `claude/cq-003-context-propagation-<session-id>`
- `claude/sec-001-path-traversal-<session-id>`

---

## Post-Completion Checklist

After all prompts are executed, verify:

- [ ] All queue.md statuses updated to "completed"
- [ ] All completed items moved to completed.md
- [ ] All specs moved from in-progress/ to complete/
- [ ] CLAUDE.md updated with new commands/features
- [ ] docs/LOCKFILE_SCHEMA.md exists and is complete
- [ ] All tests pass: `go test ./...`
- [ ] No race conditions: `go test -race ./...`
- [ ] Build succeeds: `make build`
