package core

import (
	"fmt"

	"github.com/EmundoT/git-vendor/internal/core/providers"
	"github.com/EmundoT/git-vendor/internal/types"
)

// RemoteExplorerInterface defines the contract for remote repository browsing and URL parsing.
// This interface enables mocking in tests and potential alternative exploration strategies.
type RemoteExplorerInterface interface {
	FetchRepoDir(url, ref, subdir string) ([]string, error)
	ListLocalDir(path string) ([]string, error)
	ParseSmartURL(rawURL string) (string, string, string)
}

// RemoteExplorer handles remote repository browsing and URL parsing
type RemoteExplorer struct {
	gitClient GitClient
	fs        FileSystem
	registry  *providers.ProviderRegistry
}

// NewRemoteExplorer creates a new RemoteExplorer
func NewRemoteExplorer(gitClient GitClient, fs FileSystem) *RemoteExplorer {
	return &RemoteExplorer{
		gitClient: gitClient,
		fs:        fs,
		registry:  providers.NewProviderRegistry(),
	}
}

// FetchRepoDir fetches directory listing from remote repository
func (e *RemoteExplorer) FetchRepoDir(url, ref, subdir string) ([]string, error) {
	// Show progress indication to user
	fmt.Println("⠿ Cloning repository...")

	tempDir, err := e.fs.CreateTemp("", "git-vendor-index-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = e.fs.RemoveAll(tempDir) //nolint:errcheck // cleanup in defer
	}()

	// Clone with filter=blob:none to avoid downloading file contents
	opts := &types.CloneOptions{
		Filter:     "blob:none",
		NoCheckout: true,
		Depth:      1,
	}

	if err := e.gitClient.Clone(tempDir, url, opts); err != nil {
		return nil, err
	}

	// Fetch specific ref if needed (best-effort, may already be available)
	if ref != "" && ref != "HEAD" {
		// Ignore error - if fetch fails, ListTree below will handle it
		_ = e.gitClient.Fetch(tempDir, 0, ref) //nolint:errcheck
	}

	// Determine target ref
	target := ref
	if target == "" {
		target = "HEAD"
	}

	return e.gitClient.ListTree(tempDir, target, subdir)
}

// ListLocalDir lists local directory contents
func (e *RemoteExplorer) ListLocalDir(path string) ([]string, error) {
	return e.fs.ReadDir(path)
}

// ParseSmartURL parses URLs from any supported git hosting platform
// Supports GitHub, GitLab, Bitbucket, and generic git URLs
func (e *RemoteExplorer) ParseSmartURL(rawURL string) (string, string, string) {
	baseURL, ref, path, err := e.registry.ParseURL(rawURL)
	if err != nil {
		// Fallback to returning just the URL for compatibility
		// (generic provider should never error, but just in case)
		return rawURL, "", ""
	}
	return baseURL, ref, path
}

// GetProviderName returns the detected provider name for a URL
// Useful for debugging and logging
func (e *RemoteExplorer) GetProviderName(url string) string {
	provider := e.registry.DetectProvider(url)
	return provider.Name()
}
