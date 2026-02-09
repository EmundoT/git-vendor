//go:build integration

package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// Position Sync Integration Tests (Tests 5–10)
// ============================================================================

// TestIntegration_CacheSkipWithPositions verifies that syncing twice with the
// same commit hash hits the cache (canSkipSync returns true) AND that position
// lockfile entries are preserved across the cache-hit path.
func TestIntegration_CacheSkipWithPositions(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository with a file containing multiple lines
	srcRepo := createTestRepository(t, "cache-pos-src")
	sourceContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\n"
	writeFile(t, filepath.Join(srcRepo, "data.txt"), sourceContent)
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Initial data")

	// Setup project directory
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	spec := &types.VendorSpec{
		Name:    "pos-cache-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L2-L4", To: "extracted/snippet.txt"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// First sync — populates cache and lockfile
	err := manager.Sync()
	if err != nil {
		t.Fatalf("First sync failed: %v", err)
	}

	// Verify destination file was created with correct content
	snippetPath := filepath.Join(projectDir, "extracted/snippet.txt")
	content, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("Snippet file not found after first sync: %v", err)
	}
	expectedContent := "line2\nline3\nline4"
	if string(content) != expectedContent {
		t.Errorf("First sync content = %q, want %q", string(content), expectedContent)
	}

	// Check lockfile has position entries after first sync
	lock1, err := manager.GetLock()
	if err != nil {
		t.Fatalf("Failed to load lockfile after first sync: %v", err)
	}
	if len(lock1.Vendors) == 0 {
		t.Fatal("Lockfile has no vendor entries after first sync")
	}
	if len(lock1.Vendors[0].Positions) == 0 {
		t.Fatal("Lockfile has no position entries after first sync")
	}
	positionsBefore := lock1.Vendors[0].Positions

	// Second sync — should hit cache (same commit hash, files unchanged)
	err = manager.Sync()
	if err != nil {
		t.Fatalf("Second sync (cache hit) failed: %v", err)
	}

	// Verify position lockfile entries are preserved after cache-hit sync
	lock2, err := manager.GetLock()
	if err != nil {
		t.Fatalf("Failed to load lockfile after second sync: %v", err)
	}
	if len(lock2.Vendors) == 0 {
		t.Fatal("Lockfile lost vendor entries after cache-hit sync")
	}
	if len(lock2.Vendors[0].Positions) == 0 {
		t.Fatal("Position entries lost after cache-hit sync — cache path must preserve positions")
	}

	// Verify position data integrity
	positionsAfter := lock2.Vendors[0].Positions
	if len(positionsAfter) != len(positionsBefore) {
		t.Errorf("Position count changed: before=%d, after=%d", len(positionsBefore), len(positionsAfter))
	}
	for i, pos := range positionsAfter {
		if i >= len(positionsBefore) {
			break
		}
		if pos.From != positionsBefore[i].From {
			t.Errorf("Position[%d].From = %q, want %q", i, pos.From, positionsBefore[i].From)
		}
		if pos.To != positionsBefore[i].To {
			t.Errorf("Position[%d].To = %q, want %q", i, pos.To, positionsBefore[i].To)
		}
		if pos.SourceHash != positionsBefore[i].SourceHash {
			t.Errorf("Position[%d].SourceHash = %q, want %q", i, pos.SourceHash, positionsBefore[i].SourceHash)
		}
	}

	// Verify destination file is still intact
	content2, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("Snippet file disappeared after cache-hit sync: %v", err)
	}
	if string(content2) != expectedContent {
		t.Errorf("Content after cache-hit sync = %q, want %q", string(content2), expectedContent)
	}
}

