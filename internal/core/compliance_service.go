package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ComplianceOptions configures compliance check and propagation behavior.
type ComplianceOptions struct {
	VendorName string // Empty = all internal vendors
	DryRun     bool
	Reverse    bool // Apply dest->source for source-canonical mode
}

// ComplianceServiceInterface defines the contract for internal vendor compliance operations.
type ComplianceServiceInterface interface {
	// Check computes drift direction for all internal vendor mappings.
	Check(opts ComplianceOptions) (*types.ComplianceResult, error)
	// Propagate performs Check, then copies changed files per compliance mode.
	Propagate(opts ComplianceOptions) (*types.ComplianceResult, error)
}

// Compile-time interface satisfaction check.
var _ ComplianceServiceInterface = (*ComplianceService)(nil)

// ComplianceService handles drift detection and propagation for internal vendors.
type ComplianceService struct {
	configStore ConfigStore
	lockStore   LockStore
	cache       CacheStore
	fs          FileSystem
	rootDir     string
}

// NewComplianceService creates a new ComplianceService.
func NewComplianceService(
	configStore ConfigStore,
	lockStore LockStore,
	cache CacheStore,
	fs FileSystem,
	rootDir string,
) *ComplianceService {
	return &ComplianceService{
		configStore: configStore,
		lockStore:   lockStore,
		cache:       cache,
		fs:          fs,
		rootDir:     rootDir,
	}
}

// Check computes current drift state for all internal vendor mappings.
func (s *ComplianceService) Check(opts ComplianceOptions) (*types.ComplianceResult, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	result := &types.ComplianceResult{
		SchemaVersion: "1.0",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Entries:       make([]types.ComplianceEntry, 0),
	}

	// Build config map for compliance mode lookup
	configMap := make(map[string]types.VendorSpec)
	for _, v := range config.Vendors {
		if v.Source == SourceInternal {
			configMap[v.Name] = v
		}
	}

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		if lockEntry.Source != SourceInternal {
			continue
		}
		if opts.VendorName != "" && lockEntry.Name != opts.VendorName {
			continue
		}

		vendorConfig, exists := configMap[lockEntry.Name]
		if !exists {
			continue
		}

		compliance := vendorConfig.Direction
		if compliance == "" {
			compliance = ComplianceSourceCanonical
		}

		entries := s.checkLockEntry(lockEntry, compliance)
		result.Entries = append(result.Entries, entries...)
	}

	// Compute summary
	result.Summary = s.computeSummary(result.Entries)
	return result, nil
}

// Propagate checks for drift and copies files according to compliance rules.
func (s *ComplianceService) Propagate(opts ComplianceOptions) (*types.ComplianceResult, error) {
	result, err := s.Check(opts)
	if err != nil {
		return nil, err
	}

	// Process each drifted entry
	var propagationErrors []string
	for i := range result.Entries {
		entry := &result.Entries[i]
		if entry.Direction == types.DriftSynced {
			continue
		}

		if entry.Direction == types.DriftBothDrift {
			propagationErrors = append(propagationErrors,
				NewComplianceConflictError(entry.VendorName, entry.FromPath, entry.ToPath).Error())
			continue
		}

		if opts.DryRun {
			entry.Action = fmt.Sprintf("would %s", entry.Action)
			continue
		}

		if err := s.propagateEntry(entry, opts); err != nil {
			propagationErrors = append(propagationErrors, err.Error())
		}
	}

	if len(propagationErrors) > 0 {
		return result, fmt.Errorf("propagation errors:\n  %s", strings.Join(propagationErrors, "\n  "))
	}

	// Update lockfile after successful propagation
	if !opts.DryRun {
		if err := s.updateLockfileHashes(opts); err != nil {
			return result, fmt.Errorf("update lockfile after propagation: %w", err)
		}
	}

	// Recompute summary after propagation
	result.Summary = s.computeSummary(result.Entries)
	return result, nil
}

