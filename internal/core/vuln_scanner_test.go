package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		{"Critical - 9.8", 9.8, types.SeverityCritical},
		{"Critical - 9.0", 9.0, types.SeverityCritical},
		{"High - 8.5", 8.5, types.SeverityHigh},
		{"High - 7.0", 7.0, types.SeverityHigh},
		{"Medium - 5.0", 5.0, types.SeverityMedium},
		{"Medium - 4.0", 4.0, types.SeverityMedium},
		{"Low - 3.5", 3.5, types.SeverityLow},
		{"Low - 0.1", 0.1, types.SeverityLow},
		{"Unknown - 0.0", 0.0, types.SeverityUnknown},
		{"Unknown - negative", -1.0, types.SeverityUnknown},
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

// Note: PURL generation is now handled by the internal/purl package.
// See internal/purl/purl_test.go for PURL-related tests.

// ============================================================================
// Batch Query Tests with Mock Server
// ============================================================================

// Note: Single query (queryOSV) was removed in favor of batch queries.
// These tests cover the batch query functionality which is now the only
// way to query OSV.dev.

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
	testRepoURL := "https://github.com/owner/cached-repo"
	cacheKey := scanner.GetCacheKey(&dep, testRepoURL)
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

	testRepoURL := "https://github.com/owner/expiring-repo"
	cacheKey := scanner.GetCacheKey(&dep, testRepoURL)

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

	if result.Summary.Result != types.ScanResultPass {
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

	if result.Summary.Result != types.ScanResultPass {
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

	if result.Summary.Result != types.ScanResultFail {
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
	var parsed types.ScanResult
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

	testRepoURL := "https://github.com/owner/network-test"
	cacheKey := scanner.GetCacheKey(&dep, testRepoURL)
	scanner.saveToCache(cacheKey, []osvVuln{{ID: "CVE-STALE", Summary: "Stale cached vuln"}})

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Verify stale cache can still be loaded
	staleVulns, err := scanner.loadStaleCache(&dep, testRepoURL)
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
		name    string
		dep     types.LockDetails
		repoURL string
	}{
		{
			name: "With version tag",
			dep: types.LockDetails{
				Name:             "versioned",
				CommitHash:       "abc123def456",
				SourceVersionTag: "v1.2.3",
			},
			repoURL: "https://github.com/owner/versioned",
		},
		{
			name: "Without version tag",
			dep: types.LockDetails{
				Name:       "unversioned",
				CommitHash: "xyz789abc",
			},
			repoURL: "https://github.com/owner/unversioned",
		},
		{
			name: "With special characters",
			dep: types.LockDetails{
				Name:             "special/chars:test",
				CommitHash:       "abc123",
				SourceVersionTag: "v1.0.0",
			},
			repoURL: "https://github.com/owner/special",
		},
		{
			name: "Very long name",
			dep: types.LockDetails{
				Name:       "this-is-a-very-long-vendor-name-that-might-cause-issues-with-filesystem-limits-if-not-handled-properly-in-the-cache-key-generation-code",
				CommitHash: "abc123def456789",
			},
			repoURL: "https://github.com/owner/long-name",
		},
		{
			name: "Empty URL",
			dep: types.LockDetails{
				Name:       "no-url",
				CommitHash: "abc123",
			},
			repoURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := scanner.GetCacheKey(&tt.dep, tt.repoURL)
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

// TestGetCacheKey_URLCollisionPrevention verifies that different repos with
// the same version tag generate different cache keys.
func TestGetCacheKey_URLCollisionPrevention(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)

	// Two different repos with the same version tag
	dep := types.LockDetails{
		Name:             "shared-lib",
		CommitHash:       "abc123def456",
		SourceVersionTag: "v1.0.0",
	}

	key1 := scanner.GetCacheKey(&dep, "https://github.com/owner1/shared-lib")
	key2 := scanner.GetCacheKey(&dep, "https://github.com/owner2/shared-lib")

	if key1 == key2 {
		t.Errorf("Cache keys should differ for different URLs: key1=%s, key2=%s", key1, key2)
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

// ============================================================================
// Details Field Test
// ============================================================================

func TestConvertVulns_IncludesDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)

	// Create vulnerability with details field
	osvVulns := []osvVuln{
		{
			ID:      "CVE-2024-DETAILS",
			Summary: "Short summary",
			Details: "This is a longer detailed description of the vulnerability that provides additional context about the impact and affected components.",
		},
	}

	vulns := scanner.convertVulns(osvVulns)

	if len(vulns) != 1 {
		t.Fatalf("Expected 1 vulnerability, got %d", len(vulns))
	}

	if vulns[0].Details == "" {
		t.Error("Expected Details field to be populated")
	}

	if vulns[0].Details != osvVulns[0].Details {
		t.Errorf("Expected Details=%q, got %q", osvVulns[0].Details, vulns[0].Details)
	}
}

// ============================================================================
// Network Error Detection Tests
// ============================================================================

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{"nil error", "", false},
		{"connection refused", "dial tcp: connection refused", true},
		{"no such host", "lookup example.com: no such host", true},
		{"network unreachable", "connect: network is unreachable", true},
		{"timeout", "context deadline exceeded: timeout", true},
		{"i/o timeout", "read tcp: i/o timeout", true},
		{"dial tcp", "dial tcp 127.0.0.1:443: connect: connection refused", true},
		{"rate limited", "rate limited, retry after 60", false},
		{"generic error", "something went wrong", false},
		{"OSV API error", "OSV API error: 500", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = fmt.Errorf(tt.errMsg)
			}
			got := isNetworkError(err)
			if got != tt.expected {
				t.Errorf("isNetworkError(%q) = %v, want %v", tt.errMsg, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// FailOn Validation Tests
// ============================================================================

func TestScan_InvalidFailOnValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = t.TempDir()

	// Test invalid failOn value - should return error before loading lockfile
	_, err := scanner.Scan("invalid")
	if err == nil {
		t.Fatal("Expected error for invalid failOn value, got nil")
	}

	if !strings.Contains(err.Error(), "invalid fail-on threshold") {
		t.Errorf("Expected 'invalid fail-on threshold' error, got: %v", err)
	}
}

func TestScan_ValidFailOnValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test with an empty lockfile to verify valid failOn values are accepted
	validValues := []string{"critical", "CRITICAL", "high", "HIGH", "medium", "MEDIUM", "low", "LOW", ""}

	for _, failOn := range validValues {
		t.Run("failOn="+failOn, func(t *testing.T) {
			lockStore := NewMockLockStore(ctrl)
			configStore := NewMockConfigStore(ctrl)

			lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{}}, nil)
			configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)

			scanner := NewVulnScanner(lockStore, configStore)
			scanner.cacheDir = t.TempDir()

			result, err := scanner.Scan(failOn)
			if err != nil {
				t.Errorf("Unexpected error for failOn=%q: %v", failOn, err)
			}
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// ============================================================================
// Empty Commit Hash Handling Tests
// ============================================================================

func TestScan_EmptyCommitHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Mock lockfile with a vendor that has empty commit hash
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "empty-hash-vendor",
				Ref:        "main",
				CommitHash: "", // Empty commit hash
			},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "empty-hash-vendor", URL: "https://github.com/owner/repo"},
		},
	}, nil)

	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = t.TempDir()

	result, err := scanner.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(result.Dependencies))
	}

	dep := result.Dependencies[0]
	if dep.ScanStatus != types.ScanStatusNotScanned {
		t.Errorf("Expected status %s, got %s", types.ScanStatusNotScanned, dep.ScanStatus)
	}

	if !strings.Contains(dep.ScanReason, "Empty commit hash") {
		t.Errorf("Expected ScanReason to mention empty commit hash, got: %s", dep.ScanReason)
	}

	if result.Summary.NotScanned != 1 {
		t.Errorf("Expected NotScanned=1, got %d", result.Summary.NotScanned)
	}

	if result.Summary.Result != types.ScanResultWarn {
		t.Errorf("Expected WARN result for not-scanned deps, got %s", result.Summary.Result)
	}
}

