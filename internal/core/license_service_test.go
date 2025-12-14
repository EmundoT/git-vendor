package core

import (
	"testing"
)

// ============================================================================
// License Validation Tests
// ============================================================================

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

// ============================================================================
// GitHub License Checker Tests
// ============================================================================

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
