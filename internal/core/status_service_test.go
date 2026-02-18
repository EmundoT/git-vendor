package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// statusStubVerify returns a pre-configured VerifyResult.
type statusStubVerify struct {
	result *types.VerifyResult
	err    error
}

func (s *statusStubVerify) Verify(_ context.Context) (*types.VerifyResult, error) {
	return s.result, s.err
}

// statusStubOutdated returns a pre-configured OutdatedResult.
type statusStubOutdated struct {
	result *types.OutdatedResult
	err    error
}

func (s *statusStubOutdated) Outdated(_ context.Context, _ OutdatedOptions) (*types.OutdatedResult, error) {
	return s.result, s.err
}

// statusStubLockStore returns a pre-configured VendorLock.
type statusStubLockStore struct {
	lock types.VendorLock
	err  error
}

func (s *statusStubLockStore) Load() (types.VendorLock, error)       { return s.lock, s.err }
func (s *statusStubLockStore) Save(_ types.VendorLock) error         { return nil }
func (s *statusStubLockStore) Path() string                          { return "vendor.lock" }
func (s *statusStubLockStore) GetHash(_, _ string) string            { return "" }

func TestStatusService_AllClean(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{
					TotalFiles: 3,
					Verified:   3,
					Result:     "PASS",
				},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "c.go", Vendor: &vendor1, Status: "verified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				Dependencies: []types.UpdateCheckResult{
					{VendorName: "mylib", Ref: "main", CurrentHash: "abc123", LatestHash: "abc123", UpToDate: true},
				},
				TotalChecked: 1,
				UpToDate:     1,
			},
		},
		nil, // configStore not used
		&statusStubLockStore{
			lock: types.VendorLock{
				Vendors: []types.LockDetails{
					{Name: "mylib", Ref: "main", CommitHash: "abc123def456"},
				},
			},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS, got %s", result.Summary.Result)
	}
	if result.Summary.TotalVendors != 1 {
		t.Errorf("expected 1 vendor, got %d", result.Summary.TotalVendors)
	}
	if result.Summary.Verified != 3 {
		t.Errorf("expected 3 verified, got %d", result.Summary.Verified)
	}
	if len(result.Vendors) != 1 {
		t.Fatalf("expected 1 vendor detail, got %d", len(result.Vendors))
	}
	v := result.Vendors[0]
	if v.UpstreamStale == nil || *v.UpstreamStale {
		t.Error("expected upstream not stale")
	}
}

func TestStatusService_ModifiedFile_FAIL(t *testing.T) {
	vendor1 := "mylib"
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
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL, got %s", result.Summary.Result)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("expected 1 modified, got %d", result.Summary.Modified)
	}
	if len(result.Vendors[0].ModifiedPaths) != 1 || result.Vendors[0].ModifiedPaths[0] != "b.go" {
		t.Errorf("expected modified path b.go, got %v", result.Vendors[0].ModifiedPaths)
	}
}

func TestStatusService_UpstreamStale_FAIL(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Verified: 1, Result: "PASS"},
				Files:   []types.FileStatus{{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"}},
			},
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				Dependencies: []types.UpdateCheckResult{
					{VendorName: "mylib", Ref: "main", CurrentHash: "abc", LatestHash: "def", UpToDate: false},
				},
				TotalChecked: 1, Outdated: 1,
			},
		},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL, got %s", result.Summary.Result)
	}
	if result.Summary.Stale != 1 {
		t.Errorf("expected 1 stale, got %d", result.Summary.Stale)
	}
}

func TestStatusService_AddedFile_WARN(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Verified: 1, Added: 1, Result: "WARN"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "extra.go", Vendor: &vendor1, Status: "added", Type: "file"},
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
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("expected WARN, got %s", result.Summary.Result)
	}
	if result.Summary.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Summary.Added)
	}
}

func TestStatusService_OfflineSkipsRemote(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Verified: 1, Result: "PASS"},
				Files:   []types.FileStatus{{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"}},
			},
		},
		&statusStubOutdated{
			// This should NOT be called when --offline
			err: errForTest,
		},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error (should not call outdated): %v", err)
	}

	if result.Vendors[0].UpstreamStale != nil {
		t.Error("expected UpstreamStale to be nil in offline mode")
	}
}

