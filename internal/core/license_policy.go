package core

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// DefaultLicensePolicy returns a policy based on the existing AllowedLicenses list.
// DefaultLicensePolicy is used when no .git-vendor-policy.yml file is found.
func DefaultLicensePolicy() types.LicensePolicy {
	return types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   AllowedLicenses,
			Deny:    []string{},
			Warn:    []string{},
			Unknown: types.PolicyWarn,
		},
	}
}

// LoadLicensePolicy reads and parses a license policy file.
// LoadLicensePolicy returns DefaultLicensePolicy when the file does not exist.
// LoadLicensePolicy returns an error if the file exists but is malformed.
func LoadLicensePolicy(path string) (types.LicensePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultLicensePolicy(), nil
		}
		return types.LicensePolicy{}, fmt.Errorf("read license policy %s: %w", path, err)
	}

	var policy types.LicensePolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return types.LicensePolicy{}, fmt.Errorf("parse license policy %s: %w", path, err)
	}

	if err := validatePolicy(&policy); err != nil {
		return types.LicensePolicy{}, fmt.Errorf("invalid license policy %s: %w", path, err)
	}

	// Normalize: default unknown to "warn" if not set
	if policy.LicensePolicy.Unknown == "" {
		policy.LicensePolicy.Unknown = types.PolicyWarn
	}

	return policy, nil
}

// validatePolicy checks a license policy for logical errors.
// A license MUST NOT appear in more than one list (allow, deny, warn).
// The "unknown" field MUST be one of: "allow", "warn", "deny".
func validatePolicy(policy *types.LicensePolicy) error {
	rules := policy.LicensePolicy

	// Check unknown field value
	if rules.Unknown != "" {
		switch rules.Unknown {
		case types.PolicyAllow, types.PolicyDeny, types.PolicyWarn:
			// valid
		default:
			return fmt.Errorf("unknown field must be \"allow\", \"warn\", or \"deny\", got %q", rules.Unknown)
		}
	}

	// Build a set of all licenses to check for overlaps
	seen := make(map[string]string) // license → list name
	for _, lic := range rules.Allow {
		seen[strings.ToUpper(lic)] = "allow"
	}
	for _, lic := range rules.Deny {
		upper := strings.ToUpper(lic)
		if prev, ok := seen[upper]; ok {
			return fmt.Errorf("license %q appears in both %s and deny lists", lic, prev)
		}
		seen[upper] = "deny"
	}
	for _, lic := range rules.Warn {
		upper := strings.ToUpper(lic)
		if prev, ok := seen[upper]; ok {
			return fmt.Errorf("license %q appears in both %s and warn lists", lic, prev)
		}
	}

	return nil
}
