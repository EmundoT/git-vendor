package tui

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
	return AskToOverrideCompliance(title)
}

func (t *TUICallback) ShowLicenseCompliance(license string) {
	PrintComplianceSuccess(license)
}

func (t *TUICallback) StyleTitle(title string) string {
	return StyleTitle(title)
}
