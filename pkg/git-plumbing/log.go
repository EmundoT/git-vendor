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
	Trailers []Trailer // ordered key-value pairs parsed from commit body trailers
	Body     string    // full commit body (after subject); populated when IncludeBody is set
	Notes    string    // content from git notes; populated when NotesRef is set
}

// TrailerValue returns the value of the first trailer matching key, or "".
func (c Commit) TrailerValue(key string) string {
	for _, t := range c.Trailers {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}

// TrailerValues returns all values for trailers matching key, preserving order.
func (c Commit) TrailerValues(key string) []string {
	var vals []string
	for _, t := range c.Trailers {
		if t.Key == key {
			vals = append(vals, t.Value)
		}
	}
	return vals
}

// HasTrailer reports whether any trailer with the given key exists.
func (c Commit) HasTrailer(key string) bool {
	for _, t := range c.Trailers {
		if t.Key == key {
			return true
		}
	}
	return false
}

// TagQuery represents a tag-based filter on a comma-separated trailer value.
// TagQuery uses MatchTagPrefix for segment-wise prefix matching against each
// tag in the trailer value.
type TagQuery struct {
	Key string // trailer key (e.g., "Tags", "Touch")
	Tag string // tag to search for using prefix matching
}

// LogOpts configures a log query.
type LogOpts struct {
	Range         string // e.g., "abc123..def456" or "HEAD~5..HEAD"
	MaxCount      int
	Author        string            // --author filter
	Grep          string            // --grep filter (matches commit message)
	All           bool              // --all flag
	NoMerges      bool              // --no-merges flag: exclude commits with 2+ parents
	OneLine       bool              // simplified one-line format (hash + subject only)
	IncludeBody   bool              // include body and notes in output (incompatible with OneLine)
	NotesRef      string            // e.g. "refs/notes/agent" — adds --notes=<ref> to git args
	TrailerFilter map[string]string // post-parse filter: only include commits matching all trailer key=value pairs
	TagFilter     []TagQuery        // post-parse filter: prefix-match tags in comma-separated trailer values
}

// Log returns commits matching the given options.
// Uses null-byte delimiters for safe parsing. When IncludeBody is true,
// Log uses a record-separator (\x1e) between commits to handle multi-line
// body content safely.
func (g *Git) Log(ctx context.Context, opts LogOpts) ([]Commit, error) {
	includeBody := opts.IncludeBody && !opts.OneLine

	var format string
	switch {
	case opts.OneLine:
		format = "--pretty=format:%H%x00%h%x00%s"
	case includeBody:
		format = "--pretty=format:%H%x00%h%x00%s%x00%an%x00%aI%x00%b%x00%N%x1e"
	default:
		format = "--pretty=format:%H%x00%h%x00%s%x00%an%x00%aI"
	}
	args := []string{"log", format}

	if opts.NotesRef != "" {
		args = append(args, "--notes="+opts.NotesRef)
	}
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
	if opts.NoMerges {
		args = append(args, "--no-merges")
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

	if includeBody {
		commits = parseBodyRecords(out)
	} else {
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
	}

	if len(opts.TrailerFilter) > 0 {
		commits = filterByTrailers(commits, opts.TrailerFilter)
	}
	if len(opts.TagFilter) > 0 {
		commits = filterByTagQueries(commits, opts.TagFilter)
	}

	return commits, nil
}

// parseBodyRecords parses git log output using \x1e record separators
// and \x00 field delimiters. Expected fields per record:
// hash, short, subject, author, date, body, notes (7 fields).
func parseBodyRecords(out string) []Commit {
	records := strings.Split(out, "\x1e")
	var commits []Commit
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.Split(rec, "\x00")
		if len(parts) < 7 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, parts[4])
		body := strings.TrimSpace(parts[5])
		notes := strings.TrimSpace(parts[6])
		trailers := parseTrailers(body)

		commits = append(commits, Commit{
			Hash:     parts[0],
			Short:    parts[1],
			Subject:  parts[2],
			Author:   parts[3],
			Date:     date,
			Body:     body,
			Notes:    notes,
			Trailers: trailers,
		})
	}
	return commits
}

// parseTrailers extracts key-value trailer pairs from the end of a commit body.
// Trailers are consecutive "Key: Value" lines at the very end of the body,
// preceded by a blank line. A trailer key consists of letters, digits, and
// hyphens, followed by ": " and the value. Returns trailers in top-to-bottom
// order, preserving duplicate keys.
func parseTrailers(body string) []Trailer {
	if body == "" {
		return nil
	}
	lines := strings.Split(body, "\n")

	// Walk backwards from the end to find consecutive trailer lines.
	var reversed []Trailer
	i := len(lines) - 1
	for i >= 0 {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}
		key, value, ok := splitTrailerLine(line)
		if !ok {
			break
		}
		reversed = append(reversed, Trailer{Key: key, Value: value})
		i--
	}
	if len(reversed) == 0 {
		return nil
	}
	// Reverse to restore top-to-bottom order.
	trailers := make([]Trailer, len(reversed))
	for j, t := range reversed {
		trailers[len(reversed)-1-j] = t
	}
	return trailers
}

// splitTrailerLine checks if a line matches the trailer format "Key: Value".
// The key must start with a letter and contain only letters, digits, and hyphens.
func splitTrailerLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ": ")
	if idx < 1 {
		return "", "", false
	}
	key = line[:idx]
	for i, r := range key {
		if i == 0 {
			if !isLetter(r) {
				return "", "", false
			}
			continue
		}
		if !isLetter(r) && !isDigit(r) && r != '-' {
			return "", "", false
		}
	}
	return key, line[idx+2:], true
}

func isLetter(r rune) bool { return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') }
func isDigit(r rune) bool  { return r >= '0' && r <= '9' }

// filterByTrailers returns only commits whose Trailers match every entry in filter.
// Uses Commit.TrailerValue (first value wins) for comparison.
func filterByTrailers(commits []Commit, filter map[string]string) []Commit {
	var result []Commit
	for _, c := range commits {
		if matchesTrailers(c, filter) {
			result = append(result, c)
		}
	}
	return result
}

func matchesTrailers(c Commit, filter map[string]string) bool {
	for k, v := range filter {
		if c.TrailerValue(k) != v {
			return false
		}
	}
	return true
}

// filterByTagQueries returns only commits where every TagQuery matches.
// Each TagQuery checks whether the trailer value (comma-separated list of tags)
// contains any tag matching the query via MatchTagInList (segment-wise prefix).
func filterByTagQueries(commits []Commit, queries []TagQuery) []Commit {
	var result []Commit
	for _, c := range commits {
		if matchesTagQueries(c, queries) {
			result = append(result, c)
		}
	}
	return result
}

func matchesTagQueries(c Commit, queries []TagQuery) bool {
	for _, q := range queries {
		if !MatchTagInList(q.Tag, c.TrailerValue(q.Key)) {
			return false
		}
	}
	return true
}
