package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// SyncVendor Tests - Comprehensive tests for the core sync function
// ============================================================================

func TestSyncVendor_HappyPath_LockedRef(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Create a simple vendor with one spec
	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "abc123def456"}

	// Mock: Create temp directory
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: All git operations succeed
	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def456", nil
	}

	// Mock: License file exists
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		if path == "/tmp/test-12345/LICENSE" {
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hashes, err := syncer.syncVendor(vendor, lockedRefs)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 1 {
		t.Errorf("Expected 1 hash, got %d", len(hashes))
	}
	if hashes["main"] != "abc123def456" {
		t.Errorf("Expected hash abc123def456, got %s", hashes["main"])
	}

	// Verify git operations were called in correct order
	if len(git.InitCalls) != 1 {
		t.Errorf("Expected 1 Init call, got %d", len(git.InitCalls))
	}
	if len(git.AddRemoteCalls) != 1 {
		t.Errorf("Expected 1 AddRemote call, got %d", len(git.AddRemoteCalls))
	}
	if len(git.FetchCalls) != 1 {
		t.Errorf("Expected 1 Fetch call, got %d", len(git.FetchCalls))
	}
	if len(git.CheckoutCalls) != 1 {
		t.Errorf("Expected 1 Checkout call, got %d", len(git.CheckoutCalls))
	}

	// Verify checkout was called with locked hash
	if git.CheckoutCalls[0][1] != "abc123def456" {
		t.Errorf("Expected checkout of locked hash, got %s", git.CheckoutCalls[0][1])
	}
}

func TestSyncVendor_HappyPath_UnlockedRef(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "latest789", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		if path == "/tmp/test-12345/LICENSE" {
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute with nil lockedRefs (unlocked mode)
	hashes, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if hashes["main"] != "latest789" {
		t.Errorf("Expected hash latest789, got %s", hashes["main"])
	}

	// Verify checkout was called with FETCH_HEAD (unlocked mode)
	if len(git.CheckoutCalls) != 1 {
		t.Errorf("Expected 1 Checkout call, got %d", len(git.CheckoutCalls))
	}
	if git.CheckoutCalls[0][1] != "FETCH_HEAD" {
		t.Errorf("Expected checkout of FETCH_HEAD, got %s", git.CheckoutCalls[0][1])
	}
}

func TestSyncVendor_ShallowFetchSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify shallow fetch was attempted
	if len(git.FetchCalls) != 1 {
		t.Errorf("Expected 1 Fetch call, got %d", len(git.FetchCalls))
	}
	if git.FetchCalls[0][1].(int) != 1 {
		t.Errorf("Expected shallow fetch (depth=1), got depth=%d", git.FetchCalls[0][1])
	}

	// Verify no fallback to FetchAll
	if len(git.FetchAllCalls) != 0 {
		t.Errorf("Expected no FetchAll calls, got %d", len(git.FetchAllCalls))
	}
}

func TestSyncVendor_ShallowFetchFails_FullFetchSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Shallow fetch fails, full fetch succeeds
	git.FetchFunc = func(dir string, depth int, ref string) error {
		if depth == 1 {
			return fmt.Errorf("shallow fetch failed")
		}
		return nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify fallback to FetchAll was called
	if len(git.FetchAllCalls) != 1 {
		t.Errorf("Expected 1 FetchAll call (fallback), got %d", len(git.FetchAllCalls))
	}
}

func TestSyncVendor_BothFetchesFail(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Both fetches fail
	git.FetchFunc = func(dir string, depth int, ref string) error {
		return fmt.Errorf("network error")
	}
	git.FetchAllFunc = func(dir string) error {
		return fmt.Errorf("network error")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to fetch ref") {
		t.Errorf("Expected 'failed to fetch ref' error, got: %v", err)
	}
}

func TestSyncVendor_StaleCommitHashDetection(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	lockedRefs := map[string]string{"main": "stale123"}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Checkout fails with stale commit error
	git.CheckoutFunc = func(dir, ref string) error {
		if ref == "stale123" {
			return fmt.Errorf("reference is not a tree: stale123")
		}
		return nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, lockedRefs)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "no longer exists in the repository") {
		t.Errorf("Expected stale commit error message, got: %v", err)
	}
	if !contains(err.Error(), "git-vendor update") {
		t.Errorf("Expected helpful update message, got: %v", err)
	}
}

func TestSyncVendor_CheckoutFETCH_HEADFails_RefFallbackSucceeds(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: Checkout FETCH_HEAD fails, checkout ref succeeds
	checkoutAttempts := 0
	git.CheckoutFunc = func(dir, ref string) error {
		checkoutAttempts++
		if ref == "FETCH_HEAD" {
			return fmt.Errorf("FETCH_HEAD not available")
		}
		return nil // Checkout of "main" succeeds
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success (fallback), got error: %v", err)
	}
	if checkoutAttempts != 2 {
		t.Errorf("Expected 2 checkout attempts (FETCH_HEAD then ref), got %d", checkoutAttempts)
	}
}

