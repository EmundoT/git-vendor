package purl

import (
	"testing"
)

func TestPURL_String(t *testing.T) {
	tests := []struct {
		name     string
		purl     PURL
		expected string
	}{
		{
			name: "GitHub basic",
			purl: PURL{
				Type:      TypeGitHub,
				Namespace: "owner",
				Name:      "repo",
				Version:   "abc123",
			},
			expected: "pkg:github/owner/repo@abc123",
		},
		{
			name: "GitLab nested groups",
			purl: PURL{
				Type:      TypeGitLab,
				Namespace: "group/subgroup",
				Name:      "repo",
				Version:   "def456",
			},
			expected: "pkg:gitlab/group%2Fsubgroup/repo@def456",
		},
		{
			name: "Generic without namespace",
			purl: PURL{
				Type:    TypeGeneric,
				Name:    "my-lib",
				Version: "v1.0.0",
			},
			expected: "pkg:generic/my-lib@v1.0.0",
		},
		{
			name: "Special characters in name - space",
			purl: PURL{
				Type:      TypeGitHub,
				Namespace: "owner",
				Name:      "repo with space",
				Version:   "abc123",
			},
			expected: "pkg:github/owner/repo%20with%20space@abc123",
		},
		{
			name:     "Empty PURL",
			purl:     PURL{},
			expected: "",
		},
		{
			name: "Missing version",
			purl: PURL{
				Type:      TypeGitHub,
				Namespace: "owner",
				Name:      "repo",
			},
			expected: "pkg:github/owner/repo",
		},
		{
			name: "With qualifiers",
			purl: PURL{
				Type:       TypeGitHub,
				Namespace:  "owner",
				Name:       "repo",
				Version:    "v1.0.0",
				Qualifiers: map[string]string{"vcs_url": "git://example.com"},
			},
			expected: "pkg:github/owner/repo@v1.0.0?vcs_url=git%3A%2F%2Fexample.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.purl.String()
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestFromGitURL(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		version     string
		expectNil   bool
		expectType  Type
		expectNS    string
		expectName  string
		expectVer   string
	}{
		{
			name:       "GitHub standard",
			repoURL:    "https://github.com/owner/repo",
			version:    "abc123",
			expectType: TypeGitHub,
			expectNS:   "owner",
			expectName: "repo",
			expectVer:  "abc123",
		},
		{
			name:       "GitHub with .git suffix",
			repoURL:    "https://github.com/owner/repo.git",
			version:    "def456",
			expectType: TypeGitHub,
			expectNS:   "owner",
			expectName: "repo",
			expectVer:  "def456",
		},
		{
			name:       "GitLab standard",
			repoURL:    "https://gitlab.com/owner/repo",
			version:    "ghi789",
			expectType: TypeGitLab,
			expectNS:   "owner",
			expectName: "repo",
			expectVer:  "ghi789",
		},
		{
			name:       "GitLab nested groups",
			repoURL:    "https://gitlab.com/group/subgroup/deep/repo",
			version:    "jkl012",
			expectType: TypeGitLab,
			expectNS:   "group/subgroup/deep",
			expectName: "repo",
			expectVer:  "jkl012",
		},
		{
			name:       "Bitbucket",
			repoURL:    "https://bitbucket.org/owner/repo",
			version:    "mno345",
			expectType: TypeBitbucket,
			expectNS:   "owner",
			expectName: "repo",
			expectVer:  "mno345",
		},
		{
			name:       "Unknown host falls back to generic",
			repoURL:    "https://git.internal.corp/team/project",
			version:    "pqr678",
			expectType: TypeGeneric,
			expectNS:   "team",
			expectName: "project",
			expectVer:  "pqr678",
		},
		{
			name:      "Empty URL returns nil",
			repoURL:   "",
			version:   "abc123",
			expectNil: true,
		},
		{
			name:      "Invalid URL returns nil",
			repoURL:   "://invalid",
			version:   "abc123",
			expectNil: true,
		},
		{
			name:      "URL with only one path segment returns nil",
			repoURL:   "https://github.com/onlyowner",
			version:   "abc123",
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURL(tc.repoURL, tc.version)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil PURL")
			}

			if result.Type != tc.expectType {
				t.Errorf("Type: expected %q, got %q", tc.expectType, result.Type)
			}
			if result.Namespace != tc.expectNS {
				t.Errorf("Namespace: expected %q, got %q", tc.expectNS, result.Namespace)
			}
			if result.Name != tc.expectName {
				t.Errorf("Name: expected %q, got %q", tc.expectName, result.Name)
			}
			if result.Version != tc.expectVer {
				t.Errorf("Version: expected %q, got %q", tc.expectVer, result.Version)
			}
		})
	}
}

