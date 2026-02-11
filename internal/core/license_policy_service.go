package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

const licensePolicySchemaVersion = "1.0"

// LicensePolicyServiceInterface defines the contract for license policy evaluation.
// LicensePolicyServiceInterface enables mocking in tests and alternative policy backends.
type LicensePolicyServiceInterface interface {
	// Evaluate determines the policy decision for a given SPDX license identifier.
	// Evaluate returns one of types.PolicyAllow, types.PolicyDeny, or types.PolicyWarn.
	Evaluate(license string) string

	// GenerateReport produces a full license compliance report for all vendored dependencies.
	// failOn specifies which decision level triggers a FAIL result ("deny" or "warn").
	GenerateReport(failOn string) (*types.LicenseReportResult, error)

	// PolicyFile returns the path to the policy file that was loaded (or "default").
	PolicyFile() string
}

// Compile-time interface satisfaction check.
var _ LicensePolicyServiceInterface = (*LicensePolicyService)(nil)

// LicensePolicyService evaluates licenses against a loaded LicensePolicy.
type LicensePolicyService struct {
	policy      types.LicensePolicy
	policyFile  string // "default" or absolute path
	configStore ConfigStore
	lockStore   LockStore
}

// NewLicensePolicyService creates a LicensePolicyService from a loaded policy.
func NewLicensePolicyService(
	policy *types.LicensePolicy,
	policyFile string,
	configStore ConfigStore,
	lockStore LockStore,
) *LicensePolicyService {
	return &LicensePolicyService{
		policy:      *policy,
		policyFile:  policyFile,
		configStore: configStore,
		lockStore:   lockStore,
	}
}

// Evaluate determines the policy decision for a given SPDX license identifier.
// Evaluation order: deny list → allow list → warn list → unknown rule.
func (s *LicensePolicyService) Evaluate(license string) string {
	rules := s.policy.LicensePolicy

	// Check deny list first (deny always wins)
	for _, denied := range rules.Deny {
		if strings.EqualFold(denied, license) {
			return types.PolicyDeny
		}
	}

	// Check allow list
	for _, allowed := range rules.Allow {
		if strings.EqualFold(allowed, license) {
			return types.PolicyAllow
		}
	}

	// Check warn list
	for _, warned := range rules.Warn {
		if strings.EqualFold(warned, license) {
			return types.PolicyWarn
		}
	}

	// License not in any list — apply unknown rule
	if license == "UNKNOWN" || license == "NONE" || license == "" {
		return rules.Unknown
	}

	// License detected but not in any list — also use unknown rule
	return rules.Unknown
}

// GenerateReport builds a license compliance report for all vendored dependencies.
// failOn: "deny" (default) means only denied licenses cause FAIL.
// failOn: "warn" means both denied and warned licenses cause FAIL.
func (s *LicensePolicyService) GenerateReport(failOn string) (*types.LicenseReportResult, error) {
	if failOn == "" {
		failOn = types.PolicyDeny
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config for license report: %w", err)
	}

	lock, lockErr := s.lockStore.Load()

	result := &types.LicenseReportResult{
		SchemaVersion: licensePolicySchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		PolicyFile:    s.policyFile,
		Vendors:       make([]types.VendorLicenseStatus, 0, len(cfg.Vendors)),
	}

	for _, vendor := range cfg.Vendors {
		// Skip internal vendors — no external license concept
		if vendor.Source == SourceInternal {
			continue
		}

		license := vendor.License

		// Try to enrich from lockfile if config license is empty
		if license == "" && lockErr == nil {
			license = findLicenseInLock(lock, vendor.Name)
		}

		if license == "" {
			license = "UNKNOWN"
		}

		decision := s.Evaluate(license)
		reason := buildReason(license, decision, &s.policy)

		result.Vendors = append(result.Vendors, types.VendorLicenseStatus{
			Name:     vendor.Name,
			URL:      vendor.URL,
			License:  license,
			Decision: decision,
			Reason:   reason,
		})

		// Update summary counts
		switch decision {
		case types.PolicyAllow:
			result.Summary.Allowed++
		case types.PolicyDeny:
			result.Summary.Denied++
		case types.PolicyWarn:
			result.Summary.Warned++
		}

		if license == "UNKNOWN" || license == "NONE" {
			result.Summary.Unknown++
		}
	}

	result.Summary.TotalVendors = len(cfg.Vendors)

	// Determine overall result
	switch {
	case result.Summary.Denied > 0:
		result.Summary.Result = "FAIL"
	case failOn == types.PolicyWarn && result.Summary.Warned > 0:
		result.Summary.Result = "FAIL"
	case result.Summary.Warned > 0:
		result.Summary.Result = "WARN"
	default:
		result.Summary.Result = "PASS"
	}

	return result, nil
}

// PolicyFile returns the path to the loaded policy file.
func (s *LicensePolicyService) PolicyFile() string {
	return s.policyFile
}

// findLicenseInLock searches the lockfile for a license SPDX identifier for a vendor.
func findLicenseInLock(lock types.VendorLock, vendorName string) string {
	for _, details := range lock.Vendors {
		if details.Name == vendorName && details.LicenseSPDX != "" {
			return details.LicenseSPDX
		}
	}
	return ""
}

// buildReason generates a human-readable reason for the policy decision.
func buildReason(license, decision string, policy *types.LicensePolicy) string {
	switch {
	case license == "UNKNOWN" || license == "NONE" || license == "":
		return fmt.Sprintf("license not detected; unknown policy is %q", policy.LicensePolicy.Unknown)
	case decision == types.PolicyAllow:
		return fmt.Sprintf("%s is in the allow list", license)
	case decision == types.PolicyDeny:
		return fmt.Sprintf("%s is in the deny list", license)
	case decision == types.PolicyWarn:
		// Could be explicit warn list or unknown rule
		for _, w := range policy.LicensePolicy.Warn {
			if strings.EqualFold(w, license) {
				return fmt.Sprintf("%s is in the warn list", license)
			}
		}
		return fmt.Sprintf("%s is not in any list; unknown policy is %q", license, policy.LicensePolicy.Unknown)
	default:
		return ""
	}
}
