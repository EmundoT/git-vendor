package core

import (
	"fmt"
	"time"

	"git-vendor/internal/types"
)

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

// CheckUpdates compares lockfile commit hashes with latest available commits
func (c *UpdateChecker) CheckUpdates() ([]types.UpdateCheckResult, error) {
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
	for _, entry := range lock.Vendors {
		if lockMap[entry.Name] == nil {
			lockMap[entry.Name] = make(map[string]types.LockDetails)
		}
		lockMap[entry.Name][entry.Ref] = entry
	}

	var results []types.UpdateCheckResult

	// Check each vendor's specs
	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			// Get locked details
			lockEntry, hasLock := lockMap[vendor.Name][spec.Ref]
			if !hasLock {
				// Not synced yet, skip
				continue
			}

			// Fetch latest commit hash for the ref
			latestHash, err := c.fetchLatestHash(vendor.URL, spec.Ref)
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

// fetchLatestHash fetches the latest commit hash for a given ref
func (c *UpdateChecker) fetchLatestHash(url, ref string) (string, error) {
	// Create temporary directory for fetch
	tempDir, err := c.fs.CreateTemp("", "update-check-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer c.fs.RemoveAll(tempDir) //nolint:errcheck

	// Initialize git repo
	if err := c.gitClient.Init(tempDir); err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}

	// Add remote
	if err := c.gitClient.AddRemote(tempDir, "origin", url); err != nil {
		return "", fmt.Errorf("git remote add failed: %w", err)
	}

	// Fetch the specific ref with depth 1 (we only need the latest commit)
	if err := c.gitClient.Fetch(tempDir, 1, ref); err != nil {
		return "", fmt.Errorf("git fetch failed: %w", err)
	}

	// Get the commit hash
	hash, err := c.gitClient.GetHeadHash(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	return hash, nil
}

// formatTimeSince formats the time since last update in a human-readable way
func formatTimeSince(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "unknown"
	}

	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < 365*24*time.Hour:
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
