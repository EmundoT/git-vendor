# Phase 7 Implementation Plan: Enhanced Testing & Quality

## Executive Summary

**Current State:**
- Test coverage: 43.1% in internal/core (down from previously reported 52.7%)
- 9 test files with comprehensive gomock-based unit tests
- No integration tests, property-based tests, benchmarks, or concurrent tests
- Well-established test infrastructure with mockgen

**Target State:**
- Test coverage: 70%+
- Integration tests with real git operations
- Property-based tests for security-critical validation
- Concurrent operation tests
- Benchmark tests for performance regression detection
- Comprehensive test fixtures and testdata

**Estimated Effort:** 6-8 hours (as per Phase 7 prompt)

---

## Current Test Infrastructure Analysis

### Strengths
✅ Well-organized test files by service/domain
✅ Gomock infrastructure fully established
✅ Helper functions for test setup (setupMocks, createMockSyncer)
✅ Table-driven tests for multiple scenarios
✅ Error path testing implemented

### Gaps
❌ Basic Audit() test only checks for no panic (vendor_repository_test.go:134)
❌ No integration tests with real git
❌ No property-based testing for security validation
❌ No concurrent operation tests
❌ No benchmark tests
❌ No testdata directory with fixtures
❌ Limited edge case testing for config/lock parsing

---

## Implementation Steps (Prioritized)

### Step 1: Add Missing Unit Tests (High Priority)
**Coverage Impact:** +15-20%
**Time:** 2-3 hours

#### 1.1 Expand Audit() Tests
File: `internal/core/vendor_repository_test.go`

Add comprehensive table-driven tests for Audit():
- All vendors in sync
- Vendors missing from lockfile
- Vendors with outdated refs
- Empty config and lock
- Lock file with orphaned vendors (not in config)
- Config load errors
- Lock load errors

**Implementation approach:**
- Create a capturing UI callback to test output messages
- Verify correct messages for each scenario
- Test both JSON and normal output modes

#### 1.2 Config/Lock Parsing Edge Cases
File: `internal/core/stores_test.go`

Add tests for:
- Invalid YAML syntax
- Wrong types for fields
- Unknown fields (should be ignored)
- Empty files
- Very large configs (100+ vendors)
- Unicode in vendor names/paths
- Special characters in paths

#### 1.3 Path Security Validation
File: Create `internal/core/security_test.go`

Comprehensive tests for `ValidateDestPath`:
- Absolute paths (Unix and Windows)
- Parent directory traversal (../)
- Hidden traversal (%2e%2e, Unicode tricks)
- Null bytes
- Very long paths (4096+ chars)
- Symlink-like patterns

### Step 2: Property-Based Testing (Medium Priority)
**Coverage Impact:** +5%
**Time:** 1-2 hours

File: Create `internal/core/property_test.go`

#### 2.1 Security Property Tests
```go
// Property: ValidateDestPath never allows traversal
func TestValidateDestPath_NeverAllowsTraversal(t *testing.T)

// Property: All paths with ".." are rejected
func TestValidateDestPath_RejectsParentRefs(t *testing.T)

// Property: All absolute paths are rejected
func TestValidateDestPath_RejectsAbsolutePaths(t *testing.T)
```

#### 2.2 Serialization Property Tests
```go
// Property: Config round-trip is identity
func TestConfigSerialization_RoundTrip(t *testing.T)

// Property: Lock round-trip is identity
func TestLockSerialization_RoundTrip(t *testing.T)

// Property: PathMapping round-trip is identity
func TestPathMapping_RoundTrip(t *testing.T)
```

**Dependencies:** Use `testing/quick` package (stdlib, no external deps)

### Step 3: Integration Tests (Medium Priority)
**Coverage Impact:** +5-10%
**Time:** 2-3 hours

File: Create `internal/core/integration_test.go`

#### 3.1 Setup Integration Test Framework
```go
// +build integration

// Helper to create real git test repositories
func createTestRepository(t *testing.T) string

// Helper to run git commands in test repo
func runGit(t *testing.T, dir string, args ...string)
```

#### 3.2 Integration Test Cases
- Full workflow: Init → Add → Sync → Update
- Real git clone with depth=1
- Real file copying with path mappings
- License file detection and copying
- Lock file generation and reading
- Multi-vendor sync
- Update with commit changes

**Run with:** `go test -tags=integration ./internal/core/...`

**CI Integration:** Add separate CI job for integration tests

