package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Policy Loading Tests
// ============================================================================

func TestLoadLicensePolicy_DefaultWhenNoFile(t *testing.T) {
	policy, err := LoadLicensePolicy("/nonexistent/.git-vendor-policy.yml")
	if err != nil {
		t.Fatalf("LoadLicensePolicy returned error for missing file: %v", err)
	}

	// Default policy should match AllowedLicenses
	if len(policy.LicensePolicy.Allow) != len(AllowedLicenses) {
		t.Errorf("expected %d allowed licenses, got %d", len(AllowedLicenses), len(policy.LicensePolicy.Allow))
	}
	if len(policy.LicensePolicy.Deny) != 0 {
		t.Errorf("expected empty deny list, got %d entries", len(policy.LicensePolicy.Deny))
	}
	if policy.LicensePolicy.Unknown != types.PolicyWarn {
		t.Errorf("expected unknown=%q, got %q", types.PolicyWarn, policy.LicensePolicy.Unknown)
	}
}

func TestLoadLicensePolicy_ParsesValidFile(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	content := `license_policy:
  allow:
    - MIT
    - Apache-2.0
  deny:
    - GPL-3.0-only
    - AGPL-3.0-only
  warn:
    - LGPL-2.1-only
    - MPL-2.0
  unknown: deny
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadLicensePolicy returned error: %v", err)
	}

	if len(policy.LicensePolicy.Allow) != 2 {
		t.Errorf("expected 2 allowed, got %d", len(policy.LicensePolicy.Allow))
	}
	if len(policy.LicensePolicy.Deny) != 2 {
		t.Errorf("expected 2 denied, got %d", len(policy.LicensePolicy.Deny))
	}
	if len(policy.LicensePolicy.Warn) != 2 {
		t.Errorf("expected 2 warned, got %d", len(policy.LicensePolicy.Warn))
	}
	if policy.LicensePolicy.Unknown != "deny" {
		t.Errorf("expected unknown=deny, got %q", policy.LicensePolicy.Unknown)
	}
}

func TestLoadLicensePolicy_RejectsDuplicateAcrossLists(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	content := `license_policy:
  allow:
    - MIT
  deny:
    - MIT
  unknown: warn
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for duplicate license across allow and deny lists")
	}
}

func TestLoadLicensePolicy_RejectsInvalidUnknown(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	content := `license_policy:
  allow:
    - MIT
  unknown: reject
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for invalid unknown value")
	}
}

func TestLoadLicensePolicy_DefaultsUnknownToWarn(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	content := `license_policy:
  allow:
    - MIT
  deny:
    - GPL-3.0-only
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadLicensePolicy returned error: %v", err)
	}
	if policy.LicensePolicy.Unknown != types.PolicyWarn {
		t.Errorf("expected unknown to default to %q, got %q", types.PolicyWarn, policy.LicensePolicy.Unknown)
	}
}

