package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// GitClient handles git command operations
type GitClient interface {
	Init(dir string) error
	AddRemote(dir, name, url string) error
	Fetch(dir string, depth int, ref string) error
	FetchAll(dir string) error
	Checkout(dir, ref string) error
	GetHeadHash(dir string) (string, error)
	Clone(dir, url string, opts *types.CloneOptions) error
	ListTree(dir, ref, subdir string) ([]string, error)
	GetCommitLog(dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error)
	GetTagForCommit(dir, commitHash string) (string, error)
}

// SystemGitClient implements GitClient using system git commands
type SystemGitClient struct {
	verbose bool
}

// NewSystemGitClient creates a new SystemGitClient
func NewSystemGitClient(verbose bool) *SystemGitClient {
	return &SystemGitClient{verbose: verbose}
}

// Init initializes a git repository
func (g *SystemGitClient) Init(dir string) error {
	return g.run(dir, "init")
}

// AddRemote adds a git remote
func (g *SystemGitClient) AddRemote(dir, name, url string) error {
	return g.run(dir, "remote", "add", name, url)
}

// Fetch fetches from remote with optional depth
func (g *SystemGitClient) Fetch(dir string, depth int, ref string) error {
	args := []string{"fetch"}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", depth))
	}
	args = append(args, "origin", ref)
	return g.run(dir, args...)
}

// FetchAll fetches all refs from origin
func (g *SystemGitClient) FetchAll(dir string) error {
	return g.run(dir, "fetch", "origin")
}

// Checkout checks out a git ref
func (g *SystemGitClient) Checkout(dir, ref string) error {
	return g.run(dir, "checkout", ref)
}

// GetHeadHash returns the current HEAD commit hash
func (g *SystemGitClient) GetHeadHash(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Clone clones a repository with options
func (g *SystemGitClient) Clone(dir, url string, opts *types.CloneOptions) error {
	args := []string{"clone"}

	if opts != nil {
		if opts.Filter != "" {
			args = append(args, "--filter="+opts.Filter)
		}
		if opts.NoCheckout {
			args = append(args, "--no-checkout")
		}
		if opts.Depth > 0 {
			args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
		}
	}

	args = append(args, url, ".")
	return g.run(dir, args...)
}

// ListTree lists files/directories at a given ref and subdir
func (g *SystemGitClient) ListTree(dir, ref, subdir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	target := ref
	if target == "" {
		target = "HEAD"
	}

	cmd := exec.CommandContext(ctx, "git", "ls-tree", target)
	if subdir != "" && subdir != "." {
		cleanSub := strings.TrimSuffix(subdir, "/")
		cmd.Args = append(cmd.Args, cleanSub+"/")
	}
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil && subdir != "" {
		// Try without trailing slash
		cmd = exec.CommandContext(ctx, "git", "ls-tree", target, strings.TrimSuffix(subdir, "/"))
		cmd.Dir = dir
		out, err = cmd.Output()
	}
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var items []string

	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) < 4 {
			continue
		}

		objType := parts[1]
		fullPath := strings.Join(parts[3:], " ")

		relName := fullPath
		if subdir != "" && subdir != "." {
			cleanSub := strings.TrimSuffix(subdir, "/") + "/"
			if !strings.HasPrefix(fullPath, cleanSub) {
				continue
			}
			relName = strings.TrimPrefix(fullPath, cleanSub)
		}
		if relName == "" {
			continue
		}

		if objType == "tree" {
			items = append(items, relName+"/")
		} else {
			items = append(items, relName)
		}
	}

	sort.Strings(items)
	return items, nil
}

// GetCommitLog retrieves commit history between two commits
func (g *SystemGitClient) GetCommitLog(dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error) {
	// Format: hash|shortHash|subject|author|date
	formatString := "--pretty=format:%H|%h|%s|%an|%ai"

	args := []string{"log", formatString}
	if maxCount > 0 {
		args = append(args, fmt.Sprintf("-%d", maxCount))
	}

	// Use range syntax: oldHash..newHash shows commits in newHash not in oldHash
	rangeSpec := fmt.Sprintf("%s..%s", oldHash, newHash)
	args = append(args, rangeSpec)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var commits []types.CommitInfo

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			continue
		}

		commits = append(commits, types.CommitInfo{
			Hash:      parts[0],
			ShortHash: parts[1],
			Subject:   parts[2],
			Author:    parts[3],
			Date:      parts[4],
		})
	}

	return commits, nil
}

// GetTagForCommit returns a git tag that points to the given commit hash, if any.
// Prefers semver-looking tags (v1.0.0, 1.0.0) over other tags.
func (g *SystemGitClient) GetTagForCommit(dir, commitHash string) (string, error) {
	// Get tags pointing to this exact commit
	cmd := exec.Command("git", "tag", "--points-at", commitHash)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		// No tags found, not an error
		return "", nil
	}

	tagsOutput := strings.TrimSpace(string(out))
	if tagsOutput == "" {
		return "", nil
	}

	tags := strings.Split(tagsOutput, "\n")
	if len(tags) == 0 || tags[0] == "" {
		return "", nil
	}

	// Prefer semver-looking tags (v1.0.0, 1.0.0)
	for _, tag := range tags {
		if isSemverTag(tag) {
			return tag, nil
		}
	}

	// Fall back to first tag
	return tags[0], nil
}

// isSemverTag checks if a tag looks like a semantic version
func isSemverTag(tag string) bool {
	tag = strings.TrimPrefix(tag, "v")
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+`, tag)
	return matched
}

// GetGitUserIdentity returns the git user identity in "Name <email>" format.
// Returns empty string if not configured.
func GetGitUserIdentity() string {
	nameCmd := exec.Command("git", "config", "user.name")
	nameOut, err := nameCmd.Output()
	if err != nil {
		return ""
	}

	emailCmd := exec.Command("git", "config", "user.email")
	emailOut, err := emailCmd.Output()
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(nameOut))
	email := strings.TrimSpace(string(emailOut))

	if name == "" && email == "" {
		return ""
	}

	if name != "" && email != "" {
		return fmt.Sprintf("%s <%s>", name, email)
	}

	if name != "" {
		return name
	}

	return email
}

// run executes a git command
func (g *SystemGitClient) run(dir string, args ...string) error {
	if g.verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] git %s (in %s)\n", strings.Join(args, " "), dir)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", string(output))
	}

	return nil
}

// ParseSmartURL extracts repository, ref, and path from GitHub URLs
func ParseSmartURL(rawURL string) (baseURL, ref, path string) {
	rawURL = cleanURL(rawURL)
	reDeep := regexp.MustCompile(`(github\.com/[^/]+/[^/]+)/(blob|tree)/([^/]+)/(.+)`)
	matches := reDeep.FindStringSubmatch(rawURL)

	if len(matches) == 5 {
		return "https://" + matches[1], matches[3], matches[4]
	}

	base := strings.TrimSuffix(rawURL, "/")
	base = strings.TrimSuffix(base, ".git")
	return base, "", ""
}

// cleanURL trims whitespace and backslashes
func cleanURL(raw string) string {
	return strings.TrimLeft(strings.TrimSpace(raw), "\\")
}
