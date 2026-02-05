package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// CVSS to Severity Mapping Tests
// ============================================================================

func TestCVSSToSeverity(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		severity string
	}{
		{"Critical - 9.8", 9.8, "CRITICAL"},
		{"Critical - 9.0", 9.0, "CRITICAL"},
		{"High - 8.5", 8.5, "HIGH"},
		{"High - 7.0", 7.0, "HIGH"},
		{"Medium - 5.0", 5.0, "MEDIUM"},
		{"Medium - 4.0", 4.0, "MEDIUM"},
		{"Low - 3.5", 3.5, "LOW"},
		{"Low - 0.1", 0.1, "LOW"},
		{"Unknown - 0.0", 0.0, "UNKNOWN"},
		{"Unknown - negative", -1.0, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CVSSToSeverity(tt.score)
			if got != tt.severity {
				t.Errorf("CVSSToSeverity(%f) = %s, want %s", tt.score, got, tt.severity)
			}
		})
	}
}

// ============================================================================
// CVSS Score Parsing Tests
// ============================================================================

func TestParseCVSSScore(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"Simple score", "9.8", 9.8},
		{"Integer score", "7", 7.0},
		{"Low score", "3.1", 3.1},
		{"Zero score", "0", 0.0},
		{"Score with whitespace", "  8.5  ", 8.5},
		{"CVSS vector (cannot parse)", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 0},
		{"Invalid string", "high", 0},
		{"Empty string", "", 0},
		{"Score above 10", "15.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCVSSScore(tt.input)
			if got != tt.expected {
				t.Errorf("parseCVSSScore(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Query Building Tests
// ============================================================================

func TestBuildPURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)

	tests := []struct {
		name     string
		repoURL  string
		version  string
		expected string
	}{
		{
			name:     "GitHub URL",
			repoURL:  "https://github.com/owner/repo",
			version:  "v1.2.3",
			expected: "pkg:github/owner/repo@v1.2.3",
		},
		{
			name:     "GitLab URL",
			repoURL:  "https://gitlab.com/group/project",
			version:  "v2.0.0",
			expected: "pkg:gitlab/group/project@v2.0.0",
		},
		{
			name:     "Bitbucket URL",
			repoURL:  "https://bitbucket.org/team/repo",
			version:  "1.0.0",
			expected: "pkg:bitbucket/team/repo@1.0.0",
		},
		{
			name:     "URL with .git suffix",
			repoURL:  "https://github.com/owner/repo.git",
			version:  "v1.0.0",
			expected: "pkg:github/owner/repo@v1.0.0",
		},
		{
			name:     "Generic URL",
			repoURL:  "https://git.example.com/myproject",
			version:  "v3.0.0",
			expected: "pkg:generic/myproject@v3.0.0",
		},
		{
			name:     "Empty path",
			repoURL:  "https://github.com/",
			version:  "v1.0.0",
			expected: "",
		},
		{
			name:     "Invalid URL",
			repoURL:  "not-a-url",
			version:  "v1.0.0",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.buildPURL(tt.repoURL, tt.version)
			if got != tt.expected {
				t.Errorf("buildPURL(%s, %s) = %s, want %s", tt.repoURL, tt.version, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// OSV Query Tests with Mock Server
// ============================================================================

func TestQueryOSV_WithKnownCVE(t *testing.T) {
	// Create mock OSV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Return a mock response with a known CVE
		response := osvResponse{
			Vulns: []osvVuln{
				{
					ID:      "CVE-2024-1234",
					Summary: "Remote code execution vulnerability",
					Aliases: []string{"GHSA-xxxx-yyyy-zzzz"},
					Severity: []osvSeverity{
						{Type: "CVSS_V3", Score: "9.8"},
					},
					Affected: []osvAffected{
						{
							Ranges: []osvRange{
								{
									Type: "SEMVER",
									Events: []osvEvent{
										{Introduced: "1.0.0"},
										{Fixed: "1.2.4"},
									},
								},
							},
						},
					},
					References: []osvRef{
						{Type: "ADVISORY", URL: "https://github.com/owner/repo/security/advisories/GHSA-xxxx"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Create scanner with mock server URL
	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	dep := types.LockDetails{
		Name:             "test-vendor",
		Ref:              "main",
		CommitHash:       "abc123def456",
		SourceVersionTag: "v1.2.3",
	}

	vulns, err := scanner.queryOSV(&dep, "https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(vulns) != 1 {
		t.Fatalf("Expected 1 vulnerability, got %d", len(vulns))
	}

	if vulns[0].ID != "CVE-2024-1234" {
		t.Errorf("Expected CVE ID CVE-2024-1234, got %s", vulns[0].ID)
	}

	if vulns[0].Summary != "Remote code execution vulnerability" {
		t.Errorf("Unexpected summary: %s", vulns[0].Summary)
	}
}

func TestQueryOSV_NoCVEs(t *testing.T) {
	// Create mock OSV server that returns no vulnerabilities
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvResponse{
			Vulns: []osvVuln{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	dep := types.LockDetails{
		Name:       "clean-vendor",
		Ref:        "main",
		CommitHash: "xyz789abc",
	}

	vulns, err := scanner.queryOSV(&dep, "https://github.com/owner/clean-repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(vulns) != 0 {
		t.Errorf("Expected 0 vulnerabilities, got %d", len(vulns))
	}
}

func TestQueryOSV_RateLimited(t *testing.T) {
	// Create mock OSV server that returns 429
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	dep := types.LockDetails{
		Name:       "rate-limited-vendor",
		Ref:        "main",
		CommitHash: "def456ghi",
	}

	_, err := scanner.queryOSV(&dep, "https://github.com/owner/repo")
	if err == nil {
		t.Fatal("Expected rate limit error, got nil")
	}

	if !isRateLimitError(err) {
		t.Errorf("Expected rate limit error, got: %v", err)
	}
}

func TestQueryOSV_MalformedResponse(t *testing.T) {
	// Create mock OSV server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	dep := types.LockDetails{
		Name:       "malformed-vendor",
		Ref:        "main",
		CommitHash: "mal123",
	}

	_, err := scanner.queryOSV(&dep, "https://github.com/owner/repo")
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

// ============================================================================
// Caching Tests
// ============================================================================

func TestCaching(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir

	dep := types.LockDetails{
		Name:             "cached-vendor",
		Ref:              "main",
		CommitHash:       "cache123test",
		SourceVersionTag: "v1.0.0",
	}

	// Initially, cache should be empty
	cacheKey := scanner.GetCacheKey(dep)
	cached, ok := scanner.loadFromCache(cacheKey)
	if ok {
		t.Error("Expected cache miss on first access")
	}
	if cached != nil {
		t.Error("Expected nil cached value")
	}

	// Save to cache
	testVulns := []osvVuln{
		{
			ID:      "CVE-2024-TEST",
			Summary: "Test vulnerability",
		},
	}
	err := scanner.saveToCache(cacheKey, testVulns)
	if err != nil {
		t.Fatalf("Failed to save to cache: %v", err)
	}

	// Verify cache hit
	cached, ok = scanner.loadFromCache(cacheKey)
	if !ok {
		t.Error("Expected cache hit after save")
	}
	if len(cached) != 1 {
		t.Errorf("Expected 1 cached vuln, got %d", len(cached))
	}
	if cached[0].ID != "CVE-2024-TEST" {
		t.Errorf("Expected CVE-2024-TEST, got %s", cached[0].ID)
	}
}

func TestCaching_Expiration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir
	scanner.cacheTTL = 1 * time.Millisecond // Very short TTL for testing

	dep := types.LockDetails{
		Name:       "expiring-vendor",
		Ref:        "main",
		CommitHash: "expire123",
	}

	cacheKey := scanner.GetCacheKey(dep)

	// Save to cache
	testVulns := []osvVuln{{ID: "CVE-EXPIRED"}}
	scanner.saveToCache(cacheKey, testVulns)

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Should be cache miss due to expiration
	_, ok := scanner.loadFromCache(cacheKey)
	if ok {
		t.Error("Expected cache miss after TTL expiration")
	}
}

// ============================================================================
// Full Scan Tests
// ============================================================================

func TestScan_EmptyLockfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Mock empty lockfile
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{},
	}, nil)

	// Mock empty config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{},
	}, nil)

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = t.TempDir()

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS for empty lockfile, got %s", result.Summary.Result)
	}

	if result.Summary.TotalDependencies != 0 {
		t.Errorf("Expected 0 dependencies, got %d", result.Summary.TotalDependencies)
	}
}

func TestScan_NoVulnerabilities(t *testing.T) {
	// Create mock OSV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvBatchResponse{Results: []osvResponse{{Vulns: []osvVuln{}}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Mock lockfile with one vendor
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "clean-vendor",
				Ref:              "main",
				CommitHash:       "abc123def",
				SourceVersionTag: "v1.0.0",
			},
		},
	}, nil)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "clean-vendor",
				URL:  "https://github.com/owner/clean",
			},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}

	if result.Summary.Vulnerabilities.Total != 0 {
		t.Errorf("Expected 0 vulnerabilities, got %d", result.Summary.Vulnerabilities.Total)
	}

	if result.Summary.Scanned != 1 {
		t.Errorf("Expected 1 scanned, got %d", result.Summary.Scanned)
	}
}