func TestLoadLicensePolicy_RejectsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	if err := os.WriteFile(policyPath, []byte("{{{{bad yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

// ============================================================================
// Policy Evaluation Tests
// ============================================================================

func TestEvaluate_AllowedLicense(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT", "Apache-2.0"},
			Deny:    []string{"GPL-3.0-only"},
			Warn:    []string{"MPL-2.0"},
			Unknown: "deny",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("MIT"); got != types.PolicyAllow {
		t.Errorf("Evaluate(MIT) = %q, want %q", got, types.PolicyAllow)
	}
	if got := svc.Evaluate("Apache-2.0"); got != types.PolicyAllow {
		t.Errorf("Evaluate(Apache-2.0) = %q, want %q", got, types.PolicyAllow)
	}
}

func TestEvaluate_DeniedLicense(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Unknown: "warn",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("GPL-3.0-only"); got != types.PolicyDeny {
		t.Errorf("Evaluate(GPL-3.0-only) = %q, want %q", got, types.PolicyDeny)
	}
}

func TestEvaluate_WarnedLicense(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Warn:    []string{"LGPL-2.1-only", "MPL-2.0"},
			Unknown: "deny",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("MPL-2.0"); got != types.PolicyWarn {
		t.Errorf("Evaluate(MPL-2.0) = %q, want %q", got, types.PolicyWarn)
	}
}

func TestEvaluate_UnknownLicenseDeny(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "deny",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("UNKNOWN"); got != types.PolicyDeny {
		t.Errorf("Evaluate(UNKNOWN) = %q, want %q", got, types.PolicyDeny)
	}
}

func TestEvaluate_UnknownLicenseAllow(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "allow",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("UNKNOWN"); got != types.PolicyAllow {
		t.Errorf("Evaluate(UNKNOWN) = %q, want %q", got, types.PolicyAllow)
	}
}

func TestEvaluate_UnlistedLicenseUsesUnknownRule(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Unknown: "warn",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	// BSL-1.0 is not in any list
	if got := svc.Evaluate("BSL-1.0"); got != types.PolicyWarn {
		t.Errorf("Evaluate(BSL-1.0) = %q, want %q", got, types.PolicyWarn)
	}
}

func TestEvaluate_CaseInsensitive(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Unknown: "warn",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	if got := svc.Evaluate("mit"); got != types.PolicyAllow {
		t.Errorf("Evaluate(mit) = %q, want %q", got, types.PolicyAllow)
	}
	if got := svc.Evaluate("gpl-3.0-only"); got != types.PolicyDeny {
		t.Errorf("Evaluate(gpl-3.0-only) = %q, want %q", got, types.PolicyDeny)
	}
}

func TestEvaluate_DenyOverridesAllow(t *testing.T) {
	// If somehow a license appears in both (shouldn't pass validation, but test defense-in-depth)
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"MIT"},
			Unknown: "warn",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	// Deny MUST win over allow
	if got := svc.Evaluate("MIT"); got != types.PolicyDeny {
		t.Errorf("Evaluate(MIT) with both allow and deny = %q, want %q", got, types.PolicyDeny)
	}
}

// ============================================================================
// Report Generation Tests
// ============================================================================

// stubConfigStore returns a fixed config for report generation tests.
type stubConfigStore struct {
	config types.VendorConfig
	err    error
}

func (s *stubConfigStore) Load() (types.VendorConfig, error) { return s.config, s.err }
func (s *stubConfigStore) Save(_ types.VendorConfig) error    { return nil }
func (s *stubConfigStore) Path() string                       { return ".git-vendor/vendor.yml" }

// stubLockStore returns a fixed lock for report generation tests.
type stubLockStore struct {
	lock types.VendorLock
	err  error
}

func (s *stubLockStore) Load() (types.VendorLock, error) { return s.lock, s.err }
func (s *stubLockStore) Save(_ types.VendorLock) error    { return nil }
func (s *stubLockStore) Path() string                     { return ".git-vendor/vendor.lock" }
func (s *stubLockStore) GetHash(vendorName, ref string) string {
	for _, v := range s.lock.Vendors {
		if v.Name == vendorName && v.Ref == ref {
			return v.CommitHash
		}
	}
	return ""
}

func TestGenerateReport_AllAllowed(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT", "Apache-2.0"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: "MIT"},
			{Name: "lib-b", URL: "https://github.com/owner/lib-b", License: "Apache-2.0"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "default", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS, got %q", result.Summary.Result)
	}
	if result.Summary.Allowed != 2 {
		t.Errorf("expected 2 allowed, got %d", result.Summary.Allowed)
	}
	if result.Summary.Denied != 0 {
		t.Errorf("expected 0 denied, got %d", result.Summary.Denied)
	}
}

func TestGenerateReport_DeniedLicenseFails(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: "MIT"},
			{Name: "lib-gpl", URL: "https://github.com/owner/lib-gpl", License: "GPL-3.0-only"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL, got %q", result.Summary.Result)
	}
	if result.Summary.Denied != 1 {
		t.Errorf("expected 1 denied, got %d", result.Summary.Denied)
	}
}

func TestGenerateReport_WarnedLicenseWarnResult(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Warn:    []string{"MPL-2.0"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: "MIT"},
			{Name: "lib-mpl", URL: "https://github.com/owner/lib-mpl", License: "MPL-2.0"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("expected WARN, got %q", result.Summary.Result)
	}
	if result.Summary.Warned != 1 {
		t.Errorf("expected 1 warned, got %d", result.Summary.Warned)
	}
}

