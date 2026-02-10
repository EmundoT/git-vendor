package core

import (
	"context"
	"errors"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Test stubs for audit sub-services
// ============================================================================

// stubAuditVerifyService implements VerifyServiceInterface for audit tests.
type stubAuditVerifyService struct {
	result *types.VerifyResult
	err    error
	called bool
}

func (s *stubAuditVerifyService) Verify(_ context.Context) (*types.VerifyResult, error) {
	s.called = true
	return s.result, s.err
}

// stubAuditVulnScanner implements VulnScannerInterface for audit tests.
type stubAuditVulnScanner struct {
	result *types.ScanResult
	err    error
	called bool
	failOn string // captures the failOn arg passed to Scan
}

func (s *stubAuditVulnScanner) Scan(_ context.Context, failOn string) (*types.ScanResult, error) {
	s.called = true
	s.failOn = failOn
	return s.result, s.err
}

func (s *stubAuditVulnScanner) ClearCache() error { return nil }

// stubAuditDriftService implements DriftServiceInterface for audit tests.
type stubAuditDriftService struct {
	result *types.DriftResult
	err    error
	called bool
}

func (s *stubAuditDriftService) Drift(_ context.Context, _ DriftOptions) (*types.DriftResult, error) {
	s.called = true
	return s.result, s.err
}

// ============================================================================
// Helper constructors
// ============================================================================

func passingVerifyResult() *types.VerifyResult {
	return &types.VerifyResult{
		Summary: types.VerifySummary{
			TotalFiles: 10,
			Verified:   10,
			Result:     "PASS",
		},
	}
}

func failingVerifyResult() *types.VerifyResult {
	return &types.VerifyResult{
		Summary: types.VerifySummary{
			TotalFiles: 10,
			Verified:   8,
			Modified:   2,
			Result:     "FAIL",
		},
	}
}

func warnVerifyResult() *types.VerifyResult {
	return &types.VerifyResult{
		Summary: types.VerifySummary{
			TotalFiles: 10,
			Verified:   10,
			Added:      1,
			Result:     "WARN",
		},
	}
}

func passingScanResult() *types.ScanResult {
	return &types.ScanResult{
		Summary: types.ScanSummary{
			TotalDependencies: 5,
			Scanned:           5,
			Result:            types.ScanResultPass,
		},
	}
}

func failingScanResult() *types.ScanResult {
	return &types.ScanResult{
		Summary: types.ScanSummary{
			TotalDependencies: 5,
			Scanned:           5,
			Vulnerabilities:   types.VulnCounts{Critical: 1, Total: 1},
			Result:            types.ScanResultFail,
		},
	}
}

func warnScanResult() *types.ScanResult {
	return &types.ScanResult{
		Summary: types.ScanSummary{
			TotalDependencies: 5,
			Scanned:           5,
			Vulnerabilities:   types.VulnCounts{Low: 2, Total: 2},
			Result:            types.ScanResultWarn,
		},
	}
}

func cleanDriftResult() *types.DriftResult {
	return &types.DriftResult{
		Summary: types.DriftSummary{
			TotalDependencies: 3,
			Clean:             3,
			Result:            types.DriftResultClean,
		},
	}
}

func driftedResult() *types.DriftResult {
	return &types.DriftResult{
		Summary: types.DriftSummary{
			TotalDependencies: 3,
			DriftedUpstream:   1,
			Clean:             2,
			Result:            types.DriftResultDrifted,
		},
	}
}

// newTestAuditService creates an AuditService with the provided stubs.
// configStore and lockStore can be nil when license check is skipped.
func newTestAuditService(
	verify VerifyServiceInterface,
	scanner VulnScannerInterface,
	drift DriftServiceInterface,
) *AuditService {
	return NewAuditService(verify, scanner, drift, nil, nil)
}

// ============================================================================
// Tests
// ============================================================================

func TestAuditService_AllPass(t *testing.T) {
	verify := &stubAuditVerifyService{result: passingVerifyResult()}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipLicense: true, // skip license to avoid needing config/lock stores
	})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if result.Summary.Result != types.AuditResultPass {
		t.Errorf("Audit() result = %q, want %q", result.Summary.Result, types.AuditResultPass)
	}
	if result.Summary.Checks != 3 {
		t.Errorf("Audit() checks_run = %d, want 3", result.Summary.Checks)
	}
	if result.Summary.Passed != 3 {
		t.Errorf("Audit() checks_passed = %d, want 3", result.Summary.Passed)
	}
	if result.Summary.Failed != 0 {
		t.Errorf("Audit() checks_failed = %d, want 0", result.Summary.Failed)
	}
	if result.Verify == nil || result.Scan == nil || result.Drift == nil {
		t.Error("Audit() expected non-nil sub-results for verify, scan, drift")
	}
	if result.License != nil {
		t.Error("Audit() expected nil license result when skipped")
	}
}

