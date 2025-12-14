package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"
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
		return "", fmt.Errorf("invalid URL format")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/license", matches[1], matches[2])

	// Retry with exponential backoff for rate limit errors
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "git-vendor-cli")

		// Add GitHub token if available (increases rate limit from 60/hr to 5000/hr)
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "token "+token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Handle rate limiting
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			resp.Body.Close()
			lastErr = fmt.Errorf("GitHub API rate limit exceeded. Set GITHUB_TOKEN environment variable to increase rate limit (60/hr → 5000/hr)")
			if attempt < 2 {
				continue // Retry with backoff
			}
			return "", lastErr
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return "NONE", nil
		}

		var res struct {
			License struct {
				SpdxID string `json:"spdx_id"`
			} `json:"license"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			resp.Body.Close()
			return "", err
		}
		resp.Body.Close()

		if res.License.SpdxID == "" {
			return "UNKNOWN", nil
		}

		return res.License.SpdxID, nil
	}

	return "", lastErr
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
