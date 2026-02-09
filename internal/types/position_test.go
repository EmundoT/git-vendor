package types

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// ParsePathPosition Tests
// ============================================================================

func TestParsePathPosition_NoPosition(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantFile string
	}{
		{name: "simple file", path: "src/file.go", wantFile: "src/file.go"},
		{name: "directory", path: "src/", wantFile: "src/"},
		{name: "root file", path: "README.md", wantFile: "README.md"},
		{name: "nested path", path: "a/b/c/d.txt", wantFile: "a/b/c/d.txt"},
		{name: "empty string", path: "", wantFile: ""},
		{name: "colon but not L", path: "src:file.go", wantFile: "src:file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if pos != nil {
				t.Errorf("position = %+v, want nil", pos)
			}
		})
	}
}

func TestParsePathPosition_SingleLine(t *testing.T) {
	file, pos, err := ParsePathPosition("src/file.go:L5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "src/file.go" {
		t.Errorf("file = %q, want %q", file, "src/file.go")
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if pos.StartLine != 5 {
		t.Errorf("StartLine = %d, want 5", pos.StartLine)
	}
	if pos.EndLine != 0 {
		t.Errorf("EndLine = %d, want 0 (single line)", pos.EndLine)
	}
	if !pos.IsSingleLine() {
		t.Error("expected IsSingleLine() to be true")
	}
}

func TestParsePathPosition_LineRange(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantFile  string
		wantStart int
		wantEnd   int
	}{
		{
			name:      "dash syntax",
			path:      "src/file.go:L5-L20",
			wantFile:  "src/file.go",
			wantStart: 5,
			wantEnd:   20,
		},
		{
			name:      "colon syntax",
			path:      "api/types.ts:L10:L50",
			wantFile:  "api/types.ts",
			wantStart: 10,
			wantEnd:   50,
		},
		{
			name:      "same start and end",
			path:      "file.go:L7-L7",
			wantFile:  "file.go",
			wantStart: 7,
			wantEnd:   7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if pos == nil {
				t.Fatal("expected position, got nil")
			}
			if pos.StartLine != tt.wantStart {
				t.Errorf("StartLine = %d, want %d", pos.StartLine, tt.wantStart)
			}
			if pos.EndLine != tt.wantEnd {
				t.Errorf("EndLine = %d, want %d", pos.EndLine, tt.wantEnd)
			}
			if pos.HasColumns() {
				t.Error("expected HasColumns() to be false")
			}
		})
	}
}

func TestParsePathPosition_LineToEOF(t *testing.T) {
	file, pos, err := ParsePathPosition("src/config.go:L10-EOF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "src/config.go" {
		t.Errorf("file = %q, want %q", file, "src/config.go")
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if pos.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", pos.StartLine)
	}
	if !pos.ToEOF {
		t.Error("expected ToEOF to be true")
	}
	if pos.IsSingleLine() {
		t.Error("expected IsSingleLine() to be false for EOF range")
	}
}

func TestParsePathPosition_ColumnPrecise(t *testing.T) {
	file, pos, err := ParsePathPosition("src/api.rs:L5C20:L5C45")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "src/api.rs" {
		t.Errorf("file = %q, want %q", file, "src/api.rs")
	}
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if pos.StartLine != 5 || pos.EndLine != 5 {
		t.Errorf("lines = %d-%d, want 5-5", pos.StartLine, pos.EndLine)
	}
	if pos.StartCol != 20 || pos.EndCol != 45 {
		t.Errorf("cols = %d-%d, want 20-45", pos.StartCol, pos.EndCol)
	}
	if !pos.HasColumns() {
		t.Error("expected HasColumns() to be true")
	}
}

func TestParsePathPosition_ColumnMultiLine(t *testing.T) {
	file, pos, err := ParsePathPosition("schema.json:L8C3:L15C20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "schema.json" {
		t.Errorf("file = %q, want %q", file, "schema.json")
	}
	if pos.StartLine != 8 || pos.EndLine != 15 {
		t.Errorf("lines = %d-%d, want 8-15", pos.StartLine, pos.EndLine)
	}
	if pos.StartCol != 3 || pos.EndCol != 20 {
		t.Errorf("cols = %d-%d, want 3-20", pos.StartCol, pos.EndCol)
	}
}

// ============================================================================
// Error Cases
// ============================================================================

