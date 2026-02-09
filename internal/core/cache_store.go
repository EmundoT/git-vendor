package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// CacheStore handles incremental sync cache I/O operations
type CacheStore interface {
	Load(vendorName, ref string) (types.IncrementalSyncCache, error)
	Save(cache *types.IncrementalSyncCache) error
	Delete(vendorName, ref string) error
	ComputeFileChecksum(path string) (string, error)
	BuildCache(vendorName, ref, commitHash string, files []string) (types.IncrementalSyncCache, error)
}

// FileCacheStore implements CacheStore using JSON files in vendor/.cache/
type FileCacheStore struct {
	fs      FileSystem
	rootDir string
}

// NewFileCacheStore creates a new FileCacheStore
func NewFileCacheStore(fs FileSystem, rootDir string) *FileCacheStore {
	return &FileCacheStore{
		fs:      fs,
		rootDir: rootDir,
	}
}

// cacheDir returns the cache directory path
func (s *FileCacheStore) cacheDir() string {
	return filepath.Join(s.rootDir, VendorDir, ".cache")
}

// cachePath returns the cache file path for a vendor@ref
func (s *FileCacheStore) cachePath(vendorName, ref string) string {
	// Sanitize vendor name and ref for filename
	filename := fmt.Sprintf("%s-%s.json", sanitizeFilename(vendorName), sanitizeFilename(ref))
	return filepath.Join(s.cacheDir(), filename)
}

// sanitizeFilename replaces invalid filename characters with underscores
func sanitizeFilename(s string) string {
	result := []rune{}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

// Load reads the cache file for a vendor@ref
func (s *FileCacheStore) Load(vendorName, ref string) (types.IncrementalSyncCache, error) {
	var cache types.IncrementalSyncCache

	path := s.cachePath(vendorName, ref)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Cache miss - return empty cache (not an error)
			return cache, nil
		}
		return cache, err
	}

	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache - return empty cache and log warning
		return types.IncrementalSyncCache{}, fmt.Errorf("corrupted cache file %s: %w", path, err)
	}

	return cache, nil
}

// Save writes the cache file for a vendor@ref
func (s *FileCacheStore) Save(cache *types.IncrementalSyncCache) error {
	// Ensure cache directory exists
	cacheDir := s.cacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	path := s.cachePath(cache.VendorName, cache.Ref)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Delete removes the cache file for a vendor@ref
func (s *FileCacheStore) Delete(vendorName, ref string) error {
	path := s.cachePath(vendorName, ref)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}
	return nil
}

// ComputeFileChecksum computes SHA-256 hash of a file
func (s *FileCacheStore) ComputeFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum for %s: %w", path, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// BuildCache creates a cache entry by computing checksums for all files
func (s *FileCacheStore) BuildCache(vendorName, ref, commitHash string, files []string) (types.IncrementalSyncCache, error) {
	cache := types.IncrementalSyncCache{
		VendorName: vendorName,
		Ref:        ref,
		CommitHash: commitHash,
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		Files:      make([]types.FileChecksum, 0, len(files)),
	}

	// Limit cache size to prevent excessive memory usage
	const maxCacheFiles = 1000
	if len(files) > maxCacheFiles {
		files = files[:maxCacheFiles]
	}

	for _, path := range files {
		// Compute checksum for the file
		hash, err := s.ComputeFileChecksum(path)
		if err != nil {
			// Skip files that can't be read (they might be deleted)
			continue
		}

		cache.Files = append(cache.Files, types.FileChecksum{
			Path: path,
			Hash: hash,
		})
	}

	return cache, nil
}
