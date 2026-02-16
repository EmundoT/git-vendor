package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Stub Infrastructure for Integration Tests
//
// These stubs replace only external system dependencies (git, license API,
// hooks) while using real filesystem stores, real file I/O, and real service
// wiring. This exercises the full sync → lockfile → verify pipeline.
// ============================================================================

// stubGitClient implements GitClient by populating pre-configured source files
// into the working directory on Checkout. All other operations are no-ops.
type stubGitClient struct {
	headHash    string
	sourceFiles map[string]string // relative path -> content
}

func (s *stubGitClient) Init(_ context.Context, _ string) error            { return nil }
func (s *stubGitClient) AddRemote(_ context.Context, _, _, _ string) error { return nil }
func (s *stubGitClient) Fetch(_ context.Context, _, _ string, _ int, _ string) error {
	return nil
}
func (s *stubGitClient) FetchAll(_ context.Context, _, _ string) error { return nil }
func (s *stubGitClient) SetRemoteURL(_ context.Context, _, _, _ string) error {
	return nil
}

// Checkout populates the git working directory with pre-configured source files.
func (s *stubGitClient) Checkout(_ context.Context, dir, _ string) error {
	for relPath, content := range s.sourceFiles {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s *stubGitClient) GetHeadHash(_ context.Context, _ string) (string, error) {
	return s.headHash, nil
}
func (s *stubGitClient) Clone(_ context.Context, _, _ string, _ *types.CloneOptions) error {
	return nil
}
func (s *stubGitClient) ListTree(_ context.Context, _, _, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubGitClient) GetCommitLog(_ context.Context, _, _, _ string, _ int) ([]types.CommitInfo, error) {
	return nil, nil
}
func (s *stubGitClient) GetTagForCommit(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (s *stubGitClient) Add(_ context.Context, _ string, _ ...string) error { return nil }
func (s *stubGitClient) Commit(_ context.Context, _ string, _ types.CommitOptions) error {
	return nil
}
func (s *stubGitClient) AddNote(_ context.Context, _, _, _, _ string) error { return nil }
func (s *stubGitClient) GetNote(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}
func (s *stubGitClient) ConfigSet(_ context.Context, _, _, _ string) error { return nil }
func (s *stubGitClient) ConfigGet(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (s *stubGitClient) LsRemote(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

// stubLicenseService and stubHookExecutor are defined in testhelpers_gomock_test.go.

// positionHash computes the sha256 hash in the format used by ExtractPosition.
func positionHash(content string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
}

// chdirTest changes the working directory to dir for the duration of the test.
// Not safe for t.Parallel() — these integration tests run sequentially.
func chdirTest(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig) //nolint:errcheck // best-effort restore
	})
}

// ============================================================================
// Helper: set up a project root with vendor stores and services
// ============================================================================

// positionTestEnv holds all wired services and paths for a single integration test.
type positionTestEnv struct {
	rootDir     string
	configStore ConfigStore
	lockStore   LockStore
	cacheStore  CacheStore
	fs          FileSystem
	fileCopy    *FileCopyService
	syncSvc     *SyncService
	updateSvc   *UpdateService
	verifySvc   *VerifyService
	gitClient   *stubGitClient
}

// newPositionTestEnv creates a fully wired test environment with real filesystem
// stores and a stubbed git client. sourceFiles maps relative path -> content
// that the stub git client returns on Checkout. CWD is changed to rootDir.
func newPositionTestEnv(t *testing.T, sourceFiles map[string]string, commitHash string) *positionTestEnv {
	t.Helper()

	rootDir := t.TempDir()
	chdirTest(t, rootDir)

	// Create vendor directory structure
	vendorDir := filepath.Join(rootDir, VendorDir)
	if err := os.MkdirAll(filepath.Join(vendorDir, LicensesDir), 0755); err != nil {
		t.Fatal(err)
	}

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, rootDir)
	fileCopy := NewFileCopyService(osFS)
	ui := &SilentUICallback{}

	git := &stubGitClient{
		headHash:    commitHash,
		sourceFiles: sourceFiles,
	}

	syncSvc := NewSyncService(
		configStore, lockStore, git, osFS, fileCopy,
		&stubLicenseService{}, cacheStore, &stubHookExecutor{}, ui, rootDir, nil,
	)
	updateSvc := NewUpdateService(configStore, lockStore, syncSvc, nil, cacheStore, ui, rootDir)
	verifySvc := NewVerifyService(configStore, lockStore, cacheStore, osFS, rootDir)

	return &positionTestEnv{
		rootDir:     rootDir,
		configStore: configStore,
		lockStore:   lockStore,
		cacheStore:  cacheStore,
		fs:          osFS,
		fileCopy:    fileCopy,
		syncSvc:     syncSvc,
		updateSvc:   updateSvc,
		verifySvc:   verifySvc,
		gitClient:   git,
	}
}

