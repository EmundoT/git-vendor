package core

import (
	"fmt"
	"testing"

	"git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Init Tests
// ============================================================================

func TestInit_CreatesEmptyConfig(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: MkdirAll called for vendor/ and vendor/licenses/ (use gomock.Any() for cross-platform paths)
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	// Mock: Config save should be called with empty vendor list (not empty vendor)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors == nil {
			t.Error("Expected Vendors slice to be initialized, got nil")
		}
		if len(cfg.Vendors) != 0 {
			t.Errorf("Expected 0 vendors in config, got %d", len(cfg.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Init()

	// Verify
	assertNoError(t, err, "Init should succeed")
}

func TestInit_DirectoryCreationFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: MkdirAll fails for vendor directory
	fs.EXPECT().MkdirAll("/mock/vendor", gomock.Any()).Return(fmt.Errorf("permission denied"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Init()

	// Verify
	assertError(t, err, "Init should fail when directory creation fails")
	if !contains(err.Error(), "permission denied") {
		t.Errorf("Expected permission denied error, got: %v", err)
	}
}

func TestInit_ConfigSaveFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: Directories created successfully (use gomock.Any() for cross-platform paths)
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	// Mock: Config save fails
	config.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("disk full"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Init()

	// Verify
	assertError(t, err, "Init should fail when config save fails")
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

// ============================================================================
// GetConfig Tests
// ============================================================================

func TestGetConfig(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

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
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	lock.EXPECT().GetHash("test-vendor", "main").Return("abc123hash")

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
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	lock.EXPECT().Load().Return(createTestLock(
		createTestLockEntry("vendor-a", "main", "hash123456"),
		createTestLockEntry("vendor-b", "main", "hash789012"),
	), nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute (just verify no panic)
	syncer.Audit()
}

// ============================================================================
// SaveVendor Tests
// ============================================================================

func TestSaveVendor_NewVendor(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("new-vendor", "https://github.com/owner/repo", "main")

	// First Load returns empty config, subsequent Loads return config with vendor
	savedConfig := createTestConfig()
	config.EXPECT().Load().Return(savedConfig, nil).Times(1)
	config.EXPECT().Load().Return(createTestConfig(vendor), nil).AnyTimes()

	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if len(cfg.Vendors) != 1 {
			t.Errorf("Expected 1 vendor in config, got %d", len(cfg.Vendors))
		}
		if cfg.Vendors[0].Name != "new-vendor" {
			t.Errorf("Expected vendor name 'new-vendor', got '%s'", cfg.Vendors[0].Name)
		}
		savedConfig = cfg
		return nil
	})

	// UpdateAll will sync the new vendor - mock all sync operations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)
	git.EXPECT().Init("/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote("/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch("/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout("/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash("/tmp/test-12345").Return("abc123hash", nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file.go", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	// UpdateAll saves the lock after syncing
	lock.EXPECT().Save(gomock.Any()).Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.SaveVendor(vendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for new vendor")
}

func TestSaveVendor_UpdateExisting(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Start with existing vendor - Load() is called multiple times (by repository.Save and update.UpdateAll)
	existingVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/old-repo", "main")
	config.EXPECT().Load().Return(createTestConfig(existingVendor), nil).AnyTimes()

	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if len(cfg.Vendors) != 1 {
			t.Errorf("Expected 1 vendor (updated, not added), got %d", len(cfg.Vendors))
		}
		if cfg.Vendors[0].URL != "https://github.com/owner/new-repo" {
			t.Errorf("Expected URL to be updated, got '%s'", cfg.Vendors[0].URL)
		}
		return nil
	})

	// UpdateAll will sync the updated vendor - mock all sync operations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)
	git.EXPECT().Init("/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote("/tmp/test-12345", "origin", "https://github.com/owner/new-repo").Return(nil)
	git.EXPECT().Fetch("/tmp/test-12345", 1, "develop").Return(nil)
	git.EXPECT().Checkout("/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash("/tmp/test-12345").Return("def456hash", nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file.go", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	// UpdateAll saves the lock after syncing
	lock.EXPECT().Save(gomock.Any()).Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - update URL
	updatedVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/new-repo", "develop")
	err := syncer.SaveVendor(updatedVendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for existing vendor")
}

func TestSaveVendor_ConfigSaveFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(createTestConfig(), nil)

	// Mock: Config save fails
	config.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("permission denied"))

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
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Start with 2 vendors
	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	vendor2 := createTestVendorSpec("vendor-2", "https://github.com/owner/repo2", "main")

	// First Load returns 2 vendors, subsequent Loads return config with only vendor-2
	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2), nil).Times(1)
	config.EXPECT().Load().Return(createTestConfig(vendor2), nil).AnyTimes()

	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if len(cfg.Vendors) != 1 {
			t.Errorf("Expected 1 vendor remaining, got %d", len(cfg.Vendors))
		}
		if cfg.Vendors[0].Name != "vendor-2" {
			t.Errorf("Expected vendor-2 to remain, got '%s'", cfg.Vendors[0].Name)
		}
		return nil
	})

	// Verify license file removal was attempted
	fs.EXPECT().Remove(gomock.Any()).Return(nil)

	// UpdateAll will sync the remaining vendor (vendor-2) - mock all sync operations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)
	git.EXPECT().Init("/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote("/tmp/test-12345", "origin", "https://github.com/owner/repo2").Return(nil)
	git.EXPECT().Fetch("/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout("/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash("/tmp/test-12345").Return("xyz789hash", nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file.go", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	// UpdateAll saves the lock after syncing
	lock.EXPECT().Save(gomock.Any()).Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - remove vendor-1
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertNoError(t, err, "RemoveVendor should succeed")
}

func TestRemoveVendor_VendorNotFound(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor1), nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - try to remove nonexistent vendor
	err := syncer.RemoveVendor("nonexistent-vendor")

	// Verify
	assertError(t, err, "RemoveVendor should fail for nonexistent vendor")
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestRemoveVendor_ConfigLoadFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: Config load fails
	config.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config file corrupted"))

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
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor1), nil).AnyTimes()

	// Mock: Config save fails - this causes early return, so fs.Remove is never called
	config.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("disk full"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertError(t, err, "RemoveVendor should fail when config save fails")
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}
