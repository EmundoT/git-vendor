package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/EmundoT/git-vendor/internal/types"
)

// --- VendorTrailers tests ---

func TestVendorTrailers_SingleVendor(t *testing.T) {
	locks := []types.LockDetails{
		{
			Name:             "my-lib",
			Ref:              "main",
			CommitHash:       "abc123def456789012345678901234567890abcd",
			LicenseSPDX:      "MIT",
			SourceVersionTag: "v1.2.3",
		},
	}

	trailers := VendorTrailers(locks)

	// Commit-Schema + Tags + Name + Ref + Commit + License + Source-Tag = 7
	if len(trailers) != 7 {
		t.Fatalf("expected 7 trailers, got %d: %v", len(trailers), trailers)
	}

	assertTrailer(t, trailers, 0, "Commit-Schema", "vendor/v1")
	assertTrailer(t, trailers, 1, "Tags", "vendor.update")
	assertTrailer(t, trailers, 2, "Vendor-Name", "my-lib")
	assertTrailer(t, trailers, 3, "Vendor-Ref", "main")
	assertTrailer(t, trailers, 4, "Vendor-Commit", "abc123def456789012345678901234567890abcd")
	assertTrailer(t, trailers, 5, "Vendor-License", "MIT")
	assertTrailer(t, trailers, 6, "Vendor-Source-Tag", "v1.2.3")
}

func TestVendorTrailers_OptionalFields(t *testing.T) {
	locks := []types.LockDetails{
		{
			Name:       "bare-lib",
			Ref:        "v2",
			CommitHash: "0000000000000000000000000000000000000000",
		},
	}

	trailers := VendorTrailers(locks)

	// Commit-Schema + Tags + Name + Ref + Commit = 5 (no License, no Source-Tag)
	if len(trailers) != 5 {
		t.Fatalf("expected 5 trailers, got %d", len(trailers))
	}

	for _, tr := range trailers {
		if tr.Key == "Vendor-License" || tr.Key == "Vendor-Source-Tag" {
			t.Errorf("unexpected optional trailer: %s=%s", tr.Key, tr.Value)
		}
	}
}

func TestVendorTrailers_MultiVendor(t *testing.T) {
	locks := []types.LockDetails{
		{Name: "lib-a", Ref: "main", CommitHash: "aaaa000000000000000000000000000000000000"},
		{Name: "lib-b", Ref: "v2", CommitHash: "bbbb000000000000000000000000000000000000", LicenseSPDX: "MIT"},
	}

	trailers := VendorTrailers(locks)

	// 1 (Commit-Schema) + 1 (Tags) + 3 (lib-a: Name+Ref+Commit) + 4 (lib-b: Name+Ref+Commit+License) = 9
	if len(trailers) != 9 {
		t.Fatalf("expected 9 trailers, got %d", len(trailers))
	}

	assertTrailer(t, trailers, 0, "Commit-Schema", "vendor/v1")
	assertTrailer(t, trailers, 1, "Tags", "vendor.update")
	// lib-a group
	assertTrailer(t, trailers, 2, "Vendor-Name", "lib-a")
	assertTrailer(t, trailers, 3, "Vendor-Ref", "main")
	assertTrailer(t, trailers, 4, "Vendor-Commit", "aaaa000000000000000000000000000000000000")
	// lib-b group
	assertTrailer(t, trailers, 5, "Vendor-Name", "lib-b")
	assertTrailer(t, trailers, 6, "Vendor-Ref", "v2")
	assertTrailer(t, trailers, 7, "Vendor-Commit", "bbbb000000000000000000000000000000000000")
	assertTrailer(t, trailers, 8, "Vendor-License", "MIT")
}

