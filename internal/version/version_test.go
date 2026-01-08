package version

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"development build", "dev", "dev"},
		{"release v1.0.0", "v1.0.0", "v1.0.0"},
		{"release v0.1.0-beta.1", "v0.1.0-beta.1", "v0.1.0-beta.1"},
		{"release v2.3.4", "v2.3.4", "v2.3.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalVersion := Version
			defer func() { Version = originalVersion }()

			// Set test value
			Version = tt.version

			result := GetVersion()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetFullVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
	}{
		{
			name:    "development build",
			version: "dev",
			commit:  "none",
			date:    "unknown",
		},
		{
			name:    "release build",
			version: "v1.0.0",
			commit:  "abc123def456",
			date:    "2024-12-27T10:30:00Z",
		},
		{
			name:    "beta release",
			version: "v0.1.0-beta.1",
			commit:  "fedcba654321",
			date:    "2024-01-15T09:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalVersion := Version
			originalCommit := Commit
			originalDate := Date
			defer func() {
				Version = originalVersion
				Commit = originalCommit
				Date = originalDate
			}()

			// Set test values
			Version = tt.version
			Commit = tt.commit
			Date = tt.date

			result := GetFullVersion()

			// Verify format: "version (commit: hash, built: date)"
			expected := tt.version + " (commit: " + tt.commit + ", built: " + tt.date + ")"
			if result != expected {
				t.Errorf("Expected '%s', got '%s'", expected, result)
			}

			// Verify it contains all parts
			if !strings.Contains(result, tt.version) {
				t.Errorf("Result should contain version '%s'", tt.version)
			}
			if !strings.Contains(result, tt.commit) {
				t.Errorf("Result should contain commit '%s'", tt.commit)
			}
			if !strings.Contains(result, tt.date) {
				t.Errorf("Result should contain date '%s'", tt.date)
			}
		})
	}
}

func TestGetFullVersion_Format(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
	}()

	// Set known values
	Version = "v1.2.3"
	Commit = "abcdef123456"
	Date = "2024-12-25T12:00:00Z"

	result := GetFullVersion()

	// Should have format: "version (commit: hash, built: date)"
	if !strings.HasPrefix(result, "v1.2.3 (") {
		t.Error("Should start with version followed by '('")
	}
	if !strings.Contains(result, "commit: abcdef123456") {
		t.Error("Should contain 'commit: hash'")
	}
	if !strings.Contains(result, "built: 2024-12-25T12:00:00Z") {
		t.Error("Should contain 'built: date'")
	}
	if !strings.HasSuffix(result, ")") {
		t.Error("Should end with ')'")
	}
}
