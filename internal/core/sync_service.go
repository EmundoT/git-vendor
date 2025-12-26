package core

import (
	"fmt"
	"strings"

	"git-vendor/internal/types"
)

// SyncOptions configures sync operation behavior
type SyncOptions struct {
	DryRun     bool
	VendorName string // Empty = all vendors
	Force      bool
}

// SyncService handles vendor synchronization operations
type SyncService struct {
	configStore ConfigStore
	lockStore   LockStore
	gitClient   GitClient
	fs          FileSystem
	fileCopy    *FileCopyService
	license     *LicenseService
	ui          UICallback
	rootDir     string
}

// NewSyncService creates a new SyncService
func NewSyncService(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	fileCopy *FileCopyService,
	license *LicenseService,
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
		ui:          ui,
		rootDir:     rootDir,
	}
}

// Sync performs synchronization based on the provided options
// Note: Caller should check for lockfile existence before calling this
func (s *SyncService) Sync(opts SyncOptions) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return err
	}

	// Build lock map for quick lookups
	lockMap := s.buildLockMap(lock)

	// Validate vendor exists if filtering by name
	if opts.VendorName != "" {
		if err := s.validateVendorExists(config, opts.VendorName); err != nil {
			return err
		}
	}

	// Print header
	if opts.DryRun {
		fmt.Println(s.ui.StyleTitle("Sync Plan:"))
		fmt.Println()
	} else {
		s.printSyncHeader(config, opts.VendorName)
	}

	// Track total stats across all vendors
	var totalStats CopyStats

	// Sync vendors
	for _, v := range config.Vendors {
		// Skip vendors that don't match the filter
		if opts.VendorName != "" && v.Name != opts.VendorName {
			continue
		}

		if opts.DryRun {
			s.previewSyncVendor(v, lockMap[v.Name])
		} else {
			// If force is true, pass nil to ignore lock and re-download
			refs := lockMap[v.Name]
			if opts.Force {
				refs = nil
			}
			_, stats, err := s.syncVendor(v, refs)
			if err != nil {
				return err
			}
			totalStats.Add(stats)
		}
	}

	// Display summary (only in non-dry-run mode)
	if !opts.DryRun && totalStats.FileCount > 0 {
		fmt.Println()
		fmt.Printf("Summary: Synced %s across all vendors\n", Pluralize(totalStats.FileCount, "file", "files"))
	}

	return nil
}

// buildLockMap builds a map of vendor name to ref to commit hash
func (s *SyncService) buildLockMap(lock types.VendorLock) map[string]map[string]string {
	lockMap := make(map[string]map[string]string)
	for _, l := range lock.Vendors {
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
		return fmt.Errorf(ErrVendorNotFound, vendorName)
	}
	return nil
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
func (s *SyncService) previewSyncVendor(v types.VendorSpec, lockedRefs map[string]string) {
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

// syncVendor syncs a single vendor
// Returns a map of ref to commit hash and total stats for all synced refs
func (s *SyncService) syncVendor(v types.VendorSpec, lockedRefs map[string]string) (map[string]string, CopyStats, error) {
	fmt.Printf("⠿ %s (cloning repository...)\n", v.Name)

	// Create temp directory for cloning
	tempDir, err := s.fs.CreateTemp("", "git-vendor-*")
	if err != nil {
		return nil, CopyStats{}, err
	}
	defer func() { _ = s.fs.RemoveAll(tempDir) }()

	// Initialize git repo
	if err := s.gitClient.Init(tempDir); err != nil {
		return nil, CopyStats{}, fmt.Errorf("failed to initialize git repository for %s: %w", v.Name, err)
	}
	if err := s.gitClient.AddRemote(tempDir, "origin", v.URL); err != nil {
		return nil, CopyStats{}, fmt.Errorf("failed to add remote for %s (%s): %w\n\nPlease verify the repository URL is correct and accessible", v.Name, v.URL, err)
	}

	results := make(map[string]string)
	var totalStats CopyStats

	// Sync each ref
	for _, spec := range v.Specs {
		hash, stats, err := s.syncRef(tempDir, v, spec, lockedRefs)
		if err != nil {
			return nil, CopyStats{}, err
		}
		results[spec.Ref] = hash
		totalStats.Add(stats)

		// Display stats with proper pluralization
		fmt.Printf("  ✓ %s @ %s (synced %s: %s)\n",
			v.Name, spec.Ref,
			Pluralize(len(spec.Mapping), "path", "paths"),
			Pluralize(stats.FileCount, "file", "files"))
	}

	return results, totalStats, nil
}

// syncRef syncs a single ref for a vendor
func (s *SyncService) syncRef(tempDir string, v types.VendorSpec, spec types.BranchSpec, lockedRefs map[string]string) (string, CopyStats, error) {
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
		if err := s.fetchWithFallback(tempDir, spec.Ref); err != nil {
			return "", CopyStats{}, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
		}
		if err := s.gitClient.Checkout(tempDir, targetCommit); err != nil {
			// Detect stale lock hash error and provide helpful message
			errMsg := err.Error()
			if strings.Contains(errMsg, "reference is not a tree") || strings.Contains(errMsg, "not a valid object") {
				return "", CopyStats{}, fmt.Errorf(ErrStaleCommitMsg, targetCommit[:7])
			}
			return "", CopyStats{}, fmt.Errorf(ErrCheckoutFailed, targetCommit, err)
		}
	} else {
		// Unlocked sync - checkout latest
		if err := s.fetchWithFallback(tempDir, spec.Ref); err != nil {
			return "", CopyStats{}, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
		}
		if err := s.gitClient.Checkout(tempDir, "FETCH_HEAD"); err != nil {
			if err := s.gitClient.Checkout(tempDir, spec.Ref); err != nil {
				return "", CopyStats{}, fmt.Errorf(ErrRefCheckoutFailed, spec.Ref, err)
			}
		}
	}

	// Get current commit hash
	hash, err := s.gitClient.GetHeadHash(tempDir)
	if err != nil {
		return "", CopyStats{}, fmt.Errorf("failed to get commit hash for %s @ %s: %w", v.Name, spec.Ref, err)
	}

	// Copy license file (don't count in stats)
	if err := s.license.CopyLicense(tempDir, v.Name); err != nil {
		return "", CopyStats{}, err
	}

	// Copy files according to mappings and collect stats
	fmt.Printf("  ⠿ Copying files...\n")
	stats, err := s.fileCopy.CopyMappings(tempDir, v, spec)
	if err != nil {
		return "", CopyStats{}, err
	}

	return hash, stats, nil
}

// fetchWithFallback tries shallow fetch first, falls back to full fetch if needed
// This eliminates duplicate fetch retry logic
func (s *SyncService) fetchWithFallback(tempDir, ref string) error {
	// Try shallow fetch first
	if err := s.gitClient.Fetch(tempDir, 1, ref); err != nil {
		// Shallow fetch failed, try full fetch
		if err := s.gitClient.FetchAll(tempDir); err != nil {
			return err
		}
	}
	return nil
}
