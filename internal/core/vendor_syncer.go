package core

import (
	"context"
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
	repository        VendorRepositoryInterface
	sync              SyncServiceInterface
	update            UpdateServiceInterface
	license           LicenseServiceInterface
	validation        ValidationServiceInterface
	explorer          RemoteExplorerInterface
	updateChecker     UpdateCheckerInterface
	verifyService     VerifyServiceInterface
	vulnScanner       VulnScannerInterface
	driftService      DriftServiceInterface
	auditService      AuditServiceInterface
	complianceService ComplianceServiceInterface // Spec 070
	outdatedSvc       OutdatedServiceInterface

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
	DriftService  DriftServiceInterface
	AuditService      AuditServiceInterface
	ComplianceService ComplianceServiceInterface
	OutdatedService   OutdatedServiceInterface
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
	internalSyncSvc := NewInternalSyncService(configStore, lockStore, fileCopy, cache, fs, rootDir)
	syncSvc := NewSyncService(configStore, lockStore, gitClient, fs, fileCopy, license, cache, hooks, ui, rootDir, internalSyncSvc)
	updateSvc := NewUpdateService(configStore, lockStore, syncSvc, internalSyncSvc, cache, ui, rootDir)
	validation := NewValidationService(configStore)
	explorer := NewRemoteExplorer(gitClient, fs)
	updateChecker := NewUpdateChecker(configStore, lockStore, gitClient, fs, ui)
	verifyService := NewVerifyService(configStore, lockStore, cache, fs, rootDir)
	vulnScanner := VulnScannerInterface(NewVulnScanner(lockStore, configStore))
	driftSvc := DriftServiceInterface(NewDriftService(configStore, lockStore, gitClient, fs, ui, rootDir))
	auditSvc := AuditServiceInterface(NewAuditService(verifyService, vulnScanner, driftSvc, configStore, lockStore))
	complianceSvc := ComplianceServiceInterface(NewComplianceService(configStore, lockStore, cache, fs, rootDir))
	outdatedSvc := OutdatedServiceInterface(NewOutdatedService(configStore, lockStore, gitClient))

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
		driftService:   driftSvc,
		auditService:      auditSvc,
		complianceService: complianceSvc,
		outdatedSvc:       outdatedSvc,
		configStore:       configStore,
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
	if overrides.DriftService != nil {
		syncer.driftService = overrides.DriftService
	}
	if overrides.AuditService != nil {
		syncer.auditService = overrides.AuditService
	}
	if overrides.ComplianceService != nil {
		syncer.complianceService = overrides.ComplianceService
	}
	if overrides.OutdatedService != nil {
		syncer.outdatedSvc = overrides.OutdatedService
	}

	return syncer
}

