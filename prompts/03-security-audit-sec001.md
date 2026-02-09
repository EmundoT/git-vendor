## Task: Security Audit SEC-001 — Path Traversal Verification

Audit ALL file I/O code paths to verify `ValidateDestPath` is called before every
file write operation. Produce a findings report and add missing test cases.

### Constraints
- ONLY modify `internal/core/filesystem_test.go` (add tests)
- DO NOT modify any .go implementation source files
- DO NOT touch `git_operations.go`, `engine.go`, `go.mod`, `go.sum` (migration in progress)
- Generate mocks first: `make mocks`
- This is an AUDIT — document findings, add tests, flag gaps

### Audit checklist

**1. Trace all file write paths:**
Read these files and trace every `os.Create`, `os.WriteFile`, `os.MkdirAll`, `CopyFile`,
`CopyDir`, `PlaceContent` call:
- `internal/core/filesystem.go` — CopyFile, CopyDir, ValidateDestPath
- `internal/core/file_copy_service.go` — CopyMappings, copyWithPosition
- `internal/core/position_extract.go` — PlaceContent
- `internal/core/sync_service.go` — syncRef
- `internal/core/update_service.go` — computeFileHashes
- `internal/core/vendor_syncer.go` — license file writes
- `internal/core/hook_service.go` — hook execution (command injection vector?)

**2. For each write path, verify:**
- Is the destination path validated via `ValidateDestPath` BEFORE the write?
- Can a malicious `vendor.yml` entry with `to: "../../../etc/passwd"` reach the write?
- Can position specifiers bypass validation? (e.g., `to: "../etc/passwd:L1"`)
- Are symlinks followed? (symlink in vendor dir pointing outside project)

**3. Add test cases to filesystem_test.go:**
- Path traversal via `..` sequences (already exists? verify completeness)
- Absolute path rejection on Linux (`/etc/passwd`) and Windows (`C:\Windows`)
- URL-encoded traversal: `%2e%2e%2f` sequences
- Null byte injection: `file.go\x00.txt`
- Unicode normalization attacks if applicable
- Position-specifier-prefixed traversal: `../etc/passwd:L1-L5`
- Symlink following (if applicable to CopyFile/CopyDir)

**4. Produce findings:**
Write a summary at the top of your commit message listing:
- PASS: paths where ValidateDestPath is correctly called
- FAIL: paths where validation is missing or bypassable
- N/A: paths that don't involve user-controlled destinations

### Definition of Done
1. All new tests in `filesystem_test.go` pass
2. `make lint` is green
3. Commit message contains the audit summary
4. Any FAIL findings are clearly documented (we will fix in a follow-up)
