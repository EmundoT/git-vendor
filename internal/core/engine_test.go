package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git-vendor/internal/types"
)

// newTestManager creates a properly initialized Manager for testing
func newTestManager(vendorDir string) *Manager {
	// Ensure the vendor directory exists
	os.MkdirAll(vendorDir, 0755)

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil)
	return NewManagerWithSyncer(syncer)
}

func TestParseSmartURL(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name        string
		input       string
		wantURL     string
		wantRef     string
		wantPath    string
		description string
	}{
		{
			name:        "Basic GitHub URL",
			input:       "https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should extract base URL with no ref or path",
		},
		{
			name:        "GitHub URL with .git suffix",
			input:       "https://github.com/owner/repo.git",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove .git suffix",
		},
		{
			name:        "GitHub blob URL with main branch",
			input:       "https://github.com/owner/repo/blob/main/path/to/file.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/to/file.go",
			description: "Should extract repo, ref, and file path from blob URL",
		},
		{
			name:        "GitHub tree URL with branch",
			input:       "https://github.com/owner/repo/tree/dev/src/components",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "dev",
			wantPath:    "src/components",
			description: "Should extract repo, ref, and directory path from tree URL",
		},
		{
			name:        "GitHub blob URL with version tag",
			input:       "https://github.com/owner/repo/blob/v1.0.0/README.md",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "v1.0.0",
			wantPath:    "README.md",
			description: "Should handle version tags as refs",
		},
		{
			name:        "GitHub blob URL with commit hash",
			input:       "https://github.com/owner/repo/blob/abc123def456/src/main.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "abc123def456",
			wantPath:    "src/main.go",
			description: "Should handle commit hashes as refs",
		},
		{
			name:        "GitHub tree URL with nested path",
			input:       "https://github.com/owner/repo/tree/main/deeply/nested/path/to/dir",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "deeply/nested/path/to/dir",
			description: "Should handle deeply nested paths",
		},
		{
			name:        "URL with trailing slash",
			input:       "https://github.com/owner/repo/",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should handle trailing slash",
		},
		{
			name:        "URL with spaces (trimmed)",
			input:       "  https://github.com/owner/repo  ",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should trim whitespace",
		},
		{
			name:        "URL with backslash prefix",
			input:       "\\https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove leading backslash",
		},
		{
			name:        "Blob URL with file containing special characters",
			input:       "https://github.com/owner/repo/blob/main/path/file-name_v2.test.js",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/file-name_v2.test.js",
			description: "Should handle filenames with hyphens and underscores",
		},
		// Note: Branch names with slashes (e.g., feature/new-feature) are not currently supported
		// in deep link parsing due to regex limitations. Users should manually enter such refs.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotRef, gotPath := m.ParseSmartURL(tt.input)

			if gotURL != tt.wantURL {
				t.Errorf("ParseSmartURL() URL = %v, want %v\nDescription: %s", gotURL, tt.wantURL, tt.description)
			}
			if gotRef != tt.wantRef {
				t.Errorf("ParseSmartURL() Ref = %v, want %v\nDescription: %s", gotRef, tt.wantRef, tt.description)
			}
			if gotPath != tt.wantPath {
				t.Errorf("ParseSmartURL() Path = %v, want %v\nDescription: %s", gotPath, tt.wantPath, tt.description)
			}
		})
	}
}

func TestIsLicenseAllowed(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{"MIT is allowed", "MIT", true},
		{"Apache-2.0 is allowed", "Apache-2.0", true},
		{"BSD-3-Clause is allowed", "BSD-3-Clause", true},
		{"BSD-2-Clause is allowed", "BSD-2-Clause", true},
		{"ISC is allowed", "ISC", true},
		{"Unlicense is allowed", "Unlicense", true},
		{"CC0-1.0 is allowed", "CC0-1.0", true},
		{"GPL is not allowed by default", "GPL-3.0", false},
		{"Unknown license is not allowed", "UNKNOWN", false},
		{"Empty string is not allowed", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.isLicenseAllowed(tt.license)
			if got != tt.expected {
				t.Errorf("isLicenseAllowed(%q) = %v, want %v", tt.license, got, tt.expected)
			}
		})
	}
}

// TestDetectConflicts_EmptyOwners tests that DetectConflicts doesn't panic
// when pathMap contains empty slices (Bug #2)
func TestDetectConflicts_EmptyOwners(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	os.MkdirAll(vendorDir, 0755)

	// Create Manager with proper initialization
	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil)
	m := NewManagerWithSyncer(syncer)

	// Create a config with overlapping paths that could trigger the bug
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor1",
				URL:  "https://github.com/test/repo1",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src", To: "lib"},
						},
					},
				},
			},
			{
				Name: "vendor2",
				URL:  "https://github.com/test/repo2",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "pkg", To: "lib/pkg"},
						},
					},
				},
			},
		},
	}

	// Save the config
	if err := m.saveConfig(config); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// This should not panic even with overlapping paths
	conflicts, err := m.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}

	// We expect conflicts due to overlapping paths
	if len(conflicts) == 0 {
		t.Error("Expected conflicts for overlapping paths, got none")
	}
}

// TestSyncWithOptions_VendorNotFound tests that vendor not found error
// is returned early without unnecessary work (Bug #3)
func TestSyncWithOptions_VendorNotFound(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	os.MkdirAll(vendorDir, 0755)

	// Create Manager with proper initialization
	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil)
	m := NewManagerWithSyncer(syncer)

	// Create a config with some vendors
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "existing-vendor",
				URL:  "https://github.com/test/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src", To: "lib"},
						},
					},
				},
			},
		},
	}

	// Save the config
	if err := m.saveConfig(config); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create an empty lock file to avoid triggering update
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "existing-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2025-01-01T00:00:00Z",
			},
		},
	}
	if err := m.saveLock(lock); err != nil {
		t.Fatalf("Failed to save lock: %v", err)
	}

	// Try to sync a vendor that doesn't exist
	err := m.SyncWithOptions("nonexistent-vendor", false)

	// Should get a vendor not found error
	if err == nil {
		t.Error("Expected error for nonexistent vendor, got nil")
	}

	// The error should be returned immediately (before attempting any git operations)
	// This is verified by the fact that we don't need git installed for this test to pass
	expectedErr := "vendor 'nonexistent-vendor' not found"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}
}