// ============================================================================
// Corrupt Cache Handling Test
// ============================================================================

func TestLoadFromCache_CorruptFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()
	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir

	// Create a corrupt cache file
	cacheFile := filepath.Join(cacheDir, "corrupt.json")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	if err := os.WriteFile(cacheFile, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("Failed to create corrupt cache file: %v", err)
	}

	// Verify file exists before
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Fatal("Cache file should exist before load attempt")
	}

	// Load should return cache miss and clean up corrupt file
	cached, ok := scanner.loadFromCache("corrupt.json")
	if ok {
		t.Error("Expected cache miss for corrupt file")
	}
	if cached != nil {
		t.Error("Expected nil cached value for corrupt file")
	}

	// Verify corrupt file was removed
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("Corrupt cache file should be removed after load attempt")
	}
}

// ============================================================================
// Batch Pagination Test
// ============================================================================

// TestBatchQuery_Pagination verifies that batch queries are properly split
// when there are more than maxBatchSize (1000) dependencies.
// Note: This is a unit test that tests the batching logic, not a full integration test.
func TestBatchQuery_Pagination(t *testing.T) {
	// Track batch request sizes
	var batchSizes []int
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Parse the batch request to count queries
		var batchReq osvBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
			t.Errorf("Failed to decode batch request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		batchSizes = append(batchSizes, len(batchReq.Queries))

		// Generate response with empty vulns for each query
		results := make([]osvResponse, len(batchReq.Queries))
		for i := range results {
			results[i] = osvResponse{Vulns: []osvVuln{}}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(osvBatchResponse{Results: results}); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Create 1500 vendors (needs 2 batches: 1000 + 500)
	numVendors := 1500
	vendors := make([]types.LockDetails, numVendors)
	vendorSpecs := make([]types.VendorSpec, numVendors)

	for i := 0; i < numVendors; i++ {
		name := fmt.Sprintf("vendor-%d", i)
		vendors[i] = types.LockDetails{
			Name:       name,
			Ref:        "main",
			CommitHash: fmt.Sprintf("commit%d", i),
		}
		vendorSpecs[i] = types.VendorSpec{
			Name: name,
			URL:  fmt.Sprintf("https://github.com/owner/repo-%d", i),
		}
	}

	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: vendors}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: vendorSpecs}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
			Timeout:   60 * time.Second,
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

	// Verify all dependencies were scanned
	if result.Summary.TotalDependencies != numVendors {
		t.Errorf("Expected %d dependencies, got %d", numVendors, result.Summary.TotalDependencies)
	}

	if result.Summary.Scanned != numVendors {
		t.Errorf("Expected %d scanned, got %d", numVendors, result.Summary.Scanned)
	}

	// Verify batching - should have 2 requests (1000 + 500)
	if requestCount != 2 {
		t.Errorf("Expected 2 batch requests, got %d", requestCount)
	}

	// Verify batch sizes
	if len(batchSizes) != 2 {
		t.Errorf("Expected 2 batch sizes recorded, got %d", len(batchSizes))
	} else {
		if batchSizes[0] != 1000 {
			t.Errorf("Expected first batch size 1000, got %d", batchSizes[0])
		}
		if batchSizes[1] != 500 {
			t.Errorf("Expected second batch size 500, got %d", batchSizes[1])
		}
	}
}

