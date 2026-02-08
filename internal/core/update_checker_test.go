package core

import (
	"fmt"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Mock UICallback
// ============================================================================

type mockUICallback struct {
	SilentUICallback
	warningCalls int
}

func (m *mockUICallback) ShowWarning(_, _ string) {
	m.warningCalls++
}

// ============================================================================
// CheckUpdates Tests
// ============================================================================

func TestUpdateChecker_CheckUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	// Setup test data
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor1",
				URL:  "https://github.com/test/repo1",
				Specs: []types.BranchSpec{
					{Ref: "main"},
				},
			},
			{
				Name: "vendor2",
				URL:  "https://github.com/test/repo2",
				Specs: []types.BranchSpec{
					{Ref: "v1.0"},
				},
			},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "vendor1",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
			},
			{
				Name:       "vendor2",
				Ref:        "v1.0",
				CommitHash: "def456",
				Updated:    "2024-01-02T00:00:00Z",
			},
		},
	}

	// Mock expectations
	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Mock git operations for vendor1 (up to date)
	mockFS.EXPECT().CreateTemp("", "update-check-*").Return("/tmp/check1", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/check1").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/check1", "origin", "https://github.com/test/repo1").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/check1", 1, "main").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/check1").Return("abc123", nil) // Same hash = up to date
	mockFS.EXPECT().RemoveAll("/tmp/check1").Return(nil)

	// Mock git operations for vendor2 (outdated)
	mockFS.EXPECT().CreateTemp("", "update-check-*").Return("/tmp/check2", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/check2").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/check2", "origin", "https://github.com/test/repo2").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/check2", 1, "v1.0").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/check2").Return("xyz789", nil) // Different hash = outdated
	mockFS.EXPECT().RemoveAll("/tmp/check2").Return(nil)

	// Test: CheckUpdates
	results, err := checker.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates() error = %v", err)
	}

	// Verify results
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check vendor1 (up to date)
	if results[0].VendorName != "vendor1" {
		t.Errorf("Result[0] vendor name = %s, want vendor1", results[0].VendorName)
	}
	if results[0].Ref != "main" {
		t.Errorf("Result[0] ref = %s, want main", results[0].Ref)
	}
	if results[0].CurrentHash != "abc123" {
		t.Errorf("Result[0] current hash = %s, want abc123", results[0].CurrentHash)
	}
	if results[0].LatestHash != "abc123" {
		t.Errorf("Result[0] latest hash = %s, want abc123", results[0].LatestHash)
	}
	if !results[0].UpToDate {
		t.Errorf("Result[0] up to date = false, want true")
	}

	// Check vendor2 (outdated)
	if results[1].VendorName != "vendor2" {
		t.Errorf("Result[1] vendor name = %s, want vendor2", results[1].VendorName)
	}
	if results[1].Ref != "v1.0" {
		t.Errorf("Result[1] ref = %s, want v1.0", results[1].Ref)
	}
	if results[1].CurrentHash != "def456" {
		t.Errorf("Result[1] current hash = %s, want def456", results[1].CurrentHash)
	}
	if results[1].LatestHash != "xyz789" {
		t.Errorf("Result[1] latest hash = %s, want xyz789", results[1].LatestHash)
	}
	if results[1].UpToDate {
		t.Errorf("Result[1] up to date = true, want false")
	}
}

func TestUpdateChecker_CheckUpdates_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	// Mock config load error
	mockConfig.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config not found"))

	// Test: CheckUpdates should fail
	_, err := checker.CheckUpdates()
	if err == nil {
		t.Error("Expected error when config load fails, got nil")
	}
	if !contains(err.Error(), "failed to load config") {
		t.Errorf("Error should contain 'failed to load config', got: %v", err)
	}
}

func TestUpdateChecker_CheckUpdates_LockLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor1", URL: "https://github.com/test/repo1", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}

	// Mock config load success, lock load error
	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(types.VendorLock{}, fmt.Errorf("lock not found"))

	// Test: CheckUpdates should fail
	_, err := checker.CheckUpdates()
	if err == nil {
		t.Error("Expected error when lock load fails, got nil")
	}
	if !contains(err.Error(), "failed to load lockfile") {
		t.Errorf("Error should contain 'failed to load lockfile', got: %v", err)
	}
}

func TestUpdateChecker_CheckUpdates_SkipUnsynced(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	// Config has vendor1, but lockfile is empty (vendor not synced yet)
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor1", URL: "https://github.com/test/repo1", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{}, // Empty - vendor1 not synced
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Test: CheckUpdates should skip unsynced vendor
	results, err := checker.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates() error = %v", err)
	}

	// Should have 0 results (vendor1 skipped)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for unsynced vendor, got %d", len(results))
	}
}

func TestUpdateChecker_CheckUpdates_FetchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor1", URL: "https://github.com/test/repo1", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor1", Ref: "main", CommitHash: "abc123"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Mock git fetch failure
	mockFS.EXPECT().CreateTemp("", "update-check-*").Return("/tmp/check1", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/check1").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/check1", "origin", "https://github.com/test/repo1").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/check1", 1, "main").Return(fmt.Errorf("network error"))
	mockFS.EXPECT().RemoveAll("/tmp/check1").Return(nil)

	// Test: CheckUpdates should skip vendor with fetch error and return empty results
	results, err := checker.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates() should not error on fetch failure, got: %v", err)
	}

	// Should have 0 results (vendor1 skipped due to fetch error)
	if len(results) != 0 {
		t.Errorf("Expected 0 results when fetch fails, got %d", len(results))
	}

	// Verify warning was shown
	if mockUI.warningCalls != 1 {
		t.Errorf("Expected 1 warning call, got %d", mockUI.warningCalls)
	}
}

func TestUpdateChecker_CheckUpdates_MultipleSpecs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	mockUI := &mockUICallback{}

	checker := NewUpdateChecker(mockConfig, mockLock, mockGit, mockFS, mockUI)

	// Vendor with multiple refs
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor1",
				URL:  "https://github.com/test/repo1",
				Specs: []types.BranchSpec{
					{Ref: "main"},
					{Ref: "develop"},
				},
			},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor1", Ref: "main", CommitHash: "abc123"},
			{Name: "vendor1", Ref: "develop", CommitHash: "def456"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Mock git operations for main
	mockFS.EXPECT().CreateTemp("", "update-check-*").Return("/tmp/check1", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/check1").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/check1", "origin", "https://github.com/test/repo1").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/check1", 1, "main").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/check1").Return("abc123", nil)
	mockFS.EXPECT().RemoveAll("/tmp/check1").Return(nil)

	// Mock git operations for develop
	mockFS.EXPECT().CreateTemp("", "update-check-*").Return("/tmp/check2", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/check2").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/check2", "origin", "https://github.com/test/repo1").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/check2", 1, "develop").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/check2").Return("xyz789", nil)
	mockFS.EXPECT().RemoveAll("/tmp/check2").Return(nil)

	// Test: CheckUpdates should check both specs
	results, err := checker.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates() error = %v", err)
	}

	// Should have 2 results (one per spec)
	if len(results) != 2 {
		t.Fatalf("Expected 2 results for 2 specs, got %d", len(results))
	}

	// Verify both results are for vendor1
	if results[0].VendorName != "vendor1" || results[1].VendorName != "vendor1" {
		t.Error("Both results should be for vendor1")
	}

	// Verify different refs
	if results[0].Ref != "main" {
		t.Errorf("Result[0] ref = %s, want main", results[0].Ref)
	}
	if results[1].Ref != "develop" {
		t.Errorf("Result[1] ref = %s, want develop", results[1].Ref)
	}
}
