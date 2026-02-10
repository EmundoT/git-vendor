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
// Path Validation Tests
// ============================================================================

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
		// Additional security edge cases
		{
			name:      "Multiple parent traversals",
			destPath:  "../../../../../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		{
			name:      "Windows backslash parent traversal",
			destPath:  "..\\..\\windows\\system32",
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name:      "Mixed separators with traversal",
			destPath:  "lib/../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		{
			name:      "Windows UNC path",
			destPath:  "\\\\server\\share\\file",
			wantError: true,
			errorMsg:  "absolute paths are not allowed",
		},
		{
			name:      "Double slash Unix root",
			destPath:  "//etc/passwd",
			wantError: true,
			errorMsg:  "absolute paths are not allowed",
		},
		{
			name:      "Windows drive letter variations",
			destPath:  "D:\\Users\\Public",
			wantError: true,
			errorMsg:  "absolute paths are not allowed",
		},
		{
			name:      "Parent at end normalizes to current dir (allowed)",
			destPath:  "lib/..",
			wantError: false, // filepath.Clean("lib/..") = "." which is allowed
		},
		{
			name:      "Single parent reference",
			destPath:  "../",
			wantError: true,
			errorMsg:  "path traversal with .. is not allowed",
		},
		// Valid edge cases
		{
			name:      "Path with spaces",
			destPath:  "lib/my file.go",
			wantError: false,
		},
		{
			name:      "Path with dashes",
			destPath:  "some-lib/some-file.go",
			wantError: false,
		},
		{
			name:      "Path with underscores",
			destPath:  "some_lib/some_file.go",
			wantError: false,
		},
		{
			name:      "Hidden file (dot prefix)",
			destPath:  ".hidden/file.go",
			wantError: false,
		},
		{
			name:      "Path with dots in filename",
			destPath:  "lib/file.test.go",
			wantError: false,
		},
		{
			name:      "Unicode in filename",
			destPath:  "lib/文件.go",
			wantError: false,
		},
		{
			name:      "Redundant slashes (normalized by filepath.Clean)",
			destPath:  "lib//file.go",
			wantError: false,
		},
		{
			name:      "Self reference (normalized by filepath.Clean)",
			destPath:  "lib/./file.go",
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

func TestValidateDestPath_SecurityRegression(t *testing.T) {
	// These are real-world path traversal attack vectors
	// Ensure they are ALL rejected
	attackVectors := map[string]string{
		"Simple parent":               "../",
		"Windows parent":              "..\\",
		"Deep Unix traversal":         "../../../../../../../etc/passwd",
		"Deep Windows traversal":      "..\\..\\..\\..\\..\\..\\..\\windows\\system32",
		"Traversal with prefix":       "foo/../../etc/passwd",
		"Deep traversal with prefix":  "foo/../../../etc/passwd",
		"Unix absolute":               "/etc/passwd",
		"Windows absolute":            "C:\\Windows\\System32",
		"Double slash":                "//etc/passwd",
		"Windows pipe":                "\\\\.\\pipe\\vulnerable",
		"UNC path":                    "\\\\server\\share",
		"Multiple drives":             "E:\\secret",
		"Root with traversal":         "/var/../etc/passwd",
		"Windows root with traversal": "C:\\..\\Windows",
		"Backslash traversal":         "..\\",
		"Forward slash traversal":     "../",
		"Combined traversal":          "lib/../../../../../../etc/shadow",
	}

	for name, attack := range attackVectors {
		t.Run(name, func(t *testing.T) {
			err := ValidateDestPath(attack)
			if err == nil {
				t.Errorf("SECURITY: ValidateDestPath(%q) = nil, MUST reject path traversal attack", attack)
			}
		})
	}
}

// ============================================================================
// Config Validation Tests
// ============================================================================

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
			vendorDir := filepath.Join(tempDir, VendorDir)
			m := newTestManager(vendorDir)
			// Create vendor directory before saving config
			_ = os.MkdirAll(vendorDir, 0755)

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

// ============================================================================
// Conflict Detection Tests
// ============================================================================

// TestDetectConflicts_EmptyOwners tests that DetectConflicts doesn't panic
// when pathMap contains empty slices (Bug #2)
func TestDetectConflicts_EmptyOwners(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	// Create Manager with proper initialization
	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil, nil)
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

func TestDetectConflicts_NoPanic(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(false)
	fs := NewOSFileSystem()
	licenseChecker := NewGitHubLicenseChecker(nil, AllowedLicenses)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, vendorDir, nil, nil)
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

func TestDetectConflicts_Comprehensive(t *testing.T) {
	t.Run("Detect same path conflict", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		// Create vendor directory before saving config
		_ = os.MkdirAll(vendorDir, 0755)

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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		// Create vendor directory before saving config
		_ = os.MkdirAll(vendorDir, 0755)

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

// ============================================================================
// Edge Case Tests for Coverage
// ============================================================================

func TestValidateConfig_LoadError(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	m := newTestManager(vendorDir)

	// Delete vendor.yml to cause load error
	configPath := filepath.Join(vendorDir, "vendor.yml")
	_ = os.Remove(configPath)

	// ValidateConfig should fail with load error
	err := m.ValidateConfig()
	if err == nil {
		t.Error("Expected error when config cannot be loaded, got nil")
	}
}

func TestDetectConflicts_LoadError(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	m := newTestManager(vendorDir)

	// Delete vendor.yml to cause load error
	configPath := filepath.Join(vendorDir, "vendor.yml")
	_ = os.Remove(configPath)

	// DetectConflicts with missing config returns empty conflicts (not an error)
	conflicts, err := m.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() unexpected error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts for missing config, got %d", len(conflicts))
	}
}

func TestBuildPathOwnershipMap_DotDestination(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	m := newTestManager(vendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	// Config with "." as destination (should use auto-path)
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/test/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "."},
						},
					},
				},
			},
		},
	}

	if err := m.saveConfig(config); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// This should not cause any errors
	conflicts, err := m.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}

	// No conflicts expected
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}
}

