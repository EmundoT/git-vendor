package core

import (
	"context"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// --- ResolvedPolicy tests ---

func TestResolvedPolicy_Defaults(t *testing.T) {
	r := types.ResolvedPolicy(nil, nil)

	if !*r.BlockOnDrift {
		t.Error("ResolvedPolicy default BlockOnDrift should be true")
	}
	if *r.BlockOnStale {
		t.Error("ResolvedPolicy default BlockOnStale should be false")
	}
	if *r.MaxStalenessDays != 0 {
		t.Errorf("ResolvedPolicy default MaxStalenessDays should be 0, got %d", *r.MaxStalenessDays)
	}
}

func TestResolvedPolicy_GlobalOnly(t *testing.T) {
	f := false
	days := 30
	global := &types.VendorPolicy{
		BlockOnDrift:     &f,
		MaxStalenessDays: &days,
	}
	r := types.ResolvedPolicy(global, nil)

	if *r.BlockOnDrift {
		t.Error("ResolvedPolicy should use global BlockOnDrift=false")
	}
	if *r.BlockOnStale {
		t.Error("ResolvedPolicy should keep default BlockOnStale=false")
	}
	if *r.MaxStalenessDays != 30 {
		t.Errorf("ResolvedPolicy should use global MaxStalenessDays=30, got %d", *r.MaxStalenessDays)
	}
}

func TestResolvedPolicy_PerVendorOnly(t *testing.T) {
	tr := true
	days := 7
	perVendor := &types.VendorPolicy{
		BlockOnStale:     &tr,
		MaxStalenessDays: &days,
	}
	r := types.ResolvedPolicy(nil, perVendor)

	if !*r.BlockOnDrift {
		t.Error("ResolvedPolicy should keep default BlockOnDrift=true")
	}
	if !*r.BlockOnStale {
		t.Error("ResolvedPolicy should use per-vendor BlockOnStale=true")
	}
	if *r.MaxStalenessDays != 7 {
		t.Errorf("ResolvedPolicy should use per-vendor MaxStalenessDays=7, got %d", *r.MaxStalenessDays)
	}
}

func TestResolvedPolicy_PerVendorOverridesGlobal(t *testing.T) {
	tr := true
	f := false
	globalDays := 30
	vendorDays := 7

	global := &types.VendorPolicy{
		BlockOnDrift:     &tr,
		BlockOnStale:     &f,
		MaxStalenessDays: &globalDays,
	}
	perVendor := &types.VendorPolicy{
		BlockOnStale:     &tr,
		MaxStalenessDays: &vendorDays,
	}
	r := types.ResolvedPolicy(global, perVendor)

	if !*r.BlockOnDrift {
		t.Error("ResolvedPolicy should keep global BlockOnDrift=true (not overridden)")
	}
	if !*r.BlockOnStale {
		t.Error("ResolvedPolicy should use per-vendor BlockOnStale=true")
	}
	if *r.MaxStalenessDays != 7 {
		t.Errorf("ResolvedPolicy should use per-vendor MaxStalenessDays=7, got %d", *r.MaxStalenessDays)
	}
}

// --- PolicyService.EvaluatePolicy tests ---

func TestEvaluatePolicy_NoDrift_NoViolations(t *testing.T) {
	svc := NewPolicyService()
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", FilesVerified: 3},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for clean vendor, got %d", len(violations))
	}
}

func TestEvaluatePolicy_DriftBlocked(t *testing.T) {
	svc := NewPolicyService()
	config := &types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", FilesModified: 2},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Type != "drift" {
		t.Errorf("expected drift violation, got %s", violations[0].Type)
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity (block_on_drift defaults true), got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_DriftNotBlocked_WhenPolicyFalse(t *testing.T) {
	svc := NewPolicyService()
	f := false
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnDrift: &f},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", FilesModified: 1},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("expected warning severity when block_on_drift=false, got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleBlocked(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Type != "stale" {
		t.Errorf("expected stale violation, got %s", violations[0].Type)
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity when block_on_stale=true, got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleWarning_DefaultPolicy(t *testing.T) {
	svc := NewPolicyService()
	config := &types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("expected warning severity (block_on_stale defaults false), got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_PerVendorOverride(t *testing.T) {
	svc := NewPolicyService()
	f := false
	tr := true
	config := &types.VendorConfig{
		Policy: &types.VendorPolicy{BlockOnDrift: &tr}, // global: block drift
		Vendors: []types.VendorSpec{
			{Name: "relaxed-lib", Policy: &types.VendorPolicy{BlockOnDrift: &f}}, // override: don't block
			{Name: "strict-lib"},                                                  // inherits global
		},
	}
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "relaxed-lib", Ref: "main", FilesModified: 1},
			{Name: "strict-lib", Ref: "main", FilesModified: 1},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}

	// relaxed-lib should be warning (block_on_drift=false)
	var relaxed, strict *types.PolicyViolation
	for i := range violations {
		if violations[i].VendorName == "relaxed-lib" {
			relaxed = &violations[i]
		}
		if violations[i].VendorName == "strict-lib" {
			strict = &violations[i]
		}
	}
	if relaxed == nil || strict == nil {
		t.Fatal("expected violations for both vendors")
	}
	if relaxed.Severity != "warning" {
		t.Errorf("relaxed-lib should have warning severity, got %s", relaxed.Severity)
	}
	if strict.Severity != "error" {
		t.Errorf("strict-lib should have error severity, got %s", strict.Severity)
	}
}

func TestEvaluatePolicy_NilStatus(t *testing.T) {
	svc := NewPolicyService()
	violations := svc.EvaluatePolicy(nil, nil)
	if violations != nil {
		t.Errorf("expected nil violations for nil status, got %v", violations)
	}
}

func TestEvaluatePolicy_NilConfig(t *testing.T) {
	svc := NewPolicyService()
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", FilesModified: 1},
		},
	}
	// nil config means defaults apply (block_on_drift=true)
	violations := svc.EvaluatePolicy(nil, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation with nil config, got %d", len(violations))
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity with nil config (defaults), got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_DeletedFilesCountAsDrift(t *testing.T) {
	svc := NewPolicyService()
	config := &types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", FilesDeleted: 1},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for deleted files, got %d", len(violations))
	}
	if violations[0].Type != "drift" {
		t.Errorf("expected drift violation for deleted files, got %s", violations[0].Type)
	}
}

