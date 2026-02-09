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
//
// Each top-level test is a comprehensive suite with t.Run subtests covering
// happy paths, edge cases, and boundary conditions.
// ============================================================================

// positionTestProject sets up a reusable project directory with Manager.
// Caller owns cleanup via t.TempDir. Returns (projectDir, manager, cleanup).
// cleanup restores the original working directory.
func positionTestProject(t *testing.T) (string, *Manager, func()) {
	t.Helper()
	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(projectDir)

	manager := NewManager()
	manager.SetUICallback(&SilentUICallback{})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return projectDir, manager, func() { os.Chdir(origDir) }
}

// countVerifyByTypeStatus counts verify results by type and status.
func countVerifyByTypeStatus(result *types.VerifyResult, typ, status string) int {
	n := 0
	for _, f := range result.Files {
		if f.Type == typ && f.Status == status {
			n++
		}
	}
	return n
}

// findFileStatus finds the first FileStatus matching type and status.
func findFileStatus(result *types.VerifyResult, typ, status string) *types.FileStatus {
	for i := range result.Files {
		f := &result.Files[i]
		if f.Type == typ && f.Status == status {
			return f
		}
	}
	return nil
}

// ============================================================================
// Test 5: Cache skip path with positions
// ============================================================================

func TestIntegration_CacheSkipWithPositions(t *testing.T) {
	skipIfNoGit(t)

	// Shared source repo for all subtests that don't need their own
	srcRepo := createTestRepository(t, "cache-pos-src")
	writeFile(t, filepath.Join(srcRepo, "data.txt"),
		"line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Initial data")

	t.Run("BasicCacheHitPreservesPositions", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "pos-cache", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L2-L4", To: "extracted/snippet.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)

		// First sync
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		lock1, _ := manager.GetLock()
		posBefore := lock1.Vendors[0].Positions
		if len(posBefore) == 0 {
			t.Fatal("No position entries after first sync")
		}

		// Second sync (should cache-hit)
		if err := manager.Sync(); err != nil {
			t.Fatalf("Second sync: %v", err)
		}

		lock2, _ := manager.GetLock()
		posAfter := lock2.Vendors[0].Positions
		if len(posAfter) != len(posBefore) {
			t.Fatalf("Position count changed: %d → %d", len(posBefore), len(posAfter))
		}
		for i := range posAfter {
			if posAfter[i].From != posBefore[i].From ||
				posAfter[i].To != posBefore[i].To ||
				posAfter[i].SourceHash != posBefore[i].SourceHash {
				t.Errorf("Position[%d] changed across cache-hit sync", i)
			}
		}

		// Content intact
		content, _ := os.ReadFile(filepath.Join(projectDir, "extracted/snippet.txt"))
		if string(content) != "line2\nline3\nline4" {
			t.Errorf("Content = %q", string(content))
		}
	})

	t.Run("MultiplePositionMappingsCached", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "multi-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L1-L3", To: "out/first.txt"},
					{From: "data.txt:L5-L7", To: "out/second.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		lock1, _ := manager.GetLock()
		if len(lock1.Vendors[0].Positions) != 2 {
			t.Fatalf("Expected 2 positions, got %d", len(lock1.Vendors[0].Positions))
		}

		// Second sync
		if err := manager.Sync(); err != nil {
			t.Fatalf("Second sync: %v", err)
		}

		lock2, _ := manager.GetLock()
		if len(lock2.Vendors[0].Positions) != 2 {
			t.Fatalf("Position count after cache-hit: %d", len(lock2.Vendors[0].Positions))
		}
	})

	t.Run("ForceFlagBypassesCacheWithPositions", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "force-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L2-L4", To: "forced/snippet.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		// Force sync should still succeed
		if err := manager.SyncWithOptions("force-pos", true, false); err != nil {
			t.Fatalf("Force sync: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(projectDir, "forced/snippet.txt"))
		if string(content) != "line2\nline3\nline4" {
			t.Errorf("Content after force sync = %q", string(content))
		}

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) == 0 {
			t.Error("Positions lost after force sync")
		}
	})

	t.Run("NoCacheFlagBypassesCache", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "nocache-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L3-L5", To: "nc/snippet.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		// noCache sync
		if err := manager.SyncWithOptions("nocache-pos", false, true); err != nil {
			t.Fatalf("noCache sync: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(projectDir, "nc/snippet.txt"))
		if string(content) != "line3\nline4\nline5" {
			t.Errorf("Content after noCache sync = %q", string(content))
		}
	})

	t.Run("CacheInvalidationWhenDestModified", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "invalidate-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L1-L2", To: "inv/snippet.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		// Tamper with dest file to invalidate cache
		writeFile(t, filepath.Join(projectDir, "inv/snippet.txt"), "TAMPERED\n")

		// Third sync should re-download (cache invalidated by file content change)
		if err := manager.Sync(); err != nil {
			t.Fatalf("Sync after tamper: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(projectDir, "inv/snippet.txt"))
		if string(content) != "line1\nline2" {
			t.Errorf("Content after cache-invalidation sync = %q", string(content))
		}
	})

	t.Run("SingleLinePositionCached", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "single-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L5", To: "single/line.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(projectDir, "single/line.txt"))
		if string(content) != "line5" {
			t.Errorf("Single line content = %q", string(content))
		}

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) != 1 {
			t.Errorf("Expected 1 position, got %d", len(lock.Vendors[0].Positions))
		}

		// Second sync
		if err := manager.Sync(); err != nil {
			t.Fatalf("Second sync: %v", err)
		}

		lock2, _ := manager.GetLock()
		if len(lock2.Vendors[0].Positions) != 1 {
			t.Error("Single-line position lost after cache-hit sync")
		}
	})

	t.Run("EOFPositionCached", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "eof-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L6-EOF", To: "eof/tail.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("First sync: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(projectDir, "eof/tail.txt"))
		// Lines 6-EOF: "line6\nline7\nline8\n" (trailing newline → extra empty line)
		if !strings.HasPrefix(string(content), "line6\nline7\nline8") {
			t.Errorf("EOF content = %q", string(content))
		}

		// Second sync
		if err := manager.Sync(); err != nil {
			t.Fatalf("Second sync: %v", err)
		}

		lock2, _ := manager.GetLock()
		if len(lock2.Vendors[0].Positions) == 0 {
			t.Error("EOF position lost after cache-hit")
		}
	})

	t.Run("ThirdSyncStillCacheHit", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "triple-pos", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "data.txt:L4-L6", To: "triple/mid.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)

		for i := 1; i <= 3; i++ {
			if err := manager.Sync(); err != nil {
				t.Fatalf("Sync #%d: %v", i, err)
			}
		}

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) != 1 {
			t.Errorf("Position count after triple sync: %d", len(lock.Vendors[0].Positions))
		}
	})
}

