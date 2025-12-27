package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GitLabAPIChecker implements license detection via GitLab API
type GitLabAPIChecker struct {
	httpClient *http.Client
	token      string
}

// NewGitLabAPIChecker creates a new GitLab API license checker
func NewGitLabAPIChecker() *GitLabAPIChecker {
	return &GitLabAPIChecker{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      os.Getenv("GITLAB_TOKEN"),
	}
}

// CheckLicense queries the GitLab API for license information
//
// GitLab API endpoint: GET /api/v4/projects/:id
// Documentation: https://docs.gitlab.com/ee/api/projects.html#get-single-project
//
// Supports both gitlab.com and self-hosted instances
func (c *GitLabAPIChecker) CheckLicense(repoURL string) (string, error) {
	projectPath, apiHost, err := extractGitLabProjectPath(repoURL)
	if err != nil {
		return "", err
	}

	// URL-encode the project path (GitLab API requirement)
	encodedPath := url.PathEscape(projectPath)

	// GitLab API endpoint
	apiURL := fmt.Sprintf("https://%s/api/v4/projects/%s", apiHost, encodedPath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	// Add authentication if token available
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 404 - repository not found or no license
	if resp.StatusCode == http.StatusNotFound {
		return "NONE", nil
	}

	// Handle other non-OK status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	// Parse response
	var project struct {
		License struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"license"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return "", fmt.Errorf("failed to parse GitLab API response: %w", err)
	}

	// Normalize license key to SPDX format
	return normalizeLicenseKey(project.License.Key), nil
}

// extractGitLabProjectPath extracts the project path and API host from a GitLab URL
// Returns: (projectPath, apiHost, error)
//
// Examples:
//   - https://gitlab.com/owner/repo → ("owner/repo", "gitlab.com", nil)
//   - https://gitlab.com/owner/group/repo → ("owner/group/repo", "gitlab.com", nil)
//   - https://gitlab.example.com/team/project → ("team/project", "gitlab.example.com", nil)
func extractGitLabProjectPath(repoURL string) (string, string, error) {
	// Remove protocol
	cleaned := strings.TrimPrefix(repoURL, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")

	// Find domain boundary (first slash)
	parts := strings.SplitN(cleaned, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitLab URL: missing project path")
	}

	apiHost := parts[0]
	projectPath := parts[1]

	// Clean up trailing slash and .git
	projectPath = strings.TrimSuffix(projectPath, "/")
	projectPath = strings.TrimSuffix(projectPath, ".git")

	return projectPath, apiHost, nil
}

// normalizeLicenseKey converts license keys to standard SPDX format
// GitLab and different platforms may use different casing or naming
func normalizeLicenseKey(key string) string {
	if key == "" {
		return "UNKNOWN"
	}

	lower := strings.ToLower(key)

	// Map common license keys to SPDX identifiers
	switch lower {
	case "mit":
		return "MIT"
	case "apache-2.0", "apache_2_0", "apache 2.0":
		return "Apache-2.0"
	case "bsd-3-clause", "bsd_3_clause", "bsd 3-clause":
		return "BSD-3-Clause"
	case "bsd-2-clause", "bsd_2_clause", "bsd 2-clause":
		return "BSD-2-Clause"
	case "gpl-3.0", "gpl_3_0", "gpl-3.0-only":
		return "GPL-3.0"
	case "gpl-2.0", "gpl_2_0", "gpl-2.0-only":
		return "GPL-2.0"
	case "mpl-2.0", "mpl_2_0", "mozilla public license 2.0":
		return "MPL-2.0"
	case "isc":
		return "ISC"
	case "unlicense":
		return "Unlicense"
	case "cc0-1.0", "cc0_1_0":
		return "CC0-1.0"
	default:
		// Return with first letter uppercase for consistency
		return strings.ToUpper(key[:1]) + key[1:]
	}
}
