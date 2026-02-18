package core

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// acceptTestLock builds a VendorLock with one vendor entry including file hashes.
func acceptTestLock(name, ref, hash string, fileHashes map[string]string) types.VendorLock {
	return types.VendorLock{
		SchemaVersion: CurrentSchemaVersion,
		Vendors: []types.LockDetails{{
			Name:       name,
			Ref:        ref,
			CommitHash: hash,
			Updated:    "2025-01-01",
			FileHashes: fileHashes,
		}},
	}
}

// TestAccept_SingleFile verifies AcceptService accepts drift for one file
// when the local hash differs from the upstream hash.
func TestAccept_SingleFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	upstreamHash := "aaa111"
	localHash := "bbb222"

	lockData := acceptTestLock("mylib", "main", "abc123", map[string]string{
		"lib/file.go": upstreamHash,
	})

	cache.files["lib/file.go"] = localHash

	lock.EXPECT().Load().Return(lockData, nil)
	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(saved types.VendorLock) error {
		entry := saved.Vendors[0]
		if entry.AcceptedDrift == nil {
			t.Fatal("expected AcceptedDrift to be set")
		}
		if got := entry.AcceptedDrift["lib/file.go"]; got != localHash {
			t.Errorf("expected accepted drift hash %q, got %q", localHash, got)
		}
		return nil
	})

	svc := NewAcceptService(lock, cache)
	result, err := svc.Accept(AcceptOptions{
		VendorName: "mylib",
		FilePath:   "lib/file.go",
	})
	if err != nil {
		t.Fatalf("Accept returned error: %v", err)
	}

	if len(result.AcceptedFiles) != 1 {
		t.Errorf("expected 1 accepted file, got %d", len(result.AcceptedFiles))
	}
	if result.AcceptedFiles[0] != "lib/file.go" {
		t.Errorf("expected accepted file lib/file.go, got %s", result.AcceptedFiles[0])
	}
}

// TestAccept_AllModifiedFiles verifies AcceptService accepts all modified files
// for a vendor when no specific file path is provided.
func TestAccept_AllModifiedFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := acceptTestLock("mylib", "main", "abc123", map[string]string{
		"lib/a.go": "upstream_a",
		"lib/b.go": "upstream_b",
		"lib/c.go": "unchanged",
	})

	cache.files["lib/a.go"] = "local_a"
	cache.files["lib/b.go"] = "local_b"
	cache.files["lib/c.go"] = "unchanged" // matches upstream

	lock.EXPECT().Load().Return(lockData, nil)
	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(saved types.VendorLock) error {
		entry := saved.Vendors[0]
		if len(entry.AcceptedDrift) != 2 {
			t.Errorf("expected 2 accepted drift entries, got %d", len(entry.AcceptedDrift))
		}
		return nil
	})

	svc := NewAcceptService(lock, cache)
	result, err := svc.Accept(AcceptOptions{VendorName: "mylib"})
	if err != nil {
		t.Fatalf("Accept returned error: %v", err)
	}

	if len(result.AcceptedFiles) != 2 {
		t.Errorf("expected 2 accepted files, got %d", len(result.AcceptedFiles))
	}
}

// TestAccept_NoModifiedFiles verifies AcceptService returns an error when
// no files have drift to accept.
func TestAccept_NoModifiedFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := acceptTestLock("mylib", "main", "abc123", map[string]string{
		"lib/file.go": "same_hash",
	})

	cache.files["lib/file.go"] = "same_hash"

	lock.EXPECT().Load().Return(lockData, nil)

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{VendorName: "mylib"})
	if err == nil {
		t.Fatal("expected error when no modified files found")
	}
}

// TestAccept_VendorNotFound verifies AcceptService returns an error for
// unknown vendor names.
func TestAccept_VendorNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := types.VendorLock{
		Vendors: []types.LockDetails{{
			Name: "other-vendor",
			Ref:  "main",
		}},
	}

	lock.EXPECT().Load().Return(lockData, nil)

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{VendorName: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent vendor")
	}
}

