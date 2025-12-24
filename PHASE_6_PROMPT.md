# Phase 6: Multi-Platform Git Support

**Prerequisites:** Phase 5 complete (CI/CD in place)
**Goal:** Support GitLab, Bitbucket, and generic git hosting beyond GitHub
**Priority:** MEDIUM - Expands user base significantly
**Estimated Effort:** 8-12 hours

---

## Current State

**What Works:**
- ✅ GitHub URL parsing and smart URL detection
- ✅ GitHub API license detection
- ✅ All git operations work with any git host

**Limitations:**
- ❌ Smart URL parsing only works with GitHub URLs
- ❌ License detection requires GitHub API
- ❌ Documentation implies GitHub-only
- ❌ No GitLab/Bitbucket provider support

**User Impact:**
```bash
# Currently works:
git-vendor add https://github.com/owner/repo/blob/main/file.go

# Currently fails:
git-vendor add https://gitlab.com/owner/repo/-/blob/main/file.go  # URL parse fails
git-vendor add https://bitbucket.org/owner/repo/src/main/file.go  # URL parse fails
```

---

## Goals

1. **Abstract Git Hosting** - Create provider interface for GitHub, GitLab, Bitbucket
2. **Universal URL Parsing** - Support all major git hosting platforms
3. **Multi-Platform License Detection** - API support for GitLab and Bitbucket
4. **Fallback License Detection** - Manual LICENSE file reading when API unavailable
5. **Provider Auto-Detection** - Automatically detect host from URL
6. **Self-Hosted Support** - Work with custom GitLab/GitHub instances

---

## Architecture Design

### New Interface: GitHostingProvider

Create `internal/core/git_hosting.go`:

```go
package core

import "git-vendor/internal/types"

// GitHostingProvider abstracts operations specific to git hosting platforms
type GitHostingProvider interface {
	// Name returns the provider name ("github", "gitlab", "bitbucket", "generic")
	Name() string

	// ParseURL extracts repository info from platform-specific URLs
	// Returns: baseURL, ref, path, error
	ParseURL(rawURL string) (string, string, string, error)

	// DetectLicense attempts to detect the repository's license
	// Returns: license identifier (SPDX), error
	DetectLicense(repoURL string) (string, error)

	// Supports returns true if this provider can handle the given URL
	Supports(url string) bool
}

// GitHostingRegistry manages available providers
type GitHostingRegistry struct {
	providers []GitHostingProvider
	fallback  GitHostingProvider
}

func NewGitHostingRegistry() *GitHostingRegistry {
	return &GitHostingRegistry{
		providers: []GitHostingProvider{
			NewGitHubProvider(),
			NewGitLabProvider(),
			NewBitbucketProvider(),
		},
		fallback: NewGenericProvider(),
	}
}

func (r *GitHostingRegistry) DetectProvider(url string) GitHostingProvider {
	for _, provider := range r.providers {
		if provider.Supports(url) {
			return provider
		}
	}
	return r.fallback
}
```

---

## Implementation Steps

### 1. GitHub Provider (Refactor Existing)

Create `internal/core/providers/github.go`:

```go
package providers

import (
	"fmt"
	"regexp"
	"strings"
)

type GitHubProvider struct {
	apiClient *GitHubAPIClient
}

func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{
		apiClient: NewGitHubAPIClient(),
	}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) Supports(url string) bool {
	return strings.Contains(url, "github.com")
}

func (p *GitHubProvider) ParseURL(rawURL string) (string, string, string, error) {
	// Current logic from ParseSmartURL in git_operations.go
	cleaned := strings.TrimSpace(rawURL)
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "https://")
	cleaned = strings.TrimPrefix(cleaned, "www.")
	cleaned = strings.TrimSuffix(cleaned, ".git")

	// Pattern: github.com/owner/repo(/blob|/tree)/ref/path
	pattern := `^github\.com/([^/]+)/([^/]+)(?:/(?:blob|tree)/([^/]+)/(.+))?$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(cleaned)

	if matches == nil {
		// Try basic pattern: github.com/owner/repo
		basicPattern := `^github\.com/([^/]+)/([^/]+)$`
		basicRe := regexp.MustCompile(basicPattern)
		basicMatches := basicRe.FindStringSubmatch(cleaned)
		if basicMatches == nil {
			return "", "", "", fmt.Errorf("invalid GitHub URL format")
		}
		owner := basicMatches[1]
		repo := basicMatches[2]
		return fmt.Sprintf("https://github.com/%s/%s", owner, repo), "", "", nil
	}

	owner := matches[1]
	repo := matches[2]
	ref := matches[3]
	path := matches[4]

	baseURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	return baseURL, ref, path, nil
}