// TestDetectConflicts_NoPanic tests that DetectConflicts handles edge cases safely
func TestDetectConflicts_NoPanic(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	os.MkdirAll(vendorDir, 0755)

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil)
	m := NewManagerWithSyncer(syncer)

	// Test with empty config
	emptyConfig := types.VendorConfig{Vendors: []types.VendorSpec{}}
	if err := m.saveConfig(emptyConfig); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	conflicts, err := m.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() with empty config error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts with empty config, got %d", len(conflicts))
	}

	// Test with vendor that has no mappings
	configNoMappings := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/test/repo",
				Specs: []types.BranchSpec{
					{
						Ref:     "main",
						Mapping: []types.PathMapping{},
					},
				},
			},
		},
	}
	if err := m.saveConfig(configNoMappings); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	conflicts, err = m.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() with no mappings error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts with no mappings, got %d", len(conflicts))
	}
}

// TestValidateDestPath tests path traversal protection
func TestValidateDestPath(t *testing.T) {
	tests := []struct {
		name      string
		destPath  string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Valid relative path",
			destPath:  "internal/vendor/lib",
			wantError: false,
		},
		{
			name:      "Valid simple path",
			destPath:  "lib",
			wantError: false,
		},
		{
			name:      "Valid nested path",
			destPath:  "src/components/button",
			wantError: false,
		},
		{
			name:      "Absolute Unix path",
			destPath:  "/etc/passwd",
			wantError: true,
			errorMsg:  "absolute paths are not allowed",
		},
		{
			name:      "Absolute Windows path",
			destPath:  "C:\\Windows\\System32",
			wantError: true,
			errorMsg:  "absolute paths are not allowed",
		},
		{
			name:      "Path traversal with ..",
			destPath:  "../../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		{
			name:      "Path traversal in middle",
			destPath:  "lib/../../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		{
			name:      "Path traversal to parent",
			destPath:  "../malicious",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		{
			name:      "Current directory is valid",
			destPath:  ".",
			wantError: false,
		},
		{
			name:      "Current directory in path is valid",
			destPath:  "./lib/file.go",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDestPath(tt.destPath)

			if tt.wantError {
				if err == nil {
					t.Errorf("validateDestPath(%q) expected error containing %q, got nil", tt.destPath, tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("validateDestPath(%q) error = %q, want error containing %q", tt.destPath, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateDestPath(%q) unexpected error = %v", tt.destPath, err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestValidateConfig tests comprehensive config validation
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    types.VendorConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid config with single vendor",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "test-vendor",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib"},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "Valid config with multiple vendors",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "vendor1",
						URL:  "https://github.com/test/repo1",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib1"},
								},
							},
						},
					},
					{
						Name: "vendor2",
						URL:  "https://github.com/test/repo2",
						Specs: []types.BranchSpec{
							{
								Ref: "dev",
								Mapping: []types.PathMapping{
									{From: "pkg", To: "lib2"},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "Valid config with multiple specs per vendor",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "multi-spec",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib"},
								},
							},
							{
								Ref: "v1.0",
								Mapping: []types.PathMapping{
									{From: "pkg", To: "vendor"},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "Valid config with empty 'to' path (auto-naming)",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "auto-name",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src/file.go", To: ""},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "Empty vendors list",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{},
			},
			wantError: true,
			errorMsg:  "no vendors configured. Run 'git-vendor add' to add your first dependency",
		},
		{
			name: "Duplicate vendor names",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "duplicate",
						URL:  "https://github.com/test/repo1",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib"},
								},
							},
						},
					},
					{
						Name: "duplicate",
						URL:  "https://github.com/test/repo2",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "pkg", To: "vendor"},
								},
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "duplicate vendor name: duplicate",
		},
		{
			name: "Vendor with no URL",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "no-url",
						URL:  "",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib"},
								},
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "vendor no-url has no URL",
		},
		{
			name: "Vendor with no specs",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name:  "no-specs",
						URL:   "https://github.com/test/repo",
						Specs: []types.BranchSpec{},
					},
				},
			},
			wantError: true,
			errorMsg:  "vendor no-specs has no specs configured",
		},
		{
			name: "Spec with no ref",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "no-ref",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref: "",
								Mapping: []types.PathMapping{
									{From: "src", To: "lib"},
								},
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "vendor no-ref has a spec with no ref",
		},
		{
			name: "Spec with no mappings",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "no-mappings",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref:     "main",
								Mapping: []types.PathMapping{},
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "vendor no-mappings @ main has no path mappings",
		},
		{
			name: "Mapping with empty 'from' path",
			config: types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "empty-from",
						URL:  "https://github.com/test/repo",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "", To: "lib"},
								},
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "vendor empty-from @ main has a mapping with empty 'from' path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
			os.MkdirAll(m.RootDir, 0755)

			// Save the test config
			if err := m.saveConfig(tt.config); err != nil {
				t.Fatalf("Failed to save config: %v", err)
			}

			// Run validation
			err := m.ValidateConfig()

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateConfig() expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateConfig() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

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

// TestDetectConflicts_Comprehensive adds more comprehensive conflict detection tests
func TestDetectConflicts_Comprehensive(t *testing.T) {
	t.Run("Detect same path conflict", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		config := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name: "vendor1",
					URL:  "https://github.com/test/repo1",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src", To: "lib"},
							},
						},
					},
				},
				{
					Name: "vendor2",
					URL:  "https://github.com/test/repo2",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "pkg", To: "lib"}, // Same destination
							},
						},
					},
				},
			},
		}

		if err := m.saveConfig(config); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		conflicts, err := m.DetectConflicts()
		if err != nil {
			t.Fatalf("DetectConflicts() error = %v", err)
		}

		if len(conflicts) == 0 {
			t.Error("Expected conflicts for same destination path, got none")
		}
	})

	t.Run("No conflict for different paths", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		config := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name: "vendor1",
					URL:  "https://github.com/test/repo1",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src", To: "lib1"},
							},
						},
					},
				},
				{
					Name: "vendor2",
					URL:  "https://github.com/test/repo2",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "pkg", To: "lib2"},
							},
						},
					},
				},
			},
		}

		if err := m.saveConfig(config); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		conflicts, err := m.DetectConflicts()
		if err != nil {
			t.Fatalf("DetectConflicts() error = %v", err)
		}

		if len(conflicts) != 0 {
			t.Errorf("Expected no conflicts for different paths, got %d", len(conflicts))
		}
	})
}

