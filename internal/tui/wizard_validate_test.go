package tui

import "testing"

func TestValidateFromPath_Empty(t *testing.T) {
	err := validateFromPath("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateFromPath_PlainPath(t *testing.T) {
	err := validateFromPath("src/config.go")
	if err != nil {
		t.Errorf("unexpected error for plain path: %v", err)
	}
}

func TestValidateFromPath_WithLineRange(t *testing.T) {
	err := validateFromPath("src/config.go:L5-L20")
	if err != nil {
		t.Errorf("unexpected error for path with line range: %v", err)
	}
}

func TestValidateFromPath_WithColumnRange(t *testing.T) {
	err := validateFromPath("src/config.go:L5C10:L10C30")
	if err != nil {
		t.Errorf("unexpected error for path with column range: %v", err)
	}
}

func TestValidateFromPath_InvalidPosition(t *testing.T) {
	err := validateFromPath("src/config.go:L0")
	if err == nil {
		t.Error("expected error for L0 (line must be >= 1)")
	}
}

func TestValidateToPath_Empty(t *testing.T) {
	err := validateToPath("")
	if err != nil {
		t.Error("empty should be allowed (auto-naming)")
	}
}

func TestValidateToPath_PlainPath(t *testing.T) {
	err := validateToPath("vendor/config.go")
	if err != nil {
		t.Errorf("unexpected error for plain path: %v", err)
	}
}

func TestValidateToPath_WithPosition(t *testing.T) {
	err := validateToPath("vendor/config.go:L10-L12")
	if err != nil {
		t.Errorf("unexpected error for path with position: %v", err)
	}
}

func TestValidateToPath_InvalidPosition(t *testing.T) {
	err := validateToPath("vendor/config.go:L10-L5")
	if err == nil {
		t.Error("expected error for end line < start line")
	}
}
