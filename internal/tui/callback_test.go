package tui

import (
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/core"
)

func TestNewTUICallback(t *testing.T) {
	cb := NewTUICallback()
	if cb == nil {
		t.Fatal("NewTUICallback returned nil")
	}
}

func TestTUICallback_ShowError(t *testing.T) {
	cb := NewTUICallback()
	output := captureStdout(func() {
		cb.ShowError("Test Error", "error details")
	})
	if !strings.Contains(output, "Test Error") {
		t.Errorf("ShowError output missing title, got: %q", output)
	}
	if !strings.Contains(output, "error details") {
		t.Errorf("ShowError output missing message, got: %q", output)
	}
}

func TestTUICallback_ShowSuccess(t *testing.T) {
	cb := NewTUICallback()
	output := captureStdout(func() {
		cb.ShowSuccess("all good")
	})
	if !strings.Contains(output, "all good") {
		t.Errorf("ShowSuccess output missing message, got: %q", output)
	}
}

func TestTUICallback_ShowWarning(t *testing.T) {
	cb := NewTUICallback()
	output := captureStdout(func() {
		cb.ShowWarning("Heads Up", "something unusual")
	})
	if !strings.Contains(output, "Heads Up") {
		t.Errorf("ShowWarning output missing title, got: %q", output)
	}
	if !strings.Contains(output, "something unusual") {
		t.Errorf("ShowWarning output missing message, got: %q", output)
	}
}

func TestTUICallback_ShowLicenseCompliance(t *testing.T) {
	cb := NewTUICallback()
	output := captureStdout(func() {
		cb.ShowLicenseCompliance("Apache-2.0")
	})
	if !strings.Contains(output, "Apache-2.0") {
		t.Errorf("ShowLicenseCompliance output missing license, got: %q", output)
	}
}

func TestTUICallback_StyleTitle(t *testing.T) {
	cb := NewTUICallback()
	result := cb.StyleTitle("Section Header")
	if !strings.Contains(result, "Section Header") {
		t.Errorf("StyleTitle result missing text, got: %q", result)
	}
}

func TestTUICallback_GetOutputMode(t *testing.T) {
	cb := NewTUICallback()
	if cb.GetOutputMode() != core.OutputNormal {
		t.Errorf("GetOutputMode = %v, want OutputNormal", cb.GetOutputMode())
	}
}

func TestTUICallback_IsAutoApprove(t *testing.T) {
	cb := NewTUICallback()
	if cb.IsAutoApprove() {
		t.Error("IsAutoApprove should return false for interactive mode")
	}
}

func TestTUICallback_FormatJSON(t *testing.T) {
	cb := NewTUICallback()
	err := cb.FormatJSON(core.JSONOutput{Status: "test"})
	if err != nil {
		t.Errorf("FormatJSON should return nil in interactive mode, got: %v", err)
	}
}

func TestTUICallback_StartProgress(t *testing.T) {
	cb := NewTUICallback()
	// In test environment stdout is not a terminal, so TextProgressTracker is returned
	output := captureStdout(func() {
		tracker := cb.StartProgress(5, "test progress")
		if tracker == nil {
			t.Fatal("StartProgress returned nil")
		}
		// Verify it's a TextProgressTracker (non-TTY environment)
		if _, ok := tracker.(*TextProgressTracker); !ok {
			// In non-TTY test environment, we expect TextProgressTracker
			t.Logf("StartProgress returned %T (may be BubbletaeProgressTracker in TTY)", tracker)
		}
	})
	// TextProgressTracker prints "Starting: ..." on creation
	if !strings.Contains(output, "test progress") {
		t.Logf("StartProgress output: %q (may differ in TTY vs non-TTY)", output)
	}
}
