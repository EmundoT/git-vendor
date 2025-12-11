package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

// LicenseChecker checks repository licenses
type LicenseChecker interface {
	CheckLicense(url string) (string, error)
	IsAllowed(license string) bool
}

// GitHubLicenseChecker implements LicenseChecker for GitHub
type GitHubLicenseChecker struct {
	httpClient      *http.Client
	allowedLicenses []string
}

// NewGitHubLicenseChecker creates a new GitHubLicenseChecker
func NewGitHubLicenseChecker(httpClient *http.Client, allowedLicenses []string) *GitHubLicenseChecker {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GitHubLicenseChecker{
		httpClient:      httpClient,
		allowedLicenses: allowedLicenses,
	}
}

// CheckLicense queries GitHub API for repository license
func (c *GitHubLicenseChecker) CheckLicense(rawURL string) (string, error) {
	clean := cleanURL(rawURL)
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/\.]+)(\.git)?`)
	matches := re.FindStringSubmatch(clean)

	if len(matches) < 3 {
		return "", fmt.Errorf(ErrInvalidURL)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/license", matches[1], matches[2])

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "git-vendor-cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "NONE", nil
	}

	var res struct {
		License struct {
			SpdxID string `json:"spdx_id"`
		} `json:"license"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.License.SpdxID == "" {
		return "UNKNOWN", nil
	}

	return res.License.SpdxID, nil
}

// IsAllowed checks if a license is in the allowed list
func (c *GitHubLicenseChecker) IsAllowed(license string) bool {
	for _, l := range c.allowedLicenses {
		if license == l {
			return true
		}
	}
	return false
}