func (p *GitHubProvider) DetectLicense(repoURL string) (string, error) {
	return p.apiClient.FetchLicense(repoURL)
}

// GitHubAPIClient handles GitHub API interactions
type GitHubAPIClient struct {
	token      string
	httpClient *http.Client
}

func NewGitHubAPIClient() *GitHubAPIClient {
	return &GitHubAPIClient{
		token:      os.Getenv("GITHUB_TOKEN"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitHubAPIClient) FetchLicense(repoURL string) (string, error) {
	// Move existing logic from github_client.go
	// Add token authentication if available
	// Add retry logic with exponential backoff
	// ...
}
```

### 2. GitLab Provider

Create `internal/core/providers/gitlab.go`:

```go
package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type GitLabProvider struct {
	apiClient *GitLabAPIClient
}

func NewGitLabProvider() *GitLabProvider {
	return &GitLabProvider{
		apiClient: NewGitLabAPIClient(),
	}
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) Supports(url string) bool {
	return strings.Contains(url, "gitlab.com")
}

func (p *GitLabProvider) ParseURL(rawURL string) (string, string, string, error) {
	cleaned := strings.TrimSpace(rawURL)
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "https://")
	cleaned = strings.TrimPrefix(cleaned, "www.")

	// Pattern: gitlab.com/owner/repo/-/blob/ref/path
	// or: gitlab.com/owner/group/repo/-/blob/ref/path (subgroups)
	pattern := `^gitlab\.com/(.+?)/-/(?:blob|tree)/([^/]+)/(.+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(cleaned)

	if matches != nil {
		projectPath := matches[1] // owner/repo or owner/group/repo
		ref := matches[2]
		path := matches[3]
		baseURL := fmt.Sprintf("https://gitlab.com/%s", projectPath)
		return baseURL, ref, path, nil
	}

	// Try basic pattern: gitlab.com/owner/repo
	basicPattern := `^gitlab\.com/(.+)$`
	basicRe := regexp.MustCompile(basicPattern)
	basicMatches := basicRe.FindStringSubmatch(cleaned)
	if basicMatches == nil {
		return "", "", "", fmt.Errorf("invalid GitLab URL format")
	}

	projectPath := basicMatches[1]
	baseURL := fmt.Sprintf("https://gitlab.com/%s", projectPath)
	return baseURL, "", "", nil
}

func (p *GitLabProvider) DetectLicense(repoURL string) (string, error) {
	return p.apiClient.FetchLicense(repoURL)
}

// GitLabAPIClient handles GitLab API interactions
type GitLabAPIClient struct {
	token      string
	httpClient *http.Client
}

func NewGitLabAPIClient() *GitLabAPIClient {
	return &GitLabAPIClient{
		token:      os.Getenv("GITLAB_TOKEN"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitLabAPIClient) FetchLicense(repoURL string) (string, error) {
	// Extract project path from URL
	// GitLab API: GET /projects/:id/licenses
	// Documentation: https://docs.gitlab.com/ee/api/templates/licenses.html

	projectPath := extractGitLabProjectPath(repoURL)
	// URL encode the project path
	encodedPath := url.PathEscape(projectPath)

	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", encodedPath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query GitLab API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var project struct {
		License struct {
			Key string `json:"key"`
		} `json:"license"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return "", err
	}

	// Convert GitLab license keys to SPDX format
	return normalizeLicenseKey(project.License.Key), nil
}

func extractGitLabProjectPath(repoURL string) string {
	// Extract "owner/repo" or "owner/group/repo" from URL
	cleaned := strings.TrimPrefix(repoURL, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "gitlab.com/")
	return cleaned
}
```

### 3. Bitbucket Provider

Create `internal/core/providers/bitbucket.go`:

```go
package providers

import (
	"fmt"
	"regexp"
	"strings"
)

type BitbucketProvider struct {
	apiClient *BitbucketAPIClient
}

func NewBitbucketProvider() *BitbucketProvider {
	return &BitbucketProvider{
		apiClient: NewBitbucketAPIClient(),
	}
}

func (p *BitbucketProvider) Name() string {
	return "bitbucket"
}

func (p *BitbucketProvider) Supports(url string) bool {
	return strings.Contains(url, "bitbucket.org")
}

