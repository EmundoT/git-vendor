## Task: Wrap Bare Error Returns with Context

The audit found 10 instances of bare `return err` without wrapping context. Fix these
in the scoped files below.

### Constraints
- ONLY modify these files (and their corresponding test files):
  - `internal/core/verify_service.go` + `verify_service_test.go`
  - `internal/core/validation_service.go` + `validation_service_test.go`
  - `internal/core/sbom_generator.go` + `sbom_generator_test.go`
  - `internal/core/license_service.go` + `license_service_test.go`
  - `internal/core/utils.go` + `utils_test.go`
- DO NOT modify `git_operations.go`, `engine.go`, `filesystem.go`, `go.mod`, `go.sum`
- DO NOT modify `file_copy_service.go`, `position_extract.go`, `sync_service.go`, `update_service.go`
- Generate mocks first: `make mocks`

### Pattern

Replace bare returns like `return err` with contextual wrapping using this format:

    fmt.Errorf("<function_name>: <what failed>: %w", err)

Example transformation:

    // BEFORE
    func (v *VerifyService) Verify() (*VerifyResult, error) {
        ...
        return nil, err
    }

    // AFTER
    func (v *VerifyService) Verify() (*VerifyResult, error) {
        ...
        return nil, fmt.Errorf("Verify: read lockfile: %w", err)
    }

### Rules
- Only wrap errors that are currently bare (no existing context)
- Do NOT double-wrap errors that already have `fmt.Errorf` wrapping
- Use `%w` (not `%v`) to preserve error chain for `errors.Is`/`errors.As`
- Keep messages lowercase, no trailing punctuation
- Update any tests that assert on exact error strings (use `errors.Is` or `strings.Contains` instead)

### Definition of Done
1. Zero bare `return err` in scoped files (verify with grep)
2. `go test ./internal/core/` all pass
3. `make lint` is green
4. No existing test broken by changed error messages
