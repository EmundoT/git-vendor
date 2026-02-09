package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo is a temporary git repository for testing.
type TestRepo struct {
	Dir string
	t   *testing.T
}

// NewTestRepo creates an initialized git repository in t.TempDir().
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "config", "user.email", "test@example.com")
	run(t, dir, "config", "user.name", "Test User")
	return &TestRepo{Dir: dir, t: t}
}

// Commit creates a commit with the given files and returns the commit SHA.
func (r *TestRepo) Commit(msg string, files map[string]string) string {
	r.t.Helper()
	for path, content := range files {
		writeFile(r.t, r.Dir, path, content)
	}
	run(r.t, r.Dir, "add", ".")
	run(r.t, r.Dir, "commit", "-m", msg)
	return strings.TrimSpace(run(r.t, r.Dir, "rev-parse", "HEAD"))
}

// Branch creates and checks out a new branch.
func (r *TestRepo) Branch(name string) {
	r.t.Helper()
	run(r.t, r.Dir, "checkout", "-b", name)
}

// Checkout switches to an existing branch or ref.
func (r *TestRepo) Checkout(ref string) {
	r.t.Helper()
	run(r.t, r.Dir, "checkout", ref)
}

// Tag creates a lightweight tag at HEAD.
func (r *TestRepo) Tag(name string) {
	r.t.Helper()
	run(r.t, r.Dir, "tag", name)
}

// Merge merges a branch into the current branch.
func (r *TestRepo) Merge(branch string) {
	r.t.Helper()
	run(r.t, r.Dir, "merge", "--no-ff", "-m", "Merge "+branch, branch)
}

// CurrentBranch returns the name of the current branch.
func (r *TestRepo) CurrentBranch() string {
	r.t.Helper()
	return strings.TrimSpace(run(r.t, r.Dir, "rev-parse", "--abbrev-ref", "HEAD"))
}

// WriteFile creates a file in the repo directory. Exported for test use.
func (r *TestRepo) WriteFile(name, content string) {
	r.t.Helper()
	writeFile(r.t, r.Dir, name, content)
}

// StageFile stages a specific file via git add.
func (r *TestRepo) StageFile(path string) {
	r.t.Helper()
	run(r.t, r.Dir, "add", path)
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// Disable commit signing to avoid environment-specific failures
	// (ported from git-vendor's runGitCommand pattern)
	fullArgs := append([]string{"-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	cmd.Env = sanitizedEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// sanitizedEnv returns os.Environ() with git hook variables removed so that
// test repos are isolated from any outer git hook context (e.g., pre-commit).
// Strips both repo-targeting vars (GIT_DIR) and author/committer overrides
// (GIT_AUTHOR_NAME, etc.) that git sets during commit hooks.
func sanitizedEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(e, "=", 2)[0])
		if strings.HasPrefix(key, "GIT_AUTHOR_") ||
			strings.HasPrefix(key, "GIT_COMMITTER_") {
			continue
		}
		switch key {
		case "GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES":
			continue
		}
		env = append(env, e)
	}
	return env
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