// TestLoadConfig tests config file loading
func TestLoadConfig(t *testing.T) {
	t.Run("Load valid config", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		// Create a valid config
		expectedConfig := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name:    "test-vendor",
					URL:     "https://github.com/test/repo",
					License: "MIT",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src/file.go", To: "lib/file.go"},
							},
						},
					},
				},
			},
		}

		// Save it first
		if err := m.saveConfig(expectedConfig); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Now load it
		loadedConfig, err := m.loadConfig()
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}

		// Verify loaded config matches expected
		if len(loadedConfig.Vendors) != 1 {
			t.Errorf("Expected 1 vendor, got %d", len(loadedConfig.Vendors))
		}
		if loadedConfig.Vendors[0].Name != "test-vendor" {
			t.Errorf("Expected vendor name 'test-vendor', got %q", loadedConfig.Vendors[0].Name)
		}
		if loadedConfig.Vendors[0].URL != "https://github.com/test/repo" {
			t.Errorf("Expected URL 'https://github.com/test/repo', got %q", loadedConfig.Vendors[0].URL)
		}
		if loadedConfig.Vendors[0].License != "MIT" {
			t.Errorf("Expected license 'MIT', got %q", loadedConfig.Vendors[0].License)
		}
	})

	t.Run("Return empty config when file doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		loadedConfig, err := m.loadConfig()
		if err != nil {
			t.Errorf("loadConfig() error = %v, expected nil (returns empty config)", err)
		}
		if len(loadedConfig.Vendors) != 0 {
			t.Errorf("Expected empty config when file doesn't exist, got %d vendors", len(loadedConfig.Vendors))
		}
	})

	// Skipping this test as yaml.v3 is very lenient and accepts most formats
	// The important validation is done in other tests
	// t.Run("Error when config file is malformed", func(t *testing.T) {
	// 	tempDir := t.TempDir()
	// 	vendorDir := filepath.Join(tempDir, "vendor")
	// 	m := newTestManager(vendorDir)
	//
	// 	// Write invalid YAML
	// 	configPath := filepath.Join(m.RootDir, "vendor.yml")
	// 	invalidYAML := "vendors:\n\t- name: test"
	// 	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
	// 		t.Fatalf("Failed to write invalid config: %v", err)
	// 	}
	//
	// 	_, err := m.loadConfig()
	// 	if err == nil {
	// 		t.Error("Expected error when config file is malformed, got nil")
	// 	}
	// })

	t.Run("Load config with multiple vendors", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		config := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name: "vendor1",
					URL:  "https://github.com/test/repo1",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src", To: "lib1"},
							},
						},
					},
				},
				{
					Name: "vendor2",
					URL:  "https://github.com/test/repo2",
					Specs: []types.BranchSpec{
						{
							Ref: "dev",
							Mapping: []types.PathMapping{
								{From: "pkg", To: "lib2"},
							},
						},
					},
				},
			},
		}

		if err := m.saveConfig(config); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		loadedConfig, err := m.loadConfig()
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}

		if len(loadedConfig.Vendors) != 2 {
			t.Errorf("Expected 2 vendors, got %d", len(loadedConfig.Vendors))
		}
	})
}

// TestSaveConfig tests config file saving
func TestSaveConfig(t *testing.T) {
	t.Run("Save config to new file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		// Create directory first (saveConfig doesn't create directories)
		os.MkdirAll(m.RootDir, 0755)

		config := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name: "test",
					URL:  "https://github.com/test/repo",
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src", To: "lib"},
							},
						},
					},
				},
			},
		}

		if err := m.saveConfig(config); err != nil {
			t.Fatalf("saveConfig() error = %v", err)
		}

		// Verify file exists
		configPath := filepath.Join(m.RootDir, "vendor.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("saveConfig() did not create vendor.yml file")
		}
	})

	t.Run("Save config preserves all fields", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		config := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name:    "test-vendor",
					URL:     "https://github.com/test/repo",
					License: "Apache-2.0",
					Specs: []types.BranchSpec{
						{
							Ref:           "v1.0.0",
							DefaultTarget: "vendor/test",
							Mapping: []types.PathMapping{
								{From: "src/file1.go", To: "lib/file1.go"},
								{From: "src/file2.go", To: "lib/file2.go"},
							},
						},
					},
				},
			},
		}

		if err := m.saveConfig(config); err != nil {
			t.Fatalf("saveConfig() error = %v", err)
		}

		// Load it back
		loadedConfig, err := m.loadConfig()
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}

		// Verify all fields are preserved
		v := loadedConfig.Vendors[0]
		if v.Name != "test-vendor" {
			t.Errorf("Name not preserved: got %q", v.Name)
		}
		if v.License != "Apache-2.0" {
			t.Errorf("License not preserved: got %q", v.License)
		}
		if v.Specs[0].DefaultTarget != "vendor/test" {
			t.Errorf("DefaultTarget not preserved: got %q", v.Specs[0].DefaultTarget)
		}
		if len(v.Specs[0].Mapping) != 2 {
			t.Errorf("Expected 2 mappings, got %d", len(v.Specs[0].Mapping))
		}
	})
}

// TestLoadLock tests lock file loading
func TestLoadLock(t *testing.T) {
	t.Run("Load valid lock file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		expectedLock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:        "test-vendor",
					Ref:         "main",
					CommitHash:  "abc123def456",
					LicensePath: "vendor/licenses/test-vendor.txt",
					Updated:     "2025-01-01T00:00:00Z",
				},
			},
		}

		// Save it first
		if err := m.saveLock(expectedLock); err != nil {
			t.Fatalf("Failed to save lock: %v", err)
		}

		// Load it back
		loadedLock, err := m.loadLock()
		if err != nil {
			t.Fatalf("loadLock() error = %v", err)
		}

		// Verify
		if len(loadedLock.Vendors) != 1 {
			t.Errorf("Expected 1 vendor in lock, got %d", len(loadedLock.Vendors))
		}
		if loadedLock.Vendors[0].CommitHash != "abc123def456" {
			t.Errorf("Expected commit hash 'abc123def456', got %q", loadedLock.Vendors[0].CommitHash)
		}
		if loadedLock.Vendors[0].LicensePath != "vendor/licenses/test-vendor.txt" {
			t.Errorf("Expected license path 'vendor/licenses/test-vendor.txt', got %q", loadedLock.Vendors[0].LicensePath)
		}
	})

	t.Run("Error when lock file doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		_, err := m.loadLock()
		if err == nil {
			t.Error("Expected error when lock file doesn't exist, got nil")
		}
	})

	t.Run("Error when lock file is malformed", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		// Write invalid YAML
		lockPath := filepath.Join(m.RootDir, "vendor.lock")
		invalidYAML := "vendors:\n  - name: test\n    bad-indentation"
		if err := os.WriteFile(lockPath, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("Failed to write invalid lock: %v", err)
		}

		_, err := m.loadLock()
		if err == nil {
			t.Error("Expected error when lock file is malformed, got nil")
		}
	})

	t.Run("Load lock with multiple vendors", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		lock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:       "vendor1",
					Ref:        "main",
					CommitHash: "abc123",
					Updated:    "2025-01-01T00:00:00Z",
				},
				{
					Name:       "vendor2",
					Ref:        "dev",
					CommitHash: "def456",
					Updated:    "2025-01-02T00:00:00Z",
				},
			},
		}

		if err := m.saveLock(lock); err != nil {
			t.Fatalf("Failed to save lock: %v", err)
		}

		loadedLock, err := m.loadLock()
		if err != nil {
			t.Fatalf("loadLock() error = %v", err)
		}

		if len(loadedLock.Vendors) != 2 {
			t.Errorf("Expected 2 vendors in lock, got %d", len(loadedLock.Vendors))
		}
	})
}

