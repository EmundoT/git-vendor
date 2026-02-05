package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	fs := NewMockFileSystem(ctrl)

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

	generator := NewSBOMGenerator(lockStore, configStore, fs, "my-project")
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

	// Check PURL
	if comp["purl"] != "pkg:github/owner/test-lib@abc1234567890def" {
		t.Errorf("Expected PURL 'pkg:github/owner/test-lib@abc1234567890def', got %v", comp["purl"])
	}

	// Check BOM ref
	if comp["bom-ref"] != "test-lib@abc1234" {
		t.Errorf("Expected bom-ref 'test-lib@abc1234', got %v", comp["bom-ref"])
	}

	// Check license
	licenses, ok := comp["licenses"].([]interface{})
	if !ok || len(licenses) != 1 {
		t.Errorf("Expected 1 license, got %v", comp["licenses"])
	}
}

func TestGenerateCycloneDX_MultipleVendors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

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
			{Name: "lib-a", Ref: "main", CommitHash: "aaa111", LicenseSPDX: "MIT"},
			{Name: "lib-b", Ref: "v2.0", CommitHash: "bbb222", LicenseSPDX: "Apache-2.0"},
			{Name: "lib-c", Ref: "master", CommitHash: "ccc333", LicenseSPDX: "BSD-3-Clause"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "multi-vendor-project")
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
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "no-license-lib", URL: "https://github.com/owner/no-license-lib"},
		},
	}, nil)

	// Mock lock WITHOUT license
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "no-license-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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
	fs := NewMockFileSystem(ctrl)

	// Mock empty config and lock
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "empty-project")
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

// ============================================================================
// SPDX Tests
// ============================================================================

func TestGenerateSPDX_ValidOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

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

	generator := NewSBOMGenerator(lockStore, configStore, fs, "spdx-project")
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

	// Verify document namespace starts correctly
	namespace, ok := result["documentNamespace"].(string)
	if !ok || !strings.HasPrefix(namespace, "https://git-vendor.dev/spdx/spdx-project/") {
		t.Errorf("Expected documentNamespace to start with 'https://git-vendor.dev/spdx/spdx-project/', got %v", result["documentNamespace"])
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
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test-vendor", URL: "https://github.com/owner/test"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Also verify the format is correct
	if !strings.HasPrefix(packageSPDXID, "SPDXRef-Package-") {
		t.Errorf("Package SPDXID should start with 'SPDXRef-Package-', got '%s'", packageSPDXID)
	}
}

func TestGenerateSPDX_SpecialCharactersInVendorName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config with special characters in name
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor@special/chars!", URL: "https://github.com/owner/special"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor@special/chars!", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Should be sanitized: vendor@special/chars! -> vendor-special-chars-
	expectedSPDXID := "SPDXRef-Package-vendor-special-chars-"
	if packageSPDXID != expectedSPDXID {
		t.Errorf("Expected sanitized SPDXID '%s', got '%s'", expectedSPDXID, packageSPDXID)
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
	fs := NewMockFileSystem(ctrl)

	// Mock EMPTY config (vendor exists in lock but not in config)
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{},
	}, nil)

	// Mock lock with vendor
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "orphan-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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
	fs := NewMockFileSystem(ctrl)

	// Mock empty config and lock
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "empty-project")
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

// ============================================================================
// PURL Tests
// ============================================================================

