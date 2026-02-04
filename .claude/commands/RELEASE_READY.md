# Release Ready Workflow

**Role:** You are a release manager working in a concurrent multi-agent Git environment. Your goal is to validate that the codebase is ready for release by orchestrating multiple validation workflows, generating fix prompts for blocking issues, and achieving sign-off through iterative verification cycles.

**Branch Structure:**
- `main` - Parent branch where release will be cut from
- `{your-current-branch}` - Your release validation branch (already created for you)

**Key Principle:** A release is only ready when ALL gates pass. No exceptions. No "we'll fix it in the next release."

## Phase 1: Sync & Release Scope

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Determine Release Version:**
  ```bash
  # Check current version
  grep -i "version" CHANGELOG.md | head -5

  # Check internal/version/version.go
  cat internal/version/version.go

  # Check for unreleased changes
  git log --oneline $(git describe --tags --abbrev=0)..HEAD
  ```

- **Identify Release Scope:**
  - New features since last release
  - Bug fixes included
  - Breaking changes
  - Lockfile schema changes (migration requirements)

## Phase 2: Gate Validation

Run each validation gate. ALL must pass for release.

### Gate 1: Security Audit (BLOCKING)

Run `/SECURITY_AUDIT` workflow checks:

```bash
# Path traversal vulnerabilities
grep -rn "filepath.Join\|os.Open\|os.Create" internal/ | grep -v "_test.go"

# Check ValidateDestPath usage
grep -rn "ValidateDestPath" internal/core/

# Command injection
grep -rn "exec.Command" internal/

# Credential exposure
grep -rni "password\|secret\|token\|key" internal/ | grep -v "_test.go"

# Panic in library code
grep -rn "panic(" internal/core/ | grep -v "_test.go"

# Run race detector
go test -race ./...
```

**Gate Status:**
- PASS: Zero CRITICAL or HIGH security issues
- FAIL: Any CRITICAL/HIGH security issue

### Gate 2: Test Coverage (BLOCKING)

Run `/TEST_COVERAGE` workflow checks:

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Check coverage
go test -cover ./internal/core

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

**Gate Status:**
- PASS: All tests pass, coverage >= 70%
- WARN: Tests pass, coverage 50-70%
- FAIL: Any test failure

### Gate 3: Documentation Sync (BLOCKING)

Run `/DOC_SYNC` workflow checks:

```bash
# Check CLAUDE.md accuracy
grep -E "git-vendor|git vendor" CLAUDE.md | head -20

# Check README commands match actual CLI
grep -E 'case "[a-z]' main.go | wc -l
grep -E "git-vendor [a-z]+" README.md | wc -l

# Verify help text exists
./git-vendor --help
./git-vendor sync --help
```

**Gate Status:**
- PASS: No CRITICAL/HIGH drift
- WARN: MEDIUM drift only
- FAIL: Documentation actively misleading

### Gate 4: Code Quality (BLOCKING)

Run `/CODE_REVIEW` workflow checks:

```bash
# gofmt check
gofmt -l internal/

# go vet
go vet ./...

# Check for fmt.Print in library code
grep -rn "fmt.Print" internal/core/ | grep -v "_test.go"

# Check error handling
grep -rn "_, err :=" internal/core/ | head -10

# Check for TODO/FIXME
grep -rn "TODO\|FIXME" internal/ | wc -l
```

**Gate Status:**
- PASS: Zero gofmt issues, zero go vet issues
- WARN: Minor issues only
- FAIL: Any CRITICAL/HIGH code quality issue

### Gate 5: Build Verification (BLOCKING)

```bash
# Build for multiple platforms
make build

# Or build manually
go build -ldflags="-s -w" -o git-vendor

# Verify binary runs
./git-vendor version
./git-vendor --help

# Cross-platform builds (if applicable)
GOOS=linux GOARCH=amd64 go build -o git-vendor-linux
GOOS=darwin GOARCH=amd64 go build -o git-vendor-darwin
GOOS=windows GOARCH=amd64 go build -o git-vendor.exe
```

