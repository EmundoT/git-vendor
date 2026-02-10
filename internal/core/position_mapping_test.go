package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Test 1: Column-Precise Extraction Round-Trip
//
// L5C10:L5C30 extraction → sync → lockfile → verify → tamper single column → drift detection.
// Validates that column-level precision is preserved through the entire pipeline.
// ============================================================================

func TestColumnPrecise_RoundTrip_SyncVerifyTamperDrift(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Source file with a line long enough for column extraction.
	// Line 5: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" (36 chars)
	var srcLines []string
	for i := 1; i <= 10; i++ {
		if i == 5 {
			srcLines = append(srcLines, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		} else {
			srcLines = append(srcLines, "padding-line")
		}
	}
	srcContent := strings.Join(srcLines, "\n") + "\n"
	srcPath := filepath.Join(repoDir, "source.go")
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Step 1: Extract L5C10:L5C30 from source
	srcPos := &types.PositionSpec{StartLine: 5, EndLine: 5, StartCol: 10, EndCol: 30}
	extracted, srcHash, err := ExtractPosition(srcPath, srcPos)
	if err != nil {
		t.Fatalf("extract from source: %v", err)
	}
	// line[StartCol-1 : EndCol] = line[9:30] = "9ABCDEFGHIJKLMNOPQRST" (21 chars)
	wantExtracted := "9ABCDEFGHIJKLMNOPQRST"
	if extracted != wantExtracted {
		t.Fatalf("extracted = %q, want %q", extracted, wantExtracted)
	}

	// Step 2: Pre-create destination file and place extracted content at column position.
	// Dest line 1 = 31 chars: 5 prefix + 21 placeholder + 5 suffix.
	// Placing 21 chars at C6:C26 replaces cols 6-26 (line[5:26]) with the extracted content.
	destPath := filepath.Join(workDir, "dest.go")
	destInitial := "AAAAA_____________________BBBBB\nline2\n"
	if err := os.WriteFile(destPath, []byte(destInitial), 0644); err != nil {
		t.Fatal(err)
	}

	destPos := &types.PositionSpec{StartLine: 1, EndLine: 1, StartCol: 6, EndCol: 26}
	if err := PlaceContent(destPath, extracted, destPos); err != nil {
		t.Fatalf("place content: %v", err)
	}

	// Verify destination line 1 = "AAAAA" + extracted + "BBBBB"
	afterPlace, _ := os.ReadFile(destPath)
	wantDest := "AAAAA9ABCDEFGHIJKLMNOPQRSTBBBBB\nline2\n"
	if string(afterPlace) != wantDest {
		t.Fatalf("after place = %q, want %q", string(afterPlace), wantDest)
	}

	// Step 3: Verify content round-trips through extraction at the destination
	reExtracted, reHash, err := ExtractPosition(destPath, destPos)
	if err != nil {
		t.Fatalf("re-extract from dest: %v", err)
	}
	if reExtracted != extracted {
		t.Errorf("round-trip content mismatch: got %q, want %q", reExtracted, extracted)
	}
	if reHash != srcHash {
		t.Errorf("round-trip hash mismatch: got %q, want %q", reHash, srcHash)
	}

	// Step 4: Simulate lockfile with position lock
	posLock := types.PositionLock{
		From:       "source.go:L5C10:L5C30",
		To:         destPath + ":L1C6:L1C26",
		SourceHash: srcHash,
	}

	// Step 5: Verify through VerifyService — should PASS
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	realCache := NewFileCacheStore(NewOSFileSystem(), workDir)
	wholeFileHash, err := realCache.ComputeFileChecksum(destPath)
	if err != nil {
		t.Fatalf("compute whole-file hash: %v", err)
	}

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "col-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "source.go:L5C10:L5C30", To: destPath + ":L1C6:L1C26"}},
			}},
		}},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "col-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destPath: wholeFileHash},
			Positions:  []types.PositionLock{posLock},
		}},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), workDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS before tamper, got %s", result.Summary.Result)
	}

	// Step 6: Tamper a single column within the extracted range.
	// Line 1 = "AAAAA9ABCDEFGHIJKLMNOPQRSTBBBBB" — placed content is at indices 5-25.
	// Tamper index 10 (col 11, inside the placed region).
	destData, _ := os.ReadFile(destPath)
	tampered := []byte(string(destData))
	tampered[10] = 'X' // Modify byte at index 10 (col 11), inside placed region
	if err := os.WriteFile(destPath, tampered, 0644); err != nil {
		t.Fatal(err)
	}

	// Step 7: Re-verify — should FAIL (drift detected at position level)
	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	configStore2 := NewMockConfigStore(ctrl2)
	lockStore2 := NewMockLockStore(ctrl2)

	// Recompute whole-file hash for modified file so whole-file also drifts
	configStore2.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "col-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "source.go:L5C10:L5C30", To: destPath + ":L1C6:L1C26"}},
			}},
		}},
	}, nil)

	lockStore2.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{
			Name:       "col-vendor",
			Ref:        "main",
			CommitHash: "abc123",
			FileHashes: map[string]string{destPath: wholeFileHash}, // original hash
			Positions:  []types.PositionLock{posLock},
		}},
	}, nil)

	service2 := NewVerifyService(configStore2, lockStore2, realCache, NewOSFileSystem(), workDir)
	result2, err := service2.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify after tamper: %v", err)
	}
	if result2.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL after column tamper, got %s", result2.Summary.Result)
	}

	// Confirm position-level drift is detected
	posModified := false
	for _, f := range result2.Files {
		if f.Type == "position" && f.Status == "modified" {
			posModified = true
			if f.Position == nil {
				t.Error("position-type modified entry missing Position detail")
			}
			break
		}
	}
	if !posModified {
		t.Error("expected position-type modified entry after column tamper")
	}
}

