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
