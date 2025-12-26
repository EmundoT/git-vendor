package core

import (
	"fmt"
	"path/filepath"

	"git-vendor/internal/types"
)

// FindVendor returns the vendor with matching name, or nil if not found.
// This consolidates 4 duplicate instances of vendor lookup across vendor_syncer.go
func FindVendor(vendors []types.VendorSpec, name string) *types.VendorSpec {
	for i := range vendors {
		if vendors[i].Name == name {
			return &vendors[i]
		}
	}
	return nil
}

// FindVendorIndex returns the index of vendor with matching name, or -1 if not found.
// Useful for update and delete operations that need the index.
func FindVendorIndex(vendors []types.VendorSpec, name string) int {
	for i, v := range vendors {
		if v.Name == name {
			return i
		}
	}
	return -1
}

// ForEachVendor applies function to each vendor in config.
// Returns early if function returns an error.
func ForEachVendor(config types.VendorConfig, fn func(types.VendorSpec) error) error {
	for _, v := range config.Vendors {
		if err := fn(v); err != nil {
			return err
		}
	}
	return nil
}

// ForEachMapping iterates over all path mappings in a vendor.
// This replaces duplicate triple-nested loops (2 instances in vendor_syncer.go).
func ForEachMapping(vendor types.VendorSpec, fn func(spec types.BranchSpec, mapping types.PathMapping) error) error {
	for _, spec := range vendor.Specs {
		for _, mapping := range spec.Mapping {
			if err := fn(spec, mapping); err != nil {
				return err
			}
		}
	}
	return nil
}

// ComputeAutoPath generates automatic path name with consistent logic.
// This consolidates 4 duplicate instances of auto-naming logic in vendor_syncer.go.
//
// Parameters:
//   - sourcePath: The source path from the remote repository
//   - defaultTarget: Optional default target directory from spec.DefaultTarget
//   - fallbackName: Fallback name to use if path cannot be derived (typically vendor name)
//
// Returns: The computed destination path
func ComputeAutoPath(sourcePath, defaultTarget, fallbackName string) string {
	// Get basename from source path
	autoName := filepath.Base(sourcePath)

	// Handle edge cases: empty, ".", "/"
	if autoName == "" || autoName == "." || autoName == "/" {
		if fallbackName != "" {
			return fallbackName
		}
		return "."
	}

	// Apply default target if specified
	if defaultTarget != "" {
		return filepath.Join(defaultTarget, autoName)
	}

	return autoName
}

// Pluralize returns the singular or plural form based on count.
// Examples:
//
//	Pluralize(1, "vendor", "vendors") => "1 vendor"
//	Pluralize(2, "vendor", "vendors") => "2 vendors"
//	Pluralize(0, "vendor", "vendors") => "0 vendors"
func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
