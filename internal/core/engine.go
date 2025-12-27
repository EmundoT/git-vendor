package core

import (
	"os"
	"os/exec"

	"git-vendor/internal/core/providers"
	"git-vendor/internal/types"
)

// Constants for the new "Clean Root" structure
const (
	VendorDir  = "vendor"
	ConfigName = "vendor.yml"
	LockName   = "vendor.lock"
	LicenseDir = "licenses"
)

// AllowedLicenses defines the list of open-source licenses permitted by default.
var AllowedLicenses = []string{
	"MIT",
	"Apache-2.0",
	"BSD-3-Clause",
	"BSD-2-Clause",
	"ISC",
	"Unlicense",
	"CC0-1.0",
}

// Error messages
const (
	ErrStaleCommitMsg    = "locked commit %s no longer exists in the repository.\n\nThis usually happens when the remote repository has been force-pushed or the commit was deleted.\nRun 'git-vendor update' to fetch the latest commit and update the lockfile, then try syncing again"
	ErrCheckoutFailed    = "checkout locked hash %s failed: %w"
	ErrRefCheckoutFailed = "checkout ref %s failed: %w"
	ErrPathNotFound      = "path '%s' not found"
	ErrInvalidURL        = "invalid url"
	ErrVendorNotFound    = "vendor '%s' not found"
	ErrComplianceFailed  = "compliance check failed"
)

// LicenseFileNames lists standard filenames checked when searching for repository licenses.
var LicenseFileNames = []string{"LICENSE", "LICENSE.txt", "COPYING"}

// Verbose controls whether git commands are logged
var Verbose = false

// Manager provides the main API for git-vendor operations
// It delegates to VendorSyncer for all business logic
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
	fs := NewOSFileSystem()

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

	// Create syncer with injected dependencies
	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, licenseChecker, rootDir, ui)

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
	return m.syncer.rootDir + "/" + LicenseDir + "/" + name + ".txt"
}

// IsGitInstalled checks if git is available on the system
func IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsVendorInitialized checks if the vendor directory structure exists
func IsVendorInitialized() bool {
	info, err := os.Stat(VendorDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ErrNotInitialized is returned when vendor directory doesn't exist
const ErrNotInitialized = "vendor directory not found. Run 'git-vendor init' first"

// Init initializes the vendor directory structure
func (m *Manager) Init() error {
	return m.syncer.Init()
}

// ParseSmartURL extracts repository, ref, and path from URLs
func (m *Manager) ParseSmartURL(rawURL string) (string, string, string) {
	return m.syncer.ParseSmartURL(rawURL)
}

// FetchRepoDir fetches directory listing from a remote repository
func (m *Manager) FetchRepoDir(url, ref, subdir string) ([]string, error) {
	return m.syncer.FetchRepoDir(url, ref, subdir)
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
func (m *Manager) SaveVendor(spec types.VendorSpec) error {
	return m.syncer.SaveVendor(spec)
}

// AddVendor adds a new vendor with license compliance check
func (m *Manager) AddVendor(spec types.VendorSpec) error {
	return m.syncer.AddVendor(spec)
}

// Sync performs locked synchronization
func (m *Manager) Sync() error {
	return m.syncer.Sync()
}

// SyncDryRun performs a dry-run sync
func (m *Manager) SyncDryRun() error {
	return m.syncer.SyncDryRun()
}

// SyncWithOptions performs sync with vendor filter, force, and cache options
func (m *Manager) SyncWithOptions(vendorName string, force, noCache bool) error {
	return m.syncer.SyncWithOptions(vendorName, force, noCache)
}

// UpdateAll updates all vendors and regenerates lockfile
func (m *Manager) UpdateAll() error {
	return m.syncer.UpdateAll()
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

// Audit checks lockfile status
func (m *Manager) Audit() {
	m.syncer.Audit()
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
