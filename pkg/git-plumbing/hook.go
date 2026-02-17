package git

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// reAICoAuthor matches Co-Authored-By trailers from known AI providers.
// Used by HookPrepareCommitMsg to select assisted/v1 vs manual/v1 namespace.
var reAICoAuthor = regexp.MustCompile(`(?im)^Co-Authored-By:.*\b(Claude|GPT|Copilot|Gemini)\b`)

// SharedTrailers computes the auto-generated COMMIT-SCHEMA v1 enrichment
// trailers from staged changes in the git repo at dir. Returns trailers for:
// Touch (if any #tags found), Diff-Additions, Diff-Deletions, Diff-Files,
// and Diff-Surface.
//
// SharedTrailers does NOT include Commit-Schema (namespace varies by caller)
// or Tags (author-declared, not auto-computed). Callers that need those
// trailers append them separately.
func SharedTrailers(ctx context.Context, dir string) ([]Trailer, error) {
	g := New(dir)

	names, err := g.DiffCachedNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("SharedTrailers DiffCachedNames: %w", err)
	}

	var trailers []Trailer

	// Touch: extract #tags from staged files
	if len(names) > 0 {
		absPaths := make([]string, len(names))
		for i, n := range names {
			absPaths[i] = filepath.Join(dir, n)
		}
		scanResult, _ := TagScan(absPaths)
		tags := MergeTags(scanResult)
		if len(tags) > 0 {
			trailers = append(trailers, Trailer{Key: "Touch", Value: strings.Join(tags, ", ")})
		}
	}

	// Diff metrics
	stat, err := g.DiffCachedStat(ctx)
	if err != nil {
		return nil, fmt.Errorf("SharedTrailers DiffCachedStat: %w", err)
	}
	trailers = append(trailers,
		Trailer{Key: "Diff-Additions", Value: strconv.Itoa(stat.Total.Added)},
		Trailer{Key: "Diff-Deletions", Value: strconv.Itoa(stat.Total.Removed)},
		Trailer{Key: "Diff-Files", Value: strconv.Itoa(len(stat.Files))},
	)

	// Diff-Surface
	surface := ClassifySurface(names, DefaultSurfaceRules())
	trailers = append(trailers, Trailer{Key: "Diff-Surface", Value: string(surface)})

	return trailers, nil
}

// HookPrepareCommitMsg enriches a commit message with shared trailers:
// Commit-Schema, Touch, Diff-Additions, Diff-Deletions, Diff-Files, and
// Diff-Surface. Reads staged state from the git repo at dir.
// Does not write the file — caller decides whether to write.
// Existing trailer keys are never overwritten (idempotent).
func HookPrepareCommitMsg(ctx context.Context, dir string, msg string) (string, error) {
	// Detect AI-authored commits via Co-Authored-By trailer.
	// assisted/v1 = LLM composed the message, hook enriched it.
	// manual/v1   = human commit, hook enriched it.
	ns := "manual/v1"
	if reAICoAuthor.MatchString(msg) {
		ns = "assisted/v1"
	}
	msg = AppendTrailer(msg, "Commit-Schema", ns)

	trailers, err := SharedTrailers(ctx, dir)
	if err != nil {
		return "", err
	}
	for _, t := range trailers {
		msg = AppendTrailer(msg, t.Key, t.Value)
	}

	return msg, nil
}

