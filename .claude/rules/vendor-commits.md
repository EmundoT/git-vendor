---
paths:
  - "internal/core/commit_service.go"
  - "internal/core/commit_service_test.go"
  - "internal/core/git_operations.go"
---

# vendor/v1 Namespace (COMMIT-SCHEMA v1 Delta)

This rule covers git-vendor's extensions to the base COMMIT-SCHEMA v1 protocol. For subject line format, Tags/Touch/Diff enrichment, tag grammar, trailer API, and hook flow, see the git-plumbing `commit-schema` skill.

## Vendor-Specific Trailers

Multi-valued, positionally associated (Nth Name = Nth Ref = Nth Commit):

```text
Commit-Schema: vendor/v1
Vendor-Name: frontend-lib
Vendor-Ref: main
Vendor-Commit: abc123def456...
Vendor-Name: backend-utils
Vendor-Ref: v2.0
Vendor-Commit: def456abc789...
```

## Git Notes (vendor/v1 addition)

JSON note attached under `refs/notes/vendor` (VendorNoteRef constant). Contains per-vendor metadata: file hashes, URLs, paths, timestamps. This is the machine-readable source of truth — trailers are human-readable only.

Note attachment is best-effort — failure does NOT fail the commit.

## Key Functions

- `VendorTrailers()` — Build ordered []Trailer from []LockDetails (exported for composition)
- `VendorNoteJSON()` — Build JSON note content
- `VendorCommitSubject()` — Format subject per COMMIT-SCHEMA v1 spec
- `CommitVendorChanges()` — Stage files, single commit with trailers + note
- `AnnotateVendorCommit()` — Retroactively attach note to existing commit (no new commit)
- `collectVendorPaths()` — Gather file paths to stage
- `GetTagForCommit()` — Read tags with semver preference (client-side filtering)

## Type Conversion Gotcha

`types.Trailer` and `git.Trailer` are separate Go types with identical fields. `SystemGitClient.Commit()` MUST explicitly convert `[]types.Trailer` -> `[]git.Trailer`. Direct assignment = compile error. Same boundary rule applies to any new git-plumbing types.

## Legacy Trap: One-commit-per-vendor

Original design created N commits for N vendors. Replaced with single atomic commit + multi-valued trailers. MUST NOT regress.

## Behavioral Notes

- `--commit` ignored during `--dry-run` (with warning)
- Pre-existing staged files MAY be included (v2: porcelain check)
- `annotate` retroactively attaches notes (for humans who manually committed vendor changes)
- HEAD resolution is post-commit — `GetHeadHash()` reads the new SHA after `Commit()`
