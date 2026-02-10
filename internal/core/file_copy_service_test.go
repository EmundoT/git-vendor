package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// File Copy Tests
// ============================================================================

// TestCopyFile tests file copying functionality
func TestCopyFile(t *testing.T) {
	fs := NewOSFileSystem()

	t.Run("Successful file copy", func(t *testing.T) {
		tempDir := t.TempDir()
		srcFile := filepath.Join(tempDir, "source.txt")
		dstFile := filepath.Join(tempDir, "dest.txt")

		// Create source file
		content := "test content"
		if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		// Copy file
		if _, err := fs.CopyFile(srcFile, dstFile); err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		// Verify destination exists and has same content
		got, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("Failed to read destination file: %v", err)
		}
		if string(got) != content {
			t.Errorf("copyFile() content = %q, want %q", string(got), content)
		}
	})

	t.Run("Error when source doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		srcFile := filepath.Join(tempDir, "nonexistent.txt")
		dstFile := filepath.Join(tempDir, "dest.txt")

		_, err := fs.CopyFile(srcFile, dstFile)
		if err == nil {
			t.Error("CopyFile() expected error for nonexistent source, got nil")
		}
	})

	t.Run("Error when destination directory doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		srcFile := filepath.Join(tempDir, "source.txt")
		dstFile := filepath.Join(tempDir, "nonexistent", "dest.txt")

		// Create source file
		if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		_, err := fs.CopyFile(srcFile, dstFile)
		if err == nil {
			t.Error("CopyFile() expected error for nonexistent destination directory, got nil")
		}
	})
}

// ============================================================================
// Directory Copy Tests
// ============================================================================

// TestCopyDir tests directory copying functionality
func TestCopyDir(t *testing.T) {
	fs := NewOSFileSystem()

	t.Run("Successful directory copy", func(t *testing.T) {
		tempDir := t.TempDir()
		srcDir := filepath.Join(tempDir, "source")
		dstDir := filepath.Join(tempDir, "dest")

		// Create source directory with files
		_ = os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
		_ = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
		_ = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

		// Copy directory
		if _, err := fs.CopyDir(srcDir, dstDir); err != nil {
			t.Fatalf("CopyDir() error = %v", err)
		}

		// Verify destination files exist
		if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); err != nil {
			t.Errorf("copyDir() file1.txt not copied: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); err != nil {
			t.Errorf("copyDir() subdir/file2.txt not copied: %v", err)
		}

		// Verify content
		content, _ := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
		if string(content) != "content1" {
			t.Errorf("copyDir() file1.txt content = %q, want %q", string(content), "content1")
		}
	})

	t.Run("Directory copy skips .git directories", func(t *testing.T) {
		tempDir := t.TempDir()
		srcDir := filepath.Join(tempDir, "source")
		dstDir := filepath.Join(tempDir, "dest")

		// Create source with .git directory
		_ = os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
		_ = os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644)
		_ = os.WriteFile(filepath.Join(srcDir, ".git", "config"), []byte("gitconfig"), 0644)

		// Copy directory
		if _, err := fs.CopyDir(srcDir, dstDir); err != nil {
			t.Fatalf("CopyDir() error = %v", err)
		}

		// Verify regular file was copied
		if _, err := os.Stat(filepath.Join(dstDir, "file.txt")); err != nil {
			t.Errorf("copyDir() file.txt not copied: %v", err)
		}

		// Verify .git was NOT copied
		if _, err := os.Stat(filepath.Join(dstDir, ".git", "config")); err == nil {
			t.Error("copyDir() .git/config was copied, but should have been skipped")
		}
	})

	t.Run("Error when source directory doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		srcDir := filepath.Join(tempDir, "nonexistent")
		dstDir := filepath.Join(tempDir, "dest")

		_, err := fs.CopyDir(srcDir, dstDir)
		if err == nil {
			t.Error("CopyDir() expected error for nonexistent source, got nil")
		}
	})
}

// ============================================================================
// Path Mapping Copy Tests
// ============================================================================

func TestCopyMappings_AutoNaming(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Test auto-naming with empty "to" field
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: ""}, // Empty "to" triggers auto-naming
				},
			},
		},
	}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success (auto-naming), got error: %v", err)
	}
}