// TestAccept_Clear verifies AcceptService removes all accepted_drift entries
// for a vendor when Clear is true.
func TestAccept_Clear(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := types.VendorLock{
		SchemaVersion: CurrentSchemaVersion,
		Vendors: []types.LockDetails{{
			Name:       "mylib",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{
				"lib/a.go": "upstream_a",
				"lib/b.go": "upstream_b",
			},
			AcceptedDrift: map[string]string{
				"lib/a.go": "local_a",
				"lib/b.go": "local_b",
			},
		}},
	}

	lock.EXPECT().Load().Return(lockData, nil)
	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(saved types.VendorLock) error {
		entry := saved.Vendors[0]
		if entry.AcceptedDrift != nil {
			t.Errorf("expected AcceptedDrift to be nil after clear, got %v", entry.AcceptedDrift)
		}
		return nil
	})

	svc := NewAcceptService(lock, cache)
	result, err := svc.Accept(AcceptOptions{VendorName: "mylib", Clear: true})
	if err != nil {
		t.Fatalf("Accept --clear returned error: %v", err)
	}

	if len(result.ClearedFiles) != 2 {
		t.Errorf("expected 2 cleared files, got %d", len(result.ClearedFiles))
	}
}

// TestAccept_ClearSpecificFile verifies AcceptService removes a single
// accepted_drift entry when Clear and FilePath are both set.
func TestAccept_ClearSpecificFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := types.VendorLock{
		SchemaVersion: CurrentSchemaVersion,
		Vendors: []types.LockDetails{{
			Name:       "mylib",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{
				"lib/a.go": "upstream_a",
				"lib/b.go": "upstream_b",
			},
			AcceptedDrift: map[string]string{
				"lib/a.go": "local_a",
				"lib/b.go": "local_b",
			},
		}},
	}

	lock.EXPECT().Load().Return(lockData, nil)
	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(saved types.VendorLock) error {
		entry := saved.Vendors[0]
		if len(entry.AcceptedDrift) != 1 {
			t.Errorf("expected 1 remaining accepted drift entry, got %d", len(entry.AcceptedDrift))
		}
		if _, exists := entry.AcceptedDrift["lib/a.go"]; exists {
			t.Error("lib/a.go should have been cleared")
		}
		if _, exists := entry.AcceptedDrift["lib/b.go"]; !exists {
			t.Error("lib/b.go should still be present")
		}
		return nil
	})

	svc := NewAcceptService(lock, cache)
	result, err := svc.Accept(AcceptOptions{
		VendorName: "mylib",
		FilePath:   "lib/a.go",
		Clear:      true,
	})
	if err != nil {
		t.Fatalf("Accept --clear --file returned error: %v", err)
	}

	if len(result.ClearedFiles) != 1 || result.ClearedFiles[0] != "lib/a.go" {
		t.Errorf("expected cleared file lib/a.go, got %v", result.ClearedFiles)
	}
}

// TestAccept_ClearNoDrift verifies AcceptService returns an error when
// trying to clear drift that doesn't exist.
func TestAccept_ClearNoDrift(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := types.VendorLock{
		SchemaVersion: CurrentSchemaVersion,
		Vendors: []types.LockDetails{{
			Name:       "mylib",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{
				"lib/file.go": "hash",
			},
		}},
	}

	lock.EXPECT().Load().Return(lockData, nil)

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{VendorName: "mylib", Clear: true})
	if err == nil {
		t.Fatal("expected error when clearing vendor with no accepted drift")
	}
}

// ============================================================================
// T2: Accept service error path tests
// ============================================================================

// TestAccept_EmptyVendorName verifies AcceptService returns an error when
// the vendor name is empty.
func TestAccept_EmptyVendorName(t *testing.T) {
	svc := NewAcceptService(nil, nil)
	_, err := svc.Accept(AcceptOptions{VendorName: ""})
	if err == nil {
		t.Fatal("expected error for empty vendor name")
	}
	if !contains(err.Error(), "vendor name is required") {
		t.Errorf("expected 'vendor name is required' error, got: %v", err)
	}
}

