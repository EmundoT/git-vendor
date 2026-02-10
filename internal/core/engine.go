package core

import (
	"context"
	"os"

	git "github.com/EmundoT/git-plumbing"

	"github.com/EmundoT/git-vendor/internal/core/providers"
	"github.com/EmundoT/git-vendor/internal/types"
)

// Verbose controls whether git commands are logged
var Verbose = false

// Manager provides the main API for git-vendor operations.
// Manager delegates to VendorSyncer for all business logic.
// All long-running methods accept context.Context for cancellation support.
type Manager struct {
	RootDir string
	syncer  *VendorSyncer
}

// NewManager creates a new Manager with default dependencies
func NewManager() *Manager {
	rootDir := VendorDir

	// Create default implementations of all dependencies
	configStore := NewFileConfigStore(rootDir)
	lockStore := NewFileLockStore(rootDir)
	gitClient := NewSystemGitClient(Verbose)
	fs := NewRootedFileSystem(".")

	// Create provider registry for multi-platform URL parsing
	providerRegistry := providers.NewProviderRegistry()

	// Create multi-platform license checker (supports GitHub, GitLab, Bitbucket, generic)
	licenseChecker := NewMultiPlatformLicenseChecker(
		providerRegistry,
		fs,
		gitClient,
		AllowedLicenses,
	)

	ui := &SilentUICallback{} // Default to silent

	// Create syncer with injected dependencies (nil overrides = all defaults)
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, rootDir, ui, nil)

	return &Manager{
		RootDir: rootDir,
		syncer:  syncer,
	}
}

// SetUICallback sets the UI callback for user interactions
func (m *Manager) SetUICallback(ui UICallback) {
	m.syncer.ui = ui
}

// NewManagerWithSyncer creates a Manager with a custom VendorSyncer (useful for testing)
func NewManagerWithSyncer(syncer *VendorSyncer) *Manager {
	return &Manager{
		RootDir: syncer.rootDir,
		syncer:  syncer,
	}
}

// ConfigPath returns the path to vendor.yml
func (m *Manager) ConfigPath() string {
	return m.syncer.configStore.Path()
}

// LockPath returns the path to vendor.lock
func (m *Manager) LockPath() string {
	return m.syncer.lockStore.Path()
}

// LicensePath returns the path for a vendor's license file
func (m *Manager) LicensePath(name string) string {
	return m.syncer.rootDir + "/" + LicensesDir + "/" + name + ".txt"
}

// IsGitInstalled checks if git is available on the system
func IsGitInstalled() bool {
	return git.IsInstalled()
}