// TestIntegration_MixedWholeFileAndPositionMappings verifies that a single
// vendor with both whole-file and position mappings produces correct FileHashes
// AND Positions in the lockfile, and that verify detects drift independently
// for each type. Uses position-on-destination so verify hash formats align.
func TestIntegration_MixedWholeFileAndPositionMappings(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository with files
	srcRepo := createTestRepository(t, "mixed-src")
	writeFile(t, filepath.Join(srcRepo, "whole.txt"), "whole file content here\n")
	writeFile(t, filepath.Join(srcRepo, "partial.txt"), "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add mixed files")

	// Setup project
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	// Pre-create destination file for the position mapping so PlaceContent can
	// replace a range within it. 8 placeholder lines give room for L3-L5.
	destPosFile := filepath.Join(projectDir, "dest/lib.go")
	writeFile(t, destPosFile, "line1\nline2\nPLACEHOLDER3\nPLACEHOLDER4\nPLACEHOLDER5\nline6\nline7\nline8\n")

	spec := &types.VendorSpec{
		Name:    "mixed-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "whole.txt", To: "dest/whole.txt"},
					{From: "partial.txt:L2-L4", To: "dest/lib.go:L3-L5"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// Sync
	err := manager.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify both files exist
	wholeFile := filepath.Join(projectDir, "dest/whole.txt")
	if _, err := os.Stat(wholeFile); os.IsNotExist(err) {
		t.Error("Whole file not synced")
	}
	if _, err := os.Stat(destPosFile); os.IsNotExist(err) {
		t.Error("Position-mapped file not found")
	}

	// Verify position content was placed correctly at L3-L5
	libContent, _ := os.ReadFile(destPosFile)
	libLines := strings.Split(string(libContent), "\n")
	// Lines 3-5 (0-indexed 2-4) should now be beta, gamma, delta
	if len(libLines) < 5 {
		t.Fatalf("dest/lib.go has only %d lines, expected at least 5", len(libLines))
	}
	if libLines[2] != "beta" || libLines[3] != "gamma" || libLines[4] != "delta" {
		t.Errorf("Position content not placed correctly. Lines 3-5 = %q, %q, %q",
			libLines[2], libLines[3], libLines[4])
	}
	// Lines before and after should be preserved
	if libLines[0] != "line1" || libLines[1] != "line2" {
		t.Error("Lines before position range were not preserved")
	}

	// Check lockfile has both FileHashes and Positions
	lock, err := manager.GetLock()
	if err != nil {
		t.Fatalf("Failed to load lockfile: %v", err)
	}
	if len(lock.Vendors) == 0 {
		t.Fatal("No vendor entries in lockfile")
	}

	entry := lock.Vendors[0]
	if len(entry.FileHashes) == 0 {
		t.Error("FileHashes is empty — whole-file mapping not tracked")
	}
	if len(entry.Positions) == 0 {
		t.Error("Positions is empty — position mapping not tracked")
	}

	// Verify should pass initially
	result, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	// Count verified entries by type
	fileVerified, posVerified := 0, 0
	for _, f := range result.Files {
		if f.Status == "verified" && f.Type == "file" {
			fileVerified++
		}
		if f.Status == "verified" && f.Type == "position" {
			posVerified++
		}
	}
	if fileVerified == 0 {
		t.Error("No whole-file entries verified initially")
	}
	if posVerified == 0 {
		t.Error("No position entries verified initially")
	}

	// Modify ONLY the whole file — position should remain verified
	writeFile(t, wholeFile, "tampered whole file\n")

	result2, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify after whole-file modification failed: %v", err)
	}
	if result2.Summary.Result != "FAIL" {
		t.Error("Expected FAIL after modifying whole file")
	}

	var wholeStatus, posStatus string
	for _, f := range result2.Files {
		if f.Type == "file" && strings.HasSuffix(f.Path, "whole.txt") {
			wholeStatus = f.Status
		}
		if f.Type == "position" {
			posStatus = f.Status
		}
	}
	if wholeStatus != "modified" {
		t.Errorf("Whole file status = %q, want 'modified'", wholeStatus)
	}
	if posStatus != "verified" {
		t.Errorf("Position status = %q, want 'verified' (drift should be independent)", posStatus)
	}

	// Restore whole file, tamper ONLY the position range in dest/lib.go
	writeFile(t, wholeFile, "whole file content here\n")
	// Rewrite lib.go with tampered content at L3-L5
	writeFile(t, destPosFile, "line1\nline2\nTAMPERED\nTAMPERED\nTAMPERED\nline6\nline7\nline8\n")

	result3, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify after position modification failed: %v", err)
	}
	if result3.Summary.Result != "FAIL" {
		t.Error("Expected FAIL after modifying position range")
	}

	var wholeStatus2, posStatus2 string
	for _, f := range result3.Files {
		if f.Type == "file" && strings.HasSuffix(f.Path, "whole.txt") {
			wholeStatus2 = f.Status
		}
		if f.Type == "position" {
			posStatus2 = f.Status
		}
	}
	if wholeStatus2 != "verified" {
		t.Errorf("Whole file status = %q, want 'verified' after restore", wholeStatus2)
	}
	if posStatus2 != "modified" {
		t.Errorf("Position status = %q, want 'modified' after position tamper", posStatus2)
	}
}

