// Package types defines data structures for git-vendor configuration and state management.
package types

// LicensePolicy defines configurable allow/deny/warn lists for license compliance enforcement.
// LicensePolicy is loaded from .git-vendor-policy.yml and evaluated by LicensePolicyService.
type LicensePolicy struct {
	LicensePolicy LicensePolicyRules `yaml:"license_policy"`
}

// LicensePolicyRules contains the allow/deny/warn lists and the unknown license handling strategy.
type LicensePolicyRules struct {
	Allow   []string `yaml:"allow"`   // Licenses explicitly permitted (SPDX identifiers)
	Deny    []string `yaml:"deny"`    // Licenses explicitly blocked (SPDX identifiers)
	Warn    []string `yaml:"warn"`    // Licenses that emit warnings but do not block (SPDX identifiers)
	Unknown string   `yaml:"unknown"` // How to handle undetected licenses: "allow", "warn", or "deny"
}

// PolicyDecision represents the outcome of evaluating a license against a LicensePolicy.
const (
	PolicyAllow = "allow"
	PolicyDeny  = "deny"
	PolicyWarn  = "warn"
)

// LicenseReportResult represents the complete license policy report output.
// LicenseReportResult is the top-level structure returned by the license command
// and used for both JSON and table output formats.
type LicenseReportResult struct {
	SchemaVersion string                `json:"schema_version"`
	Timestamp     string                `json:"timestamp"`
	PolicyFile    string                `json:"policy_file"` // Path to policy file used, or "default" if none
	Summary       LicenseReportSummary  `json:"summary"`
	Vendors       []VendorLicenseStatus `json:"vendors"`
}

// LicenseReportSummary contains aggregate statistics for the license report.
type LicenseReportSummary struct {
	TotalVendors int    `json:"total_vendors"`
	Allowed      int    `json:"allowed"`
	Denied       int    `json:"denied"`
	Warned       int    `json:"warned"`
	Unknown      int    `json:"unknown"`
	Result       string `json:"result"` // PASS, FAIL, WARN
}

// VendorLicenseStatus represents the license compliance status for a single vendor.
type VendorLicenseStatus struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	License  string `json:"license"`  // Detected SPDX license identifier
	Decision string `json:"decision"` // "allow", "deny", or "warn"
	Reason   string `json:"reason"`   // Human-readable reason for the decision
}
