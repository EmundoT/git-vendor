package core

import (
	"github.com/EmundoT/git-vendor/internal/types"
)

// LockStore handles vendor.lock I/O operations
type LockStore interface {
	Load() (types.VendorLock, error)
	Save(lock types.VendorLock) error
	Path() string
	GetHash(vendorName, ref string) string
}

// FileLockStore implements LockStore using YAMLStore
type FileLockStore struct {
	store *YAMLStore[types.VendorLock]
}

// NewFileLockStore creates a new FileLockStore
func NewFileLockStore(rootDir string) *FileLockStore {
	return &FileLockStore{
		store: NewYAMLStore[types.VendorLock](rootDir, LockName, false), // allowMissing=false
	}
}

// Path returns the lock file path
func (s *FileLockStore) Path() string {
	return s.store.Path()
}

// Load reads and parses vendor.lock
func (s *FileLockStore) Load() (types.VendorLock, error) {
	return s.store.Load()
}

// Save writes vendor.lock
func (s *FileLockStore) Save(lock types.VendorLock) error {
	return s.store.Save(lock)
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
