package core

import (
	"fmt"

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
//   - BlockOnStale=true + stale          -> severity "error"
//   - BlockOnStale=false + stale         -> severity "warning"
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

		// Check staleness: upstream is ahead
		if v.UpstreamStale != nil && *v.UpstreamStale {
			severity := "warning"
			if *resolved.BlockOnStale {
				severity = "error"
			}
			violations = append(violations, types.PolicyViolation{
				VendorName: v.Name,
				Type:       "stale",
				Message:    fmt.Sprintf("%s is behind upstream", v.Name),
				Severity:   severity,
			})
		}
	}

	return violations
}
