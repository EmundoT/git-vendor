package tui

import (
	"encoding/json"
	"fmt"
	"os"

	"git-vendor/internal/core"
)

// NonInteractiveTUICallback handles non-interactive mode output
type NonInteractiveTUICallback struct {
	flags core.NonInteractiveFlags
}

// NewNonInteractiveTUICallback creates a new non-interactive callback
func NewNonInteractiveTUICallback(flags core.NonInteractiveFlags) *NonInteractiveTUICallback {
	return &NonInteractiveTUICallback{flags: flags}
}

// ShowError displays an error message
func (n *NonInteractiveTUICallback) ShowError(title, message string) {
	if n.flags.Mode == core.OutputJSON {
		n.FormatJSON(core.JSONOutput{
			Status: "error",
			Error: &core.JSONError{
				Title:   title,
				Message: message,
			},
		})
	} else if n.flags.Mode != core.OutputQuiet {
		// Print to stderr for non-quiet mode
		fmt.Fprintf(os.Stderr, "Error: %s - %s\n", title, message)
	}
}

// ShowSuccess displays a success message
func (n *NonInteractiveTUICallback) ShowSuccess(message string) {
	if n.flags.Mode == core.OutputJSON {
		n.FormatJSON(core.JSONOutput{
			Status:  "success",
			Message: message,
		})
	} else if n.flags.Mode != core.OutputQuiet {
		fmt.Println(message)
	}
}

// ShowWarning displays a warning message
func (n *NonInteractiveTUICallback) ShowWarning(title, message string) {
	if n.flags.Mode == core.OutputJSON {
		n.FormatJSON(core.JSONOutput{
			Status:  "warning",
			Message: fmt.Sprintf("%s: %s", title, message),
		})
	} else if n.flags.Mode != core.OutputQuiet {
		fmt.Fprintf(os.Stderr, "Warning: %s - %s\n", title, message)
	}
}

// AskConfirmation handles confirmation prompts
func (n *NonInteractiveTUICallback) AskConfirmation(title, message string) bool {
	if n.flags.Yes {
		return true // Auto-approve
	}
	// In non-interactive mode without --yes, fail for safety
	n.ShowError("Interactive Prompt Required",
		fmt.Sprintf("%s: %s\nUse --yes to auto-approve", title, message))
	return false
}

// ShowLicenseCompliance displays license compliance information
func (n *NonInteractiveTUICallback) ShowLicenseCompliance(license string) {
	if n.flags.Mode == core.OutputJSON {
		n.FormatJSON(core.JSONOutput{
			Status:  "success",
			Message: fmt.Sprintf("License Verified: %s", license),
			Data: map[string]interface{}{
				"license": license,
			},
		})
	} else if n.flags.Mode != core.OutputQuiet {
		fmt.Printf("License Verified: %s\n", license)
	}
}

// StyleTitle returns a styled title (no styling in non-interactive mode)
func (n *NonInteractiveTUICallback) StyleTitle(title string) string {
	// Return plain text in non-interactive mode
	return title
}

// GetOutputMode returns the current output mode
func (n *NonInteractiveTUICallback) GetOutputMode() core.OutputMode {
	return n.flags.Mode
}

// IsAutoApprove returns whether auto-approve is enabled
func (n *NonInteractiveTUICallback) IsAutoApprove() bool {
	return n.flags.Yes
}

// FormatJSON formats and outputs JSON to stdout
func (n *NonInteractiveTUICallback) FormatJSON(output core.JSONOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
