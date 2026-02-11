---
paths:
  - "internal/core/commit_service.go"
  - "internal/core/commit_service_test.go"
---

# COMMIT-SCHEMA v1

The --commit flag creates a single atomic commit with multi-valued trailers and a JSON git note.

## Trailers

One group per vendor, multi-valued:
```text
Vendor-Name: frontend-lib
Vendor-Ref: main
Vendor-Commit: abc123
Vendor-Name: backend-utils
Vendor-Ref: v2.0
Vendor-Commit: def456
```

## Git Notes

JSON note attached under refs/notes/vendor (VendorNoteRef constant) with rich per-vendor metadata: file hashes, URLs, paths. Note attachment is best-effort -- failure does NOT fail the commit.

## Key Functions

- VendorTrailers() -- Build ordered []Trailer from []LockDetails (exported for git-agent composition)
- VendorNoteJSON() -- Build JSON note content
- VendorCommitSubject() -- Format conventional-commits subject line
- CommitVendorChanges() -- Stage all vendor files, create single commit with trailers, attach JSON note
- AnnotateVendorCommit() -- Retroactively attach vendor metadata note to existing commit (no new commit)
- collectVendorPaths() -- Gather all file paths to stage

## Legacy Trap: One-commit-per-vendor

Original design created N commits for N vendors. Replaced with single-commit + multi-valued trailers for atomic semantics. MUST NOT regress.

## Type Conversion Gotcha

types.Trailer and git.Trailer have identical Key/Value fields but are SEPARATE Go types. SystemGitClient.Commit() MUST explicitly convert []types.Trailer -> []git.Trailer. Direct assignment = compile error.

## Behavioral Notes

- --commit ignored during --dry-run (with warning)
- Pre-existing staged files MAY be included in vendor commits (v2: porcelain check)
- annotate command retroactively attaches notes to existing commits (used when humans manually commit vendor changes)
