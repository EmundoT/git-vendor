// Package types defines data structures for git-vendor configuration and state management.
package types

// SeverityLevel constants for vulnerability severity
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityUnknown  = "UNKNOWN"
)

// ScanStatus constants for dependency scan status
const (
	ScanStatusScanned    = "scanned"
	ScanStatusNotScanned = "not_scanned"
	ScanStatusError      = "error"
)

// ScanResultCode constants for overall scan result
const (
	ScanResultPass = "PASS"
	ScanResultFail = "FAIL"
	ScanResultWarn = "WARN"
)

// ScanResult represents the complete vulnerability scan output.
// This is the top-level structure returned by the scan command
// and used for both JSON and table output formats.
type ScanResult struct {
	SchemaVersion string           `json:"schema_version"`
	Timestamp     string           `json:"timestamp"`
	Summary       ScanSummary      `json:"summary"`
	Dependencies  []DependencyScan `json:"dependencies"`
}

// ScanSummary contains aggregate statistics for the scan.
// It provides a quick overview of the scan results including
// counts by severity level and the overall result determination.
type ScanSummary struct {
	TotalDependencies int        `json:"total_dependencies"`
	Scanned           int        `json:"scanned"`
	NotScanned        int        `json:"not_scanned"`
	Vulnerabilities   VulnCounts `json:"vulnerabilities"`
	Result            string     `json:"result"` // PASS, FAIL, WARN
	FailOnThreshold   string     `json:"fail_on_threshold,omitempty"`
	ThresholdExceeded bool       `json:"threshold_exceeded,omitempty"`
}

// VulnCounts holds vulnerability counts by severity level.
// Used in ScanSummary for aggregate reporting.
type VulnCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Total    int `json:"total"`
}

// DependencyScan represents scan results for a single vendored dependency.
// Each dependency in the lockfile gets a corresponding DependencyScan entry.
type DependencyScan struct {
	Name            string          `json:"name"`
	Version         *string         `json:"version"`
	Commit          string          `json:"commit"`
	URL             string          `json:"url,omitempty"`
	ScanStatus      string          `json:"scan_status"` // scanned, not_scanned, error
	ScanReason      string          `json:"scan_reason,omitempty"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

// Vulnerability represents a single CVE/vulnerability finding.
// This structure is designed to capture essential vulnerability information
// from the OSV.dev API response in a normalized format.
type Vulnerability struct {
	ID           string   `json:"id"`
	Aliases      []string `json:"aliases"`
	Severity     string   `json:"severity"`
	CVSSScore    float64  `json:"cvss_score,omitempty"`
	Summary      string   `json:"summary"`
	Details      string   `json:"details,omitempty"` // Extended description from OSV.dev
	FixedVersion string   `json:"fixed_version,omitempty"`
	References   []string `json:"references"`
}

// ValidSeverityThresholds defines valid values for the --fail-on flag.
// Used for validation in both CLI and Scan() method.
var ValidSeverityThresholds = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   true,
	"low":      true,
}

// SeverityThreshold maps severity names to numeric levels for comparison.
// Higher numbers indicate more severe vulnerabilities.
var SeverityThreshold = map[string]int{
	SeverityCritical: 4,
	SeverityHigh:     3,
	SeverityMedium:   2,
	SeverityLow:      1,
	SeverityUnknown:  0,
}
