package core

import (
	"fmt"
	"path/filepath"
	"time"

	"git-vendor/internal/types"
)

// UpdateService handles update operations and lockfile regeneration
type UpdateService struct {
	configStore ConfigStore
	lockStore   LockStore
	syncService *SyncService
	ui          UICallback
	rootDir     string
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(
	configStore ConfigStore,
	lockStore LockStore,
	syncService *SyncService,
	ui UICallback,
	rootDir string,
) *UpdateService {
	return &UpdateService{
		configStore: configStore,
		lockStore:   lockStore,
		syncService: syncService,
		ui:          ui,
		rootDir:     rootDir,
	}
}

// UpdateAll updates all vendors and regenerates the lockfile
func (s *UpdateService) UpdateAll() error {
	return s.UpdateAllWithOptions(types.ParallelOptions{Enabled: false})
}

// UpdateAllWithOptions updates all vendors with optional parallel processing
func (s *UpdateService) UpdateAllWithOptions(parallelOpts types.ParallelOptions) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	if parallelOpts.Enabled {
		return s.updateAllParallel(config, parallelOpts)
	}

	return s.updateAllSequential(config)
}

// updateAllSequential performs sequential update (original implementation)
func (s *UpdateService) updateAllSequential(config types.VendorConfig) error {
	lock := types.VendorLock{}

	// Start progress tracking
	progress := s.ui.StartProgress(len(config.Vendors), "Updating vendors")
	defer progress.Complete()

	// Update each vendor
	for _, v := range config.Vendors {
		// Sync vendor without lock (force latest)
		// During update, we always force and skip cache (we want fresh data)
		updatedRefs, _, err := s.syncService.syncVendor(v, nil, SyncOptions{Force: true, NoCache: true})
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			// Continue on error - don't fail the whole update
			progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
			continue
		}

		// Add lock entries for each ref
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

		progress.Increment(fmt.Sprintf("✓ %s", v.Name))
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}

// updateAllParallel performs parallel update using worker pool
func (s *UpdateService) updateAllParallel(config types.VendorConfig, parallelOpts types.ParallelOptions) error {
	// Start progress tracking
	progress := s.ui.StartProgress(len(config.Vendors), "Updating vendors (parallel)")
	defer progress.Complete()

	// Create parallel executor
	executor := NewParallelExecutor(parallelOpts, s.ui)

	// Define update function for a single vendor
	updateFunc := func(v types.VendorSpec, opts SyncOptions) (map[string]string, error) {
		updatedRefs, _, err := s.syncService.syncVendor(v, nil, opts)
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			progress.Increment(fmt.Sprintf("✗ %s (failed)", v.Name))
			return nil, err
		}

		// Show success for this vendor
		for ref, hash := range updatedRefs {
			s.ui.ShowSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, hash[:7]))
		}
		progress.Increment(fmt.Sprintf("✓ %s", v.Name))

		return updatedRefs, nil
	}

	// Execute parallel updates
	results, err := executor.ExecuteParallelUpdate(config.Vendors, updateFunc)
	if err != nil {
		// Continue even if some vendors failed - we still want to save successful updates
		s.ui.ShowWarning("Some Updates Failed", err.Error())
	}

	// Build new lockfile from results
	lock := types.VendorLock{}
	for _, result := range results {
		if result.Error != nil {
			// Skip failed vendors
			continue
		}

		// Add lock entries for each ref
		for ref, hash := range result.UpdatedRefs {
			licenseFile := filepath.Join(s.rootDir, LicenseDir, result.Vendor.Name+".txt")

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name:        result.Vendor.Name,
				Ref:         ref,
				CommitHash:  hash,
				LicensePath: licenseFile,
				Updated:     time.Now().Format(time.RFC3339),
			})
		}
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}
