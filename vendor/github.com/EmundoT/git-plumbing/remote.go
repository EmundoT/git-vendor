package git

import (
	"context"
	"fmt"
	"strings"
)

// CloneOpts configures a clone operation.
type CloneOpts struct {
	Filter     string // e.g., "blob:none" for treeless clone
	NoCheckout bool
	Depth      int
}

// Init initializes a new git repository.
func (g *Git) Init(ctx context.Context) error {
	return g.RunSilent(ctx, "init")
}

// AddRemote adds a named remote.
func (g *Git) AddRemote(ctx context.Context, name, url string) error {
	return g.RunSilent(ctx, "remote", "add", name, url)
}

// Clone clones a repository into this directory.
func (g *Git) Clone(ctx context.Context, url string, opts *CloneOpts) error {
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
	return g.RunSilent(ctx, args...)
}

// Fetch fetches from a remote with optional depth.
func (g *Git) Fetch(ctx context.Context, remote, ref string, depth int) error {
	args := []string{"fetch"}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", depth))
	}
	args = append(args, remote, ref)
	return g.RunSilent(ctx, args...)
}

// FetchAll fetches all refs from a remote.
func (g *Git) FetchAll(ctx context.Context, remote string) error {
	return g.RunSilent(ctx, "fetch", remote)
}

// Checkout checks out a ref (branch, tag, or commit hash).
func (g *Git) Checkout(ctx context.Context, ref string) error {
	return g.RunSilent(ctx, "checkout", ref)
}

// LsRemote queries a remote for the commit hash of a ref without cloning.
// LsRemote handles branches (refs/heads/*) and tags (refs/tags/*^{} preferred
// over lightweight refs/tags/*). Returns the resolved commit SHA.
func (g *Git) LsRemote(ctx context.Context, url, ref string) (string, error) {
	out, err := g.Run(ctx, "ls-remote", url, ref)
	if err != nil {
		return "", fmt.Errorf("ls-remote %s %s: %w", url, ref, err)
	}
	return ParseLsRemoteOutput(out, ref)
}

// ParseLsRemoteOutput extracts the commit hash from git ls-remote output.
// When multiple lines match (e.g., annotated tags), ParseLsRemoteOutput prefers
// the dereferenced entry (^{}) which points to the underlying commit.
func ParseLsRemoteOutput(output, ref string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("no matching ref %q in ls-remote output", ref)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var bestHash string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		refName := parts[1]

		// Prefer ^{} (dereferenced annotated tag → commit)
		if strings.HasSuffix(refName, "^{}") {
			return hash, nil
		}
		if bestHash == "" {
			bestHash = hash
		}
	}

	if bestHash == "" {
		return "", fmt.Errorf("no matching ref %q in ls-remote output", ref)
	}
	return bestHash, nil
}