// TestIntegration_PositionVerifyAfterDeletion verifies that when a destination
// file with a position mapping is deleted entirely, verify reports "deleted"
// status with the correct expected hash.
func TestIntegration_PositionVerifyAfterDeletion(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository
	srcRepo := createTestRepository(t, "pos-delete-src")
	writeFile(t, filepath.Join(srcRepo, "source.txt"), "AAA\nBBB\nCCC\nDDD\nEEE\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add source")

	// Setup project
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	spec := &types.VendorSpec{
		Name:    "delete-test-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// Sync
	err := manager.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify file exists and verify passes
	targetFile := filepath.Join(projectDir, "out/target.txt")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Fatal("Target file not created by sync")
	}

	// Capture the expected hash from lockfile
	lock, err := manager.GetLock()
	if err != nil {
		t.Fatalf("Failed to load lockfile: %v", err)
	}
	if len(lock.Vendors[0].Positions) == 0 {
		t.Fatal("No position entries in lockfile")
	}
	expectedHash := lock.Vendors[0].Positions[0].SourceHash
	if expectedHash == "" {
		t.Fatal("SourceHash is empty in lockfile position entry")
	}

	// Verify the hash matches the expected extracted content
	expectedContent := "BBB\nCCC\nDDD"
	computedHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(expectedContent)))
	if expectedHash != computedHash {
		t.Errorf("Lockfile hash = %q, computed from expected content = %q", expectedHash, computedHash)
	}

	// Delete the destination file entirely
	if err := os.Remove(targetFile); err != nil {
		t.Fatalf("Failed to delete target file: %v", err)
	}

	// Verify should report "deleted" status
	result, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify after deletion failed: %v", err)
	}
	if result.Summary.Result != "FAIL" {
		t.Errorf("Expected FAIL after deleting position target, got %s", result.Summary.Result)
	}
	if result.Summary.Deleted == 0 {
		t.Error("Expected at least one deleted entry in verify summary")
	}

	// Find the position entry and check its details
	var found bool
	for _, f := range result.Files {
		if f.Type == "position" && f.Status == "deleted" {
			found = true
			if f.ExpectedHash == nil || *f.ExpectedHash != expectedHash {
				wantHash := expectedHash
				gotHash := "<nil>"
				if f.ExpectedHash != nil {
					gotHash = *f.ExpectedHash
				}
				t.Errorf("Deleted position expected_hash = %q, want %q", gotHash, wantHash)
			}
			if f.Position == nil {
				t.Error("Deleted position entry missing Position detail")
			} else if f.Position.SourceHash != expectedHash {
				t.Errorf("Position.SourceHash = %q, want %q", f.Position.SourceHash, expectedHash)
			}
			break
		}
	}
	if !found {
		t.Error("No 'deleted' position entry found in verify results")
		for _, f := range result.Files {
			t.Logf("  File: path=%s type=%s status=%s", f.Path, f.Type, f.Status)
		}
	}
}

