package core

import (
	"bytes"
	"context"
	"crypto/sha256"
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
)

const (
	osvAPIEndpoint      = "https://api.osv.dev/v1/query"
	osvBatchAPIEndpoint = "https://api.osv.dev/v1/querybatch"
	defaultCacheTTL     = 24 * time.Hour
	defaultCacheDir     = ".git-vendor-cache/osv"
	scanSchemaVersion   = "1.0"
)

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
	TotalDependencies int             `json:"total_dependencies"`
	Scanned           int             `json:"scanned"`
	NotScanned        int             `json:"not_scanned"`
	Vulnerabilities   VulnCounts      `json:"vulnerabilities"`
	Result            string          `json:"result"` // PASS, FAIL, WARN
	FailOnThreshold   string          `json:"fail_on_threshold,omitempty"`
	ThresholdExceeded bool            `json:"threshold_exceeded,omitempty"`
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
	client     *http.Client
	cacheDir   string
	cacheTTL   time.Duration
	lockStore  LockStore
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

	return &VulnScanner{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir:    defaultCacheDir,
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
	ID        string        `json:"id"`
	Summary   string        `json:"summary"`
	Details   string        `json:"details"`
	Aliases   []string      `json:"aliases"`
	Severity  []osvSeverity `json:"severity"`
	Affected  []osvAffected `json:"affected"`
	References []osvRef     `json:"references"`
}

// osvSeverity represents severity info in OSV response
type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

// osvAffected represents affected package info
type osvAffected struct {
	Package *osvPackage  `json:"package"`
	Ranges  []osvRange   `json:"ranges"`
}

// osvRange represents version range info
type osvRange struct {
	Type   string      `json:"type"`
	Events []osvEvent  `json:"events"`
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
	Vulns     []osvVuln `json:"vulns"`
	CachedAt  time.Time `json:"cached_at"`
	QueryHash string    `json:"query_hash"`
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

		// Query OSV
		vulns, err := s.queryOSV(lockEntry, vendorURLs[lockEntry.Name])
		if err != nil {
			// Handle different error types
			if isRateLimitError(err) {
				depScan.ScanStatus = "not_scanned"
				depScan.ScanReason = "Rate limited by OSV.dev API"
				result.Summary.NotScanned++
			} else if isNetworkError(err) {
				// Try to use stale cache
				staleVulns, cacheErr := s.loadStaleCache(lockEntry)
				if cacheErr == nil && staleVulns != nil {
					vulns = staleVulns
					depScan.ScanStatus = "scanned"
					depScan.ScanReason = "Using cached data (network unavailable)"
				} else {
					depScan.ScanStatus = "not_scanned"
					depScan.ScanReason = fmt.Sprintf("Network error: %v", err)
					result.Summary.NotScanned++
				}
			} else {
				depScan.ScanStatus = "error"
				depScan.ScanReason = err.Error()
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

// queryOSV queries OSV.dev for vulnerabilities
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
	req.Header.Set("User-Agent", "git-vendor/1.0")

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
	s.saveToCache(cacheKey, result.Vulns)

	return result.Vulns, nil
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

	// Fallback to commit query
	query := osvQuery{
		Commit: dep.CommitHash,
	}

	// Add package info if we can determine it
	if repoURL != "" {
		ecosystem := detectEcosystem(repoURL)
		pkgName := extractPackageName(repoURL)
		if ecosystem != "" && pkgName != "" {
			query.Package = &osvPackage{
				Name:      pkgName,
				Ecosystem: ecosystem,
			}
		}
	}

	return query
}

// buildPURL creates a Package URL for the dependency
func (s *VulnScanner) buildPURL(repoURL, version string) string {
	// Parse URL to extract owner/repo
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}

	// Clean the path
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	// Determine PURL type based on host
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
		// Generic git URL
		return fmt.Sprintf("pkg:generic/%s@%s", path, version)
	}
}

// detectEcosystem attempts to determine the ecosystem from URL
func detectEcosystem(repoURL string) string {
	// For now, default to empty which lets OSV.dev try all ecosystems
	// In the future, we could analyze vendored file extensions
	lower := strings.ToLower(repoURL)

	// Some heuristics based on common patterns
	switch {
	case strings.Contains(lower, "golang"), strings.Contains(lower, "/go/"):
		return "Go"
	case strings.Contains(lower, "npm"), strings.Contains(lower, "node"):
		return "npm"
	case strings.Contains(lower, "pypi"), strings.Contains(lower, "python"):
		return "PyPI"
	case strings.Contains(lower, "crates.io"), strings.Contains(lower, "rust"):
		return "crates.io"
	default:
		return ""
	}
}

// extractPackageName extracts the package name from a repo URL
func extractPackageName(repoURL string) string {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}

	// Clean and return path
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	if path == "" {
		return ""
	}

	// For GitHub-style URLs, include the host in the name
	host := parsed.Host
	return host + "/" + path
}

