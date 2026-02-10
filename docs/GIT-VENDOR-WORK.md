# git-vendor Work Plan

Implementation spec for git-vendor's COMMIT-SCHEMA v1 conformance.
This document is self-contained.

---

## Context

git-vendor is a dependency vendoring tool. It copies external
dependencies into a project's `.git-vendor/` directory and can
create git commits that track what was vendored, at what version,
from what commit.

The COMMIT-SCHEMA v1 protocol defines a `vendor/v1` namespace with
structured trailers on vendor commits. This work implements:
1. Single commit with multi-valued trailers (one group per vendor)
2. Git notes under `refs/notes/vendor` for rich metadata
3. Retroactive annotation via `git vendor annotate`

### Relationship to the Ecosystem

```text
git-plumbing  -> primitives ([]Trailer, notes, custom refs, hooks)
git-agent     -> LLM/agent commits (agent/v1 namespace)
git-vendor    -> dependency vendoring (vendor/v1 namespace) <- THIS
```

### Design Principles

1. **Single commit per operation** — N vendors in one atomic commit
2. **Multi-valued trailers** — repeated Vendor-Name/Ref/Commit keys
3. **Git notes for rich data** — file hashes, URLs, paths in JSON
4. **Retroactive annotation** — humans commit, then annotate
5. **Composable exports** — `VendorTrailers()` for git-agent merging
6. **Best-effort notes** — note failure does not fail commit

---

## Architecture

### Commit Structure

A `git vendor sync --commit` or `git vendor update --commit` with
two vendors produces:

```text
chore(vendor): sync 2 vendors

Commit-Schema: vendor/v1
Vendor-Name: lib-a
Vendor-Ref: main
Vendor-Commit: aaaa000000000000000000000000000000000000
Vendor-Name: lib-b
Vendor-Ref: v2
Vendor-Commit: bbbb000000000000000000000000000000000000
Vendor-License: MIT
```

**Key**: `Commit-Schema` appears once. `Vendor-Name`, `Vendor-Ref`,
`Vendor-Commit` repeat per vendor (Nth occurrence = Nth vendor).
Optional trailers (`Vendor-License`, `Vendor-Source-Tag`) appear
per-vendor only when non-empty.

### Git Note Structure

After the commit, a JSON note is attached under `refs/notes/vendor`:

```json
{
  "schema": "vendor/v1",
  "vendors": [
    {
      "name": "lib-a",
      "url": "https://github.com/owner/lib-a",
      "ref": "main",
      "commit_hash": "aaaa000000000000000000000000000000000000",
      "file_hashes": { "vendor/a.go": "sha256:..." },
      "paths": ["vendor/a.go"]
    },
    {
      "name": "lib-b",
      "url": "https://github.com/owner/lib-b",
      "ref": "v2",
      "commit_hash": "bbbb000000000000000000000000000000000000",
      "license_spdx": "MIT",
      "paths": ["vendor/b.go"]
    }
  ],
  "created": "2026-02-10T12:00:00Z"
}
```

Notes provide data too verbose for trailers: file hashes, full URLs,
destination paths. Tools query notes via `git notes --ref=refs/notes/vendor show <commit>`.

### Retroactive Annotation

When a human manually commits vendor changes:

```bash
git add vendor/ .git-vendor/
git commit -m "update lib-a"
git vendor annotate           # annotates HEAD
git vendor annotate abc123    # annotates specific commit
git vendor annotate --vendor lib-a  # annotates for one vendor only
```

`annotate` attaches the same JSON note structure but does not create
a new commit. It reads the current lockfile state.

---

## Data Flow

```text
vendor.yml + vendor.lock
       |
       v
VendorTrailers([]LockDetails) -> []types.Trailer (ordered, multi-valued)
VendorCommitSubject([]LockDetails, op) -> "chore(vendor): sync lib to main"
VendorNoteJSON([]LockDetails, specMap) -> JSON string
       |
       v
CommitVendorChanges():
  1. Load config + lock
  2. Filter by vendorFilter
  3. Collect all dest paths
  4. git add (one call)
  5. git commit (one call, multi-valued trailers)
  6. git notes add (best-effort)
```

### Lock Field to Trailer Mapping

| Lock Field | Trailer | Required | Example |
|---|---|---|---|
| `name` | `Vendor-Name` | REQUIRED | `git-plumbing` |
| `ref` | `Vendor-Ref` | REQUIRED | `main` |
| `commit_hash` | `Vendor-Commit` | REQUIRED | full 40-char SHA |
| `license_spdx` | `Vendor-License` | OPTIONAL | `MIT` |
| `source_version_tag` | `Vendor-Source-Tag` | OPTIONAL | `v1.2.3` |

