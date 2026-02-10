package core

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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

// ============================================================================
// Sync-Time Local Modification Detection
// ============================================================================

func TestCheckLocalModifications_NoExistingFile(t *testing.T) {
	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	// File doesn't exist — no warning
	w := svc.checkLocalModifications("/nonexistent/file.go", nil, "new content")
	if w != "" {
		t.Errorf("expected no warning for missing file, got %q", w)
	}
}

func TestCheckLocalModifications_WholeFile_NoChange(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "dest.go")
	content := "same content"
	if err := os.WriteFile(destFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	w := svc.checkLocalModifications(destFile, nil, content)
	if w != "" {
		t.Errorf("expected no warning when content is identical, got %q", w)
	}
}

func TestCheckLocalModifications_WholeFile_Modified(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "dest.go")
	if err := os.WriteFile(destFile, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	w := svc.checkLocalModifications(destFile, nil, "new content from source")
	if w == "" {
		t.Error("expected warning when content differs")
	}
	if !strings.Contains(w, "local modifications") {
		t.Errorf("warning should mention 'local modifications', got %q", w)
	}
}

func TestCheckLocalModifications_Position_NoChange(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "dest.go")
	if err := os.WriteFile(destFile, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	pos := &types.PositionSpec{StartLine: 2}
	w := svc.checkLocalModifications(destFile, pos, "line2")
	if w != "" {
		t.Errorf("expected no warning when position content matches, got %q", w)
	}
}

func TestCheckLocalModifications_Position_Modified(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "dest.go")
	if err := os.WriteFile(destFile, []byte("line1\nmodified-line2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	pos := &types.PositionSpec{StartLine: 2}
	w := svc.checkLocalModifications(destFile, pos, "original-line2")
	if w == "" {
		t.Error("expected warning when position content differs")
	}
	if !strings.Contains(w, "target position") {
		t.Errorf("warning should mention 'target position', got %q", w)
	}
}

func TestCheckLocalModifications_WholeFile_CRLF_NoSpuriousWarning(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "dest.go")
	// Destination file has CRLF line endings
	if err := os.WriteFile(destFile, []byte("same\r\ncontent\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	// Incoming content is LF-normalized (as produced by ExtractPosition)
	w := svc.checkLocalModifications(destFile, nil, "same\ncontent\n")
	if w != "" {
		t.Errorf("expected no warning when content differs only in line endings, got %q", w)
	}
}

// TestCheckLocalModifications_Position_InvalidRange_SuppressesWarning verifies that
// checkLocalModifications silently returns no warning when the position range is
// invalid on the existing destination file (e.g., file has 2 lines but pos targets L5).
// This is intentional: the file may have been created/modified since last sync,
// and an invalid range means "no comparison possible", not "modification detected".
func TestCheckLocalModifications_Position_InvalidRange_SuppressesWarning(t *testing.T) {
	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "short.go")
	if err := os.WriteFile(destFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &OSFileSystem{}
	svc := &FileCopyService{fs: fs}

	// Position L5 doesn't exist in 3-line file — ExtractPosition errors, warning suppressed
	pos := &types.PositionSpec{StartLine: 5}
	w := svc.checkLocalModifications(destFile, pos, "some content")
	if w != "" {
		t.Errorf("expected no warning for invalid range on existing file, got %q", w)
	}
}

func TestCopyStats_WarningsAggregation(t *testing.T) {
	s1 := CopyStats{Warnings: []string{"warn1"}}
	s2 := CopyStats{Warnings: []string{"warn2", "warn3"}}
	s1.Add(s2)
	if len(s1.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(s1.Warnings))
	}
}

// ============================================================================
// Edge Case 1: Unicode / Multi-byte Characters
// Column semantics are byte-offset based (Go string indexing), not rune-based.
// ============================================================================

func TestExtractPosition_Unicode_LineExtraction(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "unicode.txt")
	// Line contains emoji (4 bytes), CJK (3 bytes each), accented (2 bytes)
	content := "Hello 🌍!\n你好世界\ncafé\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Whole-line extraction works correctly regardless of encoding
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "Hello 🌍!" {
		t.Errorf("line 1: extracted = %q, want %q", extracted, "Hello 🌍!")
	}

	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{StartLine: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "你好世界" {
		t.Errorf("line 2: extracted = %q, want %q", extracted, "你好世界")
	}

	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{StartLine: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "café" {
		t.Errorf("line 3: extracted = %q, want %q", extracted, "café")
	}
}

func TestExtractPosition_Unicode_ColumnByteSemantics(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "unicode.txt")
	// "Hello 🌍!" in UTF-8 bytes:
	// H(1) e(2) l(3) l(4) o(5) ' '(6) 🌍(7-10) !(11)
	content := "Hello 🌍!\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract ASCII portion: bytes 1-5 = "Hello"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "Hello" {
		t.Errorf("ASCII cols: extracted = %q, want %q", extracted, "Hello")
	}

	// Extract full emoji: bytes 7-10 = 🌍 (4 UTF-8 bytes)
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 7, EndCol: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "🌍" {
		t.Errorf("emoji cols: extracted = %q, want %q", extracted, "🌍")
	}
}

func TestExtractPosition_Unicode_CJK_ByteColumns(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "cjk.txt")
	// "你好world" — each CJK char is 3 UTF-8 bytes
	// 你(1-3) 好(4-6) w(7) o(8) r(9) l(10) d(11)
	content := "你好world\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract first CJK char: bytes 1-3 = "你"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "你" {
		t.Errorf("CJK single: extracted = %q, want %q", extracted, "你")
	}

	// Extract both CJK chars: bytes 1-6 = "你好"
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 6,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "你好" {
		t.Errorf("CJK pair: extracted = %q, want %q", extracted, "你好")
	}

	// Extract "world": bytes 7-11
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 7, EndCol: 11,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "world" {
		t.Errorf("ASCII after CJK: extracted = %q, want %q", extracted, "world")
	}
}

