# CLI Redesign: Bidirectional Vendoring

**Status:** Approved (design)
**Date:** 2026-02-17
**Affects:** Phase 1 (Foundation Hardening), all subsequent phases

## Problem Statement

The current CLI has three usability issues:

1. **Confusing command names.** `sync` means "lock → disk" and `update` means "fetch latest → lock." Both sound like they do the same thing. Users (human and LLM) consistently guess wrong about which to use.
2. **Inspection is fragmented.** Three separate commands (`verify`, `diff`, `outdated`) answer aspects of one question: "what's different?" Users don't know which to reach for.
3. **Edits are one-directional.** Local modifications to vendored files are treated as corruption. There is no path to propose changes upstream, no commit guard against accidental drift, and no way to legitimize intentional local divergence.

## Design: New Command Surface

### Core Verbs

```text
git vendor pull [name] [--locked] [--prune] [--keep-local] [--interactive]
git vendor push [name] [--file path]
git vendor status [--offline] [--remote-only]
git vendor accept [name] [--file path]
```

Plus unchanged: `add`, `remove`, `config`, `version`.

### `pull` — Downstream Sync

Replaces `update` + `sync`. One command: "get the latest from upstream."

Default behavior (no flags):
1. Fetch latest commit for each vendor's ref
2. Update lock with new commit hash + file hashes
3. Copy files to disk
4. Save cache

Flags:
- `--locked` — Use existing lock hashes, don't fetch latest. This is the old `sync` behavior. Use case: reproducing exact state from a committed lockfile.
- `--prune` — When a mapped source file no longer exists upstream, automatically remove the mapping from `vendor.yml` (in addition to deleting the local copy and lock entry). Without this flag, dead mappings produce a warning but persist.
- `--keep-local` — When a local file has been modified (lock mismatch), skip overwriting it. Default: overwrite with upstream.
- `--interactive` — Prompt per-file when conflicts exist (local modifications or upstream removals).
- `--force` — Skip cache, force re-fetch. Retained from current CLI.
- `--no-cache` — Don't persist cache after pull. Retained from current CLI.

Upstream file removal handling:
1. `fs.Stat(srcPath)` returns not-found during copy
2. Log: `[removed] <path> (no longer in <vendor>@<ref>)`
3. Delete local copy
4. Remove from lock `FileHashes`
5. If `--prune`: remove mapping from `vendor.yml`
6. Continue remaining files

Local modification handling:
1. Detect lock mismatch before overwriting
2. Default: overwrite (pull wins)
3. `--keep-local`: skip, leave local version
4. `--interactive`: prompt per file

### `push` — Upstream Proposal

New command. Proposes local changes to vendored files back to the source repo.

Flow:
1. Detect locally modified vendored files (lock mismatch) for the target vendor
2. Clone source repo to temp dir
3. Apply local diffs to the corresponding source paths (reverse the `from → to` mapping)
4. Create branch: `vendor-push/<downstream-project>/<YYYY-MM-DD>`
5. Commit with message referencing the downstream project
6. Create PR via `gh pr create` (or output manual instructions if `gh` unavailable)
7. Print PR URL

Scope constraints:
- `--file <path>` — Push only a specific file's changes (default: all modified files for the vendor)
- Requires `gh` CLI for PR creation; falls back to printing git instructions

### `status` — Unified Inspection

Replaces `verify`, `diff`, and `outdated`. One command shows everything.

Output structure:
```text
$ git vendor status
  git-ecosystem (main @ abc123)
    12 files verified
    1 file modified locally: .claude/skills/nominate/SKILL.md
    3 new commits available upstream

  libfoo (v2.1 @ def456)
    5 files verified
    up to date
```

Execution order:
1. Offline checks first (lock vs disk — fast, always runs)
2. Remote checks second (lock vs upstream — requires network)

Flags:
- `--offline` — Skip remote checks. Only show lock-vs-disk status. This is the old `verify`.
- `--remote-only` — Skip disk checks. Only show lock-vs-upstream status. This is the old `outdated`.
- `--format json` — Machine-readable output (retained from current `verify`).

Exit codes:
- 0 = everything matches (PASS)
- 1 = discrepancies found (FAIL — modified, deleted, or upstream drift)
- 2 = warnings only (WARN — added files, accepted drift)

New check: config/lock coherence
- Mapping in `vendor.yml` with no corresponding `FileHashes` entry → stale config
- `FileHashes` entry with no corresponding mapping → orphaned lock entry

### `accept` — Acknowledge Drift

New command. Legitimizes local modifications to vendored files.

Flow:
1. Identify files with lock mismatch for the target vendor
2. Re-hash the local files
3. Write `accepted_drift` entries in lock (path → local hash)
4. `status` will show these as "accepted" rather than "modified"
5. `pull` will warn before overwriting accepted-drift files

Lock schema addition:
```yaml
vendors:
  - name: git-ecosystem
    ref: main
    commit_hash: abc123...
    accepted_drift:
      .claude/skills/nominate/SKILL.md: sha256:9a3f17c...
    file_hashes:
      .claude/skills/nominate/SKILL.md: sha256:bd528ed...
```

`file_hashes` retains the upstream hash. `accepted_drift` records the local hash. This preserves the audit trail: you can always see what upstream had vs. what you intentionally changed.

Clearing accepted drift:
- `pull` overwriting the file clears the entry
- `push` + merge + `pull` clears it naturally (upstream now matches local)
- Manual: `accept --clear [name]`

### Commit Guard

Pre-commit hook integration. Blocks commits that include vendored files with unacknowledged lock mismatch.

```text
$ git commit
  [git-vendor] Lock mismatch detected:
    .claude/skills/nominate/SKILL.md
      lock:  sha256:bd528ed...
      disk:  sha256:9a3f17c...

  Resolve with:
    git vendor pull           # discard local changes, get latest
    git vendor push           # propose changes upstream
    git vendor accept         # acknowledge drift, update lock

  Commit blocked.
```

Implementation: shell hook that runs `git vendor status --offline --format json`, checks for `status: "modified"` entries that are NOT in `accepted_drift`.

Files with accepted drift pass the commit guard — the user has explicitly acknowledged them.

## Backward Compatibility

Old commands become aliases with deprecation warnings for 2 minor versions:

```text
git vendor sync     → git vendor pull --locked  + deprecation notice
git vendor update   → git vendor pull           + deprecation notice
git vendor verify   → git vendor status --offline
git vendor diff     → git vendor status
git vendor outdated → git vendor status --remote-only
```

After 2 minor versions: remove aliases, update all docs.

## Implementation Sequence

1. **`status`** — Merge verify + diff + outdated. Add config/lock coherence checks. Lowest risk, highest immediate value.
2. **`pull`** — Merge update + sync. Add upstream removal handling. Core workflow change.
3. **`accept`** — Lock schema addition (`accepted_drift`). Enables commit guard.
4. **Commit guard** — Pre-commit hook. Depends on `status` and `accept`.
5. **`push`** — Upstream PR creation. Independent of other changes. Can ship last.
6. **Aliases + deprecation** — Wire old commands to new ones. Ship with step 2.

## Non-Goals

- Auto-merging conflicts between local and upstream changes. `pull` overwrites or skips; `push` creates a PR for human review. No three-way merge.
- Watching for upstream changes (polling/webhooks). `status` is always user-initiated.
- Bidirectional sync for internal vendors (Spec 070 already handles this separately via `--reverse`).