// ============================================================================
// Test 1: Full lifecycle — sync → lockfile → verify → modify → drift → re-sync → verify
// ============================================================================

func TestPositionIntegration_FullLifecycle(t *testing.T) {
	// Source file content (simulates remote repo)
	sourceContent := "package api\n\nconst (\n\tFoo = 1\n\tBar = 2\n\tBaz = 3\n)\n\nfunc Hello() {}\n"
	commitHash := "abc123def456789012345678901234567890abcd"

	env := newPositionTestEnv(t, map[string]string{
		"api/constants.go": sourceContent,
	}, commitHash)

	// Pre-populate destination with 15 lines (position L10-L12 must exist)
	destRelPath := "local/constants.go"
	destFilePath := filepath.Join(env.rootDir, destRelPath)
	if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
		t.Fatal(err)
	}
	destLines := make([]string, 15)
	for i := range destLines {
		destLines[i] = fmt.Sprintf("// line %d", i+1)
	}
	if err := os.WriteFile(destFilePath, []byte(strings.Join(destLines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Configure vendor with position mapping: extract L4-L6 from source, place at L10-L12 in dest
	vendor := types.VendorSpec{
		Name:    "mylib",
		URL:     "https://github.com/test/mylib",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "api/constants.go:L4-L6", To: "local/constants.go:L10-L12"},
			},
		}},
	}
	cfg := types.VendorConfig{Vendors: []types.VendorSpec{vendor}}
	if err := env.configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}

	// === Phase 1: Sync via SyncVendor (update mode — nil lockedRefs) ===
	refMeta, stats, err := env.syncSvc.SyncVendor(context.Background(), &vendor, nil, SyncOptions{Force: true, NoCache: true})
	if err != nil {
		t.Fatalf("SyncVendor failed: %v", err)
	}

	// Verify CopyStats has position records
	if len(stats.Positions) != 1 {
		t.Fatalf("expected 1 position record, got %d", len(stats.Positions))
	}
	if stats.Positions[0].From != "api/constants.go:L4-L6" {
		t.Errorf("position From = %q, want api/constants.go:L4-L6", stats.Positions[0].From)
	}
	if stats.Positions[0].To != "local/constants.go:L10-L12" {
		t.Errorf("position To = %q, want local/constants.go:L10-L12", stats.Positions[0].To)
	}
	if !strings.HasPrefix(stats.Positions[0].SourceHash, "sha256:") {
		t.Errorf("SourceHash should start with sha256:, got %q", stats.Positions[0].SourceHash)
	}

	// Verify RefMetadata carries positions
	meta, ok := refMeta["main"]
	if !ok {
		t.Fatal("expected 'main' in refMeta")
	}
	if len(meta.Positions) != 1 {
		t.Fatalf("expected 1 position in RefMetadata, got %d", len(meta.Positions))
	}

	// === Phase 2: Build and save lockfile with file hashes + positions ===
	wholeFileHash, err := env.cacheStore.ComputeFileChecksum(destRelPath)
	if err != nil {
		t.Fatalf("compute hash for %s: %v", destRelPath, err)
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "mylib",
			Ref:        "main",
			CommitHash: meta.CommitHash,
			Updated:    time.Now().UTC().Format(time.RFC3339),
			FileHashes: map[string]string{destRelPath: wholeFileHash},
			Positions:  toPositionLocks(meta.Positions),
		}},
	}
	if err := env.lockStore.Save(lock); err != nil {
		t.Fatal(err)
	}

	// === Phase 3: Verify passes ===
	result, err := env.verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS after sync, got %s (verified=%d, modified=%d, deleted=%d)",
			result.Summary.Result, result.Summary.Verified, result.Summary.Modified, result.Summary.Deleted)
	}

	// Position entry should be verified
	foundPositionVerified := false
	for _, f := range result.Files {
		if strings.Contains(f.Path, "local/constants.go:L10-L12") && f.Status == "verified" {
			foundPositionVerified = true
		}
	}
	if !foundPositionVerified {
		t.Error("expected a verified position entry for local/constants.go:L10-L12")
	}

	// === Phase 4: Modify destination at target position → verify detects drift ===
	destData, err := os.ReadFile(destFilePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(destData), "\n")
	lines[9] = "\tFoo = 999 // tampered" // 0-indexed line 10
	if err := os.WriteFile(destFilePath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	result, err = env.verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify after tamper failed: %v", err)
	}
	if result.Summary.Modified == 0 {
		t.Error("expected modified count > 0 after tampering target position")
	}
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL after tamper, got %s", result.Summary.Result)
	}

	// === Phase 5: Re-sync to restore ===
	_, _, err = env.syncSvc.SyncVendor(context.Background(), &vendor, nil, SyncOptions{Force: true, NoCache: true})
	if err != nil {
		t.Fatalf("Re-sync failed: %v", err)
	}

	// Update lockfile hashes after re-sync
	newWholeHash, err := env.cacheStore.ComputeFileChecksum(destRelPath)
	if err != nil {
		t.Fatalf("compute hash after re-sync: %v", err)
	}
	lock.Vendors[0].FileHashes[destRelPath] = newWholeHash
	if err := env.lockStore.Save(lock); err != nil {
		t.Fatal(err)
	}

	// === Phase 6: Verify passes again ===
	result, err = env.verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify after re-sync failed: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS after re-sync, got %s (modified=%d)", result.Summary.Result, result.Summary.Modified)
	}
}

