## Task: TUI Test Coverage (CQ-005)

Raise test coverage for `internal/tui/` from 11.6% to ≥60%.

### Constraints
- ONLY modify files in `internal/tui/`
- DO NOT touch any files in `internal/core/`, `internal/types/`, or root-level files
- DO NOT modify `wizard.go` behavior — add tests only, or create `wizard_test.go`
- Generate mocks first: `make mocks`
- Run `go test -cover ./internal/tui/` to verify coverage

### Scope of wizard.go test coverage needed
1. `runMappingManager` — test add/edit/delete mapping flows
2. `runMappingCreator` — test From/To input validation (position specifier hints, empty path rejection)
3. `runRemoteBrowser` — test directory navigation and selection
4. `truncate` — test string truncation at various lengths
5. URL validation in `runAddWizard` — test GitHub/GitLab/Bitbucket URL acceptance
6. `runEditWizard` — test vendor selection and field editing

### Strategy
- The TUI uses charmbracelet/huh which is hard to test interactively. Focus on:
  - Extracting testable pure functions from wizard.go where possible
  - Testing validation functions passed to huh form fields
  - Testing data transformation logic (truncate, URL parsing, path formatting)
  - Testing any exported functions
- If huh form execution can't be unit tested, test the validation callbacks in isolation

### Definition of Done
1. `go test -cover ./internal/tui/` shows ≥60%
2. `go test -v ./internal/tui/` all pass
3. `make lint` is green
4. Doc comments added to any new exported test helpers
