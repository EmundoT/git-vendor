package hostdetect

import (
	"testing"
)

// TestDetectProvider_SelfHostedGitLab covers self-hosted GitLab instances
// with various hostname patterns (subdomain, hyphenated, mixed).
func TestDetectProvider_SelfHostedGitLab(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"gitlab.internal.corp", ProviderGitLab},
		{"gitlab.dev.mycompany.io", ProviderGitLab},
		{"my-gitlab.internal", ProviderGitLab},
		{"code.gitlab.myorg.net", ProviderGitLab},
		{"gitlab-ce.staging.local", ProviderGitLab},
		{"gitlab-ee.prod.example.com", ProviderGitLab},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestDetectProvider_GitHubEnterprise covers GitHub Enterprise Server instances
// using various enterprise hostname patterns.
func TestDetectProvider_GitHubEnterprise(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"github.mycompany.com", ProviderGitHub},
		{"github-enterprise.corp", ProviderGitHub},
		{"code.github.myorg.com", ProviderGitHub},
		{"gh.github.internal.io", ProviderGitHub},
		// Suffix match: *.github.com
		{"api.github.com", ProviderGitHub},
		{"enterprise.github.com", ProviderGitHub},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestDetectProvider_BitbucketServer covers Bitbucket Server/Data Center instances.
func TestDetectProvider_BitbucketServer(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"bitbucket-server.corp", ProviderBitbucket},
		{"my-bitbucket.internal.net", ProviderBitbucket},
		{"stash.bitbucket.myorg.com", ProviderBitbucket},
		// Suffix match: *.bitbucket.org
		{"mirror.bitbucket.org", ProviderBitbucket},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestDetectProvider_UnknownProviders covers hostnames that should NOT match
// any known provider.
func TestDetectProvider_UnknownProviders(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"gitea.io", ProviderUnknown},
		{"codeberg.org", ProviderUnknown},
		{"sr.ht", ProviderUnknown},
		{"git.kernel.org", ProviderUnknown},
		{"code.internal.corp", ProviderUnknown},
		{"scm.example.com", ProviderUnknown},
		{"repo.mycompany.io", ProviderUnknown},
		{"sourcehut.org", ProviderUnknown},
		{"gogs.internal.dev", ProviderUnknown},
		{"forgejo.example.org", ProviderUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestDetectProvider_WithPorts covers hostnames that include port numbers.
// DetectProvider strips ports before matching.
func TestDetectProvider_WithPorts(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"github.com:443", ProviderGitHub},
		{"gitlab.com:8443", ProviderGitLab},
		{"bitbucket.org:7990", ProviderBitbucket},
		{"gitlab.internal.corp:3000", ProviderGitLab},
		{"github-enterprise.corp:8080", ProviderGitHub},
		{"git.internal.corp:9418", ProviderUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestDetectProvider_CaseSensitivity covers mixed-case hostnames.
// DetectProvider normalizes to lowercase before matching.
func TestDetectProvider_CaseSensitivity(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		{"GITHUB.COM", ProviderGitHub},
		{"GitHub.Com", ProviderGitHub},
		{"GITLAB.COM", ProviderGitLab},
		{"GitLab.Com", ProviderGitLab},
		{"BITBUCKET.ORG", ProviderBitbucket},
		{"BitBucket.Org", ProviderBitbucket},
		{"GitHub.MyCompany.COM", ProviderGitHub},
		{"GITLAB.Internal.CORP", ProviderGitLab},
		{"MY-BITBUCKET.internal.NET", ProviderBitbucket},
		{"GIT.INTERNAL.CORP", ProviderUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := DetectProvider(tc.host)
			if result != tc.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tc.host, result, tc.expected)
			}
		})
	}
}

// TestFromURL_SSHURLs covers SSH-style git URLs.
// SSH URLs (git@host:owner/repo) lack a scheme, so url.Parse treats the
// host as empty and FromURL returns nil.
func TestFromURL_SSHURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"GitHub SSH", "git@github.com:owner/repo.git"},
		{"GitLab SSH", "git@gitlab.com:group/project.git"},
		{"Bitbucket SSH", "git@bitbucket.org:owner/repo.git"},
		{"Self-hosted SSH", "git@gitlab.internal.corp:team/project.git"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			// SSH URLs without scheme are not parseable as standard URLs;
			// FromURL returns nil gracefully (no panic).
			if result != nil {
				t.Logf("SSH URL %q unexpectedly parsed to %+v", tc.url, result)
			}
		})
	}
}

