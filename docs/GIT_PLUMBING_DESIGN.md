# git-plumbing — Phase 3: Shared Library Design

> API specification for the `git-plumbing` shared library, extracted from the Phase 1-2 audit.

---

## Design Principles

1. **Shell-out only** — executes the `git` binary via `os/exec`, no Go git libraries
2. **Thin wrappers** — each function runs one git command, parses output, returns a typed result
3. **Typed returns** — no raw strings; `Status()` returns `RepoStatus`, not `string`
4. **Typed errors** — callers use `errors.Is(err, git.ErrNotRepo)`, not string matching
5. **Context-aware** — all functions take `context.Context` for cancellation and timeouts
6. **Working-dir parameterized** — all functions accept a repo path, never assume CWD
7. **Zero dependencies** — stdlib only (except `testutil/`, which uses `testing` helpers)
8. **Null-byte delimiters** — machine-safe parsing using `%x00` instead of `|` in format strings

---

## Package Structure

```text
git-plumbing/
├── git.go           # Core: Git struct, Run, RunLines, RunSilent
├── git_test.go
├── errors.go        # Typed errors: ErrNotRepo, ErrDirtyTree, etc.
├── refs.go          # rev-parse, symbolic-ref, branch name, HEAD state
├── refs_test.go
├── status.go        # Parse status --porcelain, clean check, in-progress ops
├── status_test.go
├── log.go           # Log queries with format strings, author/grep filters
├── log_test.go
├── diff.go          # Diff stats: cached, working tree, numstat parsing
├── diff_test.go
├── tag.go           # Tag CRUD: create, list, delete, points-at
├── tag_test.go
├── remote.go        # Init, clone, remote add, fetch
├── remote_test.go
├── tree.go          # ls-tree directory listing
├── tree_test.go
├── commit.go        # Commit creation with trailer support
├── commit_test.go
├── branch.go        # Branch listing, creation
├── branch_test.go
├── config.go        # git config read/write, user identity
├── config_test.go
├── testutil/
│   ├── repo.go      # Create temp repos with known state
│   ├── commits.go   # Seed repos with N commits, branches, tags
│   └── fixtures.go  # Pre-built DAG configurations
└── go.mod
```

---

## Core Types

### git.go — Execution Layer

```go
package git

import (
    "context"
    "os/exec"
    "strings"
)

// Git represents a git repository at a specific directory.
type Git struct {
    Dir     string // working directory
    Verbose bool   // log commands to stderr
}

// New creates a Git instance for the given directory.
func New(dir string) *Git {
    return &Git{Dir: dir}
}

// Run executes a git command and returns trimmed stdout.
func (g *Git) Run(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = g.Dir
    out, err := cmd.Output()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return "", &GitError{
                Args:   args,
                Stderr: string(exitErr.Stderr),
                Err:    err,
            }
        }
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

// RunLines executes a git command and returns stdout split by newlines.
func (g *Git) RunLines(ctx context.Context, args ...string) ([]string, error) {
    out, err := g.Run(ctx, args...)
    if err != nil {
        return nil, err
    }
    if out == "" {
        return nil, nil
    }
    return strings.Split(out, "\n"), nil
}

// RunSilent executes a git command, discarding output on success.
// On error, includes combined stdout+stderr in the error message.
func (g *Git) RunSilent(ctx context.Context, args ...string) error {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = g.Dir
    if output, err := cmd.CombinedOutput(); err != nil {
        return &GitError{
            Args:   args,
            Stderr: string(output),
            Err:    err,
        }
    }
    return nil
}
```

### errors.go — Typed Errors

```go
package git

import "errors"

// Sentinel errors for common git failure modes.
var (
    ErrNotRepo      = errors.New("not a git repository")
    ErrDirtyTree    = errors.New("working tree has uncommitted changes")
    ErrDetachedHead = errors.New("HEAD is detached")
    ErrRefNotFound  = errors.New("ref not found")
    ErrConflict     = errors.New("merge conflict in progress")
)

// GitError wraps an exec error with the command that was run and stderr output.
type GitError struct {
    Args   []string // git subcommand and arguments
    Stderr string   // stderr output from git
    Err    error    // underlying exec error
}

func (e *GitError) Error() string {
    return strings.TrimSpace(e.Stderr)
}

func (e *GitError) Unwrap() error {
    return e.Err
}
```

