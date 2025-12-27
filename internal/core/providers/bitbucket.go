package providers

import (
	"fmt"
	"regexp"
	"strings"
)

// BitbucketProvider handles Bitbucket.org URLs
type BitbucketProvider struct{}

// NewBitbucketProvider creates a new Bitbucket provider
func NewBitbucketProvider() *BitbucketProvider {
	return &BitbucketProvider{}
}

// Name returns the provider identifier
func (p *BitbucketProvider) Name() string {
	return "bitbucket"
}

// Supports returns true if the URL contains bitbucket.org
func (p *BitbucketProvider) Supports(url string) bool {
	cleaned := cleanURL(url)
	return strings.Contains(cleaned, "bitbucket.org")
}

// ParseURL extracts repository, ref, and path from Bitbucket URLs
//
// Bitbucket URL patterns:
//   - bitbucket.org/owner/repo
//   - bitbucket.org/owner/repo/src/main/path/file.py
//   - bitbucket.org/owner/repo/src/v1.0.0/src/components/
//
// Key differences from GitHub/GitLab:
//   - Uses /src/ instead of /blob/ or /-/blob/
//   - Same path segment for both files and directories (no blob/tree distinction)
func (p *BitbucketProvider) ParseURL(rawURL string) (string, string, string, error) {
	cleaned := cleanURL(rawURL)

	// Pattern: bitbucket.org/owner/repo/src/ref/path
	deepPattern := `^(https?://)?bitbucket\.org/([^/]+)/([^/]+)/src/([^/]+)/(.+)$`
	re := regexp.MustCompile(deepPattern)
	matches := re.FindStringSubmatch(cleaned)

	if matches != nil {
		owner := matches[2]
		repo := matches[3]
		ref := matches[4]
		path := matches[5]

		baseURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
		return baseURL, ref, path, nil
	}

	// Basic pattern: bitbucket.org/owner/repo
	basicPattern := `^(https?://)?bitbucket\.org/([^/]+)/([^/]+)$`
	basicRe := regexp.MustCompile(basicPattern)
	basicMatches := basicRe.FindStringSubmatch(cleaned)

	if basicMatches != nil {
		owner := basicMatches[2]
		repo := strings.TrimSuffix(basicMatches[3], ".git")

		baseURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
		return baseURL, "", "", nil
	}

	return "", "", "", fmt.Errorf("invalid Bitbucket URL format")
}
