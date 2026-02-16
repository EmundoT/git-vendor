package core

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// UpdateOptions configures update operation behavior.
// VendorName and Group are mutually exclusive filters; when set, only matching
// vendors are re-fetched and their lock entries regenerated. Non-matching
// vendors retain their existing lock entries unchanged.
type UpdateOptions struct {
	Parallel   types.ParallelOptions
	Local      bool   // Allow file:// and local path vendor URLs
	VendorName string // Filter to single vendor by name (empty = all)
	Group      string // Filter to vendor group (empty = all)
}

// UpdateServiceInterface defines the contract for update operations and lockfile regeneration.
// UpdateServiceInterface enables mocking in tests and alternative update strategies.
// All methods accept a context.Context for cancellation support (e.g., Ctrl+C).
type UpdateServiceInterface interface {
	UpdateAll(ctx context.Context) error
	UpdateAllWithOptions(ctx context.Context, opts UpdateOptions) error
}

// Compile-time interface satisfaction check.
var _ UpdateServiceInterface = (*UpdateService)(nil)

// UpdateService handles update operations and lockfile regeneration
type UpdateService struct {
	configStore  ConfigStore
	lockStore    LockStore
	syncService  SyncServiceInterface
	internalSync InternalSyncServiceInterface // Spec 070
	cache        CacheStore
	ui           UICallback
	rootDir      string
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(
	configStore ConfigStore,
	lockStore LockStore,
	syncService SyncServiceInterface,
	internalSync InternalSyncServiceInterface,
	cache CacheStore,
	ui UICallback,
	rootDir string,
) *UpdateService {
	return &UpdateService{
		configStore:  configStore,
		lockStore:    lockStore,
		syncService:  syncService,
		internalSync: internalSync,
		cache:        cache,
		ui:           ui,
		rootDir:      rootDir,
	}
}

// UpdateAll updates all vendors and regenerates the lockfile.
// ctx controls cancellation of git operations during update.
func (s *UpdateService) UpdateAll(ctx context.Context) error {
	return s.UpdateAllWithOptions(ctx, UpdateOptions{})
}

// UpdateAllWithOptions updates vendors with optional parallel processing, local path support,
// and vendor name/group filtering. When VendorName or Group is set, only matching vendors
// are re-fetched; non-matching vendors retain their existing lock entries.
// ctx controls cancellation of git operations during update.
func (s *UpdateService) UpdateAllWithOptions(ctx context.Context, opts UpdateOptions) error {
	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validate vendor name filter
	if opts.VendorName != "" {
		if err := s.validateVendorExists(config, opts.VendorName); err != nil {
			return err
		}
	}

	// Validate group filter
	if opts.Group != "" {
		if err := s.validateGroupExists(config, opts.Group); err != nil {
			return err
		}
	}

	if opts.Parallel.Enabled {
		return s.updateAllParallel(ctx, config, opts)
	}

	return s.updateAllSequential(ctx, config, opts)
}

// updateAllSequential performs sequential update (original implementation).
// When opts.VendorName or opts.Group is set, only matching vendors are updated;
// non-matching vendors retain their existing lock entries verbatim.
// ctx controls cancellation — checked at each vendor boundary.
func (s *UpdateService) updateAllSequential(ctx context.Context, config types.VendorConfig, opts UpdateOptions) error {
	filtered := s.isFiltered(opts)

	// Load existing lock to preserve VendoredAt/VendoredBy and carry forward
	// unfiltered vendor entries when a name/group filter is active.
	//nolint:errcheck // Lock file may not exist yet, empty struct is acceptable
	existingLock, _ := s.lockStore.Load()
	existingEntries := make(map[string]types.LockDetails)
	for i := range existingLock.Vendors {
		entry := &existingLock.Vendors[i]
		key := entry.Name + "@" + entry.Ref
		existingEntries[key] = *entry
	}

	lock := types.VendorLock{}
	now := time.Now().UTC().Format(time.RFC3339)
	user := GetGitUserIdentity()

	// Determine which vendors to update
	vendorsToUpdate := s.filterVendors(config.Vendors, opts)

	// Start progress tracking
	progress := s.ui.StartProgress(len(vendorsToUpdate), "Updating vendors")
	defer progress.Complete()

	// Track which vendor names were targeted for update (for lock merge)
	updatedVendorNames := make(map[string]bool)

	// Update each targeted vendor
	for _, v := range vendorsToUpdate {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		updatedVendorNames[v.Name] = true

		var updatedRefs map[string]RefMetadata

		if v.Source == SourceInternal {
			// Internal vendor: sync locally, no git operations
			if s.internalSync == nil {
				s.ui.ShowError("Update Failed", fmt.Sprintf("%s: internal sync service not configured", v.Name))
				progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
				continue
			}
			refs, _, err := s.internalSync.SyncInternalVendor(&v, SyncOptions{Force: true, NoCache: true})
			if err != nil {
				s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
				progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
				continue
			}
			updatedRefs = refs
		} else {
			// External vendor: sync via git
			refs, _, err := s.syncService.SyncVendor(ctx, &v, nil, SyncOptions{Force: true, NoCache: true, Local: opts.Local})
			if err != nil {
				s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
				progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
				continue
			}
			updatedRefs = refs
		}

		// Add lock entries for each ref
		for ref, metadata := range updatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicensesDir, v.Name+".txt")

			// Compute file hashes for all destination files
			fileHashes := s.computeFileHashes(&v, ref)

			// Compute source file hashes for internal vendors
			var sourceFileHashes map[string]string
			if v.Source == SourceInternal {
				sourceFileHashes = s.computeSourceFileHashes(&v, ref)
			}

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

			entry := types.LockDetails{
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
				Positions:        toPositionLocks(metadata.Positions),
			}

			if v.Source == SourceInternal {
				entry.Source = SourceInternal
				entry.SourceFileHashes = sourceFileHashes
				entry.LicensePath = "" // Internal vendors have no license
			}

			lock.Vendors = append(lock.Vendors, entry)

			hashDisplay := metadata.CommitHash
			if len(hashDisplay) > 7 {
				hashDisplay = hashDisplay[:7]
			}
			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, hashDisplay))
		}

		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// When filtered, carry forward existing lock entries for non-targeted vendors
	if filtered {
		for _, entry := range existingLock.Vendors {
			if !updatedVendorNames[entry.Name] {
				lock.Vendors = append(lock.Vendors, entry)
			}
		}
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}