func TestParsePathPosition_Errors(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "empty file path", path: ":L5"},
		{name: "invalid format", path: "file.go:L5XYZ"},
		{name: "zero line", path: "file.go:L0"},
		{name: "end before start", path: "file.go:L20-L5"},
		{name: "zero start col", path: "file.go:L5C0:L5C10"},
		{name: "zero end col", path: "file.go:L5C1:L5C0"},
		{name: "end col before start col same line", path: "file.go:L5C30:L5C10"},
		{name: "end line before start line with cols", path: "file.go:L10C1:L5C10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParsePathPosition(tt.path)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// ============================================================================
// PositionSpec Method Tests
// ============================================================================

func TestPositionSpec_IsSingleLine(t *testing.T) {
	tests := []struct {
		name string
		pos  PositionSpec
		want bool
	}{
		{name: "single line (EndLine=0)", pos: PositionSpec{StartLine: 5}, want: true},
		{name: "same start and end", pos: PositionSpec{StartLine: 5, EndLine: 5}, want: true},
		{name: "range", pos: PositionSpec{StartLine: 5, EndLine: 10}, want: false},
		{name: "to EOF", pos: PositionSpec{StartLine: 5, ToEOF: true}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pos.IsSingleLine(); got != tt.want {
				t.Errorf("IsSingleLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionSpec_HasColumns(t *testing.T) {
	tests := []struct {
		name string
		pos  PositionSpec
		want bool
	}{
		{name: "no columns", pos: PositionSpec{StartLine: 5}, want: false},
		{name: "with columns", pos: PositionSpec{StartLine: 5, StartCol: 1, EndLine: 5, EndCol: 10}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pos.HasColumns(); got != tt.want {
				t.Errorf("HasColumns() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestParsePathPosition_PathWithMultipleColons(t *testing.T) {
	// Paths that contain colons should work — we split on the last ":L"
	file, pos, err := ParsePathPosition("C:/Users/dev/file.go:L10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "C:/Users/dev/file.go" {
		t.Errorf("file = %q, want %q", file, "C:/Users/dev/file.go")
	}
	if pos == nil || pos.StartLine != 10 {
		t.Errorf("expected position L10, got %+v", pos)
	}
}

func TestParsePathPosition_LargeLineNumbers(t *testing.T) {
	file, pos, err := ParsePathPosition("big.txt:L99999-L100000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "big.txt" {
		t.Errorf("file = %q, want %q", file, "big.txt")
	}
	if pos.StartLine != 99999 || pos.EndLine != 100000 {
		t.Errorf("lines = %d-%d, want 99999-100000", pos.StartLine, pos.EndLine)
	}
}

// ============================================================================
// PositionLock YAML Round-Trip
// ============================================================================

func TestPositionLock_YAMLRoundTrip(t *testing.T) {
	lock := LockDetails{
		Name:       "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Updated:    "2025-01-01T00:00:00Z",
		Positions: []PositionLock{
			{
				From:       "src/api.go:L5-L20",
				To:         "lib/api.go",
				SourceHash: "sha256:deadbeef",
			},
			{
				From:       "src/types.go:L1C5:L1C30",
				To:         "lib/types.go:L10-L10",
				SourceHash: "sha256:cafebabe",
			},
		},
	}

	// Marshal
	data, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Unmarshal
	var got LockDetails
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(got.Positions) != 2 {
		t.Fatalf("positions count = %d, want 2", len(got.Positions))
	}
	if got.Positions[0].From != "src/api.go:L5-L20" {
		t.Errorf("positions[0].From = %q, want %q", got.Positions[0].From, "src/api.go:L5-L20")
	}
	if got.Positions[0].SourceHash != "sha256:deadbeef" {
		t.Errorf("positions[0].SourceHash = %q, want %q", got.Positions[0].SourceHash, "sha256:deadbeef")
	}
	if got.Positions[1].To != "lib/types.go:L10-L10" {
		t.Errorf("positions[1].To = %q, want %q", got.Positions[1].To, "lib/types.go:L10-L10")
	}
}

// ============================================================================
// Windows Path Tests
// ============================================================================

func TestParsePathPosition_WindowsDriveLetter(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantFile  string
		wantStart int
	}{
		{
			name:      "forward slash Windows path with position",
			path:      "C:/Users/dev/file.go:L5",
			wantFile:  "C:/Users/dev/file.go",
			wantStart: 5,
		},
		{
			name:      "backslash Windows path with position",
			path:      `C:\Users\dev\file.go:L10`,
			wantFile:  `C:\Users\dev\file.go`,
			wantStart: 10,
		},
		{
			name:     "Windows path without position",
			path:     `C:\Users\dev\file.go`,
			wantFile: `C:\Users\dev\file.go`,
		},
		{
			name:     "drive letter colon not confused with position",
			path:     `D:\src\main.rs`,
			wantFile: `D:\src\main.rs`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if tt.wantStart > 0 {
				if pos == nil {
					t.Fatalf("expected position, got nil")
				}
				if pos.StartLine != tt.wantStart {
					t.Errorf("StartLine = %d, want %d", pos.StartLine, tt.wantStart)
				}
			} else {
				if pos != nil {
					t.Errorf("expected no position, got %+v", pos)
				}
			}
		})
	}
}

// ============================================================================
// Non-Position Paths (look like positions but aren't)
// ============================================================================

func TestParsePathPosition_NonPositionColonPaths(t *testing.T) {
	// These paths contain colons but do NOT match the ":L<digit>" pattern,
	// so they should be returned as-is with nil position.
	tests := []struct {
		name string
		path string
	}{
		{name: "colon L no digit", path: "file.go:L"},
		{name: "colon L dash number", path: "file.go:L-1"},
		{name: "colon L letters", path: "file.go:LABC"},
		{name: "colon lowercase l", path: "file.go:l5"},
		{name: "colon no L", path: "file.go:5"},
		{name: "multiple colons no position", path: "host:port:path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tt.path {
				t.Errorf("file = %q, want %q (full path returned unchanged)", file, tt.path)
			}
			if pos != nil {
				t.Errorf("expected nil position for non-position path, got %+v", pos)
			}
		})
	}
}

// ============================================================================
// Additional Error Cases
// ============================================================================

func TestParsePathPosition_ZeroLineErrors(t *testing.T) {
	// Line 0 is invalid (1-indexed) across all position formats
	tests := []struct {
		name string
		path string
	}{
		{name: "single line zero", path: "file.go:L0"},
		{name: "line range zero start", path: "file.go:L0-L5"},
		{name: "EOF range zero start", path: "file.go:L0-EOF"},
		{name: "column range zero start line", path: "file.go:L0C1:L5C10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParsePathPosition(tt.path)
			if err == nil {
				t.Error("expected error for line 0, got nil")
			}
		})
	}
}

// ============================================================================
// Property-Based Tests
// ============================================================================

// formatPositionSpec converts a PositionSpec back to a string representation
// suitable for re-parsing. Used for round-trip property tests.
func formatPositionSpec(filePath string, pos *PositionSpec) string {
	if pos == nil {
		return filePath
	}
	if pos.HasColumns() {
		return fmt.Sprintf("%s:L%dC%d:L%dC%d", filePath, pos.StartLine, pos.StartCol, pos.EndLine, pos.EndCol)
	}
	if pos.ToEOF {
		return fmt.Sprintf("%s:L%d-EOF", filePath, pos.StartLine)
	}
	if pos.EndLine > 0 && pos.EndLine != pos.StartLine {
		return fmt.Sprintf("%s:L%d-L%d", filePath, pos.StartLine, pos.EndLine)
	}
	return fmt.Sprintf("%s:L%d", filePath, pos.StartLine)
}

func TestPositionSpec_PropertyRoundTrip(t *testing.T) {
	// Any valid PositionSpec formatted to string and re-parsed should yield
	// an equivalent PositionSpec.
	testCases := []struct {
		name string
		file string
		pos  *PositionSpec
	}{
		{
			name: "nil position",
			file: "src/file.go",
			pos:  nil,
		},
		{
			name: "single line",
			file: "path/to/file.rs",
			pos:  &PositionSpec{StartLine: 42},
		},
		{
			name: "line range",
			file: "api/handler.go",
			pos:  &PositionSpec{StartLine: 10, EndLine: 50},
		},
		{
			name: "to EOF",
			file: "config.yaml",
			pos:  &PositionSpec{StartLine: 100, ToEOF: true},
		},
		{
			name: "column range same line",
			file: "types.ts",
			pos:  &PositionSpec{StartLine: 5, EndLine: 5, StartCol: 10, EndCol: 30},
		},
		{
			name: "column range multi line",
			file: "schema.json",
			pos:  &PositionSpec{StartLine: 8, EndLine: 15, StartCol: 3, EndCol: 20},
		},
		{
			name: "large line numbers",
			file: "big.log",
			pos:  &PositionSpec{StartLine: 99999, EndLine: 100000},
		},
		{
			name: "line 1",
			file: "first.go",
			pos:  &PositionSpec{StartLine: 1},
		},
		{
			name: "line 1 to EOF",
			file: "whole.go",
			pos:  &PositionSpec{StartLine: 1, ToEOF: true},
		},
		{
			name: "column range line 1",
			file: "start.go",
			pos:  &PositionSpec{StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 1},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			formatted := formatPositionSpec(tt.file, tt.pos)

			parsedFile, parsedPos, err := ParsePathPosition(formatted)
			if err != nil {
				t.Fatalf("re-parse failed for %q: %v", formatted, err)
			}
			if parsedFile != tt.file {
				t.Errorf("file path mismatch: got %q, want %q", parsedFile, tt.file)
			}

			if tt.pos == nil {
				if parsedPos != nil {
					t.Errorf("expected nil position, got %+v", parsedPos)
				}
				return
			}
			if parsedPos == nil {
				t.Fatalf("expected position, got nil")
			}

			// Compare relevant fields
			if parsedPos.StartLine != tt.pos.StartLine {
				t.Errorf("StartLine: got %d, want %d", parsedPos.StartLine, tt.pos.StartLine)
			}
			if parsedPos.ToEOF != tt.pos.ToEOF {
				t.Errorf("ToEOF: got %v, want %v", parsedPos.ToEOF, tt.pos.ToEOF)
			}
			if parsedPos.StartCol != tt.pos.StartCol {
				t.Errorf("StartCol: got %d, want %d", parsedPos.StartCol, tt.pos.StartCol)
			}
			if parsedPos.EndCol != tt.pos.EndCol {
				t.Errorf("EndCol: got %d, want %d", parsedPos.EndCol, tt.pos.EndCol)
			}

			// EndLine comparison: parser returns 0 for single line, but we
			// may have set EndLine = StartLine for single-line specs
			wantEndLine := tt.pos.EndLine
			if parsedPos.EndLine != wantEndLine {
				t.Errorf("EndLine: got %d, want %d", parsedPos.EndLine, wantEndLine)
			}
		})
	}
}

// ============================================================================
// PositionLock YAML Round-Trip (continued)
// ============================================================================

func TestPositionLock_OmittedWhenEmpty(t *testing.T) {
	lock := LockDetails{
		Name:       "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Updated:    "2025-01-01T00:00:00Z",
	}

	data, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Should not contain "positions" key when empty
	if strings.Contains(string(data), "positions") {
		t.Errorf("YAML should omit positions when empty, got:\n%s", string(data))
	}
}

// ============================================================================
// Edge Case Tests — Malformed Specs and Boundary Conditions
// ============================================================================

func TestParsePathPosition_MalformedSpecs(t *testing.T) {
	// Specs that almost match a valid pattern but should fail or return no position.
	tests := []struct {
		name      string
		path      string
		wantErr   bool
		wantNoPos bool // true = expect no position (path returned as-is)
	}{
		// Malformed: digit after L but incomplete range syntax
		{name: "L5-L (no end digit)", path: "file.go:L5-L", wantErr: true},
		// Malformed: column specifier missing digit after C
		{name: "L5C:L10C3 (C with no start col)", path: "file.go:L5C:L10C3", wantErr: true},
		// Malformed: trailing garbage after valid single line
		{name: "L5XYZ (trailing garbage)", path: "file.go:L5XYZ", wantErr: true},
		// Malformed: EOF but with extra text
		{name: "L5-EOFX (extra after EOF)", path: "file.go:L5-EOFX", wantErr: true},
		// Malformed: dash without second L
		{name: "L5-10 (dash without L prefix)", path: "file.go:L5-10", wantErr: true},
		// Malformed: colon range without second L
		{name: "L5:10 (colon without L prefix)", path: "file.go:L5:10", wantErr: true},
		// No position: colon followed by L but no digit
		{name: ":L no digit", path: "file.go:L", wantNoPos: true},
		// No position: colon followed by lowercase l + digit
		{name: "lowercase l5", path: "file.go:l5", wantNoPos: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got file=%q pos=%+v", file, pos)
				}
				return
			}
			if tt.wantNoPos {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if pos != nil {
					t.Errorf("expected nil position, got %+v", pos)
				}
				if file != tt.path {
					t.Errorf("file = %q, want %q (full path unchanged)", file, tt.path)
				}
				return
			}
		})
	}
}