### refs.go — Reference Resolution

```go
package git

import "context"

// HEAD returns the full SHA of the current HEAD commit.
func (g *Git) HEAD(ctx context.Context) (string, error) {
    return g.Run(ctx, "rev-parse", "HEAD")
}

// CurrentBranch returns the short name of the current branch.
// Returns ErrDetachedHead if HEAD is not on a branch.
func (g *Git) CurrentBranch(ctx context.Context) (string, error) {
    out, err := g.Run(ctx, "symbolic-ref", "--short", "HEAD")
    if err != nil {
        return "", ErrDetachedHead
    }
    return out, nil
}

// IsDetached returns true if HEAD is in detached state.
func (g *Git) IsDetached(ctx context.Context) (bool, error) {
    _, err := g.CurrentBranch(ctx)
    if errors.Is(err, ErrDetachedHead) {
        return true, nil
    }
    return false, err
}

// ResolveRef resolves a ref name to its full SHA.
func (g *Git) ResolveRef(ctx context.Context, ref string) (string, error) {
    out, err := g.Run(ctx, "rev-parse", ref)
    if err != nil {
        return "", ErrRefNotFound
    }
    return out, nil
}
```

### status.go — Working Tree State

```go
package git

import "context"

// FileStatus represents a file's status in the working tree.
type FileStatus struct {
    Path    string
    Index   byte // status in index (staged): ' ', M, A, D, R, C, U, ?
    WorkDir byte // status in working directory: ' ', M, D, ?, !
}

// RepoStatus represents the full status of a git repository.
type RepoStatus struct {
    Clean      bool
    Staged     []FileStatus
    Unstaged   []FileStatus
    Untracked  []string
    InProgress *InProgressOp // nil if no operation in progress
}

// InProgressOp indicates a git operation that is currently in progress.
type InProgressOp struct {
    Type string // "rebase", "merge", "cherry-pick", "am", "bisect"
}

// Status returns the full working tree status.
func (g *Git) Status(ctx context.Context) (*RepoStatus, error) {
    lines, err := g.RunLines(ctx, "status", "--porcelain=v1")
    if err != nil {
        return nil, err
    }
    status := &RepoStatus{Clean: len(lines) == 0}
    for _, line := range lines {
        if len(line) < 4 {
            continue
        }
        fs := FileStatus{
            Index:   line[0],
            WorkDir: line[1],
            Path:    line[3:],
        }
        switch {
        case fs.Index == '?' && fs.WorkDir == '?':
            status.Untracked = append(status.Untracked, fs.Path)
        case fs.Index != ' ' && fs.Index != '?':
            status.Staged = append(status.Staged, fs)
        case fs.WorkDir != ' ' && fs.WorkDir != '?':
            status.Unstaged = append(status.Unstaged, fs)
        }
    }
    status.InProgress = g.detectInProgressOp()
    return status, nil
}

// IsClean returns true if the working tree has no changes.
func (g *Git) IsClean(ctx context.Context) (bool, error) {
    s, err := g.Status(ctx)
    if err != nil {
        return false, err
    }
    return s.Clean, nil
}
```

### log.go — History Queries

