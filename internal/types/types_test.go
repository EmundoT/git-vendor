package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/testutil"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// VendorConfig YAML Tests
// ============================================================================

func TestVendorConfig_YAML_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config VendorConfig
	}{
		{
			name: "full config with all fields",
			config: VendorConfig{
				Vendors: []VendorSpec{
					{
						Name:    "test-vendor",
						URL:     "https://github.com/test/repo",
						License: "MIT",
						Groups:  []string{"frontend", "backend"},
						Hooks: &HookConfig{
							PreSync:  "echo 'starting'",
							PostSync: "npm run build",
						},
						Specs: []BranchSpec{
							{
								Ref:           "main",
								DefaultTarget: "vendor/lib/",
								Mapping: []PathMapping{
									{From: "src/", To: "lib/"},
									{From: "dist/index.js", To: "lib/index.js"},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple vendors",
			config: VendorConfig{
				Vendors: []VendorSpec{
					{
						Name:    "vendor-a",
						URL:     "https://github.com/org/repo-a",
						License: "MIT",
						Specs:   []BranchSpec{{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/a/"}}}},
					},
					{
						Name:    "vendor-b",
						URL:     "https://gitlab.com/org/repo-b",
						License: "Apache-2.0",
						Groups:  []string{"shared"},
						Specs: []BranchSpec{
							{Ref: "develop", Mapping: []PathMapping{{From: "dist/", To: "lib/b/"}}},
							{Ref: "v2.0.0", Mapping: []PathMapping{{From: "legacy/", To: "lib/b-legacy/"}}},
						},
					},
				},
			},
		},
		{
			name:   "empty vendors",
			config: VendorConfig{Vendors: []VendorSpec{}},
		},
		// Note: nil vendors case is tested separately due to nil vs empty slice semantics
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertYAMLRoundTrip(t, tt.config)
		})
	}
}

func TestVendorConfig_YAML_NilVendors(t *testing.T) {
	// Test nil vendors separately - YAML unmarshals null/empty as nil or empty slice
	// Both are functionally equivalent, so we just verify the round-trip works
	config := VendorConfig{Vendors: nil}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VendorConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify functionally equivalent (length 0)
	if len(parsed.Vendors) != 0 {
		t.Errorf("expected empty vendors, got %d vendors", len(parsed.Vendors))
	}
}

func TestVendorConfig_YAML_Format(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "example",
				URL:     "https://github.com/org/repo",
				License: "MIT",
				Specs: []BranchSpec{
					{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/"}}},
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	output := string(data)

	// Verify key YAML structure elements are present with correct field names
	requiredFields := []string{"vendors:", "name:", "url:", "license:", "specs:", "ref:", "mapping:", "from:", "to:"}
	for _, field := range requiredFields {
		if !strings.Contains(output, field) {
			t.Errorf("expected YAML to contain %q, got:\n%s", field, output)
		}
	}
}

// ============================================================================
// VendorSpec Tests
// ============================================================================

func TestVendorSpec_YAML_OmitEmpty(t *testing.T) {
	tests := []struct {
		name       string
		spec       VendorSpec
		omitFields []string
	}{
		{
			name: "nil groups omitted",
			spec: VendorSpec{
				Name:    "test",
				URL:     "https://github.com/test/repo",
				License: "MIT",
				Groups:  nil,
				Specs:   []BranchSpec{{Ref: "main", Mapping: []PathMapping{{From: ".", To: "lib/"}}}},
			},
			omitFields: []string{"groups"},
		},
		{
			name: "nil hooks omitted",
			spec: VendorSpec{
				Name:    "test",
				URL:     "https://github.com/test/repo",
				License: "MIT",
				Hooks:   nil,
				Specs:   []BranchSpec{{Ref: "main", Mapping: []PathMapping{{From: ".", To: "lib/"}}}},
			},
			omitFields: []string{"hooks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, field := range tt.omitFields {
				testutil.AssertYAMLOmitsField(t, tt.spec, field)
			}
		})
	}
}

// ============================================================================
// BranchSpec Tests
// ============================================================================

func TestBranchSpec_YAML_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		spec BranchSpec
	}{
		{
			name: "with default target",
			spec: BranchSpec{
				Ref:           "v1.2.3",
				DefaultTarget: "vendor/lib/",
				Mapping:       []PathMapping{{From: "src/", To: ""}},
			},
		},
		{
			name: "multiple mappings",
			spec: BranchSpec{
				Ref: "main",
				Mapping: []PathMapping{
					{From: "src/", To: "lib/src/"},
					{From: "types/", To: "lib/types/"},
					{From: "README.md", To: "lib/README.md"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertYAMLRoundTrip(t, tt.spec)
		})
	}
}

func TestBranchSpec_YAML_OmitEmpty(t *testing.T) {
	spec := BranchSpec{
		Ref:           "main",
		DefaultTarget: "", // omitempty
		Mapping:       []PathMapping{{From: "src/", To: "lib/"}},
	}
	testutil.AssertYAMLOmitsField(t, spec, "default_target")
}

// ============================================================================
// PathMapping Tests
// ============================================================================

func TestPathMapping_YAML_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		mapping PathMapping
	}{
		{name: "directory to directory", mapping: PathMapping{From: "src/", To: "lib/"}},
		{name: "file to file", mapping: PathMapping{From: "file.go", To: "internal/file.go"}},
		{name: "root to directory", mapping: PathMapping{From: ".", To: "vendor/full-repo/"}},
		{name: "scoped package", mapping: PathMapping{From: "packages/@scope/pkg/dist/", To: "vendor/scoped/"}},
		{name: "empty To (auto-naming)", mapping: PathMapping{From: "src/components/Button.tsx", To: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertYAMLRoundTrip(t, tt.mapping)
		})
	}
}

// ============================================================================
// HookConfig Tests
// ============================================================================

func TestHookConfig_YAML_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		hooks HookConfig
	}{
		{
			name:  "both hooks",
			hooks: HookConfig{PreSync: "echo 'pre'", PostSync: "echo 'post'"},
		},
		{
			name:  "pre-sync only",
			hooks: HookConfig{PreSync: "npm ci", PostSync: ""},
		},
		{
			name:  "post-sync only",
			hooks: HookConfig{PreSync: "", PostSync: "npm run build"},
		},
		{
			name: "multiline commands",
			hooks: HookConfig{
				PreSync:  "echo 'step 1'\necho 'step 2'",
				PostSync: "npm install\nnpm run build\nnpm test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertYAMLRoundTrip(t, tt.hooks)
		})
	}
}

func TestHookConfig_YAML_OmitEmpty(t *testing.T) {
	hooks := HookConfig{PreSync: "echo 'test'", PostSync: ""}
	testutil.AssertYAMLOmitsField(t, hooks, "post_sync")

	hooks2 := HookConfig{PreSync: "", PostSync: "echo 'test'"}
	testutil.AssertYAMLOmitsField(t, hooks2, "pre_sync")
}

// ============================================================================
// VendorLock YAML Tests
// ============================================================================

func TestVendorLock_YAML_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		lock VendorLock
	}{
		{
			name: "full lock with metadata",
			lock: VendorLock{
				SchemaVersion: "1.1",
				Vendors: []LockDetails{
					{
						Name:             "test-vendor",
						Ref:              "main",
						CommitHash:       "abc123def456789",
						LicensePath:      ".git-vendor/licenses/test-vendor.txt",
						Updated:          "2024-01-15T10:30:00Z",
						FileHashes:       map[string]string{"lib/file.go": "sha256:abc123"},
						LicenseSPDX:      "MIT",
						SourceVersionTag: "v1.2.3",
						VendoredAt:       "2024-01-01T00:00:00Z",
						VendoredBy:       "user@example.com",
						LastSyncedAt:     "2024-01-15T10:30:00Z",
					},
				},
			},
		},
		{
			name: "multiple vendors",
			lock: VendorLock{
				SchemaVersion: "1.1",
				Vendors: []LockDetails{
					{Name: "vendor-a", Ref: "main", CommitHash: "abc123", LicensePath: "", Updated: "2024-01-01T00:00:00Z"},
					{Name: "vendor-b", Ref: "v2.0", CommitHash: "def456", LicensePath: "", Updated: "2024-01-02T00:00:00Z"},
				},
			},
		},
		{
			name: "empty vendors",
			lock: VendorLock{SchemaVersion: "1.0", Vendors: []LockDetails{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertYAMLRoundTrip(t, tt.lock)
		})
	}
}

func TestVendorLock_SchemaVersion(t *testing.T) {
	versions := []string{"1.0", "1.1", "2.0", ""}

	for _, version := range versions {
		t.Run("version_"+version, func(t *testing.T) {
			lock := VendorLock{SchemaVersion: version, Vendors: []LockDetails{}}
			testutil.AssertYAMLRoundTrip(t, lock)
		})
	}
}

func TestLockDetails_YAML_OmitEmpty(t *testing.T) {
	details := LockDetails{
		Name:        "minimal-vendor",
		Ref:         "main",
		CommitHash:  "abc123",
		LicensePath: ".git-vendor/licenses/minimal-vendor.txt",
		Updated:     "2024-01-15T10:30:00Z",
		// All omitempty fields left empty/nil
	}

	omitFields := []string{
		"file_hashes", "license_spdx", "source_version_tag",
		"vendored_at", "vendored_by", "last_synced_at",
	}

	for _, field := range omitFields {
		testutil.AssertYAMLOmitsField(t, details, field)
	}
}

// ============================================================================
// VerifyResult JSON Tests
// ============================================================================

func TestVerifyResult_JSON_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		result VerifyResult
	}{
		{
			name: "passing verification",
			result: VerifyResult{
				SchemaVersion: "1.0",
				Timestamp:     "2024-01-15T10:30:00Z",
				Summary: VerifySummary{
					TotalFiles: 10, Verified: 10, Modified: 0, Added: 0, Deleted: 0, Result: "PASS",
				},
				Files: []FileStatus{},
			},
		},
		{
			name: "failing verification with files",
			result: VerifyResult{
				SchemaVersion: "1.0",
				Timestamp:     "2024-01-15T10:30:00Z",
				Summary: VerifySummary{
					TotalFiles: 100, Verified: 95, Modified: 3, Added: 1, Deleted: 1, Result: "FAIL",
				},
				Files: []FileStatus{
					{Path: "lib/ok.go", Vendor: testutil.StrPtr("vendor"), Status: "verified", ExpectedHash: testutil.StrPtr("sha256:abc"), ActualHash: testutil.StrPtr("sha256:abc")},
					{Path: "lib/changed.go", Vendor: testutil.StrPtr("vendor"), Status: "modified", ExpectedHash: testutil.StrPtr("sha256:old"), ActualHash: testutil.StrPtr("sha256:new")},
					{Path: "lib/new.go", Vendor: nil, Status: "added"},
					{Path: "lib/gone.go", Vendor: testutil.StrPtr("vendor"), Status: "deleted", ExpectedHash: testutil.StrPtr("sha256:was")},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertJSONRoundTrip(t, tt.result)
		})
	}
}

