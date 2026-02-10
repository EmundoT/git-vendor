package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/EmundoT/git-vendor/internal/types"
)

// LicenseServiceInterface defines the contract for license checking and file management.
// LicenseServiceInterface enables mocking in tests and alternative license backends.
type LicenseServiceInterface interface {
	CheckCompliance(url string) (string, error)
	CopyLicense(tempDir, vendorName string) error
	GetLicensePath(vendorName string) string
	CheckLicense(url string) (string, error)
}

// Compile-time interface satisfaction check.
var _ LicenseServiceInterface = (*LicenseService)(nil)

// LicenseService handles license checking and file management
type LicenseService struct {
	licenseChecker LicenseChecker
	fs             FileSystem
	rootDir        string
	ui             UICallback
}

// NewLicenseService creates a new LicenseService
func NewLicenseService(licenseChecker LicenseChecker, fs FileSystem, rootDir string, ui UICallback) *LicenseService {
	return &LicenseService{
		licenseChecker: licenseChecker,
		fs:             fs,
		rootDir:        rootDir,
		ui:             ui,
	}
}

// CheckCompliance checks license compliance for a URL.
// When a .git-vendor-policy.yml file exists on disk, CheckCompliance evaluates the
// detected license against the policy's deny/warn/allow lists.
// Denied licenses block the add (no user override). Warned licenses prompt
// for confirmation. Allowed licenses pass silently.
// A malformed policy file returns an error (no silent fallback).
// When no policy file exists, CheckCompliance falls back to the legacy
// AllowedLicenses list with a confirmation prompt for unlisted licenses.
func (s *LicenseService) CheckCompliance(url string) (string, error) {
	detectedLicense, err := s.licenseChecker.CheckLicense(url)
	if err != nil {
		// If detection failed, use UNKNOWN
		detectedLicense = "UNKNOWN"
	}

	// Check if a policy file exists on disk (not a heuristic — actual stat)
	_, statErr := os.Stat(PolicyFile)
	if statErr == nil {
		// Policy file exists — load and enforce it
		policy, policyErr := LoadLicensePolicy(PolicyFile)
		if policyErr != nil {
			return "", fmt.Errorf("license policy error: %w", policyErr)
		}
		return s.checkWithPolicy(detectedLicense, &policy)
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("check policy file: %w", statErr)
	}

	// No policy file — legacy AllowedLicenses check
	if !s.licenseChecker.IsAllowed(detectedLicense) {
		if !s.ui.AskConfirmation(
			fmt.Sprintf("Accept %s License?", detectedLicense),
			"This license is not in the allowed list. Continue anyway?",
		) {
			return "", ErrComplianceFailed
		}
	} else {
		s.ui.ShowLicenseCompliance(detectedLicense)
	}

	return detectedLicense, nil
}

// checkWithPolicy evaluates a license using the policy file's deny/warn/allow semantics.
// Denied licenses are hard-blocked (no user override). Warned licenses prompt for confirmation.
func (s *LicenseService) checkWithPolicy(license string, policy *types.LicensePolicy) (string, error) {
	svc := NewLicensePolicyService(policy, PolicyFile, nil, nil)
	decision := svc.Evaluate(license)

	switch decision {
	case types.PolicyDeny:
		s.ui.ShowError("License Denied",
			fmt.Sprintf("%s is denied by license policy (%s)", license, PolicyFile))
		return "", ErrComplianceFailed

	case types.PolicyWarn:
		if !s.ui.AskConfirmation(
			fmt.Sprintf("License Warning: %s", license),
			fmt.Sprintf("This license triggers a policy warning (%s). Continue anyway?", PolicyFile),
		) {
			return "", ErrComplianceFailed
		}
		return license, nil

	default: // allow
		s.ui.ShowLicenseCompliance(license)
		return license, nil
	}
}

// CopyLicense copies license file from temp repo to .git-vendor/licenses.
// Validates vendorName to prevent path traversal via malicious vendor.yml entries.
func (s *LicenseService) CopyLicense(tempDir, vendorName string) error {
	// SEC-001: Validate vendorName before constructing filesystem path.
	// Without this check, a malicious vendor.yml with name: "../../../etc/cron.d/evil"
	// would write the license file outside the project directory.
	if err := ValidateVendorName(vendorName); err != nil {
		return fmt.Errorf("license copy blocked: %w", err)
	}

	// Find license file in temp directory
	var licenseSrc string
	for _, name := range LicenseFileNames {
		path := filepath.Join(tempDir, name)
		if _, err := s.fs.Stat(path); err == nil {
			licenseSrc = path
			break
		}
	}

	// If no license file found, return without error (optional license)
	if licenseSrc == "" {
		return nil
	}

	// Ensure license directory exists
	licenseDir := filepath.Join(s.rootDir, LicensesDir)
	if err := s.fs.MkdirAll(licenseDir, 0755); err != nil {
		return fmt.Errorf("CopyLicense: create license directory: %w", err)
	}

	// Copy license file
	dest := filepath.Join(licenseDir, vendorName+".txt")
	if _, err := s.fs.CopyFile(licenseSrc, dest); err != nil {
		return fmt.Errorf("failed to copy license from %s to %s: %w", licenseSrc, dest, err)
	}

	return nil
}

// GetLicensePath returns the path to a vendor's license file
func (s *LicenseService) GetLicensePath(vendorName string) string {
	return filepath.Join(s.rootDir, LicensesDir, vendorName+".txt")
}

// CheckLicense checks the license for a URL (delegates to checker)
func (s *LicenseService) CheckLicense(url string) (string, error) {
	return s.licenseChecker.CheckLicense(url)
}
