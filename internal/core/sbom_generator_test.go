package core

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// CycloneDX Tests
// ============================================================================

func TestGenerateCycloneDX_SingleVendor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "test-lib",
				URL:     "https://github.com/owner/test-lib",
				License: "MIT",
			},
		},
	}, nil)

	// Mock lock with full metadata
	lockStore.EXPECT().Load().Return(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{
				Name:             "test-lib",
				Ref:              "main",
				CommitHash:       "abc1234567890def",
				LicenseSPDX:      "MIT",
				SourceVersionTag: "v1.2.3",
				VendoredAt:       "2026-01-15T10:00:00Z",
				VendoredBy:       "User <user@example.com>",
				LastSyncedAt:     "2026-02-04T12:00:00Z",
				FileHashes: map[string]string{
					"lib/test-lib/file.go": "sha256hash123",
				},
			},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "my-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse and validate JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Verify required fields
	if result["bomFormat"] != "CycloneDX" {
		t.Errorf("Expected bomFormat 'CycloneDX', got %v", result["bomFormat"])
	}

	if result["specVersion"] != "1.5" {
		t.Errorf("Expected specVersion '1.5', got %v", result["specVersion"])
	}

	// Verify serial number starts with urn:uuid:
	serialNumber, ok := result["serialNumber"].(string)
	if !ok || !strings.HasPrefix(serialNumber, "urn:uuid:") {
		t.Errorf("Expected serialNumber to start with 'urn:uuid:', got %v", result["serialNumber"])
	}

	// Verify components
	components, ok := result["components"].([]interface{})
	if !ok || len(components) != 1 {
		t.Fatalf("Expected 1 component, got %v", result["components"])
	}

	comp := components[0].(map[string]interface{})
	if comp["name"] != "test-lib" {
		t.Errorf("Expected component name 'test-lib', got %v", comp["name"])
	}

	// Version should prefer source_version_tag
	if comp["version"] != "v1.2.3" {
		t.Errorf("Expected component version 'v1.2.3', got %v", comp["version"])
	}

	// Check PURL - now properly URL encoded
	if comp["purl"] != "pkg:github/owner/test-lib@abc1234567890def" {
		t.Errorf("Expected PURL 'pkg:github/owner/test-lib@abc1234567890def', got %v", comp["purl"])
	}

	// Check BOM ref - now includes commit hash suffix
	bomRef := comp["bom-ref"].(string)
	if !strings.HasPrefix(bomRef, "test-lib@") {
		t.Errorf("Expected bom-ref to start with 'test-lib@', got %v", bomRef)
	}

	// Check license
	licenses, ok := comp["licenses"].([]interface{})
	if !ok || len(licenses) != 1 {
		t.Errorf("Expected 1 license, got %v", comp["licenses"])
	}

	// Check supplier (Issue #12)
	supplier, ok := comp["supplier"].(map[string]interface{})
	if !ok {
		t.Error("Expected supplier field in component")
	} else if supplier["name"] != "owner" {
		t.Errorf("Expected supplier name 'owner', got %v", supplier["name"])
	}
}

func TestGenerateCycloneDX_MultipleVendors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config with multiple vendors
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: "MIT"},
			{Name: "lib-b", URL: "https://gitlab.com/owner/lib-b", License: "Apache-2.0"},
			{Name: "lib-c", URL: "https://bitbucket.org/owner/lib-c", License: "BSD-3-Clause"},
		},
	}, nil)

	// Mock lock with multiple vendors
	lockStore.EXPECT().Load().Return(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", CommitHash: "aaa1111", LicenseSPDX: "MIT"},
			{Name: "lib-b", Ref: "v2.0", CommitHash: "bbb2222", LicenseSPDX: "Apache-2.0"},
			{Name: "lib-c", Ref: "master", CommitHash: "ccc3333", LicenseSPDX: "BSD-3-Clause"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "multi-vendor-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Verify all 3 components present
	components, ok := result["components"].([]interface{})
	if !ok || len(components) != 3 {
		t.Fatalf("Expected 3 components, got %d", len(components))
	}

	// Verify each component exists with correct name
	names := make(map[string]bool)
	for _, c := range components {
		comp := c.(map[string]interface{})
		names[comp["name"].(string)] = true
	}

	if !names["lib-a"] || !names["lib-b"] || !names["lib-c"] {
		t.Errorf("Missing expected components, got names: %v", names)
	}
}

func TestGenerateCycloneDX_MissingLicense(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "no-license-lib", URL: "https://github.com/owner/no-license-lib"},
		},
	}, nil)

	// Mock lock WITHOUT license
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "no-license-lib", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Verify component has no licenses field (omitted, not empty)
	components := result["components"].([]interface{})
	comp := components[0].(map[string]interface{})

	// licenses should be nil/omitted when not set
	if licenses, exists := comp["licenses"]; exists && licenses != nil {
		t.Errorf("Expected no licenses field for vendor without license, got %v", licenses)
	}
}

