package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// SyncOptions configures sync operation behavior
type SyncOptions struct {
	DryRun       bool
	VendorName   string // Empty = all vendors
	GroupName    string // Empty = all groups, filters vendors by group
	Force        bool
	NoCache      bool                  // Disable incremental sync cache
	Parallel     types.ParallelOptions // Parallel processing options
	Commit       bool                  // Auto-commit after sync with vendor trailers
	InternalOnly bool                  // Only sync internal vendors (Spec 070)
	Reverse      bool                  // Propagate dest changes back to source (Spec 070)
	Local        bool                  // Allow file:// and local path vendor URLs
}

// RefMetadata holds per-ref metadata collected during sync
type RefMetadata struct {
	CommitHash string
	VersionTag string             // Git tag pointing to commit, if any
	Positions  []positionRecord   // Position extractions performed during sync
}

// SyncServiceInterface defines the contract for vendor synchronization.
// SyncServiceInterface enables mocking in tests and alternative sync strategies.
// All methods accept a context.Context for cancellation support (e.g., Ctrl+C).
type SyncServiceInterface interface {
	// Sync synchronizes vendors based on the provided options, loading config and lock internally.
	// ctx controls cancellation of git operations during sync.
	Sync(ctx context.Context, opts SyncOptions) error

	// SyncVendor syncs a single vendor's refs and returns per-ref metadata and copy stats.
	// ctx controls cancellation of git operations during sync.
	//
	// lockedRefs controls the sync mode:
	//   - nil: update mode — fetches the latest commit on each ref (FETCH_HEAD).
	//   - non-nil map: sync mode — checks out the exact commit hash for each ref.
	//     Missing or empty entries within the map are treated as unlocked for that ref.
	SyncVendor(ctx context.Context, v *types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]RefMetadata, CopyStats, error)
}

// Compile-time interface satisfaction check.
var _ SyncServiceInterface = (*SyncService)(nil)

// SyncService handles vendor synchronization operations
type SyncService struct {
	configStore  ConfigStore
	lockStore    LockStore
	gitClient    GitClient
	fs           FileSystem
	fileCopy     FileCopyServiceInterface
	license      LicenseServiceInterface
	cache        CacheStore
	hooks        HookExecutor
	ui           UICallback
	rootDir      string
	internalSync InternalSyncServiceInterface // Spec 070
}

// NewSyncService creates a new SyncService.
// internalSync may be nil if no internal vendor support is needed (will be set by VendorSyncer).
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
	internalSync InternalSyncServiceInterface,
) *SyncService {
	return &SyncService{
		configStore:  configStore,
		lockStore:    lockStore,
		gitClient:    gitClient,
		fs:           fs,
		fileCopy:     fileCopy,
		license:      license,
		cache:        cache,
		hooks:        hooks,
		ui:           ui,
		rootDir:      rootDir,
		internalSync: internalSync,
	}
}

// Sync performs synchronization based on the provided options.
// ctx controls cancellation of git operations during sync.
// Note: Caller should check for lockfile existence before calling this.
func (s *SyncService) Sync(ctx context.Context, opts SyncOptions) error {
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
		return s.syncParallel(ctx, vendorsToSync, lockMap, opts)
	}

	return s.syncSequential(ctx, vendorsToSync, lockMap, opts)
}

// syncDryRun performs dry-run preview (always sequential)
func (s *SyncService) syncDryRun(vendors []types.VendorSpec, lockMap map[string]map[string]string) error {
	progress := s.ui.StartProgress(len(vendors), "Previewing sync")
	defer progress.Complete()

	for _, v := range vendors {
		if v.Source == SourceInternal {
			// Internal vendor dry-run: use InternalSyncService with DryRun option
			if s.internalSync != nil {
				//nolint:errcheck // Dry-run errors are non-fatal for preview
				_, _, _ = s.internalSync.SyncInternalVendor(&v, SyncOptions{DryRun: true})
			}
			progress.Increment(v.Name)
			continue
		}
		s.previewSyncVendor(&v, lockMap[v.Name])
		progress.Increment(v.Name)
	}

	return nil
}