func TestVendorTrailers_CommitSchemaAlwaysFirst(t *testing.T) {
	locks := []types.LockDetails{
		{Name: "x", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
	}

	trailers := VendorTrailers(locks)

	if trailers[0].Key != "Commit-Schema" || trailers[0].Value != "vendor/v1" {
		t.Errorf("first trailer MUST be Commit-Schema: vendor/v1, got %s: %s", trailers[0].Key, trailers[0].Value)
	}
}

// --- VendorCommitSubject tests ---

func TestVendorCommitSubject_SingleVendor(t *testing.T) {
	locks := []types.LockDetails{{Name: "my-lib", Ref: "main"}}
	got := VendorCommitSubject(locks, "sync")
	want := "chore(vendor): sync my-lib to main"
	if got != want {
		t.Errorf("VendorCommitSubject = %q, want %q", got, want)
	}
}

func TestVendorCommitSubject_MultiVendor(t *testing.T) {
	locks := []types.LockDetails{
		{Name: "lib-a", Ref: "main"},
		{Name: "lib-b", Ref: "v2"},
		{Name: "lib-c", Ref: "dev"},
	}
	got := VendorCommitSubject(locks, "update")
	want := "chore(vendor): update 3 vendors"
	if got != want {
		t.Errorf("VendorCommitSubject = %q, want %q", got, want)
	}
}

// --- VendorNoteJSON tests ---

func TestVendorNoteJSON_Structure(t *testing.T) {
	locks := []types.LockDetails{
		{
			Name:             "my-lib",
			Ref:              "main",
			CommitHash:       "abc123def456789012345678901234567890abcd",
			LicenseSPDX:      "MIT",
			SourceVersionTag: "v1.2.3",
			FileHashes:       map[string]string{"vendor/a.go": "sha256:abc"},
		},
	}
	specMap := map[string]*types.VendorSpec{
		"my-lib": {
			URL: "https://github.com/owner/my-lib",
			Specs: []types.BranchSpec{
				{Ref: "main", Mapping: []types.PathMapping{{From: "src/a.go", To: "vendor/a.go"}}},
			},
		},
	}

	raw, err := VendorNoteJSON(locks, specMap)
	if err != nil {
		t.Fatalf("VendorNoteJSON error: %v", err)
	}

	var note VendorNoteData
	if err := json.Unmarshal([]byte(raw), &note); err != nil {
		t.Fatalf("note is not valid JSON: %v", err)
	}

	if note.Schema != "vendor/v1" {
		t.Errorf("schema = %q, want 'vendor/v1'", note.Schema)
	}
	if len(note.Vendors) != 1 {
		t.Fatalf("expected 1 vendor entry, got %d", len(note.Vendors))
	}

	v := note.Vendors[0]
	if v.Name != "my-lib" {
		t.Errorf("vendor name = %q", v.Name)
	}
	if v.URL != "https://github.com/owner/my-lib" {
		t.Errorf("vendor URL = %q", v.URL)
	}
	if v.CommitHash != "abc123def456789012345678901234567890abcd" {
		t.Errorf("vendor commit = %q", v.CommitHash)
	}
	if len(v.FileHashes) != 1 {
		t.Errorf("expected 1 file hash, got %d", len(v.FileHashes))
	}
	if len(v.Paths) != 1 || v.Paths[0] != "vendor/a.go" {
		t.Errorf("paths = %v, want [vendor/a.go]", v.Paths)
	}
}

// --- collectVendorPaths tests ---

func TestCollectVendorPaths_Basic(t *testing.T) {
	spec := &types.VendorSpec{
		Name: "my-lib",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/lib.go", To: "vendor/my-lib/lib.go"},
					{From: "src/util.go", To: "vendor/my-lib/util.go"},
				},
			},
		},
	}
	// Create the license file so collectVendorPaths can verify it exists
	tmpDir := t.TempDir()
	licenseDir := filepath.Join(tmpDir, ".git-vendor", "licenses")
	if err := os.MkdirAll(licenseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(licenseDir, "my-lib.txt"), []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}

	lock := types.LockDetails{
		Name:        "my-lib",
		LicensePath: ".git-vendor/licenses/my-lib.txt",
	}

	paths := collectVendorPaths(spec, lock, tmpDir)
	// 2 mapping paths + LockPath + ConfigPath + license = 5
	if len(paths) != 5 {
		t.Errorf("expected 5 paths, got %d: %v", len(paths), paths)
	}

	hasLock := false
	hasConfig := false
	hasLicense := false
	for _, p := range paths {
		switch p {
		case LockPath:
			hasLock = true
		case ConfigPath:
			hasConfig = true
		case ".git-vendor/licenses/my-lib.txt":
			hasLicense = true
		}
	}
	if !hasLock {
		t.Error("LockPath not found in paths")
	}
	if !hasConfig {
		t.Error("ConfigPath not found in paths")
	}
	if !hasLicense {
		t.Error("license path not found in paths")
	}
}

