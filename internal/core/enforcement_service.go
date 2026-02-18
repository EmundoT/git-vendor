package core

import "github.com/EmundoT/git-vendor/internal/types"

// EnforcementService resolves per-vendor compliance enforcement levels
// and computes exit codes based on drift state and enforcement configuration (Spec 075).
type EnforcementService struct{}

// NewEnforcementService creates an EnforcementService.
func NewEnforcementService() *EnforcementService {
	return &EnforcementService{}
}

// ResolveVendorEnforcement returns a map of vendor name → resolved enforcement level.
// Resolution logic:
//   - nil ComplianceConfig → all vendors get EnforcementLenient (backward compat)
//   - mode "override" → all vendors get the global default regardless of per-vendor setting
//   - mode "default" (or empty) → per-vendor Enforcement wins; fallback to global Default; fallback to "lenient"
func (e *EnforcementService) ResolveVendorEnforcement(config *types.VendorConfig) map[string]string {
	result := make(map[string]string, len(config.Vendors))

	globalDefault := EnforcementLenient
	isOverride := false

	if config.Compliance != nil {
		if config.Compliance.Default != "" {
			globalDefault = config.Compliance.Default
		}
		isOverride = config.Compliance.Mode == ComplianceModeOverride
	}

	for _, vendor := range config.Vendors {
		if isOverride {
			result[vendor.Name] = globalDefault
		} else if vendor.Enforcement != "" {
			result[vendor.Name] = vendor.Enforcement
		} else {
			result[vendor.Name] = globalDefault
		}
	}

	return result
}

// ComputeExitCode determines the process exit code from vendor status details
// and their resolved enforcement levels.
//
// "Drift" for enforcement purposes means modified or deleted files only
// (FilesModified + FilesDeleted). Added files are not considered enforcement
// drift — they are handled by the legacy summary as WARN.
//
// Exit code semantics:
//   - 0: PASS (no actionable drift, or all drift is info-level)
//   - 1: FAIL (strict vendor has unacknowledged drift)
//   - 2: WARN (lenient vendor has unacknowledged drift, no strict failures)
//
// When multiple vendors have different enforcement levels, the highest severity wins.
func (e *EnforcementService) ComputeExitCode(vendors []types.VendorStatusDetail, enforcementMap map[string]string) int {
	hasStrict := false
	hasLenient := false

	for i := range vendors {
		v := &vendors[i]
		unackedDrift := v.FilesModified + v.FilesDeleted
		if unackedDrift == 0 {
			continue
		}

		level := enforcementMap[v.Name]
		switch level {
		case EnforcementStrict:
			hasStrict = true
		case EnforcementLenient:
			hasLenient = true
		case EnforcementInfo:
			// Info drift doesn't affect exit code
		}
	}

	switch {
	case hasStrict:
		return 1
	case hasLenient:
		return 2
	default:
		return 0
	}
}
