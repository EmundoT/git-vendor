package tui

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/EmundoT/git-vendor/internal/core"
)

func TestNonInteractiveTUICallback_ShowError_Quiet(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputQuiet,
	})

	callback.ShowError("Test Error", "This should not appear")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if buf.String() != "" {
		t.Errorf("Expected no output in quiet mode, got: %s", buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowError_JSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	callback.ShowError("Test Error", "Test message")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var output core.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", output.Status)
	}

	if output.Error == nil {
		t.Fatal("Expected error object to be present")
	}

	if output.Error.Title != "Test Error" {
		t.Errorf("Expected error title 'Test Error', got '%s'", output.Error.Title)
	}

	if output.Error.Message != "Test message" {
		t.Errorf("Expected error message 'Test message', got '%s'", output.Error.Message)
	}
}

func TestNonInteractiveTUICallback_ShowSuccess_Normal(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	callback.ShowSuccess("Operation succeeded")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	expected := "Operation succeeded\n"
	if buf.String() != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowSuccess_Quiet(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputQuiet,
	})

	callback.ShowSuccess("This should not appear")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if buf.String() != "" {
		t.Errorf("Expected no output in quiet mode, got: %s", buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowSuccess_JSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	callback.ShowSuccess("Operation succeeded")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var output core.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", output.Status)
	}

	if output.Message != "Operation succeeded" {
		t.Errorf("Expected message 'Operation succeeded', got '%s'", output.Message)
	}
}

func TestNonInteractiveTUICallback_AskConfirmation_Yes(t *testing.T) {
	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Yes: true,
	})

	result := callback.AskConfirmation("Test Confirmation", "Proceed?")

	if !result {
		t.Error("Expected auto-approve to return true with --yes flag")
	}
}

func TestNonInteractiveTUICallback_AskConfirmation_NoYes(t *testing.T) {
	// Capture stderr (where error will be shown)
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Yes:  false,
		Mode: core.OutputNormal,
	})

	result := callback.AskConfirmation("Test Confirmation", "Proceed?")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if result {
		t.Error("Expected confirmation to return false without --yes flag")
	}

	if buf.Len() == 0 {
		t.Error("Expected error message to be shown when confirmation is requested without --yes")
	}
}

func TestNonInteractiveTUICallback_FormatJSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	output := core.JSONOutput{
		Status:  "success",
		Message: "Test message",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	err := callback.FormatJSON(output)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var parsed core.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if parsed.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", parsed.Status)
	}

	if parsed.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", parsed.Message)
	}

	if parsed.Data["key1"] != "value1" {
		t.Errorf("Expected data.key1 'value1', got '%v'", parsed.Data["key1"])
	}

	// JSON numbers are unmarshaled as float64
	if parsed.Data["key2"] != float64(42) {
		t.Errorf("Expected data.key2 42, got '%v'", parsed.Data["key2"])
	}
}

func TestNonInteractiveTUICallback_GetOutputMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     core.OutputMode
		expected core.OutputMode
	}{
		{"Normal mode", core.OutputNormal, core.OutputNormal},
		{"Quiet mode", core.OutputQuiet, core.OutputQuiet},
		{"JSON mode", core.OutputJSON, core.OutputJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
				Mode: tt.mode,
			})

			if callback.GetOutputMode() != tt.expected {
				t.Errorf("Expected output mode %v, got %v", tt.expected, callback.GetOutputMode())
			}
		})
	}
}

func TestNonInteractiveTUICallback_IsAutoApprove(t *testing.T) {
	tests := []struct {
		name     string
		yes      bool
		expected bool
	}{
		{"With --yes flag", true, true},
		{"Without --yes flag", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
				Yes: tt.yes,
			})

			if callback.IsAutoApprove() != tt.expected {
				t.Errorf("Expected IsAutoApprove %v, got %v", tt.expected, callback.IsAutoApprove())
			}
		})
	}
}

func TestNonInteractiveTUICallback_ShowWarning_JSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	callback.ShowWarning("Test Warning", "Warning message")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var output core.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Status != "warning" {
		t.Errorf("Expected status 'warning', got '%s'", output.Status)
	}

	if output.Message != "Test Warning: Warning message" {
		t.Errorf("Expected message 'Test Warning: Warning message', got '%s'", output.Message)
	}
}

func TestNonInteractiveTUICallback_ShowLicenseCompliance_JSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	callback.ShowLicenseCompliance("MIT")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var output core.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", output.Status)
	}

	if output.Data["license"] != "MIT" {
		t.Errorf("Expected license 'MIT', got '%v'", output.Data["license"])
	}
}

func TestNonInteractiveTUICallback_StyleTitle(t *testing.T) {
	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	input := "Test Title"
	result := callback.StyleTitle(input)

	// In non-interactive mode, StyleTitle should return plain text
	if result != input {
		t.Errorf("Expected StyleTitle to return plain text '%s', got '%s'", input, result)
	}
}

func TestNonInteractiveTUICallback_ShowWarning_Normal(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	callback.ShowWarning("Test Warning", "This is a warning")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	expected := "Warning: Test Warning - This is a warning\n"
	if buf.String() != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowWarning_Quiet(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputQuiet,
	})

	callback.ShowWarning("Test Warning", "This should not appear")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if buf.String() != "" {
		t.Errorf("Expected no output in quiet mode, got: %s", buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowLicenseCompliance_Normal(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	callback.ShowLicenseCompliance("Apache-2.0")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	expected := "License Verified: Apache-2.0\n"
	if buf.String() != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, buf.String())
	}
}

func TestNonInteractiveTUICallback_ShowLicenseCompliance_Quiet(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputQuiet,
	})

	callback.ShowLicenseCompliance("MIT")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if buf.String() != "" {
		t.Errorf("Expected no output in quiet mode, got: %s", buf.String())
	}
}

func TestNonInteractiveTUICallback_StartProgress_Normal(t *testing.T) {
	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	tracker := callback.StartProgress(10, "Test Progress")

	if tracker == nil {
		t.Fatal("Expected progress tracker to be created")
	}

	// Verify it's a TextProgressTracker (not NoOp)
	_, isNoOp := tracker.(*NoOpProgressTracker)
	if isNoOp {
		t.Error("Expected TextProgressTracker in normal mode, got NoOpProgressTracker")
	}
}

func TestNonInteractiveTUICallback_StartProgress_Quiet(t *testing.T) {
	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputQuiet,
	})

	tracker := callback.StartProgress(10, "Test Progress")

	if tracker == nil {
		t.Fatal("Expected progress tracker to be created")
	}

	// Verify it's a NoOpProgressTracker in quiet mode
	_, isNoOp := tracker.(*NoOpProgressTracker)
	if !isNoOp {
		t.Error("Expected NoOpProgressTracker in quiet mode")
	}
}

func TestNonInteractiveTUICallback_StartProgress_JSON(t *testing.T) {
	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputJSON,
	})

	tracker := callback.StartProgress(10, "Test Progress")

	if tracker == nil {
		t.Fatal("Expected progress tracker to be created")
	}

	// Verify it's a NoOpProgressTracker in JSON mode
	_, isNoOp := tracker.(*NoOpProgressTracker)
	if !isNoOp {
		t.Error("Expected NoOpProgressTracker in JSON mode")
	}
}
