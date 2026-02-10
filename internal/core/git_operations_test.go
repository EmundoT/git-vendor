package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// GetCommitLog Tests
// ============================================================================

func TestSystemGitClient_GetCommitLog(t *testing.T) {
	git := NewSystemGitClient(false)
	tempDir := t.TempDir()

	// Initialize git repository
	if err := git.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	configureGitUser(t, tempDir)

	// Create first commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitSilent(t, tempDir, "add", "file1.txt")
	runGitSilent(t, tempDir, "commit", "-m", "First commit")

	// Get first commit hash
	hash1, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get first commit hash: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second commit
	file2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0644)
	runGitSilent(t, tempDir, "add", "file2.txt")
	runGitSilent(t, tempDir, "commit", "-m", "Second commit")

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create third commit
	file3 := filepath.Join(tempDir, "file3.txt")
	os.WriteFile(file3, []byte("content3"), 0644)
	runGitSilent(t, tempDir, "add", "file3.txt")
	runGitSilent(t, tempDir, "commit", "-m", "Third commit")

	// Get final commit hash
	hash3, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get final commit hash: %v", err)
	}

	// Test: Get commit log between first and third commit
	commits, err := git.GetCommitLog(context.Background(), tempDir, hash1, hash3, 0)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}

	// Should have 2 commits (second and third)
	if len(commits) != 2 {
		t.Errorf("Expected 2 commits, got %d", len(commits))
	}

	// Verify commit order (newest first)
	if len(commits) >= 2 {
		if commits[0].Subject != "Third commit" {
			t.Errorf("Expected first commit subject 'Third commit', got '%s'", commits[0].Subject)
		}
		if commits[1].Subject != "Second commit" {
			t.Errorf("Expected second commit subject 'Second commit', got '%s'", commits[1].Subject)
		}
	}

	// Verify commit structure
	if len(commits) > 0 {
		commit := commits[0]
		if commit.Hash == "" {
			t.Error("Commit hash is empty")
		}
		if commit.ShortHash == "" {
			t.Error("Commit short hash is empty")
		}
		if commit.Author == "" {
			t.Error("Commit author is empty")
		}
		if commit.Date == "" {
			t.Error("Commit date is empty")
		}
		// Short hash should be prefix of full hash
		if len(commit.Hash) < len(commit.ShortHash) {
			t.Errorf("Short hash '%s' is longer than full hash '%s'", commit.ShortHash, commit.Hash)
		}
	}
}

func TestSystemGitClient_GetCommitLog_MaxCount(t *testing.T) {
	git := NewSystemGitClient(false)
	tempDir := t.TempDir()

	// Initialize git repository
	if err := git.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create first commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitSilent(t, tempDir, "add", "file1.txt")
	runGitSilent(t, tempDir, "commit", "-m", "First commit")

	hash1, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get first commit hash: %v", err)
	}

	// Create 5 more commits
	for i := 2; i <= 6; i++ {
		time.Sleep(10 * time.Millisecond)
		fileName := filepath.Join(tempDir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(fileName, []byte("content"), 0644)
		runGitSilent(t, tempDir, "add", ".")
		runGitSilent(t, tempDir, "commit", "-m", "Commit "+string(rune('0'+i)))
	}

	hash6, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get final commit hash: %v", err)
	}

	// Test: Limit to 3 commits
	commits, err := git.GetCommitLog(context.Background(), tempDir, hash1, hash6, 3)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}

	// Should have exactly 3 commits (limited by maxCount)
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits with maxCount=3, got %d", len(commits))
	}
}

func TestSystemGitClient_GetCommitLog_EmptyRange(t *testing.T) {
	git := NewSystemGitClient(false)
	tempDir := t.TempDir()

	// Initialize git repository
	if err := git.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create single commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitSilent(t, tempDir, "add", "file1.txt")
	runGitSilent(t, tempDir, "commit", "-m", "First commit")

	hash1, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Test: Get log from hash to itself (no new commits)
	commits, err := git.GetCommitLog(context.Background(), tempDir, hash1, hash1, 0)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}

	// Should have 0 commits (same hash)
	if len(commits) != 0 {
		t.Errorf("Expected 0 commits for same hash range, got %d", len(commits))
	}
}

