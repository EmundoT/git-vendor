package providers

import (
	"fmt"
	"regexp"
	"strings"
)

// GitLabProvider handles GitLab.com and self-hosted GitLab instances
type GitLabProvider struct{}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider() *GitLabProvider {
	return &GitLabProvider{}
}

// Name returns the provider identifier
func (p *GitLabProvider) Name() string {
	return "gitlab"
}

// Supports returns true if the URL is likely a GitLab URL
// Detection strategy:
// 1. Contains "gitlab.com" (gitlab.com URLs)
// 2. Contains "/-/blob/" or "/-/tree/" (GitLab-specific URL pattern)
func (p *GitLabProvider) Supports(url string) bool {
	cleaned := cleanURL(url)
	return strings.Contains(cleaned, "gitlab.com") ||
		strings.Contains(cleaned, "/-/blob/") ||
		strings.Contains(cleaned, "/-/tree/")
}

// ParseURL extracts repository, ref, and path from GitLab URLs
//
// GitLab URL patterns:
//   - gitlab.com/owner/repo
//   - gitlab.com/owner/repo/-/blob/main/path/file.go
//   - gitlab.com/owner/group/subgroup/repo/-/blob/v1.0.0/lib/util.go (nested groups)
//   - gitlab.example.com/team/project/-/blob/dev/README.md (self-hosted)
//
// Key differences from GitHub:
//   - Uses /-/blob/ instead of /blob/ (note the dash!)
//   - Supports nested groups with unlimited depth
//   - Self-hosted instances are common
func (p *GitLabProvider) ParseURL(rawURL string) (string, string, string, error) {
	cleaned := cleanURL(rawURL)

	// Pattern: [protocol://]host/path/to/repo/-/(blob|tree)/ref/path
	// Note: repo path can have arbitrary depth due to GitLab's group/subgroup structure
	deepPattern := `^(https?://)?([^/]+)/(.+?)/-/(blob|tree)/([^/]+)/(.+)$`
	re := regexp.MustCompile(deepPattern)
	matches := re.FindStringSubmatch(cleaned)

	if matches != nil {
		protocol := matches[1]
		if protocol == "" {
			protocol = "https://"
		}
		host := matches[2]        // gitlab.com or gitlab.example.com
		projectPath := matches[3] // owner/repo or owner/group/subgroup/repo
		// matches[4] is blob|tree (not needed)
		ref := matches[5]
		path := matches[6]

		baseURL := protocol + host + "/" + projectPath
		return baseURL, ref, path, nil
	}

	// Basic pattern: [protocol://]host/path/to/repo
	// This handles URLs without blob/tree links
	basicPattern := `^(https?://)?(.+)$`
	basicRe := regexp.MustCompile(basicPattern)
	basicMatches := basicRe.FindStringSubmatch(cleaned)

	if basicMatches != nil {
		protocol := basicMatches[1]
		if protocol == "" {
			protocol = "https://"
		}
		remaining := basicMatches[2]

		// Clean up trailing slash and .git
		remaining = strings.TrimSuffix(remaining, "/")
		remaining = strings.TrimSuffix(remaining, ".git")

		baseURL := protocol + remaining
		return baseURL, "", "", nil
	}

	return "", "", "", fmt.Errorf("invalid GitLab URL format")
}