// TestIntegration_ReSyncAfterPositionRangeShift verifies that when a user edits
// the destination file (inserting lines above the position range), verify
// detects drift, and a re-sync overwrites the original position coordinates
// correctly. Uses position-on-destination to enable accurate verify.
func TestIntegration_ReSyncAfterPositionRangeShift(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository
	srcRepo := createTestRepository(t, "shift-src")
	writeFile(t, filepath.Join(srcRepo, "code.go"), "package main\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add code")

	// Setup project
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	// Pre-create destination file with placeholder content (8 lines)
	funcsFile := filepath.Join(projectDir, "lib/funcs.go")
	writeFile(t, funcsFile, "package lib\n\nOLD_A\nOLD_B\nOLD_C\n\n// footer\n")

	// Map source L3-L5 (func A, B, C) into dest L3-L5
	spec := &types.VendorSpec{
		Name:    "shift-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "code.go:L3-L5", To: "lib/funcs.go:L3-L5"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// Initial sync
	err := manager.Sync()
	if err != nil {
		t.Fatalf("Initial sync failed: %v", err)
	}

	// Read and verify dest L3-L5 contain the source functions
	destContent, err := os.ReadFile(funcsFile)
	if err != nil {
		t.Fatalf("Failed to read synced file: %v", err)
	}
	lines := strings.Split(string(destContent), "\n")
	if len(lines) < 5 {
		t.Fatalf("Dest file has only %d lines after sync", len(lines))
	}
	if lines[2] != "func A() {}" || lines[3] != "func B() {}" || lines[4] != "func C() {}" {
		t.Errorf("Lines 3-5 after sync = %q, %q, %q; want func A/B/C",
			lines[2], lines[3], lines[4])
	}

	// Verify passes initially
	result1, err := manager.Verify()
	if err != nil {
		t.Fatalf("Initial verify failed: %v", err)
	}
	initialPositionVerified := false
	for _, f := range result1.Files {
		if f.Type == "position" && f.Status == "verified" {
			initialPositionVerified = true
		}
	}
	if !initialPositionVerified {
		t.Error("Expected position to be verified after initial sync")
	}

	// User edits the destination file: insert lines before L3, shifting the synced content down.
	// Lines 3-5 now have the inserted headers, not the synced functions.
	writeFile(t, funcsFile, "package lib\n\n// INSERTED HEADER\n// ANOTHER LINE\nfunc A() {}\nfunc B() {}\nfunc C() {}\n\n// footer\n")

	// Verify should detect drift (L3-L5 now contain inserted headers, not the source content)
	result2, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify after edit failed: %v", err)
	}

	positionDrifted := false
	for _, f := range result2.Files {
		if f.Type == "position" && f.Status == "modified" {
			positionDrifted = true
		}
	}
	if !positionDrifted {
		t.Error("Expected position to be 'modified' after inserting lines at target range")
	}

	// Re-sync should overwrite L3-L5 at the original position coordinates
	err = manager.SyncWithOptions("shift-vendor", true, true)
	if err != nil {
		t.Fatalf("Re-sync failed: %v", err)
	}

	// Verify L3-L5 are restored to the source functions
	restoredContent, err := os.ReadFile(funcsFile)
	if err != nil {
		t.Fatalf("Failed to read re-synced file: %v", err)
	}
	restoredLines := strings.Split(string(restoredContent), "\n")
	if len(restoredLines) < 5 {
		t.Fatalf("Re-synced file has only %d lines", len(restoredLines))
	}
	if restoredLines[2] != "func A() {}" || restoredLines[3] != "func B() {}" || restoredLines[4] != "func C() {}" {
		t.Errorf("Re-synced L3-L5 = %q, %q, %q; want func A/B/C",
			restoredLines[2], restoredLines[3], restoredLines[4])
	}

	// Verify should pass again after re-sync
	result3, err := manager.Verify()
	if err != nil {
		t.Fatalf("Verify after re-sync failed: %v", err)
	}
	restoredVerified := false
	for _, f := range result3.Files {
		if f.Type == "position" && f.Status == "verified" {
			restoredVerified = true
		}
	}
	if !restoredVerified {
		t.Error("Expected position to be verified after re-sync")
	}
}

