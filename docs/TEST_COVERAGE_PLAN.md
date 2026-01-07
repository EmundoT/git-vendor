# Test Coverage Plan for v1.0.0 Release

**Status**: Pre-release blocker
**Target**: Complete before v1.0.0 release
**Estimated Effort**: 4-6 hours

---

## Overview

This document details the test coverage gaps discovered during pre-release audit. While core functionality has 63 passing tests with solid coverage, several advanced features documented in `ADVANCED.md` lack unit tests.

**Current State**:
- ✅ Core sync/update: Well-tested (48.0% overall, 84-100% on critical paths)
- ✅ Multi-ref tracking: Tested
- ✅ Auto-naming: Tested
- ❌ **Custom Hooks: NOT TESTED** (security-sensitive - HIGH RISK)
- ❌ **Vendor Groups: NOT TESTED** (filtering logic - MEDIUM RISK)
- ⚠️ **Cache/Parallel: Only in benchmarks** (need proper unit tests - MEDIUM RISK)
- ⚠️ **Watch Mode: NOT TESTED** (simple wrapper - LOW RISK)

**Why This Matters**:
- Hooks execute arbitrary shell commands (security risk if broken)
- Groups filter vendors via CLI flags (user-facing feature)
- Cache and parallel have complex logic that could regress
- v1.0.0 should have confidence in all documented features

---

## Test Infrastructure Context

### Testing Framework
- **Framework**: Go standard `testing` package
- **Mocking**: `github.com/golang/mock/mockgen` (gomock)
- **Location**: Tests live alongside source in `internal/core/*_test.go`
- **Helpers**: `testhelpers_gomock_test.go` for mock setup utilities

### Existing Patterns to Follow

**Example: Sync service test structure (sync_service_test.go)**
```go
func TestSyncVendor_FeatureName(t *testing.T) {
    // Setup
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockGit := NewMockGitClient(ctrl)
    mockFS := NewMockFileSystem(ctrl)
    // ... other mocks

    syncer := createMockSyncer(ctrl, mockGit, mockFS, ...)

    // Configure mock expectations
    mockGit.EXPECT().Clone(gomock.Any(), gomock.Any()).Return(nil)

    // Execute
    err := syncer.SyncVendor(...)

    // Assert
    if err != nil {
        t.Fatalf("Expected success, got error: %v", err)
    }
}
```

**Helper function pattern (testhelpers_gomock_test.go)**
```go
func createMockSyncer(ctrl *gomock.Controller, git GitClient, fs FileSystem, ...) *VendorSyncer {
    return &VendorSyncer{
        git: git,
        fs: fs,
        // ... inject mocks
    }
}
```

### Running Tests

```bash
# Generate mocks (required before running tests)
make mocks

# Run all tests
go test ./internal/core

# Run specific test
go test ./internal/core -run TestHookService_PreSyncExecution

# Run with coverage
go test -cover ./internal/core

# Run with race detector
go test -race ./internal/core
```

---

## Priority 1: Custom Hooks Tests (CRITICAL - Security Sensitive)

### Files Involved
- **Implementation**: `internal/core/hook_service.go` (109 lines)
- **Types**: `internal/types/types.go` (HookConfig, HookContext)
- **Test file to create**: `internal/core/hook_service_test.go`

### What Hooks Do
Custom hooks execute shell commands before/after vendor sync:
- **Pre-sync**: Runs before git clone (e.g., cleanup, preparation)
- **Post-sync**: Runs after file copy (e.g., npm install, build)
- **Environment**: Injects GIT_VENDOR_* environment variables
- **Execution**: Uses `sh -c` for full shell support (pipes, multiline)

### Interface to Test
```go
type HookExecutor interface {
    ExecutePreSync(vendor *types.VendorSpec, ctx *types.HookContext) error
    ExecutePostSync(vendor *types.VendorSpec, ctx *types.HookContext) error
}
```

### Required Test Cases

