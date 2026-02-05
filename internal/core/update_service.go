package core

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// UpdateService handles update operations and lockfile regeneration
type UpdateService struct {
	configStore ConfigStore
	lockStore   LockStore
	syncService *SyncService
	cache       CacheStore
	ui          UICallback
	rootDir     string
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(
	configStore ConfigStore,
	lockStore LockStore,
	syncService *SyncService,
	cache CacheStore,
	ui UICallback,
	rootDir string,
) *UpdateService {
	return &UpdateService{
		configStore: configStore,
		lockStore:   lockStore,
		syncService: syncService,
		cache:       cache,
		ui:          ui,
		rootDir:     rootDir,
	}
}

// UpdateAll updates all vendors and regenerates the lockfile
func (s *UpdateService) UpdateAll() error {
	return s.UpdateAllWithOptions(types.ParallelOptions{Enabled: false})
}

// UpdateAllWithOptions updates all vendors with optional parallel processing
func (s *UpdateService) UpdateAllWithOptions(parallelOpts types.ParallelOptions) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	if parallelOpts.Enabled {
		return s.updateAllParallel(config, parallelOpts)
	}

	return s.updateAllSequential(config)
}

// updateAllSequential performs sequential update (original implementation)
func (s *UpdateService) updateAllSequential(config types.VendorConfig) error {
	// Load existing lock to preserve VendoredAt and VendoredBy
	//nolint:errcheck // Lock file may not exist yet, empty struct is acceptable
	existingLock, _ := s.lockStore.Load()
	existingEntries := make(map[string]types.LockDetails)
	for _, entry := range existingLock.Vendors {
		key := entry.Name + "@" + entry.Ref
		existingEntries[key] = entry
	}

	lock := types.VendorLock{}
	now := time.Now().UTC().Format(time.RFC3339)
	user := GetGitUserIdentity()

	// Start progress tracking
	progress := s.ui.StartProgress(len(config.Vendors), "Updating vendors")
	defer progress.Complete()

	// Update each vendor
	for _, v := range config.Vendors {
		// Sync vendor without lock (force latest)
		// During update, we always force and skip cache (we want fresh data)
		updatedRefs, _, err := s.syncService.syncVendor(&v, nil, SyncOptions{Force: true, NoCache: true})
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			// Continue on error - don't fail the whole update
			progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
			continue
		}

		// Add lock entries for each ref
		for ref, metadata := range updatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicenseDir, v.Name+".txt")

			// Compute file hashes for all destination files
			fileHashes := s.computeFileHashes(&v, ref)

			// Preserve VendoredAt and VendoredBy from existing entry, or set to now
			key := v.Name + "@" + ref
			vendoredAt := now
			vendoredBy := user
			if existing, ok := existingEntries[key]; ok {
				if existing.VendoredAt != "" {
					vendoredAt = existing.VendoredAt
				}
				if existing.VendoredBy != "" {
					vendoredBy = existing.VendoredBy
				}
			}

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name:             v.Name,
				Ref:              ref,
				CommitHash:       metadata.CommitHash,
				LicensePath:      licenseFile,
				Updated:          now,
				FileHashes:       fileHashes,
				LicenseSPDX:      v.License,
				SourceVersionTag: metadata.VersionTag,
				VendoredAt:       vendoredAt,
				VendoredBy:       vendoredBy,
				LastSyncedAt:     now,
			})

			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, metadata.CommitHash[:7]))
		}

		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}

// updateAllParallel performs parallel update using worker pool
func (s *UpdateService) updateAllParallel(config types.VendorConfig, parallelOpts types.ParallelOptions) error {
	// Load existing lock to preserve VendoredAt and VendoredBy
	//nolint:errcheck // Lock file may not exist yet, empty struct is acceptable
	existingLock, _ := s.lockStore.Load()
	existingEntries := make(map[string]types.LockDetails)
	for _, entry := range existingLock.Vendors {
		key := entry.Name + "@" + entry.Ref
		existingEntries[key] = entry
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := GetGitUserIdentity()

	// Start progress tracking
	progress := s.ui.StartProgress(len(config.Vendors), "Updating vendors (parallel)")
	defer progress.Complete()

	// Create parallel executor
	executor := NewParallelExecutor(parallelOpts, s.ui)

	// Define update function for a single vendor
	updateFunc := func(v types.VendorSpec, opts SyncOptions) (map[string]RefMetadata, error) {
		updatedRefs, _, err := s.syncService.syncVendor(&v, nil, opts)
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
			return nil, err
		}

		// Show success for this vendor
		for ref, metadata := range updatedRefs {
			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, metadata.CommitHash[:7]))
		}
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))

		return updatedRefs, nil
	}

	// Execute parallel updates
	results, err := executor.ExecuteParallelUpdate(config.Vendors, updateFunc)
	if err != nil {
		// Continue even if some vendors failed - we still want to save successful updates
		s.ui.ShowWarning("Some Updates Failed", err.Error())
	}

	// Build new lockfile from results
	lock := types.VendorLock{}
	for i := range results {
		if results[i].Error != nil {
			// Skip failed vendors
			continue
		}

		// Add lock entries for each ref
		for ref, metadata := range results[i].UpdatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicenseDir, results[i].Vendor.Name+".txt")

			// Compute file hashes for all destination files
			fileHashes := s.computeFileHashes(&results[i].Vendor, ref)

			// Preserve VendoredAt and VendoredBy from existing entry, or set to now
			key := results[i].Vendor.Name + "@" + ref
			vendoredAt := now
			vendoredBy := user
			if existing, ok := existingEntries[key]; ok {
				if existing.VendoredAt != "" {
					vendoredAt = existing.VendoredAt
				}
				if existing.VendoredBy != "" {
					vendoredBy = existing.VendoredBy
				}
			}

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name:             results[i].Vendor.Name,
				Ref:              ref,
				CommitHash:       metadata.CommitHash,
				LicensePath:      licenseFile,
				Updated:          now,
				FileHashes:       fileHashes,
				LicenseSPDX:      results[i].Vendor.License,
				SourceVersionTag: metadata.VersionTag,
				VendoredAt:       vendoredAt,
				VendoredBy:       vendoredBy,
				LastSyncedAt:     now,
			})
		}
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}

// computeFileHashes calculates SHA-256 hashes for all destination files of a vendor
func (s *UpdateService) computeFileHashes(vendor *types.VendorSpec, ref string) map[string]string {
	fileHashes := make(map[string]string)

	// Find the matching spec for this ref
	var matchingSpec *types.BranchSpec
	for i := range vendor.Specs {
		if vendor.Specs[i].Ref == ref {
			matchingSpec = &vendor.Specs[i]
			break
		}
	}

	if matchingSpec == nil {
		return fileHashes
	}

	// Iterate through mappings and compute hashes
	for _, mapping := range matchingSpec.Mapping {
		destPath := mapping.To
		if destPath == "" {
			// Use auto-computed path
			destPath = ComputeAutoPath(mapping.From, matchingSpec.DefaultTarget, vendor.Name)
		}

		// Compute hash for this file
		hash, err := s.cache.ComputeFileChecksum(destPath)
		if err == nil {
			fileHashes[destPath] = hash
		}
	}

	return fileHashes
}