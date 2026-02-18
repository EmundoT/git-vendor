package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// MockCacheStore for testing
// ============================================================================

type mockCacheStore struct {
	files  map[string]string // path -> hash
	caches map[string]types.IncrementalSyncCache
}

func newMockCacheStore() *mockCacheStore {
	return &mockCacheStore{
		files:  make(map[string]string),
		caches: make(map[string]types.IncrementalSyncCache),
	}
}

func (m *mockCacheStore) Load(vendorName, ref string) (types.IncrementalSyncCache, error) {
	key := vendorName + "@" + ref
	if cache, ok := m.caches[key]; ok {
		return cache, nil
	}
	return types.IncrementalSyncCache{}, nil
}

func (m *mockCacheStore) Save(cache *types.IncrementalSyncCache) error {
	key := cache.VendorName + "@" + cache.Ref
	m.caches[key] = *cache
	return nil
}

func (m *mockCacheStore) Delete(vendorName, ref string) error {
	key := vendorName + "@" + ref
	delete(m.caches, key)
	return nil
}

func (m *mockCacheStore) ComputeFileChecksum(path string) (string, error) {
	if hash, ok := m.files[path]; ok {
		return hash, nil
	}
	return "", os.ErrNotExist
}

func (m *mockCacheStore) BuildCache(vendorName, ref, commitHash string, files []string) (types.IncrementalSyncCache, error) {
	cache := types.IncrementalSyncCache{
		VendorName: vendorName,
		Ref:        ref,
		CommitHash: commitHash,
		Files:      make([]types.FileChecksum, 0),
	}
	for _, path := range files {
		if hash, ok := m.files[path]; ok {
			cache.Files = append(cache.Files, types.FileChecksum{Path: path, Hash: hash})
		}
	}
	return cache, nil
}

// ============================================================================
// Verify Service Tests
// ============================================================================

func TestVerify_AllPass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup: File exists with correct hash
	cache.files["lib/test-vendor/file.go"] = "abc123hash"

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with file hashes
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "abc123hash",
				},
			},
		},
	}, nil)

	// Mock fs.Stat for findAddedFiles - return file (not dir) so no walk occurs
	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}

	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified file, got %d", result.Summary.Verified)
	}

	if result.Summary.TotalFiles != 1 {
		t.Errorf("Expected 1 total file, got %d", result.Summary.TotalFiles)
	}

	if result.Files[0].Type != "file" {
		t.Errorf("Expected type 'file', got '%s'", result.Files[0].Type)
	}
}

func TestVerify_ModifiedFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup: File exists but hash is different
	cache.files["lib/test-vendor/file.go"] = "modified123hash"

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with original hash
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "original123hash",
				},
			},
		},
	}, nil)

	// Mock fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	if result.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified file, got %d", result.Summary.Modified)
	}

	// Check file status details
	if len(result.Files) != 1 {
		t.Fatalf("Expected 1 file status, got %d", len(result.Files))
	}

	if result.Files[0].Status != "modified" {
		t.Errorf("Expected status 'modified', got '%s'", result.Files[0].Status)
	}

	if result.Files[0].Type != "file" {
		t.Errorf("Expected type 'file', got '%s'", result.Files[0].Type)
	}

	if result.Files[0].ExpectedHash == nil || *result.Files[0].ExpectedHash != "original123hash" {
		t.Errorf("Expected hash mismatch in result")
	}

	if result.Files[0].ActualHash == nil || *result.Files[0].ActualHash != "modified123hash" {
		t.Errorf("Actual hash not recorded correctly")
	}
}

func TestVerify_DeletedFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup: File does NOT exist (not in cache.files)

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with file hash (but file is deleted)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "original123hash",
				},
			},
		},
	}, nil)

	// Mock fs.Stat for findAddedFiles - file doesn't exist
	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	if result.Summary.Deleted != 1 {
		t.Errorf("Expected 1 deleted file, got %d", result.Summary.Deleted)
	}

	// Check file status details
	if len(result.Files) != 1 {
		t.Fatalf("Expected 1 file status, got %d", len(result.Files))
	}

	if result.Files[0].Status != "deleted" {
		t.Errorf("Expected status 'deleted', got '%s'", result.Files[0].Status)
	}

	if result.Files[0].Type != "file" {
		t.Errorf("Expected type 'file', got '%s'", result.Files[0].Type)
	}

	if result.Files[0].ActualHash != nil {
		t.Errorf("Deleted file should have nil actual hash")
	}
}

func TestVerify_AddedFile(t *testing.T) {
	// Requires a real filesystem for the walkdir to work
	// Create temporary directory and work from within the directory
	tmpDir, err := os.MkdirTemp("", "verify-added-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current directory and change to tmpDir
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	// Create test directory and files (relative paths from tmpDir)
	vendorDir := filepath.Join("lib", "test-vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	// Create expected file
	expectedFilePath := filepath.Join(vendorDir, "file.go")
	if err := os.WriteFile(expectedFilePath, []byte("package test\n"), 0644); err != nil {
		t.Fatalf("Failed to write expected file: %v", err)
	}

	// Create extra file (not in lockfile)
	extraFilePath := filepath.Join(vendorDir, "extra.go")
	if err := os.WriteFile(extraFilePath, []byte("package extra\n"), 0644); err != nil {
		t.Fatalf("Failed to write extra file: %v", err)
	}

	// Compute hashes using relative paths
	realCache := NewFileCacheStore(NewOSFileSystem(), ".")
	expectedHash, err := realCache.ComputeFileChecksum(expectedFilePath)
	if err != nil {
		t.Fatalf("Failed to compute expected hash: %v", err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config - use relative paths
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/", To: vendorDir},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with only the expected file using relative paths
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					expectedFilePath: expectedHash,
				},
			},
		},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), ".")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// With only added files (no modified/deleted), result should be WARN
	if result.Summary.Result != "WARN" {
		t.Errorf("Expected WARN, got %s", result.Summary.Result)
	}

	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified file, got %d", result.Summary.Verified)
	}

	if result.Summary.Added != 1 {
		t.Errorf("Expected 1 added file, got %d", result.Summary.Added)
	}
}

func TestVerify_JSONOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup: File exists with correct hash
	cache.files["lib/test-vendor/file.go"] = "abc123hash"

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with file hashes
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "abc123hash",
				},
			},
		},
	}, nil)

	// Mock fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify JSON can be marshalled
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Verify JSON structure
	var parsed types.VerifyResult
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed.SchemaVersion != "1.0" {
		t.Errorf("Expected schema version 1.0, got %s", parsed.SchemaVersion)
	}

	if parsed.Summary.Result != "PASS" {
		t.Errorf("Expected PASS in parsed JSON, got %s", parsed.Summary.Result)
	}

	// Verify type field is present in JSON roundtrip
	if len(parsed.Files) > 0 && parsed.Files[0].Type != "file" {
		t.Errorf("Expected type 'file' in parsed JSON, got '%s'", parsed.Files[0].Type)
	}
}