**Gate Status:**
- PASS: All builds succeed, binary runs correctly
- FAIL: Any build failure

### Gate 6: Performance Baseline (NON-BLOCKING)

Run `/PERFORMANCE_BASELINE` workflow checks:

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Compare to baseline if exists
# benchstat benchmark-baseline.txt benchmark-current.txt
```

**Gate Status:**
- PASS: No regressions detected
- WARN: Minor regressions (<20%)
- FAIL: Critical regression (>50%)

### Gate 7: Dependency Health (NON-BLOCKING)

Run `/DEPENDENCY_AUDIT` workflow checks:

```bash
# Check for outdated dependencies
go list -u -m all

# Verify mod is tidy
go mod tidy
git diff go.mod go.sum

# Check for known vulnerabilities (if govulncheck installed)
govulncheck ./...

# Verify build with latest dependencies
go build ./...
```

**Gate Status:**
- PASS: Clean dependency graph
- WARN: Minor version updates available
- FAIL: Known vulnerabilities

### Gate 8: Schema Migration (CONDITIONAL)

Run `/MIGRATION_PLANNER` workflow if lockfile schema changes:

```bash
# Check for schema changes
git diff $(git describe --tags --abbrev=0)..HEAD -- internal/types/types.go | grep -E "Lock|Vendor"

# If changes exist:
# - Verify backward compatibility
# - Ensure schema_version updated
# - Test old lockfile parsing
```

**Gate Status:**
- PASS: Migrations ready and tested
- N/A: No schema changes
- FAIL: Missing migrations for schema changes

## Phase 3: Gate Summary

```markdown
## Release Validation Summary

### Gate Status

| Gate | Status | Blocking | Details |
|------|--------|----------|---------|
| Security | PASS/FAIL | YES | [summary] |
| Tests | PASS/FAIL | YES | [summary] |
| Documentation | PASS/FAIL | YES | [summary] |
| Code Quality | PASS/FAIL | YES | [summary] |
| Build | PASS/FAIL | YES | [summary] |
| Performance | PASS/WARN/FAIL | NO | [summary] |
| Dependencies | PASS/WARN/FAIL | NO | [summary] |
| Migrations | PASS/N-A/FAIL | CONDITIONAL | [summary] |

### Release Decision

**READY FOR RELEASE:** YES / NO

If NO, blocking issues:
1. [Issue 1]
2. [Issue 2]
```

## Phase 4: Blocking Issue Prompts

For each blocking gate failure, generate remediation prompts:

### Prompt Template

```
TASK: [RELEASE BLOCKER] Fix [Issue] for v[X.Y.Z] release

BLOCKING GATE: [Gate Name]

ISSUE:
[Detailed description]

IMPACT ON RELEASE:
- Release CANNOT proceed until this is fixed
- Target release date: [date if known]

REQUIRED FIX:
[Specific steps]

VERIFICATION:
- [ ] Gate re-check passes
- [ ] No new issues introduced
- [ ] Ready for re-validation

PRIORITY: CRITICAL - This blocks the release

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

## Phase 5: Verification Cycle

After blocking issues are fixed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Re-Run Failed Gates:**
  Only re-run gates that previously failed

- **Verify All Gates Pass:**
  Update gate summary with new status

- **Grade Fix Prompts:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Gate now passes | Mark resolved |
  | **PARTIAL** | Improved but still failing | Follow-up prompt |
  | **FAIL** | No improvement | Escalate, reassess |

- **Iterate:** Repeat until all blocking gates pass

## Phase 6: Release Preparation

When all gates pass:

### 6.1 Version Bump

```bash
# Update version in version.go (if manual)
# GoReleaser handles this via ldflags

# Update CHANGELOG.md
```

### 6.2 Changelog Update

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- [New features]

### Changed
- [Changes to existing features]

### Fixed
- [Bug fixes]

### Security
- [Security fixes]

### Migration
- [Required lockfile migrations if any]
```

### 6.3 Final Validation

```bash
# Full test suite one more time
go test ./...
go test -race ./...

# Build final binary
make build