func (p *BitbucketProvider) ParseURL(rawURL string) (string, string, string, error) {
	cleaned := strings.TrimSpace(rawURL)
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "https://")
	cleaned = strings.TrimPrefix(cleaned, "www.")

	// Pattern: bitbucket.org/owner/repo/src/ref/path
	pattern := `^bitbucket\.org/([^/]+)/([^/]+)/src/([^/]+)/(.+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(cleaned)

	if matches != nil {
		owner := matches[1]
		repo := matches[2]
		ref := matches[3]
		path := matches[4]
		baseURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
		return baseURL, ref, path, nil
	}

	// Try basic pattern: bitbucket.org/owner/repo
	basicPattern := `^bitbucket\.org/([^/]+)/([^/]+)$`
	basicRe := regexp.MustCompile(basicPattern)
	basicMatches := basicRe.FindStringSubmatch(cleaned)
	if basicMatches == nil {
		return "", "", "", fmt.Errorf("invalid Bitbucket URL format")
	}

	owner := basicMatches[1]
	repo := basicMatches[2]
	baseURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
	return baseURL, "", "", nil
}

func (p *BitbucketProvider) DetectLicense(repoURL string) (string, error) {
	// Bitbucket doesn't have a license detection API
	// Fall back to manual LICENSE file reading
	return "", fmt.Errorf("bitbucket does not support API license detection, will read LICENSE file manually")
}
```

### 4. Generic Provider (Fallback)

Create `internal/core/providers/generic.go`:

```go
package providers

import "fmt"

type GenericProvider struct{}

func NewGenericProvider() *GenericProvider {
	return &GenericProvider{}
}

func (p *GenericProvider) Name() string {
	return "generic"
}

func (p *GenericProvider) Supports(url string) bool {
	// Generic provider accepts everything
	return true
}

func (p *GenericProvider) ParseURL(rawURL string) (string, string, string, error) {
	// For generic git URLs, don't try to parse ref/path
	// Just return the URL as-is
	return rawURL, "", "", nil
}

func (p *GenericProvider) DetectLicense(repoURL string) (string, error) {
	// Generic provider can't detect license via API
	// Will need to clone and read LICENSE file
	return "", fmt.Errorf("no API available for license detection, will read LICENSE file manually")
}
```

### 5. Fallback License Detection

Update `internal/core/license_service.go`:

```go
// FallbackLicenseDetection reads LICENSE file directly from git repository
func (s *VendorSyncer) FallbackLicenseDetection(repoURL, tempDir string) (string, error) {
	// List of common license file names
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "COPYING.txt"}

	for _, filename := range licenseFiles {
		// Use git show to read file without checking out
		cmd := exec.Command("git", "show", "HEAD:"+filename)
		cmd.Dir = tempDir
		output, err := cmd.Output()
		if err != nil {
			continue // File doesn't exist, try next
		}

		// Parse license from file content
		license := parseLicenseFromContent(string(output))
		if license != "" {
			return license, nil
		}
	}

	return "", fmt.Errorf("could not detect license")
}

func parseLicenseFromContent(content string) string {
	content = strings.ToLower(content)

	// Simple pattern matching
	if strings.Contains(content, "mit license") {
		return "MIT"
	}
	if strings.Contains(content, "apache license") && strings.Contains(content, "version 2.0") {
		return "Apache-2.0"
	}
	if strings.Contains(content, "bsd 3-clause") || strings.Contains(content, "bsd-3-clause") {
		return "BSD-3-Clause"
	}
	if strings.Contains(content, "bsd 2-clause") || strings.Contains(content, "bsd-2-clause") {
		return "BSD-2-Clause"
	}
	if strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 3") {
		return "GPL-3.0"
	}

	return ""
}
```

### 6. Update VendorSyncer

Integrate provider registry into `vendor_syncer.go`:

```go
type VendorSyncer struct {
	configStore    ConfigStore
	lockStore      LockStore
	gitClient      GitClient
	fs             FileSystem
	licenseChecker LicenseChecker
	hostingRegistry *GitHostingRegistry  // NEW
	rootDir        string
	ui             UICallback
}

func NewVendorSyncer(...) *VendorSyncer {
	return &VendorSyncer{
		// ... existing fields
		hostingRegistry: NewGitHostingRegistry(),
	}
}

func (s *VendorSyncer) ParseSmartURL(rawURL string) (string, string, string) {
	provider := s.hostingRegistry.DetectProvider(rawURL)
	baseURL, ref, path, err := provider.ParseURL(rawURL)
	if err != nil {
		// Return raw URL if parsing fails
		return rawURL, "", ""
	}
	return baseURL, ref, path
}

func (s *VendorSyncer) CheckLicense(repoURL string) (string, error) {
	provider := s.hostingRegistry.DetectProvider(repoURL)

	// Try provider API first
	license, err := provider.DetectLicense(repoURL)
	if err == nil && license != "" {
		return license, nil
	}

	// Fall back to reading LICENSE file
	tempDir, err := s.fs.CreateTemp("", "license-check-*")
	if err != nil {
		return "", err
	}
	defer s.fs.RemoveAll(tempDir)

	// Shallow clone
	if err := s.gitClient.Clone(repoURL, tempDir, 1); err != nil {
		return "", err
	}

	return s.FallbackLicenseDetection(repoURL, tempDir)
}
```