func TestVerify_FallbackToCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup: File exists with correct hash
	cache.files["lib/test-vendor/file.go"] = "abc123hash"

	// Setup: Cache has the expected files
	cache.caches["test-vendor@main"] = types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123def",
		Files: []types.FileChecksum{
			{Path: "lib/test-vendor/file.go", Hash: "abc123hash"},
		},
	}

	// Mock config
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock WITHOUT file hashes (should fallback to cache)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				// No FileHashes - should fallback to cache
			},
		},
	}, nil)

	// Mock fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS (from cache fallback), got %s", result.Summary.Result)
	}

	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified file, got %d", result.Summary.Verified)
	}
}

// ============================================================================
// Integration Test with Real Filesystem
// ============================================================================

func TestVerify_IntegrationWithRealFiles(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "verify-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	vendorDir := filepath.Join(tmpDir, "lib", "test-vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	testFile := filepath.Join(vendorDir, "file.go")
	content := []byte("package test\n")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Compute actual hash of file
	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	actualHash, err := realCache.ComputeFileChecksum(testFile)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Mock config — use absolute testFile path to match lock FileHashes keys.
	// Coherence detection (VFY-001) cross-references config destinations against
	// lock FileHashes; mismatched path forms would produce false stale/orphaned results.
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: testFile},
						},
					},
				},
			},
		},
	}, nil)

	// Mock lock with correct hash
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					testFile: actualHash, // Use actual hash
				},
			},
		},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)

	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}

	// Now modify the file and verify again
	if err := os.WriteFile(testFile, []byte("package modified\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Need to reset mocks for second call
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: testFile},
						},
					},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					testFile: actualHash, // Original hash (should not match)
				},
			},
		},
	}, nil)

	result, err = service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL after modification, got %s", result.Summary.Result)
	}

	if result.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified file, got %d", result.Summary.Modified)
	}
}

// ============================================================================
// Position Verification Tests (gap #3)
// ============================================================================

func TestVerify_PositionExtraction_Verified(t *testing.T) {
	// Create temp dir with a dest file that has the expected content at a specific position
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "config.ts")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}
	// Write file: lines 10-12 contain the vendored content
	var lines []string
	for i := 1; i <= 15; i++ {
		if i >= 10 && i <= 12 {
			lines = append(lines, fmt.Sprintf("vendored-line-%d", i))
		} else {
			lines = append(lines, fmt.Sprintf("local-line-%d", i))
		}
	}
	content := joinLines(lines)
	if err := os.WriteFile(destFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute the hash of the extracted position (lines 10-12)
	_, sourceHash, err := ExtractPosition(destFile, &types.PositionSpec{StartLine: 10, EndLine: 12})
	if err != nil {
		t.Fatalf("failed to compute position hash: %v", err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "api-constants",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "api/constants.go:L4-L6", To: destFile + ":L10-L12"}},
			}},
		}},
	}, nil)

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeFileHash, _ := realCache.ComputeFileChecksum(destFile)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "api-constants",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: wholeFileHash},
			Positions: []types.PositionLock{{
				From:       "api/constants.go:L4-L6",
				To:         destFile + ":L10-L12",
				SourceHash: sourceHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 1 verified for whole-file hash, 1 verified for position hash
	if result.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified (whole + position), got %d", result.Summary.Verified)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}

	// Verify type fields are set correctly
	fileFound, posFound := false, false
	for _, f := range result.Files {
		switch f.Type {
		case "file":
			fileFound = true
		case "position":
			posFound = true
			if f.Position == nil {
				t.Error("Expected Position detail to be set for position-type entry")
			} else {
				if f.Position.From != "api/constants.go:L4-L6" {
					t.Errorf("Expected Position.From 'api/constants.go:L4-L6', got '%s'", f.Position.From)
				}
			}
		}
	}
	if !fileFound {
		t.Error("Expected a file-type entry in results")
	}
	if !posFound {
		t.Error("Expected a position-type entry in results")
	}
}

func TestVerify_PositionExtraction_Modified(t *testing.T) {
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "config.ts")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}

	// Write file with original content
	original := "line1\nline2\nvendored-a\nvendored-b\nline5\n"
	if err := os.WriteFile(destFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute source hash from the original vendored content
	_, sourceHash, err := ExtractPosition(destFile, &types.PositionSpec{StartLine: 3, EndLine: 4})
	if err != nil {
		t.Fatal(err)
	}
	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeFileHash, _ := realCache.ComputeFileChecksum(destFile)

	// Now modify ONLY the vendored lines (position 3-4)
	modified := "line1\nline2\nMODIFIED-a\nMODIFIED-b\nline5\n"
	if err := os.WriteFile(destFile, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/a.go:L3-L4", To: destFile + ":L3-L4"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: wholeFileHash}, // old hash
			Positions: []types.PositionLock{{
				From:       "src/a.go:L3-L4",
				To:         destFile + ":L3-L4",
				SourceHash: sourceHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}
	// Both whole-file and position should be modified
	if result.Summary.Modified < 1 {
		t.Errorf("Expected at least 1 modified, got %d", result.Summary.Modified)
	}

	// Find the position-level entry
	found := false
	for _, f := range result.Files {
		if f.Path == destFile+":L3-L4" && f.Status == "modified" {
			found = true
			if f.Type != "position" {
				t.Errorf("Expected type 'position' for position entry, got '%s'", f.Type)
			}
			if f.Position == nil {
				t.Error("Expected Position detail for position-type modified entry")
			}
			break
		}
	}
	if !found {
		t.Error("Expected a position-level 'modified' entry for destFile:L3-L4")
	}
}

func TestVerify_PositionExtraction_WholeFileDest(t *testing.T) {
	// Position extraction to whole file (no dest position) — verify by whole-file hash
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "snippet.go")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFile, []byte("extracted content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeHash, _ := realCache.ComputeFileChecksum(destFile)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/a.go:L5-L10", To: destFile}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: wholeHash},
			Positions: []types.PositionLock{{
				From:       "src/a.go:L5-L10",
				To:         destFile,
				SourceHash: "sha256:" + wholeHash, // ExtractPosition always uses "sha256:" prefix
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Both whole-file and position-level should verify
	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}
	if result.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified, got %d", result.Summary.Verified)
	}

	// Verify type fields
	for _, f := range result.Files {
		if f.Type != "file" && f.Type != "position" {
			t.Errorf("Unexpected type '%s'", f.Type)
		}
	}
}

