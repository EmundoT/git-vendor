# Code Audit Report: git-vendor

**Date**: 2026-02-05
**Auditor**: Claude (Opus 4.5)
**Scope**: Full codebase review per ROADMAP.md Section 2.1

## Executive Summary

This audit examines the existing features and capabilities of git-vendor to identify weaknesses in code, gaps in test coverage, structural problems, questionable design decisions, and areas for improvement.

### Current Test Coverage

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/core` | 66.1% | Good coverage for core logic |
| `internal/tui` | 10.2% | Critical gap |
| `internal/types` | 0% | No test files |
| `main.go` | 0% | CLI untested |
| `cmd/completion` | 100% | Excellent |
| `internal/version` | 100% | Excellent |

---

## Findings (Ordered by Difficulty)

### TIER 1: Low-Hanging Fruit

#### 1.1 Missing Types Package Tests

**Location**: `internal/types/types.go`
**Issue**: Zero test coverage for data models
**Impact**: No validation that struct tags work correctly, JSON/YAML marshalling behaves as expected

**Recommended Tests**:
- YAML marshalling/unmarshalling for `VendorConfig`, `VendorSpec`, `BranchSpec`
- JSON marshalling for `VerifyResult`, `FileStatus`, `VerifySummary`
- Edge cases: empty slices, nil pointers, omitempty behavior

#### 1.2 Untested Engine Facade Methods

**Locations** (all at 0% coverage):
- `internal/core/config_store.go:28` - `Path()` method
- `internal/core/engine.go:133` - `Init()`
- `internal/core/engine.go:143` - `FetchRepoDir()`
- `internal/core/engine.go:148` - `ListLocalDir()`
- `internal/core/engine.go:153` - `RemoveVendor()`

**Fix**: Add unit tests for these facade methods using mocked dependencies.

#### 1.3 Direct `fmt.Print` Bypasses UICallback

**Affected Files** (40+ instances):
- `internal/core/sync_service.go` - 15 instances
- `internal/core/hook_service.go` - 3 instances
- `internal/core/watch_service.go` - 4 instances
- `internal/core/remote_explorer.go` - 1 instance
- `internal/core/vendor_syncer.go` - 9 instances

**Issue**: Output hardcoded to stdout instead of using the UICallback interface.

**Impact**:
- Testing difficulty (can't capture/verify output)
- Breaks non-interactive mode consistency
- Can't redirect output for logging

**Fix**: Replace `fmt.Printf` calls with `ui.ShowInfo()`, `ui.ShowProgress()`, or similar UICallback methods.

#### 1.4 Missing Sentinel Error Definitions

**Current State**: 125 `fmt.Errorf` calls across 30 files, only 1 custom error type (`JSONError`).

**Impact**:
- Error handling is string-based, requiring substring matching
- No type switching for error handling
- Harder to handle specific errors programmatically

**Recommended Error Types**:
```go
var (
    ErrVendorNotFound     = errors.New("vendor not found")
    ErrPathTraversal      = errors.New("path traversal not allowed")
    ErrConfigInvalid      = errors.New("invalid configuration")
    ErrLockFileCorrupt    = errors.New("corrupted lockfile")
    ErrNetworkFailure     = errors.New("network operation failed")
    ErrGitOperationFailed = errors.New("git operation failed")
)
```

#### 1.5 Hardcoded Magic Strings

**Scattered Across**:
- File paths: `"vendor/vendor.yml"`, `"vendor/vendor.lock"`, `"vendor/licenses/"`
- License names: `"MIT"`, `"Apache-2.0"`, `"BSD-3-Clause"`
- Git refs: `"FETCH_HEAD"`, `"main"`

**Fix**: Create `internal/core/constants.go`:
```go
const (
    ConfigPath   = "vendor/vendor.yml"
    LockPath     = "vendor/vendor.lock"
    LicensesDir  = "vendor/licenses"
    DefaultRef   = "main"
)

