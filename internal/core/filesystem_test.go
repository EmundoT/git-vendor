package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// ============================================================================
// SEC-001: Security Audit — Path Traversal Attack Surface Tests
// ============================================================================
//
// Audit scope: Verify ValidateDestPath blocks all known path traversal vectors.
//
// Findings summary (SEC-001) — updated after fixes:
//
//   PASS — file_copy_service.go:copyMapping calls ValidateDestPath(destFile) at line 66
//          before all CopyFile/CopyDir/PlaceContent operations.
//   PASS — cache_store.go:Save uses sanitizeFilename() which strips path separators.
//   PASS — ValidateDestPath strips position specifiers before validation (../x:L1 caught).
//   FIXED — license_service.go:CopyLicense now calls ValidateVendorName before path construction.
//   FIXED — validation_service.go:ValidateConfig now calls ValidateVendorName at parse time.
//   FIXED — ValidateDestPath now rejects embedded null bytes (defense in depth).
//   A3 — CopyFile/CopyDir now self-validate via ValidateWritePath (root-aware containment).
//        PlaceContent self-validates relative paths via ValidateDestPath.
//        Production uses NewRootedFileSystem(".") which validates all writes resolve within CWD.
//        Unrooted filesystems (NewOSFileSystem) skip validation for back-compat.
//   N/A  — vendor_syncer.go:Init uses hardcoded paths (vendor/, vendor/licenses/).
//   N/A  — hook_service.go executes shell commands by design (same model as npm scripts).
//   N/A  — update_service.go:computeFileHashes is read-only.

// TestValidateDestPath_PositionSpecifierTraversal verifies that position specifiers
// appended to traversal paths do NOT bypass ValidateDestPath.
// Attack vector: "../etc/passwd:L1-L5" should be rejected despite the position suffix.
func TestValidateDestPath_PositionSpecifierTraversal(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		// Traversal with position specifiers — MUST be rejected
		{"traversal with single line", "../etc/passwd:L1", true},
		{"traversal with line range", "../etc/passwd:L1-L5", true},
		{"traversal with line to EOF", "../../etc/shadow:L1-EOF", true},
		{"traversal with column range", "../../../etc/hosts:L1C1:L1C50", true},
		{"deep traversal with position", "a/b/../../../../etc/passwd:L1-L10", true},

		// Valid paths with position specifiers — MUST be accepted
		{"valid path with single line", "src/config.go:L5", false},
		{"valid path with line range", "internal/core/file.go:L10-L20", false},
		{"valid path with EOF", "README.md:L1-EOF", false},
		{"valid path with columns", "main.go:L5C1:L5C80", false},

		// Edge: position specifier on bare traversal
		{"bare dotdot with position", "..:L1", true},
		{"dotdot slash with position", "../:L1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error (path traversal with position specifier), got nil", tt.path)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got: %v", tt.path, err)
			}
		})
	}
}

// TestValidateDestPath_URLEncodedTraversal verifies that URL-encoded path traversal
// sequences are handled. YAML does NOT URL-decode values, so literal "%2e%2e%2f"
// stays as-is and is NOT a traversal. This test documents that behavior.
func TestValidateDestPath_URLEncodedTraversal(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
		note      string
	}{
		// URL-encoded sequences are NOT decoded by YAML parser.
		// "%2e" is literal percent-two-e, NOT a dot.
		// These are safe as-is but tested for documentation.
		{
			name:      "percent-encoded dotdot slash",
			path:      "%2e%2e%2fetc/passwd",
			wantError: false,
			note:      "literal %2e is not decoded — safe, treated as filename",
		},
		{
			name:      "mixed encoding",
			path:      "..%2fetc/passwd",
			wantError: true,
			note:      "starts with .. which filepath.Clean normalizes — still caught",
		},
		{
			name:      "percent-encoded backslash",
			path:      "%2e%2e%5cetc%5cpasswd",
			wantError: false,
			note:      "literal percent sequences — no decoding, safe",
		},
		{
			name:      "double URL encoding",
			path:      "%252e%252e%252fetc/passwd",
			wantError: false,
			note:      "double-encoded — stays literal, safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error, got nil (%s)", tt.path, tt.note)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got: %v (%s)", tt.path, err, tt.note)
			}
		})
	}
}

