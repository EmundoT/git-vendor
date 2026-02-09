# Migration Plan: git-vendor → git-plumbing

## Context

git-plumbing was extracted from git-vendor as a shared Go library for typed git CLI wrappers. git-vendor still has its own `SystemGitClient` implementation that directly shells out to git via `exec.CommandContext`. This migration replaces that implementation with delegations to git-plumbing, eliminating the duplicated git execution code.

The `GitClient` interface is preserved exactly so mocks, tests, and all consumers remain untouched. Only the `SystemGitClient` implementation body changes.

### Repository locations

- **git-plumbing**: `../git-plumbing` (sibling directory, `C:\Users\{x}\Documents\Projects\code\git-plumbing`)
- **git-vendor**: this repository (`C:\Users\{x}\Documents\Projects\code\git-vendor`)

---

## Phase 0: Fix git-plumbing module path

**Repo:** git-plumbing (`../git-plumbing`)

git-plumbing's `go.mod` declares `github.com/emundoT/git-plumbing` (lowercase 'e') but the GitHub username is `EmundoT`. Fix to `github.com/EmundoT/git-plumbing`.

**Files to change:**
- `../git-plumbing/go.mod` — change module path on line 1 to `github.com/EmundoT/git-plumbing`

Since git-plumbing is package `git` with no internal imports of its own module path (the `testutil/` package imports `git` by package name, not module path), only `go.mod` line 1 needs changing. However, check `testutil/repo.go` and `testutil/fixtures.go` for any import of `github.com/emundoT/git-plumbing` — if found, update those too.

**Verify:**
```bash
cd ../git-plumbing
go build ./...
go test ./...
```

Commit this change to git-plumbing and push before proceeding to Phase 1.

---

## Phase 1: Add git-plumbing dependency to git-vendor

**Repo:** git-vendor (this repo)

```bash
go get github.com/EmundoT/git-plumbing@latest
```

Zero transitive deps added (git-plumbing is stdlib-only).

If git-plumbing is not yet published to a Go module proxy, use a `replace` directive temporarily:
```bash
go mod edit -replace github.com/EmundoT/git-plumbing=../git-plumbing
```

---

## Phase 2: Rewrite SystemGitClient to delegate to git-plumbing

**File:** `internal/core/git_operations.go`

### Keep unchanged:
- `GitClient` interface (lines 19-30) — no signature changes
- `SystemGitClient` struct and `NewSystemGitClient()` constructor
- `isSemverTag()` and `semverRegex` — still used by GetTagForCommit
- `ParseSmartURL()` and `cleanURL()` — not git operations

### Add:
- Import: `git "github.com/EmundoT/git-plumbing"`
- Helper method on SystemGitClient:

```go
func (g *SystemGitClient) gitFor(dir string) *git.Git {
    return &git.Git{Dir: dir, Verbose: g.verbose}
}
```

This creates a `*git.Git` instance per call. It's cheap (single struct allocation, no I/O) and necessary because git-vendor passes `dir` per-call while git-plumbing stores it on the struct.

### Rewrite each method:

| Method | Delegation | Notes |
|--------|-----------|-------|
| `Init` | `g.gitFor(dir).Init(ctx)` | Direct |
| `AddRemote` | `g.gitFor(dir).AddRemote(ctx, name, url)` | Direct |
| `Fetch` | `g.gitFor(dir).Fetch(ctx, "origin", ref, depth)` | Pass `"origin"` as remote; note param order swap (git-vendor: `depth, ref` → git-plumbing: `remote, ref, depth`) |
| `FetchAll` | `g.gitFor(dir).FetchAll(ctx, "origin")` | Pass `"origin"` as remote |
| `Checkout` | `g.gitFor(dir).Checkout(ctx, ref)` | Direct |
| `GetHeadHash` | `g.gitFor(dir).HEAD(ctx)` | Rename only |
| `Clone` | `g.gitFor(dir).Clone(ctx, url, plumbingOpts)` | Convert `types.CloneOptions` → `git.CloneOpts` (same fields: Filter, NoCheckout, Depth) |
| `ListTree` | `g.gitFor(dir).ListTree(ctx, ref, subdir)` | Direct — git-plumbing has identical parsing logic (was extracted from git-vendor) |
| `GetCommitLog` | `g.gitFor(dir).Log(ctx, git.LogOpts{...})` | See detailed notes below |
| `GetTagForCommit` | `g.gitFor(dir).TagsAt(ctx, hash)` | See detailed notes below |

#### GetCommitLog details

git-plumbing's `Log()` returns `[]git.Commit` with `Date` as `time.Time` and `Short` instead of `ShortHash`. The adapter must:
1. Build `git.LogOpts{Range: oldHash+".."+newHash, MaxCount: maxCount}`
2. Convert each `git.Commit` to `types.CommitInfo`:
   - `Hash` → `Hash`
   - `Short` → `ShortHash`
   - `Subject` → `Subject`
   - `Author` → `Author`
   - `Date.Format("2006-01-02 15:04:05 -0700")` → `Date` (string)

The Go format `"2006-01-02 15:04:05 -0700"` produces the exact same output as git's `%ai` format (e.g., `"2024-01-15 10:30:00 +0000"`), which is what `formatDate()` in `diff_service.go` expects.

#### GetTagForCommit details