---

## Implementation Files

| File | Role |
|------|------|
| `internal/core/commit_service.go` | VendorTrailers, VendorCommitSubject, VendorNoteJSON, CommitVendorChanges, AnnotateVendorCommit |
| `internal/core/commit_service_test.go` | 24 tests: pure function + mock-based orchestration |
| `internal/core/git_operations.go` | GitClient interface (Add, Commit, AddNote, GetNote) + SystemGitClient |
| `internal/types/types.go` | Trailer struct, CommitOptions with []Trailer |
| `internal/core/engine.go` | Manager.CommitVendorChanges(), Manager.AnnotateVendorCommit() |
| `main.go` | --commit flag on sync/update, annotate command |
| `pkg/git-plumbing/commit.go` | []Trailer-based CommitOpts (foundation change) |
| `pkg/git-plumbing/notes.go` | AddNote, GetNote, RemoveNote, ListNotes |

---

## Type Design

### []Trailer (not map[string]string)

The foundational change: `CommitOpts.Trailers` is `[]Trailer` not
`map[string]string`. This enables:
- Multi-valued keys (multiple Vendor-Name entries)
- Preserved insertion order
- Multi-namespace composition (Commit-Schema appears twice for agent+vendor)

Both `types.Trailer` and `git.Trailer` have identical `Key`/`Value`
fields but are distinct Go types. `SystemGitClient.Commit()` converts
explicitly.

### VendorNoteData / VendorNoteEntry

JSON types for the git note payload. Not YAML — notes are for
tooling, not human editing.

---

## Multi-Namespace Composition

When git-agent invokes git-vendor, the commit carries both namespaces:

```go
// git-agent calls VendorTrailers for the vendor portion
vendorTrailers := core.VendorTrailers(lockEntries)

// git-agent adds its own trailers
allTrailers := append(agentTrailers, vendorTrailers...)

// Result: two Commit-Schema values in one commit
// Commit-Schema: agent/v1
// Commit-Schema: vendor/v1
// Agent-Id: ...
// Vendor-Name: ...
```

git-vendor exposes `VendorTrailers()` as a pure function for this purpose.

---

## Hook Integration

git-vendor commits go through normal git commit path (no `--no-verify`).
git-plumbing hooks enrich with Touch, Diff-*, Diff-Surface.

git-vendor does NOT compute shared trailers — that's git-plumbing's concern.

---

## CLI Usage

```bash
# Sync with auto-commit
git-vendor sync --commit

# Update with auto-commit
git-vendor update --commit

# Retroactive annotation
git-vendor annotate              # annotate HEAD
git-vendor annotate abc1234      # annotate specific commit
git-vendor annotate --vendor lib # annotate for specific vendor
```

`--commit` + `--dry-run`: prints warning, disables commit.

---

## Acceptance Criteria

1. `go build ./...` succeeds
2. `go test ./...` passes (24 new tests)
3. `git-vendor sync --commit` produces a single commit with:
   - `Commit-Schema: vendor/v1` (once)
   - `Vendor-Name`, `Vendor-Ref`, `Vendor-Commit` (per vendor)
   - `Vendor-License` (if SPDX available)
   - Subject: `chore(vendor): ...` (conventional-commits format)
4. Multi-vendor sync produces ONE commit (not N)
5. Git note attached under `refs/notes/vendor` with JSON metadata
6. Note failure is non-fatal (commit still succeeds)
7. `git vendor annotate` attaches note to existing commit
8. `VendorTrailers()` exported for git-agent composition

---

## Legacy Traps

- **One-commit-per-vendor**: Original design. Replaced with
  single-commit + multi-valued trailers. Do not regress.
- **map[string]string trailers**: Cannot represent multi-valued
  keys. Replaced with []Trailer. The read-side (git log parsing)
  still uses map (first-value-wins). Multi-value read requires
  scanning raw trailer lines or future TrailerList API.
- **Vendor-Compliance/Vendor-Reason**: Dropped from COMMIT-SCHEMA v1.
  Do not add.

---

## Boundaries (what git-vendor does NOT do)

- Does NOT compute Touch, Diff-*, Diff-Surface (hooks handle it)
- Does NOT know about agent identity, model, intent, confidence
- Does NOT validate other namespace trailers
- Does NOT install git-plumbing (peer dependency)
- Does NOT write `Commit-Schema: agent/v1` (git-agent's concern)
- Does NOT parse commit history or query trailers
- Does NOT create a vendor hook (all vendor commits come via CLI)

git-vendor writes vendor commits. git-plumbing enriches them.
git-agent composes with them when agents do vendor operations.

---