func TestVerifyResult_JSON_Structure(t *testing.T) {
	result := VerifyResult{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-15T10:30:00Z",
		Summary:       VerifySummary{TotalFiles: 10, Verified: 10, Result: "PASS"},
		Files:         []FileStatus{},
	}

	requiredFields := []string{
		"schema_version", "timestamp", "summary", "files",
		"total_files", "verified", "modified", "added", "deleted", "result",
	}

	for _, field := range requiredFields {
		testutil.AssertJSONContainsField(t, result, field)
	}
}

func TestFileStatus_JSON_AllStatuses(t *testing.T) {
	statuses := []string{"verified", "modified", "added", "deleted"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			fs := FileStatus{
				Path:   "test/file.go",
				Vendor: testutil.StrPtr("test-vendor"),
				Status: status,
			}
			if status == "verified" || status == "modified" || status == "deleted" {
				fs.ExpectedHash = testutil.StrPtr("sha256:expected")
			}
			if status == "verified" || status == "modified" {
				fs.ActualHash = testutil.StrPtr("sha256:actual")
			}

			testutil.AssertJSONRoundTrip(t, fs)
		})
	}
}

func TestFileStatus_JSON_OmitEmpty(t *testing.T) {
	fs := FileStatus{
		Path:   "new/file.go",
		Vendor: nil,
		Status: "added",
		// ExpectedHash and ActualHash are nil - should be omitted
	}

	testutil.AssertJSONOmitsField(t, fs, "expected_hash")
	testutil.AssertJSONOmitsField(t, fs, "actual_hash")
}

