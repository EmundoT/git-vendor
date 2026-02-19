package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ValidationServiceInterface defines the contract for config validation and conflict detection.
// ValidationServiceInterface enables mocking in tests and alternative validation strategies.
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

	// Validate global compliance config (Spec 075)
	if config.Compliance != nil {
		if config.Compliance.Default != "" && config.Compliance.Default != EnforcementStrict &&
			config.Compliance.Default != EnforcementLenient && config.Compliance.Default != EnforcementInfo {
			return fmt.Errorf("compliance.default must be %q, %q, or %q",
				EnforcementStrict, EnforcementLenient, EnforcementInfo)
		}
		if config.Compliance.Mode != "" && config.Compliance.Mode != ComplianceModeDefault &&
			config.Compliance.Mode != ComplianceModeOverride {
			return fmt.Errorf("compliance.mode must be %q or %q",
				ComplianceModeDefault, ComplianceModeOverride)
		}
	}

	// Check for duplicate vendor names and validate name safety
	names := make(map[string]bool)
	for _, vendor := range config.Vendors {
		// SEC-001: Reject vendor names containing path traversal sequences.
		// Vendor names are used in filesystem paths (license files, cache files).
		if err := ValidateVendorName(vendor.Name); err != nil {
			return fmt.Errorf("vendor config rejected: %w", err)
		}

		if names[vendor.Name] {
			return fmt.Errorf("duplicate vendor name: %s", vendor.Name)
		}
		names[vendor.Name] = true

		// Route to internal or external validation
		if vendor.Source == SourceInternal {
			if err := s.validateInternalVendor(&vendor); err != nil {
				return fmt.Errorf("ValidateConfig: %w", err)
			}
		} else {
			if err := s.validateVendor(&vendor); err != nil {
				return fmt.Errorf("ValidateConfig: %w", err)
			}
		}
	}

	// Detect circular dependencies among internal vendors
	if err := s.detectInternalCycles(config); err != nil {
		return fmt.Errorf("ValidateConfig: %w", err)
	}

	return nil
}

