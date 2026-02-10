package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// CopyStats tracks file copy statistics
type CopyStats struct {
	FileCount int
	ByteCount int64
	Positions []positionRecord // Position-extracted mappings (for lockfile tracking)
	Warnings  []string         // Non-fatal warnings generated during copy
}

// positionRecord tracks a single position extraction during copy
type positionRecord struct {
	From       string // Source path with position specifier
	To         string // Destination path with optional position specifier
	SourceHash string // SHA-256 hash of extracted content
}

// Add adds another CopyStats to this one
func (s *CopyStats) Add(other CopyStats) {
	s.FileCount += other.FileCount
	s.ByteCount += other.ByteCount
	s.Positions = append(s.Positions, other.Positions...)
	s.Warnings = append(s.Warnings, other.Warnings...)
}

// FileSystem abstracts file system operations for testing.
// Implementations that support write validation (e.g., rooted filesystems) SHOULD
// enforce path containment in ValidateWritePath, CopyFile, and CopyDir.
type FileSystem interface {
	CopyFile(src, dst string) (CopyStats, error)
	CopyDir(src, dst string) (CopyStats, error)
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(path string) ([]string, error)
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	CreateTemp(dir, pattern string) (string, error)
	RemoveAll(path string) error
	// ValidateWritePath checks that path is a safe write destination.
	// For rooted filesystems, this verifies the resolved path is within projectRoot.
	// For unrooted filesystems, this returns nil (no restriction).
	ValidateWritePath(path string) error
}

// OSFileSystem implements FileSystem using standard os package.
// When projectRoot is set (via NewRootedFileSystem), write operations self-validate
// that destinations resolve within the root — preventing path traversal even if
// callers forget to call ValidateDestPath.
type OSFileSystem struct {
	projectRoot string // Absolute path; empty = unrooted (no write validation)
}

// NewOSFileSystem creates an unrooted OSFileSystem with no write validation.
// Use NewRootedFileSystem for production code that handles user-controlled paths.
func NewOSFileSystem() *OSFileSystem {
	return &OSFileSystem{}
}

// NewRootedFileSystem creates an OSFileSystem that validates all write destinations
// resolve within projectRoot. Production code SHOULD use this constructor.
func NewRootedFileSystem(projectRoot string) *OSFileSystem {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		abs = projectRoot
	}
	return &OSFileSystem{projectRoot: abs}
}

// ValidateWritePath checks that path resolves within projectRoot.
// Returns nil if the filesystem is unrooted (projectRoot is empty).
func (fs *OSFileSystem) ValidateWritePath(path string) error {
	if fs.projectRoot == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve write path %q: %w", path, err)
	}
	// Check containment: resolved path must be within projectRoot.
	// Use separator suffix to prevent prefix collision (e.g., /tmp/foo vs /tmp/foobar).
	root := fs.projectRoot + string(filepath.Separator)
	if abs != fs.projectRoot && !strings.HasPrefix(abs, root) {
		return fmt.Errorf("write blocked: path %q resolves to %q which is outside project root %q", path, abs, fs.projectRoot)
	}
	return nil
}

// CopyFile copies a single file from src to dst.
//
// Security: When the filesystem is rooted (created via NewRootedFileSystem), CopyFile
// self-validates that dst resolves within projectRoot. For unrooted filesystems,
// callers MUST call ValidateDestPath(dst) before invoking CopyFile with user-controlled
// destination paths. See file_copy_service.go:copyMapping for the caller-level validation.
func (fs *OSFileSystem) CopyFile(src, dst string) (CopyStats, error) {
	if err := fs.ValidateWritePath(dst); err != nil {
		return CopyStats{}, err
	}

	source, err := os.Open(src)
	if err != nil {
		return CopyStats{}, err
	}
	defer func() { _ = source.Close() }()

	dest, err := os.Create(dst)
	if err != nil {
		return CopyStats{}, err
	}
	defer func() { _ = dest.Close() }()

	bytes, err := io.Copy(dest, source)
	if err != nil {
		return CopyStats{}, err
	}

	return CopyStats{FileCount: 1, ByteCount: bytes}, nil
}