func TestSystemGitClient_GetCommitLog_InvalidRange(t *testing.T) {
	git := NewSystemGitClient(false)
	tempDir := t.TempDir()

	// Initialize git repository
	if err := git.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitSilent(t, tempDir, "add", "file1.txt")
	runGitSilent(t, tempDir, "commit", "-m", "First commit")

	// Test: Invalid hash should error
	_, err := git.GetCommitLog(context.Background(), tempDir, "invalid-hash", "another-invalid-hash", 0)
	if err == nil {
		t.Error("Expected error for invalid commit hashes, got nil")
	}
}

// GetTagForCommit Tests — Semver Preference Logic
// ============================================================================

func TestSystemGitClient_GetTagForCommit_SemverPreference(t *testing.T) {
	gitClient := NewSystemGitClient(false)
	tempDir := t.TempDir()

	if err := gitClient.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	configureGitUser(t, tempDir)

	// Create a commit with both semver and non-semver tags
	file := filepath.Join(tempDir, "file.txt")
	os.WriteFile(file, []byte("content"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "Tagged commit")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Tag with non-semver first, then semver — GetTagForCommit should prefer semver
	runGitSilent(t, tempDir, "tag", "release-2025")
	runGitSilent(t, tempDir, "tag", "v1.2.3")

	tag, err := gitClient.GetTagForCommit(context.Background(), tempDir, hash)
	if err != nil {
		t.Fatalf("GetTagForCommit failed: %v", err)
	}
	if tag != "v1.2.3" {
		t.Errorf("Expected semver tag 'v1.2.3', got '%s'", tag)
	}
}

func TestSystemGitClient_GetTagForCommit_NonSemverFallback(t *testing.T) {
	gitClient := NewSystemGitClient(false)
	tempDir := t.TempDir()

	if err := gitClient.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	configureGitUser(t, tempDir)

	file := filepath.Join(tempDir, "file.txt")
	os.WriteFile(file, []byte("content"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "Non-semver commit")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Only non-semver tags — should fall back to first tag
	runGitSilent(t, tempDir, "tag", "release-2025")

	tag, err := gitClient.GetTagForCommit(context.Background(), tempDir, hash)
	if err != nil {
		t.Fatalf("GetTagForCommit failed: %v", err)
	}
	if tag != "release-2025" {
		t.Errorf("Expected fallback tag 'release-2025', got '%s'", tag)
	}
}

func TestSystemGitClient_GetTagForCommit_NoTags(t *testing.T) {
	gitClient := NewSystemGitClient(false)
	tempDir := t.TempDir()

	if err := gitClient.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	configureGitUser(t, tempDir)

	file := filepath.Join(tempDir, "file.txt")
	os.WriteFile(file, []byte("content"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "Untagged commit")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// No tags at all — should return empty string
	tag, err := gitClient.GetTagForCommit(context.Background(), tempDir, hash)
	if err != nil {
		t.Fatalf("GetTagForCommit failed: %v", err)
	}
	if tag != "" {
		t.Errorf("Expected empty tag for untagged commit, got '%s'", tag)
	}
}

func TestSystemGitClient_GetTagForCommit_SemverWithoutPrefix(t *testing.T) {
	gitClient := NewSystemGitClient(false)
	tempDir := t.TempDir()

	if err := gitClient.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	configureGitUser(t, tempDir)

	file := filepath.Join(tempDir, "file.txt")
	os.WriteFile(file, []byte("content"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "Semver no prefix")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Semver without 'v' prefix should still be preferred
	runGitSilent(t, tempDir, "tag", "build-42")
	runGitSilent(t, tempDir, "tag", "2.0.0")

	tag, err := gitClient.GetTagForCommit(context.Background(), tempDir, hash)
	if err != nil {
		t.Fatalf("GetTagForCommit failed: %v", err)
	}
	if tag != "2.0.0" {
		t.Errorf("Expected semver tag '2.0.0', got '%s'", tag)
	}
}

// ============================================================================
// ParseSmartURL Tests
// ============================================================================

func TestParseSmartURL_GitHubBlobURL(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/blob/main/src/file.go")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", ref)
	}
	if path != "src/file.go" {
		t.Errorf("Expected path 'src/file.go', got '%s'", path)
	}
}

func TestParseSmartURL_GitHubTreeURL(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/tree/main/src/dir/")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", ref)
	}
	if path != "src/dir/" {
		t.Errorf("Expected path 'src/dir/', got '%s'", path)
	}
}