func TestGetPURL_GitHub(t *testing.T) {
	generator := &SBOMGenerator{}

	tests := []struct {
		name       string
		url        string
		commitHash string
		expected   string
	}{
		{
			name:       "GitHub standard URL",
			url:        "https://github.com/owner/repo",
			commitHash: "abc123",
			expected:   "pkg:github/owner/repo@abc123",
		},
		{
			name:       "GitHub URL with .git suffix",
			url:        "https://github.com/owner/repo.git",
			commitHash: "def456",
			expected:   "pkg:github/owner/repo@def456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.getPURL("test", tc.url, tc.commitHash)
			if result != tc.expected {
				t.Errorf("Expected PURL '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestGetPURL_GitLab(t *testing.T) {
	generator := &SBOMGenerator{}

	tests := []struct {
		name       string
		url        string
		commitHash string
		expected   string
	}{
		{
			name:       "GitLab standard URL",
			url:        "https://gitlab.com/owner/repo",
			commitHash: "ghi789",
			expected:   "pkg:gitlab/owner/repo@ghi789",
		},
		{
			name:       "GitLab nested groups",
			url:        "https://gitlab.com/group/subgroup/repo",
			commitHash: "jkl012",
			expected:   "pkg:gitlab/group%2Fsubgroup/repo@jkl012",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.getPURL("test", tc.url, tc.commitHash)
			if result != tc.expected {
				t.Errorf("Expected PURL '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestGetPURL_Bitbucket(t *testing.T) {
	generator := &SBOMGenerator{}

	result := generator.getPURL("test", "https://bitbucket.org/owner/repo", "mno345")
	expected := "pkg:bitbucket/owner/repo@mno345"

	if result != expected {
		t.Errorf("Expected PURL '%s', got '%s'", expected, result)
	}
}

func TestGetPURL_Generic(t *testing.T) {
	generator := &SBOMGenerator{}

	tests := []struct {
		name       string
		vendorName string
		url        string
		commitHash string
		expected   string
	}{
		{
			name:       "Empty URL",
			vendorName: "custom-lib",
			url:        "",
			commitHash: "pqr678",
			expected:   "pkg:generic/custom-lib@pqr678",
		},
		{
			name:       "Unknown host",
			vendorName: "private-lib",
			url:        "https://git.internal.company.com/team/repo",
			commitHash: "stu901",
			expected:   "pkg:generic/private-lib@stu901",
		},
		{
			name:       "Invalid URL",
			vendorName: "bad-url-lib",
			url:        "not-a-valid-url",
			commitHash: "xyz000",
			expected:   "pkg:generic/bad-url-lib@xyz000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.getPURL(tc.vendorName, tc.url, tc.commitHash)
			if result != tc.expected {
				t.Errorf("Expected PURL '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestSanitizeSPDXID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with.dot", "with.dot"},
		{"with space", "with-space"},
		{"special@chars!", "special-chars-"},
		{"CamelCase123", "CamelCase123"},
		{"path/to/file", "path-to-file"},
		{"under_score", "under-score"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeSPDXID(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeSPDXID(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
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
	fs := NewMockFileSystem(ctrl)

	// Mock successful loads
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test")
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
	fs := NewMockFileSystem(ctrl)

	// Mock lockfile load failure
	lockStore.EXPECT().Load().Return(types.VendorLock{}, errors.New("lockfile not found"))

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test")
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
	fs := NewMockFileSystem(ctrl)

	// Mock successful lock load, but config fails
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{}, errors.New("config not found"))

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test")
	_, err := generator.Generate(SBOMFormatCycloneDX)

	if err == nil {
		t.Fatal("Expected error when config fails to load")
	}

	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("Expected 'load config' in error, got: %v", err)
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
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "hashed-lib", URL: "https://github.com/owner/hashed"},
		},
	}, nil)

	// Mock lock with multiple file hashes
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "hashed-lib",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{
					"lib/file1.go": "sha256hashvalue1",
					"lib/file2.go": "sha256hashvalue2",
					"lib/file3.go": "sha256hashvalue3",
				},
			},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify hashes array exists
	hashes, ok := comp["hashes"].([]interface{})
	if !ok {
		t.Fatal("Expected 'hashes' array in component")
	}

	if len(hashes) != 3 {
		t.Errorf("Expected 3 hashes, got %d", len(hashes))
	}

	// Verify hash structure
	hash := hashes[0].(map[string]interface{})
	if hash["alg"] != "SHA-256" {
		t.Errorf("Expected hash algorithm 'SHA-256', got %v", hash["alg"])
	}

	// Verify hash content is present (don't check exact value since map iteration order varies)
	if hash["content"] == nil || hash["content"] == "" {
		t.Error("Expected hash content to be present")
	}
}

func TestGenerateSPDX_IncludesChecksums(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "checksummed-lib", URL: "https://github.com/owner/checksummed"},
		},
	}, nil)

	// Mock lock with file hashes
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "checksummed-lib",
				Ref:        "main",
				CommitHash: "def456",
				FileHashes: map[string]string{
					"src/main.go":  "checksumvalue1",
					"src/utils.go": "checksumvalue2",
				},
			},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify checksums array exists
	checksums, ok := pkg["checksums"].([]interface{})
	if !ok {
		t.Fatal("Expected 'checksums' array in package")
	}

	if len(checksums) != 2 {
		t.Errorf("Expected 2 checksums, got %d", len(checksums))
	}

	// Verify checksum structure
	checksum := checksums[0].(map[string]interface{})
	if checksum["algorithm"] != "SHA256" {
		t.Errorf("Expected checksum algorithm 'SHA256', got %v", checksum["algorithm"])
	}

	if checksum["checksumValue"] == nil || checksum["checksumValue"] == "" {
		t.Error("Expected checksumValue to be present")
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
	fs := NewMockFileSystem(ctrl)

	repoURL := "https://github.com/owner/vcs-test-lib"

	// Mock config with URL
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vcs-lib", URL: repoURL},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vcs-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify externalReferences array exists
	extRefs, ok := comp["externalReferences"].([]interface{})
	if !ok {
		t.Fatal("Expected 'externalReferences' array in component")
	}

	if len(extRefs) != 1 {
		t.Fatalf("Expected 1 external reference, got %d", len(extRefs))
	}

	// Verify VCS reference structure
	vcsRef := extRefs[0].(map[string]interface{})
	if vcsRef["type"] != "vcs" {
		t.Errorf("Expected external reference type 'vcs', got %v", vcsRef["type"])
	}

	if vcsRef["url"] != repoURL {
		t.Errorf("Expected external reference URL '%s', got %v", repoURL, vcsRef["url"])
	}
}

func TestGenerateCycloneDX_NoVCSReferenceWhenNoURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config WITHOUT URL (vendor missing from config)
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "no-url-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify externalReferences is NOT present when URL is empty
	if extRefs, exists := comp["externalReferences"]; exists && extRefs != nil {
		t.Errorf("Expected no externalReferences when URL is empty, got %v", extRefs)
	}
}

// ============================================================================
// Custom Properties Tests
// ============================================================================

func TestGenerateCycloneDX_IncludesGitVendorProperties(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "props-lib", URL: "https://github.com/owner/props"},
		},
	}, nil)

	// Mock lock with full metadata
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:         "props-lib",
				Ref:          "v2.0",
				CommitHash:   "abc123def456",
				VendoredAt:   "2026-01-15T10:00:00Z",
				VendoredBy:   "Alice <alice@example.com>",
				LastSyncedAt: "2026-02-01T12:00:00Z",
			},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify properties array exists
	properties, ok := comp["properties"].([]interface{})
	if !ok {
		t.Fatal("Expected 'properties' array in component")
	}

	// Build a map of property names to values for easier checking
	propMap := make(map[string]string)
	for _, p := range properties {
		prop := p.(map[string]interface{})
		name := prop["name"].(string)
		value := prop["value"].(string)
		propMap[name] = value
	}

	// Verify required properties
	expectedProps := map[string]string{
		"git-vendor:commit":        "abc123def456",
		"git-vendor:ref":           "v2.0",
		"git-vendor:vendored_at":   "2026-01-15T10:00:00Z",
		"git-vendor:vendored_by":   "Alice <alice@example.com>",
		"git-vendor:last_synced_at": "2026-02-01T12:00:00Z",
	}

	for name, expected := range expectedProps {
		if actual, exists := propMap[name]; !exists {
			t.Errorf("Missing property '%s'", name)
		} else if actual != expected {
			t.Errorf("Property '%s': expected '%s', got '%s'", name, expected, actual)
		}
	}
}

