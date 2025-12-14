package core

import (
	"fmt"
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// GetConfig Tests
// ============================================================================

func TestGetConfig(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	cfg, err := syncer.GetConfig()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(cfg.Vendors) != 1 {
		t.Errorf("Expected 1 vendor, got %d", len(cfg.Vendors))
	}
}

// ============================================================================
// GetLockHash Tests
// ============================================================================

func TestGetLockHash(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	lock.Lock = createTestLock(createTestLockEntry("test-vendor", "main", "abc123hash"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hash := syncer.GetLockHash("test-vendor", "main")

	// Verify
	if hash != "abc123hash" {
		t.Errorf("Expected hash 'abc123hash', got '%s'", hash)
	}
}

// ============================================================================
// Audit Tests
// ============================================================================

func TestAudit(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	lock.Lock = createTestLock(
		createTestLockEntry("vendor-a", "main", "hash123456"),
		createTestLockEntry("vendor-b", "main", "hash789012"),
	)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute (just verify no panic)
	syncer.Audit()
}

// ============================================================================
// SaveVendor Tests
// ============================================================================

func TestSaveVendor_NewVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with empty config
	config.Config = createTestConfig()

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	vendor := createTestVendorSpec("new-vendor", "https://github.com/owner/repo", "main")
	err := syncer.SaveVendor(vendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for new vendor")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor in config, got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].Name != "new-vendor" {
		t.Errorf("Expected vendor name 'new-vendor', got '%s'", savedConfig.Vendors[0].Name)
	}
}

func TestSaveVendor_UpdateExisting(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with existing vendor
	existingVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/old-repo", "main")
	config.Config = createTestConfig(existingVendor)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - update URL
	updatedVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/new-repo", "develop")
	err := syncer.SaveVendor(updatedVendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for existing vendor")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor (updated, not added), got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].URL != "https://github.com/owner/new-repo" {
		t.Errorf("Expected URL to be updated, got '%s'", savedConfig.Vendors[0].URL)
	}
}

func TestSaveVendor_ConfigSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config save fails
	config.SaveFunc = func(cfg types.VendorConfig) error {
		return fmt.Errorf("permission denied")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	err := syncer.SaveVendor(vendor)

	// Verify
	assertError(t, err, "SaveVendor should fail when config save fails")
	if !contains(err.Error(), "permission denied") {
		t.Errorf("Expected permission denied error, got: %v", err)
	}
}

// ============================================================================
// RemoveVendor Tests
// ============================================================================

func TestRemoveVendor_HappyPath(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with 2 vendors
	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	vendor2 := createTestVendorSpec("vendor-2", "https://github.com/owner/repo2", "main")
	config.Config = createTestConfig(vendor1, vendor2)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - remove vendor-1
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertNoError(t, err, "RemoveVendor should succeed")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor remaining, got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].Name != "vendor-2" {
		t.Errorf("Expected vendor-2 to remain, got '%s'", savedConfig.Vendors[0].Name)
	}
	// Verify license file removal was attempted
	if len(fs.RemoveCalls) == 0 {
		t.Error("Expected license file removal to be called")
	}
}

func TestRemoveVendor_VendorNotFound(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.Config = createTestConfig(vendor1)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - try to remove nonexistent vendor
	err := syncer.RemoveVendor("nonexistent-vendor")

	// Verify
	assertError(t, err, "RemoveVendor should fail for nonexistent vendor")
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
	// Verify config was not saved
	if len(config.SaveCalls) > 0 {
		t.Error("Expected config to not be saved when vendor not found")
	}
}

func TestRemoveVendor_ConfigLoadFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config load fails
	config.LoadFunc = func() (types.VendorConfig, error) {
		return types.VendorConfig{}, fmt.Errorf("config file corrupted")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.RemoveVendor("any-vendor")

	// Verify
	assertError(t, err, "RemoveVendor should fail when config load fails")
	if !contains(err.Error(), "config file corrupted") {
		t.Errorf("Expected corrupted config error, got: %v", err)
	}
}

func TestRemoveVendor_ConfigSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.Config = createTestConfig(vendor1)

	// Mock: Config save fails
	config.SaveFunc = func(cfg types.VendorConfig) error {
		return fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertError(t, err, "RemoveVendor should fail when config save fails")
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}
