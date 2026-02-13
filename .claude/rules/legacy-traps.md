---
paths:
  - "internal/**/*.go"
---

# Legacy Traps

Rejected approaches and deferred decisions. Prevents re-proposing ideas that were already evaluated.

For runtime gotchas, see CLAUDE.md "Essential Gotchas". For position-specific edge cases, see `position-extraction.md`. For security constraints, see `security.md`.

## Rejected Approaches

### net/http.DetectContentType for binary detection
Rejected in favor of git's null-byte heuristic (scan first 8000 bytes for \x00). DetectContentType only inspects 512 bytes and can misclassify source code as application/octet-stream.

### One-commit-per-vendor
Original design created N commits for N vendors. Replaced with single atomic commit + multi-valued trailers. MUST NOT regress.

### Transform pipeline for internal vendors (Spec 070)
REJECTED: Transforms (extract-section, embed-json, template rendering) are out of scope. Internal vendors support exact file copy and position extraction/placement only.

### Parallel sync for internal vendors
REJECTED: Internal vendors sync sequentially even with `--parallel`. Internal mappings may share destination files; concurrent writes would race.

### Full compliance enforcement modes (Spec 075)
DEFERRED: Spec 070 implements drift detection and propagation only. Strict/lenient/info enforcement levels, CI exit codes, and policy-based compliance gating are deferred to Spec 075.