// CopyDir recursively copies a directory from src to dst.
//
// Security: When the filesystem is rooted (created via NewRootedFileSystem), CopyDir
// self-validates that dst resolves within projectRoot. For unrooted filesystems,
// callers MUST call ValidateDestPath(dst) before invoking CopyDir with user-controlled
// destination paths. See file_copy_service.go:copyMapping for the caller-level validation.
func (fs *OSFileSystem) CopyDir(src, dst string) (CopyStats, error) {
	if err := fs.ValidateWritePath(dst); err != nil {
		return CopyStats{}, err
	}

	var stats CopyStats

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if strings.Contains(relPath, ".git") {
			return nil
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file and add to stats
		fileStats, err := fs.CopyFile(path, destPath)
		if err != nil {
			return err
		}
		stats.Add(fileStats)

		return nil
	})

	return stats, err
}

// MkdirAll creates a directory path
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ReadDir lists directory contents
func (fs *OSFileSystem) ReadDir(path string) ([]string, error) {
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		items = append(items, name)
	}

	sort.Strings(items)
	return items, nil
}

// Stat returns file info
func (fs *OSFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Remove removes a file
func (fs *OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}

// CreateTemp creates a temporary directory
func (fs *OSFileSystem) CreateTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// RemoveAll removes a directory tree
func (fs *OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// ValidateDestPath ensures destination path is safe and doesn't allow path traversal.
// ValidateDestPath strips any position specifier (e.g., ":L5-L10") before validation,
// so paths like "config.go:L5-L10" are accepted if the file path part is valid.
func ValidateDestPath(destPath string) error {
	// Strip position specifier before validation — only validate the file path part
	pathPart, _, parseErr := types.ParsePathPosition(destPath)
	if parseErr != nil {
		pathPart = destPath // Fallback to raw path if position parsing fails
	}
	destPath = pathPart

	// Reject embedded null bytes — defense in depth (OS also rejects at syscall level,
	// but fail early with a clear message instead of relying on EINVAL).
	if strings.ContainsRune(destPath, 0) {
		return fmt.Errorf("invalid destination path: (null bytes are not allowed)")
	}

	// Clean the path to normalize it
	cleaned := filepath.Clean(destPath)

	// Check if path starts with / or \ (Unix-style absolute or root-relative)
	if strings.HasPrefix(destPath, "/") || strings.HasPrefix(destPath, "\\") {
		return fmt.Errorf("invalid destination path: %s (absolute paths are not allowed)", destPath)
	}

	// Check for Windows drive letters (cross-platform check: C:, D:, etc.)
	// This works even on Linux to prevent Windows-style absolute paths
	if len(destPath) >= 2 && destPath[1] == ':' && destPath[0] >= 'A' && destPath[0] <= 'Z' ||
		len(destPath) >= 2 && destPath[1] == ':' && destPath[0] >= 'a' && destPath[0] <= 'z' {
		return fmt.Errorf("invalid destination path: %s (absolute paths are not allowed)", destPath)
	}

	// Check if path is absolute (security risk) - handles platform-specific absolute paths
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("invalid destination path: %s (absolute paths are not allowed)", destPath)
	}

	// Check if path contains .. (path traversal attack)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return fmt.Errorf("invalid destination path: %s (path traversal with .. is not allowed)", destPath)
	}

	return nil
}

// ValidateVendorName ensures a vendor name is safe for use in filesystem paths.
// Rejects names containing path separators, traversal sequences, or null bytes.
// Called during config validation and before license file copy to block malicious
// vendor.yml entries before they reach any filesystem operation.
func ValidateVendorName(name string) error {
	if name == "" {
		return fmt.Errorf("vendor name must not be empty")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("invalid vendor name %q: null bytes are not allowed", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid vendor name %q: path separators are not allowed", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid vendor name %q: path traversal sequences are not allowed", name)
	}
	return nil
}
