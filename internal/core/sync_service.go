package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// SyncOptions configures sync operation behavior
type SyncOptions struct {
	DryRun     bool
	VendorName string // Empty = all vendors
	GroupName  string // Empty = all groups, filters vendors by group
	Force      bool
	NoCache    bool                  // Disable incremental sync cache
	Parallel   types.ParallelOptions // Parallel processing options
}

// RefMetadata holds per-ref metadata collected during sync
type RefMetadata struct {
	CommitHash string
	VersionTag string             // Git tag pointing to commit, if any
	Positions  []positionRecord   // Position extractions performed during sync
}

// SyncServiceInterface defines the contract for vendor synchronization.
// This interface enables mocking in tests and potential alternative sync strategies.
type SyncServiceInterface interface {
	// Sync synchronizes vendors based on the provided options, loading config and lock internally.
	Sync(opts SyncOptions) error

	// SyncVendor syncs a single vendor's refs and returns per-ref metadata and copy stats.
	//
	// lockedRefs controls the sync mode:
	//   - nil: update mode — fetches the latest commit on each ref (FETCH_HEAD).
	//   - non-nil map: sync mode — checks out the exact commit hash for each ref.
	//     Missing or empty entries within the map are treated as unlocked for that ref.
	SyncVendor(v *types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]RefMetadata, CopyStats, error)
}

// Compile-time interface satisfaction check.
var _ SyncServiceInterface = (*SyncService)(nil)

// SyncService handles vendor synchronization operations
type SyncService struct {
	configStore ConfigStore
	lockStore   LockStore
	gitClient   GitClient
	fs          FileSystem
	fileCopy    FileCopyServiceInterface
	license     LicenseServiceInterface
	cache       CacheStore
	hooks       HookExecutor
	ui          UICallback
	rootDir     string
}

// NewSyncService creates a new SyncService
func NewSyncService(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	fileCopy FileCopyServiceInterface,
	license LicenseServiceInterface,
	cache CacheStore,
	hooks HookExecutor,
	ui UICallback,
	rootDir string,
) *SyncService {
	return &SyncService{
		configStore: configStore,
		lockStore:   lockStore,
		gitClient:   gitClient,
		fs:          fs,
		fileCopy:    fileCopy,
		license:     license,
		cache:       cache,
		hooks:       hooks,
		ui:          ui,
		rootDir:     rootDir,
	}
}

// Sync performs synchronization based on the provided options
// Note: Caller should check for lockfile existence before calling this
func (s *SyncService) Sync(opts SyncOptions) error {
	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return fmt.Errorf("load lockfile: %w", err)
	}

	// Build lock map for quick lookups
	lockMap := s.buildLockMap(lock)

	// Validate vendor exists if filtering by name
	if opts.VendorName != "" {
		if err := s.validateVendorExists(config, opts.VendorName); err != nil {
			return err // Already a structured error type
		}
	}

	// Validate group exists if filtering by group
	if opts.GroupName != "" {
		if err := s.validateGroupExists(config, opts.GroupName); err != nil {
			return err // Already a structured error type
		}
	}

	// Print header
	if opts.DryRun {
		fmt.Println(s.ui.StyleTitle("Sync Plan:"))
		fmt.Println()
	} else {
		s.printSyncHeader(config, opts.VendorName)
	}

	// Filter vendors based on options
	var vendorsToSync []types.VendorSpec
	for _, v := range config.Vendors {
		if s.shouldSyncVendor(&v, opts) {
			vendorsToSync = append(vendorsToSync, v)
		}
	}

	// Dry-run mode always uses sequential processing
	if opts.DryRun {
		return s.syncDryRun(vendorsToSync, lockMap)
	}

	// Use parallel or sequential sync based on options
	if opts.Parallel.Enabled {
		return s.syncParallel(vendorsToSync, lockMap, opts)
	}

	return s.syncSequential(vendorsToSync, lockMap, opts)
}

// syncDryRun performs dry-run preview (always sequential)
func (s *SyncService) syncDryRun(vendors []types.VendorSpec, lockMap map[string]map[string]string) error {
	progress := s.ui.StartProgress(len(vendors), "Previewing sync")
	defer progress.Complete()

	for _, v := range vendors {
		s.previewSyncVendor(&v, lockMap[v.Name])
		progress.Increment(v.Name)
	}

	return nil
}