var AllowedLicenses = []string{
    "MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause",
    "ISC", "Unlicense", "CC0-1.0",
}
```

---

### TIER 2: Medium Effort

#### 2.1 TUI Package at 10.2% Coverage

**Location**: `internal/tui/wizard.go` (604 lines)

**Root Cause**: The `check(err)` helper function calls `os.Exit(1)`, making the entire wizard untestable:
```go
func check(err error) {
    if err != nil {
        fmt.Println("Aborted.")
        os.Exit(1)  // Makes testing impossible
    }
}
```

**Impact**: Critical user-facing code has virtually no tests.

**Fix Strategy**:
1. Replace `check(err)` with error returns
2. Extract form building logic into testable pure functions
3. Create `VendorManager` interface mock for testing
4. Add table-driven tests for URL validation, path mapping

#### 2.2 main.go Has Zero Test Coverage

**Stats**: 1091 lines, 64 `os.Exit()` calls

**Issue**: CLI dispatcher completely untested.

**Impact**:
- Command routing untested
- Flag parsing untested
- Exit codes untested
- Error message formatting untested

**Fix Strategy**:
1. Extract command handlers into a separate `cmd/handlers.go` file
2. Create `type App struct` that can be constructed with mocked dependencies
3. Add integration tests that capture stdout/stderr
4. Test exit codes via subprocess testing pattern

#### 2.3 Missing Edge Case Tests for Path Traversal

**Current Test**: `TestSyncVendor_PathTraversalBlocked` only tests `../../../etc/passwd`

**Missing Test Cases**:
```go
// Windows paths
{"C:\\Windows\\System32", true},
{"\\\\server\\share\\file", true},

// URL-encoded paths
{"%2e%2e%2fpasswd", true},
{"%252e%252e%252f", true},  // Double encoding

// Unicode edge cases
{"..／etc/passwd", true},  // Full-width slash U+FF0F
{"..＼etc/passwd", true},  // Full-width backslash U+FF3C

// Symlink considerations
{"./safe/../../../etc/passwd", true},

// Empty and whitespace
{"", false},  // Should this be allowed?
{"   ", true},  // Whitespace-only path
```

#### 2.4 Parallel Executor Discards Errors

**Location**: `internal/core/parallel_executor.go:97-99`

```go
if len(errors) > 0 {
    return allResults, errors[0]  // Other errors silently lost
}
```

**Missing Tests**:
- Multiple vendor failures (verify all errors are captured)
- Error ordering (is first or worst returned?)
- Partial success handling (some vendors succeed, some fail)

**Fix**: Either aggregate all errors or document the first-error-wins behavior explicitly.

#### 2.5 Hook Environment Variables Untested

**Location**: `internal/core/hook_service.go:88-115`

The `buildEnvironment()` function creates environment variables but they're never verified in tests:
- `GIT_VENDOR_NAME`
- `GIT_VENDOR_URL`
- `GIT_VENDOR_REF`
- `GIT_VENDOR_COMMIT`
- `GIT_VENDOR_ROOT`
- `GIT_VENDOR_FILES_COPIED`
- `GIT_VENDOR_DIRS_CREATED`

**Add Test**:
```go
func TestBuildEnvironment_AllVariablesSet(t *testing.T) {
    ctx := &types.HookContext{
        VendorName:  "test-vendor",
        VendorURL:   "https://github.com/test/repo",
        // ...
    }
    env := h.buildEnvironment(ctx)

    assertEnvContains(t, env, "GIT_VENDOR_NAME=test-vendor")
    // ... verify all variables
}
```

---

### TIER 3: Higher Effort (Architectural)

#### 3.1 VendorSyncer is a God Object

**Location**: `internal/core/vendor_syncer.go`

The VendorSyncer creates all services internally:
```go
type VendorSyncer struct {
    git      GitClient
    fs       FileSystem
    config   ConfigStore
    lock     LockStore
    license  LicenseChecker
    hooks    HookExecutor
    cache    CacheStore
    ui       UICallback
    validate *ValidationService
    update   *UpdateService
    sync     *SyncService
    diff     *DiffService
    // ... etc
}
```

**Impact**:
- Tight coupling between all services
- Difficult to test individual services in isolation
- Hard to swap implementations

**Fix**: Use constructor injection:
```go
func NewVendorSyncer(
    git GitClient,
    fs FileSystem,
    config ConfigStore,
    lock LockStore,
    sync SyncService,  // Pass as interface
    // ...
) *VendorSyncer
```

#### 3.2 Cache Fallback Can Use Stale Data

**Location**: `internal/core/verify_service.go:108-130`

```go
// If lockfile has no FileHashes, falls back to cache
if len(lockEntry.FileHashes) == 0 {
    expectedFiles = s.buildExpectedFilesFromCache(lockEntry)
}
```

**Issue**: No warning when using cache fallback; verification might pass with outdated expectations.

**Fix**:
1. Log a warning when using cache fallback
2. Verify cache commit hash matches lockfile commit hash
3. Optionally fail if commit hashes differ

#### 3.3 No Structured Logging

**Current State**: Mix of `fmt.Printf`, UICallback, and direct stdout writes.

**Impact**:
- No log levels (debug, info, warn, error)
- No structured output for debugging
- Hard to trace issues in production

**Recommendation**: Implement a Logger interface:
```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

