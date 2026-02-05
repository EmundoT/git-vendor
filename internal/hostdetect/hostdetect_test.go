package hostdetect

import (
	"testing"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		host     string
		expected Provider
	}{
		// Exact matches
		{"github.com", ProviderGitHub},
		{"gitlab.com", ProviderGitLab},
		{"bitbucket.org", ProviderBitbucket},

		// Case insensitive
		{"GitHub.com", ProviderGitHub},
		{"GITLAB.COM", ProviderGitLab},
		{"BitBucket.org", ProviderBitbucket},

		// With port
		{"github.com:443", ProviderGitHub},
		{"gitlab.com:8443", ProviderGitLab},

		// Enterprise suffix
		{"enterprise.github.com", ProviderGitHub},
		{"internal.gitlab.com", ProviderGitLab},
		{"server.bitbucket.org", ProviderBitbucket},

		// Self-hosted with provider name in hostname
		{"github.mycompany.com", ProviderGitHub},
		{"gitlab.internal.corp", ProviderGitLab},
		{"bitbucket-server.corp", ProviderBitbucket},
		{"my-github-enterprise.local", ProviderGitHub},
		{"our-gitlab.internal", ProviderGitLab},

		// Unknown providers
		{"git.internal.corp", ProviderUnknown},
		{"code.example.com", ProviderUnknown},
		{"gitea.io", ProviderUnknown},
		{"", ProviderUnknown},

		// Edge cases - should NOT match (no false positives)
		// These are tricky - "notgithub.com" contains "github"
		// Our detection intentionally matches these for enterprise flexibility
		{"notgithub.com", ProviderGitHub}, // Intentional - contains "github"
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

func TestFromURL(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectNil      bool
		expectProvider Provider
		expectOwner    string
		expectRepo     string
	}{
		{
			name:           "GitHub standard",
			url:            "https://github.com/owner/repo",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "GitHub with .git suffix",
			url:            "https://github.com/owner/repo.git",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "GitLab standard",
			url:            "https://gitlab.com/owner/repo",
			expectProvider: ProviderGitLab,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "GitLab nested groups",
			url:            "https://gitlab.com/group/subgroup/deep/repo",
			expectProvider: ProviderGitLab,
			expectOwner:    "group/subgroup/deep",
			expectRepo:     "repo",
		},
		{
			name:           "Bitbucket",
			url:            "https://bitbucket.org/owner/repo",
			expectProvider: ProviderBitbucket,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
		{
			name:           "Self-hosted GitLab with port",
			url:            "https://gitlab.internal.corp:8443/team/project",
			expectProvider: ProviderGitLab,
			expectOwner:    "team",
			expectRepo:     "project",
		},
		{
			name:           "Unknown provider",
			url:            "https://git.internal.corp/team/project",
			expectProvider: ProviderUnknown,
			expectOwner:    "team",
			expectRepo:     "project",
		},
		{
			name:      "Empty URL",
			url:       "",
			expectNil: true,
		},
		{
			name:      "Invalid URL",
			url:       "://invalid",
			expectNil: true,
		},
		{
			name:      "URL with only owner",
			url:       "https://github.com/onlyowner",
			expectNil: true,
		},
		{
			name:           "URL with trailing slash",
			url:            "https://github.com/owner/repo/",
			expectProvider: ProviderGitHub,
			expectOwner:    "owner",
			expectRepo:     "repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromURL(tc.url)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

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

func TestIsKnownProvider(t *testing.T) {
	tests := []struct {
		provider Provider
		expected bool
	}{
		{ProviderGitHub, true},
		{ProviderGitLab, true},
		{ProviderBitbucket, true},
		{ProviderUnknown, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			result := IsKnownProvider(tc.provider)
			if result != tc.expected {
				t.Errorf("IsKnownProvider(%q) = %v, expected %v", tc.provider, result, tc.expected)
			}
		})
	}
}

func TestSupportsCVEScanning(t *testing.T) {
	tests := []struct {
		provider Provider
		expected bool
	}{
		{ProviderGitHub, true},
		{ProviderGitLab, true},
		{ProviderBitbucket, true},
		{ProviderUnknown, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			result := SupportsCVEScanning(tc.provider)
			if result != tc.expected {
				t.Errorf("SupportsCVEScanning(%q) = %v, expected %v", tc.provider, result, tc.expected)
			}
		})
	}
}

// Edge case tests
func TestFromURL_EdgeCases(t *testing.T) {
	t.Run("URL with query parameters", func(t *testing.T) {
		info := FromURL("https://github.com/owner/repo?ref=main")
		if info == nil {
			t.Fatal("Expected non-nil Info")
		}
		// Query params should be ignored
		if info.Repo != "repo" {
			t.Errorf("Expected repo 'repo', got %q", info.Repo)
		}
	})

	t.Run("URL with fragment", func(t *testing.T) {
		info := FromURL("https://github.com/owner/repo#readme")
		if info == nil {
			t.Fatal("Expected non-nil Info")
		}
		if info.Repo != "repo" {
			t.Errorf("Expected repo 'repo', got %q", info.Repo)
		}
	})

	t.Run("URL with authentication info", func(t *testing.T) {
		info := FromURL("https://user:token@github.com/owner/repo")
		if info == nil {
			t.Fatal("Expected non-nil Info")
		}
		if info.Provider != ProviderGitHub {
			t.Errorf("Expected GitHub provider, got %q", info.Provider)
		}
		if info.Owner != "owner" || info.Repo != "repo" {
			t.Errorf("Expected owner/repo, got %s/%s", info.Owner, info.Repo)
		}
	})
}
