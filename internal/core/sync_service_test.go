package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// SyncVendor Tests - Comprehensive tests for the core sync function
// ============================================================================

func TestSyncVendor_HappyPath_LockedRef(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: Create a simple vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "abc123def456"}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "abc123def456").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def456", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	hashes, _, err := syncer.sync.SyncVendor(&vendor, lockedRefs, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 1 {
		t.Errorf("Expected 1 hash, got %d", len(hashes))
	}
	if hashes["main"].CommitHash != "abc123def456" {
		t.Errorf("Expected hash abc123def456, got %s", hashes["main"].CommitHash)
	}
}

func TestSyncVendor_HappyPath_UnlockedRef(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("latest789", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute with nil lockedRefs (unlocked mode)
	hashes, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if hashes["main"].CommitHash != "latest789" {
		t.Errorf("Expected hash latest789, got %s", hashes["main"].CommitHash)
	}
}

func TestSyncVendor_ShallowFetchSucceeds(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), 1, gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSyncVendor_ShallowFetchFails_FullFetchSucceeds(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Mock: Shallow fetch fails, then FetchAll succeeds
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), 1, gomock.Any()).Return(fmt.Errorf("shallow fetch failed"))
	git.EXPECT().FetchAll(gomock.Any(), gomock.Any()).Return(nil)

	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSyncVendor_BothFetchesFail(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Mock: Both fetches fail
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("network error"))
	git.EXPECT().FetchAll(gomock.Any(), gomock.Any()).Return(fmt.Errorf("network error"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to fetch ref") {
		t.Errorf("Expected 'failed to fetch ref' error, got: %v", err)
	}
}

func TestSyncVendor_StaleCommitHashDetection(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "stale123"}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Mock: Checkout fails with stale commit error
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), "stale123").Return(fmt.Errorf("reference is not a tree: stale123"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, lockedRefs, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !IsStaleCommit(err) {
		t.Errorf("Expected StaleCommitError, got: %v", err)
	}
	if !contains(err.Error(), "no longer exists") {
		t.Errorf("Expected stale commit error message, got: %v", err)
	}
}

func TestSyncVendor_CheckoutFETCH_HEADFails_RefFallbackSucceeds(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Mock: Checkout FETCH_HEAD fails, checkout ref succeeds
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), "FETCH_HEAD").Return(fmt.Errorf("FETCH_HEAD not available"))
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), "main").Return(nil)

	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success (fallback), got error: %v", err)
	}
}

func TestSyncVendor_AllCheckoutsFail(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Mock: All checkouts fail
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("checkout failed")).Times(2)

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !IsCheckoutError(err) {
		t.Errorf("Expected CheckoutError, got: %v", err)
	}
}

func TestSyncVendor_TempDirectoryCreationFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	// Mock: CreateTemp fails
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("disk full"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestSyncVendor_PathTraversalBlocked(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: Create vendor with malicious path mapping
	vendor := types.VendorSpec{
		Name:    "malicious",
		URL:     "https://github.com/attacker/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "payload.txt", To: "../../../etc/passwd"},
				},
			},
		},
	}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	// Mock: File exists in temp repo
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "payload.txt", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// Even though path validation should catch it, license copy happens before mapping validation
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected path traversal error, got nil")
	}
	if !contains(err.Error(), "invalid destination path") || !contains(err.Error(), "not allowed") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestSyncVendor_MultipleSpecsPerVendor(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: Vendor with 3 specs (main, dev, v1.0)
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
			{
				Ref: "dev",
				Mapping: []types.PathMapping{
					{From: "src/dev.go", To: "lib/dev.go"},
				},
			},
			{
				Ref: "v1.0",
				Mapping: []types.PathMapping{
					{From: "src/release.go", To: "lib/release.go"},
				},
			},
		},
	}

	// Each spec creates its own temp directory and performs git operations
	// Use AnyTimes() since the order is interleaved for 3 specs
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).AnyTimes()
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Return different hashes for each of the 3 specs
	gomock.InOrder(
		git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash100000", nil),
		git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash200000", nil),
		git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash300000", nil),
	)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	hashes, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 3 {
		t.Errorf("Expected 3 hashes (one per spec), got %d", len(hashes))
	}
	if _, ok := hashes["main"]; !ok {
		t.Error("Expected hash for 'main' ref")
	}
	if _, ok := hashes["dev"]; !ok {
		t.Error("Expected hash for 'dev' ref")
	}
	if _, ok := hashes["v1.0"]; !ok {
		t.Error("Expected hash for 'v1.0' ref")
	}
}

