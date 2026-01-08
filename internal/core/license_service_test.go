package core

import (
	"fmt"
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
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: License checker returns MIT
	license.EXPECT().CheckLicense("https://github.com/owner/repo").Return("MIT", nil)

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	detectedLicense, err := syncer.CheckGitHubLicense("https://github.com/owner/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if detectedLicense != "MIT" {
		t.Errorf("Expected MIT license, got '%s'", detectedLicense)
	}
}

// ============================================================================
// License Compliance Error Tests
// ============================================================================

func TestCheckCompliance_RejectedLicense(t *testing.T) {
	ctrl, _, fs, _, _, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: License checker returns GPL-3.0 (not allowed)
	license.EXPECT().CheckLicense("https://github.com/owner/gpl-repo").Return("GPL-3.0", nil)
	license.EXPECT().IsAllowed("GPL-3.0").Return(false)

	// Mock: User rejects the license
	mockUI := &capturingUICallback{confirmResp: false}
	licenseService := NewLicenseService(license, fs, "vendor", mockUI)

	// Execute
	detectedLicense, err := licenseService.CheckCompliance("https://github.com/owner/gpl-repo")

	// Verify: Should error when user rejects
	if err == nil {
		t.Fatal("Expected error when user rejects non-allowed license")
	}
	if err.Error() != ErrComplianceFailed {
		t.Errorf("Expected '%s', got '%s'", ErrComplianceFailed, err.Error())
	}
	if detectedLicense != "" {
		t.Errorf("Expected empty license on rejection, got '%s'", detectedLicense)
	}
}

func TestCheckCompliance_AcceptedNonStandardLicense(t *testing.T) {
	ctrl, _, fs, _, _, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: License checker returns GPL-3.0 (not allowed)
	license.EXPECT().CheckLicense("https://github.com/owner/gpl-repo").Return("GPL-3.0", nil)
	license.EXPECT().IsAllowed("GPL-3.0").Return(false)

	// Mock: User accepts the license
	mockUI := &capturingUICallback{confirmResp: true}
	licenseService := NewLicenseService(license, fs, "vendor", mockUI)

	// Execute
	detectedLicense, err := licenseService.CheckCompliance("https://github.com/owner/gpl-repo")

	// Verify: Should succeed when user accepts
	if err != nil {
		t.Fatalf("Expected success when user accepts, got error: %v", err)
	}
	if detectedLicense != "GPL-3.0" {
		t.Errorf("Expected 'GPL-3.0', got '%s'", detectedLicense)
	}
}

func TestCheckCompliance_DetectionFailureFallsBackToUnknown(t *testing.T) {
	ctrl, _, fs, _, _, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: License detection fails
	license.EXPECT().CheckLicense("https://github.com/owner/repo").Return("", fmt.Errorf("API error"))
	license.EXPECT().IsAllowed("UNKNOWN").Return(false)

	// Mock: User rejects unknown license
	mockUI := &capturingUICallback{confirmResp: false}
	licenseService := NewLicenseService(license, fs, "vendor", mockUI)

	// Execute
	detectedLicense, err := licenseService.CheckCompliance("https://github.com/owner/repo")

	// Verify: Should use UNKNOWN when detection fails
	if err == nil {
		t.Fatal("Expected error when user rejects UNKNOWN license")
	}
	if detectedLicense != "" {
		t.Errorf("Expected empty license on rejection, got '%s'", detectedLicense)
	}
}

func TestCheckCompliance_AllowedLicenseShowsCompliance(t *testing.T) {
	ctrl, _, fs, _, _, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: License checker returns MIT (allowed)
	license.EXPECT().CheckLicense("https://github.com/owner/repo").Return("MIT", nil)
	license.EXPECT().IsAllowed("MIT").Return(true)

	mockUI := &capturingUICallback{}
	licenseService := NewLicenseService(license, fs, "vendor", mockUI)

	// Execute
	detectedLicense, err := licenseService.CheckCompliance("https://github.com/owner/repo")

	// Verify: Should succeed and show compliance
	if err != nil {
		t.Fatalf("Expected success for allowed license, got error: %v", err)
	}
	if detectedLicense != "MIT" {
		t.Errorf("Expected 'MIT', got '%s'", detectedLicense)
	}
	if mockUI.licenseMsg != "MIT" {
		t.Errorf("Expected ShowLicenseCompliance('MIT'), got '%s'", mockUI.licenseMsg)
	}
}
