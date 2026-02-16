package git

import "context"

// Trailer represents a single key-value git trailer.
// Trailers are ordered and support duplicate keys, enabling multi-valued
// semantics (e.g., multiple Vendor-Name entries with positional association).
type Trailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CommitOpts configures a commit operation.
type CommitOpts struct {
	Message  string
	Trailers []Trailer // ordered key-value pairs added via --trailer
}

// Commit creates a new commit with the given options.
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