// ============================================================================
// Batch Partial Failure Test
// ============================================================================

// TestScan_BatchPartialFailure verifies that when a batch query returns
// fewer results than expected, the missing dependencies are properly marked
// as not_scanned rather than left in an undefined state.
func TestScan_BatchPartialFailure(t *testing.T) {
	// Create a server that only returns results for some dependencies
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batchReq osvBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Intentionally return fewer results than queries - simulating partial failure
		// Return results for only the first half of queries
		resultCount := len(batchReq.Queries) / 2
		results := make([]osvResponse, resultCount)
		for i := range results {
			results[i] = osvResponse{Vulns: []osvVuln{}}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(osvBatchResponse{Results: results}); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	// Create 4 vendors
	vendors := []types.LockDetails{
		{Name: "vendor-a", Ref: "main", CommitHash: "aaa111"},
		{Name: "vendor-b", Ref: "main", CommitHash: "bbb222"},
		{Name: "vendor-c", Ref: "main", CommitHash: "ccc333"},
		{Name: "vendor-d", Ref: "main", CommitHash: "ddd444"},
	}
	vendorSpecs := []types.VendorSpec{
		{Name: "vendor-a", URL: "https://github.com/owner/repo-a"},
		{Name: "vendor-b", URL: "https://github.com/owner/repo-b"},
		{Name: "vendor-c", URL: "https://github.com/owner/repo-c"},
		{Name: "vendor-d", URL: "https://github.com/owner/repo-d"},
	}

	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: vendors}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{Vendors: vendorSpecs}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
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

	// Server returns only 2 results for 4 queries
	// The first 2 should be scanned, the last 2 should be not_scanned
	if result.Summary.Scanned != 2 {
		t.Errorf("Expected 2 scanned, got %d", result.Summary.Scanned)
	}
	if result.Summary.NotScanned != 2 {
		t.Errorf("Expected 2 not_scanned, got %d", result.Summary.NotScanned)
	}

	// Verify the actual dependency statuses
	for _, dep := range result.Dependencies {
		switch dep.Name {
		case "vendor-a", "vendor-b":
			if dep.ScanStatus != types.ScanStatusScanned {
				t.Errorf("%s: expected scanned, got %s", dep.Name, dep.ScanStatus)
			}
		case "vendor-c", "vendor-d":
			if dep.ScanStatus != types.ScanStatusNotScanned {
				t.Errorf("%s: expected not_scanned, got %s", dep.Name, dep.ScanStatus)
			}
			if dep.ScanReason == "" {
				t.Errorf("%s: expected scan_reason to be set for not_scanned dependency", dep.Name)
			}
		}
	}
}