func TestParsePathPosition_DoubleColonPaths(t *testing.T) {
	// Double-colon in path: the first ":L<digit>" wins
	tests := []struct {
		name      string
		path      string
		wantFile  string
		wantStart int
		wantNoPos bool
	}{
		{
			name:      "double colon then position",
			path:      "host::path:L5",
			wantFile:  "host::path",
			wantStart: 5,
		},
		{
			name:      "adjacent colon before L",
			path:      "file.go::L10",
			wantFile:  "file.go:",
			wantStart: 10,
		},
		{
			name:      "double colon no position",
			path:      "host::port::data",
			wantNoPos: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNoPos {
				if pos != nil {
					t.Errorf("expected no position, got %+v", pos)
				}
				return
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if pos == nil {
				t.Fatalf("expected position, got nil")
			}
			if pos.StartLine != tt.wantStart {
				t.Errorf("StartLine = %d, want %d", pos.StartLine, tt.wantStart)
			}
		})
	}
}

func TestParsePathPosition_OverflowLineNumbers(t *testing.T) {
	// Numbers that overflow int should produce a strconv error, not a panic
	_, _, err := ParsePathPosition("file.go:L99999999999999999999")
	if err == nil {
		t.Error("expected error for overflow line number, got nil")
	}

	_, _, err = ParsePathPosition("file.go:L99999999999999999999-L99999999999999999999")
	if err == nil {
		t.Error("expected error for overflow line range, got nil")
	}

	_, _, err = ParsePathPosition("file.go:L99999999999999999999-EOF")
	if err == nil {
		t.Error("expected error for overflow EOF range, got nil")
	}

	_, _, err = ParsePathPosition("file.go:L99999999999999999999C1:L99999999999999999999C1")
	if err == nil {
		t.Error("expected error for overflow column range, got nil")
	}
}

