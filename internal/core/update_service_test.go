package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// UpdateAll Tests - Comprehensive tests for update operations
// ============================================================================

func TestUpdateAll_HappyPath_SingleVendor(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: Single vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), "/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), "/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch(gomock.Any(), "/tmp/test-12345", "origin", 1, "main").Return(nil)
	git.EXPECT().Checkout(gomock.Any(), "/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), "/tmp/test-12345").Return("abc123def456", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		// Verify lock content
		if len(l.Vendors) != 1 {
			t.Errorf("Expected 1 lock entry, got %d", len(l.Vendors))
		}
		entry := l.Vendors[0]
		if entry.Name != "test-vendor" {
			t.Errorf("Expected vendor name 'test-vendor', got '%s'", entry.Name)
		}
		if entry.Ref != "main" {
			t.Errorf("Expected ref 'main', got '%s'", entry.Ref)
		}
		if entry.CommitHash != "abc123def456" {
			t.Errorf("Expected hash 'abc123def456', got '%s'", entry.CommitHash)
		}
		if entry.Updated == "" {
			t.Error("Expected Updated timestamp, got empty string")
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAll_HappyPath_MultipleVendors(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "dev")
	vendor3 := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "v1.0")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2, vendor3), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(3)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(3)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)

	callCount := 0
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string) (string, error) {
		callCount++
		return fmt.Sprintf("hash%d00000", callCount), nil
	}).Times(3)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 3 {
			t.Errorf("Expected 3 lock entries, got %d", len(l.Vendors))
		}
		vendorNames := make(map[string]bool)
		for _, entry := range l.Vendors {
			vendorNames[entry.Name] = true
		}
		if !vendorNames["vendor-a"] || !vendorNames["vendor-b"] || !vendorNames["vendor-c"] {
			t.Error("Not all vendors were locked")
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAll_ConfigLoadFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Mock: Config load fails
	config.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config file corrupt"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "config file corrupt") {
		t.Errorf("Expected config error, got: %v", err)
	}
}

func TestUpdateAll_OneVendorFails_OthersContinue(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: 3 vendors
	vendor1 := createTestVendorSpec("vendor-good-1", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-bad", "https://github.com/owner/repo-b", "main")
	vendor3 := createTestVendorSpec("vendor-good-2", "https://github.com/owner/repo-c", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2, vendor3), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(3)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(3)

	// Mock: vendor-bad fails (second call), others succeed
	callCount := 0
	git.EXPECT().Init(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("git init failed")
		}
		return nil
	}).Times(3)

	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil).Times(2)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 2 {
			t.Errorf("Expected 2 lock entries (vendor-bad skipped), got %d", len(l.Vendors))
		}
		for _, entry := range l.Vendors {
			if entry.Name == "vendor-bad" {
				t.Error("vendor-bad should not be in lock file")
			}
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify: Overall success (UpdateAll continues on individual failures)
	if err != nil {
		t.Fatalf("Expected success (continue on error), got: %v", err)
	}
}