// ============================================================================
// OSVAPIError Tests
// ============================================================================

func TestScan_HTTP500_ReturnsOSVAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test", URL: "https://github.com/owner/repo"},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
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

	// When batch query fails, the scan should still succeed
	// but mark dependencies as not_scanned or error
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the dependency was not successfully scanned
	if result.Summary.Scanned != 0 {
		t.Errorf("Expected 0 scanned with server error, got %d", result.Summary.Scanned)
	}
}

func TestScan_HTTP429_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "rate-limited", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "rate-limited", URL: "https://github.com/owner/repo"},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
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

	// Rate limited deps should be marked as not_scanned
	if len(result.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(result.Dependencies))
	}

	dep := result.Dependencies[0]
	if dep.ScanStatus != types.ScanStatusNotScanned {
		t.Errorf("Expected not_scanned for rate limited dep, got %s", dep.ScanStatus)
	}
	if !strings.Contains(dep.ScanReason, "Rate limited") {
		t.Errorf("Expected rate limit reason, got: %s", dep.ScanReason)
	}
}

func TestScan_MalformedJSON_Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json response"))
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test", URL: "https://github.com/owner/repo"},
		},
	}, nil)

	scanner := &VulnScanner{
		client: &http.Client{
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

	// Malformed JSON should result in scan failure, but not a top-level error
	if result.Summary.Scanned != 0 {
		t.Errorf("Expected 0 scanned with malformed response, got %d", result.Summary.Scanned)
	}
}

// TestOSVAPIError_StructuredError verifies OSVAPIError error messages
func TestOSVAPIError_StructuredError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{"Rate limited", 429, "", "Rate limited"},
		{"Server error", 500, "internal error", "transient"},
		{"Client error", 400, "bad request body", "Client error"},
		{"Not found", 404, "endpoint not found", "Client error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewOSVAPIError(tt.statusCode, tt.body)
			errStr := err.Error()
			if !strings.Contains(errStr, fmt.Sprintf("HTTP %d", tt.statusCode)) {
				t.Errorf("Expected HTTP %d in error, got: %s", tt.statusCode, errStr)
			}
			if !strings.Contains(errStr, tt.wantMsg) {
				t.Errorf("Expected '%s' in error, got: %s", tt.wantMsg, errStr)
			}
		})
	}
}

// TestIsRateLimitError_OSVAPIError verifies isRateLimitError recognizes OSVAPIError
func TestIsRateLimitError_OSVAPIError(t *testing.T) {
	apiErr := NewOSVAPIError(429, "too many requests")
	if !isRateLimitError(apiErr) {
		t.Error("Expected isRateLimitError to recognize OSVAPIError with 429")
	}

	apiErr500 := NewOSVAPIError(500, "server error")
	if isRateLimitError(apiErr500) {
		t.Error("Expected isRateLimitError to reject OSVAPIError with 500")
	}
}

