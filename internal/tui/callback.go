package tui

import (
	"git-vendor/internal/core"

	"github.com/charmbracelet/huh"
)

// TUICallback implements core.UICallback using charmbracelet TUI
type TUICallback struct{}

func NewTUICallback() *TUICallback {
	return &TUICallback{}
}

func (t *TUICallback) ShowError(title, message string) {
	PrintError(title, message)
}

func (t *TUICallback) ShowSuccess(message string) {
	PrintSuccess(message)
}

func (t *TUICallback) ShowWarning(title, message string) {
	PrintWarning(title, message)
}

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

func (t *TUICallback) ShowLicenseCompliance(license string) {
	PrintComplianceSuccess(license)
}

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
func (t *TUICallback) FormatJSON(output core.JSONOutput) error {
	return nil
}