# Smoke test
./git-vendor version
./git-vendor init
./git-vendor list
```

### 6.4 Tag & Release

```bash
# Create release tag
git tag -a v[X.Y.Z] -m "Release v[X.Y.Z]"

# Push tag
git push origin v[X.Y.Z]

# GoReleaser will handle the rest if configured
# goreleaser release --clean
```

## Phase 7: Release Report

```markdown
## Release v[X.Y.Z] - Validation Complete

### Release Summary

| Metric | Value |
|--------|-------|
| Version | X.Y.Z |
| Release Date | YYYY-MM-DD |
| Commits Since Last Release | N |
| Issues Fixed | M |
| New Features | K |

### Gate Validation Results

| Gate | Final Status | Issues Fixed |
|------|--------------|--------------|
| Security | PASS | 0 |
| Tests | PASS | 2 fixed |
| Documentation | PASS | 3 synced |
| Code Quality | PASS | 1 fixed |
| Build | PASS | 0 |
| Performance | PASS | 0 |
| Dependencies | PASS | 0 |
| Migrations | N/A | - |

### Validation Cycles Required

- Initial validation: [X gates failed]
- Cycle 2: [Y gates failed]
- Final: All gates pass

### Release Contents

#### New Features
- [Feature 1]
- [Feature 2]

#### Bug Fixes
- [Fix 1]
- [Fix 2]

#### Breaking Changes
- [None / List]

#### Migration Notes
- [None / Lockfile schema changes]

### Post-Release Checklist

- [ ] Tag created and pushed
- [ ] Release notes published (GitHub Releases)
- [ ] Documentation updated
- [ ] README updated if needed
```

---

## Quick Release Checklist

### Pre-Release

- [ ] All gates pass (Phase 2)
- [ ] CHANGELOG.md updated
- [ ] Version documented
- [ ] Full test suite passes
- [ ] Build succeeds on all platforms

### Release

- [ ] Tag created: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
- [ ] Tag pushed: `git push origin vX.Y.Z`
- [ ] GoReleaser triggered (if configured)

### Post-Release

- [ ] GitHub Release notes published
- [ ] Binary assets uploaded
- [ ] Announce release
- [ ] Monitor for issues

---

## Gate Quick Reference

### Run All Gates Quickly

```bash
#!/bin/bash
echo "=== RELEASE GATE VALIDATION ==="

echo ""
echo "Gate 1: Security..."
SEC_ISSUES=$(grep -rn "panic(\|exec.Command" internal/core/ 2>/dev/null | grep -v "_test.go" | wc -l)
echo "  Potential issues: $SEC_ISSUES"

echo ""
echo "Gate 2: Tests..."
go test ./... > /dev/null 2>&1
echo "  Test result: $?"

echo ""
echo "Gate 3: Documentation..."
# Quick drift check
echo "  Commands in main.go: $(grep -c 'case "' main.go)"

echo ""
echo "Gate 4: Code Quality..."
GOFMT_ISSUES=$(gofmt -l internal/ 2>/dev/null | wc -l)
echo "  gofmt issues: $GOFMT_ISSUES"

echo ""
echo "Gate 5: Build..."
go build -o /tmp/git-vendor-test > /dev/null 2>&1
echo "  Build result: $?"

echo ""
echo "=== SUMMARY ==="
if [ "$SEC_ISSUES" -lt 5 ] && [ "$GOFMT_ISSUES" -eq 0 ]; then
  echo "Basic gates: LIKELY PASS"
else
  echo "Basic gates: NEEDS ATTENTION"
fi
```

---

## Integration Points

This workflow orchestrates:
- `/SECURITY_AUDIT` - Security gate
- `/TEST_COVERAGE` - Test gate
- `/DOC_SYNC` - Documentation gate
- `/CODE_REVIEW` - Code quality gate
- `/PERFORMANCE_BASELINE` - Performance gate
- `/DEPENDENCY_AUDIT` - Dependency gate
- `/MIGRATION_PLANNER` - Migration gate

Output:
- `CHANGELOG.md` - Release notes
- Git tag - Version marker
- GitHub Release - Published artifacts
