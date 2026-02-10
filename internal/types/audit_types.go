// Package types defines data structures for git-vendor configuration and state management.
package types

// AuditResult is the top-level result for the unified audit command.
// AuditResult aggregates results from verify, scan, license, and drift sub-checks
// and produces a combined pass/fail summary.
type AuditResult struct {
	SchemaVersion string              `json:"schema_version"`
	Timestamp     string              `json:"timestamp"`
	Verify        *VerifyResult       `json:"verify,omitempty"`
	Scan          *ScanResult         `json:"scan,omitempty"`
	License       *LicenseReportResult `json:"license,omitempty"`
	Drift         *DriftResult        `json:"drift,omitempty"`
	Summary       AuditSummary        `json:"summary"`
}

// AuditSummary contains aggregate pass/fail/warn counts across all audit sub-checks.
type AuditSummary struct {
	Result   string   `json:"result"`         // "PASS", "FAIL", "WARN"
	Checks   int      `json:"checks_run"`     // Number of sub-checks executed
	Passed   int      `json:"checks_passed"`  // Sub-checks that passed
	Failed   int      `json:"checks_failed"`  // Sub-checks that failed
	Warnings int      `json:"checks_warned"`  // Sub-checks that warned
	Errors   []string `json:"errors,omitempty"` // Non-fatal errors (e.g., network failures)
}

// Audit result constants for AuditSummary.Result.
const (
	AuditResultPass = "PASS"
	AuditResultFail = "FAIL"
	AuditResultWarn = "WARN"
)