func TestCollectVendorPaths_NoLicense(t *testing.T) {
	spec := &types.VendorSpec{
		Name: "bare",
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}},
		},
	}
	lock := types.LockDetails{Name: "bare"}

	paths := collectVendorPaths(spec, lock, ".")
	// 1 mapping + LockPath + ConfigPath = 3
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(paths), paths)
	}
}

func TestCollectVendorPaths_AutoNamed(t *testing.T) {
	spec := &types.VendorSpec{
		Name: "auto",
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "dir/file.go", To: ""}}},
		},
	}
	lock := types.LockDetails{Name: "auto"}

	paths := collectVendorPaths(spec, lock, ".")
	found := false
	for _, p := range paths {
		if p == "file.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-named path 'file.go' in %v", paths)
	}
}

func TestCollectVendorPaths_Dedup(t *testing.T) {
	spec := &types.VendorSpec{
		Name: "dedup",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "a.go", To: "vendor/x.go"},
					{From: "b.go", To: "vendor/x.go"}, // duplicate dest
				},
			},
		},
	}
	lock := types.LockDetails{Name: "dedup"}

	// collectVendorPaths does NOT dedup — dedup is applied at the commit level.
	// But the raw output has: vendor/x.go, vendor/x.go, LockPath, ConfigPath = 4
	// After dedup in CommitVendorChanges it would be 3.
	paths := collectVendorPaths(spec, lock, ".")
	if len(paths) != 4 {
		t.Errorf("expected 4 raw paths, got %d: %v", len(paths), paths)
	}
}

// --- dedup tests ---

func TestDedup(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	got := dedup(input)
	if len(got) != 3 {
		t.Errorf("expected 3 unique items, got %d: %v", len(got), got)
	}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("unexpected order: %v", got)
	}
}

func TestDedup_WindowsPathNormalization(t *testing.T) {
	input := []string{"a/b", "a\\b"}
	got := dedup(input)
	if len(got) != 1 {
		t.Errorf("expected 1 after normalizing slashes, got %d: %v", len(got), got)
	}
}

// --- Mock-based CommitVendorChanges tests ---

func TestCommitVendorChanges_SingleVendor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "my-lib",
				URL:  "https://github.com/owner/my-lib",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src/a.go", To: "vendor/a.go"}}},
				},
			},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "my-lib",
				Ref:        "main",
				CommitHash: "abc123def456789012345678901234567890abcd",
			},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Single Add call with all paths
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, paths ...string) error {
			// vendor/a.go + LockPath + ConfigPath = 3
			if len(paths) != 3 {
				t.Errorf("Add called with %d paths, expected 3: %v", len(paths), paths)
			}
			return nil
		},
	)

	// Single Commit with multi-valued trailers + shared trailers
	// SharedTrailers runs real git commands (bypasses mock), so shared trailer
	// values depend on actual git state. Assert vendor trailers exactly, shared by key.
	mockGit.EXPECT().Commit(gomock.Any(), ".", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, opts types.CommitOptions) error {
			if opts.Message != "chore(vendor): sync my-lib to main" {
				t.Errorf("commit message = %q", opts.Message)
			}
			// Vendor trailers: Schema + Tags + Name + Ref + Commit = 5
			if len(opts.Trailers) < 5 {
				t.Fatalf("expected at least 5 trailers, got %d: %v", len(opts.Trailers), opts.Trailers)
			}
			assertTrailer(t, opts.Trailers, 0, "Commit-Schema", "vendor/v1")
			assertTrailer(t, opts.Trailers, 1, "Tags", "vendor.update")
			assertTrailer(t, opts.Trailers, 2, "Vendor-Name", "my-lib")
			assertTrailer(t, opts.Trailers, 3, "Vendor-Ref", "main")
			assertTrailer(t, opts.Trailers, 4, "Vendor-Commit", "abc123def456789012345678901234567890abcd")
			// Vendor Touch extracted from destination paths
			assertHasTrailerKey(t, opts.Trailers, "Touch")
			// Verify Touch contains vendor area from mapping dest "vendor/a.go" → "vendor"
			touchValue := findTrailerValue(opts.Trailers, "Touch")
			if touchValue == "" {
				t.Error("Touch trailer value is empty")
			}
			// Shared trailers computed from real git state — verify presence, not values
			assertHasTrailerKey(t, opts.Trailers, "Diff-Additions")
			assertHasTrailerKey(t, opts.Trailers, "Diff-Deletions")
			assertHasTrailerKey(t, opts.Trailers, "Diff-Files")
			assertHasTrailerKey(t, opts.Trailers, "Diff-Surface")
			return nil
		},
	)

	// GetHeadHash for note attachment
	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("abc123def456789012345678901234567890abcd", nil)

	// AddNote for rich metadata
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, "abc123def456789012345678901234567890abcd", gomock.Any()).Return(nil)

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "")
	if err != nil {
		t.Fatalf("CommitVendorChanges returned error: %v", err)
	}
}

