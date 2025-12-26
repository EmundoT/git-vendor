package core

import (
	"os"
	"path/filepath"
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// Path Validation Tests
// ============================================================================

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

// ============================================================================
// Config Validation Tests
// ============================================================================

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
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

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

// TestDetectConflicts_NoPanic tests that DetectConflicts handles edge cases safely
func TestDetectConflicts_NoPanic(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

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

// TestDetectConflicts_Comprehensive adds more comprehensive conflict detection tests
func TestDetectConflicts_Comprehensive(t *testing.T) {
	t.Run("Detect same path conflict", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, "vendor")
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
		vendorDir := filepath.Join(tempDir, "vendor")
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
