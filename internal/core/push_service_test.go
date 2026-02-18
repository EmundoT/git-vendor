package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// TestBuildReverseMapping verifies buildReverseMapping creates the correct
// to -> from mapping from a VendorSpec's path mappings.
func TestBuildReverseMapping(t *testing.T) {
	vendor := &types.VendorSpec{
		Name: "test-vendor",
		URL:  "https://github.com/example/repo",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/lib.go", To: "vendor/lib.go"},
					{From: "src/util.go", To: "vendor/util.go"},
				},
			},
		},
	}

	result := buildReverseMapping(vendor)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["vendor/lib.go"] != "src/lib.go" {
		t.Errorf("expected vendor/lib.go -> src/lib.go, got %s", result["vendor/lib.go"])
	}
	if result["vendor/util.go"] != "src/util.go" {
		t.Errorf("expected vendor/util.go -> src/util.go, got %s", result["vendor/util.go"])
	}
}

// TestBuildReverseMapping_MultiSpec verifies buildReverseMapping merges mappings
// from multiple BranchSpecs.
func TestBuildReverseMapping_MultiSpec(t *testing.T) {
	vendor := &types.VendorSpec{
		Name: "multi",
		URL:  "https://github.com/example/repo",
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "local/a.go"}}},
			{Ref: "v2", Mapping: []types.PathMapping{{From: "b.go", To: "local/b.go"}}},
		},
	}

	result := buildReverseMapping(vendor)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

// TestDetectModifiedFiles verifies that detectModifiedFiles correctly identifies
// files whose local checksum differs from the lockfile hash.
func TestDetectModifiedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create a local file
	localPath := "vendor/lib.go"
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("modified content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute the hash of the "original" content (different from what's on disk)
	cache := NewFileCacheStore(NewOSFileSystem(), ".")

	// The lock has a hash that does NOT match the current file
	lockEntry := &types.LockDetails{
		Name: "test-vendor",
		FileHashes: map[string]string{
			"vendor/lib.go": "deadbeef_not_matching",
		},
	}
	reverseMap := map[string]string{
		"vendor/lib.go": "src/lib.go",
	}

	modified, err := detectModifiedFiles(cache, lockEntry, reverseMap, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 {
		t.Fatalf("expected 1 modified file, got %d", len(modified))
	}
	if modified[0] != "vendor/lib.go" {
		t.Errorf("expected vendor/lib.go, got %s", modified[0])
	}
}

// TestDetectModifiedFiles_NoChanges verifies detectModifiedFiles returns empty
// when local hashes match lockfile hashes.
func TestDetectModifiedFiles_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	localPath := "vendor/lib.go"
	content := []byte("original content")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cache := NewFileCacheStore(NewOSFileSystem(), ".")
	actualHash, err := cache.ComputeFileChecksum(localPath)
	if err != nil {
		t.Fatal(err)
	}

	lockEntry := &types.LockDetails{
		Name: "test-vendor",
		FileHashes: map[string]string{
			"vendor/lib.go": actualHash,
		},
	}
	reverseMap := map[string]string{
		"vendor/lib.go": "src/lib.go",
	}

	modified, err := detectModifiedFiles(cache, lockEntry, reverseMap, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Fatalf("expected 0 modified files, got %d", len(modified))
	}
}

// TestDetectModifiedFiles_FileFilter verifies that --file filtering works
// correctly, only checking the specified file.
func TestDetectModifiedFiles_FileFilter(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create two files, both modified
	for _, path := range []string{"vendor/a.go", "vendor/b.go"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cache := NewFileCacheStore(NewOSFileSystem(), ".")
	lockEntry := &types.LockDetails{
		Name: "test-vendor",
		FileHashes: map[string]string{
			"vendor/a.go": "old_hash_a",
			"vendor/b.go": "old_hash_b",
		},
	}
	reverseMap := map[string]string{
		"vendor/a.go": "src/a.go",
		"vendor/b.go": "src/b.go",
	}

	// Filter to only vendor/a.go
	modified, err := detectModifiedFiles(cache, lockEntry, reverseMap, "vendor/a.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 {
		t.Fatalf("expected 1 modified file (filtered), got %d", len(modified))
	}
	if modified[0] != "vendor/a.go" {
		t.Errorf("expected vendor/a.go, got %s", modified[0])
	}
}

// TestDetectModifiedFiles_DeletedFile verifies that deleted local files are
// skipped (not treated as modifications to push).
func TestDetectModifiedFiles_DeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Don't create the file — it's "deleted"
	cache := NewFileCacheStore(NewOSFileSystem(), ".")
	lockEntry := &types.LockDetails{
		Name: "test-vendor",
		FileHashes: map[string]string{
			"vendor/deleted.go": "some_hash",
		},
	}
	reverseMap := map[string]string{
		"vendor/deleted.go": "src/deleted.go",
	}

	modified, err := detectModifiedFiles(cache, lockEntry, reverseMap, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Fatalf("expected 0 modified files (deleted skipped), got %d", len(modified))
	}
}