// IsVendorInitialized checks if the vendor directory structure exists
func IsVendorInitialized() bool {
	info, err := os.Stat(VendorDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Init initializes the vendor directory structure
func (m *Manager) Init() error {
	return m.syncer.Init()
}

// ParseSmartURL extracts repository, ref, and path from URLs
func (m *Manager) ParseSmartURL(rawURL string) (string, string, string) {
	return m.syncer.ParseSmartURL(rawURL)
}

// FetchRepoDir fetches directory listing from a remote repository.
// ctx controls cancellation of git clone/fetch/ls-tree operations.
func (m *Manager) FetchRepoDir(ctx context.Context, url, ref, subdir string) ([]string, error) {
	return m.syncer.FetchRepoDir(ctx, url, ref, subdir)
}

// ListLocalDir lists contents of a local directory
func (m *Manager) ListLocalDir(path string) ([]string, error) {
	return m.syncer.ListLocalDir(path)
}

// RemoveVendor removes a vendor by name
func (m *Manager) RemoveVendor(name string) error {
	return m.syncer.RemoveVendor(name)
}

// SaveVendor saves or updates a vendor spec
func (m *Manager) SaveVendor(spec *types.VendorSpec) error {
	return m.syncer.SaveVendor(spec)
}

// AddVendor adds a new vendor with license compliance check
func (m *Manager) AddVendor(spec *types.VendorSpec) error {
	return m.syncer.AddVendor(spec)
}

// Sync performs locked synchronization.
// ctx controls cancellation of git operations during sync.
func (m *Manager) Sync(ctx context.Context) error {
	return m.syncer.Sync(ctx)
}

// SyncDryRun performs a dry-run sync.
// ctx controls cancellation of the preview operation.
func (m *Manager) SyncDryRun(ctx context.Context) error {
	return m.syncer.SyncDryRun(ctx)
}

// SyncWithOptions performs sync with vendor filter, force, and cache options.
// ctx controls cancellation of git operations during sync.
func (m *Manager) SyncWithOptions(ctx context.Context, vendorName string, force, noCache bool) error {
	return m.syncer.SyncWithOptions(ctx, vendorName, force, noCache)
}

// SyncWithGroup performs sync for all vendors in a group.
// ctx controls cancellation of git operations during sync.
func (m *Manager) SyncWithGroup(ctx context.Context, groupName string, force, noCache bool) error {
	return m.syncer.SyncWithGroup(ctx, groupName, force, noCache)
}

// SyncWithParallel performs sync with parallel processing.
// ctx controls cancellation of git operations during sync.
func (m *Manager) SyncWithParallel(ctx context.Context, vendorName string, force, noCache bool, parallelOpts types.ParallelOptions) error {
	return m.syncer.SyncWithParallel(ctx, vendorName, force, noCache, parallelOpts)
}

// UpdateAll updates all vendors and regenerates lockfile.
// ctx controls cancellation of git operations during update.
func (m *Manager) UpdateAll(ctx context.Context) error {
	return m.syncer.UpdateAll(ctx)
}

// UpdateAllWithParallel updates all vendors with parallel processing.
// ctx controls cancellation of git operations during update.
func (m *Manager) UpdateAllWithParallel(ctx context.Context, parallelOpts types.ParallelOptions) error {
	return m.syncer.UpdateAllWithParallel(ctx, parallelOpts)
}

// CheckGitHubLicense checks a repository's license via GitHub API
func (m *Manager) CheckGitHubLicense(rawURL string) (string, error) {
	return m.syncer.CheckGitHubLicense(rawURL)
}

// GetConfig returns the vendor configuration
func (m *Manager) GetConfig() (types.VendorConfig, error) {
	return m.syncer.GetConfig()
}

// GetLockHash retrieves the locked commit hash for a vendor@ref
func (m *Manager) GetLockHash(vendorName, ref string) string {
	return m.syncer.GetLockHash(vendorName, ref)
}

// GetLock returns the current lockfile
func (m *Manager) GetLock() (types.VendorLock, error) {
	return m.syncer.lockStore.Load()
}

// RunAudit runs the unified audit (verify + scan + license + drift) and returns a combined result.
// ctx controls cancellation for network-dependent sub-checks (scan, drift).
func (m *Manager) RunAudit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error) {
	return m.syncer.RunAudit(ctx, opts)
}

// DetectConflicts checks for path conflicts between vendors
func (m *Manager) DetectConflicts() ([]types.PathConflict, error) {
	return m.syncer.DetectConflicts()
}

// ValidateConfig performs comprehensive config validation
func (m *Manager) ValidateConfig() error {
	return m.syncer.ValidateConfig()
}

// CheckSyncStatus checks if local files are in sync with the lockfile
func (m *Manager) CheckSyncStatus() (types.SyncStatus, error) {
	return m.syncer.CheckSyncStatus()
}

// CheckUpdates checks for available updates for all vendors.
// ctx controls cancellation of git fetch operations for each vendor.
func (m *Manager) CheckUpdates(ctx context.Context) ([]types.UpdateCheckResult, error) {
	return m.syncer.CheckUpdates(ctx)
}

// Verify checks all vendored files against the lockfile.
// ctx is accepted for cancellation support and future network-based verification.
func (m *Manager) Verify(ctx context.Context) (*types.VerifyResult, error) {
	return m.syncer.Verify(ctx)
}

// Scan performs vulnerability scanning against OSV.dev.
// ctx controls cancellation of in-flight HTTP requests to OSV.dev.
func (m *Manager) Scan(ctx context.Context, failOn string) (*types.ScanResult, error) {
	return m.syncer.Scan(ctx, failOn)
}

// LicenseReport generates a license compliance report.
// policyPath overrides the default policy file location; empty string uses PolicyFile constant.
// failOn: "deny" (default) or "warn" to also fail on warnings.
func (m *Manager) LicenseReport(policyPath, failOn string) (*types.LicenseReportResult, error) {
	if policyPath == "" {
		policyPath = PolicyFile
	}
	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		return nil, err
	}
	svc := NewLicensePolicyService(&policy, policyPath, m.syncer.configStore, m.syncer.lockStore)
	return m.syncer.LicenseReport(svc, failOn)
}