// ============================================================================
// Test 2: Sync with local modifications → warning captured in CopyStats
// ============================================================================

func TestPositionIntegration_LocalModificationWarning(t *testing.T) {
	sourceContent := "line1\nline2\nline3\nline4\nline5\n"
	commitHash := "def456abc789012345678901234567890abcdef01"

	env := newPositionTestEnv(t, map[string]string{
		"src/data.txt": sourceContent,
	}, commitHash)

	// Pre-create destination file with different content at the target position
	destFilePath := filepath.Join(env.rootDir, "local/data.txt")
	if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFilePath, []byte("keep1\nLOCAL_EDIT_A\nLOCAL_EDIT_B\nkeep4\nkeep5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vendor := types.VendorSpec{
		Name:    "datalib",
		URL:     "https://github.com/test/datalib",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/data.txt:L2-L3", To: "local/data.txt:L2-L3"},
			},
		}},
	}

	// Create a temp dir simulating what git checkout provides
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte(sourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := env.fileCopy.CopyMappings(tempDir, &vendor, vendor.Specs[0])
	if err != nil {
		t.Fatalf("CopyMappings failed: %v", err)
	}

	// CopyStats.Warnings should contain a local modifications warning
	if len(stats.Warnings) == 0 {
		t.Fatal("expected at least one warning about local modifications")
	}
	foundWarning := false
	for _, w := range stats.Warnings {
		if strings.Contains(w, "local modifications") && strings.Contains(w, "target position") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected warning containing 'local modifications' and 'target position', got: %v", stats.Warnings)
	}

	// Verify position record tracked in CopyStats
	if len(stats.Positions) != 1 {
		t.Fatalf("expected 1 position record, got %d", len(stats.Positions))
	}
	if stats.Positions[0].From != "src/data.txt:L2-L3" {
		t.Errorf("position From = %q", stats.Positions[0].From)
	}

	// After copy, destination L2-L3 should have the source content (overwritten)
	destData, err := os.ReadFile(destFilePath)
	if err != nil {
		t.Fatal(err)
	}
	destLines := strings.Split(string(destData), "\n")
	if destLines[1] != "line2" {
		t.Errorf("expected line 2 = %q, got %q", "line2", destLines[1])
	}
	if destLines[2] != "line3" {
		t.Errorf("expected line 3 = %q, got %q", "line3", destLines[2])
	}
	// Surrounding lines preserved
	if destLines[0] != "keep1" {
		t.Errorf("expected line 1 preserved as %q, got %q", "keep1", destLines[0])
	}
	if destLines[3] != "keep4" {
		t.Errorf("expected line 4 preserved as %q, got %q", "keep4", destLines[3])
	}
}

