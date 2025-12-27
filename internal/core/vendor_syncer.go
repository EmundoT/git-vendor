package core

import (
	"fmt"
	"path/filepath"

	"git-vendor/internal/types"
)

// UICallback handles user interaction during vendor operations
type UICallback interface {
	ShowError(title, message string)
	ShowSuccess(message string)
	ShowWarning(title, message string)
	AskConfirmation(title, message string) bool
	ShowLicenseCompliance(license string)
	StyleTitle(title string) string

	// New methods for non-interactive mode
	GetOutputMode() OutputMode
	IsAutoApprove() bool
	FormatJSON(output JSONOutput) error

	// Progress tracking
	StartProgress(total int, label string) types.ProgressTracker
}

// SilentUICallback is a no-op implementation (for testing/CI)
type SilentUICallback struct{}

// ShowError implements UICallback for silent operation (no output).
func (s *SilentUICallback) ShowError(_, _ string) {}

// ShowSuccess implements UICallback for silent operation (no output).
func (s *SilentUICallback) ShowSuccess(_ string) {}

// ShowWarning implements UICallback for silent operation (no output).
func (s *SilentUICallback) ShowWarning(_, _ string) {}

// AskConfirmation implements UICallback for silent operation (always returns false).
func (s *SilentUICallback) AskConfirmation(_, _ string) bool { return false }

// ShowLicenseCompliance implements UICallback for silent operation (no output).
func (s *SilentUICallback) ShowLicenseCompliance(_ string) {}

// StyleTitle implements UICallback for silent operation (returns plain text).
func (s *SilentUICallback) StyleTitle(title string) string { return title }

// GetOutputMode implements UICallback for silent operation (returns OutputNormal).
func (s *SilentUICallback) GetOutputMode() OutputMode { return OutputNormal }

// IsAutoApprove implements UICallback for silent operation (returns false).
func (s *SilentUICallback) IsAutoApprove() bool { return false }

// FormatJSON implements UICallback for silent operation (does nothing).
func (s *SilentUICallback) FormatJSON(_ JSONOutput) error { return nil }

// StartProgress implements UICallback for silent operation (no-op).
func (s *SilentUICallback) StartProgress(total int, label string) types.ProgressTracker {
	return &NoOpProgressTracker{}
}

// NoOpProgressTracker is a no-op implementation for testing
type NoOpProgressTracker struct{}

func (t *NoOpProgressTracker) Increment(message string) {}
func (t *NoOpProgressTracker) SetTotal(total int)       {}
func (t *NoOpProgressTracker) Complete()                {}
func (t *NoOpProgressTracker) Fail(err error)           {}

// VendorSyncer orchestrates vendor operations using domain services
type VendorSyncer struct {
	repository     *VendorRepository
	sync           *SyncService
	update         *UpdateService
	license        *LicenseService
	validation     *ValidationService
	explorer       *RemoteExplorer
	updateChecker  *UpdateChecker
	configStore    ConfigStore
	lockStore      LockStore
	gitClient      GitClient
	licenseChecker LicenseChecker
	fs             FileSystem
	rootDir        string
	ui             UICallback
}

// NewVendorSyncer creates a new VendorSyncer with injected dependencies
func NewVendorSyncer(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	licenseChecker LicenseChecker,
	rootDir string,
	ui UICallback,
) *VendorSyncer {
	if ui == nil {
		ui = &SilentUICallback{}
	}

	// Create domain services
	repository := NewVendorRepository(configStore)
	fileCopy := NewFileCopyService(fs)
	license := NewLicenseService(licenseChecker, fs, rootDir, ui)
	cache := NewFileCacheStore(fs, rootDir)
	sync := NewSyncService(configStore, lockStore, gitClient, fs, fileCopy, license, cache, ui, rootDir)
	update := NewUpdateService(configStore, lockStore, sync, ui, rootDir)
	validation := NewValidationService(configStore)
	explorer := NewRemoteExplorer(gitClient, fs)
	updateChecker := NewUpdateChecker(configStore, lockStore, gitClient, fs, ui)

	return &VendorSyncer{
		repository:     repository,
		sync:           sync,
		update:         update,
		license:        license,
		validation:     validation,
		explorer:       explorer,
		updateChecker:  updateChecker,
		configStore:    configStore,
		lockStore:      lockStore,
		gitClient:      gitClient,
		licenseChecker: licenseChecker,
		fs:             fs,
		rootDir:        rootDir,
		ui:             ui,
	}
}

