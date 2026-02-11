---
paths:
  - "internal/core/commit_service.go"
  - "internal/core/commit_service_test.go"
  - "internal/core/git_operations.go"
---

# Vendor Commit Infrastructure

## Overview

The --commit flag on sync/update creates a single atomic commit with structured metadata. Two complementary systems carry vendor provenance:

1. **Trailers** — human-readable, appended to commit message. Write-only metadata for `git log` readability.
2. **Git notes** — machine-readable JSON under `refs/notes/vendor`. Durable source of truth for programmatic queries.

Trailers are lossy when read back. Notes are the canonical multi-vendor metadata store.

## COMMIT-SCHEMA v1

### Trailers (Write-Only)

One group per vendor, multi-valued:
```text
Vendor-Name: frontend-lib
Vendor-Ref: main
Vendor-Commit: abc123
Vendor-Name: backend-utils
Vendor-Ref: v2.0
Vendor-Commit: def456
```

**Trailer reading is lossy**: git-plumbing parses trailers into `map[string]string` (via `parseTrailers()` in log.go). Multi-valued keys like `Vendor-Name` appearing twice collapse to last-wins. Order is lost. MUST NOT rely on `Commit.Trailers` to recover multi-vendor groups. Use notes JSON instead.

### Git Notes (Source of Truth)

JSON note attached under `refs/notes/vendor` (VendorNoteRef constant). Contains rich per-vendor metadata: file hashes, URLs, paths, timestamps. Note attachment is best-effort — failure does NOT fail the commit.

## Git-Plumbing Notes API

git-plumbing exposes full note lifecycle:

| Method | Behavior |
|--------|----------|
| `AddNote(ctx, ref, commitHash, content)` | Add or **overwrite** note (forced, -f flag). Idempotent. |
| `GetNote(ctx, ref, commitHash)` | Retrieve note content. Returns `GitError` if not found (not nil, not empty). |
| `RemoveNote(ctx, ref, commitHash)` | Delete note from commit. |
| `ListNotes(ctx, ref)` | List all note-to-commit mappings under namespace. Returns `[]NoteEntry{NoteHash, CommitHash}`. |

Key behaviors:
- Notes are **per-commit, per-namespace**. One note per NoteRef per commit.
- AddNote **overwrites** previous notes (no merge). This is by design for idempotent retries.
- Namespaces are independent: vendor (`refs/notes/vendor`), agent, review can coexist on the same commit without interference.
- GetNote throws on missing note — use `errors.As(&gitErr)` for type checking.

## Git-Plumbing Tag System

| Method | Behavior |
|--------|----------|
| `TagsAt(ctx, commitHash)` | Get all tags pointing at a commit |
| `CreateTag(ctx, name)` | Create **lightweight** tag at HEAD (no annotated tag API) |
| `DeleteTag(ctx, name)` | Remove a tag |
| `ListTags(ctx, pattern)` | List tags matching pattern, sorted newest-first |

Tag handling in git-vendor:
- `GetTagForCommit()` wraps `TagsAt()` then applies `isSemverTag()` preference — semver tags (v1.0.0) win over arbitrary names
- Semver preference is **client-side filtering**, not git-plumbing behavior
- `SourceVersionTag` in LockDetails stores the matched tag (schema v1.1)

## Querying Vendor Commits

git-plumbing `LogOpts` supports filtering vendor commits:
- `TrailerFilter: map[string]string` — Filter by trailer key=value (e.g., `{"Commit-Schema": "vendor/v1"}`)
- `NotesRef: NoteRef` — Include notes in query output (`Commit.Notes` field)
- `IncludeBody: true` — Include full commit body (safe with multi-line; uses `\x1e` delimiters)

**Pattern for reading vendor metadata**:
1. Query with TrailerFilter to find vendor commits
2. Read `Commit.Notes` (JSON) for multi-vendor metadata — this is reliable
3. Do NOT reconstruct vendor groups from `Commit.Trailers` map — it's lossy

## Type Conversion Adapter

git-plumbing uses its own types. SystemGitClient converts at the boundary:

| git-plumbing | git-vendor | Conversion site |
|---|---|---|
| `git.Trailer` | `types.Trailer` | `SystemGitClient.Commit()` — explicit loop |
| `git.Commit` | `types.CommitInfo` | `GetCommitLog()` — date formatting |
| `git.NoteRef` | string constant | `VendorNoteRef` cast to `git.NoteRef` |

Rule: When extending git-vendor to use new git-plumbing types (e.g., `git.NoteEntry`), create a corresponding `types.*` wrapper and convert in SystemGitClient. Do NOT expose git-plumbing types in core service APIs.

## Environment Safety

git-plumbing sanitizes GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE, etc. before running git commands. Vendor commits are safe to create inside git hooks and subprocesses — no environment leakage from parent process.

## Key Functions

- `VendorTrailers()` — Build ordered []Trailer from []LockDetails (exported for composition)
- `VendorNoteJSON()` — Build JSON note content with per-vendor metadata
- `VendorCommitSubject()` — Format conventional-commits subject line
- `CommitVendorChanges()` — Stage files, create single commit with trailers, attach note
- `AnnotateVendorCommit()` — Retroactively attach vendor metadata note to existing commit (no new commit created)
- `collectVendorPaths()` — Gather all file paths to stage
- `GetTagForCommit()` — Read tags at commit with semver preference

## Legacy Trap: One-commit-per-vendor

Original design created N commits for N vendors. Replaced with single-commit + multi-valued trailers for atomic semantics. MUST NOT regress.

## Behavioral Notes

- --commit ignored during --dry-run (with warning)
- Pre-existing staged files MAY be included in vendor commits (v2: porcelain check)
- `annotate` command retroactively attaches notes to existing commits (for humans who manually commit vendor changes)
- HEAD resolution happens after commit creation — `GetHeadHash()` called post-`Commit()` to read the new SHA
- Verbose flag (`Git.Verbose=true`) propagates through all git operations for debugging
