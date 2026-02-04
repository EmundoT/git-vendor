package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// expectedFileInfo holds expected file metadata for verification
type expectedFileInfo struct {
	vendor string
	hash   string
}

// VerifyService handles verification of vendored files against lockfile
type VerifyService struct {
	configStore ConfigStore
	lockStore   LockStore
	cache       CacheStore
	fs          FileSystem
	rootDir     string
}

// NewVerifyService creates a new VerifyService
func NewVerifyService(
	configStore ConfigStore,
	lockStore LockStore,
	cache CacheStore,
	fs FileSystem,
	rootDir string,
) *VerifyService {
	return &VerifyService{
		configStore: configStore,
		lockStore:   lockStore,
		cache:       cache,
		fs:          fs,
		rootDir:     rootDir,
	}
}

// Verify checks all vendored files against the lockfile
func (s *VerifyService) Verify() (*types.VerifyResult, error) {
	// Load lockfile
	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Load config for destination paths
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result := &types.VerifyResult{
		SchemaVersion: "1.0",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Files:         make([]types.FileStatus, 0),
	}

	// Build map of expected files from lockfile
	expectedFiles := make(map[string]expectedFileInfo)

	for _, lockEntry := range lock.Vendors {
		if lockEntry.FileHashes != nil {
			for path, hash := range lockEntry.FileHashes {
				expectedFiles[path] = expectedFileInfo{
					vendor: lockEntry.Name,
					hash:   hash,
				}
			}
		}
	}

	// If lockfile has no file hashes, try to use cache as fallback
	if len(expectedFiles) == 0 {
		expectedFiles, err = s.buildExpectedFilesFromCache(lock)
		if err != nil {
			return nil, fmt.Errorf("no file hashes in lockfile and cache unavailable: %w", err)
		}
	}

	// Check all expected files
	for path, expected := range expectedFiles {
		vendorName := expected.vendor
		expectedHash := expected.hash

		// Check if file exists
		actualHash, err := s.cache.ComputeFileChecksum(path)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted
				result.Files = append(result.Files, types.FileStatus{
					Path:         path,
					Vendor:       &vendorName,
					Status:       "deleted",
					ExpectedHash: &expectedHash,
					ActualHash:   nil,
				})
				result.Summary.Deleted++
				continue
			}
			return nil, fmt.Errorf("hash file %s: %w", path, err)
		}

		if actualHash == expectedHash {
			// File verified
			result.Files = append(result.Files, types.FileStatus{
				Path:         path,
				Vendor:       &vendorName,
				Status:       "verified",
				ExpectedHash: &expectedHash,
				ActualHash:   &actualHash,
			})
			result.Summary.Verified++
		} else {
			// File modified
			result.Files = append(result.Files, types.FileStatus{
				Path:         path,
				Vendor:       &vendorName,
				Status:       "modified",
				ExpectedHash: &expectedHash,
				ActualHash:   &actualHash,
			})
			result.Summary.Modified++
		}
	}

	// Scan for added files (in vendor directories but not in lockfile)
	addedFiles, err := s.findAddedFiles(config, expectedFiles)
	if err != nil {
		return nil, fmt.Errorf("scan for added files: %w", err)
	}
	for _, af := range addedFiles {
		result.Files = append(result.Files, af)
		result.Summary.Added++
	}

	// Compute totals and result
	result.Summary.TotalFiles = len(result.Files)
	switch {
	case result.Summary.Modified > 0 || result.Summary.Deleted > 0:
		result.Summary.Result = "FAIL"
	case result.Summary.Added > 0:
		result.Summary.Result = "WARN"
	default:
		result.Summary.Result = "PASS"
	}

	return result, nil
}

// buildExpectedFilesFromCache builds expected files map from cache (fallback)
func (s *VerifyService) buildExpectedFilesFromCache(lock types.VendorLock) (map[string]expectedFileInfo, error) {
	expectedFiles := make(map[string]expectedFileInfo)

	for _, lockEntry := range lock.Vendors {
		// Load cache for this vendor@ref
		cache, err := s.cache.Load(lockEntry.Name, lockEntry.Ref)
		if err != nil {
			continue // Skip if cache not available
		}

		// Check if cache commit matches lockfile commit
		if cache.CommitHash != lockEntry.CommitHash {
			continue // Cache is stale
		}

		for _, fc := range cache.Files {
			expectedFiles[fc.Path] = expectedFileInfo{
				vendor: lockEntry.Name,
				hash:   fc.Hash,
			}
		}
	}

	if len(expectedFiles) == 0 {
		return nil, fmt.Errorf("no cached file hashes available")
	}

	return expectedFiles, nil
}

// findAddedFiles scans vendor destination directories for files not in lockfile
func (s *VerifyService) findAddedFiles(config types.VendorConfig, expectedFiles map[string]expectedFileInfo) ([]types.FileStatus, error) {
	var added []types.FileStatus

	// Collect all destination directories from config
	destDirs := make(map[string]bool)
	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				destPath := mapping.To
				if destPath == "" {
					// Auto-computed path - use vendor name as base
					destPath = filepath.Join("lib", vendor.Name)
				}

				// Check if destPath is a directory or file
				info, err := s.fs.Stat(destPath)
				if err != nil {
					continue // Path doesn't exist
				}

				if info.IsDir() {
					destDirs[destPath] = true
				} else {
					// For files, add parent directory
					destDirs[filepath.Dir(destPath)] = true
				}
			}
		}
	}

	// Walk each destination directory
	for destDir := range destDirs {
		err := filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			// Check if this file is in expected files
			if _, exists := expectedFiles[path]; !exists {
				// This is an added file
				hash, hashErr := s.cache.ComputeFileChecksum(path)
				var hashPtr *string
				if hashErr == nil {
					hashPtr = &hash
				}
				added = append(added, types.FileStatus{
					Path:       path,
					Vendor:     nil, // Unknown vendor for added files
					Status:     "added",
					ActualHash: hashPtr,
				})
			}

			return nil
		})

		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	return added, nil
}
