package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// InternalSyncServiceInterface defines the contract for syncing internal (same-repo) vendors.
// Internal vendors copy files from one location in the project to another,
// without any git clone operations.
type InternalSyncServiceInterface interface {
	// SyncInternalVendor syncs all mappings for an internal vendor.
	// Returns per-ref metadata (keyed by RefLocal), copy stats, and any error.
	SyncInternalVendor(v *types.VendorSpec, opts SyncOptions) (map[string]RefMetadata, CopyStats, error)
}

// Compile-time interface satisfaction check.
var _ InternalSyncServiceInterface = (*InternalSyncService)(nil)

// InternalSyncService handles syncing files within the same repository.
type InternalSyncService struct {
	configStore ConfigStore
	lockStore   LockStore
	fileCopy    FileCopyServiceInterface
	cache       CacheStore
	fs          FileSystem
	rootDir     string
}

// NewInternalSyncService creates a new InternalSyncService.
func NewInternalSyncService(
	configStore ConfigStore,
	lockStore LockStore,
	fileCopy FileCopyServiceInterface,
	cache CacheStore,
	fs FileSystem,
	rootDir string,
) *InternalSyncService {
	return &InternalSyncService{
		configStore: configStore,
		lockStore:   lockStore,
		fileCopy:    fileCopy,
		cache:       cache,
		fs:          fs,
		rootDir:     rootDir,
	}
}

// SyncInternalVendor syncs all mappings for an internal vendor by copying
// files from source to destination within the project.
func (s *InternalSyncService) SyncInternalVendor(v *types.VendorSpec, opts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
	results := make(map[string]RefMetadata)
	var totalStats CopyStats

	for _, spec := range v.Specs {
		metadata, stats, err := s.syncInternalRef(v, spec, opts)
		if err != nil {
			return nil, CopyStats{}, err
		}
		results[spec.Ref] = metadata
		totalStats.Add(stats)
	}

	return results, totalStats, nil
}

// syncInternalRef syncs a single ref for an internal vendor.
func (s *InternalSyncService) syncInternalRef(v *types.VendorSpec, spec types.BranchSpec, opts SyncOptions) (RefMetadata, CopyStats, error) {
	var totalStats CopyStats
	sourceHashes := make(map[string]string) // source path -> SHA-256

	for _, mapping := range spec.Mapping {
		stats, srcHash, err := s.syncInternalMapping(v.Name, mapping, opts)
		if err != nil {
			return RefMetadata{}, CopyStats{}, fmt.Errorf("internal sync %s mapping %s: %w", v.Name, mapping.From, err)
		}
		totalStats.Add(stats)

		// Track source file hash (file-level granularity, strip position)
		srcFile, _, parseErr := types.ParsePathPosition(mapping.From)
		if parseErr != nil {
			srcFile = mapping.From
		}
		sourceHashes[srcFile] = srcHash
	}

	// Build a content-addressed "commit hash" from sorted source hashes.
	// This enables canSkipSync cache to work for internal vendors.
	contentHash := s.computeContentHash(sourceHashes)

	metadata := RefMetadata{
		CommitHash: contentHash,
		Positions:  totalStats.Positions,
	}

	if !opts.DryRun {
		fmt.Printf("  ✓ %s (internal: synced %s)\n",
			v.Name,
			Pluralize(totalStats.FileCount, "file", "files"))
	}

	return metadata, totalStats, nil
}

// syncInternalMapping copies a single mapping from source to destination.
// Returns copy stats, the SHA-256 hash of the source file, and any error.
func (s *InternalSyncService) syncInternalMapping(vendorName string, mapping types.PathMapping, opts SyncOptions) (CopyStats, string, error) {
	// Parse source path and position
	srcFile, srcPos, err := types.ParsePathPosition(mapping.From)
	if err != nil {
		return CopyStats{}, "", fmt.Errorf("invalid source position: %w", err)
	}

	// Resolve source path relative to project root
	srcPath := srcFile
	if !filepath.IsAbs(srcPath) {
		srcPath = filepath.Join(".", srcPath)
	}

	// Compute source file hash for lockfile tracking
	srcHash, err := s.cache.ComputeFileChecksum(srcPath)
	if err != nil {
		return CopyStats{}, "", fmt.Errorf("compute source hash for %s: %w", srcFile, err)
	}

	if opts.DryRun {
		dest := mapping.To
		if dest == "" {
			dest = "(auto)"
		}
		fmt.Printf("    → %s → %s (internal)\n", mapping.From, dest)
		return CopyStats{FileCount: 1}, srcHash, nil
	}

	// Compute destination path
	destRaw := mapping.To
	if destRaw == "" {
		destRaw = ComputeAutoPath(srcFile, "", vendorName)
	}
	destFile, destPos, err := types.ParsePathPosition(destRaw)
	if err != nil {
		return CopyStats{}, "", fmt.Errorf("invalid destination position: %w", err)
	}

	// Validate destination path
	if err := ValidateDestPath(destFile); err != nil {
		return CopyStats{}, "", err
	}

	// Position extraction mode
	if srcPos != nil {
		content, hash, extractErr := ExtractPosition(srcPath, srcPos)
		if extractErr != nil {
			return CopyStats{}, "", fmt.Errorf("extract position from %s: %w", srcFile, extractErr)
		}

		// Ensure destination directory exists
		if mkErr := s.fs.MkdirAll(filepath.Dir(destFile), 0755); mkErr != nil {
			return CopyStats{}, "", mkErr
		}

		if placeErr := PlaceContent(destFile, content, destPos); placeErr != nil {
			return CopyStats{}, "", fmt.Errorf("place content at %s: %w", destFile, placeErr)
		}

		stats := CopyStats{
			FileCount: 1,
			ByteCount: int64(len(content)),
			Positions: []positionRecord{{
				From:       mapping.From,
				To:         mapping.To,
				SourceHash: hash,
			}},
		}
		return stats, srcHash, nil
	}

	// Standard copy (no position specifier)
	info, err := os.Stat(srcPath)
	if err != nil {
		return CopyStats{}, "", NewPathNotFoundError(srcFile, vendorName, RefLocal)
	}

	if info.IsDir() {
		if mkErr := s.fs.MkdirAll(destFile, 0755); mkErr != nil {
			return CopyStats{}, "", mkErr
		}
		stats, copyErr := s.fs.CopyDir(srcPath, destFile)
		if copyErr != nil {
			return CopyStats{}, "", fmt.Errorf("copy directory %s to %s: %w", srcPath, destFile, copyErr)
		}
		return stats, srcHash, nil
	}

	if mkErr := s.fs.MkdirAll(filepath.Dir(destFile), 0755); mkErr != nil {
		return CopyStats{}, "", mkErr
	}
	stats, copyErr := s.fs.CopyFile(srcPath, destFile)
	if copyErr != nil {
		return CopyStats{}, "", fmt.Errorf("copy file %s to %s: %w", srcPath, destFile, copyErr)
	}
	return stats, srcHash, nil
}

// computeContentHash computes a deterministic hash from sorted source file hashes.
// Used as the "commit hash" equivalent for internal vendors, enabling cache skip.
func (s *InternalSyncService) computeContentHash(sourceHashes map[string]string) string {
	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(sourceHashes))
	for k := range sourceHashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(":")
		sb.WriteString(sourceHashes[k])
		sb.WriteString("\n")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", hash)
}
