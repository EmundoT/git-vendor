package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	progressStyleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	progressStyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	progressStyleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
)

// ========================================
// Bubbletea Progress Model
// ========================================

// progressModel is a bubbletea model for rendering progress
type progressModel struct {
	current int
	total   int
	label   string
	message string
	done    bool
	failed  bool
	err     error
	width   int
}

func (m progressModel) Init() tea.Cmd {
	return nil
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case progressIncrementMsg:
		m.current++
		m.message = msg.message
	case progressSetTotalMsg:
		m.total = msg.total
	case progressCompleteMsg:
		m.done = true
		return m, tea.Quit
	case progressFailMsg:
		m.failed = true
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m progressModel) View() string {
	if m.done {
		return progressStyleSuccess.Render(fmt.Sprintf("✓ %s (completed: %d/%d)", m.label, m.current, m.total))
	}

	if m.failed {
		return progressStyleErr.Render(fmt.Sprintf("✗ %s (failed: %v)", m.label, m.err))
	}

	// Render progress bar
	percent := float64(m.current) / float64(m.total)
	barWidth := 40
	if m.width < 80 {
		barWidth = 20
	}
	filled := int(percent * float64(barWidth))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	status := fmt.Sprintf("[%s] %d/%d", bar, m.current, m.total)
	if m.message != "" {
		status += fmt.Sprintf(" - %s", m.message)
	}

	return fmt.Sprintf("%s\n%s", progressStyleTitle.Render(m.label), status)
}

// ========================================
// Bubbletea Messages
// ========================================

type progressIncrementMsg struct {
	message string
}

type progressSetTotalMsg struct {
	total int
}

type progressCompleteMsg struct{}

type progressFailMsg struct {
	err error
}

// ========================================
// BubbletaeProgressTracker Implementation
// ========================================

// BubbletaeProgressTracker manages progress using bubbletea
type BubbletaeProgressTracker struct {
	program *tea.Program
}

// NewBubbletaeProgressTracker creates a new bubbletea progress tracker
func NewBubbletaeProgressTracker(total int, label string) *BubbletaeProgressTracker {
	m := progressModel{
		current: 0,
		total:   total,
		label:   label,
		width:   80,
	}

	p := tea.NewProgram(m)

	tracker := &BubbletaeProgressTracker{
		program: p,
	}

	// Start program in background
	go func() {
		_, _ = p.Run()
	}()

	return tracker
}

// Increment updates progress with a message.
func (t *BubbletaeProgressTracker) Increment(message string) {
	t.program.Send(progressIncrementMsg{message: message})
}

// SetTotal sets the total count for the progress tracker.
func (t *BubbletaeProgressTracker) SetTotal(total int) {
	t.program.Send(progressSetTotalMsg{total: total})
}

// Complete marks the operation as complete.
func (t *BubbletaeProgressTracker) Complete() {
	t.program.Send(progressCompleteMsg{})
	time.Sleep(100 * time.Millisecond) // Allow final render
}

// Fail marks the operation as failed with an error.
func (t *BubbletaeProgressTracker) Fail(err error) {
	t.program.Send(progressFailMsg{err: err})
	time.Sleep(100 * time.Millisecond) // Allow final render
}

// ========================================
// Text Progress (Non-TTY)
// ========================================

// TextProgressTracker provides simple text-based progress
type TextProgressTracker struct {
	current int
	total   int
	label   string
}

// NewTextProgressTracker creates a new text progress tracker
func NewTextProgressTracker(total int, label string) *TextProgressTracker {
	fmt.Printf("Starting: %s (0/%d)\n", label, total)
	return &TextProgressTracker{
		current: 0,
		total:   total,
		label:   label,
	}
}

// Increment updates progress with a message.
func (t *TextProgressTracker) Increment(message string) {
	t.current++
	msg := fmt.Sprintf("  [%d/%d]", t.current, t.total)
	if message != "" {
		msg += " " + message
	}
	fmt.Println(msg)
}

// SetTotal sets the total count for the progress tracker.
func (t *TextProgressTracker) SetTotal(total int) {
	t.total = total
}

// Complete marks the operation as complete.
func (t *TextProgressTracker) Complete() {
	fmt.Printf("✓ %s: Completed (%d/%d)\n", t.label, t.current, t.total)
}

// Fail marks the operation as failed with an error.
func (t *TextProgressTracker) Fail(err error) {
	fmt.Printf("✗ %s: Failed - %v\n", t.label, err)
}

// ========================================
// No-Op Progress (Quiet/JSON)
// ========================================

// NoOpProgressTracker does nothing (for quiet/JSON/testing modes)
type NoOpProgressTracker struct{}

// NewNoOpProgressTracker creates a new no-op progress tracker
func NewNoOpProgressTracker() *NoOpProgressTracker {
	return &NoOpProgressTracker{}
}

// Increment does nothing (no-op implementation).
func (t *NoOpProgressTracker) Increment(_ string) {}

// SetTotal does nothing (no-op implementation).
func (t *NoOpProgressTracker) SetTotal(_ int) {}

// Complete does nothing (no-op implementation).
func (t *NoOpProgressTracker) Complete() {}

// Fail does nothing (no-op implementation).
func (t *NoOpProgressTracker) Fail(_ error) {}
