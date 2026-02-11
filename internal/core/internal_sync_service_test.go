package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// InternalSyncService Tests
// ============================================================================

func TestInternalSync_WholeFileCopy(t *testing.T) {
	// Whole-file copy from local source to destination.
	// InternalSyncService uses os.Stat for source, but ValidateDestPath requires
	// relative destination paths. We chdir into tmpDir so both paths can be relative.
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	// Create source file (relative path)
	srcDir := filepath.Join("pkg", "shared")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join("pkg", "shared", "types.go")
	if err := os.WriteFile(srcFile, []byte("package shared\n\ntype Foo struct{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	destFile := filepath.Join("internal", "vendor", "types.go")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// ComputeFileChecksum uses the relative path (srcPath = "./pkg/shared/types.go")
	cache.files[filepath.Join(".", srcFile)] = "abc123src"

	// MkdirAll for destination parent directory, then CopyFile
	mockFS.EXPECT().MkdirAll(filepath.Dir(destFile), os.FileMode(0755)).Return(nil)
	mockFS.EXPECT().CopyFile(filepath.Join(".", srcFile), destFile).Return(CopyStats{FileCount: 1, ByteCount: 33}, nil)

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "shared-types",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{
				Ref: RefLocal,
				Mapping: []types.PathMapping{
					{From: srcFile, To: destFile},
				},
			},
		},
	}

	results, stats, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("SyncInternalVendor() unexpected error: %v", err)
	}

	if stats.FileCount != 1 {
		t.Errorf("expected FileCount=1, got %d", stats.FileCount)
	}

	meta, ok := results[RefLocal]
	if !ok {
		t.Fatal("expected results to contain key 'local'")
	}
	if meta.CommitHash == "" {
		t.Error("expected non-empty content hash in CommitHash")
	}
}

func TestInternalSync_NonExistentSourceReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()
	// Source file hash not in cache → ComputeFileChecksum returns os.ErrNotExist

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "missing-src",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{
				Ref: RefLocal,
				Mapping: []types.PathMapping{
					{From: "nonexistent.go", To: "dest.go"},
				},
			},
		},
	}

	_, _, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err == nil {
		t.Fatal("expected error for non-existent source file, got nil")
	}
	if !contains(err.Error(), "compute source hash") {
		t.Errorf("expected error about computing source hash, got: %v", err)
	}
}

func TestInternalSync_SourceHashesPopulatedInMetadata(t *testing.T) {
	// Verify that the content-addressed hash changes when the source changes.
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	srcFile := "src.go"
	if err := os.WriteFile(srcFile, []byte("version1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	destFile := "dest.go"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files[filepath.Join(".", srcFile)] = "hash_version1"

	mockFS.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockFS.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 10}, nil).AnyTimes()

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "hash-test",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{
				Ref: RefLocal,
				Mapping: []types.PathMapping{
					{From: srcFile, To: destFile},
				},
			},
		},
	}

	results1, _, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("first sync error: %v", err)
	}
	hash1 := results1[RefLocal].CommitHash

	// Change source hash
	cache.files[filepath.Join(".", srcFile)] = "hash_version2"

	results2, _, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	hash2 := results2[RefLocal].CommitHash

	if hash1 == hash2 {
		t.Error("expected different content hashes for different source content, got same")
	}
	if hash1 == "" || hash2 == "" {
		t.Error("expected non-empty content hashes")
	}
}

func TestInternalSync_DryRunDoesNotCopy(t *testing.T) {
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	srcFile := "src.go"
	if err := os.WriteFile(srcFile, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()
	cache.files[filepath.Join(".", srcFile)] = "somehash"

	// No CopyFile or MkdirAll expectations → will fail if called

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "dry-run-test",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{
				Ref: RefLocal,
				Mapping: []types.PathMapping{
					{From: srcFile, To: "dest.go"},
				},
			},
		},
	}

	results, stats, err := svc.SyncInternalVendor(vendor, SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run sync error: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("expected dry-run FileCount=1, got %d", stats.FileCount)
	}

	meta, ok := results[RefLocal]
	if !ok {
		t.Fatal("expected results to contain key 'local'")
	}
	if meta.CommitHash == "" {
		t.Error("expected non-empty content hash even in dry-run")
	}
}

