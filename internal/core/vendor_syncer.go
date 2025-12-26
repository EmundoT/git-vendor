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
}

// SilentUICallback is a no-op implementation (for testing/CI)
type SilentUICallback struct{}

func (s *SilentUICallback) ShowError(title, message string)            {}
func (s *SilentUICallback) ShowSuccess(message string)                 {}
func (s *SilentUICallback) ShowWarning(title, message string)          {}
func (s *SilentUICallback) AskConfirmation(title, msg string) bool     { return false }
func (s *SilentUICallback) ShowLicenseCompliance(license string)       {}
func (s *SilentUICallback) StyleTitle(title string) string             { return title }
func (s *SilentUICallback) GetOutputMode() OutputMode                  { return OutputNormal }
func (s *SilentUICallback) IsAutoApprove() bool                        { return false }
func (s *SilentUICallback) FormatJSON(output JSONOutput) error         { return nil }

// VendorSyncer orchestrates vendor operations using domain services
type VendorSyncer struct {
	repository     *VendorRepository
	sync           *SyncService
	update         *UpdateService
	license        *LicenseService
	validation     *ValidationService
	explorer       *RemoteExplorer
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
	sync := NewSyncService(configStore, lockStore, gitClient, fs, fileCopy, license, ui, rootDir)
	update := NewUpdateService(configStore, lockStore, sync, ui, rootDir)
	validation := NewValidationService(configStore)
	explorer := NewRemoteExplorer(gitClient, fs)

	return &VendorSyncer{
		repository:     repository,
		sync:           sync,
		update:         update,
		license:        license,
		validation:     validation,
		explorer:       explorer,
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
	s.fs.Remove(licensePath)

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

// SyncWithOptions performs sync with vendor filter and force option
func (s *VendorSyncer) SyncWithOptions(vendorName string, force bool) error {
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

// syncVendor is exposed for testing - delegates to sync service
func (s *VendorSyncer) syncVendor(v types.VendorSpec, lockedRefs map[string]string) (map[string]string, CopyStats, error) {
	return s.sync.syncVendor(v, lockedRefs)
}