func TestGenerateCycloneDX_EmptyLockfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock empty config and lock
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "empty-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse JSON - should be valid even with no components
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Verify it's still a valid CycloneDX document
	if result["bomFormat"] != "CycloneDX" {
		t.Errorf("Expected bomFormat 'CycloneDX', got %v", result["bomFormat"])
	}

	// Components should be empty array
	components := result["components"].([]interface{})
	if len(components) != 0 {
		t.Errorf("Expected 0 components for empty lockfile, got %d", len(components))
	}
}

func TestGenerateCycloneDX_UsesToolsComponents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Verify metadata.tools.components is used (Issue #9 - migrate from deprecated Tools)
	metadata := result["metadata"].(map[string]interface{})
	tools := metadata["tools"].(map[string]interface{})

	// Should have "components" not "tools" array
	if _, hasComponents := tools["components"]; !hasComponents {
		t.Error("Expected tools.components to be present (using new format)")
	}

	// Verify the tool component has git-vendor info
	toolComponents := tools["components"].([]interface{})
	if len(toolComponents) == 0 {
		t.Fatal("Expected at least one tool component")
	}

	toolComp := toolComponents[0].(map[string]interface{})
	if toolComp["name"] != "git-vendor" {
		t.Errorf("Expected tool name 'git-vendor', got %v", toolComp["name"])
	}
}

// ============================================================================
// SPDX Tests
// ============================================================================

func TestGenerateSPDX_ValidOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "spdx-test-lib", URL: "https://github.com/owner/spdx-test", License: "Apache-2.0"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{
				Name:             "spdx-test-lib",
				Ref:              "v1.0.0",
				CommitHash:       "def456789012",
				LicenseSPDX:      "Apache-2.0",
				SourceVersionTag: "v1.0.0",
				VendoredAt:       "2026-01-20T15:00:00Z",
				VendoredBy:       "DevTeam <dev@example.com>",
				FileHashes: map[string]string{
					"lib/spdx-test/main.go": "hash123abc",
				},
			},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "spdx-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse and validate JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Verify required SPDX fields
	if result["spdxVersion"] != "SPDX-2.3" {
		t.Errorf("Expected spdxVersion 'SPDX-2.3', got %v", result["spdxVersion"])
	}

	if result["dataLicense"] != "CC0-1.0" {
		t.Errorf("Expected dataLicense 'CC0-1.0', got %v", result["dataLicense"])
	}

	if result["SPDXID"] != "SPDXRef-DOCUMENT" {
		t.Errorf("Expected SPDXID 'SPDXRef-DOCUMENT', got %v", result["SPDXID"])
	}

	// Verify document namespace uses default (Issue #11)
	namespace, ok := result["documentNamespace"].(string)
	if !ok || !strings.HasPrefix(namespace, "https://spdx.org/spdxdocs/spdx-project/") {
		t.Errorf("Expected documentNamespace to start with default domain, got %v", result["documentNamespace"])
	}

	// Verify packages
	packages, ok := result["packages"].([]interface{})
	if !ok || len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %v", result["packages"])
	}

	pkg := packages[0].(map[string]interface{})
	if pkg["name"] != "spdx-test-lib" {
		t.Errorf("Expected package name 'spdx-test-lib', got %v", pkg["name"])
	}

	if pkg["versionInfo"] != "v1.0.0" {
		t.Errorf("Expected versionInfo 'v1.0.0', got %v", pkg["versionInfo"])
	}

	if pkg["licenseDeclared"] != "Apache-2.0" {
		t.Errorf("Expected licenseDeclared 'Apache-2.0', got %v", pkg["licenseDeclared"])
	}

	if pkg["downloadLocation"] != "https://github.com/owner/spdx-test" {
		t.Errorf("Expected downloadLocation 'https://github.com/owner/spdx-test', got %v", pkg["downloadLocation"])
	}

	// Verify supplier (Issue #12)
	supplier, ok := pkg["supplier"].(string)
	if !ok || supplier == "" {
		t.Error("Expected supplier field in package")
	} else if !strings.Contains(supplier, "owner") {
		t.Errorf("Expected supplier to contain 'owner', got %v", supplier)
	}

	// Verify relationships
	relationships, ok := result["relationships"].([]interface{})
	if !ok || len(relationships) != 1 {
		t.Fatalf("Expected 1 relationship, got %v", result["relationships"])
	}

	rel := relationships[0].(map[string]interface{})
	if rel["relationshipType"] != "DESCRIBES" {
		t.Errorf("Expected relationshipType 'DESCRIBES', got %v", rel["relationshipType"])
	}
}