func TestCommitVendorChanges_MultiVendor_SingleCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", URL: "https://github.com/o/a", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "va.go"}}}}},
			{Name: "lib-b", URL: "https://github.com/o/b", Specs: []types.BranchSpec{{Ref: "v2", Mapping: []types.PathMapping{{From: "b.go", To: "vb.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", CommitHash: "aaaa000000000000000000000000000000000000"},
			{Name: "lib-b", Ref: "v2", CommitHash: "bbbb000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Exactly ONE Add + ONE Commit (not two)
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, paths ...string) error {
			// va.go + vb.go + LockPath + ConfigPath = 4
			if len(paths) != 4 {
				t.Errorf("expected 4 deduped paths, got %d: %v", len(paths), paths)
			}
			return nil
		},
	)

	mockGit.EXPECT().Commit(gomock.Any(), ".", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, opts types.CommitOptions) error {
			if opts.Message != "chore(vendor): update 2 vendors" {
				t.Errorf("commit message = %q, want multi-vendor subject", opts.Message)
			}
			// Vendor trailers: Schema + Tags + 2*(Name+Ref+Commit) = 8
			if len(opts.Trailers) < 8 {
				t.Errorf("expected at least 8 trailers, got %d", len(opts.Trailers))
			}
			// Verify multi-valued Vendor-Name
			names := filterTrailerValues(opts.Trailers, "Vendor-Name")
			if len(names) != 2 || names[0] != "lib-a" || names[1] != "lib-b" {
				t.Errorf("Vendor-Name values = %v, want [lib-a, lib-b]", names)
			}
			return nil
		},
	)

	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("1111111111111111111111111111111111111111", nil)
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, gomock.Any(), gomock.Any()).Return(nil)

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "update", "")
	if err != nil {
		t.Fatalf("CommitVendorChanges returned error: %v", err)
	}
}

func TestCommitVendorChanges_VendorFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "va.go"}}}}},
			{Name: "lib-b", Specs: []types.BranchSpec{{Ref: "v2", Mapping: []types.PathMapping{{From: "b.go", To: "vb.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib-a", Ref: "main", CommitHash: "aaaa000000000000000000000000000000000000"},
			{Name: "lib-b", Ref: "v2", CommitHash: "bbbb000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// Only lib-a should be in the commit
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).Return(nil)
	mockGit.EXPECT().Commit(gomock.Any(), ".", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, opts types.CommitOptions) error {
			names := filterTrailerValues(opts.Trailers, "Vendor-Name")
			if len(names) != 1 || names[0] != "lib-a" {
				t.Errorf("expected only lib-a, got Vendor-Name values: %v", names)
			}
			return nil
		},
	)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("1111111111111111111111111111111111111111", nil)
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, gomock.Any(), gomock.Any()).Return(nil)

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "lib-a")
	if err != nil {
		t.Fatalf("CommitVendorChanges returned error: %v", err)
	}
}

func TestCommitVendorChanges_AddFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).Return(fmt.Errorf("git add failed"))

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "")
	if err == nil {
		t.Fatal("expected error from Add failure, got nil")
	}
}

func TestCommitVendorChanges_CommitFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).Return(nil)
	mockGit.EXPECT().Commit(gomock.Any(), ".", gomock.Any()).Return(fmt.Errorf("nothing to commit"))

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "")
	if err == nil {
		t.Fatal("expected error from Commit failure, got nil")
	}
}

func TestCommitVendorChanges_OrphanedLockEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{Vendors: []types.VendorSpec{}}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "orphan", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)

	// No Add/Commit/GetHeadHash/AddNote calls expected
	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "")
	if err != nil {
		t.Fatalf("expected no error for orphaned lock entry, got: %v", err)
	}
}

