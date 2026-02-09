package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ValidationServiceInterface defines the contract for config validation and conflict detection.
// This interface enables mocking in tests and potential alternative validation strategies.
type ValidationServiceInterface interface {
	ValidateConfig() error
	DetectConflicts() ([]types.PathConflict, error)
}

// Compile-time interface satisfaction check.
var _ ValidationServiceInterface = (*ValidationService)(nil)

// ValidationService handles config validation and conflict detection
type ValidationService struct {
	configStore ConfigStore
}

// NewValidationService creates a new ValidationService
func NewValidationService(configStore ConfigStore) *ValidationService {
	return &ValidationService{
		configStore: configStore,
	}
}

// ValidateConfig performs comprehensive configuration validation
func (s *ValidationService) ValidateConfig() error {
	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check for empty vendors
	if len(config.Vendors) == 0 {
		return fmt.Errorf("no vendors configured. Run 'git-vendor add' to add your first dependency")
	}

	// Check for duplicate vendor names
	names := make(map[string]bool)
	for _, vendor := range config.Vendors {
		if names[vendor.Name] {
			return fmt.Errorf("duplicate vendor name: %s", vendor.Name)
		}
		names[vendor.Name] = true

		// Validate vendor
		if err := s.validateVendor(&vendor); err != nil {
			return fmt.Errorf("ValidateConfig: %w", err)
		}
	}

	return nil
}

// validateVendor validates a single vendor spec
func (s *ValidationService) validateVendor(vendor *types.VendorSpec) error {
	// Validate vendor has URL
	if vendor.URL == "" {
		return fmt.Errorf("vendor %s has no URL", vendor.Name)
	}

	// Validate vendor has at least one spec
	if len(vendor.Specs) == 0 {
		return fmt.Errorf("vendor %s has no specs configured", vendor.Name)
	}

	// Validate each spec
	for _, spec := range vendor.Specs {
		if err := s.validateSpec(vendor.Name, spec); err != nil {
			return fmt.Errorf("validateVendor: %w", err)
		}
	}

	return nil
}

// validateSpec validates a single branch spec
func (s *ValidationService) validateSpec(vendorName string, spec types.BranchSpec) error {
	if spec.Ref == "" {
		return fmt.Errorf("vendor %s has a spec with no ref", vendorName)
	}

	if len(spec.Mapping) == 0 {
		return fmt.Errorf("vendor %s @ %s has no path mappings", vendorName, spec.Ref)
	}

	// Validate each mapping
	for _, mapping := range spec.Mapping {
		if mapping.From == "" {
			return fmt.Errorf("vendor %s @ %s has a mapping with empty 'from' path", vendorName, spec.Ref)
		}
	}

	return nil
}

// DetectConflicts checks for path conflicts between vendors
func (s *ValidationService) DetectConflicts() ([]types.PathConflict, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("DetectConflicts: load config: %w", err)
	}

	// Build path ownership map
	pathMap := s.buildPathOwnershipMap(config)

	// Detect exact path conflicts
	conflicts := s.detectExactPathConflicts(pathMap)

	// Detect overlapping path conflicts
	overlappingConflicts := s.detectOverlappingPathConflicts(pathMap)
	conflicts = append(conflicts, overlappingConflicts...)

	return conflicts, nil
}

// PathOwner tracks which vendor owns a path
type PathOwner struct {
	VendorName string
	Mapping    types.PathMapping
	Ref        string
}

// buildPathOwnershipMap builds a map of destination paths to vendors
func (s *ValidationService) buildPathOwnershipMap(config types.VendorConfig) map[string][]PathOwner {
	pathMap := make(map[string][]PathOwner)

	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				destPath := mapping.To

				// Use auto-path computation if destination not explicitly specified
				if destPath == "" || destPath == "." {
					// Strip position from source before auto-path computation
					srcFile, _, err := types.ParsePathPosition(mapping.From)
					if err != nil {
						srcFile = mapping.From
					}
					destPath = ComputeAutoPath(srcFile, spec.DefaultTarget, vendor.Name)
				}

				// Strip position specifier for conflict detection (compare file paths only)
				destFile, _, err := types.ParsePathPosition(destPath)
				if err != nil {
					destFile = destPath
				}

				// Normalize path
				destPath = filepath.Clean(destFile)

				pathMap[destPath] = append(pathMap[destPath], PathOwner{
					VendorName: vendor.Name,
					Mapping:    mapping,
					Ref:        spec.Ref,
				})
			}
		}
	}

	return pathMap
}

// detectExactPathConflicts detects when multiple vendors map to the same path
func (s *ValidationService) detectExactPathConflicts(pathMap map[string][]PathOwner) []types.PathConflict {
	var conflicts []types.PathConflict

	for path, owners := range pathMap {
		if len(owners) > 1 {
			// Multiple vendors map to the same path
			for i := 0; i < len(owners)-1; i++ {
				for j := i + 1; j < len(owners); j++ {
					conflicts = append(conflicts, types.PathConflict{
						Path:     path,
						Vendor1:  owners[i].VendorName,
						Vendor2:  owners[j].VendorName,
						Mapping1: owners[i].Mapping,
						Mapping2: owners[j].Mapping,
					})
				}
			}
		}
	}

	return conflicts
}

// detectOverlappingPathConflicts detects when one path is a subdirectory of another
func (s *ValidationService) detectOverlappingPathConflicts(pathMap map[string][]PathOwner) []types.PathConflict {
	var conflicts []types.PathConflict

	// Get all paths for comparison
	var allPaths []string
	for path := range pathMap {
		allPaths = append(allPaths, path)
	}

	// Compare all path pairs
	for i := 0; i < len(allPaths)-1; i++ {
		for j := i + 1; j < len(allPaths); j++ {
			path1 := allPaths[i]
			path2 := allPaths[j]

			// Check if one path is a subdirectory of another
			if isSubPath(path1, path2) {
				owners1 := pathMap[path1]
				owners2 := pathMap[path2]

				// Skip malformed entries (empty slices)
				if len(owners1) == 0 || len(owners2) == 0 {
					continue
				}

				// Only report if different vendors
				if owners1[0].VendorName != owners2[0].VendorName {
					conflicts = append(conflicts, types.PathConflict{
						Path:     fmt.Sprintf("%s overlaps with %s", path1, path2),
						Vendor1:  owners1[0].VendorName,
						Vendor2:  owners2[0].VendorName,
						Mapping1: owners1[0].Mapping,
						Mapping2: owners2[0].Mapping,
					})
				}
			}
		}
	}

	return conflicts
}

// isSubPath checks if path1 is a subdirectory of path2 or vice versa
func isSubPath(path1, path2 string) bool {
	path1 = filepath.Clean(path1)
	path2 = filepath.Clean(path2)

	// Check if path2 is under path1
	rel, err := filepath.Rel(path1, path2)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	// Check if path1 is under path2
	rel, err = filepath.Rel(path2, path1)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	return false
}