func TestExtractPosition_Unicode_AccentedChars(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accent.txt")
	// "café" — é is 2 UTF-8 bytes (0xC3 0xA9)
	// c(1) a(2) f(3) é(4-5)
	content := "café\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract "caf": bytes 1-3
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "caf" {
		t.Errorf("ASCII portion: extracted = %q, want %q", extracted, "caf")
	}

	// Extract full "café": bytes 1-5
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "café" {
		t.Errorf("full accented: extracted = %q, want %q", extracted, "café")
	}
}

// ============================================================================
// Edge Case 2: Windows CRLF Line Endings
// CRLF (\r\n) is normalized to LF (\n) before extraction and placement.
// ============================================================================

func TestExtractPosition_CRLF_LineExtraction(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "crlf.txt")
	// Write file with CRLF line endings
	content := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Single line extraction should NOT include \r
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "line1" {
		t.Errorf("single line: extracted = %q, want %q", extracted, "line1")
	}
	if strings.Contains(extracted, "\r") {
		t.Error("extracted content contains \\r — CRLF not normalized")
	}

	// Line range extraction should use LF joins
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{StartLine: 1, EndLine: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "line1\nline2\nline3" {
		t.Errorf("line range: extracted = %q, want %q", extracted, "line1\nline2\nline3")
	}
	if strings.Contains(extracted, "\r") {
		t.Error("extracted range contains \\r — CRLF not normalized")
	}
}

func TestExtractPosition_CRLF_ColumnExtraction(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "crlf.txt")
	content := "Hello World!\r\nSecond line\r\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Column extraction on CRLF file — \r should not affect column offsets
	// After CRLF normalization, line is "Hello World!" (12 bytes)
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 7, EndCol: 11,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "World" {
		t.Errorf("CRLF column: extracted = %q, want %q", extracted, "World")
	}
}