// TestDetectProjectName verifies detectProjectName extracts the repo name
// from the origin remote URL.
func TestDetectProjectName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	git := NewMockGitClient(ctrl)
	git.EXPECT().ConfigGet(gomock.Any(), ".", "remote.origin.url").
		Return("https://github.com/user/my-project.git", nil)

	name := detectProjectName(context.Background(), git)
	if name != "my-project" {
		t.Errorf("expected my-project, got %s", name)
	}
}

// TestDetectProjectName_Fallback verifies detectProjectName returns "downstream"
// when the origin URL cannot be determined.
func TestDetectProjectName_Fallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	git := NewMockGitClient(ctrl)
	git.EXPECT().ConfigGet(gomock.Any(), ".", "remote.origin.url").
		Return("", nil)

	name := detectProjectName(context.Background(), git)
	if name != "downstream" {
		t.Errorf("expected downstream, got %s", name)
	}
}

// TestBuildPRURL verifies PR URL construction for GitHub repos.
func TestBuildPRURL(t *testing.T) {
	tests := []struct {
		repoURL    string
		branchName string
		want       string
	}{
		{
			repoURL:    "https://github.com/user/repo.git",
			branchName: "vendor-push/proj/2026-02-18",
			want:       "https://github.com/user/repo/compare/vendor-push/proj/2026-02-18?expand=1",
		},
		{
			repoURL:    "git@github.com:user/repo.git",
			branchName: "vendor-push/proj/2026-02-18",
			want:       "https://github.com/user/repo/compare/vendor-push/proj/2026-02-18?expand=1",
		},
		{
			repoURL:    "https://gitlab.com/user/repo.git",
			branchName: "vendor-push/proj/2026-02-18",
			want:       "https://gitlab.com/user/repo/-/merge_requests/new?merge_request[source_branch]=vendor-push/proj/2026-02-18",
		},
	}

	for _, tt := range tests {
		got := buildPRURL(tt.repoURL, tt.branchName)
		if got != tt.want {
			t.Errorf("buildPRURL(%q, %q) = %q, want %q", tt.repoURL, tt.branchName, got, tt.want)
		}
	}
}

// TestBuildManualInstructions verifies the manual instructions contain
// the essential information (branch name, source URL, file list).
func TestBuildManualInstructions(t *testing.T) {
	instructions := buildManualInstructions(
		"https://github.com/user/repo.git",
		"vendor-push/proj/2026-02-18",
		"proj",
		[]string{"vendor/lib.go"},
	)

	for _, want := range []string{
		"vendor-push/proj/2026-02-18",
		"github.com/user/repo",
		"vendor/lib.go",
	} {
		if !containsSubstring(instructions, want) {
			t.Errorf("manual instructions missing %q", want)
		}
	}
}

// TestPushVendor_VendorNotFound verifies PushVendor returns an error
// when the specified vendor does not exist in config.
func TestPushVendor_VendorNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "other-vendor", URL: "https://example.com"},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)

	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
	}

	_, err := syncer.PushVendor(context.Background(), PushOptions{VendorName: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent vendor")
	}
	if !containsSubstring(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %s", err.Error())
	}
}

