package core

import (
	"fmt"
	"math"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// PolicyService evaluates vendor policy rules against status results.
// PolicyService resolves per-vendor policy overrides against global defaults
// and produces PolicyViolation entries for drift and staleness conditions.
// #guard #policy #drift-detection
type PolicyService struct{}

// NewPolicyService creates a PolicyService.
func NewPolicyService() *PolicyService {
	return &PolicyService{}
}

// EvaluatePolicy checks all vendors in a StatusResult against the policy
// defined in VendorConfig. EvaluatePolicy returns a slice of PolicyViolation
// for each vendor that violates its resolved policy (global merged with
// per-vendor overrides).
//
// Violation types:
//   - "drift": vendor has unacknowledged modified files and BlockOnDrift is true
//   - "stale": vendor is behind upstream and BlockOnStale is true
//
// Severity mapping:
//   - BlockOnDrift=true + drift present  -> severity "error"
//   - BlockOnDrift=false + drift present -> severity "warning"
//   - BlockOnStale=false + stale         -> severity "warning"
//   - BlockOnStale=true + stale + MaxStalenessDays=0 -> severity "error" (no grace)
//   - BlockOnStale=true + stale + age > MaxStalenessDays -> severity "error"
//   - BlockOnStale=true + stale + age <= MaxStalenessDays -> severity "warning" (within grace)
//   - BlockOnStale=true + stale + unknown age -> severity "error" (conservative)
//
// #guard #policy #drift-detection #staleness
func (p *PolicyService) EvaluatePolicy(config *types.VendorConfig, statusResult *types.StatusResult) []types.PolicyViolation {
	if statusResult == nil {
		return nil
	}

	// Build lookup from vendor name to VendorSpec for per-vendor policy
	specMap := make(map[string]*types.VendorSpec)
	if config != nil {
		for i := range config.Vendors {
			specMap[config.Vendors[i].Name] = &config.Vendors[i]
		}
	}

	var violations []types.PolicyViolation

	for i := range statusResult.Vendors {
		v := &statusResult.Vendors[i]

		// Resolve policy: global defaults + per-vendor overrides
		var perVendor *types.VendorPolicy
		if spec, ok := specMap[v.Name]; ok {
			perVendor = spec.Policy
		}
		var globalPolicy *types.VendorPolicy
		if config != nil {
			globalPolicy = config.Policy
		}
		resolved := types.ResolvedPolicy(globalPolicy, perVendor)

		// Check drift: unacknowledged modifications
		unackedDrift := v.FilesModified + v.FilesDeleted
		if unackedDrift > 0 {
			severity := "warning"
			if *resolved.BlockOnDrift {
				severity = "error"
			}
			violations = append(violations, types.PolicyViolation{
				VendorName: v.Name,
				Type:       "drift",
				Message:    fmt.Sprintf("%s has %d unacknowledged drifted file(s)", v.Name, unackedDrift),
				Severity:   severity,
			})
		}

		// Check staleness: upstream is ahead (GRD-003).
		// When MaxStalenessDays > 0, staleness within the grace window produces a
		// "warning" even if BlockOnStale is true. Beyond the window (or when the
		// lock age is unknown) the severity follows BlockOnStale.
		if v.UpstreamStale != nil && *v.UpstreamStale {
			severity := staleSeverity(resolved, v.LastUpdated)
			msg := fmt.Sprintf("%s is behind upstream", v.Name)
			if days := lockAgeDays(v.LastUpdated); days >= 0 && *resolved.MaxStalenessDays > 0 {
				msg = fmt.Sprintf("%s is behind upstream (lock age: %d days, threshold: %d days)",
					v.Name, days, *resolved.MaxStalenessDays)
			}
			violations = append(violations, types.PolicyViolation{
				VendorName: v.Name,
				Type:       "stale",
				Message:    msg,
				Severity:   severity,
			})
		}
	}

	return violations
}

// staleSeverity determines the violation severity for a stale vendor based on
// the resolved policy and the lock entry's age. staleSeverity implements the
// MaxStalenessDays grace window: when a positive threshold is set and the lock
// age is within the window, severity is "warning" even if BlockOnStale is true.
// #guard #policy #staleness
func staleSeverity(resolved types.VendorPolicy, lastUpdated string) string {
	if !*resolved.BlockOnStale {
		return "warning"
	}

	maxDays := *resolved.MaxStalenessDays
	if maxDays <= 0 {
		// No grace period configured — block immediately.
		return "error"
	}

	days := lockAgeDays(lastUpdated)
	if days < 0 {
		// Unknown age (unparseable or empty LastUpdated) — conservative: block.
		return "error"
	}

	if days <= maxDays {
		return "warning"
	}
	return "error"
}

// lockAgeDays parses an RFC3339 timestamp and returns the number of whole days
// since that timestamp. lockAgeDays returns -1 if the timestamp is empty or
// unparseable (caller should treat unknown age conservatively).
// #guard #staleness
func lockAgeDays(lastUpdated string) int {
	if lastUpdated == "" {
		return -1
	}
	t, err := time.Parse(time.RFC3339, lastUpdated)
	if err != nil {
		return -1
	}
	return int(math.Floor(time.Since(t).Hours() / 24))
}