// ============================================================================
// Test 2: EOF Position with File Growth
//
// Sync with L10-EOF, then append lines to source, re-update, verify new
// content includes appended lines and hash changes correctly.
// ============================================================================

func TestEOFPosition_FileGrowth_HashChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial source file with 15 lines
	srcPath := filepath.Join(tmpDir, "source.go")
	var initialLines []string
	for i := 1; i <= 15; i++ {
		initialLines = append(initialLines, "line"+fmt.Sprintf("%d", i))
	}
	initialContent := strings.Join(initialLines, "\n")
	if err := os.WriteFile(srcPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Step 1: Extract L10-EOF from original file (lines 10-15)
	eofPos := &types.PositionSpec{StartLine: 10, ToEOF: true}
	extracted1, hash1, err := ExtractPosition(srcPath, eofPos)
	if err != nil {
		t.Fatalf("initial extraction: %v", err)
	}

	wantExtracted := "line10\nline11\nline12\nline13\nline14\nline15"
	if extracted1 != wantExtracted {
		t.Fatalf("initial extract = %q, want %q", extracted1, wantExtracted)
	}

	// Step 2: Place into destination file (whole-file placement)
	destPath := filepath.Join(tmpDir, "dest.go")
	if err := PlaceContent(destPath, extracted1, nil); err != nil {
		t.Fatalf("initial place: %v", err)
	}

	// Step 3: Append lines to source file (simulate upstream growth)
	var grownLines []string
	grownLines = append(grownLines, initialLines...)
	grownLines = append(grownLines, "line16", "line17", "line18")
	grownContent := strings.Join(grownLines, "\n")
	if err := os.WriteFile(srcPath, []byte(grownContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Step 4: Re-extract L10-EOF — should now include appended lines
	extracted2, hash2, err := ExtractPosition(srcPath, eofPos)
	if err != nil {
		t.Fatalf("re-extraction after growth: %v", err)
	}

	wantGrown := "line10\nline11\nline12\nline13\nline14\nline15\nline16\nline17\nline18"
	if extracted2 != wantGrown {
		t.Errorf("grown extract = %q, want %q", extracted2, wantGrown)
	}

	// Step 5: Hash MUST change after file growth
	if hash1 == hash2 {
		t.Errorf("hash should change after appending lines, both = %q", hash1)
	}

	// Step 6: Place new content to destination, verify destination includes grown content
	if err := PlaceContent(destPath, extracted2, nil); err != nil {
		t.Fatalf("place after growth: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != wantGrown {
		t.Errorf("dest after re-sync = %q, want %q", string(got), wantGrown)
	}

	// Step 7: Verify old hash no longer matches (drift detection)
	_, currentHash, err := ExtractPosition(destPath, &types.PositionSpec{StartLine: 1, ToEOF: true})
	if err != nil {
		t.Fatalf("final extraction: %v", err)
	}
	if currentHash == hash1 {
		t.Error("destination hash should differ from original after file growth re-sync")
	}
	if currentHash != hash2 {
		t.Errorf("destination hash = %q, want %q (same as grown source)", currentHash, hash2)
	}
}

// ============================================================================
// Test 3: Overlapping Position Mappings
//
// Two vendors writing to the same destination file at different line ranges
// (vendor A → output.go:L1-L5, vendor B → output.go:L10-L15).
// Verify both coexist and verify independently.
// ============================================================================

func TestOverlappingPositionMappings_TwoVendors_IndependentVerify(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	repoA := filepath.Join(tmpDir, "repoA")
	repoB := filepath.Join(tmpDir, "repoB")
	for _, d := range []string{workDir, repoA, repoB} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source files for each vendor
	srcA := filepath.Join(repoA, "api.go")
	if err := os.WriteFile(srcA, []byte("vendor-A-1\nvendor-A-2\nvendor-A-3\nvendor-A-4\nvendor-A-5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srcB := filepath.Join(repoB, "lib.go")
	if err := os.WriteFile(srcB, []byte("vendor-B-1\nvendor-B-2\nvendor-B-3\nvendor-B-4\nvendor-B-5\nvendor-B-6\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create destination file with 20 lines (placeholders)
	destPath := filepath.Join(workDir, "output.go")
	var destLines []string
	for i := 1; i <= 20; i++ {
		destLines = append(destLines, "placeholder-"+fmt.Sprintf("%d", i))
	}
	destContent := strings.Join(destLines, "\n") + "\n"
	if err := os.WriteFile(destPath, []byte(destContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Step 1: Extract from vendor A (L1-L5) and place at output.go:L1-L5
	extractedA, hashA, err := ExtractPosition(srcA, &types.PositionSpec{StartLine: 1, EndLine: 5})
	if err != nil {
		t.Fatalf("extract vendor A: %v", err)
	}
	if err := PlaceContent(destPath, extractedA, &types.PositionSpec{StartLine: 1, EndLine: 5}); err != nil {
		t.Fatalf("place vendor A: %v", err)
	}

	// Step 2: Extract from vendor B (L1-L6) and place at output.go:L10-L15
	extractedB, hashB, err := ExtractPosition(srcB, &types.PositionSpec{StartLine: 1, EndLine: 6})
	if err != nil {
		t.Fatalf("extract vendor B: %v", err)
	}
	if err := PlaceContent(destPath, extractedB, &types.PositionSpec{StartLine: 10, EndLine: 15}); err != nil {
		t.Fatalf("place vendor B: %v", err)
	}

	// Step 3: Verify both regions coexist in the destination file
	gotDest, _ := os.ReadFile(destPath)
	destStr := string(gotDest)

	// Vendor A content at lines 1-5
	if !strings.Contains(destStr, "vendor-A-1") {
		t.Error("vendor A content missing from destination")
	}
	// Vendor B content at lines 10-15
	if !strings.Contains(destStr, "vendor-B-1") {
		t.Error("vendor B content missing from destination")
	}
	// Placeholder lines (6-9) should be preserved
	if !strings.Contains(destStr, "placeholder-6") {
		t.Error("placeholder content between vendors was overwritten")
	}

	// Step 4: Verify each position independently through re-extraction
	reExtractedA, reHashA, err := ExtractPosition(destPath, &types.PositionSpec{StartLine: 1, EndLine: 5})
	if err != nil {
		t.Fatalf("re-extract vendor A: %v", err)
	}
	if reExtractedA != extractedA {
		t.Errorf("vendor A round-trip mismatch: got %q, want %q", reExtractedA, extractedA)
	}
	if reHashA != hashA {
		t.Errorf("vendor A hash mismatch: got %q, want %q", reHashA, hashA)
	}

	reExtractedB, reHashB, err := ExtractPosition(destPath, &types.PositionSpec{StartLine: 10, EndLine: 15})
	if err != nil {
		t.Fatalf("re-extract vendor B: %v", err)
	}
	if reExtractedB != extractedB {
		t.Errorf("vendor B round-trip mismatch: got %q, want %q", reExtractedB, extractedB)
	}
	if reHashB != hashB {
		t.Errorf("vendor B hash mismatch: got %q, want %q", reHashB, hashB)
	}

	// Step 5: Verify through VerifyService — both positions should pass
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	realCache := NewFileCacheStore(NewOSFileSystem(), workDir)
	wholeHash, _ := realCache.ComputeFileChecksum(destPath)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/ownerA/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "api.go:L1-L5", To: destPath + ":L1-L5"}},
				}},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/ownerB/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "lib.go:L1-L6", To: destPath + ":L10-L15"}},
				}},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "vendor-a",
				Ref:        "main",
				CommitHash: "aaa111",
				FileHashes: map[string]string{destPath: wholeHash},
				Positions: []types.PositionLock{{
					From:       "api.go:L1-L5",
					To:         destPath + ":L1-L5",
					SourceHash: hashA,
				}},
			},
			{
				Name:       "vendor-b",
				Ref:        "main",
				CommitHash: "bbb222",
				Positions: []types.PositionLock{{
					From:       "lib.go:L1-L6",
					To:         destPath + ":L10-L15",
					SourceHash: hashB,
				}},
			},
		},
	}, nil)

	service := NewVerifyService(configStore, lockStore, realCache, NewOSFileSystem(), workDir)
	result, err := service.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Summary.Result != "PASS" {
		t.Errorf("expected PASS with both vendors intact, got %s", result.Summary.Result)
	}

	// Count position-type verified entries
	posVerified := 0
	for _, f := range result.Files {
		if f.Type == "position" && f.Status == "verified" {
			posVerified++
		}
	}
	if posVerified != 2 {
		t.Errorf("expected 2 position-verified entries, got %d", posVerified)
	}

	// Step 6: Tamper vendor A's region only — vendor B should still verify
	tamperedDest, _ := os.ReadFile(destPath)
	tamperedStr := string(tamperedDest)
	tamperedStr = strings.Replace(tamperedStr, "vendor-A-1", "TAMPERED-A", 1)
	if err := os.WriteFile(destPath, []byte(tamperedStr), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	configStore2 := NewMockConfigStore(ctrl2)
	lockStore2 := NewMockLockStore(ctrl2)

	configStore2.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/ownerA/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "api.go:L1-L5", To: destPath + ":L1-L5"}},
				}},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/ownerB/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "lib.go:L1-L6", To: destPath + ":L10-L15"}},
				}},
			},
		},
	}, nil)

	lockStore2.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "vendor-a",
				Ref:        "main",
				CommitHash: "aaa111",
				FileHashes: map[string]string{destPath: wholeHash}, // stale whole-file hash
				Positions: []types.PositionLock{{
					From:       "api.go:L1-L5",
					To:         destPath + ":L1-L5",
					SourceHash: hashA,
				}},
			},
			{
				Name:       "vendor-b",
				Ref:        "main",
				CommitHash: "bbb222",
				Positions: []types.PositionLock{{
					From:       "lib.go:L1-L6",
					To:         destPath + ":L10-L15",
					SourceHash: hashB,
				}},
			},
		},
	}, nil)

	service2 := NewVerifyService(configStore2, lockStore2, realCache, NewOSFileSystem(), workDir)
	result2, err := service2.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify after vendor-A tamper: %v", err)
	}
	if result2.Summary.Result != "FAIL" {
		t.Errorf("expected FAIL after tampering vendor A, got %s", result2.Summary.Result)
	}

	// Vendor A's position should be modified, vendor B's should still verify
	aModified, bVerified := false, false
	for _, f := range result2.Files {
		if f.Type == "position" {
			if f.Vendor != nil && *f.Vendor == "vendor-a" && f.Status == "modified" {
				aModified = true
			}
			if f.Vendor != nil && *f.Vendor == "vendor-b" && f.Status == "verified" {
				bVerified = true
			}
		}
	}
	if !aModified {
		t.Error("expected vendor-a position to be 'modified' after tamper")
	}
	if !bVerified {
		t.Error("expected vendor-b position to remain 'verified' after tampering vendor-a only")
	}
}