// validateVendor validates a single vendor spec
func (s *ValidationService) validateVendor(vendor *types.VendorSpec) error {
	// Validate vendor has URL
	if vendor.URL == "" {
		return fmt.Errorf("vendor %s has no URL", vendor.Name)
	}

	// SEC-011: Reject dangerous URL schemes (file://, ftp://, etc.)
	if err := ValidateVendorURL(vendor.URL); err != nil {
		return fmt.Errorf("vendor %s: %w", vendor.Name, err)
	}

	// Validate mirror URLs
	for i, mirror := range vendor.Mirrors {
		if mirror == "" {
			return fmt.Errorf("vendor %s: mirror[%d] is empty", vendor.Name, i)
		}
		if mirror == vendor.URL {
			return fmt.Errorf("vendor %s: mirror[%d] duplicates primary URL", vendor.Name, i)
		}
		if err := ValidateVendorURL(mirror); err != nil {
			return fmt.Errorf("vendor %s: mirror[%d]: %w", vendor.Name, i, err)
		}
	}

	// Validate per-vendor enforcement level (Spec 075)
	if vendor.Enforcement != "" && vendor.Enforcement != EnforcementStrict &&
		vendor.Enforcement != EnforcementLenient && vendor.Enforcement != EnforcementInfo {
		// Detect stale Spec 070 values that used the same YAML key for sync direction
		if vendor.Enforcement == ComplianceBidirectional || vendor.Enforcement == ComplianceSourceCanonical {
			return NewValidationError(vendor.Name, "", "compliance",
				fmt.Sprintf("compliance %q is a sync direction (Spec 070), not an enforcement level (Spec 075); use the 'direction' YAML key for %q and 'compliance' for %q/%q/%q",
					vendor.Enforcement, vendor.Enforcement, EnforcementStrict, EnforcementLenient, EnforcementInfo))
		}
		return NewValidationError(vendor.Name, "", "compliance",
			fmt.Sprintf("compliance must be empty, %q, %q, or %q",
				EnforcementStrict, EnforcementLenient, EnforcementInfo))
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

// validateInternalVendor validates a vendor with Source="internal".
// Internal vendors MUST NOT have URL, License, or Hooks; MUST use Ref="local".
func (s *ValidationService) validateInternalVendor(vendor *types.VendorSpec) error {
	if vendor.URL != "" {
		return NewValidationError(vendor.Name, "", "url", "internal vendors MUST NOT have a URL")
	}
	if len(vendor.Mirrors) > 0 {
		return NewValidationError(vendor.Name, "", "mirrors", "internal vendors MUST NOT have mirrors")
	}
	if vendor.License != "" {
		return NewValidationError(vendor.Name, "", "license", "internal vendors MUST NOT have a license")
	}
	if vendor.Hooks != nil {
		return NewValidationError(vendor.Name, "", "hooks", "internal vendors MUST NOT have hooks")
	}
	if vendor.Direction != "" && vendor.Direction != ComplianceSourceCanonical && vendor.Direction != ComplianceBidirectional {
		return NewValidationError(vendor.Name, "", "direction",
			fmt.Sprintf("direction must be empty, %q, or %q", ComplianceSourceCanonical, ComplianceBidirectional))
	}
	// Validate per-vendor enforcement level (Spec 075)
	if vendor.Enforcement != "" && vendor.Enforcement != EnforcementStrict &&
		vendor.Enforcement != EnforcementLenient && vendor.Enforcement != EnforcementInfo {
		// Detect stale Spec 070 values that used the same YAML key for sync direction
		if vendor.Enforcement == ComplianceBidirectional || vendor.Enforcement == ComplianceSourceCanonical {
			return NewValidationError(vendor.Name, "", "compliance",
				fmt.Sprintf("compliance %q is a sync direction (Spec 070), not an enforcement level (Spec 075); use the 'direction' YAML key for %q and 'compliance' for %q/%q/%q",
					vendor.Enforcement, vendor.Enforcement, EnforcementStrict, EnforcementLenient, EnforcementInfo))
		}
		return NewValidationError(vendor.Name, "", "compliance",
			fmt.Sprintf("compliance must be empty, %q, %q, or %q",
				EnforcementStrict, EnforcementLenient, EnforcementInfo))
	}
	if len(vendor.Specs) == 0 {
		return fmt.Errorf("vendor %s has no specs configured", vendor.Name)
	}
	for _, spec := range vendor.Specs {
		if spec.Ref != RefLocal {
			return NewValidationError(vendor.Name, spec.Ref, "ref",
				fmt.Sprintf("internal vendors MUST use ref %q", RefLocal))
		}
		if len(spec.Mapping) == 0 {
			return fmt.Errorf("vendor %s @ %s has no path mappings", vendor.Name, spec.Ref)
		}
		for _, mapping := range spec.Mapping {
			if mapping.From == "" {
				return fmt.Errorf("vendor %s @ %s has a mapping with empty 'from' path", vendor.Name, spec.Ref)
			}
			// Strip position specifier before checking file existence
			srcFile, _, err := types.ParsePathPosition(mapping.From)
			if err != nil {
				srcFile = mapping.From
			}
			if _, statErr := os.Stat(srcFile); statErr != nil {
				return NewValidationError(vendor.Name, spec.Ref, "mapping.from",
					fmt.Sprintf("source file %q does not exist", srcFile))
			}
		}
	}
	return nil
}

// detectInternalCycles builds a directed graph from internal vendor mappings
// and checks for cycles via DFS. Returns CycleError if a cycle is found.
// Uses file-level granularity (positions stripped) because any write to a file
// invalidates all position reads from that file.
func (s *ValidationService) detectInternalCycles(config types.VendorConfig) error {
	// Build adjacency list: source file -> destination files
	graph := make(map[string][]string)
	for _, vendor := range config.Vendors {
		if vendor.Source != SourceInternal {
			continue
		}
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				fromFile, _, err := types.ParsePathPosition(mapping.From)
				if err != nil {
					fromFile = mapping.From
				}
				fromFile = filepath.Clean(fromFile)

				toFile := mapping.To
				if toFile == "" {
					continue // Auto-named paths can't form cycles with source files
				}
				toClean, _, err := types.ParsePathPosition(toFile)
				if err != nil {
					toClean = toFile
				}
				toClean = filepath.Clean(toClean)

				graph[fromFile] = append(graph[fromFile], toClean)
			}
		}
	}

	if len(graph) == 0 {
		return nil
	}

	// DFS cycle detection
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully explored
	)
	color := make(map[string]int)
	parent := make(map[string]string)

	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray
		for _, neighbor := range graph[node] {
			if color[neighbor] == gray {
				// Reconstruct cycle path
				cycle := []string{neighbor, node}
				cur := node
				for cur != neighbor {
					cur = parent[cur]
					if cur == "" {
						break
					}
					cycle = append(cycle, cur)
				}
				// Reverse to get forward order
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return cycle
			}
			if color[neighbor] == white {
				parent[neighbor] = node
				if cycle := dfs(neighbor); cycle != nil {
					return cycle
				}
			}
		}
		color[node] = black
		return nil
	}

	for node := range graph {
		if color[node] == white {
			if cycle := dfs(node); cycle != nil {
				return NewCycleError(cycle)
			}
		}
	}

	return nil
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