func TestGenerateReport_FailOnWarnEscalates(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Warn:    []string{"MPL-2.0"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-mpl", URL: "https://github.com/owner/lib-mpl", License: "MPL-2.0"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("warn")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL with --fail-on=warn, got %q", result.Summary.Result)
	}
}

func TestGenerateReport_UnknownLicenseCountedSeparately(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-unknown", URL: "https://example.com/repo", License: "UNKNOWN"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "default", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Unknown != 1 {
		t.Errorf("expected 1 unknown, got %d", result.Summary.Unknown)
	}
}

func TestGenerateReport_FallsBackToLockfile(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Unknown: "deny",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: ""}, // No license in config
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", LicenseSPDX: "MIT"},
		},
	}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS (MIT from lockfile), got %q", result.Summary.Result)
	}
	if result.Vendors[0].License != "MIT" {
		t.Errorf("expected license MIT from lockfile, got %q", result.Vendors[0].License)
	}
}

func TestGenerateReport_EmptyVendorList(t *testing.T) {
	policy := DefaultLicensePolicy()
	config := &stubConfigStore{config: types.VendorConfig{Vendors: []types.VendorSpec{}}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "default", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS for empty vendor list, got %q", result.Summary.Result)
	}
	if result.Summary.TotalVendors != 0 {
		t.Errorf("expected 0 total vendors, got %d", result.Summary.TotalVendors)
	}
}

func TestGenerateReport_PolicyFileRecorded(t *testing.T) {
	policy := DefaultLicensePolicy()
	config := &stubConfigStore{config: types.VendorConfig{Vendors: []types.VendorSpec{}}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "/my/custom/policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	if result.PolicyFile != "/my/custom/policy.yml" {
		t.Errorf("expected policy file /my/custom/policy.yml, got %q", result.PolicyFile)
	}
}

// ============================================================================
// Validation Edge Cases
// ============================================================================

func TestValidatePolicy_AllowAndWarnOverlap(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yml")
	content := `license_policy:
  allow:
    - MIT
  warn:
    - MIT
  unknown: warn
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for license appearing in both allow and warn lists")
	}
}

func TestValidatePolicy_DenyAndWarnOverlap(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yml")
	content := `license_policy:
  deny:
    - GPL-3.0-only
  warn:
    - GPL-3.0-only
  unknown: warn
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for license appearing in both deny and warn lists")
	}
}

func TestValidatePolicy_CaseInsensitiveDuplicateDetection(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yml")
	content := `license_policy:
  allow:
    - mit
  deny:
    - MIT
  unknown: warn
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for case-insensitive duplicate across allow and deny lists")
	}
}

// ============================================================================
// Evaluate Edge Cases
// ============================================================================

func TestEvaluate_EmptyStringLicense(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "deny",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	// Empty string MUST use the unknown rule
	if got := svc.Evaluate(""); got != types.PolicyDeny {
		t.Errorf("Evaluate(\"\") = %q, want %q", got, types.PolicyDeny)
	}
}

func TestEvaluate_NONELicense(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "warn",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	// "NONE" (no license detected in repo) MUST use the unknown rule
	if got := svc.Evaluate("NONE"); got != types.PolicyWarn {
		t.Errorf("Evaluate(NONE) = %q, want %q", got, types.PolicyWarn)
	}
}

func TestEvaluate_AllListsEmpty(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{},
			Deny:    []string{},
			Warn:    []string{},
			Unknown: "allow",
		},
	}
	svc := NewLicensePolicyService(&policy, "test", nil, nil)

	// Everything falls through to unknown rule when all lists empty
	if got := svc.Evaluate("MIT"); got != types.PolicyAllow {
		t.Errorf("Evaluate(MIT) with empty lists = %q, want %q (unknown rule)", got, types.PolicyAllow)
	}
}

// ============================================================================
// checkWithPolicy Integration Tests (via CheckCompliance)
// ============================================================================

func TestCheckCompliance_PolicyDeniesLicense(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	policyContent := `license_policy:
  allow:
    - MIT
  deny:
    - GPL-3.0
  unknown: warn
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockUI := &capturingUICallback{}
	// checkWithPolicy only needs UI callback; checker and fs are not called
	licenseService := NewLicenseService(nil, nil, "vendor", mockUI)

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadLicensePolicy: %v", err)
	}
	result, err := licenseService.checkWithPolicy("GPL-3.0", &policy)

	// Denied license MUST return ErrComplianceFailed
	if !errors.Is(err, ErrComplianceFailed) {
		t.Errorf("expected ErrComplianceFailed, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for denied license, got %q", result)
	}
	if mockUI.errorMsg == "" {
		t.Error("expected ShowError to be called for denied license")
	}
}

