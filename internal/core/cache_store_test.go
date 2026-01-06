package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// TestCacheStore_SaveAndLoad tests basic cache save and load functionality
func TestCacheStore_SaveAndLoad(t *testing.T) {
	// Create temp directory for cache
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create a cache entry
	cache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "abc123def456",
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		Files: []types.FileChecksum{
			{Path: "lib/file1.go", Hash: "hash1"},
			{Path: "lib/file2.go", Hash: "hash2"},
		},
	}

	// Save cache
	err := cacheStore.Save(&cache)
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load cache
	loaded, err := cacheStore.Load("test-vendor", "main")
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// Verify loaded cache matches saved cache
	if loaded.VendorName != cache.VendorName {
		t.Errorf("Expected VendorName %s, got %s", cache.VendorName, loaded.VendorName)
	}
	if loaded.Ref != cache.Ref {
		t.Errorf("Expected Ref %s, got %s", cache.Ref, loaded.Ref)
	}
	if loaded.CommitHash != cache.CommitHash {
		t.Errorf("Expected CommitHash %s, got %s", cache.CommitHash, loaded.CommitHash)
	}
	if len(loaded.Files) != len(cache.Files) {
		t.Errorf("Expected %d files, got %d", len(cache.Files), len(loaded.Files))
	}
	for i := range loaded.Files {
		if loaded.Files[i].Path != cache.Files[i].Path {
			t.Errorf("File %d: Expected Path %s, got %s", i, cache.Files[i].Path, loaded.Files[i].Path)
		}
		if loaded.Files[i].Hash != cache.Files[i].Hash {
			t.Errorf("File %d: Expected Hash %s, got %s", i, cache.Files[i].Hash, loaded.Files[i].Hash)
		}
	}
}

// TestCacheStore_Load_CacheHit tests loading an existing cache (cache hit)
func TestCacheStore_Load_CacheHit(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create and save a cache
	expectedCache := types.IncrementalSyncCache{
		VendorName: "cached-vendor",
		Ref:        "v1.0",
		CommitHash: "commit-hash-123",
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		Files: []types.FileChecksum{
			{Path: "file1.go", Hash: "checksum1"},
			{Path: "file2.go", Hash: "checksum2"},
		},
	}

	err := cacheStore.Save(&expectedCache)
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load the cache (should hit)
	loaded, err := cacheStore.Load("cached-vendor", "v1.0")
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// Verify cache was loaded correctly
	if loaded.CommitHash != expectedCache.CommitHash {
		t.Errorf("Expected CommitHash %s, got %s", expectedCache.CommitHash, loaded.CommitHash)
	}
}

// TestCacheStore_Load_CommitMismatch tests detecting commit hash changes
func TestCacheStore_Load_CommitMismatch(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create and save a cache with an old commit hash
	oldCache := types.IncrementalSyncCache{
		VendorName: "test-vendor",
		Ref:        "main",
		CommitHash: "old-commit-hash",
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		Files:      []types.FileChecksum{},
	}

	err := cacheStore.Save(&oldCache)
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Load the cache
	loaded, err := cacheStore.Load("test-vendor", "main")
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// The new commit hash (simulated)
	newCommitHash := "new-commit-hash"

	// Verify that commit hashes don't match (caller would invalidate cache)
	if loaded.CommitHash == newCommitHash {
		t.Errorf("Expected commit hash mismatch, but hashes matched")
	}
	if loaded.CommitHash != "old-commit-hash" {
		t.Errorf("Expected old commit hash 'old-commit-hash', got %s", loaded.CommitHash)
	}
}

// TestCacheStore_Load_FileNotFound tests graceful handling of missing cache file
func TestCacheStore_Load_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Try to load cache that doesn't exist
	loaded, err := cacheStore.Load("nonexistent-vendor", "main")

	// Should return empty cache without error (cache miss is not an error)
	if err != nil {
		t.Fatalf("Expected no error for missing cache, got: %v", err)
	}

	if loaded.CommitHash != "" {
		t.Errorf("Expected empty cache, got CommitHash: %s", loaded.CommitHash)
	}
	if len(loaded.Files) != 0 {
		t.Errorf("Expected empty Files list, got %d files", len(loaded.Files))
	}
}