// TestAccept_LockSaveFailure verifies AcceptService propagates lock save errors.
func TestAccept_LockSaveFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := acceptTestLock("mylib", "main", "abc123", map[string]string{
		"lib/file.go": "upstream_hash",
	})
	cache.files["lib/file.go"] = "local_hash"

	lock.EXPECT().Load().Return(lockData, nil)
	lock.EXPECT().Save(gomock.Any()).Return(fmt.Errorf("disk full"))

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{VendorName: "mylib"})
	if err == nil {
		t.Fatal("expected error from lock save failure")
	}
	if !contains(err.Error(), "save lockfile") {
		t.Errorf("expected 'save lockfile' in error, got: %v", err)
	}
}

// TestAccept_FileNotFoundInFileHashes verifies AcceptService returns an error
// when --file specifies a path that does not exist in the vendor's FileHashes.
func TestAccept_FileNotFoundInFileHashes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := acceptTestLock("mylib", "main", "abc123", map[string]string{
		"lib/file.go": "upstream_hash",
	})

	lock.EXPECT().Load().Return(lockData, nil)

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{
		VendorName: "mylib",
		FilePath:   "lib/nonexistent.go",
	})
	if err == nil {
		t.Fatal("expected error for file not found in file hashes")
	}
	if !contains(err.Error(), "not found in vendor") {
		t.Errorf("expected 'not found in vendor' in error, got: %v", err)
	}
}

// TestAccept_EmptyFileHashes verifies AcceptService returns an error when
// the vendor has nil/empty FileHashes.
func TestAccept_EmptyFileHashes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lock := NewMockLockStore(ctrl)
	cache := newMockCacheStore()

	lockData := acceptTestLock("mylib", "main", "abc123", nil)

	lock.EXPECT().Load().Return(lockData, nil)

	svc := NewAcceptService(lock, cache)
	_, err := svc.Accept(AcceptOptions{VendorName: "mylib"})
	if err == nil {
		t.Fatal("expected error for empty file hashes")
	}
	if !contains(err.Error(), "no file hashes") {
		t.Errorf("expected 'no file hashes' in error, got: %v", err)
	}
}

// TestVerify_AcceptedDrift verifies that the verify service reports files with
// accepted drift as "accepted" rather than "modified".
func TestVerify_AcceptedDrift(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	cache := newMockCacheStore()
	fs := NewMockFileSystem(ctrl)

	upstreamHash := "upstream_hash"
	localHash := "local_hash"

	config.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "mylib",
			URL:  "https://example.com/mylib",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		SchemaVersion: CurrentSchemaVersion,
		Vendors: []types.LockDetails{{
			Name:       "mylib",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{
				"lib/file.go": upstreamHash,
			},
			AcceptedDrift: map[string]string{
				"lib/file.go": localHash,
			},
		}},
	}, nil)

	cache.files["lib/file.go"] = localHash

	// findAddedFiles will walk directories - stub the Stat call
	fs.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist).AnyTimes()

	svc := NewVerifyService(config, lockStore, cache, fs, ".")
	result, err := svc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	// Check that the file is reported as accepted, not modified
	foundAccepted := false
	for _, f := range result.Files {
		if f.Path == "lib/file.go" {
			if f.Status != "accepted" {
				t.Errorf("expected status 'accepted', got %q", f.Status)
			}
			foundAccepted = true
		}
	}
	if !foundAccepted {
		t.Error("lib/file.go not found in verify results")
	}

	if result.Summary.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", result.Summary.Accepted)
	}
	if result.Summary.Modified != 0 {
		t.Errorf("expected 0 modified, got %d", result.Summary.Modified)
	}
	if result.Summary.Result != "WARN" {
		t.Errorf("expected result WARN (accepted drift), got %s", result.Summary.Result)
	}
}
