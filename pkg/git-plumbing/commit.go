package git

import "context"

// Trailer represents a single key-value trailer for git commits.
// Multiple Trailers with the same Key are valid — git's --trailer flag
// supports duplicate keys (e.g., multiple Vendor-Name entries).
// Order is preserved: the Nth occurrence of a key corresponds to the Nth
// logical group when keys repeat.
type Trailer struct {
	Key   string
	Value string
}

// CommitOpts configures a commit operation.
type CommitOpts struct {
	Message  string
	Trailers []Trailer // ordered key=value pairs added via --trailer
}

// Commit creates a new commit with the given options.
// Commit passes each trailer to git via --trailer key=value.
// Duplicate keys are passed as separate --trailer arguments,
// resulting in multi-valued trailers in the commit message.
func (g *Git) Commit(ctx context.Context, opts CommitOpts) error {
	args := []string{"commit", "-m", opts.Message}
	for _, t := range opts.Trailers {
		args = append(args, "--trailer", t.Key+"="+t.Value)
	}
	return g.RunSilent(ctx, args...)
}

// Add stages files for the next commit.
func (g *Git) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add"}, paths...)
	return g.RunSilent(ctx, args...)
}