func TestExtractPosition_CRLF_DeterministicHash(t *testing.T) {
	tempDir := t.TempDir()

	// Same content with LF and CRLF endings should produce identical hashes
	lfPath := filepath.Join(tempDir, "lf.txt")
	crlfPath := filepath.Join(tempDir, "crlf.txt")

	if err := os.WriteFile(lfPath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(crlfPath, []byte("line1\r\nline2\r\nline3\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 1, EndLine: 3}
	_, lfHash, err := ExtractPosition(lfPath, pos)
	if err != nil {
		t.Fatalf("LF extraction error: %v", err)
	}
	_, crlfHash, err := ExtractPosition(crlfPath, pos)
	if err != nil {
		t.Fatalf("CRLF extraction error: %v", err)
	}

	if lfHash != crlfHash {
		t.Errorf("hash mismatch: LF=%q, CRLF=%q — CRLF normalization produces different hash", lfHash, crlfHash)
	}
}

func TestPlaceContent_CRLF_NormalizesExistingContent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "crlf.txt")

	// Existing file with CRLF
	if err := os.WriteFile(filePath, []byte("keep1\r\nold2\r\nkeep3\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 2}
	if err := PlaceContent(filePath, "replaced2", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "keep1\nreplaced2\nkeep3\n"
	if string(got) != want {
		t.Errorf("after placement: got %q, want %q", string(got), want)
	}
	if strings.Contains(string(got), "\r") {
		t.Error("output contains \\r — CRLF should be normalized to LF in output")
	}
}

// ============================================================================
// Edge Case 3: Trailing Newline Behavior
// strings.Split("a\nb\n", "\n") = ["a", "b", ""] — empty trailing element
// counts as a line. This affects line counts and EOF behavior.
// ============================================================================

func TestExtractPosition_TrailingNewline_SingleLine(t *testing.T) {
	tempDir := t.TempDir()

	// File WITH trailing newline
	withNL := filepath.Join(tempDir, "with_nl.txt")
	if err := os.WriteFile(withNL, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// File WITHOUT trailing newline
	withoutNL := filepath.Join(tempDir, "without_nl.txt")
	if err := os.WriteFile(withoutNL, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// L1 should extract "hello" in both cases
	for _, tc := range []struct {
		name string
		path string
	}{
		{"with trailing newline", withNL},
		{"without trailing newline", withoutNL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extracted, _, err := ExtractPosition(tc.path, &types.PositionSpec{StartLine: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if extracted != "hello" {
				t.Errorf("extracted = %q, want %q", extracted, "hello")
			}
		})
	}
}

func TestExtractPosition_TrailingNewline_L5EOF_FiveLineFile(t *testing.T) {
	tempDir := t.TempDir()

	// 5-line file WITH trailing newline: split produces 6 elements (last is "")
	withNL := filepath.Join(tempDir, "with_nl.txt")
	if err := os.WriteFile(withNL, []byte("l1\nl2\nl3\nl4\nl5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// L5-EOF on file with trailing newline: extracts line 5 + empty trailing element
	extracted, _, err := ExtractPosition(withNL, &types.PositionSpec{StartLine: 5, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// strings.Split("...l5\n", "\n") -> [..., "l5", ""]
	// lines[4:6] = ["l5", ""], joined = "l5\n"
	if extracted != "l5\n" {
		t.Errorf("with trailing NL: extracted = %q, want %q", extracted, "l5\n")
	}

	// 5-line file WITHOUT trailing newline
	withoutNL := filepath.Join(tempDir, "without_nl.txt")
	if err := os.WriteFile(withoutNL, []byte("l1\nl2\nl3\nl4\nl5"), 0644); err != nil {
		t.Fatal(err)
	}

	// L5-EOF without trailing newline: just line 5
	extracted, _, err = ExtractPosition(withoutNL, &types.PositionSpec{StartLine: 5, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "l5" {
		t.Errorf("without trailing NL: extracted = %q, want %q", extracted, "l5")
	}
}

func TestExtractPosition_TrailingNewline_SingleLineFileL1(t *testing.T) {
	tempDir := t.TempDir()

	withNL := filepath.Join(tempDir, "single_nl.txt")
	if err := os.WriteFile(withNL, []byte("only\n"), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(withNL, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "only" {
		t.Errorf("extracted = %q, want %q", extracted, "only")
	}
}

// ============================================================================
// Edge Case 4: Empty File
// os.ReadFile on 0-byte file → "" → strings.Split("", "\n") = [""] → 1 line.
// ============================================================================

func TestExtractPosition_EmptyFile_L1(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// L1 on empty file: split produces [""] (1 element), line 1 exists
	extracted, hash, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "" {
		t.Errorf("extracted = %q, want empty string", extracted)
	}
	// Hash of empty string is well-known
	emptyHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("")))
	if hash != emptyHash {
		t.Errorf("hash = %q, want %q", hash, emptyHash)
	}
}

func TestExtractPosition_EmptyFile_L2_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// L2 on empty file: only 1 line exists, should error
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 2})
	if err == nil {
		t.Fatal("expected error for L2 on empty file")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want message about line not existing", err.Error())
	}
}

func TestExtractPosition_EmptyFile_L1EOF(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// L1-EOF on empty file: should extract empty string
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "" {
		t.Errorf("extracted = %q, want empty string", extracted)
	}
}

func TestPlaceContent_EmptyFile_WithPosition(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Place content at L1 of empty file
	if err := PlaceContent(filePath, "inserted", &types.PositionSpec{StartLine: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "inserted" {
		t.Errorf("after placement: got %q, want %q", string(got), "inserted")
	}
}

// ============================================================================
// Edge Case 5: Overlapping Positions (Sequential PlaceContent Calls)
// Two vendors writing to different positions in the same destination file.
// ============================================================================

func TestPlaceContent_SequentialNonOverlapping(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// First vendor writes to L2
	if err := PlaceContent(filePath, "vendor-A", &types.PositionSpec{StartLine: 2}); err != nil {
		t.Fatalf("first placement error: %v", err)
	}

	// Second vendor writes to L4 (non-overlapping, same line count)
	if err := PlaceContent(filePath, "vendor-B", &types.PositionSpec{StartLine: 4}); err != nil {
		t.Fatalf("second placement error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "line1\nvendor-A\nline3\nvendor-B\nline5\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

func TestPlaceContent_SequentialLineCountChange(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	content := "line1\nline2\nline3\nline4\nline5\nline6\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// First vendor replaces L2-L3 with a single line (removes 1 line)
	if err := PlaceContent(filePath, "replaced-2-3", &types.PositionSpec{StartLine: 2, EndLine: 3}); err != nil {
		t.Fatalf("first placement error: %v", err)
	}

	// After first placement: "line1\nreplaced-2-3\nline4\nline5\nline6\n" (5 lines + empty)
	// Second vendor writes to L5 — but L5 is now "line6" not "line5"
	// because the first placement shifted lines down.
	if err := PlaceContent(filePath, "vendor-B-at-L5", &types.PositionSpec{StartLine: 5}); err != nil {
		t.Fatalf("second placement error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	// L5 in the modified file is "line6" (shifted by the first placement)
	want := "line1\nreplaced-2-3\nline4\nline5\nvendor-B-at-L5\n"
	if string(got) != want {
		t.Errorf("got %q, want %q\nNote: second placement operates on already-modified content", string(got), want)
	}
}

func TestPlaceContent_SequentialColumnNonOverlapping(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")
	// "Hello World Goodbye" (19 chars)
	content := "Hello World Goodbye\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// First vendor replaces cols 7-11 ("World") with "Earth"
	pos1 := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 7, EndCol: 11}
	if err := PlaceContent(filePath, "Earth", pos1); err != nil {
		t.Fatalf("first placement error: %v", err)
	}

	// Now file is "Hello Earth Goodbye\n"
	// Second vendor replaces cols 13-19 ("Goodbye") with "Friends"
	pos2 := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 13, EndCol: 19}
	if err := PlaceContent(filePath, "Friends", pos2); err != nil {
		t.Fatalf("second placement error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "Hello Earth Friends\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

// ============================================================================
// Edge Case 6: Single Character Column Extraction (StartCol == EndCol)
// L1C5:L1C5 should extract exactly 1 byte via line[4:5].
// ============================================================================

func TestExtractPosition_SingleCharColumn(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "abcdefghij\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C5:L1C5 extracts byte at position 5 = "e"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "e" {
		t.Errorf("single char: extracted = %q, want %q", extracted, "e")
	}

	// First character: L1C1:L1C1
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "a" {
		t.Errorf("first char: extracted = %q, want %q", extracted, "a")
	}

	// Last character: L1C10:L1C10
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 10, EndCol: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "j" {
		t.Errorf("last char: extracted = %q, want %q", extracted, "j")
	}
}

func TestPlaceContent_SingleCharColumn_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	// Extract a single char from source
	srcPath := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("abcdefghij\n"), 0644); err != nil {
		t.Fatal(err)
	}
	extracted, hash, err := ExtractPosition(srcPath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if extracted != "e" {
		t.Fatalf("extracted = %q, want %q", extracted, "e")
	}

	// Place into target at same position
	dstPath := filepath.Join(tempDir, "target.txt")
	if err := os.WriteFile(dstPath, []byte("0123456789\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5}
	if err := PlaceContent(dstPath, extracted, pos); err != nil {
		t.Fatalf("place error: %v", err)
	}

	// Verify the target
	got, _ := os.ReadFile(dstPath)
	// Replaces byte 5 ("4") with "e": "0123e56789\n"
	if string(got) != "0123e56789\n" {
		t.Errorf("after placement: got %q, want %q", string(got), "0123e56789\n")
	}

	// Re-extract and verify hash matches
	reExtracted, reHash, err := ExtractPosition(dstPath, pos)
	if err != nil {
		t.Fatalf("re-extract error: %v", err)
	}
	if reExtracted != extracted {
		t.Errorf("round-trip content mismatch: %q vs %q", reExtracted, extracted)
	}
	if reHash != hash {
		t.Errorf("round-trip hash mismatch: %q vs %q", reHash, hash)
	}
}

// ============================================================================
// Edge Case 7: L1-EOF Spanning Entire File (Hash Equivalence)
// L1-EOF should produce content identical to the raw file, so hashes match.
// ============================================================================

func TestExtractPosition_L1EOF_HashMatchesWholeFile_NoTrailingNewline(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// L1-EOF extraction
	extracted, extractedHash, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, ToEOF: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whole-file hash
	rawData, _ := os.ReadFile(filePath)
	wholeFileHash := fmt.Sprintf("sha256:%x", sha256.Sum256(rawData))

	if extracted != content {
		t.Errorf("L1-EOF extracted = %q, want %q (raw file)", extracted, content)
	}
	if extractedHash != wholeFileHash {
		t.Errorf("hash mismatch:\n  L1-EOF:     %s\n  whole file: %s", extractedHash, wholeFileHash)
	}
}

func TestExtractPosition_L1EOF_HashMatchesWholeFile_WithTrailingNewline(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// L1-EOF extraction
	extracted, extractedHash, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, ToEOF: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whole-file hash
	rawData, _ := os.ReadFile(filePath)
	wholeFileHash := fmt.Sprintf("sha256:%x", sha256.Sum256(rawData))

	if extracted != content {
		t.Errorf("L1-EOF extracted = %q, want raw file content %q", extracted, content)
	}
	if extractedHash != wholeFileHash {
		t.Errorf("hash mismatch:\n  L1-EOF:     %s\n  whole file: %s", extractedHash, wholeFileHash)
	}
}

func TestExtractPosition_L1EOF_SingleLineFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "single.txt")
	content := "only line"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, extractedHash, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, ToEOF: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawData, _ := os.ReadFile(filePath)
	wholeFileHash := fmt.Sprintf("sha256:%x", sha256.Sum256(rawData))

	if extracted != content {
		t.Errorf("extracted = %q, want %q", extracted, content)
	}
	if extractedHash != wholeFileHash {
		t.Errorf("hash mismatch:\n  L1-EOF:     %s\n  whole file: %s", extractedHash, wholeFileHash)
	}
}

// ============================================================================
// CRLF + Trailing Newline Interaction
// ============================================================================

func TestExtractPosition_CRLF_TrailingNewline_L1EOF(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "crlf_trail.txt")
	content := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// L1-EOF on CRLF file: after normalization, content is "line1\nline2\nline3\n"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, ToEOF: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "line1\nline2\nline3\n"
	if extracted != want {
		t.Errorf("extracted = %q, want %q", extracted, want)
	}
	if strings.Contains(extracted, "\r") {
		t.Error("extracted CRLF L1-EOF contains \\r")
	}
}

// ============================================================================
// normalizeCRLF Unit Test
// ============================================================================

func TestNormalizeCRLF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no CRLF", "abc\ndef\n", "abc\ndef\n"},
		{"CRLF only", "abc\r\ndef\r\n", "abc\ndef\n"},
		{"mixed LF and CRLF", "abc\ndef\r\nghi\n", "abc\ndef\nghi\n"},
		{"standalone CR preserved", "abc\rdef\n", "abc\rdef\n"},
		{"CRLF in middle of line", "no\r\nnewline\r\nhere\r\n", "no\nnewline\nhere\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCRLF(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCRLF(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Edge Case 8: Binary File Detection
// Position extraction on binary files is rejected with a clear error.
// Detection uses git's heuristic: null byte in first 8000 bytes.
// ============================================================================

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"plain text", []byte("hello world\n"), false},
		{"text with high bytes", []byte("café 你好 🌍"), false},
		{"null at start", []byte{0x00, 'a', 'b'}, true},
		{"null in middle", []byte("abc\x00def"), true},
		{"null at byte 7999", append(bytes.Repeat([]byte{'x'}, 7999), 0x00), true},
		{"null beyond scan limit", append(append(bytes.Repeat([]byte{'x'}, 8000), 0x00), 'x'), false},
		{"PNG header", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, true},
		{"ELF header", []byte{0x7f, 0x45, 0x4C, 0x46, 0x00}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinaryContent(tt.data)
			if got != tt.want {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestExtractPosition_BinaryFile_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "binary.dat")
	// Binary content with null bytes
	data := []byte("line1\nline2\x00binary\nline3\n")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err == nil {
		t.Fatal("expected error for binary file")
	}
	if !strings.Contains(err.Error(), "binary file") {
		t.Errorf("error = %q, want message about binary file", err.Error())
	}
}

func TestExtractPosition_TextFile_NoFalsePositive(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "text.go")
	// Valid text with various Unicode — no null bytes
	content := "package main\n\n// café 你好 🌍\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 3})
	if err != nil {
		t.Fatalf("false positive: text file rejected as binary: %v", err)
	}
	if extracted != "// café 你好 🌍" {
		t.Errorf("extracted = %q, want %q", extracted, "// café 你好 🌍")
	}
}

func TestPlaceContent_BinaryTarget_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "binary.dat")
	data := []byte("line1\nline2\x00binary\nline3\n")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Position-based placement into binary file should error
	err := PlaceContent(filePath, "replacement", &types.PositionSpec{StartLine: 1})
	if err == nil {
		t.Fatal("expected error for binary target")
	}
	if !strings.Contains(err.Error(), "binary file") {
		t.Errorf("error = %q, want message about binary file", err.Error())
	}
}

func TestPlaceContent_BinaryTarget_NilPos_Allowed(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "output.dat")
	// Whole-file replacement (nil pos) bypasses binary check — writing new content
	if err := PlaceContent(filePath, "new text content", nil); err != nil {
		t.Fatalf("nil-pos placement should succeed regardless: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "new text content" {
		t.Errorf("content = %q, want %q", string(got), "new text content")
	}
}

// ============================================================================
// Tests merged from main — Line Extraction Edge Cases
// ============================================================================

// TestExtractPosition_SingleLineFile extracts L1 from a file with exactly one line.
func TestExtractPosition_SingleLineFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "one.go")
	if err := os.WriteFile(filePath, []byte("only line"), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, hash, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "only line" {
		t.Errorf("extracted = %q, want %q", extracted, "only line")
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("hash = %q, want sha256: prefix", hash)
	}
}

// TestExtractPosition_L1ToEOF_EntireFile verifies that L1-EOF extracts the full file content.
func TestExtractPosition_L1ToEOF_EntireFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "full.go")
	content := "alpha\nbeta\ngamma"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != content {
		t.Errorf("extracted = %q, want %q", extracted, content)
	}
}

// NOTE: TestExtractPosition_EmptyFile removed — duplicated by the more comprehensive
// TestExtractPosition_EmptyFile_L1, _L2_Error, and _L1EOF tests above (lines 931-985).

// TestExtractPosition_NoTrailingNewline verifies that extraction preserves content
// exactly when the file has no trailing newline.
func TestExtractPosition_NoTrailingNewline(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "notrim.go")
	content := "first\nsecond\nthird"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// File with no trailing newline: "first\nsecond\nthird" → 3 lines
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "third" {
		t.Errorf("extracted = %q, want %q", extracted, "third")
	}
}

// TestExtractPosition_TrailingNewlineCreatesEmptyLine verifies that a trailing newline
// creates an additional empty line in the split result. "a\nb\n" → ["a", "b", ""] = 3 lines.
func TestExtractPosition_TrailingNewlineCreatesEmptyLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "trailing.go")
	content := "a\nb\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Line 3 should be the empty string after the trailing newline
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "" {
		t.Errorf("extracted = %q, want empty string for trailing newline phantom line", extracted)
	}

	// Line 4 should not exist
	_, _, err = ExtractPosition(filePath, &types.PositionSpec{StartLine: 4})
	if err == nil {
		t.Fatal("expected error for line beyond trailing newline phantom")
	}
}

// NOTE: TestExtractPosition_MixedLineEndings removed — duplicated by the more
// comprehensive TestExtractPosition_CRLF_LineExtraction test above (line 724).

// ============================================================================
// Column Extraction Edge Cases — StartCol Boundary Asymmetry
// ============================================================================

// TestExtractPosition_StartColBoundary_SingleVsMultiLine documents the intentional
// asymmetry between single-line and multi-line column extraction for StartCol at line
// boundary. Single-line mode errors when StartCol > len(line), but multi-line mode
// allows StartCol = len(line)+1 (clamped to end). This matches the semantics:
// in multi-line, starting "past the end" of the first line means extracting only
// from subsequent lines, which is valid. In single-line, there's nothing to extract.
func TestExtractPosition_StartColBoundary_SingleVsMultiLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "boundary.go")
	// "abc" = 3 bytes
	if err := os.WriteFile(filePath, []byte("abc\ndef\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Single-line: StartCol=4 on "abc" (len=3) → error
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 4, EndCol: 4,
	})
	if err == nil {
		t.Fatal("single-line: expected error for StartCol > len(line)")
	}

	// Multi-line: StartCol=4 on "abc" (len=3) → allowed, clamped to end of first line.
	// extractColumns multi-line allows StartCol up to len(firstLine)+1.
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 2, StartCol: 4, EndCol: 3,
	})
	if err != nil {
		t.Fatalf("multi-line: unexpected error for StartCol at line boundary: %v", err)
	}
	// First line from col 4 on "abc" (len=3): clamped → ""
	// Last line "def" to col 3: "def"
	if extracted != "\ndef" {
		t.Errorf("multi-line: extracted = %q, want %q", extracted, "\ndef")
	}
}

// ============================================================================
// Column Extraction Edge Cases (continued)
// ============================================================================

// TestExtractPosition_FullLineViaColumns verifies that L1C1:L1C{len} extracts the entire line.
func TestExtractPosition_FullLineViaColumns(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "fullcol.go")
	line := "hello world"
	if err := os.WriteFile(filePath, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: len(line),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != line {
		t.Errorf("extracted = %q, want %q", extracted, line)
	}
}

// TestExtractPosition_MultiLineColumnPreservesMiddle verifies that intermediate
// lines in a multi-line column extraction are included in full.
func TestExtractPosition_MultiLineColumnPreservesMiddle(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "multi.go")
	content := "111AAA\n222222\n333333\n444BBB\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C4:L4C3 → first line from col 4: "AAA", middle lines full: "222222", "333333",
	// last line to col 3: "444"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 4, StartCol: 4, EndCol: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "AAA\n222222\n333333\n444"
	if extracted != want {
		t.Errorf("extracted = %q, want %q", extracted, want)
	}
}

// TestExtractPosition_StartColExceedsLine_SingleLine verifies error when StartCol > line length
// on single-line column extraction.
func TestExtractPosition_StartColExceedsLine_SingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short.go")
	if err := os.WriteFile(filePath, []byte("abc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Line "abc" has length 3. StartCol=5 exceeds the line length.
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5,
	})
	if err == nil {
		t.Fatal("expected error for StartCol exceeding line length")
	}
	if !strings.Contains(err.Error(), "exceeds line length") {
		t.Errorf("error = %q, want 'exceeds line length'", err.Error())
	}
}

