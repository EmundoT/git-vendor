package providers

// GitHostingProvider abstracts git hosting platform-specific operations
type GitHostingProvider interface {
	// Name returns the provider identifier ("github", "gitlab", "bitbucket", "generic")
	Name() string

	// Supports returns true if this provider can handle the given URL
	Supports(url string) bool

	// ParseURL extracts repository base URL, ref, and path from platform-specific URLs
	// Returns: (baseURL, ref, path, error)
	// - baseURL: The base repository URL (e.g., "https://github.com/owner/repo")
	// - ref: The branch, tag, or commit reference (empty string if not specified)
	// - path: The file or directory path within the repository (empty string if not specified)
	// - error: Any error encountered during parsing
	ParseURL(rawURL string) (string, string, string, error)
}

// ProviderRegistry manages available git hosting providers with auto-detection
type ProviderRegistry struct {
	providers []GitHostingProvider
	fallback  GitHostingProvider
}

// NewProviderRegistry creates a new registry with all supported providers
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: []GitHostingProvider{
			NewGitHubProvider(),
			NewGitLabProvider(),
			NewBitbucketProvider(),
		},
		fallback: NewGenericProvider(),
	}
}

// DetectProvider returns the appropriate provider for the given URL
// If no specific provider matches, returns the generic fallback provider
func (r *ProviderRegistry) DetectProvider(url string) GitHostingProvider {
	for _, provider := range r.providers {
		if provider.Supports(url) {
			return provider
		}
	}
	return r.fallback
}

// ParseURL delegates URL parsing to the appropriate provider based on auto-detection
// Returns: (baseURL, ref, path, error)
func (r *ProviderRegistry) ParseURL(url string) (string, string, string, error) {
	provider := r.DetectProvider(url)
	return provider.ParseURL(url)
}
