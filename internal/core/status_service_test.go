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

// errForTest is a sentinel error for asserting a stub was not called.
var errForTest = &testSentinelError{msg: "should not be called"}

type testSentinelError struct{ msg string }

func (e *testSentinelError) Error() string { return e.msg }
