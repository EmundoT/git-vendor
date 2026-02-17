---
status: pending
assigned: 2026-02-16
updated: 2026-02-16
project: git-vendor
---

# Current Task

Touch trailer support + vendor.update tag

## Objective

Emit Touch trailers on vendor sync commits listing affected code areas. Add structured `vendor.update` tag to vendor commits. Completes ROADMAP Tasks 5 + 8 — the last COMMIT-SCHEMA conformance gaps in git-vendor.

## Acceptance Criteria

- [ ] Vendor sync commits include `Touch:` trailer with affected file areas
- [ ] Vendor commits include `Tags: vendor.update` (or more specific variant)
- [ ] Touch values extracted from vendored file paths (e.g., `pkg.git-plumbing`, `claude.hooks`)
- [ ] Unit tests for Touch extraction from vendor operations
- [ ] Existing vendor commit tests updated to verify new trailers

## Notes

- Previous: Update filter fix (2026-02-16), local path sync (2026-02-13)
- `VendorTrailers()` in `commit_service.go` already emits `Tags: vendor.update` — verify and extend

## Completed

- **Update filter fix** — VendorName/Group filter passthrough in sync paths (2026-02-16)
- **Diff/lock filtering** — Diff ref/group filtering and lock conflict detection (2026-02-16)
- **Local path sync** — `--local` flag for file:// and local paths (2026-02-13)
- **8/8 COMMIT-SCHEMA v1 vendor namespace** — All vendor trailers, hook integration, tags

## Pending Tasks

### E3 (next)
1. **Touch trailer support + vendor.update tag** — ROADMAP Tasks 5 + 8

### Lower priority
2. **Vendor diff preview before sync** — Show what files will change before committing
3. **Lock file conflict resolution** — Guided merge when vendor.lock has conflicts
4. **Multi-remote support** — Vendor from multiple upstream sources
