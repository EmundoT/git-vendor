package core

import (
	"errors"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"

	"github.com/golang/mock/gomock"
)

func TestDiffVendor_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	// Setup config
	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "test-vendor",
				URL:     "https://github.com/owner/repo",
				License: "MIT",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src", To: "lib/src"},
						},
					},
				},
			},
		},
	}, nil)

	// Setup lock
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01 10:00:00 +0000",
			},
		},
	}, nil)

	// Setup git operations for diff
	mockFS.EXPECT().CreateTemp("", "diff-check-*").Return("/tmp/test", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/test").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/test", "origin", "https://github.com/owner/repo").Return(nil)
	mockGit.EXPECT().FetchAll(gomock.Any(), "/tmp/test").Return(nil)
	mockGit.EXPECT().Checkout(gomock.Any(), "/tmp/test", "FETCH_HEAD").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test").Return("def456", nil)
	mockGit.EXPECT().GetCommitLog(gomock.Any(), "/tmp/test", "abc123", "def456", 10).Return([]types.CommitInfo{
		{
			ShortHash: "def456",
			Subject:   "Update dependencies",
			Date:      "2024-01-02 15:30:00 +0000",
		},
	}, nil)
	mockFS.EXPECT().RemoveAll("/tmp/test").Return(nil)

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, nil, "", &SilentUICallback{}, nil)

	diffs, err := syncer.DiffVendor("test-vendor")
	if err != nil {
		t.Fatalf("DiffVendor() error = %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	diff := diffs[0]
	if diff.VendorName != "test-vendor" {
		t.Errorf("Expected vendor name 'test-vendor', got '%s'", diff.VendorName)
	}
	if diff.Ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", diff.Ref)
	}
	if diff.OldHash != "abc123" {
		t.Errorf("Expected old hash 'abc123', got '%s'", diff.OldHash)
	}
	if diff.NewHash != "def456" {
		t.Errorf("Expected new hash 'def456', got '%s'", diff.NewHash)
	}
	if diff.CommitCount != 1 {
		t.Errorf("Expected 1 commit, got %d", diff.CommitCount)
	}
}

func TestDiffVendor_VendorNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{}, nil)

	syncer := NewVendorSyncer(mockConfig, mockLock, nil, nil, nil, "", &SilentUICallback{}, nil)

	_, err := syncer.DiffVendor("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent vendor")
	}

	expectedMsg := "vendor 'nonexistent' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestDiffVendor_NotLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				Specs: []types.BranchSpec{
					{Ref: "main"},
				},
			},
		},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{}, // Empty - not locked
	}, nil)

	syncer := NewVendorSyncer(mockConfig, mockLock, nil, nil, nil, "", &SilentUICallback{}, nil)

	diffs, err := syncer.DiffVendor("test-vendor")
	if err != nil {
		t.Fatalf("DiffVendor() error = %v", err)
	}

	// Should return empty diffs for unlocked vendor
	if len(diffs) != 0 {
		t.Errorf("Expected 0 diffs for unlocked vendor, got %d", len(diffs))
	}
}

func TestFormatDiffOutput_UpToDate(t *testing.T) {
	diff := &types.VendorDiff{
		VendorName: "test-vendor",
		Ref:        "main",
		OldHash:    "abc123def",
		NewHash:    "abc123def", // Same = up to date
	}

	output := FormatDiffOutput(diff)

	if !contains(output, "test-vendor") {
		t.Error("Output should contain vendor name")
	}
	if !contains(output, "main") {
		t.Error("Output should contain ref")
	}
	if !contains(output, "Up to date") {
		t.Error("Output should indicate up to date status")
	}
	if !contains(output, "abc123d") {
		t.Error("Output should contain short hash")
	}
}