// --- GRD-003: Staleness with max_staleness_days threshold ---

func TestEvaluatePolicy_StaleWithinGracePeriod_Warning(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	days := 30
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr, MaxStalenessDays: &days},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	// Lock updated 5 days ago — within the 30-day grace window
	recentUpdate := time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: recentUpdate},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("expected warning severity within grace period, got %s", violations[0].Severity)
	}
	if violations[0].Type != "stale" {
		t.Errorf("expected stale violation, got %s", violations[0].Type)
	}
}

func TestEvaluatePolicy_StaleBeyondGracePeriod_Error(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	days := 7
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr, MaxStalenessDays: &days},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	// Lock updated 15 days ago — beyond the 7-day grace window
	oldUpdate := time.Now().UTC().Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: oldUpdate},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity beyond grace period, got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleNoGracePeriod_ImmediateError(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	// Lock updated recently, but max_staleness_days=0 (default, no grace)
	recentUpdate := time.Now().UTC().Add(-1 * 24 * time.Hour).Format(time.RFC3339)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: recentUpdate},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity with no grace period (max_staleness_days=0), got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleUnknownAge_Error(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	days := 30
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr, MaxStalenessDays: &days},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	// No LastUpdated (old lock entry without timestamp)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: ""},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity for unknown lock age (conservative), got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleNotBlocked_AlwaysWarning(t *testing.T) {
	svc := NewPolicyService()
	days := 7
	config := &types.VendorConfig{
		// BlockOnStale defaults to false
		Policy:  &types.VendorPolicy{MaxStalenessDays: &days},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	// Lock updated 15 days ago — beyond threshold, but block_on_stale=false
	oldUpdate := time.Now().UTC().Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: oldUpdate},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("expected warning severity when block_on_stale=false regardless of age, got %s", violations[0].Severity)
	}
}

func TestEvaluatePolicy_StaleMessageIncludesAge(t *testing.T) {
	svc := NewPolicyService()
	tr := true
	days := 7
	config := &types.VendorConfig{
		Policy:  &types.VendorPolicy{BlockOnStale: &tr, MaxStalenessDays: &days},
		Vendors: []types.VendorSpec{{Name: "mylib"}},
	}
	stale := true
	oldUpdate := time.Now().UTC().Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	status := &types.StatusResult{
		Vendors: []types.VendorStatusDetail{
			{Name: "mylib", Ref: "main", UpstreamStale: &stale, LastUpdated: oldUpdate},
		},
	}

	violations := svc.EvaluatePolicy(config, status)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if !contains(violations[0].Message, "lock age:") {
		t.Errorf("expected message to include lock age info, got %q", violations[0].Message)
	}
	if !contains(violations[0].Message, "threshold:") {
		t.Errorf("expected message to include threshold info, got %q", violations[0].Message)
	}
}

// --- lockAgeDays unit tests ---

func TestLockAgeDays_ValidTimestamp(t *testing.T) {
	ts := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	days := lockAgeDays(ts)
	if days < 9 || days > 11 {
		t.Errorf("expected ~10 days, got %d", days)
	}
}

func TestLockAgeDays_EmptyString(t *testing.T) {
	days := lockAgeDays("")
	if days != -1 {
		t.Errorf("expected -1 for empty string, got %d", days)
	}
}