// EvaluateLicensePolicy loads the policy and evaluates a single license.
// EvaluateLicensePolicy is used during "add" to check a license against the policy.
// policyPath overrides the default policy file location; empty string uses PolicyFile constant.
func (m *Manager) EvaluateLicensePolicy(license, policyPath string) string {
	if policyPath == "" {
		policyPath = PolicyFile
	}
	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		// If policy can't be loaded, fall back to default allow-list behavior
		policy = DefaultLicensePolicy()
	}
	svc := NewLicensePolicyService(&policy, policyPath, m.syncer.configStore, m.syncer.lockStore)
	return svc.Evaluate(license)
}

// Drift detects drift between vendored files and their origin.
// ctx controls cancellation of git operations (clone, fetch, checkout).
func (m *Manager) Drift(ctx context.Context, opts DriftOptions) (*types.DriftResult, error) {
	return m.syncer.Drift(ctx, opts)
}

// MigrateLockfile updates an existing lockfile to add missing metadata fields
func (m *Manager) MigrateLockfile() (int, error) {
	return m.syncer.MigrateLockfile()
}

// DiffVendor shows commit differences between locked and latest versions
func (m *Manager) DiffVendor(vendorName string) ([]types.VendorDiff, error) {
	return m.syncer.DiffVendor(vendorName)
}

// WatchConfig watches for changes to vendor.yml and triggers a callback
func (m *Manager) WatchConfig(callback func() error) error {
	return m.syncer.WatchConfig(callback)
}

// GenerateSBOM generates a Software Bill of Materials in the specified format
func (m *Manager) GenerateSBOM(format SBOMFormat, projectName string) ([]byte, error) {
	generator := NewSBOMGenerator(m.syncer.lockStore, m.syncer.configStore, projectName)
	return generator.Generate(format)
}

// === LLM-Friendly CLI Commands (Spec 072) ===

// CreateVendorEntry adds a new vendor to config without triggering sync/update.
func (m *Manager) CreateVendorEntry(name, url, ref, license string) error {
	return m.syncer.CreateVendorEntry(name, url, ref, license)
}

// RenameVendor renames a vendor across config, lockfile, and license file.
func (m *Manager) RenameVendor(oldName, newName string) error {
	return m.syncer.RenameVendor(oldName, newName)
}

// AddMappingToVendor adds a path mapping to an existing vendor.
func (m *Manager) AddMappingToVendor(vendorName, from, to, ref string) error {
	return m.syncer.AddMappingToVendor(vendorName, from, to, ref)
}

// RemoveMappingFromVendor removes a path mapping from a vendor by source path.
func (m *Manager) RemoveMappingFromVendor(vendorName, from string) error {
	return m.syncer.RemoveMappingFromVendor(vendorName, from)
}

// UpdateMappingInVendor changes the destination of an existing mapping.
func (m *Manager) UpdateMappingInVendor(vendorName, from, newTo string) error {
	return m.syncer.UpdateMappingInVendor(vendorName, from, newTo)
}

// ShowVendor returns detailed vendor info combining config and lockfile data.
func (m *Manager) ShowVendor(name string) (map[string]interface{}, error) {
	return m.syncer.ShowVendor(name)
}

// GetConfigValue retrieves a config value by dotted key path.
func (m *Manager) GetConfigValue(key string) (interface{}, error) {
	return m.syncer.GetConfigValue(key)
}

// SetConfigValue sets a config value by dotted key path.
func (m *Manager) SetConfigValue(key, value string) error {
	return m.syncer.SetConfigValue(key, value)
}

// CheckVendorStatus checks the sync status of a single vendor.
func (m *Manager) CheckVendorStatus(vendorName string) (map[string]interface{}, error) {
	return m.syncer.CheckVendorStatus(vendorName)
}

// UpdateVerboseMode updates the verbose flag for git operations
func (m *Manager) UpdateVerboseMode(verbose bool) {
	// Update the global git client
	gitClient := NewSystemGitClient(verbose)
	m.syncer.gitClient = gitClient
}

// Test helper methods - these expose internal functionality for testing

func (m *Manager) isLicenseAllowed(license string) bool {
	return m.syncer.licenseChecker.IsAllowed(license)
}

func (m *Manager) loadConfig() (types.VendorConfig, error) {
	return m.syncer.configStore.Load()
}

func (m *Manager) saveConfig(cfg types.VendorConfig) error {
	return m.syncer.configStore.Save(cfg)
}

func (m *Manager) loadLock() (types.VendorLock, error) {
	return m.syncer.lockStore.Load()
}

func (m *Manager) saveLock(lock types.VendorLock) error {
	return m.syncer.lockStore.Save(lock)
}
