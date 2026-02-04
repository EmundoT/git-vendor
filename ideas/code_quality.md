# Code Quality Issues Queue

> Code quality improvements, consistency fixes, and technical debt items for git-vendor. Items with specs have detailed implementation docs in `ideas/specs/code-quality/`.

## HIGH Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| CQ-001 | pending | Interface Mock Coverage | Ensure all interfaces have mock implementations for testing | - |
| CQ-002 | pending | Error Wrapping Consistency | Use fmt.Errorf with %w for all error wrapping, enable error chain inspection | - |
| CQ-003 | pending | Context Propagation | Add context.Context to all long-running operations for cancellation/timeout | - |
| CQ-004 | pending | Godoc Coverage | Add godoc comments to all exported functions and types | - |

## MEDIUM Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| CQ-010 | pending | Table-Driven Tests | Convert existing tests to table-driven pattern for better coverage | - |
| CQ-011 | pending | Test Fixture Organization | Organize test fixtures in testdata/ with clear naming conventions | - |
| CQ-012 | pending | Logging Consistency | Replace fmt.Print* with structured logging or consistent output pattern | - |
| CQ-013 | pending | Flag Naming Consistency | Audit all CLI flags for consistent naming (kebab-case, no abbreviation conflicts) | - |
| CQ-014 | pending | Magic Number Extraction | Extract hardcoded values (timeouts, limits) to constants or config | - |
| CQ-015 | pending | Parallel Test Safety | Ensure all tests can run with -parallel flag via t.Parallel() | - |

## LOW Priority

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| CQ-020 | pending | Linter Configuration | Add golangci-lint with custom config, integrate into CI | - |
| CQ-021 | pending | Code Generation Comments | Add //go:generate comments for mock generation | - |
| CQ-022 | pending | Import Organization | Standardize import grouping (stdlib, external, internal) | - |
| CQ-023 | pending | Panic Audit | Replace any panic() in library code with error returns | - |
| CQ-024 | pending | String Builder Usage | Use strings.Builder instead of concatenation in hot paths | - |
| CQ-025 | pending | Slice Preallocation | Preallocate slices where size is known to reduce allocations | - |

---

## Technical Debt from ROADMAP

Per ROADMAP.md Section 2.3, address before new features:

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| CQ-030 | pending | Integration Test Coverage | Ensure all commands have integration tests with real Git operations | - |
| CQ-031 | pending | Error Format Consistency | Implement Error/Context/Fix format per ROADMAP Section 9.5 | - |
| CQ-032 | pending | Lockfile Forward Compatibility | Verify older parsers handle unknown fields gracefully | - |

---

## Completed Issue Details

*No issues completed yet. This section will track completion notes.*

---

## Code Quality Guidelines

### Go Conventions
- Use `gofmt` and `go vet` on all code
- Follow Effective Go and Go Code Review Comments
- Prefer returning errors over panicking in library code

### Testing Standards
- Use gomock for interface mocking
- Table-driven tests for multiple cases
- Run `go test -race ./...` to detect races
- Target ≥80% coverage on core packages

### Error Handling
```go
// Preferred: Wrap with context
return fmt.Errorf("sync vendor %s: %w", name, err)

// Avoid: Bare error return without context
return err
```

### Interface Design
```go
// Small, focused interfaces for testability
type GitClient interface {
    Clone(ctx context.Context, url, dest string) error
    // ...
}
```
