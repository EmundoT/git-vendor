package core

import (
	"encoding/json"
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
