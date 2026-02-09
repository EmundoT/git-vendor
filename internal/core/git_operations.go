package core

import (
	"context"
	"regexp"
	"strings"

	git "github.com/emundoT/git-plumbing"

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

// gitFor creates a git-plumbing Git instance for the given directory.
// Cheap allocation (single struct, no I/O) — required because git-vendor
// passes dir per-call while git-plumbing stores it on the struct.
func (g *SystemGitClient) gitFor(dir string) *git.Git {
	return &git.Git{Dir: dir, Verbose: g.verbose}
}

// Init initializes a git repository
func (g *SystemGitClient) Init(ctx context.Context, dir string) error {
	return g.gitFor(dir).Init(ctx)
}

// AddRemote adds a git remote
func (g *SystemGitClient) AddRemote(ctx context.Context, dir, name, url string) error {
	return g.gitFor(dir).AddRemote(ctx, name, url)
}

// Fetch fetches from remote with optional depth
func (g *SystemGitClient) Fetch(ctx context.Context, dir string, depth int, ref string) error {
	return g.gitFor(dir).Fetch(ctx, "origin", ref, depth)
}

// FetchAll fetches all refs from origin
func (g *SystemGitClient) FetchAll(ctx context.Context, dir string) error {
	return g.gitFor(dir).FetchAll(ctx, "origin")
}

// Checkout checks out a git ref
func (g *SystemGitClient) Checkout(ctx context.Context, dir, ref string) error {
	return g.gitFor(dir).Checkout(ctx, ref)
}

// GetHeadHash returns the current HEAD commit hash
func (g *SystemGitClient) GetHeadHash(ctx context.Context, dir string) (string, error) {
	return g.gitFor(dir).HEAD(ctx)
}

// Clone clones a repository with options.
// Converts types.CloneOptions to git.CloneOpts for the git-plumbing layer.
func (g *SystemGitClient) Clone(ctx context.Context, dir, url string, opts *types.CloneOptions) error {
	var plumbingOpts *git.CloneOpts
	if opts != nil {
		plumbingOpts = &git.CloneOpts{
			Filter:     opts.Filter,
			NoCheckout: opts.NoCheckout,
			Depth:      opts.Depth,
		}
	}
	return g.gitFor(dir).Clone(ctx, url, plumbingOpts)
}

// ListTree lists files/directories at a given ref and subdir.
// The caller should set a deadline on ctx if a timeout is desired.
// Delegates to git-plumbing which handles ls-tree parsing internally.
func (g *SystemGitClient) ListTree(ctx context.Context, dir, ref, subdir string) ([]string, error) {
	return g.gitFor(dir).ListTree(ctx, ref, subdir)
}

// GetCommitLog retrieves commit history between two commits.
// Delegates to git-plumbing Log() and converts git.Commit to types.CommitInfo.
func (g *SystemGitClient) GetCommitLog(ctx context.Context, dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error) {
	opts := git.LogOpts{
		Range:    oldHash + ".." + newHash,
		MaxCount: maxCount,
	}

	plumbingCommits, err := g.gitFor(dir).Log(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(plumbingCommits) == 0 {
		return nil, nil
	}

	commits := make([]types.CommitInfo, 0, len(plumbingCommits))
	for _, c := range plumbingCommits {
		commits = append(commits, types.CommitInfo{
			Hash:      c.Hash,
			ShortHash: c.Short,
			Subject:   c.Subject,
			Author:    c.Author,
			Date:      c.Date.Format("2006-01-02 15:04:05 -0700"),
		})
	}

	return commits, nil
}

// GetTagForCommit returns a git tag that points to the given commit hash, if any.
// Prefers semver-looking tags (v1.0.0, 1.0.0) over other tags.
// Delegates to git-plumbing TagsAt() for tag retrieval, applies semver preference locally.
func (g *SystemGitClient) GetTagForCommit(ctx context.Context, dir, commitHash string) (string, error) {
	tags, err := g.gitFor(dir).TagsAt(ctx, commitHash)
	if err != nil || len(tags) == 0 {
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
// Uses git-plumbing with empty Dir to match original behavior (process working directory).
func GetGitUserIdentity() string {
	g := &git.Git{}
	return g.UserIdentity(context.Background())
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