// ============================================================================
// Test 6: Mixed whole-file and position mappings in one vendor
// ============================================================================

func TestIntegration_MixedWholeFileAndPositionMappings(t *testing.T) {
	skipIfNoGit(t)

	srcRepo := createTestRepository(t, "mixed-src")
	writeFile(t, filepath.Join(srcRepo, "whole.txt"), "whole file content here\n")
	writeFile(t, filepath.Join(srcRepo, "partial.txt"),
		"alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add mixed files")

	// Helper to set up a mixed project with position-on-destination
	setupMixed := func(t *testing.T, name string) (string, *Manager, func()) {
		t.Helper()
		projectDir, manager, cleanup := positionTestProject(t)

		// Pre-create dest file for position placement
		destPosFile := filepath.Join(projectDir, "dest/lib.go")
		writeFile(t, destPosFile,
			"line1\nline2\nPLACEHOLDER3\nPLACEHOLDER4\nPLACEHOLDER5\nline6\nline7\nline8\n")

		spec := &types.VendorSpec{
			Name: name, URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "whole.txt", To: "dest/whole.txt"},
					{From: "partial.txt:L2-L4", To: "dest/lib.go:L3-L5"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		return projectDir, manager, cleanup
	}

	t.Run("BothTypesInLockfile", func(t *testing.T) {
		_, manager, cleanup := setupMixed(t, "mixed-both")
		defer cleanup()

		lock, _ := manager.GetLock()
		entry := lock.Vendors[0]
		if len(entry.FileHashes) == 0 {
			t.Error("FileHashes empty")
		}
		if len(entry.Positions) == 0 {
			t.Error("Positions empty")
		}
	})

	t.Run("PositionContentPlacedCorrectly", func(t *testing.T) {
		projectDir, _, cleanup := setupMixed(t, "mixed-place")
		defer cleanup()

		libContent, _ := os.ReadFile(filepath.Join(projectDir, "dest/lib.go"))
		lines := strings.Split(string(libContent), "\n")
		if len(lines) < 5 {
			t.Fatalf("Only %d lines", len(lines))
		}
		if lines[2] != "beta" || lines[3] != "gamma" || lines[4] != "delta" {
			t.Errorf("L3-L5 = %q %q %q", lines[2], lines[3], lines[4])
		}
	})

	t.Run("SurroundingLinesPreserved", func(t *testing.T) {
		projectDir, _, cleanup := setupMixed(t, "mixed-surr")
		defer cleanup()

		libContent, _ := os.ReadFile(filepath.Join(projectDir, "dest/lib.go"))
		lines := strings.Split(string(libContent), "\n")
		if lines[0] != "line1" || lines[1] != "line2" {
			t.Error("Lines before position range modified")
		}
		if len(lines) > 5 && lines[5] != "line6" {
			t.Errorf("Line after position range = %q, want 'line6'", lines[5])
		}
	})

	t.Run("VerifyPassesInitially", func(t *testing.T) {
		_, manager, cleanup := setupMixed(t, "mixed-vpass")
		defer cleanup()

		result, err := manager.Verify()
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if countVerifyByTypeStatus(result, "file", "verified") == 0 {
			t.Error("No whole-file verified entries")
		}
		if countVerifyByTypeStatus(result, "position", "verified") == 0 {
			t.Error("No position verified entries")
		}
	})

	t.Run("WholeFileDriftIndependentOfPosition", func(t *testing.T) {
		projectDir, manager, cleanup := setupMixed(t, "mixed-wdrift")
		defer cleanup()

		writeFile(t, filepath.Join(projectDir, "dest/whole.txt"), "tampered\n")

		result, _ := manager.Verify()
		if result.Summary.Result != "FAIL" {
			t.Error("Expected FAIL")
		}

		var wholeStatus, posStatus string
		for _, f := range result.Files {
			if f.Type == "file" && strings.HasSuffix(f.Path, "whole.txt") {
				wholeStatus = f.Status
			}
			if f.Type == "position" {
				posStatus = f.Status
			}
		}
		if wholeStatus != "modified" {
			t.Errorf("Whole file = %q, want 'modified'", wholeStatus)
		}
		if posStatus != "verified" {
			t.Errorf("Position = %q, want 'verified'", posStatus)
		}
	})

	t.Run("PositionDriftIndependentOfWholeFile", func(t *testing.T) {
		projectDir, manager, cleanup := setupMixed(t, "mixed-pdrift")
		defer cleanup()

		// Tamper position range only
		writeFile(t, filepath.Join(projectDir, "dest/lib.go"),
			"line1\nline2\nTAMPERED\nTAMPERED\nTAMPERED\nline6\nline7\nline8\n")

		result, _ := manager.Verify()
		if result.Summary.Result != "FAIL" {
			t.Error("Expected FAIL")
		}

		var wholeStatus, posStatus string
		for _, f := range result.Files {
			if f.Type == "file" && strings.HasSuffix(f.Path, "whole.txt") {
				wholeStatus = f.Status
			}
			if f.Type == "position" {
				posStatus = f.Status
			}
		}
		if wholeStatus != "verified" {
			t.Errorf("Whole file = %q, want 'verified'", wholeStatus)
		}
		if posStatus != "modified" {
			t.Errorf("Position = %q, want 'modified'", posStatus)
		}
	})

	t.Run("BothModifiedSimultaneously", func(t *testing.T) {
		projectDir, manager, cleanup := setupMixed(t, "mixed-bothdrift")
		defer cleanup()

		writeFile(t, filepath.Join(projectDir, "dest/whole.txt"), "tampered\n")
		writeFile(t, filepath.Join(projectDir, "dest/lib.go"),
			"line1\nline2\nTAMPERED\nTAMPERED\nTAMPERED\nline6\nline7\nline8\n")

		result, _ := manager.Verify()
		if result.Summary.Result != "FAIL" {
			t.Error("Expected FAIL")
		}
		if result.Summary.Modified < 2 {
			t.Errorf("Modified count = %d, want >= 2", result.Summary.Modified)
		}
	})

	t.Run("WholeFileContentCorrect", func(t *testing.T) {
		projectDir, _, cleanup := setupMixed(t, "mixed-wcontent")
		defer cleanup()

		content, _ := os.ReadFile(filepath.Join(projectDir, "dest/whole.txt"))
		if string(content) != "whole file content here\n" {
			t.Errorf("Whole file content = %q", string(content))
		}
	})

	t.Run("PositionHashFormatValid", func(t *testing.T) {
		_, manager, cleanup := setupMixed(t, "mixed-hashfmt")
		defer cleanup()

		lock, _ := manager.GetLock()
		for _, pos := range lock.Vendors[0].Positions {
			if !strings.HasPrefix(pos.SourceHash, "sha256:") {
				t.Errorf("SourceHash missing sha256: prefix: %q", pos.SourceHash)
			}
			if len(pos.SourceHash) != 71 {
				t.Errorf("SourceHash length = %d, want 71", len(pos.SourceHash))
			}
		}
	})

	t.Run("SyncStatusReflectsBothTypes", func(t *testing.T) {
		_, manager, cleanup := setupMixed(t, "mixed-syncstat")
		defer cleanup()

		status, err := manager.CheckSyncStatus()
		if err != nil {
			t.Fatalf("CheckSyncStatus: %v", err)
		}
		if !status.AllSynced {
			t.Error("Expected AllSynced=true")
		}
		for _, vs := range status.VendorStatuses {
			if vs.FileCount != 2 {
				t.Errorf("FileCount = %d, want 2 (both mapping types)", vs.FileCount)
			}
			if vs.PositionCount == 0 {
				t.Error("PositionCount = 0, want > 0")
			}
		}
	})
}

// ============================================================================
// Test 7: Position verify after destination file deletion
// ============================================================================

func TestIntegration_PositionVerifyAfterDeletion(t *testing.T) {
	skipIfNoGit(t)

	srcRepo := createTestRepository(t, "pos-del-src")
	writeFile(t, filepath.Join(srcRepo, "source.txt"), "AAA\nBBB\nCCC\nDDD\nEEE\n")
	writeFile(t, filepath.Join(srcRepo, "extra.txt"), "X1\nX2\nX3\nX4\nX5\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add source")

	t.Run("DeletedStatusReported", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-test", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		targetFile, _ := filepath.Abs("out/target.txt")
		os.Remove(targetFile)

		result, _ := manager.Verify()
		if result.Summary.Result != "FAIL" {
			t.Errorf("Result = %q, want FAIL", result.Summary.Result)
		}
		if result.Summary.Deleted == 0 {
			t.Error("Deleted count = 0")
		}
	})

	t.Run("ExpectedHashPreservedInDeletedEntry", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-hash", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		lock, _ := manager.GetLock()
		expectedHash := lock.Vendors[0].Positions[0].SourceHash

		targetFile, _ := filepath.Abs("out/target.txt")
		os.Remove(targetFile)

		result, _ := manager.Verify()
		f := findFileStatus(result, "position", "deleted")
		if f == nil {
			t.Fatal("No deleted position entry found")
		}
		if f.ExpectedHash == nil || *f.ExpectedHash != expectedHash {
			t.Errorf("ExpectedHash = %v, want %q", f.ExpectedHash, expectedHash)
		}
	})

	t.Run("PositionDetailPresentOnDeleted", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-detail", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		targetFile, _ := filepath.Abs("out/target.txt")
		os.Remove(targetFile)

		result, _ := manager.Verify()
		f := findFileStatus(result, "position", "deleted")
		if f == nil {
			t.Fatal("No deleted position entry")
		}
		if f.Position == nil {
			t.Fatal("Position detail nil on deleted entry")
		}
		if f.Position.From != "source.txt:L2-L4" {
			t.Errorf("Position.From = %q", f.Position.From)
		}
		if f.Position.To != "out/target.txt" {
			t.Errorf("Position.To = %q", f.Position.To)
		}
		if f.Position.SourceHash == "" {
			t.Error("Position.SourceHash empty")
		}
	})

	t.Run("HashMatchesExpectedContent", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-match", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		lock, _ := manager.GetLock()
		storedHash := lock.Vendors[0].Positions[0].SourceHash

		expectedContent := "BBB\nCCC\nDDD"
		computed := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(expectedContent)))
		if storedHash != computed {
			t.Errorf("Stored hash %q != computed %q", storedHash, computed)
		}
	})

	t.Run("RecoveryAfterReSync", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-recover", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L2-L4", To: "out/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		targetFile := filepath.Join(projectDir, "out/target.txt")
		os.Remove(targetFile)

		result1, _ := manager.Verify()
		if result1.Summary.Deleted == 0 {
			t.Error("Expected deleted before re-sync")
		}

		// Re-sync restores
		manager.SyncWithOptions("del-recover", true, true)

		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			t.Error("File not restored after re-sync")
		}

		content, _ := os.ReadFile(targetFile)
		if string(content) != "BBB\nCCC\nDDD" {
			t.Errorf("Restored content = %q", string(content))
		}
	})

	t.Run("DeleteParentDirectory", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-parent", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L1-L3", To: "deep/nested/target.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		os.RemoveAll(filepath.Join(projectDir, "deep"))

		result, _ := manager.Verify()
		if result.Summary.Deleted == 0 {
			t.Error("Expected deleted when parent dir removed")
		}
	})

	t.Run("MultiplePositionsOneDeleted", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		// Pre-create dest for position-on-dest mapping
		writeFile(t, filepath.Join(projectDir, "pos/lib.go"),
			"P1\nP2\nP3\nP4\nP5\nP6\nP7\nP8\n")

		spec := &types.VendorSpec{
			Name: "del-multi", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L1-L2", To: "out/first.txt"},
					{From: "source.txt:L3-L4", To: "pos/lib.go:L3-L4"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		// Delete only the first file
		os.Remove(filepath.Join(projectDir, "out/first.txt"))

		result, _ := manager.Verify()
		deleted := countVerifyByTypeStatus(result, "position", "deleted")
		if deleted == 0 {
			t.Error("Expected at least one deleted position entry")
		}

		// The lib.go position should still be verified
		verified := countVerifyByTypeStatus(result, "position", "verified")
		if verified == 0 {
			t.Error("Expected at least one verified position (lib.go)")
		}
	})

	t.Run("VendorNameInDeletedEntry", func(t *testing.T) {
		_, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "del-vendor-name", URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "source.txt:L1", To: "out/single.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		os.Remove("out/single.txt")

		result, _ := manager.Verify()
		f := findFileStatus(result, "position", "deleted")
		if f == nil {
			t.Fatal("No deleted entry")
		}
		if f.Vendor == nil || *f.Vendor != "del-vendor-name" {
			t.Errorf("Vendor = %v, want 'del-vendor-name'", f.Vendor)
		}
	})
}