func TestGenerateSPDX_RelationshipMatchesPackageID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test-vendor", URL: "https://github.com/owner/test"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-vendor", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Get package SPDXID
	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})
	packageSPDXID := pkg["SPDXID"].(string)

	// Get relationship target
	relationships := result["relationships"].([]interface{})
	rel := relationships[0].(map[string]interface{})
	relatedElement := rel["relatedSpdxElement"].(string)

	// CRITICAL: The relationship must point to the actual package SPDXID
	if packageSPDXID != relatedElement {
		t.Errorf("Relationship target '%s' does not match package SPDXID '%s'", relatedElement, packageSPDXID)
	}

	// Also verify the format is correct - now includes hash for uniqueness (Issue #5)
	if !strings.HasPrefix(packageSPDXID, "SPDXRef-Package-") {
		t.Errorf("Package SPDXID should start with 'SPDXRef-Package-', got '%s'", packageSPDXID)
	}
}

func TestGenerateSPDX_SpecialCharactersInVendorName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config with special characters in name
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor@special/chars!", URL: "https://github.com/owner/special"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor@special/chars!", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Verify package SPDXID is sanitized (no special chars except . and -)
	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})
	packageSPDXID := pkg["SPDXID"].(string)

	// Should be sanitized with hash suffix
	if !strings.HasPrefix(packageSPDXID, "SPDXRef-Package-vendor-special-chars-") {
		t.Errorf("Expected sanitized SPDXID to start with 'SPDXRef-Package-vendor-special-chars-', got '%s'", packageSPDXID)
	}

	// Verify relationship still points to correct package
	relationships := result["relationships"].([]interface{})
	rel := relationships[0].(map[string]interface{})
	relatedElement := rel["relatedSpdxElement"].(string)

	if packageSPDXID != relatedElement {
		t.Errorf("Relationship target '%s' does not match package SPDXID '%s'", relatedElement, packageSPDXID)
	}
}

func TestGenerateSPDX_VendorMissingFromConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock EMPTY config (vendor exists in lock but not in config)
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{},
	}, nil)

	// Mock lock with vendor
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "orphan-vendor", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Package should have NOASSERTION for download location
	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})

	if pkg["downloadLocation"] != "NOASSERTION" {
		t.Errorf("Expected downloadLocation 'NOASSERTION' for vendor missing from config, got %v", pkg["downloadLocation"])
	}
}

func TestGenerateSPDX_EmptyLockfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock empty config and lock
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "empty-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Verify it's still a valid SPDX document
	if result["spdxVersion"] != "SPDX-2.3" {
		t.Errorf("Expected spdxVersion 'SPDX-2.3', got %v", result["spdxVersion"])
	}

	// Packages should be empty array
	packages := result["packages"].([]interface{})
	if len(packages) != 0 {
		t.Errorf("Expected 0 packages for empty lockfile, got %d", len(packages))
	}

	// Relationships should be empty array
	relationships := result["relationships"].([]interface{})
	if len(relationships) != 0 {
		t.Errorf("Expected 0 relationships for empty lockfile, got %d", len(relationships))
	}
}

