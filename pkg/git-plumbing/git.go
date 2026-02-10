package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Git represents a git repository at a specific directory.
type Git struct {
	Dir     string // working directory
	Verbose bool   // log commands to stderr
}

// New creates a Git instance for the given directory.
func New(dir string) *Git {
	return &Git{Dir: dir}
}

// Run executes a git command and returns trimmed stdout.
func (g *Git) Run(ctx context.Context, args ...string) (string, error) {
	if g.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] git %s (in %s)\n", strings.Join(args, " "), g.Dir)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	cmd.Env = sanitizedEnv()
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Args:   args,
				Stderr: string(exitErr.Stderr),
				Err:    err,
			}
		}
		return "", err
	}
	return strings.TrimRight(string(out), " \t\r\n"), nil
}

// RunLines executes a git command and returns stdout split by newlines.
func (g *Git) RunLines(ctx context.Context, args ...string) ([]string, error) {
	out, err := g.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// RunSilent executes a git command, discarding output on success.
// On error, includes combined stdout+stderr in the error message.
func (g *Git) RunSilent(ctx context.Context, args ...string) error {
	if g.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] git %s (in %s)\n", strings.Join(args, " "), g.Dir)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	cmd.Env = sanitizedEnv()
	if output, err := cmd.CombinedOutput(); err != nil {
		return &GitError{
			Args:   args,
			Stderr: string(output),
			Err:    err,
		}
	}
	return nil
}

// IsInstalled returns true if the git binary is available on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// sanitizedEnv returns the current environment with git hook variables removed.
// When git-plumbing runs inside a git hook (pre-commit, post-merge, etc.),
// GIT_DIR and GIT_INDEX_FILE point at the outer repo and override cmd.Dir,
// causing commands to target the wrong repository.
func sanitizedEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		switch strings.ToUpper(key) {
		case "GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES":
			continue
		}
		env = append(env, e)
	}
	return env
}
