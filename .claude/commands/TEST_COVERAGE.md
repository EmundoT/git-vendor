# Test Coverage Workflow

**Role:** You are a test coverage analyst working in a concurrent multi-agent Git environment. Your goal is to identify coverage gaps, generate test creation prompts for other Claude instances, and verify new tests actually exercise the code through iterative review cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your coverage branch (already created for you)

**Key Principle:** A test that exists but doesn't exercise the code is worse than no test (false confidence). Verify tests actually test what they claim.

## Phase 1: Sync & Coverage Analysis

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Generate Mocks:** Ensure mocks are up to date:
  ```bash
  make mocks
  ```

- **Run Coverage Analysis:**
  ```bash
  # Get coverage report
  go test -coverprofile=coverage.out ./...

  # View coverage by function
  go tool cover -func=coverage.out

  # Generate HTML report
  go tool cover -html=coverage.out -o coverage.html
  ```

- **Inventory Functions:**
  ```bash
  # List all exported functions in core package
  grep -rn "^func [A-Z]" internal/core/*.go | grep -v "_test.go"

  # List all interfaces
  grep -rn "type.*interface" internal/core/*.go
  ```

- **Inventory Tests:**
  ```bash
  # List all test files
  find . -name "*_test.go" -type f

  # Count test functions
  grep -rn "^func Test" internal/core/*_test.go | wc -l
  ```

- **Cross-Reference Analysis:**

  | Check | Method |
  |-------|--------|
  | Functions without tests | Compare function list to test file coverage |
  | Tests without functions | Find orphaned test files |
  | Mock completeness | Check all interfaces have mocks |
  | Coverage gaps | Look for < 50% coverage in coverage.out |

- **Calculate Coverage Metrics:**
  ```
  Total Functions: X
  With Tests: Y
  Coverage: Z%

  Packages:
  - internal/core: X%
  - internal/tui: Y%
  - internal/types: Z%
  ```

## Phase 2: Gap Prioritization

Categorize coverage gaps by risk:

| Priority | Criteria | Examples |
|----------|----------|----------|
| **CRITICAL** | Core functionality, no tests at all | VendorSyncer.syncVendor, Manager methods |
| **HIGH** | User-facing features, partial coverage | Git operations, config parsing |
| **MEDIUM** | Supporting features, edge cases missing | Validation helpers, error paths |
| **LOW** | Internal utilities, rarely used paths | Debug functions, internal helpers |

Consider:
- **Complexity** - More complex functions need more tests
- **Usage frequency** - High-use functions are higher priority
- **Change frequency** - Frequently modified code needs test protection
- **Failure impact** - What breaks if this fails?

## Phase 3: Test Creation Prompts

Generate prompts for test creation:

### Prompt Template

```
TASK: Add tests for [package].[FunctionName]

CURRENT COVERAGE: [percentage or NONE]

FUNCTION ANALYSIS:
- Location: internal/core/[file].go:[line]
- Signature: func [signature]
- Dependencies: [interfaces it uses]
- Return: [what it returns]
- Side effects: [files written, state changes]
- Error paths: [what can fail]

TEST FILE: internal/core/[name]_test.go

REQUIRED TEST CASES:

1. **Happy Path**
   - [Normal operation test description]
   - Expected: [outcome]

2. **Input Validation**
   - nil/empty parameters: [expected behavior]
   - Invalid values: [expected behavior]
   - Edge cases: [boundary values]

3. **Error Handling**
   - [Error condition]: [expected error]
   - [Mock failure]: [expected outcome]

4. **Integration**
   - [How it interacts with mocked dependencies]
   - [State requirements]

MOCK SETUP:
```go
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockGit := NewMockGitClient(ctrl)
mockFS := NewMockFileSystem(ctrl)
// ... setup expectations
```

TEST PATTERN TO FOLLOW:
[Reference similar existing test file for patterns]

VERIFICATION:
- [ ] Test file compiles without errors
- [ ] Tests actually call the function (not just existence checks)
- [ ] Error paths are tested
- [ ] Mocks verify expected calls

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Test Quality Guidelines

- **Don't just test existence** - Actually invoke the function
- **Test behavior, not implementation** - Focus on inputs/outputs
- **Include negative tests** - What should fail?
- **Use table-driven tests** - Go idiom for multiple cases
- **Mock dependencies** - Use gomock for interfaces

## Phase 4: Coverage Report

```markdown
## Test Coverage Report

### Coverage Summary