func TestStatusService_RemoteOnlySkipsDisk(t *testing.T) {
	svc := NewStatusService(
		&statusStubVerify{
			// This should NOT be called when --remote-only
			err: errForTest,
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				Dependencies: []types.UpdateCheckResult{
					{VendorName: "mylib", Ref: "main", CurrentHash: "abc", LatestHash: "abc", UpToDate: true},
				},
				TotalChecked: 1, UpToDate: 1,
			},
		},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{RemoteOnly: true})
	if err != nil {
		t.Fatalf("Status returned error (should not call verify): %v", err)
	}

	// No disk checks ran, so verified should be 0
	if result.Summary.Verified != 0 {
		t.Errorf("expected 0 verified in remote-only mode, got %d", result.Summary.Verified)
	}
}

func TestStatusService_DriftDetails_ModifiedAndAccepted(t *testing.T) {
	vendor1 := "mylib"
	lockHash := "sha256:aaa111"
	modifiedHash := "sha256:bbb222"
	acceptedHash := "sha256:ccc333"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 3, Verified: 1, Modified: 1, Accepted: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor1, Status: "modified", Type: "file",
						ExpectedHash: &lockHash, ActualHash: &modifiedHash},
					{Path: "c.go", Vendor: &vendor1, Status: "accepted", Type: "file",
						ExpectedHash: &lockHash, ActualHash: &acceptedHash},
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
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	v := result.Vendors[0]
	if len(v.DriftDetails) != 2 {
		t.Fatalf("expected 2 drift details, got %d", len(v.DriftDetails))
	}

	// First drift detail: modified file (not accepted)
	d0 := v.DriftDetails[0]
	if d0.Path != "b.go" {
		t.Errorf("drift_details[0].path = %q, want %q", d0.Path, "b.go")
	}
	if d0.LockHash != lockHash {
		t.Errorf("drift_details[0].lock_hash = %q, want %q", d0.LockHash, lockHash)
	}
	if d0.DiskHash != modifiedHash {
		t.Errorf("drift_details[0].disk_hash = %q, want %q", d0.DiskHash, modifiedHash)
	}
	if d0.Accepted {
		t.Error("drift_details[0].accepted should be false")
	}

	// Second drift detail: accepted file
	d1 := v.DriftDetails[1]
	if d1.Path != "c.go" {
		t.Errorf("drift_details[1].path = %q, want %q", d1.Path, "c.go")
	}
	if d1.LockHash != lockHash {
		t.Errorf("drift_details[1].lock_hash = %q, want %q", d1.LockHash, lockHash)
	}
	if d1.DiskHash != acceptedHash {
		t.Errorf("drift_details[1].disk_hash = %q, want %q", d1.DiskHash, acceptedHash)
	}
	if !d1.Accepted {
		t.Error("drift_details[1].accepted should be true")
	}
}

func TestStatusService_DriftDetails_AllClean(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Verified: 1, Result: "PASS"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	// No modified or accepted files => no drift details
	if len(result.Vendors[0].DriftDetails) != 0 {
		t.Errorf("expected 0 drift details for clean vendor, got %d", len(result.Vendors[0].DriftDetails))
	}
}

func TestStatusService_DriftDetails_JSONOutput(t *testing.T) {
	// Verify DriftDetail serializes to expected JSON field names for hook parsing.
	d := types.DriftDetail{
		Path:     ".claude/skills/nominate/SKILL.md",
		LockHash: "sha256:bd528ed",
		DiskHash: "sha256:9a3f17c",
		Accepted: false,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	s := string(data)
	for _, key := range []string{`"path"`, `"lock_hash"`, `"disk_hash"`, `"accepted"`} {
		if !contains(s, key) {
			t.Errorf("JSON output missing key %s: %s", key, s)
		}
	}
}

// TestStatusService_AcceptedOnly_WARN verifies that accepted-only drift produces
// a WARN result (exit code 2 semantics), not PASS. This is C5 from review findings.
func TestStatusService_AcceptedOnly_WARN(t *testing.T) {
	vendor1 := "mylib"
	lockHash := "sha256:aaa111"
	acceptedHash := "sha256:bbb222"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Verified: 1, Accepted: 1, Result: "WARN"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor1, Status: "accepted", Type: "file",
						ExpectedHash: &lockHash, ActualHash: &acceptedHash},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("expected WARN for accepted-only drift, got %s", result.Summary.Result)
	}
	if result.Summary.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", result.Summary.Accepted)
	}
	if result.Summary.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", result.Summary.Modified)
	}
}

