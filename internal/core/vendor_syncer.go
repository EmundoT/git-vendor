package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
}

// SilentUICallback is a no-op implementation (for testing/CI)
type SilentUICallback struct{}

func (s *SilentUICallback) ShowError(title, message string)            {}
func (s *SilentUICallback) ShowSuccess(message string)                 {}
func (s *SilentUICallback) ShowWarning(title, message string)          {}
func (s *SilentUICallback) AskConfirmation(title, msg string) bool     { return false }
func (s *SilentUICallback) ShowLicenseCompliance(license string)       {}
func (s *SilentUICallback) StyleTitle(title string) string             { return title }

// VendorSyncer orchestrates vendor operations using injected dependencies
type VendorSyncer struct {
	configStore    ConfigStore
	lockStore      LockStore
	gitClient      GitClient
	fs             FileSystem
	licenseChecker LicenseChecker
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
	return &VendorSyncer{
		configStore:    configStore,
		lockStore:      lockStore,
		gitClient:      gitClient,
		fs:             fs,
		licenseChecker: licenseChecker,
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
	return s.configStore.Save(types.VendorConfig{})
}

// AddVendor adds a new vendor with license compliance check
func (s *VendorSyncer) AddVendor(spec types.VendorSpec) error {
	config, _ := s.configStore.Load()

	// Check if vendor already exists
	existing := FindVendor(config.Vendors, spec.Name)

	// If new vendor, check license compliance
	if existing == nil {
		detectedLicense, err := s.licenseChecker.CheckLicense(spec.URL)
		if err == nil {
			spec.License = detectedLicense
		} else {
			spec.License = "UNKNOWN"
		}

		if !s.licenseChecker.IsAllowed(spec.License) {
			if !s.ui.AskConfirmation(
				fmt.Sprintf("Accept %s License?", spec.License),
				"This license is not in the allowed list. Continue anyway?",
			) {
				return fmt.Errorf("%s", ErrComplianceFailed)
			}
		} else {
			s.ui.ShowLicenseCompliance(spec.License)
		}
	}

	return s.SaveVendor(spec)
}

// SaveVendor saves or updates a vendor spec
func (s *VendorSyncer) SaveVendor(spec types.VendorSpec) error {
	config, _ := s.configStore.Load()

	index := FindVendorIndex(config.Vendors, spec.Name)
	if index >= 0 {
		config.Vendors[index] = spec
	} else {
		config.Vendors = append(config.Vendors, spec)
	}

	if err := s.configStore.Save(config); err != nil {
		return err
	}

	return s.UpdateAll()
}

// RemoveVendor removes a vendor by name
func (s *VendorSyncer) RemoveVendor(name string) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	index := FindVendorIndex(config.Vendors, name)
	if index < 0 {
		return fmt.Errorf(ErrVendorNotFound, name)
	}

	config.Vendors = append(config.Vendors[:index], config.Vendors[index+1:]...)

	// Remove license file
	licensePath := filepath.Join(s.rootDir, LicenseDir, name+".txt")
	s.fs.Remove(licensePath)

	if err := s.configStore.Save(config); err != nil {
		return err
	}

	return s.UpdateAll()
}

// Sync performs locked synchronization
func (s *VendorSyncer) Sync() error {
	return s.SyncWithOptions("", false)
}

// SyncDryRun performs a dry-run sync
func (s *VendorSyncer) SyncDryRun() error {
	return s.sync(true, "", false)
}

// SyncWithOptions performs sync with vendor filter and force option
func (s *VendorSyncer) SyncWithOptions(vendorName string, force bool) error {
	return s.sync(false, vendorName, force)
}

