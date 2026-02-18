package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// MatchesExclude Unit Tests
// ============================================================================

func TestMatchesExclude_SimpleGlob(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{"match md extension", "README.md", "*.md", true},
		{"no match go file", "main.go", "*.md", false},
		{"match hidden file", ".gitignore", ".gitignore", true},
		{"no match nested md", "docs/guide.md", "*.md", false}, // * does not cross /
		{"match txt", "notes.txt", "*.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesExclude(tt.path, []string{tt.pattern})
			if got != tt.want {
				t.Errorf("MatchesExclude(%q, [%q]) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesExclude_DirectoryGlob(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{"match dir child", ".claude/settings.json", ".claude/**", true},
		{"match dir nested", ".claude/rules/foo.md", ".claude/**", true},
		{"match dir itself", ".claude", ".claude/**", true},
		{"no match sibling", "src/main.go", ".claude/**", false},
		{"match github dir", ".github/workflows/ci.yml", ".github/**", true},
		{"match docs internal", "docs/internal/design.md", "docs/internal/**", true},
		{"no match docs root", "docs/README.md", "docs/internal/**", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesExclude(tt.path, []string{tt.pattern})
			if got != tt.want {
				t.Errorf("MatchesExclude(%q, [%q]) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesExclude_MultiplePatterns(t *testing.T) {
	patterns := []string{".claude/**", ".github/**", "README.md"}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"match claude", ".claude/settings.json", true},
		{"match github", ".github/workflows/ci.yml", true},
		{"match readme", "README.md", true},
		{"no match source", "src/main.go", false},
		{"no match nested readme", "docs/README.md", false}, // exact match only for non-glob
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesExclude(tt.path, patterns)
			if got != tt.want {
				t.Errorf("MatchesExclude(%q, %v) = %v, want %v", tt.path, patterns, got, tt.want)
			}
		})
	}
}

func TestMatchesExclude_NoPatterns(t *testing.T) {
	// No patterns means nothing is excluded
	if MatchesExclude("any/file.go", nil) {
		t.Error("MatchesExclude with nil patterns should return false")
	}
	if MatchesExclude("any/file.go", []string{}) {
		t.Error("MatchesExclude with empty patterns should return false")
	}
}

func TestMatchesExclude_RecursiveGlobSuffix(t *testing.T) {
	// Pattern: **/*.md should match .md files at any depth
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"root level md", "README.md", true},
		{"nested md", "docs/guide.md", true},
		{"deep nested md", "a/b/c/notes.md", true},
		{"go file", "main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesExclude(tt.path, []string{"**/*.md"})
			if got != tt.want {
				t.Errorf("MatchesExclude(%q, [**/*.md]) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestMatchesExclude_CrossPlatformSlashes(t *testing.T) {
	// Backslashes should be normalized to forward slashes
	got := MatchesExclude(filepath.Join(".claude", "settings.json"), []string{".claude/**"})
	if !got {
		t.Error("MatchesExclude should normalize OS path separators for matching")
	}
}

// ============================================================================
// CopyDir with Excludes Integration Tests
// ============================================================================

// TestCopyDir_ExcludeSimpleGlob verifies that simple glob patterns (e.g., "*.md")
// exclude matching files at the root of the copied directory.
func TestCopyDir_ExcludeSimpleGlob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# readme"), 0644)
	os.WriteFile(filepath.Join(srcDir, "utils.go"), []byte("package utils"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	stats, err := svc.copyDirWithExcludes(srcDir, dstDir, []string{"*.md"})
	if err != nil {
		t.Fatalf("copyDirWithExcludes failed: %v", err)
	}

	// 2 .go files copied, 1 .md excluded
	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}
	if stats.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1", stats.Excluded)
	}

	// Verify README.md was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md should have been excluded")
	}
	// Verify .go files WERE copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); err != nil {
		t.Errorf("main.go should have been copied: %v", err)
	}
}