func TestGenerateSPDX_CustomNamespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	// Test custom namespace (Issue #11)
	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName:   "my-project",
		SPDXNamespace: "https://example.com/sbom",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	namespace := result["documentNamespace"].(string)
	if !strings.HasPrefix(namespace, "https://example.com/sbom/my-project/") {
		t.Errorf("Expected custom namespace prefix, got %v", namespace)
	}
}

// ============================================================================
// Error Cases
// ============================================================================

func TestGenerate_UnknownFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock successful loads
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test",
	})
	_, err := generator.Generate("unknown-format")

	if err == nil {
		t.Fatal("Expected error for unknown format")
	}

	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("Expected 'unknown format' in error, got: %v", err)
	}
}

func TestGenerate_LockfileLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock lockfile load failure
	lockStore.EXPECT().Load().Return(types.VendorLock{}, errors.New("lockfile not found"))

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test",
	})
	_, err := generator.Generate(SBOMFormatCycloneDX)

	if err == nil {
		t.Fatal("Expected error when lockfile fails to load")
	}

	if !strings.Contains(err.Error(), "load lockfile") {
		t.Errorf("Expected 'load lockfile' in error, got: %v", err)
	}
}

func TestGenerate_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock successful lock load, but config fails
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{}, errors.New("config not found"))

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test",
	})
	_, err := generator.Generate(SBOMFormatCycloneDX)

	if err == nil {
		t.Fatal("Expected error when config fails to load")
	}

	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("Expected 'load config' in error, got: %v", err)
	}
}

// ============================================================================
// Issue #3: Empty Project Name Validation
// ============================================================================

func TestGenerate_EmptyProjectName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	// Test with empty project name - should use default
	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	metadata := result["metadata"].(map[string]interface{})
	component := metadata["component"].(map[string]interface{})

	// Should use default project name, not empty
	if component["name"] == "" {
		t.Error("Project name should not be empty - should use default")
	}
}

// ============================================================================
// Issue #4: SPDX Comment Empty Values
// ============================================================================

func TestGenerateSPDX_CommentOmitsEmptyValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test-lib", URL: "https://github.com/owner/test"},
		},
	}, nil)

	// Lock with only ref and commit, no vendoredAt or vendoredBy
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})

	comment, ok := pkg["comment"].(string)
	if !ok {
		t.Skip("No comment field present - which is acceptable for minimal metadata")
	}

	// Comment should NOT contain empty values like "vendored_at=,"
	if strings.Contains(comment, "vendored_at=,") || strings.Contains(comment, "vendored_by=,") {
		t.Errorf("Comment should not contain empty values: %s", comment)
	}
}

// ============================================================================
// Issue #5 & #6: ID Uniqueness for Multiple Refs
// ============================================================================

func TestGenerateCycloneDX_VendorWithMultipleRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "multi-ref-lib", URL: "https://github.com/owner/multi-ref"},
		},
	}, nil)

	// Same vendor tracking MULTIPLE refs
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "multi-ref-lib", Ref: "main", CommitHash: "maincommit123"},
			{Name: "multi-ref-lib", Ref: "dev", CommitHash: "devcommit4567"},
			{Name: "multi-ref-lib", Ref: "v1.0", CommitHash: "v1commit78901", SourceVersionTag: "v1.0.0"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	components := result["components"].([]interface{})
	if len(components) != 3 {
		t.Fatalf("Expected 3 components for vendor with 3 refs, got %d", len(components))
	}

	// Verify BOM-refs are UNIQUE (Issue #6)
	bomRefs := make(map[string]bool)
	for _, c := range components {
		comp := c.(map[string]interface{})
		bomRef := comp["bom-ref"].(string)
		if bomRefs[bomRef] {
			t.Errorf("Duplicate bom-ref found: %s", bomRef)
		}
		bomRefs[bomRef] = true
	}

	if len(bomRefs) != 3 {
		t.Errorf("Expected 3 unique bom-refs, got %d", len(bomRefs))
	}
}