func TestLockAgeDays_InvalidFormat(t *testing.T) {
	days := lockAgeDays("not-a-date")
	if days != -1 {
		t.Errorf("expected -1 for invalid format, got %d", days)
	}
}

func TestLockAgeDays_ZeroDays(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	days := lockAgeDays(ts)
	if days != 0 {
		t.Errorf("expected 0 for just-now timestamp, got %d", days)
	}
}

// --- Integration: policy violations in status output ---

func TestStatusService_PolicyViolations_InOutput(t *testing.T) {
	vendor1 := "mylib"
	tr := true

	// ConfigStore stub that returns policy config
	configStore := &statusStubConfigStore{
		config: types.VendorConfig{
			Policy:  &types.VendorPolicy{BlockOnDrift: &tr},
			Vendors: []types.VendorSpec{{Name: "mylib"}},
		},
	}

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Verified: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				Dependencies: []types.UpdateCheckResult{
					{VendorName: "mylib", Ref: "main", CurrentHash: "abc", LatestHash: "abc", UpToDate: true},
				},
				TotalChecked: 1, UpToDate: 1,
			},
		},
		configStore,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if len(result.PolicyViolations) != 1 {
		t.Fatalf("expected 1 policy violation, got %d", len(result.PolicyViolations))
	}
	if result.PolicyViolations[0].Type != "drift" {
		t.Errorf("expected drift violation, got %s", result.PolicyViolations[0].Type)
	}
	if result.PolicyViolations[0].Severity != "error" {
		t.Errorf("expected error severity, got %s", result.PolicyViolations[0].Severity)
	}

	// Also check per-vendor violations
	if len(result.Vendors[0].PolicyViolations) != 1 {
		t.Errorf("expected 1 per-vendor violation, got %d", len(result.Vendors[0].PolicyViolations))
	}
}

func TestStatusService_NoPolicySection_NoViolations(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Verified: 1, Result: "PASS"},
				Files:   []types.FileStatus{{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"}},
			},
		},
		&statusStubOutdated{err: errForTest},
		nil, // nil configStore
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if len(result.PolicyViolations) != 0 {
		t.Errorf("expected 0 policy violations with nil configStore, got %d", len(result.PolicyViolations))
	}
}

// --- YAML parsing test ---

func TestYAMLParsing_PolicySection(t *testing.T) {
	yamlData := `
policy:
  block_on_drift: true
  block_on_stale: false
  max_staleness_days: 30
vendors:
  - name: critical-schema
    url: https://example.com/schema.git
    license: MIT
    policy:
      block_on_stale: true
      max_staleness_days: 7
    specs:
      - ref: main
        mapping:
          - from: schema.proto
            to: proto/schema.proto
`
	var config types.VendorConfig
	err := yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		t.Fatalf("YAML unmarshal failed: %v", err)
	}

	if config.Policy == nil {
		t.Fatal("expected global policy to be parsed")
	}
	if !*config.Policy.BlockOnDrift {
		t.Error("expected global block_on_drift=true")
	}
	if *config.Policy.BlockOnStale {
		t.Error("expected global block_on_stale=false")
	}
	if *config.Policy.MaxStalenessDays != 30 {
		t.Errorf("expected global max_staleness_days=30, got %d", *config.Policy.MaxStalenessDays)
	}

	if len(config.Vendors) != 1 {
		t.Fatalf("expected 1 vendor, got %d", len(config.Vendors))
	}
	v := config.Vendors[0]
	if v.Policy == nil {
		t.Fatal("expected per-vendor policy to be parsed")
	}
	if !*v.Policy.BlockOnStale {
		t.Error("expected per-vendor block_on_stale=true")
	}
	if *v.Policy.MaxStalenessDays != 7 {
		t.Errorf("expected per-vendor max_staleness_days=7, got %d", *v.Policy.MaxStalenessDays)
	}
	// BlockOnDrift not set at per-vendor level — should be nil
	if v.Policy.BlockOnDrift != nil {
		t.Error("expected per-vendor block_on_drift to be nil (not set)")
	}

	// Test resolved policy merging
	resolved := types.ResolvedPolicy(config.Policy, v.Policy)
	if !*resolved.BlockOnDrift {
		t.Error("resolved block_on_drift should inherit global true")
	}
	if !*resolved.BlockOnStale {
		t.Error("resolved block_on_stale should use per-vendor true")
	}
	if *resolved.MaxStalenessDays != 7 {
		t.Errorf("resolved max_staleness_days should use per-vendor 7, got %d", *resolved.MaxStalenessDays)
	}
}

// --- Test stubs ---

// statusStubConfigStore returns a pre-configured VendorConfig.
type statusStubConfigStore struct {
	config types.VendorConfig
	err    error
}

func (s *statusStubConfigStore) Load() (types.VendorConfig, error) { return s.config, s.err }
func (s *statusStubConfigStore) Save(_ types.VendorConfig) error   { return nil }
func (s *statusStubConfigStore) Path() string                      { return "vendor.yml" }
