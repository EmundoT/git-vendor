# SEC-001: Path Traversal Security Audit

## Priority: CRITICAL

## Task

Audit and verify that `ValidateDestPath` is called before ALL file operations with no bypasses.

## Problem

`ValidateDestPath` is the security boundary preventing path traversal attacks. If bypassed, an attacker could:
- Write to arbitrary filesystem locations (`/etc/passwd`, `~/.ssh/authorized_keys`)
- Overwrite system files via `../../../` traversal
- Escape the project directory

## Audit Scope

### 1. Review ValidateDestPath Implementation

File: `internal/core/filesystem.go`

Verify it correctly:
- Rejects absolute paths (`/etc/passwd`, `C:\Windows`)
- Rejects parent directory traversal (`../`, `..\\`)
- Rejects encoded variants (`%2e%2e%2f`, unicode normalization attacks)
- Only allows relative paths within project root
- Handles symlinks (doesn't follow symlinks that escape project)

### 2. Find All File Write Operations

```bash
grep -rn "CopyFile\|CopyDir\|WriteFile\|os.Create\|os.OpenFile\|os.MkdirAll\|os.Mkdir" internal/
```

For each operation found:
- Document the file:line location
- Verify `ValidateDestPath` is called BEFORE the write
- Check the call chain to ensure validation isn't bypassed

### 3. Check for Bypasses

- Direct `os.*` calls that skip the FileSystem interface
- Path manipulation AFTER validation (validation then modification = bypass)
- Symlink following that escapes project root
- Time-of-check-time-of-use (TOCTOU) races

## Required Deliverables

1. **Audit Document**: List every file write location with validation status
2. **Fixes**: Add `ValidateDestPath` calls where missing
3. **Tests**: Add edge case tests if not covered:
   - Absolute paths: `/etc/passwd`, `C:\Windows\System32`
   - Traversal: `../../../etc/passwd`, `foo/../../../bar`
   - Encoded: `%2e%2e%2f`, `..%2f`, unicode variants
   - Symlinks: symlink pointing outside project
4. **Documentation**: Update security.md with findings

## Acceptance Criteria

- [ ] Every file write operation preceded by ValidateDestPath (or proven safe)
- [ ] No bypasses exist in the codebase
- [ ] Edge cases tested: absolute paths, ../, encoded chars, symlinks
- [ ] Audit documented with specific file:line references
- [ ] Run `go test ./...` - all tests pass

## Tracking Updates

1. Update `ideas/security.md` - change SEC-001 status to "completed"
2. Add remediation notes under "## Completed Issue Details":
   ```
   ### SEC-001: Path Traversal Audit
   - Completed: YYYY-MM-DD
   - Files reviewed: [list]
   - Findings: [summary]
   - Fixes applied: [list if any]
   - Tests added: [list if any]
   ```

## Git Workflow

1. Commit changes with descriptive message
2. Merge from main: `git fetch origin main && git merge origin/main`
3. Resolve any conflicts
4. Push: `git push -u origin <branch-name>`
