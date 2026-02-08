package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// ExtractPosition Tests
// ============================================================================

func TestExtractPosition_SingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, hash, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "line3" {
		t.Errorf("extracted = %q, want %q", extracted, "line3")
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("hash should start with sha256:, got %q", hash)
	}
}

func TestExtractPosition_LineRange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 2, EndLine: 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "line2\nline3\nline4" {
		t.Errorf("extracted = %q, want %q", extracted, "line2\nline3\nline4")
	}
}

func TestExtractPosition_ToEOF(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 3, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "line3\nline4\nline5" {
		t.Errorf("extracted = %q, want %q", extracted, "line3\nline4\nline5")
	}
}

func TestExtractPosition_ColumnPrecise_SingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "0123456789abcdef\nother line\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// EndCol is 1-indexed inclusive bound: L1C5:L1C10 extracts cols 5-10 (6 chars)
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "456789" {
		t.Errorf("extracted = %q, want %q", extracted, "456789")
	}
}

func TestExtractPosition_ColumnPrecise_MultiLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "AAAAABBBBB\nCCCCCDDDDD\nEEEEEFFFFF\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 3, StartCol: 6, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First line from col 6: "BBBBB"
	// Middle line (full): "CCCCCDDDDDD"
	// Last line to col 5: "EEEEE"
	if extracted != "BBBBB\nCCCCCDDDDD\nEEEEE" {
		t.Errorf("extracted = %q, want %q", extracted, "BBBBB\nCCCCCDDDDD\nEEEEE")
	}
}

// ============================================================================
// ExtractPosition Error Cases
// ============================================================================

func TestExtractPosition_LineOutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 10})
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want message about line not existing", err.Error())
	}
}

func TestExtractPosition_EndLineOutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1, EndLine: 100})
	if err == nil {
		t.Fatal("expected error for out-of-range end line")
	}
}

func TestExtractPosition_ColumnExceedsLineLength(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "short\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 100,
	})
	if err == nil {
		t.Fatal("expected error for column exceeding line length")
	}
	if !strings.Contains(err.Error(), "exceeds line length") {
		t.Errorf("error = %q, want column exceeds message", err.Error())
	}
}