func TestInternalSync_PositionExtraction(t *testing.T) {
	// Position extraction: From has L2-L4 position spec.
	// InternalSyncService should extract lines 2-4 from source and write to dest.
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	// Create source file with 5 lines
	srcFile := "src.go"
	srcContent := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Destination file (will be created by PlaceContent)
	destFile := filepath.Join("out", "extracted.go")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()
	cache.files[filepath.Join(".", srcFile)] = "src_hash_123"

	// MkdirAll for dest parent dir (position extraction path)
	mockFS.EXPECT().MkdirAll("out", os.FileMode(0755)).Return(nil)

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "pos-extract",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{
				Ref: RefLocal,
				Mapping: []types.PathMapping{
					{From: srcFile + ":L2-L4", To: destFile},
				},
			},
		},
	}

	// Create the dest directory so PlaceContent can write
	if err := os.MkdirAll("out", 0755); err != nil {
		t.Fatal(err)
	}

	results, stats, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("SyncInternalVendor() unexpected error: %v", err)
	}

	if stats.FileCount != 1 {
		t.Errorf("expected FileCount=1, got %d", stats.FileCount)
	}

	// Position record should be present
	if len(stats.Positions) != 1 {
		t.Fatalf("expected 1 position record, got %d", len(stats.Positions))
	}
	if stats.Positions[0].SourceHash == "" {
		t.Error("expected non-empty source hash in position record")
	}

	meta, ok := results[RefLocal]
	if !ok {
		t.Fatal("expected results to contain key 'local'")
	}
	if meta.CommitHash == "" {
		t.Error("expected non-empty content hash in CommitHash")
	}

	// Verify extracted content: lines 2-4 = "line2\nline3\nline4"
	// ExtractPosition does not append a trailing newline.
	destContent, readErr := os.ReadFile(destFile)
	if readErr != nil {
		t.Fatalf("read dest file: %v", readErr)
	}
	expected := "line2\nline3\nline4"
	if string(destContent) != expected {
		t.Errorf("expected extracted content %q, got %q", expected, string(destContent))
	}
}

func TestInternalSync_CacheSkipWhenHashesUnchanged(t *testing.T) {
	// Two consecutive syncs with identical source hashes should produce
	// identical content hashes, enabling cache skip logic.
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	srcFile := "src.go"
	if err := os.WriteFile(srcFile, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()
	cache.files[filepath.Join(".", srcFile)] = "stable_hash"

	mockFS.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockFS.EXPECT().CopyFile(gomock.Any(), gomock.Any()).Return(CopyStats{FileCount: 1, ByteCount: 8}, nil).AnyTimes()

	svc := NewInternalSyncService(configStore, lockStore, &stubFileCopyService{}, cache, mockFS, tmpDir)

	vendor := &types.VendorSpec{
		Name:   "cache-test",
		Source: SourceInternal,
		Specs: []types.BranchSpec{
			{Ref: RefLocal, Mapping: []types.PathMapping{{From: srcFile, To: "dest.go"}}},
		},
	}

	results1, _, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("first sync error: %v", err)
	}
	hash1 := results1[RefLocal].CommitHash

	// Second sync with same hash
	results2, _, err := svc.SyncInternalVendor(vendor, SyncOptions{})
	if err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	hash2 := results2[RefLocal].CommitHash

	if hash1 != hash2 {
		t.Errorf("expected identical content hashes for unchanged source, got %q vs %q", hash1, hash2)
	}
	if hash1 == "" {
		t.Error("expected non-empty content hash")
	}
}
