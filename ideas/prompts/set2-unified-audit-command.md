# Prompt: Set 2 — Unified Audit Command (Spec 020)

## Concurrency
CONFLICTS with Sets 3 and 4 (all touch main.go, engine.go, vendor_syncer.go).
SAFE with Sets 1 and 5.

## Branch
Create and work on a branch named `claude/set2-audit-command-<session-suffix>`.

## Context
git-vendor now has four independent compliance/integrity commands:
- `verify` — checks vendored file hashes against lockfile (VerifyResult)
- `scan` — queries OSV.dev for CVEs (ScanResult)
- `license` — checks license policy compliance (LicenseReportResult)
- `drift` — detects drift between vendored and origin (DriftResult)

All four work independently but there's no single command to run them all and produce a combined report. This is Spec 020 from the roadmap — the natural next step on the critical path (prerequisites 010, 011, 012, 013 are all complete). It unlocks Spec 022 (GitHub Action) and Spec 023 (Compliance Reports).

## Task
Implement `git vendor audit` — a unified command that runs verify + scan + license + drift and produces a combined pass/fail report.

### Architecture

Follow the existing service pattern:

1. **`internal/types/audit_types.go`** — New types:

        type AuditResult struct {
            Verify  *VerifyResult         `json:"verify,omitempty"`
            Scan    *ScanResult           `json:"scan,omitempty"`
            License *LicenseReportResult  `json:"license,omitempty"`
            Drift   *DriftResult          `json:"drift,omitempty"`
            Summary AuditSummary          `json:"summary"`
        }

        type AuditSummary struct {
            Result     string   `json:"result"`      // "PASS", "FAIL", "WARN"
            Checks     int      `json:"checks_run"`
            Passed     int      `json:"checks_passed"`
            Failed     int      `json:"checks_failed"`
            Warnings   int      `json:"checks_warned"`
            Errors     []string `json:"errors,omitempty"`  // non-fatal errors (e.g., network failures)
        }

2. **`internal/core/audit_service.go`** — New service:
   - `AuditServiceInterface` with `Audit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error)`
   - `AuditOptions` struct: `SkipVerify`, `SkipScan`, `SkipLicense`, `SkipDrift` bools, plus `ScanFailOn`, `LicenseFailOn`, `LicensePolicyPath` strings
   - Implementation delegates to existing services: `verifyService.Verify()`, `vulnScanner.Scan(ctx, failOn)`, `LicenseReport(svc, failOn)`, `driftService.Drift(opts)`
   - Each sub-check wrapped in error handling — a failed sub-check should NOT abort the others
   - Combined result: FAIL if any sub-check fails, WARN if any warns (but none fail), PASS otherwise

3. **`internal/core/vendor_syncer.go`** — Wire `AuditServiceInterface` into `ServiceOverrides` and `VendorSyncer`
   - Add `Audit(ctx context.Context, opts AuditOptions) (*types.AuditResult, error)` method

4. **`internal/core/engine.go`** — Add `Manager.Audit(ctx, opts)` delegation

5. **`main.go`** — Add `case "audit":` command handler:

        git vendor audit [flags]
          --format table|json     Output format (default: table)
          --skip-verify           Skip file integrity check
          --skip-scan             Skip vulnerability scan
          --skip-license          Skip license compliance check
          --skip-drift            Skip drift detection
          --fail-on <severity>    Severity threshold for scan (critical|high|medium|low)
          --license-fail-on <lvl> License fail level: deny (default) or warn
          --policy <path>         License policy file path
          --verbose, -v           Show git commands

   Use signal-aware context like `scan` does:

        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
        defer stop()

   Exit codes: 0=PASS, 1=FAIL, 2=WARN

### Table output format example

        === Audit Report ===

        Verify ........... PASS (42 files, 0 modified)
        Scan ............. WARN (2 vulnerabilities found, 0 above threshold)
        License .......... PASS (5 vendors, all compliant)
        Drift ............ FAIL (1 vendor has upstream changes)

        Result: FAIL

### Testing

Create `internal/core/audit_service_test.go` with:
- Test all-pass scenario
- Test one sub-check fails
- Test skip flags work
- Test context cancellation
- Test error in one sub-check doesn't abort others
- Test combined result logic (FAIL > WARN > PASS)

Use stubs/mocks for the sub-services — do NOT call real verify/scan/license/drift.

### Key patterns to follow
- `Scan()` already takes `context.Context` — match this pattern
- `LicenseReport()` requires loading policy first — see engine.go:226-235 for the pattern
- `Drift()` takes `DriftOptions` — see engine.go:255
- Error handling: wrap with `fmt.Errorf("audit %s: %w", checkName, err)`
- Interface goes in audit_service.go, added to ServiceOverrides in vendor_syncer.go

### Definition of Done
1. `go build` succeeds
2. `go test ./...` passes (all existing + new tests)
3. `go vet ./...` clean
4. Inline docs on all exported types/functions
5. CLAUDE.md updated: add audit command to Quick Reference, add audit_service.go to Important Functions
6. Commit and push