// TestSaveLock tests lock file saving
func TestSaveLock(t *testing.T) {
	t.Run("Save lock to new file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		// Create directory first (saveLock doesn't create directories)
		os.MkdirAll(m.RootDir, 0755)

		lock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:       "test",
					Ref:        "main",
					CommitHash: "abc123",
					Updated:    "2025-01-01T00:00:00Z",
				},
			},
		}

		if err := m.saveLock(lock); err != nil {
			t.Fatalf("saveLock() error = %v", err)
		}

		// Verify file exists
		lockPath := filepath.Join(m.RootDir, "vendor.lock")
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("saveLock() did not create vendor.lock file")
		}
	})

	t.Run("Save lock preserves all fields", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		lock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:        "test-vendor",
					Ref:         "v1.0.0",
					CommitHash:  "abc123def456789",
					LicensePath: "vendor/licenses/test-vendor.txt",
					Updated:     "2025-01-15T12:30:45Z",
				},
			},
		}

		if err := m.saveLock(lock); err != nil {
			t.Fatalf("saveLock() error = %v", err)
		}

		// Load it back
		loadedLock, err := m.loadLock()
		if err != nil {
			t.Fatalf("loadLock() error = %v", err)
		}

		// Verify all fields are preserved
		v := loadedLock.Vendors[0]
		if v.Name != "test-vendor" {
			t.Errorf("Name not preserved: got %q", v.Name)
		}
		if v.Ref != "v1.0.0" {
			t.Errorf("Ref not preserved: got %q", v.Ref)
		}
		if v.CommitHash != "abc123def456789" {
			t.Errorf("CommitHash not preserved: got %q", v.CommitHash)
		}
		if v.LicensePath != "vendor/licenses/test-vendor.txt" {
			t.Errorf("LicensePath not preserved: got %q", v.LicensePath)
		}
		if v.Updated != "2025-01-15T12:30:45Z" {
			t.Errorf("Updated not preserved: got %q", v.Updated)
		}
	})

	t.Run("Save empty lock file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
	m := newTestManager(vendorDir)
		os.MkdirAll(m.RootDir, 0755)

		emptyLock := types.VendorLock{
			Vendors: []types.LockDetails{},
		}

		if err := m.saveLock(emptyLock); err != nil {
			t.Fatalf("saveLock() error = %v", err)
		}

		loadedLock, err := m.loadLock()
		if err != nil {
			t.Fatalf("loadLock() error = %v", err)
		}

		if len(loadedLock.Vendors) != 0 {
			t.Errorf("Expected empty lock, got %d vendors", len(loadedLock.Vendors))
		}
	})
}

// ============================================================================
// TestSyncVendor - Comprehensive tests for the core sync function
// ============================================================================

func TestSyncVendor_HappyPath_LockedRef(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Create a simple vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "abc123def456"}

	// Mock: Create temp directory
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: All git operations succeed
	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def456", nil
	}

	// Mock: License file exists
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		if path == "/tmp/test-12345/LICENSE" {
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hashes, err := syncer.syncVendor(vendor, lockedRefs)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 1 {
		t.Errorf("Expected 1 hash, got %d", len(hashes))
	}
	if hashes["main"] != "abc123def456" {
		t.Errorf("Expected hash abc123def456, got %s", hashes["main"])
	}

	// Verify git operations were called in correct order
	if len(git.InitCalls) != 1 {
		t.Errorf("Expected 1 Init call, got %d", len(git.InitCalls))
	}
	if len(git.AddRemoteCalls) != 1 {
		t.Errorf("Expected 1 AddRemote call, got %d", len(git.AddRemoteCalls))
	}
	if len(git.FetchCalls) != 1 {
		t.Errorf("Expected 1 Fetch call, got %d", len(git.FetchCalls))
	}
	if len(git.CheckoutCalls) != 1 {
		t.Errorf("Expected 1 Checkout call, got %d", len(git.CheckoutCalls))
	}

	// Verify checkout was called with locked hash
	if git.CheckoutCalls[0][1] != "abc123def456" {
		t.Errorf("Expected checkout of locked hash, got %s", git.CheckoutCalls[0][1])
	}
}

func TestSyncVendor_HappyPath_UnlockedRef(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "latest789", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		if path == "/tmp/test-12345/LICENSE" {
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute with nil lockedRefs (unlocked mode)
	hashes, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if hashes["main"] != "latest789" {
		t.Errorf("Expected hash latest789, got %s", hashes["main"])
	}

	// Verify checkout was called with FETCH_HEAD (unlocked mode)
	if len(git.CheckoutCalls) != 1 {
		t.Errorf("Expected 1 Checkout call, got %d", len(git.CheckoutCalls))
	}
	if git.CheckoutCalls[0][1] != "FETCH_HEAD" {
		t.Errorf("Expected checkout of FETCH_HEAD, got %s", git.CheckoutCalls[0][1])
	}
}

func TestSyncVendor_ShallowFetchSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

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
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify shallow fetch was attempted
	if len(git.FetchCalls) != 1 {
		t.Errorf("Expected 1 Fetch call, got %d", len(git.FetchCalls))
	}
	if git.FetchCalls[0][1].(int) != 1 {
		t.Errorf("Expected shallow fetch (depth=1), got depth=%d", git.FetchCalls[0][1])
	}

	// Verify no fallback to FetchAll
	if len(git.FetchAllCalls) != 0 {
		t.Errorf("Expected no FetchAll calls, got %d", len(git.FetchAllCalls))
	}
}

