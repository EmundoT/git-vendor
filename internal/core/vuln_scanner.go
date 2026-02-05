package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/EmundoT/git-vendor/internal/version"
)

const (
	osvAPIEndpoint      = "https://api.osv.dev/v1/query"
	osvBatchAPIEndpoint = "https://api.osv.dev/v1/querybatch"
	defaultCacheTTL     = 24 * time.Hour
	cacheSubDir         = ".cache/osv" // Relative to vendor directory
	scanSchemaVersion   = "1.0"
	maxBatchSize        = 1000 // OSV.dev batch limit
)

// Package-level compiled regex for CVSS score extraction
var cvssScoreRegex = regexp.MustCompile(`^(\d+\.?\d*)$`)

// SeverityThreshold maps severity names to numeric levels for comparison
var SeverityThreshold = map[string]int{
	"CRITICAL": 4,
	"HIGH":     3,
	"MEDIUM":   2,
	"LOW":      1,
	"UNKNOWN":  0,
}

// ScanResult represents the complete vulnerability scan output
type ScanResult struct {
	SchemaVersion string           `json:"schema_version"`
	Timestamp     string           `json:"timestamp"`
	Summary       ScanSummary      `json:"summary"`
	Dependencies  []DependencyScan `json:"dependencies"`
}

// ScanSummary contains aggregate statistics for the scan
type ScanSummary struct {
	TotalDependencies int        `json:"total_dependencies"`
	Scanned           int        `json:"scanned"`
	NotScanned        int        `json:"not_scanned"`
	Vulnerabilities   VulnCounts `json:"vulnerabilities"`
	Result            string     `json:"result"` // PASS, FAIL, WARN
	FailOnThreshold   string     `json:"fail_on_threshold,omitempty"`
	ThresholdExceeded bool       `json:"threshold_exceeded,omitempty"`
}

// VulnCounts holds vulnerability counts by severity
type VulnCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Total    int `json:"total"`
}