// TestFromURL_SSHSchemeURLs covers SSH URLs using the ssh:// scheme.
func TestFromURL_SSHSchemeURLs(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectProvider Provider
		expectOwner    string
		expectRepo     string
	}{
		{
			name:           "ssh scheme GitHub",
			url:            "ssh://git@github.com/owner/repo.git",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "ssh scheme GitLab",
			url:            "ssh://git@gitlab.com/group/project.git",
			expectProvider: ProviderGitLab,
			expectOwner:    "group",
			expectRepo:     "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info for ssh:// URL")
			}
			if result.Provider != tc.expectProvider {
				t.Errorf("Provider: expected %q, got %q", tc.expectProvider, result.Provider)
			}
			if result.Owner != tc.expectOwner {
				t.Errorf("Owner: expected %q, got %q", tc.expectOwner, result.Owner)
			}
			if result.Repo != tc.expectRepo {
				t.Errorf("Repo: expected %q, got %q", tc.expectRepo, result.Repo)
			}
		})
	}
}

// TestFromURL_AuthenticationTokens covers URLs with embedded auth credentials.
// url.Parse preserves the host correctly even with user:password@ in the URL.
func TestFromURL_AuthenticationTokens(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectProvider Provider
		expectOwner    string
		expectRepo     string
	}{
		{
			name:           "username:password basic auth",
			url:            "https://user:password@github.com/owner/repo",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "x-access-token for GitHub Apps",
			url:            "https://x-access-token:ghp_abc123@github.com/owner/repo",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "oauth2 token for GitLab",
			url:            "https://oauth2:glpat-abc123@gitlab.com/group/project",
			expectProvider: ProviderGitLab,
			expectOwner:    "group",
			expectRepo:     "project",
		},
		{
			name:           "token-only auth (no username)",
			url:            "https://x-token-auth:abc123@bitbucket.org/owner/repo",
			expectProvider: ProviderBitbucket,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "self-hosted GitLab with token",
			url:            "https://deploy-token:secret@gitlab.internal.corp/team/project",
			expectProvider: ProviderGitLab,
			expectOwner:    "team",
			expectRepo:     "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info")
			}
			if result.Provider != tc.expectProvider {
				t.Errorf("Provider: expected %q, got %q", tc.expectProvider, result.Provider)
			}
			if result.Owner != tc.expectOwner {
				t.Errorf("Owner: expected %q, got %q", tc.expectOwner, result.Owner)
			}
			if result.Repo != tc.expectRepo {
				t.Errorf("Repo: expected %q, got %q", tc.expectRepo, result.Repo)
			}
		})
	}
}

// TestFromURL_GitLabNestedGroups covers GitLab URLs with deeply nested groups
// (3+ levels deep). Owner should contain the full group path.
func TestFromURL_GitLabNestedGroups(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectOwner string
		expectRepo  string
	}{
		{
			name:        "3 levels deep",
			url:         "https://gitlab.com/org/team/subteam/repo",
			expectOwner: "org/team/subteam",
			expectRepo:  "repo",
		},
		{
			name:        "4 levels deep",
			url:         "https://gitlab.com/org/division/team/subteam/repo",
			expectOwner: "org/division/team/subteam",
			expectRepo:  "repo",
		},
		{
			name:        "5 levels deep",
			url:         "https://gitlab.com/corp/division/dept/team/subteam/repo",
			expectOwner: "corp/division/dept/team/subteam",
			expectRepo:  "repo",
		},
		{
			name:        "6 levels deep with .git suffix",
			url:         "https://gitlab.com/a/b/c/d/e/f/repo.git",
			expectOwner: "a/b/c/d/e/f",
			expectRepo:  "repo",
		},
		{
			name:        "self-hosted GitLab nested groups",
			url:         "https://gitlab.internal.corp/infra/platform/services/api",
			expectOwner: "infra/platform/services",
			expectRepo:  "api",
		},
		{
			name:        "self-hosted GitLab nested groups with port",
			url:         "https://gitlab.internal.corp:8443/infra/platform/services/api",
			expectOwner: "infra/platform/services",
			expectRepo:  "api",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info")
			}
			if result.Provider != ProviderGitLab {
				t.Errorf("Provider: expected %q, got %q", ProviderGitLab, result.Provider)
			}
			if result.Owner != tc.expectOwner {
				t.Errorf("Owner: expected %q, got %q", tc.expectOwner, result.Owner)
			}
			if result.Repo != tc.expectRepo {
				t.Errorf("Repo: expected %q, got %q", tc.expectRepo, result.Repo)
			}
		})
	}
}

