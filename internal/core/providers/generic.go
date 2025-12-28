package providers

import "strings"

// GenericProvider handles unknown git platforms and raw git URLs
// This is a fallback provider that accepts any URL
type GenericProvider struct{}

// NewGenericProvider creates a new generic provider
func NewGenericProvider() *GenericProvider {
	return &GenericProvider{}
}

// Name returns the provider identifier
func (p *GenericProvider) Name() string {
	return "generic"
}

// Supports always returns true as this is the fallback provider
func (p *GenericProvider) Supports(_ string) bool {
	return true
}

// ParseURL normalizes git URLs without attempting platform-specific parsing
//
// Supported URL formats:
//   - https://git.example.com/project/repo.git
//   - git://git.example.com/project/repo
//   - git@git.company.com:team/project.git
//   - ssh://git@server.com/path/to/repo
//
// Since there's no standard format for ref/path in generic git URLs,
// this provider only returns the normalized base URL with empty ref and path.
func (p *GenericProvider) ParseURL(rawURL string) (string, string, string, error) {
	cleaned := cleanURL(rawURL)

	// Add https:// prefix if no protocol specified
	if !strings.HasPrefix(cleaned, "http://") &&
		!strings.HasPrefix(cleaned, "https://") &&
		!strings.HasPrefix(cleaned, "git://") &&
		!strings.HasPrefix(cleaned, "git@") &&
		!strings.HasPrefix(cleaned, "ssh://") {
		cleaned = "https://" + cleaned
	}

	// Remove .git suffix for consistency
	cleaned = strings.TrimSuffix(cleaned, ".git")
	cleaned = strings.TrimSuffix(cleaned, "/")

	// For generic URLs, we cannot reliably extract ref/path
	// Return normalized URL with empty ref and path
	return cleaned, "", "", nil
}
