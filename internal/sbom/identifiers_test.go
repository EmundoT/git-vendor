package sbom

import (
	"strings"
	"testing"
)

func TestVendorIdentity_ShortHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected string
	}{
		{"full hash", "abc1234567890def", "abc1234"},
		{"exactly 7 chars", "1234567", "1234567"},
		{"less than 7 chars", "abc", "abc"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := VendorIdentity{CommitHash: tc.hash}
			if v.ShortHash() != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, v.ShortHash())
			}
		})
	}
}

func TestGenerateBOMRef(t *testing.T) {
	tests := []struct {
		name     string
		identity VendorIdentity
		expected string
	}{
		{
			name:     "standard",
			identity: VendorIdentity{Name: "my-lib", CommitHash: "abc1234567890"},
			expected: "my-lib@abc1234",
		},
		{
			name:     "short hash",
			identity: VendorIdentity{Name: "lib", CommitHash: "abc"},
			expected: "lib@abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateBOMRef(tc.identity)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestGenerateBOMRef_Uniqueness(t *testing.T) {
	// Same vendor, different commits = different BOM refs
	v1 := VendorIdentity{Name: "lib", Ref: "main", CommitHash: "aaa1111"}
	v2 := VendorIdentity{Name: "lib", Ref: "dev", CommitHash: "bbb2222"}

	ref1 := GenerateBOMRef(v1)
	ref2 := GenerateBOMRef(v2)

	if ref1 == ref2 {
		t.Errorf("BOM refs should be different for different commits: %q == %q", ref1, ref2)
	}
}

func TestGenerateSPDXID(t *testing.T) {
	tests := []struct {
		name     string
		identity VendorIdentity
		expected string
	}{
		{
			name:     "standard",
			identity: VendorIdentity{Name: "my-lib", CommitHash: "abc1234567890"},
			expected: "Package-my-lib-abc1234",
		},
		{
			name:     "special characters sanitized",
			identity: VendorIdentity{Name: "lib@special/chars!", CommitHash: "def5678"},
			expected: "Package-lib-special-chars--def5678",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateSPDXID(tc.identity)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestGenerateSPDXID_Uniqueness(t *testing.T) {
	// Same vendor, different commits = different SPDX IDs
	v1 := VendorIdentity{Name: "lib", Ref: "main", CommitHash: "aaa1111"}
	v2 := VendorIdentity{Name: "lib", Ref: "dev", CommitHash: "bbb2222"}

	id1 := GenerateSPDXID(v1)
	id2 := GenerateSPDXID(v2)

	if id1 == id2 {
		t.Errorf("SPDX IDs should be different for different commits: %q == %q", id1, id2)
	}
}

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
		{"", "unknown"}, // Empty returns "unknown"
		{"   ", "---"},  // Whitespace is replaced
		{"emoji🎉", "emoji-"},
		{"汉字", "--"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := SanitizeSPDXID(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeSPDXID(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatSPDXRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DOCUMENT", "SPDXRef-DOCUMENT"},
		{"Package-my-lib-abc1234", "SPDXRef-Package-my-lib-abc1234"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := FormatSPDXRef(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestExtractSupplier(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectNil    bool
		expectedName string
	}{
		{
			name:         "GitHub URL",
			url:          "https://github.com/kubernetes/kubernetes",
			expectedName: "kubernetes",
		},
		{
			name:         "GitLab URL",
			url:          "https://gitlab.com/gitlab-org/gitlab",
			expectedName: "gitlab-org",
		},
		{
			name:         "Bitbucket URL",
			url:          "https://bitbucket.org/atlassian/stash",
			expectedName: "atlassian",
		},
		{
			name:      "Empty URL",
			url:       "",
			expectNil: true,
		},
		{
			name:      "Unknown host",
			url:       "https://internal.corp/team/repo",
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractSupplier(tc.url)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil supplier")
			}

			if result.Name != tc.expectedName {
				t.Errorf("Expected name %q, got %q", tc.expectedName, result.Name)
			}

			if result.URL != tc.url {
				t.Errorf("Expected URL %q, got %q", tc.url, result.URL)
			}
		})
	}
}

func TestMetadataComment(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		commit     string
		vendoredAt string
		vendoredBy string
		contains   []string
		excludes   []string
	}{
		{
			name:       "all fields",
			ref:        "main",
			commit:     "abc123",
			vendoredAt: "2026-01-15",
			vendoredBy: "user@example.com",
			contains:   []string{"ref=main", "commit=abc123", "vendored_at=2026-01-15", "vendored_by=user@example.com"},
		},
		{
			name:     "only required fields",
			ref:      "v1.0",
			commit:   "def456",
			contains: []string{"ref=v1.0", "commit=def456"},
			excludes: []string{"vendored_at=", "vendored_by="},
		},
		{
			name:       "partial optional fields",
			ref:        "main",
			commit:     "abc123",
			vendoredAt: "2026-01-15",
			contains:   []string{"ref=main", "commit=abc123", "vendored_at=2026-01-15"},
			excludes:   []string{"vendored_by="},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MetadataComment(tc.ref, tc.commit, tc.vendoredAt, tc.vendoredBy)

			for _, s := range tc.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Expected comment to contain %q, got %q", s, result)
				}
			}

			for _, s := range tc.excludes {
				if strings.Contains(result, s) {
					t.Errorf("Expected comment to NOT contain %q, got %q", s, result)
				}
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-project", "my-project"},
		{"  trimmed  ", "trimmed"},
		{"", DefaultProjectName},
		{"   ", DefaultProjectName},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ValidateProjectName(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestBuildSPDXNamespace(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		projectName string
		uuid        string
		expected    string
	}{
		{
			name:        "custom base URL",
			baseURL:     "https://example.com/spdx",
			projectName: "my-project",
			uuid:        "abc-123",
			expected:    "https://example.com/spdx/my-project/abc-123",
		},
		{
			name:        "default base URL",
			baseURL:     "",
			projectName: "my-project",
			uuid:        "abc-123",
			expected:    "https://spdx.org/spdxdocs/my-project/abc-123",
		},
		{
			name:        "trailing slash in base URL",
			baseURL:     "https://example.com/spdx/",
			projectName: "project",
			uuid:        "xyz",
			expected:    "https://example.com/spdx/project/xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildSPDXNamespace(tc.baseURL, tc.projectName, tc.uuid)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// Edge case: Very long vendor name
func TestSanitizeSPDXID_LongName(t *testing.T) {
	longName := strings.Repeat("a", 1000)
	result := SanitizeSPDXID(longName)

	if len(result) != 1000 {
		t.Errorf("Expected length 1000, got %d", len(result))
	}

	// Verify all characters are valid
	for i, r := range result {
		if !isValidSPDXChar(r) {
			t.Errorf("Invalid character at position %d: %c", i, r)
		}
	}
}
