package core

import (
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

	result, err := service.Verify()

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

	result, err := service.Verify()

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

	result, err := service.Verify()

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
	// This test requires a real filesystem for the walkdir to work
	// Create temporary directory and work from within it
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

	result, err := service.Verify()

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

	result, err := service.Verify()

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

	result, err := service.Verify()

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
							{From: "src/file.go", To: filepath.Join("lib", "test-vendor", "file.go")},
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

	result, err := service.Verify()

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
							{From: "src/file.go", To: filepath.Join("lib", "test-vendor", "file.go")},
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

	result, err = service.Verify()

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
	result, err := service.Verify()
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
	result, err := service.Verify()
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
				SourceHash: wholeHash, // source content = whole file content
			}},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), tmpDir)
	result, err := service.Verify()
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
	// Don't create the file — it's been deleted

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
	result, err := service.Verify()
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
	result, err := service.Verify()
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
	result, err := service.Verify()
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
	result, err := service.Verify()
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
	result, err := service.Verify()
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
