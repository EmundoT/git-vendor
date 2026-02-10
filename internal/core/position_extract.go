package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ExtractPosition reads a file and extracts the content specified by a PositionSpec.
// Returns the extracted content as a string and the SHA-256 hash of that content.
// Returns an error if the file appears to be binary (contains null bytes).
func ExtractPosition(filePath string, pos *types.PositionSpec) (string, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("read file %s: %w", filePath, err)
	}

	if isBinaryContent(data) {
		return "", "", fmt.Errorf("position extraction on binary file %s is not supported", filePath)
	}

	content, err := extractFromContent(string(data), pos, filePath)
	if err != nil {
		return "", "", err
	}

	hash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	return content, hash, nil
}

// extractFromContent extracts a position range from file content.
// filePath is used only for error messages.
// CRLF line endings are normalized to LF before processing (see PositionSpec docs).
func extractFromContent(data string, pos *types.PositionSpec, filePath string) (string, error) {
	data = normalizeCRLF(data)
	lines := strings.Split(data, "\n")
	totalLines := len(lines)

	// Validate start line
	if pos.StartLine > totalLines {
		return "", fmt.Errorf("line %d does not exist in %s (%d lines)", pos.StartLine, filePath, totalLines)
	}

	// Determine effective end line
	endLine := pos.StartLine
	if pos.ToEOF {
		endLine = totalLines
	} else if pos.EndLine > 0 {
		endLine = pos.EndLine
	}

	if endLine > totalLines {
		return "", fmt.Errorf("line %d does not exist in %s (%d lines)", endLine, filePath, totalLines)
	}

	// Column-precise extraction
	if pos.HasColumns() {
		return extractColumns(lines, pos, filePath)
	}

	// Line-range extraction (1-indexed to 0-indexed)
	extracted := lines[pos.StartLine-1 : endLine]
	return strings.Join(extracted, "\n"), nil
}

// extractColumns handles column-precise extraction.
//
// StartCol boundary asymmetry (intentional):
// Single-line mode errors when StartCol > len(line) because there is nothing to extract.
// Multi-line mode allows StartCol up to len(firstLine)+1 and clamps to the end,
// because starting "past the end" of the first line means extracting only from
// subsequent lines, which is semantically valid.
func extractColumns(lines []string, pos *types.PositionSpec, filePath string) (string, error) {
	// Single-line column extraction
	if pos.StartLine == pos.EndLine {
		line := lines[pos.StartLine-1]
		if pos.StartCol > len(line) {
			return "", fmt.Errorf("column %d exceeds line length (%d chars) in %s line %d",
				pos.StartCol, len(line), filePath, pos.StartLine)
		}
		endCol := pos.EndCol
		if endCol > len(line) {
			return "", fmt.Errorf("column %d exceeds line length (%d chars) in %s line %d",
				endCol, len(line), filePath, pos.StartLine)
		}
		return line[pos.StartCol-1 : endCol], nil
	}

	// Multi-line column extraction
	var result []string

	// First line: from startCol to end of line
	firstLine := lines[pos.StartLine-1]
	if pos.StartCol > len(firstLine)+1 {
		return "", fmt.Errorf("column %d exceeds line length (%d chars) in %s line %d",
			pos.StartCol, len(firstLine), filePath, pos.StartLine)
	}
	startIdx := pos.StartCol - 1
	if startIdx > len(firstLine) {
		startIdx = len(firstLine)
	}
	result = append(result, firstLine[startIdx:])

	// Middle lines: full lines
	for i := pos.StartLine; i < pos.EndLine-1; i++ {
		result = append(result, lines[i])
	}

	// Last line: from start to endCol
	lastLine := lines[pos.EndLine-1]
	endCol := pos.EndCol
	if endCol > len(lastLine) {
		return "", fmt.Errorf("column %d exceeds line length (%d chars) in %s line %d",
			endCol, len(lastLine), filePath, pos.EndLine)
	}
	result = append(result, lastLine[:endCol])

	return strings.Join(result, "\n"), nil
}

