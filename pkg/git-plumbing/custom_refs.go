package git

import (
	"context"
	"strings"
)

// RefInfo represents a reference name and its target hash.
type RefInfo struct {
	Name string // full ref name (e.g., "refs/agents/claude/session")
	Hash string // SHA the ref points to
}

// UpdateRef creates or updates a ref to point at the given target.
func (g *Git) UpdateRef(ctx context.Context, refName, target string) error {
	return g.RunSilent(ctx, "update-ref", refName, target)
}

// DeleteRef removes a ref.
func (g *Git) DeleteRef(ctx context.Context, refName string) error {
	return g.RunSilent(ctx, "update-ref", "-d", refName)
}

// ShowRef returns the hash that refName points to.
// Returns ErrRefNotFound if the ref does not exist.
func (g *Git) ShowRef(ctx context.Context, refName string) (string, error) {
	out, err := g.Run(ctx, "rev-parse", "--verify", refName)
	if err != nil {
		return "", ErrRefNotFound
	}
	return out, nil
}

// ForEachRef lists refs matching pattern.
// Returns an empty slice (not nil) when no refs match.
func (g *Git) ForEachRef(ctx context.Context, pattern string) ([]RefInfo, error) {
	out, err := g.Run(ctx, "for-each-ref", "--format=%(refname) %(objectname)", pattern)
	if err != nil {
		return []RefInfo{}, nil
	}
	if out == "" {
		return []RefInfo{}, nil
	}

	var refs []RefInfo
	for _, line := range strings.Split(out, "\n") {
		// refnames cannot contain spaces; split on the last space to separate
		// the ref name from the 40-char SHA hash.
		idx := strings.LastIndex(line, " ")
		if idx < 0 {
			continue
		}
		refs = append(refs, RefInfo{
			Name: line[:idx],
			Hash: line[idx+1:],
		})
	}
	return refs, nil
}