func TestCommitVendorChanges_NoteFailureNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().Add(gomock.Any(), ".", gomock.Any()).Return(nil)
	mockGit.EXPECT().Commit(gomock.Any(), ".", gomock.Any()).Return(nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("1111111111111111111111111111111111111111", nil)
	// Note fails — should NOT cause CommitVendorChanges to return error
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, gomock.Any(), gomock.Any()).Return(fmt.Errorf("notes not supported"))

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, ".", "sync", "")
	if err != nil {
		t.Fatalf("note failure should be non-fatal, got: %v", err)
	}
}

// --- Shared trailer enrichment tests ---

func TestCommitVendorChanges_SharedTrailerEnrichmentFailureNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	// Use a non-git temp directory so git.SharedTrailers fails (not a git repo).
	// SharedTrailers creates its own git instance (bypasses mock), so the only
	// way to trigger its failure path is with a directory that's not a git repo.
	tmpDir := t.TempDir()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib", Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}}}},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().Add(gomock.Any(), tmpDir, gomock.Any()).Return(nil)

	mockGit.EXPECT().Commit(gomock.Any(), tmpDir, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, opts types.CommitOptions) error {
			// Verify no Diff-* trailers (SharedTrailers failed)
			for _, tr := range opts.Trailers {
				if tr.Key == "Diff-Additions" || tr.Key == "Diff-Deletions" || tr.Key == "Diff-Files" || tr.Key == "Diff-Surface" {
					t.Errorf("unexpected shared trailer: %s=%s", tr.Key, tr.Value)
				}
			}
			// But vendor Touch SHOULD still be present (from dest path extraction)
			// Mapping dest "b.go" is root-level, so no area — Touch may be absent
			return nil
		},
	)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), tmpDir).Return("0000000000000000000000000000000000000000", nil)
	mockGit.EXPECT().AddNote(gomock.Any(), tmpDir, VendorNoteRef, gomock.Any(), gomock.Any()).Return(nil)

	err := CommitVendorChanges(context.Background(), mockGit, mockConfig, mockLock, tmpDir, "sync", "")
	if err != nil {
		t.Fatalf("shared trailer failure should be non-fatal, got: %v", err)
	}
}

func TestVendorTrailers_TagsPresent(t *testing.T) {
	locks := []types.LockDetails{
		{Name: "x", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
	}

	trailers := VendorTrailers(locks)

	// Tags MUST be second trailer (after Commit-Schema, before vendor trailers)
	assertTrailer(t, trailers, 1, "Tags", "vendor.update")
}

// --- Touch extraction tests ---

func TestPathToTouchArea_Basic(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"pkg/git-plumbing/hook.go", "pkg.git-plumbing"},
		{".claude/hooks/guard.sh", "claude.hooks"},
		{"internal/core/engine.go", "internal.core"},
		{"a/b/c/d.go", "a.b.c"},
		{"root.go", ""},
		{"docs/ROADMAP.md", "docs"},
		{".claude/skills/commit-schema.md", "claude.skills"},
		{"vendor/a.go", "vendor"},
	}

	for _, tt := range tests {
		got := pathToTouchArea(tt.path)
		if got != tt.want {
			t.Errorf("pathToTouchArea(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestPathToTouchArea_WindowsSlashes(t *testing.T) {
	got := pathToTouchArea("pkg\\git-plumbing\\hook.go")
	if got != "pkg.git-plumbing" {
		t.Errorf("pathToTouchArea with backslashes = %q, want %q", got, "pkg.git-plumbing")
	}
}

func TestPathToTouchArea_LeadingDots(t *testing.T) {
	got := pathToTouchArea(".hidden/dir/file.go")
	if got != "hidden.dir" {
		t.Errorf("pathToTouchArea(.hidden/dir) = %q, want %q", got, "hidden.dir")
	}
}

func TestPathToTouchArea_NumericSegment(t *testing.T) {
	// Segments starting with digits are skipped (tag grammar requires leading letter)
	got := pathToTouchArea("2ndparty/src/file.go")
	if got != "src" {
		t.Errorf("pathToTouchArea(2ndparty/src) = %q, want %q", got, "src")
	}

	// "v2" starts with a letter — valid tag segment
	got = pathToTouchArea("v2/src/file.go")
	if got != "v2.src" {
		t.Errorf("pathToTouchArea(v2/src) = %q, want %q", got, "v2.src")
	}
}

func TestExtractVendorTouch_SingleSpec(t *testing.T) {
	specs := []*types.VendorSpec{
		{
			Name: "git-plumbing",
			Specs: []types.BranchSpec{
				{
					Ref: "main",
					Mapping: []types.PathMapping{
						{From: "git.go", To: "pkg/git-plumbing/git.go"},
						{From: "hook.go", To: "pkg/git-plumbing/hook.go"},
					},
				},
			},
		},
	}

	got := ExtractVendorTouch(specs)
	if len(got) != 1 || got[0] != "pkg.git-plumbing" {
		t.Errorf("ExtractVendorTouch = %v, want [pkg.git-plumbing]", got)
	}
}

func TestExtractVendorTouch_MultiSpec(t *testing.T) {
	specs := []*types.VendorSpec{
		{
			Name: "plumbing",
			Specs: []types.BranchSpec{
				{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "pkg/git-plumbing/a.go"}}},
			},
		},
		{
			Name: "ecosystem",
			Specs: []types.BranchSpec{
				{Ref: "main", Mapping: []types.PathMapping{
					{From: "rules/tags.md", To: ".claude/rules/tags.md"},
					{From: "skills/commit.md", To: ".claude/skills/commit.md"},
				}},
			},
		},
	}

	got := ExtractVendorTouch(specs)
	// Expect sorted: claude.rules, claude.skills, pkg.git-plumbing
	if len(got) != 3 {
		t.Fatalf("ExtractVendorTouch = %v, want 3 areas", got)
	}
	if got[0] != "claude.rules" || got[1] != "claude.skills" || got[2] != "pkg.git-plumbing" {
		t.Errorf("ExtractVendorTouch = %v", got)
	}
}

