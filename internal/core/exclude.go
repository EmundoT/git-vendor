package core

import (
	"path/filepath"
	"strings"
)

// MatchesExclude reports whether relPath matches any of the given exclude patterns.
// Patterns use gitignore-style globs:
//   - "*" matches any sequence of non-separator characters
//   - "**" matches any sequence of characters including separators (recursive)
//   - "?" matches any single non-separator character
//
// Examples:
//   - "*.md" matches "README.md" but not "docs/guide.md"
//   - ".claude/**" matches ".claude/settings.json" and ".claude/rules/foo.md"
//   - "docs/internal/**" matches "docs/internal/design.md"
//
// All paths are normalized to forward slashes before matching.
func MatchesExclude(relPath string, patterns []string) bool {
	// Normalize to forward slashes for consistent cross-platform matching
	normalized := filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		if matchGlob(normalized, pattern) {
			return true
		}
	}
	return false
}

// matchGlob matches a path against a single glob pattern with ** support.
// Both path and pattern MUST be forward-slash normalized before calling matchGlob.
// matchGlob handles three cases:
//  1. No "**" in pattern: delegate to matchSimple (cross-platform filepath.Match)
//  2. "**" at start or end: match the fixed part and allow any prefix/suffix
//  3. "**" in middle: split and match both sides
func matchGlob(path, pattern string) bool {
	// Fast path: no ** means standard glob matching
	if !strings.Contains(pattern, "**") {
		return matchSimple(path, pattern)
	}

	// Handle ** patterns by splitting on "**" segments
	return matchDoublestar(path, pattern)
}

// matchDoublestar handles glob patterns containing "**".
// "**" matches zero or more path segments (including separators).
func matchDoublestar(path, pattern string) bool {
	// Split pattern on "**" — may produce multiple segments
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]

		// Remove trailing/leading slashes from prefix/suffix that border the **
		prefix = strings.TrimSuffix(prefix, "/")
		suffix = strings.TrimPrefix(suffix, "/")

		// Case: prefix/** (e.g., ".claude/**")
		if suffix == "" {
			if prefix == "" {
				return true // bare "**" matches everything
			}
			return path == prefix || strings.HasPrefix(path, prefix+"/")
		}

		// Case: **/suffix (e.g., "**/*.md")
		if prefix == "" {
			// Match suffix against the path itself and every subdirectory tail
			if matchSimple(path, suffix) {
				return true
			}
			for i := 0; i < len(path); i++ {
				if path[i] == '/' {
					if matchSimple(path[i+1:], suffix) {
						return true
					}
				}
			}
			return false
		}

		// Case: prefix/**/suffix (e.g., "docs/**/README.md")
		if !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}
		// Try matching suffix against every possible tail after the prefix
		remaining := strings.TrimPrefix(path, prefix+"/")
		if matchSimple(remaining, suffix) {
			return true
		}
		for i := 0; i < len(remaining); i++ {
			if remaining[i] == '/' {
				if matchSimple(remaining[i+1:], suffix) {
					return true
				}
			}
		}
		return false
	}

	// Multiple ** segments: try each split point recursively
	// This handles unusual patterns like "a/**/b/**/c"
	firstStar := strings.Index(pattern, "**")
	prefix := pattern[:firstStar]
	rest := pattern[firstStar+2:]

	prefix = strings.TrimSuffix(prefix, "/")
	rest = strings.TrimPrefix(rest, "/")

	if prefix == "" {
		// ** at the start — try matching rest against every tail
		if matchGlob(path, rest) {
			return true
		}
		for i := 0; i < len(path); i++ {
			if path[i] == '/' {
				if matchGlob(path[i+1:], rest) {
					return true
				}
			}
		}
		return false
	}

	if !strings.HasPrefix(path, prefix+"/") && path != prefix {
		return false
	}
	remaining := strings.TrimPrefix(path, prefix+"/")
	if matchGlob(remaining, rest) {
		return true
	}
	for i := 0; i < len(remaining); i++ {
		if remaining[i] == '/' {
			if matchGlob(remaining[i+1:], rest) {
				return true
			}
		}
	}
	return false
}

// matchSimple matches a path against a pattern without ** (standard glob only).
// matchSimple converts both path and pattern to OS-native separators before calling
// filepath.Match, ensuring that '*' never crosses directory boundaries regardless
// of platform (on Windows, filepath.Match uses '\' as separator, so forward slashes
// in normalized paths would be treated as literal characters without this conversion).
func matchSimple(path, pattern string) bool {
	matched, _ := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(path))
	return matched
}
