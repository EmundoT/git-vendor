# Security Issues Queue

> Security vulnerabilities and hardening tasks for git-vendor. Items with specs have detailed implementation docs in `ideas/specs/security/`.

## CRITICAL Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-001 | completed | Path Traversal Audit | RootedFileSystem with ValidateWritePath in engine.go, comprehensive tests in filesystem_test.go | - |

## HIGH Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-010 | completed | Git Command Injection Review | All git commands delegated to git-plumbing, no exec.Command in git-vendor | - |
| SEC-011 | completed | URL Validation Hardening | ValidateVendorURL rejects file://, ftp://, javascript:, data: schemes; tests in security_hardening_test.go | - |
| SEC-012 | completed | Hook Execution Security | sanitizeEnvValue strips newlines/null bytes, 5-min timeout, documented in CLAUDE.md and docs/HOOK_THREAT_MODEL.md | - |
| SEC-013 | completed | Credential Exposure Prevention | SanitizeURL strips credentials; tests verify tokens not in error messages | - |

## MEDIUM Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-020 | completed | YAML Parsing Limits | 1 MB size limit (maxYAMLFileSize) in YAMLStore.Load; tests in security_hardening_test.go | - |
| SEC-021 | completed | Temp Directory Cleanup | defer-based cleanup verified in security_hardening_test.go | - |
| SEC-022 | completed | Symlink Handling | Symlink detection in security_hardening_test.go; CopyDir does not follow directory symlinks | - |
| SEC-023 | completed | Binary File Detection | IsBinaryContent exported, null-byte heuristic (first 8000 bytes) in position_extract.go | - |

## LOW Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| SEC-030 | completed | Security Documentation | SECURITY.md at project root, docs/HOOK_THREAT_MODEL.md created | - |
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

**Current Status:** Completed. RootedFileSystem with ValidateWritePath enforces path safety. Comprehensive tests in filesystem_test.go and security_hardening_test.go.

### 2. Git Operations (git_operations.go)

```go
// Git commands must not be injectable
// - URL validation before use
// - Ref/branch validation (no shell metacharacters)
// - All commands delegated to git-plumbing (no exec.Command)
```

**Current Status:** Completed. All git operations delegated to git-plumbing. No direct exec.Command calls in git-vendor.

### 3. Hook Execution (hook_service.go)

```go
// Hooks execute arbitrary shell commands
// - Runs with user's permissions (acceptable - same as npm scripts)
// - 5-minute timeout prevents hangs
// - Environment variables sanitized (newlines/null bytes stripped)
// - Documented in docs/HOOK_THREAT_MODEL.md
```

**Current Status:** Completed. sanitizeEnvValue implemented, timeout enforced, threat model documented.

### 4. YAML Parsing (config_store.go, lock_store.go)

```go
// YAML input protection
// - 1 MB file size limit (maxYAMLFileSize)
// - Structure validation after parsing
// - Standard gopkg.in/yaml.v3 with size guard
```

**Current Status:** Completed. Size limits tested in security_hardening_test.go.

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

| ID | Completed | Notes |
|----|-----------|-------|
| SEC-001 | 2026-02-09 | RootedFileSystem with ValidateWritePath, comprehensive tests |
| SEC-010 | 2026-02-09 | All git operations via git-plumbing, no exec.Command |
| SEC-011 | 2026-02-09 | ValidateVendorURL rejects dangerous schemes |
| SEC-012 | 2026-02-09 | sanitizeEnvValue, 5-min timeout, HOOK_THREAT_MODEL.md |
| SEC-013 | 2026-02-09 | SanitizeURL strips credentials from error messages |
| SEC-020 | 2026-02-09 | 1 MB YAML size limit enforced |
| SEC-021 | 2026-02-09 | defer-based temp dir cleanup verified |
| SEC-022 | 2026-02-09 | Symlink detection, CopyDir does not follow dir symlinks |
| SEC-023 | 2026-02-09 | IsBinaryContent exported, null-byte heuristic |
| SEC-030 | 2026-02-09 | SECURITY.md and HOOK_THREAT_MODEL.md created |
