package purl

import (
	"strings"
	"testing"
)

// TestFromGitURL_GitLabNestedGroupPURLs verifies PURL generation for GitLab
// repositories with deeply nested groups. Namespace slashes are URL-encoded
// in the PURL string.
func TestFromGitURL_GitLabNestedGroupPURLs(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		version    string
		expectPURL string
	}{
		{
			name:       "3 levels deep",
			url:        "https://gitlab.com/org/team/subteam/repo",
			version:    "abc123",
			expectPURL: "pkg:gitlab/org%2Fteam%2Fsubteam/repo@abc123",
		},
		{
			name:       "5 levels deep",
			url:        "https://gitlab.com/corp/div/dept/team/sub/repo",
			version:    "def456",
			expectPURL: "pkg:gitlab/corp%2Fdiv%2Fdept%2Fteam%2Fsub/repo@def456",
		},
		{
			name:       "self-hosted GitLab nested groups",
			url:        "https://gitlab.internal.corp/infra/platform/services/api",
			version:    "aaa111",
			expectPURL: "pkg:gitlab/infra%2Fplatform%2Fservices/api@aaa111",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL(tc.url, tc.version)
			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}
			if result.String() != tc.expectPURL {
				t.Errorf("Expected %q, got %q", tc.expectPURL, result.String())
			}
		})
	}
}

// TestFromGitURL_BitbucketPURLs verifies PURL generation for Bitbucket repos,
// including self-hosted Bitbucket Server instances.
func TestFromGitURL_BitbucketPURLs(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		version    string
		expectPURL string
	}{
		{
			name:       "Bitbucket Cloud",
			url:        "https://bitbucket.org/atlassian/stash",
			version:    "v2.0.0",
			expectPURL: "pkg:bitbucket/atlassian/stash@v2.0.0",
		},
		{
			name:       "Bitbucket with .git suffix",
			url:        "https://bitbucket.org/owner/repo.git",
			version:    "abc123",
			expectPURL: "pkg:bitbucket/owner/repo@abc123",
		},
		{
			name:       "Bitbucket Server (self-hosted)",
			url:        "https://bitbucket-server.corp/team/project",
			version:    "def456",
			expectPURL: "pkg:bitbucket/team/project@def456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL(tc.url, tc.version)
			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}
			if result.String() != tc.expectPURL {
				t.Errorf("Expected %q, got %q", tc.expectPURL, result.String())
			}
		})
	}
}

// TestFromGitURL_GenericFallback verifies that unknown git hosts produce
// PURLs with type "generic".
func TestFromGitURL_GenericFallback(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		version    string
		expectType Type
	}{
		{
			name:       "Gitea instance",
			url:        "https://gitea.io/owner/repo",
			version:    "abc123",
			expectType: TypeGeneric,
		},
		{
			name:       "Codeberg",
			url:        "https://codeberg.org/owner/repo",
			version:    "def456",
			expectType: TypeGeneric,
		},
		{
			name:       "self-hosted plain git",
			url:        "https://git.internal.corp/team/project",
			version:    "ghi789",
			expectType: TypeGeneric,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL(tc.url, tc.version)
			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}
			if result.Type != tc.expectType {
				t.Errorf("Type: expected %q, got %q", tc.expectType, result.Type)
			}
		})
	}
}

// TestFromGitURL_GitSuffixStripping verifies that .git suffix is stripped
// from the repository name in the generated PURL.
func TestFromGitURL_GitSuffixStripping(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		expectName string
	}{
		{"GitHub .git", "https://github.com/owner/repo.git", "repo"},
		{"GitLab .git", "https://gitlab.com/owner/repo.git", "repo"},
		{"Bitbucket .git", "https://bitbucket.org/owner/repo.git", "repo"},
		{"no .git suffix", "https://github.com/owner/mylib", "mylib"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL(tc.url, "abc123")
			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}
			if result.Name != tc.expectName {
				t.Errorf("Name: expected %q, got %q", tc.expectName, result.Name)
			}
		})
	}
}

// TestFromGitURL_VersionFormats covers version strings with and without
// v prefix, full commit SHAs, and empty versions.
func TestFromGitURL_VersionFormats(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		expectPURL string
	}{
		{
			name:       "semver with v prefix",
			version:    "v1.2.3",
			expectPURL: "pkg:github/owner/repo@v1.2.3",
		},
		{
			name:       "semver without v prefix",
			version:    "1.2.3",
			expectPURL: "pkg:github/owner/repo@1.2.3",
		},
		{
			name:       "full commit SHA",
			version:    "abc1234567890def1234567890abc1234567890de",
			expectPURL: "pkg:github/owner/repo@abc1234567890def1234567890abc1234567890de",
		},
		{
			name:       "short commit SHA",
			version:    "abc1234",
			expectPURL: "pkg:github/owner/repo@abc1234",
		},
		{
			name:       "empty version omits @",
			version:    "",
			expectPURL: "pkg:github/owner/repo",
		},
		{
			name:       "tag with pre-release",
			version:    "v2.0.0-rc.1",
			expectPURL: "pkg:github/owner/repo@v2.0.0-rc.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL("https://github.com/owner/repo", tc.version)
			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}
			if result.String() != tc.expectPURL {
				t.Errorf("Expected %q, got %q", tc.expectPURL, result.String())
			}
		})
	}
}

