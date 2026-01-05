package core

import (
	"github.com/EmundoT/git-vendor/internal/core/providers"
)

// MultiPlatformLicenseChecker implements LicenseChecker interface
// with support for multiple git hosting platforms
type MultiPlatformLicenseChecker struct {
	registry        *providers.ProviderRegistry
	githubChecker   *GitHubLicenseChecker
	gitlabChecker   *GitLabAPIChecker
	fallbackChecker *FallbackLicenseChecker
	allowedLicenses []string
}

// NewMultiPlatformLicenseChecker creates a new multi-platform license checker
func NewMultiPlatformLicenseChecker(
	registry *providers.ProviderRegistry,
	fs FileSystem,
	gitClient GitClient,
	allowedLicenses []string,
) *MultiPlatformLicenseChecker {
	return &MultiPlatformLicenseChecker{
		registry:        registry,
		githubChecker:   NewGitHubLicenseChecker(nil, allowedLicenses),
		gitlabChecker:   NewGitLabAPIChecker(),
		fallbackChecker: NewFallbackLicenseChecker(fs, gitClient),
		allowedLicenses: allowedLicenses,
	}
}

// CheckLicense detects license using platform-specific API or fallback
//
// Strategy:
//  1. Detect provider from URL (GitHub, GitLab, Bitbucket, or generic)
//  2. Try platform-specific API if available (GitHub, GitLab)
//  3. If API fails or unavailable, fall back to reading LICENSE file
//  4. Return normalized SPDX license identifier
func (c *MultiPlatformLicenseChecker) CheckLicense(url string) (string, error) {
	provider := c.registry.DetectProvider(url)

	// Try platform-specific API first
	var license string
	var err error

	switch provider.Name() {
	case "github":
		// Try GitHub API
		license, err = c.githubChecker.CheckLicense(url)
		if err == nil && license != "" && license != "UNKNOWN" {
			return license, nil
		}
		// If API failed, fall through to fallback

	case "gitlab":
		// Try GitLab API
		license, err = c.gitlabChecker.CheckLicense(url)
		if err == nil && license != "" && license != "UNKNOWN" {
			return license, nil
		}
		// If API failed, fall through to fallback

	default:
		// For Bitbucket and generic providers, skip directly to fallback
		// (no API available)
	}

	// Fall back to reading LICENSE file directly
	license, err = c.fallbackChecker.CheckLicense(url)
	if err != nil {
		// Fallback also failed - return UNKNOWN but don't hard fail
		// (license detection is best-effort)
		return "UNKNOWN", nil
	}

	return license, nil
}

// IsAllowed checks if the given license is in the allowed list
func (c *MultiPlatformLicenseChecker) IsAllowed(license string) bool {
	for _, allowed := range c.allowedLicenses {
		if license == allowed {
			return true
		}
	}
	return false
}

// GetProviderName returns the detected provider name for debugging
func (c *MultiPlatformLicenseChecker) GetProviderName(url string) string {
	provider := c.registry.DetectProvider(url)
	return provider.Name()
}
