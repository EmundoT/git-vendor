package core

import (
	"fmt"
	"os"
	"path/filepath"

	"git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// LockStore handles vendor.lock I/O operations
type LockStore interface {
	Load() (types.VendorLock, error)
	Save(lock types.VendorLock) error
	Path() string
	GetHash(vendorName, ref string) string
}

// FileLockStore implements LockStore using the filesystem
type FileLockStore struct {
	rootDir string
}

// NewFileLockStore creates a new FileLockStore
func NewFileLockStore(rootDir string) *FileLockStore {
	return &FileLockStore{rootDir: rootDir}
}

// Path returns the lock file path
func (s *FileLockStore) Path() string {
	return filepath.Join(s.rootDir, LockName)
}

// Load reads and parses vendor.lock
func (s *FileLockStore) Load() (types.VendorLock, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return types.VendorLock{}, err
	}

	var lock types.VendorLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return types.VendorLock{}, fmt.Errorf("invalid vendor.lock: %w", err)
	}

	return lock, nil
}

// Save writes vendor.lock
func (s *FileLockStore) Save(lock types.VendorLock) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("failed to marshal lock: %w", err)
	}

	if err := os.WriteFile(s.Path(), data, 0644); err != nil {
		return fmt.Errorf("failed to write vendor.lock: %w", err)
	}

	return nil
}

// GetHash retrieves the locked commit hash for a vendor@ref
func (s *FileLockStore) GetHash(vendorName, ref string) string {
	lock, err := s.Load()
	if err != nil {
		return ""
	}

	for _, l := range lock.Vendors {
		if l.Name == vendorName && l.Ref == ref {
			return l.CommitHash
		}
	}

	return ""
}