func TestParseSmartURL_TagURLWithSlashInPath(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/tree/v1.0/src/nested/dir")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "v1.0" {
		t.Errorf("Expected ref 'v1.0', got '%s'", ref)
	}
	if path != "src/nested/dir" {
		t.Errorf("Expected path 'src/nested/dir', got '%s'", path)
	}
}

func TestParseSmartURL_BareRepoURL(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "" {
		t.Errorf("Expected empty ref, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}
}

func TestParseSmartURL_TrailingSlash(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "" {
		t.Errorf("Expected empty ref, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}
}

func TestParseSmartURL_DotGitSuffix(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo.git")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "" {
		t.Errorf("Expected empty ref, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}
}

func TestParseSmartURL_SSHURLFormat(t *testing.T) {
	// SSH URLs (git@github.com:owner/repo) don't match the deep-link regex.
	// ParseSmartURL should return the cleaned URL as base, no ref/path.
	base, ref, path := ParseSmartURL("git@github.com:owner/repo")
	if ref != "" {
		t.Errorf("Expected empty ref for SSH URL, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path for SSH URL, got '%s'", path)
	}
	// SSH URL won't match deep link regex, base returned as-is (minus trailing slash/.git)
	if base != "git@github.com:owner/repo" {
		t.Errorf("Expected base 'git@github.com:owner/repo', got '%s'", base)
	}
}

func TestParseSmartURL_SSHURLWithDotGit(t *testing.T) {
	base, ref, path := ParseSmartURL("git@github.com:owner/repo.git")
	if base != "git@github.com:owner/repo" {
		t.Errorf("Expected base 'git@github.com:owner/repo', got '%s'", base)
	}
	if ref != "" {
		t.Errorf("Expected empty ref, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}
}

func TestParseSmartURL_HTTPvsHTTPS(t *testing.T) {
	// HTTP URL should not match deep-link regex (the regex targets github.com specifically)
	base, ref, path := ParseSmartURL("http://github.com/owner/repo")
	if ref != "" {
		t.Errorf("Expected empty ref for HTTP URL, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path for HTTP URL, got '%s'", path)
	}
	if base != "http://github.com/owner/repo" {
		t.Errorf("Expected base 'http://github.com/owner/repo', got '%s'", base)
	}
}

func TestParseSmartURL_GitLabNestedGroupURL(t *testing.T) {
	// GitLab nested groups don't match the GitHub-specific deep-link regex
	base, ref, path := ParseSmartURL("https://gitlab.com/group/subgroup/repo")
	if ref != "" {
		t.Errorf("Expected empty ref for GitLab URL, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path for GitLab URL, got '%s'", path)
	}
	if base != "https://gitlab.com/group/subgroup/repo" {
		t.Errorf("Expected base 'https://gitlab.com/group/subgroup/repo', got '%s'", base)
	}
}

func TestParseSmartURL_BitbucketURL(t *testing.T) {
	base, ref, path := ParseSmartURL("https://bitbucket.org/owner/repo")
	if base != "https://bitbucket.org/owner/repo" {
		t.Errorf("Expected base 'https://bitbucket.org/owner/repo', got '%s'", base)
	}
	if ref != "" {
		t.Errorf("Expected empty ref, got '%s'", ref)
	}
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}
}

func TestParseSmartURL_WhitespaceHandling(t *testing.T) {
	base, ref, path := ParseSmartURL("  https://github.com/owner/repo/blob/main/file.go  ")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "main" {
		t.Errorf("Expected ref 'main', got '%s'", ref)
	}
	if path != "file.go" {
		t.Errorf("Expected path 'file.go', got '%s'", path)
	}
}

func TestParseSmartURL_DeepLinkWithNestedPath(t *testing.T) {
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/blob/v2.0.0/src/internal/pkg/util.go")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	if ref != "v2.0.0" {
		t.Errorf("Expected ref 'v2.0.0', got '%s'", ref)
	}
	if path != "src/internal/pkg/util.go" {
		t.Errorf("Expected path 'src/internal/pkg/util.go', got '%s'", path)
	}
}

func TestParseSmartURL_BranchLikeFeaturePath(t *testing.T) {
	// Branch names with slashes can't be parsed — documented limitation.
	// feature/v2/main appears as ref="feature" path="v2/main/src/file.go"
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/blob/feature/v2/main/src/file.go")
	if base != "https://github.com/owner/repo" {
		t.Errorf("Expected base 'https://github.com/owner/repo', got '%s'", base)
	}
	// The regex captures the first path segment as ref
	if ref != "feature" {
		t.Errorf("Expected ref 'feature' (slash branch limitation), got '%s'", ref)
	}
	// Rest becomes path
	if path != "v2/main/src/file.go" {
		t.Errorf("Expected path 'v2/main/src/file.go', got '%s'", path)
	}
}

func TestParseSmartURL_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBase string
		wantRef  string
		wantPath string
	}{
		{
			name:     "GitHub blob with commit SHA",
			input:    "https://github.com/owner/repo/blob/abc123def/src/main.go",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "abc123def",
			wantPath: "src/main.go",
		},
		{
			name:     "GitHub tree with tag",
			input:    "https://github.com/owner/repo/tree/v1.2.3/internal/",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "v1.2.3",
			wantPath: "internal/",
		},
		{
			name:     "Bare URL with trailing .git and slash",
			input:    "https://github.com/owner/repo.git/",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "Self-hosted GitHub Enterprise",
			input:    "https://github.example.com/owner/repo",
			wantBase: "https://github.example.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "URL with only owner (no repo)",
			input:    "https://github.com/owner",
			wantBase: "https://github.com/owner",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "Empty string",
			input:    "",
			wantBase: "",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "Just a domain",
			input:    "https://github.com",
			wantBase: "https://github.com",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "Single file at root blob",
			input:    "https://github.com/owner/repo/blob/main/README.md",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotRef, gotPath := ParseSmartURL(tt.input)
			if gotBase != tt.wantBase {
				t.Errorf("base: got '%s', want '%s'", gotBase, tt.wantBase)
			}
			if gotRef != tt.wantRef {
				t.Errorf("ref: got '%s', want '%s'", gotRef, tt.wantRef)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path: got '%s', want '%s'", gotPath, tt.wantPath)
			}
		})
	}
}

// GetCommitLog Date Format Test
// ============================================================================

func TestSystemGitClient_GetCommitLog_DateFormat(t *testing.T) {
	gitClient := NewSystemGitClient(false)
	tempDir := t.TempDir()

	if err := gitClient.Init(context.Background(), tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	configureGitUser(t, tempDir)

	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "First commit")
	hash1, _ := gitClient.GetHeadHash(context.Background(), tempDir)

	time.Sleep(10 * time.Millisecond)

	file2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0644)
	runGitSilent(t, tempDir, "add", ".")
	runGitSilent(t, tempDir, "commit", "-m", "Second commit")
	hash2, _ := gitClient.GetHeadHash(context.Background(), tempDir)

	commits, err := gitClient.GetCommitLog(context.Background(), tempDir, hash1, hash2, 0)
	if err != nil {
		t.Fatalf("GetCommitLog failed: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(commits))
	}

	// Verify date format matches "YYYY-MM-DD HH:MM:SS ±ZZZZ" (git %ai format)
	// formatDate() in diff_service.go splits on whitespace and expects parts[0] = date
	date := commits[0].Date
	parts := strings.Fields(date)
	if len(parts) != 3 {
		t.Errorf("Expected date with 3 space-separated parts (date time tz), got %d parts: %q", len(parts), date)
	}
	if len(parts) >= 1 && len(parts[0]) != 10 {
		t.Errorf("Expected YYYY-MM-DD (10 chars) for date part, got %q", parts[0])
	}
}

// ============================================================================
// cleanURL Tests
// ============================================================================

func TestCleanURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  https://github.com/owner/repo  ", "https://github.com/owner/repo"},
		{"\\https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"", ""},
		{"  \\\\url  ", "url"},
	}

	for _, tt := range tests {
		got := cleanURL(tt.input)
		if got != tt.want {
			t.Errorf("cleanURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================================
// isSemverTag Tests
// ============================================================================

func TestIsSemverTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"v1.0.0", true},
		{"1.0.0", true},
		{"v2.3.4", true},
		{"v0.0.1", true},
		{"v1.0.0-beta", true},
		{"release-1.0", false},
		{"latest", false},
		{"", false},
		{"v", false},
	}

	for _, tt := range tests {
		got := isSemverTag(tt.tag)
		if got != tt.want {
			t.Errorf("isSemverTag(%q) = %v, want %v", tt.tag, got, tt.want)
		}
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func configureGitUser(t *testing.T, dir string) {
	t.Helper()
	runGitSilent(t, dir, "config", "user.name", "Test User")
	runGitSilent(t, dir, "config", "user.email", "test@example.com")
}

// runGitSilent executes a git command discarding stdout. Automatically injects
// -c commit.gpgsign=false per invocation for non-integration tests that lack
// per-repo config. Contrast with runGitOutput (integration_test.go) which
// returns output and relies on per-repo gpgsign config.
func runGitSilent(t *testing.T, dir string, args ...string) {
	t.Helper()
	// Disable commit signing for tests to avoid environment-specific failures
	fullArgs := append([]string{"-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
}

// ============================================================================
// ParseSmartURL Multi-Provider Edge Cases
// ============================================================================

func TestParseSmartURL_GitHubDeepLinks(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
		wantRef  string
		wantPath string
	}{
		{
			name:     "blob link",
			rawURL:   "https://github.com/owner/repo/blob/main/src/file.go",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "src/file.go",
		},
		{
			name:     "tree link with tag",
			rawURL:   "https://github.com/owner/repo/tree/v1.0/src/",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "v1.0",
			wantPath: "src/",
		},
		{
			name:     "bare repo URL",
			rawURL:   "https://github.com/owner/repo",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "bare with trailing slash",
			rawURL:   "https://github.com/owner/repo/",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "with .git suffix",
			rawURL:   "https://github.com/owner/repo.git",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "deep link nested path",
			rawURL:   "https://github.com/owner/repo/blob/main/a/b/c/d.go",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "a/b/c/d.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ref = %q, want %q", ref, tt.wantRef)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestParseSmartURL_GitLabNestedGroups(t *testing.T) {
	// GitLab nested groups do NOT match the github.com regex,
	// so they should return the URL as base with no ref or path.
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{
			name:     "simple GitLab",
			rawURL:   "https://gitlab.com/owner/repo",
			wantBase: "https://gitlab.com/owner/repo",
		},
		{
			name:     "nested group",
			rawURL:   "https://gitlab.com/owner/group/subgroup/repo",
			wantBase: "https://gitlab.com/owner/group/subgroup/repo",
		},
		{
			name:     "deeply nested group",
			rawURL:   "https://gitlab.com/org/team/sub1/sub2/repo",
			wantBase: "https://gitlab.com/org/team/sub1/sub2/repo",
		},
		{
			name:     "GitLab with .git suffix",
			rawURL:   "https://gitlab.com/owner/repo.git",
			wantBase: "https://gitlab.com/owner/repo",
		},
		{
			name:     "self-hosted GitLab",
			rawURL:   "https://gitlab.mycompany.com/team/project",
			wantBase: "https://gitlab.mycompany.com/team/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != "" {
				t.Errorf("ref = %q, want empty (GitLab not parsed for deep links)", ref)
			}
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
		})
	}
}

func TestParseSmartURL_BitbucketURLs(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{
			name:     "Bitbucket cloud",
			rawURL:   "https://bitbucket.org/owner/repo",
			wantBase: "https://bitbucket.org/owner/repo",
		},
		{
			name:     "Bitbucket with .git suffix",
			rawURL:   "https://bitbucket.org/owner/repo.git",
			wantBase: "https://bitbucket.org/owner/repo",
		},
		{
			name:     "Bitbucket with trailing slash",
			rawURL:   "https://bitbucket.org/owner/repo/",
			wantBase: "https://bitbucket.org/owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != "" {
				t.Errorf("ref = %q, want empty", ref)
			}
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
		})
	}
}

func TestParseSmartURL_AuthTokenURLs(t *testing.T) {
	// URLs with embedded auth tokens
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{
			name:     "token@ prefix in GitHub URL",
			rawURL:   "https://token@github.com/owner/repo",
			wantBase: "https://token@github.com/owner/repo",
		},
		{
			name:     "oauth2 token in GitLab URL",
			rawURL:   "https://oauth2:glpat-abc123@gitlab.com/owner/repo",
			wantBase: "https://oauth2:glpat-abc123@gitlab.com/owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			// Auth token URLs should not match deep link regex (no blob/tree)
			if ref != "" {
				t.Errorf("ref = %q, want empty", ref)
			}
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
		})
	}
}