// ============================================================================
// Test 3: Parallel sync with 2 vendors, each having position mappings
// ============================================================================

func TestPositionIntegration_ParallelSync(t *testing.T) {
	commitHashA := "aaa111bbb222ccc333ddd444eee555fff666aaa11"
	commitHashB := "bbb222ccc333ddd444eee555fff666aaa111bbb22"

	sourceContentA := "pkgA line1\npkgA line2\npkgA line3\npkgA line4\npkgA line5\n"
	sourceContentB := "pkgB line1\npkgB line2\npkgB line3\npkgB line4\npkgB line5\n"

	rootDir := t.TempDir()
	chdirTest(t, rootDir)

	vendorDir := filepath.Join(rootDir, VendorDir)
	if err := os.MkdirAll(filepath.Join(vendorDir, LicensesDir), 0755); err != nil {
		t.Fatal(err)
	}

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	osFS := NewOSFileSystem()
	cacheStore := NewFileCacheStore(osFS, rootDir)
	fileCopy := NewFileCopyService(osFS)
	ui := &SilentUICallback{}

	gitA := &stubGitClient{
		headHash:    commitHashA,
		sourceFiles: map[string]string{"src/a.go": sourceContentA},
	}
	gitB := &stubGitClient{
		headHash:    commitHashB,
		sourceFiles: map[string]string{"src/b.go": sourceContentB},
	}

	vendorA := types.VendorSpec{
		Name:    "vendor-a",
		URL:     "https://github.com/test/vendor-a",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/a.go:L2-L4", To: "lib/a_extract.go"},
			},
		}},
	}
	vendorB := types.VendorSpec{
		Name:    "vendor-b",
		URL:     "https://github.com/test/vendor-b",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/b.go:L1-L3", To: "lib/b_extract.go"},
			},
		}},
	}

	cfg := types.VendorConfig{Vendors: []types.VendorSpec{vendorA, vendorB}}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Create per-vendor sync services (each with its own stubbed git)
	syncA := NewSyncService(configStore, lockStore, gitA, osFS, fileCopy,
		&stubLicenseService{}, cacheStore, &stubHookExecutor{}, ui, rootDir, nil)
	syncB := NewSyncService(configStore, lockStore, gitB, osFS, fileCopy,
		&stubLicenseService{}, cacheStore, &stubHookExecutor{}, ui, rootDir, nil)

	syncMap := map[string]*SyncService{
		"vendor-a": syncA,
		"vendor-b": syncB,
	}
	syncFunc := func(ctx context.Context, v types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		svc, ok := syncMap[v.Name]
		if !ok {
			return nil, CopyStats{}, fmt.Errorf("unknown vendor: %s", v.Name)
		}
		return svc.SyncVendor(ctx, &v, lockedRefs, opts)
	}

	executor := NewParallelExecutor(types.ParallelOptions{Enabled: true, MaxWorkers: 2}, ui)
	results, err := executor.ExecuteParallelSync(
		context.Background(),
		[]types.VendorSpec{vendorA, vendorB},
		nil,
		SyncOptions{Force: true, NoCache: true},
		syncFunc,
	)
	if err != nil {
		t.Fatalf("parallel sync failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Build lockfile from parallel results — include both FileHashes and Positions
	lock := types.VendorLock{}
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("vendor %s failed: %v", r.Vendor.Name, r.Error)
		}
		for ref, meta := range r.UpdatedRefs {
			// Compute file hashes for each destination (required by verify)
			fileHashes := make(map[string]string)
			for _, spec := range r.Vendor.Specs {
				if spec.Ref != ref {
					continue
				}
				for _, m := range spec.Mapping {
					destFile, _, _ := types.ParsePathPosition(m.To)
					hash, hashErr := cacheStore.ComputeFileChecksum(destFile)
					if hashErr == nil {
						fileHashes[destFile] = hash
					}
				}
			}

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name:       r.Vendor.Name,
				Ref:        ref,
				CommitHash: meta.CommitHash,
				Updated:    time.Now().UTC().Format(time.RFC3339),
				FileHashes: fileHashes,
				Positions:  toPositionLocks(meta.Positions),
			})
		}
	}

	// Verify both vendors have position entries
	for _, entry := range lock.Vendors {
		if len(entry.Positions) != 1 {
			t.Errorf("vendor %s: expected 1 position lock, got %d", entry.Name, len(entry.Positions))
		}
		if !strings.HasPrefix(entry.Positions[0].SourceHash, "sha256:") {
			t.Errorf("vendor %s: SourceHash format invalid: %q", entry.Name, entry.Positions[0].SourceHash)
		}
	}

	if err := lockStore.Save(lock); err != nil {
		t.Fatal(err)
	}

	verifySvc := NewVerifyService(configStore, lockStore, cacheStore, osFS, rootDir)
	result, err := verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS, got %s (verified=%d, modified=%d, deleted=%d)",
			result.Summary.Result, result.Summary.Verified, result.Summary.Modified, result.Summary.Deleted)
	}
	// Whole-file + position entries: at least 2 verified for positions
	if result.Summary.Verified < 2 {
		t.Errorf("expected at least 2 verified entries, got %d", result.Summary.Verified)
	}
}