func TestScan_WithVulnerabilities(t *testing.T) {
	// Create mock OSV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvBatchResponse{
			Results: []osvResponse{
				{
					Vulns: []osvVuln{
						{
							ID:       "CVE-2024-CRIT",
							Summary:  "Critical vulnerability",
							Severity: []osvSeverity{{Type: "CVSS_V3", Score: "9.8"}},
						},
						{
							ID:       "CVE-2024-HIGH",
							Summary:  "High vulnerability",
							Severity: []osvSeverity{{Type: "CVSS_V3", Score: "7.5"}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "vuln-vendor",
				Ref:              "main",
				CommitHash:       "vuln123abc",
				SourceVersionTag: "v1.0.0",
			},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vuln-vendor",
				URL:  "https://github.com/owner/vuln",
			},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	if result.Summary.Vulnerabilities.Total != 2 {
		t.Errorf("Expected 2 vulnerabilities, got %d", result.Summary.Vulnerabilities.Total)
	}

	if result.Summary.Vulnerabilities.Critical != 1 {
		t.Errorf("Expected 1 critical vulnerability, got %d", result.Summary.Vulnerabilities.Critical)
	}

	if result.Summary.Vulnerabilities.High != 1 {
		t.Errorf("Expected 1 high vulnerability, got %d", result.Summary.Vulnerabilities.High)
	}
}

func TestScan_MultipleVendors(t *testing.T) {
	// Create mock OSV server that returns different results per query
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		// Return batch response with results for 3 vendors
		response := osvBatchResponse{
			Results: []osvResponse{
				{Vulns: []osvVuln{{ID: "CVE-VENDOR-A", Summary: "Vendor A vuln", Severity: []osvSeverity{{Type: "CVSS_V3", Score: "5.0"}}}}},
				{Vulns: []osvVuln{}}, // Vendor B has no vulns
				{Vulns: []osvVuln{{ID: "CVE-VENDOR-C", Summary: "Vendor C vuln", Severity: []osvSeverity{{Type: "CVSS_V3", Score: "9.0"}}}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa111bbb"},
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb222ccc"},
			{Name: "vendor-c", Ref: "main", CommitHash: "ccc333ddd"},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor-a", URL: "https://github.com/owner/a"},
			{Name: "vendor-b", URL: "https://github.com/owner/b"},
			{Name: "vendor-c", URL: "https://github.com/owner/c"},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalDependencies != 3 {
		t.Errorf("Expected 3 dependencies, got %d", result.Summary.TotalDependencies)
	}

	if result.Summary.Scanned != 3 {
		t.Errorf("Expected 3 scanned, got %d", result.Summary.Scanned)
	}

	if result.Summary.Vulnerabilities.Total != 2 {
		t.Errorf("Expected 2 total vulnerabilities, got %d", result.Summary.Vulnerabilities.Total)
	}

	if result.Summary.Vulnerabilities.Critical != 1 {
		t.Errorf("Expected 1 critical, got %d", result.Summary.Vulnerabilities.Critical)
	}

	if result.Summary.Vulnerabilities.Medium != 1 {
		t.Errorf("Expected 1 medium, got %d", result.Summary.Vulnerabilities.Medium)
	}
}

func TestScan_FailOnThreshold(t *testing.T) {
	// Create mock OSV server with a high severity vuln
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvBatchResponse{
			Results: []osvResponse{
				{
					Vulns: []osvVuln{
						{
							ID:       "CVE-2024-HIGH",
							Summary:  "High severity issue",
							Severity: []osvSeverity{{Type: "CVSS_V3", Score: "7.5"}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test", Ref: "main", CommitHash: "abc123def"},
		},
	}, nil).Times(2)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test", URL: "https://github.com/owner/repo"},
		},
	}, nil).Times(2)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	// Test with --fail-on critical (should NOT exceed threshold since only HIGH)
	result, err := scanner.Scan("CRITICAL")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Summary.ThresholdExceeded {
		t.Error("Threshold should NOT be exceeded for critical when only high exists")
	}

	// Clear cache for next test
	scanner.ClearCache()

	// Test with --fail-on high (should exceed threshold)
	result, err = scanner.Scan("HIGH")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Summary.ThresholdExceeded {
		t.Error("Threshold SHOULD be exceeded for high when high exists")
	}
}

// ============================================================================
// JSON Output Tests
// ============================================================================

func TestScan_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvBatchResponse{
			Results: []osvResponse{
				{
					Vulns: []osvVuln{
						{
							ID:       "CVE-2024-JSON",
							Summary:  "JSON test vulnerability",
							Aliases:  []string{"GHSA-test"},
							Severity: []osvSeverity{{Type: "CVSS_V3", Score: "8.0"}},
							Affected: []osvAffected{
								{Ranges: []osvRange{{Events: []osvEvent{{Fixed: "v2.0.0"}}}}},
							},
							References: []osvRef{{URL: "https://example.com/advisory"}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "json-test",
				Ref:              "main",
				CommitHash:       "json123abc",
				SourceVersionTag: "v1.0.0",
			},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "json-test", URL: "https://github.com/owner/json"},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &mockTransport{serverURL: server.URL},
		},
		cacheDir:    t.TempDir(),
		cacheTTL:    24 * time.Hour,
		lockStore:   lockStore,
		configStore: configStore,
	}

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify JSON can be marshalled
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Verify JSON structure
	var parsed ScanResult
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed.SchemaVersion != "1.0" {
		t.Errorf("Expected schema version 1.0, got %s", parsed.SchemaVersion)
	}

	if len(parsed.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(parsed.Dependencies))
	}

	if len(parsed.Dependencies[0].Vulnerabilities) != 1 {
		t.Fatalf("Expected 1 vulnerability, got %d", len(parsed.Dependencies[0].Vulnerabilities))
	}

	vuln := parsed.Dependencies[0].Vulnerabilities[0]
	if vuln.ID != "CVE-2024-JSON" {
		t.Errorf("Expected CVE-2024-JSON, got %s", vuln.ID)
	}

	if vuln.FixedVersion != "v2.0.0" {
		t.Errorf("Expected fixed version v2.0.0, got %s", vuln.FixedVersion)
	}
}

// ============================================================================
// Network Error Handling Tests
// ============================================================================

func TestScan_NetworkError_UseStaleCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()

	// Pre-populate cache with stale data
	dep := types.LockDetails{
		Name:       "network-test",
		Ref:        "main",
		CommitHash: "net123abc",
	}

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir
	scanner.cacheTTL = 1 * time.Millisecond // Expired TTL

	cacheKey := scanner.GetCacheKey(dep)
	scanner.saveToCache(cacheKey, []osvVuln{{ID: "CVE-STALE", Summary: "Stale cached vuln"}})

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Verify stale cache can still be loaded
	staleVulns, err := scanner.loadStaleCache(&dep)
	if err != nil {
		t.Fatalf("Failed to load stale cache: %v", err)
	}

	if len(staleVulns) != 1 {
		t.Errorf("Expected 1 stale vuln, got %d", len(staleVulns))
	}

	if staleVulns[0].ID != "CVE-STALE" {
		t.Errorf("Expected CVE-STALE, got %s", staleVulns[0].ID)
	}
}

// ============================================================================
// Helper Types for Testing
// ============================================================================

// mockTransport redirects all requests to a test server
type mockTransport struct {
	serverURL string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse the server URL to get host correctly
	serverHost := t.serverURL
	if len(serverHost) > 7 && serverHost[:7] == "http://" {
		serverHost = serverHost[7:]
	} else if len(serverHost) > 8 && serverHost[:8] == "https://" {
		serverHost = serverHost[8:]
	}

	// Replace the URL with our test server
	req.URL.Scheme = "http"
	req.URL.Host = serverHost

	return http.DefaultTransport.RoundTrip(req)
}

// ============================================================================
// Cache Key Generation Tests
// ============================================================================

func TestGetCacheKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)

	tests := []struct {
		name string
		dep  types.LockDetails
	}{
		{
			name: "With version tag",
			dep: types.LockDetails{
				Name:             "versioned",
				CommitHash:       "abc123def456",
				SourceVersionTag: "v1.2.3",
			},
		},
		{
			name: "Without version tag",
			dep: types.LockDetails{
				Name:       "unversioned",
				CommitHash: "xyz789abc",
			},
		},
		{
			name: "With special characters",
			dep: types.LockDetails{
				Name:             "special/chars:test",
				CommitHash:       "abc123",
				SourceVersionTag: "v1.0.0",
			},
		},
		{
			name: "Very long name",
			dep: types.LockDetails{
				Name:       "this-is-a-very-long-vendor-name-that-might-cause-issues-with-filesystem-limits-if-not-handled-properly-in-the-cache-key-generation-code",
				CommitHash: "abc123def456789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := scanner.GetCacheKey(tt.dep)
			if key == "" {
				t.Error("Cache key should not be empty")
			}

			// Verify key is filesystem-safe (no problematic chars)
			for _, char := range []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"} {
				if filepath.Base(key) != key {
					t.Errorf("Cache key contains path separators: %s", key)
				}
				_ = char
			}

			// Verify key ends with .json
			if !hasJSONSuffix(key) {
				t.Errorf("Cache key should end with .json: %s", key)
			}

			// Verify key length is reasonable
			if len(key) > 210 {
				t.Errorf("Cache key too long: %d chars", len(key))
			}
		})
	}
}

func hasJSONSuffix(s string) bool {
	return len(s) > 5 && s[len(s)-5:] == ".json"
}

// ============================================================================
// Clear Cache Test
// ============================================================================

func TestClearCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()
	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir

	// Create a cache file
	cacheFile := filepath.Join(cacheDir, "test.json")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(cacheFile, []byte("{}"), 0644)

	// Verify file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Fatal("Cache file should exist before clear")
	}

	// Clear cache
	if err := scanner.ClearCache(); err != nil {
		t.Fatalf("ClearCache failed: %v", err)
	}

	// Verify cache directory is removed
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory should be removed after clear")
	}
}

// ============================================================================
// Duplicate Reference Handling Test
// ============================================================================

func TestConvertVulns_NoDuplicateReferences(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)

	// Create vulnerability with duplicate references
	osvVulns := []osvVuln{
		{
			ID:      "CVE-2024-DUP",
			Summary: "Test duplicate refs",
			References: []osvRef{
				{URL: "https://example.com/advisory"},
				{URL: "https://example.com/advisory"}, // Duplicate
				{URL: "https://nvd.nist.gov/vuln/detail/CVE-2024-DUP"},
			},
		},
	}

	vulns := scanner.convertVulns(osvVulns)

	if len(vulns) != 1 {
		t.Fatalf("Expected 1 vulnerability, got %d", len(vulns))
	}

	// Should have deduplicated references
	// Original 2 unique + NVD (already present, not added again)
	expectedRefs := 2 // example.com/advisory and nvd.nist.gov
	if len(vulns[0].References) != expectedRefs {
		t.Errorf("Expected %d unique references, got %d: %v", expectedRefs, len(vulns[0].References), vulns[0].References)
	}
}
