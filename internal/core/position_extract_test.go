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

func TestCopyStats_WarningsAggregation(t *testing.T) {
	s1 := CopyStats{Warnings: []string{"warn1"}}
	s2 := CopyStats{Warnings: []string{"warn2", "warn3"}}
	s1.Add(s2)
	if len(s1.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(s1.Warnings))
	}
}

// ============================================================================
// ExtractPosition Edge Cases — Line Extraction
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

// TestExtractPosition_EmptyFile verifies behavior on a zero-byte file.
// strings.Split("", "\n") yields [""] (one empty line), so L1 returns ""
// and L2 errors. This is documented as intentional: an empty file has one empty line.
func TestExtractPosition_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.go")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// L1 on empty file → empty string (the single empty line)
	extracted, hash, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 1})
	if err != nil {
		t.Fatalf("unexpected error for L1 on empty file: %v", err)
	}
	if extracted != "" {
		t.Errorf("extracted = %q, want empty string", extracted)
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("hash = %q, want sha256: prefix", hash)
	}

	// L2 on empty file → error
	_, _, err = ExtractPosition(filePath, &types.PositionSpec{StartLine: 2})
	if err == nil {
		t.Fatal("expected error for L2 on empty file")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist' message", err.Error())
	}
}

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

// TestExtractPosition_MixedLineEndings verifies that \r\n line endings are preserved
// (not normalized). Since split is on \n, \r remains at end of each line.
// Column indexing operates on bytes including \r. This is documented as intentional:
// position extraction is byte-based.
func TestExtractPosition_MixedLineEndings(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "crlf.go")
	content := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// strings.Split on \n: ["line1\r", "line2\r", "line3\r", ""]
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{StartLine: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// \r is preserved — extraction is byte-level, not line-ending-aware
	if extracted != "line2\r" {
		t.Errorf("extracted = %q, want %q (\\r preserved)", extracted, "line2\r")
	}
}

// ============================================================================
// ExtractPosition Edge Cases — Column Extraction
// ============================================================================

// TestExtractPosition_SingleCharColumn verifies L1C5:L1C5 extracts exactly one character.
func TestExtractPosition_SingleCharColumn(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "char.go")
	if err := os.WriteFile(filePath, []byte("abcdefghij\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// L1C5:L1C5 → line[4:5] = "e" (1 char)
	extracted, _, err := ExtractPosition(filePath, &types.PositionSpec{
		StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extracted != "e" {
		t.Errorf("extracted = %q, want %q", extracted, "e")
	}
}

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

	// Line "abc" has length 3. StartCol=5 exceeds it.
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
// ExtractPosition Edge Cases — Hash Verification
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
// PlaceContent Edge Cases
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
// Round-Trip Edge Cases
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
