package core

import (
	"os"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Common Test Helpers
// ============================================================================

// createTestVendorSpec creates a basic vendor spec for testing
func createTestVendorSpec(name, url, ref string) types.VendorSpec {
	return types.VendorSpec{
		Name:    name,
		URL:     url,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: ref,
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
		},
	}
}

// createTestConfig creates a vendor config with the given vendors
func createTestConfig(vendors ...types.VendorSpec) types.VendorConfig {
	return types.VendorConfig{Vendors: vendors}
}

// createTestLockEntry creates a lock entry for testing
func createTestLockEntry(name, ref, hash string) types.LockDetails {
	return types.LockDetails{
		Name:        name,
		Ref:         ref,
		CommitHash:  hash,
		Updated:     time.Now().Format(time.RFC3339),
		LicensePath: "",
	}
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 1024 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

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

// ============================================================================
// Test Assertion Helpers
// ============================================================================

// assertNoError fails the test if err is not nil
func assertNoError(t interface {
	Fatalf(format string, args ...interface{})
}, err error, msg string) {
	if err != nil {
		t.Fatalf("%s: expected no error, got: %v", msg, err)
	}
}

// assertError fails the test if err is nil
func assertError(t interface {
	Fatalf(format string, args ...interface{})
}, err error, msg string) {
	if err == nil {
		t.Fatalf("%s: expected error, got nil", msg)
	}
}

// ============================================================================
// Manager Test Helpers
// ============================================================================

// newTestManager creates a Manager with real implementations for integration testing
func newTestManager(vendorDir string) *Manager {
	config := NewFileConfigStore(vendorDir)
	lock := NewFileLockStore(vendorDir)
	git := NewSystemGitClient(false) // not verbose
	fs := NewOSFileSystem()
	license := NewGitHubLicenseChecker(nil, AllowedLicenses)
	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(config, lock, git, fs, license, vendorDir, ui)
	return NewManagerWithSyncer(syncer)
}