// Init initializes vendor directory structure and configures git hooks.
// Init creates the .git-vendor/ tree, saves an empty config, and sets
// core.hooksPath to .githooks if that directory already exists in the
// project root. Hook setup is best-effort — failures do not fail Init()
// since the core vendor directory setup already succeeded.
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

	// Set core.hooksPath if .githooks/ exists in the project root.
	if s.gitClient != nil {
		projectRoot := filepath.Dir(s.rootDir)
		if projectRoot == "" {
			projectRoot = "."
		}
		hooksDir := filepath.Join(projectRoot, ".githooks")
		if _, err := s.fs.Stat(hooksDir); err == nil {
			if err := s.gitClient.ConfigSet(context.Background(), projectRoot, "core.hooksPath", ".githooks"); err == nil {
				s.ui.ShowSuccess("Configured core.hooksPath = .githooks")
			}
		}
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

// SaveVendor saves or updates a vendor spec.
// Uses context.Background() because SaveVendor is called from interactive wizards
// where signal handling is not wired.
func (s *VendorSyncer) SaveVendor(spec *types.VendorSpec) error {
	if err := s.repository.Save(spec); err != nil {
		return fmt.Errorf("save vendor %s: %w", spec.Name, err)
	}
	return s.update.UpdateAll(context.Background())
}

// RemoveVendor removes a vendor by name.
// Uses context.Background() because RemoveVendor is called from interactive wizards
// where signal handling is not wired.
func (s *VendorSyncer) RemoveVendor(name string) error {
	// Delete vendor from config
	if err := s.repository.Delete(name); err != nil {
		return err // Already a structured VendorNotFoundError
	}

	// Remove license file
	licensePath := filepath.Join(s.rootDir, LicensesDir, name+".txt")
	_ = s.fs.Remove(licensePath) //nolint:errcheck // cleanup operation, error not critical

	// Update lockfile
	return s.update.UpdateAll(context.Background())
}

// syncWithAutoUpdate calls sync.Sync and falls back to UpdateAllWithOptions on stale lockfile errors.
// When a locked commit no longer exists in the remote (e.g., after force-push),
// syncWithAutoUpdate regenerates the lockfile via UpdateAllWithOptions, which also re-syncs files.
func (s *VendorSyncer) syncWithAutoUpdate(ctx context.Context, opts SyncOptions) error {
	err := s.sync.Sync(ctx, opts)
	if err == nil {
		return nil
	}
	if !IsStaleCommit(err) {
		return err
	}
	fmt.Println("⚠ Stale lockfile detected — auto-updating...")
	if updateErr := s.update.UpdateAllWithOptions(ctx, UpdateOptions{
		Local:      opts.Local,
		VendorName: opts.VendorName,
		Group:      opts.GroupName,
	}); updateErr != nil {
		return fmt.Errorf("auto-update after stale commit: %w", updateErr)
	}
	return nil
}

// Sync performs locked synchronization.
// ctx controls cancellation of git operations during sync.
func (s *VendorSyncer) Sync(ctx context.Context) error {
	return s.SyncWithFullOpts(ctx, SyncOptions{})
}

// SyncWithOptions performs sync with vendor filter, force, and cache options.
// ctx controls cancellation of git operations during sync.
func (s *VendorSyncer) SyncWithOptions(ctx context.Context, vendorName string, force, noCache bool) error {
	return s.SyncWithFullOpts(ctx, SyncOptions{
		VendorName: vendorName,
		Force:      force,
		NoCache:    noCache,
	})
}

// SyncWithFullOpts performs sync with a full SyncOptions struct.
// Supports DryRun, InternalOnly, Reverse, and Local flags.
func (s *VendorSyncer) SyncWithFullOpts(ctx context.Context, opts SyncOptions) error {
	// Check if lockfile exists
	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		if opts.DryRun {
			fmt.Println("No lockfile found. Would generate lockfile from latest commits, then sync files.")
			return nil
		}
		fmt.Println("No lockfile found. Generating lockfile from latest commits...")
		if err := s.update.UpdateAllWithOptions(ctx, UpdateOptions{
			Local:      opts.Local,
			VendorName: opts.VendorName,
			Group:      opts.GroupName,
		}); err != nil {
			return fmt.Errorf("generate lockfile: %w", err)
		}
		fmt.Println()
		fmt.Println("Lockfile created. Now syncing files...")
	}
	if opts.DryRun {
		return s.sync.Sync(ctx, opts)
	}
	return s.syncWithAutoUpdate(ctx, opts)
}

// SyncWithGroup performs sync for all vendors in a group.
// ctx controls cancellation of git operations during sync.
func (s *VendorSyncer) SyncWithGroup(ctx context.Context, groupName string, force, noCache bool) error {
	return s.SyncWithFullOpts(ctx, SyncOptions{
		GroupName: groupName,
		Force:     force,
		NoCache:   noCache,
	})
}

// SyncWithParallel performs sync with parallel processing.
// ctx controls cancellation of git operations during sync.
func (s *VendorSyncer) SyncWithParallel(ctx context.Context, vendorName string, force, noCache bool, parallelOpts types.ParallelOptions) error {
	return s.SyncWithFullOpts(ctx, SyncOptions{
		VendorName: vendorName,
		Force:      force,
		NoCache:    noCache,
		Parallel:   parallelOpts,
	})
}

// UpdateAll updates all vendors and regenerates lockfile.
// ctx controls cancellation of git operations during update.
func (s *VendorSyncer) UpdateAll(ctx context.Context) error {
	return s.update.UpdateAll(ctx)
}

// UpdateAllWithOptions updates all vendors with optional parallel processing and local path support.
// ctx controls cancellation of git operations during update.
func (s *VendorSyncer) UpdateAllWithOptions(ctx context.Context, opts UpdateOptions) error {
	return s.update.UpdateAllWithOptions(ctx, opts)
}

