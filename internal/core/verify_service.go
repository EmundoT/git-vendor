package core

import (
	"errors"
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

// VerifyServiceInterface defines the contract for file verification against lockfile.
// This interface enables mocking in tests and potential alternative verification strategies.
type VerifyServiceInterface interface {
	Verify() (*types.VerifyResult, error)
}

// Compile-time interface satisfaction check.
var _ VerifyServiceInterface = (*VerifyService)(nil)

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

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
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
			if errors.Is(err, os.ErrNotExist) {
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

	// Verify position-extracted content against lockfile source hashes.
	// This is a local-only check: read the destination file, extract the
	// target range, hash it, and compare to the source_hash stored at sync time.
	s.verifyPositions(lock, result)

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

// verifyPositions checks position-extracted content against lockfile source hashes.
// For each PositionLock entry, verifyPositions reads the destination file locally,
// extracts the target range, and compares the computed hash to PositionLock.SourceHash.
// No network access required — purely local verification.
func (s *VerifyService) verifyPositions(lock types.VendorLock, result *types.VerifyResult) {
	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		for _, pos := range lockEntry.Positions {
			vendorName := lockEntry.Name

			// Parse destination path and position
			destFile, destPos, err := types.ParsePathPosition(pos.To)
			if err != nil {
				// If To is empty, fall back to parsing From for auto-path
				// (position verify only makes sense when we know where the content went)
				continue
			}

			// Determine what to verify:
			// - If destination has a position → extract that range and hash it
			// - If destination has no position → hash the whole file
			var actualHash string
			var displayPath string

			if destPos != nil {
				displayPath = pos.To
				_, actualHash, err = ExtractPosition(destFile, destPos)
			} else {
				displayPath = destFile
				actualHash, err = s.cache.ComputeFileChecksum(destFile)
			}

			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					result.Files = append(result.Files, types.FileStatus{
						Path:         displayPath,
						Vendor:       &vendorName,
						Status:       "deleted",
						ExpectedHash: &pos.SourceHash,
					})
					result.Summary.Deleted++
					continue
				}
				// Extraction error (e.g., position out of range) — treat as modified
				status := "modified"
				errStr := err.Error()
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       status,
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &errStr,
				})
				result.Summary.Modified++
				continue
			}

			if actualHash == pos.SourceHash {
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       "verified",
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &actualHash,
				})
				result.Summary.Verified++
			} else {
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       "modified",
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &actualHash,
				})
				result.Summary.Modified++
			}
		}
	}
}

// buildExpectedFilesFromCache builds expected files map from cache (fallback)
func (s *VerifyService) buildExpectedFilesFromCache(lock types.VendorLock) (map[string]expectedFileInfo, error) {
	expectedFiles := make(map[string]expectedFileInfo)

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
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

				// Strip position specifier from destination path for file system access
				destFile, _, parseErr := types.ParsePathPosition(destPath)
				if parseErr != nil {
					destFile = destPath
				}

				// Check if destFile is a directory or file
				info, err := s.fs.Stat(destFile)
				if err != nil {
					continue // Path doesn't exist
				}

				if info.IsDir() {
					destDirs[destFile] = true
				} else {
					// For files, add parent directory
					destDirs[filepath.Dir(destFile)] = true
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

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return added, nil
}