func TestCheckCompliance_PolicyWarnsLicense_UserRejects(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	policyContent := `license_policy:
  allow:
    - MIT
  warn:
    - MPL-2.0
  unknown: deny
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// User REJECTS the warned license
	mockUI := &capturingUICallback{confirmResp: false}
	licenseService := NewLicenseService(nil, nil, "vendor", mockUI)

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := licenseService.checkWithPolicy("MPL-2.0", &policy)

	if !errors.Is(err, ErrComplianceFailed) {
		t.Errorf("expected ErrComplianceFailed when user rejects warned license, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestCheckCompliance_PolicyWarnsLicense_UserAccepts(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	policyContent := `license_policy:
  allow:
    - MIT
  warn:
    - MPL-2.0
  unknown: deny
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// User ACCEPTS the warned license
	mockUI := &capturingUICallback{confirmResp: true}
	licenseService := NewLicenseService(nil, nil, "vendor", mockUI)

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := licenseService.checkWithPolicy("MPL-2.0", &policy)

	if err != nil {
		t.Fatalf("expected success when user accepts warned license, got %v", err)
	}
	if result != "MPL-2.0" {
		t.Errorf("expected MPL-2.0, got %q", result)
	}
}

func TestCheckCompliance_PolicyAllowsLicense(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	policyContent := `license_policy:
  allow:
    - MIT
  deny:
    - GPL-3.0
  unknown: deny
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockUI := &capturingUICallback{}
	licenseService := NewLicenseService(nil, nil, "vendor", mockUI)

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := licenseService.checkWithPolicy("MIT", &policy)

	if err != nil {
		t.Fatalf("expected success for allowed license, got %v", err)
	}
	if result != "MIT" {
		t.Errorf("expected MIT, got %q", result)
	}
	if mockUI.licenseMsg != "MIT" {
		t.Errorf("expected ShowLicenseCompliance(MIT), got %q", mockUI.licenseMsg)
	}
}

func TestCheckCompliance_DetectionFailureWithPolicy_UnknownDenied(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	policyContent := `license_policy:
  allow:
    - MIT
  unknown: deny
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockUI := &capturingUICallback{}
	licenseService := NewLicenseService(nil, nil, "vendor", mockUI)

	policy, err := LoadLicensePolicy(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	// "UNKNOWN" license against policy with unknown=deny
	result, err := licenseService.checkWithPolicy("UNKNOWN", &policy)

	if !errors.Is(err, ErrComplianceFailed) {
		t.Errorf("expected ErrComplianceFailed for UNKNOWN with unknown=deny, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// ============================================================================
// buildReason Tests
// ============================================================================

func TestBuildReason_AllBranches(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0"},
			Warn:    []string{"MPL-2.0"},
			Unknown: "deny",
		},
	}

	tests := []struct {
		name     string
		license  string
		decision string
		contains string
	}{
		{"UNKNOWN license", "UNKNOWN", types.PolicyDeny, "license not detected"},
		{"NONE license", "NONE", types.PolicyDeny, "license not detected"},
		{"empty license", "", types.PolicyDeny, "license not detected"},
		{"allowed license", "MIT", types.PolicyAllow, "is in the allow list"},
		{"denied license", "GPL-3.0", types.PolicyDeny, "is in the deny list"},
		{"warned license in warn list", "MPL-2.0", types.PolicyWarn, "is in the warn list"},
		{"warned via unknown rule", "BSL-1.0", types.PolicyWarn, "is not in any list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := buildReason(tt.license, tt.decision, &policy)
			if !strings.Contains(reason, tt.contains) {
				t.Errorf("buildReason(%q, %q) = %q, expected to contain %q",
					tt.license, tt.decision, reason, tt.contains)
			}
		})
	}
}

// ============================================================================
// findLicenseInLock Tests
// ============================================================================

func TestFindLicenseInLock_Found(t *testing.T) {
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", LicenseSPDX: "MIT"},
			{Name: "lib-b", Ref: "main", LicenseSPDX: "Apache-2.0"},
		},
	}
	if got := findLicenseInLock(lock, "lib-b"); got != "Apache-2.0" {
		t.Errorf("findLicenseInLock(lib-b) = %q, want Apache-2.0", got)
	}
}