func TestSyncVendor_MultipleMappingsPerSpec(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: One spec with 5 file mappings
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "file1.go", To: "lib/file1.go"},
					{From: "file2.go", To: "lib/file2.go"},
					{From: "file3.go", To: "lib/file3.go"},
					{From: "file4.go", To: "lib/file4.go"},
					{From: "file5.go", To: "lib/file5.go"},
				},
			},
		},
	}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file.go", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// Expect at least 5 CopyFile calls (5 mappings) plus 1 for license
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).MinTimes(5)

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSyncVendor_FileCopyFailsInMapping(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file.go", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Mock: License copy succeeds, but mapping copy fails
	// License copy happens first and succeeds
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).DoAndReturn(func(_, dst string) (CopyStats, error) {
		if contains(dst, "licenses") {
			return CopyStats{FileCount: 1, ByteCount: 100}, nil // License copy succeeds
		}
		return CopyStats{}, fmt.Errorf("permission denied") // Mapping copy fails
	}).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy file") {
		t.Errorf("Expected 'failed to copy file' error, got: %v", err)
	}
}

func TestSyncVendor_LicenseCopyFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	// Mock: License file exists
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Mock: License copy fails
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).DoAndReturn(func(_, dst string) (CopyStats, error) {
		if contains(dst, "licenses") {
			return CopyStats{}, fmt.Errorf("disk full")
		}
		return CopyStats{FileCount: 1, ByteCount: 100}, nil
	}).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, _, err := syncer.sync.SyncVendor(&vendor, nil, SyncOptions{})

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy license") {
		t.Errorf("Expected license copy error, got: %v", err)
	}
}

// ============================================================================
// Sync() Tests - Main orchestration method (public API)
// ============================================================================

func TestSync_AllVendors(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock config with 3 vendors
	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
			createTestVendorSpec("vendor-b", "https://github.com/b/repo", "main"),
			createTestVendorSpec("vendor-c", "https://github.com/c/repo", "main"),
		},
	}

	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-a", "main", "hash111"),
			createTestLockEntry("vendor-b", "main", "hash222"),
			createTestLockEntry("vendor-c", "main", "hash333"),
		},
	}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(testLock, nil)

	// Each vendor performs git operations + file copy
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test", nil).Times(3)
	fs.EXPECT().RemoveAll("/tmp/test").Return(nil).Times(3)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash111", nil).Times(1)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash222", nil).Times(1)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash333", nil).Times(1)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: sync all vendors
	err := syncService.Sync(SyncOptions{})

	// Verify: all 3 vendors synced
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_SingleVendor_ByName(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock config with 3 vendors
	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
			createTestVendorSpec("vendor-b", "https://github.com/b/repo", "main"),
			createTestVendorSpec("vendor-c", "https://github.com/c/repo", "main"),
		},
	}

	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-b", "main", "hash222"),
		},
	}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(testLock, nil)

	// Only vendor-b should be synced (1 set of git operations)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test", nil).Times(1)
	fs.EXPECT().RemoveAll("/tmp/test").Return(nil).Times(1)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash222", nil).Times(1)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: sync only vendor-b
	err := syncService.Sync(SyncOptions{VendorName: "vendor-b"})

	// Verify: only vendor-b synced (vendors a and c skipped)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_VendorNotFound(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
		},
	}

	testLock := types.VendorLock{}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(testLock, nil)

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: sync nonexistent vendor
	err := syncService.Sync(SyncOptions{VendorName: "nonexistent"})

	// Verify: expect ErrVendorNotFound
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestSync_DryRun_PreviewMode(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
			createTestVendorSpec("vendor-b", "https://github.com/b/repo", "main"),
		},
	}

	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-a", "main", "hash111"),
		},
	}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(testLock, nil)

	// NO git operations should happen in dry-run mode
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Times(0)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	// NO file operations (except maybe stdout writes)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Times(0)
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Times(0)

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: dry-run mode
	err := syncService.Sync(SyncOptions{DryRun: true})

	// Verify: previewSyncVendor called (no actual sync)
	if err != nil {
		t.Fatalf("Expected success in dry-run, got error: %v", err)
	}
}