// checkLockEntry computes drift entries for a single internal lockfile entry.
func (s *ComplianceService) checkLockEntry(lockEntry *types.LockDetails, compliance string) []types.ComplianceEntry {
	var entries []types.ComplianceEntry

	for srcPath, lockedSrcHash := range lockEntry.SourceFileHashes {
		currentSrcHash, srcErr := s.cache.ComputeFileChecksum(srcPath)
		sourceDrifted := srcErr != nil || currentSrcHash != lockedSrcHash

		for destPath, lockedDestHash := range lockEntry.FileHashes {
			currentDestHash, destErr := s.cache.ComputeFileChecksum(destPath)
			destDrifted := destErr != nil || currentDestHash != lockedDestHash

			entry := types.ComplianceEntry{
				VendorName:        lockEntry.Name,
				FromPath:          srcPath,
				ToPath:            destPath,
				SyncDirection:     compliance,
				SourceHashLocked:  lockedSrcHash,
				SourceHashCurrent: hashOrError(currentSrcHash, srcErr),
				DestHashLocked:    lockedDestHash,
				DestHashCurrent:   hashOrError(currentDestHash, destErr),
			}

			switch {
			case !sourceDrifted && !destDrifted:
				entry.Direction = types.DriftSynced
				entry.Action = "none"
			case sourceDrifted && !destDrifted:
				entry.Direction = types.DriftSourceDrift
				entry.Action = "propagate source → dest"
			case !sourceDrifted && destDrifted:
				entry.Direction = types.DriftDestDrift
				if compliance == ComplianceBidirectional {
					entry.Action = "propagate dest → source"
				} else {
					entry.Action = "warning: dest modified (source-canonical)"
				}
			default:
				entry.Direction = types.DriftBothDrift
				entry.Action = "conflict: manual resolution required"
			}

			entries = append(entries, entry)
		}
	}

	return entries
}

// propagateEntry copies a file based on drift direction and compliance mode.
// After copying, propagateEntry calls updatePositionSpecs if line count changed.
func (s *ComplianceService) propagateEntry(entry *types.ComplianceEntry, opts ComplianceOptions) error {
	var src, dest string

	switch entry.Direction {
	case types.DriftSourceDrift:
		src, dest = entry.FromPath, entry.ToPath
	case types.DriftDestDrift:
		if entry.SyncDirection == ComplianceBidirectional || opts.Reverse {
			src, dest = entry.ToPath, entry.FromPath
		} else {
			// Source-canonical without --reverse: warn only, no copy
			fmt.Printf("  ⚠ %s: destination %s modified (source-canonical mode, use --reverse to apply)\n",
				entry.VendorName, entry.ToPath)
			return nil
		}
	default:
		return nil
	}

	// Read old dest content for line count comparison
	oldData, _ := os.ReadFile(dest)
	oldLineCount := countLinesBytes(oldData)

	if err := s.copyFile(src, dest); err != nil {
		return err
	}

	// Read new dest content for line count comparison
	newData, _ := os.ReadFile(dest)
	newLineCount := countLinesBytes(newData)

	// Auto-update position specs if line count changed
	if oldLineCount != newLineCount {
		if err := s.updatePositionSpecs(entry.VendorName, dest, oldLineCount, newLineCount); err != nil {
			return fmt.Errorf("auto-update position specs for %s: %w", dest, err)
		}
	}

	return nil
}

// countLinesBytes returns the number of lines in data (split by \n).
// Empty data returns 0. Data without a trailing newline still counts the last line.
func countLinesBytes(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := 1
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}

// copyFile copies a file from src to dest, creating parent directories.
func (s *ComplianceService) copyFile(src, dest string) error {
	// Read source content
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source %s: %w", src, err)
	}

	// Ensure destination directory exists
	if mkErr := s.fs.MkdirAll(filepath.Dir(dest), 0755); mkErr != nil {
		return mkErr
	}

	// Write to destination
	if writeErr := os.WriteFile(dest, data, 0644); writeErr != nil {
		return fmt.Errorf("write destination %s: %w", dest, writeErr)
	}

	fmt.Printf("  → %s → %s (propagated)\n", src, dest)
	return nil
}

