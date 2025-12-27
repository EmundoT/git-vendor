package tui

import (
	"errors"
	"testing"
)

// TestNoOpProgressTracker verifies no-op tracker doesn't panic
func TestNoOpProgressTracker(t *testing.T) {
	tracker := NewNoOpProgressTracker()

	// Should not panic
	tracker.Increment("test")
	tracker.SetTotal(100)
	tracker.Complete()
	tracker.Fail(nil)
	tracker.Fail(errors.New("test error"))
}

// TestTextProgressTracker verifies text tracker basic functionality
// Output is manually inspected, not asserted
func TestTextProgressTracker(t *testing.T) {
	tracker := NewTextProgressTracker(5, "Test operation")

	tracker.Increment("Step 1")
	tracker.Increment("Step 2")
	tracker.SetTotal(10) // Dynamic total
	tracker.Increment("Step 3")
	tracker.Complete()
}

// TestTextProgressTrackerFailure verifies failure handling
func TestTextProgressTrackerFailure(t *testing.T) {
	tracker := NewTextProgressTracker(3, "Test operation")

	tracker.Increment("Step 1")
	tracker.Fail(errors.New("simulated error"))
}

// Note: BubbletaeProgressTracker not tested in unit tests
// Requires TTY environment, tested manually