// TestFromGitURL_CommitOnlyPURLs verifies PURLs generated using only a
// commit hash as version (no tag/branch ref).
func TestFromGitURL_CommitOnlyPURLs(t *testing.T) {
	fullSHA := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	result := FromGitURL("https://github.com/owner/repo", fullSHA)
	if result == nil {
		t.Fatal("Expected non-nil PURL")
	}
	if result.Version != fullSHA {
		t.Errorf("Version: expected %q, got %q", fullSHA, result.Version)
	}

	purlStr := result.String()
	if !strings.Contains(purlStr, "@"+fullSHA) {
		t.Errorf("PURL string should contain full commit SHA: %q", purlStr)
	}
}

// TestFromGitURLWithFallback_EdgeCases covers fallback behavior when the
// URL is invalid, empty, or has insufficient path components.
func TestFromGitURLWithFallback_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		repoURL    string
		version    string
		vendorName string
		expectType Type
		expectName string
	}{
		{
			name:       "empty URL falls back to generic with vendor name",
			repoURL:    "",
			version:    "abc123",
			vendorName: "my-lib",
			expectType: TypeGeneric,
			expectName: "my-lib",
		},
		{
			name:       "URL with single path segment falls back",
			repoURL:    "https://github.com/onlyowner",
			version:    "def456",
			vendorName: "fallback-name",
			expectType: TypeGeneric,
			expectName: "fallback-name",
		},
		{
			name:       "SSH URL without scheme falls back",
			repoURL:    "git@github.com:owner/repo.git",
			version:    "ghi789",
			vendorName: "ssh-vendor",
			expectType: TypeGeneric,
			expectName: "ssh-vendor",
		},
		{
			name:       "completely invalid URL falls back",
			repoURL:    "not a url at all",
			version:    "jkl012",
			vendorName: "invalid-vendor",
			expectType: TypeGeneric,
			expectName: "invalid-vendor",
		},
		{
			name:       "fallback preserves vendor name with special chars",
			repoURL:    "",
			version:    "abc123",
			vendorName: "my-lib_v2.0",
			expectType: TypeGeneric,
			expectName: "my-lib_v2.0",
		},
		{
			name:       "fallback with empty version",
			repoURL:    "",
			version:    "",
			vendorName: "no-version-lib",
			expectType: TypeGeneric,
			expectName: "no-version-lib",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURLWithFallback(tc.repoURL, tc.version, tc.vendorName)
			if result == nil {
				t.Fatal("FromGitURLWithFallback should never return nil")
			}
			if result.Type != tc.expectType {
				t.Errorf("Type: expected %q, got %q", tc.expectType, result.Type)
			}
			if result.Name != tc.expectName {
				t.Errorf("Name: expected %q, got %q", tc.expectName, result.Name)
			}
		})
	}
}

// TestSupportsVulnScanning_AllTypes verifies SupportsVulnScanning for every
// defined PURL type constant.
func TestSupportsVulnScanning_AllTypes(t *testing.T) {
	tests := []struct {
		purlType Type
		expected bool
	}{
		{TypeGitHub, true},
		{TypeGitLab, true},
		{TypeBitbucket, true},
		{TypeGeneric, false},
		{Type(""), false},
		{Type("npm"), false},
		{Type("pypi"), false},
	}

	for _, tc := range tests {
		name := string(tc.purlType)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			p := &PURL{Type: tc.purlType, Name: "test"}
			if p.SupportsVulnScanning() != tc.expected {
				t.Errorf("SupportsVulnScanning() for type %q = %v, expected %v",
					tc.purlType, p.SupportsVulnScanning(), tc.expected)
			}
		})
	}
}

// TestToOSVPackage_NestedNamespace verifies ToOSVPackage formats correctly
// for GitLab nested group namespaces.
func TestToOSVPackage_NestedNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		pkgName   string
		expected  string
	}{
		{
			name:      "simple namespace",
			namespace: "owner",
			pkgName:   "repo",
			expected:  "owner/repo",
		},
		{
			name:      "nested namespace",
			namespace: "org/team/subteam",
			pkgName:   "repo",
			expected:  "org/team/subteam/repo",
		},
		{
			name:      "deeply nested namespace",
			namespace: "a/b/c/d/e",
			pkgName:   "project",
			expected:  "a/b/c/d/e/project",
		},
		{
			name:      "empty namespace",
			namespace: "",
			pkgName:   "standalone",
			expected:  "standalone",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := PURL{Namespace: tc.namespace, Name: tc.pkgName}
			result := p.ToOSVPackage()
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestPURL_String_Subpath verifies PURL string generation with a subpath field.
func TestPURL_String_Subpath(t *testing.T) {
	p := PURL{
		Type:      TypeGitHub,
		Namespace: "owner",
		Name:      "repo",
		Version:   "v1.0.0",
		Subpath:   "src/lib",
	}
	expected := "pkg:github/owner/repo@v1.0.0#src/lib"
	if p.String() != expected {
		t.Errorf("Expected %q, got %q", expected, p.String())
	}
}

// TestPURL_String_MissingType verifies that PURL with empty Type returns
// empty string.
func TestPURL_String_MissingType(t *testing.T) {
	p := PURL{Name: "repo", Version: "abc123"}
	if p.String() != "" {
		t.Errorf("Expected empty string for PURL without type, got %q", p.String())
	}
}

// TestPURL_String_MissingName verifies that PURL with empty Name returns
// empty string.
func TestPURL_String_MissingName(t *testing.T) {
	p := PURL{Type: TypeGitHub, Version: "abc123"}
	if p.String() != "" {
		t.Errorf("Expected empty string for PURL without name, got %q", p.String())
	}
}