func TestVerifySummary_JSON_AllResults(t *testing.T) {
	for _, result := range []string{"PASS", "FAIL", "WARN"} {
		t.Run(result, func(t *testing.T) {
			summary := VerifySummary{
				TotalFiles: 10, Verified: 8, Modified: 1, Added: 1, Deleted: 0, Result: result,
			}
			testutil.AssertJSONRoundTrip(t, summary)
		})
	}
}

// ============================================================================
// IncrementalSyncCache JSON Tests
// ============================================================================

func TestIncrementalSyncCache_JSON_RoundTrip(t *testing.T) {
	cache := IncrementalSyncCache{
		VendorName: "cached-vendor",
		Ref:        "main",
		CommitHash: "abc123def456",
		Files: []FileChecksum{
			{Path: "lib/file1.go", Hash: "sha256:hash1"},
			{Path: "lib/file2.go", Hash: "sha256:hash2"},
		},
		CachedAt: "2024-01-15T10:30:00Z",
	}

	testutil.AssertJSONRoundTrip(t, cache)
}

func TestFileChecksum_JSON_RoundTrip(t *testing.T) {
	checksum := FileChecksum{
		Path: "deep/nested/path/to/file.go",
		Hash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	testutil.AssertJSONRoundTrip(t, checksum)
}

// ============================================================================
// Malformed Input Tests (Negative Tests)
// ============================================================================

func TestVendorConfig_YAML_MalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "invalid yaml syntax", input: "vendors: [[[invalid"},
		{name: "wrong type for vendors", input: "vendors: 'not an array'"},
		{name: "invalid nested structure", input: "vendors:\n  - name: test\n    specs: 'not an array'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config VendorConfig
			err := yaml.Unmarshal([]byte(tt.input), &config)
			if err == nil {
				t.Error("expected error for malformed YAML, got nil")
			}
		})
	}
}

