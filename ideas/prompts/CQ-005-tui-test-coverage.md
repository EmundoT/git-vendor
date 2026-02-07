# PROMPT: CQ-005 TUI Test Coverage

## Task
Increase test coverage for the `internal/tui` package from 9.9% to at least 60%.

## Priority
HIGH - TUI is a critical user-facing component with minimal test coverage.

## Problem
The `internal/tui/` package has only 9.9% test coverage. This package handles all user interaction and output formatting, making it a high-risk area for regressions.

## Current State
```
ok   github.com/EmundoT/git-vendor/internal/tui   coverage: 9.9% of statements
```

## Affected Files
- `internal/tui/wizard.go` - Main TUI implementation
- `internal/tui/wizard_test.go` - Test file to expand (or create)

## Key Functions to Test

### 1. TUICallback Methods
```go
func TestTUICallback_ShowError(t *testing.T) {
    // Capture stdout
    // Call ShowError
    // Verify output format
}

func TestTUICallback_ShowSuccess(t *testing.T) {
    // Similar pattern
}

func TestTUICallback_FormatJSON(t *testing.T) {
    output := core.JSONOutput{
        Status: "success",
        Message: "test",
    }
    // Verify JSON formatting
}
```

### 2. NonInteractiveTUICallback
```go
func TestNonInteractiveTUICallback_QuietMode(t *testing.T) {
    flags := core.NonInteractiveFlags{Mode: core.OutputQuiet}
    callback := NewNonInteractiveTUICallback(flags)
    // Verify quiet mode suppresses output
}

func TestNonInteractiveTUICallback_JSONMode(t *testing.T) {
    flags := core.NonInteractiveFlags{Mode: core.OutputJSON}
    callback := NewNonInteractiveTUICallback(flags)
    // Verify JSON output
}
```

### 3. Progress Tracking
```go
func TestProgressTracker_Increment(t *testing.T) {
    // Test progress updates
}

func TestProgressTracker_Complete(t *testing.T) {
    // Test completion
}

func TestProgressTracker_Fail(t *testing.T) {
    // Test failure handling
}
```

### 4. Style Functions
```go
func TestStyleTitle(t *testing.T) {
    result := StyleTitle("Test")
    // Verify styling applied
}

func TestStyleSuccess(t *testing.T) {
    result := StyleSuccess("Done")
    // Verify green styling
}
```

### 5. PrintHelp
```go
func TestPrintHelp(t *testing.T) {
    // Capture stdout
    PrintHelp()
    // Verify help text contains expected sections
}
```

## Testing Strategy

### Output Capture Pattern
```go
func captureOutput(f func()) string {
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    f()

    w.Close()
    os.Stdout = old

    var buf bytes.Buffer
    io.Copy(&buf, r)
    return buf.String()
}
```

### Test Organization
- Use table-driven tests for multiple input scenarios
- Group tests by component (callback, progress, styles)
- Test both happy path and error cases

## Mandatory Tracking Updates
1. Update `ideas/code_quality.md` - change CQ-005 status to "completed"
2. Add completion notes with final coverage percentage

## Acceptance Criteria
- [ ] `internal/tui` coverage ≥60%
- [ ] TUICallback methods have tests (ShowError, ShowSuccess, FormatJSON)
- [ ] NonInteractiveTUICallback modes tested (quiet, json)
- [ ] Progress tracker methods tested
- [ ] Style functions tested
- [ ] PrintHelp tested
- [ ] `go test ./internal/tui -cover` shows ≥60%
- [ ] All tests pass with `go test ./...`

## GIT WORKFLOW (MANDATORY)
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   ```
   git fetch origin main
   git merge origin/main
   ```
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   ```
   git push -u origin <your-branch-name>
   ```
