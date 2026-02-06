package core

import (
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"

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
) *VendorSyncer {
	return NewVendorSyncer(config, lock, git, fs, license, "/mock/vendor", &SilentUICallback{}, nil)
}

// capturingUICallback captures UI output for testing
type capturingUICallback struct {
	errorMsg    string
	successMsg  string
	warningMsg  string
	confirmResp bool
	licenseMsg  string
}

func (c *capturingUICallback) ShowError(title, message string) {
	c.errorMsg = title + ": " + message
}

func (c *capturingUICallback) ShowSuccess(message string) {
	c.successMsg = message
}

func (c *capturingUICallback) ShowWarning(title, message string) {
	c.warningMsg = title + ": " + message
}

func (c *capturingUICallback) AskConfirmation(_, _ string) bool {
	return c.confirmResp
}

func (c *capturingUICallback) ShowLicenseCompliance(license string) {
	c.licenseMsg = license
}

func (c *capturingUICallback) StyleTitle(title string) string {
	return title
}

func (c *capturingUICallback) GetOutputMode() OutputMode {
	return OutputNormal
}

func (c *capturingUICallback) IsAutoApprove() bool {
	return false
}

func (c *capturingUICallback) FormatJSON(_ JSONOutput) error {
	return nil
}

func (c *capturingUICallback) StartProgress(_ int, _ string) types.ProgressTracker {
	return &NoOpProgressTracker{}
}
