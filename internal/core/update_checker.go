package core

import (
	"context"
	"fmt"

	"github.com/EmundoT/git-vendor/internal/types"
)

// UpdateCheckerInterface defines the contract for checking vendor updates.
// UpdateCheckerInterface enables mocking in tests and alternative update check strategies.
// ctx is accepted for cancellation of network operations (git fetch).
type UpdateCheckerInterface interface {
	CheckUpdates(ctx context.Context) ([]types.UpdateCheckResult, error)
}

// Compile-time interface satisfaction check.
var _ UpdateCheckerInterface = (*UpdateChecker)(nil)

// UpdateChecker handles checking for vendor updates
type UpdateChecker struct {
	configStore ConfigStore
	lockStore   LockStore
	gitClient   GitClient
	fs          FileSystem
	ui          UICallback
}

// NewUpdateChecker creates a new UpdateChecker
func NewUpdateChecker(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	ui UICallback,
) *UpdateChecker {
	return &UpdateChecker{
		configStore: configStore,
		lockStore:   lockStore,
		gitClient:   gitClient,
		fs:          fs,
		ui:          ui,
	}
}

// CheckUpdates compares lockfile commit hashes with latest available commits.
// ctx controls cancellation of git fetch operations for each vendor.
func (c *UpdateChecker) CheckUpdates(ctx context.Context) ([]types.UpdateCheckResult, error) {
	config, err := c.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	lock, err := c.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Build lock map for quick lookups
	lockMap := make(map[string]map[string]types.LockDetails)
	for i := range lock.Vendors {
		entry := &lock.Vendors[i]
		if lockMap[entry.Name] == nil {
			lockMap[entry.Name] = make(map[string]types.LockDetails)
		}
		lockMap[entry.Name][entry.Ref] = *entry
	}

	var results []types.UpdateCheckResult

	// Check each vendor's specs
	for _, vendor := range config.Vendors {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for _, spec := range vendor.Specs {
			// Get locked details
			lockEntry, hasLock := lockMap[vendor.Name][spec.Ref]
			if !hasLock {
				// Not synced yet, skip
				continue
			}

			// Fetch latest commit hash for the ref (tries primary URL + mirrors)
			urls := ResolveVendorURLs(&vendor)
			latestHash, err := c.fetchLatestHash(ctx, urls, spec.Ref)
			if err != nil {
				// Failed to fetch, skip this vendor (don't fail entire check)
				c.ui.ShowWarning("Fetch Failed", fmt.Sprintf("Could not check updates for %s @ %s: %s", vendor.Name, spec.Ref, err.Error()))
				continue
			}

			// Compare hashes
			upToDate := latestHash == lockEntry.CommitHash

			results = append(results, types.UpdateCheckResult{
				VendorName:  vendor.Name,
				Ref:         spec.Ref,
				CurrentHash: lockEntry.CommitHash,
				LatestHash:  latestHash,
				LastUpdated: lockEntry.Updated,
				UpToDate:    upToDate,
			})
		}
	}

	return results, nil
}

// fetchLatestHash fetches the latest commit hash for a given ref.
// fetchLatestHash tries each URL in order (primary + mirrors) via FetchWithFallback.
// ctx controls cancellation of git operations.
func (c *UpdateChecker) fetchLatestHash(ctx context.Context, urls []string, ref string) (string, error) {
	// Create temporary directory for fetch
	tempDir, err := c.fs.CreateTemp("", "update-check-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer c.fs.RemoveAll(tempDir) //nolint:errcheck

	// Initialize git repo
	if err := c.gitClient.Init(ctx, tempDir); err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}

	// Fetch the specific ref with depth 1 via FetchWithFallback (handles AddRemote + mirrors)
	if _, err := FetchWithFallback(ctx, c.gitClient, c.fs, c.ui, tempDir, urls, ref, 1); err != nil {
		return "", fmt.Errorf("git fetch failed: %w", err)
	}

	// Get the commit hash
	hash, err := c.gitClient.GetHeadHash(ctx, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	return hash, nil
}
