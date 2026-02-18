package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// PullVendors Tests - Tests for the pull command orchestration
// ============================================================================

// pullTestEnv creates a real filesystem-backed test environment for pull tests.
// pullTestEnv returns a VendorSyncer with real config/lock stores and a cache,
// enabling end-to-end pull orchestration tests without gomock.
type pullTestEnv struct {
	syncer    *VendorSyncer
	syncSvc   *stubSyncService
	updateSvc *stubUpdateService
	rootDir   string
	configDir string
	t         *testing.T
}

func setupPullTestEnv(t *testing.T) *pullTestEnv {
	t.Helper()

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, VendorDir)
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configStore := NewFileConfigStore(rootDir)
	lockStore := NewFileLockStore(rootDir)
	fs := NewRootedFileSystem(tmpDir)

	syncSvc := &stubSyncService{}
	updateSvc := &stubUpdateService{}

	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
		sync:        syncSvc,
		update:      updateSvc,
		fs:          fs,
		rootDir:     rootDir,
		ui:          &SilentUICallback{},
	}

	return &pullTestEnv{
		syncer:    syncer,
		syncSvc:   syncSvc,
		updateSvc: updateSvc,
		rootDir:   rootDir,
		configDir: tmpDir,
		t:         t,
	}
}

// writeConfig writes a VendorConfig to the test environment's vendor.yml.
func (e *pullTestEnv) writeConfig(cfg types.VendorConfig) {
	e.t.Helper()
	if err := e.syncer.configStore.Save(cfg); err != nil {
		e.t.Fatal(err)
	}
}

// writeLock writes a VendorLock to the test environment's vendor.lock.
func (e *pullTestEnv) writeLock(lock types.VendorLock) {
	e.t.Helper()
	if err := e.syncer.lockStore.Save(lock); err != nil {
		e.t.Fatal(err)
	}
}

func testLock() types.VendorLock {
	return types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
				FileHashes: map[string]string{"lib/file.go": "deadbeef"},
			},
		},
	}
}

func TestPullVendors_DefaultMode_CallsUpdateThenSync(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	result, err := env.syncer.PullVendors(context.Background(), PullOptions{})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if env.updateSvc.callCount != 1 {
		t.Errorf("Expected update called once, got %d", env.updateSvc.callCount)
	}
	if !env.syncSvc.syncCalled {
		t.Error("Expected sync to be called")
	}
	if result.Updated != 1 {
		t.Errorf("Expected Updated=1, got %d", result.Updated)
	}
}

func TestPullVendors_LockedMode_SkipsUpdate(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	result, err := env.syncer.PullVendors(context.Background(), PullOptions{Locked: true})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if env.updateSvc.callCount != 0 {
		t.Error("Expected update NOT to be called in --locked mode")
	}
	if !env.syncSvc.syncCalled {
		t.Error("Expected sync to be called in --locked mode")
	}
	if result.Updated != 0 {
		t.Errorf("Expected Updated=0 in --locked mode, got %d", result.Updated)
	}
}

func TestPullVendors_VendorFilter_PassedToUpdateAndSync(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("my-lib", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{
				Name:       "my-lib",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
				FileHashes: map[string]string{"lib/file.go": "deadbeef"},
			},
		},
	})

	_, err := env.syncer.PullVendors(context.Background(), PullOptions{VendorName: "my-lib"})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if env.updateSvc.lastOpts.VendorName != "my-lib" {
		t.Errorf("Expected update VendorName='my-lib', got '%s'", env.updateSvc.lastOpts.VendorName)
	}
	if env.syncSvc.syncOpts.VendorName != "my-lib" {
		t.Errorf("Expected sync VendorName='my-lib', got '%s'", env.syncSvc.syncOpts.VendorName)
	}
}

func TestPullVendors_ForceAndNoCache_PassedToSync(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	_, err := env.syncer.PullVendors(context.Background(), PullOptions{Force: true, NoCache: true})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if !env.syncSvc.syncOpts.Force {
		t.Error("Expected sync Force=true")
	}
	if !env.syncSvc.syncOpts.NoCache {
		t.Error("Expected sync NoCache=true")
	}
}

func TestPullVendors_UpdateError_PropagatesError(t *testing.T) {
	env := setupPullTestEnv(t)
	env.updateSvc.updateErr = os.ErrPermission

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))

	_, err := env.syncer.PullVendors(context.Background(), PullOptions{})
	if err == nil {
		t.Fatal("Expected error from update, got nil")
	}

	if env.syncSvc.syncCalled {
		t.Error("Expected sync NOT to be called when update fails")
	}
}

func TestPullVendors_Prune_RemovesDeadMappings(t *testing.T) {
	env := setupPullTestEnv(t)

	// Config has two mappings, but lock only knows about one
	vendor := types.VendorSpec{
		Name:    "test-vendor",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
					{From: "src/deleted.go", To: "lib/deleted.go"},
				},
			},
		},
	}
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
				FileHashes: map[string]string{"lib/file.go": "deadbeef"},
			},
		},
	})

	result, err := env.syncer.PullVendors(context.Background(), PullOptions{Prune: true})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if result.MappingsPruned != 1 {
		t.Errorf("Expected 1 mapping pruned, got %d", result.MappingsPruned)
	}

	// Verify config was updated
	cfg, err := env.syncer.configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Vendors[0].Specs[0].Mapping) != 1 {
		t.Errorf("Expected 1 mapping remaining, got %d", len(cfg.Vendors[0].Specs[0].Mapping))
	}
	if cfg.Vendors[0].Specs[0].Mapping[0].From != "src/file.go" {
		t.Errorf("Expected remaining mapping from='src/file.go', got '%s'", cfg.Vendors[0].Specs[0].Mapping[0].From)
	}
}

