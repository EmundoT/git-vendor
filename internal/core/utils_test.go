package core

import (
	"fmt"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// ForEachVendor Tests
// ============================================================================

func TestForEachVendor(t *testing.T) {
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor1", URL: "https://example.com/v1"},
			{Name: "vendor2", URL: "https://example.com/v2"},
			{Name: "vendor3", URL: "https://example.com/v3"},
		},
	}

	// Test: Collect all vendor names
	var names []string
	err := ForEachVendor(config, func(v types.VendorSpec) error {
		names = append(names, v.Name)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEachVendor returned unexpected error: %v", err)
	}

	expected := []string{"vendor1", "vendor2", "vendor3"}
	if len(names) != len(expected) {
		t.Fatalf("Expected %d vendors, got %d", len(expected), len(names))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected vendor[%d] = %s, got %s", i, expected[i], name)
		}
	}
}

func TestForEachVendor_EarlyReturn(t *testing.T) {
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor1"},
			{Name: "vendor2"},
			{Name: "vendor3"},
		},
	}

	// Test: Return error on second vendor
	count := 0
	err := ForEachVendor(config, func(v types.VendorSpec) error {
		count++
		if v.Name == "vendor2" {
			return fmt.Errorf("stop at vendor2")
		}
		return nil
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err.Error() != "stop at vendor2" {
		t.Errorf("Expected 'stop at vendor2' error, got '%s'", err.Error())
	}
	if count != 2 {
		t.Errorf("Expected to process 2 vendors before stopping, processed %d", count)
	}
}

func TestForEachVendor_EmptyConfig(t *testing.T) {
	config := types.VendorConfig{Vendors: []types.VendorSpec{}}

	count := 0
	err := ForEachVendor(config, func(_ types.VendorSpec) error {
		count++
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error for empty config, got %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 iterations for empty config, got %d", count)
	}
}

// ============================================================================
// ForEachMapping Tests
// ============================================================================

func TestForEachMapping(t *testing.T) {
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file1.go", To: "dest/file1.go"},
					{From: "src/file2.go", To: "dest/file2.go"},
				},
			},
			{
				Ref: "develop",
				Mapping: []types.PathMapping{
					{From: "lib/util.go", To: "vendor/util.go"},
				},
			},
		},
	}

	// Test: Collect all mappings
	var mappings []string
	err := ForEachMapping(vendor, func(spec types.BranchSpec, mapping types.PathMapping) error {
		mappings = append(mappings, fmt.Sprintf("%s:%s->%s", spec.Ref, mapping.From, mapping.To))
		return nil
	})

	if err != nil {
		t.Fatalf("ForEachMapping returned unexpected error: %v", err)
	}

	expected := []string{
		"main:src/file1.go->dest/file1.go",
		"main:src/file2.go->dest/file2.go",
		"develop:lib/util.go->vendor/util.go",
	}

	if len(mappings) != len(expected) {
		t.Fatalf("Expected %d mappings, got %d", len(expected), len(mappings))
	}
	for i, mapping := range mappings {
		if mapping != expected[i] {
			t.Errorf("Expected mapping[%d] = %s, got %s", i, expected[i], mapping)
		}
	}
}

func TestForEachMapping_EarlyReturn(t *testing.T) {
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "file1.go", To: "dest1.go"},
					{From: "file2.go", To: "dest2.go"},
					{From: "file3.go", To: "dest3.go"},
				},
			},
		},
	}

	// Test: Stop at second mapping
	count := 0
	err := ForEachMapping(vendor, func(_ types.BranchSpec, mapping types.PathMapping) error {
		count++
		if mapping.From == "file2.go" {
			return fmt.Errorf("stop at file2")
		}
		return nil
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if count != 2 {
		t.Errorf("Expected to process 2 mappings before stopping, processed %d", count)
	}
}

func TestForEachMapping_NoMappings(t *testing.T) {
	vendor := &types.VendorSpec{
		Name:  "empty-vendor",
		Specs: []types.BranchSpec{},
	}

	count := 0
	err := ForEachMapping(vendor, func(_ types.BranchSpec, _ types.PathMapping) error {
		count++
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error for empty mappings, got %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 iterations for empty mappings, got %d", count)
	}
}

// ============================================================================
// ComputeAutoPath Tests
// ============================================================================

func TestComputeAutoPath(t *testing.T) {
	tests := []struct {
		name           string
		sourcePath     string
		defaultTarget  string
		fallbackName   string
		expectedResult string
	}{
		{
			name:           "simple filename",
			sourcePath:     "file.go",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: "file.go",
		},
		{
			name:           "path with directory",
			sourcePath:     "src/lib/util.go",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: "util.go",
		},
		{
			name:           "with default target",
			sourcePath:     "src/file.go",
			defaultTarget:  "vendor/lib",
			fallbackName:   "",
			expectedResult: "vendor/lib/file.go",
		},
		{
			name:           "directory path",
			sourcePath:     "src/components/",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: "components",
		},
		{
			name:           "empty path with fallback",
			sourcePath:     "",
			defaultTarget:  "",
			fallbackName:   "my-vendor",
			expectedResult: "my-vendor",
		},
		{
			name:           "dot path with fallback",
			sourcePath:     ".",
			defaultTarget:  "",
			fallbackName:   "my-vendor",
			expectedResult: "my-vendor",
		},
		{
			name:           "slash path with fallback",
			sourcePath:     "/",
			defaultTarget:  "",
			fallbackName:   "my-vendor",
			expectedResult: "my-vendor",
		},
		{
			name:           "empty path no fallback",
			sourcePath:     "",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: ".",
		},
		{
			name:           "dot path no fallback",
			sourcePath:     ".",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: ".",
		},
		{
			name:           "slash path no fallback",
			sourcePath:     "/",
			defaultTarget:  "",
			fallbackName:   "",
			expectedResult: ".",
		},
		{
			name:           "edge case with default target and empty source",
			sourcePath:     "",
			defaultTarget:  "target/dir",
			fallbackName:   "fallback",
			expectedResult: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeAutoPath(tt.sourcePath, tt.defaultTarget, tt.fallbackName)
			if result != tt.expectedResult {
				t.Errorf("ComputeAutoPath(%q, %q, %q) = %q, want %q",
					tt.sourcePath, tt.defaultTarget, tt.fallbackName, result, tt.expectedResult)
			}
		})
	}
}
