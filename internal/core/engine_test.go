package core

import (
	"testing"
)

func TestParseSmartURL(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name        string
		input       string
		wantURL     string
		wantRef     string
		wantPath    string
		description string
	}{
		{
			name:        "Basic GitHub URL",
			input:       "https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should extract base URL with no ref or path",
		},
		{
			name:        "GitHub URL with .git suffix",
			input:       "https://github.com/owner/repo.git",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove .git suffix",
		},
		{
			name:        "GitHub blob URL with main branch",
			input:       "https://github.com/owner/repo/blob/main/path/to/file.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/to/file.go",
			description: "Should extract repo, ref, and file path from blob URL",
		},
		{
			name:        "GitHub tree URL with branch",
			input:       "https://github.com/owner/repo/tree/dev/src/components",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "dev",
			wantPath:    "src/components",
			description: "Should extract repo, ref, and directory path from tree URL",
		},
		{
			name:        "GitHub blob URL with version tag",
			input:       "https://github.com/owner/repo/blob/v1.0.0/README.md",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "v1.0.0",
			wantPath:    "README.md",
			description: "Should handle version tags as refs",
		},
		{
			name:        "GitHub blob URL with commit hash",
			input:       "https://github.com/owner/repo/blob/abc123def456/src/main.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "abc123def456",
			wantPath:    "src/main.go",
			description: "Should handle commit hashes as refs",
		},
		{
			name:        "GitHub tree URL with nested path",
			input:       "https://github.com/owner/repo/tree/main/deeply/nested/path/to/dir",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "deeply/nested/path/to/dir",
			description: "Should handle deeply nested paths",
		},
		{
			name:        "URL with trailing slash",
			input:       "https://github.com/owner/repo/",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should handle trailing slash",
		},
		{
			name:        "URL with spaces (trimmed)",
			input:       "  https://github.com/owner/repo  ",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should trim whitespace",
		},
		{
			name:        "URL with backslash prefix",
			input:       "\\https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove leading backslash",
		},
		{
			name:        "Blob URL with file containing special characters",
			input:       "https://github.com/owner/repo/blob/main/path/file-name_v2.test.js",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/file-name_v2.test.js",
			description: "Should handle filenames with hyphens and underscores",
		},
		// Note: Branch names with slashes (e.g., feature/new-feature) are not currently supported
		// in deep link parsing due to regex limitations. Users should manually enter such refs.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotRef, gotPath := m.ParseSmartURL(tt.input)

			if gotURL != tt.wantURL {
				t.Errorf("ParseSmartURL() URL = %v, want %v\nDescription: %s", gotURL, tt.wantURL, tt.description)
			}
			if gotRef != tt.wantRef {
				t.Errorf("ParseSmartURL() Ref = %v, want %v\nDescription: %s", gotRef, tt.wantRef, tt.description)
			}
			if gotPath != tt.wantPath {
				t.Errorf("ParseSmartURL() Path = %v, want %v\nDescription: %s", gotPath, tt.wantPath, tt.description)
			}
		})
	}
}

func TestIsLicenseAllowed(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{"MIT is allowed", "MIT", true},
		{"Apache-2.0 is allowed", "Apache-2.0", true},
		{"BSD-3-Clause is allowed", "BSD-3-Clause", true},
		{"BSD-2-Clause is allowed", "BSD-2-Clause", true},
		{"ISC is allowed", "ISC", true},
		{"Unlicense is allowed", "Unlicense", true},
		{"CC0-1.0 is allowed", "CC0-1.0", true},
		{"GPL is not allowed by default", "GPL-3.0", false},
		{"Unknown license is not allowed", "UNKNOWN", false},
		{"Empty string is not allowed", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.isLicenseAllowed(tt.license)
			if got != tt.expected {
				t.Errorf("isLicenseAllowed(%q) = %v, want %v", tt.license, got, tt.expected)
			}
		})
	}
}