func TestExtractVendorTouch_RootLevelFile(t *testing.T) {
	specs := []*types.VendorSpec{
		{
			Name: "simple",
			Specs: []types.BranchSpec{
				{Ref: "main", Mapping: []types.PathMapping{{From: "lib.go", To: "lib.go"}}},
			},
		},
	}

	got := ExtractVendorTouch(specs)
	if len(got) != 0 {
		t.Errorf("ExtractVendorTouch for root file = %v, want empty", got)
	}
}

func TestExtractVendorTouch_AutoNamedDest(t *testing.T) {
	specs := []*types.VendorSpec{
		{
			Name: "auto",
			Specs: []types.BranchSpec{
				{Ref: "main", Mapping: []types.PathMapping{{From: "src/deep/file.go", To: ""}}},
			},
		},
	}

	got := ExtractVendorTouch(specs)
	// Auto-named: filepath.Base("src/deep/file.go") = "file.go" — root level, no area
	if len(got) != 0 {
		t.Errorf("ExtractVendorTouch for auto-named = %v, want empty", got)
	}
}

func TestMergeTouch_VendorOnly(t *testing.T) {
	vendorTouch := []string{"pkg.git-plumbing", "claude.hooks"}
	merged, filtered := mergeTouch(vendorTouch, nil)

	if len(merged) != 2 || merged[0] != "claude.hooks" || merged[1] != "pkg.git-plumbing" {
		t.Errorf("mergeTouch vendor-only = %v", merged)
	}
	if len(filtered) != 0 {
		t.Errorf("filtered should be empty, got %v", filtered)
	}
}

func TestMergeTouch_SharedOnly(t *testing.T) {
	shared := []types.Trailer{
		{Key: "Touch", Value: "auth, security"},
		{Key: "Diff-Files", Value: "3"},
	}
	merged, filtered := mergeTouch(nil, shared)

	if len(merged) != 2 || merged[0] != "auth" || merged[1] != "security" {
		t.Errorf("mergeTouch shared-only = %v", merged)
	}
	if len(filtered) != 1 || filtered[0].Key != "Diff-Files" {
		t.Errorf("filtered = %v, want [Diff-Files]", filtered)
	}
}

