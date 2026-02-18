package core

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/EmundoT/git-vendor/internal/types"
)

// PullOptions configures pull operation behavior.
// PullOptions merges update + sync into a single "get the latest" operation.
type PullOptions struct {
	Locked      bool   // Use existing lock hashes, don't fetch latest (old sync behavior)
	Prune       bool   // Remove dead mappings from vendor.yml when upstream file is missing
	KeepLocal   bool   // Skip overwriting locally modified files (lock hash mismatch)
	Interactive bool   // Prompt per-file on conflicts (deferred — prints message for now)
	Force       bool   // Skip cache, force re-fetch
	NoCache     bool   // Don't persist cache after pull
	VendorName  string // Empty = all vendors
	Local       bool   // Allow file:// and local path vendor URLs
	Commit      bool   // Auto-commit after pull with vendor trailers
}

// PullResult summarizes what a pull operation did.
type PullResult struct {
	Updated       int      // Vendors whose lock entries were refreshed
	Synced        int      // Vendors whose files were copied to disk
	FilesWritten  int      // Total files written
	FilesSkipped  int      // Files skipped due to --keep-local
	FilesRemoved  int      // Files removed (upstream deletion)
	MappingsPruned int     // Mappings removed from vendor.yml (--prune)
	Warnings      []string // Non-fatal warnings
}

// PullVendors performs the combined update+sync operation.
//
// Default flow (no --locked):
//  1. Update: fetch latest commit for each vendor ref, regenerate lock
//  2. Sync: copy locked files to disk
//
// With --locked:
//  1. Sync only: use existing lock hashes (deterministic rebuild)
//
// With --keep-local:
//  1. Before overwriting, check if local file hash matches lock hash
//  2. If mismatch (local modification detected), skip that file
//
// With --prune:
//  1. After sync, remove mappings from vendor.yml whose upstream source no longer exists
func (s *VendorSyncer) PullVendors(ctx context.Context, opts PullOptions) (*PullResult, error) {
	if opts.Interactive {
		fmt.Println("Note: --interactive mode is not yet implemented. Using default (overwrite) behavior.")
	}

	result := &PullResult{}

	// Phase 1: Update lock (unless --locked)
	if !opts.Locked {
		updateOpts := UpdateOptions{
			Local:      opts.Local,
			VendorName: opts.VendorName,
		}
		if err := s.update.UpdateAllWithOptions(ctx, updateOpts); err != nil {
			return nil, fmt.Errorf("pull update phase: %w", err)
		}
		// Count updated vendors
		lock, err := s.lockStore.Load()
		if err == nil {
			result.Updated = len(lock.Vendors)
			if opts.VendorName != "" {
				result.Updated = 0
				for _, l := range lock.Vendors {
					if l.Name == opts.VendorName {
						result.Updated++
					}
				}
			}
		}
	}

	// Phase 2: If --keep-local, snapshot local file hashes and back up modified files BEFORE sync
	var backups map[string]string
	if opts.KeepLocal {
		localHashes, err := s.snapshotLocalFileHashes(opts.VendorName)
		if err != nil {
			return nil, fmt.Errorf("snapshot local hashes: %w", err)
		}
		if len(localHashes) > 0 {
			backups, err = s.backupLocallyModified(localHashes)
			if err != nil {
				cleanupBackups(backups)
				return nil, fmt.Errorf("backup locally modified files: %w", err)
			}
		}
	}

	// Phase 3: Sync (lock → disk)
	syncOpts := SyncOptions{
		VendorName: opts.VendorName,
		Force:      opts.Force,
		NoCache:    opts.NoCache,
		Local:      opts.Local,
	}
	if err := s.syncWithAutoUpdate(ctx, syncOpts); err != nil {
		cleanupBackups(backups)
		return nil, fmt.Errorf("pull sync phase: %w", err)
	}

	// Phase 4: If --keep-local, restore backed-up locally modified files after sync
	if len(backups) > 0 {
		restored, err := s.restoreLocallyModified(backups)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("keep-local restore: %s", err))
		}
		result.FilesSkipped = restored
	}

	// Phase 5: Collect sync stats from lock and count upstream removals (I1).
	// Files in the lock's FileHashes that no longer exist on disk were removed
	// by the sync's upstream-removal handling (VFY-003 / CopyStats.Removed).
	lock, err := s.lockStore.Load()
	if err == nil {
		for _, l := range lock.Vendors {
			if opts.VendorName != "" && l.Name != opts.VendorName {
				continue
			}
			result.Synced++
			for destPath := range l.FileHashes {
				if _, statErr := os.Stat(destPath); errors.Is(statErr, os.ErrNotExist) {
					result.FilesRemoved++
				} else {
					result.FilesWritten++
				}
			}
		}
	}

	// Phase 6: If --prune, remove dead mappings from vendor.yml
	if opts.Prune {
		pruned, pruneWarnings, err := s.pruneDeadMappings(opts.VendorName)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("prune: %s", err))
		}
		result.MappingsPruned = pruned
		result.Warnings = append(result.Warnings, pruneWarnings...)
	}

	return result, nil
}