func TestExtractPosition_FileNotFound(t *testing.T) {
	_, _, err := ExtractPosition("/nonexistent/file.go", &types.PositionSpec{StartLine: 1})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// ============================================================================
// PlaceContent Tests
// ============================================================================

func TestPlaceContent_ReplaceEntireFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := PlaceContent(filePath, "new content", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "new content" {
		t.Errorf("file content = %q, want %q", string(got), "new content")
	}
}

func TestPlaceContent_ReplaceLineRange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 2, EndLine: 4}
	if err := PlaceContent(filePath, "replaced2\nreplaced3", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "line1\nreplaced2\nreplaced3\nline5\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestPlaceContent_ReplaceSingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 2}
	if err := PlaceContent(filePath, "REPLACED", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "line1\nREPLACED\nline3\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestPlaceContent_ReplaceToEOF(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "keep1\nkeep2\nreplace3\nreplace4\nreplace5"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 3, ToEOF: true}
	if err := PlaceContent(filePath, "new3\nnew4", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "keep1\nkeep2\nnew3\nnew4"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestPlaceContent_ColumnReplace_SingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	// "Hello World!" (12 chars)
	content := "Hello World!\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Replace columns 7-11 ("World"), EndCol is exclusive boundary
	// line[:6] + "Go" + line[11:] = "Hello " + "Go" + "!"
	pos := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 7, EndCol: 11}
	if err := PlaceContent(filePath, "Go", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "Hello Go!\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestPlaceContent_CreateNewFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "new.go")

	if err := PlaceContent(filePath, "brand new content", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "brand new content" {
		t.Errorf("file content = %q, want %q", string(got), "brand new content")
	}
}

// ============================================================================
// PlaceContent Error Cases
// ============================================================================

func TestPlaceContent_TargetLineOutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "line1\nline2\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 100}
	err := PlaceContent(filePath, "content", pos)
	if err == nil {
		t.Fatal("expected error for out-of-range target line")
	}
}

// ============================================================================
// Round-Trip: Extract then Place
// ============================================================================

func TestExtractThenPlace_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.go")
	srcContent := "package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract lines 3-5 (the Add function)
	extracted, hash, err := ExtractPosition(srcPath, &types.PositionSpec{StartLine: 3, EndLine: 5})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if !strings.Contains(extracted, "func Add") {
		t.Errorf("extracted should contain func Add, got: %q", extracted)
	}

	// Place into target file
	dstPath := filepath.Join(tempDir, "dest.go")
	if err := PlaceContent(dstPath, extracted, nil); err != nil {
		t.Fatalf("place error: %v", err)
	}

	// Re-extract from dest should produce identical content
	reExtracted, reHash, err := ExtractPosition(dstPath, &types.PositionSpec{StartLine: 1, EndLine: 3})
	if err != nil {
		t.Fatalf("re-extract error: %v", err)
	}
	if reExtracted != extracted {
		t.Errorf("round-trip mismatch:\noriginal: %q\nre-extracted: %q", extracted, reExtracted)
	}
	if reHash != hash {
		t.Errorf("hash mismatch: original %q, re-extracted %q", hash, reHash)
	}
}

func TestExtractThenPlace_IntoExistingFile(t *testing.T) {
	tempDir := t.TempDir()

	// Source: extract lines 2-3
	srcPath := filepath.Join(tempDir, "source.go")
	if err := os.WriteFile(srcPath, []byte("header\nfoo\nbar\nfooter\n"), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(srcPath, &types.PositionSpec{StartLine: 2, EndLine: 3})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}

	// Target: replace lines 2-3
	dstPath := filepath.Join(tempDir, "dest.go")
	if err := os.WriteFile(dstPath, []byte("keep1\nold2\nold3\nkeep4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := PlaceContent(dstPath, extracted, &types.PositionSpec{StartLine: 2, EndLine: 3}); err != nil {
		t.Fatalf("place error: %v", err)
	}

	got, _ := os.ReadFile(dstPath)
	want := "keep1\nfoo\nbar\nkeep4\n"
	if string(got) != want {
		t.Errorf("result = %q, want %q", string(got), want)
	}
}

// ============================================================================
// CopyStats Position Tracking
// ============================================================================

func TestCopyStats_PositionsAggregation(t *testing.T) {
	s1 := CopyStats{
		FileCount: 1,
		ByteCount: 100,
		Positions: []positionRecord{
			{From: "a.go:L1-L5", To: "b.go", SourceHash: "sha256:aaa"},
		},
	}
	s2 := CopyStats{
		FileCount: 2,
		ByteCount: 200,
		Positions: []positionRecord{
			{From: "c.go:L10-L20", To: "d.go:L5-L15", SourceHash: "sha256:bbb"},
		},
	}

	s1.Add(s2)

	if s1.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", s1.FileCount)
	}
	if s1.ByteCount != 300 {
		t.Errorf("ByteCount = %d, want 300", s1.ByteCount)
	}
	if len(s1.Positions) != 2 {
		t.Fatalf("Positions length = %d, want 2", len(s1.Positions))
	}
	if s1.Positions[0].SourceHash != "sha256:aaa" {
		t.Errorf("Positions[0].SourceHash = %q, want sha256:aaa", s1.Positions[0].SourceHash)
	}
	if s1.Positions[1].From != "c.go:L10-L20" {
		t.Errorf("Positions[1].From = %q, want c.go:L10-L20", s1.Positions[1].From)
	}
}

func TestToPositionLocks(t *testing.T) {
	// Empty input returns nil
	result := toPositionLocks(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}

	result = toPositionLocks([]positionRecord{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}

	// Non-empty input
	records := []positionRecord{
		{From: "src/api.go:L5-L20", To: "lib/api.go", SourceHash: "sha256:abc123"},
		{From: "src/types.go:L1C5:L1C30", To: "lib/types.go:L10-L10", SourceHash: "sha256:def456"},
	}
	result = toPositionLocks(records)
	if len(result) != 2 {
		t.Fatalf("expected 2 locks, got %d", len(result))
	}
	if result[0].From != "src/api.go:L5-L20" || result[0].SourceHash != "sha256:abc123" {
		t.Errorf("lock[0] = %+v, unexpected", result[0])
	}
	if result[1].To != "lib/types.go:L10-L10" || result[1].SourceHash != "sha256:def456" {
		t.Errorf("lock[1] = %+v, unexpected", result[1])
	}
}