| Package | Functions | Covered | Coverage |
|---------|-----------|---------|----------|
| internal/core | X | Y | Z% |
| internal/tui | X | Y | Z% |
| internal/types | X | Y | Z% |
| **TOTAL** | **X** | **Y** | **Z%** |

### Critical Gaps (No Tests)

| Function | File | Priority | Risk |
|----------|------|----------|------|
| [Name] | [File:Line] | CRITICAL | [Impact if broken] |

### Partial Coverage (Missing Cases)

| Function | Has | Missing |
|----------|-----|---------|
| [Name] | Happy path | Error paths, edge cases |

### Prompts Generated

| # | Function | Test File | Priority |
|---|----------|-----------|----------|
| 1 | [Name] | internal/core/[name]_test.go | CRITICAL |

---

## PROMPT 1: Add tests for [Function]
[Full prompt]

---
```

## Phase 5: Verification Cycle

After test creation prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Regenerate Mocks:**
  ```bash
  make mocks
  ```

- **Verify Tests Exist:**
  ```bash
  # Check new test files exist
  ls internal/core/*_test.go
  ```

- **Verify Tests Actually Test:**
  - Read the test file
  - Confirm it calls the function (not just checks it exists)
  - Check for assertions on return values
  - Verify error paths are covered

- **Run the Tests:**
  ```bash
  # Run all tests
  go test -v ./...

  # Run with race detector
  go test -race ./...

  # Run specific test
  go test -v -run TestFunctionName ./internal/core
  ```

- **Check Coverage Improved:**
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -func=coverage.out | grep [function_name]
  ```

- **Grade Each Test Prompt:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Tests exist, run, cover cases | Complete |
  | **PARTIAL** | Tests exist but don't cover all cases | Follow-up for missing cases |
  | **EXISTENCE-ONLY** | Tests just check function exists | Redo with real tests |
  | **FAIL** | No tests created | Redo prompt |

- **Generate Follow-Up Prompts:**

  For EXISTENCE-ONLY tests:
  ```
  TASK: Improve tests for [Function] - tests exist but don't exercise functionality

  CURRENT STATE: internal/core/[file]_test.go only checks function exists

  REQUIRED: Add actual functionality tests that:
  - Call the function with real parameters
  - Assert on return values or side effects
  - Test error conditions with mocked failures

  [Include specific test cases needed]
  ```

## Phase 6: Coverage Completion

When all gaps are addressed:

- **Recalculate Coverage:**
  - Run full coverage analysis again
  - Update coverage percentages
  - Verify improvement

- **Run Full Test Suite:**
  ```bash
  # Full test run
  go test ./...

  # With race detector
  go test -race ./...

  # With coverage
  go test -cover ./...
  ```

- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```

- **Coverage Report:**

  ```markdown
  ## Test Coverage Cycle Complete

  ### Before/After

  | Metric | Before | After | Change |
  |--------|--------|-------|--------|
  | Total Coverage | X% | Y% | +Z% |
  | Critical Gaps | N | M | -K |
  | Functions Tested | A | B | +C |

  ### Tests Added

  | Function | Test File | Cases |
  |----------|-----------|-------|
  | [Name] | [Path] | [Count] |

  ### Remaining Gaps
  [Any deferred items with justification]

  ### Recommendations
  - [Areas needing more coverage]
  - [Test infrastructure improvements]
  ```

---

## Coverage Analysis Quick Reference

### Find Untested Functions

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# Find functions with 0% coverage
go tool cover -func=coverage.out | grep "0.0%"

# Find packages below threshold
go tool cover -func=coverage.out | grep -E "[0-4][0-9]\.[0-9]%"
```

### Check Test Quality

```bash
# Tests that use mocks properly
grep -rn "gomock.NewController" internal/core/*_test.go

# Tests with assertions
grep -rn "if.*!=" internal/core/*_test.go | head -20

# Table-driven tests
grep -rn "tests := \[\]struct" internal/core/*_test.go
```

### gomock Pattern

```go
func TestSomething(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockGit := NewMockGitClient(ctrl)
    mockFS := NewMockFileSystem(ctrl)

    // Set expectations
    mockGit.EXPECT().Clone(gomock.Any(), gomock.Any()).Return(nil)
    mockFS.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(nil)

    // Create system under test
    syncer := NewVendorSyncer(mockGit, mockFS, ...)

    // Call and assert
    err := syncer.SyncVendor(...)
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

---

## Integration Points

- **internal/core/*_test.go** - Test files
- **internal/core/*_mock_test.go** - Generated mocks (gitignored)
- **Makefile** - `make mocks` target
- **go.mod** - Test dependencies
- **CLAUDE.md** - Test coverage documentation
