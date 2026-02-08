package types

import (
	"fmt"
	"regexp"
	"strconv"
)

// PositionSpec represents a line/column range extracted from a path specifier.
// Supports: L5, L5-L20, L5:L20, L5-EOF, L5C10:L10C30
type PositionSpec struct {
	StartLine int // 1-indexed
	EndLine   int // 1-indexed, 0 means same as StartLine (single line)
	StartCol  int // 1-indexed, 0 means no column specified
	EndCol    int // 1-indexed inclusive, 0 means no column specified
	ToEOF     bool
}

// IsSingleLine returns true if the position targets a single line (no range).
func (p *PositionSpec) IsSingleLine() bool {
	return !p.ToEOF && (p.EndLine == 0 || p.EndLine == p.StartLine)
}

// HasColumns returns true if column-level precision is specified.
func (p *PositionSpec) HasColumns() bool {
	return p.StartCol > 0
}

// positionRegexes are compiled once for parsing position specifiers.
var (
	// L5C10:L10C30 — column-precise range
	reColRange = regexp.MustCompile(`^L(\d+)C(\d+):L(\d+)C(\d+)$`)
	// L5-L10 or L5:L10 — line range
	reLineRange = regexp.MustCompile(`^L(\d+)[-:]L(\d+)$`)
	// L5-EOF — line to end of file
	reLineEOF = regexp.MustCompile(`^L(\d+)-EOF$`)
	// L5 — single line
	reSingleLine = regexp.MustCompile(`^L(\d+)$`)
)

// ParsePathPosition splits a path string into the file path and an optional PositionSpec.
// Returns (filePath, position, error). position is nil if no position specifier is found.
//
// Examples:
//
//	"src/file.go"           -> ("src/file.go", nil, nil)
//	"src/file.go:L5"        -> ("src/file.go", &PositionSpec{StartLine:5}, nil)
//	"src/file.go:L5-L20"    -> ("src/file.go", &PositionSpec{StartLine:5, EndLine:20}, nil)
//	"src/file.go:L10-EOF"   -> ("src/file.go", &PositionSpec{StartLine:10, ToEOF:true}, nil)
//	"src/file.go:L5C10:L5C30" -> ("src/file.go", &PositionSpec{...columns...}, nil)
func ParsePathPosition(path string) (string, *PositionSpec, error) {
	// Find the first occurrence of ":L<digit>" which marks the position specifier.
	// We search for ":L<digit>" rather than ":" to avoid splitting on Windows drive letters
	// or other colon-containing paths. We use the first match because the position specifier
	// itself may contain internal colons (e.g., L5C10:L10C30).
	idx := findPositionStart(path)
	if idx == -1 {
		return path, nil, nil
	}

	filePath := path[:idx]
	specStr := path[idx+1:] // everything after the first colon (e.g., "L5C10:L10C30")

	if filePath == "" {
		return "", nil, fmt.Errorf("empty file path in position specifier: %s", path)
	}

	pos, err := parsePositionSpecifier(specStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid position specifier in %q: %w", path, err)
	}

	return filePath, pos, nil
}

// findPositionStart finds the index of the first ":L" followed by a digit.
// Returns -1 if no position specifier is found.
func findPositionStart(path string) int {
	for i := 0; i < len(path)-2; i++ {
		if path[i] == ':' && path[i+1] == 'L' && path[i+2] >= '0' && path[i+2] <= '9' {
			return i
		}
	}
	return -1
}

// parsePositionSpecifier parses a position string (without the leading colon).
func parsePositionSpecifier(spec string) (*PositionSpec, error) {
	// Try column-precise range: L5C10:L10C30
	if m := reColRange.FindStringSubmatch(spec); m != nil {
		startLine, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid start line %q: %w", m[1], err)
		}
		startCol, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("invalid start column %q: %w", m[2], err)
		}
		endLine, err := strconv.Atoi(m[3])
		if err != nil {
			return nil, fmt.Errorf("invalid end line %q: %w", m[3], err)
		}
		endCol, err := strconv.Atoi(m[4])
		if err != nil {
			return nil, fmt.Errorf("invalid end column %q: %w", m[4], err)
		}

		if err := validateLineCol(startLine, startCol, endLine, endCol); err != nil {
			return nil, err
		}

		return &PositionSpec{
			StartLine: startLine,
			EndLine:   endLine,
			StartCol:  startCol,
			EndCol:    endCol,
		}, nil
	}

	// Try line range: L5-L10 or L5:L10
	if m := reLineRange.FindStringSubmatch(spec); m != nil {
		startLine, _ := strconv.Atoi(m[1])
		endLine, _ := strconv.Atoi(m[2])

		if startLine < 1 {
			return nil, fmt.Errorf("start line must be >= 1, got %d", startLine)
		}
		if endLine < startLine {
			return nil, fmt.Errorf("end line (%d) must be >= start line (%d)", endLine, startLine)
		}

		return &PositionSpec{
			StartLine: startLine,
			EndLine:   endLine,
		}, nil
	}

	// Try line-to-EOF: L5-EOF
	if m := reLineEOF.FindStringSubmatch(spec); m != nil {
		startLine, _ := strconv.Atoi(m[1])
		if startLine < 1 {
			return nil, fmt.Errorf("start line must be >= 1, got %d", startLine)
		}
		return &PositionSpec{
			StartLine: startLine,
			ToEOF:     true,
		}, nil
	}

	// Try single line: L5
	if m := reSingleLine.FindStringSubmatch(spec); m != nil {
		startLine, _ := strconv.Atoi(m[1])
		if startLine < 1 {
			return nil, fmt.Errorf("line must be >= 1, got %d", startLine)
		}
		return &PositionSpec{
			StartLine: startLine,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized position format: %s (expected L<n>, L<n>-L<m>, L<n>-EOF, or L<n>C<c>:L<m>C<d>)", spec)
}

// validateLineCol validates column-precise position parameters.
func validateLineCol(startLine, startCol, endLine, endCol int) error {
	if startLine < 1 {
		return fmt.Errorf("start line must be >= 1, got %d", startLine)
	}
	if startCol < 1 {
		return fmt.Errorf("start column must be >= 1, got %d", startCol)
	}
	if endLine < startLine {
		return fmt.Errorf("end line (%d) must be >= start line (%d)", endLine, startLine)
	}
	if endCol < 1 {
		return fmt.Errorf("end column must be >= 1, got %d", endCol)
	}
	if startLine == endLine && endCol < startCol {
		return fmt.Errorf("on same line, end column (%d) must be >= start column (%d)", endCol, startCol)
	}
	return nil
}