// TestValidateDestPath_NullByteInjection verifies that ValidateDestPath rejects null bytes
// as defense in depth (the OS also rejects at syscall level).
func TestValidateDestPath_NullByteInjection(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"null before extension", "file.go\x00.txt"},
		{"null truncation attack", "safe.txt\x00../../../etc/passwd"},
		{"null in directory", "dir\x00evil/file.txt"},
		{"null at start", "\x00../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ValidateDestPath MUST reject null bytes directly
			err := ValidateDestPath(tt.path)
			if err == nil {
				t.Errorf("ValidateDestPath(%q) should reject null byte, got nil", tt.path)
			}
			if err != nil && !strings.Contains(err.Error(), "null bytes") {
				t.Errorf("ValidateDestPath(%q) error should mention null bytes, got: %v", tt.path, err)
			}

			// Also verify the OS rejects null bytes (belt and suspenders)
			tempDir := t.TempDir()
			fullPath := filepath.Join(tempDir, tt.path)
			_, osErr := os.Create(fullPath)
			if osErr == nil {
				os.Remove(fullPath)
				t.Errorf("OS allowed file creation with null byte in path %q", tt.path)
			}
		})
	}
}

// TestValidateDestPath_UnicodeNormalization verifies that Unicode characters that
// visually resemble path separators or dots are handled safely.
func TestValidateDestPath_UnicodeNormalization(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
		note      string
	}{
		{
			name:      "fullwidth period",
			path:      "\uff0e\uff0e/etc/passwd",
			wantError: false,
			note:      "fullwidth dots (U+FF0E) are NOT ASCII dots — not a traversal",
		},
		{
			name:      "fullwidth slash",
			path:      "..\uff0fetc/passwd",
			wantError: true,
			note:      "starts with .. — caught regardless of following chars",
		},
		{
			name:      "combining dot below",
			path:      "file\u0323.txt",
			wantError: false,
			note:      "combining characters do not affect path safety",
		},
		{
			name:      "homoglyph slash U+2215",
			path:      "..\u2215etc\u2215passwd",
			wantError: true,
			note:      "starts with .. — caught even with homoglyph separators",
		},
		{
			name:      "fullwidth dot-dot real slash",
			path:      "\uff0e\uff0e/file.txt",
			wantError: false,
			note:      "fullwidth dots are not real dots — safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error, got nil (%s)", tt.path, tt.note)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got: %v (%s)", tt.path, err, tt.note)
			}
		})
	}
}

// TestValidateDestPath_WindowsDriveLetterVariants covers additional Windows drive letter
// edge cases beyond the basic test, including lowercase letters and UNC paths.
func TestValidateDestPath_WindowsDriveLetterVariants(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		// Lowercase drive letters
		{"lowercase c drive", "c:\\Windows\\System32", true},
		{"lowercase d drive", "d:\\path\\to\\file", true},
		{"lowercase c forward slash", "c:/Windows/System32", true},

		// Boundary drive letters
		{"drive letter A", "A:\\", true},
		{"drive letter a", "a:\\", true},
		{"drive letter Z", "Z:\\path", true},
		{"drive letter z", "z:\\path", true},

		// UNC paths (network shares) — start with backslash so caught by prefix check
		{"UNC path", "\\\\server\\share\\file.txt", true},
		{"UNC path forward slash", "//server/share/file.txt", true},

		// NOT drive letters — should be allowed
		{"colon in filename", "file:name.txt", false},
		{"number then colon", "1:file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error, got nil", tt.path)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got: %v", tt.path, err)
			}
		})
	}
}