func TestGenerateCycloneDX_OmitsEmptyProperties(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "minimal-lib", URL: "https://github.com/owner/minimal"},
		},
	}, nil)

	// Mock lock with MINIMAL metadata (no vendored_at, vendored_by, last_synced_at)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "minimal-lib",
				Ref:        "main",
				CommitHash: "abc123",
				// No VendoredAt, VendoredBy, LastSyncedAt
			},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	properties := comp["properties"].([]interface{})

	// Build a map of property names
	propNames := make(map[string]bool)
	for _, p := range properties {
		prop := p.(map[string]interface{})
		propNames[prop["name"].(string)] = true
	}

	// Should have commit and ref (always present)
	if !propNames["git-vendor:commit"] {
		t.Error("Missing required property 'git-vendor:commit'")
	}
	if !propNames["git-vendor:ref"] {
		t.Error("Missing required property 'git-vendor:ref'")
	}

	// Should NOT have empty optional properties
	optionalProps := []string{"git-vendor:vendored_at", "git-vendor:vendored_by", "git-vendor:last_synced_at"}
	for _, name := range optionalProps {
		if propNames[name] {
			t.Errorf("Property '%s' should be omitted when empty", name)
		}
	}
}

// ============================================================================
// Multiple Refs Per Vendor Tests
// ============================================================================

