# Audit Execution Plan

**Generated**: 2026-02-05
**PM Branch**: claude/audit-features-tests-ly7GY
**Total Prompts**: 12
**Estimated Parallel Groups**: 5

## Execution Order Summary

```
[Group A: Foundations - No Dependencies] ─── Run First
├── PROMPT 1: AUDIT-01 (Constants & Error Types)
└── PROMPT 2: AUDIT-02 (Types Package Tests)

[Group B: Depends on Group A] ─── After Group A
├── PROMPT 3: AUDIT-03 (Engine Facade Tests)
├── PROMPT 4: AUDIT-04 (Path Traversal Tests)
├── PROMPT 5: AUDIT-05 (Hook Env Tests)
└── PROMPT 6: AUDIT-06 (Parallel Executor Fix)

[Group C: Independent] ─── Can run alongside Group B
├── PROMPT 7: AUDIT-07 (fmt.Print Cleanup)
└── PROMPT 8: AUDIT-08 (Timeout Consistency)

[Group D: Requires A-C] ─── After Groups A-C
├── PROMPT 9: AUDIT-09 (TUI Refactoring)
└── PROMPT 10: AUDIT-10 (main.go Tests)

[Group E: After B] ─── After Group B
├── PROMPT 11: AUDIT-11 (Cache Warning)
└── PROMPT 12: AUDIT-12 (Race Condition Tests)
```

## Prompt Summary Table

| # | ID | Title | Effort | Dependencies | Group |
|---|-----|-------|--------|--------------|-------|
| 1 | AUDIT-01 | Constants, Error Types & Magic Strings | Low | None | A |
| 2 | AUDIT-02 | Types Package Tests | Low | None | A |
| 3 | AUDIT-03 | Engine Facade Method Tests | Low | AUDIT-01 | B |
| 4 | AUDIT-04 | Path Traversal Edge Case Tests | Medium | AUDIT-01 | B |
| 5 | AUDIT-05 | Hook Environment Variable Tests | Low | None | B |
| 6 | AUDIT-06 | Parallel Executor Error Aggregation | Medium | None | B |
| 7 | AUDIT-07 | fmt.Print → UICallback Migration | Medium | None | C |
| 8 | AUDIT-08 | Timeout Consistency (Hook/Git/Watch) | Medium | None | C |
| 9 | AUDIT-09 | TUI Refactoring & Tests | High | AUDIT-01 | D |
| 10 | AUDIT-10 | main.go Test Infrastructure | High | AUDIT-01, AUDIT-09 | D |
| 11 | AUDIT-11 | Cache Stale Data Warning | Low | None | E |
| 12 | AUDIT-12 | Race Condition Tests | Medium | AUDIT-06 | E |

## Concurrent Execution Strategy

### Phase 1: Run Group A (2 prompts in parallel)
- **PROMPT 1** and **PROMPT 2** have no dependencies
- Both can be dispatched simultaneously
- Expected completion: Fast (Low effort each)

### Phase 2: Run Groups B + C (6 prompts in parallel)
- After Group A completes, dispatch all of B and C together
- **PROMPT 3, 4, 5, 6** (Group B) + **PROMPT 7, 8** (Group C)
- 6 independent work streams
- Expected completion: Medium

### Phase 3: Run Groups D + E (4 prompts in parallel)
- After Phase 2 completes, dispatch all of D and E
- **PROMPT 9, 10** (Group D) + **PROMPT 11, 12** (Group E)
- 4 independent work streams
- Expected completion: High effort for D, Low-Medium for E

## Total Concurrency

| Phase | Prompts | Max Parallel |
|-------|---------|--------------|
| 1 | 1, 2 | 2 |
| 2 | 3, 4, 5, 6, 7, 8 | 6 |
| 3 | 9, 10, 11, 12 | 4 |

**Maximum concurrent agents**: 6 (in Phase 2)

---

## Prompts

### PROMPT 1: AUDIT-01 - Constants, Error Types & Magic Strings

