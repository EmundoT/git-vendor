package git

import (
	"context"
	"fmt"
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