func TestParseSmartURL_SSHURLs(t *testing.T) {
	// SSH URLs should be returned as-is (minus .git suffix)
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{
			name:     "SSH with .git suffix",
			rawURL:   "git@github.com:owner/repo.git",
			wantBase: "git@github.com:owner/repo",
		},
		{
			name:     "SSH without .git suffix",
			rawURL:   "git@github.com:owner/repo",
			wantBase: "git@github.com:owner/repo",
		},
		{
			name:     "SSH GitLab",
			rawURL:   "git@gitlab.com:owner/repo.git",
			wantBase: "git@gitlab.com:owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != "" {
				t.Errorf("ref = %q, want empty", ref)
			}
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
		})
	}
}

func TestParseSmartURL_BranchWithSlashes(t *testing.T) {
	// Branch names with slashes are a documented limitation.
	// The regex captures the first path segment as the ref, which is incorrect
	// for branches like "feature/foo". This test documents the limitation.
	base, ref, path := ParseSmartURL("https://github.com/owner/repo/blob/feature/foo/src/file.go")

	// Due to the regex, "feature" is captured as ref, and "foo/src/file.go" as path.
	// This is the known limitation documented in CLAUDE.md.
	if base != "https://github.com/owner/repo" {
		t.Errorf("base = %q, want 'https://github.com/owner/repo'", base)
	}
	if ref != "feature" {
		t.Errorf("ref = %q, want 'feature' (known limitation: only first segment captured)", ref)
	}
	if path != "foo/src/file.go" {
		t.Errorf("path = %q, want 'foo/src/file.go' (known limitation: rest captured as path)", path)
	}
}