// syncSequential performs sequential sync (original implementation).
// ctx controls cancellation — checked at each vendor boundary.
// Internal vendors sync first (no network), then external vendors.
func (s *SyncService) syncSequential(ctx context.Context, vendors []types.VendorSpec, lockMap map[string]map[string]string, opts SyncOptions) error {
	// Start progress tracking
	progress := s.ui.StartProgress(len(vendors), "Syncing vendors")
	defer progress.Complete()

	// Track total stats across all vendors
	var totalStats CopyStats

	// Phase 1: Internal vendors (no network, fast)
	for _, v := range vendors {
		if v.Source != SourceInternal {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.internalSync == nil {
			return fmt.Errorf("internal sync service not configured for vendor %s", v.Name)
		}
		_, stats, err := s.internalSync.SyncInternalVendor(&v, opts)
		if err != nil {
			progress.Fail(err)
			return fmt.Errorf("sync internal vendor %s: %w", v.Name, err)
		}
		totalStats.Add(stats)
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// Phase 2: External vendors (git clone, slower)
	for _, v := range vendors {
		if v.Source == SourceInternal {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// If force is true, pass nil to ignore lock and re-download
		refs := lockMap[v.Name]
		if opts.Force {
			refs = nil
		}
		_, stats, err := s.SyncVendor(ctx, &v, refs, opts)
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

// syncParallel performs parallel sync using worker pool.
// ctx controls cancellation — passed to the parallel executor and each worker.
// Internal vendors always sync sequentially first (no parallel — may share dest files).
func (s *SyncService) syncParallel(ctx context.Context, vendors []types.VendorSpec, lockMap map[string]map[string]string, opts SyncOptions) error {
	// Start progress tracking
	progress := s.ui.StartProgress(len(vendors), "Syncing vendors (parallel)")
	defer progress.Complete()

	var totalStats CopyStats

	// Phase 1: Internal vendors — sequential (no parallel for internal, may share dest files)
	for _, v := range vendors {
		if v.Source != SourceInternal {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.internalSync == nil {
			return fmt.Errorf("internal sync service not configured for vendor %s", v.Name)
		}
		_, stats, err := s.internalSync.SyncInternalVendor(&v, opts)
		if err != nil {
			progress.Fail(err)
			return fmt.Errorf("sync internal vendor %s: %w", v.Name, err)
		}
		totalStats.Add(stats)
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// Phase 2: External vendors — parallel
	var externalVendors []types.VendorSpec
	for _, v := range vendors {
		if v.Source != SourceInternal {
			externalVendors = append(externalVendors, v)
		}
	}

	if len(externalVendors) > 0 {
		// Create parallel executor
		executor := NewParallelExecutor(opts.Parallel, s.ui)

		// Define sync function for a single vendor
		syncFunc := func(workerCtx context.Context, v types.VendorSpec, lockedRefs map[string]string, syncOpts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
			updatedRefs, stats, err := s.SyncVendor(workerCtx, &v, lockedRefs, syncOpts)
			if err != nil {
				progress.Fail(err)
				return nil, CopyStats{}, err
			}
			progress.Increment(fmt.Sprintf("✓ %s", v.Name))
			return updatedRefs, stats, nil
		}

		// Execute parallel sync
		results, err := executor.ExecuteParallelSync(ctx, externalVendors, lockMap, opts, syncFunc)
		if err != nil {
			return fmt.Errorf("parallel sync: %w", err)
		}

		// Calculate total stats from parallel results
		for i := range results {
			totalStats.Add(results[i].Stats)
		}
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
	// If InternalOnly, skip external vendors
	if opts.InternalOnly && v.Source != SourceInternal {
		return false
	}

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
// ctx controls cancellation of git operations during sync.
// Returns a map of ref to RefMetadata and total stats for all synced refs.
func (s *SyncService) SyncVendor(ctx context.Context, v *types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
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

	// Resolve clone URL: detect local paths and apply --local gating
	cloneURL := v.URL
	if IsLocalPath(v.URL) {
		if !opts.Local {
			return nil, CopyStats{}, fmt.Errorf("vendor %s uses a local path (%s); pass --local to allow local filesystem access", v.Name, v.URL)
		}
		resolved, err := ResolveLocalURL(v.URL, s.rootDir)
		if err != nil {
			return nil, CopyStats{}, fmt.Errorf("resolve local URL for %s: %w", v.Name, err)
		}
		cloneURL = resolved
	}

	// Execute pre-sync hook before cloning
	if v.Hooks != nil && v.Hooks.PreSync != "" {
		hookCtx := types.HookContext{
			VendorName: v.Name,
			VendorURL:  cloneURL,
			RootDir:    s.rootDir,
		}
		if err := s.hooks.ExecutePreSync(v, &hookCtx); err != nil {
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

	// Initialize git repo
	if err := s.gitClient.Init(ctx, tempDir); err != nil {
		return nil, CopyStats{}, fmt.Errorf("failed to initialize git repository for %s: %w", v.Name, err)
	}
	if err := s.gitClient.AddRemote(ctx, tempDir, "origin", cloneURL); err != nil {
		// SEC-013: Sanitize URL in error output to prevent credential leakage
		return nil, CopyStats{}, fmt.Errorf("failed to add remote for %s (%s): %w\n\nPlease verify the repository URL is correct and accessible", v.Name, SanitizeURL(cloneURL), err)
	}

	results := make(map[string]RefMetadata)
	var totalStats CopyStats

	// Sync each ref
	for _, spec := range v.Specs {
		metadata, stats, err := s.syncRef(ctx, tempDir, v, spec, lockedRefs, opts)
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

		hookCtx := types.HookContext{
			VendorName:  v.Name,
			VendorURL:   cloneURL,
			Ref:         firstRef,
			CommitHash:  firstHash,
			RootDir:     s.rootDir,
			FilesCopied: totalStats.FileCount,
		}
		if err := s.hooks.ExecutePostSync(v, &hookCtx); err != nil {
			return nil, CopyStats{}, fmt.Errorf("post-sync hook failed: %w", err)
		}
	}

	return results, totalStats, nil
}

// syncRef syncs a single ref for a vendor.
// ctx controls cancellation of git operations during sync.
func (s *SyncService) syncRef(ctx context.Context, tempDir string, v *types.VendorSpec, spec types.BranchSpec, lockedRefs map[string]string, opts SyncOptions) (RefMetadata, CopyStats, error) {
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

	if isLocked {
		// Locked sync - checkout specific commit
		if err := s.fetchWithFallback(ctx, tempDir, spec.Ref); err != nil {
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
		if err := s.fetchWithFallback(ctx, tempDir, spec.Ref); err != nil {
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

// fetchWithFallback tries shallow fetch first, falls back to full fetch if needed.
// ctx controls cancellation of git fetch operations.
func (s *SyncService) fetchWithFallback(ctx context.Context, tempDir, ref string) error {
	// Try shallow fetch first
	if err := s.gitClient.Fetch(ctx, tempDir, "origin", 1, ref); err != nil {
		// Shallow fetch failed, try full fetch
		if err := s.gitClient.FetchAll(ctx, tempDir, "origin"); err != nil {
			return fmt.Errorf("fetch ref %s: %w", ref, err)
		}
	}
	return nil
}

// canSkipSync checks if a vendor@ref can skip sync based on cache.
// Returns false (forcing a re-sync) on any cache error, missing files, or checksum mismatch.
func (s *SyncService) canSkipSync(vendorName, ref, commitHash string, mappings []types.PathMapping) bool {
	// Load cache for this vendor@ref
	cache, err := s.cache.Load(vendorName, ref)
	if err != nil {
		// Log corrupted cache so the user knows why cache was skipped
		fmt.Printf("  ⚠ Warning: cache error for %s@%s: %v\n", vendorName, ref, err)
		return false
	}
	if cache.CommitHash == "" {
		// Cache miss - can't skip
		return false
	}

	// Check if commit hash matches
	if cache.CommitHash != commitHash {
		// Commit hash changed - cache is stale
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

		// Check if file exists.
		// Uses errors.Is instead of os.IsNotExist to correctly handle wrapped errors
		// (see Legacy Trap in CLAUDE.md: "os.IsNotExist for wrapped errors").
		fullPath := filepath.Join(s.rootDir, destFile)
		if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
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
