// Package core implements the business logic for git-vendor including configuration, syncing, and Git operations.
package core

import (
	"github.com/EmundoT/git-vendor/internal/types"
)

// ConfigStore handles vendor.yml I/O operations
type ConfigStore interface {
	Load() (types.VendorConfig, error)
	Save(config types.VendorConfig) error
	Path() string
}

// FileConfigStore implements ConfigStore using YAMLStore
type FileConfigStore struct {
	store *YAMLStore[types.VendorConfig]
}

// NewFileConfigStore creates a new FileConfigStore
func NewFileConfigStore(rootDir string) *FileConfigStore {
	return &FileConfigStore{
		store: NewYAMLStore[types.VendorConfig](rootDir, ConfigName, true), // allowMissing=true
	}
}

// Path returns the config file path
func (s *FileConfigStore) Path() string {
	return s.store.Path()
}

// Load reads and parses vendor.yml
func (s *FileConfigStore) Load() (types.VendorConfig, error) {
	return s.store.Load()
}

// Save writes vendor.yml
func (s *FileConfigStore) Save(cfg types.VendorConfig) error {
	return s.store.Save(cfg)
}