func TestAuditService_OneSubcheckFails(t *testing.T) {
	verify := &stubAuditVerifyService{result: failingVerifyResult()}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipLicense: true,
	})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if result.Summary.Result != types.AuditResultFail {
		t.Errorf("Audit() result = %q, want %q", result.Summary.Result, types.AuditResultFail)
	}
	if result.Summary.Failed != 1 {
		t.Errorf("Audit() checks_failed = %d, want 1", result.Summary.Failed)
	}
	if result.Summary.Passed != 2 {
		t.Errorf("Audit() checks_passed = %d, want 2", result.Summary.Passed)
	}
}

func TestAuditService_SkipFlags(t *testing.T) {
	verify := &stubAuditVerifyService{result: passingVerifyResult()}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipVerify:  true,
		SkipScan:    true,
		SkipLicense: true,
		SkipDrift:   true,
	})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if result.Summary.Checks != 0 {
		t.Errorf("Audit() checks_run = %d, want 0", result.Summary.Checks)
	}
	if result.Summary.Result != types.AuditResultPass {
		t.Errorf("Audit() result = %q, want PASS when all skipped", result.Summary.Result)
	}
	if verify.called {
		t.Error("verify stub was called despite SkipVerify=true")
	}
	if scanner.called {
		t.Error("scanner stub was called despite SkipScan=true")
	}
	if drift.called {
		t.Error("drift stub was called despite SkipDrift=true")
	}
}

func TestAuditService_ContextCancellation(t *testing.T) {
	verify := &stubAuditVerifyService{result: passingVerifyResult()}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)

	// Cancel context before audit starts
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Audit(ctx, AuditOptions{SkipLicense: true})
	if err == nil {
		t.Fatal("Audit() expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Audit() error = %v, want context.Canceled", err)
	}
}

func TestAuditService_ErrorInSubcheckDoesNotAbortOthers(t *testing.T) {
	// Verify errors, but scan and drift should still run
	verify := &stubAuditVerifyService{err: errors.New("verify disk error")}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipLicense: true,
	})

	if err != nil {
		t.Fatalf("Audit() should not return top-level error for sub-check failure: %v", err)
	}

	// Verify was called and errored
	if !verify.called {
		t.Error("verify stub was not called")
	}
	// Scan and drift should still run
	if !scanner.called {
		t.Error("scanner stub was not called despite verify error")
	}
	if !drift.called {
		t.Error("drift stub was not called despite verify error")
	}

	// Result should reflect the error
	if result.Verify != nil {
		t.Error("expected nil verify result when verify errored")
	}
	if len(result.Summary.Errors) != 1 {
		t.Fatalf("expected 1 error in summary, got %d", len(result.Summary.Errors))
	}
	if result.Summary.Errors[0] != "verify: verify disk error" {
		t.Errorf("unexpected error message: %q", result.Summary.Errors[0])
	}
	// Checks count: verify was attempted (counted) but errored;
	// only scan and drift produced results
	if result.Summary.Checks != 3 {
		t.Errorf("checks_run = %d, want 3 (verify attempted + scan + drift)", result.Summary.Checks)
	}
	if result.Summary.Passed != 2 {
		t.Errorf("checks_passed = %d, want 2", result.Summary.Passed)
	}
}

func TestAuditService_CombinedResult_FailGTWarnGTPass(t *testing.T) {
	tests := []struct {
		name           string
		verifyResult   string
		scanResult     string
		driftResult    string
		expectedResult string
	}{
		{
			name:           "All PASS",
			verifyResult:   "PASS",
			scanResult:     types.ScanResultPass,
			driftResult:    types.DriftResultClean,
			expectedResult: types.AuditResultPass,
		},
		{
			name:           "One WARN rest PASS",
			verifyResult:   "PASS",
			scanResult:     types.ScanResultWarn,
			driftResult:    types.DriftResultClean,
			expectedResult: types.AuditResultWarn,
		},
		{
			name:           "One FAIL rest PASS",
			verifyResult:   "FAIL",
			scanResult:     types.ScanResultPass,
			driftResult:    types.DriftResultClean,
			expectedResult: types.AuditResultFail,
		},
		{
			name:           "FAIL trumps WARN",
			verifyResult:   "FAIL",
			scanResult:     types.ScanResultWarn,
			driftResult:    types.DriftResultClean,
			expectedResult: types.AuditResultFail,
		},
		{
			name:           "Drift DRIFTED maps to FAIL",
			verifyResult:   "PASS",
			scanResult:     types.ScanResultPass,
			driftResult:    types.DriftResultDrifted,
			expectedResult: types.AuditResultFail,
		},
		{
			name:           "Drift CONFLICT maps to FAIL",
			verifyResult:   "PASS",
			scanResult:     types.ScanResultPass,
			driftResult:    types.DriftResultConflict,
			expectedResult: types.AuditResultFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verify := &stubAuditVerifyService{result: &types.VerifyResult{
				Summary: types.VerifySummary{Result: tt.verifyResult},
			}}
			scanner := &stubAuditVulnScanner{result: &types.ScanResult{
				Summary: types.ScanSummary{Result: tt.scanResult},
			}}
			drift := &stubAuditDriftService{result: &types.DriftResult{
				Summary: types.DriftSummary{Result: tt.driftResult},
			}}

			svc := newTestAuditService(verify, scanner, drift)
			result, err := svc.Audit(context.Background(), AuditOptions{
				SkipLicense: true,
			})

			if err != nil {
				t.Fatalf("Audit() unexpected error: %v", err)
			}
			if result.Summary.Result != tt.expectedResult {
				t.Errorf("Audit() result = %q, want %q", result.Summary.Result, tt.expectedResult)
			}
		})
	}
}