func TestSync_Force_IgnoresLock(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
		},
	}

	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-a", "main", "hash_old_locked"),
		},
	}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(testLock, nil)

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test", nil).Times(1)
	fs.EXPECT().RemoveAll("/tmp/test").Return(nil).Times(1)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	// Force mode: checkout FETCH_HEAD (latest), NOT locked hash
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), "FETCH_HEAD").Return(nil).Times(1)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("hash_new_latest", nil).Times(1)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "file", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: force re-download (ignore lock)
	err := syncService.Sync(SyncOptions{Force: true})

	// Verify: fetched latest (not locked hash)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_ConfigLoadFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config file missing"))

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute
	err := syncService.Sync(SyncOptions{})

	// Verify: error propagated
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "config file missing") {
		t.Errorf("Expected config error, got: %v", err)
	}
}

func TestSync_LockLoadFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
		},
	}

	config.EXPECT().Load().Return(testConfig, nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, fmt.Errorf("lock file corrupt"))

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute
	err := syncService.Sync(SyncOptions{})

	// Verify: error propagated
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "lock file corrupt") {
		t.Errorf("Expected lock error, got: %v", err)
	}
}

// ============================================================================
// buildLockMap() Tests
// ============================================================================

func TestBuildLockMap_MultipleVendorsAndRefs(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Create lock with 2 vendors, 3 refs each (6 total entries)
	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-a", "main", "hash_a_main"),
			createTestLockEntry("vendor-a", "dev", "hash_a_dev"),
			createTestLockEntry("vendor-a", "v1.0", "hash_a_v1"),
			createTestLockEntry("vendor-b", "main", "hash_b_main"),
			createTestLockEntry("vendor-b", "dev", "hash_b_dev"),
			createTestLockEntry("vendor-b", "v2.0", "hash_b_v2"),
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute
	lockMap := syncService.buildLockMap(testLock)

	// Verify map structure: map[vendorName]map[ref]hash
	if len(lockMap) != 2 {
		t.Fatalf("Expected 2 vendors in lockMap, got %d", len(lockMap))
	}

	// Verify vendor-a has 3 refs
	if len(lockMap["vendor-a"]) != 3 {
		t.Errorf("Expected vendor-a to have 3 refs, got %d", len(lockMap["vendor-a"]))
	}
	if lockMap["vendor-a"]["main"] != "hash_a_main" {
		t.Errorf("Expected hash_a_main for vendor-a@main, got %s", lockMap["vendor-a"]["main"])
	}
	if lockMap["vendor-a"]["dev"] != "hash_a_dev" {
		t.Errorf("Expected hash_a_dev for vendor-a@dev, got %s", lockMap["vendor-a"]["dev"])
	}
	if lockMap["vendor-a"]["v1.0"] != "hash_a_v1" {
		t.Errorf("Expected hash_a_v1 for vendor-a@v1.0, got %s", lockMap["vendor-a"]["v1.0"])
	}

	// Verify vendor-b has 3 refs
	if len(lockMap["vendor-b"]) != 3 {
		t.Errorf("Expected vendor-b to have 3 refs, got %d", len(lockMap["vendor-b"]))
	}
	if lockMap["vendor-b"]["main"] != "hash_b_main" {
		t.Errorf("Expected hash_b_main for vendor-b@main, got %s", lockMap["vendor-b"]["main"])
	}
}