// sync is the internal sync implementation
func (s *VendorSyncer) sync(dryRun bool, vendorName string, force bool) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	lock, err := s.lockStore.Load()
	if err != nil || len(lock.Vendors) == 0 {
		if dryRun {
			fmt.Println("No lockfile found. Would run update to create lockfile.")
			return nil
		}
		fmt.Println("No lockfile found. Running update...")
		return s.UpdateAll()
	}

	// Build lock map
	lockMap := make(map[string]map[string]string)
	for _, l := range lock.Vendors {
		if lockMap[l.Name] == nil {
			lockMap[l.Name] = make(map[string]string)
		}
		lockMap[l.Name][l.Ref] = l.CommitHash
	}

	// Validate vendor exists BEFORE doing any work
	if vendorName != "" {
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
	}

	if dryRun {
		fmt.Println(s.ui.StyleTitle("Sync Plan:"))
		fmt.Println()
	} else {
		// Count vendors to sync
		vendorCount := 0
		for _, v := range config.Vendors {
			if vendorName == "" || v.Name == vendorName {
				vendorCount++
			}
		}
		if vendorCount > 0 {
			fmt.Printf("Syncing %d vendor(s)...\n", vendorCount)
		}
	}

	for _, v := range config.Vendors {
		// Skip vendors that don't match the filter
		if vendorName != "" && v.Name != vendorName {
			continue
		}

		if dryRun {
			s.previewSyncVendor(v, lockMap[v.Name])
		} else {
			// If force is true, pass nil to ignore lock and re-download
			refs := lockMap[v.Name]
			if force {
				refs = nil
			}
			if _, err := s.syncVendor(v, refs); err != nil {
				return err
			}
		}
	}

	return nil
}

// previewSyncVendor shows what would be synced in dry-run mode
func (s *VendorSyncer) previewSyncVendor(v types.VendorSpec, lockedRefs map[string]string) {
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

// UpdateAll updates all vendors and regenerates lockfile
func (s *VendorSyncer) UpdateAll() error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	lock := types.VendorLock{}

	for _, v := range config.Vendors {
		updatedRefs, err := s.syncVendor(v, nil)
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			continue
		}

		for ref, hash := range updatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicenseDir, v.Name+".txt")

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name:        v.Name,
				Ref:         ref,
				CommitHash:  hash,
				LicensePath: licenseFile,
				Updated:     time.Now().Format(time.RFC3339),
			})

			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, hash[:7]))
		}
	}

	return s.lockStore.Save(lock)
}

// syncVendor syncs a single vendor
func (s *VendorSyncer) syncVendor(v types.VendorSpec, lockedRefs map[string]string) (map[string]string, error) {
	fmt.Printf("⠿ %s (cloning repository...)\n", v.Name)

	tempDir, err := s.fs.CreateTemp("", "git-vendor-*")
	if err != nil {
		return nil, err
	}
	defer s.fs.RemoveAll(tempDir)

	if err := s.gitClient.Init(tempDir); err != nil {
		return nil, err
	}
	if err := s.gitClient.AddRemote(tempDir, "origin", v.URL); err != nil {
		return nil, err
	}

	results := make(map[string]string)

	for _, spec := range v.Specs {
		targetCommit := ""
		isLocked := false

		if lockedRefs != nil {
			if h, ok := lockedRefs[spec.Ref]; ok && h != "" {
				targetCommit = h
				isLocked = true
			}
		}

		if isLocked {
			// Try shallow fetch first, fall back to full fetch if needed
			if err := s.gitClient.Fetch(tempDir, 1, spec.Ref); err != nil {
				// Shallow fetch failed, try full fetch
				if err := s.gitClient.FetchAll(tempDir); err != nil {
					return nil, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
				}
			}
			if err := s.gitClient.Checkout(tempDir, targetCommit); err != nil {
				// Detect stale lock hash error and provide helpful message
				errMsg := err.Error()
				if strings.Contains(errMsg, "reference is not a tree") || strings.Contains(errMsg, "not a valid object") {
					return nil, fmt.Errorf(ErrStaleCommitMsg, targetCommit[:7])
				}
				return nil, fmt.Errorf(ErrCheckoutFailed, targetCommit, err)
			}
		} else {
			// Try shallow fetch first, fall back to full fetch if needed
			if err := s.gitClient.Fetch(tempDir, 1, spec.Ref); err != nil {
				// Shallow fetch failed, try full fetch
				if err := s.gitClient.FetchAll(tempDir); err != nil {
					return nil, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
				}
			}
			if err := s.gitClient.Checkout(tempDir, "FETCH_HEAD"); err != nil {
				if err := s.gitClient.Checkout(tempDir, spec.Ref); err != nil {
					return nil, fmt.Errorf(ErrRefCheckoutFailed, spec.Ref, err)
				}
			}
		}

		hash, _ := s.gitClient.GetHeadHash(tempDir)
		results[spec.Ref] = hash

		// License Automation
		if err := s.copyLicense(tempDir, v.Name); err != nil {
			return nil, err
		}

		// Copy files according to mappings
		if err := s.copyMappings(tempDir, v, spec); err != nil {
			return nil, err
		}

		fmt.Printf("  ✓ %s @ %s (synced %d path(s))\n", v.Name, spec.Ref, len(spec.Mapping))
	}

	return results, nil
}

