package git

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Commit represents a parsed git commit.
type Commit struct {
	Hash     string
	Short    string
	Subject  string
	Author   string
	Date     time.Time
	Trailers map[string]string // key-value pairs from commit trailers
}

// LogOpts configures a log query.
type LogOpts struct {
	Range    string // e.g., "abc123..def456" or "HEAD~5..HEAD"
	MaxCount int
	Author   string // --author filter
	Grep     string // --grep filter (matches commit message)
	All      bool   // --all flag
	OneLine  bool   // simplified one-line format (hash + subject only)
}

// Log returns commits matching the given options.
// Uses null-byte delimiters for safe parsing.
func (g *Git) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
	var format string
	if opts.OneLine {
		format = "--pretty=format:%H%x00%h%x00%s"
	} else {
		format = "--pretty=format:%H%x00%h%x00%s%x00%an%x00%aI"
	}
	args := []string{"log", format}

	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-%d", opts.MaxCount))
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.Grep != "" {
		args = append(args, "--grep="+opts.Grep)
	}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.Range != "" {
		args = append(args, opts.Range)
	}

	out, err := g.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\x00")
		if opts.OneLine {
			if len(parts) != 3 {
				continue
			}
			commits = append(commits, Commit{
				Hash:    parts[0],
				Short:   parts[1],
				Subject: parts[2],
			})
		} else {
			if len(parts) != 5 {
				continue
			}
			date, _ := time.Parse(time.RFC3339, parts[4])
			commits = append(commits, Commit{
				Hash:    parts[0],
				Short:   parts[1],
				Subject: parts[2],
				Author:  parts[3],
				Date:    date,
			})
		}
	}
	return commits, nil
}
