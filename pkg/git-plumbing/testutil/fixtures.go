package testutil

import (
	"fmt"
	"os/exec"
	"testing"
)

// LinearHistory creates a repo with n sequential commits on the default branch.
func LinearHistory(t *testing.T, n int) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	for i := 1; i <= n; i++ {
		repo.Commit(
			fmt.Sprintf("commit %d", i),
			map[string]string{
				fmt.Sprintf("file%d.txt", i): fmt.Sprintf("content %d", i),
			},
		)
	}
	return repo
}

// DiamondMerge creates a repo with a feature branch merged back to the default branch.
func DiamondMerge(t *testing.T) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	repo.Commit("initial", map[string]string{"README.md": "init"})
	mainBranch := repo.CurrentBranch()
	repo.Branch("feature")
	repo.Commit("feature work", map[string]string{"feature.txt": "work"})
	repo.Checkout(mainBranch)
	repo.Merge("feature")
	return repo
}

// DirtyWorkingTree creates a repo with uncommitted changes.
func DirtyWorkingTree(t *testing.T) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	repo.Commit("initial", map[string]string{"file.txt": "original"})
	writeFile(t, repo.Dir, "file.txt", "modified")
	writeFile(t, repo.Dir, "untracked.txt", "new file")
	return repo
}

// DetachedHead creates a repo with HEAD detached at the first commit.
func DetachedHead(t *testing.T) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	sha := repo.Commit("first", map[string]string{"a.txt": "a"})
	repo.Commit("second", map[string]string{"b.txt": "b"})
	repo.Checkout(sha)
	return repo
}

// WithMergeConflict creates a repo with a merge conflict in progress.
func WithMergeConflict(t *testing.T) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	repo.Commit("initial", map[string]string{"conflict.txt": "base"})
	mainBranch := repo.CurrentBranch()
	repo.Branch("conflict-branch")
	repo.Commit("branch change", map[string]string{"conflict.txt": "branch version"})
	repo.Checkout(mainBranch)
	repo.Commit("main change", map[string]string{"conflict.txt": "main version"})
	// Attempt the merge — it will fail with a conflict, which is what we want
	cmd := exec.Command("git", "merge", "conflict-branch")
	cmd.Dir = repo.Dir
	_ = cmd.Run() // ignore error; conflict is expected
	return repo
}
