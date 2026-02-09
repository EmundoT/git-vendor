package git

import (
	"context"
	"errors"
)

// HEAD returns the full SHA of the current HEAD commit.
func (g *Git) HEAD(ctx context.Context) (string, error) {
	return g.Run(ctx, "rev-parse", "HEAD")
}

// CurrentBranch returns the short name of the current branch.
// Returns ErrDetachedHead if HEAD is not on a branch.
func (g *Git) CurrentBranch(ctx context.Context) (string, error) {
	out, err := g.Run(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", ErrDetachedHead
	}
	return out, nil
}

// IsDetached returns true if HEAD is in detached state.
func (g *Git) IsDetached(ctx context.Context) (bool, error) {
	_, err := g.CurrentBranch(ctx)
	if errors.Is(err, ErrDetachedHead) {
		return true, nil
	}
	return false, err
}

// ResolveRef resolves a ref name to its full SHA.
func (g *Git) ResolveRef(ctx context.Context, ref string) (string, error) {
	out, err := g.Run(ctx, "rev-parse", ref)
	if err != nil {
		return "", ErrRefNotFound
	}
	return out, nil
}
