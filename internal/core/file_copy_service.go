package core

import (
	"fmt"
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

// CopyMappings copies all files according to path mappings for a vendor spec
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
		return s.copyWithPosition(srcPath, destFile, srcPos, destPos, vendor.Name, spec.Ref, srcFile)
	}

	// Standard copy (no position specifier) — existing behavior
	info, err := s.fs.Stat(srcPath)
	if err != nil {
		return CopyStats{}, NewPathNotFoundError(srcFile, vendor.Name, spec.Ref)
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
	stats, err := s.fs.CopyFile(srcPath, destFile)
	if err != nil {
		return CopyStats{}, fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destFile, err)
	}
	return stats, nil
}

// copyWithPosition handles position-based extraction and placement.
func (s *FileCopyService) copyWithPosition(srcPath, destFile string, srcPos, destPos *types.PositionSpec, vendorName, ref, srcClean string) (CopyStats, error) {
	// Extract content from source at the specified position
	content, _, err := ExtractPosition(srcPath, srcPos)
	if err != nil {
		if strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "does not exist") {
			return CopyStats{}, NewPathNotFoundError(srcClean, vendorName, ref)
		}
		return CopyStats{}, fmt.Errorf("extract position from %s: %w", srcClean, err)
	}

	// Ensure destination directory exists
	if err := s.fs.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return CopyStats{}, err
	}

	// Place content at destination
	if err := PlaceContent(destFile, content, destPos); err != nil {
		return CopyStats{}, fmt.Errorf("place content at %s: %w", destFile, err)
	}

	return CopyStats{FileCount: 1, ByteCount: int64(len(content))}, nil
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
		srcFile, _, _ := types.ParsePathPosition(srcClean)
		destPath = ComputeAutoPath(srcFile, spec.DefaultTarget, vendor.Name)
	}

	return destPath
}