func TestVerify_PositionExtraction_DeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "lib", "missing.go")
	// Don't create the file — simulate a deleted destination

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/a.go:L5-L10", To: destFile + ":L2-L5"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: "somehash"},
			Positions: []types.PositionLock{{
				From:       "src/a.go:L5-L10",
				To:         destFile + ":L2-L5",
				SourceHash: "sha256:abc",
			}},
		}},
	}, nil)

	// Whole-file hash check finds the file deleted
	// cache.ComputeFileChecksum returns os.ErrNotExist (file not in cache.files)

	// Mock fs.Stat for findAddedFiles
	fs.EXPECT().Stat(destFile).Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}
	// Should have 2 deleted entries: whole-file + position
	if result.Summary.Deleted < 2 {
		t.Errorf("Expected at least 2 deleted (whole + position), got %d", result.Summary.Deleted)
	}

	// Verify both types are represented
	hasFile, hasPos := false, false
	for _, f := range result.Files {
		if f.Type == "file" {
			hasFile = true
		}
		if f.Type == "position" {
			hasPos = true
			if f.Position == nil {
				t.Error("Expected Position detail on position-type deleted entry")
			}
		}
	}
	if !hasFile {
		t.Error("Expected a file-type deleted entry")
	}
	if !hasPos {
		t.Error("Expected a position-type deleted entry")
	}
}

// ============================================================================
// Empty Lockfile Tests
// ============================================================================

func TestVerify_EmptyLockfile_NoCacheFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}},
			}},
		}},
	}, nil)

	// Empty lockfile — no FileHashes, no Positions
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")

	// No cache available either → should error
	_, err := service.Verify(context.Background())
	if err == nil {
		t.Fatal("Expected error for empty lockfile with no cache, got nil")
	}
	if !contains(err.Error(), "no file hashes") {
		t.Errorf("Expected 'no file hashes' error, got: %v", err)
	}
}

func TestVerify_EmptyLockfile_WithCacheFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Setup cache with files
	cache.files["lib/file.go"] = "hash123"
	cache.caches["test-vendor@main"] = types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123",
		Files: []types.FileChecksum{
			{Path: "lib/file.go", Hash: "hash123"},
		},
	}

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
		}},
	}, nil)

	fs.EXPECT().Stat("lib/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}
	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified, got %d", result.Summary.Verified)
	}
}

// ============================================================================
// Mixed Position and Whole-File Tests
// ============================================================================

func TestVerify_MixedPositionAndWholeFile_SameVendor(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two destination files
	wholeFile := filepath.Join(tmpDir, "lib", "whole.go")
	posFile := filepath.Join(tmpDir, "lib", "partial.go")
	if err := os.MkdirAll(filepath.Dir(wholeFile), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(wholeFile, []byte("whole file content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Position file with 5 lines: lines 3-4 are vendored
	posContent := "line1\nline2\nvendored-a\nvendored-b\nline5\n"
	if err := os.WriteFile(posFile, []byte(posContent), 0644); err != nil {
		t.Fatal(err)
	}

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeHash, _ := realCache.ComputeFileChecksum(wholeFile)
	posFileHash, _ := realCache.ComputeFileChecksum(posFile)
	_, posSourceHash, _ := ExtractPosition(posFile, &types.PositionSpec{StartLine: 3, EndLine: 4})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "multi-type-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/whole.go", To: wholeFile},
					{From: "src/partial.go:L10-L11", To: posFile + ":L3-L4"},
				},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "multi-type-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{
				wholeFile: wholeHash,
				posFile:   posFileHash,
			},
			Positions: []types.PositionLock{{
				From:       "src/partial.go:L10-L11",
				To:         posFile + ":L3-L4",
				SourceHash: posSourceHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}
	// 2 whole-file verified + 1 position verified = 3
	if result.Summary.Verified != 3 {
		t.Errorf("Expected 3 verified (2 whole-file + 1 position), got %d", result.Summary.Verified)
	}

	// Verify type breakdown
	fileCount, posCount := 0, 0
	for _, f := range result.Files {
		switch f.Type {
		case "file":
			fileCount++
		case "position":
			posCount++
		}
	}
	if fileCount != 2 {
		t.Errorf("Expected 2 file-type entries, got %d", fileCount)
	}
	if posCount != 1 {
		t.Errorf("Expected 1 position-type entry, got %d", posCount)
	}
}

func TestVerify_PositionDriftAtTargetRange(t *testing.T) {
	// Test: position content has drifted (modified) while whole file hash also changed
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "config.go")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}

	// Write original content, compute hashes
	original := "line1\nline2\noriginal-vendored\nline4\nline5\n"
	if err := os.WriteFile(destFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	originalWholeHash, _ := realCache.ComputeFileChecksum(destFile)
	_, originalPosHash, _ := ExtractPosition(destFile, &types.PositionSpec{StartLine: 3, EndLine: 3})

	// Now modify only the vendored line
	modified := "line1\nline2\nMODIFIED-vendored\nline4\nline5\n"
	if err := os.WriteFile(destFile, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/config.go:L3", To: destFile + ":L3"},
				},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: originalWholeHash},
			Positions: []types.PositionLock{{
				From:       "src/config.go:L3",
				To:         destFile + ":L3",
				SourceHash: originalPosHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	// Both whole-file and position should show modified
	if result.Summary.Modified < 2 {
		t.Errorf("Expected at least 2 modified (whole-file + position), got %d", result.Summary.Modified)
	}

	// Find position-type modified entry
	posModified := false
	for _, f := range result.Files {
		if f.Type == "position" && f.Status == "modified" {
			posModified = true
			if f.Position == nil {
				t.Error("Expected Position detail on modified position entry")
			}
		}
	}
	if !posModified {
		t.Error("Expected a position-type modified entry")
	}
}

func TestVerify_MultipleVendors_MixedResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Vendor A: file verified
	cache.files["lib/vendor-a/file.go"] = "hashA"
	// Vendor B: file modified (different hash)
	cache.files["lib/vendor-b/file.go"] = "hashB-modified"
	// Vendor C: file deleted (not in cache.files)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor-a", URL: "https://github.com/owner/a", Specs: []types.BranchSpec{{
				Ref: "main", Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/vendor-a/file.go"}},
			}}},
			{Name: "vendor-b", URL: "https://github.com/owner/b", Specs: []types.BranchSpec{{
				Ref: "main", Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/vendor-b/file.go"}},
			}}},
			{Name: "vendor-c", URL: "https://github.com/owner/c", Specs: []types.BranchSpec{{
				Ref: "main", Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/vendor-c/file.go"}},
			}}},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa", FileHashes: map[string]string{"lib/vendor-a/file.go": "hashA"}},
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb", FileHashes: map[string]string{"lib/vendor-b/file.go": "hashB-original"}},
			{Name: "vendor-c", Ref: "main", CommitHash: "ccc", FileHashes: map[string]string{"lib/vendor-c/file.go": "hashC"}},
		},
	}, nil)

	// fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/vendor-a/file.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/vendor-b/file.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/vendor-c/file.go").Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}
	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified, got %d", result.Summary.Verified)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified, got %d", result.Summary.Modified)
	}
	if result.Summary.Deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", result.Summary.Deleted)
	}
	if result.Summary.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", result.Summary.TotalFiles)
	}
}

