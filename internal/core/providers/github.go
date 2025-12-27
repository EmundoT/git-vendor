package providers

import (
	"regexp"
	"strings"
)

// GitHubProvider handles GitHub and GitHub Enterprise URLs
type GitHubProvider struct{}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

// Name returns the provider identifier
func (p *GitHubProvider) Name() string {
	return "github"
}

// Supports returns true if the URL contains github.com
// Also supports GitHub Enterprise instances
func (p *GitHubProvider) Supports(url string) bool {
	cleaned := cleanURL(url)
	return strings.Contains(cleaned, "github.com")
}

// ParseURL extracts repository, ref, and path from GitHub URLs
// This implementation maintains exact compatibility with the original ParseSmartURL
// from git_operations.go:179-191
func (p *GitHubProvider) ParseURL(rawURL string) (string, string, string, error) {
	rawURL = cleanURL(rawURL)

	// Pattern: github.com/owner/repo(/blob|/tree)/ref/path
	// This is the EXACT regex from the original implementation
	reDeep := regexp.MustCompile(`(github\.com/[^/]+/[^/]+)/(blob|tree)/([^/]+)/(.+)`)
	matches := reDeep.FindStringSubmatch(rawURL)

	if len(matches) == 5 {
		// Deep link with ref and path
		baseURL := "https://" + matches[1]
		ref := matches[3]
		path := matches[4]
		return baseURL, ref, path, nil
	}

	// Basic URL without ref/path
	base := strings.TrimSuffix(rawURL, "/")
	base = strings.TrimSuffix(base, ".git")

	// Add https:// prefix if not present
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}

	return base, "", "", nil
}

// cleanURL trims whitespace and backslashes
// This is the EXACT implementation from git_operations.go:194-196
func cleanURL(raw string) string {
	return strings.TrimLeft(strings.TrimSpace(raw), "\\")
}
