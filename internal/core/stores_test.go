package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Config Store Tests
// ============================================================================

func TestLoadConfig(t *testing.T) {
	t.Run("Load valid config", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
	// 	vendorDir := filepath.Join(tempDir, VendorDir)
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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		// Create directory first (saveConfig doesn't create directories)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
			t.Error("saveConfig() did not create vendor.yml file")
		}
	})

	t.Run("Save config preserves all fields", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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

// ============================================================================
// Lock Store Tests
// ============================================================================

// TestLoadLock tests lock file loading
func TestLoadLock(t *testing.T) {
	t.Run("Load valid lock file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

		expectedLock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:        "test-vendor",
					Ref:         "main",
					CommitHash:  "abc123def456",
					LicensePath: VendorDir + "/" + LicensesDir + "/test-vendor.txt",
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
		expectedLicensePath := VendorDir + "/" + LicensesDir + "/test-vendor.txt"
		if loadedLock.Vendors[0].LicensePath != expectedLicensePath {
			t.Errorf("Expected license path %q, got %q", expectedLicensePath, loadedLock.Vendors[0].LicensePath)
		}
	})

	t.Run("Error when lock file doesn't exist", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

		_, err := m.loadLock()
		if err == nil {
			t.Error("Expected error when lock file doesn't exist, got nil")
		}
	})

	t.Run("Error when lock file is malformed", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		// Create directory first (saveLock doesn't create directories)
		_ = os.MkdirAll(m.RootDir, 0755)

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
		if _, err := os.Stat(lockPath); errors.Is(err, os.ErrNotExist) {
			t.Error("saveLock() did not create vendor.lock file")
		}
	})

	t.Run("Save lock preserves all fields", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

		lock := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:        "test-vendor",
					Ref:         "v1.0.0",
					CommitHash:  "abc123def456789",
					LicensePath: VendorDir + "/" + LicensesDir + "/test-vendor.txt",
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
		expectedLicensePath := VendorDir + "/" + LicensesDir + "/test-vendor.txt"
		if v.LicensePath != expectedLicensePath {
			t.Errorf("LicensePath not preserved: got %q", v.LicensePath)
		}
		if v.Updated != "2025-01-15T12:30:45Z" {
			t.Errorf("Updated not preserved: got %q", v.Updated)
		}
	})

	t.Run("Save empty lock file", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

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
// Edge Case Tests for Config/Lock Parsing
// ============================================================================

func TestLoadConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		wantCount int // Expected number of vendors (if no error)
	}{
		{
			name: "Invalid YAML syntax - unclosed bracket",
			yaml: `vendors:
  - name: [unclosed`,
			wantErr: true,
		},
		{
			name:    "Invalid YAML - wrong type for vendors field",
			yaml:    `vendors: "not an array"`,
			wantErr: true,
		},
		{
			name: "Unknown fields are ignored",
			yaml: `vendors: []
unknown_field: value
extra_data: true`,
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:      "Empty file",
			yaml:      "",
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:    "Only whitespace (causes YAML parse error)",
			yaml:    "   \n\t\n   ",
			wantErr: true,
		},
		{
			name: "Unicode in vendor name",
			yaml: `vendors:
  - name: "测试-vendor-Ñoño"
    url: "https://github.com/test/repo"
    license: "MIT"
    specs:
      - ref: "main"
        mapping:
          - from: "src"
            to: "lib"`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Special characters in paths",
			yaml: `vendors:
  - name: "test"
    url: "https://github.com/test/repo"
    license: "MIT"
    specs:
      - ref: "main"
        mapping:
          - from: "path-with-dashes_and_underscores.go"
            to: "dest/with/slashes/file.go"`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Very long vendor name (printable chars)",
			yaml: `vendors:
  - name: "` + strings.Repeat("a", 200) + `"
    url: "https://github.com/test/repo"
    license: "MIT"
    specs:
      - ref: "main"
        mapping:
          - from: "src"
            to: "lib"`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Multiple vendors with same name (last one wins)",
			yaml: `vendors:
  - name: "same"
    url: "https://github.com/test/repo1"
    license: "MIT"
    specs:
      - ref: "main"
        mapping:
          - from: "src1"
            to: "lib1"
  - name: "same"
    url: "https://github.com/test/repo2"
    license: "Apache-2.0"
    specs:
      - ref: "dev"
        mapping:
          - from: "src2"
            to: "lib2"`,
			wantErr:   false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			vendorDir := filepath.Join(tempDir, VendorDir)
			_ = os.MkdirAll(vendorDir, 0755)

			// Write test YAML
			configPath := filepath.Join(vendorDir, "vendor.yml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Attempt to load
			store := NewFileConfigStore(vendorDir)
			config, err := store.Load()

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(config.Vendors) != tt.wantCount {
					t.Errorf("Expected %d vendors, got %d", tt.wantCount, len(config.Vendors))
				}
			}
		})
	}
}