func TestVerify_CacheFallback_StaleCache(t *testing.T) {
	// Cache commit hash doesn't match lockfile → should be skipped
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Cache has a different commit hash than lockfile
	cache.caches["test-vendor@main"] = types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "old-stale-hash", // Different from lockfile
		Files: []types.FileChecksum{
			{Path: "lib/file.go", Hash: "hash123"},
		},
	}

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "new-current-hash", // Different from cache
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	_, err := service.Verify(context.Background())

	// Should error because no valid hashes available
	if err == nil {
		t.Fatal("Expected error for stale cache, got nil")
	}
	if !contains(err.Error(), "no") {
		t.Errorf("Expected 'no cached file hashes' error, got: %v", err)
	}
}

// joinLines joins strings with newlines and adds trailing newline
func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}

// ============================================================================
// Verify Position Edge Cases
// ============================================================================

func TestVerify_PositionExtraction_FileShrunk(t *testing.T) {
	// File had 15 lines at sync time; position L10-L12 was valid.
	// File has since shrunk to 5 lines. Extraction should fail → "modified" status
	// with error string in ActualHash.
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "shrunk.go")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}

	// Write 15-line file, compute hash at L10-L12
	var bigLines []string
	for i := 1; i <= 15; i++ {
		bigLines = append(bigLines, fmt.Sprintf("line-%d", i))
	}
	if err := os.WriteFile(destFile, []byte(joinLines(bigLines)), 0644); err != nil {
		t.Fatal(err)
	}
	_, sourceHash, err := ExtractPosition(destFile, &types.PositionSpec{StartLine: 10, EndLine: 12})
	if err != nil {
		t.Fatal(err)
	}

	// Shrink file to 5 lines
	var smallLines []string
	for i := 1; i <= 5; i++ {
		smallLines = append(smallLines, fmt.Sprintf("line-%d", i))
	}
	if err := os.WriteFile(destFile, []byte(joinLines(smallLines)), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "shrunk-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/a.go:L10-L12", To: destFile + ":L10-L12"}},
			}},
		}},
	}, nil)

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeFileHash, _ := realCache.ComputeFileChecksum(destFile)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "shrunk-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: wholeFileHash},
			Positions: []types.PositionLock{{
				From:       "src/a.go:L10-L12",
				To:         destFile + ":L10-L12",
				SourceHash: sourceHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	// Position entry should be "modified" with error in ActualHash
	var posEntry *types.FileStatus
	for i := range result.Files {
		if result.Files[i].Type == "position" {
			posEntry = &result.Files[i]
			break
		}
	}
	if posEntry == nil {
		t.Fatal("Expected a position-type entry in results")
	}
	if posEntry.Status != "modified" {
		t.Errorf("Expected status 'modified' for out-of-range position, got %q", posEntry.Status)
	}
	// ActualHash should contain the extraction error, not a hash
	if posEntry.ActualHash == nil {
		t.Fatal("Expected non-nil ActualHash with error string")
	}
	if !contains(*posEntry.ActualHash, "does not exist") {
		t.Errorf("Expected ActualHash to contain extraction error, got %q", *posEntry.ActualHash)
	}
}

func TestVerify_PositionExtraction_MixedResults(t *testing.T) {
	// Whole-file hash matches but position content has drifted.
	// Verifies that position and whole-file produce independent results.
	tmpDir := t.TempDir()

	destFile := filepath.Join(tmpDir, "lib", "mixed.go")
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		t.Fatal(err)
	}

	// Write original content; record position hash at L3-L4
	original := "line1\nline2\nvendored-a\nvendored-b\nline5\n"
	if err := os.WriteFile(destFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeFileHash, _ := realCache.ComputeFileChecksum(destFile)
	_, posHash, err := ExtractPosition(destFile, &types.PositionSpec{StartLine: 3, EndLine: 4})
	if err != nil {
		t.Fatal(err)
	}

	// Modify only lines 3-4 (position changes, whole-file hash will also change)
	modified := "line1\nline2\nMODIFIED-a\nMODIFIED-b\nline5\n"
	if err := os.WriteFile(destFile, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "mixed-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/a.go:L3-L4", To: destFile + ":L3-L4"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "mixed-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destFile: wholeFileHash},
			Positions: []types.PositionLock{{
				From:       "src/a.go:L3-L4",
				To:         destFile + ":L3-L4",
				SourceHash: posHash,
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	// Both whole-file and position should independently report modified
	fileModified, posModified := false, false
	for _, f := range result.Files {
		if f.Type == "file" && f.Status == "modified" {
			fileModified = true
		}
		if f.Type == "position" && f.Status == "modified" {
			posModified = true
		}
	}
	if !fileModified {
		t.Error("Expected whole-file entry to be 'modified'")
	}
	if !posModified {
		t.Error("Expected position entry to be 'modified'")
	}
	if result.Summary.Modified != 2 {
		t.Errorf("Expected 2 modified (file + position), got %d", result.Summary.Modified)
	}
}

func TestVerify_PositionExtraction_MultipleVendorsWithPositions(t *testing.T) {
	// Two vendors, each with a position lock entry, targeting different files.
	tmpDir := t.TempDir()

	destA := filepath.Join(tmpDir, "lib", "a.go")
	destB := filepath.Join(tmpDir, "lib", "b.go")
	if err := os.MkdirAll(filepath.Dir(destA), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(destA, []byte("a1\na2\na3\na4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destB, []byte("b1\nb2\nb3\nb4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, hashA, _ := ExtractPosition(destA, &types.PositionSpec{StartLine: 2, EndLine: 3})
	_, hashB, _ := ExtractPosition(destB, &types.PositionSpec{StartLine: 1, EndLine: 2})

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	wholeA, _ := realCache.ComputeFileChecksum(destA)
	wholeB, _ := realCache.ComputeFileChecksum(destB)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor-a", URL: "https://github.com/a/repo", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "x:L2-L3", To: destA + ":L2-L3"}}}}},
			{Name: "vendor-b", URL: "https://github.com/b/repo", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "y:L1-L2", To: destB + ":L1-L2"}}}}},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa", FileHashes: map[string]string{destA: wholeA}, Positions: []types.PositionLock{{From: "x:L2-L3", To: destA + ":L2-L3", SourceHash: hashA}}},
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb", FileHashes: map[string]string{destB: wholeB}, Positions: []types.PositionLock{{From: "y:L1-L2", To: destB + ":L1-L2", SourceHash: hashB}}},
		},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS, got %s", result.Summary.Result)
	}
	// 2 whole-file + 2 position = 4 verified
	if result.Summary.Verified != 4 {
		t.Errorf("Expected 4 verified, got %d", result.Summary.Verified)
	}

	// Verify vendor names are tracked on position entries
	vendorNames := make(map[string]bool)
	for _, f := range result.Files {
		if f.Type == "position" && f.Vendor != nil {
			vendorNames[*f.Vendor] = true
		}
	}
	if !vendorNames["vendor-a"] || !vendorNames["vendor-b"] {
		t.Errorf("Expected position entries for both vendors, got %v", vendorNames)
	}
}

func TestVerify_PositionExtraction_EmptyToProducesDeleted(t *testing.T) {
	// Position lock with empty To path: ParsePathPosition("") returns ("", nil, nil)
	// which is not an error. The code proceeds to ComputeFileChecksum("") which
	// returns os.ErrNotExist → "deleted" position entry.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/file.go"] = "hash1"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/file.go"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "test-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{"lib/file.go": "hash1"},
			Positions: []types.PositionLock{{
				From:       "src/types.go:L5-L10",
				To:         "", // Empty To — empty path does not exist as a file
				SourceHash: "sha256:abc",
			}},
		}},
	}, nil)

	fs.EXPECT().Stat("lib/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Whole-file verified + position "deleted" (empty path not found) → FAIL
	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL (position entry deleted), got %s", result.Summary.Result)
	}
	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified (whole-file only), got %d", result.Summary.Verified)
	}
	if result.Summary.Deleted != 1 {
		t.Errorf("Expected 1 deleted (empty-path position), got %d", result.Summary.Deleted)
	}
}