```go
package git

import (
    "context"
    "strings"
    "time"
)

// Commit represents a parsed git commit.
type Commit struct {
    Hash     string
    Short    string
    Subject  string
    Author   string
    Date     time.Time
    Trailers map[string]string // key-value pairs from commit trailers
}

// LogOpts configures a log query.
type LogOpts struct {
    Range   string // e.g., "abc123..def456" or "HEAD~5..HEAD"
    MaxCount int
    Author  string // --author filter
    Grep    string // --grep filter (matches commit message)
    All     bool   // --all flag
    OneLine bool   // simplified one-line format
}

// Log returns commits matching the given options.
// Uses null-byte delimiters for safe parsing.
func (g *Git) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
    format := "--pretty=format:%H%x00%h%x00%s%x00%an%x00%aI"
    args := []string{"log", format}

    if opts.MaxCount > 0 {
        args = append(args, fmt.Sprintf("-%d", opts.MaxCount))
    }
    if opts.Author != "" {
        args = append(args, "--author="+opts.Author)
    }
    if opts.Grep != "" {
        args = append(args, "--grep="+opts.Grep)
    }
    if opts.All {
        args = append(args, "--all")
    }
    if opts.Range != "" {
        args = append(args, opts.Range)
    }

    out, err := g.Run(ctx, args...)
    if err != nil {
        return nil, err
    }
    if out == "" {
        return nil, nil
    }

    var commits []Commit
    for _, line := range strings.Split(out, "\n") {
        parts := strings.Split(line, "\x00")
        if len(parts) != 5 {
            continue
        }
        date, _ := time.Parse(time.RFC3339, parts[4])
        commits = append(commits, Commit{
            Hash:    parts[0],
            Short:   parts[1],
            Subject: parts[2],
            Author:  parts[3],
            Date:    date,
        })
    }
    return commits, nil
}
```

### diff.go — Change Statistics

```go
package git

import "context"

// FileStat represents line-change statistics for a single file.
type FileStat struct {
    Path    string
    Added   int
    Removed int
}

// DiffStat represents aggregate diff statistics.
type DiffStat struct {
    Files []FileStat
    Total FileStat // aggregate totals
}

// DiffCachedStat returns line-change stats for staged (cached) changes.
func (g *Git) DiffCachedStat(ctx context.Context) (*DiffStat, error) {
    // --numstat returns: added\tremoved\tpath
    lines, err := g.RunLines(ctx, "diff", "--cached", "--numstat")
    if err != nil {
        return nil, err
    }
    return parseNumstat(lines), nil
}

// DiffCachedNames returns file paths with staged changes.
func (g *Git) DiffCachedNames(ctx context.Context) ([]string, error) {
    return g.RunLines(ctx, "diff", "--cached", "--name-only")
}
```

### tag.go — Tag Operations

```go
package git

import "context"

// TagsAt returns all tags pointing at the given commit.
// Prefers semver-looking tags when multiple match.
func (g *Git) TagsAt(ctx context.Context, commitHash string) ([]string, error) {
    lines, err := g.RunLines(ctx, "tag", "--points-at", commitHash)
    if err != nil {
        return nil, nil // no tags is not an error
    }
    return lines, nil
}

// CreateTag creates a lightweight tag at the current HEAD.
func (g *Git) CreateTag(ctx context.Context, name string) error {
    return g.RunSilent(ctx, "tag", name)
}

// DeleteTag removes a tag.
func (g *Git) DeleteTag(ctx context.Context, name string) error {
    return g.RunSilent(ctx, "tag", "-d", name)
}

// ListTags returns tags matching a pattern, sorted by creation date (newest first).
func (g *Git) ListTags(ctx context.Context, pattern string) ([]string, error) {
    args := []string{"tag", "-l", "--sort=-creatordate"}
    if pattern != "" {
        args = append(args, pattern)
    }
    return g.RunLines(ctx, args...)
}
```

### remote.go — Repository Setup

```go
package git

import "context"

// CloneOpts configures a clone operation.
type CloneOpts struct {
    Filter     string // e.g., "blob:none" for treeless clone
    NoCheckout bool
    Depth      int
}

// Init initializes a new git repository.
func (g *Git) Init(ctx context.Context) error {
    return g.RunSilent(ctx, "init")
}

// AddRemote adds a named remote.
func (g *Git) AddRemote(ctx context.Context, name, url string) error {
    return g.RunSilent(ctx, "remote", "add", name, url)
}

// Clone clones a repository into this directory.
func (g *Git) Clone(ctx context.Context, url string, opts *CloneOpts) error {
    args := []string{"clone"}
    if opts != nil {
        if opts.Filter != "" {
            args = append(args, "--filter="+opts.Filter)
        }
        if opts.NoCheckout {
            args = append(args, "--no-checkout")
        }
        if opts.Depth > 0 {
            args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
        }
    }
    args = append(args, url, ".")
    return g.RunSilent(ctx, args...)
}

// Fetch fetches from a remote with optional depth.
func (g *Git) Fetch(ctx context.Context, remote, ref string, depth int) error {
    args := []string{"fetch"}
    if depth > 0 {
        args = append(args, "--depth", fmt.Sprintf("%d", depth))
    }
    args = append(args, remote, ref)
    return g.RunSilent(ctx, args...)
}

// FetchAll fetches all refs from a remote.
func (g *Git) FetchAll(ctx context.Context, remote string) error {
    return g.RunSilent(ctx, "fetch", remote)
}

// Checkout checks out a ref (branch, tag, or commit hash).
func (g *Git) Checkout(ctx context.Context, ref string) error {
    return g.RunSilent(ctx, "checkout", ref)
}
```