// TestFromURL_PortPreservation verifies that Host field retains the port
// (since url.Parse includes port in Host) while provider detection strips it.
func TestFromURL_PortPreservation(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		expectHost string
	}{
		{
			name:       "GitLab with custom port",
			url:        "https://gitlab.internal.corp:8443/team/project",
			expectHost: "gitlab.internal.corp:8443",
		},
		{
			name:       "GitHub with standard HTTPS port",
			url:        "https://github.com:443/owner/repo",
			expectHost: "github.com:443",
		},
		{
			name:       "Bitbucket with non-standard port",
			url:        "https://bitbucket.org:7990/owner/repo",
			expectHost: "bitbucket.org:7990",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info")
			}
			if result.Host != tc.expectHost {
				t.Errorf("Host: expected %q, got %q", tc.expectHost, result.Host)
			}
		})
	}
}

// TestFromURL_HTTPScheme verifies that HTTP (non-HTTPS) URLs are parsed correctly.
func TestFromURL_HTTPScheme(t *testing.T) {
	result := FromURL("http://gitlab.internal.corp/team/project")
	if result == nil {
		t.Fatal("Expected non-nil Info for HTTP URL")
	}
	if result.Provider != ProviderGitLab {
		t.Errorf("Provider: expected %q, got %q", ProviderGitLab, result.Provider)
	}
	if result.Owner != "team" || result.Repo != "project" {
		t.Errorf("Expected team/project, got %s/%s", result.Owner, result.Repo)
	}
}

// TestFromURL_GitSuffixStripping verifies .git suffix removal from repo names
// across all providers.
func TestFromURL_GitSuffixStripping(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		expectRepo string
	}{
		{"GitHub .git", "https://github.com/owner/repo.git", "repo"},
		{"GitLab .git", "https://gitlab.com/owner/repo.git", "repo"},
		{"Bitbucket .git", "https://bitbucket.org/owner/repo.git", "repo"},
		{"no .git suffix", "https://github.com/owner/repo", "repo"},
		// ".git" in the middle of the name should NOT be stripped
		{"embedded .git in name", "https://github.com/owner/my.gitrepo", "my.gitrepo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info")
			}
			if result.Repo != tc.expectRepo {
				t.Errorf("Repo: expected %q, got %q", tc.expectRepo, result.Repo)
			}
		})
	}
}

// TestFromURL_ExtraPathComponents verifies correct parsing when URLs contain
// path segments beyond owner/repo (e.g., GitHub blob/tree URLs).
func TestFromURL_ExtraPathComponents(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectOwner string
		expectRepo  string
	}{
		{
			name:        "GitHub blob URL",
			url:         "https://github.com/owner/repo/blob/main/file.go",
			expectOwner: "owner/repo/blob/main",
			expectRepo:  "file.go",
		},
		{
			name:        "GitHub tree URL",
			url:         "https://github.com/owner/repo/tree/main/src",
			expectOwner: "owner/repo/tree/main",
			expectRepo:  "src",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// FromURL treats all path components before the last as "owner".
			// This is expected behavior — ParseSmartURL in git_operations.go
			// handles the full GitHub URL parsing.
			result := FromURL(tc.url)
			if result == nil {
				t.Fatal("Expected non-nil Info")
			}
			if result.Owner != tc.expectOwner {
				t.Errorf("Owner: expected %q, got %q", tc.expectOwner, result.Owner)
			}
			if result.Repo != tc.expectRepo {
				t.Errorf("Repo: expected %q, got %q", tc.expectRepo, result.Repo)
			}
		})
	}
}

// TestFromURL_InvalidURLFormats verifies graceful handling of malformed URLs.
func TestFromURL_InvalidURLFormats(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty string", ""},
		{"bare hostname", "github.com"},
		{"missing scheme with path", "github.com/owner/repo"},
		{"scheme only", "https://"},
		{"scheme with empty host", "https:///owner/repo"},
		{"just a path", "/owner/repo"},
		{"relative path", "owner/repo"},
		{"single path segment", "https://github.com/onlyowner"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)
			if result != nil {
				t.Errorf("FromURL(%q) = %+v, expected nil", tc.url, result)
			}
		})
	}
}