// updateLockfileHashes recomputes source and dest hashes after propagation.
func (s *ComplianceService) updateLockfileHashes(opts ComplianceOptions) error {
	lock, err := s.lockStore.Load()
	if err != nil {
		return err
	}

	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		if lockEntry.Source != SourceInternal {
			continue
		}
		if opts.VendorName != "" && lockEntry.Name != opts.VendorName {
			continue
		}

		// Recompute source file hashes
		for srcPath := range lockEntry.SourceFileHashes {
			hash, hashErr := s.cache.ComputeFileChecksum(srcPath)
			if hashErr == nil {
				lockEntry.SourceFileHashes[srcPath] = hash
			}
		}

		// Recompute dest file hashes
		for destPath := range lockEntry.FileHashes {
			hash, hashErr := s.cache.ComputeFileChecksum(destPath)
			if hashErr == nil {
				lockEntry.FileHashes[destPath] = hash
			}
		}

		lockEntry.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	}

	return s.lockStore.Save(lock)
}

// computeSummary aggregates compliance entries into a summary.
func (s *ComplianceService) computeSummary(entries []types.ComplianceEntry) types.ComplianceSummary {
	summary := types.ComplianceSummary{
		Total: len(entries),
	}

	for _, e := range entries {
		switch e.Direction {
		case types.DriftSynced:
			summary.Synced++
		case types.DriftSourceDrift:
			summary.SourceDrift++
		case types.DriftDestDrift:
			summary.DestDrift++
		case types.DriftBothDrift:
			summary.BothDrift++
		}
	}

	switch {
	case summary.BothDrift > 0:
		summary.Result = "CONFLICT"
	case summary.SourceDrift > 0 || summary.DestDrift > 0:
		summary.Result = "DRIFTED"
	default:
		summary.Result = "SYNCED"
	}

	return summary
}

// updatePositionSpecs adjusts line-range position specifiers in vendor.yml
// when propagation changes the line count of a destination file.
//
// Scope limitations (documented):
//   - ToEOF specs: no update needed (auto-expand)
//   - Single-line specs: no update needed
//   - Column specs: NOT auto-updated (too complex, documented as limitation)
//
// updatePositionSpecs loads the config, finds the matching mapping for vendorName,
// adjusts the EndLine of any line-range position spec by the delta, and saves.
func (s *ComplianceService) updatePositionSpecs(vendorName string, path string, oldLineCount, newLineCount int) error {
	if oldLineCount == newLineCount {
		return nil
	}

	delta := newLineCount - oldLineCount

	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config for position update: %w", err)
	}

	modified := false
	for vi := range config.Vendors {
		v := &config.Vendors[vi]
		if v.Name != vendorName || v.Source != SourceInternal {
			continue
		}

		for si := range v.Specs {
			spec := &v.Specs[si]
			for mi := range spec.Mapping {
				mapping := &spec.Mapping[mi]

				// Check both From and To for position specs matching the path
				for _, target := range []*string{&mapping.From, &mapping.To} {
					filePath, pos, parseErr := types.ParsePathPosition(*target)
					if parseErr != nil || pos == nil {
						continue
					}

					// Only match the file that changed
					if filePath != path {
						continue
					}

					// Skip: ToEOF (auto-expands), single-line, column specs (too complex)
					if pos.ToEOF || pos.IsSingleLine() || pos.HasColumns() {
						continue
					}

					// Adjust EndLine by delta
					newEndLine := pos.EndLine + delta
					if newEndLine < pos.StartLine {
						return fmt.Errorf(
							"position auto-update for %s would make EndLine (%d) < StartLine (%d) after delta %d",
							*target, newEndLine, pos.StartLine, delta)
					}

					// Reconstruct position string
					*target = fmt.Sprintf("%s:L%d-L%d", filePath, pos.StartLine, newEndLine)
					modified = true
				}
			}
		}
	}

	if modified {
		return s.configStore.Save(config)
	}
	return nil
}

// hashOrError returns the hash string or an error description.
func hashOrError(hash string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return hash
}
