package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
)

func TestManager_ConfigPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, "vendor", ui)
	manager := NewManagerWithSyncer(syncer)

	mockConfig.EXPECT().Path().Return("vendor/vendor.yml")

	path := manager.ConfigPath()
	if path != "vendor/vendor.yml" {
		t.Errorf("Expected 'vendor/vendor.yml', got '%s'", path)
	}
}

func TestManager_LockPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, "vendor", ui)
	manager := NewManagerWithSyncer(syncer)

	mockLock.EXPECT().Path().Return("vendor/vendor.lock")

	path := manager.LockPath()
	if path != "vendor/vendor.lock" {
		t.Errorf("Expected 'vendor/vendor.lock', got '%s'", path)
	}
}

func TestManager_LicensePath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, "vendor", ui)
	manager := NewManagerWithSyncer(syncer)

	tests := []struct {
		name     string
		expected string
	}{
		{"test-vendor", "vendor/licenses/test-vendor.txt"},
		{"another-lib", "vendor/licenses/another-lib.txt"},
		{"my-package", "vendor/licenses/my-package.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := manager.LicensePath(tt.name)
			if path != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, path)
			}
		})
	}
}

func TestManager_SetUICallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, "vendor", ui)
	manager := NewManagerWithSyncer(syncer)

	// Initially should be silent UI
	if manager.syncer.ui != ui {
		t.Error("Expected initial UI to be SilentUICallback")
	}

	// Set new UI callback
	newUI := &SilentUICallback{}
	manager.SetUICallback(newUI)

	// Verify it changed
	if manager.syncer.ui != newUI {
		t.Error("Expected UI callback to be updated")
	}
}

func TestIsGitInstalled(t *testing.T) {
	// Git should be installed in the test environment (CI requires it)
	installed := IsGitInstalled()
	if !installed {
		t.Error("Expected git to be installed in test environment")
	}
}

func TestIsVendorInitialized(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Should not be initialized initially
	if IsVendorInitialized() {
		t.Error("Expected vendor to not be initialized")
	}

	// Create vendor directory
	vendorPath := filepath.Join(tempDir, VendorDir)
	if err := os.Mkdir(vendorPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Should be initialized now
	if !IsVendorInitialized() {
		t.Error("Expected vendor to be initialized after creating directory")
	}

	// Remove vendor directory
	if err := os.RemoveAll(vendorPath); err != nil {
		t.Fatal(err)
	}

	// Should not be initialized again
	if IsVendorInitialized() {
		t.Error("Expected vendor to not be initialized after removal")
	}
}