// ============================================================================
// Config/Lock Coherence Tests (VFY-001)
// ============================================================================

func TestVerify_StaleMapping(t *testing.T) {
	// Config has a mapping destination that does not appear in lock FileHashes.
	// This means the mapping was added to config but never synced.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// One file is synced (in both config and lock)
	cache.files["lib/test-vendor/file.go"] = "abc123hash"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
							{From: "src/new.go", To: "lib/test-vendor/new.go"}, // stale: not in lock
						},
					},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "abc123hash",
					// "lib/test-vendor/new.go" is NOT here — stale
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/test-vendor/new.go").Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("Expected WARN for stale mapping, got %s", result.Summary.Result)
	}

	if result.Summary.Stale != 1 {
		t.Errorf("Expected 1 stale, got %d", result.Summary.Stale)
	}

	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified, got %d", result.Summary.Verified)
	}

	// Verify stale entry details
	found := false
	for _, f := range result.Files {
		if f.Status == "stale" {
			found = true
			if f.Path != "lib/test-vendor/new.go" {
				t.Errorf("Expected stale path 'lib/test-vendor/new.go', got '%s'", f.Path)
			}
			if f.Type != "coherence" {
				t.Errorf("Expected type 'coherence', got '%s'", f.Type)
			}
			if f.Vendor == nil || *f.Vendor != "test-vendor" {
				t.Errorf("Expected vendor 'test-vendor' on stale entry")
			}
		}
	}
	if !found {
		t.Error("Expected a stale entry in results")
	}
}

func TestVerify_OrphanedLockEntry(t *testing.T) {
	// Lock has a FileHashes entry for a path not referenced by any config mapping.
	// This means the mapping was removed from config but lock was not regenerated.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/test-vendor/file.go"] = "abc123hash"
	cache.files["lib/test-vendor/old.go"] = "oldhash"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/file.go", To: "lib/test-vendor/file.go"},
							// "lib/test-vendor/old.go" is NOT in config anymore
						},
					},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123def",
				FileHashes: map[string]string{
					"lib/test-vendor/file.go": "abc123hash",
					"lib/test-vendor/old.go":  "oldhash", // orphaned: not in config
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/test-vendor/file.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("Expected WARN for orphaned lock entry, got %s", result.Summary.Result)
	}

	if result.Summary.Orphaned != 1 {
		t.Errorf("Expected 1 orphaned, got %d", result.Summary.Orphaned)
	}

	if result.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified (both files exist with correct hash), got %d", result.Summary.Verified)
	}

	// Verify orphaned entry details
	found := false
	for _, f := range result.Files {
		if f.Status == "orphaned" {
			found = true
			if f.Path != "lib/test-vendor/old.go" {
				t.Errorf("Expected orphaned path 'lib/test-vendor/old.go', got '%s'", f.Path)
			}
			if f.Type != "coherence" {
				t.Errorf("Expected type 'coherence', got '%s'", f.Type)
			}
			if f.Vendor == nil || *f.Vendor != "test-vendor" {
				t.Errorf("Expected vendor 'test-vendor' on orphaned entry")
			}
		}
	}
	if !found {
		t.Error("Expected an orphaned entry in results")
	}
}

func TestVerify_CoherenceClean(t *testing.T) {
	// Config and lock are perfectly aligned — no stale or orphaned entries.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/vendor-a/file.go"] = "hashA"
	cache.files["lib/vendor-b/util.go"] = "hashB"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor-a", URL: "https://github.com/a/repo", Specs: []types.BranchSpec{{
				Ref: "main", Mapping: []types.PathMapping{{From: "src/file.go", To: "lib/vendor-a/file.go"}},
			}}},
			{Name: "vendor-b", URL: "https://github.com/b/repo", Specs: []types.BranchSpec{{
				Ref: "main", Mapping: []types.PathMapping{{From: "src/util.go", To: "lib/vendor-b/util.go"}},
			}}},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa", FileHashes: map[string]string{"lib/vendor-a/file.go": "hashA"}},
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb", FileHashes: map[string]string{"lib/vendor-b/util.go": "hashB"}},
		},
	}, nil)

	fs.EXPECT().Stat("lib/vendor-a/file.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/vendor-b/util.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS for coherent config/lock, got %s", result.Summary.Result)
	}

	if result.Summary.Stale != 0 {
		t.Errorf("Expected 0 stale, got %d", result.Summary.Stale)
	}

	if result.Summary.Orphaned != 0 {
		t.Errorf("Expected 0 orphaned, got %d", result.Summary.Orphaned)
	}

	if result.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified, got %d", result.Summary.Verified)
	}
}

