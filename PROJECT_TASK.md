---
status: complete
assigned: 2026-02-17
updated: 2026-02-17
project: git-vendor
---

# Current Task

Vendor diff preview before sync

## Objective

Show what files will change before committing a vendor sync — a dry-run preview that lets users inspect incoming changes and abort if unexpected.

## Acceptance Criteria

- [x] `sync --dry-run` (or `sync --preview`) shows files that would be added, modified, or deleted
- [x] Output includes per-file diff summary (added/removed lines or at minimum changed/unchanged status)
- [x] No filesystem mutations occur during preview
- [x] Works with `--group` and vendor name filters
- [x] Unit tests for preview logic

## Notes

- 2026-02-17: Promoted from pending. Previous task (Touch trailer + vendor.update tag) confirmed complete — all acceptance criteria met in commit a8fdf78.
- 2026-02-17: Implemented enriched `sync --dry-run`. Unified through `SyncWithFullOptions` (deleted separate `SyncDryRun` codepath). `previewSyncVendor` now classifies each mapping as [add]/[update]/[unchanged] via fs.Stat + lock hash comparison. Per-ref summary counts. All filters (--group, vendor name, --internal) work via shared `shouldSyncVendor`. 7 new tests + 3 updated existing tests. Build/test/vet green.

## Completed

- **Touch trailer support + vendor.update tag** — ROADMAP Tasks 5 + 8 (2026-02-17)
- **Update filter fix** — VendorName/Group filter passthrough in sync paths (2026-02-16)
- **Diff/lock filtering** — Diff ref/group filtering and lock conflict detection (2026-02-16)
- **Local path sync** — `--local` flag for file:// and local paths (2026-02-13)
- **8/8 COMMIT-SCHEMA v1 vendor namespace** — All vendor trailers, hook integration, tags

## Pending Tasks

### E3 (next)
1. **Vendor diff preview before sync** — Show what files will change before committing

### Lower priority
2. **Lock file conflict resolution** — Guided merge when vendor.lock has conflicts
3. **Multi-remote support** — Vendor from multiple upstream sources