func TestParsePathPosition_ShortStrings(t *testing.T) {
	// Test strings near the boundary of findPositionStart's i < len(path)-2 guard.
	tests := []struct {
		name      string
		path      string
		wantFile  string
		wantNoPos bool
		wantErr   bool
	}{
		{name: "len 0 (empty)", path: "", wantFile: "", wantNoPos: true},
		{name: "len 1", path: "x", wantFile: "x", wantNoPos: true},
		{name: "len 2 :L", path: ":L", wantFile: ":L", wantNoPos: true},     // no digit after L
		{name: "len 3 :L1", path: ":L1", wantErr: true},                      // empty file path
		{name: "len 4 a:L1", path: "a:L1", wantFile: "a"},                    // minimal valid
		{name: "len 3 L5X", path: "L5X", wantFile: "L5X", wantNoPos: true},   // no colon
		{name: "len 2 :5", path: ":5", wantFile: ":5", wantNoPos: true},      // no L
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNoPos {
				if pos != nil {
					t.Errorf("expected nil position, got %+v", pos)
				}
				if file != tt.wantFile {
					t.Errorf("file = %q, want %q", file, tt.wantFile)
				}
				return
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if pos == nil {
				t.Error("expected position, got nil")
			}
		})
	}
}

func TestParsePathPosition_ColumnZeroValueFields(t *testing.T) {
	// Zero-value column fields should be rejected
	tests := []struct {
		name string
		path string
	}{
		{name: "zero start col", path: "file.go:L1C0:L1C5"},
		{name: "zero end col", path: "file.go:L1C1:L1C0"},
		{name: "zero both cols", path: "file.go:L1C0:L1C0"},
		{name: "zero start line with cols", path: "file.go:L0C1:L5C10"},
		{name: "end line before start with cols", path: "file.go:L10C1:L5C10"},
		{name: "end col before start col same line", path: "file.go:L5C20:L5C10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParsePathPosition(tt.path)
			if err == nil {
				t.Error("expected error for invalid column spec, got nil")
			}
		})
	}
}

func TestParsePathPosition_WindowsDoubleColonDriveLetter(t *testing.T) {
	// Windows UNC and paths with multiple colons combined with position specs
	tests := []struct {
		name      string
		path      string
		wantFile  string
		wantStart int
		wantNoPos bool
	}{
		{
			name:      "UNC-like path with position",
			path:      `\\server\share\file.go:L15`,
			wantFile:  `\\server\share\file.go`,
			wantStart: 15,
		},
		{
			name:      "double backslash no position",
			path:      `\\server\share\file.go`,
			wantFile:  `\\server\share\file.go`,
			wantNoPos: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, pos, err := ParsePathPosition(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNoPos {
				if pos != nil {
					t.Errorf("expected nil position, got %+v", pos)
				}
				return
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if pos == nil {
				t.Fatalf("expected position, got nil")
			}
			if pos.StartLine != tt.wantStart {
				t.Errorf("StartLine = %d, want %d", pos.StartLine, tt.wantStart)
			}
		})
	}
}