// TestIntegration_HooksWithPositionSync verifies that pre/post hooks fire
// with correct GIT_VENDOR_FILES_COPIED count and environment variables
// when syncing with position-mapped files.
func TestIntegration_HooksWithPositionSync(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository
	srcRepo := createTestRepository(t, "hook-pos-src")
	writeFile(t, filepath.Join(srcRepo, "api.go"), "package api\n\nconst Version = \"1.0\"\nconst Name = \"test\"\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add API")
	commitHash := getCommitHash(t, srcRepo)

	// Setup project
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	// Create hook scripts that dump env vars to files for verification
	preSyncLog := filepath.Join(projectDir, "pre_sync.log")
	postSyncLog := filepath.Join(projectDir, "post_sync.log")

	preCmd := fmt.Sprintf(`echo "PRE_NAME=$GIT_VENDOR_NAME" > %s && echo "PRE_URL=$GIT_VENDOR_URL" >> %s`,
		preSyncLog, preSyncLog)
	postCmd := fmt.Sprintf(`echo "POST_NAME=$GIT_VENDOR_NAME" > %s && echo "POST_FILES=$GIT_VENDOR_FILES_COPIED" >> %s && echo "POST_COMMIT=$GIT_VENDOR_COMMIT" >> %s`,
		postSyncLog, postSyncLog, postSyncLog)

	spec := &types.VendorSpec{
		Name:    "hook-pos-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Hooks: &types.HookConfig{
			PreSync:  preCmd,
			PostSync: postCmd,
		},
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/version.txt"},
					{From: "api.go:L4", To: "lib/name.txt"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// Sync (UpdateAll already ran via SaveVendor, so we just sync)
	err := manager.Sync()
	if err != nil {
		t.Fatalf("Sync with hooks failed: %v", err)
	}

	// Verify pre-sync hook fired with correct env
	preData, err := os.ReadFile(preSyncLog)
	if err != nil {
		t.Fatalf("Pre-sync hook log not found: %v", err)
	}
	preLog := string(preData)
	if !strings.Contains(preLog, "PRE_NAME=hook-pos-vendor") {
		t.Errorf("Pre-sync hook GIT_VENDOR_NAME incorrect:\n%s", preLog)
	}
	if !strings.Contains(preLog, "PRE_URL=file://"+srcRepo) {
		t.Errorf("Pre-sync hook GIT_VENDOR_URL incorrect:\n%s", preLog)
	}

	// Verify post-sync hook fired with correct env
	postData, err := os.ReadFile(postSyncLog)
	if err != nil {
		t.Fatalf("Post-sync hook log not found: %v", err)
	}
	postLog := string(postData)
	if !strings.Contains(postLog, "POST_NAME=hook-pos-vendor") {
		t.Errorf("Post-sync hook GIT_VENDOR_NAME incorrect:\n%s", postLog)
	}
	// Two position mappings → 2 files copied
	if !strings.Contains(postLog, "POST_FILES=2") {
		t.Errorf("Post-sync hook GIT_VENDOR_FILES_COPIED should be 2:\n%s", postLog)
	}
	if !strings.Contains(postLog, "POST_COMMIT="+commitHash) {
		t.Errorf("Post-sync hook GIT_VENDOR_COMMIT should contain commit hash %s:\n%s", commitHash[:7], postLog)
	}

	// Verify the position-mapped files were actually created
	versionFile := filepath.Join(projectDir, "lib/version.txt")
	nameFile := filepath.Join(projectDir, "lib/name.txt")
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		t.Error("Position-mapped version.txt not created")
	}
	if _, err := os.Stat(nameFile); os.IsNotExist(err) {
		t.Error("Position-mapped name.txt not created")
	}

	vContent, _ := os.ReadFile(versionFile)
	if !strings.Contains(string(vContent), "Version") {
		t.Errorf("version.txt content = %q, expected to contain 'Version'", string(vContent))
	}
}