#### 3.4 Validation Logic Scattered

**Locations**:
- `filesystem.go:121` - `ValidateDestPath()`
- `validation_service.go` - `ValidateConfig()`
- `wizard.go:56-65` - URL validation
- `git_operations.go` - ParseSmartURL validation

**Fix**: Centralize in `internal/validation/` package.

#### 3.5 Missing Race Condition Tests

**Claim**: Code passes `-race` flag
**Reality**: No explicit tests for race conditions

**Missing Tests**:
- Concurrent writes to results channel
- Lock map concurrent read/write
- Progress callback thread safety

---

### TIER 4: Design Concerns

#### 4.1 Hook Execution Has No Timeout

**Location**: `internal/core/hook_service.go:61-84`

```go
output, err := cmd.CombinedOutput()  // No timeout!
```

**Impact**: Malicious or buggy hooks can hang indefinitely.

**Fix**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
cmd := exec.CommandContext(ctx, "sh", "-c", command)
```

#### 4.2 Git Operations Have Inconsistent Timeouts

- `remote_explorer.go:25` uses 30s context
- Other git operations have no timeout

**Fix**: Add consistent timeout handling across all git operations.

#### 4.3 Watch Service No Graceful Shutdown

**Location**: `internal/core/watch_service.go`

Catches `os.Interrupt` but doesn't clean up properly.

**Fix**: Add proper context cancellation and cleanup.

#### 4.4 ParseSmartURL Only Fully Supports GitHub

**Location**: `internal/core/git_operations.go:183-261`

Deep URL parsing (extracting ref/path from blob/tree URLs) only works for GitHub.

**Impact**: GitLab/Bitbucket deep links won't auto-extract ref/path.

#### 4.5 No Retry Logic for Transient Failures

Git operations fail immediately on network errors.

**Fix**: Add configurable retry with exponential backoff.

---

## Priority Matrix

| Priority | Items | Effort |
|----------|-------|--------|
| **P0 - Critical** | TUI tests (2.1), main.go tests (2.2) | Medium |
| **P1 - High** | Types tests (1.1), Path traversal (2.3), Error types (1.4) | Low-Medium |
| **P2 - Medium** | Engine methods (1.2), Parallel errors (2.4), Hook env (2.5) | Low-Medium |
| **P3 - Low** | Magic strings (1.5), fmt.Print (1.3), Timeouts (4.1, 4.2) | Low |

---

## Recommendations

1. **Immediate**: Add test coverage for TUI and main.go (highest risk areas)
2. **Short-term**: Define error types and constants, add missing unit tests
3. **Medium-term**: Refactor VendorSyncer to use dependency injection
4. **Long-term**: Add structured logging, centralize validation

---

## Appendix: Commands Used

```bash
# Generate mocks
make mocks

# Run tests with coverage
go test -cover -coverprofile=coverage.out ./...

# View uncovered functions
go tool cover -func=coverage.out | grep "0.0%"

# Run race detector
go test -race ./...
```
