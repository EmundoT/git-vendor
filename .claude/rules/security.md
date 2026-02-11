---
paths:
  - "internal/core/filesystem.go"
  - "internal/core/git_operations.go"
  - "internal/core/hook_service.go"
  - "internal/core/position_extract.go"
  - "internal/core/github_client.go"
  - "internal/core/vuln_scanner.go"
---

# Security

## Path Traversal Protection (SEC-011)

ValidateDestPath (filesystem.go) is the security boundary:
- Rejects absolute paths (/etc/passwd, C:\Windows\System32)
- Rejects parent directory references (../../../etc/passwd)
- Only allows relative paths within project directory
- Called before ALL file copy operations in vendor_syncer.go

## URL Scheme Validation (SEC-011)

ValidateVendorURL() rejects dangerous schemes: file://, ftp://, javascript:, data:
Allowed: https://, http://, ssh://, git://, git+ssh://, SCP-style (git@host:path), bare hostnames.
Called during config validation.

## Credential Sanitization (SEC-013)

SanitizeURL() strips userinfo from URLs before logging. MUST NOT include raw URLs in error messages when URL may contain embedded credentials.

## YAML Size Limits (SEC-020)

YAMLStore.Load() enforces maxYAMLFileSize (1 MB). Normal configs well under 100 KB. Prevents memory exhaustion.

## Binary File Detection (SEC-023)

IsBinaryContent() (exported) scans first 8000 bytes for null bytes. Matches git's xdl_mmfile_istext heuristic. Used in:
- ExtractPosition: rejects binary files for position extraction
- PlaceContent with position: rejects binary destinations
- Whole-file PlaceContent (nil pos): bypasses check
- Null byte beyond 8000 bytes is NOT detected

## Hook Threat Model (SEC-012)

See docs/HOOK_THREAT_MODEL.md for full analysis. Key points:
- Hooks execute arbitrary shell commands BY DESIGN (same trust model as npm scripts)
- 5-minute timeout via context.WithTimeout prevents hangs
- Env var values sanitized (newlines/null bytes stripped)
- Runs in project root with current user permissions
- No sandboxing or privilege restriction
- Cross-platform: sh -c (Unix), cmd /c (Windows)

## Symlink Handling (SEC-022)

CopyDir uses filepath.Walk (os.Lstat, does not follow symlinks). Directory symlinks cause error. File symlinks ARE dereferenced (content copied). Intentional safe behavior.

## OSV.dev Integration

- Response bodies limited to 10 MB via io.LimitReader
- Rate limit (429), server error (5xx), client error (4xx) produce distinct OSVAPIError types
- GIT_VENDOR_OSV_ENDPOINT env var allows air-gapped proxy deployment
