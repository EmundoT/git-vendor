# Audit Completed Ideas

Perform a comprehensive quality audit of completed ideas in `ideas/completed.md`.

## Instructions

1. **Read `ideas/completed.md`** to get the current list of completed items
2. **For each completed idea**, audit against the criteria below
3. **Check the spec** in `ideas/specs/complete/` if one exists
4. **Generate a fresh audit report** using the template at the bottom

## Audit Criteria

### 1. Code Quality Assessment

For each completed idea, verify:
- **Go Best Practices**: Follows Go conventions (gofmt, naming, error handling)
- **Interface Design**: Uses dependency injection pattern consistent with codebase
- **Security**: No path traversal vulnerabilities, no command injection, proper input validation
- **Performance**: Efficient operations, no unnecessary allocations, proper goroutine handling
- **Maintainability**: Clear naming, modular structure, consistent formatting

### 2. Documentation Verification

Check that each completed idea has:
- **Function Comments**: Exported functions have godoc-style comments
- **CLAUDE.md Updates**: New features/commands mentioned in CLAUDE.md if significant
- **README Updates**: User-facing features documented in README
- **Help Text**: CLI commands have proper help text (`--help` output)

### 3. Test Coverage Analysis

Verify testing for each completed idea:
- **Unit Tests Exist**: Check `internal/core/*_test.go` for related tests
- **Mock Usage**: Tests use gomock for interface mocking
- **Edge Cases**: Null handling, empty inputs, boundary conditions tested
- **Integration Points**: Tests verify interaction with dependencies

### 4. Implementation Completeness

For each spec in `ideas/specs/complete/`, verify:
- **All Planned Files Created**: Compare spec's "Files to create" with actual implementation
- **All Features Implemented**: Each feature in the spec is functional
- **Error Handling**: Errors are properly handled and returned to callers
- **CLI Integration**: New commands are properly routed in main.go

### 5. Questions and Concerns

Flag any:
- **Incomplete implementations**: Features partially done or missing
- **Technical debt**: Workarounds, TODOs in code, known limitations
- **Integration gaps**: Features not connected to rest of framework
- **Missing tests**: Critical paths without test coverage
- **Documentation gaps**: Undocumented parameters or behaviors

---

## Audit Report Template

Generate a report following this structure:

```markdown
## Audit Report - [DATE]

### Executive Summary

| Category | Pass | Fail | Warning | Total |
|----------|------|------|---------|-------|
| Code Quality | X | X | X | X |
| Documentation | X | X | X | X |
| Tests | X | X | X | X |
| Completeness | X | X | X | X |

**Overall Status**: PASS / PASS with WARNINGS / FAIL

---

### Detailed Findings

#### [ID]: [Title]

| Criterion | Status | Details |
|-----------|--------|---------|
| Code Quality | PASS/WARN/FAIL | [specifics] |
| Documentation | PASS/WARN/FAIL | [specifics] |
| Tests | PASS/WARN/FAIL | [test file locations or gaps] |
| Completeness | PASS/WARN/FAIL | [specifics] |

**Concerns**: [any issues or "None"]

---

[Repeat for each completed idea]

---

### Summary of Concerns

#### High Priority
[List critical gaps]

#### Medium Priority
[List moderate issues]

#### Low Priority
[List minor improvements]

---

### Recommendations

1. [Immediate actions needed]
2. [Future improvements]
```

---

## Scope Options

You can limit the audit scope by specifying:
- **All** (default): Audit everything in completed.md
- **Recent**: Only items completed in the last 7 days
- **Category**: Only Feature Ideas, Security Issues, or Code Quality Issues
- **Specific IDs**: Audit specific items by ID (e.g., "001, 005, 017")

---

## Go-Specific Checks

### Code Quality Quick Checks

```bash
# Run go fmt check
gofmt -l internal/

# Run go vet
go vet ./...

# Check for unused code
go build -gcflags="-m" ./... 2>&1 | grep "can inline"

# Run staticcheck if available
staticcheck ./...
```

### Test Verification

```bash
# Run tests with coverage
go test -cover ./internal/core

# Run tests with race detector
go test -race ./...

# Check test file existence
ls internal/core/*_test.go
```

### Documentation Checks

```bash
# Find exported functions without comments
grep -rn "^func [A-Z]" internal/ | head -20

# Check CLAUDE.md mentions new features
grep -i "[feature name]" CLAUDE.md
```

---

## Integration Points

- **ideas/completed.md** - Primary tracking file
- **ideas/specs/complete/** - Completed specifications
- **internal/core/*_test.go** - Test files
- **CLAUDE.md** - Feature documentation
- **README.md** - User-facing documentation
