package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// FileCopyServiceInterface defines the contract for copying files according to path mappings.
type FileCopyServiceInterface interface {
	CopyMappings(tempDir string, vendor *types.VendorSpec, spec types.BranchSpec) (CopyStats, error)
}

// Compile-time interface satisfaction check.
var _ FileCopyServiceInterface = (*FileCopyService)(nil)

// FileCopyService handles copying files according to path mappings
type FileCopyService struct {
	fs FileSystem
}

// NewFileCopyService creates a new FileCopyService
func NewFileCopyService(fs FileSystem) *FileCopyService {
	return &FileCopyService{
		fs: fs,
	}
}

// CopyMappings copies all files according to path mappings for a vendor spec.
// Security: CopyMappings validates all destination paths via ValidateDestPath
// in copyMapping before any file I/O occurs.
func (s *FileCopyService) CopyMappings(tempDir string, vendor *types.VendorSpec, spec types.BranchSpec) (CopyStats, error) {
	var totalStats CopyStats

	for _, mapping := range spec.Mapping {
		stats, err := s.copyMapping(tempDir, vendor, spec, mapping)
		if err != nil {
			return totalStats, err
		}
		totalStats.Add(stats)
	}

	return totalStats, nil
}

// copyMapping copies a single path mapping
func (s *FileCopyService) copyMapping(tempDir string, vendor *types.VendorSpec, spec types.BranchSpec, mapping types.PathMapping) (CopyStats, error) {
	// Parse position specifiers from source and destination paths
	srcRaw := s.cleanSourcePath(mapping.From, spec.Ref)
	srcFile, srcPos, err := types.ParsePathPosition(srcRaw)
	if err != nil {
		return CopyStats{}, fmt.Errorf("invalid source position in mapping for %s: %w", vendor.Name, err)
	}

	srcPath := filepath.Join(tempDir, srcFile)

	// Compute destination path (strip position for path computation, parse position separately)
	destRaw := s.computeDestPath(mapping, spec, vendor)
	destFile, destPos, err := types.ParsePathPosition(destRaw)
	if err != nil {
		return CopyStats{}, fmt.Errorf("invalid destination position in mapping for %s: %w", vendor.Name, err)
	}

	// Validate destination path to prevent path traversal attacks
	if err := ValidateDestPath(destFile); err != nil {
		return CopyStats{}, err
	}

	// Position extraction mode: extract specific lines/columns from source
	if srcPos != nil {
		return s.copyWithPosition(srcPath, destFile, srcPos, destPos, vendor.Name, spec.Ref, srcFile, mapping.From, mapping.To)
	}

	// Standard copy (no position specifier) — existing behavior
	info, err := s.fs.Stat(srcPath)
	if err != nil {
		// VFY-003: When source file is missing during sync, handle gracefully
		// instead of aborting. Delete the local copy if it exists and record
		// the removal so the caller can prune the lock's FileHashes.
		return s.handleMissingSource(destFile, srcFile, vendor.Name, spec.Ref)
	}

	if info.IsDir() {
		if err := s.fs.MkdirAll(destFile, 0755); err != nil {
			return CopyStats{}, err
		}
		stats, err := s.fs.CopyDir(srcPath, destFile)
		if err != nil {
			return CopyStats{}, fmt.Errorf("failed to copy directory %s to %s: %w", srcPath, destFile, err)
		}
		return stats, nil
	}

	if err := s.fs.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return CopyStats{}, err
	}

	// SEC-023: Check for binary content in whole-file copies and emit advisory warning.
	// Binary files are allowed (user chose to vendor them) but get a warning to surface
	// the fact. Uses the same null-byte heuristic as position extraction (first 8000 bytes).
	var warnings []string
	if srcData, readErr := os.ReadFile(srcPath); readErr == nil && IsBinaryContent(srcData) {
		warnings = append(warnings, fmt.Sprintf("%s appears to be a binary file", srcFile))
	}

	stats, err := s.fs.CopyFile(srcPath, destFile)
	if err != nil {
		return CopyStats{}, fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destFile, err)
	}
	stats.Warnings = warnings
	return stats, nil
}