func TestMergeTouch_Combined(t *testing.T) {
	vendorTouch := []string{"pkg.git-plumbing"}
	shared := []types.Trailer{
		{Key: "Touch", Value: "auth, pkg.git-plumbing"},
		{Key: "Diff-Additions", Value: "10"},
	}
	merged, filtered := mergeTouch(vendorTouch, shared)

	// Deduped: auth + pkg.git-plumbing
	if len(merged) != 2 || merged[0] != "auth" || merged[1] != "pkg.git-plumbing" {
		t.Errorf("mergeTouch combined = %v, want [auth, pkg.git-plumbing]", merged)
	}
	if len(filtered) != 1 || filtered[0].Key != "Diff-Additions" {
		t.Errorf("filtered = %v", filtered)
	}
}

// --- AnnotateVendorCommit tests ---

func TestAnnotateVendorCommit_DefaultsToHEAD(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib", URL: "https://github.com/o/lib"},
		},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("deadbeef12345678901234567890123456789012", nil)
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, "deadbeef12345678901234567890123456789012", gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _, _ string, content string) error {
			var note VendorNoteData
			if err := json.Unmarshal([]byte(content), &note); err != nil {
				t.Fatalf("note is not valid JSON: %v", err)
			}
			if note.Schema != "vendor/v1" {
				t.Errorf("schema = %q", note.Schema)
			}
			if len(note.Vendors) != 1 || note.Vendors[0].Name != "lib" {
				t.Errorf("unexpected vendors: %+v", note.Vendors)
			}
			return nil
		},
	)

	err := AnnotateVendorCommit(context.Background(), mockGit, mockConfig, mockLock, ".", "", "")
	if err != nil {
		t.Fatalf("AnnotateVendorCommit returned error: %v", err)
	}
}

func TestAnnotateVendorCommit_ExplicitHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "lib"}},
	}
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "lib", Ref: "main", CommitHash: "0000000000000000000000000000000000000000"},
		},
	}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	// No GetHeadHash call — explicit hash provided
	mockGit.EXPECT().AddNote(gomock.Any(), ".", VendorNoteRef, "cafebabe12345678901234567890123456789012", gomock.Any()).Return(nil)

	err := AnnotateVendorCommit(context.Background(), mockGit, mockConfig, mockLock, ".", "cafebabe12345678901234567890123456789012", "")
	if err != nil {
		t.Fatalf("AnnotateVendorCommit returned error: %v", err)
	}
}

func TestAnnotateVendorCommit_NoMatchingVendors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	config := types.VendorConfig{Vendors: []types.VendorSpec{}}
	lock := types.VendorLock{Vendors: []types.LockDetails{}}

	mockConfig.EXPECT().Load().Return(config, nil)
	mockLock.EXPECT().Load().Return(lock, nil)
	mockGit.EXPECT().GetHeadHash(gomock.Any(), ".").Return("1111111111111111111111111111111111111111", nil)

	err := AnnotateVendorCommit(context.Background(), mockGit, mockConfig, mockLock, ".", "", "")
	if err == nil {
		t.Fatal("expected error for no matching vendors")
	}
}

// --- minLen test ---

func TestMinLen(t *testing.T) {
	if minLen("hello", 3) != 3 {
		t.Error("minLen should cap at n")
	}
	if minLen("hi", 10) != 2 {
		t.Error("minLen should return len(s) when shorter")
	}
}

// --- helpers ---

func assertHasTrailerKey(t *testing.T, trailers []types.Trailer, key string) {
	t.Helper()
	for _, tr := range trailers {
		if tr.Key == key {
			return
		}
	}
	t.Errorf("expected trailer with key %q not found in %v", key, trailers)
}

func assertTrailer(t *testing.T, trailers []types.Trailer, idx int, key, value string) {
	t.Helper()
	if idx >= len(trailers) {
		t.Fatalf("trailer index %d out of range (len=%d)", idx, len(trailers))
	}
	if trailers[idx].Key != key {
		t.Errorf("trailers[%d].Key = %q, want %q", idx, trailers[idx].Key, key)
	}
	if trailers[idx].Value != value {
		t.Errorf("trailers[%d].Value = %q, want %q", idx, trailers[idx].Value, value)
	}
}

func filterTrailerValues(trailers []types.Trailer, key string) []string {
	var values []string
	for _, t := range trailers {
		if t.Key == key {
			values = append(values, t.Value)
		}
	}
	return values
}

func findTrailerValue(trailers []types.Trailer, key string) string {
	for _, t := range trailers {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}
