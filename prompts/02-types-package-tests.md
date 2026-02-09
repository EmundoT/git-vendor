## Task: Types Package Test Coverage

The types package has low test coverage per audit findings. Add comprehensive tests
for YAML/JSON marshalling, position parsing edge cases, and data model invariants.

### Constraints
- ONLY modify files in `internal/types/`
- DO NOT touch `internal/core/`, `internal/tui/`, or root-level files
- Generate mocks first: `make mocks`

### Files to test
- `internal/types/types.go` — VendorConfig, VendorSpec, BranchSpec, PathMapping, VendorLock, LockDetails
- `internal/types/position.go` — PositionSpec, ParsePathPosition

### Required test cases

**YAML round-trip tests (types.go):**
1. VendorConfig marshal then unmarshal preserves all fields
2. VendorSpec with hooks, groups, multiple specs round-trips correctly
3. VendorLock with PositionLock entries round-trips correctly
4. Empty optional fields (groups, hooks, default_target) omit correctly with `omitempty`
5. Unknown fields in YAML are silently ignored (forward compatibility)

**Position parser edge cases (position.go):**
1. Windows drive letters: `C:\path\file.go:L5` — colon in drive letter must not confuse parser
2. Paths with no position: `path/to/file.go` returns nil PositionSpec
3. Single line: `file.go:L5` returns StartLine=5, EndLine=5
4. Line range: `file.go:L5-L20` returns StartLine=5, EndLine=20
5. EOF range: `file.go:L10-EOF` returns StartLine=10, ToEOF=true
6. Column range: `file.go:L5C10:L10C30` returns full PositionSpec
7. Invalid specs: `file.go:L`, `file.go:L-1`, `file.go:LABC` return error
8. EndLine less than StartLine: `file.go:L20-L5` returns error
9. Zero line: `file.go:L0` returns error (1-indexed)

**Property-based tests:**
- Any valid PositionSpec can be formatted back to string and re-parsed to the same spec

### Definition of Done
1. `go test -cover ./internal/types/` shows ≥85%
2. `go test -v ./internal/types/` all pass
3. `make lint` is green