// TestValidateDestPath_TraversalVariations covers additional path traversal patterns
// including mixed separators, redundant slashes, and deeply nested escapes.
func TestValidateDestPath_TraversalVariations(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		// Mixed separator attacks
		{"backslash traversal", "..\\etc\\passwd", true},
		{"mixed separators", "..\\..\\etc/passwd", true},

		// Redundant separators
		{"double slash traversal", "..//etc/passwd", true},
		{"triple dot attempt", ".../etc/passwd", true}, // "..." starts with ".." after Clean — caught by prefix check

		// Deeply nested cancellation
		{"deep nested escape", "a/b/c/d/../../../../etc/passwd", false}, // resolves to "etc/passwd" — within project
		{"exact cancellation", "a/b/../../file.txt", false}, // resolves to file.txt
		{"one-over cancellation", "a/b/../../../file.txt", true},

		// Trailing dot-dot — resolves to "." via filepath.Clean, which is safe
		{"trailing dotdot", "dir/..", false},
		{"trailing dotdot slash", "dir/../", false},

		// Dot-dot as directory name (should be caught)
		{"dotdot in middle", "a/../b/../c/../../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateDestPath(%q) expected error, got nil", tt.path)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateDestPath(%q) expected no error, got: %v", tt.path, err)
			}
		})
	}
}

// TestCopyFile_FollowsSymlinks verifies that CopyFile follows symlinks on the source side.
// This is expected behavior — the destination is validated by callers, but the source
// content comes from whatever the symlink points to (including files outside the repo).
// SEC-001 note: This is inherent to file copy operations and mitigated by the fact that
// source files come from a git clone (git handles symlinks conservatively).
func TestCopyFile_FollowsSymlinks(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create a real file
	realFile := filepath.Join(tempDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("secret content"), 0644); err != nil {
		t.Fatalf("Failed to create real file: %v", err)
	}

	// Create a symlink to realFile
	symlink := filepath.Join(tempDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	// CopyFile should follow the symlink and copy the real content
	dest := filepath.Join(tempDir, "dest.txt")
	stats, err := fs.CopyFile(symlink, dest)
	if err != nil {
		t.Fatalf("CopyFile through symlink failed: %v", err)
	}

	if stats.FileCount != 1 {
		t.Errorf("Expected FileCount=1, got %d", stats.FileCount)
	}

	// Verify content was copied from the real file
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("Failed to read dest: %v", err)
	}
	if string(data) != "secret content" {
		t.Errorf("Expected 'secret content', got %q", string(data))
	}
}

// TestCopyDir_SkipsGitDirectories verifies that CopyDir skips .git directories,
// which prevents leaking git metadata during vendor copy operations.
func TestCopyDir_SkipsGitDirectories(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create source directory with .git
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644)

	// Copy directory
	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)
	stats, err := fs.CopyDir(srcDir, destDir)
	if err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Should have copied file.txt but NOT .git contents
	if stats.FileCount != 1 {
		t.Errorf("Expected 1 file copied (skipping .git), got %d", stats.FileCount)
	}

	// Verify .git was not copied
	if _, err := os.Stat(filepath.Join(destDir, ".git", "HEAD")); !os.IsNotExist(err) {
		t.Error(".git directory should not be copied")
	}

	// Verify regular file was copied
	data, err := os.ReadFile(filepath.Join(destDir, "file.txt"))
	if err != nil {
		t.Fatalf("file.txt not copied: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("Expected 'content', got %q", string(data))
	}
}

// TestValidateDestPath_LicensePathTraversalVector documents the SEC-001 FAIL finding.
// license_service.go:CopyLicense constructs destination as:
//
//	filepath.Join(rootDir, "licenses", vendorName+".txt")
//
// If vendorName contains path traversal sequences, the resulting path escapes
// the vendor/licenses/ directory. ValidateDestPath is NOT called on the license path.
//
// ValidateDestPath WOULD catch these paths if called on the constructed license path.
func TestValidateDestPath_LicensePathTraversalVector(t *testing.T) {
	// These are vendorName values that, when used in
	// filepath.Join("vendor", "licenses", vendorName+".txt"),
	// produce a path that escapes the project directory.
	maliciousNames := []struct {
		name         string
		vendorName   string
		expectedPath string
	}{
		{
			name:         "simple traversal",
			vendorName:   "../../tmp/evil",
			expectedPath: "tmp/evil.txt",
		},
		{
			name:         "deep traversal",
			vendorName:   "../../../etc/cron.d/evil",
			expectedPath: "../etc/cron.d/evil.txt",
		},
		{
			name:         "escape to project root",
			vendorName:   "../../malicious",
			expectedPath: "malicious.txt",
		},
	}

	for _, tt := range maliciousNames {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what license_service.go:CopyLicense does
			dest := filepath.Join("vendor", "licenses", tt.vendorName+".txt")
			cleaned := filepath.Clean(dest)

			// Show the resulting path escapes vendor/licenses/
			t.Logf("vendorName=%q → dest=%q → cleaned=%q", tt.vendorName, dest, cleaned)

			// ValidateDestPath WOULD catch the cleaned path if called
			err := ValidateDestPath(cleaned)
			if err == nil {
				// If ValidateDestPath accepts the path, the path may still be within
				// the project but outside vendor/licenses/. Log for audit review.
				t.Logf("AUDIT: ValidateDestPath accepted %q — path stays within project but escapes vendor/licenses/", cleaned)
			}
			// Note: we don't assert error here because some paths normalize to within-project
			// but the key finding is that CopyLicense never calls ValidateDestPath at all.
		})
	}
}

