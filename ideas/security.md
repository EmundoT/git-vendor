# Security Issues Queue

> Security vulnerabilities and hardening tasks for git-vendor. Items with specs have detailed implementation docs in `ideas/specs/security/`.

## CRITICAL Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-001 | pending | Path Traversal Audit | Verify ValidateDestPath called before ALL file operations, no bypasses | - |

## HIGH Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-010 | pending | Git Command Injection Review | Audit all exec.Command calls for injection vulnerabilities in git_operations.go | - |
| SEC-011 | pending | URL Validation Hardening | Ensure URL parsing rejects malicious URLs (file://, ftp://, etc.) | - |
| SEC-012 | pending | Hook Execution Security | Document security model for hook execution, ensure no env var injection | - |
| SEC-013 | pending | Credential Exposure Prevention | Ensure tokens/passwords never logged, printed, or included in errors | - |

## MEDIUM Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-020 | pending | YAML Parsing Limits | Add size limits and structure validation for vendor.yml parsing | - |
| SEC-021 | pending | Temp Directory Cleanup | Ensure all temp directories are cleaned up even on error/panic | - |
| SEC-022 | pending | Symlink Handling | Verify symlinks in vendored content are handled safely | - |
| SEC-023 | pending | Binary File Detection | Warn or block vendoring of binary/executable files | - |

## LOW Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-030 | pending | Security Documentation | Create docs/SECURITY.md with threat model and security guarantees | - |
| SEC-031 | pending | Dependency Vulnerability Scan | Set up govulncheck in CI for dependency scanning | - |
| SEC-032 | pending | Release Signing | Sign release binaries with GPG or Sigstore | - |

---

## Security Hotspots

Per CLAUDE.md and codebase analysis, these areas require ongoing vigilance:

### 1. Path Validation (filesystem.go)

```go
// ValidateDestPath MUST be called before ALL file operations
// - Rejects absolute paths
// - Rejects parent directory traversal (../)
// - Only allows relative paths within project
```

**Current Status:** Implemented, needs audit for complete coverage.

### 2. Git Operations (git_operations.go)

```go
// Git commands must not be injectable
// - URL validation before use
// - Ref/branch validation (no shell metacharacters)
// - Use array-based exec.Command, not string concatenation
```

**Current Status:** Uses exec.Command with explicit args, needs review.

### 3. Hook Execution (hook_service.go)

```go
// Hooks execute arbitrary shell commands
// - Runs with user's permissions (acceptable - same as npm scripts)
// - Documents that users control vendor.yml
// - Ensures working directory is project root
```

**Current Status:** Documented in CLAUDE.md, consistent with npm/make model.

### 4. YAML Parsing (config_store.go, lock_store.go)

```go
// YAML can be attack vector
// - Consider input size limits
// - Validate structure after parsing
// - Reject unexpected types
```

**Current Status:** Uses standard gopkg.in/yaml.v3, no custom limits.

---

## Severity Definitions

| Severity | SLA | Criteria |
|----------|-----|----------|
| **CRITICAL** | 24 hours | Active exploitation possible, arbitrary code execution |
| **HIGH** | 72 hours | Significant vulnerability requiring specific conditions |
| **MEDIUM** | 1 week | Limited impact, defense in depth issue |
| **LOW** | 2 weeks | Best practice violation, hardening opportunity |

---

## Completed Issue Details

*No issues completed yet. This section will track remediation notes.*
