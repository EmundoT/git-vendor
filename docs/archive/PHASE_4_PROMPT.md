# Phase 4: Reorganize Mock Infrastructure

Continue implementing Phase 4 of the code quality refactor plan located at:
/home/emt/.claude/plans/transient-exploring-mist.md

Phase 3 (Split Monolithic Test File) is complete. Now implement Phase 4: Reorganize Mock Infrastructure.

This phase involves replacing 522 lines of manual mock implementations with auto-generated mockgen mocks.

## Current State

**Manual Mocks (mocks_test.go - 522 lines):**
- MockGitClient (~120 lines)
- MockFileSystem (~180 lines)
- MockConfigStore (~70 lines)
- MockLockStore (~80 lines)
- MockLicenseChecker (~70 lines)
- Plus helper functions

**Problems:**
- Extensive boilerplate for each mock method
- Manual tracking of calls (InitCalls, FetchCalls, etc.)
- No compile-time safety when interfaces change
- Time-consuming to update when interfaces evolve
- 48+ similar mock method implementations

## Goals

1. **Install mockgen** - Add gomock/mockgen to the project
2. **Create Makefile** - Add mock generation targets
3. **Generate mocks** - Auto-generate mocks for all interfaces
4. **Update tests** - Replace manual mocks with generated ones
5. **Delete manual mocks** - Remove mocks_test.go
6. **Verify tests** - Ensure all tests still pass

## Implementation Steps

### 1. Create Makefile

Create `Makefile` in project root:

```makefile
.PHONY: mocks
mocks:
	@echo "Generating mocks..."
	go install github.com/golang/mock/mockgen@latest
	mockgen -source=internal/core/git_operations.go -destination=internal/core/mocks/git_client_mock.go -package=mocks
	mockgen -source=internal/core/filesystem.go -destination=internal/core/mocks/filesystem_mock.go -package=mocks
	mockgen -source=internal/core/config_store.go -destination=internal/core/mocks/config_store_mock.go -package=mocks
	mockgen -source=internal/core/lock_store.go -destination=internal/core/mocks/lock_store_mock.go -package=mocks
	mockgen -source=internal/core/github_client.go -destination=internal/core/mocks/license_checker_mock.go -package=mocks
	@echo "Done!"

.PHONY: test
test:
	go test -v ./...

.PHONY: coverage
coverage:
	go test -cover ./...

.PHONY: test-core
test-core:
	go test -v ./internal/core/...
```

### 2. Generate Mocks

Run mock generation:
```bash
make mocks
```

This creates:
```
internal/core/mocks/
├── git_client_mock.go         (auto-generated)
├── filesystem_mock.go          (auto-generated)
├── config_store_mock.go        (auto-generated)
├── lock_store_mock.go          (auto-generated)
└── license_checker_mock.go     (auto-generated)
```

### 3. Update Test Helper Functions

Update helper functions in testhelpers.go to use mockgen mocks:

**Before (manual mocks):**
```go
func setupMocks() (*MockGitClient, *MockFileSystem, *MockConfigStore, *MockLockStore, *MockLicenseChecker) {
	git := &MockGitClient{}
	fs := NewMockFileSystem()
	config := &MockConfigStore{
		Config: types.VendorConfig{Vendors: []types.VendorSpec{}},
	}
	lock := &MockLockStore{
		Lock: types.VendorLock{Vendors: []types.LockDetails{}},
	}
	license := &MockLicenseChecker{}
	return git, fs, config, lock, license
}
```

**After (mockgen):**
```go
func setupMocks(t *testing.T) (*mocks.MockGitClient, *mocks.MockFileSystem, *mocks.MockConfigStore, *mocks.MockLockStore, *mocks.MockLicenseChecker) {
	ctrl := gomock.NewController(t)

	git := mocks.NewMockGitClient(ctrl)
	fs := mocks.NewMockFileSystem(ctrl)
	config := mocks.NewMockConfigStore(ctrl)
	lock := mocks.NewMockLockStore(ctrl)
	license := mocks.NewMockLicenseChecker(ctrl)

	// Set up default behaviors
	config.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil).AnyTimes()
	lock.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil).AnyTimes()

	return git, fs, config, lock, license
}
```

### 4. Update Test Files

For each test file, update to use mockgen syntax:

**Before (manual):**
```go
func TestSyncVendor_HappyPath(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock behaviors
	git.InitFunc = func(dir string) error { return nil }
	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123", nil
	}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test", nil
	}

	// Test code...
}
```

**After (mockgen):**
```go
func TestSyncVendor_HappyPath(t *testing.T) {
	git, fs, config, lock, license := setupMocks(t)

	// Mock expectations
	git.EXPECT().Init(gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any()).Return("abc123", nil)

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test", nil)

	// Test code...
}
```

### 5. Update TestBuilder

Update the TestBuilder in testhelpers.go to use mockgen:

```go
type TestBuilder struct {
	t           *testing.T
	ctrl        *gomock.Controller
	vendorDir   string
	config      types.VendorConfig
	lock        types.VendorLock
	gitClient   *mocks.MockGitClient
	fs          *mocks.MockFileSystem
	configStore *mocks.MockConfigStore
	lockStore   *mocks.MockLockStore
	license     *mocks.MockLicenseChecker
	ui          UICallback
}

func NewTestBuilder(t *testing.T) *TestBuilder {
	ctrl := gomock.NewController(t)

	return &TestBuilder{
		t:         t,
		ctrl:      ctrl,
		vendorDir: "/mock/vendor",
		config:    types.VendorConfig{Vendors: []types.VendorSpec{}},
		lock:      types.VendorLock{Vendors: []types.LockDetails{}},
		gitClient: mocks.NewMockGitClient(ctrl),
		fs:        mocks.NewMockFileSystem(ctrl),
		license:   mocks.NewMockLicenseChecker(ctrl),
		ui:        &SilentUICallback{},
	}
}
```