func TestIsSubPath_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	m := newTestManager(vendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	tests := []struct {
		name           string
		path1          string
		path2          string
		expectConflict bool
	}{
		{
			name:           "Same paths",
			path1:          "lib",
			path2:          "lib",
			expectConflict: true, // Same path, different vendors = conflict
		},
		{
			name:           "Parent-child relationship",
			path1:          "lib",
			path2:          "lib/subdir",
			expectConflict: true,
		},
		{
			name:           "Child-parent relationship",
			path1:          "lib/subdir",
			path2:          "lib",
			expectConflict: true,
		},
		{
			name:           "Sibling paths",
			path1:          "lib1",
			path2:          "lib2",
			expectConflict: false,
		},
		{
			name:           "Nested deep paths",
			path1:          "vendor/lib/pkg",
			path2:          "vendor/lib/pkg/subpkg",
			expectConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.VendorConfig{
				Vendors: []types.VendorSpec{
					{
						Name: "vendor1",
						URL:  "https://github.com/test/repo1",
						Specs: []types.BranchSpec{
							{
								Ref: "main",
								Mapping: []types.PathMapping{
									{From: "src", To: tt.path1},
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
									{From: "pkg", To: tt.path2},
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

			hasConflict := len(conflicts) > 0
			if hasConflict != tt.expectConflict {
				t.Errorf("Expected conflict=%v, got conflict=%v (conflicts: %d)",
					tt.expectConflict, hasConflict, len(conflicts))
			}
		})
	}
}

func TestDetectConflicts_MultipleOwnersPerPath(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	m := newTestManager(vendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	// Three vendors mapping to the same path
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
							{From: "pkg", To: "lib"},
						},
					},
				},
			},
			{
				Name: "vendor3",
				URL:  "https://github.com/test/repo3",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "mod", To: "lib"},
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

	// Should have 3 conflicts: v1-v2, v1-v3, v2-v3
	if len(conflicts) != 3 {
		t.Errorf("Expected 3 conflicts for 3 vendors mapping to same path, got %d", len(conflicts))
	}
}

// ============================================================================
// ValidateConfig — Gomock-based unit tests (no real filesystem)
// ============================================================================

func TestValidateConfig_Gomock_DuplicateNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "dupe",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "src", To: "lib"}}}},
			},
			{
				Name: "dupe",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "pkg", To: "vendor"}}}},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for duplicate vendor names")
	}
	if !contains(err.Error(), "duplicate vendor name: dupe") {
		t.Errorf("error = %q, want 'duplicate vendor name' message", err.Error())
	}
}

