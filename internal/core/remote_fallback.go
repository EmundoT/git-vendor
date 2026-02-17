package core

import (
	"context"
	"fmt"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ResolveVendorURLs returns the ordered list of URLs to try for a vendor.
// Primary URL first, then mirrors in declaration order. Internal vendors
// (Source == "internal") return nil — they have no remote URLs.
func ResolveVendorURLs(v *types.VendorSpec) []string {
	if v.Source == SourceInternal {
		return nil
	}
	urls := make([]string, 0, 1+len(v.Mirrors))
	urls = append(urls, v.URL)
	urls = append(urls, v.Mirrors...)
	return urls
}

// FetchWithFallback tries fetching from each URL in order until one succeeds.
// FetchWithFallback returns the URL that succeeded and nil error, or empty
// string and the last error if all URLs fail.
//
// Strategy: the first URL is added as "origin" via AddRemote. Subsequent URLs
// are swapped in via SetRemoteURL to avoid multiple named remotes (which would
// complicate ref resolution like "origin/main" vs "mirror1/main").
func FetchWithFallback(
	ctx context.Context,
	gitClient GitClient,
	fs FileSystem,
	ui UICallback,
	tempDir string,
	urls []string,
	ref string,
	depth int,
) (usedURL string, err error) {
	if len(urls) == 0 {
		return "", fmt.Errorf("no URLs provided for fetch")
	}

	var lastErr error
	for i, url := range urls {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("fetch cancelled: %w", err)
		}

		if i == 0 {
			// First URL: add as "origin"
			if addErr := gitClient.AddRemote(ctx, tempDir, "origin", url); addErr != nil {
				lastErr = fmt.Errorf("add remote %s: %w", SanitizeURL(url), addErr)
				continue
			}
		} else {
			// Subsequent URLs: switch origin's URL
			if setErr := gitClient.SetRemoteURL(ctx, tempDir, "origin", url); setErr != nil {
				lastErr = fmt.Errorf("set remote URL to %s: %w", SanitizeURL(url), setErr)
				continue
			}
			if Verbose {
				ui.ShowWarning("Mirror Fallback", fmt.Sprintf("Trying %s", SanitizeURL(url)))
			}
		}

		// Attempt fetch
		fetchErr := gitClient.Fetch(ctx, tempDir, "origin", depth, ref)
		if fetchErr == nil {
			return url, nil
		}
		lastErr = fetchErr

		if len(urls) > 1 {
			ui.ShowWarning("Fetch Failed", fmt.Sprintf("%s: %v", SanitizeURL(url), fetchErr))
		}
	}

	return "", fmt.Errorf("all URLs failed for ref %s (last error: %w)", ref, lastErr)
}