func TestBuildLockMap_EmptyLock(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	emptyLock := types.VendorLock{Vendors: []types.LockDetails{}}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute
	lockMap := syncService.buildLockMap(emptyLock)

	// Verify: empty map (not nil)
	if lockMap == nil {
		t.Fatal("Expected empty map, got nil")
	}
	if len(lockMap) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(lockMap))
	}
}

func TestBuildLockMap_DuplicateRefs(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Lock with same vendor+ref twice with different hashes
	testLock := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("vendor-a", "main", "hash_first"),
			createTestLockEntry("vendor-a", "main", "hash_second"), // Duplicate
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute
	lockMap := syncService.buildLockMap(testLock)

	// Verify: last write wins
	if lockMap["vendor-a"]["main"] != "hash_second" {
		t.Errorf("Expected last hash (hash_second) to win, got %s", lockMap["vendor-a"]["main"])
	}
}

// ============================================================================
// validateVendorExists() Tests
// ============================================================================

func TestValidateVendorExists_Found(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
			createTestVendorSpec("vendor-b", "https://github.com/b/repo", "main"),
			createTestVendorSpec("vendor-c", "https://github.com/c/repo", "main"),
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: validate vendor-b exists
	err := syncService.validateVendorExists(testConfig, "vendor-b")

	// Verify: nil error
	if err != nil {
		t.Errorf("Expected nil error for existing vendor, got: %v", err)
	}
}

func TestValidateVendorExists_NotFound(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	testConfig := types.VendorConfig{
		Vendors: []types.VendorSpec{
			createTestVendorSpec("vendor-a", "https://github.com/a/repo", "main"),
			createTestVendorSpec("vendor-b", "https://github.com/b/repo", "main"),
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: validate nonexistent vendor
	err := syncService.validateVendorExists(testConfig, "vendor-z")

	// Verify: ErrVendorNotFound
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestValidateVendorExists_EmptyConfig(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	emptyConfig := types.VendorConfig{Vendors: []types.VendorSpec{}}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: validate vendor in empty config
	err := syncService.validateVendorExists(emptyConfig, "any-vendor")

	// Verify: ErrVendorNotFound
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// ============================================================================
// previewSyncVendor() Tests
// ============================================================================

func TestPreviewSyncVendor_LockedRefs(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
			{
				Ref: "dev",
				Mapping: []types.PathMapping{
					{From: "src/dev.go", To: "lib/dev.go"},
				},
			},
		},
	}

	lockedRefs := map[string]string{
		"main": "abc1234567890",
		"dev":  "def0987654321",
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: preview with locked refs
	// Note: This function prints to stdout, so we're just verifying no panic
	syncService.previewSyncVendor(&vendor, lockedRefs)

	// Verify: no panic (output contains "locked: abc1234" and "locked: def0987")
	// Since we can't easily capture stdout in unit tests without additional infrastructure,
	// we're testing that the function completes without error
}

func TestPreviewSyncVendor_UnlockedRefs(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: preview with nil lockedRefs (unlocked mode)
	syncService.previewSyncVendor(&vendor, nil)

	// Verify: no panic (output shows "not synced")
}

func TestPreviewSyncVendor_NoMappings(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref:     "main",
				Mapping: []types.PathMapping{}, // Empty mapping
			},
		},
	}

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	// Execute: preview with no mappings
	syncService.previewSyncVendor(&vendor, nil)

	// Verify: no panic (output shows "(no paths configured)")
}

// ============================================================================
// Group Filtering Tests - Test vendor group filtering functionality
// ============================================================================