func TestVerify_CoherenceWithPositions(t *testing.T) {
	// Config has a position-spec destination (e.g., "lib/config.go:L5-L10").
	// The position spec should be stripped before comparing against lock FileHashes
	// which use bare file paths.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/config.go"] = "confighash"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							// Position spec on destination: should strip ":L5-L10" for coherence check
							{From: "src/constants.go:L1-L5", To: "lib/config.go:L5-L10"},
						},
					},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{
					"lib/config.go": "confighash", // bare path matches after stripping position spec
				},
				Positions: []types.PositionLock{{
					From:       "src/constants.go:L1-L5",
					To:         "lib/config.go:L5-L10",
					SourceHash: "sha256:abc",
				}},
			},
		},
	}, nil)

	// fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/config.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// No stale or orphaned — position spec stripped correctly
	if result.Summary.Stale != 0 {
		t.Errorf("Expected 0 stale (position spec stripped), got %d", result.Summary.Stale)
	}

	if result.Summary.Orphaned != 0 {
		t.Errorf("Expected 0 orphaned, got %d", result.Summary.Orphaned)
	}
}

func TestVerify_CoherenceBothStaleAndOrphaned(t *testing.T) {
	// Config has a mapping not in lock (stale) AND lock has an entry not in config (orphaned).
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/vendor/common.go"] = "commonhash"
	cache.files["lib/vendor/removed.go"] = "removedhash"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "my-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{{
					Ref: "main",
					Mapping: []types.PathMapping{
						{From: "src/common.go", To: "lib/vendor/common.go"},
						{From: "src/brand-new.go", To: "lib/vendor/brand-new.go"}, // stale
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "my-vendor",
				Ref:        "main",
				CommitHash: "aaa",
				FileHashes: map[string]string{
					"lib/vendor/common.go":  "commonhash",
					"lib/vendor/removed.go": "removedhash", // orphaned
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/vendor/common.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/vendor/brand-new.go").Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "WARN" {
		t.Errorf("Expected WARN, got %s", result.Summary.Result)
	}

	if result.Summary.Stale != 1 {
		t.Errorf("Expected 1 stale, got %d", result.Summary.Stale)
	}

	if result.Summary.Orphaned != 1 {
		t.Errorf("Expected 1 orphaned, got %d", result.Summary.Orphaned)
	}

	if result.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified, got %d", result.Summary.Verified)
	}
}

func TestVerify_CoherenceJSON(t *testing.T) {
	// Verify stale/orphaned fields appear in JSON output
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/v/file.go"] = "hash1"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "v", URL: "https://github.com/o/r",
			Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{
				{From: "src/file.go", To: "lib/v/file.go"},
				{From: "src/new.go", To: "lib/v/new.go"}, // stale
			}}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name: "v", Ref: "main", CommitHash: "abc",
			FileHashes: map[string]string{
				"lib/v/file.go": "hash1",
				"lib/v/old.go":  "hash2", // orphaned
			},
		}},
	}, nil)

	fs.EXPECT().Stat("lib/v/file.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/v/new.go").Return(nil, os.ErrNotExist)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	var parsed types.VerifyResult
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed.Summary.Stale != 1 {
		t.Errorf("Expected stale=1 in JSON roundtrip, got %d", parsed.Summary.Stale)
	}

	if parsed.Summary.Orphaned != 1 {
		t.Errorf("Expected orphaned=1 in JSON roundtrip, got %d", parsed.Summary.Orphaned)
	}

	// Verify coherence-type entries exist in Files
	staleFound, orphanedFound := false, false
	for _, f := range parsed.Files {
		if f.Status == "stale" && f.Type == "coherence" {
			staleFound = true
		}
		if f.Status == "orphaned" && f.Type == "coherence" {
			orphanedFound = true
		}
	}
	if !staleFound {
		t.Error("Expected a stale coherence entry in JSON output")
	}
	if !orphanedFound {
		t.Error("Expected an orphaned coherence entry in JSON output")
	}
}

// ============================================================================
// Internal Vendor Drift Tests (Spec 070)
// ============================================================================

func TestVerify_InternalVendor_SourceDrift(t *testing.T) {
	// Source file changed since last sync but destination still matches lock.
	// verifyInternalEntries should report DriftSourceDrift direction.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Destination file matches lock hash (no drift at dest)
	cache.files["lib/internal/config.go"] = "dest-hash-locked"
	// Source file has CHANGED since sync (different from locked source hash)
	cache.files["src/config.go"] = "source-hash-NEW"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-cfg",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: "src/config.go", To: "lib/internal/config.go"},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "internal-cfg",
				Ref:        RefLocal,
				CommitHash: "content-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					"lib/internal/config.go": "dest-hash-locked",
				},
				SourceFileHashes: map[string]string{
					"src/config.go": "source-hash-OLD", // different from current
				},
			},
		},
	}, nil)

	// fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/internal/config.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Whole-file should be verified (dest matches lock)
	if result.Summary.Verified != 1 {
		t.Errorf("Expected 1 verified file, got %d", result.Summary.Verified)
	}

	// Should have internal status entries
	if len(result.InternalStatus) == 0 {
		t.Fatal("Expected InternalStatus entries for internal vendor")
	}

	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftSourceDrift {
		t.Errorf("Expected direction %q, got %q", types.DriftSourceDrift, entry.Direction)
	}
	if entry.VendorName != "internal-cfg" {
		t.Errorf("Expected vendor name 'internal-cfg', got %q", entry.VendorName)
	}
	if entry.FromPath != "src/config.go" {
		t.Errorf("Expected from path 'src/config.go', got %q", entry.FromPath)
	}
	if entry.ToPath != "lib/internal/config.go" {
		t.Errorf("Expected to path 'lib/internal/config.go', got %q", entry.ToPath)
	}
	if entry.Action != "propagate source → dest" {
		t.Errorf("Expected action 'propagate source → dest', got %q", entry.Action)
	}
	if entry.Compliance != ComplianceSourceCanonical {
		t.Errorf("Expected compliance %q, got %q", ComplianceSourceCanonical, entry.Compliance)
	}
}

func TestVerify_InternalVendor_DestDrift(t *testing.T) {
	// Destination file changed since last sync but source still matches lock.
	// verifyInternalEntries should report DriftDestDrift direction.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Destination file has CHANGED (different from locked dest hash)
	cache.files["lib/internal/util.go"] = "dest-hash-NEW"
	// Source file still matches lock
	cache.files["src/util.go"] = "source-hash-locked"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-util",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: "src/util.go", To: "lib/internal/util.go"},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "internal-util",
				Ref:        RefLocal,
				CommitHash: "content-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					"lib/internal/util.go": "dest-hash-locked", // different from current
				},
				SourceFileHashes: map[string]string{
					"src/util.go": "source-hash-locked",
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/internal/util.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Whole-file check: dest hash changed → modified
	if result.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified file (dest hash changed), got %d", result.Summary.Modified)
	}

	// Internal status should show dest drift
	if len(result.InternalStatus) == 0 {
		t.Fatal("Expected InternalStatus entries for internal vendor")
	}

	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftDestDrift {
		t.Errorf("Expected direction %q, got %q", types.DriftDestDrift, entry.Direction)
	}
	if entry.Action != "warning: dest modified (source-canonical)" {
		t.Errorf("Expected source-canonical warning action, got %q", entry.Action)
	}
	if entry.SourceHashCurrent != "source-hash-locked" {
		t.Errorf("Expected current source hash 'source-hash-locked', got %q", entry.SourceHashCurrent)
	}
	if entry.DestHashCurrent != "dest-hash-NEW" {
		t.Errorf("Expected current dest hash 'dest-hash-NEW', got %q", entry.DestHashCurrent)
	}
}

