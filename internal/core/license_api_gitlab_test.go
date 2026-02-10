package core

import (
	"testing"
)

func TestExtractGitLabProjectPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPath    string
		wantHost    string
		wantErr     bool
	}{
		{
			name:     "standard gitlab.com project",
			input:    "https://gitlab.com/owner/repo",
			wantPath: "owner/repo",
			wantHost: "gitlab.com",
		},
		{
			name:     "nested group project",
			input:    "https://gitlab.com/owner/group/repo",
			wantPath: "owner/group/repo",
			wantHost: "gitlab.com",
		},
		{
			name:     "deeply nested groups",
			input:    "https://gitlab.com/org/team/sub/project",
			wantPath: "org/team/sub/project",
			wantHost: "gitlab.com",
		},
		{
			name:     "self-hosted GitLab",
			input:    "https://gitlab.example.com/team/project",
			wantPath: "team/project",
			wantHost: "gitlab.example.com",
		},
		{
			name:     "trailing .git suffix stripped",
			input:    "https://gitlab.com/owner/repo.git",
			wantPath: "owner/repo",
			wantHost: "gitlab.com",
		},
		{
			name:     "trailing slash stripped",
			input:    "https://gitlab.com/owner/repo/",
			wantPath: "owner/repo",
			wantHost: "gitlab.com",
		},
		{
			name:     "http protocol",
			input:    "http://gitlab.com/owner/repo",
			wantPath: "owner/repo",
			wantHost: "gitlab.com",
		},
		{
			name:    "missing project path",
			input:   "https://gitlab.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, host, err := extractGitLabProjectPath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
		})
	}
}

func TestNormalizeLicenseKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "UNKNOWN"},
		{"mit", "MIT"},
		{"MIT", "MIT"},
		{"apache-2.0", "Apache-2.0"},
		{"apache_2_0", "Apache-2.0"},
		{"Apache 2.0", "Apache-2.0"},
		{"bsd-3-clause", "BSD-3-Clause"},
		{"bsd_3_clause", "BSD-3-Clause"},
		{"bsd-2-clause", "BSD-2-Clause"},
		{"gpl-3.0", "GPL-3.0"},
		{"gpl_3_0", "GPL-3.0"},
		{"gpl-3.0-only", "GPL-3.0"},
		{"gpl-2.0", "GPL-2.0"},
		{"gpl_2_0", "GPL-2.0"},
		{"gpl-2.0-only", "GPL-2.0"},
		{"mpl-2.0", "MPL-2.0"},
		{"mpl_2_0", "MPL-2.0"},
		{"mozilla public license 2.0", "MPL-2.0"},
		{"isc", "ISC"},
		{"unlicense", "Unlicense"},
		{"cc0-1.0", "CC0-1.0"},
		{"cc0_1_0", "CC0-1.0"},
		{"custom-license", "Custom-license"}, // default: uppercase first letter
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLicenseKey(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLicenseKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
