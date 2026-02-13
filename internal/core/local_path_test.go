package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// IsLocalPath Tests
// ============================================================================

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		expect bool
	}{
		// file:// scheme
		{"file:// URL", "file:///home/user/repo", true},
		{"file:// URL uppercase", "FILE:///home/user/repo", true},
		{"file:// URL Windows", "file:///C:/repos/project", true},

		// Relative paths
		{"relative dot-slash", "./sibling-repo", true},
		{"relative dot-dot-slash", "../other-repo", true},
		{"relative dot-backslash", ".\\sibling-repo", true},
		{"relative dot-dot-backslash", "..\\other-repo", true},

		// Unix absolute paths
		{"unix absolute", "/home/user/repo", true},
		{"unix root", "/", true},

		// Windows drive letters
		{"windows C drive forward", "C:/repos/project", true},
		{"windows D drive backslash", "D:\\repos\\project", true},
		{"windows lowercase drive", "c:/repos/project", true},

		// Remote URLs (should NOT match)
		{"https URL", "https://github.com/owner/repo", false},
		{"http URL", "http://github.com/owner/repo", false},
		{"ssh URL", "ssh://git@github.com/owner/repo", false},
		{"git URL", "git://github.com/owner/repo", false},
		{"SCP-style SSH", "git@github.com:owner/repo.git", false},

		// Edge cases
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"bare hostname", "github.com/owner/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLocalPath(tt.url)
			if got != tt.expect {
				t.Errorf("IsLocalPath(%q) = %v, want %v", tt.url, got, tt.expect)
			}
		})
	}
}

// ============================================================================
// ResolveLocalURL Tests
// ============================================================================

func TestResolveLocalURL_RelativePath(t *testing.T) {
	// Create a temp directory structure: project/vendor-dir + project/source-repo
	projectDir := t.TempDir()
	vendorDir := filepath.Join(projectDir, ".git-vendor")
	sourceRepo := filepath.Join(projectDir, "source-repo")

	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("mkdir vendorDir: %v", err)
	}
	if err := os.MkdirAll(sourceRepo, 0755); err != nil {
		t.Fatalf("mkdir sourceRepo: %v", err)
	}

	resolved, err := ResolveLocalURL("./source-repo", vendorDir)
	if err != nil {
		t.Fatalf("ResolveLocalURL() error: %v", err)
	}

	// Resolved URL should be a file:// URL pointing to sourceRepo
	expectedSuffix := "source-repo"
	if !strings.HasPrefix(resolved, "file://") {
		t.Errorf("resolved = %q, want file:// prefix", resolved)
	}
	if !strings.HasSuffix(resolved, expectedSuffix) {
		t.Errorf("resolved = %q, want suffix %q", resolved, expectedSuffix)
	}
}

func TestResolveLocalURL_AbsoluteFileURL(t *testing.T) {
	sourceRepo := t.TempDir()

	var fileURL string
	if runtime.GOOS == "windows" {
		fileURL = "file:///" + filepath.ToSlash(sourceRepo)
	} else {
		fileURL = "file://" + sourceRepo
	}

	resolved, err := ResolveLocalURL(fileURL, "/unused/vendor-dir")
	if err != nil {
		t.Fatalf("ResolveLocalURL() error: %v", err)
	}

	if !strings.HasPrefix(resolved, "file://") {
		t.Errorf("resolved = %q, want file:// prefix", resolved)
	}
}

func TestResolveLocalURL_NonexistentPath(t *testing.T) {
	vendorDir := t.TempDir()
	_, err := ResolveLocalURL("./does-not-exist", vendorDir)
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want 'does not exist'", err.Error())
	}
}

func TestResolveLocalURL_FileNotDirectory(t *testing.T) {
	projectDir := t.TempDir()
	vendorDir := filepath.Join(projectDir, ".git-vendor")
	os.MkdirAll(vendorDir, 0755)

	// Create a regular file (not a directory)
	regularFile := filepath.Join(projectDir, "not-a-dir.txt")
	os.WriteFile(regularFile, []byte("content"), 0644)

	_, err := ResolveLocalURL("./not-a-dir.txt", vendorDir)
	if err == nil {
		t.Fatal("expected error for non-directory path, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %q, want 'not a directory'", err.Error())
	}
}

// ============================================================================
// ValidateVendorURL file:// hint Tests
// ============================================================================

func TestValidateVendorURL_FileScheme_MentionsLocalFlag(t *testing.T) {
	err := ValidateVendorURL("file:///home/user/repo")
	if err == nil {
		t.Fatal("expected error for file:// URL, got nil")
	}
	if !strings.Contains(err.Error(), "--local") {
		t.Errorf("error = %q, want mention of --local flag", err.Error())
	}
}