func TestVerify_InternalVendor_DestDrift_Bidirectional(t *testing.T) {
	// Same as dest drift but with bidirectional compliance.
	// Action should suggest propagating dest back to source.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/internal/shared.go"] = "dest-hash-NEW"
	cache.files["src/shared.go"] = "source-hash-locked"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-bidir",
				Source:     SourceInternal,
				Compliance: ComplianceBidirectional,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: "src/shared.go", To: "lib/internal/shared.go"},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "internal-bidir",
				Ref:        RefLocal,
				CommitHash: "content-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					"lib/internal/shared.go": "dest-hash-locked",
				},
				SourceFileHashes: map[string]string{
					"src/shared.go": "source-hash-locked",
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/internal/shared.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.InternalStatus) == 0 {
		t.Fatal("Expected InternalStatus entries")
	}

	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftDestDrift {
		t.Errorf("Expected direction %q, got %q", types.DriftDestDrift, entry.Direction)
	}
	if entry.Action != "propagate dest → source" {
		t.Errorf("Expected bidirectional propagation action, got %q", entry.Action)
	}
	if entry.Compliance != ComplianceBidirectional {
		t.Errorf("Expected compliance %q, got %q", ComplianceBidirectional, entry.Compliance)
	}
}

func TestVerify_InternalVendor_Synced(t *testing.T) {
	// Both source and destination match their locked hashes — synced state.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	cache.files["lib/internal/clean.go"] = "dest-hash"
	cache.files["src/clean.go"] = "source-hash"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:   "internal-clean",
				Source: SourceInternal,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: "src/clean.go", To: "lib/internal/clean.go"},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "internal-clean",
				Ref:        RefLocal,
				CommitHash: "content-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					"lib/internal/clean.go": "dest-hash",
				},
				SourceFileHashes: map[string]string{
					"src/clean.go": "source-hash",
				},
			},
		},
	}, nil)

	fs.EXPECT().Stat("lib/internal/clean.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.Result != "PASS" {
		t.Errorf("Expected PASS for synced internal vendor, got %s", result.Summary.Result)
	}

	if len(result.InternalStatus) == 0 {
		t.Fatal("Expected InternalStatus entries")
	}

	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftSynced {
		t.Errorf("Expected direction %q, got %q", types.DriftSynced, entry.Direction)
	}
	if entry.Action != "none" {
		t.Errorf("Expected action 'none', got %q", entry.Action)
	}
}

// ============================================================================
// Comprehensive Mixed Discrepancy Test (VFY-002)
// ============================================================================

func TestVerify_Integration_AllDiscrepancyTypes(t *testing.T) {
	// Single verify run producing every discrepancy type:
	// - verified file
	// - modified file
	// - deleted file
	// - added file (requires real filesystem for WalkDir)
	// - stale config entry
	// - orphaned lock entry
	// - internal vendor drift
	//
	// This test uses a real filesystem for the added-file detection
	// while mocking config/lock stores.
	tmpDir := t.TempDir()

	// Create vendor directories and files
	vendorDir := filepath.Join(tmpDir, "lib", "multi")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Verified file: matches lock hash exactly
	verifiedFile := filepath.Join(vendorDir, "verified.go")
	if err := os.WriteFile(verifiedFile, []byte("package verified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modified file: exists but content differs from lock hash
	modifiedFile := filepath.Join(vendorDir, "modified.go")
	if err := os.WriteFile(modifiedFile, []byte("package modified-NEW\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Added file: exists on disk but NOT in lock FileHashes
	addedFile := filepath.Join(vendorDir, "added.go")
	if err := os.WriteFile(addedFile, []byte("package added\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Internal vendor source file (source drifted)
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	internalSrcFile := filepath.Join(srcDir, "internal.go")
	if err := os.WriteFile(internalSrcFile, []byte("package internal-NEW\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Internal vendor dest file
	internalDestDir := filepath.Join(tmpDir, "lib", "internal")
	if err := os.MkdirAll(internalDestDir, 0755); err != nil {
		t.Fatal(err)
	}
	internalDestFile := filepath.Join(internalDestDir, "internal.go")
	if err := os.WriteFile(internalDestFile, []byte("package internal-dest\n"), 0644); err != nil {
		t.Fatal(err)
	}

	realCache := NewFileCacheStore(NewOSFileSystem(), tmpDir)
	verifiedHash, _ := realCache.ComputeFileChecksum(verifiedFile)
	internalDestHash, _ := realCache.ComputeFileChecksum(internalDestFile)

	// Deleted file: in lock FileHashes but NOT on disk
	deletedFile := filepath.Join(vendorDir, "deleted.go")
	// (not created — simulates deletion)

	// Orphaned file: in lock FileHashes but NOT in config mapping.
	// Must exist on disk so the whole-file check counts it as "verified"
	// (not double-counted as "deleted").
	orphanedFile := filepath.Join(vendorDir, "orphaned.go")
	if err := os.WriteFile(orphanedFile, []byte("package orphaned\n"), 0644); err != nil {
		t.Fatal(err)
	}
	orphanedHash, _ := realCache.ComputeFileChecksum(orphanedFile)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "multi-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{{
					Ref: "main",
					Mapping: []types.PathMapping{
						{From: "src/verified.go", To: verifiedFile},
						{From: "src/modified.go", To: modifiedFile},
						{From: "src/deleted.go", To: deletedFile},
						{From: "src/stale.go", To: filepath.Join(vendorDir, "stale.go")}, // stale: not in lock
					},
				}},
			},
			{
				Name:   "internal-vendor",
				Source: SourceInternal,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: internalSrcFile, To: internalDestFile},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "multi-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{
					verifiedFile: verifiedHash,
					modifiedFile: "old-modified-hash", // mismatches current
					deletedFile:  "deleted-file-hash", // file doesn't exist
					orphanedFile: orphanedHash,        // orphaned: not in config but file exists
				},
			},
			{
				Name:       "internal-vendor",
				Ref:        RefLocal,
				CommitHash: "internal-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					internalDestFile: internalDestHash,
				},
				SourceFileHashes: map[string]string{
					internalSrcFile: "old-source-hash", // source has drifted
				},
			},
		},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify aggregate counts.
	// Verified: verifiedFile + orphanedFile (hash matches) + internalDestFile = 3
	// The orphanedFile is verified by the whole-file check AND flagged as orphaned
	// by coherence detection. Both counts increment independently.
	if result.Summary.Verified != 3 {
		t.Errorf("Expected 3 verified (verified + orphaned-on-disk + internal-dest), got %d", result.Summary.Verified)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified, got %d", result.Summary.Modified)
	}
	if result.Summary.Deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", result.Summary.Deleted)
	}
	if result.Summary.Added != 1 {
		t.Errorf("Expected 1 added, got %d", result.Summary.Added)
	}
	if result.Summary.Stale != 1 {
		t.Errorf("Expected 1 stale, got %d", result.Summary.Stale)
	}
	if result.Summary.Orphaned != 1 {
		t.Errorf("Expected 1 orphaned, got %d", result.Summary.Orphaned)
	}

	// Overall result: FAIL (modified + deleted present)
	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL, got %s", result.Summary.Result)
	}

	// TotalFiles should count all file-level entries (not InternalStatus)
	expectedTotal := result.Summary.Verified + result.Summary.Modified +
		result.Summary.Deleted + result.Summary.Added +
		result.Summary.Stale + result.Summary.Orphaned
	if result.Summary.TotalFiles != expectedTotal {
		t.Errorf("Expected TotalFiles=%d, got %d", expectedTotal, result.Summary.TotalFiles)
	}

	// Internal drift should be present
	if len(result.InternalStatus) == 0 {
		t.Error("Expected InternalStatus entries for internal vendor drift")
	} else if result.InternalStatus[0].Direction != types.DriftSourceDrift {
		t.Errorf("Expected source drift for internal vendor, got %q", result.InternalStatus[0].Direction)
	}

	// Verify all status types are present in Files
	statusCounts := make(map[string]int)
	for _, f := range result.Files {
		statusCounts[f.Status]++
	}
	for _, expected := range []string{"verified", "modified", "deleted", "added", "stale", "orphaned"} {
		if statusCounts[expected] == 0 {
			t.Errorf("Expected at least one %q entry in Files", expected)
		}
	}
}