#### Test 1: Pre-sync hook execution
```go
func TestHookService_PreSyncExecution(t *testing.T)
```
**Setup**:
- Vendor with `hooks.pre_sync: "echo 'pre-sync executed'"`
- HookContext with test values
**Execute**: `ExecutePreSync(vendor, ctx)`
**Assert**: Command executed successfully, output captured

#### Test 2: Post-sync hook execution
```go
func TestHookService_PostSyncExecution(t *testing.T)
```
**Setup**:
- Vendor with `hooks.post_sync: "echo 'post-sync executed'"`
**Assert**: Command executed successfully

#### Test 3: Environment variable injection
```go
func TestHookService_EnvironmentVariables(t *testing.T)
```
**Setup**:
- Hook command: `"env | grep GIT_VENDOR"`
- HookContext with known values
**Assert**: All expected env vars present (GIT_VENDOR_NAME, URL, REF, COMMIT, ROOT, FILES_COPIED)

#### Test 4: Multiline hook support
```go
func TestHookService_MultilineCommand(t *testing.T)
```
**Setup**:
- Hook with multiple lines (e.g., `"echo line1\necho line2"`)
**Assert**: Both commands executed

#### Test 5: Hook failure handling
```go
func TestHookService_CommandFailure(t *testing.T)
```
**Setup**:
- Hook command: `"exit 1"`
**Assert**: ExecutePreSync returns error

#### Test 6: No hook configured (no-op)
```go
func TestHookService_NoHookConfigured(t *testing.T)
```
**Setup**:
- Vendor with `hooks: nil`
**Assert**: ExecutePreSync returns nil (no error)

#### Test 7: Empty hook command (no-op)
```go
func TestHookService_EmptyHookCommand(t *testing.T)
```
**Setup**:
- Vendor with `hooks.pre_sync: ""`
**Assert**: No execution, no error

#### Test 8: Working directory validation
```go
func TestHookService_WorkingDirectory(t *testing.T)
```
**Setup**:
- Hook command: `"pwd"`
- HookContext with specific RootDir
**Assert**: Command runs in RootDir

### Test Implementation Example

```go
package core

import (
    "testing"
    "github.com/EmundoT/git-vendor/internal/types"
)

func TestHookService_PreSyncExecution(t *testing.T) {
    // Setup
    ui := &testUICallback{}
    hookService := NewHookService(ui)

    vendor := &types.VendorSpec{
        Name: "test-vendor",
        Hooks: &types.HookConfig{
            PreSync: "echo 'pre-sync test'",
        },
    }

    ctx := &types.HookContext{
        VendorName: "test-vendor",
        VendorURL:  "https://github.com/test/repo",
        Ref:        "main",
        CommitHash: "abc123",
        RootDir:    "/tmp/test",
        FilesCopied: 5,
    }

    // Execute
    err := hookService.ExecutePreSync(vendor, ctx)

    // Assert
    if err != nil {
        t.Fatalf("Expected success, got error: %v", err)
    }
}

func TestHookService_EnvironmentVariables(t *testing.T) {
    ui := &testUICallback{}
    hookService := NewHookService(ui)

    vendor := &types.VendorSpec{
        Name: "env-test",
        Hooks: &types.HookConfig{
            PreSync: "env | grep GIT_VENDOR",
        },
    }

    ctx := &types.HookContext{
        VendorName:  "env-test",
        VendorURL:   "https://github.com/test/repo",
        Ref:         "main",
        CommitHash:  "def456",
        RootDir:     "/tmp/test",
        FilesCopied: 10,
    }

    // Execute
    err := hookService.ExecutePreSync(vendor, ctx)

    // Assert
    if err != nil {
        t.Fatalf("Expected success, got error: %v", err)
    }

    // Note: To properly test env vars, you'd need to capture output
    // and verify GIT_VENDOR_NAME, GIT_VENDOR_URL, etc. are present
}

func TestHookService_CommandFailure(t *testing.T) {
    ui := &testUICallback{}
    hookService := NewHookService(ui)

    vendor := &types.VendorSpec{
        Name: "fail-test",
        Hooks: &types.HookConfig{
            PreSync: "exit 1",
        },
    }

    ctx := &types.HookContext{
        VendorName: "fail-test",
        RootDir:    "/tmp/test",
    }

    // Execute
    err := hookService.ExecutePreSync(vendor, ctx)

    // Assert
    if err == nil {
        t.Fatal("Expected error for failing command, got nil")
    }

    if !contains(err.Error(), "hook failed") {
        t.Errorf("Expected 'hook failed' error, got: %v", err)
    }
}

// Helper
func contains(s, substr string) bool {
    return len(s) >= len(substr) &&
           (s == substr || len(s) > len(substr) &&
            (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

// testUICallback for testing
type testUICallback struct{}
func (t *testUICallback) ShowSpinner(msg string) {}
func (t *testUICallback) UpdateSpinner(msg string) {}
func (t *testUICallback) StopSpinner(success bool, msg string) {}
func (t *testUICallback) ShowError(title, msg string) {}
func (t *testUICallback) ShowWarning(msg string) {}
```