### config.go — Git Configuration

```go
package git

import "context"

// UserIdentity returns the configured user in "Name <email>" format.
// Returns empty string if not configured.
func (g *Git) UserIdentity(ctx context.Context) string {
    name, _ := g.Run(ctx, "config", "user.name")
    email, _ := g.Run(ctx, "config", "user.email")
    switch {
    case name != "" && email != "":
        return name + " <" + email + ">"
    case name != "":
        return name
    case email != "":
        return email
    default:
        return ""
    }
}

// ConfigGet reads a git config value.
func (g *Git) ConfigGet(ctx context.Context, key string) (string, error) {
    return g.Run(ctx, "config", key)
}

// ConfigSet writes a git config value.
func (g *Git) ConfigSet(ctx context.Context, key, value string) error {
    return g.RunSilent(ctx, "config", key, value)
}
```

### commit.go — Commit Creation (git-agent needs)

```go
package git

import "context"

// CommitOpts configures a commit operation.
type CommitOpts struct {
    Message  string
    Trailers map[string]string // key=value pairs added via --trailer
}

// Commit creates a new commit with the given options.
func (g *Git) Commit(ctx context.Context, opts CommitOpts) error {
    args := []string{"commit", "-m", opts.Message}
    for key, value := range opts.Trailers {
        args = append(args, "--trailer", key+"="+value)
    }
    return g.RunSilent(ctx, args...)
}

// Add stages files for the next commit.
func (g *Git) Add(ctx context.Context, paths ...string) error {
    args := append([]string{"add"}, paths...)
    return g.RunSilent(ctx, args...)
}
```

### tree.go — Directory Listing

```go
package git

import "context"

// ListTree lists files and directories at a given ref and path.
// Returns entries with "/" suffix for directories.
func (g *Git) ListTree(ctx context.Context, ref, subdir string) ([]string, error) {
    // Implementation extracted from git-vendor's SystemGitClient.ListTree
    // Uses parseListTreeOutput helper for parsing
    target := ref
    if target == "" {
        target = "HEAD"
    }
    args := []string{"ls-tree", target}
    if subdir != "" && subdir != "." {
        args = append(args, strings.TrimSuffix(subdir, "/")+"/")
    }
    out, err := g.Run(ctx, args...)
    if err != nil && subdir != "" {
        args = []string{"ls-tree", target, strings.TrimSuffix(subdir, "/")}
        out, err = g.Run(ctx, args...)
    }
    if err != nil {
        return nil, err
    }
    return parseTreeOutput(out, subdir), nil
}
```

---

## testutil/ — Test Infrastructure