// TestExtractPosition_EndColExceedsLine_SingleLine verifies error when EndCol > line length
// on single-line column extraction.
func TestExtractPosition_EndColExceedsLine_SingleLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short2.go")
	if err := os.WriteFile(filePath, []byte("abc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 10,
	})
	if err == nil {
		t.Fatal("expected error for EndCol exceeding line length")
	}
	if !strings.Contains(err.Error(), "exceeds line length") {
		t.Errorf("error = %q, want 'exceeds line length'", err.Error())
	}
}

// TestExtractPosition_UnicodeByteIndexing verifies that column extraction
// is byte-based, not rune-based. Unicode characters that use multiple bytes
// will consume multiple column positions.
func TestExtractPosition_UnicodeByteIndexing(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "unicode.go")
	// "café" in UTF-8: c(1) a(1) f(1) é(2) = 5 bytes, 4 runes
	line := "café"
	if err := os.WriteFile(filePath, []byte(line+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// len("café") = 5 bytes. Extracting L1C1:L1C5 should give full string.
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: len(line),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != line {
		t.Errorf("extracted = %q, want %q", extracted, line)
	}

	// Extracting L1C4:L1C4 gives the first byte of 'é' (partial rune).
	// This is byte-based extraction — partial rune is expected.
	extracted, _, err = ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 4, EndCol: 4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(extracted) != 1 {
		t.Errorf("extracted length = %d bytes, want 1 (byte-based indexing)", len(extracted))
	}
}

// ============================================================================
// Tests merged from main — Hash Verification Edge Cases
// ============================================================================

// TestExtractPosition_HashDeterministic verifies that extracting the same content
// twice produces identical SHA-256 hashes.
func TestExtractPosition_HashDeterministic(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "hashtest.go")
	if err := os.WriteFile(filePath, []byte("stable content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, hash1, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("first extraction: %v", err)
	}
	_, hash2, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("second extraction: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("hash mismatch: %q != %q", hash1, hash2)
	}
}

// TestExtractPosition_HashDiffersForDifferentContent verifies that different content
// produces different SHA-256 hashes.
func TestExtractPosition_HashDiffersForDifferentContent(t *testing.T) {
	tempDir := t.TempDir()
	fileA := filepath.Join(tempDir, "a.go")
	fileB := filepath.Join(tempDir, "b.go")
	if err := os.WriteFile(fileA, []byte("content A\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("content B\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, hashA, err := ExtractPosition(fileA, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("extract A: %v", err)
	}
	_, hashB, err := ExtractPosition(fileB, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("extract B: %v", err)
	}
	if hashA == hashB {
		t.Errorf("different content should produce different hashes, both = %q", hashA)
	}
}

// TestExtractPosition_HashStableAcrossFiles verifies that the same content
// extracted from different files produces the same hash (content-addressed).
func TestExtractPosition_HashStableAcrossFiles(t *testing.T) {
	tempDir := t.TempDir()
	fileA := filepath.Join(tempDir, "a.go")
	fileB := filepath.Join(tempDir, "b.go")
	sameContent := "identical content\n"
	if err := os.WriteFile(fileA, []byte(sameContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte(sameContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, hashA, err := ExtractPosition(fileA, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("extract A: %v", err)
	}
	_, hashB, err := ExtractPosition(fileB, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("extract B: %v", err)
	}
	if hashA != hashB {
		t.Errorf("same content from different files should produce same hash: %q != %q", hashA, hashB)
	}
}

// ============================================================================
// Tests merged from main — PlaceContent Edge Cases
// ============================================================================

// TestPlaceContent_PositionIntoNonexistentFile verifies that placing with a position spec
// into a nonexistent file returns an error (cannot read the file to determine line positions).
func TestPlaceContent_PositionIntoNonexistentFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "ghost.go")

	err := PlaceContent(filePath, "content", &types.PositionSpec{StartLine: 1})
	if err == nil {
		t.Fatal("expected error placing with position into nonexistent file")
	}
	if !strings.Contains(err.Error(), "read target file") {
		t.Errorf("error = %q, want 'read target file' message", err.Error())
	}
}

// TestPlaceContent_AtFirstLine verifies replacing the first line of a file.
func TestPlaceContent_AtFirstLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "first.go")
	if err := os.WriteFile(filePath, []byte("old-first\nsecond\nthird\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := PlaceContent(filePath, "NEW-FIRST", &types.PositionSpec{StartLine: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "NEW-FIRST\nsecond\nthird\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", string(got), want)
	}
}

// TestPlaceContent_EndLineBeyondFileEnd verifies that placing at a line beyond
// the file's last line returns an error.
func TestPlaceContent_EndLineBeyondFileEnd(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short.go")
	if err := os.WriteFile(filePath, []byte("only\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// File has 2 lines ("only" and ""), endLine=5 is out of range
	err := PlaceContent(filePath, "stuff", &types.PositionSpec{StartLine: 1, EndLine: 5})
	if err == nil {
		t.Fatal("expected error for endLine beyond file length")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist' message", err.Error())
	}
}

// TestPlaceContent_ColumnPreservesSurrounding verifies that column-precise replacement
// preserves content before StartCol and after EndCol.
func TestPlaceContent_ColumnPreservesSurrounding(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "surround.go")
	if err := os.WriteFile(filePath, []byte("prefix_MIDDLE_suffix\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// "prefix_MIDDLE_suffix" = 20 chars
	// Replace cols 8-14 ("MIDDLE_") with "REPLACED"
	// line[:7] + "REPLACED" + line[14:] = "prefix_" + "REPLACED" + "suffix"
	pos := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 8, EndCol: 14}
	if err := PlaceContent(filePath, "REPLACED", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "prefix_REPLACEDsuffix\nline2\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", string(got), want)
	}
}

// TestPlaceContent_IdempotentSameContent verifies that overwriting a range with
// the same content that's already there is idempotent.
func TestPlaceContent_IdempotentSameContent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "idem.go")
	original := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Replace line 2 with "line2" (same content)
	if err := PlaceContent(filePath, "line2", &types.PositionSpec{StartLine: 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != original {
		t.Errorf("idempotent replace changed file: %q, want %q", string(got), original)
	}
}

// TestPlaceContent_OverwriteDifferentContent verifies that overwriting a range
// with different content fully replaces the old content.
func TestPlaceContent_OverwriteDifferentContent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "overwrite.go")
	if err := os.WriteFile(filePath, []byte("keep\nOLD_VALUE\nkeep\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// First overwrite
	if err := PlaceContent(filePath, "FIRST_REPLACE", &types.PositionSpec{StartLine: 2}); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	got, _ := os.ReadFile(filePath)
	if string(got) != "keep\nFIRST_REPLACE\nkeep\n" {
		t.Errorf("after first: %q", string(got))
	}

	// Second overwrite at same position
	if err := PlaceContent(filePath, "SECOND_REPLACE", &types.PositionSpec{StartLine: 2}); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	got, _ = os.ReadFile(filePath)
	if string(got) != "keep\nSECOND_REPLACE\nkeep\n" {
		t.Errorf("after second: %q, old content should be fully gone", string(got))
	}
}

// TestPlaceContent_MultiLineColumnReplace verifies multi-line column-precise replacement.
func TestPlaceContent_MultiLineColumnReplace(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "multicol.go")
	content := "aaaBBB\nCCCCCC\nDDDDDD\nEEEfff\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Replace from L1C4 to L4C3: "BBB\nCCCCCC\nDDDDDD\nEEE" → "REPLACEMENT"
	// prefix = "aaa", suffix = "fff"
	// Result: lines before L1 (none) + "aaa" + "REPLACEMENT" + "fff" + lines after L4 ("")
	pos := &types.PositionSpec{StartLine: 1, EndLine: 4, StartCol: 4, EndCol: 3}
	if err := PlaceContent(filePath, "REPLACEMENT", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "aaaREPLACEMENTfff\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", string(got), want)
	}
}

// TestPlaceContent_ColumnReplace_EndLineBeyondFile verifies error when column
// replacement targets a line beyond the file.
func TestPlaceContent_ColumnReplace_EndLineBeyondFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short.go")
	if err := os.WriteFile(filePath, []byte("abc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 1, EndLine: 10, StartCol: 1, EndCol: 3}
	err := PlaceContent(filePath, "X", pos)
	if err == nil {
		t.Fatal("expected error for endLine beyond file")
	}
}

// ============================================================================
// Tests merged from main — Round-Trip Edge Cases
// ============================================================================

// TestExtractThenPlace_ColumnRoundTrip verifies extract → place round-trip
// with column-precise specs preserves surrounding content.
func TestExtractThenPlace_ColumnRoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	// Source file
	srcPath := filepath.Join(tempDir, "src.go")
	if err := os.WriteFile(srcPath, []byte("aaaBBBccc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract columns 4-6 ("BBB")
	extracted, hash, err := ExtractPosition(srcPath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 4, EndCol: 6,
	})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if extracted != "BBB" {
		t.Fatalf("extracted = %q, want %q", extracted, "BBB")
	}

	// Place into target at same column range
	dstPath := filepath.Join(tempDir, "dst.go")
	if err := os.WriteFile(dstPath, []byte("xxxYYYzzz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := PlaceContent(dstPath, extracted, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 4, EndCol: 6,
	}); err != nil {
		t.Fatalf("place error: %v", err)
	}

	got, _ := os.ReadFile(dstPath)
	if string(got) != "xxxBBBzzz\n" {
		t.Errorf("round-trip result = %q, want %q", string(got), "xxxBBBzzz\n")
	}

	// Verify hash matches re-extraction from destination
	_, reHash, err := ExtractPosition(dstPath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 4, EndCol: 6,
	})
	if err != nil {
		t.Fatalf("re-extract error: %v", err)
	}
	if reHash != hash {
		t.Errorf("hash mismatch after round-trip: %q != %q", reHash, hash)
	}
}

// ============================================================================
// Tests merged from main — Binary Input (updated for binary rejection)
// Binary files are now rejected by ExtractPosition and PlaceContent (with position).
// These tests verify the rejection behavior matches the guard implemented in
// position_extract.go via isBinaryContent().
// ============================================================================

// TestExtractPosition_BinaryFileInput_Rejected verifies that ExtractPosition rejects
// binary data (files with null bytes in the first 8000 bytes).
func TestExtractPosition_BinaryFileInput_Rejected(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "binary.bin")
	// Binary data with null bytes, high bytes, and embedded newlines
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG header
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0xFF, 0xFE, 0xFD, 0x0A, // embedded \n
		0x01, 0x02, 0x03, 0x0A, // embedded \n
		0x04, 0x05, 0x06}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Extract line 1 — should error due to binary detection
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err == nil {
		t.Fatal("expected error for binary file extraction")
	}
	if !strings.Contains(err.Error(), "binary file") {
		t.Errorf("error = %q, want message about binary file", err.Error())
	}
}