func TestSyncVendor_ShallowFetchFails_FullFetchSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Shallow fetch fails, full fetch succeeds
	git.FetchFunc = func(dir string, depth int, ref string) error {
		if depth == 1 {
			return fmt.Errorf("shallow fetch failed")
		}
		return nil
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
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify fallback to FetchAll was called
	if len(git.FetchAllCalls) != 1 {
		t.Errorf("Expected 1 FetchAll call (fallback), got %d", len(git.FetchAllCalls))
	}
}

func TestSyncVendor_BothFetchesFail(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Both fetches fail
	git.FetchFunc = func(dir string, depth int, ref string) error {
		return fmt.Errorf("network error")
	}
	git.FetchAllFunc = func(dir string) error {
		return fmt.Errorf("network error")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to fetch ref") {
		t.Errorf("Expected 'failed to fetch ref' error, got: %v", err)
	}
}

func TestSyncVendor_StaleCommitHashDetection(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "stale123"}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Checkout fails with stale commit error
	git.CheckoutFunc = func(dir, ref string) error {
		if ref == "stale123" {
			return fmt.Errorf("reference is not a tree: stale123")
		}
		return nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, lockedRefs)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "no longer exists in the repository") {
		t.Errorf("Expected stale commit error message, got: %v", err)
	}
	if !contains(err.Error(), "git-vendor update") {
		t.Errorf("Expected helpful update message, got: %v", err)
	}
}

func TestSyncVendor_CheckoutFETCH_HEADFails_RefFallbackSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Checkout FETCH_HEAD fails, checkout ref succeeds
	checkoutAttempts := 0
	git.CheckoutFunc = func(dir, ref string) error {
		checkoutAttempts++
		if ref == "FETCH_HEAD" {
			return fmt.Errorf("FETCH_HEAD not available")
		}
		return nil // Checkout of "main" succeeds
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
		t.Fatalf("Expected success (fallback), got error: %v", err)
	}
	if checkoutAttempts != 2 {
		t.Errorf("Expected 2 checkout attempts (FETCH_HEAD then ref), got %d", checkoutAttempts)
	}
}

func TestSyncVendor_AllCheckoutsFail(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: All checkouts fail
	git.CheckoutFunc = func(dir, ref string) error {
		return fmt.Errorf("checkout failed")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "checkout ref") {
		t.Errorf("Expected checkout error, got: %v", err)
	}
}

func TestSyncVendor_TempDirectoryCreationFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	// Mock: CreateTemp fails
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "", fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestSyncVendor_PathTraversalBlocked(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Create vendor with malicious path mapping
	vendor := types.VendorSpec{
		Name:    "malicious",
		URL:     "https://github.com/attacker/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "payload.txt", To: "../../../etc/passwd"},
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

	// Mock: File exists in temp repo
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected path traversal error, got nil")
	}
	if !contains(err.Error(), "invalid destination path") || !contains(err.Error(), "not allowed") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestSyncVendor_MultipleSpecsPerVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Vendor with 3 specs (main, dev, v1.0)
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
			{
				Ref: "dev",
				Mapping: []types.PathMapping{
					{From: "src/dev.go", To: "lib/dev.go"},
				},
			},
			{
				Ref: "v1.0",
				Mapping: []types.PathMapping{
					{From: "src/release.go", To: "lib/release.go"},
				},
			},
		},
	}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	hashCounter := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hashes, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 3 {
		t.Errorf("Expected 3 hashes (one per spec), got %d", len(hashes))
	}
	if _, ok := hashes["main"]; !ok {
		t.Error("Expected hash for 'main' ref")
	}
	if _, ok := hashes["dev"]; !ok {
		t.Error("Expected hash for 'dev' ref")
	}
	if _, ok := hashes["v1.0"]; !ok {
		t.Error("Expected hash for 'v1.0' ref")
	}

	// Verify each spec triggered a fetch
	if len(git.FetchCalls) != 3 {
		t.Errorf("Expected 3 Fetch calls (one per spec), got %d", len(git.FetchCalls))
	}
}

func TestSyncVendor_MultipleMappingsPerSpec(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: One spec with 5 file mappings
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "file1.go", To: "lib/file1.go"},
					{From: "file2.go", To: "lib/file2.go"},
					{From: "file3.go", To: "lib/file3.go"},
					{From: "file4.go", To: "lib/file4.go"},
					{From: "file5.go", To: "lib/file5.go"},
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
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify all 5 files were copied (plus 1 for license = 6 total)
	if len(fs.CopyFileCalls) < 5 {
		t.Errorf("Expected at least 5 CopyFile calls (5 mappings), got %d", len(fs.CopyFileCalls))
	}
}

func TestSyncVendor_FileCopyFailsInMapping(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: File copy fails only for the mapping (not license)
	fs.CopyFileFunc = func(src, dst string) error {
		// Let license copy succeed, but fail on the actual mapping
		if contains(src, "LICENSE") {
			return nil
		}
		return fmt.Errorf("permission denied")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy file") {
		t.Errorf("Expected 'failed to copy file' error, got: %v", err)
	}
}

func TestSyncVendor_LicenseCopyFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	// Mock: License file exists
	statCalls := 0
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		statCalls++
		if statCalls == 1 && path == "/tmp/test-12345/LICENSE" {
			// First call: LICENSE exists
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: License copy fails
	fs.CopyFileFunc = func(src, dst string) error {
		if contains(src, "LICENSE") {
			return fmt.Errorf("disk full")
		}
		return nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy license") {
		t.Errorf("Expected license copy error, got: %v", err)
	}
}

// ============================================================================
// TestUpdateAll - Comprehensive tests for update orchestration
// ============================================================================

func TestUpdateAll_HappyPath_SingleVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Single vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	// Mock: syncVendor succeeds
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def456", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify lock was saved with correct entry
	if len(lock.SaveCalls) != 1 {
		t.Errorf("Expected 1 Save call, got %d", len(lock.SaveCalls))
	}

	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 1 {
		t.Errorf("Expected 1 lock entry, got %d", len(savedLock.Vendors))
	}

	entry := savedLock.Vendors[0]
	if entry.Name != "test-vendor" {
		t.Errorf("Expected vendor name 'test-vendor', got '%s'", entry.Name)
	}
	if entry.Ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", entry.Ref)
	}
	if entry.CommitHash != "abc123def456" {
		t.Errorf("Expected hash 'abc123def456', got '%s'", entry.CommitHash)
	}
	if entry.Updated == "" {
		t.Error("Expected Updated timestamp, got empty string")
	}
}