// ============================================================================
// Comprehensive JSON Output Test (VFY-002)
// ============================================================================

func TestVerify_Integration_JSONOutput_AllFields(t *testing.T) {
	// Verifies that JSON output contains all expected fields including
	// stale/orphaned counts, internal_status, schema_version, and timestamp.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Verified file
	cache.files["lib/v/ok.go"] = "hash-ok"
	// Modified file
	cache.files["lib/v/changed.go"] = "hash-changed-NEW"
	// Internal vendor files
	cache.files["lib/int/dest.go"] = "int-dest-hash"
	cache.files["src/int.go"] = "int-src-hash-NEW"

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "ext-vendor",
				URL:  "https://github.com/o/r",
				Specs: []types.BranchSpec{{
					Ref: "main",
					Mapping: []types.PathMapping{
						{From: "src/ok.go", To: "lib/v/ok.go"},
						{From: "src/changed.go", To: "lib/v/changed.go"},
						{From: "src/stale.go", To: "lib/v/stale.go"}, // stale
					},
				}},
			},
			{
				Name:   "int-vendor",
				Source: SourceInternal,
				Specs: []types.BranchSpec{{
					Ref: RefLocal,
					Mapping: []types.PathMapping{
						{From: "src/int.go", To: "lib/int/dest.go"},
					},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "ext-vendor",
				Ref:        "main",
				CommitHash: "abc",
				FileHashes: map[string]string{
					"lib/v/ok.go":      "hash-ok",
					"lib/v/changed.go": "hash-changed-OLD", // modified
					"lib/v/orphan.go":  "orphan-hash",      // orphaned
				},
			},
			{
				Name:       "int-vendor",
				Ref:        RefLocal,
				CommitHash: "int-hash",
				Source:     SourceInternal,
				FileHashes: map[string]string{
					"lib/int/dest.go": "int-dest-hash",
				},
				SourceFileHashes: map[string]string{
					"src/int.go": "int-src-hash-OLD", // source drifted
				},
			},
		},
	}, nil)

	// fs.Stat for findAddedFiles
	fs.EXPECT().Stat("lib/v/ok.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/v/changed.go").Return(&mockFileInfo{isDir: false}, nil)
	fs.EXPECT().Stat("lib/v/stale.go").Return(nil, os.ErrNotExist)
	fs.EXPECT().Stat("lib/int/dest.go").Return(&mockFileInfo{isDir: false}, nil)

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Marshal and unmarshal to verify JSON roundtrip
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	var parsed types.VerifyResult
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Schema version
	if parsed.SchemaVersion != "1.0" {
		t.Errorf("Expected schema_version '1.0', got %q", parsed.SchemaVersion)
	}

	// Timestamp should be non-empty and RFC3339 parseable
	if parsed.Timestamp == "" {
		t.Error("Expected non-empty timestamp")
	}

	// Summary fields
	if parsed.Summary.Verified != 2 {
		t.Errorf("Expected 2 verified (ext ok + internal dest), got %d", parsed.Summary.Verified)
	}
	if parsed.Summary.Modified != 1 {
		t.Errorf("Expected 1 modified, got %d", parsed.Summary.Modified)
	}
	if parsed.Summary.Stale != 1 {
		t.Errorf("Expected 1 stale, got %d", parsed.Summary.Stale)
	}
	if parsed.Summary.Orphaned != 1 {
		t.Errorf("Expected 1 orphaned, got %d", parsed.Summary.Orphaned)
	}
	if parsed.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL result, got %q", parsed.Summary.Result)
	}

	// Files array: check type diversity
	typeSet := make(map[string]bool)
	statusSet := make(map[string]bool)
	for _, f := range parsed.Files {
		typeSet[f.Type] = true
		statusSet[f.Status] = true
	}
	if !typeSet["file"] {
		t.Error("Expected 'file' type in JSON Files array")
	}
	if !typeSet["coherence"] {
		t.Error("Expected 'coherence' type in JSON Files array")
	}
	if !statusSet["verified"] {
		t.Error("Expected 'verified' status in JSON Files array")
	}
	if !statusSet["modified"] {
		t.Error("Expected 'modified' status in JSON Files array")
	}
	if !statusSet["stale"] {
		t.Error("Expected 'stale' status in JSON Files array")
	}
	if !statusSet["orphaned"] {
		t.Error("Expected 'orphaned' status in JSON Files array")
	}

	// InternalStatus should be present in JSON
	if len(parsed.InternalStatus) == 0 {
		t.Error("Expected internal_status in JSON output")
	} else {
		is := parsed.InternalStatus[0]
		if is.VendorName != "int-vendor" {
			t.Errorf("Expected internal_status vendor 'int-vendor', got %q", is.VendorName)
		}
		if string(is.Direction) != string(types.DriftSourceDrift) {
			t.Errorf("Expected source_drifted direction in JSON, got %q", is.Direction)
		}
	}
}