```
TASK: AUDIT-01 - Create Constants Package and Sentinel Error Types

CONTEXT: The codebase has 125 fmt.Errorf calls but only 1 custom error type. Magic strings like "vendor/vendor.yml" are repeated across multiple files. This makes error handling brittle and constants hard to maintain.

SCOPE:
- Create: internal/core/constants.go
- Create: internal/core/errors.go
- Modify: Files that use magic strings (update imports, use constants)

IMPLEMENTATION:

1. Create internal/core/constants.go:
```go
package core

// File paths
const (
    VendorDir     = "vendor"
    ConfigFile    = "vendor.yml"
    LockFile      = "vendor.lock"
    LicensesDir   = "licenses"
    CacheDir      = ".cache"

    ConfigPath    = VendorDir + "/" + ConfigFile
    LockPath      = VendorDir + "/" + LockFile
    LicensesPath  = VendorDir + "/" + LicensesDir
    CachePath     = VendorDir + "/" + CacheDir
)

// Git refs
const (
    DefaultRef  = "main"
    FetchHead   = "FETCH_HEAD"
)

// Allowed licenses (SPDX identifiers)
var AllowedLicenses = []string{
    "MIT",
    "Apache-2.0",
    "BSD-3-Clause",
    "BSD-2-Clause",
    "ISC",
    "Unlicense",
    "CC0-1.0",
}

// License file names to check (in order)
var LicenseFileNames = []string{
    "LICENSE",
    "LICENSE.txt",
    "LICENSE.md",
    "COPYING",
}
```

2. Create internal/core/errors.go:
```go
package core

import "errors"

// Sentinel errors for type-based error handling
var (
    ErrVendorNotFound     = errors.New("vendor not found")
    ErrPathTraversal      = errors.New("path traversal not allowed")
    ErrConfigInvalid      = errors.New("invalid configuration")
    ErrLockFileCorrupt    = errors.New("corrupted lockfile")
    ErrLockFileMissing    = errors.New("lockfile not found")
    ErrNetworkFailure     = errors.New("network operation failed")
    ErrGitOperationFailed = errors.New("git operation failed")
    ErrLicenseNotAllowed  = errors.New("license not in allowed list")
    ErrHookFailed         = errors.New("hook execution failed")
    ErrCacheStale         = errors.New("cache is stale")
    ErrHookTimeout        = errors.New("hook execution timed out")
)
```

3. Update existing files to use constants:
- Replace "vendor/vendor.yml" with core.ConfigPath
- Replace "vendor/vendor.lock" with core.LockPath
- Replace "vendor/licenses/" with core.LicensesPath

4. Update error returns to wrap sentinel errors:
- Change: fmt.Errorf("vendor '%s' not found", name)
- To: fmt.Errorf("%w: %s", ErrVendorNotFound, name)

VERIFICATION:
```bash
# Constants defined
grep -r "ConfigPath\|LockPath" internal/core/

# Errors defined
grep -r "ErrVendorNotFound\|ErrPathTraversal" internal/core/

# Tests still pass
go test ./...
```

ACCEPTANCE CRITERIA:
- [ ] constants.go created with all file paths
- [ ] errors.go created with sentinel errors
- [ ] At least 10 magic string usages replaced with constants
- [ ] At least 5 error returns wrapped with sentinel errors
- [ ] All tests pass

Commit with message: "refactor: add constants package and sentinel error types"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 2: AUDIT-02 - Types Package Tests

```
TASK: AUDIT-02 - Add Comprehensive Tests for Types Package

CONTEXT: internal/types/types.go has 0% test coverage. These are critical data structures used throughout the codebase. We need to verify YAML/JSON marshalling works correctly.

SCOPE:
- Create: internal/types/types_test.go

IMPLEMENTATION:

Create internal/types/types_test.go with tests for:

1. VendorConfig YAML marshalling/unmarshalling
2. VendorLock JSON marshalling (for --json output)
3. VerifyResult JSON marshalling
4. Edge cases (empty slices, nil pointers, omitempty behavior)
5. Schema version handling

