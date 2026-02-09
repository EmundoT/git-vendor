package core

import (
	"fmt"
	"path/filepath"

	"github.com/EmundoT/git-vendor/internal/types"
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
func (s *SilentUICallback) StartProgress(_ int, _ string) types.ProgressTracker {
	return &NoOpProgressTracker{}
}

// NoOpProgressTracker is a no-op implementation for testing
type NoOpProgressTracker struct{}

// Increment does nothing (no-op implementation).
func (t *NoOpProgressTracker) Increment(_ string) {}

// SetTotal does nothing (no-op implementation).
func (t *NoOpProgressTracker) SetTotal(_ int) {}

// Complete does nothing (no-op implementation).
func (t *NoOpProgressTracker) Complete() {}

// Fail does nothing (no-op implementation).
func (t *NoOpProgressTracker) Fail(_ error) {}

// VendorSyncer orchestrates vendor operations using domain services
type VendorSyncer struct {
	// Domain services (injectable via ServiceOverrides)
	repository    VendorRepositoryInterface
	sync          SyncServiceInterface
	update        UpdateServiceInterface
	license       LicenseServiceInterface
	validation    ValidationServiceInterface
	explorer      RemoteExplorerInterface
	updateChecker UpdateCheckerInterface
	verifyService VerifyServiceInterface
	vulnScanner   VulnScannerInterface

	// Infrastructure dependencies
	configStore    ConfigStore
	lockStore      LockStore
	gitClient      GitClient
	licenseChecker LicenseChecker
	fs             FileSystem
	rootDir        string
	ui             UICallback
}

// ServiceOverrides allows injecting custom service implementations into VendorSyncer.
// All fields are optional — nil values cause the default implementation to be created.
// This enables targeted mocking in tests without affecting other services.
type ServiceOverrides struct {
	Repository    VendorRepositoryInterface
	Sync          SyncServiceInterface
	Update        UpdateServiceInterface
	License       LicenseServiceInterface
	Validation    ValidationServiceInterface
	Explorer      RemoteExplorerInterface
	UpdateChecker UpdateCheckerInterface
	VerifyService VerifyServiceInterface
	VulnScanner   VulnScannerInterface
}

// NewVendorSyncer creates a new VendorSyncer with injected dependencies.
// overrides is optional — pass nil to use all default service implementations.
// Individual service fields within overrides that are nil also use defaults.
func NewVendorSyncer(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	licenseChecker LicenseChecker,
	rootDir string,
	ui UICallback,
	overrides *ServiceOverrides,
) *VendorSyncer {
	if ui == nil {
		ui = &SilentUICallback{}
	}
	if overrides == nil {
		overrides = &ServiceOverrides{}
	}

	// Build all default concrete services first (preserving internal wiring)
	repository := NewVendorRepository(configStore)
	fileCopy := NewFileCopyService(fs)
	license := NewLicenseService(licenseChecker, fs, rootDir, ui)
	cache := NewFileCacheStore(fs, rootDir)
	hooks := NewHookService(ui)
	syncSvc := NewSyncService(configStore, lockStore, gitClient, fs, fileCopy, license, cache, hooks, ui, rootDir)
	updateSvc := NewUpdateService(configStore, lockStore, syncSvc, cache, ui, rootDir)
	validation := NewValidationService(configStore)
	explorer := NewRemoteExplorer(gitClient, fs)
	updateChecker := NewUpdateChecker(configStore, lockStore, gitClient, fs, ui)
	verifyService := NewVerifyService(configStore, lockStore, cache, fs, rootDir)
	vulnScanner := VulnScannerInterface(NewVulnScanner(lockStore, configStore))

	// Apply overrides where provided
	syncer := &VendorSyncer{
		repository:     repository,
		sync:           syncSvc,
		update:         updateSvc,
		license:        license,
		validation:     validation,
		explorer:       explorer,
		updateChecker:  updateChecker,
		verifyService:  verifyService,
		vulnScanner:    vulnScanner,
		configStore:    configStore,
		lockStore:      lockStore,
		gitClient:      gitClient,
		licenseChecker: licenseChecker,
		fs:             fs,
		rootDir:        rootDir,
		ui:             ui,
	}

	if overrides.Repository != nil {
		syncer.repository = overrides.Repository
	}
	if overrides.Sync != nil {
		syncer.sync = overrides.Sync
	}
	if overrides.Update != nil {
		syncer.update = overrides.Update
	}
	if overrides.License != nil {
		syncer.license = overrides.License
	}
	if overrides.Validation != nil {
		syncer.validation = overrides.Validation
	}
	if overrides.Explorer != nil {
		syncer.explorer = overrides.Explorer
	}
	if overrides.UpdateChecker != nil {
		syncer.updateChecker = overrides.UpdateChecker
	}
	if overrides.VerifyService != nil {
		syncer.verifyService = overrides.VerifyService
	}
	if overrides.VulnScanner != nil {
		syncer.vulnScanner = overrides.VulnScanner
	}

	return syncer
}

// Init initializes vendor directory structure
func (s *VendorSyncer) Init() error {
	if err := s.fs.MkdirAll(s.rootDir, 0755); err != nil {
		return fmt.Errorf("create vendor directory: %w", err)
	}
	if err := s.fs.MkdirAll(filepath.Join(s.rootDir, LicensesDir), 0755); err != nil {
		return fmt.Errorf("create licenses directory: %w", err)
	}
	// Save empty config with no vendors instead of empty vendor
	if err := s.configStore.Save(types.VendorConfig{Vendors: []types.VendorSpec{}}); err != nil {
		return fmt.Errorf("save initial config: %w", err)
	}
	return nil
}

// AddVendor adds a new vendor with license compliance check
func (s *VendorSyncer) AddVendor(spec *types.VendorSpec) error {
	// Check if vendor already exists
	exists, err := s.repository.Exists(spec.Name)
	if err != nil {
		exists = false
	}

	// If new vendor, check license compliance
	if !exists {
		detectedLicense, err := s.license.CheckCompliance(spec.URL)
		if err != nil {
			return fmt.Errorf("check license compliance for %s: %w", spec.Name, err)
		}
		spec.License = detectedLicense
	}

	return s.SaveVendor(spec)
}

// SaveVendor saves or updates a vendor spec
func (s *VendorSyncer) SaveVendor(spec *types.VendorSpec) error {
	if err := s.repository.Save(spec); err != nil {
		return fmt.Errorf("save vendor %s: %w", spec.Name, err)
	}
	return s.update.UpdateAll()
}

// RemoveVendor removes a vendor by name
func (s *VendorSyncer) RemoveVendor(name string) error {
	// Delete vendor from config
	if err := s.repository.Delete(name); err != nil {
		return err // Already a structured VendorNotFoundError
	}

	// Remove license file
	licensePath := filepath.Join(s.rootDir, LicensesDir, name+".txt")
	_ = s.fs.Remove(licensePath) //nolint:errcheck // cleanup operation, error not critical

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
			return fmt.Errorf("generate lockfile: %w", err)
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
			return fmt.Errorf("generate lockfile: %w", err)
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
			return fmt.Errorf("generate lockfile: %w", err)
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

// SyncWithParallel performs sync with parallel processing
func (s *VendorSyncer) SyncWithParallel(vendorName string, force, noCache bool, parallelOpts types.ParallelOptions) error {
	// Check if lockfile exists, if not, run UpdateAll
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Generating lockfile from latest commits...")
		if err := s.update.UpdateAll(); err != nil {
			return fmt.Errorf("generate lockfile: %w", err)
		}
		fmt.Println()
		fmt.Println("Lockfile created. Now syncing files...")
	}
	return s.sync.Sync(SyncOptions{
		VendorName: vendorName,
		Force:      force,
		NoCache:    noCache,
		Parallel:   parallelOpts,
	})
}

// UpdateAll updates all vendors and regenerates lockfile
func (s *VendorSyncer) UpdateAll() error {
	return s.update.UpdateAll()
}

// UpdateAllWithParallel updates all vendors with parallel processing
func (s *VendorSyncer) UpdateAllWithParallel(parallelOpts types.ParallelOptions) error {
	return s.update.UpdateAllWithOptions(parallelOpts)
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

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
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
				// Strip position specifier from source before auto-path computation
				srcFile, _, parseErr := types.ParsePathPosition(srcClean)
				if parseErr != nil {
					srcFile = srcClean
				}
				srcFile = filepath.Clean(srcFile)
				destPath = ComputeAutoPath(srcFile, matchingSpec.DefaultTarget, vendorConfig.Name)
			}

			// Strip position specifier from destination path for file system access
			destFile, _, parseErr := types.ParsePathPosition(destPath)
			if parseErr != nil {
				destFile = destPath
			}

			// Check if path exists (don't join with rootDir since destFile is relative to CWD)
			_, err := s.fs.Stat(destFile)
			if err != nil {
				// Path doesn't exist or error accessing it
				missingPaths = append(missingPaths, destFile)
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

// CheckUpdates checks for available updates for all vendors
func (s *VendorSyncer) CheckUpdates() ([]types.UpdateCheckResult, error) {
	return s.updateChecker.CheckUpdates()
}

// Verify checks all vendored files against the lockfile
func (s *VendorSyncer) Verify() (*types.VerifyResult, error) {
	return s.verifyService.Verify()
}

// Scan performs vulnerability scanning against OSV.dev
func (s *VendorSyncer) Scan(failOn string) (*types.ScanResult, error) {
	return s.vulnScanner.Scan(failOn)
}

// MigrateLockfile updates an existing lockfile to add missing metadata fields.
// For fields that can't be computed (VendoredAt, VendoredBy), it uses best guesses.
// Returns the number of entries migrated and any error.
func (s *VendorSyncer) MigrateLockfile() (int, error) {
	lock, err := s.lockStore.Load()
	if err != nil {
		return 0, fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Load config to get license info
	config, err := s.configStore.Load()
	if err != nil {
		return 0, fmt.Errorf("failed to load config: %w", err)
	}

	// Build vendor config map for license lookup
	vendorLicenses := make(map[string]string)
	for _, v := range config.Vendors {
		vendorLicenses[v.Name] = v.License
	}

	migrated := 0
	for i := range lock.Vendors {
		entry := &lock.Vendors[i]

		// Check if entry needs migration (missing VendoredAt indicates old format)
		if entry.VendoredAt != "" {
			continue
		}

		migrated++

		// Use Updated timestamp as VendoredAt (best guess)
		if entry.Updated != "" {
			entry.VendoredAt = entry.Updated
		}

		// Set VendoredBy to "unknown" since we can't determine original user
		entry.VendoredBy = "unknown (migrated)"

		// Use Updated as LastSyncedAt
		if entry.Updated != "" {
			entry.LastSyncedAt = entry.Updated
		}

		// Get LicenseSPDX from config
		if license, ok := vendorLicenses[entry.Name]; ok && entry.LicenseSPDX == "" {
			entry.LicenseSPDX = license
		}

		// Note: SourceVersionTag cannot be determined without network access
		// It will be populated on next update
	}

	if migrated > 0 {
		if err := s.lockStore.Save(lock); err != nil {
			return 0, fmt.Errorf("failed to save migrated lockfile: %w", err)
		}
	}

	return migrated, nil
}
