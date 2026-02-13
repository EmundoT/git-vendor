package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// helper: builds a VendorConfig with one external vendor and one spec.
func outdatedConfig(name, url, ref string) types.VendorConfig {
	return types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: name,
			URL:  url,
			Specs: []types.BranchSpec{{
				Ref: ref,
			}},
		}},
	}
}

// helper: builds a VendorLock with one entry.
func outdatedLock(name, ref, hash string) types.VendorLock {
	return types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       name,
			Ref:        ref,
			CommitHash: hash,
			Updated:    "2025-01-01",
		}},
	}
}

// TestOutdated_AllUpToDate verifies zero Outdated count when locked hash matches upstream.
func TestOutdated_AllUpToDate(t *testing.T) {
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	hash := "abc123def456789012345678901234567890abcd"
	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(outdatedLock("mylib", "main", hash), nil)
	git.EXPECT().LsRemote(gomock.Any(), "https://github.com/org/mylib", "main").Return(hash, nil)

	svc := NewOutdatedService(config, lock, git)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.Outdated != 0 {
		t.Errorf("expected 0 outdated, got %d", result.Outdated)
	}
	if result.UpToDate != 1 {
		t.Errorf("expected 1 up-to-date, got %d", result.UpToDate)
	}
	if result.TotalChecked != 1 {
		t.Errorf("expected 1 total checked, got %d", result.TotalChecked)
	}
}

// TestOutdated_SomeOutdated verifies correct counts when upstream has a newer commit.
func TestOutdated_SomeOutdated(t *testing.T) {
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	lockedHash := "aaa1111111111111111111111111111111111111a"
	latestHash := "bbb2222222222222222222222222222222222222b"
	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(outdatedLock("mylib", "main", lockedHash), nil)
	git.EXPECT().LsRemote(gomock.Any(), "https://github.com/org/mylib", "main").Return(latestHash, nil)

	svc := NewOutdatedService(config, lock, git)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.Outdated != 1 {
		t.Errorf("expected 1 outdated, got %d", result.Outdated)
	}
	if result.UpToDate != 0 {
		t.Errorf("expected 0 up-to-date, got %d", result.UpToDate)
	}
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.CurrentHash != lockedHash {
		t.Errorf("expected current hash %s, got %s", lockedHash, dep.CurrentHash)
	}
	if dep.LatestHash != latestHash {
		t.Errorf("expected latest hash %s, got %s", latestHash, dep.LatestHash)
	}
	if dep.UpToDate {
		t.Error("expected UpToDate=false")
	}
}

// TestOutdated_VendorFilter verifies that only the matching vendor is checked.
func TestOutdated_VendorFilter(t *testing.T) {
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	hash := "abc123def456789012345678901234567890abcd"
	cfg := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/org/lib-a", Specs: []types.BranchSpec{{Ref: "main"}}},
			{Name: "lib-b", URL: "https://github.com/org/lib-b", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}
	lck := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", CommitHash: hash},
			{Name: "lib-b", Ref: "main", CommitHash: hash},
		},
	}
	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(lck, nil)

	// Only lib-a should be queried
	git.EXPECT().LsRemote(gomock.Any(), "https://github.com/org/lib-a", "main").Return(hash, nil)

	svc := NewOutdatedService(config, lock, git)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{Vendor: "lib-a"})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.TotalChecked != 1 {
		t.Errorf("expected 1 total checked (filtered), got %d", result.TotalChecked)
	}
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Dependencies))
	}
	if result.Dependencies[0].VendorName != "lib-a" {
		t.Errorf("expected lib-a, got %s", result.Dependencies[0].VendorName)
	}
}

// TestOutdated_InternalVendorSkipped verifies internal vendors are excluded.
func TestOutdated_InternalVendorSkipped(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	cfg := types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name:   "internal-lib",
			Source: "internal",
			Specs:  []types.BranchSpec{{Ref: RefLocal}},
		}},
	}
	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	// No LsRemote call expected

	svc := NewOutdatedService(config, lock, nil) // nil gitClient — should never be called
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.TotalChecked != 0 {
		t.Errorf("expected 0 total checked for internal vendor, got %d", result.TotalChecked)
	}
}

// TestOutdated_ConfigLoadError verifies config load errors are propagated.
func TestOutdated_ConfigLoadError(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config broken"))

	svc := NewOutdatedService(config, nil, nil)
	_, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestOutdated_LockLoadError verifies lockfile load errors are propagated.
func TestOutdated_LockLoadError(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, fmt.Errorf("lock broken"))

	svc := NewOutdatedService(config, lock, nil)
	_, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestOutdated_LsRemoteError verifies that LsRemote errors are non-fatal (vendor is skipped).
func TestOutdated_LsRemoteError(t *testing.T) {
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(outdatedLock("mylib", "main", "abc123def456789012345678901234567890abcd"), nil)
	git.EXPECT().LsRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return("", fmt.Errorf("network timeout"))

	svc := NewOutdatedService(config, lock, git)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error (should be non-fatal): %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected 0 total checked when LsRemote fails, got %d", result.TotalChecked)
	}
}

// TestOutdated_UnsyncedVendor verifies vendors without a lock entry are skipped.
func TestOutdated_UnsyncedVendor(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil) // empty lock

	svc := NewOutdatedService(config, lock, nil)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped for unsynced vendor, got %d", result.Skipped)
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected 0 total checked, got %d", result.TotalChecked)
	}
}

// TestOutdated_MultipleSpecsPerVendor verifies each spec is checked independently.
func TestOutdated_MultipleSpecsPerVendor(t *testing.T) {
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	hashMain := "aaa1111111111111111111111111111111111111a"
	hashDev := "bbb2222222222222222222222222222222222222b"
	newDev := "ccc3333333333333333333333333333333333333c"

	cfg := types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "mylib",
			URL:  "https://github.com/org/mylib",
			Specs: []types.BranchSpec{
				{Ref: "main"},
				{Ref: "develop"},
			},
		}},
	}
	lck := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "mylib", Ref: "main", CommitHash: hashMain},
			{Name: "mylib", Ref: "develop", CommitHash: hashDev},
		},
	}
	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(lck, nil)
	git.EXPECT().LsRemote(gomock.Any(), "https://github.com/org/mylib", "main").Return(hashMain, nil)
	git.EXPECT().LsRemote(gomock.Any(), "https://github.com/org/mylib", "develop").Return(newDev, nil)

	svc := NewOutdatedService(config, lock, git)
	result, err := svc.Outdated(context.Background(), OutdatedOptions{})
	if err != nil {
		t.Fatalf("Outdated returned error: %v", err)
	}

	if result.TotalChecked != 2 {
		t.Errorf("expected 2 total checked, got %d", result.TotalChecked)
	}
	if result.UpToDate != 1 {
		t.Errorf("expected 1 up-to-date, got %d", result.UpToDate)
	}
	if result.Outdated != 1 {
		t.Errorf("expected 1 outdated, got %d", result.Outdated)
	}
}

// TestOutdated_ContextCancellation verifies that a cancelled context returns ctx.Err().
func TestOutdated_ContextCancellation(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(outdatedConfig("mylib", "https://github.com/org/mylib", "main"), nil)
	lock.EXPECT().Load().Return(outdatedLock("mylib", "main", "abc123def456789012345678901234567890abcd"), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	svc := NewOutdatedService(config, lock, nil)
	_, err := svc.Outdated(ctx, OutdatedOptions{})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