Test examples:
```go
func TestVendorConfig_YAML_RoundTrip(t *testing.T) {
    config := VendorConfig{
        Vendors: []VendorSpec{
            {
                Name:    "test-vendor",
                URL:     "https://github.com/test/repo",
                License: "MIT",
                Groups:  []string{"frontend", "backend"},
                Specs: []BranchSpec{
                    {
                        Ref: "main",
                        Mapping: []PathMapping{
                            {From: "src/", To: "lib/"},
                        },
                    },
                },
            },
        },
    }

    data, err := yaml.Marshal(config)
    require.NoError(t, err)

    var parsed VendorConfig
    err = yaml.Unmarshal(data, &parsed)
    require.NoError(t, err)

    assert.Equal(t, config, parsed)
}

func TestVendorLock_JSON_RoundTrip(t *testing.T)
func TestLockDetails_JSON_OmitEmpty(t *testing.T)
func TestVerifyResult_JSON_Structure(t *testing.T)
func TestFileStatus_AllStatuses(t *testing.T)
func TestVendorConfig_EmptyVendors(t *testing.T)
func TestVendorSpec_NoGroups(t *testing.T)
func TestPathMapping_EmptyTo(t *testing.T)
func TestHookConfig_NilHooks(t *testing.T)
func TestVendorLock_SchemaVersion(t *testing.T)
```

VERIFICATION:
```bash
go test -v ./internal/types/
go test -cover ./internal/types/
```

ACCEPTANCE CRITERIA:
- [ ] types_test.go created
- [ ] YAML round-trip tests for VendorConfig, VendorSpec
- [ ] JSON round-trip tests for VendorLock, VerifyResult
- [ ] Edge case tests for empty/nil values
- [ ] Coverage > 80% for types package
- [ ] All tests pass

Commit with message: "test: add comprehensive tests for types package"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 3: AUDIT-03 - Engine Facade Method Tests

```
TASK: AUDIT-03 - Add Tests for 0% Coverage Engine Methods

CONTEXT: Several engine.go facade methods have 0% test coverage: Init(), FetchRepoDir(), ListLocalDir(), RemoveVendor(). These are user-facing operations that need tests.

SCOPE:
- Create/Modify: internal/core/engine_test.go

IMPLEMENTATION:

Add tests for each 0% coverage method using the existing mock infrastructure:

1. Init() tests:
- TestManager_Init_CreatesDirectories
- TestManager_Init_AlreadyExists
- TestManager_Init_PermissionError

2. FetchRepoDir() tests:
- TestManager_FetchRepoDir_Success
- TestManager_FetchRepoDir_InvalidURL
- TestManager_FetchRepoDir_Timeout

3. ListLocalDir() tests:
- TestManager_ListLocalDir_Success
- TestManager_ListLocalDir_NotExists
- TestManager_ListLocalDir_NotDirectory

4. RemoveVendor() tests:
- TestManager_RemoveVendor_Success
- TestManager_RemoveVendor_NotFound
- TestManager_RemoveVendor_RemovesLicense

5. Path() method (config_store.go:28):
- TestFileConfigStore_Path

VERIFICATION:
```bash
go test -v ./internal/core/ -run "TestManager_Init\|TestManager_FetchRepoDir\|TestManager_ListLocalDir\|TestManager_RemoveVendor"
go test -cover ./internal/core/
```

ACCEPTANCE CRITERIA:
- [ ] Init() has 3+ test cases
- [ ] FetchRepoDir() has 3+ test cases
- [ ] ListLocalDir() has 3+ test cases
- [ ] RemoveVendor() has 3+ test cases
- [ ] All tests use mocks (no real git operations)
- [ ] All tests pass

Commit with message: "test: add tests for engine facade methods"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 4: AUDIT-04 - Path Traversal Edge Case Tests

```
TASK: AUDIT-04 - Add Comprehensive Path Traversal Security Tests

CONTEXT: ValidateDestPath() in filesystem.go only has one basic test. Security-critical code needs comprehensive edge case testing including Windows paths, URL encoding, and Unicode attacks.

SCOPE:
- Modify: internal/core/filesystem_test.go (or create if not exists)

IMPLEMENTATION:

Create comprehensive table-driven tests with 20+ edge cases:

