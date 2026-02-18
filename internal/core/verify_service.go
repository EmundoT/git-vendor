package core

import (
	"context"
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
// VerifyServiceInterface enables mocking in tests and alternative verification strategies.
// ctx is accepted for cancellation support and future network-based verification.
type VerifyServiceInterface interface {
	Verify(ctx context.Context) (*types.VerifyResult, error)
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

// Verify checks all vendored files against the lockfile.
// ctx is accepted for cancellation support and future network-based verification.
func (s *VerifyService) Verify(_ context.Context) (*types.VerifyResult, error) {
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
	// Build map of accepted drift hashes (CLI-003): path -> accepted local hash
	acceptedDrift := make(map[string]string)

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
		for path, hash := range lockEntry.AcceptedDrift {
			acceptedDrift[path] = hash
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
					Type:         "file",
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
				Type:         "file",
				ExpectedHash: &expectedHash,
				ActualHash:   &actualHash,
			})
			result.Summary.Verified++
		} else if acceptedHash, ok := acceptedDrift[path]; ok && actualHash == acceptedHash {
			// File has accepted drift — local hash matches the accepted hash (CLI-003)
			result.Files = append(result.Files, types.FileStatus{
				Path:         path,
				Vendor:       &vendorName,
				Status:       "accepted",
				Type:         "file",
				ExpectedHash: &expectedHash,
				ActualHash:   &actualHash,
			})
			result.Summary.Accepted++
		} else {
			// File modified
			result.Files = append(result.Files, types.FileStatus{
				Path:         path,
				Vendor:       &vendorName,
				Status:       "modified",
				Type:         "file",
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

	// Verify internal vendor entries — compare source and destination hashes
	// to detect drift direction (Spec 070).
	s.verifyInternalEntries(lock, config, result)

	// Register position-destination files in expectedFiles so findAddedFiles
	// does not flag them as "added". Position entries are verified separately
	// by verifyPositions above; this loop runs after the whole-file verify loop
	// so the empty hash sentinel never triggers a false "modified" result.
	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		for _, pos := range lockEntry.Positions {
			destFile, _, parseErr := types.ParsePathPosition(pos.To)
			if parseErr != nil {
				continue
			}
			if _, exists := expectedFiles[destFile]; !exists {
				expectedFiles[destFile] = expectedFileInfo{
					vendor: lockEntry.Name,
					hash:   "",
				}
			}
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

	// Detect config/lock coherence issues (VFY-001)
	s.detectCoherenceIssues(config, lock, result)

	// Compute totals and result
	result.Summary.TotalFiles = len(result.Files)
	switch {
	case result.Summary.Modified > 0 || result.Summary.Deleted > 0:
		result.Summary.Result = "FAIL"
	case result.Summary.Added > 0 || result.Summary.Accepted > 0 || result.Summary.Stale > 0 || result.Summary.Orphaned > 0:
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
				// ComputeFileChecksum returns bare hex; normalize to "sha256:" prefix
				// to match SourceHash format from ExtractPosition.
				var hexHash string
				hexHash, err = s.cache.ComputeFileChecksum(destFile)
				if err == nil {
					actualHash = fmt.Sprintf("sha256:%s", hexHash)
				}
			}

			posDetail := &types.PositionDetail{
				From:       pos.From,
				To:         pos.To,
				SourceHash: pos.SourceHash,
			}

			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					result.Files = append(result.Files, types.FileStatus{
						Path:         displayPath,
						Vendor:       &vendorName,
						Status:       "deleted",
						Type:         "position",
						ExpectedHash: &pos.SourceHash,
						Position:     posDetail,
					})
					result.Summary.Deleted++
					continue
				}
				// Extraction error (e.g., position out of range) — treat as modified
				errStr := err.Error()
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       "modified",
					Type:         "position",
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &errStr,
					Position:     posDetail,
				})
				result.Summary.Modified++
				continue
			}

			if actualHash == pos.SourceHash {
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       "verified",
					Type:         "position",
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &actualHash,
					Position:     posDetail,
				})
				result.Summary.Verified++
			} else {
				result.Files = append(result.Files, types.FileStatus{
					Path:         displayPath,
					Vendor:       &vendorName,
					Status:       "modified",
					Type:         "position",
					ExpectedHash: &pos.SourceHash,
					ActualHash:   &actualHash,
					Position:     posDetail,
				})
				result.Summary.Modified++
			}
		}
	}
}

