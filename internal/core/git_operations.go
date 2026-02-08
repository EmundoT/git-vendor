package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// Package-level compiled regex for semver matching
var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// GitClient handles git command operations
type GitClient interface {
	Init(ctx context.Context, dir string) error
	AddRemote(ctx context.Context, dir, name, url string) error
	Fetch(ctx context.Context, dir string, depth int, ref string) error
	FetchAll(ctx context.Context, dir string) error
	Checkout(ctx context.Context, dir, ref string) error
	GetHeadHash(ctx context.Context, dir string) (string, error)
	Clone(ctx context.Context, dir, url string, opts *types.CloneOptions) error
	ListTree(ctx context.Context, dir, ref, subdir string) ([]string, error)
	GetCommitLog(ctx context.Context, dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error)
	GetTagForCommit(ctx context.Context, dir, commitHash string) (string, error)
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
func (g *SystemGitClient) Init(ctx context.Context, dir string) error {
	return g.run(ctx, dir, "init")
}

// AddRemote adds a git remote
func (g *SystemGitClient) AddRemote(ctx context.Context, dir, name, url string) error {
	return g.run(ctx, dir, "remote", "add", name, url)
}

// Fetch fetches from remote with optional depth
func (g *SystemGitClient) Fetch(ctx context.Context, dir string, depth int, ref string) error {
	args := []string{"fetch"}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", depth))
	}
	args = append(args, "origin", ref)
	return g.run(ctx, dir, args...)
}

// FetchAll fetches all refs from origin
func (g *SystemGitClient) FetchAll(ctx context.Context, dir string) error {
	return g.run(ctx, dir, "fetch", "origin")
}

// Checkout checks out a git ref
func (g *SystemGitClient) Checkout(ctx context.Context, dir, ref string) error {
	return g.run(ctx, dir, "checkout", ref)
}

// GetHeadHash returns the current HEAD commit hash
func (g *SystemGitClient) GetHeadHash(ctx context.Context, dir string) (string, error) {
	return g.runOutput(ctx, dir, "rev-parse", "HEAD")
}

// Clone clones a repository with options
func (g *SystemGitClient) Clone(ctx context.Context, dir, url string, opts *types.CloneOptions) error {
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
	return g.run(ctx, dir, args...)
}

// ListTree lists files/directories at a given ref and subdir.
// The caller should set a deadline on ctx if a timeout is desired.
func (g *SystemGitClient) ListTree(ctx context.Context, dir, ref, subdir string) ([]string, error) {
	target := ref
	if target == "" {
		target = "HEAD"
	}

	args := []string{"ls-tree", target}
	if subdir != "" && subdir != "." {
		cleanSub := strings.TrimSuffix(subdir, "/")
		args = append(args, cleanSub+"/")
	}

	output, err := g.runOutput(ctx, dir, args...)
	if err != nil && subdir != "" {
		// Try without trailing slash
		args = []string{"ls-tree", target, strings.TrimSuffix(subdir, "/")}
		output, err = g.runOutput(ctx, dir, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %w", err)
	}

	return parseListTreeOutput(output, subdir), nil
}

// parseListTreeOutput parses git ls-tree output into a sorted list of entries
func parseListTreeOutput(output, subdir string) []string {
	lines := strings.Split(output, "\n")
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
	return items
}

// GetCommitLog retrieves commit history between two commits.
// Uses null-byte delimiter (%x00) for safe parsing of commit subjects
// that may contain pipe characters.
func (g *SystemGitClient) GetCommitLog(ctx context.Context, dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error) {
	// Format: hash\x00shortHash\x00subject\x00author\x00date
	formatString := "--pretty=format:%H%x00%h%x00%s%x00%an%x00%ai"

	args := []string{"log", formatString}
	if maxCount > 0 {
		args = append(args, fmt.Sprintf("-%d", maxCount))
	}

	// Use range syntax: oldHash..newHash shows commits in newHash not in oldHash
	rangeSpec := fmt.Sprintf("%s..%s", oldHash, newHash)
	args = append(args, rangeSpec)

	output, err := g.runOutput(ctx, dir, args...)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	var commits []types.CommitInfo

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
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
func (g *SystemGitClient) GetTagForCommit(ctx context.Context, dir, commitHash string) (string, error) {
	output, err := g.runOutput(ctx, dir, "tag", "--points-at", commitHash)
	if err != nil {
		// No tags found, not an error
		return "", nil
	}

	if output == "" {
		return "", nil
	}

	tags := strings.Split(output, "\n")
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
	return semverRegex.MatchString(tag)
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

// run executes a git command, discarding output on success.
// Uses CombinedOutput to include stderr in error messages.
func (g *SystemGitClient) run(ctx context.Context, dir string, args ...string) error {
	if g.verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] git %s (in %s)\n", strings.Join(args, " "), dir)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", string(output))
	}

	return nil
}

// runOutput executes a git command and returns trimmed stdout.
// Uses Output (not CombinedOutput) so stderr is available in ExitError.
func (g *SystemGitClient) runOutput(ctx context.Context, dir string, args ...string) (string, error) {
	if g.verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] git %s (in %s)\n", strings.Join(args, " "), dir)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
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
