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
)

// Type represents the package type in a PURL
type Type string

const (
	TypeGitHub    Type = "github"
	TypeGitLab    Type = "gitlab"
	TypeBitbucket Type = "bitbucket"
	TypeGeneric   Type = "generic"
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

// FromGitURL creates a PURL from a git repository URL and version/commit
func FromGitURL(repoURL, version string) *PURL {
	if repoURL == "" {
		return nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil
	}

	host := strings.ToLower(u.Host)
	purlType := detectType(host)

	var namespace, name string
	if len(parts) > 2 {
		// GitLab nested groups: group/subgroup/repo
		namespace = strings.Join(parts[:len(parts)-1], "/")
		name = parts[len(parts)-1]
	} else {
		namespace = parts[0]
		name = parts[1]
	}

	// Strip .git suffix from name
	name = strings.TrimSuffix(name, ".git")

	return &PURL{
		Type:      purlType,
		Namespace: namespace,
		Name:      name,
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

// detectType determines the PURL type from a hostname
func detectType(host string) Type {
	switch {
	case strings.Contains(host, "github.com") || strings.Contains(host, "github"):
		return TypeGitHub
	case strings.Contains(host, "gitlab.com") || strings.Contains(host, "gitlab"):
		return TypeGitLab
	case strings.Contains(host, "bitbucket.org") || strings.Contains(host, "bitbucket"):
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