// TestValidateDestPath_CacheStoreSanitization verifies that the cache store's
// sanitizeFilename function prevents path traversal in cache file paths.
// This confirms the PASS finding for cache_store.go:Save.
func TestValidateDestPath_CacheStoreSanitization(t *testing.T) {
	maliciousInputs := []struct {
		name  string
		input string
	}{
		{"traversal slashes", "../../etc/passwd"},
		{"backslash traversal", "..\\..\\etc\\passwd"},
		{"slash in vendor name", "evil/../../escape"},
		{"null byte", "evil\x00../passwd"},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeFilename(tt.input)

			// Verify no path separators survive sanitization
			if filepath.Base(sanitized) != sanitized {
				t.Errorf("sanitizeFilename(%q) = %q — still contains path separators", tt.input, sanitized)
			}

			// Verify no ".." sequences survive
			if filepath.Clean(sanitized) != sanitized && sanitized != "" {
				// filepath.Clean might differ due to double dots, check explicitly
				if len(sanitized) >= 2 && sanitized[0] == '.' && sanitized[1] == '.' {
					t.Errorf("sanitizeFilename(%q) = %q — starts with '..'", tt.input, sanitized)
				}
			}
		})
	}
}

// ============================================================================
// SEC-001 Fix Verification: ValidateVendorName
// ============================================================================

// TestValidateVendorName verifies the new ValidateVendorName function rejects
// names containing path traversal sequences, separators, or null bytes.
func TestValidateVendorName(t *testing.T) {
	tests := []struct {
		name      string
		vendor    string
		wantError bool
		errSubstr string
	}{
		// Valid names
		{"simple name", "my-lib", false, ""},
		{"name with dots", "lib.v2", false, ""},
		{"name with spaces", "my lib", false, ""},
		{"name with hyphens and numbers", "react-18", false, ""},

		// Invalid: path separators
		{"forward slash", "evil/lib", true, "path separators"},
		{"backslash", "evil\\lib", true, "path separators"},
		{"traversal via slash", "../../../etc/evil", true, "path separators"},
		{"deeply nested traversal", "../../tmp/evil", true, "path separators"},

		// Invalid: dotdot without slashes (still dangerous in filepath.Join)
		{"bare dotdot", "..", true, "path traversal"},
		{"dotdot prefix", "..name", true, "path traversal"},

		// Invalid: null bytes
		{"null byte", "evil\x00lib", true, "null bytes"},

		// Invalid: empty
		{"empty string", "", true, "must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVendorName(tt.vendor)
			if tt.wantError && err == nil {
				t.Errorf("ValidateVendorName(%q) expected error, got nil", tt.vendor)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateVendorName(%q) expected no error, got: %v", tt.vendor, err)
			}
			if tt.wantError && err != nil && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("ValidateVendorName(%q) error should contain %q, got: %v", tt.vendor, tt.errSubstr, err)
				}
			}
		})
	}
}

