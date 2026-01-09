package core

import (
	"os"
	"os/exec"
	"path/filepath"
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
	if err := git.Init(tempDir); err != nil {
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
	hash1, err := git.GetHeadHash(tempDir)
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
	hash3, err := git.GetHeadHash(tempDir)
	if err != nil {
		t.Fatalf("Failed to get final commit hash: %v", err)
	}

	// Test: Get commit log between first and third commit
	commits, err := git.GetCommitLog(tempDir, hash1, hash3, 0)
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
	if err := git.Init(tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create first commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

	hash1, err := git.GetHeadHash(tempDir)
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

	hash6, err := git.GetHeadHash(tempDir)
	if err != nil {
		t.Fatalf("Failed to get final commit hash: %v", err)
	}

	// Test: Limit to 3 commits
	commits, err := git.GetCommitLog(tempDir, hash1, hash6, 3)
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
	if err := git.Init(tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create single commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

	hash1, err := git.GetHeadHash(tempDir)
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}

	// Test: Get log from hash to itself (no new commits)
	commits, err := git.GetCommitLog(tempDir, hash1, hash1, 0)
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
	if err := git.Init(tempDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	configureGitUser(t, tempDir)

	// Create commit
	file1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	runGitCommand(t, tempDir, "add", "file1.txt")
	runGitCommand(t, tempDir, "commit", "-m", "First commit")

	// Test: Invalid hash should error
	_, err := git.GetCommitLog(tempDir, "invalid-hash", "another-invalid-hash", 0)
	if err == nil {
		t.Error("Expected error for invalid commit hashes, got nil")
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
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
}