func TestGenerateSPDX_VendorWithMultipleRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "multi-ref-lib", URL: "https://github.com/owner/multi-ref"},
		},
	}, nil)

	// Same vendor tracking MULTIPLE refs
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "multi-ref-lib", Ref: "main", CommitHash: "aaa11111"},
			{Name: "multi-ref-lib", Ref: "feature", CommitHash: "bbb22222"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	packages := result["packages"].([]interface{})
	if len(packages) != 2 {
		t.Fatalf("Expected 2 packages for vendor with 2 refs, got %d", len(packages))
	}

	// Verify SPDX IDs are UNIQUE (Issue #5)
	spdxIDs := make(map[string]bool)
	for _, p := range packages {
		pkg := p.(map[string]interface{})
		spdxID := pkg["SPDXID"].(string)
		if spdxIDs[spdxID] {
			t.Errorf("Duplicate SPDXID found: %s", spdxID)
		}
		spdxIDs[spdxID] = true
	}

	if len(spdxIDs) != 2 {
		t.Errorf("Expected 2 unique SPDXIDs, got %d", len(spdxIDs))
	}

	// Verify relationships match packages
	relationships := result["relationships"].([]interface{})
	if len(relationships) != 2 {
		t.Fatalf("Expected 2 relationships, got %d", len(relationships))
	}

	for _, r := range relationships {
		rel := r.(map[string]interface{})
		target := rel["relatedSpdxElement"].(string)
		if !spdxIDs[target] {
			t.Errorf("Relationship target '%s' does not match any package SPDXID", target)
		}
	}
}

// ============================================================================
// Issue #10: Schema Validation
// ============================================================================

func TestGenerate_WithValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test-lib", URL: "https://github.com/owner/test"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	// Enable validation
	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
		Validate:    true,
	})

	// CycloneDX with validation
	output, err := generator.Generate(SBOMFormatCycloneDX)
	if err != nil {
		t.Fatalf("CycloneDX validation failed: %v", err)
	}

	// Verify it's valid by parsing with CycloneDX library
	decoder := cdx.NewBOMDecoder(strings.NewReader(string(output)), cdx.BOMFileFormatJSON)
	var bom cdx.BOM
	if err := decoder.Decode(&bom); err != nil {
		t.Errorf("CycloneDX output not parseable by library: %v", err)
	}
}

func TestGenerate_SPDXWithValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test-lib", URL: "https://github.com/owner/test"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	// Enable validation
	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
		Validate:    true,
	})

	// SPDX with validation
	output, err := generator.Generate(SBOMFormatSPDX)
	if err != nil {
		t.Fatalf("SPDX validation failed: %v", err)
	}

	// Verify required fields are present
	var doc spdxJSON
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("SPDX output not valid JSON: %v", err)
	}

	if doc.SPDXVersion == "" {
		t.Error("SPDX document missing spdxVersion")
	}
	if doc.SPDXID == "" {
		t.Error("SPDX document missing SPDXID")
	}
}

// ============================================================================
// Issue #13: Integration Parse Tests
// ============================================================================

func TestCycloneDX_ParseableByLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a"},
			{Name: "lib-b", URL: "https://gitlab.com/group/subgroup/lib-b"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", CommitHash: "abc123", LicenseSPDX: "MIT"},
			{Name: "lib-b", Ref: "v1.0", CommitHash: "def456", LicenseSPDX: "Apache-2.0"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "integration-test",
	})

	output, err := generator.Generate(SBOMFormatCycloneDX)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	// Parse with CycloneDX library
	decoder := cdx.NewBOMDecoder(strings.NewReader(string(output)), cdx.BOMFileFormatJSON)
	var bom cdx.BOM
	if err := decoder.Decode(&bom); err != nil {
		t.Fatalf("CycloneDX library failed to parse output: %v", err)
	}

	// Verify parsed content
	if bom.Components == nil || len(*bom.Components) != 2 {
		t.Errorf("Expected 2 components, got %v", bom.Components)
	}

	if bom.Metadata == nil || bom.Metadata.Component == nil {
		t.Error("Missing metadata component")
	} else if bom.Metadata.Component.Name != "integration-test" {
		t.Errorf("Expected project name 'integration-test', got %s", bom.Metadata.Component.Name)
	}
}