// updateAllParallel performs parallel update using worker pool.
// When opts.VendorName or opts.Group is set, only matching vendors are updated;
// non-matching vendors retain their existing lock entries verbatim.
// ctx controls cancellation — passed to the parallel executor and each worker.
func (s *UpdateService) updateAllParallel(ctx context.Context, config types.VendorConfig, opts UpdateOptions) error {
	filtered := s.isFiltered(opts)
	parallelOpts := opts.Parallel

	// Load existing lock to preserve VendoredAt/VendoredBy and carry forward
	// unfiltered vendor entries when a name/group filter is active.
	//nolint:errcheck // Lock file may not exist yet, empty struct is acceptable
	existingLock, _ := s.lockStore.Load()
	existingEntries := make(map[string]types.LockDetails)
	for i := range existingLock.Vendors {
		entry := &existingLock.Vendors[i]
		key := entry.Name + "@" + entry.Ref
		existingEntries[key] = *entry
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := GetGitUserIdentity()

	// Filter vendors based on options
	vendorsToUpdate := s.filterVendors(config.Vendors, opts)

	// Start progress tracking
	progress := s.ui.StartProgress(len(vendorsToUpdate), "Updating vendors (parallel)")
	defer progress.Complete()

	// Create parallel executor
	executor := NewParallelExecutor(parallelOpts, s.ui)

	// Track which vendor names were targeted for update (for lock merge)
	updatedVendorNames := make(map[string]bool)

	// Phase 1: Internal vendors — sequential (before parallel external vendors)
	lock := types.VendorLock{}
	var externalVendors []types.VendorSpec

	for _, v := range vendorsToUpdate {
		updatedVendorNames[v.Name] = true

		if v.Source == SourceInternal {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if s.internalSync == nil {
				s.ui.ShowError("Update Failed", fmt.Sprintf("%s: internal sync service not configured", v.Name))
				progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
				continue
			}
			refs, _, err := s.internalSync.SyncInternalVendor(&v, SyncOptions{Force: true, NoCache: true})
			if err != nil {
				s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
				progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
				continue
			}

			for ref, metadata := range refs {
				fileHashes := s.computeFileHashes(&v, ref)
				sourceFileHashes := s.computeSourceFileHashes(&v, ref)
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
					Updated:          now,
					FileHashes:       fileHashes,
					VendoredAt:       vendoredAt,
					VendoredBy:       vendoredBy,
					LastSyncedAt:     now,
					Positions:        toPositionLocks(metadata.Positions),
					Source:           SourceInternal,
					SourceFileHashes: sourceFileHashes,
				})

				hashDisplay := metadata.CommitHash
				if len(hashDisplay) > 7 {
					hashDisplay = hashDisplay[:7]
				}
				s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, hashDisplay))
			}
			progress.Increment(fmt.Sprintf("✓ %s", v.Name))
		} else {
			externalVendors = append(externalVendors, v)
		}
	}

	// Phase 2: External vendors — parallel
	// Define update function for a single vendor
	updateFunc := func(workerCtx context.Context, v types.VendorSpec, syncOpts SyncOptions) (map[string]RefMetadata, error) {
		syncOpts.Local = opts.Local
		updatedRefs, _, err := s.syncService.SyncVendor(workerCtx, &v, nil, syncOpts)
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
			return nil, err
		}

		for ref, metadata := range updatedRefs {
			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, metadata.CommitHash[:7]))
		}
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))

		return updatedRefs, nil
	}

	// Execute parallel updates for external vendors
	results, err := executor.ExecuteParallelUpdate(ctx, externalVendors, updateFunc)
	if err != nil {
		s.ui.ShowWarning("Some Updates Failed", err.Error())
	}

	// Build lockfile entries from parallel results
	for i := range results {
		if results[i].Error != nil {
			continue
		}

		for ref, metadata := range results[i].UpdatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicensesDir, results[i].Vendor.Name+".txt")
			fileHashes := s.computeFileHashes(&results[i].Vendor, ref)

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
				Positions:        toPositionLocks(metadata.Positions),
			})
		}
	}

	// When filtered, carry forward existing lock entries for non-targeted vendors
	if filtered {
		for _, entry := range existingLock.Vendors {
			if !updatedVendorNames[entry.Name] {
				lock.Vendors = append(lock.Vendors, entry)
			}
		}
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}