func TestFindLicenseInLock_NotFound(t *testing.T) {
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", LicenseSPDX: "MIT"},
		},
	}
	if got := findLicenseInLock(lock, "lib-nonexistent"); got != "" {
		t.Errorf("findLicenseInLock(lib-nonexistent) = %q, want empty", got)
	}
}

func TestFindLicenseInLock_EmptyLockfile(t *testing.T) {
	lock := types.VendorLock{}
	if got := findLicenseInLock(lock, "lib-a"); got != "" {
		t.Errorf("findLicenseInLock on empty lock = %q, want empty", got)
	}
}

func TestFindLicenseInLock_MultipleRefsReturnsFirst(t *testing.T) {
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", LicenseSPDX: "MIT"},
			{Name: "lib-a", Ref: "v2.0", LicenseSPDX: "Apache-2.0"},
		},
	}
	// Returns first matching entry
	if got := findLicenseInLock(lock, "lib-a"); got != "MIT" {
		t.Errorf("findLicenseInLock(lib-a) = %q, want MIT (first match)", got)
	}
}

func TestFindLicenseInLock_SkipsEmptySPDX(t *testing.T) {
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", LicenseSPDX: ""},     // Empty SPDX
			{Name: "lib-a", LicenseSPDX: "MIT"},   // Non-empty SPDX
		},
	}
	if got := findLicenseInLock(lock, "lib-a"); got != "MIT" {
		t.Errorf("findLicenseInLock should skip empty SPDX, got %q", got)
	}
}

// ============================================================================
// GenerateReport Error Cases
// ============================================================================

func TestGenerateReport_ConfigLoadError(t *testing.T) {
	policy := DefaultLicensePolicy()
	config := &stubConfigStore{err: fmt.Errorf("permission denied")}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "default", config, lock)
	_, err := svc.GenerateReport("deny")
	if err == nil {
		t.Fatal("expected error when config load fails")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("error should mention config loading, got %q", err.Error())
	}
}

func TestGenerateReport_LockfileErrorFallsBackToUnknown(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "warn",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/owner/lib-a", License: ""}, // No config license
		},
	}}
	// Lockfile load fails
	lock := &stubLockStore{err: fmt.Errorf("lockfile corrupt")}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport should not fail on lockfile error, got %v", err)
	}

	// Without lockfile fallback, vendor should have UNKNOWN license
	if result.Vendors[0].License != "UNKNOWN" {
		t.Errorf("expected UNKNOWN when lockfile unavailable, got %q", result.Vendors[0].License)
	}
	if result.Summary.Unknown != 1 {
		t.Errorf("expected 1 unknown, got %d", result.Summary.Unknown)
	}
}