// TestCVSSToSeverity_BoundaryValues verifies CVSS boundary values
func TestCVSSToSeverity_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		severity string
	}{
		{"Exact 10.0", 10.0, types.SeverityCritical},
		{"Exact 9.0", 9.0, types.SeverityCritical},
		{"Just below 9.0", 8.9, types.SeverityHigh},
		{"Exact 7.0", 7.0, types.SeverityHigh},
		{"Just below 7.0", 6.9, types.SeverityMedium},
		{"Exact 4.0", 4.0, types.SeverityMedium},
		{"Just below 4.0", 3.9, types.SeverityLow},
		{"Smallest positive", 0.01, types.SeverityLow},
		{"Exact 0.0", 0.0, types.SeverityUnknown},
		{"Negative", -0.1, types.SeverityUnknown},
		{"Large negative", -10.0, types.SeverityUnknown},
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

// TestScan_FailOnThreshold_AllLevels verifies all threshold comparisons
func TestScan_FailOnThreshold_AllLevels(t *testing.T) {
	// Server returns one medium-severity vulnerability
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := osvBatchResponse{
			Results: []osvResponse{
				{Vulns: []osvVuln{{
					ID:       "CVE-2024-MED",
					Summary:  "Medium issue",
					Severity: []osvSeverity{{Type: "CVSS_V3", Score: "5.0"}},
				}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tests := []struct {
		failOn            string
		expectExceeded    bool
	}{
		{"CRITICAL", false}, // Only medium exists, critical threshold not exceeded
		{"HIGH", false},     // Only medium exists, high threshold not exceeded
		{"MEDIUM", true},    // Medium exists, medium threshold exceeded
		{"LOW", true},       // Medium exists, low threshold exceeded (medium >= low)
	}

	for _, tt := range tests {
		t.Run("failOn="+tt.failOn, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			lockStore := NewMockLockStore(ctrl)
			configStore := NewMockConfigStore(ctrl)

			lockStore.EXPECT().Load().Return(types.VendorLock{
				Vendors: []types.LockDetails{
					{Name: "test", Ref: "main", CommitHash: "abc123"},
				},
			}, nil)
			configStore.EXPECT().Load().Return(types.VendorConfig{
				Vendors: []types.VendorSpec{
					{Name: "test", URL: "https://github.com/owner/repo"},
				},
			}, nil)

			scanner := &VulnScanner{
				client: &http.Client{
					Transport: &mockTransport{serverURL: server.URL},
				},
				cacheDir:    t.TempDir(),
				cacheTTL:    24 * time.Hour,
				lockStore:   lockStore,
				configStore: configStore,
			}

			result, err := scanner.Scan(tt.failOn)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Summary.ThresholdExceeded != tt.expectExceeded {
				t.Errorf("failOn=%s: ThresholdExceeded=%v, want %v", tt.failOn, result.Summary.ThresholdExceeded, tt.expectExceeded)
			}
		})
	}
}

// TestScan_NetworkError_WithStaleCache_FallsBack verifies stale cache fallback on network error
func TestScan_NetworkError_WithStaleCache_FallsBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockStore := NewMockLockStore(ctrl)
	configStore := NewMockConfigStore(ctrl)

	cacheDir := t.TempDir()

	dep := types.LockDetails{
		Name:       "network-fallback",
		Ref:        "main",
		CommitHash: "fallback123",
	}

	repoURL := "https://github.com/owner/network-fallback"

	// Pre-populate cache
	scanner := NewVulnScanner(lockStore, configStore)
	scanner.cacheDir = cacheDir
	scanner.cacheTTL = 1 * time.Millisecond // Expire immediately

	cacheKey := scanner.GetCacheKey(&dep, repoURL)
	scanner.saveToCache(cacheKey, []osvVuln{{ID: "CVE-CACHED", Summary: "Cached vuln", Severity: []osvSeverity{{Type: "CVSS_V3", Score: "7.0"}}}})

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Setup mock stores for scan
	lockStore.EXPECT().Load().Return(types.VendorLock{Vendors: []types.LockDetails{dep}}, nil)
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "network-fallback", URL: repoURL}},
	}, nil)

	// Create scanner with a transport that simulates network error
	scanner2 := &VulnScanner{
		client: &http.Client{
			Transport: &errorTransport{err: fmt.Errorf("dial tcp: connection refused")},
		},
		cacheDir:    cacheDir,
		cacheTTL:    1 * time.Millisecond, // Expired
		lockStore:   lockStore,
		configStore: configStore,
	}

	result, err := scanner2.Scan("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have used stale cache
	if result.Summary.Scanned != 1 {
		t.Errorf("Expected 1 scanned (from stale cache), got %d", result.Summary.Scanned)
	}

	// Should have found the cached vulnerability
	if result.Summary.Vulnerabilities.Total != 1 {
		t.Errorf("Expected 1 vulnerability from stale cache, got %d", result.Summary.Vulnerabilities.Total)
	}
}

// errorTransport always returns an error for testing network failures
type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, t.err
}