// copyLicense copies license file from temp repo to vendor/licenses
func (s *VendorSyncer) copyLicense(tempDir, vendorName string) error {
	var licenseSrc string
	for _, name := range LicenseFileNames {
		path := filepath.Join(tempDir, name)
		if _, err := s.fs.Stat(path); err == nil {
			licenseSrc = path
			break
		}
	}

	if licenseSrc != "" {
		if err := s.fs.MkdirAll(filepath.Join(s.rootDir, LicenseDir), 0755); err != nil {
			return err
		}
		dest := filepath.Join(s.rootDir, LicenseDir, vendorName+".txt")
		if err := s.fs.CopyFile(licenseSrc, dest); err != nil {
			return fmt.Errorf("failed to copy license from %s to %s: %w", licenseSrc, dest, err)
		}
	}

	return nil
}

// copyMappings copies files according to path mappings
func (s *VendorSyncer) copyMappings(tempDir string, v types.VendorSpec, spec types.BranchSpec) error {
	for _, mapping := range spec.Mapping {
		srcClean := strings.Replace(mapping.From, "blob/"+spec.Ref+"/", "", 1)
		srcClean = strings.Replace(srcClean, "tree/"+spec.Ref+"/", "", 1)

		srcPath := filepath.Join(tempDir, srcClean)
		destPath := mapping.To

		// Use auto-path computation if destination not explicitly specified
		if destPath == "" || destPath == "." {
			destPath = ComputeAutoPath(srcClean, spec.DefaultTarget, v.Name)
		}

		// Validate destination path to prevent path traversal attacks
		if err := ValidateDestPath(destPath); err != nil {
			return err
		}

		info, err := s.fs.Stat(srcPath)
		if err != nil {
			return fmt.Errorf(ErrPathNotFound, srcClean)
		}

		if info.IsDir() {
			if err := s.fs.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := s.fs.CopyDir(srcPath, destPath); err != nil {
				return fmt.Errorf("failed to copy directory %s to %s: %w", srcPath, destPath, err)
			}
		} else {
			if err := s.fs.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			if err := s.fs.CopyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destPath, err)
			}
		}
	}

	return nil
}

// GetConfig returns the vendor configuration
func (s *VendorSyncer) GetConfig() (types.VendorConfig, error) {
	return s.configStore.Load()
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
	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check for empty vendors
	if len(config.Vendors) == 0 {
		return fmt.Errorf("no vendors configured. Run 'git-vendor add' to add your first dependency")
	}

	// Check for duplicate vendor names
	names := make(map[string]bool)
	for _, vendor := range config.Vendors {
		if names[vendor.Name] {
			return fmt.Errorf("duplicate vendor name: %s", vendor.Name)
		}
		names[vendor.Name] = true

		// Validate vendor has URL
		if vendor.URL == "" {
			return fmt.Errorf("vendor %s has no URL", vendor.Name)
		}

		// Validate vendor has at least one spec
		if len(vendor.Specs) == 0 {
			return fmt.Errorf("vendor %s has no specs configured", vendor.Name)
		}

		// Validate each spec
		for _, spec := range vendor.Specs {
			if spec.Ref == "" {
				return fmt.Errorf("vendor %s has a spec with no ref", vendor.Name)
			}

			if len(spec.Mapping) == 0 {
				return fmt.Errorf("vendor %s @ %s has no path mappings", vendor.Name, spec.Ref)
			}

			// Validate each mapping
			for _, mapping := range spec.Mapping {
				if mapping.From == "" {
					return fmt.Errorf("vendor %s @ %s has a mapping with empty 'from' path", vendor.Name, spec.Ref)
				}
			}
		}
	}

	return nil
}

