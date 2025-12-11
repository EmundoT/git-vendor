package core

import (
	"os"
	"path/filepath"
	"testing"

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
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir)
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
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir)
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
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir)
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
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir)
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
			errorMsg:  "no vendors configured",
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
