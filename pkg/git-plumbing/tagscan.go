package git

import (
	"os"
	"regexp"
	"slices"
)

// tagRe is the canonical regex for extracting #tag annotations from source code.
// Tags start with a lowercase letter and contain lowercase letters, digits, dots,
// and hyphens. The # must be preceded by a non-alphanumeric character or start of line.
// Defined in COMMIT-SCHEMA §6.2 — MUST NOT be modified.
var tagRe = regexp.MustCompile(`(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)`)

// maxTagLen is the maximum allowed length for an extracted tag.
const maxTagLen = 128

// binaryProbeSize is the number of bytes scanned for null bytes to detect binary files.
const binaryProbeSize = 8192

// TagScanResult maps file paths to the tags found in each file.
type TagScanResult map[string][]string

// TagScan extracts #tag annotations from the given file paths.
// Tags are extracted using the canonical regex from COMMIT-SCHEMA §6.2.
// Returns a map of file path to deduplicated, sorted tags found.
// Files that contain no tags are omitted from the result.
// Files that cannot be read (missing, binary, permission denied) are silently skipped.
func TagScan(paths []string) (TagScanResult, error) {
	result := make(TagScanResult)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if isBinary(data) {
			continue
		}
		tags := TagScanContent(string(data))
		if len(tags) > 0 {
			result[p] = tags
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// TagScanContent extracts #tag annotations from raw string content.
// Returns a deduplicated, sorted slice of tag names (without the leading #).
// Useful when content is already in memory (e.g., from a staged diff).
func TagScanContent(content string) []string {
	matches := tagRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var tags []string
	for _, m := range matches {
		tag := m[1]
		if len(tag) > maxTagLen {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return tags
}

// MergeTags collects all unique tags across a TagScanResult.
// Returns a deduplicated, sorted slice.
func MergeTags(result TagScanResult) []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, fileTags := range result {
		for _, tag := range fileTags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)
	return tags
}

// isBinary reports whether data appears to be a binary file by scanning
// the first binaryProbeSize bytes for null bytes.
func isBinary(data []byte) bool {
	end := len(data)
	if end > binaryProbeSize {
		end = binaryProbeSize
	}
	for i := 0; i < end; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