func TestUpdateAll_LockSaveFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Mock: Lock save fails
	lock.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("disk full"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestUpdateAll_EmptyConfig(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: Empty config (no vendors)
	config.EXPECT().Load().Return(types.VendorConfig{Vendors: []types.VendorSpec{}}, nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 0 {
			t.Errorf("Expected empty lock, got %d entries", len(l.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success (empty is valid), got error: %v", err)
	}
}

func TestUpdateAll_TimestampFormat(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		entry := l.Vendors[0]
		if entry.Updated == "" {
			t.Fatal("Expected non-empty timestamp")
		}
		// Try to parse the timestamp (should not error)
		_, err := time.Parse(time.RFC3339, entry.Updated)
		if err != nil {
			t.Errorf("Timestamp not in RFC3339 format: %v", err)
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAll_MultipleSpecsPerVendor(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// Setup: 1 vendor with 3 specs
	vendor := types.VendorSpec{
		Name:    "multi-spec-vendor",
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

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	// syncVendor creates ONE temp dir and clones ONCE, then processes all 3 specs
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(1)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(1)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// Each spec gets fetched, checked out, and hash retrieved
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)

	hashCounter := 0
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}).Times(3)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 3 {
			t.Errorf("Expected 3 lock entries (one per spec), got %d", len(l.Vendors))
		}
		refs := make(map[string]bool)
		for _, entry := range l.Vendors {
			refs[entry.Ref] = true
		}
		if !refs["main"] || !refs["dev"] || !refs["v1.0"] {
			t.Error("Not all refs were locked")
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAll_LicensePathSet(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		entry := l.Vendors[0]
		expectedPath := filepath.Join("/mock/vendor", LicensesDir, "test-vendor.txt")
		if entry.LicensePath != expectedPath {
			t.Errorf("Expected license path '%s', got '%s'", expectedPath, entry.LicensePath)
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	err := syncer.UpdateAll(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

// ============================================================================
// toPositionLocks Tests
// ============================================================================

func TestToPositionLocks_NilInput(t *testing.T) {
	result := toPositionLocks(nil)
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}
}

func TestToPositionLocks_EmptySlice(t *testing.T) {
	result := toPositionLocks([]positionRecord{})
	if result != nil {
		t.Errorf("Expected nil for empty slice, got %v", result)
	}
}

func TestToPositionLocks_SingleRecord(t *testing.T) {
	records := []positionRecord{
		{From: "api/constants.go:L4-L6", To: "lib/config.go:L10-L12", SourceHash: "sha256:abc123"},
	}

	result := toPositionLocks(records)

	if len(result) != 1 {
		t.Fatalf("Expected 1 lock, got %d", len(result))
	}
	if result[0].From != "api/constants.go:L4-L6" {
		t.Errorf("Expected From 'api/constants.go:L4-L6', got '%s'", result[0].From)
	}
	if result[0].To != "lib/config.go:L10-L12" {
		t.Errorf("Expected To 'lib/config.go:L10-L12', got '%s'", result[0].To)
	}
	if result[0].SourceHash != "sha256:abc123" {
		t.Errorf("Expected SourceHash 'sha256:abc123', got '%s'", result[0].SourceHash)
	}
}

func TestToPositionLocks_MultipleRecords(t *testing.T) {
	records := []positionRecord{
		{From: "src/a.go:L1-L5", To: "lib/a.go:L10-L14", SourceHash: "sha256:hash1"},
		{From: "src/b.go:L20-L30", To: "lib/b.go", SourceHash: "sha256:hash2"},
		{From: "src/c.go:L1-EOF", To: "lib/c.go:L1-EOF", SourceHash: "sha256:hash3"},
	}

	result := toPositionLocks(records)

	if len(result) != 3 {
		t.Fatalf("Expected 3 locks, got %d", len(result))
	}

	for i, r := range records {
		if result[i].From != r.From {
			t.Errorf("Record %d: From mismatch: got '%s', want '%s'", i, result[i].From, r.From)
		}
		if result[i].To != r.To {
			t.Errorf("Record %d: To mismatch: got '%s', want '%s'", i, result[i].To, r.To)
		}
		if result[i].SourceHash != r.SourceHash {
			t.Errorf("Record %d: SourceHash mismatch: got '%s', want '%s'", i, result[i].SourceHash, r.SourceHash)
		}
	}
}

// ============================================================================
// computeFileHashes Tests
// ============================================================================

func TestComputeFileHashes_EmptyMappings(t *testing.T) {
	cache := newMockCacheStore()

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref:     "main",
			Mapping: []types.PathMapping{},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 0 {
		t.Errorf("Expected empty hashes for empty mappings, got %d", len(result))
	}
}

func TestComputeFileHashes_NoMatchingRef(t *testing.T) {
	cache := newMockCacheStore()

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/file.go", To: "lib/file.go"},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "non-existent-ref")
	if len(result) != 0 {
		t.Errorf("Expected empty hashes for non-matching ref, got %d", len(result))
	}
}

func TestComputeFileHashes_SingleFile(t *testing.T) {
	cache := newMockCacheStore()
	cache.files["lib/file.go"] = "sha256:abc123"

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/file.go", To: "lib/file.go"},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 1 {
		t.Fatalf("Expected 1 hash, got %d", len(result))
	}
	if result["lib/file.go"] != "sha256:abc123" {
		t.Errorf("Expected hash 'sha256:abc123', got '%s'", result["lib/file.go"])
	}
}

func TestComputeFileHashes_MultipleMappings(t *testing.T) {
	cache := newMockCacheStore()
	cache.files["lib/a.go"] = "hash-a"
	cache.files["lib/b.go"] = "hash-b"
	cache.files["lib/c.go"] = "hash-c"

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/a.go", To: "lib/a.go"},
				{From: "src/b.go", To: "lib/b.go"},
				{From: "src/c.go", To: "lib/c.go"},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 3 {
		t.Fatalf("Expected 3 hashes, got %d", len(result))
	}
	for _, path := range []string{"lib/a.go", "lib/b.go", "lib/c.go"} {
		if _, ok := result[path]; !ok {
			t.Errorf("Missing hash for %s", path)
		}
	}
}

func TestComputeFileHashes_MissingFile(t *testing.T) {
	cache := newMockCacheStore()
	cache.files["lib/a.go"] = "hash-a"

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/a.go", To: "lib/a.go"},
				{From: "src/b.go", To: "lib/b.go"},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 1 {
		t.Fatalf("Expected 1 hash (missing file skipped), got %d", len(result))
	}
	if _, ok := result["lib/a.go"]; !ok {
		t.Error("Expected hash for lib/a.go")
	}
	if _, ok := result["lib/b.go"]; ok {
		t.Error("Should not have hash for missing lib/b.go")
	}
}

func TestComputeFileHashes_AutoPath(t *testing.T) {
	cache := newMockCacheStore()
	autoPath := ComputeAutoPath("src/file.go", "", "test-vendor")
	cache.files[autoPath] = "hash-auto"

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/file.go", To: ""},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 1 {
		t.Fatalf("Expected 1 hash for auto-path, got %d", len(result))
	}
	if result[autoPath] != "hash-auto" {
		t.Errorf("Expected hash 'hash-auto' at auto-path '%s', got '%s'", autoPath, result[autoPath])
	}
}

func TestComputeFileHashes_PositionStripped(t *testing.T) {
	cache := newMockCacheStore()
	cache.files["lib/config.go"] = "hash-config"

	svc := &UpdateService{cache: cache}
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/config.go:L5-L10", To: "lib/config.go:L20-L25"},
			},
		}},
	}

	result := svc.computeFileHashes(vendor, "main")
	if len(result) != 1 {
		t.Fatalf("Expected 1 hash, got %d", len(result))
	}
	if result["lib/config.go"] != "hash-config" {
		t.Errorf("Expected hash at 'lib/config.go', got keys: %v", result)
	}
}

func TestComputeFileHashes_LargeFileConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.go")

	var content string
	for i := 0; i < 1000; i++ {
		content += fmt.Sprintf("// line %d: some generated content\n", i)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	hash1, err := realCache.ComputeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("First hash failed: %v", err)
	}

	hash2, err := realCache.ComputeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("Second hash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash inconsistency: %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-char SHA-256 hex string, got %d chars", len(hash1))
	}
}

