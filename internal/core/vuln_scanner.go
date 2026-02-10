package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/purl"
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

	// Timeout rationale:
	// - Single query: 30s is generous for a single HTTP round-trip to OSV.dev,
	//   accounting for occasional API slowness while avoiding indefinite hangs.
	// - Batch query: 60s allows for larger payloads (up to 1000 queries),
	//   which take longer for OSV.dev to process.
	// These values align with typical CLI tool expectations for network operations.
	singleQueryTimeout = 30 * time.Second
	batchQueryTimeout  = 60 * time.Second

	// maxResponseBodySize limits response body reads to prevent OOM from malicious
	// or misconfigured servers. 10 MB is generous for OSV.dev batch responses
	// (typical response for 1000 queries is ~500 KB).
	maxResponseBodySize = 10 * 1024 * 1024 // 10 MB
)

// Package-level compiled regex for CVSS score extraction
var cvssScoreRegex = regexp.MustCompile(`^(\d+\.?\d*)$`)

// VulnScannerInterface defines the contract for vulnerability scanning.
// VulnScannerInterface enables mocking in tests and alternative implementations.
type VulnScannerInterface interface {
	// Scan performs vulnerability scanning on all vendored dependencies.
	// ctx controls cancellation — a cancelled context aborts in-flight HTTP requests.
	// failOn specifies the severity threshold (critical|high|medium|low) for failing.
	// Returns nil error even if vulnerabilities are found; check result.Summary.Result.
	Scan(ctx context.Context, failOn string) (*types.ScanResult, error)

	// ClearCache removes all cached OSV responses.
	ClearCache() error
}

// Compile-time interface satisfaction check.
var _ VulnScannerInterface = (*VulnScanner)(nil)

// VulnScanner handles vulnerability scanning against OSV.dev.
// The batch API endpoint is configurable via the GIT_VENDOR_OSV_ENDPOINT
// environment variable (defaults to https://api.osv.dev). This enables
// testing against local servers and air-gapped proxy deployments.
type VulnScanner struct {
	client        *http.Client
	batchEndpoint string // Full URL for batch queries (e.g., "https://api.osv.dev/v1/querybatch")
	cacheDir      string
	cacheTTL      time.Duration
	lockStore     LockStore
	configStore   ConfigStore
}