// TestCacheStore_BuildCache tests building a cache with file checksums
func TestCacheStore_BuildCache(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create test files
	file1Path := filepath.Join(tempDir, "file1.txt")
	file2Path := filepath.Join(tempDir, "file2.txt")

	err := os.WriteFile(file1Path, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(file2Path, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Build cache
	files := []string{file1Path, file2Path}
	cache, err := cacheStore.BuildCache("test-vendor", "main", "commit123", files)
	if err != nil {
		t.Fatalf("Failed to build cache: %v", err)
	}

	// Verify cache structure
	if cache.VendorName != "test-vendor" {
		t.Errorf("Expected VendorName 'test-vendor', got %s", cache.VendorName)
	}
	if cache.Ref != "main" {
		t.Errorf("Expected Ref 'main', got %s", cache.Ref)
	}
	if cache.CommitHash != "commit123" {
		t.Errorf("Expected CommitHash 'commit123', got %s", cache.CommitHash)
	}
	if len(cache.Files) != 2 {
		t.Errorf("Expected 2 files in cache, got %d", len(cache.Files))
	}

	// Verify checksums are computed
	for _, fc := range cache.Files {
		if fc.Hash == "" {
			t.Errorf("File %s has empty checksum", fc.Path)
		}
		if fc.Path != file1Path && fc.Path != file2Path {
			t.Errorf("Unexpected file path in cache: %s", fc.Path)
		}
	}
}

// TestCacheStore_ComputeFileChecksum tests checksum computation
func TestCacheStore_ComputeFileChecksum(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create test file with known content
	testFile := filepath.Join(tempDir, "test.txt")
	content := []byte("test content for checksum")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Compute checksum
	checksum1, err := cacheStore.ComputeFileChecksum(testFile)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	// Verify checksum is not empty
	if checksum1 == "" {
		t.Error("Expected non-empty checksum")
	}

	// Compute checksum again (should be same)
	checksum2, err := cacheStore.ComputeFileChecksum(testFile)
	if err != nil {
		t.Fatalf("Failed to compute checksum second time: %v", err)
	}

	if checksum1 != checksum2 {
		t.Errorf("Checksums don't match: %s vs %s", checksum1, checksum2)
	}

	// Modify file content
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Compute checksum of modified file
	checksum3, err := cacheStore.ComputeFileChecksum(testFile)
	if err != nil {
		t.Fatalf("Failed to compute checksum of modified file: %v", err)
	}

	// Checksum should be different
	if checksum1 == checksum3 {
		t.Error("Expected different checksum after file modification")
	}
}

// TestCacheStore_Delete tests cache deletion
func TestCacheStore_Delete(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create and save a cache
	cache := types.IncrementalSyncCache{
		VendorName: "delete-test",
		Ref:        "main",
		CommitHash: "hash123",
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		Files:      []types.FileChecksum{},
	}

	err := cacheStore.Save(&cache)
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Verify cache exists
	loaded, err := cacheStore.Load("delete-test", "main")
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}
	if loaded.CommitHash == "" {
		t.Fatal("Cache should exist before deletion")
	}

	// Delete cache
	err = cacheStore.Delete("delete-test", "main")
	if err != nil {
		t.Fatalf("Failed to delete cache: %v", err)
	}

	// Verify cache is deleted (load should return empty cache)
	loaded, err = cacheStore.Load("delete-test", "main")
	if err != nil {
		t.Fatalf("Failed to load cache after deletion: %v", err)
	}
	if loaded.CommitHash != "" {
		t.Error("Cache should be empty after deletion")
	}

	// Delete non-existent cache (should not error)
	err = cacheStore.Delete("nonexistent", "main")
	if err != nil {
		t.Errorf("Expected no error deleting non-existent cache, got: %v", err)
	}
}

// TestCacheStore_LargeCacheLimit tests that cache size is limited
func TestCacheStore_LargeCacheLimit(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewOSFileSystem()
	cacheStore := NewFileCacheStore(fs, tempDir)

	// Create 1500 test files (exceeds max of 1000)
	var files []string
	for i := 0; i < 1500; i++ {
		filePath := filepath.Join(tempDir, "file"+string(rune('0'+i%10))+".txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			err = os.WriteFile(filePath, []byte("content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
		files = append(files, filePath)
	}

	// Build cache (should be limited to 1000 files)
	cache, err := cacheStore.BuildCache("large-vendor", "main", "hash", files)
	if err != nil {
		t.Fatalf("Failed to build cache: %v", err)
	}

	// Verify cache is limited to 1000 files (or fewer due to deduplication)
	if len(cache.Files) > 1000 {
		t.Errorf("Expected at most 1000 files in cache, got %d", len(cache.Files))
	}
}