---

## Priority 2: Vendor Groups Tests (MEDIUM - User-Facing Feature)

### Files Involved
- **Implementation**: `internal/core/sync_service.go` (lines 87-88, 265-268)
- **Types**: `internal/types/types.go` (VendorSpec.Groups field)
- **CLI**: `main.go` (--group flag handling)
- **Test file to update**: `internal/core/sync_service_test.go`

### What Groups Do
Vendors can be tagged with groups for batch operations:
```yaml
vendors:
  - name: frontend-lib
    groups: ["frontend", "ui"]
  - name: backend-api
    groups: ["backend"]
```

Usage: `git-vendor sync --group frontend` (syncs only vendors with "frontend" tag)

### Implementation Details
```go
// In sync_service.go
if opts.GroupName != "" {
    if err := s.validateGroupExists(config, opts.GroupName); err != nil {
        return err
    }
}

// Filter vendors by group
if opts.GroupName != "" {
    hasGroup := false
    for _, g := range vendor.Groups {
        if g == opts.GroupName {
            hasGroup = true
            break
        }
    }
    if !hasGroup {
        continue // Skip vendor
    }
}
```

### Required Test Cases

#### Test 1: Sync single group
```go
func TestSync_GroupFilter_SingleGroup(t *testing.T)
```
**Setup**:
- 3 vendors: vendor-a (groups: ["frontend"]), vendor-b (groups: ["backend"]), vendor-c (groups: ["frontend", "backend"])
- SyncOptions with GroupName: "frontend"
**Execute**: Sync(opts)
**Assert**: Only vendor-a and vendor-c synced (vendor-b skipped)

#### Test 2: Sync different group
```go
func TestSync_GroupFilter_BackendGroup(t *testing.T)
```
**Setup**: Same 3 vendors, GroupName: "backend"
**Assert**: Only vendor-b and vendor-c synced

