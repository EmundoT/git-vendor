package core

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// Schema version constants
const (
	// CurrentSchemaVersion is the version written to new lockfiles
	CurrentSchemaVersion = "1.1"
	// MaxSupportedMajor is the maximum major version this CLI can handle
	MaxSupportedMajor = 1
	// MaxSupportedMinor is the maximum minor version this CLI fully understands
	MaxSupportedMinor = 1
)

// parseSchemaVersion parses a schema version string into major and minor components.
// Empty string defaults to version 1.0 for backward compatibility.
func parseSchemaVersion(version string) (major, minor int, err error) {
	if version == "" {
		return 1, 0, nil // Default to 1.0 for backward compatibility
	}

	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid schema version format: %q (expected major.minor)", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	if major < 0 || minor < 0 {
		return 0, 0, fmt.Errorf("negative version numbers not allowed: %q", version)
	}

	return major, minor, nil
}

// validateSchemaVersion checks if the lockfile version is compatible with this CLI.
// Returns an error if the major version is unsupported.
// Writes a warning to the provided writer if minor version is newer than expected.
func validateSchemaVersion(version string, warnWriter io.Writer) error {
	major, minor, err := parseSchemaVersion(version)
	if err != nil {
		return fmt.Errorf("parse schema version: %w", err)
	}

	if major > MaxSupportedMajor {
		return fmt.Errorf(
			"lockfile schema version %q requires a newer git-vendor version\n"+
				"  Your CLI supports schema v%d.x, but lockfile is v%d.x\n"+
				"  Run 'git vendor version' to check your version, then update git-vendor",
			version, MaxSupportedMajor, major)
	}

	if major == MaxSupportedMajor && minor > MaxSupportedMinor {
		if warnWriter != nil {
			fmt.Fprintf(warnWriter,
				"Warning: Lockfile schema version %q is newer than expected (%d.%d)\n"+
					"  Some metadata fields may be ignored. Consider updating git-vendor.\n",
				version, MaxSupportedMajor, MaxSupportedMinor)
		}
	}

	return nil
}

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

// Load reads and parses vendor.lock, validating schema version compatibility.
// Returns an error if the major version is unsupported.
// Writes a warning to stderr if minor version is newer than expected.
func (s *FileLockStore) Load() (types.VendorLock, error) {
	lock, err := s.store.Load()
	if err != nil {
		return lock, err
	}

	// Validate schema version compatibility
	if err := validateSchemaVersion(lock.SchemaVersion, os.Stderr); err != nil {
		return types.VendorLock{}, err
	}

	return lock, nil
}

// Save writes vendor.lock, always setting the current schema version.
func (s *FileLockStore) Save(lock types.VendorLock) error {
	lock.SchemaVersion = CurrentSchemaVersion
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