// ============================================================================
// Test 4: Position Extraction on Empty/Single-Line Files
//
// Edge cases: L1 on a 1-line file, L1-EOF on an empty file, column extraction
// on a line shorter than the column range. Verify graceful error handling
// through the full pipeline.
// ============================================================================

func TestPositionEdgeCases_EmptyAndSingleLine(t *testing.T) {
	t.Run("L1_on_single_line_file_through_full_pipeline", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo")
		workDir := filepath.Join(tmpDir, "work")
		os.MkdirAll(repoDir, 0755)
		os.MkdirAll(workDir, 0755)
		oldDir, _ := os.Getwd()
		os.Chdir(workDir)
		defer func() { _ = os.Chdir(oldDir) }()

		// Single-line file (no trailing newline)
		srcPath := filepath.Join(repoDir, "single.go")
		if err := os.WriteFile(srcPath, []byte("only line"), 0644); err != nil {
			t.Fatal(err)
		}

		// Extract L1
		extracted, hash, err := ExtractPosition(srcPath, &types.PositionSpec{StartLine: 1})
		if err != nil {
			t.Fatalf("extract L1 from single-line file: %v", err)
		}
		if extracted != "only line" {
			t.Errorf("extracted = %q, want %q", extracted, "only line")
		}

		// Place into destination (whole-file)
		destPath := filepath.Join(workDir, "dest.go")
		if err := PlaceContent(destPath, extracted, nil); err != nil {
			t.Fatalf("place: %v", err)
		}

		// Re-extract and verify hash stability
		reExtracted, reHash, err := ExtractPosition(destPath, &types.PositionSpec{StartLine: 1})
		if err != nil {
			t.Fatalf("re-extract: %v", err)
		}
		if reExtracted != extracted {
			t.Errorf("round-trip content mismatch: %q vs %q", reExtracted, extracted)
		}
		if reHash != hash {
			t.Errorf("round-trip hash mismatch: %q vs %q", reHash, hash)
		}

		// Verify through CopyMappings integration
		svc := NewFileCopyService(NewOSFileSystem())
		vendor := &types.VendorSpec{Name: "edge-test"}
		spec := types.BranchSpec{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "single.go:L1", To: "output.go"},
			},
		}

		stats, err := svc.CopyMappings(repoDir, vendor, spec)
		if err != nil {
			t.Fatalf("CopyMappings: %v", err)
		}
		if len(stats.Positions) != 1 {
			t.Errorf("expected 1 position record, got %d", len(stats.Positions))
		}
		if stats.Positions[0].SourceHash != hash {
			t.Errorf("position hash = %q, want %q", stats.Positions[0].SourceHash, hash)
		}
	})

	t.Run("L1_EOF_on_empty_file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Empty file: strings.Split("", "\n") = [""] → 1 line (the empty string)
		emptyPath := filepath.Join(tmpDir, "empty.go")
		if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		// L1-EOF on empty file should succeed: extracts the single empty line
		extracted, hash, err := ExtractPosition(emptyPath, &types.PositionSpec{StartLine: 1, ToEOF: true})
		if err != nil {
			t.Fatalf("L1-EOF on empty file: %v", err)
		}
		if extracted != "" {
			t.Errorf("extracted = %q, want empty string", extracted)
		}
		if !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("hash = %q, want sha256: prefix", hash)
		}

		// Place empty content and verify round-trip
		destPath := filepath.Join(tmpDir, "dest.go")
		if err := PlaceContent(destPath, extracted, nil); err != nil {
			t.Fatalf("place empty content: %v", err)
		}

		_, reHash, err := ExtractPosition(destPath, &types.PositionSpec{StartLine: 1, ToEOF: true})
		if err != nil {
			t.Fatalf("re-extract empty: %v", err)
		}
		if reHash != hash {
			t.Errorf("empty content hash mismatch: %q vs %q", reHash, hash)
		}
	})

	t.Run("L2_on_empty_file_errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyPath := filepath.Join(tmpDir, "empty.go")
		if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		// L2 on empty file should error (only 1 line exists)
		_, _, err := ExtractPosition(emptyPath, &types.PositionSpec{StartLine: 2})
		if err == nil {
			t.Fatal("expected error for L2 on empty file")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error = %q, want 'does not exist' message", err.Error())
		}
	})

	t.Run("column_extraction_exceeds_short_line", func(t *testing.T) {
		tmpDir := t.TempDir()

		// File with a short line (3 chars)
		shortPath := filepath.Join(tmpDir, "short.go")
		if err := os.WriteFile(shortPath, []byte("abc\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Column range that exceeds the line length
		_, _, err := ExtractPosition(shortPath, &types.PositionSpec{
			StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 10,
		})
		if err == nil {
			t.Fatal("expected error for column range exceeding line length")
		}
		if !strings.Contains(err.Error(), "exceeds line length") {
			t.Errorf("error = %q, want 'exceeds line length' message", err.Error())
		}

		// StartCol beyond line length
		_, _, err = ExtractPosition(shortPath, &types.PositionSpec{
			StartLine: 1, EndLine: 1, StartCol: 5, EndCol: 5,
		})
		if err == nil {
			t.Fatal("expected error for StartCol exceeding line length")
		}
		if !strings.Contains(err.Error(), "exceeds line length") {
			t.Errorf("error = %q, want 'exceeds line length' message", err.Error())
		}
	})

	t.Run("column_on_empty_line", func(t *testing.T) {
		tmpDir := t.TempDir()

		// File where L1 is empty (trailing newline = "")
		emptyLinePath := filepath.Join(tmpDir, "emptyline.go")
		if err := os.WriteFile(emptyLinePath, []byte("\nsecond\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Column extraction on the empty first line — StartCol=1 exceeds length 0
		_, _, err := ExtractPosition(emptyLinePath, &types.PositionSpec{
			StartLine: 1, EndLine: 1, StartCol: 1, EndCol: 1,
		})
		if err == nil {
			t.Fatal("expected error for column extraction on empty line")
		}
		if !strings.Contains(err.Error(), "exceeds line length") {
			t.Errorf("error = %q, want 'exceeds line length' message", err.Error())
		}
	})

	t.Run("CopyMappings_position_on_empty_source_file", func(t *testing.T) {
		repoDir := t.TempDir()
		workDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(workDir)
		defer func() { _ = os.Chdir(oldDir) }()

		// Create an empty source file
		srcPath := filepath.Join(repoDir, "empty.go")
		if err := os.WriteFile(srcPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		svc := NewFileCopyService(NewOSFileSystem())
		vendor := &types.VendorSpec{Name: "empty-test"}

		// L1 on empty file should succeed (extracts empty string)
		spec := types.BranchSpec{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "empty.go:L1", To: "output.go"},
			},
		}

		stats, err := svc.CopyMappings(repoDir, vendor, spec)
		if err != nil {
			t.Fatalf("CopyMappings with L1 on empty file should succeed: %v", err)
		}
		if len(stats.Positions) != 1 {
			t.Errorf("expected 1 position, got %d", len(stats.Positions))
		}

		// Verify dest has empty content
		got, _ := os.ReadFile(filepath.Join(workDir, "output.go"))
		if string(got) != "" {
			t.Errorf("dest content = %q, want empty string", string(got))
		}

		// L2 on empty file should error through CopyMappings
		spec2 := types.BranchSpec{
			Ref: "main",
			Mapping: []types.PathMapping{
				{From: "empty.go:L2", To: "output2.go"},
			},
		}

		_, err = svc.CopyMappings(repoDir, vendor, spec2)
		if err == nil {
			t.Fatal("CopyMappings with L2 on empty file should error")
		}
	})
}

