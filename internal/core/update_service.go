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
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	lock := types.VendorLock{}

	// Update each vendor
	for _, v := range config.Vendors {
		// Sync vendor without lock (force latest)
		updatedRefs, _, err := s.syncService.syncVendor(v, nil)
		if err != nil {
			s.ui.ShowError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
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
	}

	// Save the new lockfile
	return s.lockStore.Save(lock)
}