func TestFromGitURLWithFallback(t *testing.T) {
	tests := []struct {
		name       string
		repoURL    string
		version    string
		vendorName string
		expectPURL string
	}{
		{
			name:       "Valid URL uses URL-based PURL",
			repoURL:    "https://github.com/owner/repo",
			version:    "abc123",
			vendorName: "my-vendor",
			expectPURL: "pkg:github/owner/repo@abc123",
		},
		{
			name:       "Empty URL falls back to vendor name",
			repoURL:    "",
			version:    "def456",
			vendorName: "custom-lib",
			expectPURL: "pkg:generic/custom-lib@def456",
		},
		{
			name:       "Invalid URL falls back to vendor name",
			repoURL:    "not-a-url",
			version:    "ghi789",
			vendorName: "internal-tool",
			expectPURL: "pkg:generic/internal-tool@ghi789",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromGitURLWithFallback(tc.repoURL, tc.version, tc.vendorName)
			if result.String() != tc.expectPURL {
				t.Errorf("Expected %q, got %q", tc.expectPURL, result.String())
			}
		})
	}
}

func TestPURL_SupportsVulnScanning(t *testing.T) {
	tests := []struct {
		purlType Type
		expected bool
	}{
		{TypeGitHub, true},
		{TypeGitLab, true},
		{TypeBitbucket, true},
		{TypeGeneric, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.purlType), func(t *testing.T) {
			p := &PURL{Type: tc.purlType, Name: "test"}
			if p.SupportsVulnScanning() != tc.expected {
				t.Errorf("Expected %v for type %s", tc.expected, tc.purlType)
			}
		})
	}
}

func TestPURL_ToOSVPackage(t *testing.T) {
	tests := []struct {
		name     string
		purl     PURL
		expected string
	}{
		{
			name:     "With namespace",
			purl:     PURL{Namespace: "owner", Name: "repo"},
			expected: "owner/repo",
		},
		{
			name:     "Without namespace",
			purl:     PURL{Name: "standalone"},
			expected: "standalone",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.purl.ToOSVPackage()
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// Edge case tests
func TestFromGitURL_EdgeCases(t *testing.T) {
	t.Run("very long URL", func(t *testing.T) {
		longName := "repo-" + string(make([]byte, 500))
		for i := range longName {
			if longName[i] == 0 {
				longName = longName[:5] + "x" + longName[6:]
			}
		}
		// This should not panic
		_ = FromGitURL("https://github.com/owner/"+longName, "abc123")
	})

	t.Run("URL with port", func(t *testing.T) {
		purl := FromGitURL("https://gitlab.internal.com:8443/team/repo", "abc123")
		if purl == nil {
			t.Fatal("Expected non-nil PURL for URL with port")
		}
		if purl.Type != TypeGitLab {
			t.Errorf("Expected gitlab type, got %s", purl.Type)
		}
	})

	t.Run("URL with trailing slash", func(t *testing.T) {
		purl := FromGitURL("https://github.com/owner/repo/", "abc123")
		if purl == nil {
			t.Fatal("Expected non-nil PURL")
		}
		if purl.Name != "repo" {
			t.Errorf("Expected name 'repo', got %q", purl.Name)
		}
	})

	t.Run("SSH URL format", func(t *testing.T) {
		// SSH URLs like git@github.com:owner/repo.git won't parse as standard URLs
		// This should gracefully return nil
		purl := FromGitURL("git@github.com:owner/repo.git", "abc123")
		if purl != nil {
			// If it somehow parses, that's fine, but it shouldn't panic
			t.Logf("SSH URL parsed to: %s", purl.String())
		}
	})
}