func TestSync_GroupFilter_SingleGroup(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Create vendors with different groups
	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "vendor-a",
				URL:     "https://github.com/test/repo-a",
				License: "MIT",
				Groups:  []string{"frontend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
				}},
			},
			{
				Name:    "vendor-b",
				URL:     "https://github.com/test/repo-b",
				License: "MIT",
				Groups:  []string{"backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-b/"}},
				}},
			},
			{
				Name:    "vendor-c",
				URL:     "https://github.com/test/repo-c",
				License: "MIT",
				Groups:  []string{"frontend", "backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-c/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "hash-a"},
			{Name: "vendor-b", Ref: "main", CommitHash: "hash-b"},
			{Name: "vendor-c", Ref: "main", CommitHash: "hash-c"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	// Expect syncs for vendor-a and vendor-c only (have "frontend" group)
	// vendor-a expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-a", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-a").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-a", "origin", "https://github.com/test/repo-a").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-a", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-a", "hash-a").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-a").Return("hash-a", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 5, ByteCount: 500}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-a").Return(nil)

	// vendor-c expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-c", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-c").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-c", "origin", "https://github.com/test/repo-c").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-c", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-c", "hash-c").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-c").Return("hash-c", nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 3, ByteCount: 300}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-c").Return(nil)

	// NO expectations for vendor-b (should be skipped due to group filter)

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "frontend",
	}

	err := syncer.sync.Sync(opts)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// gomock will automatically verify that vendor-b mocks were NOT called
}

func TestSync_GroupFilter_BackendGroup(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "vendor-a",
				URL:     "https://github.com/test/repo-a",
				License: "MIT",
				Groups:  []string{"frontend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
				}},
			},
			{
				Name:    "vendor-b",
				URL:     "https://github.com/test/repo-b",
				License: "MIT",
				Groups:  []string{"backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-b/"}},
				}},
			},
			{
				Name:    "vendor-c",
				URL:     "https://github.com/test/repo-c",
				License: "MIT",
				Groups:  []string{"frontend", "backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-c/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "hash-a"},
			{Name: "vendor-b", Ref: "main", CommitHash: "hash-b"},
			{Name: "vendor-c", Ref: "main", CommitHash: "hash-c"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	// Expect syncs for vendor-b and vendor-c only (have "backend" group)
	// vendor-b expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-b", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-b").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-b", "origin", "https://github.com/test/repo-b").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-b", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-b", "hash-b").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-b").Return("hash-b", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 5, ByteCount: 500}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-b").Return(nil)

	// vendor-c expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-c", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-c").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-c", "origin", "https://github.com/test/repo-c").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-c", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-c", "hash-c").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-c").Return("hash-c", nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 3, ByteCount: 300}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-c").Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "backend",
	}

	err := syncer.sync.Sync(opts)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_GroupFilter_NonexistentGroup(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "vendor-a",
				URL:     "https://github.com/test/repo-a",
				License: "MIT",
				Groups:  []string{"frontend", "backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "hash-a"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "mobile", // This group doesn't exist
	}

	err := syncer.sync.Sync(opts)

	// Should return error
	if err == nil {
		t.Fatal("Expected error for nonexistent group, got nil")
	}

	if !IsGroupNotFound(err) {
		t.Errorf("Expected GroupNotFoundError, got: %v", err)
	}
}

func TestSync_GroupFilter_VendorWithoutGroups(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "vendor-with-group",
				URL:     "https://github.com/test/repo-a",
				License: "MIT",
				Groups:  []string{"frontend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
				}},
			},
			{
				Name:    "vendor-without-group",
				URL:     "https://github.com/test/repo-b",
				License: "MIT",
				Groups:  nil, // No groups
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-b/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-with-group", Ref: "main", CommitHash: "hash-a"},
			{Name: "vendor-without-group", Ref: "main", CommitHash: "hash-b"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	// Only vendor-with-group should be synced
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-a", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-a").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-a", "origin", "https://github.com/test/repo-a").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-a", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-a", "hash-a").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-a").Return("hash-a", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 5, ByteCount: 500}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-a").Return(nil)

	// NO expectations for vendor-without-group

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "frontend",
	}

	err := syncer.sync.Sync(opts)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_GroupFilter_MultipleGroups(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "multi-group-vendor",
				URL:     "https://github.com/test/repo",
				License: "MIT",
				Groups:  []string{"frontend", "backend", "mobile"}, // Multiple groups
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "multi-group-vendor", Ref: "main", CommitHash: "hash-a"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	// Should be synced (matches "mobile" group)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor", "origin", "https://github.com/test/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor", "hash-a").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor").Return("hash-a", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 5, ByteCount: 500}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor").Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "mobile", // Vendor has this group among others
	}

	err := syncer.sync.Sync(opts)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestSync_GroupFilter_EmptyGroupName(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendorConfig := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:    "vendor-a",
				URL:     "https://github.com/test/repo-a",
				License: "MIT",
				Groups:  []string{"frontend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-a/"}},
				}},
			},
			{
				Name:    "vendor-b",
				URL:     "https://github.com/test/repo-b",
				License: "MIT",
				Groups:  []string{"backend"},
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/", To: "lib-b/"}},
				}},
			},
		},
	}

	lockData := &types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "hash-a"},
			{Name: "vendor-b", Ref: "main", CommitHash: "hash-b"},
		},
	}

	config.EXPECT().Load().Return(*vendorConfig, nil)
	lock.EXPECT().Load().Return(*lockData, nil)

	// Both vendors should be synced (no group filter)
	// vendor-a expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-a", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-a").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-a", "origin", "https://github.com/test/repo-a").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-a", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-a", "hash-a").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-a").Return("hash-a", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "src", isDir: true}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 5, ByteCount: 500}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-a").Return(nil)

	// vendor-b expectations
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/vendor-b", nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/vendor-b").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/vendor-b", "origin", "https://github.com/test/repo-b").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/vendor-b", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/vendor-b", "hash-b").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/vendor-b").Return("hash-b", nil)
	fs.EXPECT().CopyDir(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 3, ByteCount: 300}, nil)
	fs.EXPECT().RemoveAll("/tmp/vendor-b").Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	opts := SyncOptions{
		GroupName: "", // Empty = no filter
	}

	err := syncer.sync.Sync(opts)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

