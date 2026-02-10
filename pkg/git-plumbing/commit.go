package git

import "context"

// CommitOpts configures a commit operation.
type CommitOpts struct {
	Message  string
	Trailers map[string]string // key=value pairs added via --trailer
}

// Commit creates a new commit with the given options.
func (g *Git) Commit(ctx context.Context, opts CommitOpts) error {
	args := []string{"commit", "-m", opts.Message}
	for key, value := range opts.Trailers {
		args = append(args, "--trailer", key+"="+value)
	}
	return g.RunSilent(ctx, args...)
}

// Add stages files for the next commit.
func (g *Git) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add"}, paths...)
	return g.RunSilent(ctx, args...)
}
