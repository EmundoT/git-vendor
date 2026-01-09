package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// ============================================================================
// CheckLicense Tests
// ============================================================================

func TestGitHubLicenseChecker_CheckLicense(t *testing.T) {
	t.Skip("Requires refactoring github_client.go to accept configurable API base URL")

	// This test creates a mock server but CheckLicense() hardcodes api.github.com,
	// so it hits the real GitHub API and can fail with rate limits.
	// To properly test this, we'd need to refactor CheckLicense to accept
	// a configurable API base URL parameter.
}

func TestGitHubLicenseChecker_CheckLicense_WithMockServer(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		statusCode      int
		responseBody    string
		expectedLicense string
		expectError     bool
	}{
		{
			name:            "MIT License",
			url:             "https://github.com/owner/repo",
			statusCode:      http.StatusOK,
			responseBody:    `{"license": {"spdx_id": "MIT"}}`,
			expectedLicense: "MIT",
			expectError:     false,
		},
		{
			name:            "Apache License",
			url:             "https://github.com/owner/repo",
			statusCode:      http.StatusOK,
			responseBody:    `{"license": {"spdx_id": "Apache-2.0"}}`,
			expectedLicense: "Apache-2.0",
			expectError:     false,
		},
		{
			name:            "No License (404)",
			url:             "https://github.com/owner/repo",
			statusCode:      http.StatusNotFound,
			responseBody:    ``,
			expectedLicense: "NONE",
			expectError:     false,
		},
		{
			name:            "Unknown License (empty SPDX ID)",
			url:             "https://github.com/owner/repo",
			statusCode:      http.StatusOK,
			responseBody:    `{"license": {"spdx_id": ""}}`,
			expectedLicense: "UNKNOWN",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock GitHub API server
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				callCount++
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Override the GitHub API URL by using httpClient that redirects to test server
			// This is a limitation - we'd need to refactor github_client.go to accept apiBaseURL
			// For now, these tests document the expected behavior
			_ = server
			t.Skip("Requires refactoring github_client.go to accept configurable API base URL")
		})
	}
}

func TestGitHubLicenseChecker_CheckLicense_InvalidURL(t *testing.T) {
	checker := NewGitHubLicenseChecker(nil, []string{"MIT"})

	tests := []struct {
		name string
		url  string
	}{
		{"Empty URL", ""},
		{"Invalid format", "not-a-url"},
		{"Non-GitHub URL", "https://gitlab.com/owner/repo"},
		{"Missing repo", "https://github.com/owner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checker.CheckLicense(tt.url)
			if err == nil {
				t.Error("Expected error for invalid URL, got nil")
			}
			if !contains(err.Error(), "invalid URL format") {
				t.Errorf("Expected 'invalid URL format' error, got: %v", err)
			}
		})
	}
}

func TestGitHubLicenseChecker_CheckLicense_WithToken(t *testing.T) {
	// Test that GITHUB_TOKEN environment variable is used
	oldToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if oldToken != "" {
			os.Setenv("GITHUB_TOKEN", oldToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test-token-123")

	// Mock server to verify Authorization header
	tokenReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenReceived = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"license": {"spdx_id": "MIT"}}`))
	}))
	defer server.Close()

	// Note: This test also requires refactoring to use configurable API URL
	_ = tokenReceived
	_ = server
	t.Skip("Requires refactoring github_client.go to accept configurable API base URL")
}

// ============================================================================
// IsAllowed Tests
// ============================================================================

func TestGitHubLicenseChecker_IsAllowed(t *testing.T) {
	allowedLicenses := []string{"MIT", "Apache-2.0", "BSD-3-Clause"}
	checker := NewGitHubLicenseChecker(nil, allowedLicenses)

	tests := []struct {
		name     string
		license  string
		expected bool
	}{
		{"MIT allowed", "MIT", true},
		{"Apache allowed", "Apache-2.0", true},
		{"BSD allowed", "BSD-3-Clause", true},
		{"GPL not allowed", "GPL-3.0", false},
		{"Unknown not allowed", "UNKNOWN", false},
		{"NONE not allowed", "NONE", false},
		{"Empty not allowed", "", false},
		{"Case sensitive", "mit", false}, // Lowercase not in list
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.IsAllowed(tt.license)
			if result != tt.expected {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.license, result, tt.expected)
			}
		})
	}
}

func TestGitHubLicenseChecker_IsAllowed_EmptyList(t *testing.T) {
	checker := NewGitHubLicenseChecker(nil, []string{})

	// With empty allowed list, nothing should be allowed
	if checker.IsAllowed("MIT") {
		t.Error("IsAllowed(\"MIT\") should be false with empty allowed list")
	}
}

func TestGitHubLicenseChecker_IsAllowed_NilList(t *testing.T) {
	checker := NewGitHubLicenseChecker(nil, nil)

	// With nil allowed list, nothing should be allowed
	if checker.IsAllowed("MIT") {
		t.Error("IsAllowed(\"MIT\") should be false with nil allowed list")
	}
}

// ============================================================================
// NewGitHubLicenseChecker Tests
// ============================================================================

func TestNewGitHubLicenseChecker(t *testing.T) {
	t.Run("With custom HTTP client", func(t *testing.T) {
		customClient := &http.Client{}
		checker := NewGitHubLicenseChecker(customClient, []string{"MIT"})

		if checker.httpClient != customClient {
			t.Error("Expected custom HTTP client to be used")
		}
	})

	t.Run("With nil HTTP client", func(t *testing.T) {
		checker := NewGitHubLicenseChecker(nil, []string{"MIT"})

		if checker.httpClient != http.DefaultClient {
			t.Error("Expected http.DefaultClient to be used when nil is passed")
		}
	})

	t.Run("With allowed licenses", func(t *testing.T) {
		allowedLicenses := []string{"MIT", "Apache-2.0"}
		checker := NewGitHubLicenseChecker(nil, allowedLicenses)

		if len(checker.allowedLicenses) != 2 {
			t.Errorf("Expected 2 allowed licenses, got %d", len(checker.allowedLicenses))
		}
	})
}