// ============================================================================
// UpdateAllWithOptions (parallel) Tests
// ============================================================================

func TestUpdateAllWithOptions_SequentialFallback(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 1 {
			t.Errorf("Expected 1 lock entry, got %d", len(l.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{Parallel: types.ParallelOptions{Enabled: false}})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAllWithOptions_ParallelMultipleVendors(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(2)
	fs.EXPECT().RemoveAll(gomock.Any()).Return(nil).Times(2)

	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil).Times(2)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 2 {
			t.Errorf("Expected 2 lock entries, got %d", len(l.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{Parallel: types.ParallelOptions{Enabled: true, MaxWorkers: 2}})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAllWithOptions_ParallelPartialFailure(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-ok", "https://github.com/owner/repo-ok", "main")
	vendor2 := createTestVendorSpec("vendor-fail", "https://github.com/owner/repo-fail", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(2)
	fs.EXPECT().RemoveAll(gomock.Any()).Return(nil).Times(2)

	initCount := 0
	git.EXPECT().Init(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string) error {
		initCount++
		if initCount == 2 {
			return fmt.Errorf("git init failed for second vendor")
		}
		return nil
	}).Times(2)

	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def", nil).AnyTimes()
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) < 1 {
			t.Errorf("Expected at least 1 lock entry from successful vendor, got %d", len(l.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{Parallel: types.ParallelOptions{Enabled: true, MaxWorkers: 2}})
	if err != nil {
		t.Fatalf("Expected success (partial results saved), got error: %v", err)
	}
}

// ============================================================================
// UpdateAllWithOptions — VendorName / Group Filtering Tests
// ============================================================================

func TestUpdateAllWithOptions_VendorNameFilter(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2), nil)

	// Existing lock has entries for both vendors
	existingLock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "old_a_hash", Updated: "2024-01-01T00:00:00Z", VendoredAt: "2024-01-01T00:00:00Z", VendoredBy: "alice"},
			{Name: "vendor-b", Ref: "main", CommitHash: "old_b_hash", Updated: "2024-01-01T00:00:00Z", VendoredAt: "2024-01-01T00:00:00Z", VendoredBy: "bob"},
		},
	}
	lock.EXPECT().Load().Return(existingLock, nil)

	// Only vendor-a should be synced (one temp dir, one init, etc.)
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(1)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(1)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), "https://github.com/owner/repo-a").Return(nil).Times(1)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("new_a_hash_1234", nil).Times(1)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 2 {
			t.Errorf("Expected 2 lock entries (1 updated + 1 preserved), got %d", len(l.Vendors))
		}
		entryMap := make(map[string]types.LockDetails)
		for _, e := range l.Vendors {
			entryMap[e.Name] = e
		}
		// vendor-a should have new hash
		if a, ok := entryMap["vendor-a"]; !ok {
			t.Error("Missing vendor-a in lock")
		} else if a.CommitHash != "new_a_hash_1234" {
			t.Errorf("vendor-a should have new hash, got %s", a.CommitHash)
		}
		// vendor-b should be preserved with old hash
		if b, ok := entryMap["vendor-b"]; !ok {
			t.Error("Missing vendor-b in lock")
		} else if b.CommitHash != "old_b_hash" {
			t.Errorf("vendor-b should retain old hash 'old_b_hash', got '%s'", b.CommitHash)
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{VendorName: "vendor-a"})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAllWithOptions_GroupFilter(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	// vendor-a in "frontend" group, vendor-b in "backend" group, vendor-c in "frontend" group
	vendorA := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendorA.Groups = []string{"frontend"}
	vendorB := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")
	vendorB.Groups = []string{"backend"}
	vendorC := createTestVendorSpec("vendor-c", "https://github.com/owner/repo-c", "main")
	vendorC.Groups = []string{"frontend"}

	config.EXPECT().Load().Return(createTestConfig(vendorA, vendorB, vendorC), nil)

	existingLock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "old_a"},
			{Name: "vendor-b", Ref: "main", CommitHash: "old_b"},
			{Name: "vendor-c", Ref: "main", CommitHash: "old_c"},
		},
	}
	lock.EXPECT().Load().Return(existingLock, nil)

	// Two vendors in "frontend" group should be synced
	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(2)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(2)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("new_hash_00000", nil).Times(2)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 3 {
			t.Errorf("Expected 3 lock entries (2 updated + 1 preserved), got %d", len(l.Vendors))
		}
		entryMap := make(map[string]types.LockDetails)
		for _, e := range l.Vendors {
			entryMap[e.Name] = e
		}
		// vendor-a and vendor-c should have new hash
		if a, ok := entryMap["vendor-a"]; !ok || a.CommitHash != "new_hash_00000" {
			t.Error("vendor-a should have new hash")
		}
		if c, ok := entryMap["vendor-c"]; !ok || c.CommitHash != "new_hash_00000" {
			t.Error("vendor-c should have new hash")
		}
		// vendor-b should be preserved
		if b, ok := entryMap["vendor-b"]; !ok || b.CommitHash != "old_b" {
			t.Error("vendor-b should retain old hash")
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{Group: "frontend"})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateAllWithOptions_NonExistentVendorName(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("real-vendor", "https://github.com/owner/repo", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{VendorName: "no-such-vendor"})
	if err == nil {
		t.Fatal("Expected error for non-existent vendor, got nil")
	}

	var vnfErr *VendorNotFoundError
	if !errors.As(err, &vnfErr) {
		t.Errorf("Expected VendorNotFoundError, got: %T: %v", err, err)
	}
}

