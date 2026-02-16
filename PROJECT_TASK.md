---
status: complete
assigned: 2026-02-13
updated: 2026-02-15
project: git-vendor
---

# Current Task

None — select from Pending Tasks below.

## Completed

- **Local path sync (`--local` flag)** — completed 2026-02-13 (commit `73e1dd6`). `sync --local` and `update --local` accept `file://`, relative, and absolute filesystem paths. `IsLocalPath()` detects local URLs; `ResolveLocalURL()` resolves relative paths against project root. Without `--local`, returns error with hint.

## Pending Tasks

### Batch A (parallel with git-plumbing validate ergonomics + git-agent interactive mode)
1. **Fix `update` positional vendor filter** — `git-vendor update <name>` updates ALL vendors instead of just the named one. The positional argument is accepted but ignored. Likely a bug in `main.go` or the update handler where the filter isn't passed through to the sync service.

### Batch B (parallel pair — both are COMMIT-SCHEMA trailer enrichment)
2. **Touch trailer support** — ROADMAP Task 5: emit Touch trailers on vendor sync commits listing affected code areas
3. **vendor.update tag** — ROADMAP Task 8: structured tag for vendor update commits

### Batch C (independent, lower priority)
4. **Vendor diff preview before sync** — Show what files will change before committing a sync operation
5. **Lock file conflict resolution** — Guided merge when vendor.lock has conflicts from concurrent changes
6. **Multi-remote support** — Vendor from multiple upstream sources into the same project
