package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNoOpProgressTracker verifies no-op tracker doesn't panic
func TestNoOpProgressTracker(_ *testing.T) {
	tracker := NewNoOpProgressTracker()

	// Should not panic
	tracker.Increment("test")
	tracker.SetTotal(100)
	tracker.Complete()
	tracker.Fail(nil)
	tracker.Fail(errors.New("test error"))
}

// TestTextProgressTracker verifies text tracker basic functionality
func TestTextProgressTracker(t *testing.T) {
	output := captureStdout(func() {
		tracker := NewTextProgressTracker(5, "Test operation")
		tracker.Increment("Step 1")
		tracker.Increment("Step 2")
		tracker.SetTotal(10)
		tracker.Increment("Step 3")
		tracker.Complete()
	})
	if !strings.Contains(output, "Starting: Test operation") {
		t.Errorf("TextProgressTracker missing start message, got: %q", output)
	}
	if !strings.Contains(output, "Step 1") {
		t.Errorf("TextProgressTracker missing Step 1, got: %q", output)
	}
	if !strings.Contains(output, "Completed") {
		t.Errorf("TextProgressTracker missing completion message, got: %q", output)
	}
}

// TestTextProgressTrackerFailure verifies failure handling
func TestTextProgressTrackerFailure(t *testing.T) {
	output := captureStdout(func() {
		tracker := NewTextProgressTracker(3, "Test operation")
		tracker.Increment("Step 1")
		tracker.Fail(errors.New("simulated error"))
	})
	if !strings.Contains(output, "Failed") {
		t.Errorf("TextProgressTracker missing failure message, got: %q", output)
	}
	if !strings.Contains(output, "simulated error") {
		t.Errorf("TextProgressTracker missing error detail, got: %q", output)
	}
}

// TestTextProgressTracker_IncrementEmptyMessage verifies increment with no message
func TestTextProgressTracker_IncrementEmptyMessage(t *testing.T) {
	output := captureStdout(func() {
		tracker := NewTextProgressTracker(2, "op")
		tracker.Increment("")
	})
	if !strings.Contains(output, "[1/2]") {
		t.Errorf("TextProgressTracker increment with empty message missing count, got: %q", output)
	}
}

// --- progressModel direct tests ---

func TestProgressModel_Init(t *testing.T) {
	m := &progressModel{total: 5, label: "test"}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestProgressModel_Update_WindowSize(t *testing.T) {
	m := &progressModel{total: 5, label: "test", width: 80}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	if cmd != nil {
		t.Error("Update with WindowSizeMsg should return nil cmd")
	}
	model := updated.(*progressModel)
	if model.width != 60 {
		t.Errorf("width = %d, want 60", model.width)
	}
}

func TestProgressModel_Update_Increment(t *testing.T) {
	m := &progressModel{total: 5, label: "test", current: 0}
	updated, cmd := m.Update(progressIncrementMsg{message: "step one"})
	if cmd != nil {
		t.Error("Update with increment should return nil cmd")
	}
	model := updated.(*progressModel)
	if model.current != 1 {
		t.Errorf("current = %d, want 1", model.current)
	}
	if model.message != "step one" {
		t.Errorf("message = %q, want %q", model.message, "step one")
	}
}

func TestProgressModel_Update_SetTotal(t *testing.T) {
	m := &progressModel{total: 5, label: "test"}
	updated, _ := m.Update(progressSetTotalMsg{total: 20})
	model := updated.(*progressModel)
	if model.total != 20 {
		t.Errorf("total = %d, want 20", model.total)
	}
}

func TestProgressModel_Update_Complete(t *testing.T) {
	m := &progressModel{total: 5, label: "test", current: 5}
	updated, cmd := m.Update(progressCompleteMsg{})
	model := updated.(*progressModel)
	if !model.done {
		t.Error("done should be true after complete")
	}
	// Should return tea.Quit
	if cmd == nil {
		t.Error("complete should return a quit cmd")
	}
}

func TestProgressModel_Update_Fail(t *testing.T) {
	m := &progressModel{total: 5, label: "test"}
	testErr := errors.New("test failure")
	updated, cmd := m.Update(progressFailMsg{err: testErr})
	model := updated.(*progressModel)
	if !model.failed {
		t.Error("failed should be true after fail msg")
	}
	if model.err != testErr {
		t.Errorf("err = %v, want %v", model.err, testErr)
	}
	if cmd == nil {
		t.Error("fail should return a quit cmd")
	}
}

func TestProgressModel_View_InProgress(t *testing.T) {
	m := &progressModel{total: 10, label: "syncing", current: 5, width: 80}
	view := m.View()
	if !strings.Contains(view, "syncing") {
		t.Errorf("View missing label, got: %q", view)
	}
	if !strings.Contains(view, "5/10") {
		t.Errorf("View missing progress count, got: %q", view)
	}
}

func TestProgressModel_View_InProgressWithMessage(t *testing.T) {
	m := &progressModel{total: 10, label: "syncing", current: 3, message: "vendor-a", width: 80}
	view := m.View()
	if !strings.Contains(view, "vendor-a") {
		t.Errorf("View missing message, got: %q", view)
	}
}

func TestProgressModel_View_InProgressNarrow(t *testing.T) {
	m := &progressModel{total: 10, label: "syncing", current: 5, width: 60}
	view := m.View()
	// Narrow width (< 80) should use shorter bar
	if !strings.Contains(view, "5/10") {
		t.Errorf("View (narrow) missing progress count, got: %q", view)
	}
}

func TestProgressModel_View_Done(t *testing.T) {
	m := &progressModel{total: 5, label: "syncing", current: 5, done: true}
	view := m.View()
	if !strings.Contains(view, "completed") {
		t.Errorf("View (done) missing 'completed', got: %q", view)
	}
	if !strings.Contains(view, "5/5") {
		t.Errorf("View (done) missing count, got: %q", view)
	}
}

func TestProgressModel_View_Failed(t *testing.T) {
	m := &progressModel{total: 5, label: "syncing", failed: true, err: errors.New("timeout")}
	view := m.View()
	if !strings.Contains(view, "failed") {
		t.Errorf("View (failed) missing 'failed', got: %q", view)
	}
	if !strings.Contains(view, "timeout") {
		t.Errorf("View (failed) missing error, got: %q", view)
	}
}