// TestValidateVendorName_LicensePathSafety verifies that ValidateVendorName
// blocks the exact attack vector from SEC-001: vendor names that would cause
// license_service.go:CopyLicense to write outside vendor/licenses/.
func TestValidateVendorName_LicensePathSafety(t *testing.T) {
	attackVectors := []string{
		"../../tmp/evil",
		"../../../etc/cron.d/evil",
		"../../malicious",
		"../escape",
		"subdir/../../escape",
	}

	for _, name := range attackVectors {
		t.Run(name, func(t *testing.T) {
			err := ValidateVendorName(name)
			if err == nil {
				// Show the path that would have been constructed
				dest := filepath.Join("vendor", "licenses", name+".txt")
				t.Errorf("ValidateVendorName(%q) should reject — would produce license path %q", name, filepath.Clean(dest))
			}
		})
	}
}

// ============================================================================
// A3: Self-Validating Write Functions (Root-Aware FileSystem)
// ============================================================================

// TestNewRootedFileSystem_ValidateWritePath verifies that a rooted filesystem
// rejects writes outside the project root and accepts writes within the root.
func TestNewRootedFileSystem_ValidateWritePath(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewRootedFileSystem(tempDir)

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		// Paths within root — accepted
		{"file in root", filepath.Join(tempDir, "file.txt"), false},
		{"nested in root", filepath.Join(tempDir, "sub", "dir", "file.txt"), false},
		{"root itself", tempDir, false},

		// Paths outside root — rejected
		{"parent of root", filepath.Dir(tempDir), true},
		{"sibling of root", filepath.Join(filepath.Dir(tempDir), "sibling"), true},
		{"absolute elsewhere", "/etc/passwd", true},
		{"traversal from root", filepath.Join(tempDir, "..", "escape"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fs.ValidateWritePath(tt.path)
			if tt.wantError && err == nil {
				t.Errorf("ValidateWritePath(%q) expected error for rooted fs (root=%q)", tt.path, tempDir)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateWritePath(%q) unexpected error: %v (root=%q)", tt.path, err, tempDir)
			}
		})
	}
}

// TestNewOSFileSystem_ValidateWritePath_Unrestricted verifies that an unrooted
// filesystem's ValidateWritePath always returns nil (no restriction).
func TestNewOSFileSystem_ValidateWritePath_Unrestricted(t *testing.T) {
	fs := NewOSFileSystem()

	paths := []string{
		"/etc/passwd",
		"../../../escape",
		"/tmp/anything",
		"relative/path",
		".",
	}

	for _, path := range paths {
		if err := fs.ValidateWritePath(path); err != nil {
			t.Errorf("Unrooted ValidateWritePath(%q) should return nil, got: %v", path, err)
		}
	}
}

// TestRootedFileSystem_CopyFile_BlocksEscape verifies that CopyFile on a rooted
// filesystem rejects destinations outside the project root.
func TestRootedFileSystem_CopyFile_BlocksEscape(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewRootedFileSystem(tempDir)

	// Create a source file inside the root
	srcFile := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Attempt to copy to a destination outside the root
	escapeDst := filepath.Join(filepath.Dir(tempDir), "escaped.txt")
	_, err := fs.CopyFile(srcFile, escapeDst)
	if err == nil {
		os.Remove(escapeDst) // cleanup in case escapeDst was created
		t.Fatal("CopyFile should block write outside project root")
	}
	if !strings.Contains(err.Error(), "write blocked") {
		t.Errorf("Error should mention 'write blocked', got: %v", err)
	}

	// Verify the file was NOT created
	if _, statErr := os.Stat(escapeDst); statErr == nil {
		os.Remove(escapeDst)
		t.Error("Escaped file should not exist")
	}
}

// TestRootedFileSystem_CopyFile_AllowsWithinRoot verifies that CopyFile on a rooted
// filesystem allows writes within the project root.
func TestRootedFileSystem_CopyFile_AllowsWithinRoot(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewRootedFileSystem(tempDir)

	srcFile := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tempDir, "dst.txt")
	stats, err := fs.CopyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("CopyFile within root should succeed: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("Expected FileCount=1, got %d", stats.FileCount)
	}
}

