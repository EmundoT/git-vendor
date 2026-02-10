# Prompt: Set 1 — Housekeeping & Tracking Sync

## Concurrency
SAFE with all other sets. Touches only `ideas/*.md` files — zero overlap with Go source.

## Branch
Create and work on a branch named `claude/set1-tracking-sync-<session-suffix>`.

## Context
The `ideas/` tracking files are significantly out of date. Many completed features, security items, and code quality items still show "pending" in the queues despite being implemented and merged.

## Task
Update ALL `ideas/*.md` tracking files to reflect the actual state of the codebase. This is a documentation-only task — do NOT modify any Go source files.

### Files to update

1. **`ideas/queue.md`** — Update statuses:
   - Spec 071 (Position Extraction): status should be `completed` (implemented, merged, 94 tests in position_extract_test.go)
   - Spec 072 (LLM-Friendly CLI): status should be `completed` (implemented, merged, 53 tests in config_commands_test.go)
   - Verify all Phase 1 and Phase 2 items still show `completed` (they should)

2. **`ideas/security.md`** — Update statuses:
   - SEC-001 (Path Traversal Audit): `completed` — RootedFileSystem with ValidateWritePath in engine.go, comprehensive tests in filesystem_test.go
   - SEC-010 (Git Command Injection): `completed` — All git commands go through git-plumbing, no exec.Command in git-vendor
   - SEC-011 (URL Validation): `completed` — URL validation in security_hardening_test.go
   - SEC-012 (Hook Execution Security): `completed` — sanitizeEnvValue strips newlines/null bytes, documented in CLAUDE.md
   - SEC-013 (Credential Exposure): `completed` — Tests verify tokens not in error messages
   - SEC-020 (YAML Parsing Limits): `completed` — Size limits tested in security_hardening_test.go
   - SEC-021 (Temp Directory Cleanup): `completed` — defer-based cleanup verified
   - SEC-022 (Symlink Handling): `completed` — Symlink detection in security_hardening_test.go
   - SEC-023 (Binary File Detection): `completed` — Null-byte heuristic in position_extract.go
   - SEC-030 (Security Documentation): `completed` — SECURITY.md exists, docs/HOOK_THREAT_MODEL.md created
   - SEC-031 (Dependency Vulnerability Scan): still `pending` (govulncheck not in CI yet)
   - SEC-032 (Release Signing): still `pending`

3. **`ideas/code_quality.md`** — Update statuses:
   - CQ-001 (Interface Mock Coverage): check if all 21 interfaces have mocks — if so, `completed`
   - CQ-002 (Error Wrapping Consistency): `completed` — merged in error context wrapping PR
   - CQ-003 (Context Propagation): `in_progress` → note that Scan() has context, but Sync/Update/Verify/Drift do not yet
   - CQ-005 (TUI Test Coverage): `in_progress` → note coverage is at 63% (up from 9.9%)
   - CQ-006 (Configurable OSV Endpoint): `completed` — GIT_VENDOR_OSV_ENDPOINT env var, 4 tests

4. **`ideas/completed.md`** — Add entries for everything newly completed:
   - Spec 012 (Drift Detection) — completed 2026-02-09
   - Spec 013 (License Policy Enforcement) — completed 2026-02-09
   - Spec 071 (Position Extraction) — completed 2026-02-08
   - Spec 072 (LLM-Friendly CLI) — completed 2026-02-09
   - SEC-001 through SEC-023 and SEC-030 — completed 2026-02-09
   - CQ-002 (Error Wrapping) — completed 2026-02-09
   - CQ-006 (Configurable OSV Endpoint) — completed 2026-02-10
   Follow the existing table format in completed.md.

### Verification
After updating, grep for any remaining `pending` items that should be `completed` by cross-referencing with:
- `git log --oneline -50` (to see what actually merged)
- Existence of test files (e.g., `security_hardening_test.go`, `drift_service_test.go`, `license_policy_test.go`)

### Definition of Done
- All statuses in `ideas/*.md` accurately reflect the codebase
- `ideas/completed.md` has entries for all completed work with dates
- No Go source files modified
- Commit and push