// getCacheKey generates a cache key for a dependency
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

	// Sanitize for filesystem
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, ":", "_")

	return fmt.Sprintf("%s_%s.json", dep.Name, key)
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
func (s *VulnScanner) saveToCache(key string, vulns []osvVuln) {
	// Ensure cache directory exists
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return
	}

	entry := cacheEntry{
		Vulns:    vulns,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path := filepath.Join(s.cacheDir, key)
	_ = os.WriteFile(path, data, 0o644) //nolint:errcheck
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

		// Extract references
		for _, ref := range ov.References {
			v.References = append(v.References, ref.URL)
		}

		// Add NVD link if it's a CVE
		if strings.HasPrefix(v.ID, "CVE-") {
			nvdURL := fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", v.ID)
			v.References = append(v.References, nvdURL)
		} else if len(v.Aliases) > 0 {
			// Check aliases for CVE
			for _, alias := range v.Aliases {
				if strings.HasPrefix(alias, "CVE-") {
					nvdURL := fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", alias)
					v.References = append(v.References, nvdURL)
					break
				}
			}
		}

		vulns = append(vulns, v)
	}

	return vulns
}

// parseCVSSScore extracts the numeric score from a CVSS vector string
func parseCVSSScore(cvssVector string) float64 {
	// CVSS vector format: CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H
	// We need to calculate the score from the vector, but for simplicity
	// we'll check if a numeric score is embedded or use a regex pattern

	// Try to find a direct score (some APIs include it)
	scorePattern := regexp.MustCompile(`(\d+\.?\d*)`)
	matches := scorePattern.FindStringSubmatch(cvssVector)
	if len(matches) > 1 {
		var score float64
		fmt.Sscanf(matches[1], "%f", &score)
		if score >= 0 && score <= 10 {
			return score
		}
	}

	// If no numeric score, attempt basic vector parsing
	// This is a simplified calculation - in production you'd use a proper CVSS library
	upper := strings.ToUpper(cvssVector)

	// Very basic heuristic based on attack vector and impact
	var score float64 = 5.0 // Default medium

	if strings.Contains(upper, "AV:N") { // Network
		score += 1.5
	}
	if strings.Contains(upper, "AC:L") { // Low complexity
		score += 1.0
	}
	if strings.Contains(upper, "C:H") || strings.Contains(upper, "I:H") || strings.Contains(upper, "A:H") {
		score += 2.0
	}
	if strings.Contains(upper, "PR:N") { // No privileges required
		score += 0.5
	}

	// Cap at 10
	if score > 10 {
		score = 10.0
	}

	return score
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
	if thresholdExceeded || vulns.Total > 0 {
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
		strings.Contains(errStr, "i/o timeout")
}

// BatchQuery performs batch vulnerability queries (for efficiency)
func (s *VulnScanner) BatchQuery(deps []types.LockDetails, vendorURLs map[string]string) (map[string][]osvVuln, error) {
	// Build batch request
	queries := make([]osvQuery, 0, len(deps))
	depMap := make(map[int]types.LockDetails) // Map query index to dep

	for _, dep := range deps {
		// Check cache first
		cacheKey := s.getCacheKey(dep)
		if _, ok := s.loadFromCache(cacheKey); ok {
			continue // Skip cached entries
		}

		query := s.buildQuery(dep, vendorURLs[dep.Name])
		queries = append(queries, query)
		depMap[len(queries)-1] = dep
	}

	// If all cached, return early
	if len(queries) == 0 {
		result := make(map[string][]osvVuln)
		for _, dep := range deps {
			cacheKey := s.getCacheKey(dep)
			if vulns, ok := s.loadFromCache(cacheKey); ok {
				result[dep.Name] = vulns
			}
		}
		return result, nil
	}

	// Make batch request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	batchReq := osvBatchRequest{Queries: queries}
	reqJSON, err := json.Marshal(batchReq)
	if err != nil {
		return nil, fmt.Errorf("marshal batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvBatchAPIEndpoint, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "git-vendor/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OSV batch API error: %d - %s", resp.StatusCode, string(body))
	}

	var batchResp osvBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("decode batch response: %w", err)
	}

	// Process results and cache
	results := make(map[string][]osvVuln)
	for i, resp := range batchResp.Results {
		if dep, ok := depMap[i]; ok {
			results[dep.Name] = resp.Vulns
			// Cache individual results
			cacheKey := s.getCacheKey(dep)
			s.saveToCache(cacheKey, resp.Vulns)
		}
	}

	// Add cached entries to results
	for _, dep := range deps {
		if _, ok := results[dep.Name]; !ok {
			cacheKey := s.getCacheKey(dep)
			if vulns, ok := s.loadFromCache(cacheKey); ok {
				results[dep.Name] = vulns
			}
		}
	}

	return results, nil
}

// ClearCache removes all cached OSV responses
func (s *VulnScanner) ClearCache() error {
	return os.RemoveAll(s.cacheDir)
}

// GetCacheKey returns the cache key for testing purposes
func (s *VulnScanner) GetCacheKey(dep types.LockDetails) string {
	return s.getCacheKey(dep)
}

// ComputeQueryHash computes a hash for query deduplication
func ComputeQueryHash(query osvQuery) string {
	data, _ := json.Marshal(query)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}
