package core

import (
	"context"
	"fmt"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// AuditOptions configures which sub-checks the audit command runs and their thresholds.
type AuditOptions struct {
	SkipVerify  bool   // Skip file integrity check
	SkipScan    bool   // Skip vulnerability scan
	SkipLicense bool   // Skip license compliance check
	SkipDrift   bool   // Skip drift detection

	ScanFailOn        string // Severity threshold for scan (critical|high|medium|low)
	LicenseFailOn     string // License fail level: "deny" (default) or "warn"
	LicensePolicyPath string // Override license policy file path (empty = default)
}

// AuditServiceInterface defines the contract for the unified audit command.
type AuditServiceInterface interface {
	// Audit runs all enabled sub-checks (verify, scan, license, drift) and
	// returns a combined AuditResult. A failed sub-check does NOT abort the others.
	// ctx controls cancellation for network-dependent operations (scan, drift).
	Audit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error)
}

// Compile-time interface satisfaction check for AuditService.
var _ AuditServiceInterface = (*AuditService)(nil)

// AuditService orchestrates verify, scan, license, and drift sub-checks
// into a unified audit report.
type AuditService struct {
	verifyService VerifyServiceInterface
	vulnScanner   VulnScannerInterface
	driftService  DriftServiceInterface
	configStore   ConfigStore
	lockStore     LockStore
}

// NewAuditService creates a new AuditService with injected sub-service dependencies.
func NewAuditService(
	verifyService VerifyServiceInterface,
	vulnScanner VulnScannerInterface,
	driftService DriftServiceInterface,
	configStore ConfigStore,
	lockStore LockStore,
) *AuditService {
	return &AuditService{
		verifyService: verifyService,
		vulnScanner:   vulnScanner,
		driftService:  driftService,
		configStore:   configStore,
		lockStore:     lockStore,
	}
}

// Audit runs all enabled sub-checks and produces a combined AuditResult.
// Each sub-check is independently error-handled — a failure in one does not
// prevent the others from running. Context cancellation aborts all remaining checks.
func (s *AuditService) Audit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error) {
	result := &types.AuditResult{
		SchemaVersion: "1.0",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	var checks, passed, failed, warnings int
	var errors []string

	// Verify sub-check
	if !opts.SkipVerify {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("audit cancelled: %w", err)
		}
		checks++
		verifyResult, err := s.verifyService.Verify()
		if err != nil {
			errors = append(errors, fmt.Sprintf("verify: %s", err.Error()))
		} else {
			result.Verify = verifyResult
			switch verifyResult.Summary.Result {
			case "PASS":
				passed++
			case "WARN":
				warnings++
			default: // FAIL
				failed++
			}
		}
	}

	// Scan sub-check
	if !opts.SkipScan {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("audit cancelled: %w", err)
		}
		checks++
		scanResult, err := s.vulnScanner.Scan(ctx, opts.ScanFailOn)
		if err != nil {
			errors = append(errors, fmt.Sprintf("scan: %s", err.Error()))
		} else {
			result.Scan = scanResult
			switch scanResult.Summary.Result {
			case types.ScanResultPass:
				passed++
			case types.ScanResultWarn:
				warnings++
			default: // FAIL
				failed++
			}
		}
	}

	// License sub-check
	if !opts.SkipLicense {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("audit cancelled: %w", err)
		}
		checks++
		licenseResult, err := s.runLicenseCheck(opts)
		if err != nil {
			errors = append(errors, fmt.Sprintf("license: %s", err.Error()))
		} else {
			result.License = licenseResult
			switch licenseResult.Summary.Result {
			case "PASS":
				passed++
			case "WARN":
				warnings++
			default: // FAIL
				failed++
			}
		}
	}

	// Drift sub-check
	if !opts.SkipDrift {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("audit cancelled: %w", err)
		}
		checks++
		driftResult, err := s.driftService.Drift(DriftOptions{})
		if err != nil {
			errors = append(errors, fmt.Sprintf("drift: %s", err.Error()))
		} else {
			result.Drift = driftResult
			switch driftResult.Summary.Result {
			case types.DriftResultClean:
				passed++
			default: // DRIFTED, CONFLICT
				failed++
			}
		}
	}

	// Compute combined result: FAIL > WARN > PASS
	overallResult := types.AuditResultPass
	if warnings > 0 {
		overallResult = types.AuditResultWarn
	}
	if failed > 0 {
		overallResult = types.AuditResultFail
	}

	result.Summary = types.AuditSummary{
		Result:   overallResult,
		Checks:   checks,
		Passed:   passed,
		Failed:   failed,
		Warnings: warnings,
		Errors:   errors,
	}

	return result, nil
}

// runLicenseCheck loads the license policy and runs the license compliance report.
func (s *AuditService) runLicenseCheck(opts AuditOptions) (*types.LicenseReportResult, error) {
	policyPath := opts.LicensePolicyPath
	if policyPath == "" {
		policyPath = PolicyFile
	}

	failOn := opts.LicenseFailOn
	if failOn == "" {
		failOn = "deny"
	}

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("load license policy: %w", err)
	}

	svc := NewLicensePolicyService(&policy, policyPath, s.configStore, s.lockStore)
	return svc.GenerateReport(failOn)
}

