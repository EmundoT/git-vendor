package core

import (
	"context"

	"github.com/EmundoT/git-vendor/internal/types"
)

// StatusOptions configures the status command behavior.
type StatusOptions struct {
	Offline            bool   // Skip remote checks (only lock-vs-disk)
	RemoteOnly         bool   // Skip disk checks (only lock-vs-upstream)
	StrictOnly         bool   // Only check vendors with enforcement=strict (Spec 075)
	ComplianceOverride string // Override all vendors to this enforcement level (Spec 075)
}

// StatusServiceInterface defines the contract for the unified status command.
// StatusServiceInterface combines verify (offline) and outdated (remote) checks
// into a single per-vendor report.
type StatusServiceInterface interface {
	Status(ctx context.Context, opts StatusOptions) (*types.StatusResult, error)
}

// Compile-time interface satisfaction check.
var _ StatusServiceInterface = (*StatusService)(nil)

// StatusService merges verify and outdated results into a unified per-vendor view.
// StatusService delegates to VerifyServiceInterface and OutdatedServiceInterface
// rather than reimplementing their logic.
type StatusService struct {
	verifySvc   VerifyServiceInterface
	outdatedSvc OutdatedServiceInterface
	configStore ConfigStore
	lockStore   LockStore
}

// NewStatusService creates a StatusService with injected verify and outdated services.
func NewStatusService(
	verifySvc VerifyServiceInterface,
	outdatedSvc OutdatedServiceInterface,
	configStore ConfigStore,
	lockStore LockStore,
) *StatusService {
	return &StatusService{
		verifySvc:   verifySvc,
		outdatedSvc: outdatedSvc,
		configStore: configStore,
		lockStore:   lockStore,
	}
}

// Status runs offline and/or remote checks based on StatusOptions and returns
// a combined StatusResult with per-vendor detail and aggregate summary.
//
// Execution order per spec:
//  1. Offline checks first (lock vs disk — fast, always runs unless RemoteOnly)
//  2. Remote checks second (lock vs upstream — requires network, unless Offline)
//
// Exit code semantics (applied by caller):
//   - 0 = PASS (everything matches)
//   - 1 = FAIL (modified, deleted, or upstream stale)
//   - 2 = WARN (added files only, no failures)
func (s *StatusService) Status(ctx context.Context, opts StatusOptions) (*types.StatusResult, error) {
	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, err
	}

	// Build per-vendor detail entries from lock
	vendorMap := make(map[string]*types.VendorStatusDetail) // keyed by "name@ref"
	var vendorOrder []string                                 // preserve insertion order

	for i := range lock.Vendors {
		entry := &lock.Vendors[i]
		key := entry.Name + "@" + entry.Ref
		vendorMap[key] = &types.VendorStatusDetail{
			Name:        entry.Name,
			Ref:         entry.Ref,
			CommitHash:  entry.CommitHash,
			LastUpdated: entry.Updated,
		}
		vendorOrder = append(vendorOrder, key)
	}

	result := &types.StatusResult{}
	var verifySummary *types.VerifySummary

	// Phase 1: Offline checks (verify)
	if !opts.RemoteOnly {
		verifyResult, verifyErr := s.verifySvc.Verify(ctx)
		if verifyErr != nil {
			return nil, verifyErr
		}

		// Distribute file statuses to per-vendor entries
		for _, f := range verifyResult.Files {
			if f.Vendor == nil {
				continue
			}
			// Find the vendor entry — FileStatus.Vendor is just the name,
			// so we scan all entries with that name.
			for _, key := range vendorOrder {
				v := vendorMap[key]
				if v.Name != *f.Vendor {
					continue
				}
				switch f.Status {
				case "verified":
					v.FilesVerified++
				case "modified":
					v.FilesModified++
					v.ModifiedPaths = append(v.ModifiedPaths, f.Path)
					v.DriftDetails = append(v.DriftDetails, buildDriftDetail(f, false))
				case "added":
					v.FilesAdded++
					v.AddedPaths = append(v.AddedPaths, f.Path)
				case "deleted":
					v.FilesDeleted++
					v.DeletedPaths = append(v.DeletedPaths, f.Path)
				case "accepted":
					v.FilesAccepted++
					v.AcceptedPaths = append(v.AcceptedPaths, f.Path)
					v.DriftDetails = append(v.DriftDetails, buildDriftDetail(f, true))
				case "stale", "orphaned":
					// Coherence issues are counted in the summary (I2),
					// not in per-vendor file counts.
				}
				break // one match per file
			}
		}

		verifySummary = &verifyResult.Summary
	}

	// Phase 2: Remote checks (outdated)
	if !opts.Offline {
		outdatedResult, outdatedErr := s.outdatedSvc.Outdated(ctx, OutdatedOptions{})
		if outdatedErr != nil {
			return nil, outdatedErr
		}

		for _, dep := range outdatedResult.Dependencies {
			// Match by name+ref
			for _, key := range vendorOrder {
				v := vendorMap[key]
				if v.Name == dep.VendorName && v.Ref == dep.Ref {
					stale := !dep.UpToDate
					v.UpstreamHash = dep.LatestHash
					v.UpstreamStale = &stale
					break
				}
			}
		}

		// Mark skipped vendors (ls-remote failures)
		if outdatedResult.Skipped > 0 {
			// Vendors present in lock but absent from dependencies are skipped
			checkedSet := make(map[string]bool)
			for _, dep := range outdatedResult.Dependencies {
				checkedSet[dep.VendorName+"@"+dep.Ref] = true
			}
			for _, key := range vendorOrder {
				v := vendorMap[key]
				if v.UpstreamStale == nil && !checkedSet[key] {
					v.UpstreamSkipped = true
				}
			}
		}
	}

	// Assemble result
	for _, key := range vendorOrder {
		result.Vendors = append(result.Vendors, *vendorMap[key])
	}

	// Phase 3: Policy evaluation (GRD-002) + Enforcement resolution (Spec 075)
	var enforcementMap map[string]string
	var enfSvc *EnforcementService
	if s.configStore != nil {
		config, configErr := s.configStore.Load()
		if configErr == nil {
			// Policy evaluation (GRD-002)
			policySvc := NewPolicyService()
			violations := policySvc.EvaluatePolicy(&config, result)
			result.PolicyViolations = violations

			// Distribute violations to per-vendor entries
			for vi := range violations {
				for ri := range result.Vendors {
					if result.Vendors[ri].Name == violations[vi].VendorName {
						result.Vendors[ri].PolicyViolations = append(
							result.Vendors[ri].PolicyViolations, violations[vi])
						break
					}
				}
			}

			// Enforcement resolution (Spec 075)
			effectiveConfig := &config
			if opts.ComplianceOverride != "" {
				// CLI --compliance=<level> creates a synthetic override config
				effectiveConfig = &types.VendorConfig{
					Compliance: &types.ComplianceConfig{
						Default: opts.ComplianceOverride,
						Mode:    ComplianceModeOverride,
					},
					Vendors: config.Vendors,
				}
			}

			enfSvc = NewEnforcementService()
			enforcementMap = enfSvc.ResolveVendorEnforcement(effectiveConfig)

			// Annotate per-vendor enforcement level and expose config
			for ri := range result.Vendors {
				if level, ok := enforcementMap[result.Vendors[ri].Name]; ok {
					result.Vendors[ri].Enforcement = level
				}
			}
			if effectiveConfig.Compliance != nil {
				result.ComplianceConfig = effectiveConfig.Compliance
			}

			// Filter to strict-only vendors when requested
			if opts.StrictOnly {
				var filtered []types.VendorStatusDetail
				for _, v := range result.Vendors {
					if v.Enforcement == EnforcementStrict {
						filtered = append(filtered, v)
					}
				}
				result.Vendors = filtered
			}
		}
	}

	// Compute summary
	result.Summary = computeStatusSummary(result.Vendors, opts, verifySummary)

	// Override exit code via enforcement when compliance config is present (Spec 075).
	// Enforcement overrides drift-based results but MUST NOT mask non-drift failures
	// (upstream stale, coherence issues). The Stale count is the sentinel: if Stale > 0,
	// a FAIL was caused by upstream staleness and MUST be preserved.
	if enforcementMap != nil {
		exitCode := enfSvc.ComputeExitCode(result.Vendors, enforcementMap)
		switch exitCode {
		case 0:
			// Info: downgrade drift-based FAIL/WARN to PASS (preserve upstream stale FAIL)
			if result.Summary.Result == "FAIL" && result.Summary.Stale == 0 {
				result.Summary.Result = "PASS"
			} else if result.Summary.Result == "WARN" {
				result.Summary.Result = "PASS"
			}
		case 1:
			result.Summary.Result = "FAIL"
		case 2:
			// Lenient: downgrade drift-based FAIL to WARN (preserve upstream stale FAIL)
			if result.Summary.Result == "FAIL" && result.Summary.Stale == 0 {
				result.Summary.Result = "WARN"
			} else if result.Summary.Result != "FAIL" {
				result.Summary.Result = "WARN"
			}
		}
	}

	return result, nil
}

