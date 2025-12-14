package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"git-vendor/internal/types"
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
		if err := fs.CopyFile(srcFile, dstFile); err != nil {
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

		err := fs.CopyFile(srcFile, dstFile)
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

		err := fs.CopyFile(srcFile, dstFile)
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
		os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
		os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
		os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

		// Copy directory
		if err := fs.CopyDir(srcDir, dstDir); err != nil {
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
		os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
		os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644)
		os.WriteFile(filepath.Join(srcDir, ".git", "config"), []byte("gitconfig"), 0644)

		// Copy directory
		if err := fs.CopyDir(srcDir, dstDir); err != nil {
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

		err := fs.CopyDir(srcDir, dstDir)
		if err == nil {
			t.Error("CopyDir() expected error for nonexistent source, got nil")
		}
	})
}

// ============================================================================
// Path Mapping Copy Tests
// ============================================================================

func TestCopyMappings_AutoNaming(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

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

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success (auto-naming), got error: %v", err)
	}

	// Verify file was copied (auto-named as "file.go")
	if len(fs.CopyFileCalls) < 1 {
		t.Error("Expected at least 1 CopyFile call")
	}
}

func TestCopyMappings_DirectoryCopy(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

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

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		// Return isDir=true for directory paths
		return &mockFileInfo{name: filepath.Base(path), isDir: true}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success (directory copy), got error: %v", err)
	}

	// Verify directory was copied
	if len(fs.CopyDirCalls) < 1 {
		t.Error("Expected at least 1 CopyDir call")
	}
}

func TestCopyMappings_PathNotFound(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	// Mock: Stat returns error (path not found)
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return nil, fmt.Errorf("path not found")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error (path not found), got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}