// TestRootedFileSystem_CopyDir_BlocksEscape verifies that CopyDir on a rooted
// filesystem rejects destinations outside the project root.
func TestRootedFileSystem_CopyDir_BlocksEscape(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewRootedFileSystem(tempDir)

	// Create source directory
	srcDir := filepath.Join(tempDir, "srcdir")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("data"), 0644)

	// Attempt to copy to a destination outside the root
	escapeDst := filepath.Join(filepath.Dir(tempDir), "escaped_dir")
	_, err := fs.CopyDir(srcDir, escapeDst)
	if err == nil {
		os.RemoveAll(escapeDst)
		t.Fatal("CopyDir should block write outside project root")
	}
	if !strings.Contains(err.Error(), "write blocked") {
		t.Errorf("Error should mention 'write blocked', got: %v", err)
	}
}

// TestRootedFileSystem_PrefixCollision verifies that root containment check doesn't
// allow prefix collisions (e.g., /tmp/foo should not allow /tmp/foobar).
func TestRootedFileSystem_PrefixCollision(t *testing.T) {
	tempDir := t.TempDir() // e.g., /tmp/TestXXX123
	fs := NewRootedFileSystem(tempDir)

	// Create a sibling directory that shares a prefix with tempDir
	siblingDir := tempDir + "sibling"
	os.MkdirAll(siblingDir, 0755)
	defer os.RemoveAll(siblingDir)

	escapeDst := filepath.Join(siblingDir, "file.txt")
	err := fs.ValidateWritePath(escapeDst)
	if err == nil {
		t.Errorf("ValidateWritePath should reject prefix collision path %q (root=%q)", escapeDst, tempDir)
	}
}

// TestPlaceContent_SelfValidation_RejectsRelativeTraversal verifies that PlaceContent
// self-validates relative paths and rejects traversal attacks.
func TestPlaceContent_SelfValidation_RejectsRelativeTraversal(t *testing.T) {
	traversalPaths := []string{
		"../../../etc/passwd",
		"../escape.txt",
		"a/b/../../../escape.txt",
	}

	for _, path := range traversalPaths {
		err := PlaceContent(path, "malicious content", nil)
		if err == nil {
			t.Errorf("PlaceContent(%q) should reject relative traversal path", path)
		}
		if err != nil && !strings.Contains(err.Error(), "write blocked") {
			t.Errorf("PlaceContent(%q) error should mention 'write blocked', got: %v", path, err)
		}
	}
}

// TestPlaceContent_SelfValidation_AllowsAbsolutePaths verifies that PlaceContent
// skips validation for absolute paths (used by tests with temp dirs).
func TestPlaceContent_SelfValidation_AllowsAbsolutePaths(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "target.go")

	err := PlaceContent(filePath, "valid content", nil)
	if err != nil {
		t.Fatalf("PlaceContent with absolute temp path should succeed: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	if string(data) != "valid content" {
		t.Errorf("Content mismatch: got %q", string(data))
	}
}

// TestPlaceContent_SelfValidation_AllowsSafeRelativePaths verifies that PlaceContent
// allows valid relative paths (the production use case).
func TestPlaceContent_SelfValidation_AllowsSafeRelativePaths(t *testing.T) {
	// Create a temp dir and chdir into the temp dir so relative paths resolve there
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	os.MkdirAll("src/lib", 0755)
	err := PlaceContent("src/lib/file.go", "package lib", nil)
	if err != nil {
		t.Fatalf("PlaceContent with safe relative path should succeed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tempDir, "src/lib/file.go"))
	if string(data) != "package lib" {
		t.Errorf("Content mismatch: got %q", string(data))
	}
}

// TestRootedFileSystem_RelativePaths verifies that rooted filesystem handles
// relative paths correctly by resolving them against CWD.
func TestRootedFileSystem_RelativePaths(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	fs := NewRootedFileSystem(".")

	// Relative path within CWD — should be accepted
	err := fs.ValidateWritePath("vendor/file.txt")
	if err != nil {
		t.Errorf("Relative path within root should be accepted: %v", err)
	}

	// Relative traversal path — should be rejected
	err = fs.ValidateWritePath("../../escape.txt")
	if err == nil {
		t.Error("Relative traversal path should be rejected by rooted filesystem")
	}
}