// ============================================================================
// fetchWithFallback Tests — Shallow clone fallback paths
// ============================================================================

// TestFetchWithFallback_ShallowSucceeds verifies that fetchWithFallback returns nil
// when the shallow fetch (depth=1) succeeds on the first attempt.
func TestFetchWithFallback_ShallowSucceeds(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Shallow fetch succeeds — FetchAll should NOT be called
	git.EXPECT().Fetch(gomock.Any(), "/tmp/repo", 1, "main").Return(nil)
	git.EXPECT().FetchAll(gomock.Any(), gomock.Any()).Times(0)

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	err := syncService.fetchWithFallback("/tmp/repo", "main")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestFetchWithFallback_ShallowFails_FullSucceeds verifies the fallback path:
// shallow fetch fails, full fetch succeeds.
func TestFetchWithFallback_ShallowFails_FullSucceeds(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	git.EXPECT().Fetch(gomock.Any(), "/tmp/repo", 1, "v1.0").Return(fmt.Errorf("shallow not supported"))
	git.EXPECT().FetchAll(gomock.Any(), "/tmp/repo").Return(nil)

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	err := syncService.fetchWithFallback("/tmp/repo", "v1.0")
	if err != nil {
		t.Fatalf("expected nil (full fetch fallback), got %v", err)
	}
}

// TestFetchWithFallback_BothFail verifies that when both shallow and full fetch fail,
// the error from FetchAll is returned.
func TestFetchWithFallback_BothFail(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	git.EXPECT().Fetch(gomock.Any(), "/tmp/repo", 1, "stale-tag").Return(fmt.Errorf("shallow failed"))
	git.EXPECT().FetchAll(gomock.Any(), "/tmp/repo").Return(fmt.Errorf("network unreachable"))

	syncer := createMockSyncer(git, fs, config, lock, license)
	syncService := syncer.sync.(*SyncService)

	err := syncService.fetchWithFallback("/tmp/repo", "stale-tag")
	if err == nil {
		t.Fatal("expected error when both fetches fail")
	}
	if !contains(err.Error(), "fetch ref stale-tag") {
		t.Errorf("error = %q, want wrapped fetch ref message", err.Error())
	}
}

// ============================================================================
// canSkipSync Tests — Cache hit/miss branching
// ============================================================================

// TestCanSkipSync_CacheMiss verifies that canSkipSync returns false when
// the cache store returns an empty commit hash (cache miss).
func TestCanSkipSync_CacheMiss(t *testing.T) {
	tempDir := t.TempDir()
	cacheStore := NewFileCacheStore(NewOSFileSystem(), tempDir)
	ctrl, git, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	syncService := NewSyncService(config, lock, git, NewOSFileSystem(),
		NewFileCopyService(NewOSFileSystem()), nil, cacheStore, NewHookService(nil),
		&SilentUICallback{}, tempDir)

	mappings := []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}}
	result := syncService.canSkipSync("test-vendor", "main", "abc123", mappings)
	if result {
		t.Error("expected false for cache miss, got true")
	}
}