func TestGenerateCycloneDX_VendorWithMultipleRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config - single vendor
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "multi-ref-lib", URL: "https://github.com/owner/multi-ref"},
		},
	}, nil)

	// Mock lock with SAME vendor tracking MULTIPLE refs (main, dev, v1.0)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "multi-ref-lib", Ref: "main", CommitHash: "maincommit123"},
			{Name: "multi-ref-lib", Ref: "dev", CommitHash: "devcommit456"},
			{Name: "multi-ref-lib", Ref: "v1.0", CommitHash: "v1commit789", SourceVersionTag: "v1.0.0"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
	output, err := generator.Generate(SBOMFormatCycloneDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse CycloneDX JSON: %v", err)
	}

	// Should have 3 components (one per ref)
	components := result["components"].([]interface{})
	if len(components) != 3 {
		t.Fatalf("Expected 3 components for vendor with 3 refs, got %d", len(components))
	}

	// Build map of versions to verify all refs are represented
	versions := make(map[string]bool)
	bomRefs := make(map[string]bool)
	for _, c := range components {
		comp := c.(map[string]interface{})
		versions[comp["version"].(string)] = true
		bomRefs[comp["bom-ref"].(string)] = true
	}

	// Check versions - v1.0 should use tag, others use commit hash
	if !versions["maincommit123"] {
		t.Error("Missing component with version 'maincommit123' (main branch)")
	}
	if !versions["devcommit456"] {
		t.Error("Missing component with version 'devcommit456' (dev branch)")
	}
	if !versions["v1.0.0"] {
		t.Error("Missing component with version 'v1.0.0' (v1.0 tag)")
	}

	// Check bom-refs are unique
	if len(bomRefs) != 3 {
		t.Errorf("Expected 3 unique bom-refs, got %d", len(bomRefs))
	}
}