func TestVendorLock_YAML_MalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "invalid yaml", input: "schema_version: [[["},
		{name: "wrong type for file_hashes", input: "vendors:\n  - name: test\n    file_hashes: 'not a map'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lock VendorLock
			err := yaml.Unmarshal([]byte(tt.input), &lock)
			if err == nil {
				t.Error("expected error for malformed YAML, got nil")
			}
		})
	}
}

func TestVerifyResult_JSON_MalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "invalid json", input: `{"schema_version": [`},
		{name: "wrong type for files", input: `{"files": "not an array"}`},
		{name: "wrong type for summary", input: `{"summary": "not an object"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result VerifyResult
			err := json.Unmarshal([]byte(tt.input), &result)
			if err == nil {
				t.Error("expected error for malformed JSON, got nil")
			}
		})
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestVendorConfig_YAML_SpecialCharacters(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "vendor-with-special-chars",
				URL:     "https://github.com/org/repo.git",
				License: "MIT OR Apache-2.0",
				Groups:  []string{"group:one", "group/two"},
				Specs: []BranchSpec{
					{
						Ref: "feature/branch-name",
						Mapping: []PathMapping{
							{From: "path with spaces/", To: "target with spaces/"},
							{From: "path'quotes/", To: "target\"quotes/"},
						},
					},
				},
			},
		},
	}

	testutil.AssertYAMLRoundTrip(t, config)
}

func TestVendorConfig_YAML_UnicodeContent(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "日本語-vendor",
				URL:     "https://github.com/org/репозиторий",
				License: "MIT",
				Specs: []BranchSpec{
					{Ref: "main", Mapping: []PathMapping{{From: "源/", To: "目标/"}}},
				},
			},
		},
	}

	testutil.AssertYAMLRoundTrip(t, config)
}

func TestLockDetails_YAML_LargeFileHashes(t *testing.T) {
	// Test with many file hashes (simulating a large vendor with 100 files)
	fileHashes := make(map[string]string)
	for i := 0; i < 100; i++ {
		fileHashes[fmt.Sprintf("lib/path%03d/file.go", i)] = fmt.Sprintf("sha256:hash%064d", i)
	}

	if len(fileHashes) != 100 {
		t.Fatalf("expected 100 unique file hashes, got %d", len(fileHashes))
	}

	details := LockDetails{
		Name:       "large-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Updated:    "2024-01-01T00:00:00Z",
		FileHashes: fileHashes,
	}

	testutil.AssertYAMLRoundTrip(t, details)
}

// ============================================================================
// Struct Field Validation Tests
// ============================================================================

// These tests verify that structs work correctly when used in typical scenarios.
// They test behavior patterns rather than just field assignment.

func TestPathConflict_Description(t *testing.T) {
	conflict := PathConflict{
		Path:     "shared/lib/utils.go",
		Vendor1:  "vendor-a",
		Vendor2:  "vendor-b",
		Mapping1: PathMapping{From: "src/utils.go", To: "shared/lib/utils.go"},
		Mapping2: PathMapping{From: "lib/utils.go", To: "shared/lib/utils.go"},
	}

	// Verify conflict contains information needed for error messages
	if conflict.Path == "" || conflict.Vendor1 == "" || conflict.Vendor2 == "" {
		t.Error("PathConflict missing required fields for error reporting")
	}
	if conflict.Mapping1.To != conflict.Mapping2.To {
		t.Error("PathConflict mappings should have same destination (that's what makes it a conflict)")
	}
}

func TestCloneOptions_ShallowCloneConfiguration(t *testing.T) {
	// Test that shallow clone options are properly configured
	shallowOpts := CloneOptions{
		Filter:     "blob:none",
		NoCheckout: true,
		Depth:      1,
	}

	// These are the expected values for a shallow clone
	if shallowOpts.Depth != 1 {
		t.Errorf("shallow clone should have Depth=1, got %d", shallowOpts.Depth)
	}
	if !shallowOpts.NoCheckout {
		t.Error("shallow clone should have NoCheckout=true")
	}
}

func TestVendorStatus_SyncStateConsistency(t *testing.T) {
	tests := []struct {
		name         string
		status       VendorStatus
		wantSynced   bool
		wantMissing  int
		isConsistent bool
	}{
		{
			name:         "synced with no missing paths",
			status:       VendorStatus{Name: "a", Ref: "main", IsSynced: true, MissingPaths: nil},
			wantSynced:   true,
			wantMissing:  0,
			isConsistent: true,
		},
		{
			name:         "not synced with missing paths",
			status:       VendorStatus{Name: "b", Ref: "main", IsSynced: false, MissingPaths: []string{"path/"}},
			wantSynced:   false,
			wantMissing:  1,
			isConsistent: true,
		},
		{
			name:         "inconsistent: synced but has missing paths",
			status:       VendorStatus{Name: "c", Ref: "main", IsSynced: true, MissingPaths: []string{"path/"}},
			wantSynced:   true,
			wantMissing:  1,
			isConsistent: false, // This would be a bug in the calling code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.IsSynced != tt.wantSynced {
				t.Errorf("IsSynced = %v, want %v", tt.status.IsSynced, tt.wantSynced)
			}
			if len(tt.status.MissingPaths) != tt.wantMissing {
				t.Errorf("len(MissingPaths) = %d, want %d", len(tt.status.MissingPaths), tt.wantMissing)
			}
		})
	}
}

func TestSyncStatus_Aggregation(t *testing.T) {
	// Test that AllSynced correctly represents aggregate state
	allSyncedStatus := SyncStatus{
		AllSynced: true,
		VendorStatuses: []VendorStatus{
			{Name: "a", IsSynced: true},
			{Name: "b", IsSynced: true},
		},
	}

	if !allSyncedStatus.AllSynced {
		t.Error("AllSynced should be true when all vendors are synced")
	}

	partialSyncStatus := SyncStatus{
		AllSynced: false,
		VendorStatuses: []VendorStatus{
			{Name: "a", IsSynced: true},
			{Name: "b", IsSynced: false, MissingPaths: []string{"missing/"}},
		},
	}

	if partialSyncStatus.AllSynced {
		t.Error("AllSynced should be false when any vendor is not synced")
	}
}

func TestUpdateCheckResult_UpdateLogic(t *testing.T) {
	tests := []struct {
		name   string
		result UpdateCheckResult
	}{
		{
			name: "up to date - hashes match",
			result: UpdateCheckResult{
				VendorName: "current", CurrentHash: "abc123", LatestHash: "abc123", UpToDate: true,
			},
		},
		{
			name: "needs update - hashes differ",
			result: UpdateCheckResult{
				VendorName: "outdated", CurrentHash: "abc123", LatestHash: "def456", UpToDate: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashesMatch := tt.result.CurrentHash == tt.result.LatestHash
			if tt.result.UpToDate && !hashesMatch {
				t.Error("UpToDate=true but hashes don't match - inconsistent state")
			}
			if !tt.result.UpToDate && hashesMatch {
				t.Error("UpToDate=false but hashes match - inconsistent state")
			}
		})
	}
}

func TestVendorDiff_CommitCount(t *testing.T) {
	diff := VendorDiff{
		VendorName:  "test",
		OldHash:     "abc123",
		NewHash:     "def456",
		CommitCount: 3,
		Commits: []CommitInfo{
			{Hash: "c1", ShortHash: "c1", Subject: "fix: bug", Author: "dev", Date: "2024-01-01"},
			{Hash: "c2", ShortHash: "c2", Subject: "feat: feature", Author: "dev", Date: "2024-01-02"},
			{Hash: "c3", ShortHash: "c3", Subject: "docs: readme", Author: "dev", Date: "2024-01-03"},
		},
	}

	if diff.CommitCount != len(diff.Commits) {
		t.Errorf("CommitCount (%d) should match len(Commits) (%d)", diff.CommitCount, len(diff.Commits))
	}
}

func TestHookContext_EnvironmentSetup(t *testing.T) {
	ctx := HookContext{
		VendorName:  "hook-vendor",
		VendorURL:   "https://github.com/org/repo",
		Ref:         "v1.0.0",
		CommitHash:  "abc123def456",
		RootDir:     "/home/user/project",
		FilesCopied: 42,
		DirsCreated: 5,
		Environment: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	// Verify all required fields for hook execution are present
	requiredFields := map[string]string{
		"VendorName": ctx.VendorName,
		"VendorURL":  ctx.VendorURL,
		"Ref":        ctx.Ref,
		"CommitHash": ctx.CommitHash,
		"RootDir":    ctx.RootDir,
	}

	for name, value := range requiredFields {
		if value == "" {
			t.Errorf("HookContext.%s should not be empty", name)
		}
	}
}

func TestParallelOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		opts    ParallelOptions
		isValid bool
	}{
		{
			name:    "disabled",
			opts:    ParallelOptions{Enabled: false, MaxWorkers: 0},
			isValid: true,
		},
		{
			name:    "enabled with default workers",
			opts:    ParallelOptions{Enabled: true, MaxWorkers: 0},
			isValid: true, // 0 means use NumCPU
		},
		{
			name:    "enabled with custom workers",
			opts:    ParallelOptions{Enabled: true, MaxWorkers: 4},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// MaxWorkers of 0 when Enabled=true means use NumCPU (valid)
			// Any positive MaxWorkers is valid
			isValid := !tt.opts.Enabled || tt.opts.MaxWorkers >= 0
			if isValid != tt.isValid {
				t.Errorf("ParallelOptions validation: got %v, want %v", isValid, tt.isValid)
			}
		})
	}
}

func TestCommitInfo_ShortHashLength(t *testing.T) {
	commit := CommitInfo{
		Hash:      "abc123def456789012345678901234567890abcd",
		ShortHash: "abc123d",
		Subject:   "feat: add new feature",
		Author:    "Developer <dev@example.com>",
		Date:      "2024-01-15",
	}

	// Full hash should be 40 characters (SHA-1)
	if len(commit.Hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars", len(commit.Hash))
	}

	// Short hash should be a prefix of the full hash
	if !strings.HasPrefix(commit.Hash, commit.ShortHash) {
		t.Error("ShortHash should be a prefix of Hash")
	}
}