// TestStatusService_CoherenceIssues_Propagated verifies that coherence
// counts (stale configs, orphaned lock entries) from verify are propagated
// to StatusSummary (I2/VFY-001).
func TestStatusService_CoherenceIssues_Propagated(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{
					TotalFiles: 2, Verified: 1, Stale: 1, Orphaned: 2, Result: "WARN",
				},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor1, Status: "stale", Type: "coherence"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.StaleConfigs != 1 {
		t.Errorf("expected StaleConfigs=1, got %d", result.Summary.StaleConfigs)
	}
	if result.Summary.OrphanedLock != 2 {
		t.Errorf("expected OrphanedLock=2, got %d", result.Summary.OrphanedLock)
	}
}

// TestStatusService_LockLoadFailure verifies Status() returns an error when
// lockStore.Load() fails (T3: lock load failure path).
func TestStatusService_LockLoadFailure(t *testing.T) {
	svc := NewStatusService(
		&statusStubVerify{result: &types.VerifyResult{}},
		&statusStubOutdated{result: &types.OutdatedResult{}},
		nil,
		&statusStubLockStore{err: errForTest},
	)

	_, err := svc.Status(context.Background(), StatusOptions{})
	if err == nil {
		t.Fatal("expected error from Status() when lockStore.Load() fails, got nil")
	}
}

// TestStatusService_VerifyErrorPropagation verifies Status() returns an error
// when the verify service returns an error (T3: verify error propagation path).
func TestStatusService_VerifyErrorPropagation(t *testing.T) {
	svc := NewStatusService(
		&statusStubVerify{err: errForTest},
		&statusStubOutdated{result: &types.OutdatedResult{}},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "lib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	_, err := svc.Status(context.Background(), StatusOptions{})
	if err == nil {
		t.Fatal("expected error from Status() when verify returns error, got nil")
	}
}

// TestStatusService_UpstreamSkipped verifies that when outdated returns a result
// with Skipped > 0, vendors absent from the dependency list are marked with
// UpstreamSkipped=true and counted in Summary.UpstreamErrors (T3).
func TestStatusService_UpstreamSkipped(t *testing.T) {
	vendor1 := "mylib"
	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Verified: 1, Result: "PASS"},
				Files:   []types.FileStatus{{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"}},
			},
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				// mylib is NOT in the Dependencies list — it was skipped
				Dependencies: []types.UpdateCheckResult{},
				TotalChecked: 0,
				Skipped:      1,
			},
		},
		nil,
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if len(result.Vendors) != 1 {
		t.Fatalf("expected 1 vendor, got %d", len(result.Vendors))
	}
	v := result.Vendors[0]
	if !v.UpstreamSkipped {
		t.Error("expected UpstreamSkipped=true for vendor absent from outdated dependencies")
	}
	if v.UpstreamStale != nil {
		t.Errorf("expected UpstreamStale=nil for skipped vendor, got %v", *v.UpstreamStale)
	}
	if result.Summary.UpstreamErrors != 1 {
		t.Errorf("expected UpstreamErrors=1, got %d", result.Summary.UpstreamErrors)
	}
}

// --- Enforcement integration tests (Spec 075) ---
// statusStubConfigStore is defined in policy_service_test.go (same package).