```go
package testutil

import "testing"

// TestRepo is a temporary git repository for testing.
type TestRepo struct {
    Dir string
    t   *testing.T
}

// NewTestRepo creates an initialized git repository in t.TempDir().
func NewTestRepo(t *testing.T) *TestRepo {
    t.Helper()
    dir := t.TempDir()
    run(t, dir, "init")
    run(t, dir, "config", "user.email", "test@example.com")
    run(t, dir, "config", "user.name", "Test User")
    return &TestRepo{Dir: dir, t: t}
}

// Commit creates a commit with the given files.
// Returns the commit SHA.
func (r *TestRepo) Commit(msg string, files map[string]string) string {
    r.t.Helper()
    for path, content := range files {
        writeFile(r.t, r.Dir, path, content)
    }
    run(r.t, r.Dir, "add", ".")
    run(r.t, r.Dir, "commit", "-m", msg)
    return run(r.t, r.Dir, "rev-parse", "HEAD")
}

// Branch creates and checks out a new branch.
func (r *TestRepo) Branch(name string) {
    r.t.Helper()
    run(r.t, r.Dir, "checkout", "-b", name)
}

// Checkout switches to an existing branch.
func (r *TestRepo) Checkout(name string) {
    r.t.Helper()
    run(r.t, r.Dir, "checkout", name)
}

// Tag creates a lightweight tag at HEAD.
func (r *TestRepo) Tag(name string) {
    r.t.Helper()
    run(r.t, r.Dir, "tag", name)
}

// Merge merges a branch into the current branch.
func (r *TestRepo) Merge(branch string) {
    r.t.Helper()
    run(r.t, r.Dir, "merge", branch)
}

// Pre-built fixtures:

// LinearHistory creates a repo with n sequential commits on main.
func LinearHistory(t *testing.T, n int) *TestRepo { ... }

// DiamondMerge creates a repo with a branch and merge.
func DiamondMerge(t *testing.T) *TestRepo { ... }

// DirtyWorkingTree creates a repo with uncommitted changes.
func DirtyWorkingTree(t *testing.T) *TestRepo { ... }

// DetachedHead creates a repo with HEAD detached at a commit.
func DetachedHead(t *testing.T) *TestRepo { ... }
```

---

## Key Design Decisions

### 1. `Git` struct vs Interface

The shared library uses a **concrete struct** (`Git`), not an interface. Consumer projects (git-vendor, git-agent) define their own interfaces for testing via mock generation. This avoids premature abstraction in the shared layer.

### 2. Dir field vs parameter

git-vendor's current interface passes `dir` to every method. The shared library uses `Git.Dir` set once at construction. This eliminates parameter repetition and matches the conceptual model: you're working with *one repo*.

### 3. Remote name parameterized

git-vendor hardcodes `"origin"` in several places. The shared library parameterizes the remote name in `Fetch()` and `FetchAll()` for flexibility.

### 4. Error detection from stderr

The `GitError` type captures stderr output from failed commands. Consumer code can pattern-match on stderr strings (e.g., "not a git repository") to map to sentinel errors. The shared library provides a helper:

```go
func IsNotRepo(err error) bool {
    var gitErr *GitError
    if errors.As(err, &gitErr) {
        return strings.Contains(gitErr.Stderr, "not a git repository")
    }
    return false
}
```

### 5. Migration path from git-vendor

When git-vendor adopts git-plumbing, the migration is:

```text
Before:  gitClient.Init(ctx, tempDir)
After:   git.New(tempDir).Init(ctx)
```

The `GitClient` interface in git-vendor wraps `git.Git` methods, adapting the `dir`-as-field pattern back to `dir`-as-parameter for backward compatibility with existing test mocks.

---

## Implementation Priority

Based on the Phase 1-2 overlap matrix:

| Priority | File | Operations | Source |
|---|---|---|---|
| 1 | `git.go` | Run, RunLines, RunSilent | git-vendor `run()` + `runOutput()` |
| 2 | `errors.go` | ErrNotRepo, GitError | New |
| 3 | `refs.go` | HEAD, CurrentBranch, IsDetached | git-vendor `GetHeadHash` + new |
| 4 | `remote.go` | Init, Clone, Fetch, Checkout | git-vendor (6 methods) |
| 5 | `config.go` | UserIdentity, ConfigGet | git-vendor `GetGitUserIdentity` |
| 6 | `log.go` | Log with null-byte parsing | git-vendor `GetCommitLog` (fixed) |
| 7 | `tag.go` | TagsAt, CreateTag, DeleteTag, ListTags | git-vendor `GetTagForCommit` + new |
| 8 | `tree.go` | ListTree | git-vendor `ListTree` |
| 9 | `status.go` | Status, IsClean | New (git-agent needs) |
| 10 | `diff.go` | DiffCachedStat, DiffCachedNames | New (git-agent needs) |
| 11 | `commit.go` | Commit, Add | New (git-agent needs) |
| 12 | `branch.go` | Branches | New (git-agent needs) |
| 13 | `testutil/` | TestRepo, fixtures | New (both need) |

---

*Generated: 2026-02-07 — Phase 3 of the git-plumbing shared base extraction plan.*
