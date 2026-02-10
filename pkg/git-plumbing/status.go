package git

import (
	"context"
	"os"
	"path/filepath"
)

// FileStatus represents a file's status in the working tree.
type FileStatus struct {
	Path    string
	Index   byte // status in index (staged): ' ', M, A, D, R, C, U, ?
	WorkDir byte // status in working directory: ' ', M, D, ?, !
}

// RepoStatus represents the full status of a git repository.
type RepoStatus struct {
	Clean      bool
	Staged     []FileStatus
	Unstaged   []FileStatus
	Untracked  []string
	InProgress *InProgressOp // nil if no operation in progress
}

// InProgressOp indicates a git operation that is currently in progress.
type InProgressOp struct {
	Type string // "rebase", "merge", "cherry-pick", "revert", "bisect"
}

// Status returns the full working tree status.
func (g *Git) Status(ctx context.Context) (*RepoStatus, error) {
	lines, err := g.RunLines(ctx, "status", "--porcelain=v1")
	if err != nil {
		return nil, err
	}
	status := &RepoStatus{Clean: len(lines) == 0}
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		fs := FileStatus{
			Index:   line[0],
			WorkDir: line[1],
			Path:    line[3:],
		}
		switch {
		case fs.Index == '?' && fs.WorkDir == '?':
			status.Untracked = append(status.Untracked, fs.Path)
		case fs.Index != ' ' && fs.Index != '?':
			status.Staged = append(status.Staged, fs)
		case fs.WorkDir != ' ' && fs.WorkDir != '?':
			status.Unstaged = append(status.Unstaged, fs)
		}
	}
	status.InProgress = g.detectInProgressOp()
	return status, nil
}

// IsClean returns true if the working tree has no changes.
func (g *Git) IsClean(ctx context.Context) (bool, error) {
	s, err := g.Status(ctx)
	if err != nil {
		return false, err
	}
	return s.Clean, nil
}

// detectInProgressOp checks for git operations currently in progress
// by looking for marker files/directories in the .git directory.
func (g *Git) detectInProgressOp() *InProgressOp {
	gitDir := filepath.Join(g.Dir, ".git")

	// rebase-merge is always an interactive/standard rebase
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return &InProgressOp{Type: "rebase"}
	}
	// rebase-apply can be either "git am" or "git rebase"
	// If rebase-apply/applying exists, it's "am"; otherwise it's "rebase"
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply", "applying")); err == nil {
			return &InProgressOp{Type: "am"}
		}
		return &InProgressOp{Type: "rebase"}
	}

	checks := []struct {
		path string
		typ  string
	}{
		{"MERGE_HEAD", "merge"},
		{"CHERRY_PICK_HEAD", "cherry-pick"},
		{"REVERT_HEAD", "revert"},
		{"BISECT_LOG", "bisect"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(gitDir, c.path)); err == nil {
			return &InProgressOp{Type: c.typ}
		}
	}
	return nil
}