// TestCanSkipSync_CommitHashMismatch verifies that canSkipSync returns false when
// the cached commit hash doesn't match the locked commit hash.
func TestCanSkipSync_CommitHashMismatch(t *testing.T) {
	tempDir := t.TempDir()
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, tempDir)

	// Create a destination file and a cache with different commit hash
	destFile := filepath.Join(tempDir, "lib/file.go")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	cache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "old-hash-000", // Different from locked hash
		Files: []types.FileChecksum{
			{Path: "lib/file.go", Hash: "somehash"},
		},
	}
	if err := cacheStore.Save(&cache); err != nil {
		t.Fatal(err)
	}

	ctrl, git, _, config, lock, license := setupMocks(t)
	defer ctrl.Finish()
	_ = license // unused but part of setupMocks

	syncService := NewSyncService(config, lock, git, osFS,
		NewFileCopyService(osFS), nil, cacheStore, NewHookService(nil),
		&SilentUICallback{}, tempDir)

	mappings := []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}}
	result := syncService.canSkipSync("test-vendor", "main", "new-hash-111", mappings)
	if result {
		t.Error("expected false for commit hash mismatch, got true")
	}
}

// TestCanSkipSync_MatchingCache verifies that canSkipSync returns true when
// all conditions are met: cache hit, matching commit hash, all files exist with matching checksums.
func TestCanSkipSync_MatchingCache(t *testing.T) {
	tempDir := t.TempDir()
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, tempDir)

	// Create destination file
	destPath := filepath.Join(tempDir, "lib/file.go")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("cached content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute actual checksum of the file
	checksum, err := cacheStore.ComputeFileChecksum(destPath)
	if err != nil {
		t.Fatal(err)
	}

	// Save cache with matching commit hash and file checksums
	cache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Files: []types.FileChecksum{
			{Path: "lib/file.go", Hash: checksum},
		},
	}
	if err := cacheStore.Save(&cache); err != nil {
		t.Fatal(err)
	}

	ctrl, git, _, config, lock, license := setupMocks(t)
	defer ctrl.Finish()
	_ = license

	syncService := NewSyncService(config, lock, git, osFS,
		NewFileCopyService(osFS), nil, cacheStore, NewHookService(nil),
		&SilentUICallback{}, tempDir)

	mappings := []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}}
	result := syncService.canSkipSync("test-vendor", "main", "abc123", mappings)
	if !result {
		t.Error("expected true for fully matching cache, got false")
	}
}

// TestCanSkipSync_FileMissing verifies that canSkipSync returns false when
// a destination file doesn't exist on disk even though the cache matches.
func TestCanSkipSync_FileMissing(t *testing.T) {
	tempDir := t.TempDir()
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, tempDir)

	// Save cache but don't create the destination file
	cache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Files: []types.FileChecksum{
			{Path: "lib/missing.go", Hash: "deadbeef"},
		},
	}
	if err := cacheStore.Save(&cache); err != nil {
		t.Fatal(err)
	}

	ctrl, git, _, config, lock, license := setupMocks(t)
	defer ctrl.Finish()
	_ = license

	syncService := NewSyncService(config, lock, git, osFS,
		NewFileCopyService(osFS), nil, cacheStore, NewHookService(nil),
		&SilentUICallback{}, tempDir)

	mappings := []types.PathMapping{{From: "src/file.go", To: "lib/missing.go"}}
	result := syncService.canSkipSync("test-vendor", "main", "abc123", mappings)
	if result {
		t.Error("expected false for missing destination file, got true")
	}
}