```go
func TestValidateDestPath_SecurityEdgeCases(t *testing.T) {
    tests := []struct {
        name      string
        path      string
        wantError bool
        errorMsg  string
    }{
        // Basic traversal
        {"simple traversal", "../etc/passwd", true, "path traversal"},
        {"deep traversal", "../../../etc/passwd", true, "path traversal"},
        {"hidden traversal", "foo/../../../etc/passwd", true, "path traversal"},

        // Windows absolute paths
        {"windows drive C", "C:\\Windows\\System32", true, "absolute"},
        {"windows drive lowercase", "c:\\windows", true, "absolute"},
        {"windows UNC path", "\\\\server\\share\\file", true, "absolute"},

        // Unix absolute paths
        {"unix absolute", "/etc/passwd", true, "absolute"},
        {"unix root", "/", true, "absolute"},

        // Valid paths
        {"relative simple", "lib/file.go", false, ""},
        {"relative nested", "vendor/pkg/src/file.go", false, ""},
        {"current dir", "./file.go", false, ""},
        {"dots in name", "file..name.go", false, ""},

        // Edge cases to document behavior
        {"empty path", "", false, ""},
        {"dot only", ".", false, ""},
        {"double dot file", "..file", false, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateDestPath(tt.path)
            if tt.wantError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

VERIFICATION:
```bash
go test -v ./internal/core/ -run "TestValidateDestPath"
```

ACCEPTANCE CRITERIA:
- [ ] 20+ edge case tests added
- [ ] Windows path variants tested
- [ ] Unix absolute paths tested
- [ ] All tests pass

Commit with message: "test: add comprehensive path traversal security tests"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 5: AUDIT-05 - Hook Environment Variable Tests

```
TASK: AUDIT-05 - Add Tests for Hook Environment Variables

CONTEXT: hook_service.go buildEnvironment() creates environment variables for hooks but they're never verified in tests. This is important for users who depend on these variables in their hook scripts.

SCOPE:
- Modify: internal/core/hook_service_test.go (or create)

IMPLEMENTATION:

```go
func TestBuildEnvironment_AllVariablesSet(t *testing.T) {
    h := &HookService{}
    ctx := &HookContext{
        VendorName:   "test-vendor",
        VendorURL:    "https://github.com/test/repo",
        Ref:          "main",
        CommitHash:   "abc123def456",
        RootDir:      "/project/root",
        FilesCopied:  42,
        DirsCreated:  5,
    }

    env := h.buildEnvironment(ctx)

    envMap := make(map[string]string)
    for _, e := range env {
        parts := strings.SplitN(e, "=", 2)
        if len(parts) == 2 {
            envMap[parts[0]] = parts[1]
        }
    }

    assert.Equal(t, "test-vendor", envMap["GIT_VENDOR_NAME"])
    assert.Equal(t, "https://github.com/test/repo", envMap["GIT_VENDOR_URL"])
    assert.Equal(t, "main", envMap["GIT_VENDOR_REF"])
    assert.Equal(t, "abc123def456", envMap["GIT_VENDOR_COMMIT"])
    assert.Equal(t, "/project/root", envMap["GIT_VENDOR_ROOT"])
    assert.Equal(t, "42", envMap["GIT_VENDOR_FILES_COPIED"])
    assert.Equal(t, "5", envMap["GIT_VENDOR_DIRS_CREATED"])
}

func TestBuildEnvironment_InheritsPATH(t *testing.T)
func TestBuildEnvironment_EmptyValues(t *testing.T)
```

VERIFICATION:
```bash
go test -v ./internal/core/ -run "TestBuildEnvironment"
```

ACCEPTANCE CRITERIA:
- [ ] All 7 environment variables verified
- [ ] PATH inheritance tested
- [ ] Empty value handling tested
- [ ] All tests pass

Commit with message: "test: add hook environment variable tests"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 6: AUDIT-06 - Parallel Executor Error Aggregation