// verifyInternalEntries checks internal vendor mappings for source/dest drift.
// For each internal lockfile entry, verifyInternalEntries compares the current
// source and destination file hashes against the locked hashes to determine
// the drift direction: synced, source_drifted, dest_drifted, or both_drifted.
func (s *VerifyService) verifyInternalEntries(lock types.VendorLock, config types.VendorConfig, result *types.VerifyResult) {
	// Build vendor config map for compliance mode lookup
	configMap := make(map[string]types.VendorSpec)
	for _, v := range config.Vendors {
		configMap[v.Name] = v
	}

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		if lockEntry.Source != SourceInternal {
			continue
		}

		vendorConfig, exists := configMap[lockEntry.Name]
		if !exists {
			continue
		}

		compliance := vendorConfig.Compliance
		if compliance == "" {
			compliance = ComplianceSourceCanonical
		}

		// Check each source file hash
		for srcPath, lockedSrcHash := range lockEntry.SourceFileHashes {
			currentSrcHash, srcErr := s.cache.ComputeFileChecksum(srcPath)
			sourceDrifted := srcErr != nil || currentSrcHash != lockedSrcHash

			// Find matching destination files for this source
			for destPath, lockedDestHash := range lockEntry.FileHashes {
				currentDestHash, destErr := s.cache.ComputeFileChecksum(destPath)
				destDrifted := destErr != nil || currentDestHash != lockedDestHash

				var direction types.ComplianceDriftDirection
				var action string

				switch {
				case !sourceDrifted && !destDrifted:
					direction = types.DriftSynced
					action = "none"
				case sourceDrifted && !destDrifted:
					direction = types.DriftSourceDrift
					action = "propagate source → dest"
				case !sourceDrifted && destDrifted:
					direction = types.DriftDestDrift
					if compliance == ComplianceBidirectional {
						action = "propagate dest → source"
					} else {
						action = "warning: dest modified (source-canonical)"
					}
				default:
					direction = types.DriftBothDrift
					action = "conflict: manual resolution required"
				}

				currentSrcDisplay := currentSrcHash
				if srcErr != nil {
					currentSrcDisplay = "error: " + srcErr.Error()
				}
				currentDestDisplay := currentDestHash
				if destErr != nil {
					currentDestDisplay = "error: " + destErr.Error()
				}

				result.InternalStatus = append(result.InternalStatus, types.ComplianceEntry{
					VendorName:        lockEntry.Name,
					FromPath:          srcPath,
					ToPath:            destPath,
					Direction:         direction,
					Compliance:        compliance,
					SourceHashLocked:  lockedSrcHash,
					SourceHashCurrent: currentSrcDisplay,
					DestHashLocked:    lockedDestHash,
					DestHashCurrent:   currentDestDisplay,
					Action:            action,
				})
			}
		}
	}
}

// detectCoherenceIssues cross-references config mapping destinations against
// lock FileHashes to find two categories of incoherence:
//   - Stale: destination path in config mappings with no lock FileHashes entry
//     (config references files that were never synced or whose lock entry was removed)
//   - Orphaned: lock FileHashes entry with no corresponding config mapping destination
//     (lock has entries for files no longer referenced by any config mapping)
//
// Position specs (e.g., ":L5-L10") are stripped from config destination paths
// before comparison, since lock FileHashes keys are bare file paths.
// Internal vendor entries (Source == "internal") are excluded from orphan detection
// because their FileHashes track destination files keyed differently.
func (s *VerifyService) detectCoherenceIssues(config types.VendorConfig, lock types.VendorLock, result *types.VerifyResult) {
	// Build set of destination paths from config mappings.
	// Key: bare file path (position spec stripped). Value: vendor name.
	configDests := make(map[string]string)
	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				if mapping.To == "" {
					continue
				}
				destFile, _, parseErr := types.ParsePathPosition(mapping.To)
				if parseErr != nil {
					destFile = mapping.To
				}
				configDests[destFile] = vendor.Name
			}
		}
	}

	// Build set of all lock FileHashes paths across all vendors.
	// Key: file path. Value: vendor name.
	// Track which vendors have FileHashes populated (vs cache-fallback scenarios).
	lockPaths := make(map[string]string)
	vendorsWithHashes := make(map[string]bool)
	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		if len(lockEntry.FileHashes) > 0 {
			vendorsWithHashes[lockEntry.Name] = true
			for path := range lockEntry.FileHashes {
				lockPaths[path] = lockEntry.Name
			}
		}
	}

	// Skip coherence detection entirely when no lock vendors have FileHashes.
	// This happens during cache-fallback scenarios where the lock hasn't been
	// populated with hashes yet — coherence detection is not meaningful.
	if len(lockPaths) == 0 {
		return
	}

	// Stale: in config but not in lock.
	// Only flag a config dest as stale when its vendor has FileHashes populated
	// in the lock. If the vendor has no FileHashes, its entries were resolved
	// via cache fallback and stale detection would produce false positives.
	for destPath, vendorName := range configDests {
		if !vendorsWithHashes[vendorName] {
			continue
		}
		if _, inLock := lockPaths[destPath]; !inLock {
			vn := vendorName
			result.Files = append(result.Files, types.FileStatus{
				Path:   destPath,
				Vendor: &vn,
				Status: "stale",
				Type:   "coherence",
			})
			result.Summary.Stale++
		}
	}

	// Orphaned: in lock but not in config (skip internal vendors)
	internalVendors := make(map[string]bool)
	for i := range lock.Vendors {
		if lock.Vendors[i].Source == SourceInternal {
			internalVendors[lock.Vendors[i].Name] = true
		}
	}

	for lockPath, vendorName := range lockPaths {
		if internalVendors[vendorName] {
			continue
		}
		if _, inConfig := configDests[lockPath]; !inConfig {
			vn := vendorName
			result.Files = append(result.Files, types.FileStatus{
				Path:   lockPath,
				Vendor: &vn,
				Status: "orphaned",
				Type:   "coherence",
			})
			result.Summary.Orphaned++
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
			if err != nil {
				return fmt.Errorf("findAddedFiles: access %s: %w", path, err)
			}
			if d.IsDir() {
				return nil
			}

			// Check both OS-native path (from WalkDir) and forward-slash form
			// (lockfile/config paths use forward slashes on all platforms, but
			// filepath.WalkDir returns OS-native separators on Windows).
			_, inExpected := expectedFiles[path]
			if !inExpected {
				_, inExpected = expectedFiles[filepath.ToSlash(path)]
			}
			if !inExpected {
				// This is an added file
				hash, hashErr := s.cache.ComputeFileChecksum(path)
				var hashPtr *string
				if hashErr == nil {
					hashPtr = &hash
				}
				added = append(added, types.FileStatus{
					Path:       filepath.ToSlash(path),
					Vendor:     nil, // Unknown vendor for added files
					Status:     "added",
					Type:       "file",
					ActualHash: hashPtr,
				})
			}

			return nil
		})

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("findAddedFiles: walk %s: %w", destDir, err)
		}
	}

	return added, nil
}