// ============================================================================
// Issue #14: Long Vendor Names
// ============================================================================

func TestGenerate_VeryLongVendorName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Create a very long vendor name
	longName := strings.Repeat("a", 500)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: longName, URL: "https://github.com/owner/repo"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: longName, Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})

	// Should not panic or error
	outputCDX, err := generator.Generate(SBOMFormatCycloneDX)
	if err != nil {
		t.Fatalf("CycloneDX generation with long name failed: %v", err)
	}

	// Verify it's valid JSON
	var resultCDX map[string]interface{}
	if err := json.Unmarshal(outputCDX, &resultCDX); err != nil {
		t.Fatalf("CycloneDX output not valid JSON: %v", err)
	}

	// Re-setup mocks for SPDX test
	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()

	configStore2 := NewMockConfigStore(ctrl2)
	lockStore2 := NewMockLockStore(ctrl2)

	configStore2.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: longName, URL: "https://github.com/owner/repo"},
		},
	}, nil)

	lockStore2.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: longName, Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator2 := NewSBOMGeneratorWithOptions(lockStore2, configStore2, SBOMOptions{
		ProjectName: "test-project",
	})

	outputSPDX, err := generator2.Generate(SBOMFormatSPDX)
	if err != nil {
		t.Fatalf("SPDX generation with long name failed: %v", err)
	}

	var resultSPDX map[string]interface{}
	if err := json.Unmarshal(outputSPDX, &resultSPDX); err != nil {
		t.Fatalf("SPDX output not valid JSON: %v", err)
	}
}

// ============================================================================
// Issue #15: Concurrent Generation
// ============================================================================

func TestGenerate_ConcurrentSafety(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// We'll create multiple generators and run them concurrently
	// Each needs its own mocks
	const numGoroutines = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*2)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2) // One for CycloneDX, one for SPDX

		go func(_ int) {
			defer wg.Done()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			configStore := NewMockConfigStore(ctrl)
			lockStore := NewMockLockStore(ctrl)

			configStore.EXPECT().Load().Return(types.VendorConfig{
				Vendors: []types.VendorSpec{
					{Name: "test-lib", URL: "https://github.com/owner/test"},
				},
			}, nil)

			lockStore.EXPECT().Load().Return(types.VendorLock{
				Vendors: []types.LockDetails{
					{Name: "test-lib", Ref: "main", CommitHash: "abc123"},
				},
			}, nil)

			generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
				ProjectName: "concurrent-test",
			})

			_, err := generator.Generate(SBOMFormatCycloneDX)
			if err != nil {
				errors <- err
			}
		}(i)

		go func(_ int) {
			defer wg.Done()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			configStore := NewMockConfigStore(ctrl)
			lockStore := NewMockLockStore(ctrl)

			configStore.EXPECT().Load().Return(types.VendorConfig{
				Vendors: []types.VendorSpec{
					{Name: "test-lib", URL: "https://github.com/owner/test"},
				},
			}, nil)

			lockStore.EXPECT().Load().Return(types.VendorLock{
				Vendors: []types.LockDetails{
					{Name: "test-lib", Ref: "main", CommitHash: "abc123"},
				},
			}, nil)

			generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
				ProjectName: "concurrent-test",
			})

			_, err := generator.Generate(SBOMFormatSPDX)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent generation error: %v", err)
	}
}

// ============================================================================
// File Hashes / Checksums Tests
// ============================================================================