// NewVulnScanner creates a new vulnerability scanner.
// Reads GIT_VENDOR_OSV_ENDPOINT env var to override the default OSV.dev base URL.
// Reads GIT_VENDOR_CACHE_TTL env var to override the default 24-hour cache TTL.
func NewVulnScanner(lockStore LockStore, configStore ConfigStore) *VulnScanner {
	// Check for custom cache TTL
	cacheTTL := defaultCacheTTL
	if ttlStr := os.Getenv("GIT_VENDOR_CACHE_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			cacheTTL = d
		}
	}

	// Determine batch endpoint: env override or default
	batchEndpoint := osvBatchAPIEndpoint
	if baseURL := os.Getenv("GIT_VENDOR_OSV_ENDPOINT"); baseURL != "" {
		// Strip trailing slash for consistent concatenation
		batchEndpoint = strings.TrimRight(baseURL, "/") + "/v1/querybatch"
	}

	// Cache directory is inside vendor directory
	cacheDir := filepath.Join(VendorDir, cacheSubDir)

	return &VulnScanner{
		client: &http.Client{
			// No global timeout - let context.WithTimeout control each request.
			// This avoids the client timeout (30s) aborting batch requests (60s).
		},
		batchEndpoint: batchEndpoint,
		cacheDir:      cacheDir,
		cacheTTL:      cacheTTL,
		lockStore:     lockStore,
		configStore:   configStore,
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

// Scan performs vulnerability scanning on all vendored dependencies.
// ctx controls cancellation — a cancelled context aborts in-flight HTTP requests.
// failOn specifies the severity threshold for failing (critical|high|medium|low).
// An empty string means no threshold check.
func (s *VulnScanner) Scan(ctx context.Context, failOn string) (*types.ScanResult, error) {
	// Validate failOn parameter
	if failOn != "" {
		normalized := strings.ToLower(failOn)
		if !types.ValidSeverityThresholds[normalized] {
			return nil, fmt.Errorf("invalid fail-on threshold %q: must be one of critical, high, medium, low", failOn)
		}
		failOn = strings.ToUpper(normalized)
	}

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

	result := &types.ScanResult{
		SchemaVersion: scanSchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Dependencies:  make([]types.DependencyScan, 0, len(lock.Vendors)),
		Summary: types.ScanSummary{
			TotalDependencies: len(lock.Vendors),
			FailOnThreshold:   failOn,
		},
	}

	// Use batch query for efficiency
	vulnResults, batchErr := s.batchQuery(ctx, lock.Vendors, vendorURLs)

	// Short-circuit on context cancellation — the user requested abort (e.g. Ctrl+C).
	// Unlike network/API errors (which are handled per-vendor below), cancellation
	// means "stop everything" and returning partial results would be misleading.
	if batchErr != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Track which deps were successfully included in batch
	batchedDeps := make(map[string]bool)
	for name := range vulnResults {
		batchedDeps[name] = true
	}

	// Process each vendor
	for i := range lock.Vendors {
		lockEntry := &lock.Vendors[i]
		depScan := types.DependencyScan{
			Name:   lockEntry.Name,
			Commit: lockEntry.CommitHash,
			URL:    vendorURLs[lockEntry.Name],
		}

		// Set version if available
		if lockEntry.SourceVersionTag != "" {
			version := lockEntry.SourceVersionTag
			depScan.Version = &version
		}

		// Handle empty commit hash - skip with warning
		if lockEntry.CommitHash == "" {
			depScan.ScanStatus = types.ScanStatusNotScanned
			depScan.ScanReason = "Empty commit hash - cannot query vulnerability database"
			result.Summary.NotScanned++
			result.Dependencies = append(result.Dependencies, depScan)
			continue
		}

		// Check batch results
		var vulns []osvVuln
		var scanErr error

		if batchErr != nil {
			// Batch failed entirely
			scanErr = batchErr
		} else if v, ok := vulnResults[lockEntry.Name]; ok {
			// Found in batch results
			vulns = v
		} else if !batchedDeps[lockEntry.Name] {
			// Not in batch results - mark as not scanned due to partial failure
			// This handles the case where batch succeeded but this dep wasn't included
			depScan.ScanStatus = types.ScanStatusNotScanned
			depScan.ScanReason = "Not included in batch query results"
			result.Summary.NotScanned++
			result.Dependencies = append(result.Dependencies, depScan)
			continue
		}

		if scanErr != nil {
			// Handle different error types
			switch {
			case isRateLimitError(scanErr):
				depScan.ScanStatus = types.ScanStatusNotScanned
				depScan.ScanReason = "Rate limited by OSV.dev API"
				result.Summary.NotScanned++
			case isNetworkError(scanErr):
				// Try to use stale cache
				staleVulns, cacheErr := s.loadStaleCache(lockEntry, vendorURLs[lockEntry.Name])
				if cacheErr == nil && staleVulns != nil {
					vulns = staleVulns
					depScan.ScanStatus = types.ScanStatusScanned
					depScan.ScanReason = "Using cached data (network unavailable)"
					result.Summary.Scanned++ // Count stale cache as scanned
				} else {
					depScan.ScanStatus = types.ScanStatusNotScanned
					depScan.ScanReason = fmt.Sprintf("Network error: %v", scanErr)
					result.Summary.NotScanned++
				}
			default:
				depScan.ScanStatus = types.ScanStatusError
				depScan.ScanReason = scanErr.Error()
				result.Summary.NotScanned++
			}
		}

		if depScan.ScanStatus == "" {
			depScan.ScanStatus = types.ScanStatusScanned
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
			case types.SeverityCritical:
				result.Summary.Vulnerabilities.Critical++
			case types.SeverityHigh:
				result.Summary.Vulnerabilities.High++
			case types.SeverityMedium:
				result.Summary.Vulnerabilities.Medium++
			case types.SeverityLow:
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

// batchQuery performs batch vulnerability queries for efficiency.
// ctx is used as the parent for per-request timeouts — cancelling ctx aborts all requests.
// batchQuery handles batching (max 1000 queries per request) and caching automatically.
func (s *VulnScanner) batchQuery(ctx context.Context, deps []types.LockDetails, vendorURLs map[string]string) (map[string][]osvVuln, error) {
	results := make(map[string][]osvVuln)

	// First, check cache for all deps (also filter out empty commit hashes)
	var uncachedIdxs []int
	for i := range deps {
		// Skip deps with empty commit hash
		if deps[i].CommitHash == "" {
			continue
		}
		cacheKey := s.getCacheKey(&deps[i], vendorURLs[deps[i].Name])
		if vulns, ok := s.loadFromCache(cacheKey); ok {
			results[deps[i].Name] = vulns
		} else {
			uncachedIdxs = append(uncachedIdxs, i)
		}
	}

	// If all cached, return early
	if len(uncachedIdxs) == 0 {
		return results, nil
	}

	// Build batch request for uncached deps
	queries := make([]osvQuery, 0, len(uncachedIdxs))
	depIdxMap := make(map[int]int) // Map query index to original deps index

	for queryIdx, depIdx := range uncachedIdxs {
		query := s.buildQuery(&deps[depIdx], vendorURLs[deps[depIdx].Name])
		queries = append(queries, query)
		depIdxMap[queryIdx] = depIdx
	}

	// Split into batches if needed (OSV.dev has a 1000 query limit per batch)
	for batchStart := 0; batchStart < len(queries); batchStart += maxBatchSize {
		batchEnd := batchStart + maxBatchSize
		if batchEnd > len(queries) {
			batchEnd = len(queries)
		}

		batchQueries := queries[batchStart:batchEnd]

		// Make batch request with timeout derived from parent context.
		// See timeout rationale at package level.
		ctx, cancel := context.WithTimeout(ctx, batchQueryTimeout)

		batchReq := osvBatchRequest{Queries: batchQueries}
		reqJSON, err := json.Marshal(batchReq)
		if err != nil {
			cancel()
			return results, fmt.Errorf("marshal batch request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.batchEndpoint, bytes.NewReader(reqJSON))
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
			resp.Body.Close() //nolint:errcheck // Non-actionable
			cancel()
			return results, NewOSVAPIError(resp.StatusCode, "rate limited by OSV.dev")
		}

		if resp.StatusCode != http.StatusOK {
			// Read body with size limit to prevent OOM from oversized error responses
			limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
			body, readErr := io.ReadAll(limitedReader)
			resp.Body.Close() //nolint:errcheck // Non-actionable
			cancel()
			bodyStr := ""
			if readErr == nil {
				bodyStr = string(body)
			}
			return results, NewOSVAPIError(resp.StatusCode, bodyStr)
		}

		// Decode response with size-limited reader to prevent OOM
		limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
		var batchResp osvBatchResponse
		if err := json.NewDecoder(limitedBody).Decode(&batchResp); err != nil {
			resp.Body.Close() //nolint:errcheck // Non-actionable
			cancel()
			return results, fmt.Errorf("decode batch response: %w", err)
		}
		resp.Body.Close() //nolint:errcheck // Non-actionable
		cancel()

		// Process results and cache
		for i := range batchResp.Results {
			globalIdx := batchStart + i
			if depIdx, ok := depIdxMap[globalIdx]; ok {
				results[deps[depIdx].Name] = batchResp.Results[i].Vulns
				// Cache individual results (ignore cache write errors)
				cacheKey := s.getCacheKey(&deps[depIdx], vendorURLs[deps[depIdx].Name])
				//nolint:errcheck // Cache write errors are non-fatal
				s.saveToCache(cacheKey, batchResp.Results[i].Vulns)
			}
		}
	}

	return results, nil
}

// buildQuery creates an OSV query for a dependency.
// Uses the shared purl package for PURL generation consistency with SBOM generation.
func (s *VulnScanner) buildQuery(dep *types.LockDetails, repoURL string) osvQuery {
	// Try PURL if we have version info
	if dep.SourceVersionTag != "" && repoURL != "" {
		p := purl.FromGitURL(repoURL, dep.SourceVersionTag)
		if p != nil && p.SupportsVulnScanning() {
			return osvQuery{
				Package: &osvPackage{
					PURL: p.String(),
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

// getCacheKey generates a safe cache key for a dependency.
// It includes a hash of the repository URL to prevent collisions between
// different repos that might have the same version tag.
func (s *VulnScanner) getCacheKey(dep *types.LockDetails, repoURL string) string {
	// Create a deterministic key based on commit, version, and URL
	key := dep.CommitHash
	if dep.SourceVersionTag != "" {
		// Use at most 8 characters from commit hash
		shortHash := dep.CommitHash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}
		key = dep.SourceVersionTag + "_" + shortHash
	}

	// Add URL hash to prevent cache collisions between repos with same version
	// e.g., v1.0.0 from repo-a vs v1.0.0 from repo-b
	if repoURL != "" {
		urlHash := sha256.Sum256([]byte(repoURL))
		key = key + "_" + hex.EncodeToString(urlHash[:4]) // 8 hex chars
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
	// When truncating, add a hash suffix to prevent collisions between keys
	// that share the same first 180 characters but differ afterward.
	const maxLen = 200
	if len(key) > maxLen {
		// Use a hash of the full key to preserve uniqueness after truncation
		fullKeyHash := sha256.Sum256([]byte(key))
		hashSuffix := hex.EncodeToString(fullKeyHash[:8]) // 16 hex chars
		key = key[:maxLen-17] + "_" + hashSuffix          // 183 + 1 + 16 = 200
	}

	return key + ".json"
}

// loadFromCache loads cached OSV response if valid.
// Returns (vulns, true) on cache hit, (nil, false) on miss or error.
// Corrupt cache files are logged and deleted automatically.
func (s *VulnScanner) loadFromCache(key string) ([]osvVuln, bool) {
	path := filepath.Join(s.cacheDir, key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Corrupt cache file - log warning and remove it
		log.Printf("Warning: corrupt cache file %s, removing: %v", key, err)
		//nolint:errcheck // Best effort cleanup
		os.Remove(path)
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CachedAt) > s.cacheTTL {
		return nil, false
	}

	return entry.Vulns, true
}

// loadStaleCache loads cached data even if expired (for offline use).
// This is used as a fallback when network is unavailable.
func (s *VulnScanner) loadStaleCache(dep *types.LockDetails, repoURL string) ([]osvVuln, error) {
	key := s.getCacheKey(dep, repoURL)
	path := filepath.Join(s.cacheDir, key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Corrupt cache file - log warning and remove it
		log.Printf("Warning: corrupt cache file %s, removing: %v", key, err)
		//nolint:errcheck // Best effort cleanup
		os.Remove(path)
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
func (s *VulnScanner) convertVulns(osvVulns []osvVuln) []types.Vulnerability {
	vulns := make([]types.Vulnerability, 0, len(osvVulns))

	for i := range osvVulns {
		ov := &osvVulns[i]
		v := types.Vulnerability{
			ID:      ov.ID,
			Aliases: ov.Aliases,
			Summary: ov.Summary,
			Details: ov.Details, // Include extended description from OSV
		}

		// Extract CVSS score and severity
		for j := range ov.Severity {
			sev := &ov.Severity[j]
			if sev.Type == "CVSS_V3" || sev.Type == "CVSS_V2" {
				score := parseCVSSScore(sev.Score)
				v.CVSSScore = score
				v.Severity = CVSSToSeverity(score)
				break
			}
		}

		// If no CVSS, default to UNKNOWN
		if v.Severity == "" {
			v.Severity = types.SeverityUnknown
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

// CVSSToSeverity converts a CVSS score to severity string.
// Thresholds based on CVSS v3.0 qualitative severity rating scale:
// - Critical: 9.0-10.0
// - High: 7.0-8.9
// - Medium: 4.0-6.9
// - Low: 0.1-3.9
// - Unknown: 0.0 (no score available)
func CVSSToSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return types.SeverityCritical
	case score >= 7.0:
		return types.SeverityHigh
	case score >= 4.0:
		return types.SeverityMedium
	case score > 0:
		return types.SeverityLow
	default:
		return types.SeverityUnknown
	}
}

// determineResult determines the overall scan result based on vulnerabilities found.
func (s *VulnScanner) determineResult(result *types.ScanResult, failOn string) string {
	vulns := result.Summary.Vulnerabilities

	// Check if threshold is exceeded
	thresholdExceeded := false
	if failOn != "" {
		thresholdLevel := types.SeverityThreshold[strings.ToUpper(failOn)]
		switch {
		case vulns.Critical > 0 && types.SeverityThreshold[types.SeverityCritical] >= thresholdLevel:
			thresholdExceeded = true
		case vulns.High > 0 && types.SeverityThreshold[types.SeverityHigh] >= thresholdLevel:
			thresholdExceeded = true
		case vulns.Medium > 0 && types.SeverityThreshold[types.SeverityMedium] >= thresholdLevel:
			thresholdExceeded = true
		case vulns.Low > 0 && types.SeverityThreshold[types.SeverityLow] >= thresholdLevel:
			thresholdExceeded = true
		}
	}

	result.Summary.ThresholdExceeded = thresholdExceeded

	// Determine result
	switch {
	case vulns.Total > 0:
		return types.ScanResultFail
	case result.Summary.NotScanned > 0:
		return types.ScanResultWarn
	default:
		return types.ScanResultPass
	}
}

// Helper to check if error is rate limiting.
// Recognizes both structured OSVAPIError (HTTP 429) and legacy string-based errors.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *OSVAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return strings.Contains(err.Error(), "rate limited")
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

// GetCacheKey returns the cache key for testing purposes.
// repoURL is used to prevent cache collisions between repos with same version.
func (s *VulnScanner) GetCacheKey(dep *types.LockDetails, repoURL string) string {
	return s.getCacheKey(dep, repoURL)
}