// snapshotLocalFileHashes captures current on-disk file hashes for all vendored files.
// snapshotLocalFileHashes returns a map of dest-path -> SHA-256 for files that exist on disk
// AND differ from their lock hash (i.e., locally modified).
func (s *VendorSyncer) snapshotLocalFileHashes(vendorName string) (map[string]string, error) {
	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, err
	}

	cache := NewFileCacheStore(s.fs, s.rootDir)
	modified := make(map[string]string)

	for _, l := range lock.Vendors {
		if vendorName != "" && l.Name != vendorName {
			continue
		}
		for destPath, lockHash := range l.FileHashes {
			currentHash, err := cache.ComputeFileChecksum(destPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue // File doesn't exist locally, nothing to preserve
				}
				continue // Can't read, skip
			}
			if currentHash != lockHash {
				// File was locally modified — record its current content hash
				modified[destPath] = currentHash
			}
		}
	}

	return modified, nil
}

// backupLocallyModified copies locally modified files to temporary locations
// before sync can overwrite them. Returns a map of destPath -> temp backup path
// for files that were backed up.
func (s *VendorSyncer) backupLocallyModified(modifiedPaths map[string]string) (map[string]string, error) {
	backups := make(map[string]string)
	for destPath := range modifiedPaths {
		data, err := os.ReadFile(destPath)
		if err != nil {
			continue // file may have been deleted between snapshot and backup
		}
		tmpFile, err := os.CreateTemp("", "keep-local-*")
		if err != nil {
			return backups, fmt.Errorf("create temp for keep-local backup of %s: %w", destPath, err)
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()
		if err := os.WriteFile(tmpPath, data, 0644); err != nil {
			return backups, fmt.Errorf("write keep-local backup for %s: %w", destPath, err)
		}
		backups[destPath] = tmpPath
	}
	return backups, nil
}

// restoreLocallyModified restores backed-up locally modified files after sync
// overwrote them. Returns count of files actually restored.
func (s *VendorSyncer) restoreLocallyModified(backups map[string]string) (int, error) {
	restored := 0
	for destPath, tmpPath := range backups {
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			continue // backup lost, nothing to restore
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return restored, fmt.Errorf("restore keep-local file %s: %w", destPath, err)
		}
		restored++
		fmt.Printf("  preserved: %s (locally modified, kept by --keep-local)\n", destPath)
		// Clean up temp file
		_ = os.Remove(tmpPath)
	}
	return restored, nil
}

// cleanupBackups removes any remaining temporary backup files.
func cleanupBackups(backups map[string]string) {
	for _, tmpPath := range backups {
		_ = os.Remove(tmpPath)
	}
}

// pruneDeadMappings removes mappings from vendor.yml where the source file no longer exists
// upstream (detected by the mapping not having a corresponding lock FileHashes entry after sync).
// pruneDeadMappings returns the count of pruned mappings and any warnings.
func (s *VendorSyncer) pruneDeadMappings(vendorName string) (int, []string, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return 0, nil, fmt.Errorf("load config for prune: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return 0, nil, fmt.Errorf("load lock for prune: %w", err)
	}

	// Build set of lock file hash keys per vendor+ref
	lockFileKeys := make(map[string]map[string]bool) // "vendor:ref" -> set of dest paths
	for _, l := range lock.Vendors {
		key := l.Name + ":" + l.Ref
		if lockFileKeys[key] == nil {
			lockFileKeys[key] = make(map[string]bool)
		}
		for destPath := range l.FileHashes {
			lockFileKeys[key][destPath] = true
		}
	}

	pruned := 0
	var warnings []string
	modified := false

	for vi := range config.Vendors {
		v := &config.Vendors[vi]
		if vendorName != "" && v.Name != vendorName {
			continue
		}

		for si := range v.Specs {
			spec := &v.Specs[si]
			key := v.Name + ":" + spec.Ref
			destKeys := lockFileKeys[key]

			var kept []types.PathMapping
			for _, m := range spec.Mapping {
				// Check if this mapping's destination exists in the lock
				if destKeys != nil && destKeys[m.To] {
					kept = append(kept, m)
				} else {
					// Mapping has no corresponding lock entry — source was removed upstream
					warnings = append(warnings, fmt.Sprintf("pruned: %s → %s (source no longer exists)", m.From, m.To))
					pruned++
					modified = true
				}
			}
			spec.Mapping = kept
		}
	}

	if modified {
		if err := s.configStore.Save(config); err != nil {
			return pruned, warnings, fmt.Errorf("save config after prune: %w", err)
		}
	}

	return pruned, warnings, nil
}