func TestCopyMappings_DirectoryCopy(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Test directory copy
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/", To: "lib/"},
				},
			},
		},
	}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 3, ByteCount: 300}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes() // For license copy
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success (directory copy), got error: %v", err)
	}
}

func TestCopyMappings_PathNotFound(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	// Mock: Stat returns error (path not found)
	fs.EXPECT().Stat(gomock.Any()).Return(nil, fmt.Errorf("path not found")).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error (path not found), got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// ============================================================================
// CopyMappings Position Integration Tests
// ============================================================================

// TestCopyMappings_PositionNonexistentSource verifies that a position mapping
// with a nonexistent source file propagates an error.
func TestCopyMappings_PositionNonexistentSource(t *testing.T) {
	repoDir := t.TempDir() // Empty "clone" directory — no source files
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "pos-test"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "missing.go:L1-L5", To: "dest.go"},
		},
	}

	_, err = svc.CopyMappings(repoDir, vendor, spec)
	if err == nil {
		t.Fatal("expected error for nonexistent source file with position spec")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found' message", err.Error())
	}
}

// TestCopyMappings_PositionCreatesDestDir verifies that position mappings
// automatically create intermediate destination directories.
func TestCopyMappings_PositionCreatesDestDir(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source file in "repo"
	srcContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(repoDir, "source.go"), []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "dir-test"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "source.go:L2", To: "deep/nested/dir/dest.go"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
	if len(stats.Positions) != 1 {
		t.Fatalf("Positions = %d, want 1", len(stats.Positions))
	}

	// Verify dest file was created in nested dir
	got, err := os.ReadFile(filepath.Join(workDir, "deep/nested/dir/dest.go"))
	if err != nil {
		t.Fatalf("dest file not created: %v", err)
	}
	if string(got) != "line2" {
		t.Errorf("dest content = %q, want %q", string(got), "line2")
	}
}