func TestGenerateCycloneDX_IncludesFileHashes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "hashed-lib", URL: "https://github.com/owner/hashed"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "hashed-lib",
				Ref:        "main",
				CommitHash: "abc1234",
				FileHashes: map[string]string{
					"lib/file1.go": "sha256hashvalue1",
					"lib/file2.go": "sha256hashvalue2",
					"lib/file3.go": "sha256hashvalue3",
				},
			},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	components := result["components"].([]interface{})
	comp := components[0].(map[string]interface{})

	hashes, ok := comp["hashes"].([]interface{})
	if !ok {
		t.Fatal("Expected 'hashes' array in component")
	}

	if len(hashes) != 3 {
		t.Errorf("Expected 3 hashes, got %d", len(hashes))
	}

	hash := hashes[0].(map[string]interface{})
	if hash["alg"] != "SHA-256" {
		t.Errorf("Expected hash algorithm 'SHA-256', got %v", hash["alg"])
	}
}

func TestGenerateSPDX_IncludesChecksums(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "checksummed-lib", URL: "https://github.com/owner/checksummed"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "checksummed-lib",
				Ref:        "main",
				CommitHash: "def4567",
				FileHashes: map[string]string{
					"src/main.go":  "checksumvalue1",
					"src/utils.go": "checksumvalue2",
				},
			},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})

	checksums, ok := pkg["checksums"].([]interface{})
	if !ok {
		t.Fatal("Expected 'checksums' array in package")
	}

	if len(checksums) != 2 {
		t.Errorf("Expected 2 checksums, got %d", len(checksums))
	}

	checksum := checksums[0].(map[string]interface{})
	if checksum["algorithm"] != "SHA256" {
		t.Errorf("Expected checksum algorithm 'SHA256', got %v", checksum["algorithm"])
	}
}

// ============================================================================
// External References Tests
// ============================================================================

func TestGenerateCycloneDX_IncludesVCSReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	repoURL := "https://github.com/owner/vcs-test-lib"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vcs-lib", URL: repoURL},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vcs-lib", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	components := result["components"].([]interface{})
	comp := components[0].(map[string]interface{})

	extRefs, ok := comp["externalReferences"].([]interface{})
	if !ok {
		t.Fatal("Expected 'externalReferences' array in component")
	}

	if len(extRefs) != 1 {
		t.Fatalf("Expected 1 external reference, got %d", len(extRefs))
	}

	vcsRef := extRefs[0].(map[string]interface{})
	if vcsRef["type"] != "vcs" {
		t.Errorf("Expected external reference type 'vcs', got %v", vcsRef["type"])
	}

	if vcsRef["url"] != repoURL {
		t.Errorf("Expected external reference URL '%s', got %v", repoURL, vcsRef["url"])
	}
}

func TestGenerateSPDX_IncludesPURLExternalRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "purl-lib", URL: "https://github.com/owner/purl-test"},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "purl-lib", Ref: "main", CommitHash: "abc1234"},
		},
	}, nil)

	generator := NewSBOMGeneratorWithOptions(lockStore, configStore, SBOMOptions{
		ProjectName: "test-project",
	})
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	packages := result["packages"].([]interface{})
	pkg := packages[0].(map[string]interface{})

	extRefs, ok := pkg["externalRefs"].([]interface{})
	if !ok {
		t.Fatal("Expected 'externalRefs' array in package")
	}

	if len(extRefs) != 1 {
		t.Fatalf("Expected 1 external ref, got %d", len(extRefs))
	}

	purlRef := extRefs[0].(map[string]interface{})
	if purlRef["referenceCategory"] != "PACKAGE-MANAGER" {
		t.Errorf("Expected referenceCategory 'PACKAGE-MANAGER', got %v", purlRef["referenceCategory"])
	}

	if purlRef["referenceType"] != "purl" {
		t.Errorf("Expected referenceType 'purl', got %v", purlRef["referenceType"])
	}

	expectedPURL := "pkg:github/owner/purl-test@abc1234"
	if purlRef["referenceLocator"] != expectedPURL {
		t.Errorf("Expected referenceLocator '%s', got %v", expectedPURL, purlRef["referenceLocator"])
	}
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewSBOMGenerator_BasicConstructor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, "test-project")

	_, err := generator.Generate(SBOMFormatCycloneDX)
	if err != nil {
		t.Fatalf("Basic constructor failed: %v", err)
	}
}
