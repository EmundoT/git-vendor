package core

import (
	"fmt"

	"github.com/EmundoT/git-vendor/internal/types"
)

// AcceptOptions configures the accept command behavior.
// #cli.accept #drift
type AcceptOptions struct {
	VendorName string // Target vendor (required)
	FilePath   string // Specific file to accept (empty = all modified files)
	Clear      bool   // Remove accepted_drift entries instead of adding
	NoCommit   bool   // Skip auto-commit of lockfile change
}

// AcceptResult holds the outcome of an accept operation.
// AcceptResult reports which files were accepted or cleared for audit trail purposes.
type AcceptResult struct {
	VendorName    string   `json:"vendor_name"`              // Vendor that was operated on
	AcceptedFiles []string `json:"accepted_files,omitempty"` // Files whose drift was accepted (empty for --clear)
	ClearedFiles  []string `json:"cleared_files,omitempty"`  // Files whose accepted drift was cleared (empty for non-clear)
}

// AcceptService handles drift acceptance for vendored files.
// AcceptService re-hashes local files and writes accepted_drift entries to the lockfile,
// allowing verify/status to report those files as "accepted" rather than "modified".
type AcceptService struct {
	lockStore LockStore
	cache     CacheStore
}

// NewAcceptService creates a new AcceptService with the given dependencies.
func NewAcceptService(lockStore LockStore, cache CacheStore) *AcceptService {
	return &AcceptService{
		lockStore: lockStore,
		cache:     cache,
	}
}

// Accept processes drift acceptance or clearing for a vendor's files.
// When Clear is false, Accept computes local hashes for files with lock mismatches
// and writes them to accepted_drift. When Clear is true, Accept removes accepted_drift
// entries for the vendor (or a specific file).
func (s *AcceptService) Accept(opts AcceptOptions) (*AcceptResult, error) {
	if opts.VendorName == "" {
		return nil, fmt.Errorf("vendor name is required")
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Find the lock entry for the target vendor
	var lockEntry *types.LockDetails
	for i := range lock.Vendors {
		if lock.Vendors[i].Name == opts.VendorName {
			lockEntry = &lock.Vendors[i]
			break
		}
	}
	if lockEntry == nil {
		return nil, fmt.Errorf("vendor %q not found in lockfile", opts.VendorName)
	}

	if opts.Clear {
		return s.clearDrift(lock, lockEntry, opts)
	}
	return s.acceptDrift(lock, lockEntry, opts)
}

// acceptDrift computes local hashes for files with lock mismatches and writes
// accepted_drift entries. Only files whose actual hash differs from the upstream
// file_hashes hash are accepted — already-matching files are skipped.
func (s *AcceptService) acceptDrift(lock types.VendorLock, lockEntry *types.LockDetails, opts AcceptOptions) (*AcceptResult, error) {
	if lockEntry.FileHashes == nil || len(lockEntry.FileHashes) == 0 {
		return nil, fmt.Errorf("vendor %q has no file hashes in lockfile", opts.VendorName)
	}

	if lockEntry.AcceptedDrift == nil {
		lockEntry.AcceptedDrift = make(map[string]string)
	}

	result := &AcceptResult{VendorName: opts.VendorName}

	// Determine which files to check
	filesToCheck := make(map[string]string) // path -> expected hash
	if opts.FilePath != "" {
		expectedHash, ok := lockEntry.FileHashes[opts.FilePath]
		if !ok {
			return nil, fmt.Errorf("file %q not found in vendor %q file hashes", opts.FilePath, opts.VendorName)
		}
		filesToCheck[opts.FilePath] = expectedHash
	} else {
		for path, hash := range lockEntry.FileHashes {
			filesToCheck[path] = hash
		}
	}

	// Check each file for drift and accept if mismatched
	for path, expectedHash := range filesToCheck {
		actualHash, err := s.cache.ComputeFileChecksum(path)
		if err != nil {
			return nil, fmt.Errorf("compute checksum for %s: %w", path, err)
		}

		// Only accept files that actually differ from upstream
		if actualHash != expectedHash {
			lockEntry.AcceptedDrift[path] = actualHash
			result.AcceptedFiles = append(result.AcceptedFiles, path)
		}
	}

	if len(result.AcceptedFiles) == 0 {
		return nil, fmt.Errorf("no modified files found for vendor %q", opts.VendorName)
	}

	if err := s.lockStore.Save(lock); err != nil {
		return nil, fmt.Errorf("save lockfile: %w", err)
	}

	return result, nil
}

// clearDrift removes accepted_drift entries for the vendor.
// When FilePath is set, only that entry is removed. Otherwise all entries are cleared.
func (s *AcceptService) clearDrift(lock types.VendorLock, lockEntry *types.LockDetails, opts AcceptOptions) (*AcceptResult, error) {
	if lockEntry.AcceptedDrift == nil || len(lockEntry.AcceptedDrift) == 0 {
		return nil, fmt.Errorf("vendor %q has no accepted drift entries to clear", opts.VendorName)
	}

	result := &AcceptResult{VendorName: opts.VendorName}

	if opts.FilePath != "" {
		if _, ok := lockEntry.AcceptedDrift[opts.FilePath]; !ok {
			return nil, fmt.Errorf("file %q has no accepted drift entry for vendor %q", opts.FilePath, opts.VendorName)
		}
		result.ClearedFiles = append(result.ClearedFiles, opts.FilePath)
		delete(lockEntry.AcceptedDrift, opts.FilePath)
	} else {
		for path := range lockEntry.AcceptedDrift {
			result.ClearedFiles = append(result.ClearedFiles, path)
		}
		lockEntry.AcceptedDrift = nil
	}

	if err := s.lockStore.Save(lock); err != nil {
		return nil, fmt.Errorf("save lockfile: %w", err)
	}

	return result, nil
}
