// Package hostdetect provides git hosting provider detection utilities.
// This package is used for consistent provider identification across:
// - SBOM generation (supplier extraction)
// - PURL creation (package type detection)
// - CVE/vulnerability scanning (ecosystem detection for OSV.dev)
//
// The detection supports both well-known hosts (github.com, gitlab.com, bitbucket.org)
// and self-hosted/enterprise instances (e.g., gitlab.internal.corp, github.enterprise.com).
package hostdetect

import (
	"net/url"
	"strings"
)

// Provider represents a git hosting provider type.
type Provider string

// Provider constants for common git hosting services.
const (
	// ProviderGitHub represents GitHub (github.com and GitHub Enterprise).
	ProviderGitHub Provider = "github"

	// ProviderGitLab represents GitLab (gitlab.com and self-hosted GitLab).
	ProviderGitLab Provider = "gitlab"

	// ProviderBitbucket represents Bitbucket (bitbucket.org and Bitbucket Server).
	ProviderBitbucket Provider = "bitbucket"

	// ProviderUnknown represents an unknown or generic git hosting provider.
	ProviderUnknown Provider = "unknown"
)

// Info contains information extracted from a repository URL.
type Info struct {
	// Provider is the detected git hosting provider.
	Provider Provider

	// Host is the hostname (e.g., "github.com", "gitlab.internal.corp").
	Host string

	// Owner is the repository owner/organization (may include nested groups for GitLab).
	Owner string

	// Repo is the repository name.
	Repo string
}

// FromURL extracts provider information from a git repository URL.
// Returns nil if the URL is empty, invalid, or doesn't have enough path components.
//
// Supported URL formats:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - https://gitlab.com/group/subgroup/repo
//   - https://bitbucket.org/owner/repo
//   - https://gitlab.internal.corp:8443/team/project
func FromURL(repoURL string) *Info {
	if repoURL == "" {
		return nil
	}

	u, err := url.Parse(repoURL)
	if err != nil || u.Host == "" {
		return nil
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil
	}

	host := strings.ToLower(u.Host)
	// Remove port if present for provider detection
	hostWithoutPort := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		hostWithoutPort = host[:idx]
	}

	provider := DetectProvider(hostWithoutPort)

	var owner, repo string
	if len(parts) > 2 {
		// GitLab nested groups: group/subgroup/repo
		owner = strings.Join(parts[:len(parts)-1], "/")
		repo = parts[len(parts)-1]
	} else {
		owner = parts[0]
		repo = parts[1]
	}

	// Strip .git suffix from repo name
	repo = strings.TrimSuffix(repo, ".git")

	return &Info{
		Provider: provider,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
	}
}

// DetectProvider determines the provider type from a hostname.
//
// Detection strategy:
// 1. Exact match on well-known hosts (github.com, gitlab.com, bitbucket.org)
// 2. Suffix match for enterprise instances (e.g., github.enterprise.com)
// 3. Contains match for self-hosted instances (e.g., gitlab.internal.corp)
//
// This allows detection of enterprise/self-hosted instances while avoiding
// false positives like "notgithub.com".
func DetectProvider(host string) Provider {
	host = strings.ToLower(host)

	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// 1. Exact match on well-known hosts (fastest path)
	switch host {
	case "github.com":
		return ProviderGitHub
	case "gitlab.com":
		return ProviderGitLab
	case "bitbucket.org":
		return ProviderBitbucket
	}

	// 2. Suffix match for enterprise instances
	// e.g., "github.mycompany.com", "gitlab.internal.corp"
	switch {
	case strings.HasSuffix(host, ".github.com"):
		return ProviderGitHub
	case strings.HasSuffix(host, ".gitlab.com"):
		return ProviderGitLab
	case strings.HasSuffix(host, ".bitbucket.org"):
		return ProviderBitbucket
	}

	// 3. Contains match for self-hosted instances with provider name in hostname
	// e.g., "github-enterprise.corp", "my-gitlab.internal", "bitbucket-server.corp"
	// This is more permissive but useful for enterprise setups
	switch {
	case strings.Contains(host, "github"):
		return ProviderGitHub
	case strings.Contains(host, "gitlab"):
		return ProviderGitLab
	case strings.Contains(host, "bitbucket"):
		return ProviderBitbucket
	}

	return ProviderUnknown
}

// IsKnownProvider returns true if the provider is a recognized hosting service
// (not ProviderUnknown). This is useful for determining if PURL type should
// be specific (github, gitlab, bitbucket) or generic.
func IsKnownProvider(p Provider) bool {
	return p != ProviderUnknown
}

// SupportsCVEScanning returns true if the provider is supported by
// vulnerability databases like OSV.dev for CVE scanning.
func SupportsCVEScanning(p Provider) bool {
	switch p {
	case ProviderGitHub, ProviderGitLab, ProviderBitbucket:
		return true
	default:
		return false
	}
}