// ============================================================================
// Test 8: Re-sync after destination position range shifts
// ============================================================================

func TestIntegration_ReSyncAfterPositionRangeShift(t *testing.T) {
	skipIfNoGit(t)

	srcRepo := createTestRepository(t, "shift-src")
	writeFile(t, filepath.Join(srcRepo, "code.go"),
		"package main\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add code")

	// Helper to set up the shift test scenario
	setupShift := func(t *testing.T, name string) (string, *Manager, string, func()) {
		t.Helper()
		projectDir, manager, cleanup := positionTestProject(t)
		funcsFile := filepath.Join(projectDir, "lib/funcs.go")
		writeFile(t, funcsFile, "package lib\n\nOLD_A\nOLD_B\nOLD_C\n\n// footer\n")

		spec := &types.VendorSpec{
			Name: name, URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "code.go:L3-L5", To: "lib/funcs.go:L3-L5"},
				},
			}},
		}
		manager.SaveVendor(spec)
		if err := manager.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		return projectDir, manager, funcsFile, cleanup
	}

	t.Run("DriftDetectedAfterLineInsertion", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-detect")
		defer cleanup()

		// Insert lines at L3-L5, shifting synced content
		writeFile(t, funcsFile,
			"package lib\n\n// HEADER\n// EXTRA\nfunc A() {}\nfunc B() {}\nfunc C() {}\n\n// footer\n")

		result, _ := manager.Verify()
		if countVerifyByTypeStatus(result, "position", "modified") == 0 {
			t.Error("Expected modified position after line insertion")
		}
	})

	t.Run("ReSyncRestoresCorrectContent", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-restore")
		defer cleanup()

		writeFile(t, funcsFile,
			"package lib\n\n// HEADER\n// EXTRA\nfunc A() {}\nfunc B() {}\nfunc C() {}\n\n// footer\n")

		manager.SyncWithOptions("shift-restore", true, true)

		content, _ := os.ReadFile(funcsFile)
		lines := strings.Split(string(content), "\n")
		if lines[2] != "func A() {}" || lines[3] != "func B() {}" || lines[4] != "func C() {}" {
			t.Errorf("L3-L5 = %q %q %q", lines[2], lines[3], lines[4])
		}
	})

	t.Run("VerifyPassesAfterReSync", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-verify")
		defer cleanup()

		writeFile(t, funcsFile,
			"package lib\n\n// HEADER\nfunc A() {}\nfunc B() {}\nfunc C() {}\n\n// footer\n")

		manager.SyncWithOptions("shift-verify", true, true)

		result, _ := manager.Verify()
		if countVerifyByTypeStatus(result, "position", "verified") == 0 {
			t.Error("Expected verified after re-sync")
		}
	})

	t.Run("ContentBeforePositionPreserved", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-before")
		defer cleanup()

		// Tamper L3-L5
		writeFile(t, funcsFile,
			"package lib\n\nTAMPER1\nTAMPER2\nTAMPER3\n\n// footer\n")

		manager.SyncWithOptions("shift-before", true, true)

		content, _ := os.ReadFile(funcsFile)
		lines := strings.Split(string(content), "\n")
		if lines[0] != "package lib" || lines[1] != "" {
			t.Errorf("Lines before position modified: L1=%q L2=%q", lines[0], lines[1])
		}
	})

	t.Run("ContentAfterPositionPreserved", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-after")
		defer cleanup()

		writeFile(t, funcsFile,
			"package lib\n\nTAMPER1\nTAMPER2\nTAMPER3\n\n// footer\n")

		manager.SyncWithOptions("shift-after", true, true)

		content, _ := os.ReadFile(funcsFile)
		lines := strings.Split(string(content), "\n")
		// Lines after position (index 5+) should be preserved
		if len(lines) > 5 && lines[5] != "" {
			t.Errorf("Line 6 = %q, want empty", lines[5])
		}
		if len(lines) > 6 && lines[6] != "// footer" {
			t.Errorf("Line 7 = %q, want '// footer'", lines[6])
		}
	})

	t.Run("ReplacementWithShrunkContent", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-shrunk")
		defer cleanup()

		// Replace L3-L5 with MORE lines (simulates user expanding the range)
		writeFile(t, funcsFile,
			"package lib\n\nEX1\nEX2\nEX3\nEX4\nEX5\n\n// footer\n")

		manager.SyncWithOptions("shift-shrunk", true, true)

		content, _ := os.ReadFile(funcsFile)
		lines := strings.Split(string(content), "\n")
		// Re-sync should overwrite L3-L5 (indices 2-4) regardless of current content
		if lines[2] != "func A() {}" || lines[3] != "func B() {}" || lines[4] != "func C() {}" {
			t.Errorf("L3-L5 = %q %q %q", lines[2], lines[3], lines[4])
		}
	})

	t.Run("MultipleReSyncsIdempotent", func(t *testing.T) {
		_, manager, funcsFile, cleanup := setupShift(t, "shift-idempotent")
		defer cleanup()

		// Re-sync 3 times
		for i := 0; i < 3; i++ {
			manager.SyncWithOptions("shift-idempotent", true, true)
		}

		content, _ := os.ReadFile(funcsFile)
		lines := strings.Split(string(content), "\n")
		if lines[2] != "func A() {}" || lines[3] != "func B() {}" || lines[4] != "func C() {}" {
			t.Errorf("L3-L5 not stable after 3 re-syncs: %q %q %q", lines[2], lines[3], lines[4])
		}
	})

	t.Run("InitialSyncVerified", func(t *testing.T) {
		_, manager, _, cleanup := setupShift(t, "shift-initial")
		defer cleanup()

		result, _ := manager.Verify()
		if countVerifyByTypeStatus(result, "position", "verified") == 0 {
			t.Error("Initial sync not verified")
		}
	})
}