func TestValidateConfig_Gomock_EmptySpecs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:  "empty-specs",
				URL:   "https://github.com/a/repo",
				Specs: []types.BranchSpec{},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for vendor with no specs")
	}
	if !contains(err.Error(), "has no specs configured") {
		t.Errorf("error = %q, want 'has no specs configured'", err.Error())
	}
}

func TestValidateConfig_Gomock_MissingRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "no-ref",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "", Mapping: []types.PathMapping{{From: "src", To: "lib"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for spec with no ref")
	}
	if !contains(err.Error(), "has a spec with no ref") {
		t.Errorf("error = %q, want 'has a spec with no ref'", err.Error())
	}
}

func TestValidateConfig_Gomock_EmptyMappings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "no-mappings",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for spec with no mappings")
	}
	if !contains(err.Error(), "has no path mappings") {
		t.Errorf("error = %q, want 'has no path mappings'", err.Error())
	}
}

func TestValidateConfig_Gomock_EmptyFromPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "empty-from",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "", To: "lib"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for empty 'from' path")
	}
	if !contains(err.Error(), "empty 'from' path") {
		t.Errorf("error = %q, want 'empty from path'", err.Error())
	}
}

func TestValidateConfig_Gomock_MissingURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "no-url",
				URL:  "",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src", To: "lib"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !contains(err.Error(), "has no URL") {
		t.Errorf("error = %q, want 'has no URL'", err.Error())
	}
}

func TestValidateConfig_Gomock_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("permission denied"))

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
	if !contains(err.Error(), "permission denied") {
		t.Errorf("error = %q, want 'permission denied'", err.Error())
	}
}

// ============================================================================
// DetectConflicts — Gomock-based unit tests
// ============================================================================

func TestDetectConflicts_Gomock_OverlappingPathsBetweenVendors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src/", To: "lib/"}}},
				},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "pkg/", To: "lib/sub/"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Error("expected at least one overlap conflict for lib/ and lib/sub/")
	}
}

func TestDetectConflicts_Gomock_SameExactPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "file.go", To: "shared/file.go"}}},
				},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "other.go", To: "shared/file.go"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Error("expected conflict for same exact destination path")
	}

	// Verify the conflict references both vendors
	found := false
	for _, c := range conflicts {
		if (c.Vendor1 == "vendor-a" && c.Vendor2 == "vendor-b") ||
			(c.Vendor1 == "vendor-b" && c.Vendor2 == "vendor-a") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected conflict between vendor-a and vendor-b")
	}
}

func TestDetectConflicts_Gomock_SelfConflictSingleVendor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "self-conflict",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "file1.go", To: "output.go"},
							{From: "file2.go", To: "output.go"},
						},
					},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Two mappings from same vendor to same path → self-conflict
	if len(conflicts) == 0 {
		t.Error("expected self-conflict for same vendor mapping to same destination twice")
	}
}

func TestDetectConflicts_Gomock_PositionPathStripped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src.go:L1-L5", To: "output.go:L5"}}},
				},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "pkg.go:L1", To: "output.go:L10"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both map to "output.go" (after stripping positions) → conflict
	if len(conflicts) == 0 {
		t.Error("expected conflict: position-mapped paths to same file should conflict")
	}
}

func TestDetectConflicts_Gomock_PositionVsWholeFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "whole-vendor",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src.go", To: "target.go"}}},
				},
			},
			{
				Name: "pos-vendor",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "pkg.go:L1-L5", To: "target.go:L10-L15"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conflicts) == 0 {
		t.Error("expected conflict: whole-file and position mapping to same file should conflict")
	}
}

func TestDetectConflicts_Gomock_NoConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/a/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src.go", To: "lib-a/file.go"}}},
				},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/b/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "pkg.go", To: "lib-b/file.go"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	conflicts, err := svc.DetectConflicts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for non-overlapping paths, got %d", len(conflicts))
	}
}

func TestDetectConflicts_Gomock_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("disk error"))

	svc := NewValidationService(mockConfig)
	_, err := svc.DetectConflicts()
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
	if !contains(err.Error(), "disk error") {
		t.Errorf("error = %q, want 'disk error'", err.Error())
	}
}
