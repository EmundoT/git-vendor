package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Path Tests
// ============================================================================

func TestFileLockStore_Path(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Test: Path() should return vendor.lock path
	expectedPath := filepath.Join(vendorDir, "vendor.lock")
	actualPath := store.Path()

	if actualPath != expectedPath {
		t.Errorf("Path() = %q, want %q", actualPath, expectedPath)
	}
}

// ============================================================================
// GetHash Tests
// ============================================================================

func TestFileLockStore_GetHash(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create test lockfile
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "vendor1",
				Ref:        "main",
				CommitHash: "abc123def456",
			},
			{
				Name:       "vendor1",
				Ref:        "develop",
				CommitHash: "xyz789ghi012",
			},
			{
				Name:       "vendor2",
				Ref:        "v1.0",
				CommitHash: "111222333444",
			},
		},
	}

	// Save lockfile
	if err := store.Save(lock); err != nil {
		t.Fatalf("Failed to save lockfile: %v", err)
	}

	tests := []struct {
		name         string
		vendorName   string
		ref          string
		expectedHash string
	}{
		{
			name:         "vendor1 @ main",
			vendorName:   "vendor1",
			ref:          "main",
			expectedHash: "abc123def456",
		},
		{
			name:         "vendor1 @ develop",
			vendorName:   "vendor1",
			ref:          "develop",
			expectedHash: "xyz789ghi012",
		},
		{
			name:         "vendor2 @ v1.0",
			vendorName:   "vendor2",
			ref:          "v1.0",
			expectedHash: "111222333444",
		},
		{
			name:         "nonexistent vendor",
			vendorName:   "vendor3",
			ref:          "main",
			expectedHash: "",
		},
		{
			name:         "nonexistent ref",
			vendorName:   "vendor1",
			ref:          "nonexistent",
			expectedHash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := store.GetHash(tt.vendorName, tt.ref)
			if hash != tt.expectedHash {
				t.Errorf("GetHash(%q, %q) = %q, want %q", tt.vendorName, tt.ref, hash, tt.expectedHash)
			}
		})
	}
}

func TestFileLockStore_GetHash_EmptyLockfile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create empty lockfile
	lock := types.VendorLock{
		Vendors: []types.LockDetails{},
	}

	if err := store.Save(lock); err != nil {
		t.Fatalf("Failed to save lockfile: %v", err)
	}

	// Test: GetHash on empty lockfile should return empty string
	hash := store.GetHash("vendor1", "main")
	if hash != "" {
		t.Errorf("GetHash() on empty lockfile = %q, want empty string", hash)
	}
}

func TestFileLockStore_GetHash_MissingLockfile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Don't create lockfile - it doesn't exist

	// Test: GetHash on missing lockfile should return empty string (no error)
	hash := store.GetHash("vendor1", "main")
	if hash != "" {
		t.Errorf("GetHash() on missing lockfile = %q, want empty string", hash)
	}
}

// ============================================================================
// Load and Save Tests (additional coverage)
// ============================================================================

func TestFileLockStore_LoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create test lock
	originalLock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:        "test-vendor",
				Ref:         "main",
				CommitHash:  "abc123",
				Updated:     "2024-01-01T00:00:00Z",
				LicensePath: "vendor/licenses/test-vendor.txt",
			},
		},
	}

	// Test: Save
	if err := store.Save(originalLock); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Test: Load
	loadedLock, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded lock matches original
	if len(loadedLock.Vendors) != 1 {
		t.Fatalf("Expected 1 vendor, got %d", len(loadedLock.Vendors))
	}

	vendor := loadedLock.Vendors[0]
	if vendor.Name != "test-vendor" {
		t.Errorf("Vendor name = %q, want %q", vendor.Name, "test-vendor")
	}
	if vendor.Ref != "main" {
		t.Errorf("Vendor ref = %q, want %q", vendor.Ref, "main")
	}
	if vendor.CommitHash != "abc123" {
		t.Errorf("Vendor commit hash = %q, want %q", vendor.CommitHash, "abc123")
	}
}

func TestFileLockStore_Load_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, "vendor")
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Test: Load on missing file should error
	_, err := store.Load()
	if err == nil {
		t.Error("Expected error when loading missing lockfile, got nil")
	}
}
