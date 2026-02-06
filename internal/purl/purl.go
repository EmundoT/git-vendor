// Package purl provides Package URL (PURL) generation and parsing utilities.
// PURLs are a standardized way to identify software packages across ecosystems.
// See: https://github.com/package-url/purl-spec
//
// This package is used by:
// - SBOM generation (CycloneDX, SPDX)
// - CVE/vulnerability scanning (OSV.dev queries)
// - Compliance reporting
package purl

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/EmundoT/git-vendor/internal/hostdetect"
)

// Type represents the package type in a PURL
type Type string

// PURL type constants for common git hosting providers
const (
	TypeGitHub    Type = "github"    // GitHub repositories
	TypeGitLab    Type = "gitlab"    // GitLab repositories (including self-hosted)
	TypeBitbucket Type = "bitbucket" // Bitbucket repositories
	TypeGeneric   Type = "generic"   // Generic/unknown repository type
)

// PURL represents a parsed Package URL
type PURL struct {
	Type       Type
	Namespace  string // owner or org (may include nested groups for GitLab)
	Name       string // repository or package name
	Version    string // version or commit hash
	Qualifiers map[string]string
	Subpath    string
}

// String formats the PURL as a standard PURL string
func (p *PURL) String() string {
	if p.Type == "" || p.Name == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("pkg:")
	sb.WriteString(string(p.Type))
	sb.WriteRune('/')

	if p.Namespace != "" {
		// URL-encode namespace (important for GitLab nested groups with slashes)
		sb.WriteString(url.PathEscape(p.Namespace))
		sb.WriteRune('/')
	}

	// URL-encode name for special characters
	sb.WriteString(url.PathEscape(p.Name))

	if p.Version != "" {
		sb.WriteRune('@')
		sb.WriteString(url.PathEscape(p.Version))
	}

	if len(p.Qualifiers) > 0 {
		sb.WriteRune('?')
		first := true
		for k, v := range p.Qualifiers {
			if !first {
				sb.WriteRune('&')
			}
			sb.WriteString(url.QueryEscape(k))
			sb.WriteRune('=')
			sb.WriteString(url.QueryEscape(v))
			first = false
		}
	}

	if p.Subpath != "" {
		sb.WriteRune('#')
		sb.WriteString(p.Subpath)
	}

	return sb.String()
}

// FromGitURL creates a PURL from a git repository URL and version/commit.
// Uses the shared hostdetect package for consistent provider detection across
// the codebase (SBOM generation, supplier extraction, CVE scanning).
func FromGitURL(repoURL, version string) *PURL {
	info := hostdetect.FromURL(repoURL)
	if info == nil {
		return nil
	}

	return &PURL{
		Type:      providerToType(info.Provider),
		Namespace: info.Owner,
		Name:      info.Repo,
		Version:   version,
	}
}

// FromGitURLWithFallback creates a PURL from a git URL, falling back to generic type
// with the provided vendor name if the URL is invalid or empty
func FromGitURLWithFallback(repoURL, version, vendorName string) *PURL {
	if purl := FromGitURL(repoURL, version); purl != nil {
		return purl
	}

	// Fallback to generic type using vendor name
	return &PURL{
		Type:    TypeGeneric,
		Name:    vendorName,
		Version: version,
	}
}

// providerToType converts a hostdetect.Provider to a purl.Type.
// This bridges the shared host detection with PURL-specific type constants.
func providerToType(p hostdetect.Provider) Type {
	switch p {
	case hostdetect.ProviderGitHub:
		return TypeGitHub
	case hostdetect.ProviderGitLab:
		return TypeGitLab
	case hostdetect.ProviderBitbucket:
		return TypeBitbucket
	default:
		return TypeGeneric
	}
}

// SupportsVulnScanning returns true if this PURL type is supported by OSV.dev
func (p *PURL) SupportsVulnScanning() bool {
	switch p.Type {
	case TypeGitHub, TypeGitLab, TypeBitbucket:
		return true
	default:
		return false
	}
}

// ToOSVPackage returns the package identifier format expected by OSV.dev API
func (p *PURL) ToOSVPackage() string {
	if p.Namespace != "" {
		return fmt.Sprintf("%s/%s", p.Namespace, p.Name)
	}
	return p.Name
}