func TestFormatDiffOutput_WithCommits(t *testing.T) {
	diff := &types.VendorDiff{
		VendorName:  "test-vendor",
		Ref:         "main",
		OldHash:     "abc123def",
		NewHash:     "def456ghi",
		OldDate:     "2024-01-01 10:00:00 +0000",
		NewDate:     "2024-01-02 15:30:00 +0000",
		CommitCount: 2,
		Commits: []types.CommitInfo{
			{
				ShortHash: "def456",
				Subject:   "Update dependencies",
				Date:      "2024-01-02 15:30:00 +0000",
			},
			{
				ShortHash: "ccc777",
				Subject:   "Fix bug",
				Date:      "2024-01-01 12:00:00 +0000",
			},
		},
	}

	output := FormatDiffOutput(diff)

	if !contains(output, "test-vendor") {
		t.Error("Output should contain vendor name")
	}
	if !contains(output, "Old: abc123d") {
		t.Error("Output should contain old hash")
	}
	if !contains(output, "New: def456g") {
		t.Error("Output should contain new hash")
	}
	if !contains(output, "Commits (+2)") {
		t.Error("Output should show commit count")
	}
	if !contains(output, "Update dependencies") {
		t.Error("Output should contain commit subject")
	}
	if !contains(output, "Fix bug") {
		t.Error("Output should contain second commit subject")
	}
	if !contains(output, "Jan 02") {
		t.Error("Output should contain formatted date")
	}
}

func TestFormatDiffOutput_Diverged(t *testing.T) {
	diff := &types.VendorDiff{
		VendorName:  "test-vendor",
		Ref:         "main",
		OldHash:     "abc123def",
		NewHash:     "def456ghi",
		CommitCount: 0, // No commits = diverged
	}

	output := FormatDiffOutput(diff)

	if !contains(output, "diverged") || !contains(output, "ahead") {
		t.Error("Output should indicate diverged or ahead status")
	}
}

func TestDiffVendor_FetchAllFails_FallsBackToFetch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src", To: "lib"}}},
				},
			},
		},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-vendor", Ref: "main", CommitHash: "abc123", Updated: "2024-01-01 10:00:00 +0000"},
		},
	}, nil)

	mockFS.EXPECT().CreateTemp("", "diff-check-*").Return("/tmp/test", nil)
	mockGit.EXPECT().Init(gomock.Any(), "/tmp/test").Return(nil)
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/test", "origin", "https://github.com/owner/repo").Return(nil)

	// FetchAll fails, triggering fallback to Fetch
	mockGit.EXPECT().FetchAll(gomock.Any(), "/tmp/test").Return(errors.New("network timeout"))
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/test", 0, "main").Return(nil)

	mockGit.EXPECT().Checkout(gomock.Any(), "/tmp/test", "FETCH_HEAD").Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test").Return("def456", nil)
	mockGit.EXPECT().GetCommitLog(gomock.Any(), "/tmp/test", "abc123", "def456", 10).Return([]types.CommitInfo{
		{ShortHash: "def456", Subject: "Fix", Date: "2024-01-02 15:30:00 +0000"},
	}, nil)
	mockFS.EXPECT().RemoveAll("/tmp/test").Return(nil)

	syncer := NewVendorSyncer(mockConfig, mockLock, mockGit, mockFS, nil, "", &SilentUICallback{}, nil)

	diffs, err := syncer.DiffVendor("test-vendor")
	if err != nil {
		t.Fatalf("DiffVendor() error = %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].CommitCount != 1 {
		t.Errorf("Expected 1 commit, got %d", diffs[0].CommitCount)
	}
}

func TestDiffVendor_MultipleSpecs_SkipsUnlocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "multi-ref-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{Ref: "main"},
					{Ref: "v2"},
				},
			},
		},
	}, nil)

	// No lock entries → lockedHash="" for both refs → skip both
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{},
	}, nil)

	syncer := NewVendorSyncer(mockConfig, mockLock, nil, nil, nil, "", &SilentUICallback{}, nil)

	diffs, err := syncer.DiffVendor("multi-ref-vendor")
	if err != nil {
		t.Fatalf("DiffVendor() error = %v", err)
	}

	if len(diffs) != 0 {
		t.Errorf("Expected 0 diffs (all unlocked), got %d", len(diffs))
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid ISO date",
			input:    "2024-01-15 10:30:00 +0000",
			expected: "Jan 15",
		},
		{
			name:     "December date",
			input:    "2024-12-25 23:59:59 -0800",
			expected: "Dec 25",
		},
		{
			name:     "Invalid format",
			input:    "invalid",
			expected: "invalid",
		},
		{
			name:     "Partial date",
			input:    "2024",
			expected: "2024",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDate(tt.input)
			if result != tt.expected {
				t.Errorf("formatDate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