// TestExtractPosition_BinaryColumnExtraction_Rejected verifies column extraction
// on binary data is also rejected.
func TestExtractPosition_BinaryColumnExtraction_Rejected(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "bin_col.bin")
	// Binary data with null byte
	data := []byte{0x01, 0x02, 0x00, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0A}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// L1C2:L1C4 — should error due to binary detection
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 2, EndCol: 4,
	})
	if err == nil {
		t.Fatal("expected error for binary column extraction")
	}
	if !strings.Contains(err.Error(), "binary file") {
		t.Errorf("error = %q, want message about binary file", err.Error())
	}
}

// ============================================================================
// Tests merged from main — Position Past EOF
// ============================================================================

// TestExtractPosition_StartLinePastEOF_ErrorMessage verifies the error message
// includes the file path and line count.
func TestExtractPosition_StartLinePastEOF_ErrorMessage(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "short.go")
	if err := os.WriteFile(filePath, []byte("one\ntwo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// File has 3 lines ("one", "two", ""). L10 is past EOF.
	_, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 10})
	if err == nil {
		t.Fatal("expected error for startLine past EOF")
	}
	if !strings.Contains(err.Error(), "line 10") {
		t.Errorf("error should mention line 10: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "3 lines") {
		t.Errorf("error should mention file has 3 lines: %q", err.Error())
	}
}