func TestPullVendors_Interactive_DoesNotError(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	// --interactive should not error, just print notice
	_, err := env.syncer.PullVendors(context.Background(), PullOptions{Interactive: true})
	if err != nil {
		t.Fatalf("PullVendors with --interactive returned error: %v", err)
	}
}

func TestPullVendors_LocalFlag_PassedThrough(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	_, err := env.syncer.PullVendors(context.Background(), PullOptions{Local: true})
	if err != nil {
		t.Fatalf("PullVendors returned error: %v", err)
	}

	if !env.updateSvc.lastOpts.Local {
		t.Error("Expected update Local=true")
	}
	if !env.syncSvc.syncOpts.Local {
		t.Error("Expected sync Local=true")
	}
}

// TestPullVendors_KeepLocal_DoesNotError verifies that --keep-local flag passes
// through without error. The full file preservation flow (C1) requires real
// filesystem paths that match lock entries, which needs integration-level testing.
func TestPullVendors_KeepLocal_DoesNotError(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	result, err := env.syncer.PullVendors(context.Background(), PullOptions{KeepLocal: true})
	if err != nil {
		t.Fatalf("PullVendors with --keep-local returned error: %v", err)
	}

	// With stub sync (no-op), no files are detected as modified (relative paths
	// from lock don't resolve against test CWD), so FilesSkipped should be 0.
	if result.FilesSkipped != 0 {
		t.Errorf("Expected FilesSkipped=0 with stub sync, got %d", result.FilesSkipped)
	}
}

// TestBackupRestore_RoundTrip verifies that backupLocallyModified and
// restoreLocallyModified correctly preserve file content through a backup/restore
// cycle. This tests the core C1 mechanism independent of the full pull pipeline.
func TestBackupRestore_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, VendorDir)
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fs := NewRootedFileSystem(tmpDir)
	syncer := &VendorSyncer{
		fs:      fs,
		rootDir: rootDir,
		ui:      &SilentUICallback{},
	}

	// Create a file to back up
	testFile := filepath.Join(tmpDir, "lib", "file.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
		t.Fatal(err)
	}
	originalContent := []byte("// original local modification\npackage lib\n")
	if err := os.WriteFile(testFile, originalContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate modified paths map (path -> hash, hash value doesn't matter for backup)
	modifiedPaths := map[string]string{testFile: "somehash"}

	// Back up
	backups, err := syncer.backupLocallyModified(modifiedPaths)
	if err != nil {
		t.Fatalf("backupLocallyModified failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}

	// Simulate sync overwriting the file
	if err := os.WriteFile(testFile, []byte("// overwritten by sync\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restore
	restored, err := syncer.restoreLocallyModified(backups)
	if err != nil {
		t.Fatalf("restoreLocallyModified failed: %v", err)
	}
	if restored != 1 {
		t.Errorf("expected 1 restored, got %d", restored)
	}

	// Verify content is back to original
	afterContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after restore: %v", err)
	}
	if string(afterContent) != string(originalContent) {
		t.Errorf("restore did not preserve content.\nExpected: %q\nGot: %q", originalContent, afterContent)
	}
}

// TestPruneDeadMappings_NoDeadMappings verifies pruneDeadMappings is a no-op when all mappings are alive.
func TestPruneDeadMappings_NoDeadMappings(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor := createTestVendorSpec("test-vendor", "https://github.com/owner/repo", "main")
	env.writeConfig(createTestConfig(vendor))
	env.writeLock(testLock())

	pruned, warnings, err := env.syncer.pruneDeadMappings("")
	if err != nil {
		t.Fatalf("pruneDeadMappings returned error: %v", err)
	}
	if pruned != 0 {
		t.Errorf("Expected 0 pruned, got %d", pruned)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected 0 warnings, got %d", len(warnings))
	}
}

// TestPruneDeadMappings_VendorFilter verifies pruneDeadMappings respects vendor name filter.
func TestPruneDeadMappings_VendorFilter(t *testing.T) {
	env := setupPullTestEnv(t)

	vendor1 := types.VendorSpec{
		Name:    "vendor-a",
		URL:     "https://github.com/a/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/a.go", To: "lib/a.go"},
					{From: "src/dead.go", To: "lib/dead.go"},
				},
			},
		},
	}
	vendor2 := types.VendorSpec{
		Name:    "vendor-b",
		URL:     "https://github.com/b/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/b.go", To: "lib/b.go"},
					{From: "src/also-dead.go", To: "lib/also-dead.go"},
				},
			},
		},
	}
	env.writeConfig(types.VendorConfig{Vendors: []types.VendorSpec{vendor1, vendor2}})
	env.writeLock(types.VendorLock{
		SchemaVersion: "1.1",
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "abc", Updated: "2024-01-01T00:00:00Z", FileHashes: map[string]string{"lib/a.go": "hash-a"}},
			{Name: "vendor-b", Ref: "main", CommitHash: "def", Updated: "2024-01-01T00:00:00Z", FileHashes: map[string]string{"lib/b.go": "hash-b"}},
		},
	})

	// Prune only vendor-a
	pruned, _, err := env.syncer.pruneDeadMappings("vendor-a")
	if err != nil {
		t.Fatalf("pruneDeadMappings returned error: %v", err)
	}
	if pruned != 1 {
		t.Errorf("Expected 1 pruned (vendor-a only), got %d", pruned)
	}

	// vendor-b should still have 2 mappings
	cfg, _ := env.syncer.configStore.Load()
	for _, v := range cfg.Vendors {
		if v.Name == "vendor-b" {
			if len(v.Specs[0].Mapping) != 2 {
				t.Errorf("vendor-b should still have 2 mappings, got %d", len(v.Specs[0].Mapping))
			}
		}
	}
}