// ============================================================================
// Test 9: Hooks integration with position sync
// ============================================================================

func TestIntegration_HooksWithPositionSync(t *testing.T) {
	skipIfNoGit(t)

	srcRepo := createTestRepository(t, "hook-pos-src")
	writeFile(t, filepath.Join(srcRepo, "api.go"),
		"package api\n\nconst Version = \"1.0\"\nconst Name = \"test\"\nconst Debug = false\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add API")
	commitHash := getCommitHash(t, srcRepo)

	t.Run("PreSyncHookFires", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "pre.log")
		spec := &types.VendorSpec{
			Name: "hook-pre", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PreSync: fmt.Sprintf(`echo "FIRED" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/version.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Pre-sync hook log not found: %v", err)
		}
		if !strings.Contains(string(data), "FIRED") {
			t.Error("Pre-sync hook did not fire")
		}
	})

	t.Run("PostSyncHookFires", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "post.log")
		spec := &types.VendorSpec{
			Name: "hook-post", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "POST_FIRED" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L4", To: "lib/name.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Post-sync hook log not found: %v", err)
		}
		if !strings.Contains(string(data), "POST_FIRED") {
			t.Error("Post-sync hook did not fire")
		}
	})

	t.Run("VendorNameInEnv", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_name.log")
		spec := &types.VendorSpec{
			Name: "env-name-vendor", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_NAME" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		if !strings.Contains(string(data), "env-name-vendor") {
			t.Errorf("GIT_VENDOR_NAME = %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("VendorURLInEnv", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_url.log")
		spec := &types.VendorSpec{
			Name: "env-url", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_URL" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		if !strings.Contains(string(data), "file://"+srcRepo) {
			t.Errorf("GIT_VENDOR_URL = %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("FilesCopiedCountForPositions", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_files.log")
		spec := &types.VendorSpec{
			Name: "env-files", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_FILES_COPIED" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/version.txt"},
					{From: "api.go:L4", To: "lib/name.txt"},
					{From: "api.go:L5", To: "lib/debug.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		val := strings.TrimSpace(string(data))
		if val != "3" {
			t.Errorf("GIT_VENDOR_FILES_COPIED = %q, want '3'", val)
		}
	})

	t.Run("CommitHashInEnv", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_commit.log")
		spec := &types.VendorSpec{
			Name: "env-commit", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_COMMIT" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		val := strings.TrimSpace(string(data))
		if val != commitHash {
			t.Errorf("GIT_VENDOR_COMMIT = %q, want %q", val, commitHash)
		}
	})

	t.Run("RefInEnv", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_ref.log")
		spec := &types.VendorSpec{
			Name: "env-ref", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_REF" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		val := strings.TrimSpace(string(data))
		if val != "main" {
			t.Errorf("GIT_VENDOR_REF = %q, want 'main'", val)
		}
	})

	t.Run("RootDirInEnv", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "env_root.log")
		spec := &types.VendorSpec{
			Name: "env-root", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`echo "$GIT_VENDOR_ROOT" > %s`, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		val := strings.TrimSpace(string(data))
		// RootDir is set to VendorDir ("vendor") which is relative
		if val == "" {
			t.Error("GIT_VENDOR_ROOT is empty")
		}
	})

	t.Run("PreSyncHookFailureStopsSync", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		spec := &types.VendorSpec{
			Name: "hook-fail", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PreSync: "exit 1",
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/v.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)

		err := manager.Sync()
		if err == nil {
			t.Error("Expected sync to fail when pre-sync hook exits 1")
		}

		// File should NOT have been created
		if _, statErr := os.Stat(filepath.Join(projectDir, "lib/v.txt")); !os.IsNotExist(statErr) {
			t.Error("File created despite pre-sync hook failure")
		}
	})

	t.Run("PositionFilesCreatedBeforePostHook", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "post_check.log")
		vFile := filepath.Join(projectDir, "lib/version.txt")
		spec := &types.VendorSpec{
			Name: "hook-order", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{
				PostSync: fmt.Sprintf(`test -f %s && echo "EXISTS" > %s || echo "MISSING" > %s`,
					vFile, logFile, logFile),
			},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/version.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		if !strings.Contains(string(data), "EXISTS") {
			t.Errorf("Post-sync hook saw file as: %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("AllEnvVarsTogether", func(t *testing.T) {
		projectDir, manager, cleanup := positionTestProject(t)
		defer cleanup()

		logFile := filepath.Join(projectDir, "all_env.log")
		postCmd := fmt.Sprintf(
			`echo "NAME=$GIT_VENDOR_NAME" > %[1]s && `+
				`echo "URL=$GIT_VENDOR_URL" >> %[1]s && `+
				`echo "REF=$GIT_VENDOR_REF" >> %[1]s && `+
				`echo "COMMIT=$GIT_VENDOR_COMMIT" >> %[1]s && `+
				`echo "FILES=$GIT_VENDOR_FILES_COPIED" >> %[1]s`,
			logFile)

		spec := &types.VendorSpec{
			Name: "all-env", URL: "file://" + srcRepo, License: "MIT",
			Hooks: &types.HookConfig{PostSync: postCmd},
			Specs: []types.BranchSpec{{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "api.go:L3", To: "lib/version.txt"},
					{From: "api.go:L4", To: "lib/name.txt"},
				},
			}},
		}
		manager.SaveVendor(spec)
		manager.Sync()

		data, _ := os.ReadFile(logFile)
		log := string(data)
		checks := map[string]string{
			"NAME=all-env":              "GIT_VENDOR_NAME",
			"URL=file://" + srcRepo:     "GIT_VENDOR_URL",
			"REF=main":                  "GIT_VENDOR_REF",
			"COMMIT=" + commitHash:      "GIT_VENDOR_COMMIT",
			"FILES=2":                   "GIT_VENDOR_FILES_COPIED",
		}
		for expected, varName := range checks {
			if !strings.Contains(log, expected) {
				t.Errorf("%s incorrect in log:\n%s", varName, log)
			}
		}
	})
}

// ============================================================================
// Test 10: UpdateAll → lockfile serialization fidelity
// ============================================================================

func TestIntegration_UpdateAllLockfileSerializationFidelity(t *testing.T) {
	skipIfNoGit(t)

	srcRepo := createTestRepository(t, "serial-src")
	writeFile(t, filepath.Join(srcRepo, "config.yaml"),
		"key1: value1\nkey2: value2\nkey3: value3\nkey4: value4\nkey5: value5\n")
	writeFile(t, filepath.Join(srcRepo, "full.txt"), "complete file content\n")
	writeFile(t, filepath.Join(srcRepo, "LICENSE"), "MIT License\n\nPermission is hereby granted...")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Add config files")

	setupSerial := func(t *testing.T, name string, mappings []types.PathMapping) (*Manager, string) {
		t.Helper()
		projectDir, manager, cleanup := positionTestProject(t)
		t.Cleanup(cleanup)

		spec := &types.VendorSpec{
			Name: name, URL: "file://" + srcRepo, License: "MIT",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: mappings,
			}},
		}
		manager.SaveVendor(spec)
		return manager, filepath.Join(projectDir, "vendor", "vendor.lock")
	}

	t.Run("RawYAMLHasPositionsKey", func(t *testing.T) {
		_, lockPath := setupSerial(t, "serial-key", []types.PathMapping{
			{From: "full.txt", To: "out/full.txt"},
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		raw, _ := os.ReadFile(lockPath)
		if !strings.Contains(string(raw), "positions:") {
			t.Error("Raw YAML missing 'positions:' key")
		}
	})

	t.Run("FromFieldPreserved", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-from", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) == 0 {
			t.Fatal("No positions")
		}
		if lock.Vendors[0].Positions[0].From != "config.yaml:L2-L4" {
			t.Errorf("From = %q", lock.Vendors[0].Positions[0].From)
		}
	})

	t.Run("ToFieldPreserved", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-to", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) == 0 {
			t.Fatal("No positions")
		}
		if lock.Vendors[0].Positions[0].To != "out/partial.yaml" {
			t.Errorf("To = %q", lock.Vendors[0].Positions[0].To)
		}
	})

	t.Run("SourceHashFormatCorrect", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-hash", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		for _, pos := range lock.Vendors[0].Positions {
			if !strings.HasPrefix(pos.SourceHash, "sha256:") {
				t.Errorf("SourceHash missing prefix: %q", pos.SourceHash)
			}
			if len(pos.SourceHash) != 71 {
				t.Errorf("SourceHash length = %d", len(pos.SourceHash))
			}
		}
	})

	t.Run("RoundTripStructuralEquality", func(t *testing.T) {
		manager, lockPath := setupSerial(t, "serial-rt", []types.PathMapping{
			{From: "full.txt", To: "out/full.txt"},
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		rawYAML, _ := os.ReadFile(lockPath)
		var parsed types.VendorLock
		yaml.Unmarshal(rawYAML, &parsed)

		official, _ := manager.GetLock()

		if len(parsed.Vendors) != len(official.Vendors) {
			t.Fatalf("Vendor count: parsed=%d, official=%d",
				len(parsed.Vendors), len(official.Vendors))
		}
		for i := range parsed.Vendors {
			p := parsed.Vendors[i]
			o := official.Vendors[i]
			if p.Name != o.Name {
				t.Errorf("Vendor[%d].Name mismatch", i)
			}
			if p.CommitHash != o.CommitHash {
				t.Errorf("Vendor[%d].CommitHash mismatch", i)
			}
			if len(p.Positions) != len(o.Positions) {
				t.Errorf("Vendor[%d].Positions count mismatch", i)
				continue
			}
			for j := range p.Positions {
				if p.Positions[j].From != o.Positions[j].From ||
					p.Positions[j].To != o.Positions[j].To ||
					p.Positions[j].SourceHash != o.Positions[j].SourceHash {
					t.Errorf("Position[%d][%d] mismatch", i, j)
				}
			}
		}
	})

	t.Run("DoubleRoundTripFidelity", func(t *testing.T) {
		_, lockPath := setupSerial(t, "serial-drt", []types.PathMapping{
			{From: "config.yaml:L1-L3", To: "out/head.yaml"},
		})

		rawYAML, _ := os.ReadFile(lockPath)
		var pass1 types.VendorLock
		yaml.Unmarshal(rawYAML, &pass1)

		reSerial, _ := yaml.Marshal(&pass1)
		var pass2 types.VendorLock
		yaml.Unmarshal(reSerial, &pass2)

		if len(pass2.Vendors) != len(pass1.Vendors) {
			t.Fatal("Vendor count changed in double round-trip")
		}
		for i := range pass2.Vendors {
			if len(pass2.Vendors[i].Positions) != len(pass1.Vendors[i].Positions) {
				t.Errorf("Position count changed for vendor %d", i)
			}
			for j := range pass2.Vendors[i].Positions {
				p1 := pass1.Vendors[i].Positions[j]
				p2 := pass2.Vendors[i].Positions[j]
				if p1.From != p2.From || p1.To != p2.To || p1.SourceHash != p2.SourceHash {
					t.Errorf("Data loss in double round-trip at [%d][%d]", i, j)
				}
			}
		}
	})

	t.Run("FileHashesCoexistWithPositions", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-coexist", []types.PathMapping{
			{From: "full.txt", To: "out/full.txt"},
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		entry := lock.Vendors[0]
		if len(entry.FileHashes) == 0 {
			t.Error("FileHashes empty")
		}
		if len(entry.Positions) == 0 {
			t.Error("Positions empty")
		}
	})

	t.Run("MultiplePositionEntriesSerialized", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-multi", []types.PathMapping{
			{From: "config.yaml:L1-L2", To: "out/head.yaml"},
			{From: "config.yaml:L3-L5", To: "out/tail.yaml"},
		})

		lock, _ := manager.GetLock()
		if len(lock.Vendors[0].Positions) != 2 {
			t.Errorf("Position count = %d, want 2", len(lock.Vendors[0].Positions))
		}
	})

	t.Run("EmptyPositionsOmittedWhenNone", func(t *testing.T) {
		_, lockPath := setupSerial(t, "serial-nonepos", []types.PathMapping{
			{From: "full.txt", To: "out/full.txt"},
		})

		raw, _ := os.ReadFile(lockPath)
		if strings.Contains(string(raw), "positions:") {
			t.Error("positions: key present when no positions exist (should be omitempty)")
		}
	})

	t.Run("SchemaVersionPresent", func(t *testing.T) {
		_, lockPath := setupSerial(t, "serial-schema", []types.PathMapping{
			{From: "config.yaml:L1", To: "out/one.yaml"},
		})

		raw, _ := os.ReadFile(lockPath)
		if !strings.Contains(string(raw), "schema_version:") {
			t.Error("schema_version missing from lockfile")
		}
	})

	t.Run("MetadataFieldsPreserved", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-meta", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		entry := lock.Vendors[0]
		if entry.LicenseSPDX != "MIT" {
			t.Errorf("LicenseSPDX = %q", entry.LicenseSPDX)
		}
		if entry.Updated == "" {
			t.Error("Updated empty")
		}
		if entry.CommitHash == "" {
			t.Error("CommitHash empty")
		}
		if entry.VendoredAt == "" {
			t.Error("VendoredAt empty")
		}
		if entry.LastSyncedAt == "" {
			t.Error("LastSyncedAt empty")
		}
	})

	t.Run("RawYAMLFieldsPresent", func(t *testing.T) {
		_, lockPath := setupSerial(t, "serial-fields", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		raw := string(mustReadFile(t, lockPath))
		for _, field := range []string{"from:", "to:", "source_hash:"} {
			if !strings.Contains(raw, field) {
				t.Errorf("Raw YAML missing %q", field)
			}
		}
	})

	t.Run("SourceHashMatchesExtractedContent", func(t *testing.T) {
		manager, _ := setupSerial(t, "serial-content", []types.PathMapping{
			{From: "config.yaml:L2-L4", To: "out/partial.yaml"},
		})

		lock, _ := manager.GetLock()
		storedHash := lock.Vendors[0].Positions[0].SourceHash

		// L2-L4 of config.yaml = "key2: value2\nkey3: value3\nkey4: value4"
		expected := "key2: value2\nkey3: value3\nkey4: value4"
		computed := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(expected)))
		if storedHash != computed {
			t.Errorf("SourceHash = %q, computed = %q", storedHash, computed)
		}
	})
}

// mustReadFile reads a file or fails the test.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}
	return data
}