// TestStatusService_Enforcement_AnnotatesVendors verifies that when a ComplianceConfig
// exists in the config, StatusService annotates each vendor with its resolved enforcement
// level and populates StatusResult.ComplianceConfig.
func TestStatusService_Enforcement_AnnotatesVendors(t *testing.T) {
	vendor1 := "strict-lib"
	vendor2 := "info-lib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Verified: 2, Result: "PASS"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor2, Status: "verified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementLenient},
				Vendors: []types.VendorSpec{
					{Name: "strict-lib", URL: "https://example.com/a", Enforcement: EnforcementStrict,
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
					{Name: "info-lib", URL: "https://example.com/b", Enforcement: EnforcementInfo,
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "b.go", To: "b.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{
				{Name: "strict-lib", Ref: "main", CommitHash: "aaa"},
				{Name: "info-lib", Ref: "main", CommitHash: "bbb"},
			}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	// Check enforcement annotations
	if len(result.Vendors) != 2 {
		t.Fatalf("expected 2 vendors, got %d", len(result.Vendors))
	}
	if result.Vendors[0].Enforcement != EnforcementStrict {
		t.Errorf("vendor[0] enforcement: expected %q, got %q", EnforcementStrict, result.Vendors[0].Enforcement)
	}
	if result.Vendors[1].Enforcement != EnforcementInfo {
		t.Errorf("vendor[1] enforcement: expected %q, got %q", EnforcementInfo, result.Vendors[1].Enforcement)
	}

	// Check ComplianceConfig exposed on result
	if result.ComplianceConfig == nil {
		t.Fatal("expected ComplianceConfig on result, got nil")
	}
	if result.ComplianceConfig.Default != EnforcementLenient {
		t.Errorf("ComplianceConfig.Default: expected %q, got %q", EnforcementLenient, result.ComplianceConfig.Default)
	}
}

// TestStatusService_Enforcement_StrictDrift_ExitCode1 verifies that when a strict
// vendor has modified files, the enforcement logic overrides the summary to FAIL (exit 1).
func TestStatusService_Enforcement_StrictDrift_ExitCode1(t *testing.T) {
	vendor1 := "sec-lib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementStrict},
				Vendors: []types.VendorSpec{
					{Name: "sec-lib", URL: "https://example.com/a",
						Specs: []types.BranchSpec{{Ref: "v1", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "sec-lib", Ref: "v1", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL for strict drift, got %s", result.Summary.Result)
	}
}

// TestStatusService_Enforcement_InfoDrift_ExitCode0 verifies that when an info-level
// vendor has modified files, the enforcement logic overrides the summary to PASS (exit 0).
func TestStatusService_Enforcement_InfoDrift_ExitCode0(t *testing.T) {
	vendor1 := "exp-lib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementInfo},
				Vendors: []types.VendorSpec{
					{Name: "exp-lib", URL: "https://example.com/a",
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "exp-lib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS for info-level drift, got %s", result.Summary.Result)
	}
}

// TestStatusService_Enforcement_LenientDrift_ExitCode2 verifies that when a lenient
// vendor has modified files (no strict drift), the summary is WARN (exit 2).
func TestStatusService_Enforcement_LenientDrift_ExitCode2(t *testing.T) {
	vendor1 := "doc-lib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementLenient},
				Vendors: []types.VendorSpec{
					{Name: "doc-lib", URL: "https://example.com/a",
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "doc-lib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("expected WARN for lenient drift, got %s", result.Summary.Result)
	}
}

// TestStatusService_Enforcement_StrictOnly_Filters verifies that StrictOnly option
// filters out non-strict vendors from the result.
func TestStatusService_Enforcement_StrictOnly_Filters(t *testing.T) {
	vendor1 := "sec-lib"
	vendor2 := "doc-lib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Verified: 2, Result: "PASS"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "verified", Type: "file"},
					{Path: "b.go", Vendor: &vendor2, Status: "verified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementLenient},
				Vendors: []types.VendorSpec{
					{Name: "sec-lib", URL: "https://example.com/a", Enforcement: EnforcementStrict,
						Specs: []types.BranchSpec{{Ref: "v1", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
					{Name: "doc-lib", URL: "https://example.com/b",
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "b.go", To: "b.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{
				{Name: "sec-lib", Ref: "v1", CommitHash: "aaa"},
				{Name: "doc-lib", Ref: "main", CommitHash: "bbb"},
			}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true, StrictOnly: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	if len(result.Vendors) != 1 {
		t.Fatalf("expected 1 vendor with --strict-only, got %d", len(result.Vendors))
	}
	if result.Vendors[0].Name != "sec-lib" {
		t.Errorf("expected strict vendor sec-lib, got %s", result.Vendors[0].Name)
	}
}

// TestStatusService_Enforcement_ComplianceOverride verifies that ComplianceOverride
// creates a synthetic override config that forces all vendors to the given level.
func TestStatusService_Enforcement_ComplianceOverride(t *testing.T) {
	vendor1 := "lib-a"
	vendor2 := "lib-b"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 2, Modified: 2, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
					{Path: "b.go", Vendor: &vendor2, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementStrict},
				Vendors: []types.VendorSpec{
					{Name: "lib-a", URL: "https://example.com/a", Enforcement: EnforcementStrict,
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
					{Name: "lib-b", URL: "https://example.com/b", Enforcement: EnforcementLenient,
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "b.go", To: "b.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{
				{Name: "lib-a", Ref: "main", CommitHash: "aaa"},
				{Name: "lib-b", Ref: "main", CommitHash: "bbb"},
			}},
		},
	)

	// Override all to info — drift should be reported but result PASS
	result, err := svc.Status(context.Background(), StatusOptions{
		Offline:            true,
		ComplianceOverride: EnforcementInfo,
	})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	// Both vendors should be info
	for _, v := range result.Vendors {
		if v.Enforcement != EnforcementInfo {
			t.Errorf("vendor %s: expected enforcement %q with override, got %q", v.Name, EnforcementInfo, v.Enforcement)
		}
	}

	// ComplianceConfig should reflect the override
	if result.ComplianceConfig == nil {
		t.Fatal("expected ComplianceConfig on result with override")
	}
	if result.ComplianceConfig.Default != EnforcementInfo {
		t.Errorf("ComplianceConfig.Default: expected %q, got %q", EnforcementInfo, result.ComplianceConfig.Default)
	}
	if result.ComplianceConfig.Mode != ComplianceModeOverride {
		t.Errorf("ComplianceConfig.Mode: expected %q, got %q", ComplianceModeOverride, result.ComplianceConfig.Mode)
	}

	// Result should be PASS since all drift is info-level
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS with info override, got %s", result.Summary.Result)
	}
}

// TestStatusService_Enforcement_UpstreamStale_PreservesFailWithInfoDrift verifies
// that enforcement does NOT downgrade a FAIL from upstream staleness to PASS even
// when all enforcement drift is info-level. Upstream stale = FAIL regardless.
func TestStatusService_Enforcement_UpstreamStale_PreservesFailWithInfoDrift(t *testing.T) {
	vendor1 := "lib-a"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{
			result: &types.OutdatedResult{
				Dependencies: []types.UpdateCheckResult{
					{VendorName: "lib-a", Ref: "main", CurrentHash: "aaa", LatestHash: "bbb", UpToDate: false},
				},
				TotalChecked: 1, Outdated: 1,
			},
		},
		&statusStubConfigStore{
			config: types.VendorConfig{
				Compliance: &types.ComplianceConfig{Default: EnforcementInfo},
				Vendors: []types.VendorSpec{
					{Name: "lib-a", URL: "https://example.com/a",
						Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "a.go"}}}}},
				},
			},
		},
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "lib-a", Ref: "main", CommitHash: "aaa"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	// Even though enforcement is info (exit 0 for drift), upstream staleness keeps FAIL
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL (upstream stale preserves failure), got %s", result.Summary.Result)
	}
}

// TestStatusService_Enforcement_NoComplianceConfig_LegacyBehavior verifies that
// when no ComplianceConfig is present (nil configStore or no compliance block),
// enforcement logic is skipped and legacy behavior is unchanged.
func TestStatusService_Enforcement_NoComplianceConfig_LegacyBehavior(t *testing.T) {
	vendor1 := "mylib"

	svc := NewStatusService(
		&statusStubVerify{
			result: &types.VerifyResult{
				Summary: types.VerifySummary{TotalFiles: 1, Modified: 1, Result: "FAIL"},
				Files: []types.FileStatus{
					{Path: "a.go", Vendor: &vendor1, Status: "modified", Type: "file"},
				},
			},
		},
		&statusStubOutdated{err: errForTest},
		nil, // nil configStore — enforcement skipped
		&statusStubLockStore{
			lock: types.VendorLock{Vendors: []types.LockDetails{{Name: "mylib", Ref: "main", CommitHash: "abc"}}},
		},
	)

	result, err := svc.Status(context.Background(), StatusOptions{Offline: true})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}

	// Legacy behavior: modified = FAIL, no enforcement annotation
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL (legacy), got %s", result.Summary.Result)
	}
	if result.Vendors[0].Enforcement != "" {
		t.Errorf("expected empty enforcement with nil configStore, got %q", result.Vendors[0].Enforcement)
	}
	if result.ComplianceConfig != nil {
		t.Error("expected nil ComplianceConfig with nil configStore")
	}
}

// errForTest is a sentinel error for asserting a stub was not called.
var errForTest = &testSentinelError{msg: "should not be called"}

type testSentinelError struct{ msg string }

func (e *testSentinelError) Error() string { return e.msg }