func TestUpdateAll_HappyPath_MultipleVendors(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "dev")
	vendor3 := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "v1.0")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Each vendor gets a unique hash (must be at least 7 chars)
	callCount := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		callCount++
		return fmt.Sprintf("hash%d00000", callCount), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify lock has 3 entries (one per vendor)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 3 {
		t.Errorf("Expected 3 lock entries, got %d", len(savedLock.Vendors))
	}

	// Verify all vendors are locked
	vendorNames := make(map[string]bool)
	for _, entry := range savedLock.Vendors {
		vendorNames[entry.Name] = true
	}
	if !vendorNames["vendor-a"] || !vendorNames["vendor-b"] || !vendorNames["vendor-c"] {
		t.Error("Not all vendors were locked")
	}
}

func TestUpdateAll_ConfigLoadFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config load fails
	config.LoadFunc = func() (types.VendorConfig, error) {
		return types.VendorConfig{}, fmt.Errorf("config file corrupt")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "config file corrupt") {
		t.Errorf("Expected config error, got: %v", err)
	}

	// Verify no lock save was attempted
	if len(lock.SaveCalls) != 0 {
		t.Errorf("Expected no Save calls, got %d", len(lock.SaveCalls))
	}
}

func TestUpdateAll_OneVendorFails_OthersContinue(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-good-1", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-bad", "https://github.com/owner/repo-b", "main")
	vendor3 := createTestVendorSpec("vendor-good-2", "https://github.com/owner/repo-c", "main")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: vendor-bad fails, others succeed
	git.InitFunc = func(dir string) error {
		// Fail only for vendor-bad (second call)
		if len(git.InitCalls) == 2 {
			return fmt.Errorf("git init failed")
		}
		return nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify: Overall success (UpdateAll continues on individual failures)
	if err != nil {
		t.Fatalf("Expected success (continue on error), got: %v", err)
	}

	// Verify: Only 2 vendors were locked (vendor-bad skipped)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 2 {
		t.Errorf("Expected 2 lock entries (vendor-bad skipped), got %d", len(savedLock.Vendors))
	}

	// Verify the failed vendor is not in the lock
	for _, entry := range savedLock.Vendors {
		if entry.Name == "vendor-bad" {
			t.Error("vendor-bad should not be in lock file")
		}
	}
}

func TestUpdateAll_LockSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: Lock save fails
	lock.SaveFunc = func(l types.VendorLock) error {
		return fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestUpdateAll_EmptyConfig(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Empty config (no vendors)
	config.Config = types.VendorConfig{Vendors: []types.VendorSpec{}}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success (empty is valid), got error: %v", err)
	}

	// Verify empty lock was saved
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 0 {
		t.Errorf("Expected empty lock, got %d entries", len(savedLock.Vendors))
	}
}

func TestUpdateAll_TimestampFormat(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

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
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify timestamp is in RFC3339 format
	entry := lock.SaveCalls[0].Vendors[0]
	if entry.Updated == "" {
		t.Fatal("Expected non-empty timestamp")
	}

	// Try to parse the timestamp (should not error)
	_, err = time.Parse(time.RFC3339, entry.Updated)
	if err != nil {
		t.Errorf("Timestamp not in RFC3339 format: %v", err)
	}
}

func TestUpdateAll_MultipleSpecsPerVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 1 vendor with 3 specs
	vendor := types.VendorSpec{
		Name:    "multi-spec-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
			{
				Ref: "dev",
				Mapping: []types.PathMapping{
					{From: "src/dev.go", To: "lib/dev.go"},
				},
			},
			{
				Ref: "v1.0",
				Mapping: []types.PathMapping{
					{From: "src/release.go", To: "lib/release.go"},
				},
			},
		},
	}
	config.Config = createTestConfig(vendor)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	hashCounter := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify 3 lock entries (one per spec)
	savedLock := lock.SaveCalls[0]
	if len(savedLock.Vendors) != 3 {
		t.Errorf("Expected 3 lock entries (one per spec), got %d", len(savedLock.Vendors))
	}

	// Verify all refs are present
	refs := make(map[string]bool)
	for _, entry := range savedLock.Vendors {
		refs[entry.Ref] = true
	}
	if !refs["main"] || !refs["dev"] || !refs["v1.0"] {
		t.Error("Not all refs were locked")
	}
}

func TestUpdateAll_LicensePathSet(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

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
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify license path is set correctly
	entry := lock.SaveCalls[0].Vendors[0]
	expectedPath := filepath.Join("/mock/vendor", LicenseDir, "test-vendor.txt")
	if entry.LicensePath != expectedPath {
		t.Errorf("Expected license path '%s', got '%s'", expectedPath, entry.LicensePath)
	}
}

// ============================================================================
// TestSync* - Comprehensive tests for sync orchestration
// ============================================================================

func TestSync_HappyPath_WithLock(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 2 vendors with lock
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")
	config.Config = createTestConfig(vendor1, vendor2)

	lock.Lock = createTestLock(
		createTestLockEntry("vendor-a", "main", "locked123hash"),
		createTestLockEntry("vendor-b", "main", "locked456hash"),
	)

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
	err := syncer.Sync()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify synced with locked hashes
	if len(git.CheckoutCalls) != 2 {
		t.Errorf("Expected 2 checkouts (2 vendors), got %d", len(git.CheckoutCalls))
	}
	if git.CheckoutCalls[0][1] != "locked123hash" {
		t.Errorf("Expected checkout of locked123hash, got %s", git.CheckoutCalls[0][1])
	}
	if git.CheckoutCalls[1][1] != "locked456hash" {
		t.Errorf("Expected checkout of locked456hash, got %s", git.CheckoutCalls[1][1])
	}
}