// GetConfig returns the vendor configuration
func (s *VendorSyncer) GetConfig() (types.VendorConfig, error) {
	return s.repository.GetConfig()
}

// GetLockHash retrieves the locked commit hash for a vendor@ref
func (s *VendorSyncer) GetLockHash(vendorName, ref string) string {
	return s.lockStore.GetHash(vendorName, ref)
}

// RunAudit runs the unified audit (verify + scan + license + drift) and returns a combined result.
// ctx controls cancellation for network-dependent sub-checks.
func (s *VendorSyncer) RunAudit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error) {
	return s.auditService.Audit(ctx, opts)
}

// ValidateConfig performs comprehensive config validation
func (s *VendorSyncer) ValidateConfig() error {
	return s.validation.ValidateConfig()
}

// DetectConflicts checks for path conflicts between vendors
func (s *VendorSyncer) DetectConflicts() ([]types.PathConflict, error) {
	return s.validation.DetectConflicts()
}

// FetchRepoDir fetches directory listing from remote repository.
// ctx controls cancellation of git clone/fetch/ls-tree operations.
func (s *VendorSyncer) FetchRepoDir(ctx context.Context, url, ref, subdir string) ([]string, error) {
	return s.explorer.FetchRepoDir(ctx, url, ref, subdir)
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
			Name:          lockEntry.Name,
			Ref:           lockEntry.Ref,
			IsSynced:      isSynced,
			MissingPaths:  missingPaths,
			FileCount:     len(matchingSpec.Mapping),
			PositionCount: len(lockEntry.Positions),
		})
	}

	return types.SyncStatus{
		AllSynced:      allSynced,
		VendorStatuses: vendorStatuses,
	}, nil
}

// CheckUpdates checks for available updates for all vendors.
// ctx controls cancellation of git fetch operations for each vendor.
func (s *VendorSyncer) CheckUpdates(ctx context.Context) ([]types.UpdateCheckResult, error) {
	return s.updateChecker.CheckUpdates(ctx)
}

// Verify checks all vendored files against the lockfile.
// ctx is accepted for cancellation support and future network-based verification.
func (s *VendorSyncer) Verify(ctx context.Context) (*types.VerifyResult, error) {
	return s.verifyService.Verify(ctx)
}

// Scan performs vulnerability scanning against OSV.dev.
// ctx controls cancellation of in-flight HTTP requests.
func (s *VendorSyncer) Scan(ctx context.Context, failOn string) (*types.ScanResult, error) {
	return s.vulnScanner.Scan(ctx, failOn)
}

// LicenseReport generates a license compliance report using the provided policy service.
func (s *VendorSyncer) LicenseReport(policyService LicensePolicyServiceInterface, failOn string) (*types.LicenseReportResult, error) {
	return policyService.GenerateReport(failOn)
}

// Drift detects drift between vendored files and their origin.
// ctx controls cancellation of git operations (clone, fetch, checkout).
func (s *VendorSyncer) Drift(ctx context.Context, opts DriftOptions) (*types.DriftResult, error) {
	return s.driftService.Drift(ctx, opts)
}

// ComplianceCheck runs compliance check for internal vendors.
func (s *VendorSyncer) ComplianceCheck(opts ComplianceOptions) (*types.ComplianceResult, error) {
	return s.complianceService.Check(opts)
}

// CompliancePropagate runs compliance check and propagates changes for internal vendors.
func (s *VendorSyncer) CompliancePropagate(opts ComplianceOptions) (*types.ComplianceResult, error) {
	return s.complianceService.Propagate(opts)
}

// Outdated checks if locked versions are behind upstream HEAD using git ls-remote.
// ctx controls cancellation of ls-remote operations.
func (s *VendorSyncer) Outdated(ctx context.Context, opts OutdatedOptions) (*types.OutdatedResult, error) {
	return s.outdatedSvc.Outdated(ctx, opts)
}

// Status runs the unified status command combining verify and outdated checks.
// ctx controls cancellation of verify and ls-remote operations.
func (s *VendorSyncer) Status(ctx context.Context, opts StatusOptions) (*types.StatusResult, error) {
	svc := NewStatusService(s.verifyService, s.outdatedSvc, s.configStore, s.lockStore)
	return svc.Status(ctx, opts)
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
