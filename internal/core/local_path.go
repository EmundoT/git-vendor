package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsLocalPath detects whether rawURL points to a local filesystem path.
// Matches file:// scheme, relative paths (./  ../), Unix absolute paths (/),
// and Windows drive letters (C:\, D:/).
func IsLocalPath(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "file://") {
		return true
	}

	// Relative paths
	if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") ||
		strings.HasPrefix(trimmed, ".\\") || strings.HasPrefix(trimmed, "..\\") {
		return true
	}

	// Unix absolute path (but not a URL scheme like ssh://)
	if strings.HasPrefix(trimmed, "/") && !strings.Contains(trimmed, "://") {
		return true
	}

	// Windows drive letter: C:\ or C:/
	if len(trimmed) >= 3 && trimmed[1] == ':' &&
		((trimmed[0] >= 'A' && trimmed[0] <= 'Z') || (trimmed[0] >= 'a' && trimmed[0] <= 'z')) &&
		(trimmed[2] == '/' || trimmed[2] == '\\') {
		return true
	}

	return false
}

// ResolveLocalURL resolves a local vendor URL to an absolute file:// URL.
// Relative paths are resolved against filepath.Dir(vendorDir), which is the
// project root. Absolute file:// URLs pass through after path validation.
// Returns an error if the resolved path does not exist or is not a directory.
func ResolveLocalURL(rawURL, vendorDir string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	projectRoot := filepath.Dir(vendorDir)

	var absPath string

	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		// Strip file:// prefix — handle file:///path and file://path
		pathPart := trimmed[len("file://"):]
		// On Windows, file:///C:/path → C:/path
		if runtime.GOOS == "windows" && len(pathPart) >= 2 && pathPart[0] == '/' {
			pathPart = pathPart[1:]
		}
		absPath = filepath.Clean(pathPart)
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(projectRoot, absPath)
		}
	} else {
		// Relative or absolute filesystem path
		absPath = filepath.Clean(trimmed)
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(projectRoot, absPath)
		}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("local vendor path %q does not exist: %w", absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local vendor path %q is not a directory", absPath)
	}

	return "file://" + filepath.ToSlash(absPath), nil
}