// DetectConflicts checks for path conflicts between vendors
func (s *VendorSyncer) DetectConflicts() ([]types.PathConflict, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, err
	}

	var conflicts []types.PathConflict

	// Build a map of destination paths to vendor+mapping
	type PathOwner struct {
		VendorName string
		Mapping    types.PathMapping
		Ref        string
	}
	pathMap := make(map[string][]PathOwner)

	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				destPath := mapping.To

				// Use auto-path computation if destination not explicitly specified
				if destPath == "" || destPath == "." {
					destPath = ComputeAutoPath(mapping.From, spec.DefaultTarget, vendor.Name)
				}

				// Normalize path
				destPath = filepath.Clean(destPath)

				pathMap[destPath] = append(pathMap[destPath], PathOwner{
					VendorName: vendor.Name,
					Mapping:    mapping,
					Ref:        spec.Ref,
				})
			}
		}
	}

	// Check for conflicts
	for path, owners := range pathMap {
		if len(owners) > 1 {
			// Multiple vendors map to the same path
			for i := 0; i < len(owners)-1; i++ {
				for j := i + 1; j < len(owners); j++ {
					conflicts = append(conflicts, types.PathConflict{
						Path:     path,
						Vendor1:  owners[i].VendorName,
						Vendor2:  owners[j].VendorName,
						Mapping1: owners[i].Mapping,
						Mapping2: owners[j].Mapping,
					})
				}
			}
		}
	}

	// Also check for overlapping directory paths
	var allPaths []string
	for path := range pathMap {
		allPaths = append(allPaths, path)
	}

	for i := 0; i < len(allPaths)-1; i++ {
		for j := i + 1; j < len(allPaths); j++ {
			path1 := allPaths[i]
			path2 := allPaths[j]

			// Check if one path is a subdirectory of another
			if isSubPath(path1, path2) {
				owners1 := pathMap[path1]
				owners2 := pathMap[path2]

				// Skip malformed entries (empty slices)
				if len(owners1) == 0 || len(owners2) == 0 {
					continue
				}

				// Only report if different vendors
				if owners1[0].VendorName != owners2[0].VendorName {
					conflicts = append(conflicts, types.PathConflict{
						Path:     fmt.Sprintf("%s overlaps with %s", path1, path2),
						Vendor1:  owners1[0].VendorName,
						Vendor2:  owners2[0].VendorName,
						Mapping1: owners1[0].Mapping,
						Mapping2: owners2[0].Mapping,
					})
				}
			}
		}
	}

	return conflicts, nil
}

// isSubPath checks if path1 is a subdirectory of path2 or vice versa
func isSubPath(path1, path2 string) bool {
	path1 = filepath.Clean(path1)
	path2 = filepath.Clean(path2)

	// Check if path2 is under path1
	rel, err := filepath.Rel(path1, path2)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	// Check if path1 is under path2
	rel, err = filepath.Rel(path2, path1)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	return false
}

// FetchRepoDir fetches directory listing from remote repository
func (s *VendorSyncer) FetchRepoDir(url, ref, subdir string) ([]string, error) {
	tempDir, err := s.fs.CreateTemp("", "git-vendor-index-*")
	if err != nil {
		return nil, err
	}
	defer s.fs.RemoveAll(tempDir)

	opts := &CloneOptions{
		Filter:     "blob:none",
		NoCheckout: true,
		Depth:      1,
	}

	if err := s.gitClient.Clone(tempDir, url, opts); err != nil {
		return nil, err
	}

	if ref != "" && ref != "HEAD" {
		s.gitClient.Fetch(tempDir, 0, ref)
	}

	target := ref
	if target == "" {
		target = "HEAD"
	}

	return s.gitClient.ListTree(tempDir, target, subdir)
}

// ListLocalDir lists local directory contents
func (s *VendorSyncer) ListLocalDir(path string) ([]string, error) {
	return s.fs.ReadDir(path)
}

// ParseSmartURL delegates to the git operations parser
func (s *VendorSyncer) ParseSmartURL(rawURL string) (string, string, string) {
	return ParseSmartURL(rawURL)
}

// CheckGitHubLicense delegates to the license checker
func (s *VendorSyncer) CheckGitHubLicense(url string) (string, error) {
	return s.licenseChecker.CheckLicense(url)
}
