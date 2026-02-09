package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// ReadDir Tests
// ============================================================================

func TestOSFileSystem_ReadDir(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create test directory structure
	subDir1 := filepath.Join(tempDir, "dir1")
	subDir2 := filepath.Join(tempDir, "dir2")
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")

	os.Mkdir(subDir1, 0755)
	os.Mkdir(subDir2, 0755)
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	// Test: ReadDir should list all entries
	entries, err := fs.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	// Verify count (directories have "/" suffix added)
	if len(entries) != 4 {
		t.Errorf("Expected 4 entries, got %d", len(entries))
	}

	// Verify entries contain expected names
	names := make(map[string]bool)
	for _, name := range entries {
		names[name] = true
	}

	// Note: directories have "/" suffix
	expected := []string{"dir1/", "dir2/", "file1.txt", "file2.txt"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("Expected entry '%s' not found", name)
		}
	}
}

func TestOSFileSystem_ReadDir_NonExistent(t *testing.T) {
	fs := NewOSFileSystem()

	// Test: ReadDir on non-existent directory should error
	_, err := fs.ReadDir("/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestOSFileSystem_ReadDir_Empty(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Test: ReadDir on empty directory
	entries, err := fs.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for empty directory, got %d", len(entries))
	}
}

// ============================================================================
// Remove Tests
// ============================================================================

func TestOSFileSystem_Remove(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); errors.Is(err, os.ErrNotExist) {
		t.Fatal("Test file was not created")
	}

	// Test: Remove should delete the file
	err = fs.Remove(testFile)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(testFile); !errors.Is(err, os.ErrNotExist) {
		t.Error("File still exists after Remove")
	}
}

func TestOSFileSystem_Remove_NonExistent(t *testing.T) {
	fs := NewOSFileSystem()

	// Test: Remove on non-existent file should error
	err := fs.Remove("/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestOSFileSystem_Remove_Directory(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create test directory
	testDir := filepath.Join(tempDir, "testdir")
	err := os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test: Remove should work on empty directories
	err = fs.Remove(testDir)
	if err != nil {
		t.Fatalf("Remove failed on directory: %v", err)
	}

	// Verify directory is deleted
	if _, err := os.Stat(testDir); !errors.Is(err, os.ErrNotExist) {
		t.Error("Directory still exists after Remove")
	}
}

// ============================================================================
// ValidateDestPath Tests (Additional Edge Cases)
// ============================================================================

func TestValidateDestPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		// Valid relative paths
		{"simple file", "file.txt", false},
		{"nested path", "dir/subdir/file.txt", false},
		{"dotfile", ".hidden", false},
		{"with spaces", "my documents/file.txt", false},
		{"with dashes", "my-project/src-files/util.go", false},

		// Invalid: absolute paths
		{"absolute unix", "/etc/passwd", true},
		{"absolute windows", "C:\\Windows\\System32", true},
		{"absolute unix root", "/", true},

		// Invalid: parent directory references
		{"parent ref simple", "../file.txt", true},
		{"parent ref nested", "dir/../../etc/passwd", true},
		{"parent ref multiple", "../../../etc/passwd", true},
		{"parent ref in middle", "valid/../../bad/file.txt", true},

		// Edge cases
		{"empty string", "", false}, // filepath.Clean("") becomes "."
		{"dot only", ".", false},    // Current directory is allowed
		{"double dot only", "..", true},
		{"slash only", "/", true},
		{"backslash only", "\\", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)

			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error, got nil", tt.path)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got %v", tt.path, err)
			}
		})
	}
}

func TestValidateDestPath_WindowsAbsolutePaths(t *testing.T) {
	windowsPaths := []string{
		"C:\\",
		"D:\\path\\to\\file",
		"C:/Windows/System32",
		"Z:\\",
	}

	for _, path := range windowsPaths {
		t.Run(path, func(t *testing.T) {
			err := ValidateDestPath(path)
			if err == nil {
				t.Errorf("ValidateDestPath(%q) should reject Windows absolute path", path)
			}
		})
	}
}