func TestGenerateReport_MixedDenyAndWarn(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Deny:    []string{"GPL-3.0-only"},
			Warn:    []string{"MPL-2.0"},
			Unknown: "allow",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-mit", URL: "https://github.com/owner/lib-mit", License: "MIT"},
			{Name: "lib-gpl", URL: "https://github.com/owner/lib-gpl", License: "GPL-3.0-only"},
			{Name: "lib-mpl", URL: "https://github.com/owner/lib-mpl", License: "MPL-2.0"},
			{Name: "lib-bsd", URL: "https://github.com/owner/lib-bsd", License: "BSD-3-Clause"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)

	// With failOn=deny, FAIL because of GPL deny
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL (has denied), got %q", result.Summary.Result)
	}
	// MIT in allow list + BSD-3-Clause via unknown=allow = 2
	if result.Summary.Allowed != 2 {
		t.Errorf("expected 2 allowed (MIT + BSD via unknown=allow), got %d", result.Summary.Allowed)
	}
	if result.Summary.Denied != 1 {
		t.Errorf("expected 1 denied (GPL), got %d", result.Summary.Denied)
	}
	if result.Summary.Warned != 1 {
		t.Errorf("expected 1 warned (MPL), got %d", result.Summary.Warned)
	}
	if result.Summary.TotalVendors != 4 {
		t.Errorf("expected 4 total vendors, got %d", result.Summary.TotalVendors)
	}
}

func TestGenerateReport_FailOnWarnWithDenyAndWarn(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow: []string{"MIT"},
			Deny:  []string{"GPL-3.0-only"},
			Warn:  []string{"MPL-2.0"},
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-gpl", URL: "https://github.com/owner/lib-gpl", License: "GPL-3.0-only"},
			{Name: "lib-mpl", URL: "https://github.com/owner/lib-mpl", License: "MPL-2.0"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("warn")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	// Deny takes precedence in result determination — FAIL regardless of failOn
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL (deny takes precedence over warn), got %q", result.Summary.Result)
	}
}

func TestGenerateReport_DefaultFailOnIsEmpty(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow: []string{"MIT"},
			Warn:  []string{"MPL-2.0"},
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-mpl", License: "MPL-2.0"},
		},
	}}
	lock := &stubLockStore{lock: types.VendorLock{}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	// Pass empty failOn — should default to "deny"
	result, err := svc.GenerateReport("")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	// With default failOn="deny" and only warn violations, result should be WARN not FAIL
	if result.Summary.Result != "WARN" {
		t.Errorf("expected WARN with default failOn, got %q", result.Summary.Result)
	}
}

func TestGenerateReport_VendorInConfigButNotInLock(t *testing.T) {
	policy := types.LicensePolicy{
		LicensePolicy: types.LicensePolicyRules{
			Allow:   []string{"MIT"},
			Unknown: "deny",
		},
	}
	config := &stubConfigStore{config: types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-new", URL: "https://github.com/owner/lib-new", License: ""},
		},
	}}
	// Lock has entries but not for this vendor
	lock := &stubLockStore{lock: types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-other", LicenseSPDX: "MIT"},
		},
	}}

	svc := NewLicensePolicyService(&policy, "policy.yml", config, lock)
	result, err := svc.GenerateReport("deny")
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}

	// Vendor not found in lock → UNKNOWN → denied by unknown=deny
	if result.Vendors[0].License != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for vendor not in lock, got %q", result.Vendors[0].License)
	}
	if result.Summary.Denied != 1 {
		t.Errorf("expected 1 denied (UNKNOWN with unknown=deny), got %d", result.Summary.Denied)
	}
}

// ============================================================================
// LoadLicensePolicy Additional Edge Cases
// ============================================================================

func TestLoadLicensePolicy_ReadPermissionError(t *testing.T) {
	// Root bypasses file permissions — skip when running as root
	if os.Getuid() == 0 {
		t.Skip("test requires non-root to enforce file permissions")
	}

	dir := t.TempDir()
	policyPath := filepath.Join(dir, ".git-vendor-policy.yml")
	if err := os.WriteFile(policyPath, []byte("license_policy:\n  allow: [MIT]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Remove read permission
	if err := os.Chmod(policyPath, 0000); err != nil {
		t.Skip("cannot test permission error on this platform")
	}
	defer os.Chmod(policyPath, 0644) //nolint:errcheck

	_, err := LoadLicensePolicy(policyPath)
	if err == nil {
		t.Fatal("expected error for unreadable policy file")
	}
	if strings.Contains(err.Error(), "not exist") {
		t.Errorf("error should be about permissions, not existence: %q", err.Error())
	}
}