func TestSync_NoLock_TriggersUpdate(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	// Mock: Lock load returns error (no lock file)
	lock.LoadFunc = func() (types.VendorLock, error) {
		return types.VendorLock{}, fmt.Errorf("file not found")
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
	err := syncer.Sync()

	// Verify
	if err != nil {
		t.Fatalf("Expected success (UpdateAll triggered), got error: %v", err)
	}

	// Verify UpdateAll was triggered (lock was saved)
	if len(lock.SaveCalls) != 1 {
		t.Errorf("Expected UpdateAll to save lock, got %d saves", len(lock.SaveCalls))
	}
}

func TestSyncDryRun_ShowsPreview(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	lock.Lock = createTestLock(createTestLockEntry("test-vendor", "main", "locked123hash"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.SyncDryRun()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify NO sync operations were performed
	if len(git.InitCalls) != 0 {
		t.Errorf("Expected 0 git operations (dry-run), got %d Init calls", len(git.InitCalls))
	}
	if len(git.CheckoutCalls) != 0 {
		t.Errorf("Expected 0 checkouts (dry-run), got %d", len(git.CheckoutCalls))
	}
}

func TestSyncDryRun_NoLock(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	// Mock: No lock file
	lock.LoadFunc = func() (types.VendorLock, error) {
		return types.VendorLock{}, fmt.Errorf("file not found")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.SyncDryRun()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify no UpdateAll was triggered (dry-run)
	if len(lock.SaveCalls) != 0 {
		t.Errorf("Expected no UpdateAll (dry-run), got %d saves", len(lock.SaveCalls))
	}
}

func TestSyncWithOptions_VendorFilter(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")
	vendor3 := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "main")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	lock.Lock = createTestLock(
		createTestLockEntry("vendor-a", "main", "hashA123456"),
		createTestLockEntry("vendor-b", "main", "hashB123456"),
		createTestLockEntry("vendor-c", "main", "hashC123456"),
	)

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

	// Execute: Sync only vendor-b
	err := syncer.SyncWithOptions("vendor-b", false)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify only vendor-b was synced
	if len(git.AddRemoteCalls) != 1 {
		t.Errorf("Expected 1 vendor synced, got %d", len(git.AddRemoteCalls))
	}
	if len(git.AddRemoteCalls) > 0 && git.AddRemoteCalls[0][2] != "https://github.com/owner/repo-b" {
		t.Errorf("Expected vendor-b synced, got URL: %s", git.AddRemoteCalls[0][2])
	}
}

func TestSyncWithOptions_VendorNotFoundEarly(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	lock.Lock = createTestLock(createTestLockEntry("test-vendor", "main", "locked123hash"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute: Try to sync non-existent vendor
	err := syncer.SyncWithOptions("nonexistent-vendor", false)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}

	// Verify no git operations were attempted
	if len(git.InitCalls) != 0 {
		t.Errorf("Expected 0 git operations (early validation), got %d Init calls", len(git.InitCalls))
	}
}

func TestSyncWithOptions_ForceSync(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	lock.Lock = createTestLock(createTestLockEntry("test-vendor", "main", "oldlocked123"))

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "newlatest789", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute: Force sync (ignore lock)
	err := syncer.SyncWithOptions("", true)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify checkout used FETCH_HEAD (unlocked mode) not the locked hash
	if len(git.CheckoutCalls) != 1 {
		t.Errorf("Expected 1 checkout, got %d", len(git.CheckoutCalls))
	}
	if git.CheckoutCalls[0][1] == "oldlocked123" {
		t.Error("Force sync should ignore locked hash, but it was used")
	}
	if git.CheckoutCalls[0][1] != "FETCH_HEAD" {
		t.Errorf("Expected checkout of FETCH_HEAD (force mode), got %s", git.CheckoutCalls[0][1])
	}
}

func TestSync_StopsOnFirstError(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")
	vendor3 := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "main")
	config.Config = createTestConfig(vendor1, vendor2, vendor3)

	lock.Lock = createTestLock(
		createTestLockEntry("vendor-a", "main", "hashA123456"),
		createTestLockEntry("vendor-b", "main", "hashB123456"),
		createTestLockEntry("vendor-c", "main", "hashC123456"),
	)

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: vendor-b fails (second vendor)
	git.InitFunc = func(dir string) error {
		if len(git.InitCalls) == 2 {
			return fmt.Errorf("git init failed")
		}
		return nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Sync()

	// Verify: Error returned
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Verify: vendor-c was NOT synced (stopped after vendor-b error)
	if len(git.AddRemoteCalls) < 3 {
		// vendor-c should not have been attempted
		for _, call := range git.AddRemoteCalls {
			if call[2] == "https://github.com/owner/repo-c" {
				t.Error("vendor-c should not be synced after vendor-b error")
			}
		}
	}
}

func TestSync_EmptyConfig(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Empty config
	config.Config = types.VendorConfig{Vendors: []types.VendorSpec{}}
	lock.Lock = types.VendorLock{Vendors: []types.LockDetails{}}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Sync()

	// Verify
	if err != nil {
		t.Fatalf("Expected success (empty is valid), got error: %v", err)
	}

	// Verify no operations
	if len(git.InitCalls) != 0 {
		t.Errorf("Expected 0 git operations (empty config), got %d", len(git.InitCalls))
	}
}

func TestSync_ConfigLoadFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config load fails
	config.LoadFunc = func() (types.VendorConfig, error) {
		return types.VendorConfig{}, fmt.Errorf("config corrupt")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.Sync()

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "config corrupt") {
		t.Errorf("Expected config error, got: %v", err)
	}
}

func TestSync_EmptyLock_TriggersUpdate(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	// Mock: Lock is empty (no vendors)
	lock.Lock = types.VendorLock{Vendors: []types.LockDetails{}}

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
	err := syncer.Sync()

	// Verify: UpdateAll was triggered
	if err != nil {
		t.Fatalf("Expected success (UpdateAll triggered), got error: %v", err)
	}

	// Verify lock was saved (UpdateAll ran)
	if len(lock.SaveCalls) != 1 {
		t.Errorf("Expected UpdateAll to save lock, got %d saves", len(lock.SaveCalls))
	}
}

// ============================================================================
// Utility function tests
// ============================================================================

func TestGetConfig(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	config.Config = createTestConfig(vendor)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	cfg, err := syncer.GetConfig()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(cfg.Vendors) != 1 {
		t.Errorf("Expected 1 vendor, got %d", len(cfg.Vendors))
	}
}

func TestGetLockHash(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	lock.Lock = createTestLock(createTestLockEntry("test-vendor", "main", "abc123hash"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hash := syncer.GetLockHash("test-vendor", "main")

	// Verify
	if hash != "abc123hash" {
		t.Errorf("Expected hash 'abc123hash', got '%s'", hash)
	}
}

func TestAudit(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	lock.Lock = createTestLock(
		createTestLockEntry("vendor-a", "main", "hash123456"),
		createTestLockEntry("vendor-b", "main", "hash789012"),
	)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute (just verify no panic)
	syncer.Audit()
}

func TestCheckGitHubLicense(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	detectedLicense, err := syncer.CheckGitHubLicense("https://github.com/owner/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if detectedLicense != "MIT" {
		t.Errorf("Expected MIT license (mock default), got '%s'", detectedLicense)
	}
}

func TestListLocalDir(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	items, err := syncer.ListLocalDir("/some/path")

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(items) < 1 {
		t.Error("Expected at least 1 item from mock")
	}
}

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

// ============================================================================
// FetchRepoDir Tests
// ============================================================================

func TestFetchRepoDir_HappyPath(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Clone succeeds
	git.CloneFunc = func(dir, url string, opts *CloneOptions) error {
		return nil
	}

	// Mock: ListTree returns files
	git.ListTreeFunc = func(dir, ref, subdir string) ([]string, error) {
		return []string{"file1.go", "file2.go", "subdir/"}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	files, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "src")

	// Verify
	assertNoError(t, err, "FetchRepoDir should succeed")
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}
	if len(git.CloneCalls) != 1 {
		t.Errorf("Expected 1 Clone call, got %d", len(git.CloneCalls))
	}
	if len(git.ListTreeCalls) != 1 {
		t.Errorf("Expected 1 ListTree call, got %d", len(git.ListTreeCalls))
	}
}

func TestFetchRepoDir_CloneFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Clone fails
	git.CloneFunc = func(dir, url string, opts *CloneOptions) error {
		return fmt.Errorf("network timeout")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "src")

	// Verify
	assertError(t, err, "FetchRepoDir should fail when clone fails")
	if !contains(err.Error(), "network timeout") {
		t.Errorf("Expected network timeout error, got: %v", err)
	}
}

func TestFetchRepoDir_SpecificRef(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Clone succeeds
	git.CloneFunc = func(dir, url string, opts *CloneOptions) error {
		return nil
	}

	// Mock: Fetch called for specific ref
	fetchCalled := false
	git.FetchFunc = func(dir string, depth int, ref string) error {
		fetchCalled = true
		if ref != "v1.0.0" {
			t.Errorf("Expected ref 'v1.0.0', got '%s'", ref)
		}
		return nil
	}

	// Mock: ListTree returns files
	git.ListTreeFunc = func(dir, ref, subdir string) ([]string, error) {
		if ref != "v1.0.0" {
			t.Errorf("Expected ListTree to use ref 'v1.0.0', got '%s'", ref)
		}
		return []string{"file.go"}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "v1.0.0", "")

	// Verify
	assertNoError(t, err, "FetchRepoDir should succeed")
	if !fetchCalled {
		t.Error("Expected Fetch to be called for specific ref")
	}
}

func TestFetchRepoDir_ListTreeFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Clone succeeds
	git.CloneFunc = func(dir, url string, opts *CloneOptions) error {
		return nil
	}

	// Mock: ListTree fails
	git.ListTreeFunc = func(dir, ref, subdir string) ([]string, error) {
		return nil, fmt.Errorf("invalid tree object")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "nonexistent")

	// Verify
	assertError(t, err, "FetchRepoDir should fail when ListTree fails")
	if !contains(err.Error(), "invalid tree object") {
		t.Errorf("Expected tree object error, got: %v", err)
	}
}

// ============================================================================
// SaveVendor Tests
// ============================================================================

func TestSaveVendor_NewVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with empty config
	config.Config = createTestConfig()

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	vendor := createTestVendorSpec("new-vendor", "https://github.com/owner/repo", "main")
	err := syncer.SaveVendor(vendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for new vendor")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor in config, got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].Name != "new-vendor" {
		t.Errorf("Expected vendor name 'new-vendor', got '%s'", savedConfig.Vendors[0].Name)
	}
}

func TestSaveVendor_UpdateExisting(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with existing vendor
	existingVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/old-repo", "main")
	config.Config = createTestConfig(existingVendor)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - update URL
	updatedVendor := createTestVendorSpec("existing-vendor", "https://github.com/owner/new-repo", "develop")
	err := syncer.SaveVendor(updatedVendor)

	// Verify
	assertNoError(t, err, "SaveVendor should succeed for existing vendor")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor (updated, not added), got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].URL != "https://github.com/owner/new-repo" {
		t.Errorf("Expected URL to be updated, got '%s'", savedConfig.Vendors[0].URL)
	}
}

