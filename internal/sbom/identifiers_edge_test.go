package sbom

import (
	"strings"
	"testing"
)

// TestSanitizeSPDXID_OnlySpecialCharacters verifies that strings composed
// entirely of invalid SPDX characters are replaced with hyphens.
func TestSanitizeSPDXID_OnlySpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"symbols only", "!@#$%^&*()", "----------"},
		{"slashes only", "///", "---"},
		{"underscores only", "___", "---"},
		{"single special char", "!", "-"},
		{"mixed punctuation", "@#$!%", "-----"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeSPDXID(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeSPDXID(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestSanitizeSPDXID_UnicodeNames covers vendor names with various Unicode scripts.
// All non-ASCII characters are replaced with hyphens.
func TestSanitizeSPDXID_UnicodeNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Japanese", "my-\u30e9\u30a4\u30d6\u30e9\u30ea"},
		{"Chinese", "\u6d4b\u8bd5\u5e93"},
		{"Arabic", "\u0645\u0643\u062a\u0628\u0629"},
		{"Korean", "\ud504\ub85c\uc81d\ud2b8"},
		{"mixed ASCII and CJK", "lib-\u30e9\u30a4\u30d6-v2"},
		{"Cyrillic", "\u043f\u0440\u043e\u0435\u043a\u0442"},
		{"emoji only", "\U0001f680\U0001f525\U0001f4a5"},
		{"accented Latin", "caf\u00e9-lib-r\u00e9sum\u00e9"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeSPDXID(tc.input)
			// Result must only contain valid SPDX characters
			for i, r := range result {
				if !isValidSPDXChar(r) {
					t.Errorf("Invalid character %q at position %d in result %q", r, i, result)
				}
			}
			// Result must not be empty (replaced chars become hyphens)
			if result == "" {
				t.Error("SanitizeSPDXID should never return empty for non-empty input")
			}
		})
	}
}

// TestSanitizeSPDXID_Truncation verifies that names exceeding MaxSPDXIDLength
// are truncated correctly.
func TestSanitizeSPDXID_Truncation(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
	}{
		{"exactly at limit", strings.Repeat("a", MaxSPDXIDLength), MaxSPDXIDLength},
		{"one over limit", strings.Repeat("b", MaxSPDXIDLength+1), MaxSPDXIDLength},
		{"far over limit", strings.Repeat("c", 5000), MaxSPDXIDLength},
		// Unicode chars expand to multi-byte hyphens; length is measured in bytes
		{"unicode expands to hyphens", strings.Repeat("\u00e9", 200), MaxSPDXIDLength},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeSPDXID(tc.input)
			if len(result) > MaxSPDXIDLength {
				t.Errorf("Length %d exceeds MaxSPDXIDLength %d for result %q",
					len(result), MaxSPDXIDLength, result)
			}
		})
	}
}

// TestSanitizeSPDXID_NumbersOnly verifies that all-numeric strings are valid.
func TestSanitizeSPDXID_NumbersOnly(t *testing.T) {
	result := SanitizeSPDXID("1234567890")
	if result != "1234567890" {
		t.Errorf("Expected %q, got %q", "1234567890", result)
	}
}

// TestSanitizeSPDXID_DotsAndDashesOnly verifies that strings with only dots
// and dashes pass through unchanged (both are valid SPDX chars).
func TestSanitizeSPDXID_DotsAndDashesOnly(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"---", "---"},
		{"...", "..."},
		{"-.-.-", "-.-.-"},
		{"a.b-c", "a.b-c"},
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

