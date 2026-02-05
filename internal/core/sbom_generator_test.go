package core

import (
	"encoding/json"
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
	lockStore.EXPECT().Load().Return(types.VendorLock{}, &mockError{msg: "lockfile not found"})

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
	configStore.EXPECT().Load().Return(types.VendorConfig{}, &mockError{msg: "config not found"})

	generator := NewSBOMGenerator(lockStore, configStore, fs, "test")
	_, err := generator.Generate(SBOMFormatCycloneDX)

	if err == nil {
		t.Fatal("Expected error when config fails to load")
	}

	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("Expected 'load config' in error, got: %v", err)
	}
}

// mockError is a simple error implementation for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