// syncSequential performs sequential sync (original implementation)
func (s *SyncService) syncSequential(vendors []types.VendorSpec, lockMap map[string]map[string]string, opts SyncOptions) error {
	// Start progress tracking
	progress := s.ui.StartProgress(len(vendors), "Syncing vendors")
	defer progress.Complete()

	// Track total stats across all vendors
	var totalStats CopyStats

	// Sync vendors
	for _, v := range vendors {
		// If force is true, pass nil to ignore lock and re-download
		refs := lockMap[v.Name]
		if opts.Force {
			refs = nil
		}
		_, stats, err := s.SyncVendor(&v, refs, opts)
		if err != nil {
			progress.Fail(err)
			return fmt.Errorf("sync vendor %s: %w", v.Name, err)
		}
		totalStats.Add(stats)
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// Display summary
	if totalStats.FileCount > 0 {
		fmt.Println()
		fmt.Printf("Summary: Synced %s across all vendors\n", Pluralize(totalStats.FileCount, "file", "files"))
	}

	return nil
}

// syncParallel performs parallel sync using worker pool
func (s *SyncService) syncParallel(vendors []types.VendorSpec, lockMap map[string]map[string]string, opts SyncOptions) error {
	// Start progress tracking
	progress := s.ui.StartProgress(len(vendors), "Syncing vendors (parallel)")
	defer progress.Complete()

	// Create parallel executor
	executor := NewParallelExecutor(opts.Parallel, s.ui)

	// Define sync function for a single vendor
	syncFunc := func(v types.VendorSpec, lockedRefs map[string]string, syncOpts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		updatedRefs, stats, err := s.SyncVendor(&v, lockedRefs, syncOpts)
		if err != nil {
			progress.Fail(err)
			return nil, CopyStats{}, err
		}
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
		return updatedRefs, stats, nil
	}

	// Execute parallel sync
	results, err := executor.ExecuteParallelSync(vendors, lockMap, opts, syncFunc)
	if err != nil {
		return fmt.Errorf("parallel sync: %w", err)
	}

	// Calculate total stats
	var totalStats CopyStats
	for i := range results {
		totalStats.Add(results[i].Stats)
	}

	// Display summary
	if totalStats.FileCount > 0 {
		fmt.Println()
		fmt.Printf("Summary: Synced %s across all vendors\n", Pluralize(totalStats.FileCount, "file", "files"))
	}

	return nil
}

// buildLockMap builds a map of vendor name to ref to commit hash
func (s *SyncService) buildLockMap(lock types.VendorLock) map[string]map[string]string {
	lockMap := make(map[string]map[string]string)
	for i := range lock.Vendors {
		l := &lock.Vendors[i]
		if lockMap[l.Name] == nil {
			lockMap[l.Name] = make(map[string]string)
		}
		lockMap[l.Name][l.Ref] = l.CommitHash
	}
	return lockMap
}

// validateVendorExists checks if a vendor with the given name exists
func (s *SyncService) validateVendorExists(config types.VendorConfig, vendorName string) error {
	found := false
	for _, v := range config.Vendors {
		if v.Name == vendorName {
			found = true
			break
		}
	}
	if !found {
		return NewVendorNotFoundError(vendorName)
	}
	return nil
}

// validateGroupExists checks if any vendor has the given group
func (s *SyncService) validateGroupExists(config types.VendorConfig, groupName string) error {
	found := false
	for _, v := range config.Vendors {
		for _, g := range v.Groups {
			if g == groupName {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return NewGroupNotFoundError(groupName)
	}
	return nil
}

// shouldSyncVendor checks if a vendor should be synced based on filters
func (s *SyncService) shouldSyncVendor(v *types.VendorSpec, opts SyncOptions) bool {
	// If vendor name filter is set, only sync matching vendor
	if opts.VendorName != "" && v.Name != opts.VendorName {
		return false
	}

	// If group filter is set, only sync vendors in that group
	if opts.GroupName != "" {
		hasGroup := false
		for _, g := range v.Groups {
			if g == opts.GroupName {
				hasGroup = true
				break
			}
		}
		if !hasGroup {
			return false
		}
	}

	return true
}

// printSyncHeader prints the sync header message
func (s *SyncService) printSyncHeader(config types.VendorConfig, vendorName string) {
	vendorCount := 0
	for _, v := range config.Vendors {
		if vendorName == "" || v.Name == vendorName {
			vendorCount++
		}
	}
	if vendorCount > 0 {
		fmt.Printf("Syncing %s...\n", Pluralize(vendorCount, "vendor", "vendors"))
	}
}

// previewSyncVendor shows what would be synced in dry-run mode
func (s *SyncService) previewSyncVendor(v *types.VendorSpec, lockedRefs map[string]string) {
	fmt.Printf("✓ %s\n", v.Name)

	for _, spec := range v.Specs {
		status := "not synced"
		if lockedRefs != nil {
			if h, ok := lockedRefs[spec.Ref]; ok && h != "" {
				status = fmt.Sprintf("locked: %s", h[:7])
			}
		}

		fmt.Printf("  @ %s (%s)\n", spec.Ref, status)

		if len(spec.Mapping) == 0 {
			fmt.Printf("    (no paths configured)\n")
		} else {
			for _, m := range spec.Mapping {
				dest := m.To
				if dest == "" {
					dest = "(auto)"
				}
				fmt.Printf("    → %s → %s\n", m.From, dest)
			}
		}
	}
	fmt.Println()
}

// SyncVendor syncs a single vendor.
// Returns a map of ref to RefMetadata and total stats for all synced refs.
func (s *SyncService) SyncVendor(v *types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
	// Check cache for all refs first (if cache enabled)
	canSkipClone := false
	if !opts.NoCache && !opts.Force && lockedRefs != nil {
		allCached := true
		for _, spec := range v.Specs {
			if !s.canSkipSync(v.Name, spec.Ref, lockedRefs[spec.Ref], spec.Mapping) {
				allCached = false
				break
			}
		}
		canSkipClone = allCached
	}

	// If all refs are cached and up-to-date, skip git operations entirely
	if canSkipClone {
		fmt.Printf("⚡ %s (cache hit, skipping download)\n", v.Name)
		results := make(map[string]RefMetadata)
		var totalStats CopyStats

		for _, spec := range v.Specs {
			// For cached syncs, we don't have access to version tag
			results[spec.Ref] = RefMetadata{CommitHash: lockedRefs[spec.Ref]}
			// Files already exist, count them
			stats := CopyStats{FileCount: len(spec.Mapping)}
			totalStats.Add(stats)
			fmt.Printf("  ✓ %s @ %s (cached: %s)\n",
				v.Name, spec.Ref,
				Pluralize(len(spec.Mapping), "path", "paths"))
		}

		// Execute post-sync hook even for cached syncs
		if v.Hooks != nil && v.Hooks.PostSync != "" {
			ctx := types.HookContext{
				VendorName:  v.Name,
				VendorURL:   v.URL,
				RootDir:     s.rootDir,
				FilesCopied: totalStats.FileCount,
			}
			if err := s.hooks.ExecutePostSync(v, &ctx); err != nil {
				return nil, CopyStats{}, fmt.Errorf("post-sync hook failed: %w", err)
			}
		}

		return results, totalStats, nil
	}

	// Execute pre-sync hook before cloning
	if v.Hooks != nil && v.Hooks.PreSync != "" {
		ctx := types.HookContext{
			VendorName: v.Name,
			VendorURL:  v.URL,
			RootDir:    s.rootDir,
		}
		if err := s.hooks.ExecutePreSync(v, &ctx); err != nil {
			return nil, CopyStats{}, fmt.Errorf("pre-sync hook failed: %w", err)
		}
	}

	fmt.Printf("⠿ %s (cloning repository...)\n", v.Name)

	// Create temp directory for cloning
	tempDir, err := s.fs.CreateTemp("", "git-vendor-*")
	if err != nil {
		return nil, CopyStats{}, err
	}
	defer func() { _ = s.fs.RemoveAll(tempDir) }() //nolint:errcheck // cleanup in defer

	ctx := context.Background()

	// Initialize git repo
	if err := s.gitClient.Init(ctx, tempDir); err != nil {
		return nil, CopyStats{}, fmt.Errorf("failed to initialize git repository for %s: %w", v.Name, err)
	}
	if err := s.gitClient.AddRemote(ctx, tempDir, "origin", v.URL); err != nil {
		return nil, CopyStats{}, fmt.Errorf("failed to add remote for %s (%s): %w\n\nPlease verify the repository URL is correct and accessible", v.Name, v.URL, err)
	}

	results := make(map[string]RefMetadata)
	var totalStats CopyStats

	// Sync each ref
	for _, spec := range v.Specs {
		metadata, stats, err := s.syncRef(tempDir, v, spec, lockedRefs, opts)
		if err != nil {
			return nil, CopyStats{}, err
		}
		results[spec.Ref] = metadata
		totalStats.Add(stats)

		// Display stats with proper pluralization
		fmt.Printf("  ✓ %s @ %s (synced %s: %s)\n",
			v.Name, spec.Ref,
			Pluralize(len(spec.Mapping), "path", "paths"),
			Pluralize(stats.FileCount, "file", "files"))
	}

	// Execute post-sync hook after successful sync
	if v.Hooks != nil && v.Hooks.PostSync != "" {
		// Get the first ref's commit hash for context (if multiple refs, use the first)
		firstHash := ""
		firstRef := ""
		for ref, metadata := range results {
			firstHash = metadata.CommitHash
			firstRef = ref
			break
		}

		ctx := types.HookContext{
			VendorName:  v.Name,
			VendorURL:   v.URL,
			Ref:         firstRef,
			CommitHash:  firstHash,
			RootDir:     s.rootDir,
			FilesCopied: totalStats.FileCount,
		}
		if err := s.hooks.ExecutePostSync(v, &ctx); err != nil {
			return nil, CopyStats{}, fmt.Errorf("post-sync hook failed: %w", err)
		}
	}

	return results, totalStats, nil
}

// syncRef syncs a single ref for a vendor
func (s *SyncService) syncRef(tempDir string, v *types.VendorSpec, spec types.BranchSpec, lockedRefs map[string]string, opts SyncOptions) (RefMetadata, CopyStats, error) {
	targetCommit := ""
	isLocked := false

	// Check if we have a locked commit hash
	if lockedRefs != nil {
		if h, ok := lockedRefs[spec.Ref]; ok && h != "" {
			targetCommit = h
			isLocked = true
		}
	}

	// Fetch and checkout
	fmt.Printf("  ⠿ Fetching ref '%s'...\n", spec.Ref)
	ctx := context.Background()

	if isLocked {
		// Locked sync - checkout specific commit
		if err := s.fetchWithFallback(tempDir, spec.Ref); err != nil {
			return RefMetadata{}, CopyStats{}, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
		}
		if err := s.gitClient.Checkout(ctx, tempDir, targetCommit); err != nil {
			// Detect stale lock hash error and provide helpful message
			errMsg := err.Error()
			if strings.Contains(errMsg, "reference is not a tree") || strings.Contains(errMsg, "not a valid object") {
				return RefMetadata{}, CopyStats{}, NewStaleCommitError(targetCommit, v.Name, spec.Ref)
			}
			return RefMetadata{}, CopyStats{}, NewCheckoutError(targetCommit, v.Name, err)
		}
	} else {
		// Unlocked sync - checkout latest
		if err := s.fetchWithFallback(tempDir, spec.Ref); err != nil {
			return RefMetadata{}, CopyStats{}, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
		}
		if err := s.gitClient.Checkout(ctx, tempDir, FetchHead); err != nil {
			if err := s.gitClient.Checkout(ctx, tempDir, spec.Ref); err != nil {
				return RefMetadata{}, CopyStats{}, NewCheckoutError(spec.Ref, v.Name, err)
			}
		}
	}

	// Get current commit hash
	hash, err := s.gitClient.GetHeadHash(ctx, tempDir)
	if err != nil {
		return RefMetadata{}, CopyStats{}, fmt.Errorf("failed to get commit hash for %s @ %s: %w", v.Name, spec.Ref, err)
	}

	// Get version tag for this commit (if any)
	//nolint:errcheck // Version tag is optional, empty string is acceptable fallback
	versionTag, _ := s.gitClient.GetTagForCommit(ctx, tempDir, hash)

	// Copy license file (don't count in stats)
	if err := s.license.CopyLicense(tempDir, v.Name); err != nil {
		return RefMetadata{}, CopyStats{}, err
	}

	// Copy files according to mappings and collect stats
	fmt.Printf("  ⠿ Copying files...\n")
	stats, err := s.fileCopy.CopyMappings(tempDir, v, spec)
	if err != nil {
		return RefMetadata{}, CopyStats{}, err
	}

	// Surface any position extraction warnings (e.g., local modifications being overwritten)
	for _, w := range stats.Warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}

	// Build and save cache (if cache enabled)
	if !opts.NoCache {
		if err := s.updateCache(v.Name, spec, hash); err != nil {
			// Cache update failure shouldn't fail the sync
			// Just log a warning and continue
			fmt.Printf("  ⚠ Warning: failed to update cache: %v\n", err)
		}
	}

	return RefMetadata{CommitHash: hash, VersionTag: versionTag, Positions: stats.Positions}, stats, nil
}

// fetchWithFallback tries shallow fetch first, falls back to full fetch if needed
// This eliminates duplicate fetch retry logic
func (s *SyncService) fetchWithFallback(tempDir, ref string) error {
	ctx := context.Background()
	// Try shallow fetch first
	if err := s.gitClient.Fetch(ctx, tempDir, 1, ref); err != nil {
		// Shallow fetch failed, try full fetch
		if err := s.gitClient.FetchAll(ctx, tempDir); err != nil {
			return fmt.Errorf("fetch ref %s: %w", ref, err)
		}
	}
	return nil
}

// canSkipSync checks if a vendor@ref can skip sync based on cache
func (s *SyncService) canSkipSync(vendorName, ref, commitHash string, mappings []types.PathMapping) bool {
	// Load cache for this vendor@ref
	cache, err := s.cache.Load(vendorName, ref)
	if err != nil || cache.CommitHash == "" {
		// Cache miss or error - can't skip
		return false
	}

	// Check if commit hash matches
	if cache.CommitHash != commitHash {
		// Commit hash changed - invalidate cache
		return false
	}

	// Build a map of cached checksums for quick lookup
	cachedChecksums := make(map[string]string)
	for _, fc := range cache.Files {
		cachedChecksums[fc.Path] = fc.Hash
	}

	// Validate all destination files exist and match cached checksums
	for _, mapping := range mappings {
		destPath := mapping.To
		if destPath == "" {
			// Auto-naming not supported in cache check (too complex)
			return false
		}

		// Strip position specifier from destination path for file system access
		destFile, _, err := types.ParsePathPosition(destPath)
		if err != nil {
			destFile = destPath
		}

		// Check if file exists
		fullPath := filepath.Join(s.rootDir, destFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// File missing - can't skip
			return false
		}

		// Check checksum
		currentHash, err := s.cache.ComputeFileChecksum(fullPath)
		if err != nil {
			// Can't compute checksum - can't skip
			return false
		}

		cachedHash, exists := cachedChecksums[destFile]
		if !exists || cachedHash != currentHash {
			// Checksum mismatch or not in cache - can't skip
			return false
		}
	}

	// All checks passed - can skip sync
	return true
}

// updateCache builds and saves cache for a vendor@ref
func (s *SyncService) updateCache(vendorName string, spec types.BranchSpec, commitHash string) error {
	// Collect destination file paths
	var destPaths []string
	for _, mapping := range spec.Mapping {
		destPath := mapping.To
		if destPath == "" {
			// Skip auto-named files (too complex to track)
			continue
		}
		// Strip position specifier from destination path for file system access
		destFile, _, err := types.ParsePathPosition(destPath)
		if err != nil {
			destFile = destPath
		}
		fullPath := filepath.Join(s.rootDir, destFile)
		destPaths = append(destPaths, fullPath)
	}

	// Build cache with checksums
	cache, err := s.cache.BuildCache(vendorName, spec.Ref, commitHash, destPaths)
	if err != nil {
		return fmt.Errorf("build cache for %s@%s: %w", vendorName, spec.Ref, err)
	}

	// Save cache
	if err := s.cache.Save(&cache); err != nil {
		return fmt.Errorf("save cache for %s@%s: %w", vendorName, spec.Ref, err)
	}
	return nil
}