#### Test 3: Group not found error
```go
func TestSync_GroupFilter_NonexistentGroup(t *testing.T)
```
**Setup**: Vendors with groups ["frontend", "backend"], GroupName: "mobile"
**Assert**: Error returned (group doesn't exist)

#### Test 4: Vendor with no groups
```go
func TestSync_GroupFilter_VendorWithoutGroups(t *testing.T)
```
**Setup**: Vendor with Groups: nil, GroupName: "frontend"
**Assert**: Vendor skipped

#### Test 5: Vendor with multiple groups
```go
func TestSync_GroupFilter_MultipleGroups(t *testing.T)
```
**Setup**: Vendor with Groups: ["frontend", "backend", "mobile"]
**Assert**: Matches any of the groups when filtering

#### Test 6: Empty group name (all vendors)
```go
func TestSync_GroupFilter_EmptyGroupName(t *testing.T)
```
**Setup**: GroupName: ""
**Assert**: All vendors synced (no filtering)

### Test Implementation Example

```go
func TestSync_GroupFilter_SingleGroup(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockGit := NewMockGitClient(ctrl)
    mockFS := NewMockFileSystem(ctrl)
    mockConfig := NewMockConfigStore(ctrl)
    mockLock := NewMockLockStore(ctrl)
    mockLicense := NewMockLicenseChecker(ctrl)

    syncer := createMockSyncer(ctrl, mockGit, mockFS, mockConfig, mockLock, mockLicense)

    config := &types.VendorConfig{
        Vendors: []types.VendorSpec{
            {
                Name:   "vendor-a",
                Groups: []string{"frontend"},
                Specs: []types.BranchSpec{{
                    Ref:     "main",
                    Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
                }},
            },
            {
                Name:   "vendor-b",
                Groups: []string{"backend"},
                Specs: []types.BranchSpec{{
                    Ref:     "main",
                    Mapping: []types.PathMapping{{From: "src/", To: "lib-b/"}},
                }},
            },
            {
                Name:   "vendor-c",
                Groups: []string{"frontend", "backend"},
                Specs: []types.BranchSpec{{
                    Ref:     "main",
                    Mapping: []types.PathMapping{{From: "src/", To: "lib-c/"}},
                }},
            },
        },
    }

    lock := &types.VendorLock{Vendors: []types.LockDetails{}}

    mockConfig.EXPECT().LoadConfig().Return(config, nil)
    mockLock.EXPECT().LoadLock().Return(lock, nil)

    // Only vendor-a and vendor-c should be synced (have "frontend" group)
    // Mock expectations for vendor-a
    mockGit.EXPECT().Clone(gomock.Any(), gomock.Any()).Return(nil)
    // ... etc for vendor-a

    // Mock expectations for vendor-c
    mockGit.EXPECT().Clone(gomock.Any(), gomock.Any()).Return(nil)
    // ... etc for vendor-c

    // NO expectations for vendor-b (should be skipped)

    opts := SyncOptions{
        GroupName: "frontend",
    }

    err := syncer.Sync(opts)

    if err != nil {
        t.Fatalf("Expected success, got error: %v", err)
    }

    // Verify only frontend vendors were synced
    // (gomock will fail if vendor-b mocks are called)
}
```

---

## Priority 3: Move Cache and Parallel Tests from Benchmarks

### Files Involved
- **Cache Implementation**: `internal/core/cache_store.go`
- **Parallel Implementation**: `internal/core/parallel_executor.go`
- **Current Tests**: `internal/core/benchmark_test.go` (only performance testing)
- **Test files to create/update**: `internal/core/cache_store_test.go`, `internal/core/parallel_executor_test.go`

### Cache Tests Needed

#### Test 1: Cache save and load
```go
func TestCacheStore_SaveAndLoad(t *testing.T)
```
**Setup**: CacheEntry with known checksums
**Execute**: SaveCache(), then LoadCache()
**Assert**: Loaded cache matches saved cache

#### Test 2: Cache hit detection
```go
func TestCacheStore_ValidateCache_Hit(t *testing.T)
```
**Setup**: Cache with matching commit hash and file checksums
**Assert**: ValidateCache returns true

#### Test 3: Cache miss (commit changed)
```go
func TestCacheStore_ValidateCache_CommitMismatch(t *testing.T)
```
**Setup**: Cache with old commit hash
**Assert**: ValidateCache returns false

#### Test 4: Cache miss (file modified)
```go
func TestCacheStore_ValidateCache_FileModified(t *testing.T)
```
**Setup**: Cache with old file checksum
**Assert**: ValidateCache returns false

#### Test 5: Cache graceful failure
```go
func TestCacheStore_LoadCache_FileNotFound(t *testing.T)
```
**Setup**: Cache file doesn't exist
**Assert**: LoadCache returns empty cache, no error

### Parallel Tests Needed

#### Test 1: Parallel sync with multiple vendors
```go
func TestParallelExecutor_SyncMultipleVendors(t *testing.T)
```
**Setup**: 5 vendors
**Execute**: ExecuteParallelSync with 2 workers
**Assert**: All vendors synced, lockfile aggregated

#### Test 2: Worker count limiting
```go
func TestParallelExecutor_WorkerCountLimit(t *testing.T)
```
**Setup**: Request 100 workers
**Assert**: Workers capped at MaxWorkers (8)

#### Test 3: Fail-fast on error
```go
func TestParallelExecutor_FailFast(t *testing.T)
```
**Setup**: 5 vendors, vendor 2 fails
**Execute**: ExecuteParallelSync
**Assert**: Error returned, remaining vendors may not complete

#### Test 4: Thread safety (race detector)
```go
func TestParallelExecutor_ThreadSafety(t *testing.T)
```
**Setup**: 10 vendors
**Execute**: Run with `-race` flag
**Assert**: No race conditions detected

---

## Priority 4: Watch Mode Tests (OPTIONAL - Low Risk)

### Files Involved
- **Implementation**: `internal/core/watch_service.go` (simple fsnotify wrapper)
- **Test file to create**: `internal/core/watch_service_test.go`

**Note**: Watch mode is a thin wrapper around `fsnotify`. Tests would primarily validate:
- File watching setup
- Change detection
- Debounce logic

This is **OPTIONAL** for v1.0.0 - low risk due to simple implementation.

---

## Success Criteria

Before marking this task complete, verify:

- [ ] All Priority 1 tests (Hooks) written and passing
- [ ] All Priority 2 tests (Groups) written and passing
- [ ] Priority 3 tests (Cache, Parallel) written and passing
- [ ] `make mocks` generates required mocks
- [ ] `go test ./internal/core` passes all tests
- [ ] `go test -race ./internal/core` passes with no race conditions
- [ ] Coverage report shows improved coverage for hook_service.go, sync_service.go (group logic)
- [ ] All new tests follow existing patterns in sync_service_test.go

### Verification Commands

```bash
# Generate mocks
make mocks

# Run all tests
go test -v ./internal/core

# Run with race detector
go test -race ./internal/core

# Check coverage
go test -cover ./internal/core

# Run specific new tests
go test ./internal/core -run TestHookService
go test ./internal/core -run TestSync_GroupFilter
go test ./internal/core -run TestCacheStore
go test ./internal/core -run TestParallelExecutor
```

---

## Estimated Timeline

- **Priority 1 (Hooks)**: 2-3 hours (8 tests, security-critical)
- **Priority 2 (Groups)**: 1-2 hours (6 tests, straightforward)
- **Priority 3 (Cache/Parallel)**: 1-2 hours (move from benchmarks)
- **Total**: 4-7 hours

---

## Notes for Implementation

1. **Follow existing patterns**: Look at `sync_service_test.go` for mock setup patterns
2. **Use testhelpers**: Leverage `createMockSyncer()` and similar helpers
3. **Mock expectations**: Use gomock.EXPECT() to verify calls
4. **Error cases matter**: Test both success and failure paths
5. **Race detector**: Run with `-race` to catch concurrency issues
6. **Keep tests focused**: One test = one scenario
7. **Clear names**: Test names should describe what they validate

---

## Reference Files

**Existing test patterns**:
- `internal/core/sync_service_test.go` - Mock setup, vendor sync tests
- `internal/core/update_service_test.go` - Update service patterns
- `internal/core/validation_service_test.go` - Validation logic tests
- `internal/core/testhelpers_gomock_test.go` - Mock setup helpers

**Implementation files to test**:
- `internal/core/hook_service.go` - Hook execution logic
- `internal/core/sync_service.go` - Group filtering (lines 87-88, 265-268)
- `internal/core/cache_store.go` - Cache validation logic
- `internal/core/parallel_executor.go` - Worker pool implementation

**Type definitions**:
- `internal/types/types.go` - VendorSpec, HookConfig, HookContext, SyncOptions

---

## Contact / Questions

This document is self-contained for handoff to a fresh AI instance. If you encounter issues:

1. Check existing test files for patterns
2. Run `make mocks` if mock generation fails
3. Verify go.mod has correct module path: `github.com/EmundoT/git-vendor`
4. All tests should follow gomock patterns (see testhelpers_gomock_test.go)