func TestParseSmartURL_SelfHostedInstances(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{
			name:     "GitHub Enterprise",
			rawURL:   "https://github.mycompany.com/org/repo",
			wantBase: "https://github.mycompany.com/org/repo",
		},
		{
			name:     "custom Git server",
			rawURL:   "https://git.internal.example.com/team/project",
			wantBase: "https://git.internal.example.com/team/project",
		},
		{
			name:     "custom server with .git",
			rawURL:   "https://git.internal.example.com/team/project.git",
			wantBase: "https://git.internal.example.com/team/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if ref != "" {
				t.Errorf("ref = %q, want empty", ref)
			}
			if path != "" {
				t.Errorf("path = %q, want empty", path)
			}
		})
	}
}

func TestParseSmartURL_CleanURL(t *testing.T) {
	// Verify cleanURL trims whitespace and leading backslashes
	tests := []struct {
		name     string
		rawURL   string
		wantBase string
	}{
		{name: "leading spaces", rawURL: "  https://github.com/owner/repo", wantBase: "https://github.com/owner/repo"},
		{name: "trailing spaces", rawURL: "https://github.com/owner/repo  ", wantBase: "https://github.com/owner/repo"},
		{name: "leading backslash", rawURL: "\\https://github.com/owner/repo", wantBase: "https://github.com/owner/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, _, _ := ParseSmartURL(tt.rawURL)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
		})
	}
}