```
TASK: AUDIT-06 - Fix Parallel Executor Error Handling and Add Tests

CONTEXT: parallel_executor.go only returns the first error, silently discarding others. This makes debugging multi-vendor failures difficult.

SCOPE:
- Modify: internal/core/parallel_executor.go
- Modify/Create: internal/core/parallel_executor_test.go

IMPLEMENTATION:

1. Add MultiVendorError type:
```go
type MultiVendorError struct {
    Errors []error
}

func (e *MultiVendorError) Error() string {
    if len(e.Errors) == 1 {
        return e.Errors[0].Error()
    }
    var msgs []string
    for _, err := range e.Errors {
        msgs = append(msgs, err.Error())
    }
    return fmt.Sprintf("%d vendors failed: %s", len(e.Errors), strings.Join(msgs, "; "))
}

func (e *MultiVendorError) Unwrap() []error {
    return e.Errors
}
```

2. Update error return:
```go
// Change:
if len(errors) > 0 {
    return allResults, errors[0]
}

// To:
if len(errors) > 0 {
    return allResults, &MultiVendorError{Errors: errors}
}
```

3. Add tests:
- TestParallelExecutor_SingleFailure
- TestParallelExecutor_MultipleFailures
- TestParallelExecutor_PartialSuccess
- TestParallelExecutor_AllFail
- TestMultiVendorError_SingleError
- TestMultiVendorError_MultipleErrors
- TestMultiVendorError_Unwrap

VERIFICATION:
```bash
go test -v ./internal/core/ -run "TestParallelExecutor\|TestMultiVendorError"
go test -race ./internal/core/
```

ACCEPTANCE CRITERIA:
- [ ] MultiVendorError type created
- [ ] All errors aggregated (not just first)
- [ ] Error message shows count and all messages
- [ ] Unwrap() returns all errors
- [ ] 5+ test cases
- [ ] Race detector passes
- [ ] All tests pass

Commit with message: "fix: aggregate all errors in parallel executor"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 7: AUDIT-07 - fmt.Print → UICallback Migration

```
TASK: AUDIT-07 - Replace Direct fmt.Print with UICallback

CONTEXT: 40+ direct fmt.Printf calls bypass UICallback, breaking non-interactive mode and making testing difficult.

SCOPE:
- Modify: internal/core/sync_service.go (~15 instances)
- Modify: internal/core/vendor_syncer.go (~9 instances)
- Modify: internal/core/hook_service.go (~3 instances)
- Modify: internal/core/watch_service.go (~4 instances)
- Modify: internal/core/remote_explorer.go (~1 instance)

IMPLEMENTATION:

1. Add methods to UICallback if missing:
```go
type UICallback interface {
    ShowProgress(message string)
    ShowVendorStatus(name string, status string)
}
```

2. Replace fmt.Printf patterns:
```go
// Before:
fmt.Printf("Syncing %s...\n", Pluralize(vendorCount, "vendor", "vendors"))
// After:
s.ui.ShowProgress(fmt.Sprintf("Syncing %s...", Pluralize(vendorCount, "vendor", "vendors")))

// Before:
fmt.Printf("✓ %s\n", v.Name)
// After:
s.ui.ShowVendorStatus(v.Name, "synced")
```

3. Update NonInteractiveTUICallback to handle new methods.

VERIFICATION:
```bash
# No direct fmt.Print in core services (except test files)
grep -r "fmt\.Print" internal/core/*.go | grep -v "_test.go" | wc -l
# Should be 0 or close to 0

go test ./...
```

ACCEPTANCE CRITERIA:
- [ ] Zero fmt.Printf in sync_service.go (non-test)
- [ ] Zero fmt.Printf in vendor_syncer.go (non-test)
- [ ] Zero fmt.Printf in hook_service.go (non-test)
- [ ] Zero fmt.Printf in watch_service.go (non-test)
- [ ] All tests pass

Commit with message: "refactor: migrate fmt.Print to UICallback for testability"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 8: AUDIT-08 - Timeout Consistency (Hook/Git/Watch)