// HookCommitMsg validates and normalizes a commit message.
// Returns the normalized message, non-fatal warnings, and any fatal error.
// Warnings are emitted for subject length violations and invalid tag syntax.
// Tags and Touch trailer values are normalized to lowercase.
// Returns an error if a Commit-Schema namespace requires trailers that are
// missing: agent/v1 requires Agent-Id, Intent, Tags; vendor/v1 requires
// Vendor-Name, Vendor-Ref, Vendor-Commit. assisted/v1 and manual/v1 have no extra requirements.
func HookCommitMsg(_ context.Context, msg string) (string, []string, error) {
	var warnings []string
	lines := strings.Split(msg, "\n")

	// Warn if subject line exceeds 72 characters
	if len(lines) > 0 && len(lines[0]) > 72 {
		warnings = append(warnings, fmt.Sprintf("subject line is %d characters (max recommended: 72)", len(lines[0])))
	}

	// Normalize Tags and Touch trailer values to lowercase, validate tag syntax
	for i, line := range lines {
		key, value, ok := splitTrailerLine(strings.TrimSpace(line))
		if !ok {
			continue
		}
		if key == "Tags" || key == "Touch" {
			lower := strings.ToLower(value)
			if lower != value {
				idx := strings.Index(lines[i], ": ")
				if idx >= 0 {
					lines[i] = lines[i][:idx+2] + strings.ToLower(lines[i][idx+2:])
				}
			}
			// Validate each tag in the list
			for _, tag := range ParseTagList(lower) {
				if !IsValidTag(tag) {
					warnings = append(warnings, fmt.Sprintf("invalid tag syntax in %s: %q", key, tag))
				}
			}
		}
	}

	// Validate namespace-specific required trailers
	joined := strings.Join(lines, "\n")
	schemaValue := trailerValue(joined, "Commit-Schema")
	if schemaValue != "" {
		var required []string
		switch {
		case strings.Contains(schemaValue, "agent/v1"):
			required = []string{"Agent-Id", "Intent", "Tags"}
		case strings.Contains(schemaValue, "vendor/v1"):
			required = []string{"Vendor-Name", "Vendor-Ref", "Vendor-Commit"}
		}

		var missing []string
		for _, key := range required {
			if !hasTrailerKey(joined, key) {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return joined, warnings, fmt.Errorf("%s commit missing required trailers: %s", schemaValue, strings.Join(missing, ", "))
		}
	}

	return joined, warnings, nil
}

// IsValidTag reports whether tag matches the canonical tag regex pattern:
// starts with lowercase letter, contains only lowercase letters, digits,
// dots, and hyphens, and is at most maxTagLen characters.
func IsValidTag(tag string) bool {
	if tag == "" || len(tag) > maxTagLen {
		return false
	}
	if tag[0] < 'a' || tag[0] > 'z' {
		return false
	}
	for i := 1; i < len(tag); i++ {
		c := tag[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-' {
			continue
		}
		return false
	}
	return true
}

// AppendTrailer adds a "Key: Value" trailer to a commit message if the key
// does not already exist. Returns the message unchanged if the key is present.
// Handles empty messages, messages with body text, and messages with existing
// trailer blocks. The trailer block is separated from the body by a blank line.
func AppendTrailer(msg, key, value string) string {
	// Check if key already exists in the message
	if hasTrailerKey(msg, key) {
		return msg
	}
	trailer := key + ": " + value

	// Empty message: just the trailer
	if strings.TrimSpace(msg) == "" {
		return trailer
	}

	lines := strings.Split(msg, "\n")

	// Walk backward to find existing trailer block
	end := len(lines) - 1
	// Skip trailing blank lines
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < 0 {
		return trailer
	}

	// Check if the last non-blank lines form a trailer block
	trailerStart := end
	for trailerStart >= 0 {
		_, _, ok := splitTrailerLine(strings.TrimSpace(lines[trailerStart]))
		if !ok {
			break
		}
		trailerStart--
	}
	trailerStart++ // first trailer line

	if trailerStart <= end {
		// Existing trailer block found — check if preceded by blank line
		if trailerStart > 0 && strings.TrimSpace(lines[trailerStart-1]) == "" {
			// Insert new trailer after the last trailer line
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:end+1]...)
			result = append(result, trailer)
			if end+1 < len(lines) {
				result = append(result, lines[end+1:]...)
			}
			return strings.Join(result, "\n")
		}
	}

	// No trailer block — append with blank line separator
	// Trim trailing blank lines first
	trimmed := lines[:end+1]
	result := make([]string, 0, len(trimmed)+3)
	result = append(result, trimmed...)
	result = append(result, "", trailer)
	return strings.Join(result, "\n")
}

// hasTrailerKey reports whether msg contains a trailer line starting with "key: ".
func hasTrailerKey(msg, key string) bool {
	prefix := key + ": "
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

// trailerValue returns the value of the first trailer matching key in msg,
// or "" if not found.
func trailerValue(msg, key string) string {
	prefix := key + ": "
	for _, line := range strings.Split(msg, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return trimmed[len(prefix):]
		}
	}
	return ""
}