// FormatAuditTable formats an AuditResult as a human-readable table string.
func FormatAuditTable(result *types.AuditResult) string {
	var out string
	out += "=== Audit Report ===\n\n"

	if result.Verify != nil {
		out += formatCheckLine("Verify", result.Verify.Summary.Result,
			verifyDetail(result.Verify))
	} else if !isSkipped(result, "verify") {
		out += formatCheckLine("Verify", "ERROR", "could not complete")
	} else {
		out += formatCheckLine("Verify", "SKIP", "skipped")
	}

	if result.Scan != nil {
		out += formatCheckLine("Scan", result.Scan.Summary.Result,
			scanDetail(result.Scan))
	} else if !isSkipped(result, "scan") {
		out += formatCheckLine("Scan", "ERROR", "could not complete")
	} else {
		out += formatCheckLine("Scan", "SKIP", "skipped")
	}

	if result.License != nil {
		out += formatCheckLine("License", result.License.Summary.Result,
			licenseDetail(result.License))
	} else if !isSkipped(result, "license") {
		out += formatCheckLine("License", "ERROR", "could not complete")
	} else {
		out += formatCheckLine("License", "SKIP", "skipped")
	}

	if result.Drift != nil {
		out += formatCheckLine("Drift", driftResultToPassFail(result.Drift.Summary.Result),
			driftDetail(result.Drift))
	} else if !isSkipped(result, "drift") {
		out += formatCheckLine("Drift", "ERROR", "could not complete")
	} else {
		out += formatCheckLine("Drift", "SKIP", "skipped")
	}

	out += fmt.Sprintf("\nResult: %s\n", result.Summary.Result)

	if len(result.Summary.Errors) > 0 {
		out += "\nErrors:\n"
		for _, e := range result.Summary.Errors {
			out += fmt.Sprintf("  - %s\n", e)
		}
	}

	return out
}

// formatCheckLine produces a dotted-line format: "  Name ........... STATUS (detail)"
func formatCheckLine(name, status, detail string) string {
	dots := 20 - len(name)
	if dots < 3 {
		dots = 3
	}
	dotStr := ""
	for i := 0; i < dots; i++ {
		dotStr += "."
	}
	return fmt.Sprintf("  %s %s %s (%s)\n", name, dotStr, status, detail)
}

func verifyDetail(r *types.VerifyResult) string {
	return fmt.Sprintf("%s, %d modified",
		Pluralize(r.Summary.TotalFiles, "file", "files"),
		r.Summary.Modified)
}

func scanDetail(r *types.ScanResult) string {
	total := r.Summary.Vulnerabilities.Total
	if total == 0 {
		return fmt.Sprintf("%s, no vulnerabilities",
			Pluralize(r.Summary.TotalDependencies, "dependency", "dependencies"))
	}
	return fmt.Sprintf("%s found",
		Pluralize(total, "vulnerability", "vulnerabilities"))
}

func licenseDetail(r *types.LicenseReportResult) string {
	if r.Summary.Denied > 0 {
		return fmt.Sprintf("%s, %d denied",
			Pluralize(r.Summary.TotalVendors, "vendor", "vendors"),
			r.Summary.Denied)
	}
	if r.Summary.Warned > 0 {
		return fmt.Sprintf("%s, %d warned",
			Pluralize(r.Summary.TotalVendors, "vendor", "vendors"),
			r.Summary.Warned)
	}
	return fmt.Sprintf("%s, all compliant",
		Pluralize(r.Summary.TotalVendors, "vendor", "vendors"))
}

func driftDetail(r *types.DriftResult) string {
	if r.Summary.ConflictRisk > 0 {
		return fmt.Sprintf("%s with conflict risk",
			Pluralize(r.Summary.ConflictRisk, "vendor", "vendors"))
	}
	if r.Summary.DriftedUpstream > 0 || r.Summary.DriftedLocal > 0 {
		drifted := r.Summary.DriftedLocal + r.Summary.DriftedUpstream
		return fmt.Sprintf("%s with drift",
			Pluralize(drifted, "vendor", "vendors"))
	}
	return fmt.Sprintf("%s, no drift",
		Pluralize(r.Summary.TotalDependencies, "vendor", "vendors"))
}

// driftResultToPassFail maps drift-specific result strings to PASS/FAIL for audit display.
func driftResultToPassFail(driftResult string) string {
	if driftResult == types.DriftResultClean {
		return "PASS"
	}
	return "FAIL"
}

// isSkipped checks if a sub-check result is nil because it was skipped vs errored.
// A check is "skipped" when its result is nil and there's no corresponding error in Summary.Errors.
func isSkipped(result *types.AuditResult, checkName string) bool {
	prefix := checkName + ": "
	for _, e := range result.Summary.Errors {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			return false
		}
	}
	return true
}