// TestCopyDir_ExcludeDirectoryGlob verifies that directory glob patterns (e.g., ".claude/**")
// exclude entire directory trees.
func TestCopyDir_ExcludeDirectoryGlob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure with .claude/ directory
	os.MkdirAll(filepath.Join(srcDir, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".claude", "settings.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".claude", "rules", "rule1.md"), []byte("rule"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	stats, err := svc.copyDirWithExcludes(srcDir, dstDir, []string{".claude/**"})
	if err != nil {
		t.Fatalf("copyDirWithExcludes failed: %v", err)
	}

	// 1 .go file copied, .claude dir skipped entirely via SkipDir
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}

	// Verify .claude directory was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, ".claude")); !os.IsNotExist(err) {
		t.Error(".claude directory should have been excluded")
	}
	// Verify main.go WAS copied
	data, err := os.ReadFile(filepath.Join(dstDir, "main.go"))
	if err != nil {
		t.Fatalf("main.go not copied: %v", err)
	}
	if string(data) != "package main" {
		t.Errorf("main.go content = %q, want %q", string(data), "package main")
	}
}

// TestCopyDir_ExcludeMultiplePatterns verifies that multiple exclude patterns
// are applied together.
func TestCopyDir_ExcludeMultiplePatterns(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure
	os.MkdirAll(filepath.Join(srcDir, ".claude"), 0755)
	os.MkdirAll(filepath.Join(srcDir, ".github", "workflows"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".claude", "settings.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".github", "workflows", "ci.yml"), []byte("ci"), 0644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# readme"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "lib.go"), []byte("package lib"), 0644)

	excludes := []string{".claude/**", ".github/**", "README.md"}
	svc := NewFileCopyService(NewOSFileSystem())
	stats, err := svc.copyDirWithExcludes(srcDir, dstDir, excludes)
	if err != nil {
		t.Fatalf("copyDirWithExcludes failed: %v", err)
	}

	// 2 .go files copied; .claude (SkipDir), .github (SkipDir), README.md excluded
	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}
	// README.md is counted as excluded file; .claude and .github hit SkipDir at dir level
	if stats.Excluded < 1 {
		t.Errorf("Excluded = %d, want >= 1", stats.Excluded)
	}

	// Verify excluded files were NOT copied
	for _, excluded := range []string{".claude", ".github", "README.md"} {
		if _, err := os.Stat(filepath.Join(dstDir, excluded)); !os.IsNotExist(err) {
			t.Errorf("%s should have been excluded", excluded)
		}
	}
}

// TestCopyDir_NoExcludePatterns verifies backward compatibility — when no exclude
// patterns are specified, copyDirWithExcludes copies everything (same as CopyDir).
func TestCopyDir_NoExcludePatterns(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# readme"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	stats, err := svc.copyDirWithExcludes(srcDir, dstDir, nil)
	if err != nil {
		t.Fatalf("copyDirWithExcludes failed: %v", err)
	}

	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}
	if stats.Excluded != 0 {
		t.Errorf("Excluded = %d, want 0", stats.Excluded)
	}
}

// TestCopyDir_ExcludeSkipsGitDirectories verifies that .git directories are always
// skipped, consistent with OSFileSystem.CopyDir behavior.
func TestCopyDir_ExcludeSkipsGitDirectories(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	stats, err := svc.copyDirWithExcludes(srcDir, dstDir, []string{"*.md"})
	if err != nil {
		t.Fatalf("copyDirWithExcludes failed: %v", err)
	}

	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}

	// .git should NOT be copied
	if _, err := os.Stat(filepath.Join(dstDir, ".git")); !os.IsNotExist(err) {
		t.Error(".git should be skipped regardless of exclude patterns")
	}
}

// ============================================================================
// Sync Integration: Excluded files NOT in lock
// ============================================================================

// TestSync_ExcludedFilesNotInLock verifies that when exclude patterns are used,
// excluded files do not appear in the copied output and therefore would not
// appear in lockfile FileHashes (since hashes are computed from destination files).
func TestSync_ExcludedFilesNotInLock(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	// Create source directory simulating a repo root
	os.MkdirAll(filepath.Join(repoDir, "src"), 0755)
	os.MkdirAll(filepath.Join(repoDir, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(repoDir, "src", "lib.go"), []byte("package lib"), 0644)
	os.WriteFile(filepath.Join(repoDir, ".claude", "settings.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(repoDir, ".claude", "rules", "rule.md"), []byte("rule"), 0644)
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# readme"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "test-vendor"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{
				From:    ".",
				To:      "pkg/test-vendor",
				Exclude: []string{".claude/**", "README.md"},
			},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("CopyMappings failed: %v", err)
	}

	// Only src/lib.go should be copied (1 file)
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
	if stats.Excluded < 1 {
		t.Errorf("Excluded = %d, want >= 1", stats.Excluded)
	}

	// Verify excluded files were NOT copied to destination
	destDir := filepath.Join(workDir, "pkg", "test-vendor")
	if _, err := os.Stat(filepath.Join(destDir, ".claude")); !os.IsNotExist(err) {
		t.Error(".claude should not be in destination")
	}
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md should not be in destination")
	}

	// Verify src/lib.go WAS copied
	data, err := os.ReadFile(filepath.Join(destDir, "src", "lib.go"))
	if err != nil {
		t.Fatalf("src/lib.go not copied: %v", err)
	}
	if string(data) != "package lib" {
		t.Errorf("lib.go content = %q, want %q", string(data), "package lib")
	}
}

// TestCopyMappings_ExcludeOnlyAffectsDirectoryMappings verifies that exclude
// patterns on file-level mappings do not cause errors (they are silently ignored).
func TestCopyMappings_ExcludeOnlyAffectsDirectoryMappings(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	os.WriteFile(filepath.Join(repoDir, "single.go"), []byte("package single"), 0644)

	svc := NewFileCopyService(NewOSFileSystem())
	vendor := &types.VendorSpec{Name: "test-vendor"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{
				From:    "single.go",
				To:      "out/single.go",
				Exclude: []string{"*.md"}, // should be ignored for file mapping
			},
		},
	}

	stats, err := svc.CopyMappings(repoDir, vendor, spec)
	if err != nil {
		t.Fatalf("CopyMappings with exclude on file mapping should not error: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
}