func TestGenerateSPDX_VendorWithMultipleRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "multi-ref-lib", URL: "https://github.com/owner/multi-ref"},
		},
	}, nil)

	// Mock lock with multiple refs
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "multi-ref-lib", Ref: "main", CommitHash: "aaa111"},
			{Name: "multi-ref-lib", Ref: "feature", CommitHash: "bbb222"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
	output, err := generator.Generate(SBOMFormatSPDX)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	// Should have 2 packages
	packages := result["packages"].([]interface{})
	if len(packages) != 2 {
		t.Fatalf("Expected 2 packages for vendor with 2 refs, got %d", len(packages))
	}

	// Should have 2 relationships (one DESCRIBES per package)
	relationships := result["relationships"].([]interface{})
	if len(relationships) != 2 {
		t.Fatalf("Expected 2 relationships, got %d", len(relationships))
	}

	// Verify each relationship points to a valid package
	packageSPDXIDs := make(map[string]bool)
	for _, p := range packages {
		pkg := p.(map[string]interface{})
		packageSPDXIDs[pkg["SPDXID"].(string)] = true
	}

	for i, r := range relationships {
		rel := r.(map[string]interface{})
		target := rel["relatedSpdxElement"].(string)
		if !packageSPDXIDs[target] {
			t.Errorf("Relationship %d target '%s' does not match any package SPDXID", i, target)
		}
	}
}

// ============================================================================
// SPDX External Refs Tests
// ============================================================================

func TestGenerateSPDX_IncludesPURLExternalRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "purl-lib", URL: "https://github.com/owner/purl-test"},
		},
	}, nil)

	// Mock lock
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "purl-lib", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify externalRefs array exists
	extRefs, ok := pkg["externalRefs"].([]interface{})
	if !ok {
		t.Fatal("Expected 'externalRefs' array in package")
	}

	if len(extRefs) != 1 {
		t.Fatalf("Expected 1 external ref, got %d", len(extRefs))
	}

	// Verify PURL external ref structure
	purlRef := extRefs[0].(map[string]interface{})
	if purlRef["referenceCategory"] != "PACKAGE-MANAGER" {
		t.Errorf("Expected referenceCategory 'PACKAGE-MANAGER', got %v", purlRef["referenceCategory"])
	}

	if purlRef["referenceType"] != "purl" {
		t.Errorf("Expected referenceType 'purl', got %v", purlRef["referenceType"])
	}

	expectedPURL := "pkg:github/owner/purl-test@abc123"
	if purlRef["referenceLocator"] != expectedPURL {
		t.Errorf("Expected referenceLocator '%s', got %v", expectedPURL, purlRef["referenceLocator"])
	}
}

// ============================================================================
// SPDX Comment/Annotation Tests
// ============================================================================

func TestGenerateSPDX_IncludesMetadataComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "comment-lib", URL: "https://github.com/owner/comment"},
		},
	}, nil)

	// Mock lock with metadata
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "comment-lib",
				Ref:        "main",
				CommitHash: "abc123",
				VendoredAt: "2026-01-15T10:00:00Z",
				VendoredBy: "Bob <bob@example.com>",
			},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Verify comment contains metadata
	comment, ok := pkg["comment"].(string)
	if !ok {
		t.Fatal("Expected 'comment' field in package")
	}

	if !strings.Contains(comment, "vendored_at=2026-01-15T10:00:00Z") {
		t.Errorf("Expected comment to contain vendored_at, got: %s", comment)
	}

	if !strings.Contains(comment, "vendored_by=Bob <bob@example.com>") {
		t.Errorf("Expected comment to contain vendored_by, got: %s", comment)
	}

	if !strings.Contains(comment, "ref=main") {
		t.Errorf("Expected comment to contain ref, got: %s", comment)
	}

	if !strings.Contains(comment, "commit=abc123") {
		t.Errorf("Expected comment to contain commit, got: %s", comment)
	}
}

// ============================================================================
// Version Fallback Tests
// ============================================================================

func TestGenerateCycloneDX_UsesCommitHashWhenNoVersionTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "no-tag-lib", URL: "https://github.com/owner/no-tag"},
		},
	}, nil)

	// Mock lock WITHOUT SourceVersionTag
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "no-tag-lib", Ref: "main", CommitHash: "fullcommithash123"},
		},
	}, nil)

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test-project")
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

	// Version should fall back to commit hash when no tag
	if comp["version"] != "fullcommithash123" {
		t.Errorf("Expected version to be commit hash 'fullcommithash123', got %v", comp["version"])
	}
}