// Init initializes vendor directory structure
func (s *VendorSyncer) Init() error {
	if err := s.fs.MkdirAll(s.rootDir, 0755); err != nil {
		return err
	}
	if err := s.fs.MkdirAll(filepath.Join(s.rootDir, LicenseDir), 0755); err != nil {
		return err
	}
	// Save empty config with no vendors instead of empty vendor
	return s.configStore.Save(types.VendorConfig{Vendors: []types.VendorSpec{}})
}

// AddVendor adds a new vendor with license compliance check
func (s *VendorSyncer) AddVendor(spec types.VendorSpec) error {
	// Check if vendor already exists
	exists, err := s.repository.Exists(spec.Name)
	if err != nil {
		exists = false
	}

	// If new vendor, check license compliance
	if !exists {
		detectedLicense, err := s.license.CheckCompliance(spec.URL)
		if err != nil {
			return err
		}
		spec.License = detectedLicense
	}

	return s.SaveVendor(spec)
}

// SaveVendor saves or updates a vendor spec
func (s *VendorSyncer) SaveVendor(spec types.VendorSpec) error {
	if err := s.repository.Save(spec); err != nil {
		return err
	}
	return s.update.UpdateAll()
}

// RemoveVendor removes a vendor by name
func (s *VendorSyncer) RemoveVendor(name string) error {
	// Delete vendor from config
	if err := s.repository.Delete(name); err != nil {
		return err
	}

	// Remove license file
	licensePath := filepath.Join(s.rootDir, LicenseDir, name+".txt")
	_ = s.fs.Remove(licensePath)

	// Update lockfile
	return s.update.UpdateAll()
}

// Sync performs locked synchronization
func (s *VendorSyncer) Sync() error {
	// Check if lockfile exists, if not, run UpdateAll
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Generating lockfile from latest commits...")
		if err := s.update.UpdateAll(); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Lockfile created. Now syncing files...")
		return s.sync.Sync(SyncOptions{})
	}
	return s.sync.Sync(SyncOptions{})
}

// SyncDryRun performs a dry-run sync
func (s *VendorSyncer) SyncDryRun() error {
	// Check if lockfile exists for dry-run
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Would generate lockfile from latest commits, then sync files.")
		return nil
	}
	return s.sync.Sync(SyncOptions{DryRun: true})
}

// SyncWithOptions performs sync with vendor filter, force, and cache options
func (s *VendorSyncer) SyncWithOptions(vendorName string, force, noCache bool) error {
	// Check if lockfile exists, if not, run UpdateAll
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Generating lockfile from latest commits...")
		if err := s.update.UpdateAll(); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Lockfile created. Now syncing files...")
	}
	return s.sync.Sync(SyncOptions{
		VendorName: vendorName,
		Force:      force,
		NoCache:    noCache,
	})
}

// SyncWithGroup performs sync for all vendors in a group
func (s *VendorSyncer) SyncWithGroup(groupName string, force, noCache bool) error {
	// Check if lockfile exists, if not, run UpdateAll
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Generating lockfile from latest commits...")
		if err := s.update.UpdateAll(); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Lockfile created. Now syncing files...")
	}
	return s.sync.Sync(SyncOptions{
		GroupName: groupName,
		Force:     force,
		NoCache:   noCache,
	})
}

// UpdateAll updates all vendors and regenerates lockfile
func (s *VendorSyncer) UpdateAll() error {
	return s.update.UpdateAll()
}

// GetConfig returns the vendor configuration
func (s *VendorSyncer) GetConfig() (types.VendorConfig, error) {
	return s.repository.GetConfig()
}

// GetLockHash retrieves the locked commit hash for a vendor@ref
func (s *VendorSyncer) GetLockHash(vendorName, ref string) string {
	return s.lockStore.GetHash(vendorName, ref)
}