// TestExtractPosition_ToEOF_SingleLineFile verifies L1-EOF on a single-line file.
func TestExtractPosition_ToEOF_SingleLineFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "single.go")
	if err := os.WriteFile(filePath, []byte("sole line"), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1, ToEOF: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "sole line" {
		t.Errorf("extracted = %q, want %q", extracted, "sole line")
	}
}

// ============================================================================
// Tests merged from main — Column Boundary Cases
// ============================================================================

// TestExtractPosition_ColumnAtLineBoundary_FirstChar extracts the first character.
func TestExtractPosition_ColumnAtLineBoundary_FirstChar(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "boundary.go")
	if err := os.WriteFile(filePath, []byte("ABCDEF\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C1:L1C1 → "A"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "A" {
		t.Errorf("extracted = %q, want %q", extracted, "A")
	}
}

// TestExtractPosition_ColumnAtLineBoundary_LastChar extracts the last character.
func TestExtractPosition_ColumnAtLineBoundary_LastChar(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "boundary2.go")
	if err := os.WriteFile(filePath, []byte("ABCDEF\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C6:L1C6 → "F"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 6, EndCol: 6,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "F" {
		t.Errorf("extracted = %q, want %q", extracted, "F")
	}
}

// TestExtractPosition_MultiLineColumn_AdjacentLines verifies column extraction
// across exactly two lines (no middle lines).
func TestExtractPosition_MultiLineColumn_AdjacentLines(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "adjacent.go")
	if err := os.WriteFile(filePath, []byte("AAABBB\nCCCDDD\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C4:L2C3 → first line from col 4: "BBB", last line to col 3: "CCC"
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 2, StartCol: 4, EndCol: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "BBB\nCCC" {
		t.Errorf("extracted = %q, want %q", extracted, "BBB\nCCC")
	}
}

// ============================================================================
// Tests merged from main — PlaceContent Empty File and Edge Cases
// ============================================================================

// TestPlaceContent_NilPos_IntoEmptyFile writes content into an empty file with nil pos.
func TestPlaceContent_NilPos_IntoEmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.go")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	if err := PlaceContent(filePath, "brand new content", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "brand new content" {
		t.Errorf("content = %q, want %q", string(got), "brand new content")
	}
}

