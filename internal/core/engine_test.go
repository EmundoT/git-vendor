package core

import (
	"os"
	"path/filepath"
	"testing"

	"git-vendor/internal/types"
)

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

	// Create Manager with temp directory
	m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
	os.MkdirAll(m.RootDir, 0755)

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

	// Create Manager with temp directory
	m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
	os.MkdirAll(m.RootDir, 0755)

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
	m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
	os.MkdirAll(m.RootDir, 0755)

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
			err := validateDestPath(tt.destPath)

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
			m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
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
		if err := copyFile(srcFile, dstFile); err != nil {
			t.Fatalf("copyFile() error = %v", err)
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

		err := copyFile(srcFile, dstFile)
		if err == nil {
			t.Error("copyFile() expected error for nonexistent source, got nil")
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

		err := copyFile(srcFile, dstFile)
		if err == nil {
			t.Error("copyFile() expected error for nonexistent destination directory, got nil")
		}
	})
}

// TestCopyDir tests directory copying functionality
func TestCopyDir(t *testing.T) {
	t.Run("Successful directory copy", func(t *testing.T) {
		tempDir := t.TempDir()
		srcDir := filepath.Join(tempDir, "source")
		dstDir := filepath.Join(tempDir, "dest")

		// Create source directory with files
		os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
		os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
		os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

		// Copy directory
		if err := copyDir(srcDir, dstDir); err != nil {
			t.Fatalf("copyDir() error = %v", err)
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
		if err := copyDir(srcDir, dstDir); err != nil {
			t.Fatalf("copyDir() error = %v", err)
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

		err := copyDir(srcDir, dstDir)
		if err == nil {
			t.Error("copyDir() expected error for nonexistent source, got nil")
		}
	})
}

// TestDetectConflicts_Comprehensive adds more comprehensive conflict detection tests
func TestDetectConflicts_Comprehensive(t *testing.T) {
	t.Run("Detect same path conflict", func(t *testing.T) {
		tempDir := t.TempDir()
		m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
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
		m := &Manager{RootDir: filepath.Join(tempDir, "vendor")}
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