// Audit checks lockfile status
func (s *VendorSyncer) Audit() {
	lock, err := s.lockStore.Load()
	if err != nil {
		s.ui.ShowWarning("Audit Failed", "No lockfile.")
		return
	}
	s.ui.ShowSuccess(fmt.Sprintf("Audit Passed. %d vendors locked.", len(lock.Vendors)))
}

// ValidateConfig performs comprehensive config validation
func (s *VendorSyncer) ValidateConfig() error {
	return s.validation.ValidateConfig()
}

// DetectConflicts checks for path conflicts between vendors
func (s *VendorSyncer) DetectConflicts() ([]types.PathConflict, error) {
	return s.validation.DetectConflicts()
}

// FetchRepoDir fetches directory listing from remote repository
func (s *VendorSyncer) FetchRepoDir(url, ref, subdir string) ([]string, error) {
	return s.explorer.FetchRepoDir(url, ref, subdir)
}

// ListLocalDir lists local directory contents
func (s *VendorSyncer) ListLocalDir(path string) ([]string, error) {
	return s.explorer.ListLocalDir(path)
}

// ParseSmartURL delegates to the remote explorer
func (s *VendorSyncer) ParseSmartURL(rawURL string) (string, string, string) {
	return s.explorer.ParseSmartURL(rawURL)
}

// CheckGitHubLicense delegates to the license service
func (s *VendorSyncer) CheckGitHubLicense(url string) (string, error) {
	return s.license.CheckLicense(url)
}

// CheckSyncStatus checks if local files are in sync with the lockfile
func (s *VendorSyncer) CheckSyncStatus() (types.SyncStatus, error) {
	// Load config and lockfile
	config, err := s.configStore.Load()
	if err != nil {
		return types.SyncStatus{}, err
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return types.SyncStatus{}, err
	}

	// Build a map of vendor configs for quick lookup
	configMap := make(map[string]types.VendorSpec)
	for _, v := range config.Vendors {
		configMap[v.Name] = v
	}

	// Check each locked vendor
	var vendorStatuses []types.VendorStatus
	allSynced := true

	for _, lockEntry := range lock.Vendors {
		vendorConfig, exists := configMap[lockEntry.Name]
		if !exists {
			// Vendor in lockfile but not in config (shouldn't happen normally)
			continue
		}

		// Find the matching BranchSpec
		var matchingSpec *types.BranchSpec
		for _, spec := range vendorConfig.Specs {
			if spec.Ref == lockEntry.Ref {
				matchingSpec = &spec
				break
			}
		}

		if matchingSpec == nil {
			// No matching spec found (shouldn't happen)
			continue
		}

		// Check each path mapping
		var missingPaths []string
		for _, mapping := range matchingSpec.Mapping {
			// Compute destination path using the same logic as sync
			destPath := mapping.To
			if destPath == "" || destPath == "." {
				srcClean := mapping.From
				// Clean source path (remove blob/tree prefixes if any)
				srcClean = filepath.Clean(srcClean)
				destPath = ComputeAutoPath(srcClean, matchingSpec.DefaultTarget, vendorConfig.Name)
			}

			// Check if path exists (don't join with rootDir since destPath is relative to CWD)
			_, err := s.fs.Stat(destPath)
			if err != nil {
				// Path doesn't exist or error accessing it
				missingPaths = append(missingPaths, destPath)
			}
		}

		isSynced := len(missingPaths) == 0
		if !isSynced {
			allSynced = false
		}

		vendorStatuses = append(vendorStatuses, types.VendorStatus{
			Name:         lockEntry.Name,
			Ref:          lockEntry.Ref,
			IsSynced:     isSynced,
			MissingPaths: missingPaths,
		})
	}

	return types.SyncStatus{
		AllSynced:      allSynced,
		VendorStatuses: vendorStatuses,
	}, nil
}

// syncVendor is exposed for testing - delegates to sync service
func (s *VendorSyncer) syncVendor(v types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]string, CopyStats, error) {
	return s.sync.syncVendor(v, lockedRefs, opts)
}

// CheckUpdates checks for available updates for all vendors
func (s *VendorSyncer) CheckUpdates() ([]types.UpdateCheckResult, error) {
	return s.updateChecker.CheckUpdates()
}
