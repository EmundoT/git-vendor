# Phase 7: Enhanced Testing & Quality

**Prerequisites:** Phase 6 complete (multi-platform support)
**Goal:** Increase test coverage to 70%+, add integration tests, improve code quality
**Priority:** LOW - Current coverage adequate, but this provides production hardening
**Estimated Effort:** 6-8 hours

---

## Current State

**Test Coverage:** 52.7% overall
- Critical business logic: 84-100% ✅
- Wrapper methods: 0% (not a concern)
- Audit() function: 60%
- Some edge cases untested

**Test Quality:**
- ✅ Comprehensive unit tests with mocks
- ✅ Error path testing
- ✅ Table-driven tests
- ❌ No integration tests with real git
- ❌ No property-based testing
- ❌ No concurrent operation tests

---

## Goals

1. **Increase Coverage to 70%+** - Test more edge cases and error paths
2. **Integration Tests** - Test with real git repositories
3. **Property-Based Testing** - Fuzz test config/lock parsing
4. **Concurrent Operation Tests** - Verify thread safety
5. **Benchmark Tests** - Performance regression detection
6. **Test Fixtures** - Reusable test repositories

---

## Implementation Steps

### 1. Integration Test Framework

Create `internal/core/integration_test.go`:

```go
// +build integration

package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRealGitOperations tests with actual git repositories
func TestRealGitOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a real test repository
	testRepo := createTestRepository(t)
	defer os.RemoveAll(testRepo)

	vendorDir := filepath.Join(t.TempDir(), "vendor")
	manager := NewManager()
	manager.RootDir = vendorDir

	// Initialize
	if err := manager.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add a vendor from the test repo
	spec := types.VendorSpec{
		Name:    "test-vendor",
		URL:     testRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "test.txt", To: "vendored/test.txt"},
				},
			},
		},
	}

	if err := manager.SaveVendor(spec); err != nil {
		t.Fatalf("SaveVendor failed: %v", err)
	}

	// Sync
	if err := manager.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify file was copied
	vendoredFile := filepath.Join(vendorDir, "../vendored/test.txt")
	if _, err := os.Stat(vendoredFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist", vendoredFile)
	}

	// Update
	if err := manager.UpdateAll(); err != nil {
		t.Fatalf("UpdateAll failed: %v", err)
	}

	// Verify lockfile
	lock, err := manager.syncer.lockStore.Load()
	if err != nil {
		t.Fatalf("Failed to load lockfile: %v", err)
	}

	if len(lock.Vendors) != 1 {
		t.Errorf("Expected 1 vendor in lockfile, got %d", len(lock.Vendors))
	}
}

func createTestRepository(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create LICENSE file
	licenseFile := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(licenseFile, []byte("MIT License\n\nCopyright (c) 2025\n"), 0644); err != nil {
		t.Fatalf("Failed to create LICENSE: %v", err)
	}

	// Commit
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Initial commit")

	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
```

Run with:
```bash
go test -tags=integration ./internal/core/...
```

### 2. Property-Based Testing

Create `internal/core/property_test.go`:

```go
package core

import (
	"testing"
	"testing/quick"
)

func TestValidateDestPath_NeverAllowsTraversal(t *testing.T) {
	fs := NewOSFileSystem()

	// Property: No path should allow traversal
	property := func(path string) bool {
		err := fs.ValidateDestPath(path)

		// If path contains "..", it should be rejected
		if strings.Contains(path, "..") {
			return err != nil
		}

		// If path is absolute, it should be rejected
		if filepath.IsAbs(path) {
			return err != nil
		}

		// Otherwise, it might be valid
		return true
	}

	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

func TestPathMapping_RoundTrip(t *testing.T) {
	// Property: Serializing and deserializing should be identity
	property := func(from, to string) bool {
		mapping := types.PathMapping{From: from, To: to}

		// Serialize to YAML
		data, err := yaml.Marshal(mapping)
		if err != nil {
			return true // Skip invalid inputs
		}

		// Deserialize
		var decoded types.PathMapping
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			return false
		}

		// Should match original
		return decoded.From == from && decoded.To == to
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}
```

### 3. Concurrent Operation Tests

Create `internal/core/concurrent_test.go`:

```go
package core

import (
	"sync"
	"testing"
)

func TestConcurrentConfigReads(t *testing.T) {
	vendorDir := t.TempDir()
	store := NewFileConfigStore(vendorDir)

	// Write initial config
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("test", "https://github.com/test/repo", "main"),
		},
	}
	if err := store.Save(config); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Concurrent reads should not cause issues
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Load(); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent read failed: %v", err)
	}
}

func TestConcurrentSyncsShouldBeSerialzied(t *testing.T) {
	// This test documents current behavior (no protection)
	// and could be updated if locking is added in future

	t.Skip("No concurrent sync protection implemented yet")

	// TODO: When file locking is added, test that:
	// 1. Second sync blocks until first completes
	// 2. Or second sync fails immediately with clear error
	// 3. Lockfile is not corrupted by concurrent writes
}
```

### 4. Benchmark Tests

Create `internal/core/benchmark_test.go`:

```go
package core

import (
	"testing"
)

func BenchmarkParseSmartURL(b *testing.B) {
	urls := []string{
		"https://github.com/owner/repo",
		"https://github.com/owner/repo/blob/main/src/file.go",
		"https://gitlab.com/owner/repo/-/blob/main/file.go",
	}

	manager := NewManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := urls[i%len(urls)]
		manager.ParseSmartURL(url)
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	vendorDir := b.TempDir()
	manager := NewManager()
	manager.RootDir = vendorDir

	// Create config with multiple vendors
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor1", "https://github.com/owner/repo1", "main"),
			createTestVendorSpec("vendor2", "https://github.com/owner/repo2", "main"),
			createTestVendorSpec("vendor3", "https://github.com/owner/repo3", "main"),
		},
	}

	store := NewFileConfigStore(vendorDir)
	if err := store.Save(config); err != nil {
		b.Fatalf("Failed to save config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := manager.ValidateConfig(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSyncVendor(b *testing.B) {
	// Benchmark with mocks to isolate sync logic
	// Use real git for integration benchmark

	b.ReportAllocs()

	// ... mock setup ...

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Run sync
	}
}
```

Run with:
```bash
go test -bench=. -benchmem ./internal/core/
```

### 5. Add Missing Unit Tests

**Test Audit() function (`internal/core/vendor_repository_test.go`):**

```go
func TestAudit(t *testing.T) {
	tests := []struct {
		name   string
		config types.VendorConfig
		lock   types.VendorLock
		want   string // Expected output pattern
	}{
		{
			name: "All vendors in sync",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					createTestVendorSpec("vendor1", "url", "main"),
				},
			},
			lock: types.VendorLock{
				Vendors: []types.LockDetails{
					createTestLockEntry("vendor1", "main", "abc123"),
				},
			},
			want: "in sync",
		},
		{
			name: "Missing from lockfile",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					createTestVendorSpec("vendor1", "url", "main"),
				},
			},
			lock: types.VendorLock{Vendors: []types.LockDetails{}},
			want: "missing",
		},
		{
			name: "Empty config and lock",
			config: types.VendorConfig{Vendors: []types.VendorSpec{}},
			lock:   types.VendorLock{Vendors: []types.LockDetails{}},
			want:   "No vendors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, git, fs, config, lock, license := setupMocks(t)
			defer ctrl.Finish()

			config.EXPECT().Load().Return(tt.config, nil)
			lock.EXPECT().Load().Return(tt.lock, nil)

			syncer := createMockSyncer(git, fs, config, lock, license, nil)

			// Capture output
			var buf bytes.Buffer
			syncer.ui = &capturingUICallback{output: &buf}

			syncer.Audit()

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("Audit() output = %q, want substring %q", output, tt.want)
			}
		})
	}
}
```

**Test edge cases in config parsing (`internal/core/stores_test.go`):**

```go
func TestLoadConfig_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "Invalid YAML syntax",
			yaml:    "vendors:\n  - name: [unclosed",
			wantErr: true,
		},
		{
			name: "Wrong type for field",
			yaml: "vendors: \"not an array\"",
			wantErr: true,
		},
		{
			name: "Extra unknown fields (should be ignored)",
			yaml: "vendors: []\nunknown_field: value",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := NewFileConfigStore(dir)

			// Write malformed YAML
			configPath := filepath.Join(dir, "vendor/vendor.yml")
			os.MkdirAll(filepath.Dir(configPath), 0755)
			os.WriteFile(configPath, []byte(tt.yaml), 0644)

			_, err := store.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

### 6. Test Fixtures for Reuse

Create `internal/core/testdata/`:

```
testdata/
├── repos/
│   ├── simple/          # Simple test repository
│   │   ├── .git/
│   │   ├── file.txt
│   │   └── LICENSE
│   └── nested/          # Repository with subdirectories
│       ├── .git/
│       ├── src/
│       │   └── main.go
│       └── LICENSE
├── configs/
│   ├── valid.yml        # Valid config examples
│   ├── empty.yml
│   └── invalid.yml
└── locks/
    ├── valid.lock
    └── stale.lock
```

Use in tests:

```go
func loadTestConfig(t *testing.T, name string) types.VendorConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "configs", name))
	if err != nil {
		t.Fatalf("Failed to load test config %s: %v", name, err)
	}

	var config types.VendorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse test config: %v", err)
	}

	return config
}
```

### 7. Update Makefile

```makefile
# ... existing targets ...

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
```

---

## Verification Checklist

After implementing Phase 7, verify:

- [ ] Test coverage increased to 70%+
- [ ] Integration tests run with `go test -tags=integration`
- [ ] Property-based tests pass
- [ ] Concurrent tests pass
- [ ] Benchmarks run successfully
- [ ] No performance regressions
- [ ] CI updated to run integration tests
- [ ] Coverage report shows improvement

**Check coverage:**
```bash
make test-coverage-html
# Open coverage.html and verify:
# - Audit() function: >90%
# - All critical paths: >85%
# - Overall: >70%
```

---

## Expected Outcomes

**After Phase 7:**
- ✅ Test coverage: 70%+ (from 52.7%)
- ✅ Integration tests verify real git operations
- ✅ Property-based tests catch edge cases
- ✅ Benchmarks prevent performance regressions
- ✅ Higher confidence in production stability

**Quality Metrics:**
- Unit test count: 55 → 75+
- Integration test count: 0 → 10+
- Coverage: 52.7% → 70%+
- Critical path coverage: 84-100% (maintained)

---

## Next Steps

After Phase 7 completion:
- **Phase 8:** Advanced features (update checker, parallel sync, vendor groups)
- **Optional:** Performance optimization based on benchmark results
- **Optional:** Add mutation testing for test quality verification
