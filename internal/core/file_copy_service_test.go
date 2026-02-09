package core

import (
	"fmt"
	"os"
	"path/filepath"
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