// ============================================================================
// Test 4: CopyStats aggregation across multiple position mappings
// ============================================================================

func TestPositionIntegration_CopyStatsAggregation(t *testing.T) {
	sourceContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"

	rootDir := t.TempDir()
	chdirTest(t, rootDir)

	// Create source files in a separate temp dir (simulating git checkout dir)
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "multi.txt"), []byte(sourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	osFS := NewOSFileSystem()
	fileCopy := NewFileCopyService(osFS)

	// Pre-create destination files with position targets (relative to CWD = rootDir)
	if err := os.MkdirAll("out", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("out/a.txt", []byte("a1\na2\na3\na4\na5\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("out/b.txt", []byte("b1\nb2\nb3\nb4\nb5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vendor := types.VendorSpec{
		Name:    "multi-vendor",
		URL:     "https://github.com/test/multi",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "src/multi.txt:L1-L3", To: "out/a.txt:L2-L3"},
				{From: "src/multi.txt:L5-L7", To: "out/b.txt:L4-L5"},
				{From: "src/multi.txt:L9-L10", To: "out/c.txt"}, // whole-file dest
			},
		}},
	}

	stats, err := fileCopy.CopyMappings(tempDir, &vendor, vendor.Specs[0])
	if err != nil {
		t.Fatalf("CopyMappings failed: %v", err)
	}

	// === Verify CopyStats fields ===
	if len(stats.Positions) != 3 {
		t.Fatalf("expected 3 position records, got %d", len(stats.Positions))
	}
	if stats.FileCount != 3 {
		t.Errorf("expected FileCount=3, got %d", stats.FileCount)
	}

	// Verify From/To mapping correctness
	expectedMappings := map[string]string{
		"src/multi.txt:L1-L3":  "out/a.txt:L2-L3",
		"src/multi.txt:L5-L7":  "out/b.txt:L4-L5",
		"src/multi.txt:L9-L10": "out/c.txt",
	}
	for _, pos := range stats.Positions {
		expectedTo, ok := expectedMappings[pos.From]
		if !ok {
			t.Errorf("unexpected position From: %q", pos.From)
			continue
		}
		if pos.To != expectedTo {
			t.Errorf("position From=%q: To = %q, want %q", pos.From, pos.To, expectedTo)
		}
		if !strings.HasPrefix(pos.SourceHash, "sha256:") {
			t.Errorf("position From=%q: invalid hash format: %q", pos.From, pos.SourceHash)
		}
	}

	// Verify hash correctness
	expectedL1L3 := "line1\nline2\nline3"
	if stats.Positions[0].SourceHash != positionHash(expectedL1L3) {
		t.Errorf("hash mismatch for L1-L3: got %q, want %q",
			stats.Positions[0].SourceHash, positionHash(expectedL1L3))
	}

	// Verify destination file contents — out/a.txt had L2-L3 replaced
	destAData, err := os.ReadFile("out/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	destALines := strings.Split(string(destAData), "\n")
	if destALines[1] != "line1" {
		t.Errorf("destA line 2 = %q, want %q", destALines[1], "line1")
	}

	// Verify whole-file destination (out/c.txt created from scratch)
	destCData, err := os.ReadFile("out/c.txt")
	if err != nil {
		t.Fatalf("expected out/c.txt to exist: %v", err)
	}
	if string(destCData) != "line9\nline10" {
		t.Errorf("out/c.txt content = %q, want %q", string(destCData), "line9\nline10")
	}

	// Warnings: destA and destB had different content at target positions
	if len(stats.Warnings) < 2 {
		t.Errorf("expected at least 2 warnings for modified local content, got %d", len(stats.Warnings))
	}

	// === Verify CopyStats.Add aggregation behavior ===
	aggregate := CopyStats{}
	s1 := CopyStats{
		FileCount: 1, ByteCount: 50,
		Positions: []positionRecord{{From: "x:L1", To: "y", SourceHash: "sha256:aaa"}},
		Warnings:  []string{"warn1"},
	}
	s2 := CopyStats{
		FileCount: 2, ByteCount: 100,
		Positions: []positionRecord{
			{From: "a:L1-L5", To: "b", SourceHash: "sha256:bbb"},
			{From: "c:L2", To: "d:L3", SourceHash: "sha256:ccc"},
		},
		Warnings: []string{"warn2", "warn3"},
	}
	aggregate.Add(s1)
	aggregate.Add(s2)

	if aggregate.FileCount != 3 {
		t.Errorf("aggregate FileCount = %d, want 3", aggregate.FileCount)
	}
	if aggregate.ByteCount != 150 {
		t.Errorf("aggregate ByteCount = %d, want 150", aggregate.ByteCount)
	}
	if len(aggregate.Positions) != 3 {
		t.Errorf("aggregate Positions len = %d, want 3", len(aggregate.Positions))
	}
	if len(aggregate.Warnings) != 3 {
		t.Errorf("aggregate Warnings len = %d, want 3", len(aggregate.Warnings))
	}
}

