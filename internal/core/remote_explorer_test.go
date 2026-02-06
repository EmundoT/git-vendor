package core

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
)

// ============================================================================
// ParseSmartURL Tests
// ============================================================================

func TestParseSmartURL(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name        string
		input       string
		wantURL     string
		wantRef     string
		wantPath    string
		description string
	}{
		{
			name:        "Basic GitHub URL",
			input:       "https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should extract base URL with no ref or path",
		},
		{
			name:        "GitHub URL with .git suffix",
			input:       "https://github.com/owner/repo.git",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove .git suffix",
		},
		{
			name:        "GitHub blob URL with main branch",
			input:       "https://github.com/owner/repo/blob/main/path/to/file.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/to/file.go",
			description: "Should extract repo, ref, and file path from blob URL",
		},
		{
			name:        "GitHub tree URL with branch",
			input:       "https://github.com/owner/repo/tree/dev/src/components",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "dev",
			wantPath:    "src/components",
			description: "Should extract repo, ref, and directory path from tree URL",
		},
		{
			name:        "GitHub blob URL with version tag",
			input:       "https://github.com/owner/repo/blob/v1.0.0/README.md",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "v1.0.0",
			wantPath:    "README.md",
			description: "Should handle version tags as refs",
		},
		{
			name:        "GitHub blob URL with commit hash",
			input:       "https://github.com/owner/repo/blob/abc123def456/src/main.go",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "abc123def456",
			wantPath:    "src/main.go",
			description: "Should handle commit hashes as refs",
		},
		{
			name:        "GitHub tree URL with nested path",
			input:       "https://github.com/owner/repo/tree/main/deeply/nested/path/to/dir",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "deeply/nested/path/to/dir",
			description: "Should handle deeply nested paths",
		},
		{
			name:        "URL with trailing slash",
			input:       "https://github.com/owner/repo/",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should handle trailing slash",
		},
		{
			name:        "URL with spaces (trimmed)",
			input:       "  https://github.com/owner/repo  ",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should trim whitespace",
		},
		{
			name:        "URL with backslash prefix",
			input:       "\\https://github.com/owner/repo",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantPath:    "",
			description: "Should remove leading backslash",
		},
		{
			name:        "Blob URL with file containing special characters",
			input:       "https://github.com/owner/repo/blob/main/path/file-name_v2.test.js",
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantPath:    "path/file-name_v2.test.js",
			description: "Should handle filenames with hyphens and underscores",
		},
		// Note: Branch names with slashes (e.g., feature/new-feature) are not currently supported
		// in deep link parsing due to regex limitations. Users should manually enter such refs.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotRef, gotPath := m.ParseSmartURL(tt.input)

			if gotURL != tt.wantURL {
				t.Errorf("ParseSmartURL() URL = %v, want %v\nDescription: %s", gotURL, tt.wantURL, tt.description)
			}
			if gotRef != tt.wantRef {
				t.Errorf("ParseSmartURL() Ref = %v, want %v\nDescription: %s", gotRef, tt.wantRef, tt.description)
			}
			if gotPath != tt.wantPath {
				t.Errorf("ParseSmartURL() Path = %v, want %v\nDescription: %s", gotPath, tt.wantPath, tt.description)
			}
		})
	}
}

// ============================================================================
// ListLocalDir Tests
// ============================================================================

func TestListLocalDir(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	fs.EXPECT().ReadDir("/some/path").Return([]string{"file1.go", "file2.go", "subdir/"}, nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	items, err := syncer.ListLocalDir("/some/path")

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if len(items) < 1 {
		t.Error("Expected at least 1 item from mock")
	}
}

// ============================================================================
// FetchRepoDir Tests
// ============================================================================

func TestFetchRepoDir_HappyPath(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	// Mock: Clone succeeds
	git.EXPECT().Clone("/tmp/test-12345", "https://github.com/owner/repo", gomock.Any()).Return(nil)

	// Mock: Fetch is called after clone when ref is not HEAD
	git.EXPECT().Fetch("/tmp/test-12345", 0, "main").Return(nil)

	// Mock: ListTree returns files
	git.EXPECT().ListTree("/tmp/test-12345", "main", "src").Return([]string{"file1.go", "file2.go", "subdir/"}, nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	files, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "src")

	// Verify
	assertNoError(t, err, "FetchRepoDir should succeed")
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}
}

func TestFetchRepoDir_CloneFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	// Mock: Clone fails
	git.EXPECT().Clone("/tmp/test-12345", "https://github.com/owner/repo", gomock.Any()).Return(fmt.Errorf("network timeout"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "src")

	// Verify
	assertError(t, err, "FetchRepoDir should fail when clone fails")
	if !contains(err.Error(), "network timeout") {
		t.Errorf("Expected network timeout error, got: %v", err)
	}
}

func TestFetchRepoDir_SpecificRef(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	// Mock: Clone succeeds
	git.EXPECT().Clone("/tmp/test-12345", "https://github.com/owner/repo", gomock.Any()).Return(nil)

	// Mock: Fetch called for specific ref
	git.EXPECT().Fetch("/tmp/test-12345", gomock.Any(), "v1.0.0").Return(nil)

	// Mock: ListTree returns files
	git.EXPECT().ListTree("/tmp/test-12345", "v1.0.0", "").Return([]string{"file.go"}, nil)

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "v1.0.0", "")

	// Verify
	assertNoError(t, err, "FetchRepoDir should succeed")
}

func TestFetchRepoDir_ListTreeFails(t *testing.T) {
	ctrl, git, fs, config, lock, license := setupMocks(t)
	defer ctrl.Finish()

	fs.EXPECT().CreateTemp(gomock.Any(), gomock.Any()).Return("/tmp/test-12345", nil)
	fs.EXPECT().RemoveAll("/tmp/test-12345").Return(nil)

	// Mock: Clone succeeds
	git.EXPECT().Clone("/tmp/test-12345", "https://github.com/owner/repo", gomock.Any()).Return(nil)

	// Mock: Fetch is called after clone when ref is not HEAD
	git.EXPECT().Fetch("/tmp/test-12345", 0, "main").Return(nil)

	// Mock: ListTree fails
	git.EXPECT().ListTree("/tmp/test-12345", "main", "nonexistent").Return(nil, fmt.Errorf("invalid tree object"))

	syncer := createMockSyncer(git, fs, config, lock, license)

	// Execute
	_, err := syncer.FetchRepoDir("https://github.com/owner/repo", "main", "nonexistent")

	// Verify
	assertError(t, err, "FetchRepoDir should fail when ListTree fails")
	if !contains(err.Error(), "invalid tree object") {
		t.Errorf("Expected tree object error, got: %v", err)
	}
}
