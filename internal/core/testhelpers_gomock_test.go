package core

import (
	"testing"

	"github.com/golang/mock/gomock"
)

// ============================================================================
// Gomock Test Helpers
// ============================================================================

// setupMocks creates all mock dependencies with gomock
func setupMocks(t *testing.T) (
	*gomock.Controller,
	*MockGitClient,
	*MockFileSystem,
	*MockConfigStore,
	*MockLockStore,
	*MockLicenseChecker,
) {
	ctrl := gomock.NewController(t)

	git := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)
	config := NewMockConfigStore(ctrl)
	lock := NewMockLockStore(ctrl)
	license := NewMockLicenseChecker(ctrl)

	return ctrl, git, fs, config, lock, license
}

// createMockSyncer creates a VendorSyncer with mock dependencies
func createMockSyncer(
	git GitClient,
	fs FileSystem,
	config ConfigStore,
	lock LockStore,
	license LicenseChecker,
	ui UICallback,
) *VendorSyncer {
	if ui == nil {
		ui = &SilentUICallback{}
	}
	return NewVendorSyncer(config, lock, git, fs, license, "/mock/vendor", ui)
}
