package core

import (
	"os"
	"path/filepath"
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// TestBuilder - Fluent API for Test Setup
// ============================================================================

// TestBuilder provides a fluent API for setting up test scenarios
type TestBuilder struct {
	t          *testing.T
	vendorDir  string
	config     types.VendorConfig
	lock       types.VendorLock
	gitClient  *MockGitClient
	fs         *MockFileSystem
	configStore *MockConfigStore
	lockStore   *MockLockStore
	license    *MockLicenseChecker
	ui         UICallback
}

// NewTestBuilder creates a new test builder with default setup
func NewTestBuilder(t *testing.T) *TestBuilder {
	return &TestBuilder{
		t:         t,
		vendorDir: "/mock/vendor",
		config:    types.VendorConfig{Vendors: []types.VendorSpec{}},
		lock:      types.VendorLock{Vendors: []types.LockDetails{}},
		gitClient: &MockGitClient{},
		fs:        NewMockFileSystem(),
		license:   &MockLicenseChecker{},
		ui:        &SilentUICallback{},
	}
}

// WithVendorDir sets a custom vendor directory
func (b *TestBuilder) WithVendorDir(dir string) *TestBuilder {
	b.vendorDir = dir
	return b
}

// WithVendor adds a vendor to the configuration
func (b *TestBuilder) WithVendor(name, url string) *TestBuilder {
	vendor := types.VendorSpec{
		Name:    name,
		URL:     url,
		License: "MIT",
		Specs:   []types.BranchSpec{},
	}
	b.config.Vendors = append(b.config.Vendors, vendor)
	return b
}

// WithVendorSpec adds a complete vendor spec to configuration
func (b *TestBuilder) WithVendorSpec(vendor types.VendorSpec) *TestBuilder {
	b.config.Vendors = append(b.config.Vendors, vendor)
	return b
}

// WithRef adds a ref/branch spec to the last vendor
func (b *TestBuilder) WithRef(ref string) *TestBuilder {
	if len(b.config.Vendors) == 0 {
		b.t.Fatal("Cannot add ref: no vendors configured. Call WithVendor first.")
		return b
	}

	lastIdx := len(b.config.Vendors) - 1
	spec := types.BranchSpec{
		Ref:     ref,
		Mapping: []types.PathMapping{},
	}
	b.config.Vendors[lastIdx].Specs = append(b.config.Vendors[lastIdx].Specs, spec)
	return b
}

// WithMapping adds a path mapping to the last ref of the last vendor
func (b *TestBuilder) WithMapping(from, to string) *TestBuilder {
	if len(b.config.Vendors) == 0 {
		b.t.Fatal("Cannot add mapping: no vendors configured")
		return b
	}

	lastVendor := len(b.config.Vendors) - 1
	if len(b.config.Vendors[lastVendor].Specs) == 0 {
		b.t.Fatal("Cannot add mapping: no refs configured. Call WithRef first.")
		return b
	}

	lastSpec := len(b.config.Vendors[lastVendor].Specs) - 1
	mapping := types.PathMapping{From: from, To: to}
	b.config.Vendors[lastVendor].Specs[lastSpec].Mapping = append(
		b.config.Vendors[lastVendor].Specs[lastSpec].Mapping,
		mapping,
	)
	return b
}

// WithDefaultTarget sets the default target for the last ref
func (b *TestBuilder) WithDefaultTarget(target string) *TestBuilder {
	if len(b.config.Vendors) == 0 {
		b.t.Fatal("Cannot set default target: no vendors configured")
		return b
	}

	lastVendor := len(b.config.Vendors) - 1
	if len(b.config.Vendors[lastVendor].Specs) == 0 {
		b.t.Fatal("Cannot set default target: no refs configured")
		return b
	}

	lastSpec := len(b.config.Vendors[lastVendor].Specs) - 1
	b.config.Vendors[lastVendor].Specs[lastSpec].DefaultTarget = target
	return b
}

// WithLock adds a lock entry
func (b *TestBuilder) WithLock(name, ref, hash string) *TestBuilder {
	entry := types.LockDetails{
		Name:        name,
		Ref:         ref,
		CommitHash:  hash,
		Updated:     "2025-12-11T00:00:00Z",
		LicensePath: "vendor/licenses/" + name + ".txt",
	}
	b.lock.Vendors = append(b.lock.Vendors, entry)
	return b
}

// WithLockEntry adds a complete lock entry
func (b *TestBuilder) WithLockEntry(entry types.LockDetails) *TestBuilder {
	b.lock.Vendors = append(b.lock.Vendors, entry)
	return b
}

// WithGitBehavior configures git mock behavior
func (b *TestBuilder) WithGitBehavior(fn func(*MockGitClient)) *TestBuilder {
	fn(b.gitClient)
	return b
}

// WithFilesystem configures filesystem mock behavior
func (b *TestBuilder) WithFilesystem(fn func(*MockFileSystem)) *TestBuilder {
	fn(b.fs)
	return b
}

// WithLicenseChecker configures license checker behavior
func (b *TestBuilder) WithLicenseChecker(fn func(*MockLicenseChecker)) *TestBuilder {
	fn(b.license)
	return b
}

// WithUICallback sets a custom UI callback
func (b *TestBuilder) WithUICallback(ui UICallback) *TestBuilder {
	b.ui = ui
	return b
}

// BuildVendorSyncer creates a VendorSyncer with the configured mocks
func (b *TestBuilder) BuildVendorSyncer() *VendorSyncer {
	b.configStore = &MockConfigStore{Config: b.config}
	b.lockStore = &MockLockStore{Lock: b.lock}

	return NewVendorSyncer(
		b.configStore,
		b.lockStore,
		b.gitClient,
		b.fs,
		b.license,
		b.vendorDir,
		b.ui,
	)
}

// BuildManager creates a Manager with the configured mocks
func (b *TestBuilder) BuildManager() *Manager {
	syncer := b.BuildVendorSyncer()
	return NewManagerWithSyncer(syncer)
}

// GetMocks returns the mock objects for custom assertions
func (b *TestBuilder) GetMocks() (*MockGitClient, *MockFileSystem, *MockConfigStore, *MockLockStore, *MockLicenseChecker) {
	if b.configStore == nil {
		b.configStore = &MockConfigStore{Config: b.config}
	}
	if b.lockStore == nil {
		b.lockStore = &MockLockStore{Lock: b.lock}
	}
	return b.gitClient, b.fs, b.configStore, b.lockStore, b.license
}

// ============================================================================
// Common Test Helpers
// ============================================================================

// SetupDefaultGitSuccess configures git mocks for successful operations
func SetupDefaultGitSuccess(git *MockGitClient) {
	git.InitFunc = func(dir string) error { return nil }
	git.AddRemoteFunc = func(dir, name, url string) error { return nil }
	git.FetchFunc = func(dir string, depth int, ref string) error { return nil }
	git.FetchAllFunc = func(dir string) error { return nil }
	git.CheckoutFunc = func(dir, ref string) error { return nil }
	git.GetHeadHashFunc = func(dir string) (string, error) { return "abc123def456", nil }
	git.CloneFunc = func(dir, url string, opts *CloneOptions) error { return nil }
	git.ListTreeFunc = func(dir, ref, subdir string) ([]string, error) { return []string{"file.go"}, nil }
}

// SetupDefaultFilesystem configures filesystem mocks for successful operations
func SetupDefaultFilesystem(fs *MockFileSystem) {
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}
	fs.CopyFileFunc = func(src, dst string) error { return nil }
	fs.CopyDirFunc = func(src, dst string) error { return nil }
}

// SetupDefaultLicense configures license checker for MIT license
func SetupDefaultLicense(license *MockLicenseChecker) {
	license.CheckLicenseFunc = func(url string) (string, error) {
		return "MIT", nil
	}
	license.IsAllowedFunc = func(spdxID string) bool {
		return spdxID == "MIT" || spdxID == "Apache-2.0" || spdxID == "BSD-3-Clause"
	}
}

// ============================================================================
// String Helpers
// ============================================================================

// contains checks if string s contains substring substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring finds a substring within a string
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
