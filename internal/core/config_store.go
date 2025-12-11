package core

import (
	"fmt"
	"os"
	"path/filepath"

	"git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// ConfigStore handles vendor.yml I/O operations
type ConfigStore interface {
	Load() (types.VendorConfig, error)
	Save(config types.VendorConfig) error
	Path() string
}

// FileConfigStore implements ConfigStore using the filesystem
type FileConfigStore struct {
	rootDir string
}

// NewFileConfigStore creates a new FileConfigStore
func NewFileConfigStore(rootDir string) *FileConfigStore {
	return &FileConfigStore{rootDir: rootDir}
}

// Path returns the config file path
func (s *FileConfigStore) Path() string {
	return filepath.Join(s.rootDir, ConfigName)
}

// Load reads and parses vendor.yml
func (s *FileConfigStore) Load() (types.VendorConfig, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return types.VendorConfig{}, nil // OK: file doesn't exist yet
		}
		return types.VendorConfig{}, fmt.Errorf("failed to read vendor.yml: %w", err)
	}

	var cfg types.VendorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return types.VendorConfig{}, fmt.Errorf("invalid vendor.yml: %w", err)
	}

	return cfg, nil
}

// Save writes vendor.yml
func (s *FileConfigStore) Save(cfg types.VendorConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(s.Path(), data, 0644); err != nil {
		return fmt.Errorf("failed to write vendor.yml: %w", err)
	}

	return nil
}
