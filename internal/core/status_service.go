package core

import (
	"context"

	"github.com/EmundoT/git-vendor/internal/types"
)

// StatusOptions configures the status command behavior.
type StatusOptions struct {
	Offline    bool // Skip remote checks (only lock-vs-disk)
	RemoteOnly bool // Skip disk checks (only lock-vs-upstream)
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
				}
				break // one match per file
			}
		}
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

	// Phase 3: Policy evaluation (GRD-002)
	if s.configStore != nil {
		config, configErr := s.configStore.Load()
		if configErr == nil {
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
		}
	}

	// Compute summary
	result.Summary = computeStatusSummary(result.Vendors, opts)

	return result, nil
}

// computeStatusSummary aggregates per-vendor details into a StatusSummary.
func computeStatusSummary(vendors []types.VendorStatusDetail, opts StatusOptions) types.StatusSummary {
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
	case s.Added > 0:
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