// isFiltered reports whether the UpdateOptions specify a vendor name or group filter.
func (s *UpdateService) isFiltered(opts UpdateOptions) bool {
	return opts.VendorName != "" || opts.Group != ""
}

// validateVendorExists returns a VendorNotFoundError if no vendor with vendorName
// exists in the config.
func (s *UpdateService) validateVendorExists(config types.VendorConfig, vendorName string) error {
	for _, v := range config.Vendors {
		if v.Name == vendorName {
			return nil
		}
	}
	return NewVendorNotFoundError(vendorName)
}

// validateGroupExists returns a GroupNotFoundError if no vendor in the config
// belongs to the given group.
func (s *UpdateService) validateGroupExists(config types.VendorConfig, groupName string) error {
	for _, v := range config.Vendors {
		for _, g := range v.Groups {
			if g == groupName {
				return nil
			}
		}
	}
	return NewGroupNotFoundError(groupName)
}

// filterVendors returns the subset of vendors matching the UpdateOptions filters.
// If no filter is set, all vendors are returned.
func (s *UpdateService) filterVendors(vendors []types.VendorSpec, opts UpdateOptions) []types.VendorSpec {
	if opts.VendorName == "" && opts.Group == "" {
		return vendors
	}

	var filtered []types.VendorSpec
	for _, v := range vendors {
		if opts.VendorName != "" && v.Name != opts.VendorName {
			continue
		}
		if opts.Group != "" {
			hasGroup := false
			for _, g := range v.Groups {
				if g == opts.Group {
					hasGroup = true
					break
				}
			}
			if !hasGroup {
				continue
			}
		}
		filtered = append(filtered, v)
	}
	return filtered
}

// toPositionLocks converts internal position records to lockfile-safe types.
func toPositionLocks(records []positionRecord) []types.PositionLock {
	if len(records) == 0 {
		return nil
	}
	locks := make([]types.PositionLock, len(records))
	for i, r := range records {
		locks[i] = types.PositionLock{
			From:       r.From,
			To:         r.To,
			SourceHash: r.SourceHash,
		}
	}
	return locks
}

// computeSourceFileHashes calculates SHA-256 hashes for all source files of an internal vendor.
// Source file hashes enable drift detection: comparing current source state vs locked state.
func (s *UpdateService) computeSourceFileHashes(vendor *types.VendorSpec, ref string) map[string]string {
	sourceHashes := make(map[string]string)

	var matchingSpec *types.BranchSpec
	for i := range vendor.Specs {
		if vendor.Specs[i].Ref == ref {
			matchingSpec = &vendor.Specs[i]
			break
		}
	}
	if matchingSpec == nil {
		return sourceHashes
	}

	for _, mapping := range matchingSpec.Mapping {
		srcFile, _, err := types.ParsePathPosition(mapping.From)
		if err != nil {
			srcFile = mapping.From
		}

		hash, err := s.cache.ComputeFileChecksum(srcFile)
		if err == nil {
			sourceHashes[srcFile] = hash
		}
	}

	return sourceHashes
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
			// Use auto-computed path — strip position from source for auto-naming
			srcFile, _, err := types.ParsePathPosition(mapping.From)
			if err != nil {
				srcFile = mapping.From
			}
			destPath = ComputeAutoPath(srcFile, matchingSpec.DefaultTarget, vendor.Name)
		}

		// Strip position specifier from destination path for file system access
		destFile, _, err := types.ParsePathPosition(destPath)
		if err != nil {
			destFile = destPath
		}

		// Compute hash for this file
		hash, err := s.cache.ComputeFileChecksum(destFile)
		if err == nil {
			fileHashes[destFile] = hash
		}
	}

	return fileHashes
}
