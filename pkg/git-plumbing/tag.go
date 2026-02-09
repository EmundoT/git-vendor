package git

import "context"

// TagsAt returns all tags pointing at the given commit.
func (g *Git) TagsAt(ctx context.Context, commitHash string) ([]string, error) {
	lines, err := g.RunLines(ctx, "tag", "--points-at", commitHash)
	if err != nil {
		return nil, nil // no tags is not an error
	}
	return lines, nil
}

// CreateTag creates a lightweight tag at the current HEAD.
func (g *Git) CreateTag(ctx context.Context, name string) error {
	return g.RunSilent(ctx, "tag", name)
}

// DeleteTag removes a tag.
func (g *Git) DeleteTag(ctx context.Context, name string) error {
	return g.RunSilent(ctx, "tag", "-d", name)
}

// ListTags returns tags matching a pattern, sorted by creation date (newest first).
func (g *Git) ListTags(ctx context.Context, pattern string) ([]string, error) {
	args := []string{"tag", "-l", "--sort=-creatordate"}
	if pattern != "" {
		args = append(args, pattern)
	}
	return g.RunLines(ctx, args...)
}