// DependencyScan represents scan results for a single dependency
type DependencyScan struct {
	Name            string          `json:"name"`
	Version         *string         `json:"version"`
	Commit          string          `json:"commit"`
	URL             string          `json:"url,omitempty"`
	ScanStatus      string          `json:"scan_status"` // scanned, not_scanned, error
	ScanReason      string          `json:"scan_reason,omitempty"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

// Vulnerability represents a single CVE/vulnerability finding
type Vulnerability struct {
	ID           string   `json:"id"`
	Aliases      []string `json:"aliases"`
	Severity     string   `json:"severity"`
	CVSSScore    float64  `json:"cvss_score,omitempty"`
	Summary      string   `json:"summary"`
	FixedVersion string   `json:"fixed_version,omitempty"`
	References   []string `json:"references"`
}

// VulnScanner handles vulnerability scanning against OSV.dev
type VulnScanner struct {
	client      *http.Client
	cacheDir    string
	cacheTTL    time.Duration
	lockStore   LockStore
	configStore ConfigStore
}

// NewVulnScanner creates a new vulnerability scanner
func NewVulnScanner(lockStore LockStore, configStore ConfigStore) *VulnScanner {
	// Check for custom cache TTL
	cacheTTL := defaultCacheTTL
	if ttlStr := os.Getenv("GIT_VENDOR_CACHE_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			cacheTTL = d
		}
	}

	// Cache directory is inside vendor directory
	cacheDir := filepath.Join(VendorDir, cacheSubDir)

	return &VulnScanner{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir:    cacheDir,
		cacheTTL:    cacheTTL,
		lockStore:   lockStore,
		configStore: configStore,
	}
}

// osvQuery represents a query to OSV.dev
type osvQuery struct {
	Commit  string      `json:"commit,omitempty"`
	Package *osvPackage `json:"package,omitempty"`
}

// osvPackage represents package info in OSV query
type osvPackage struct {
	Name      string `json:"name,omitempty"`
	Ecosystem string `json:"ecosystem,omitempty"`
	PURL      string `json:"purl,omitempty"`
}

// osvResponse represents OSV API response
type osvResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

// osvVuln represents a vulnerability from OSV
type osvVuln struct {
	ID         string        `json:"id"`
	Summary    string        `json:"summary"`
	Details    string        `json:"details"`
	Aliases    []string      `json:"aliases"`
	Severity   []osvSeverity `json:"severity"`
	Affected   []osvAffected `json:"affected"`
	References []osvRef      `json:"references"`
}

// osvSeverity represents severity info in OSV response
type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

// osvAffected represents affected package info
type osvAffected struct {
	Package *osvPackage `json:"package"`
	Ranges  []osvRange  `json:"ranges"`
}

// osvRange represents version range info
type osvRange struct {
	Type   string     `json:"type"`
	Events []osvEvent `json:"events"`
}

// osvEvent represents version events (introduced, fixed)
type osvEvent struct {
	Introduced string `json:"introduced,omitempty"`
	Fixed      string `json:"fixed,omitempty"`
}

// osvRef represents reference URLs
type osvRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// osvBatchRequest represents a batch query request
type osvBatchRequest struct {
	Queries []osvQuery `json:"queries"`
}

// osvBatchResponse represents batch query response
type osvBatchResponse struct {
	Results []osvResponse `json:"results"`
}

// cacheEntry represents a cached OSV response
type cacheEntry struct {
	Vulns    []osvVuln `json:"vulns"`
	CachedAt time.Time `json:"cached_at"`
}

// Scan performs vulnerability scanning on all vendored dependencies
func (s *VulnScanner) Scan(failOn string) (*ScanResult, error) {
	// Load lockfile
	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Load config for URL info
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build vendor URL map
	vendorURLs := make(map[string]string)
	for _, v := range config.Vendors {
		vendorURLs[v.Name] = v.URL
	}

	result := &ScanResult{
		SchemaVersion: scanSchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Dependencies:  make([]DependencyScan, 0, len(lock.Vendors)),
		Summary: ScanSummary{
			TotalDependencies: len(lock.Vendors),
			FailOnThreshold:   failOn,
		},
	}

	// Use batch query for efficiency
	vulnResults, batchErr := s.batchQuery(lock.Vendors, vendorURLs)

	// Process each vendor
	for _, lockEntry := range lock.Vendors {
		depScan := DependencyScan{
			Name:   lockEntry.Name,
			Commit: lockEntry.CommitHash,
			URL:    vendorURLs[lockEntry.Name],
		}

		// Set version if available
		if lockEntry.SourceVersionTag != "" {
			depScan.Version = &lockEntry.SourceVersionTag
		}

		// Check batch results
		var vulns []osvVuln
		var scanErr error

		if batchErr != nil {
			scanErr = batchErr
		} else if v, ok := vulnResults[lockEntry.Name]; ok {
			vulns = v
		} else {
			// Fallback to individual query if not in batch results
			vulns, scanErr = s.queryOSV(lockEntry, vendorURLs[lockEntry.Name])
		}

		if scanErr != nil {
			// Handle different error types
			if isRateLimitError(scanErr) {
				depScan.ScanStatus = "not_scanned"
				depScan.ScanReason = "Rate limited by OSV.dev API"
				result.Summary.NotScanned++
			} else if isNetworkError(scanErr) {
				// Try to use stale cache
				staleVulns, cacheErr := s.loadStaleCache(lockEntry)
				if cacheErr == nil && staleVulns != nil {
					vulns = staleVulns
					depScan.ScanStatus = "scanned"
					depScan.ScanReason = "Using cached data (network unavailable)"
					result.Summary.Scanned++ // Count stale cache as scanned
				} else {
					depScan.ScanStatus = "not_scanned"
					depScan.ScanReason = fmt.Sprintf("Network error: %v", scanErr)
					result.Summary.NotScanned++
				}
			} else {
				depScan.ScanStatus = "error"
				depScan.ScanReason = scanErr.Error()
				result.Summary.NotScanned++
			}
		}

		if depScan.ScanStatus == "" {
			depScan.ScanStatus = "scanned"
			result.Summary.Scanned++
		}

		// Convert OSV vulns to our format
		if vulns != nil {
			depScan.Vulnerabilities = s.convertVulns(vulns)
		}

		// Count vulnerabilities
		for _, v := range depScan.Vulnerabilities {
			result.Summary.Vulnerabilities.Total++
			switch v.Severity {
			case "CRITICAL":
				result.Summary.Vulnerabilities.Critical++
			case "HIGH":
				result.Summary.Vulnerabilities.High++
			case "MEDIUM":
				result.Summary.Vulnerabilities.Medium++
			case "LOW":
				result.Summary.Vulnerabilities.Low++
			default:
				result.Summary.Vulnerabilities.Unknown++
			}
		}

		result.Dependencies = append(result.Dependencies, depScan)
	}

	// Determine result
	result.Summary.Result = s.determineResult(result, failOn)

	return result, nil
}

// queryOSV queries OSV.dev for vulnerabilities (single query)
func (s *VulnScanner) queryOSV(dep types.LockDetails, repoURL string) ([]osvVuln, error) {
	// Build cache key
	cacheKey := s.getCacheKey(dep)

	// Check cache first
	if cached, ok := s.loadFromCache(cacheKey); ok {
		return cached, nil
	}

	// Build query - try PURL first if we have version tag
	query := s.buildQuery(dep, repoURL)

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvAPIEndpoint, bytes.NewReader(queryJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("git-vendor/%s", version.GetVersion()))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("rate limited, retry after %s", retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OSV API error: %d - %s", resp.StatusCode, string(body))
	}

	var result osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Cache result
	if err := s.saveToCache(cacheKey, result.Vulns); err != nil {
		// Log but don't fail on cache write errors
		// In a real implementation, this could use a logger
		_ = err
	}

	return result.Vulns, nil
}

// batchQuery performs batch vulnerability queries for efficiency
func (s *VulnScanner) batchQuery(deps []types.LockDetails, vendorURLs map[string]string) (map[string][]osvVuln, error) {
	results := make(map[string][]osvVuln)

	// First, check cache for all deps
	var uncachedDeps []types.LockDetails
	for _, dep := range deps {
		cacheKey := s.getCacheKey(dep)
		if vulns, ok := s.loadFromCache(cacheKey); ok {
			results[dep.Name] = vulns
		} else {
			uncachedDeps = append(uncachedDeps, dep)
		}
	}

	// If all cached, return early
	if len(uncachedDeps) == 0 {
		return results, nil
	}

	// Build batch request for uncached deps
	queries := make([]osvQuery, 0, len(uncachedDeps))
	depMap := make(map[int]types.LockDetails) // Map query index to dep

	for i, dep := range uncachedDeps {
		query := s.buildQuery(dep, vendorURLs[dep.Name])
		queries = append(queries, query)
		depMap[i] = dep
	}

	// Split into batches if needed
	for batchStart := 0; batchStart < len(queries); batchStart += maxBatchSize {
		batchEnd := batchStart + maxBatchSize
		if batchEnd > len(queries) {
			batchEnd = len(queries)
		}

		batchQueries := queries[batchStart:batchEnd]

		// Make batch request
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		batchReq := osvBatchRequest{Queries: batchQueries}
		reqJSON, err := json.Marshal(batchReq)
		if err != nil {
			cancel()
			return results, fmt.Errorf("marshal batch request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvBatchAPIEndpoint, bytes.NewReader(reqJSON))
		if err != nil {
			cancel()
			return results, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", fmt.Sprintf("git-vendor/%s", version.GetVersion()))

		resp, err := s.client.Do(req)
		if err != nil {
			cancel()
			return results, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			cancel()
			return results, fmt.Errorf("rate limited")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()
			return results, fmt.Errorf("OSV batch API error: %d - %s", resp.StatusCode, string(body))
		}

		var batchResp osvBatchResponse
		if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
			resp.Body.Close()
			cancel()
			return results, fmt.Errorf("decode batch response: %w", err)
		}
		resp.Body.Close()
		cancel()

		// Process results and cache
		for i, osvResp := range batchResp.Results {
			globalIdx := batchStart + i
			if dep, ok := depMap[globalIdx]; ok {
				results[dep.Name] = osvResp.Vulns
				// Cache individual results
				cacheKey := s.getCacheKey(dep)
				_ = s.saveToCache(cacheKey, osvResp.Vulns)
			}
		}
	}

	return results, nil
}

// buildQuery creates an OSV query for a dependency
func (s *VulnScanner) buildQuery(dep types.LockDetails, repoURL string) osvQuery {
	// Try PURL if we have version info
	if dep.SourceVersionTag != "" && repoURL != "" {
		purl := s.buildPURL(repoURL, dep.SourceVersionTag)
		if purl != "" {
			return osvQuery{
				Package: &osvPackage{
					PURL: purl,
				},
			}
		}
	}

	// Fallback to commit query only
	// Note: We don't add ecosystem/name because it's unreliable to detect from URL
	return osvQuery{
		Commit: dep.CommitHash,
	}
}

// buildPURL creates a Package URL for the dependency
func (s *VulnScanner) buildPURL(repoURL, version string) string {
	// Parse URL to extract owner/repo
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}

	// Validate URL has a scheme and host (reject "not-a-url" type inputs)
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	// Clean the path
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	if path == "" {
		return ""
	}

	// Determine PURL type based on host
	// See: https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst
	host := strings.ToLower(parsed.Host)

	switch {
	case strings.Contains(host, "github.com"):
		// pkg:github/owner/repo@version
		return fmt.Sprintf("pkg:github/%s@%s", path, version)
	case strings.Contains(host, "gitlab.com"):
		// pkg:gitlab/owner/repo@version
		return fmt.Sprintf("pkg:gitlab/%s@%s", path, version)
	case strings.Contains(host, "bitbucket.org"):
		// pkg:bitbucket/owner/repo@version
		return fmt.Sprintf("pkg:bitbucket/%s@%s", path, version)
	default:
		// For other hosts, use generic type
		// Note: This may not match in OSV.dev but is the correct PURL format
		return fmt.Sprintf("pkg:generic/%s@%s", path, version)
	}
}

// getCacheKey generates a safe cache key for a dependency
func (s *VulnScanner) getCacheKey(dep types.LockDetails) string {
	// Create a deterministic key based on commit and version
	key := dep.CommitHash
	if dep.SourceVersionTag != "" {
		// Use at most 8 characters from commit hash
		shortHash := dep.CommitHash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}
		key = dep.SourceVersionTag + "_" + shortHash
	}

	// Comprehensive sanitization for filesystem safety
	// Replace or remove problematic characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\x00", "", // null byte
	)
	key = replacer.Replace(key)
	key = strings.ReplaceAll(dep.Name, "/", "_") + "_" + key

	// Limit length to avoid filesystem issues (255 char limit on most systems)
	if len(key) > 200 {
		key = key[:200]
	}

	return key + ".json"
}

// loadFromCache loads cached OSV response if valid
func (s *VulnScanner) loadFromCache(key string) ([]osvVuln, bool) {
	path := filepath.Join(s.cacheDir, key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CachedAt) > s.cacheTTL {
		return nil, false
	}

	return entry.Vulns, true
}

// loadStaleCache loads cached data even if expired (for offline use)
func (s *VulnScanner) loadStaleCache(dep types.LockDetails) ([]osvVuln, error) {
	key := s.getCacheKey(dep)
	path := filepath.Join(s.cacheDir, key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return entry.Vulns, nil
}

// saveToCache saves OSV response to cache
func (s *VulnScanner) saveToCache(key string, vulns []osvVuln) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	entry := cacheEntry{
		Vulns:    vulns,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	path := filepath.Join(s.cacheDir, key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	return nil
}

// convertVulns converts OSV vulnerabilities to our format
func (s *VulnScanner) convertVulns(osvVulns []osvVuln) []Vulnerability {
	vulns := make([]Vulnerability, 0, len(osvVulns))

	for _, ov := range osvVulns {
		v := Vulnerability{
			ID:      ov.ID,
			Aliases: ov.Aliases,
			Summary: ov.Summary,
		}

		// Extract CVSS score and severity
		for _, sev := range ov.Severity {
			if sev.Type == "CVSS_V3" || sev.Type == "CVSS_V2" {
				score := parseCVSSScore(sev.Score)
				v.CVSSScore = score
				v.Severity = CVSSToSeverity(score)
				break
			}
		}

		// If no CVSS, default to UNKNOWN
		if v.Severity == "" {
			v.Severity = "UNKNOWN"
		}

		// Extract fixed version
		for _, affected := range ov.Affected {
			for _, rng := range affected.Ranges {
				for _, event := range rng.Events {
					if event.Fixed != "" {
						v.FixedVersion = event.Fixed
						break
					}
				}
				if v.FixedVersion != "" {
					break
				}
			}
			if v.FixedVersion != "" {
				break
			}
		}

		// Extract references, tracking seen URLs to avoid duplicates
		seenURLs := make(map[string]bool)
		for _, ref := range ov.References {
			if !seenURLs[ref.URL] {
				v.References = append(v.References, ref.URL)
				seenURLs[ref.URL] = true
			}
		}

		// Add NVD link if it's a CVE and not already present
		cveID := ""
		if strings.HasPrefix(v.ID, "CVE-") {
			cveID = v.ID
		} else {
			// Check aliases for CVE
			for _, alias := range v.Aliases {
				if strings.HasPrefix(alias, "CVE-") {
					cveID = alias
					break
				}
			}
		}

		if cveID != "" {
			nvdURL := fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", cveID)
			if !seenURLs[nvdURL] {
				v.References = append(v.References, nvdURL)
			}
		}

		vulns = append(vulns, v)
	}

	return vulns
}

// parseCVSSScore extracts the numeric score from a CVSS string
// OSV.dev typically returns just the numeric score (e.g., "9.8") not the full vector
func parseCVSSScore(cvssStr string) float64 {
	// Trim whitespace
	cvssStr = strings.TrimSpace(cvssStr)

	// Try to parse as a direct numeric score first (most common case from OSV.dev)
	if cvssScoreRegex.MatchString(cvssStr) {
		var score float64
		if _, err := fmt.Sscanf(cvssStr, "%f", &score); err == nil {
			if score >= 0 && score <= 10 {
				return score
			}
		}
	}

	// If it's a CVSS vector string, we can't reliably calculate the score
	// without a proper CVSS library. Return 0 to indicate unknown.
	// The severity will default to "UNKNOWN" in this case.
	return 0
}

// CVSSToSeverity converts a CVSS score to severity string
func CVSSToSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// determineResult determines the overall scan result
func (s *VulnScanner) determineResult(result *ScanResult, failOn string) string {
	vulns := result.Summary.Vulnerabilities

	// Check if threshold is exceeded
	thresholdExceeded := false
	if failOn != "" {
		thresholdLevel := SeverityThreshold[strings.ToUpper(failOn)]
		if vulns.Critical > 0 && SeverityThreshold["CRITICAL"] >= thresholdLevel {
			thresholdExceeded = true
		}
		if vulns.High > 0 && SeverityThreshold["HIGH"] >= thresholdLevel {
			thresholdExceeded = true
		}
		if vulns.Medium > 0 && SeverityThreshold["MEDIUM"] >= thresholdLevel {
			thresholdExceeded = true
		}
		if vulns.Low > 0 && SeverityThreshold["LOW"] >= thresholdLevel {
			thresholdExceeded = true
		}
	}

	result.Summary.ThresholdExceeded = thresholdExceeded

	// Determine result
	if vulns.Total > 0 {
		return "FAIL"
	}
	if result.Summary.NotScanned > 0 {
		return "WARN"
	}
	return "PASS"
}

// Helper to check if error is rate limiting
func isRateLimitError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limited")
}

// Helper to check if error is network related
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "dial tcp")
}

// ClearCache removes all cached OSV responses
func (s *VulnScanner) ClearCache() error {
	return os.RemoveAll(s.cacheDir)
}

// GetCacheKey returns the cache key for testing purposes
func (s *VulnScanner) GetCacheKey(dep types.LockDetails) string {
	return s.getCacheKey(dep)
}