func TestUpdateAllWithOptions_NonExistentGroup(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("real-vendor", "https://github.com/owner/repo", "main")
	vendor.Groups = []string{"existing-group"}
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{Group: "ghost-group"})
	if err == nil {
		t.Fatal("Expected error for non-existent group, got nil")
	}

	var gnfErr *GroupNotFoundError
	if !errors.As(err, &gnfErr) {
		t.Errorf("Expected GroupNotFoundError, got: %T: %v", err, err)
	}
}

func TestUpdateAllWithOptions_NoFilter_Regression(t *testing.T) {
	// Regression: Ensure no-filter update still processes all vendors (original behavior).
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	vendor1 := createTestVendorSpec("vendor-a", "https://github.com/owner/repo-a", "main")
	vendor2 := createTestVendorSpec("vendor-b", "https://github.com/owner/repo-b", "main")

	config.EXPECT().Load().Return(createTestConfig(vendor1, vendor2), nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, nil)

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil).Times(2)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil).Times(2)
	git.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), "origin", gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().GetHeadHash(gomock.Any(), gomock.Any()).Return("abc123def456", nil).Times(2)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if len(l.Vendors) != 2 {
			t.Errorf("Expected 2 lock entries (all vendors), got %d", len(l.Vendors))
		}
		return nil
	})

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Empty options = no filter = update all
	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}