// computeStatusSummary aggregates per-vendor details into a StatusSummary.
// verifySummary is non-nil when offline checks ran; its Stale/Orphaned counts
// are propagated to StatusSummary.StaleConfigs/OrphanedLock for coherence reporting (I2/VFY-001).
func computeStatusSummary(vendors []types.VendorStatusDetail, opts StatusOptions, verifySummary *types.VerifySummary) types.StatusSummary {
	s := types.StatusSummary{
		TotalVendors: len(vendors),
	}

	for _, v := range vendors {
		s.TotalFiles += v.FilesVerified + v.FilesModified + v.FilesAdded + v.FilesDeleted + v.FilesAccepted
		s.Verified += v.FilesVerified
		s.Modified += v.FilesModified
		s.Added += v.FilesAdded
		s.Deleted += v.FilesDeleted
		s.Accepted += v.FilesAccepted
		if v.UpstreamStale != nil && *v.UpstreamStale {
			s.Stale++
		}
		if v.UpstreamSkipped {
			s.UpstreamErrors++
		}
	}

	// Propagate config/lock coherence issues from verify result (I2/VFY-001).
	if verifySummary != nil {
		s.StaleConfigs = verifySummary.Stale
		s.OrphanedLock = verifySummary.Orphaned
	}

	// Determine result code
	hasFail := s.Modified > 0 || s.Deleted > 0
	if !opts.RemoteOnly {
		// Disk checks ran — modified/deleted = FAIL
	}
	if !opts.Offline && s.Stale > 0 {
		hasFail = true
	}

	switch {
	case hasFail:
		s.Result = "FAIL"
	case s.Added > 0 || s.Accepted > 0:
		s.Result = "WARN"
	default:
		s.Result = "PASS"
	}

	return s
}

// buildDriftDetail constructs a DriftDetail from a FileStatus entry.
// buildDriftDetail extracts lock hash (ExpectedHash) and disk hash (ActualHash)
// from the verify result and marks whether the drift has been accepted.
func buildDriftDetail(f types.FileStatus, accepted bool) types.DriftDetail {
	d := types.DriftDetail{
		Path:     f.Path,
		Accepted: accepted,
	}
	if f.ExpectedHash != nil {
		d.LockHash = *f.ExpectedHash
	}
	if f.ActualHash != nil {
		d.DiskHash = *f.ActualHash
	}
	return d
}
