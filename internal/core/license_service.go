package core

import (
	"fmt"
	"path/filepath"
)

// LicenseServiceInterface defines the contract for license checking and file management.
// This interface enables mocking in tests and potential alternative license backends.
type LicenseServiceInterface interface {
	CheckCompliance(url string) (string, error)
	CopyLicense(tempDir, vendorName string) error
	GetLicensePath(vendorName string) string
	CheckLicense(url string) (string, error)
}

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

// CheckCompliance checks license compliance for a URL
// Returns the detected license or error
func (s *LicenseService) CheckCompliance(url string) (string, error) {
	detectedLicense, err := s.licenseChecker.CheckLicense(url)
	if err != nil {
		// If detection failed, use UNKNOWN
		detectedLicense = "UNKNOWN"
	}

	// Check if license is allowed
	if !s.licenseChecker.IsAllowed(detectedLicense) {
		// Ask user for confirmation
		if !s.ui.AskConfirmation(
			fmt.Sprintf("Accept %s License?", detectedLicense),
			"This license is not in the allowed list. Continue anyway?",
		) {
			return "", ErrComplianceFailed
		}
	} else {
		// Show compliance success
		s.ui.ShowLicenseCompliance(detectedLicense)
	}

	return detectedLicense, nil
}

// CopyLicense copies license file from temp repo to vendor/licenses
func (s *LicenseService) CopyLicense(tempDir, vendorName string) error {
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
		return err
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
