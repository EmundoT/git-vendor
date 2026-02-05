package types

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// ========== VendorConfig YAML Tests ==========

func TestVendorConfig_YAML_RoundTrip(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "test-vendor",
				URL:     "https://github.com/test/repo",
				License: "MIT",
				Groups:  []string{"frontend", "backend"},
				Specs: []BranchSpec{
					{
						Ref: "main",
						Mapping: []PathMapping{
							{From: "src/", To: "lib/"},
						},
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var parsed VendorConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(parsed.Vendors) != 1 {
		t.Fatalf("expected 1 vendor, got %d", len(parsed.Vendors))
	}
	if parsed.Vendors[0].Name != config.Vendors[0].Name {
		t.Errorf("expected name %q, got %q", config.Vendors[0].Name, parsed.Vendors[0].Name)
	}
	if parsed.Vendors[0].URL != config.Vendors[0].URL {
		t.Errorf("expected URL %q, got %q", config.Vendors[0].URL, parsed.Vendors[0].URL)
	}
	if parsed.Vendors[0].License != config.Vendors[0].License {
		t.Errorf("expected license %q, got %q", config.Vendors[0].License, parsed.Vendors[0].License)
	}
	if len(parsed.Vendors[0].Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(parsed.Vendors[0].Groups))
	}
	if parsed.Vendors[0].Groups[0] != "frontend" || parsed.Vendors[0].Groups[1] != "backend" {
		t.Errorf("groups mismatch: got %v", parsed.Vendors[0].Groups)
	}
}

func TestVendorConfig_YAML_WithHooks(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "hooked-vendor",
				URL:     "https://github.com/test/repo",
				License: "Apache-2.0",
				Hooks: &HookConfig{
					PreSync:  "echo 'starting sync'",
					PostSync: "npm install && npm run build",
				},
				Specs: []BranchSpec{
					{
						Ref: "v1.0.0",
						Mapping: []PathMapping{
							{From: "dist/", To: "vendor/lib/"},
						},
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var parsed VendorConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if parsed.Vendors[0].Hooks == nil {
		t.Fatal("expected hooks to be present")
	}
	if parsed.Vendors[0].Hooks.PreSync != config.Vendors[0].Hooks.PreSync {
		t.Errorf("PreSync mismatch: expected %q, got %q", config.Vendors[0].Hooks.PreSync, parsed.Vendors[0].Hooks.PreSync)
	}
	if parsed.Vendors[0].Hooks.PostSync != config.Vendors[0].Hooks.PostSync {
		t.Errorf("PostSync mismatch: expected %q, got %q", config.Vendors[0].Hooks.PostSync, parsed.Vendors[0].Hooks.PostSync)
	}
}

func TestVendorConfig_YAML_MultipleVendors(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{
			{
				Name:    "vendor-a",
				URL:     "https://github.com/org/repo-a",
				License: "MIT",
				Specs: []BranchSpec{
					{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/a/"}}},
				},
			},
			{
				Name:    "vendor-b",
				URL:     "https://gitlab.com/org/repo-b",
				License: "BSD-3-Clause",
				Groups:  []string{"shared"},
				Specs: []BranchSpec{
					{Ref: "develop", Mapping: []PathMapping{{From: "dist/", To: "lib/b/"}}},
					{Ref: "v2.0.0", Mapping: []PathMapping{{From: "legacy/", To: "lib/b-legacy/"}}},
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VendorConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Vendors) != 2 {
		t.Fatalf("expected 2 vendors, got %d", len(parsed.Vendors))
	}
	if parsed.Vendors[0].Name != "vendor-a" {
		t.Errorf("expected first vendor 'vendor-a', got %q", parsed.Vendors[0].Name)
	}
	if parsed.Vendors[1].Name != "vendor-b" {
		t.Errorf("expected second vendor 'vendor-b', got %q", parsed.Vendors[1].Name)
	}
	if len(parsed.Vendors[1].Specs) != 2 {
		t.Errorf("expected 2 specs for vendor-b, got %d", len(parsed.Vendors[1].Specs))
	}
}

func TestVendorConfig_EmptyVendors(t *testing.T) {
	config := VendorConfig{
		Vendors: []VendorSpec{},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VendorConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Vendors) != 0 {
		t.Errorf("expected 0 vendors, got %d", len(parsed.Vendors))
	}
}

func TestVendorConfig_NilVendors(t *testing.T) {
	config := VendorConfig{
		Vendors: nil,
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VendorConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// nil vendors should marshal as empty/null and unmarshal as nil or empty
	if len(parsed.Vendors) != 0 {
		t.Errorf("expected nil or empty vendors, got %v", parsed.Vendors)
	}
}

// ========== VendorSpec Tests ==========

func TestVendorSpec_NoGroups(t *testing.T) {
	spec := VendorSpec{
		Name:    "no-groups-vendor",
		URL:     "https://github.com/test/repo",
		License: "MIT",
		Groups:  nil, // omitempty should exclude this
		Specs: []BranchSpec{
			{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/"}}},
		},
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify that "groups:" is not in the output when empty/nil
	if containsField(string(data), "groups:") {
		t.Error("expected 'groups' to be omitted when nil")
	}

	var parsed VendorSpec
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Groups) != 0 {
		t.Errorf("expected nil or empty groups, got %v", parsed.Groups)
	}
}

func TestVendorSpec_EmptyGroups(t *testing.T) {
	spec := VendorSpec{
		Name:    "empty-groups-vendor",
		URL:     "https://github.com/test/repo",
		License: "MIT",
		Groups:  []string{}, // empty slice
		Specs: []BranchSpec{
			{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/"}}},
		},
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VendorSpec
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Empty slice might be preserved or become nil depending on YAML library
	if len(parsed.Groups) != 0 {
		t.Errorf("expected empty groups, got %v", parsed.Groups)
	}
}

// ========== PathMapping Tests ==========

func TestPathMapping_EmptyTo(t *testing.T) {
	mapping := PathMapping{
		From: "src/components/Button.tsx",
		To:   "", // Auto-naming will be used
	}

	data, err := yaml.Marshal(mapping)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed PathMapping
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.From != mapping.From {
		t.Errorf("From mismatch: expected %q, got %q", mapping.From, parsed.From)
	}
	if parsed.To != "" {
		t.Errorf("expected empty To, got %q", parsed.To)
	}
}

func TestPathMapping_ComplexPaths(t *testing.T) {
	testCases := []PathMapping{
		{From: ".", To: "vendor/full-repo/"},
		{From: "src/lib/utils/", To: "lib/external/utils/"},
		{From: "packages/@scope/package/dist/", To: "vendor/scoped/"},
		{From: "file.go", To: "internal/vendored/file.go"},
	}

	for _, tc := range testCases {
		data, err := yaml.Marshal(tc)
		if err != nil {
			t.Errorf("failed to marshal %v: %v", tc, err)
			continue
		}

		var parsed PathMapping
		err = yaml.Unmarshal(data, &parsed)
		if err != nil {
			t.Errorf("failed to unmarshal %v: %v", tc, err)
			continue
		}

		if parsed.From != tc.From || parsed.To != tc.To {
			t.Errorf("round-trip failed for %v: got %v", tc, parsed)
		}
	}
}

// ========== BranchSpec Tests ==========

func TestBranchSpec_WithDefaultTarget(t *testing.T) {
	spec := BranchSpec{
		Ref:           "v1.2.3",
		DefaultTarget: "vendor/lib/",
		Mapping: []PathMapping{
			{From: "src/", To: ""},
		},
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed BranchSpec
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.DefaultTarget != spec.DefaultTarget {
		t.Errorf("DefaultTarget mismatch: expected %q, got %q", spec.DefaultTarget, parsed.DefaultTarget)
	}
}

func TestBranchSpec_NoDefaultTarget(t *testing.T) {
	spec := BranchSpec{
		Ref:           "main",
		DefaultTarget: "", // omitempty should exclude this
		Mapping: []PathMapping{
			{From: "src/", To: "lib/"},
		},
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify that "default_target:" is not in the output when empty
	if containsField(string(data), "default_target:") {
		t.Error("expected 'default_target' to be omitted when empty")
	}
}

// ========== HookConfig Tests ==========

func TestHookConfig_NilHooks(t *testing.T) {
	spec := VendorSpec{
		Name:    "no-hooks-vendor",
		URL:     "https://github.com/test/repo",
		License: "MIT",
		Hooks:   nil, // omitempty should exclude this
		Specs: []BranchSpec{
			{Ref: "main", Mapping: []PathMapping{{From: "src/", To: "lib/"}}},
		},
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify that "hooks:" is not in the output when nil
	if containsField(string(data), "hooks:") {
		t.Error("expected 'hooks' to be omitted when nil")
	}

	var parsed VendorSpec
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Hooks != nil {
		t.Errorf("expected nil hooks, got %v", parsed.Hooks)
	}
}

func TestHookConfig_PreSyncOnly(t *testing.T) {
	hooks := HookConfig{
		PreSync:  "echo 'pre-sync only'",
		PostSync: "", // omitempty
	}

	data, err := yaml.Marshal(hooks)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed HookConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PreSync != hooks.PreSync {
		t.Errorf("PreSync mismatch: expected %q, got %q", hooks.PreSync, parsed.PreSync)
	}
	if parsed.PostSync != "" {
		t.Errorf("expected empty PostSync, got %q", parsed.PostSync)
	}
}

func TestHookConfig_PostSyncOnly(t *testing.T) {
	hooks := HookConfig{
		PreSync:  "", // omitempty
		PostSync: "npm run build",
	}

	data, err := yaml.Marshal(hooks)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed HookConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PreSync != "" {
		t.Errorf("expected empty PreSync, got %q", parsed.PreSync)
	}
	if parsed.PostSync != hooks.PostSync {
		t.Errorf("PostSync mismatch: expected %q, got %q", hooks.PostSync, parsed.PostSync)
	}
}

func TestHookConfig_MultilineCommands(t *testing.T) {
	hooks := HookConfig{
		PreSync: "echo 'step 1'\necho 'step 2'\necho 'step 3'",
		PostSync: `npm install
npm run build
npm run test`,
	}

	data, err := yaml.Marshal(hooks)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed HookConfig
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PreSync != hooks.PreSync {
		t.Errorf("PreSync multiline mismatch:\nexpected: %q\ngot: %q", hooks.PreSync, parsed.PreSync)
	}
	if parsed.PostSync != hooks.PostSync {
		t.Errorf("PostSync multiline mismatch:\nexpected: %q\ngot: %q", hooks.PostSync, parsed.PostSync)
	}
}

// ========== VendorLock YAML Tests ==========

func TestVendorLock_YAML_RoundTrip(t *testing.T) {
	lock := VendorLock{
		SchemaVersion: "1.1",
		Vendors: []LockDetails{
			{
				Name:        "test-vendor",
				Ref:         "main",
				CommitHash:  "abc123def456789",
				LicensePath: "vendor/licenses/test-vendor.txt",
				Updated:     "2024-01-15T10:30:00Z",
				FileHashes: map[string]string{
					"lib/file1.go": "sha256:abc123",
					"lib/file2.go": "sha256:def456",
				},
				LicenseSPDX:      "MIT",
				SourceVersionTag: "v1.2.3",
				VendoredAt:       "2024-01-01T00:00:00Z",
				VendoredBy:       "user@example.com",
				LastSyncedAt:     "2024-01-15T10:30:00Z",
			},
		},
	}

	data, err := yaml.Marshal(lock)
	if err != nil {
		t.Fatalf("failed to marshal lock: %v", err)
	}

	var parsed VendorLock
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal lock: %v", err)
	}

	if parsed.SchemaVersion != lock.SchemaVersion {
		t.Errorf("SchemaVersion mismatch: expected %q, got %q", lock.SchemaVersion, parsed.SchemaVersion)
	}
	if len(parsed.Vendors) != 1 {
		t.Fatalf("expected 1 vendor, got %d", len(parsed.Vendors))
	}

	v := parsed.Vendors[0]
	if v.Name != lock.Vendors[0].Name {
		t.Errorf("Name mismatch: expected %q, got %q", lock.Vendors[0].Name, v.Name)
	}
	if v.CommitHash != lock.Vendors[0].CommitHash {
		t.Errorf("CommitHash mismatch: expected %q, got %q", lock.Vendors[0].CommitHash, v.CommitHash)
	}
	if v.LicenseSPDX != lock.Vendors[0].LicenseSPDX {
		t.Errorf("LicenseSPDX mismatch: expected %q, got %q", lock.Vendors[0].LicenseSPDX, v.LicenseSPDX)
	}
	if len(v.FileHashes) != 2 {
		t.Errorf("expected 2 file hashes, got %d", len(v.FileHashes))
	}
}

func TestVendorLock_SchemaVersion(t *testing.T) {
	testCases := []struct {
		version string
	}{
		{"1.0"},
		{"1.1"},
		{"2.0"},
		{""},
	}

	for _, tc := range testCases {
		lock := VendorLock{
			SchemaVersion: tc.version,
			Vendors:       []LockDetails{},
		}

		data, err := yaml.Marshal(lock)
		if err != nil {
			t.Errorf("failed to marshal with version %q: %v", tc.version, err)
			continue
		}

		var parsed VendorLock
		err = yaml.Unmarshal(data, &parsed)
		if err != nil {
			t.Errorf("failed to unmarshal with version %q: %v", tc.version, err)
			continue
		}

		if parsed.SchemaVersion != tc.version {
			t.Errorf("version mismatch: expected %q, got %q", tc.version, parsed.SchemaVersion)
		}
	}
}

func TestLockDetails_YAML_OmitEmpty(t *testing.T) {
	// Test that omitempty fields are properly excluded
	details := LockDetails{
		Name:        "minimal-vendor",
		Ref:         "main",
		CommitHash:  "abc123",
		LicensePath: "vendor/licenses/minimal-vendor.txt",
		Updated:     "2024-01-15T10:30:00Z",
		// All omitempty fields left empty/nil
	}

	data, err := yaml.Marshal(details)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	yamlStr := string(data)

	// These fields should be omitted
	omitFields := []string{
		"file_hashes:",
		"license_spdx:",
		"source_version_tag:",
		"vendored_at:",
		"vendored_by:",
		"last_synced_at:",
	}

	for _, field := range omitFields {
		if containsField(yamlStr, field) {
			t.Errorf("expected %q to be omitted from YAML output", field)
		}
	}
}

func TestLockDetails_WithAllMetadata(t *testing.T) {
	details := LockDetails{
		Name:             "full-metadata-vendor",
		Ref:              "v2.0.0",
		CommitHash:       "deadbeef12345678",
		LicensePath:      "vendor/licenses/full-metadata-vendor.txt",
		Updated:          "2024-06-01T12:00:00Z",
		FileHashes:       map[string]string{"path/file.go": "hash123"},
		LicenseSPDX:      "Apache-2.0",
		SourceVersionTag: "v2.0.0",
		VendoredAt:       "2024-05-01T00:00:00Z",
		VendoredBy:       "developer@company.com",
		LastSyncedAt:     "2024-06-01T12:00:00Z",
	}

	data, err := yaml.Marshal(details)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed LockDetails
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all metadata fields preserved
	if parsed.LicenseSPDX != details.LicenseSPDX {
		t.Errorf("LicenseSPDX mismatch")
	}
	if parsed.SourceVersionTag != details.SourceVersionTag {
		t.Errorf("SourceVersionTag mismatch")
	}
	if parsed.VendoredAt != details.VendoredAt {
		t.Errorf("VendoredAt mismatch")
	}
	if parsed.VendoredBy != details.VendoredBy {
		t.Errorf("VendoredBy mismatch")
	}
	if parsed.LastSyncedAt != details.LastSyncedAt {
		t.Errorf("LastSyncedAt mismatch")
	}
}

// ========== VerifyResult JSON Tests ==========

func TestVerifyResult_JSON_RoundTrip(t *testing.T) {
	result := VerifyResult{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-15T10:30:00Z",
		Summary: VerifySummary{
			TotalFiles: 100,
			Verified:   95,
			Modified:   3,
			Added:      1,
			Deleted:    1,
			Result:     "FAIL",
		},
		Files: []FileStatus{
			{
				Path:         "lib/file1.go",
				Vendor:       strPtr("test-vendor"),
				Status:       "verified",
				ExpectedHash: strPtr("sha256:abc123"),
				ActualHash:   strPtr("sha256:abc123"),
			},
			{
				Path:         "lib/file2.go",
				Vendor:       strPtr("test-vendor"),
				Status:       "modified",
				ExpectedHash: strPtr("sha256:original"),
				ActualHash:   strPtr("sha256:changed"),
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed VerifyResult
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.SchemaVersion != result.SchemaVersion {
		t.Errorf("SchemaVersion mismatch")
	}
	if parsed.Summary.TotalFiles != result.Summary.TotalFiles {
		t.Errorf("TotalFiles mismatch: expected %d, got %d", result.Summary.TotalFiles, parsed.Summary.TotalFiles)
	}
	if parsed.Summary.Result != result.Summary.Result {
		t.Errorf("Result mismatch: expected %q, got %q", result.Summary.Result, parsed.Summary.Result)
	}
	if len(parsed.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(parsed.Files))
	}
}

func TestVerifyResult_JSON_Structure(t *testing.T) {
	result := VerifyResult{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-15T10:30:00Z",
		Summary: VerifySummary{
			TotalFiles: 10,
			Verified:   10,
			Modified:   0,
			Added:      0,
			Deleted:    0,
			Result:     "PASS",
		},
		Files: []FileStatus{},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify JSON structure contains expected fields
	jsonStr := string(data)
	expectedFields := []string{
		`"schema_version"`,
		`"timestamp"`,
		`"summary"`,
		`"files"`,
		`"total_files"`,
		`"verified"`,
		`"modified"`,
		`"added"`,
		`"deleted"`,
		`"result"`,
	}

	for _, field := range expectedFields {
		if !containsField(jsonStr, field) {
			t.Errorf("expected JSON to contain %s", field)
		}
	}
}

func TestFileStatus_AllStatuses(t *testing.T) {
	statuses := []string{"verified", "modified", "added", "deleted"}

	for _, status := range statuses {
		fs := FileStatus{
			Path:   "test/file.go",
			Vendor: strPtr("test-vendor"),
			Status: status,
		}

		if status == "verified" || status == "modified" {
			fs.ExpectedHash = strPtr("sha256:expected")
			fs.ActualHash = strPtr("sha256:actual")
		}

		data, err := json.Marshal(fs)
		if err != nil {
			t.Errorf("failed to marshal status %q: %v", status, err)
			continue
		}

		var parsed FileStatus
		err = json.Unmarshal(data, &parsed)
		if err != nil {
			t.Errorf("failed to unmarshal status %q: %v", status, err)
			continue
		}

		if parsed.Status != status {
			t.Errorf("status mismatch: expected %q, got %q", status, parsed.Status)
		}
	}
}

func TestFileStatus_JSON_OmitEmpty(t *testing.T) {
	// FileStatus with nil optional fields
	fs := FileStatus{
		Path:   "new/file.go",
		Vendor: nil, // added files may not have a vendor
		Status: "added",
		// ExpectedHash and ActualHash are nil
	}

	data, err := json.Marshal(fs)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)

	// These fields have omitempty, but vendor is not omitempty so it will be null
	if containsField(jsonStr, `"expected_hash"`) {
		t.Error("expected 'expected_hash' to be omitted")
	}
	if containsField(jsonStr, `"actual_hash"`) {
		t.Error("expected 'actual_hash' to be omitted")
	}
}

func TestVerifySummary_AllResults(t *testing.T) {
	results := []string{"PASS", "FAIL", "WARN"}

	for _, result := range results {
		summary := VerifySummary{
			TotalFiles: 10,
			Verified:   8,
			Modified:   1,
			Added:      1,
			Deleted:    0,
			Result:     result,
		}

		data, err := json.Marshal(summary)
		if err != nil {
			t.Errorf("failed to marshal result %q: %v", result, err)
			continue
		}

		var parsed VerifySummary
		err = json.Unmarshal(data, &parsed)
		if err != nil {
			t.Errorf("failed to unmarshal result %q: %v", result, err)
			continue
		}

		if parsed.Result != result {
			t.Errorf("result mismatch: expected %q, got %q", result, parsed.Result)
		}
	}
}

// ========== IncrementalSyncCache JSON Tests ==========

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

	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed IncrementalSyncCache
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.VendorName != cache.VendorName {
		t.Errorf("VendorName mismatch")
	}
	if parsed.CommitHash != cache.CommitHash {
		t.Errorf("CommitHash mismatch")
	}
	if len(parsed.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(parsed.Files))
	}
	if parsed.Files[0].Hash != cache.Files[0].Hash {
		t.Errorf("File hash mismatch")
	}
}

func TestFileChecksum_JSON_RoundTrip(t *testing.T) {
	checksum := FileChecksum{
		Path: "deep/nested/path/to/file.go",
		Hash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	data, err := json.Marshal(checksum)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed FileChecksum
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Path != checksum.Path {
		t.Errorf("Path mismatch: expected %q, got %q", checksum.Path, parsed.Path)
	}
	if parsed.Hash != checksum.Hash {
		t.Errorf("Hash mismatch: expected %q, got %q", checksum.Hash, parsed.Hash)
	}
}

// ========== PathConflict Tests ==========

func TestPathConflict_Fields(t *testing.T) {
	conflict := PathConflict{
		Path:    "shared/lib/utils.go",
		Vendor1: "vendor-a",
		Vendor2: "vendor-b",
		Mapping1: PathMapping{
			From: "src/utils.go",
			To:   "shared/lib/utils.go",
		},
		Mapping2: PathMapping{
			From: "lib/utils.go",
			To:   "shared/lib/utils.go",
		},
	}

	if conflict.Path != "shared/lib/utils.go" {
		t.Errorf("Path mismatch")
	}
	if conflict.Vendor1 != "vendor-a" {
		t.Errorf("Vendor1 mismatch")
	}
	if conflict.Vendor2 != "vendor-b" {
		t.Errorf("Vendor2 mismatch")
	}
	if conflict.Mapping1.From != "src/utils.go" {
		t.Errorf("Mapping1.From mismatch")
	}
	if conflict.Mapping2.From != "lib/utils.go" {
		t.Errorf("Mapping2.From mismatch")
	}
}

// ========== CloneOptions Tests ==========

func TestCloneOptions_Defaults(t *testing.T) {
	opts := CloneOptions{}

	if opts.Filter != "" {
		t.Errorf("expected empty Filter, got %q", opts.Filter)
	}
	if opts.NoCheckout != false {
		t.Error("expected NoCheckout to be false")
	}
	if opts.Depth != 0 {
		t.Errorf("expected Depth 0, got %d", opts.Depth)
	}
}

func TestCloneOptions_ShallowClone(t *testing.T) {
	opts := CloneOptions{
		Filter:     "blob:none",
		NoCheckout: true,
		Depth:      1,
	}

	if opts.Filter != "blob:none" {
		t.Errorf("Filter mismatch")
	}
	if !opts.NoCheckout {
		t.Error("expected NoCheckout to be true")
	}
	if opts.Depth != 1 {
		t.Errorf("expected Depth 1, got %d", opts.Depth)
	}
}

// ========== VendorStatus Tests ==========

func TestVendorStatus_Synced(t *testing.T) {
	status := VendorStatus{
		Name:         "synced-vendor",
		Ref:          "main",
		IsSynced:     true,
		MissingPaths: nil,
	}

	if !status.IsSynced {
		t.Error("expected IsSynced to be true")
	}
	if len(status.MissingPaths) != 0 {
		t.Errorf("expected no missing paths, got %v", status.MissingPaths)
	}
}

func TestVendorStatus_NotSynced(t *testing.T) {
	status := VendorStatus{
		Name:     "unsynced-vendor",
		Ref:      "develop",
		IsSynced: false,
		MissingPaths: []string{
			"lib/module1/",
			"lib/module2/file.go",
		},
	}

	if status.IsSynced {
		t.Error("expected IsSynced to be false")
	}
	if len(status.MissingPaths) != 2 {
		t.Errorf("expected 2 missing paths, got %d", len(status.MissingPaths))
	}
}

func TestSyncStatus_AllSynced(t *testing.T) {
	status := SyncStatus{
		AllSynced: true,
		VendorStatuses: []VendorStatus{
			{Name: "vendor-a", Ref: "main", IsSynced: true},
			{Name: "vendor-b", Ref: "main", IsSynced: true},
		},
	}

	if !status.AllSynced {
		t.Error("expected AllSynced to be true")
	}
	if len(status.VendorStatuses) != 2 {
		t.Errorf("expected 2 vendor statuses, got %d", len(status.VendorStatuses))
	}
}

func TestSyncStatus_NotAllSynced(t *testing.T) {
	status := SyncStatus{
		AllSynced: false,
		VendorStatuses: []VendorStatus{
			{Name: "vendor-a", Ref: "main", IsSynced: true},
			{Name: "vendor-b", Ref: "main", IsSynced: false, MissingPaths: []string{"missing/"}},
		},
	}

	if status.AllSynced {
		t.Error("expected AllSynced to be false")
	}
}

// ========== UpdateCheckResult Tests ==========

func TestUpdateCheckResult_UpToDate(t *testing.T) {
	result := UpdateCheckResult{
		VendorName:  "current-vendor",
		Ref:         "main",
		CurrentHash: "abc123",
		LatestHash:  "abc123",
		LastUpdated: "2024-01-15T10:30:00Z",
		UpToDate:    true,
	}

	if !result.UpToDate {
		t.Error("expected UpToDate to be true")
	}
	if result.CurrentHash != result.LatestHash {
		t.Error("expected hashes to match when up to date")
	}
}

func TestUpdateCheckResult_NeedsUpdate(t *testing.T) {
	result := UpdateCheckResult{
		VendorName:  "outdated-vendor",
		Ref:         "main",
		CurrentHash: "abc123",
		LatestHash:  "def456",
		LastUpdated: "2024-01-15T10:30:00Z",
		UpToDate:    false,
	}

	if result.UpToDate {
		t.Error("expected UpToDate to be false")
	}
	if result.CurrentHash == result.LatestHash {
		t.Error("expected hashes to differ when update needed")
	}
}

// ========== CommitInfo Tests ==========

func TestCommitInfo_Fields(t *testing.T) {
	commit := CommitInfo{
		Hash:      "abc123def456789012345678901234567890abcd",
		ShortHash: "abc123d",
		Subject:   "feat: add new feature",
		Author:    "Developer <dev@example.com>",
		Date:      "2024-01-15",
	}

	if len(commit.Hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars", len(commit.Hash))
	}
	if commit.ShortHash != "abc123d" {
		t.Errorf("ShortHash mismatch")
	}
	if commit.Subject != "feat: add new feature" {
		t.Errorf("Subject mismatch")
	}
}

// ========== VendorDiff Tests ==========

func TestVendorDiff_WithCommits(t *testing.T) {
	diff := VendorDiff{
		VendorName:  "diff-vendor",
		Ref:         "main",
		OldHash:     "abc123",
		NewHash:     "def456",
		OldDate:     "2024-01-01",
		NewDate:     "2024-01-15",
		CommitCount: 3,
		Commits: []CommitInfo{
			{Hash: "commit1", ShortHash: "c1", Subject: "fix: bug 1", Author: "dev1", Date: "2024-01-05"},
			{Hash: "commit2", ShortHash: "c2", Subject: "feat: feature 1", Author: "dev2", Date: "2024-01-10"},
			{Hash: "commit3", ShortHash: "c3", Subject: "docs: update readme", Author: "dev1", Date: "2024-01-15"},
		},
	}

	if diff.CommitCount != len(diff.Commits) {
		t.Errorf("CommitCount mismatch: count=%d, actual=%d", diff.CommitCount, len(diff.Commits))
	}
	if diff.OldHash == diff.NewHash {
		t.Error("expected different old and new hashes")
	}
}

func TestVendorDiff_NoChanges(t *testing.T) {
	diff := VendorDiff{
		VendorName:  "unchanged-vendor",
		Ref:         "main",
		OldHash:     "abc123",
		NewHash:     "abc123",
		OldDate:     "2024-01-01",
		NewDate:     "2024-01-01",
		CommitCount: 0,
		Commits:     []CommitInfo{},
	}

	if diff.CommitCount != 0 {
		t.Errorf("expected 0 commits, got %d", diff.CommitCount)
	}
	if diff.OldHash != diff.NewHash {
		t.Error("expected same hashes when no changes")
	}
}

// ========== HookContext Tests ==========

func TestHookContext_AllFields(t *testing.T) {
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

	if ctx.VendorName != "hook-vendor" {
		t.Errorf("VendorName mismatch")
	}
	if ctx.FilesCopied != 42 {
		t.Errorf("FilesCopied mismatch: expected 42, got %d", ctx.FilesCopied)
	}
	if ctx.DirsCreated != 5 {
		t.Errorf("DirsCreated mismatch: expected 5, got %d", ctx.DirsCreated)
	}
	if ctx.Environment["CUSTOM_VAR"] != "custom_value" {
		t.Errorf("Environment variable mismatch")
	}
}

func TestHookContext_EmptyEnvironment(t *testing.T) {
	ctx := HookContext{
		VendorName:  "minimal-hook-vendor",
		VendorURL:   "https://github.com/org/repo",
		Ref:         "main",
		CommitHash:  "abc123",
		RootDir:     "/project",
		FilesCopied: 0,
		DirsCreated: 0,
		Environment: nil,
	}

	if ctx.Environment != nil {
		t.Errorf("expected nil Environment, got %v", ctx.Environment)
	}
}

// ========== ParallelOptions Tests ==========

func TestParallelOptions_Disabled(t *testing.T) {
	opts := ParallelOptions{
		Enabled:    false,
		MaxWorkers: 0,
	}

	if opts.Enabled {
		t.Error("expected Enabled to be false")
	}
}

func TestParallelOptions_Enabled(t *testing.T) {
	opts := ParallelOptions{
		Enabled:    true,
		MaxWorkers: 4,
	}

	if !opts.Enabled {
		t.Error("expected Enabled to be true")
	}
	if opts.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers 4, got %d", opts.MaxWorkers)
	}
}

func TestParallelOptions_DefaultWorkers(t *testing.T) {
	opts := ParallelOptions{
		Enabled:    true,
		MaxWorkers: 0, // 0 means use NumCPU
	}

	if opts.MaxWorkers != 0 {
		t.Errorf("expected MaxWorkers 0 (default), got %d", opts.MaxWorkers)
	}
}

// ========== Helper Functions ==========

// containsField checks if a string contains a field pattern (case-sensitive)
func containsField(s, field string) bool {
	return len(s) > 0 && len(field) > 0 && contains(s, field)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
