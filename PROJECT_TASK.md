---
status: complete
assigned: 2026-02-12
updated: 2026-02-13
project: git-vendor
---

# Current Task

Smoother init UX

## Objective

Auto-detect remote URL, suggest ecosystem bootstrap, and display next steps message after git-vendor init

## Acceptance Criteria

- [x] Remote URL auto-detection implemented
- [x] Ecosystem bootstrap suggestion added
- [x] Next steps message displayed after init
- [x] Existing init behavior preserved as fallback

## Notes

- 2026-02-13: Remote detection, next-steps, JSON/quiet output landed in 391a625.
- 2026-02-13: Added ecosystem bootstrap suggestion — detects .githooks/ and .git-vendor-policy.yml presence. Shows actionable bootstrap commands when absent; confirms detection when present. Added HasHooks/HasPolicy to InitSummary struct, JSON output, and TUI display. 15 new tests (9 PrintInitSummary + 6 GetRemoteURL).

## Pending Tasks

1. **`git-vendor outdated` command** — CLI command comparing lockfile hashes against upstream HEAD for each dependency. Exits non-zero when stale. Promotes sync-check.sh logic (currently bash in git-ecosystem) into a universal, CI-friendly feature any git-vendor consumer can use.
2. **Local path sync (`--local` flag)** — Bypass SEC-011 URL validation to allow `file://` or relative paths for local-first workflows. Unblocks private-branch workflows where remote URLs don't reflect working state (~50-80 lines across 4 files)
3. **Touch trailer support** — ROADMAP Task 5: emit Touch trailers on vendor sync commits listing affected code areas
4. **vendor.update tag** — ROADMAP Task 8: structured tag for vendor update commits
5. **Vendor diff preview before sync** — Show what files will change before committing a sync operation
6. **Lock file conflict resolution** — Guided merge when vendor.lock has conflicts from concurrent changes
7. **Multi-remote support** — Vendor from multiple upstream sources into the same project