func TestSyncVendor_AllCheckoutsFail(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	// Mock: All checkouts fail
	git.CheckoutFunc = func(dir, ref string) error {
		return fmt.Errorf("checkout failed")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "checkout ref") {
		t.Errorf("Expected checkout error, got: %v", err)
	}
}

func TestSyncVendor_TempDirectoryCreationFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	// Mock: CreateTemp fails
	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "", fmt.Errorf("disk full")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "disk full") {
		t.Errorf("Expected disk full error, got: %v", err)
	}
}

func TestSyncVendor_PathTraversalBlocked(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Create vendor with malicious path mapping
	vendor := types.VendorSpec{
		Name:    "malicious",
		URL:     "https://github.com/attacker/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "payload.txt", To: "../../../etc/passwd"},
				},
			},
		},
	}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	// Mock: File exists in temp repo
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected path traversal error, got nil")
	}
	if !contains(err.Error(), "invalid destination path") || !contains(err.Error(), "not allowed") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestSyncVendor_MultipleSpecsPerVendor(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: Vendor with 3 specs (main, dev, v1.0)
	vendor := types.VendorSpec{
		Name:    "test-vendor",
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

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	hashCounter := 0
	git.GetHeadHashFunc = func(dir string) (string, error) {
		hashCounter++
		return fmt.Sprintf("hash%d00000", hashCounter), nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	hashes, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(hashes) != 3 {
		t.Errorf("Expected 3 hashes (one per spec), got %d", len(hashes))
	}
	if _, ok := hashes["main"]; !ok {
		t.Error("Expected hash for 'main' ref")
	}
	if _, ok := hashes["dev"]; !ok {
		t.Error("Expected hash for 'dev' ref")
	}
	if _, ok := hashes["v1.0"]; !ok {
		t.Error("Expected hash for 'v1.0' ref")
	}

	// Verify each spec triggered a fetch
	if len(git.FetchCalls) != 3 {
		t.Errorf("Expected 3 Fetch calls (one per spec), got %d", len(git.FetchCalls))
	}
}

func TestSyncVendor_MultipleMappingsPerSpec(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	// Setup: One spec with 5 file mappings
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "file1.go", To: "lib/file1.go"},
					{From: "file2.go", To: "lib/file2.go"},
					{From: "file3.go", To: "lib/file3.go"},
					{From: "file4.go", To: "lib/file4.go"},
					{From: "file5.go", To: "lib/file5.go"},
				},
			},
		},
	}

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify all 5 files were copied (plus 1 for license = 6 total)
	if len(fs.CopyFileCalls) < 5 {
		t.Errorf("Expected at least 5 CopyFile calls (5 mappings), got %d", len(fs.CopyFileCalls))
	}
}

func TestSyncVendor_FileCopyFailsInMapping(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	fs.StatFunc = func(path string) (os.FileInfo, error) {
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: File copy fails only for the mapping (not license)
	fs.CopyFileFunc = func(src, dst string) error {
		// Let license copy succeed, but fail on the actual mapping
		if contains(src, "LICENSE") {
			return nil
		}
		return fmt.Errorf("permission denied")
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy file") {
		t.Errorf("Expected 'failed to copy file' error, got: %v", err)
	}
}

func TestSyncVendor_LicenseCopyFails(t *testing.T) {
	git, fs, config, lock, license := setupMocks()

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")

	fs.CreateTempFunc = func(dir, pattern string) (string, error) {
		return "/tmp/test-12345", nil
	}

	git.GetHeadHashFunc = func(dir string) (string, error) {
		return "abc123def", nil
	}

	// Mock: License file exists
	statCalls := 0
	fs.StatFunc = func(path string) (os.FileInfo, error) {
		statCalls++
		if statCalls == 1 && path == "/tmp/test-12345/LICENSE" {
			// First call: LICENSE exists
			return &mockFileInfo{name: "LICENSE", isDir: false}, nil
		}
		return &mockFileInfo{name: filepath.Base(path), isDir: false}, nil
	}

	// Mock: License copy fails
	fs.CopyFileFunc = func(src, dst string) error {
		if contains(src, "LICENSE") {
			return fmt.Errorf("disk full")
		}
		return nil
	}

	syncer := createMockSyncer(git, fs, config, lock, license, nil)

	// Execute
	_, err := syncer.syncVendor(vendor, nil)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "failed to copy license") {
		t.Errorf("Expected license copy error, got: %v", err)
	}
}

// ============================================================================
// TestUpdateAll - Comprehensive tests for update orchestration
// ============================================================================

