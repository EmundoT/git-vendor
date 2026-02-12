package core

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	git "github.com/EmundoT/git-plumbing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// Package-level compiled regex for semver matching
var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// DiffMetrics holds aggregate diff statistics from staged changes.
// DiffMetrics is returned by GitClient.DiffCachedStat and used to compute
// shared COMMIT-SCHEMA trailers (Diff-Additions, Diff-Deletions, Diff-Files).
type DiffMetrics struct {
	Added     int
	Removed   int
	FileCount int
}

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
	Add(ctx context.Context, dir string, paths ...string) error
	Commit(ctx context.Context, dir string, opts types.CommitOptions) error
	AddNote(ctx context.Context, dir, noteRef, commitHash, content string) error
	GetNote(ctx context.Context, dir, noteRef, commitHash string) (string, error)
	DiffCachedNames(ctx context.Context, dir string) ([]string, error)
	DiffCachedStat(ctx context.Context, dir string) (DiffMetrics, error)
	ConfigSet(ctx context.Context, dir, key, value string) error
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

// Add stages files for the next commit.
// Add delegates to git-plumbing's Add method with the specified paths.
func (g *SystemGitClient) Add(ctx context.Context, dir string, paths ...string) error {
	return g.gitFor(dir).Add(ctx, paths...)
}

// Commit creates a new commit with structured trailers.
// Commit converts []types.Trailer to []git.Trailer for the git-plumbing layer.
// Both types have identical Key/Value fields but are distinct Go types across packages.
func (g *SystemGitClient) Commit(ctx context.Context, dir string, opts types.CommitOptions) error {
	plumbingTrailers := make([]git.Trailer, len(opts.Trailers))
	for i, t := range opts.Trailers {
		plumbingTrailers[i] = git.Trailer{Key: t.Key, Value: t.Value}
	}
	return g.gitFor(dir).Commit(ctx, git.CommitOpts{
		Message:  opts.Message,
		Trailers: plumbingTrailers,
	})
}

// AddNote adds or overwrites a git note on a commit under the given note ref namespace.
// AddNote delegates to git-plumbing's AddNote with NoteRef type conversion.
func (g *SystemGitClient) AddNote(ctx context.Context, dir, noteRef, commitHash, content string) error {
	return g.gitFor(dir).AddNote(ctx, git.NoteRef(noteRef), commitHash, content)
}

// GetNote retrieves a git note from a commit under the given note ref namespace.
// GetNote delegates to git-plumbing's GetNote with NoteRef type conversion.
func (g *SystemGitClient) GetNote(ctx context.Context, dir, noteRef, commitHash string) (string, error) {
	return g.gitFor(dir).GetNote(ctx, git.NoteRef(noteRef), commitHash)
}

// DiffCachedNames returns file paths with staged (cached) changes.
// DiffCachedNames delegates to git-plumbing's DiffCachedNames for the given directory.
func (g *SystemGitClient) DiffCachedNames(ctx context.Context, dir string) ([]string, error) {
	return g.gitFor(dir).DiffCachedNames(ctx)
}

// DiffCachedStat returns aggregate line-change statistics for staged changes.
// DiffCachedStat converts git-plumbing's DiffStat into DiffMetrics (added, removed, file count).
func (g *SystemGitClient) DiffCachedStat(ctx context.Context, dir string) (DiffMetrics, error) {
	stat, err := g.gitFor(dir).DiffCachedStat(ctx)
	if err != nil {
		return DiffMetrics{}, err
	}
	return DiffMetrics{
		Added:     stat.Total.Added,
		Removed:   stat.Total.Removed,
		FileCount: len(stat.Files),
	}, nil
}

// ConfigSet writes a git config key-value pair.
// ConfigSet delegates to git-plumbing's ConfigSet for the given directory.
func (g *SystemGitClient) ConfigSet(ctx context.Context, dir, key, value string) error {
	return g.gitFor(dir).ConfigSet(ctx, key, value)
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

// allowedURLSchemes lists URL schemes safe for git clone operations.
var allowedURLSchemes = []string{
	"https", "http", "ssh", "git", "git+ssh",
}

// ValidateVendorURL checks that a repository URL uses a safe scheme.
// Rejects file://, ftp://, and other non-git schemes that could access the
// local filesystem or use insecure protocols.
//
// SEC-011: Accepted schemes: https, http, ssh, git, git+ssh, and SCP-style
// (git@host:owner/repo). Bare hostnames without a scheme are also accepted
// for compatibility with custom git server configurations.
func ValidateVendorURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("vendor URL must not be empty")
	}

	lower := strings.ToLower(rawURL)

	// SCP-style SSH URLs (git@host:owner/repo) — allowed
	if strings.Contains(rawURL, "@") && !strings.Contains(rawURL, "://") {
		return nil
	}

	// Reject non-hierarchical schemes (javascript:, data:, vbscript:) that use ":"
	// but not "://". These can never be valid git URLs.
	if idx := strings.Index(lower, ":"); idx > 0 && !strings.Contains(rawURL, "://") {
		prefix := lower[:idx]
		switch prefix {
		case "javascript", "data", "vbscript":
			return fmt.Errorf("URL scheme %q: is not allowed: not a valid git URL", prefix)
		}
	}

	// Check if URL has a scheme (contains "://")
	if !strings.Contains(rawURL, "://") {
		// No scheme — bare hostname or relative path; allow for compat
		return nil
	}

	// Parse to extract the scheme
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	for _, allowed := range allowedURLSchemes {
		if scheme == allowed {
			return nil
		}
	}

	// Provide specific guidance for known dangerous schemes
	switch scheme {
	case "file":
		return fmt.Errorf("URL scheme \"file://\" is not allowed: local filesystem access via vendor URLs is a security risk")
	case "ftp", "ftps":
		return fmt.Errorf("URL scheme %q is not allowed: FTP is insecure and not supported for git operations", lower[:strings.Index(lower, "://")+3])
	default:
		return fmt.Errorf("URL scheme %q is not allowed: use https://, ssh://, or git:// instead", scheme)
	}
}

// SanitizeURL removes embedded credentials from a URL for safe logging.
// Strips userinfo (user:password@) from URLs with a scheme.
// SCP-style URLs (git@host:path) are returned unchanged because "git" is the
// username, not a secret.
func SanitizeURL(rawURL string) string {
	// SCP-style SSH (git@host:owner/repo) — no secret, return as-is
	if !strings.Contains(rawURL, "://") {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.User != nil {
		parsed.User = nil
		return parsed.String()
	}
	return rawURL
}
