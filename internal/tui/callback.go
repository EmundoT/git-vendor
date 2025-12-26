// Package tui provides terminal user interface components and callbacks for git-vendor.
package tui

import (
	"git-vendor/internal/core"

	"github.com/charmbracelet/huh"
)

// TUICallback implements UICallback for interactive terminal use with styled output.
//
//nolint:revive // Name TUICallback is intentional and descriptive
type TUICallback struct{}

// NewTUICallback creates a new interactive terminal UI callback.
func NewTUICallback() *TUICallback {
	return &TUICallback{}
}

// ShowError displays an error message with confirmation dialog.
func (t *TUICallback) ShowError(title, message string) {
	PrintError(title, message)
}

// ShowSuccess displays a success message with styled output.
func (t *TUICallback) ShowSuccess(message string) {
	PrintSuccess(message)
}

// ShowWarning displays a warning message with styled output.
func (t *TUICallback) ShowWarning(title, message string) {
	PrintWarning(title, message)
}

// AskConfirmation prompts the user for yes/no confirmation.
func (t *TUICallback) AskConfirmation(title, message string) bool {
	var confirm bool
	err := huh.NewConfirm().
		Title(title).
		Description(message).
		Value(&confirm).
		Affirmative("Yes").
		Negative("No").
		Run()
	if err != nil {
		return false
	}
	return confirm
}

// ShowLicenseCompliance displays license verification information.
func (t *TUICallback) ShowLicenseCompliance(license string) {
	PrintComplianceSuccess(license)
}

// StyleTitle returns a styled title string for terminal output.
func (t *TUICallback) StyleTitle(title string) string {
	return StyleTitle(title)
}

// GetOutputMode returns the output mode (normal for interactive TUI)
func (t *TUICallback) GetOutputMode() core.OutputMode {
	return core.OutputNormal
}

// IsAutoApprove returns whether auto-approve is enabled (always false for interactive mode)
func (t *TUICallback) IsAutoApprove() bool {
	return false
}

// FormatJSON is not used in interactive mode
func (t *TUICallback) FormatJSON(_ core.JSONOutput) error {
	return nil
}