git-plumbing's `TagsAt()` returns `[]string` (all tags). git-vendor needs a single `string` with semver preference. The adapter must:
1. Call `TagsAt(ctx, commitHash)`
2. If error or empty, return `"", nil`
3. Loop through tags, return first matching `isSemverTag()`
4. Fall back to `tags[0]`

The `isSemverTag()` function and `semverRegex` stay as-is.

### Delete:
- `run()` — internal exec helper, no longer needed
- `runOutput()` — internal exec helper, no longer needed
- `parseListTreeOutput()` — git-plumbing handles parsing internally

### Remove imports:
- `"os/exec"` — no longer used
- `"os"` — only used by deleted `run()` for stderr debug logging

---

## Phase 3: Migrate standalone functions

### GetGitUserIdentity

**File:** `internal/core/git_operations.go`

Replace `GetGitUserIdentity()` body (standalone function, not on GitClient interface):

```go
func GetGitUserIdentity() string {
    g := &git.Git{} // empty Dir matches original behavior (no Dir set on exec.Command)
    return g.UserIdentity(context.Background())
}
```

Using `&git.Git{}` with empty Dir matches the original behavior where `exec.Command("git", "config", "user.name")` ran with no `Dir` set, defaulting to the process working directory.

This also fixes a bug in the current implementation: if `user.name` lookup fails, it returns `""` immediately without trying `user.email`. git-plumbing handles all four name/email combinations correctly.

**Callers (unchanged):**
- `internal/core/update_service.go` line 83
- `internal/core/update_service.go` line 158

### IsGitInstalled

**File:** `internal/core/engine.go`

Replace `IsGitInstalled()` body:

```go
func IsGitInstalled() bool {
    return git.IsInstalled()
}
```

Add import `git "github.com/EmundoT/git-plumbing"`, remove `"os/exec"` if no longer used.

---

## Phase 4: Add test for semver tag preference logic

**File:** `internal/core/git_operations_test.go`

git-vendor's `GetTagForCommit` has semver preference logic (`isSemverTag` regex) that selects a single tag from the full list. This logic is untested. After migration it wraps `TagsAt()` from git-plumbing, so the underlying tag retrieval is well-tested, but the selection layer is not.

Add a test `TestSystemGitClient_GetTagForCommit_SemverPreference` that:
1. Creates a repo with one commit
2. Tags that commit with both a non-semver tag (`release-2025`) and a semver tag (`v1.2.3`)
3. Calls `GetTagForCommit()` and asserts the semver tag `v1.2.3` is returned
4. Also test the fallback: tag a commit with only non-semver tags, assert the first one is returned
5. Also test the empty case: commit with no tags returns `""`

This ensures the semver preference logic survives the migration intact.

---

## Phase 5: Verify

### Error compatibility

`sync_service.go` lines 499-500 does error string matching:
```go
errMsg := err.Error()
if strings.Contains(errMsg, "reference is not a tree") || strings.Contains(errMsg, "not a valid object") {
```

This still works because `git.GitError.Error()` returns `strings.TrimSpace(stderr)` — the same stderr text git produces. The error type changes from `fmt.Errorf` to `*git.GitError`, but `.Error()` returns the same content.

All mock-based tests return plain `fmt.Errorf` errors and are unaffected by the adapter change.

### Date format compatibility

`diff_service.go:166` `formatDate()` splits the date string on spaces and expects `"YYYY-MM-DD HH:MM:SS ±ZZZZ"`. The Go format string `"2006-01-02 15:04:05 -0700"` produces this exactly.

Mock tests (`diff_service_test.go:62,194`) hardcode `CommitInfo.Date` as `"2024-01-02 15:30:00 +0000"` — these use `MockGitClient` and never hit the adapter, so they're unaffected.

### Test commands

```bash
# Full build
go build ./...

# All unit + mock tests
go test ./internal/core/ -v

# Integration tests specifically (these exercise SystemGitClient with real git)
go test ./internal/core/ -run TestSystemGitClient -v

# Full suite
go test ./... -v
```

### Verify no remaining direct exec calls

```bash
grep -n "exec.Command" internal/core/git_operations.go   # Should be zero
grep -n "exec.Command" internal/core/engine.go            # Should be zero
```

---

## Phase 6: Files Changed Summary

| File | Repo | Change |
|------|------|--------|
| `go.mod` | git-plumbing | Fix module path casing (`emundoT` → `EmundoT`) |
| `go.mod` / `go.sum` | git-vendor | Add git-plumbing dependency |
| `internal/core/git_operations.go` | git-vendor | Rewrite SystemGitClient methods to delegate to git-plumbing; delete `run`/`runOutput`/`parseListTreeOutput`; rewrite `GetGitUserIdentity`; add `gitFor` helper |
| `internal/core/engine.go` | git-vendor | Replace `IsGitInstalled` body with `git.IsInstalled()`, update imports |

### Explicitly NOT changed:

- `GitClient` interface — same signatures, mocks stay valid
- `types.CloneOptions` — kept as git-vendor's domain type, adapted in Clone method
- `types.CommitInfo` — kept as git-vendor's domain type, adapted in GetCommitLog method
- `types.VendorDiff` — untouched
- All mock files (`git_client_mock_test.go`) — interface unchanged
- All test files — test through interface/mocks, unaffected
- `diff_service.go`, `sync_service.go`, `update_service.go`, `update_checker.go`, `remote_explorer.go`, `license_fallback.go` — consume GitClient interface, unaffected
