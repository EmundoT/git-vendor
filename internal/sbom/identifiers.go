// Package sbom provides shared utilities for Software Bill of Materials generation.
// This package contains common logic used by both CycloneDX and SPDX formatters,
// as well as utilities that may be reused by audit, compliance, and reporting features.
package sbom

import (
	"fmt"
	"strings"

	"github.com/EmundoT/git-vendor/internal/hostdetect"
)

// MaxSPDXIDLength is the maximum length for SPDX identifier components.
// While SPDX spec doesn't define a max, some tools have practical limits.
// We use 128 characters as a reasonable limit that works with most tools.
const MaxSPDXIDLength = 128

// VendorIdentity represents the unique identity of a vendored dependency.
// A vendor may track multiple refs, so the identity includes both name and commit hash.
type VendorIdentity struct {
	// Name is the vendor name from config (required for identification).
	Name string

	// Ref is the git ref (branch, tag, commit) being tracked.
	// This field is preserved for informational/debugging purposes but is NOT
	// used in identifier generation. The CommitHash provides uniqueness instead,
	// because the same ref can point to different commits over time.
	Ref string

	// CommitHash is the full commit SHA for the ref (required for uniqueness).
	CommitHash string
}

// ShortHash returns the first 7 characters of the commit hash.
// This is used for display and as part of identifiers.
func (v VendorIdentity) ShortHash() string {
	if len(v.CommitHash) > 7 {
		return v.CommitHash[:7]
	}
	return v.CommitHash
}

// GenerateBOMRef creates a unique CycloneDX BOM reference for a vendor.
// Format: {name}@{short-hash}
// For vendors with multiple refs pointing to the same commit, the hash ensures uniqueness.
// For different commits, the different hashes ensure uniqueness.
func GenerateBOMRef(v VendorIdentity) string {
	return fmt.Sprintf("%s@%s", v.Name, v.ShortHash())
}

// GenerateSPDXID creates a unique SPDX identifier for a package.
// Format: Package-{sanitized-name}-{short-hash}
// The hash suffix ensures uniqueness when a vendor tracks multiple refs.
// Returns the ID without the "SPDXRef-" prefix (that's added during JSON serialization).
func GenerateSPDXID(v VendorIdentity) string {
	sanitized := SanitizeSPDXID(v.Name)
	return fmt.Sprintf("Package-%s-%s", sanitized, v.ShortHash())
}

// SanitizeSPDXID converts a string to a valid SPDX identifier component.
// SPDX IDs must match the pattern [a-zA-Z0-9.-]+
// Invalid characters are replaced with hyphens.
// Empty input returns "unknown" to prevent invalid IDs.
// Very long inputs are truncated to MaxSPDXIDLength to ensure compatibility
// with SBOM tools that may have practical length limits.
func SanitizeSPDXID(s string) string {
	if s == "" {
		return "unknown"
	}

	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if isValidSPDXChar(r) {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	sanitized := result.String()

	// Truncate if exceeds max length
	if len(sanitized) > MaxSPDXIDLength {
		sanitized = sanitized[:MaxSPDXIDLength]
	}

	return sanitized
}

// isValidSPDXChar returns true if the rune is valid in an SPDX identifier.
func isValidSPDXChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '.' ||
		r == '-'
}

// SPDXDocumentID is the standard SPDX document identifier.
const SPDXDocumentID = "DOCUMENT"

// FormatSPDXRef formats an SPDX element ID with the required "SPDXRef-" prefix.
func FormatSPDXRef(elementID string) string {
	return "SPDXRef-" + elementID
}

// SupplierInfo holds supplier/manufacturer information extracted from a repository URL.
type SupplierInfo struct {
	Name string // Owner/org name
	URL  string // Full repository URL
}

// ExtractSupplier extracts supplier information from a repository URL.
// Uses the shared hostdetect package for consistent URL parsing across the codebase.
// Returns nil if the URL is empty, invalid, or from an unknown provider.
//
// Supported providers: GitHub, GitLab, Bitbucket (including self-hosted instances).
func ExtractSupplier(repoURL string) *SupplierInfo {
	info := hostdetect.FromURL(repoURL)
	if info == nil {
		return nil
	}

	// Only return supplier info for known providers
	// Unknown providers don't have reliable owner/org extraction
	if !hostdetect.IsKnownProvider(info.Provider) {
		return nil
	}

	return &SupplierInfo{
		Name: info.Owner,
		URL:  repoURL,
	}
}

// MetadataComment builds a structured comment from git-vendor metadata.
// Only includes fields that have values, avoiding empty placeholders.
func MetadataComment(ref, commit, vendoredAt, vendoredBy string) string {
	var parts []string

	if ref != "" {
		parts = append(parts, fmt.Sprintf("ref=%s", ref))
	}
	if commit != "" {
		parts = append(parts, fmt.Sprintf("commit=%s", commit))
	}
	if vendoredAt != "" {
		parts = append(parts, fmt.Sprintf("vendored_at=%s", vendoredAt))
	}
	if vendoredBy != "" {
		parts = append(parts, fmt.Sprintf("vendored_by=%s", vendoredBy))
	}

	return strings.Join(parts, ", ")
}

// DefaultProjectName returns a fallback project name when none is provided.
const DefaultProjectName = "unknown-project"

// ValidateProjectName ensures a project name is valid for use in SBOMs.
// Returns the original name if valid, or DefaultProjectName if empty.
func ValidateProjectName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultProjectName
	}
	return name
}

// DefaultSPDXNamespace is the default domain for SPDX document namespaces.
// This can be overridden via configuration.
const DefaultSPDXNamespace = "https://spdx.org/spdxdocs"

// BuildSPDXNamespace constructs a unique SPDX document namespace.
// Format: {baseURL}/{projectName}/{uuid}
func BuildSPDXNamespace(baseURL, projectName, uuid string) string {
	if baseURL == "" {
		baseURL = DefaultSPDXNamespace
	}
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), projectName, uuid)
}