// TestCopyMappings_MultiplePositionsToSameFile verifies that two position mappings
// targeting the same destination file at different ranges both apply correctly.
func TestCopyMappings_MultiplePositionsToSameFile(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source files in "repo"
	if err := os.WriteFile(filepath.Join(repoDir, "src1.go"), []byte("alpha\nbeta\ngamma\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "src2.go"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create the target file so position placement can read it
	destPath := filepath.Join(workDir, "target.go")
	if err := os.WriteFile(destPath, []byte("placeholder1\nplaceholder2\nplaceholder3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "multi-pos"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			// First mapping: extract "beta" from src1, place at line 1 of target
			{From: "src1.go:L2", To: "target.go:L1"},
			// Second mapping: extract "two" from src2, place at line 3 of target
			{From: "src2.go:L2", To: "target.go:L3"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats.Positions) != 2 {
		t.Fatalf("Positions = %d, want 2", len(stats.Positions))
	}

	// After both placements, line 1 = "beta", line 2 = "placeholder2", line 3 = "two"
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	want := "beta\nplaceholder2\ntwo\n"
	if string(got) != want {
		t.Errorf("dest content = %q, want %q", string(got), want)
	}
}

// TestCopyMappings_MixedWholeFileAndPosition verifies that a CopyMappings call
// with both whole-file and position mappings handles both code paths.
func TestCopyMappings_MixedWholeFileAndPosition(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source files in "repo"
	if err := os.WriteFile(filepath.Join(repoDir, "full.go"), []byte("full file content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "partial.go"), []byte("skip\nextract-me\nskip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "mixed-test"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			// Whole-file copy (no position spec)
			{From: "full.go", To: "copied_full.go"},
			// Position extract
			{From: "partial.go:L2", To: "excerpt.go"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whole-file copy: FileCount from CopyFile
	// Position extract: FileCount from copyWithPosition
	if stats.FileCount < 2 {
		t.Errorf("FileCount = %d, want >= 2", stats.FileCount)
	}

	// Verify whole-file copy
	got, err := os.ReadFile(filepath.Join(workDir, "copied_full.go"))
	if err != nil {
		t.Fatalf("whole-file copy not found: %v", err)
	}
	if string(got) != "full file content\n" {
		t.Errorf("whole-file content = %q, want %q", string(got), "full file content\n")
	}

	// Verify position extract
	got, err = os.ReadFile(filepath.Join(workDir, "excerpt.go"))
	if err != nil {
		t.Fatalf("position extract not found: %v", err)
	}
	if string(got) != "extract-me" {
		t.Errorf("position content = %q, want %q", string(got), "extract-me")
	}

	// Position tracking
	if len(stats.Positions) != 1 {
		t.Errorf("Positions = %d, want 1 (only the position mapping)", len(stats.Positions))
	}
}

// ============================================================================
// checkLocalModifications Tests
// ============================================================================

// TestCheckLocalModifications_WholeFile_MatchingContent verifies no warning
// when the destination file already contains the exact incoming content.
func TestCheckLocalModifications_WholeFile_MatchingContent(t *testing.T) {
	workDir := t.TempDir()
	destPath := filepath.Join(workDir, "match.go")
	content := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	warning := svc.checkLocalModifications(destPath, nil, content)
	if warning != "" {
		t.Errorf("expected no warning for matching content, got %q", warning)
	}
}

// TestCheckLocalModifications_WholeFile_Drift verifies a warning when
// the destination file content differs from incoming content.
func TestCheckLocalModifications_WholeFile_Drift(t *testing.T) {
	workDir := t.TempDir()
	destPath := filepath.Join(workDir, "drift.go")
	if err := os.WriteFile(destPath, []byte("original code\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	warning := svc.checkLocalModifications(destPath, nil, "new code\n")
	if warning == "" {
		t.Error("expected warning for modified content, got empty")
	}
	if !contains(warning, "has local modifications") {
		t.Errorf("warning = %q, want 'has local modifications'", warning)
	}
}

// TestCheckLocalModifications_WholeFile_MissingDest verifies no warning
// when the destination file does not exist (fresh install).
func TestCheckLocalModifications_WholeFile_MissingDest(t *testing.T) {
	svc := NewFileCopyService(NewOSFileSystem())
	warning := svc.checkLocalModifications("/nonexistent/path/file.go", nil, "content")
	if warning != "" {
		t.Errorf("expected no warning for missing dest, got %q", warning)
	}
}

// TestCheckLocalModifications_Position_MatchingRange verifies no warning
// when the destination file's target position already has the incoming content.
func TestCheckLocalModifications_Position_MatchingRange(t *testing.T) {
	workDir := t.TempDir()
	destPath := filepath.Join(workDir, "pos_match.go")
	if err := os.WriteFile(destPath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	pos := &types.PositionSpec{StartLine: 2}
	warning := svc.checkLocalModifications(destPath, pos, "line2")
	if warning != "" {
		t.Errorf("expected no warning for matching position content, got %q", warning)
	}
}

// TestCheckLocalModifications_Position_Drift verifies a warning when
// the destination file's target position differs from incoming content.
func TestCheckLocalModifications_Position_Drift(t *testing.T) {
	workDir := t.TempDir()
	destPath := filepath.Join(workDir, "pos_drift.go")
	if err := os.WriteFile(destPath, []byte("line1\nMODIFIED\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	pos := &types.PositionSpec{StartLine: 2}
	warning := svc.checkLocalModifications(destPath, pos, "original-line2")
	if warning == "" {
		t.Error("expected warning for position-level drift, got empty")
	}
	if !contains(warning, "at target position") {
		t.Errorf("warning = %q, want 'at target position' message", warning)
	}
}

// TestCheckLocalModifications_Position_MissingDest verifies no warning
// when the destination file doesn't exist yet (position on new file).
func TestCheckLocalModifications_Position_MissingDest(t *testing.T) {
	svc := NewFileCopyService(NewOSFileSystem())
	pos := &types.PositionSpec{StartLine: 1}
	warning := svc.checkLocalModifications("/nonexistent/pos.go", pos, "content")
	if warning != "" {
		t.Errorf("expected no warning for missing dest with position, got %q", warning)
	}
}

// ============================================================================
// copyWithPosition Tests — Destination lifecycle
// ============================================================================

// TestCopyWithPosition_DestDoesNotExist verifies that copyWithPosition creates
// a new destination file when it doesn't exist.
func TestCopyWithPosition_DestDoesNotExist(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source
	if err := os.WriteFile(filepath.Join(repoDir, "api.go"), []byte("alpha\nbeta\ngamma\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "test-vendor"}
	spec := types.BranchSpec{
		Ref:     "main",
		Mapping: []types.PathMapping{{From: "api.go:L2", To: "new_dest.go"}},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
	if len(stats.Positions) != 1 {
		t.Fatalf("Positions = %d, want 1", len(stats.Positions))
	}

	got, err := os.ReadFile(filepath.Join(workDir, "new_dest.go"))
	if err != nil {
		t.Fatalf("dest file not created: %v", err)
	}
	if string(got) != "beta" {
		t.Errorf("content = %q, want %q", string(got), "beta")
	}
}

// TestCopyWithPosition_DestHasFewerLines verifies that copyWithPosition returns
// an error when the destination file has fewer lines than the target position.
// PlaceContent does not pad — it requires the target line to exist.
func TestCopyWithPosition_DestHasFewerLines(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Source with content to extract
	if err := os.WriteFile(filepath.Join(repoDir, "src.go"), []byte("extracted\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Destination with only 2 lines (3 with trailing empty from \n)
	destPath := filepath.Join(workDir, "short_dest.go")
	if err := os.WriteFile(destPath, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "test-vendor"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			// Place at line 5, but dest only has 3 lines → PlaceContent should error
			{From: "src.go:L1", To: "short_dest.go:L5"},
		},
	}

	_, err := svc.CopyMappings(repoDir, vendor, spec)
	if err == nil {
		t.Fatal("expected error for target line past EOF, got nil")
	}
	if !contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist' message", err.Error())
	}

	// Verify original file is unchanged
	got, _ := os.ReadFile(destPath)
	if string(got) != "line1\nline2\n" {
		t.Errorf("dest should be unchanged, got %q", string(got))
	}
}

// TestCopyWithPosition_WarningRecorded verifies that local modification warnings
// from copyWithPosition are recorded in CopyStats.Warnings.
func TestCopyWithPosition_WarningRecorded(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Source
	if err := os.WriteFile(filepath.Join(repoDir, "src.go"), []byte("new-content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing destination with DIFFERENT content → triggers warning
	destPath := filepath.Join(workDir, "warn_dest.go")
	if err := os.WriteFile(destPath, []byte("locally-modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "warn-vendor"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "src.go:L1", To: "warn_dest.go"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats.Warnings) == 0 {
		t.Error("expected at least 1 warning for local modifications, got none")
	}
	if len(stats.Warnings) > 0 && !contains(stats.Warnings[0], "local modifications") {
		t.Errorf("warning = %q, want 'local modifications' message", stats.Warnings[0])
	}
}

// TestCopyWithPosition_PositionRecordFields verifies that the positionRecord
// returned by copyWithPosition has correct From, To, and SourceHash fields.
func TestCopyWithPosition_PositionRecordFields(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	if err := os.WriteFile(filepath.Join(repoDir, "api.go"), []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create destination file so PlaceContent can target L1
	if err := os.WriteFile(filepath.Join(workDir, "dest.go"), []byte("placeholder\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "record-test"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "api.go:L2-L3", To: "dest.go:L1"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats.Positions) != 1 {
		t.Fatalf("Positions = %d, want 1", len(stats.Positions))
	}

	rec := stats.Positions[0]
	if rec.From != "api.go:L2-L3" {
		t.Errorf("From = %q, want %q", rec.From, "api.go:L2-L3")
	}
	if rec.To != "dest.go:L1" {
		t.Errorf("To = %q, want %q", rec.To, "dest.go:L1")
	}
	if !strings.HasPrefix(rec.SourceHash, "sha256:") {
		t.Errorf("SourceHash = %q, want sha256: prefix", rec.SourceHash)
	}
}

// ============================================================================
// CopyMappings — Mixed Mapping Scenarios
// ============================================================================

// TestCopyMappings_MixedWholeFileAndPosition_Stats verifies that stats are
// correctly accumulated when mixing whole-file and position mappings.
func TestCopyMappings_MixedWholeFileAndPosition_Stats(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Setup: create source files
	if err := os.WriteFile(filepath.Join(repoDir, "whole.go"), []byte("whole file\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "partial.go"), []byte("skip\nextracted\nskip\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "dir", "sub.go"), []byte("sub content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "mixed-stats"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "whole.go", To: "out/whole.go"},
			{From: "partial.go:L2", To: "out/excerpt.go"},
			{From: "dir/", To: "out/dir/"},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: 1 whole file + 1 position + 1 dir file = 3+ files
	if stats.FileCount < 3 {
		t.Errorf("FileCount = %d, want >= 3", stats.FileCount)
	}
	// Only position mappings counted
	if len(stats.Positions) != 1 {
		t.Errorf("Positions = %d, want 1", len(stats.Positions))
	}

	// Verify all output files
	if _, err := os.Stat(filepath.Join(workDir, "out/whole.go")); err != nil {
		t.Errorf("whole.go not copied: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(workDir, "out/excerpt.go"))
	if string(got) != "extracted" {
		t.Errorf("excerpt content = %q, want %q", string(got), "extracted")
	}
	if _, err := os.Stat(filepath.Join(workDir, "out/dir/sub.go")); err != nil {
		t.Errorf("dir/sub.go not copied: %v", err)
	}
}

// ============================================================================
// cleanSourcePath Unit Tests
// ============================================================================

// TestCleanSourcePath_StripsBlobPrefix verifies that cleanSourcePath removes
// "blob/<ref>/" prefixes that appear in GitHub deep-link URLs.
func TestCleanSourcePath_StripsBlobPrefix(t *testing.T) {
	svc := &FileCopyService{fs: NewOSFileSystem()}

	tests := []struct {
		path, ref, want string
	}{
		{"blob/main/src/file.go", "main", "src/file.go"},
		{"tree/v1.0/src/lib/", "v1.0", "src/lib/"},
		{"src/file.go", "main", "src/file.go"},                        // no prefix → unchanged
		{"blob/main/blob/main/deep.go", "main", "blob/main/deep.go"}, // only first match stripped
	}

	for _, tt := range tests {
		got := svc.cleanSourcePath(tt.path, tt.ref)
		if got != tt.want {
			t.Errorf("cleanSourcePath(%q, %q) = %q, want %q", tt.path, tt.ref, got, tt.want)
		}
	}
}

// ============================================================================
// computeDestPath Unit Tests
// ============================================================================

// TestComputeDestPath_AutoPathStripsPosition verifies that computeDestPath strips
// position specifiers from the source path before computing auto-path when the
// destination is empty.
func TestComputeDestPath_AutoPathStripsPosition(t *testing.T) {
	svc := &FileCopyService{fs: NewOSFileSystem()}
	vendor := &types.VendorSpec{Name: "test-vendor"}

	// Source has position spec, dest is empty → auto-path from basename without position
	mapping := types.PathMapping{From: "src/config.go:L1-L5", To: ""}
	spec := types.BranchSpec{Ref: "main"}

	got := svc.computeDestPath(mapping, spec, vendor)
	// Auto-path should be based on "config.go" basename, not "config.go:L1-L5"
	if strings.Contains(got, ":L") {
		t.Errorf("computeDestPath should strip position from auto-path, got %q", got)
	}
	if !strings.Contains(got, "config.go") {
		t.Errorf("computeDestPath should preserve base filename, got %q", got)
	}
}

// TestComputeDestPath_ExplicitDestPreserved verifies that computeDestPath returns
// the explicit destination path (including any position spec) when To is non-empty.
func TestComputeDestPath_ExplicitDestPreserved(t *testing.T) {
	svc := &FileCopyService{fs: NewOSFileSystem()}
	vendor := &types.VendorSpec{Name: "test-vendor"}

	mapping := types.PathMapping{From: "src/api.go:L5-L20", To: "lib/api.go:L10-L25"}
	spec := types.BranchSpec{Ref: "main"}

	got := svc.computeDestPath(mapping, spec, vendor)
	if got != "lib/api.go:L10-L25" {
		t.Errorf("computeDestPath should preserve explicit dest, got %q", got)
	}
}
