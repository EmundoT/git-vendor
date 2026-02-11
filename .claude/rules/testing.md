---
paths:
  - "**/*_test.go"
  - "internal/core/mocks_test.go"
  - "internal/core/testhelpers*.go"
  - "Makefile"
---

# Testing

## Testing Boundaries (git-vendor vs git-plumbing)

git-vendor delegates all git operations to git-plumbing via SystemGitClient.gitFor() adapter.

### What git-plumbing tests (do NOT re-test here):
- Git CLI primitives: Clone, Fetch, Init, Checkout, Add, Commit, Log, TagsAt, ListTree, Branches, DiffStat, Status, ShowRef, ForEachRef, AddNote, GetNote
- Error sentinels: ErrNotRepo, ErrDirtyTree, ErrDetachedHead, ErrRefNotFound, ErrConflict
- GitError wrapping with stderr capture
- Parsing: null-byte delimited log format, trailer extraction, numstat parsing
- Edge cases: empty repos, detached HEAD, merge conflicts, invalid refs, shallow clones

### What git-vendor MUST test:
- Mock-based service tests: sync orchestration, update flow, config validation, license compliance, diff, file copy, parallel execution, hooks, position extraction/placement, verification
- SystemGitClient adapter logic: type conversions (git.Commit -> types.CommitInfo), semver tag preference (isSemverTag), date format handling
- URL parsing: ParseSmartURL, cleanURL -- pure string parsing
- Position extraction: ParsePathPosition, ExtractPosition, PlaceContent -- file content manipulation
- Error types: structured error formatting with Error(), Is(), As() chains

### Intentionally untested:
- main.go CLI dispatch -- too coupled to TUI/stdout/os.Exit
- TUI wizard interactions -- charmbracelet/huh forms not amenable to unit testing
- internal/version/ -- trivial var set via ldflags

### Known duplication (acceptable):
git_operations_test.go and integration_test.go contain smoke tests for adapter layer gated behind `//go:build integration`. May be pruned if git-plumbing coverage proves sufficient.

## Mock Generation

Auto-generated via MockGen. Mock files (*_mock_test.go) are git-ignored. Generate before running tests:

```bash
# Unix/Mac/Linux:
make mocks

# Windows (or no make):
go install github.com/golang/mock/mockgen@latest
# Then run mockgen for each interface (see Makefile for exact commands)
```

Interfaces with mocks: GitClient, FileSystem, ConfigStore, LockStore, LicenseChecker

## Test Infrastructure

- testhelpers_gomock_test.go: Gomock setup helpers
- testhelpers.go: Common test utilities
- mocks_test.go: Hand-written mock implementations (supplement to generated mocks)

## Test Patterns

- Table-driven tests (Go idiom for multiple cases)
- gomock for interfaces: `ctrl := gomock.NewController(t); defer ctrl.Finish()`
- All tests pass `go test -race` with no race conditions
- Generate mocks before running tests
