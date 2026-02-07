# git-plumbing — Phase 1 & 2 Audit Report

> Comprehensive audit of git-vendor's git binary interactions and overlap analysis with git-agent needs.

---

## Table of Contents

1. [Phase 1: git-vendor Git Interaction Inventory](#phase-1-git-vendor-git-interaction-inventory)
   - [Execution Infrastructure](#1-execution-infrastructure)
   - [Repository Setup Operations](#2-repository-setup-operations)
   - [Fetch & Checkout Operations](#3-fetch--checkout-operations)
   - [Query Operations](#4-query-operations)
   - [Configuration Operations](#5-configuration-operations)
   - [Non-Git Shell Execution](#6-non-git-shell-execution)
   - [Utility Checks](#7-utility-checks)
2. [Phase 2: Overlap Matrix](#phase-2-overlap-matrix)
   - [Confirmed Overlap Matrix](#confirmed-overlap-matrix)
   - [Shared Surface Area Analysis](#shared-surface-area-analysis)
3. [Extraction Recommendations](#extraction-recommendations)
4. [Appendix: Callers by Service](#appendix-callers-by-service)

---

## Phase 1: git-vendor Git Interaction Inventory

All git interactions flow through the `GitClient` interface (`internal/core/git_operations.go:19-31`) implemented by `SystemGitClient`. The interface has 10 methods. There is also 1 standalone function (`GetGitUserIdentity`) and 1 utility check (`IsGitInstalled`).

### 1. Execution Infrastructure

#### run() — Universal Git Command Executor

- **Location:** `internal/core/git_operations.go:288-301`
- **Signature:** `func (g *SystemGitClient) run(dir string, args ...string) error`
- **Command:** `git <args...>` (variadic)
- **Output capture:** `cmd.CombinedOutput()` — stdout + stderr merged
- **Error handling:** Wraps combined output in `fmt.Errorf("%s", string(output))`
- **Verbose mode:** When `g.verbose == true`, logs `[DEBUG] git <args> (in <dir>)` to stderr
- **Working dir:** Always set via `cmd.Dir = dir`
- **Extractable:** YES — this is the foundational exec wrapper both projects need
- **Used by:** Init, AddRemote, Fetch, FetchAll, Checkout, Clone (6 of 10 interface methods)

**Observations:**
- Returns `error` only (no stdout capture). Methods needing output (GetHeadHash, ListTree, GetCommitLog, GetTagForCommit) bypass `run()` and call `exec.Command` directly.
- This means there are effectively **two execution paths**: fire-and-forget via `run()`, and output-capturing via direct `exec.Command`.
- A shared library should unify these into a single executor that always captures output and optionally returns it.

---

### 2. Repository Setup Operations

#### git init

- **Location:** `internal/core/git_operations.go:44-46`
- **Command:** `git init`
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`
- **Extractable:** YES — universal git operation
- **Callers:** SyncService (line 419), DiffVendor (line 65), UpdateChecker (line 113), FallbackLicenseChecker — all create temp repos for cloning

#### git remote add

- **Location:** `internal/core/git_operations.go:48-51`
- **Command:** `git remote add <name> <url>`
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`
- **Extractable:** YES — universal git operation
- **Callers:** SyncService (line 422), DiffVendor (line 70), UpdateChecker (line 118) — always `"origin"` as name

#### git clone

- **Location:** `internal/core/git_operations.go:84-102`
- **Command:** `git clone [--filter=<filter>] [--no-checkout] [--depth N] <url> .`
- **Options via `types.CloneOptions`:**
  - `Filter` (string): e.g., `"blob:none"` for partial/treeless clone
  - `NoCheckout` (bool): `--no-checkout` flag
  - `Depth` (int): `--depth N` for shallow clone
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`
- **Extractable:** YES — universal git operation with useful option struct
- **Callers:** RemoteExplorer (line 57, with `blob:none` + no-checkout + depth 1), FallbackLicenseChecker (line 42, with depth 1)

---

### 3. Fetch & Checkout Operations

#### git fetch (with optional depth)

- **Location:** `internal/core/git_operations.go:53-61`
- **Command:** `git fetch [--depth N] origin <ref>`
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`
- **Extractable:** YES — universal git operation
- **Callers:** SyncService.fetchWithFallback (line 550), DiffVendor (line 77), UpdateChecker (line 123), RemoteExplorer (line 64)

#### git fetch origin (all refs)

- **Location:** `internal/core/git_operations.go:63-66`
- **Command:** `git fetch origin`
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`
- **Extractable:** YES — universal git operation
- **Callers:** SyncService.fetchWithFallback (line 552), DiffVendor (line 75)

#### git checkout

- **Location:** `internal/core/git_operations.go:68-71`
- **Command:** `git checkout <ref>`
- **Output parsing:** None (fire-and-forget via `run()`)
- **Error handling:** Returns error from `run()`. Callers detect specific errors:
  - `"reference is not a tree"` → `StaleCommitError`
  - `"not a valid object"` → `StaleCommitError`
- **Extractable:** YES — universal git operation
- **Callers:** SyncService.syncRef (lines 492, 505-506), DiffVendor (line 83)
- **Note:** Checkout targets include commit hashes, `FETCH_HEAD`, branch/tag refs

---

### 4. Query Operations

#### git rev-parse HEAD

- **Location:** `internal/core/git_operations.go:73-82`
- **Command:** `git rev-parse HEAD`
- **Execution:** Direct `exec.Command` (bypasses `run()`)
- **Output parsing:** `strings.TrimSpace(string(out))` — returns 40-char hex SHA
- **Error handling:** Returns raw exec error
- **Extractable:** YES — the most universal git query operation
- **Callers:** SyncService.syncRef (line 513), DiffVendor (line 87), UpdateChecker (line 128)

#### git ls-tree

- **Location:** `internal/core/git_operations.go:104-165`
- **Command:** `git ls-tree <ref> [<path>/]`
- **Execution:** Direct `exec.CommandContext` with **30-second timeout**
- **Output parsing:** Complex multi-step:
  1. `strings.Split(string(out), "\n")` — split into lines
  2. `strings.Fields(l)` — split each line into fields (mode, type, hash, path)
  3. `parts[1]` = object type (`tree` or `blob`)
  4. `strings.Join(parts[3:], " ")` = full path (handles spaces in filenames)
  5. Strips subdirectory prefix for relative names
  6. Appends `/` suffix for tree objects (directories)
  7. `sort.Strings(items)` — alphabetical sort
- **Error handling:** Two-attempt strategy:
  1. Try with trailing slash on subdir
  2. Retry without trailing slash if first attempt fails
  3. Returns `fmt.Errorf("git ls-tree failed: %w", err)` on final failure
- **Extractable:** YES — general-purpose directory listing at any ref
- **Callers:** RemoteExplorer.FetchRepoDir (line 73)

#### git log (range with format)

- **Location:** `internal/core/git_operations.go:167-212`
- **Command:** `git log --pretty=format:%H|%h|%s|%an|%ai [-N] <oldHash>..<newHash>`
- **Execution:** Direct `exec.Command` (no timeout)
- **Output format:** Pipe-delimited: `fullHash|shortHash|subject|author|dateISO`
- **Output parsing:**
  1. `strings.TrimSpace(string(out))` + split by `\n`
  2. `strings.Split(line, "|")` — expects exactly 5 fields
  3. Maps to `types.CommitInfo{Hash, ShortHash, Subject, Author, Date}`
- **Error handling:** Returns `fmt.Errorf("git log failed: %w", err)`
- **Extractable:** YES — general-purpose log query. Format is vendor-specific but the operation is universal.
- **Callers:** DiffVendor (line 93)
- **Limitation:** Pipe delimiter `|` will break if commit subjects contain `|` characters. A shared library should use `%x00` (null byte) as delimiter.

#### git tag --points-at

- **Location:** `internal/core/git_operations.go:214-246`
- **Command:** `git tag --points-at <commitHash>`
- **Execution:** Direct `exec.Command` (no timeout)
- **Output parsing:**
  1. `strings.TrimSpace(string(out))` — trim whitespace
  2. `strings.Split(tagsOutput, "\n")` — split into tag names
  3. Iterates tags, prefers semver matches via `semverRegex` (`^\d+\.\d+\.\d+`)
  4. Falls back to first tag if no semver match
- **Error handling:** Returns `("", nil)` on error — treats "no tags" as non-error
- **Extractable:** YES — general-purpose tag query
- **Callers:** SyncService.syncRef (line 520)

---

### 5. Configuration Operations

#### git config user.name / user.email

- **Location:** `internal/core/git_operations.go:254-285`
- **Function:** `GetGitUserIdentity() string` (standalone, not a method on SystemGitClient)
- **Commands:** Two separate `exec.Command` calls:
  1. `git config user.name`
  2. `git config user.email`
- **Output parsing:** `strings.TrimSpace()` on each
- **Return format:** `"Name <email>"`, `"Name"`, `"email"`, or `""` (empty on error)
- **Error handling:** Silent failure — returns empty string if either command fails
- **Extractable:** YES — universal identity query
- **Callers:** UpdateService.updateAllSequential (line 83), UpdateService.updateAllParallel (line 158)

---

### 6. Non-Git Shell Execution

#### Hook execution (sh -c / cmd /c)

- **Location:** `internal/core/hook_service.go:56-85`
- **Command:** `sh -c <command>` (Unix) or `cmd /c <command>` (Windows)
- **Not a git command** — executes arbitrary user-defined shell scripts
- **Extractable:** NO — vendor-specific hook system
- **Note:** Users could run git commands within hooks, but that's user responsibility

---

### 7. Utility Checks

#### exec.LookPath("git")

- **Location:** `internal/core/engine.go:82-85`
- **Function:** `IsGitInstalled() bool`
- **Command:** Not an execution — checks if `git` binary exists in PATH
- **Extractable:** YES — universal prerequisite check

---

## Phase 2: Overlap Matrix

### Confirmed Overlap Matrix

Based on the actual code audit of git-vendor and the git-agent design spec:

```text
Operation                       git-vendor has it?   git-agent needs it?   Extract?
────────────────────────────    ──────────────────   ───────────────────   ────────
EXECUTION INFRASTRUCTURE
  exec wrapper (run + capture)  YES (run() + direct) YES (definitely)      YES ★
  verbose/debug logging         YES (stderr)         YES (likely)          YES
  context timeout support       PARTIAL (ls-tree)    YES (all ops)         YES
  git binary check              YES (LookPath)       YES                   YES

REF RESOLUTION
  rev-parse HEAD                YES                  YES (handoff)         YES ★
  symbolic-ref (current branch) NO                   YES (5 subcommands)   YES ★
  detached HEAD detection       NO                   YES (experiment)      YES

STATUS & STATE
  status --porcelain            NO                   YES (handoff)         YES ★
  status --short                NO                   YES (sitrep)          YES
  clean working tree check      NO (uses file I/O)   YES                   YES
  in-progress op detection      NO                   YES (sitrep)          YES

LOG & HISTORY
  log with format strings       YES (pipe-delimited) YES (heavy usage)     YES ★
  log with --grep/--author      NO                   YES (agent-commit)    YES
  log --oneline range           NO                   YES (handoff)         YES

DIFF
  diff --cached --numstat       NO                   YES (agent-commit)    YES ★
  diff --cached --stat          NO                   YES (agent-commit)    YES
  diff --cached --name-only     NO                   YES (guardrails)      YES

BRANCH OPERATIONS
  branch -vv (listing)          NO                   YES (sitrep)          YES
  branch creation               NO                   YES (experiment)      MAYBE

TAG OPERATIONS
  tag --points-at               YES                  NO                    YES
  tag create (lightweight)      NO                   YES (experiment)      YES
  tag -l (list with sort)       NO                   YES (experiment)      YES
  tag -d (delete)               NO                   YES (experiment)      YES

COMMIT OPERATIONS
  commit -m with --trailer      NO                   YES (agent-commit)    YES ★
  git add/rm (staging)          NO                   YES (agent-commit)    YES

REMOTE & FETCH
  git init                      YES                  MAYBE                 YES
  git remote add                YES                  MAYBE                 YES
  git clone (with options)      YES                  MAYBE                 YES
  git fetch (with depth)        YES                  MAYBE                 YES
  git fetch origin (all)        YES                  MAYBE                 YES
  git checkout <ref>            YES                  YES (experiment)      YES

CONFIG
  git config read               YES (user identity)  MAYBE                 YES
  git config write              NO                   MAYBE                 YES

TREE OPERATIONS
  ls-tree (dir listing)         YES                  NO                    YES

REF ENUMERATION
  for-each-ref                  NO                   YES (sitrep)          YES
  stash list                    NO                   YES (sitrep)          YES

WORKING TREE MANIPULATION
  reset --hard                  NO                   YES (experiment)      YES
  clean -fd                     NO                   YES (experiment)      YES

HOOK MANAGEMENT
  .git/hooks/ write             NO                   YES (guardrails)      NO ★★

.GIT DIR FILE I/O
  Read/write custom .git files  NO                   YES (handoff)         NO ★★
```

**Legend:**
- ★ = High-priority extraction candidate (confirmed overlap or critical need)
- ★★ = git-agent specific, stays out of shared library

---

### Shared Surface Area Analysis

#### Tier 1: Confirmed Overlap (git-vendor has it, git-agent needs it)

These exist in git-vendor today and git-agent will need them. Direct extraction candidates:

| Operation | git-vendor Location | Notes |
|---|---|---|
| Exec wrapper | `git_operations.go:288-301` | Needs unification (run vs direct) |
| `git rev-parse HEAD` | `git_operations.go:73-82` | Exact match |
| `git log` with format | `git_operations.go:167-212` | git-agent needs more format flexibility |
| `git tag --points-at` | `git_operations.go:214-246` | Useful for both |
| `git config` read | `git_operations.go:254-285` | git-agent may need write too |
| `git init` | `git_operations.go:44-46` | Exact match |
| `git remote add` | `git_operations.go:48-51` | Exact match |
| `git clone` with opts | `git_operations.go:84-102` | Exact match |
| `git fetch` (shallow) | `git_operations.go:53-61` | Exact match |
| `git fetch` (all) | `git_operations.go:63-66` | Exact match |
| `git checkout` | `git_operations.go:68-71` | Exact match |
| `git ls-tree` | `git_operations.go:104-165` | May not be needed by git-agent |
| `IsGitInstalled` | `engine.go:82-85` | Both need it |
| `GetGitUserIdentity` | `git_operations.go:254-285` | Both need it |

**Count: 14 operations confirmed extractable from existing code.**

#### Tier 2: git-agent Needs, git-vendor Doesn't Have

These must be built new for the shared library:

| Operation | git-agent Usage | Priority |
|---|---|---|
| `symbolic-ref --short HEAD` | 5 subcommands (current branch) | HIGH |
| `status --porcelain` | handoff (machine-readable) | HIGH |
| `status --short` | sitrep (human-readable) | HIGH |
| `diff --cached --numstat` | agent-commit, guardrails | HIGH |
| `diff --cached --name-only` | guardrails | HIGH |
| `commit -m --trailer` | agent-commit | HIGH |
| `branch -vv --sort` | sitrep | MEDIUM |
| `tag create/list/delete` | experiment | MEDIUM |
| `for-each-ref` | sitrep | MEDIUM |
| `log --grep/--author` | agent-commit | MEDIUM |
| `stash list` | sitrep | LOW |
| `reset --hard` | experiment | LOW |
| `clean -fd` | experiment | LOW |
| In-progress op detection | sitrep | LOW |

**Count: 14 new operations needed.**

#### Tier 3: Stays in Consumer Projects

| Operation | Project | Reason |
|---|---|---|
| Hook execution (sh -c) | git-vendor | Vendor-specific lifecycle hooks |
| `.git/hooks/` management | git-agent | Agent-specific guardrails |
| `.git/AGENT_HANDOFF` I/O | git-agent | Agent-specific handoff protocol |
| URL parsing (ParseSmartURL) | git-vendor | Platform-specific URL decomposition |
| License detection | git-vendor | Vendor-specific compliance |
| File copy/checksum | git-vendor | Vendor-specific sync logic |

---

## Extraction Recommendations

### 1. Executive Summary

git-vendor has **14 extractable git operations** across **12 distinct git commands**. git-agent needs **14 additional operations**. The total shared library surface area is **~28 operations** organized into ~8 files.

### 2. Execution Layer Improvements

The current `run()` helper has a split personality — fire-and-forget methods use it, but output-capturing methods bypass it entirely. The shared library should unify this:

```text
Current git-vendor (two paths):
  run()  → CombinedOutput → discard stdout → return error
  direct → Output/CombinedOutput → parse stdout → return (value, error)

Proposed git-plumbing (one path):
  Run()      → returns (stdout string, error)    — always captures output
  RunLines() → returns ([]string, error)          — splits by newline
  RunSilent() → returns error                     — for fire-and-forget
```

### 3. Bug / Limitation Found During Audit

**`GetCommitLog` pipe delimiter vulnerability** (`git_operations.go:170`):

The log format `--pretty=format:%H|%h|%s|%an|%ai` uses `|` as delimiter. If a commit subject contains `|`, parsing breaks silently (line is skipped due to `len(parts) != 5` check on line 198).

**Recommendation:** The shared library should use null-byte delimiter `%x00` for machine-safe parsing:
```text
--pretty=format:%H%x00%h%x00%s%x00%an%x00%ai
```

### 4. Context Timeout Gap

Only `ListTree` uses `context.WithTimeout` (30s). All other operations have no timeout protection. A shared library should:
- Accept `context.Context` on every function
- Default to no timeout (caller decides)
- Provide `WithTimeout` convenience where appropriate

### 5. Proposed Extraction Order

1. **exec.go** — `Git` struct, `Run`, `RunLines`, `RunSilent`, error type detection
2. **errors.go** — `ErrNotRepo`, `ErrDirtyTree`, `ErrDetachedHead`, `ErrRefNotFound`
3. **refs.go** — `HEAD()`, `CurrentBranch()`, `IsDetached()` (3 from git-vendor, 2 new)
4. **status.go** — `Status()`, `IsClean()`, `InProgressOp()` (all new, needed by both)
5. **log.go** — `Log()` with flexible format (1 from git-vendor, enhanced)
6. **diff.go** — `DiffCachedStat()`, `DiffCachedNames()` (all new for git-agent)
7. **config.go** — `UserIdentity()`, `Get()`, `Set()` (1 from git-vendor, 2 new)
8. **tag.go** — `TagsAt()`, `CreateTag()`, `DeleteTag()`, `ListTags()` (1 from git-vendor, 3 new)
9. **remote.go** — `Init()`, `AddRemote()`, `Clone()`, `Fetch()` (all from git-vendor)
10. **tree.go** — `ListTree()` (from git-vendor)
11. **commit.go** — `Commit()` with trailer support (new for git-agent)
12. **branch.go** — `Branches()`, `CurrentBranch()` (new for git-agent)
13. **testutil/** — test repo fixtures (new, high priority early extraction)

---

## Appendix: Callers by Service

### SyncService (`sync_service.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 419 | `gitClient.Init(tempDir)` | Create temp repo for vendor clone |
| 422 | `gitClient.AddRemote(tempDir, "origin", v.URL)` | Add vendor repo as origin |
| 489-490 | `gitClient.Fetch(tempDir, 1, ref)` | Shallow fetch of specific ref |
| 492 | `gitClient.Checkout(tempDir, targetCommit)` | Checkout locked commit (sync mode) |
| 505-506 | `gitClient.Checkout(tempDir, FetchHead/ref)` | Checkout latest (update mode) |
| 513 | `gitClient.GetHeadHash(tempDir)` | Get resolved commit SHA |
| 520 | `gitClient.GetTagForCommit(tempDir, hash)` | Find version tag for commit |
| 550 | `gitClient.Fetch(tempDir, 1, ref)` | Shallow fetch (first attempt) |
| 552 | `gitClient.FetchAll(tempDir)` | Full fetch (fallback) |

### DiffService (`diff_service.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 65 | `gitClient.Init(tempDir)` | Create temp repo |
| 70 | `gitClient.AddRemote(tempDir, "origin", vendor.URL)` | Add remote |
| 75 | `gitClient.FetchAll(tempDir)` | Fetch all refs (need full history for diff) |
| 77 | `gitClient.Fetch(tempDir, 0, spec.Ref)` | Fallback: fetch specific ref |
| 83 | `gitClient.Checkout(tempDir, "FETCH_HEAD")` | Checkout latest commit |
| 87 | `gitClient.GetHeadHash(tempDir)` | Get latest commit hash |
| 93 | `gitClient.GetCommitLog(tempDir, lockedHash, latestHash, 10)` | Get commit log between versions |

### UpdateChecker (`update_checker.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 113 | `gitClient.Init(tempDir)` | Create temp repo |
| 118 | `gitClient.AddRemote(tempDir, "origin", url)` | Add remote |
| 123 | `gitClient.Fetch(tempDir, 1, ref)` | Shallow fetch latest |
| 128 | `gitClient.GetHeadHash(tempDir)` | Get latest commit hash |

### RemoteExplorer (`remote_explorer.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 57 | `gitClient.Clone(tempDir, url, opts)` | Treeless clone (blob:none, no checkout, depth 1) |
| 64 | `gitClient.Fetch(tempDir, 0, ref)` | Fetch specific ref (best-effort) |
| 73 | `gitClient.ListTree(tempDir, target, subdir)` | Browse remote directory contents |

### FallbackLicenseChecker (`license_fallback.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 42 | `gitClient.Clone(tempDir, repoURL, opts)` | Shallow clone to read LICENSE file |

### UpdateService (`update_service.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 83 | `GetGitUserIdentity()` | Get user name/email for lockfile metadata |
| 93 | `syncService.SyncVendor(...)` | Delegates to SyncService (indirect git usage) |
| 158 | `GetGitUserIdentity()` | Same (parallel path) |

### Engine (`engine.go`)

| Line | Git Operation | Purpose |
|------|--------------|---------|
| 83 | `exec.LookPath("git")` | Check if git binary is installed |

---

*Generated: 2026-02-07 — Phase 1 & 2 of the git-plumbing shared base extraction plan.*