// TestIntegration_UpdateAllLockfileSerializationFidelity verifies that after
// UpdateAll, the raw vendor.lock YAML serializes positions: entries correctly
// with from:, to:, source_hash: fields, and that deserializing back produces
// structurally identical data (no data loss through YAML round-trip).
func TestIntegration_UpdateAllLockfileSerializationFidelity(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository with multiple files for mixed mappings
	srcRepo := createTestRepository(t, "serial-src")
	writeFile(t, filepath.Join(srcRepo, "config.yaml"), "key1: value1\nkey2: value2\nkey3: value3\nkey4: value4\nkey5: value5\n")
	writeFile(t, filepath.Join(srcRepo, "full.txt"), "complete file content\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add config files")

	// Setup project
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	manager.Init()

	spec := &types.VendorSpec{
		Name:    "serial-vendor",
		URL:     "file://" + srcRepo,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "full.txt", To: "out/full.txt"},
					{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
				},
			},
		},
	}
	manager.SaveVendor(spec)

	// UpdateAll is called implicitly by SaveVendor
	// Read raw lockfile YAML
	lockPath := filepath.Join(projectDir, "vendor", "vendor.lock")
	rawYAML, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("Failed to read raw vendor.lock: %v", err)
	}

	rawStr := string(rawYAML)

	// Verify raw YAML contains position fields
	if !strings.Contains(rawStr, "positions:") {
		t.Error("Raw YAML missing 'positions:' key")
	}
	if !strings.Contains(rawStr, "from:") {
		t.Error("Raw YAML missing 'from:' field in positions")
	}
	if !strings.Contains(rawStr, "to:") {
		t.Error("Raw YAML missing 'to:' field in positions")
	}
	if !strings.Contains(rawStr, "source_hash:") {
		t.Error("Raw YAML missing 'source_hash:' field in positions")
	}

	// Deserialize from raw YAML
	var parsedLock types.VendorLock
	if err := yaml.Unmarshal(rawYAML, &parsedLock); err != nil {
		t.Fatalf("Failed to unmarshal raw vendor.lock: %v", err)
	}

	// Load through the official lockStore for structural comparison
	officialLock, err := manager.GetLock()
	if err != nil {
		t.Fatalf("Failed to load lockfile through manager: %v", err)
	}

	// Structural comparison: vendor count
	if len(parsedLock.Vendors) != len(officialLock.Vendors) {
		t.Fatalf("Vendor count mismatch: parsed=%d, official=%d",
			len(parsedLock.Vendors), len(officialLock.Vendors))
	}

	for i, parsed := range parsedLock.Vendors {
		official := officialLock.Vendors[i]

		// Compare basic fields
		if parsed.Name != official.Name {
			t.Errorf("Vendor[%d].Name: parsed=%q, official=%q", i, parsed.Name, official.Name)
		}
		if parsed.CommitHash != official.CommitHash {
			t.Errorf("Vendor[%d].CommitHash: parsed=%q, official=%q", i, parsed.CommitHash, official.CommitHash)
		}

		// Compare positions (the critical assertion)
		if len(parsed.Positions) != len(official.Positions) {
			t.Errorf("Vendor[%d].Positions count: parsed=%d, official=%d",
				i, len(parsed.Positions), len(official.Positions))
			continue
		}

		for j, pPos := range parsed.Positions {
			oPos := official.Positions[j]
			if pPos.From != oPos.From {
				t.Errorf("Position[%d][%d].From: parsed=%q, official=%q", i, j, pPos.From, oPos.From)
			}
			if pPos.To != oPos.To {
				t.Errorf("Position[%d][%d].To: parsed=%q, official=%q", i, j, pPos.To, oPos.To)
			}
			if pPos.SourceHash != oPos.SourceHash {
				t.Errorf("Position[%d][%d].SourceHash: parsed=%q, official=%q", i, j, pPos.SourceHash, oPos.SourceHash)
			}

			// Verify SourceHash is non-empty and correctly formatted
			if !strings.HasPrefix(oPos.SourceHash, "sha256:") {
				t.Errorf("Position[%d][%d].SourceHash doesn't start with 'sha256:': %q", i, j, oPos.SourceHash)
			}
			if len(oPos.SourceHash) != 71 { // "sha256:" (7) + 64 hex chars
				t.Errorf("Position[%d][%d].SourceHash has wrong length %d (expected 71): %q",
					i, j, len(oPos.SourceHash), oPos.SourceHash)
			}
		}

		// Verify FileHashes also round-trips correctly
		if len(parsed.FileHashes) != len(official.FileHashes) {
			t.Errorf("Vendor[%d].FileHashes count: parsed=%d, official=%d",
				i, len(parsed.FileHashes), len(official.FileHashes))
		}
		for path, pHash := range parsed.FileHashes {
			oHash, ok := official.FileHashes[path]
			if !ok {
				t.Errorf("Vendor[%d].FileHashes: path %q in parsed but not official", i, path)
				continue
			}
			if pHash != oHash {
				t.Errorf("Vendor[%d].FileHashes[%s]: parsed=%q, official=%q", i, path, pHash, oHash)
			}
		}
	}

	// Final: serialize back from parsed struct and compare to original (idempotency)
	reSerializedYAML, err := yaml.Marshal(&parsedLock)
	if err != nil {
		t.Fatalf("Failed to re-serialize parsed lock: %v", err)
	}

	// Re-parse the re-serialized YAML
	var reParsedLock types.VendorLock
	if err := yaml.Unmarshal(reSerializedYAML, &reParsedLock); err != nil {
		t.Fatalf("Failed to re-parse re-serialized YAML: %v", err)
	}

	// Ensure positions survive the double round-trip
	if len(reParsedLock.Vendors) != len(parsedLock.Vendors) {
		t.Fatalf("Double round-trip vendor count mismatch")
	}
	for i, rp := range reParsedLock.Vendors {
		p := parsedLock.Vendors[i]
		if len(rp.Positions) != len(p.Positions) {
			t.Errorf("Double round-trip position count mismatch for vendor %d", i)
		}
		for j, rpPos := range rp.Positions {
			pPos := p.Positions[j]
			if rpPos.From != pPos.From || rpPos.To != pPos.To || rpPos.SourceHash != pPos.SourceHash {
				t.Errorf("Double round-trip position[%d][%d] data loss: got {%s,%s,%s}, want {%s,%s,%s}",
					i, j, rpPos.From, rpPos.To, rpPos.SourceHash, pPos.From, pPos.To, pPos.SourceHash)
			}
		}
	}
}