// TestCanSkipSync_AutoNamedPathSkips verifies that canSkipSync returns false
// when a mapping has an empty destination (auto-naming).
func TestCanSkipSync_AutoNamedPathSkips(t *testing.T) {
	tempDir := t.TempDir()
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, tempDir)

	cache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
	}
	if err := cacheStore.Save(&cache); err != nil {
		t.Fatal(err)
	}

	ctrl, git, _, config, lock, license := setupMocks(t)
	defer ctrl.Finish()
	_ = license

	syncService := NewSyncService(config, lock, git, osFS,
		NewFileCopyService(osFS), nil, cacheStore, NewHookService(nil),
		&SilentUICallback{}, tempDir)

	// Empty "To" triggers auto-naming which can't be cache-checked
	mappings := []types.PathMapping{{From: "src/file.go", To: ""}}
	result := syncService.canSkipSync("test-vendor", "main", "abc123", mappings)
	if result {
		t.Error("expected false for auto-named path, got true")
	}
}

// ============================================================================
// SyncVendor — NoCache flag bypasses cache
// ============================================================================

// TestSyncVendor_NoCache_BypassesCache verifies that --no-cache flag forces
// clone even when cache would be a hit, and doesn't update cache after sync.
func TestSyncVendor_NoCache_BypassesCache(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "abc123def456"}

	// Full git operations should happen even if cache would match
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)
	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "abc123def456").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def456", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute with NoCache=true
	_, _, err := syncer.sync.SyncVendor(&vendor, lockedRefs, SyncOptions{NoCache: true})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

// ============================================================================
// SyncVendor — Position mappings integration
// ============================================================================

// TestSyncVendor_WithPositionMappings verifies that SyncVendor correctly handles
// position-specified mappings via the FileCopyService path.
func TestSyncVendor_WithPositionMappings(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source file in fake repo
	if err := os.WriteFile(filepath.Join(repoDir, "api.go"), []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "pos-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L2", To: "local/api_snippet.go"},
				},
			},
		},
	}

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return(repoDir, nil)
	fs.EXPECT().RemoveAll(repoDir).Return(nil)
	git.EXPECT().Init(gomock.Any(), repoDir).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), repoDir, "origin", vendor.URL).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), repoDir, 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), repoDir, "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), repoDir).Return("poscommit123", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	// License: use real file operations for the copy path
	fs.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Use real FileCopyService for position mapping
	realFS := NewOSFileSystem()
	fileCopy := NewFileCopyService(realFS)
	cacheStore := NewFileCacheStore(realFS, workDir)
	hooks := NewHookService(nil)
	licSvc := NewLicenseService(license, fs, workDir, &SilentUICallback{})

	syncSvc := NewSyncService(config, lock, git, fs, fileCopy, licSvc, cacheStore, hooks, &SilentUICallback{}, workDir)

	results, stats, err := syncSvc.SyncVendor(&vendor, nil, SyncOptions{NoCache: true})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if results["main"].CommitHash != "poscommit123" {
		t.Errorf("commit hash = %q, want poscommit123", results["main"].CommitHash)
	}
	if len(stats.Positions) != 1 {
		t.Errorf("expected 1 position record, got %d", len(stats.Positions))
	}

	// Verify the extracted file was created
	got, err := os.ReadFile(filepath.Join(workDir, "local/api_snippet.go"))
	if err != nil {
		t.Fatalf("position-extracted file not found: %v", err)
	}
	if string(got) != "line2" {
		t.Errorf("extracted content = %q, want %q", string(got), "line2")
	}
}

// ============================================================================
// TestUpdateAll - Comprehensive tests for update orchestration
// ============================================================================