### 6. Common Mockgen Patterns

**Expect specific value:**
```go
git.EXPECT().Init("/tmp/test").Return(nil)
```

**Accept any value:**
```go
git.EXPECT().Init(gomock.Any()).Return(nil)
```

**Multiple calls:**
```go
git.EXPECT().Fetch(gomock.Any(), 1, "main").Return(nil).Times(2)
```

**Any number of times:**
```go
config.EXPECT().Load().Return(cfg, nil).AnyTimes()
```

**Ordered calls:**
```go
gomock.InOrder(
	git.EXPECT().Init(gomock.Any()).Return(nil),
	git.EXPECT().AddRemote(gomock.Any(), "origin", gomock.Any()).Return(nil),
	git.EXPECT().Fetch(gomock.Any(), 1, "main").Return(nil),
)
```

**Custom matchers:**
```go
git.EXPECT().Clone(gomock.Any(), "https://github.com/owner/repo", gomock.Any()).
	Do(func(dir, url string, opts *CloneOptions) {
		if opts.Depth != 1 {
			t.Error("Expected shallow clone")
		}
	}).Return(nil)
```

### 7. Delete Manual Mocks

After all tests are updated and passing:
```bash
git rm internal/core/mocks_test.go
```

### 8. Update Documentation

Add to `.gitignore`:
```
# Generated mocks
internal/core/mocks/
```

Add to README or CLAUDE.md:
```markdown
## Running Tests

Generate mocks before running tests:
```bash
make mocks
make test
```
```

## Migration Strategy

1. **Generate mocks** - Run `make mocks` to create auto-generated mocks
2. **Update one test file at a time** - Start with smallest files
3. **Test after each file** - Run `go test ./internal/core/...`
4. **Keep both temporarily** - Don't delete mocks_test.go until all files updated
5. **Verify coverage** - Ensure coverage maintained or improved
6. **Delete manual mocks** - Remove mocks_test.go when all tests pass

## Order of Test File Updates

1. license_service_test.go (smallest, 2 tests)
2. stores_test.go (4 tests)
3. validation_service_test.go (5 tests)
4. file_copy_service_test.go (5 tests)
5. remote_explorer_test.go (6 tests)
6. update_service_test.go (9 tests)
7. vendor_repository_test.go (10 tests)
8. sync_service_test.go (largest, 14 tests)

## Benefits After Phase 4

✅ **Eliminated 522 lines** of manual mock boilerplate
✅ **Type-safe mocks** - Compiler catches interface changes
✅ **Auto-updates** - Regenerate when interfaces change
✅ **Industry standard** - Uses go-mock (Google's official mock framework)
✅ **Better error messages** - Clear expectation failures
✅ **Reduced maintenance** - No manual mock updates needed

## Success Criteria

- [ ] Makefile created with mock generation targets
- [ ] All mocks auto-generated in internal/core/mocks/
- [ ] All test files updated to use mockgen mocks
- [ ] All tests passing: `go test ./internal/core/...`
- [ ] Coverage maintained or improved (currently 45.9%)
- [ ] mocks_test.go deleted
- [ ] Documentation updated with mock generation instructions

## Testing Checklist

After completing Phase 4, verify:

```bash
# Generate fresh mocks
make mocks

# Run all tests
make test

# Check coverage
make coverage

# Verify no manual mocks remain
ls internal/core/mocks_test.go  # Should not exist

# Verify mocks directory exists
ls internal/core/mocks/  # Should show 5 generated files
```

## Common Pitfalls to Avoid

⚠️ **Don't skip gomock.Controller** - Every test needs `ctrl := gomock.NewController(t)`
⚠️ **Don't forget EXPECT()** - Mockgen requires explicit expectations
⚠️ **Don't ignore AnyTimes()** - Use for repeated calls in setup
⚠️ **Don't commit generated mocks** - Add to .gitignore, generate locally
⚠️ **Don't update all tests at once** - Migrate incrementally, test frequently

## Reference Links

- [GoMock Documentation](https://github.com/golang/mock)
- [Mockgen Usage Guide](https://github.com/golang/mock#running-mockgen)
- [gomock Matchers](https://pkg.go.dev/github.com/golang/mock/gomock)

## What Remains After Phase 4

### Phase 5: Decompose TUI Wizard (Week 6 - LOW RISK)
- Split 498-line wizard.go into 8 focused files
- Extract styles, helpers, browser interface
- ~80 lines saved through organization

### Phase 6: Remove Unnecessary Indirection (Week 7 - MEDIUM RISK)
- Delete 220-line engine.go wrapper
- Rename VendorSyncer → VendorManager
- Direct API usage in main.go

### Phase 7: Document Code Patterns (Week 8 - LOW RISK)
- Create ARCHITECTURE.md
- Create CONTRIBUTING.md
- Create PATTERNS.md

---

**Total Progress After Phase 4:**
- Phase 1: ✅ Complete (~120 lines saved)
- Phase 2: ✅ Complete (~400 lines saved)
- Phase 3: ✅ Complete (~700 lines saved via organization)
- Phase 4: 🔜 Next (522 lines eliminated)
- **Running Total: ~1,742 lines improved/eliminated**