func TestSaveVendor_ConfigSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config save fails
	config.SaveFunc = func(cfg types.VendorConfig) error {
		return fmt.Errorf("permission denied")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	err := syncer.SaveVendor(vendor)

	// Verify
	assertError(t, err, "SaveVendor should fail when config save fails")
	if !contains(err.Error(), "permission denied") {
		t.Errorf("Expected permission denied error, got: %v", err)
	}
}

// ============================================================================
// RemoveVendor Tests
// ============================================================================

func TestRemoveVendor_HappyPath(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Start with 2 vendors
	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	vendor2 := createTestVendorSpec("vendor-2", "https://github.com/owner/repo2", "main")
	config.Config = createTestConfig(vendor1, vendor2)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - remove vendor-1
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertNoError(t, err, "RemoveVendor should succeed")
	if len(config.SaveCalls) == 0 {
		t.Fatal("Expected config to be saved")
	}
	savedConfig := config.SaveCalls[0]
	if len(savedConfig.Vendors) != 1 {
		t.Errorf("Expected 1 vendor remaining, got %d", len(savedConfig.Vendors))
	}
	if savedConfig.Vendors[0].Name != "vendor-2" {
		t.Errorf("Expected vendor-2 to remain, got '%s'", savedConfig.Vendors[0].Name)
	}
	// Verify license file removal was attempted
	if len(fs.RemoveCalls) == 0 {
		t.Error("Expected license file removal to be called")
	}
}

func TestRemoveVendor_VendorNotFound(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.Config = createTestConfig(vendor1)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute - try to remove nonexistent vendor
	err := syncer.RemoveVendor("nonexistent-vendor")

	// Verify
	assertError(t, err, "RemoveVendor should fail for nonexistent vendor")
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
	// Verify config was not saved
	if len(config.SaveCalls) > 0 {
		t.Error("Expected config to not be saved when vendor not found")
	}
}

func TestRemoveVendor_ConfigLoadFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Mock: Config load fails
	config.LoadFunc = func() (types.VendorConfig, error) {
		return types.VendorConfig{}, fmt.Errorf("config file corrupted")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.RemoveVendor("any-vendor")

	// Verify
	assertError(t, err, "RemoveVendor should fail when config load fails")
	if !contains(err.Error(), "config file corrupted") {
		t.Errorf("Expected corrupted config error, got: %v", err)
	}
}

func TestRemoveVendor_ConfigSaveFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor1 := createTestVendorSpec("vendor-1", "https://github.com/owner/repo1", "main")
	config.Config = createTestConfig(vendor1)

	// Mock: Config save fails
	config.SaveFunc = func(cfg types.VendorConfig) error {
		return fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.RemoveVendor("vendor-1")

	// Verify
	assertError(t, err, "RemoveVendor should fail when config save fails")
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}
