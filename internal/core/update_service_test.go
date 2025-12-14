package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git-vendor/internal/types"
)

// ============================================================================
// UpdateAll Tests - Comprehensive tests for update operations
// ============================================================================

func TestUpdateAll_HappyPath_SingleVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Single vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	// Mock: syncVendor succeeds
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def456", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify lock was saved with correct entry
	if len(lock.SaveCalls) != 1 {
		t.Errorf("Expected 1 Save call, got %d", len(lock.SaveCalls))
	}

	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 1 {
		t.Errorf("Expected 1 lock entry, got %d", len(savedLock.Vendors))
	}

	entry := savedLock.Vendors[0]
	if entry.Name != "test-vendor" {
		t.Errorf("Expected vendor name 'test-vendor', got '%s'", entry.Name)
	}
	if entry.Ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", entry.Ref)
	}
	if entry.CommitHash != "abc123def456" {
		t.Errorf("Expected hash 'abc123def456', got '%s'", entry.CommitHash)
	}
	if entry.Updated == "" {
		t.Error("Expected Updated timestamp, got empty string")
	}
}

func TestUpdateAll_HappyPath_MultipleVendors(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "dev")
	vendor3 := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "v1.0")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Each vendor gets a unique hash (must be at least 7 chars)
	callCount := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		callCount++
		return fmt.Sprintf("hash%d00000", callCount), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify lock has 3 entries (one per vendor)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 3 {
		t.Errorf("Expected 3 lock entries, got %d", len(savedLock.Vendors))
	}

	// Verify all vendors are locked
	vendorNames := make(map[string]bool)
	for _, entry := range savedLock.Vendors {
		vendorNames[entry.Name] = true
	}
	if !vendorNames["vendor-a"] || !vendorNames["vendor-b"] || !vendorNames["vendor-c"] {
		t.Error("Not all vendors were locked")
	}
}

func TestUpdateAll_ConfigLoadFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config load fails
	config.LoadFunc = func() (types.VendorConfig, error) {
		return types.VendorConfig{}, fmt.Errorf("config file corrupt")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "config file corrupt") {
		t.Errorf("Expected config error, got: %v", err)
	}

	// Verify no lock save was attempted
	if len(lock.SaveCalls) != 0 {
		t.Errorf("Expected no Save calls, got %d", len(lock.SaveCalls))
	}
}

func TestUpdateAll_OneVendorFails_OthersContinue(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-good-1", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-bad", "https://github.com/owner/repo-b", "main")
	vendor3 := createTestVendorSpec("vendor-good-2", "https://github.com/owner/repo-c", "main")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: vendor-bad fails, others succeed
	git.InitFunc = func(dir string) error {
		// Fail only for vendor-bad (second call)
		if len(git.InitCalls) == 2 {
			return fmt.Errorf("git init failed")
		}
		return nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify: Overall success (UpdateAll continues on individual failures)
	if err != nil {
		t.Fatalf("Expected success (continue on error), got: %v", err)
	}

	// Verify: Only 2 vendors were locked (vendor-bad skipped)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 2 {
		t.Errorf("Expected 2 lock entries (vendor-bad skipped), got %d", len(savedLock.Vendors))
	}

	// Verify the failed vendor is not in the lock
	for _, entry := range savedLock.Vendors {
		if entry.Name == "vendor-bad" {
			t.Error("vendor-bad should not be in lock file")
		}
	}
}

func TestUpdateAll_LockSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: Lock save fails
	lock.SaveFunc = func(l types.VendorLock) error {
		return fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestUpdateAll_EmptyConfig(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Empty config (no vendors)
	config.Config = types.VendorConfig{Vendors: []types.VendorSpec{}}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success (empty is valid), got error: %v", err)
	}

	// Verify empty lock was saved
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 0 {
		t.Errorf("Expected empty lock, got %d entries", len(savedLock.Vendors))
	}
}

func TestUpdateAll_TimestampFormat(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify timestamp is in RFC3339 format
	entry := lock.SaveCalls[0].Vendors[0]
	if entry.Updated == "" {
		t.Fatal("Expected non-empty timestamp")
	}

	// Try to parse the timestamp (should not error)
	_, err = time.Parse(time.RFC3339, entry.Updated)
	if err != nil {
		t.Errorf("Timestamp not in RFC3339 format: %v", err)
	}
}

func TestUpdateAll_MultipleSpecsPerVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 1 vendor with 3 specs
	vendor := types.VendorSpec{
		Name:    "multi-spec-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
			{
				Ref: "dev",
				Mapping: []types.PathMapping{
					{From: "src/dev.go", To: "lib/dev.go"},
				},
			},
			{
				Ref: "v1.0",
				Mapping: []types.PathMapping{
					{From: "src/release.go", To: "lib/release.go"},
				},
			},
		},
	}
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	hashCounter := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify 3 lock entries (one per spec)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 3 {
		t.Errorf("Expected 3 lock entries (one per spec), got %d", len(savedLock.Vendors))
	}

	// Verify all refs are present
	refs := make(map[string]bool)
	for _, entry := range savedLock.Vendors {
		refs[entry.Ref] = true
	}
	if !refs["main"] || !refs["dev"] || !refs["v1.0"] {
		t.Error("Not all refs were locked")
	}
}

func TestUpdateAll_LicensePathSet(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify license path is set correctly
	entry := lock.SaveCalls[0].Vendors[0]
	expectedPath := filepath.Join("/mock/vendor", LicenseDir, "test-vendor.txt")
	if entry.LicensePath != expectedPath {
		t.Errorf("Expected license path '%s', got '%s'", expectedPath, entry.LicensePath)
	}
}