### Step 4: Concurrent Operation Tests (Low Priority)
**Coverage Impact:** +2-3%
**Time:** 1 hour

File: Create `internal/core/concurrent_test.go`

#### 4.1 Thread-Safety Tests
```go
// Test: Concurrent config reads should be safe
func TestConcurrentConfigReads(t *testing.T)

// Test: Concurrent lock reads should be safe
func TestConcurrentLockReads(t *testing.T)
```

#### 4.2 Document Current Limitations
```go
// Test: Document that concurrent syncs are NOT protected
func TestConcurrentSyncs_NotSupported(t *testing.T) {
    t.Skip("No concurrent sync protection implemented yet")
    // TODO: Add file locking in future phase
}
```

**Note:** This documents expected behavior and prepares for future file locking

### Step 5: Benchmark Tests (Low Priority)
**Coverage Impact:** 0% (doesn't affect coverage)
**Time:** 1 hour

File: Create `internal/core/benchmark_test.go`

#### 5.1 Core Operation Benchmarks
```go
func BenchmarkParseSmartURL(b *testing.B)
func BenchmarkValidateConfig(b *testing.B)
func BenchmarkDetectConflicts(b *testing.B)
func BenchmarkValidateDestPath(b *testing.B)
```

#### 5.2 Serialization Benchmarks
```go
func BenchmarkConfigLoad(b *testing.B)
func BenchmarkConfigSave(b *testing.B)
func BenchmarkLockLoad(b *testing.B)
func BenchmarkLockSave(b *testing.B)
```

**Run with:** `go test -bench=. -benchmem ./internal/core/`

**Output:** Save to `benchmark.txt` for regression detection

### Step 6: Test Fixtures (Low Priority)
**Coverage Impact:** +3-5%
**Time:** 1 hour

#### 6.1 Create testdata Directory Structure
```
internal/core/testdata/
├── configs/
│   ├── valid-simple.yml           # 1 vendor, 1 ref
│   ├── valid-complex.yml          # 3 vendors, multiple refs
│   ├── valid-empty.yml            # Empty vendor list
│   ├── invalid-syntax.yml         # Malformed YAML
│   ├── invalid-missing-name.yml   # Missing required fields
│   └── large.yml                  # 50+ vendors for performance
├── locks/
│   ├── valid-simple.lock
│   ├── valid-complex.lock
│   ├── stale.lock                 # Outdated commit hashes
│   └── orphaned.lock              # Vendors not in config
└── sample-files/
    ├── LICENSE                    # Sample license file
    ├── source.go                  # Sample Go file
    └── README.md                  # Sample markdown
```

#### 6.2 Create Helper Functions
```go
// loadTestConfig loads a config fixture from testdata
func loadTestConfig(t *testing.T, name string) types.VendorConfig

// loadTestLock loads a lock fixture from testdata
func loadTestLock(t *testing.T, name string) types.VendorLock

// loadTestFile loads arbitrary test data
func loadTestFile(t *testing.T, name string) []byte
```

**Usage:** Reuse fixtures across multiple test files

### Step 7: Update Build Infrastructure
**Time:** 30 minutes

#### 7.1 Update Makefile
Add new targets:
```makefile
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -tags=integration -v ./internal/core/...

.PHONY: test-all
test-all: mocks test test-integration
	@echo "All tests passed!"

.PHONY: bench
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./internal/core/ | tee benchmark.txt

.PHONY: test-coverage-html
test-coverage-html:
	@echo "Generating HTML coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in browser"

.PHONY: test-property
test-property:
	@echo "Running property-based tests..."
	go test -v -run TestProperty ./internal/core/
```

#### 7.2 Update .gitignore
```gitignore
# Test outputs
coverage.out
coverage.html
benchmark.txt

# Test data (auto-generated)
/internal/core/testdata/temp_*
```

#### 7.3 Update CI Workflow
Add to `.github/workflows/ci.yml`:
- Separate job for integration tests
- Coverage report upload to Codecov
- Benchmark comparison on PRs

---

## Execution Order (Recommended)

### Phase A: Quick Wins (Day 1)
1. Add missing unit tests for Audit() (1.1)
2. Add config/lock parsing edge cases (1.2)
3. Update Makefile with new targets (7.1)
4. Generate initial coverage report

**Goal:** Reach 55-60% coverage

### Phase B: Security Hardening (Day 2)
1. Create comprehensive path security tests (1.3)
2. Add property-based security tests (2.1)
3. Add serialization property tests (2.2)

**Goal:** Reach 60-65% coverage, validate security

### Phase C: Integration & Performance (Day 3)
1. Create integration test framework (3.1)
2. Add core integration tests (3.2)
3. Add benchmark tests (5.1, 5.2)
4. Create test fixtures (6.1, 6.2)

**Goal:** Reach 70%+ coverage

### Phase D: Documentation & Polish
1. Add concurrent tests (4.1, 4.2)
2. Update CI workflow (7.3)
3. Document test patterns in CLAUDE.md
4. Final coverage verification

**Goal:** Complete Phase 7 checklist

---

## Success Criteria

### Coverage Targets
- [ ] Overall coverage: 70%+ (currently 43.1%)
- [ ] Audit() function: 90%+ (currently minimal)
- [ ] ValidateDestPath: 100% (security-critical)
- [ ] Config/Lock I/O: Maintain 100%
- [ ] Core sync logic: Maintain 85%+

### Test Quality Metrics
- [ ] Unit tests: 55 → 80+ test cases
- [ ] Integration tests: 0 → 10+ test cases
- [ ] Property tests: 0 → 5+ properties
- [ ] Benchmark tests: 0 → 8+ benchmarks

### Infrastructure
- [ ] Integration tests run with `-tags=integration`
- [ ] Benchmarks run with `make bench`
- [ ] Coverage report generates HTML
- [ ] CI runs all test types
- [ ] Testdata fixtures reusable

### Documentation
- [ ] Test patterns documented in CLAUDE.md
- [ ] Makefile targets documented
- [ ] Integration test setup documented
- [ ] Property test rationale documented

---

## Risk Mitigation

### Risk: Coverage may not reach 70%
**Mitigation:**
- Focus on high-value untested code first
- Identify and test wrapper functions if needed
- Add tests for edge cases in git_operations.go
- Test main.go command dispatch logic

### Risk: Integration tests flaky on CI
**Mitigation:**
- Use hermetic test repositories (t.TempDir())
- Set explicit timeouts
- Mock GitHub API in integration tests (use local repos only)
- Skip if git not available (check in test setup)

### Risk: Property tests find real bugs
**Mitigation:**
- This is actually good! Fix bugs immediately
- Document findings in commit messages
- Add regression tests for found issues
- May extend timeline if critical bugs found

---

## Dependencies & Prerequisites

### Required
- Go 1.23+ (already in use)
- `testing/quick` package (stdlib)
- `github.com/golang/mock` (already installed)
- Git command-line tool (for integration tests)

### Optional
- `codecov` tool for coverage reporting
- `benchstat` for benchmark comparison

### No New External Dependencies
All testing can be done with stdlib and existing dependencies.

---

## Post-Phase 7 Recommendations

After completing Phase 7, consider:

1. **Phase 8:** Advanced features (per PHASE_8_PROMPT.md)
2. **Optional:** Mutation testing with `go-mutesting`
3. **Optional:** Fuzz testing with `go test -fuzz`
4. **Optional:** Add property tests for git URL parsing
5. **Optional:** Performance profiling with pprof

---

## Archive Previous Phases

Before starting Phase 7, move completed phase docs:

```bash
# Move Phase 5 and 6 docs to archive
mv PHASE_5_PROMPT.md docs/archive/
mv PHASE_5_VALIDATION.md docs/archive/
mv PHASE_6_PROMPT.md docs/archive/

# Update archive README
# (document what each phase accomplished)
```

---

## Notes & Gotchas

1. **Cross-platform paths:** Tests must work on Windows and Unix (use filepath.Join)
2. **Mock regeneration:** Run `make mocks` before running new tests
3. **Integration test isolation:** Each test creates its own temp directory
4. **Git availability:** Integration tests should gracefully skip if git not found
5. **Coverage calculation:** Excludes mock files (correct behavior)
6. **Property test limits:** Use reasonable MaxCount (100-1000) to keep tests fast
7. **Benchmark stability:** Run benchmarks 3x to verify consistency
8. **Testdata commits:** Check in testdata fixtures (they're test inputs, not generated)

---

## Quick Reference Commands

```bash
# Generate mocks
make mocks

# Run all unit tests
make test

# Run with coverage
make coverage

# Run integration tests
make test-integration

# Run all tests
make test-all

# Run benchmarks
make bench

# Generate HTML coverage report
make test-coverage-html

# Run property tests only
make test-property

# Run specific test
go test -v -run TestAudit ./internal/core/

# Run tests with race detector
go test -race ./...
```

---

**Ready to begin implementation!**