```
TASK: AUDIT-08 - Add Consistent Timeout Handling

CONTEXT: Hook execution has no timeout (can hang indefinitely). Git operations have inconsistent timeouts. Watch service has no graceful shutdown.

SCOPE:
- Modify: internal/core/hook_service.go
- Modify: internal/core/watch_service.go
- Create: internal/core/constants.go (add timeout constants if not exists)

IMPLEMENTATION:

1. Add timeout constants:
```go
const (
    HookTimeout        = 30 * time.Second
    GitCloneTimeout    = 5 * time.Minute
    GitFetchTimeout    = 2 * time.Minute
)
```

2. Hook service timeout:
```go
func (h *HookService) runCommand(command string, ctx *HookContext) error {
    cmdCtx, cancel := context.WithTimeout(context.Background(), HookTimeout)
    defer cancel()

    cmd := exec.CommandContext(cmdCtx, shellCmd, shellArg, command)
    // ...

    if cmdCtx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("%w: after %v", ErrHookTimeout, HookTimeout)
    }
    return err
}
```

3. Watch service graceful shutdown:
```go
func (w *WatchService) Watch(configPath string, onSync func() error) error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        cancel()
    }()
    // Use ctx in watcher loop
}
```

VERIFICATION:
```bash
grep -r "HookTimeout" internal/core/
go test ./...
```

ACCEPTANCE CRITERIA:
- [ ] HookTimeout constant defined (30s)
- [ ] Hook execution uses context with timeout
- [ ] Watch service handles SIGTERM gracefully
- [ ] All tests pass

Commit with message: "fix: add consistent timeout handling for hooks and watch"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 9: AUDIT-09 - TUI Refactoring & Tests

```
TASK: AUDIT-09 - Refactor TUI for Testability and Add Tests

CONTEXT: internal/tui/wizard.go has only 10.2% coverage. The root cause is check(err) calling os.Exit(1).

SCOPE:
- Modify: internal/tui/wizard.go
- Create: internal/tui/wizard_test.go

IMPLEMENTATION:

1. Replace check(err) with error returns:
```go
// Before:
func RunAddWizard(...) *types.VendorSpec {
    url, err := askURL()
    check(err)
}

// After:
func RunAddWizard(...) (*types.VendorSpec, error) {
    url, err := askURL()
    if err != nil {
        return nil, err
    }
}
```

2. Extract testable validation functions:
```go
func ValidateGitURL(url string) error
func ValidatePathMapping(from, to string) error
```

3. Add ErrUserAborted for clean cancellation:
```go
var ErrUserAborted = errors.New("user aborted")
```

4. Create tests:
- TestValidateGitURL_Valid
- TestValidateGitURL_Invalid
- TestValidatePathMapping_Valid
- TestValidatePathMapping_PathTraversal

5. Update main.go to handle new error returns.

VERIFICATION:
```bash
go test -v ./internal/tui/
go test -cover ./internal/tui/
```

ACCEPTANCE CRITERIA:
- [ ] check(err) pattern removed
- [ ] Functions return errors instead of os.Exit
- [ ] ValidateGitURL extracted and tested
- [ ] Coverage > 50% for tui package
- [ ] main.go updated
- [ ] All tests pass

Commit with message: "refactor: make TUI testable and add comprehensive tests"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 10: AUDIT-10 - main.go Test Infrastructure

```
TASK: AUDIT-10 - Add Test Infrastructure for main.go

CONTEXT: main.go (1091 lines, 64 os.Exit calls) has 0% test coverage.

SCOPE:
- Create: cmd/app.go
- Create: cmd/app_test.go
- Modify: main.go

IMPLEMENTATION:

1. Create cmd/app.go:
```go
package cmd

type App struct {
    Manager  ManagerInterface
    UI       UICallback
    Args     []string
    Stdout   io.Writer
    Stderr   io.Writer
}

