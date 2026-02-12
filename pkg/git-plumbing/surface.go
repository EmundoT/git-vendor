package git

import (
	"path/filepath"
	"strings"
)

// Surface represents the primary impact surface of a change.
type Surface string

const (
	SurfaceAPI      Surface = "api"
	SurfaceData     Surface = "data"
	SurfaceConfig   Surface = "config"
	SurfaceInternal Surface = "internal"
	SurfaceTest     Surface = "test"
	SurfaceDocs     Surface = "docs"
)

// surfacePriority maps each Surface to its priority rank.
// Higher values win when multiple surfaces are present.
var surfacePriority = map[Surface]int{
	SurfaceAPI:      6,
	SurfaceData:     5,
	SurfaceConfig:   4,
	SurfaceInternal: 3,
	SurfaceTest:     2,
	SurfaceDocs:     1,
}

// SurfaceRule maps a path pattern to a surface classification.
// Pattern is matched against file paths using filepath.Match on the
// basename, or strings.HasPrefix/strings.Contains for directory prefixes.
type SurfaceRule struct {
	Pattern string
	Surface Surface
}

// DefaultSurfaceRules returns the default path-to-surface mapping.
// Callers may append or replace rules for project-specific overrides.
func DefaultSurfaceRules() []SurfaceRule {
	return []SurfaceRule{
		// docs
		{Pattern: "docs/", Surface: SurfaceDocs},
		{Pattern: "*.md", Surface: SurfaceDocs},
		{Pattern: "LICENSE*", Surface: SurfaceDocs},

		// test
		{Pattern: "*_test.go", Surface: SurfaceTest},
		{Pattern: "*_test.ts", Surface: SurfaceTest},
		{Pattern: "*_test.py", Surface: SurfaceTest},
		{Pattern: "__tests__/", Surface: SurfaceTest},
		{Pattern: "*.test.*", Surface: SurfaceTest},
		{Pattern: "*.spec.*", Surface: SurfaceTest},

		// internal
		{Pattern: "internal/", Surface: SurfaceInternal},
		{Pattern: "pkg/", Surface: SurfaceInternal},

		// config
		{Pattern: "*.yml", Surface: SurfaceConfig},
		{Pattern: "*.yaml", Surface: SurfaceConfig},
		{Pattern: "*.toml", Surface: SurfaceConfig},
		{Pattern: "*.json", Surface: SurfaceConfig},
		{Pattern: "Makefile", Surface: SurfaceConfig},
		{Pattern: "Dockerfile", Surface: SurfaceConfig},
		{Pattern: "*.config.*", Surface: SurfaceConfig},

		// data
		{Pattern: "migrations/", Surface: SurfaceData},
		{Pattern: "schema*", Surface: SurfaceData},
		{Pattern: "models/", Surface: SurfaceData},

		// api
		{Pattern: "cmd/", Surface: SurfaceAPI},
		{Pattern: "api/", Surface: SurfaceAPI},
		{Pattern: "handler*", Surface: SurfaceAPI},
		{Pattern: "routes*", Surface: SurfaceAPI},
		{Pattern: "endpoint*", Surface: SurfaceAPI},
	}
}

// ClassifySurface determines the primary impact surface from a list of
// file paths. Uses the provided rules for classification. Falls back to
// SurfaceInternal for files matching no rule. Returns the highest-priority
// surface found (api > data > config > internal > test > docs).
//
// ClassifySurface returns SurfaceInternal if paths is empty.
func ClassifySurface(paths []string, rules []SurfaceRule) Surface {
	if len(paths) == 0 {
		return SurfaceInternal
	}
	best := 0
	var bestSurface Surface
	for _, p := range paths {
		s := ClassifyFile(p, rules)
		pri := surfacePriority[s]
		if pri > best {
			best = pri
			bestSurface = s
		}
	}
	return bestSurface
}

// ClassifyFile returns the surface classification for a single file path.
// Each rule is evaluated; the matching rule with the highest priority wins.
// ClassifyFile returns SurfaceInternal if no rule matches.
func ClassifyFile(path string, rules []SurfaceRule) Surface {
	best := 0
	bestSurface := SurfaceInternal
	for _, r := range rules {
		if matchRule(path, r.Pattern) {
			pri := surfacePriority[r.Surface]
			if pri > best {
				best = pri
				bestSurface = r.Surface
			}
		}
	}
	return bestSurface
}

// matchRule checks whether path matches the given pattern.
// Directory-prefix patterns (ending with "/") use substring matching
// against the full path. Glob patterns use filepath.Match against the
// basename AND each directory segment of the path.
func matchRule(path string, pattern string) bool {
	// Normalize to forward slashes for consistent matching.
	normalized := filepath.ToSlash(path)
	base := filepath.Base(normalized)

	if strings.HasSuffix(pattern, "/") {
		// Directory-prefix pattern: match anywhere in the path.
		// "internal/" matches "internal/foo.go" and "src/internal/bar.go".
		dir := pattern // e.g. "internal/"
		return strings.HasPrefix(normalized, dir) || strings.Contains(normalized, "/"+dir)
	}

	// Glob pattern: match against basename first.
	if matched, _ := filepath.Match(pattern, base); matched {
		return true
	}

	// Also match against each directory segment. This handles patterns
	// like "handler*" matching "handlers/auth.go" (segment "handlers").
	parts := strings.Split(normalized, "/")
	for _, seg := range parts[:len(parts)-1] {
		if matched, _ := filepath.Match(pattern, seg); matched {
			return true
		}
	}
	return false
}
