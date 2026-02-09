package git

import (
	"errors"
	"strings"
)

// Sentinel errors for common git failure modes.
var (
	ErrNotRepo      = errors.New("not a git repository")
	ErrDirtyTree    = errors.New("working tree has uncommitted changes")
	ErrDetachedHead = errors.New("HEAD is detached")
	ErrRefNotFound  = errors.New("ref not found")
	ErrConflict     = errors.New("merge conflict in progress")
)

// GitError wraps an exec error with the command that was run and stderr output.
type GitError struct {
	Args   []string // git subcommand and arguments
	Stderr string   // stderr output from git
	Err    error    // underlying exec error
}

func (e *GitError) Error() string {
	s := strings.TrimSpace(e.Stderr)
	if s != "" {
		return s
	}
	return e.Err.Error()
}

func (e *GitError) Unwrap() error {
	return e.Err
}

// IsNotRepo reports whether err indicates the directory is not a git repository.
func IsNotRepo(err error) bool {
	var gitErr *GitError
	if errors.As(err, &gitErr) {
		return strings.Contains(gitErr.Stderr, "not a git repository")
	}
	return false
}
