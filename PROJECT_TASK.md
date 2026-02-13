---
status: pending
assigned: 2026-02-13
updated: 2026-02-13
project: git-vendor
---

# Current Task

Local path sync (`--local` flag)

## Objective

Bypass SEC-011 URL validation to allow `file://` or relative paths for local-first workflows. Unblocks private-branch workflows where remote URLs don't reflect the developer's working state. The flag is opt-in — default behavior unchanged.

## Acceptance Criteria

- [ ] `git-vendor sync --local` accepts `file://` and relative paths in vendor.yml
- [ ] Relative paths resolved against the project root (location of vendor.yml)
- [ ] SEC-011 validation skipped only when `--local` flag is present
- [ ] `git-vendor update --local` also supports local paths
- [ ] Error message when using local paths without `--local` explains the flag
- [ ] Unit tests: relative path resolution, file:// URL handling, flag-absent rejection
- [ ] Integration test: vendor from a sibling directory on the local filesystem

## Notes

- Previous task "`git-vendor outdated` command" completed 2026-02-13.
- SEC-011 (ValidateVendorURL in internal/core/git_operations.go) currently blocks `file://`. The `--local` flag should bypass this specific check only, not other URL validation.

## Pending Tasks

1. **Fix `update` positional vendor filter** — `git-vendor update <name>` updates ALL vendors instead of just the named one. The positional argument is accepted but ignored. Likely a bug in `main.go` or the update handler where the filter isn't passed through to the sync service.
2. **Touch trailer support** — ROADMAP Task 5: emit Touch trailers on vendor sync commits listing affected code areas
3. **vendor.update tag** — ROADMAP Task 8: structured tag for vendor update commits
4. **Vendor diff preview before sync** — Show what files will change before committing a sync operation
5. **Lock file conflict resolution** — Guided merge when vendor.lock has conflicts from concurrent changes
6. **Multi-remote support** — Vendor from multiple upstream sources into the same project