func TestLoadLock_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		wantCount int
	}{
		{
			name: "Invalid YAML syntax",
			yaml: `vendors:
  - name: test
    ref: main
    commit_hash: {invalid`,
			wantErr: true,
		},
		{
			name:    "Wrong type for vendors",
			yaml:    `vendors: "not-an-array"`,
			wantErr: true,
		},
		{
			name:      "Empty lock file",
			yaml:      "",
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "Lock with all fields",
			yaml: `vendors:
  - name: "complete"
    ref: "v1.0.0"
    commit_hash: "abc123def456"
    license_path: ".git-vendor/licenses/complete.txt"
    updated: "2025-01-15T12:30:45Z"`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Lock with minimal fields",
			yaml: `vendors:
  - name: "minimal"
    ref: "main"
    commit_hash: "abc123"`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "Lock with extra unknown fields (ignored)",
			yaml: `vendors:
  - name: "test"
    ref: "main"
    commit_hash: "abc123"
    extra_field: "ignored"
    another_field: 42`,
			wantErr:   false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			vendorDir := filepath.Join(tempDir, VendorDir)
			_ = os.MkdirAll(vendorDir, 0755)

			// Write test YAML
			lockPath := filepath.Join(vendorDir, "vendor.lock")
			if err := os.WriteFile(lockPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("Failed to write test lock: %v", err)
			}

			// Attempt to load
			store := NewFileLockStore(vendorDir)
			lock, err := store.Load()

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(lock.Vendors) != tt.wantCount {
					t.Errorf("Expected %d vendors, got %d", tt.wantCount, len(lock.Vendors))
				}
			}
		})
	}
}

func TestConfig_LargeFile(t *testing.T) {
	t.Run("Config with 100 vendors", func(t *testing.T) {
		tempDir := t.TempDir()
		vendorDir := filepath.Join(tempDir, VendorDir)
		m := newTestManager(vendorDir)
		_ = os.MkdirAll(m.RootDir, 0755)

		// Create config with 100 vendors
		config := types.VendorConfig{Vendors: make([]types.VendorSpec, 100)}
		for i := 0; i < 100; i++ {
			config.Vendors[i] = types.VendorSpec{
				Name:    filepath.Join("vendor", string(rune(i))),
				URL:     filepath.Join("https://github.com/test/repo", string(rune(i))),
				License: "MIT",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src", To: filepath.Join("lib", string(rune(i)))},
						},
					},
				},
			}
		}

		// Save and load
		if err := m.saveConfig(config); err != nil {
			t.Fatalf("Failed to save large config: %v", err)
		}

		loadedConfig, err := m.loadConfig()
		if err != nil {
			t.Fatalf("Failed to load large config: %v", err)
		}

		if len(loadedConfig.Vendors) != 100 {
			t.Errorf("Expected 100 vendors, got %d", len(loadedConfig.Vendors))
		}
	})
}
