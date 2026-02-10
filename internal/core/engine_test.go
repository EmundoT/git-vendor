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

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, nil)
	manager := NewManagerWithSyncer(syncer)

	mockConfig.EXPECT().Path().Return(ConfigPath)

	path := manager.ConfigPath()
	if path != ConfigPath {
		t.Errorf("Expected '%s', got '%s'", ConfigPath, path)
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

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, nil)
	manager := NewManagerWithSyncer(syncer)

	mockLock.EXPECT().Path().Return(LockPath)

	path := manager.LockPath()
	if path != LockPath {
		t.Errorf("Expected '%s', got '%s'", LockPath, path)
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

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, nil)
	manager := NewManagerWithSyncer(syncer)

	tests := []struct {
		name     string
		expected string
	}{
		{"test-vendor", VendorDir + "/" + LicensesDir + "/test-vendor.txt"},
		{"another-lib", VendorDir + "/" + LicensesDir + "/another-lib.txt"},
		{"my-package", VendorDir + "/" + LicensesDir + "/my-package.txt"},
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

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, nil)
	manager := NewManagerWithSyncer(syncer)

	// Initially should be silent UI
	if manager.syncer.ui != ui {
		t.Error("Expected initial UI to be SilentUICallback")
	}

	// Set new UI callback
	newUI := &SilentUICallback{}
	manager.SetUICallback(newUI)

	// Verify UI callback changed
	if manager.syncer.ui != newUI {
		t.Error("Expected UI callback to be updated")
	}
}

func TestIsGitInstalled(t *testing.T) {
	// Git should be installed in the test environment (CI requires git)
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

// ============================================================================
// Manager Delegation Method Tests
// ============================================================================

func TestManager_ParseSmartURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(
		NewMockGitClient(ctrl),
		NewMockFileSystem(ctrl),
		NewMockConfigStore(ctrl),
		NewMockLockStore(ctrl),
		NewMockLicenseChecker(ctrl),
	)
	manager := NewManagerWithSyncer(syncer)

	// Test GitHub URL parsing through Manager delegation
	base, ref, path := manager.ParseSmartURL("https://github.com/owner/repo/blob/main/src/file.go")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", ref)
	}
	if path != "src/file.go" {
		t.Errorf("Expected path 'src/file.go', got '%s'", path)
	}
}

func TestManager_UpdateVerboseMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(
		NewMockGitClient(ctrl),
		NewMockFileSystem(ctrl),
		NewMockConfigStore(ctrl),
		NewMockLockStore(ctrl),
		NewMockLicenseChecker(ctrl),
	)
	manager := NewManagerWithSyncer(syncer)

	// UpdateVerboseMode should create a new git client
	manager.UpdateVerboseMode(true)

	// Verify git client was updated (by checking gitClient is not nil)
	if manager.syncer.gitClient == nil {
		t.Error("Expected git client to be updated")
	}
}

// ============================================================================
// ServiceOverrides Tests
// ============================================================================

func TestNewVendorSyncer_ServiceOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	// Create a custom VendorRepository to inject
	customRepo := NewVendorRepository(mockConfig)

	overrides := &ServiceOverrides{
		Repository: customRepo,
	}

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, overrides)

	// Verify the override was applied: repository should be our custom instance
	if syncer.repository != customRepo {
		t.Error("Expected ServiceOverrides.Repository to be injected into VendorSyncer")
	}

	// Verify non-overridden services still got defaults (not nil)
	if syncer.sync == nil {
		t.Error("Expected default SyncService when not overridden")
	}
	if syncer.update == nil {
		t.Error("Expected default UpdateService when not overridden")
	}
	if syncer.validation == nil {
		t.Error("Expected default ValidationService when not overridden")
	}
	if syncer.explorer == nil {
		t.Error("Expected default RemoteExplorer when not overridden")
	}
	if syncer.vulnScanner == nil {
		t.Error("Expected default VulnScanner when not overridden")
	}
}

func TestNewVendorSyncer_NilOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockLicense := NewMockLicenseChecker(ctrl)
	ui := &SilentUICallback{}

	// Pass nil overrides — all services should be defaults
	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, mockLicense, VendorDir, ui, nil)

	// Verify all domain services are non-nil
	if syncer.repository == nil {
		t.Error("Expected default VendorRepository")
	}
	if syncer.sync == nil {
		t.Error("Expected default SyncService")
	}
	if syncer.update == nil {
		t.Error("Expected default UpdateService")
	}
	if syncer.license == nil {
		t.Error("Expected default LicenseService")
	}
	if syncer.validation == nil {
		t.Error("Expected default ValidationService")
	}
	if syncer.explorer == nil {
		t.Error("Expected default RemoteExplorer")
	}
	if syncer.updateChecker == nil {
		t.Error("Expected default UpdateChecker")
	}
	if syncer.verifyService == nil {
		t.Error("Expected default VerifyService")
	}
	if syncer.vulnScanner == nil {
		t.Error("Expected default VulnScanner")
	}
}
