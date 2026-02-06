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
	// Clean the source path (remove blob/tree prefixes)
	srcClean := s.cleanSourcePath(mapping.From, spec.Ref)
	srcPath := filepath.Join(tempDir, srcClean)

	// Compute destination path
	destPath := s.computeDestPath(mapping, spec, vendor)

	// Validate destination path to prevent path traversal attacks
	if err := ValidateDestPath(destPath); err != nil {
		return CopyStats{}, err
	}

	// Check if source exists
	info, err := s.fs.Stat(srcPath)
	if err != nil {
		return CopyStats{}, NewPathNotFoundError(srcClean, vendor.Name, spec.Ref)
	}

	// Copy directory or file
	if info.IsDir() {
		if err := s.fs.MkdirAll(destPath, 0755); err != nil {
			return CopyStats{}, err
		}
		stats, err := s.fs.CopyDir(srcPath, destPath)
		if err != nil {
			return CopyStats{}, fmt.Errorf("failed to copy directory %s to %s: %w", srcPath, destPath, err)
		}
		return stats, nil
	}

	if err := s.fs.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return CopyStats{}, err
	}
	stats, err := s.fs.CopyFile(srcPath, destPath)
	if err != nil {
		return CopyStats{}, fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destPath, err)
	}
	return stats, nil
}

// cleanSourcePath removes blob/tree prefixes from source path
func (s *FileCopyService) cleanSourcePath(path, ref string) string {
	clean := strings.Replace(path, "blob/"+ref+"/", "", 1)
	clean = strings.Replace(clean, "tree/"+ref+"/", "", 1)
	return clean
}

// computeDestPath computes the destination path for a mapping
func (s *FileCopyService) computeDestPath(mapping types.PathMapping, spec types.BranchSpec, vendor *types.VendorSpec) string {
	destPath := mapping.To

	// Use auto-path computation if destination not explicitly specified
	if destPath == "" || destPath == "." {
		srcClean := s.cleanSourcePath(mapping.From, spec.Ref)
		destPath = ComputeAutoPath(srcClean, spec.DefaultTarget, vendor.Name)
	}

	return destPath
}
