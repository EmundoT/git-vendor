# Prompt: Set 5 — TUI Test Coverage Push (CQ-005)

## Concurrency
SAFE with all other sets. Touches only `internal/tui/*` files — zero overlap with core/ or main.go.

## Branch
Create and work on a branch named `claude/set5-tui-coverage-<session-suffix>`.

## Context
The `internal/tui` package is at 63.0% coverage — the only package below the 80% target. Current test files:
- `callback_test.go`
- `non_interactive_test.go`
- `progress_test.go`
- `wizard_test.go`
- `wizard_validate_test.go`

Source files:
- `callback.go` — UI callback types and print functions
- `non_interactive.go` — Non-interactive mode helpers
- `progress.go` — Progress indicator logic
- `wizard.go` — Main TUI wizard (charmbracelet/huh forms)
- `wizard_helpers.go` — Pure helper functions used by wizard

## Task
Increase `internal/tui` test coverage from 63% to 80%+.

### Strategy

The TUI package has two kinds of code:
1. **Pure functions** — validation, formatting, path manipulation, string processing. These are fully testable.
2. **Interactive TUI** — charmbracelet/huh form rendering. NOT unit-testable (requires terminal).

Focus exclusively on (1). Do NOT attempt to test huh form interactions.

### Step 1: Identify uncovered functions

Run coverage with HTML output to identify specific uncovered lines:

    go test -coverprofile=coverage.out ./internal/tui/
    go tool cover -func=coverage.out

Look for exported and unexported functions with 0% or low coverage.

### Step 2: Test pure functions in wizard_helpers.go

Read `wizard_helpers.go` carefully. It likely contains:
- Path manipulation helpers
- Formatting/display helpers
- Validation functions
- String processing utilities

Each of these should be testable in isolation. Add tests to `wizard_helpers_test.go` (create if needed).

### Step 3: Test callback.go print functions

`callback.go` has print/display functions. Test the ones that return values or have testable side effects. For functions that only write to stdout, use output capture:

    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    // call function
    w.Close()
    os.Stdout = old
    var buf bytes.Buffer
    io.Copy(&buf, r)
    // assert buf.String() contains expected output

### Step 4: Test non_interactive.go edge cases

Check `non_interactive.go` for untested code paths — error cases, edge cases, default values.

### Step 5: Test progress.go

Test progress indicator state management, formatting, and edge cases.

### Step 6: Test wizard_validate_test.go gaps

Check if there are validation functions in wizard.go that aren't covered by wizard_validate_test.go.

### Key constraints
- Do NOT modify any source files in `internal/tui/` unless extracting a pure function from wizard.go for testability
- If extracting functions, keep them in `wizard_helpers.go` (don't create new source files)
- Do NOT import charmbracelet/huh in test files
- Do NOT test interactive form rendering
- ALL new tests must pass with `go test -race ./internal/tui/`

### Definition of Done
1. `go test -cover ./internal/tui/` shows >= 80%
2. `go test -race ./internal/tui/` passes
3. `go test ./...` passes (no regressions)
4. `go vet ./...` clean
5. No TUI source files modified (unless extracting pure functions)
6. Commit and push