// TestPushVendor_InternalVendorRejected verifies PushVendor rejects internal vendors.
func TestPushVendor_InternalVendorRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "internal-vendor", URL: "", Source: "internal"},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)

	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
	}

	_, err := syncer.PushVendor(context.Background(), PushOptions{VendorName: "internal-vendor"})
	if err == nil {
		t.Fatal("expected error for internal vendor")
	}
	if !containsSubstring(err.Error(), "internal") {
		t.Errorf("expected 'internal' in error, got: %s", err.Error())
	}
}

// TestPushVendor_NoLockEntry verifies PushVendor returns an error
// when the vendor has no lockfile entry.
func TestPushVendor_NoLockEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "my-vendor", URL: "https://github.com/example/repo"},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{}, // No lock entry for my-vendor
	}, nil)

	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
	}

	_, err := syncer.PushVendor(context.Background(), PushOptions{VendorName: "my-vendor"})
	if err == nil {
		t.Fatal("expected error for missing lock entry")
	}
	if !containsSubstring(err.Error(), "no lock entry") {
		t.Errorf("expected 'no lock entry' in error, got: %s", err.Error())
	}
}

// TestPushVendor_DryRun verifies PushVendor in dry-run mode detects modified files
// without performing any git operations.
func TestPushVendor_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a locally modified file
	localPath := "vendor/lib.go"
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("modified locally"), 0o644); err != nil {
		t.Fatal(err)
	}

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "my-vendor",
				URL:  "https://github.com/example/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/lib.go", To: "vendor/lib.go"},
						},
					},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "my-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{
					"vendor/lib.go": "old_hash_not_matching",
				},
			},
		},
	}, nil)

	fs := NewOSFileSystem()
	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
		fs:          fs,
		rootDir:     ".",
	}

	result, err := syncer.PushVendor(context.Background(), PushOptions{
		VendorName: "my-vendor",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true in result")
	}
	if len(result.FilesModified) != 1 {
		t.Fatalf("expected 1 modified file, got %d", len(result.FilesModified))
	}
	if result.FilesModified[0] != "vendor/lib.go" {
		t.Errorf("expected vendor/lib.go, got %s", result.FilesModified[0])
	}
	if result.ReverseMapping["vendor/lib.go"] != "src/lib.go" {
		t.Errorf("expected reverse mapping vendor/lib.go -> src/lib.go, got %s", result.ReverseMapping["vendor/lib.go"])
	}
	// In dry-run mode, no branch or PR should be created
	if result.BranchName != "" {
		t.Errorf("expected empty BranchName in dry-run, got %s", result.BranchName)
	}
}

// TestPushVendor_NoModifications verifies PushVendor returns an empty result
// when no files are modified.
func TestPushVendor_NoModifications(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a file with content that matches the lock hash
	localPath := "vendor/lib.go"
	content := []byte("original content")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cache := NewFileCacheStore(NewOSFileSystem(), ".")
	actualHash, err := cache.ComputeFileChecksum(localPath)
	if err != nil {
		t.Fatal(err)
	}

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "my-vendor",
				URL:  "https://github.com/example/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src/lib.go", To: "vendor/lib.go"}}},
				},
			},
		},
	}, nil)

	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "my-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{
					"vendor/lib.go": actualHash,
				},
			},
		},
	}, nil)

	syncer := &VendorSyncer{
		configStore: configStore,
		lockStore:   lockStore,
		fs:          NewOSFileSystem(),
		rootDir:     ".",
	}

	result, err := syncer.PushVendor(context.Background(), PushOptions{VendorName: "my-vendor"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.FilesModified) != 0 {
		t.Errorf("expected 0 modified files, got %d", len(result.FilesModified))
	}
}

// TestPushVendor_EmptyVendorName verifies PushVendor returns an error
// when no vendor name is provided.
func TestPushVendor_EmptyVendorName(t *testing.T) {
	syncer := &VendorSyncer{}
	_, err := syncer.PushVendor(context.Background(), PushOptions{VendorName: ""})
	if err == nil {
		t.Fatal("expected error for empty vendor name")
	}
}

// TestIsGhInstalled verifies isGhInstalled does not panic and returns a bool.
func TestIsGhInstalled(t *testing.T) {
	// Just verify it doesn't panic — result depends on environment
	_ = isGhInstalled()
}

// containsSubstring is a test helper for checking if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