// TestPlaceContent_NilPos_CreatesNewFile creates a file that doesn't exist.
func TestPlaceContent_NilPos_CreatesNewFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "subdir", "new.go")
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := PlaceContent(filePath, "new file content", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "new file content" {
		t.Errorf("content = %q, want %q", string(got), "new file content")
	}
}

// TestPlaceContent_WithPos_IntoEmptyFile_L1 places content at L1 into an empty file.
// An empty file has 1 line (the empty string), so L1 is valid.
func TestPlaceContent_WithPos_IntoEmptyFile_L1(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty_place.go")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 1}
	if err := PlaceContent(filePath, "inserted", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "inserted" {
		t.Errorf("content = %q, want %q", string(got), "inserted")
	}
}

// TestPlaceContent_OverlappingRanges_ExpandsFile verifies that replacing a range
// with more lines than the original range expands the file.
func TestPlaceContent_OverlappingRanges_ExpandsFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "expand.go")
	if err := os.WriteFile(filePath, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Replace line 2 (single line "b") with 3 lines
	pos := &types.PositionSpec{StartLine: 2}
	if err := PlaceContent(filePath, "x\ny\nz", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "a\nx\ny\nz\nc\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", string(got), want)
	}
}

// TestPlaceContent_OverlappingRanges_ShrinksFile verifies that replacing a range
// with fewer lines than the original range shrinks the file.
func TestPlaceContent_OverlappingRanges_ShrinksFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "shrink.go")
	if err := os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Replace lines 2-4 ("b\nc\nd") with single line
	pos := &types.PositionSpec{StartLine: 2, EndLine: 4}
	if err := PlaceContent(filePath, "REPLACED", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	want := "a\nREPLACED\ne\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", string(got), want)
	}
}

// TestPlaceContent_ColumnReplace_EmptyReplacement verifies that replacing a column
// range with an empty string deletes those characters.
func TestPlaceContent_ColumnReplace_EmptyReplacement(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "delete_cols.go")
	if err := os.WriteFile(filePath, []byte("HelloWorld\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Delete cols 6-10 ("World") → "Hello"
	pos := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 6, EndCol: 10}
	if err := PlaceContent(filePath, "", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "Hello\n" {
		t.Errorf("content = %q, want %q", string(got), "Hello\n")
	}
}

// TestPlaceContent_LastLine verifies replacing the very last line of a file.
func TestPlaceContent_LastLine(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "lastline.go")
	// No trailing newline — 3 lines: "a", "b", "c"
	if err := os.WriteFile(filePath, []byte("a\nb\nc"), 0644); err != nil {
		t.Fatal(err)
	}

	pos := &types.PositionSpec{StartLine: 3}
	if err := PlaceContent(filePath, "C_REPLACED", pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filePath)
	if string(got) != "a\nb\nC_REPLACED" {
		t.Errorf("content = %q, want %q", string(got), "a\nb\nC_REPLACED")
	}
}
