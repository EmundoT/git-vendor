package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CopyStats tracks file copy statistics
type CopyStats struct {
	FileCount int
	ByteCount int64
}

// Add adds another CopyStats to this one
func (s *CopyStats) Add(other CopyStats) {
	s.FileCount += other.FileCount
	s.ByteCount += other.ByteCount
}

// FileSystem abstracts file system operations for testing
type FileSystem interface {
	CopyFile(src, dst string) (CopyStats, error)
	CopyDir(src, dst string) (CopyStats, error)
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(path string) ([]string, error)
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	CreateTemp(dir, pattern string) (string, error)
	RemoveAll(path string) error
}

// OSFileSystem implements FileSystem using standard os package
type OSFileSystem struct{}

// NewOSFileSystem creates a new OSFileSystem
func NewOSFileSystem() *OSFileSystem {
	return &OSFileSystem{}
}

// CopyFile copies a single file from src to dst
func (fs *OSFileSystem) CopyFile(src, dst string) (CopyStats, error) {
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

// CopyDir recursively copies a directory from src to dst
func (fs *OSFileSystem) CopyDir(src, dst string) (CopyStats, error) {
	var stats CopyStats

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
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

// ValidateDestPath ensures destination path is safe and doesn't allow path traversal
func ValidateDestPath(destPath string) error {
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