func TestAuditService_MultipleErrors(t *testing.T) {
	verify := &stubAuditVerifyService{err: errors.New("disk error")}
	scanner := &stubAuditVulnScanner{err: errors.New("network timeout")}
	drift := &stubAuditDriftService{err: errors.New("git error")}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipLicense: true,
	})

	if err != nil {
		t.Fatalf("Audit() should not return top-level error: %v", err)
	}
	if len(result.Summary.Errors) != 3 {
		t.Errorf("expected 3 errors, got %d: %v", len(result.Summary.Errors), result.Summary.Errors)
	}
	// No sub-check produced a result, so all are nil
	if result.Verify != nil || result.Scan != nil || result.Drift != nil {
		t.Error("expected nil sub-results when all errored")
	}
	// With zero passed/failed/warned, result should be PASS (no failures)
	if result.Summary.Result != types.AuditResultPass {
		t.Errorf("result = %q, want PASS when all errored (no explicit failures)", result.Summary.Result)
	}
}

func TestAuditService_ScanFailOnPassthrough(t *testing.T) {
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	svc := newTestAuditService(nil, scanner, nil)

	_, err := svc.Audit(context.Background(), AuditOptions{
		SkipVerify:  true,
		SkipLicense: true,
		SkipDrift:   true,
		ScanFailOn:  "critical",
	})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if scanner.failOn != "critical" {
		t.Errorf("scanner received failOn = %q, want %q", scanner.failOn, "critical")
	}
}

func TestAuditService_SchemaVersionAndTimestamp(t *testing.T) {
	svc := newTestAuditService(nil, nil, nil)
	result, err := svc.Audit(context.Background(), AuditOptions{
		SkipVerify:  true,
		SkipScan:    true,
		SkipLicense: true,
		SkipDrift:   true,
	})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if result.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %q, want %q", result.SchemaVersion, "1.0")
	}
	if result.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

// ============================================================================
// FormatAuditTable tests
// ============================================================================

func TestFormatAuditTable_AllPass(t *testing.T) {
	result := &types.AuditResult{
		Verify:  passingVerifyResult(),
		Scan:    passingScanResult(),
		Drift:   cleanDriftResult(),
		Summary: types.AuditSummary{Result: types.AuditResultPass, Checks: 3, Passed: 3},
	}

	output := FormatAuditTable(result)

	if !contains(output, "=== Audit Report ===") {
		t.Error("output missing header")
	}
	if !contains(output, "Result: PASS") {
		t.Error("output missing Result: PASS")
	}
	if !contains(output, "Verify") {
		t.Error("output missing Verify line")
	}
	if !contains(output, "Scan") {
		t.Error("output missing Scan line")
	}
	if !contains(output, "Drift") {
		t.Error("output missing Drift line")
	}
}

func TestFormatAuditTable_WithErrors(t *testing.T) {
	result := &types.AuditResult{
		Summary: types.AuditSummary{
			Result: types.AuditResultPass,
			Errors: []string{"verify: disk error"},
		},
	}

	output := FormatAuditTable(result)

	if !contains(output, "Errors:") {
		t.Error("output missing Errors section")
	}
	if !contains(output, "verify: disk error") {
		t.Error("output missing error detail")
	}
}

func TestFormatAuditTable_SkippedChecks(t *testing.T) {
	result := &types.AuditResult{
		Verify:  passingVerifyResult(),
		Summary: types.AuditSummary{Result: types.AuditResultPass, Checks: 1, Passed: 1},
	}

	output := FormatAuditTable(result)

	if !contains(output, "SKIP") {
		t.Error("output should show SKIP for absent sub-checks")
	}
}

func TestAuditService_VerifyWarnResult(t *testing.T) {
	verify := &stubAuditVerifyService{result: warnVerifyResult()}
	scanner := &stubAuditVulnScanner{result: passingScanResult()}
	drift := &stubAuditDriftService{result: cleanDriftResult()}

	svc := newTestAuditService(verify, scanner, drift)
	result, err := svc.Audit(context.Background(), AuditOptions{SkipLicense: true})

	if err != nil {
		t.Fatalf("Audit() unexpected error: %v", err)
	}
	if result.Summary.Result != types.AuditResultWarn {
		t.Errorf("result = %q, want WARN when verify warns", result.Summary.Result)
	}
	if result.Summary.Warnings != 1 {
		t.Errorf("warnings = %d, want 1", result.Summary.Warnings)
	}
}