// TestGenerateSPDXID_LongVendorName verifies that GenerateSPDXID handles
// vendor names that would exceed MaxSPDXIDLength after sanitization.
// The sanitized name is truncated, then "-{short-hash}" is appended.
func TestGenerateSPDXID_LongVendorName(t *testing.T) {
	v := VendorIdentity{
		Name:       strings.Repeat("long-name-", 50), // 500 chars
		CommitHash: "abc1234567890",
	}
	result := GenerateSPDXID(v)

	// Result format: "Package-{sanitized}-{short-hash}"
	// The sanitized part is truncated to MaxSPDXIDLength
	if !strings.HasPrefix(result, "Package-") {
		t.Errorf("Expected 'Package-' prefix, got %q", result)
	}
	if !strings.HasSuffix(result, "-abc1234") {
		t.Errorf("Expected '-abc1234' suffix, got %q", result)
	}

	// All characters must be valid SPDX
	for i, r := range result {
		if !isValidSPDXChar(r) {
			t.Errorf("Invalid character %q at position %d in %q", r, i, result)
		}
	}
}

// TestGenerateBOMRef_EmptyFields covers GenerateBOMRef behavior with
// empty name and/or hash.
func TestGenerateBOMRef_EmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		identity VendorIdentity
		expected string
	}{
		{
			name:     "empty hash",
			identity: VendorIdentity{Name: "my-lib", CommitHash: ""},
			expected: "my-lib@",
		},
		{
			name:     "empty name",
			identity: VendorIdentity{Name: "", CommitHash: "abc1234"},
			expected: "@abc1234",
		},
		{
			name:     "both empty",
			identity: VendorIdentity{Name: "", CommitHash: ""},
			expected: "@",
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

// TestGenerateBOMRef_UniquenessAcrossMultipleCalls verifies that different
// commits always produce different BOM refs, even with the same vendor name.
func TestGenerateBOMRef_UniquenessAcrossMultipleCalls(t *testing.T) {
	identities := []VendorIdentity{
		{Name: "lib", Ref: "main", CommitHash: "aaa1111222233334444"},
		{Name: "lib", Ref: "dev", CommitHash: "bbb2222333344445555"},
		{Name: "lib", Ref: "v1.0", CommitHash: "ccc3333444455556666"},
		{Name: "lib", Ref: "v2.0", CommitHash: "ddd4444555566667777"},
	}

	refs := make(map[string]bool)
	for _, v := range identities {
		ref := GenerateBOMRef(v)
		if refs[ref] {
			t.Errorf("Duplicate BOM ref %q generated for commit %q", ref, v.CommitHash)
		}
		refs[ref] = true
	}

	if len(refs) != len(identities) {
		t.Errorf("Expected %d unique refs, got %d", len(identities), len(refs))
	}
}

// TestExtractSupplier_AllProviderTypes verifies ExtractSupplier for GitHub,
// GitLab, and Bitbucket URLs (including self-hosted instances).
func TestExtractSupplier_AllProviderTypes(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectNil    bool
		expectedName string
		expectedURL  string
	}{
		{
			name:         "GitHub standard",
			url:          "https://github.com/kubernetes/kubernetes",
			expectedName: "kubernetes",
			expectedURL:  "https://github.com/kubernetes/kubernetes",
		},
		{
			name:         "GitLab standard",
			url:          "https://gitlab.com/gitlab-org/gitlab",
			expectedName: "gitlab-org",
			expectedURL:  "https://gitlab.com/gitlab-org/gitlab",
		},
		{
			name:         "Bitbucket standard",
			url:          "https://bitbucket.org/atlassian/stash",
			expectedName: "atlassian",
			expectedURL:  "https://bitbucket.org/atlassian/stash",
		},
		{
			name:         "GitHub Enterprise",
			url:          "https://github.mycompany.com/team/project",
			expectedName: "team",
			expectedURL:  "https://github.mycompany.com/team/project",
		},
		{
			name:         "self-hosted GitLab",
			url:          "https://gitlab.internal.corp/infra/platform",
			expectedName: "infra",
			expectedURL:  "https://gitlab.internal.corp/infra/platform",
		},
		{
			name:         "self-hosted GitLab nested groups",
			url:          "https://gitlab.internal.corp/org/team/subteam/project",
			expectedName: "org/team/subteam",
			expectedURL:  "https://gitlab.internal.corp/org/team/subteam/project",
		},
		{
			name:         "GitHub with .git suffix",
			url:          "https://github.com/owner/repo.git",
			expectedName: "owner",
			expectedURL:  "https://github.com/owner/repo.git",
		},
		{
			name:      "unknown provider returns nil",
			url:       "https://git.internal.corp/team/repo",
			expectNil: true,
		},
		{
			name:      "empty URL returns nil",
			url:       "",
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
				t.Fatal("Expected non-nil SupplierInfo")
			}
			if result.Name != tc.expectedName {
				t.Errorf("Name: expected %q, got %q", tc.expectedName, result.Name)
			}
			if result.URL != tc.expectedURL {
				t.Errorf("URL: expected %q, got %q", tc.expectedURL, result.URL)
			}
		})
	}
}

