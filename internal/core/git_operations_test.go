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
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

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
	runGitCommand(t, tempDir, "add", "file2.txt")
	runGitCommand(t, tempDir, "commit", "-m", "Second commit")

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create third commit
	file3 := filepath.Join(tempDir, "file3.txt")
	os.WriteFile(file3, []byte("content3"), 0644)
	runGitCommand(t, tempDir, "add", "file3.txt")
	runGitCommand(t, tempDir, "commit", "-m", "Third commit")

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
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

	hash1, err := git.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get first commit hash: %v", err)
	}

	// Create 5 more commits
	for i := 2; i <= 6; i++ {
		time.Sleep(10 * time.Millisecond)
		fileName := filepath.Join(tempDir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(fileName, []byte("content"), 0644)
		runGitCommand(t, tempDir, "add", ".")
		runGitCommand(t, tempDir, "commit", "-m", "Commit "+string(rune('0'+i)))
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
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

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
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

	// Test: Invalid hash should error
	_, err := git.GetCommitLog(context.Background(), tempDir, "invalid-hash", "another-invalid-hash", 0)
	if err == nil {
		t.Error("Expected error for invalid commit hashes, got nil")
	}
}

// ============================================================================
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
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Tagged commit")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Tag with non-semver first, then semver — GetTagForCommit should prefer semver
	runGitCommand(t, tempDir, "tag", "release-2025")
	runGitCommand(t, tempDir, "tag", "v1.2.3")

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
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Non-semver commit")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Only non-semver tags — should fall back to first tag
	runGitCommand(t, tempDir, "tag", "release-2025")

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
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Untagged commit")

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
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Semver no prefix")

	hash, err := gitClient.GetHeadHash(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Semver without 'v' prefix should still be preferred
	runGitCommand(t, tempDir, "tag", "build-42")
	runGitCommand(t, tempDir, "tag", "2.0.0")

	tag, err := gitClient.GetTagForCommit(context.Background(), tempDir, hash)
	if err != nil {
		t.Fatalf("GetTagForCommit failed: %v", err)
	}
	if tag != "2.0.0" {
		t.Errorf("Expected semver tag '2.0.0', got '%s'", tag)
	}
}

// ============================================================================
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
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")
	hash1, _ := gitClient.GetHeadHash(context.Background(), tempDir)

	time.Sleep(10 * time.Millisecond)

	file2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0644)
	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Second commit")
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
// Helper Functions
// ============================================================================

func configureGitUser(t *testing.T, dir string) {
	t.Helper()
	runGitCommand(t, dir, "config", "user.name", "Test User")
	runGitCommand(t, dir, "config", "user.email", "test@example.com")
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	// Disable commit signing for tests to avoid environment-specific failures
	fullArgs := append([]string{"-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
}