func (a *App) Run(args []string) int {
    // Command routing logic extracted from main.go
}
```

2. Update main.go:
```go
func main() {
    app := cmd.NewApp()
    os.Exit(app.Run(os.Args))
}
```

3. Create cmd/app_test.go:
- TestApp_NoArgs_ShowsHelp
- TestApp_Init_Success
- TestApp_Sync_NotInitialized
- TestApp_Verify_ExitCodes
- TestApp_UnknownCommand
- TestParseCommonFlags_Yes
- TestParseCommonFlags_JSON

VERIFICATION:
```bash
go test -v ./cmd/
go test -cover ./cmd/
```

ACCEPTANCE CRITERIA:
- [ ] App struct with dependency injection
- [ ] main.go reduced to ~10 lines
- [ ] Command routing tested
- [ ] Exit codes verified
- [ ] Coverage > 70% for cmd package
- [ ] All tests pass

Commit with message: "refactor: extract testable App from main.go"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 11: AUDIT-11 - Cache Stale Data Warning

```
TASK: AUDIT-11 - Add Warning When Using Stale Cache Data

CONTEXT: verify_service.go falls back to cache without warning, potentially causing false positives.

SCOPE:
- Modify: internal/core/verify_service.go
- Modify: internal/types/types.go

IMPLEMENTATION:

1. Add Warnings field to VerifyResult:
```go
type VerifyResult struct {
    // ... existing fields
    Warnings []string `json:"warnings,omitempty"`
}
```

2. Add warning when using cache fallback:
```go
if len(expectedFiles) == 0 {
    expectedFiles, err = s.buildExpectedFilesFromCache(lock)
    if err != nil {
        return nil, err
    }
    result.Warnings = append(result.Warnings,
        "Using cached file hashes - lockfile may be outdated. Run 'git-vendor update' to refresh.")
}
```

3. Display warnings in CLI output.

4. Add tests:
- TestVerify_CacheFallback_AddsWarning
- TestVerify_NoFallback_NoWarning

VERIFICATION:
```bash
go test -v ./internal/core/ -run "TestVerify.*Warning"
```

ACCEPTANCE CRITERIA:
- [ ] Warnings field added
- [ ] Warning added when using cache
- [ ] Warning displayed in CLI
- [ ] Tests verify behavior
- [ ] All tests pass

Commit with message: "fix: warn when using stale cache data in verify"
Pull main, merge, then push to your branch when complete.
```

---

### PROMPT 12: AUDIT-12 - Race Condition Tests

```
TASK: AUDIT-12 - Add Explicit Race Condition Tests

CONTEXT: Code claims to pass -race but has no explicit race condition tests.

SCOPE:
- Create: internal/core/parallel_executor_race_test.go

IMPLEMENTATION:

```go
//go:build race

package core

func TestParallelExecutor_Race_ResultsChannel(t *testing.T) {
    // Spawn many workers writing simultaneously
    executor := NewParallelExecutor(...)
    vendors := make([]types.VendorSpec, 100)
    // Run with max parallelism
    opts := types.ParallelOptions{Enabled: true, MaxWorkers: 100}
    _, _ = executor.ExecuteParallelSync(vendors, nil, SyncOptions{}, opts)
}

func TestParallelExecutor_Race_ErrorCollection(t *testing.T)
func TestParallelExecutor_Race_ProgressCallback(t *testing.T)
func TestSyncService_Race_ConcurrentVendors(t *testing.T)
```

VERIFICATION:
```bash
go test -race -v ./internal/core/ -run "Race"
```

ACCEPTANCE CRITERIA:
- [ ] Race tests for results channel
- [ ] Race tests for error collection
- [ ] Race tests for progress callbacks
- [ ] All tests pass with -race flag
- [ ] No data races detected

Commit with message: "test: add explicit race condition tests for parallel executor"
Pull main, merge, then push to your branch when complete.
```

---

## Verification Checklist (Post-Execution)

After all prompts complete, verify:

- [ ] All tests pass: `go test ./...`
- [ ] Race detector clean: `go test -race ./...`
- [ ] Coverage improved: `go test -cover ./...`
- [ ] No remaining 0% coverage methods in core
- [ ] TUI coverage > 50%
- [ ] Types coverage > 80%
- [ ] All sentinel errors defined
- [ ] All magic strings replaced with constants
- [ ] All fmt.Print calls migrated to UICallback
- [ ] Hook timeout implemented
- [ ] Watch graceful shutdown implemented