// TestBuildSPDXNamespace_FormatCompliance verifies that generated namespaces
// conform to the expected URI format.
func TestBuildSPDXNamespace_FormatCompliance(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		projectName string
		uuid        string
		validate    func(t *testing.T, result string)
	}{
		{
			name:        "standard format is a valid URI",
			baseURL:     "https://example.com/spdx",
			projectName: "my-project",
			uuid:        "550e8400-e29b-41d4-a716-446655440000",
			validate: func(t *testing.T, result string) {
				if !strings.HasPrefix(result, "https://") {
					t.Errorf("Namespace should start with https://, got %q", result)
				}
				if !strings.Contains(result, "my-project") {
					t.Errorf("Namespace should contain project name, got %q", result)
				}
				if !strings.HasSuffix(result, "550e8400-e29b-41d4-a716-446655440000") {
					t.Errorf("Namespace should end with UUID, got %q", result)
				}
			},
		},
		{
			name:        "no double slashes between components",
			baseURL:     "https://example.com/spdx/",
			projectName: "project",
			uuid:        "abc-123",
			validate: func(t *testing.T, result string) {
				// After "https://", there should be no "//" in the path
				afterScheme := strings.TrimPrefix(result, "https://")
				if strings.Contains(afterScheme, "//") {
					t.Errorf("Namespace has double slashes: %q", result)
				}
			},
		},
		{
			name:        "multiple trailing slashes trimmed",
			baseURL:     "https://example.com///",
			projectName: "project",
			uuid:        "xyz",
			validate: func(t *testing.T, result string) {
				afterScheme := strings.TrimPrefix(result, "https://")
				if strings.Contains(afterScheme, "//") {
					t.Errorf("Namespace has double slashes: %q", result)
				}
			},
		},
		{
			name:        "empty project name",
			baseURL:     "",
			projectName: "",
			uuid:        "abc-123",
			validate: func(t *testing.T, result string) {
				expected := DefaultSPDXNamespace + "//abc-123"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:        "special characters in project name",
			baseURL:     "",
			projectName: "my project@v2",
			uuid:        "abc",
			validate: func(t *testing.T, result string) {
				// BuildSPDXNamespace does not sanitize; caller responsibility
				if !strings.Contains(result, "my project@v2") {
					t.Errorf("Expected project name preserved, got %q", result)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildSPDXNamespace(tc.baseURL, tc.projectName, tc.uuid)
			tc.validate(t, result)
		})
	}
}

// TestMetadataComment_AllEmptyFields verifies that MetadataComment with
// all empty strings returns an empty string.
func TestMetadataComment_AllEmptyFields(t *testing.T) {
	result := MetadataComment("", "", "", "")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

// TestMetadataComment_SingleField verifies each field in isolation.
func TestMetadataComment_SingleField(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		commit     string
		vendoredAt string
		vendoredBy string
		expected   string
	}{
		{"ref only", "main", "", "", "", "ref=main"},
		{"commit only", "", "abc123", "", "", "commit=abc123"},
		{"vendoredAt only", "", "", "2026-01-15", "", "vendored_at=2026-01-15"},
		{"vendoredBy only", "", "", "", "user@example.com", "vendored_by=user@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MetadataComment(tc.ref, tc.commit, tc.vendoredAt, tc.vendoredBy)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestFormatSPDXRef_EdgeCases covers FormatSPDXRef with edge-case inputs.
func TestFormatSPDXRef_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "SPDXRef-"},
		{"DOCUMENT", "SPDXRef-DOCUMENT"},
		{"Package-my-lib-abc1234", "SPDXRef-Package-my-lib-abc1234"},
		// Already has prefix — FormatSPDXRef does NOT check for double prefix
		{"SPDXRef-DOCUMENT", "SPDXRef-SPDXRef-DOCUMENT"},
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

// TestGenerateSPDXID_EmptyName verifies GenerateSPDXID when vendor name is empty.
// SanitizeSPDXID returns "unknown" for empty input.
func TestGenerateSPDXID_EmptyName(t *testing.T) {
	v := VendorIdentity{Name: "", CommitHash: "abc1234567890"}
	result := GenerateSPDXID(v)
	expected := "Package-unknown-abc1234"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestGenerateSPDXID_UnicodeVendorName verifies that unicode vendor names
// are sanitized in the SPDX ID.
func TestGenerateSPDXID_UnicodeVendorName(t *testing.T) {
	v := VendorIdentity{
		Name:       "\u30e9\u30a4\u30d6\u30e9\u30ea",
		CommitHash: "abc1234567890",
	}
	result := GenerateSPDXID(v)

	if !strings.HasPrefix(result, "Package-") {
		t.Errorf("Expected 'Package-' prefix, got %q", result)
	}
	// Unicode chars replaced with hyphens; hash suffix still present
	if !strings.HasSuffix(result, "-abc1234") {
		t.Errorf("Expected '-abc1234' suffix, got %q", result)
	}

	// Validate all characters are valid SPDX
	for i, r := range result {
		if !isValidSPDXChar(r) {
			t.Errorf("Invalid character %q at position %d in %q", r, i, result)
		}
	}
}

// TestGenerateSPDXID_Uniqueness_MultipleRefs verifies SPDX ID uniqueness
// across 4+ vendor identities with same name but different commits.
func TestGenerateSPDXID_Uniqueness_MultipleRefs(t *testing.T) {
	identities := []VendorIdentity{
		{Name: "shared-lib", Ref: "main", CommitHash: "aaa1111222233334444"},
		{Name: "shared-lib", Ref: "dev", CommitHash: "bbb2222333344445555"},
		{Name: "shared-lib", Ref: "v1.0", CommitHash: "ccc3333444455556666"},
		{Name: "shared-lib", Ref: "v2.0", CommitHash: "ddd4444555566667777"},
		{Name: "shared-lib", Ref: "hotfix", CommitHash: "eee5555666677778888"},
	}

	ids := make(map[string]bool)
	for _, v := range identities {
		id := GenerateSPDXID(v)
		if ids[id] {
			t.Errorf("Duplicate SPDX ID %q for commit %q", id, v.CommitHash)
		}
		ids[id] = true
	}

	if len(ids) != len(identities) {
		t.Errorf("Expected %d unique IDs, got %d", len(identities), len(ids))
	}
}

// TestValidateProjectName_EdgeCases covers additional edge cases for
// ValidateProjectName.
func TestValidateProjectName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tabs and newlines", "\t\n", DefaultProjectName},
		{"mixed whitespace", " \t name \n ", "name"},
		{"unicode spaces", "\u00a0", DefaultProjectName}, // non-breaking space is NOT trimmed by TrimSpace in all Go versions
		{"single char", "x", "x"},
		{"very long name preserved", strings.Repeat("a", 500), strings.Repeat("a", 500)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateProjectName(tc.input)
			if result != tc.expected {
				t.Errorf("ValidateProjectName(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
