package types

import (
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
