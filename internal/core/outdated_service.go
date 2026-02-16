package core

import (
	"context"
	"fmt"

	"github.com/EmundoT/git-vendor/internal/types"
)

// OutdatedOptions configures the outdated check.
type OutdatedOptions struct {
	Vendor string // Filter to a specific vendor name (empty = all)
}

// OutdatedServiceInterface defines the contract for checking vendor staleness
// via lightweight ls-remote queries (no temp dirs, no clones).
type OutdatedServiceInterface interface {
	Outdated(ctx context.Context, opts OutdatedOptions) (*types.OutdatedResult, error)
}

// Compile-time interface satisfaction check.
var _ OutdatedServiceInterface = (*OutdatedService)(nil)

// OutdatedService checks whether locked vendor commits are behind upstream HEAD
// using git ls-remote (one command per vendor, no cloning).
type OutdatedService struct {
	configStore ConfigStore
	lockStore   LockStore
	gitClient   GitClient
}

// NewOutdatedService creates a new OutdatedService with the given dependencies.
func NewOutdatedService(configStore ConfigStore, lockStore LockStore, gitClient GitClient) *OutdatedService {
	return &OutdatedService{
		configStore: configStore,
		lockStore:   lockStore,
		gitClient:   gitClient,
	}
}

// Outdated compares locked commit hashes against upstream HEAD for each dependency.
// Internal vendors (Source == "internal") are skipped. Unsynced vendors (no lock
// entry) are skipped. LsRemote errors are non-fatal: the vendor is skipped with
// the Skipped count incremented.
func (s *OutdatedService) Outdated(ctx context.Context, opts OutdatedOptions) (*types.OutdatedResult, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Build lock lookup: "name@ref" → LockDetails
	lockMap := make(map[string]*types.LockDetails)
	for i := range lock.Vendors {
		entry := &lock.Vendors[i]
		key := entry.Name + "@" + entry.Ref
		lockMap[key] = entry
	}

	result := &types.OutdatedResult{}

	for _, vendor := range config.Vendors {
		// Skip internal vendors — no remote to query
		if vendor.Source == "internal" {
			continue
		}

		// Apply vendor name filter
		if opts.Vendor != "" && vendor.Name != opts.Vendor {
			continue
		}

		for _, spec := range vendor.Specs {
			// Check context cancellation before each remote query
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			key := vendor.Name + "@" + spec.Ref
			lockEntry, locked := lockMap[key]
			if !locked {
				// Vendor/ref not in lockfile — skip (not yet synced)
				result.Skipped++
				continue
			}

			urls := ResolveVendorURLs(&vendor)
			latestHash, err := s.lsRemoteWithFallback(ctx, urls, spec.Ref)
			if err != nil {
				// Network/auth error — skip, don't fail the entire check
				result.Skipped++
				continue
			}

			upToDate := latestHash == lockEntry.CommitHash
			dep := types.UpdateCheckResult{
				VendorName:  vendor.Name,
				Ref:         spec.Ref,
				CurrentHash: lockEntry.CommitHash,
				LatestHash:  latestHash,
				LastUpdated: lockEntry.Updated,
				UpToDate:    upToDate,
			}

			result.Dependencies = append(result.Dependencies, dep)
			result.TotalChecked++

			if upToDate {
				result.UpToDate++
			} else {
				result.Outdated++
			}
		}
	}

	return result, nil
}

// lsRemoteWithFallback tries LsRemote against each URL in order until one succeeds.
// lsRemoteWithFallback returns the resolved hash from the first successful URL, or
// the last error if all URLs fail.
func (s *OutdatedService) lsRemoteWithFallback(ctx context.Context, urls []string, ref string) (string, error) {
	var lastErr error
	for _, url := range urls {
		hash, err := s.gitClient.LsRemote(ctx, url, ref)
		if err == nil {
			return hash, nil
		}
		lastErr = err
	}
	return "", lastErr
}