// ============================================================================
// Test 5: Full pipeline — UpdateAll → lockfile persistence → verify → tamper → drift
// ============================================================================

func TestPositionIntegration_EndToEndWithUpdateService(t *testing.T) {
	sourceContent := "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta\ntheta\niota\nkappa\n"
	commitHash := "e2e111222333444555666777888999aaabbbcccd0"

	env := newPositionTestEnv(t, map[string]string{
		"greek/letters.txt": sourceContent,
	}, commitHash)

	vendor := types.VendorSpec{
		Name:    "greek-lib",
		URL:     "https://github.com/test/greek-lib",
		License: "MIT",
		Specs: []types.BranchSpec{{
			Ref: "v1.0",
			Mapping: []types.PathMapping{
				{From: "greek/letters.txt:L3-L5", To: "extracted/subset.txt"},
				{From: "greek/letters.txt:L8-L10", To: "extracted/tail.txt"},
			},
		}},
	}
	cfg := types.VendorConfig{Vendors: []types.VendorSpec{vendor}}
	if err := env.configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Run UpdateAll — triggers SyncVendor → builds lockfile with positions
	if err := env.updateSvc.UpdateAll(context.Background()); err != nil {
		t.Fatalf("UpdateAll failed: %v", err)
	}

	// Load lockfile and verify structure
	lock, err := env.lockStore.Load()
	if err != nil {
		t.Fatalf("load lockfile: %v", err)
	}
	if len(lock.Vendors) != 1 {
		t.Fatalf("expected 1 lock entry, got %d", len(lock.Vendors))
	}

	entry := lock.Vendors[0]
	if entry.Name != "greek-lib" {
		t.Errorf("lock entry name = %q, want greek-lib", entry.Name)
	}
	if entry.CommitHash != commitHash {
		t.Errorf("lock commit = %q, want %q", entry.CommitHash, commitHash)
	}
	if len(entry.Positions) != 2 {
		t.Fatalf("expected 2 position locks, got %d", len(entry.Positions))
	}

	// Verify position lock hashes match expected extracted content
	expectedSubset := "gamma\ndelta\nepsilon"
	if entry.Positions[0].From != "greek/letters.txt:L3-L5" {
		t.Errorf("position[0].From = %q", entry.Positions[0].From)
	}
	if entry.Positions[0].SourceHash != positionHash(expectedSubset) {
		t.Errorf("position[0].SourceHash = %q, want %q",
			entry.Positions[0].SourceHash, positionHash(expectedSubset))
	}

	expectedTail := "theta\niota\nkappa"
	if entry.Positions[1].From != "greek/letters.txt:L8-L10" {
		t.Errorf("position[1].From = %q", entry.Positions[1].From)
	}
	if entry.Positions[1].SourceHash != positionHash(expectedTail) {
		t.Errorf("position[1].SourceHash = %q, want %q",
			entry.Positions[1].SourceHash, positionHash(expectedTail))
	}

	// Verify destination files written with correct content
	subsetData, err := os.ReadFile("extracted/subset.txt")
	if err != nil {
		t.Fatalf("subset.txt not found: %v", err)
	}
	if string(subsetData) != expectedSubset {
		t.Errorf("subset.txt = %q, want %q", string(subsetData), expectedSubset)
	}

	tailData, err := os.ReadFile("extracted/tail.txt")
	if err != nil {
		t.Fatalf("tail.txt not found: %v", err)
	}
	if string(tailData) != expectedTail {
		t.Errorf("tail.txt = %q, want %q", string(tailData), expectedTail)
	}

	// Run verify — should pass
	result, err := env.verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS, got %s", result.Summary.Result)
	}
	if result.Summary.Verified < 2 {
		t.Errorf("expected >= 2 verified, got %d", result.Summary.Verified)
	}

	// Tamper with subset.txt → verify detects drift
	if err := os.WriteFile("extracted/subset.txt", []byte("tampered content"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = env.verifySvc.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify after tamper failed: %v", err)
	}
	if result.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL after tamper, got %s", result.Summary.Result)
	}
}
