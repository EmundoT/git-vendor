package core

import (
	"fmt"
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

	git.EXPECT().Init("/tmp/test-12345").Return(nil)
	git.EXPECT().AddRemote("/tmp/test-12345", "origin", "https://github.com/owner/repo").Return(nil)
	git.EXPECT().Fetch("/tmp/test-12345", 1, "main").Return(nil)
	git.EXPECT().Checkout("/tmp/test-12345", "FETCH_HEAD").Return(nil)
	git.EXPECT().GetHeadHash("/tmp/test-12345").Return("abc123def456", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	git.EXPECT().Init(gomock.Any()).Return(nil).Times(3)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil).Times(3)

	callCount := 0
	git.EXPECT().GetHeadHash(gomock.Any()).DoAndReturn(func(_ string) (string, error) {
		callCount++
		return fmt.Sprintf("hash%d00000", callCount), nil
	}).Times(3)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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
	git.EXPECT().Init(gomock.Any()).DoAndReturn(func(_ string) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("git init failed")
		}
		return nil
	}).Times(3)

	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	git.EXPECT().GetHeadHash(gomock.Any()).Return("abc123def", nil).Times(2)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	git.EXPECT().Init(gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "LICENSE", isDir: false}, nil).AnyTimes()
	fs.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 100}, nil).AnyTimes()
	fs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Mock: Lock save fails
	lock.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("disk full"))

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	git.EXPECT().Init(gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	git.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	// Each spec gets fetched, checked out, and hash retrieved
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil).Times(3)

	hashCounter := 0
	git.EXPECT().GetHeadHash(gomock.Any()).DoAndReturn(func(_ string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}).Times(3)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

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

	git.EXPECT().Init(gomock.Any()).Return(nil)
	git.EXPECT().AddRemote(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().Checkout(gomock.Any(), gomock.Any()).Return(nil)
	git.EXPECT().GetHeadHash(gomock.Any()).Return("abc123def", nil)
	git.EXPECT().GetTagForCommit(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()

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

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	err := syncer.UpdateAll()

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}