// copyWithPosition handles position-based extraction and placement.
func (s *FileCopyService) copyWithPosition(srcPath, destFile string, srcPos, destPos *types.PositionSpec, vendorName, ref, srcClean string, fromRaw, toRaw string) (CopyStats, error) {
	// Extract content from source at the specified position
	content, hash, err := ExtractPosition(srcPath, srcPos)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// VFY-003: Handle missing source in position extraction the same
			// way as whole-file copy — remove local dest and continue.
			return s.handleMissingSource(destFile, srcClean, vendorName, ref)
		}
		return CopyStats{}, fmt.Errorf("extract position from %s: %w", srcClean, err)
	}

	// Ensure destination directory exists
	if err := s.fs.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return CopyStats{}, err
	}

	// Check for local modifications that will be overwritten
	var warnings []string
	if w := s.checkLocalModifications(destFile, destPos, content); w != "" {
		warnings = append(warnings, w)
	}

	// Place content at destination
	if err := PlaceContent(destFile, content, destPos); err != nil {
		return CopyStats{}, fmt.Errorf("place content at %s: %w", destFile, err)
	}

	stats := CopyStats{
		FileCount: 1,
		ByteCount: int64(len(content)),
		Positions: []positionRecord{{
			From:       fromRaw,
			To:         toRaw,
			SourceHash: hash,
		}},
		Warnings: warnings,
	}
	return stats, nil
}

// checkLocalModifications detects if the destination has been modified since last sync.
// Returns a warning message if modifications are detected, empty string otherwise.
func (s *FileCopyService) checkLocalModifications(destFile string, destPos *types.PositionSpec, incomingContent string) string {
	if destPos != nil {
		// Destination has a position — compare just that range
		existing, _, err := ExtractPosition(destFile, destPos)
		if err != nil {
			return "" // File doesn't exist yet or range invalid — no warning needed
		}
		if existing != incomingContent {
			return fmt.Sprintf("%s has local modifications at target position that will be overwritten", destFile)
		}
	} else {
		// Destination is whole-file — compare entire content
		// Normalize CRLF to match incoming content (which was CRLF-normalized during extraction)
		data, err := os.ReadFile(destFile)
		if err != nil {
			return "" // File doesn't exist yet — no warning needed
		}
		if normalizeCRLF(string(data)) != incomingContent {
			return fmt.Sprintf("%s has local modifications that will be overwritten", destFile)
		}
	}
	return ""
}

// handleMissingSource handles the case where an upstream source file no longer exists.
// handleMissingSource deletes the local destination file (if present), emits a warning,
// and returns a CopyStats with the destination path in the Removed list so the caller
// can prune the lockfile's FileHashes. This prevents a single upstream deletion from
// aborting the entire sync operation (VFY-003).
func (s *FileCopyService) handleMissingSource(destFile, srcFile, vendorName, ref string) (CopyStats, error) {
	warning := fmt.Sprintf("upstream file %s removed from %s@%s", srcFile, vendorName, ref)

	// Delete the local copy if it exists; ignore errors if already gone
	if err := s.fs.Remove(destFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Non-trivial removal error (e.g., permission denied) — warn but continue
		warning += fmt.Sprintf(" (local delete failed: %v)", err)
	}

	return CopyStats{
		Removed:  []string{destFile},
		Warnings: []string{warning},
	}, nil
}

// cleanSourcePath removes blob/tree prefixes from source path
func (s *FileCopyService) cleanSourcePath(path, ref string) string {
	clean := strings.Replace(path, "blob/"+ref+"/", "", 1)
	clean = strings.Replace(clean, "tree/"+ref+"/", "", 1)
	return clean
}

// computeDestPath computes the destination path for a mapping.
// If the destination has a position specifier, it is preserved in the returned string.
func (s *FileCopyService) computeDestPath(mapping types.PathMapping, spec types.BranchSpec, vendor *types.VendorSpec) string {
	destPath := mapping.To

	// Use auto-path computation if destination not explicitly specified
	if destPath == "" || destPath == "." {
		srcClean := s.cleanSourcePath(mapping.From, spec.Ref)
		// Strip position from source before computing auto-path
		srcFile, _, err := types.ParsePathPosition(srcClean)
		if err != nil {
			srcFile = srcClean // Fallback to raw path if position parsing fails
		}
		destPath = ComputeAutoPath(srcFile, spec.DefaultTarget, vendor.Name)
	}

	return destPath
}
