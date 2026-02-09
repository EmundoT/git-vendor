package core

import (
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
	// Second vendor writes to L5 — but this is now "line6" not "line5"
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