### 7. Update Tests

Create `internal/core/providers/providers_test.go`:

```go
package providers

import "testing"

func TestGitHubProvider_ParseURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantBase    string
		wantRef     string
		wantPath    string
		wantErr     bool
	}{
		{
			name:     "GitHub blob URL",
			url:      "https://github.com/owner/repo/blob/main/src/file.go",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "src/file.go",
			wantErr:  false,
		},
		// ... more test cases
	}

	provider := NewGitHubProvider()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := provider.ParseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if base != tt.wantBase || ref != tt.wantRef || path != tt.wantPath {
				t.Errorf("ParseURL() = (%v, %v, %v), want (%v, %v, %v)",
					base, ref, path, tt.wantBase, tt.wantRef, tt.wantPath)
			}
		})
	}
}

func TestGitLabProvider_ParseURL(t *testing.T) {
	// Similar test structure for GitLab
}

func TestBitbucketProvider_ParseURL(t *testing.T) {
	// Similar test structure for Bitbucket
}
```

### 8. Update Documentation

Update `README.md`:

```markdown
## Supported Platforms

git-vendor supports the following git hosting platforms:

### Full Support (with license detection)
- **GitHub** - github.com and GitHub Enterprise
  - Smart URL parsing for blob/tree links
  - API-based license detection
  - Authentication via `GITHUB_TOKEN` environment variable

- **GitLab** - gitlab.com and self-hosted instances
  - Smart URL parsing for blob/tree links
  - API-based license detection
  - Authentication via `GITLAB_TOKEN` environment variable

### Partial Support (manual license)
- **Bitbucket** - bitbucket.org
  - Smart URL parsing for src links
  - License detected from LICENSE file in repository

- **Generic Git** - Any git URL
  - Use standard git clone URLs
  - License detected from LICENSE file in repository

### Examples

```bash
# GitHub
git-vendor add https://github.com/owner/repo/blob/main/src/utils.go

# GitLab
git-vendor add https://gitlab.com/owner/repo/-/blob/main/src/utils.go

# Bitbucket
git-vendor add https://bitbucket.org/owner/repo/src/main/utils.go

# Generic (any git URL)
git-vendor add https://git.example.com/owner/repo.git
```

### Authentication

For private repositories or to avoid rate limits:

```bash
# GitHub
export GITHUB_TOKEN=ghp_your_token_here

# GitLab
export GITLAB_TOKEN=glpat_your_token_here

git-vendor add <url>
```
```

---

## Verification Checklist

After implementing Phase 6, verify:

- [ ] GitHub URLs still work (backward compatibility)
- [ ] GitLab URLs parse correctly
- [ ] Bitbucket URLs parse correctly
- [ ] Generic git URLs don't break
- [ ] GitHub license detection works
- [ ] GitLab license detection works
- [ ] Fallback license detection works
- [ ] Tests pass for all providers
- [ ] Documentation updated
- [ ] Token authentication works for GitHub and GitLab

**Test with real repositories:**
```bash
# GitHub
git-vendor add https://github.com/golang/go/blob/master/src/fmt/print.go

# GitLab
git-vendor add https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/api/api.rb

# Bitbucket
git-vendor add https://bitbucket.org/atlassian/python-bitbucket/src/master/setup.py

# Generic
git-vendor add https://git.kernel.org/pub/scm/git/git.git
```

---

## Expected Outcomes

**After Phase 6:**
- ✅ Support for 4 git hosting platforms (GitHub, GitLab, Bitbucket, Generic)
- ✅ Graceful fallback when API unavailable
- ✅ Token authentication for private repos
- ✅ Backward compatibility with existing GitHub workflows
- ✅ Expanded user base beyond GitHub users

**Metrics:**
- Provider detection: <1ms
- API license detection: <500ms
- Fallback license detection: <2s
- Test coverage: Maintain 60%+

---

## Next Steps

After Phase 6 completion:
- **Phase 7:** Enhanced testing and quality (integration tests, 70% coverage)
- **Phase 8:** Advanced features (update checker, parallel sync, vendor groups)