// PlaceContent writes extracted content into a target file at the specified position.
// If pos is nil, the content replaces the entire file.
// If pos specifies a range, only that range in the target is replaced.
//
// Security: PlaceContent self-validates relative paths via ValidateDestPath to block
// path traversal (e.g., "../../../etc/passwd"). Absolute paths bypass validation
// because absolute paths originate from internal/test usage with temp directories.
// Production callers also validate at the service layer — see file_copy_service.go:copyMapping.
func PlaceContent(filePath string, content string, pos *types.PositionSpec) error {
	// Defense in depth: validate relative paths to block traversal.
	// Absolute paths are skipped — they come from internal/test code using temp dirs.
	if !filepath.IsAbs(filePath) {
		if err := ValidateDestPath(filePath); err != nil {
			return fmt.Errorf("PlaceContent write blocked: %w", err)
		}
	}

	if pos == nil {
		// Replace entire file
		return os.WriteFile(filePath, []byte(content), 0644)
	}

	// Read existing target
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read target file %s: %w", filePath, err)
	}

	if isBinaryContent(data) {
		return fmt.Errorf("position placement into binary file %s is not supported", filePath)
	}

	result, err := placeInContent(string(data), content, pos, filePath)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(result), 0644)
}

// placeInContent replaces a range in existing content with new content.
// CRLF line endings are normalized to LF before processing (see PositionSpec docs).
func placeInContent(existing, replacement string, pos *types.PositionSpec, filePath string) (string, error) {
	existing = normalizeCRLF(existing)
	lines := strings.Split(existing, "\n")
	totalLines := len(lines)

	if pos.StartLine > totalLines {
		return "", fmt.Errorf("target line %d does not exist in %s (%d lines)", pos.StartLine, filePath, totalLines)
	}

	endLine := pos.StartLine
	if pos.ToEOF {
		endLine = totalLines
	} else if pos.EndLine > 0 {
		endLine = pos.EndLine
	}

	if endLine > totalLines {
		return "", fmt.Errorf("target line %d does not exist in %s (%d lines)", endLine, filePath, totalLines)
	}

	// Column-precise placement
	if pos.HasColumns() {
		return placeColumns(lines, replacement, pos, filePath)
	}

	// Line-range replacement
	replacementLines := strings.Split(replacement, "\n")
	var result []string
	result = append(result, lines[:pos.StartLine-1]...)
	result = append(result, replacementLines...)
	result = append(result, lines[endLine:]...)

	return strings.Join(result, "\n"), nil
}

// placeColumns handles column-precise replacement.
func placeColumns(lines []string, replacement string, pos *types.PositionSpec, filePath string) (string, error) {
	if pos.StartLine == pos.EndLine {
		// Single-line column replacement
		line := lines[pos.StartLine-1]
		if pos.StartCol > len(line)+1 || pos.EndCol > len(line) {
			return "", fmt.Errorf("column range exceeds line length (%d chars) in %s line %d",
				len(line), filePath, pos.StartLine)
		}
		lines[pos.StartLine-1] = line[:pos.StartCol-1] + replacement + line[pos.EndCol:]
		return strings.Join(lines, "\n"), nil
	}

	// Multi-line column replacement
	firstLine := lines[pos.StartLine-1]
	lastLine := lines[pos.EndLine-1]

	startIdx := pos.StartCol - 1
	if startIdx > len(firstLine) {
		startIdx = len(firstLine)
	}
	endCol := pos.EndCol
	if endCol > len(lastLine) {
		return "", fmt.Errorf("column %d exceeds line length (%d chars) in %s line %d",
			endCol, len(lastLine), filePath, pos.EndLine)
	}

	prefix := firstLine[:startIdx]
	suffix := lastLine[endCol:]

	var result []string
	result = append(result, lines[:pos.StartLine-1]...)
	result = append(result, prefix+replacement+suffix)
	result = append(result, lines[pos.EndLine:]...)

	return strings.Join(result, "\n"), nil
}

// isBinaryContent checks whether data appears to be binary by scanning for null
// bytes in the first 8000 bytes. Matches git's binary detection heuristic
// (xdiff/xutils.c:xdl_mmfile_istext). Position extraction on binary files
// produces garbage output, so ExtractPosition and PlaceContent reject binary
// content with a clear error.
func isBinaryContent(data []byte) bool {
	limit := 8000
	if len(data) < limit {
		limit = len(data)
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// normalizeCRLF replaces Windows-style \r\n line endings with \n.
// Normalization ensures consistent line splitting and column offsets across platforms.
func normalizeCRLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
