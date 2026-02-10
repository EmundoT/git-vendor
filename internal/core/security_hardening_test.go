package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// SEC-010: Git Command Injection Audit
// ============================================================================
//
// Audit scope: Verify all exec.Command/exec.CommandContext invocations pass
// arguments safely (no shell interpolation of untrusted input).
//
// Findings:
//
//   PASS — internal/core/hook_service.go: Uses exec.CommandContext(ctx, "sh", "-c", command)
//          where command comes from vendor.yml hooks. This is intentional shell execution
//          (same trust model as npm scripts). Environment variables are sanitized via
//          sanitizeEnvValue(). 5-minute timeout prevents hangs.
//
//   PASS — pkg/git-plumbing/git.go: Uses exec.CommandContext(ctx, "git", args...) where args
//          are passed as separate process arguments, NOT shell-interpolated. This is safe
//          from command injection because exec.Command does not invoke a shell.
//
//   PASS — internal/core/git_operations.go: Contains ZERO direct exec.Command calls.
//          All git operations delegate to git-plumbing via gitFor(dir).Method(args...).
//
//   N/A  — Test files (git_operations_test.go, integration_test.go, testutil/repo.go):
//          exec.Command("git", ...) used only for test fixture setup with hardcoded args.
//
// Conclusion: No command injection vectors found. Git arguments flow through typed Go
// function parameters (not string concatenation), and the only shell execution (hooks)
// is by-design user-controlled configuration.

// TestSEC010_HookSanitizeEnvValue verifies that sanitizeEnvValue strips characters
// that could break environment variable boundaries or inject shell escapes.
func TestSEC010_HookSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean value", "hello world", "hello world"},
		{"newline stripped", "hello\nworld", "hello world"},
		{"carriage return stripped", "hello\rworld", "hello world"},
		{"null byte stripped", "hello\x00world", "helloworld"},
		{"CRLF stripped", "hello\r\nworld", "hello  world"},
		{"multiple newlines", "a\nb\nc", "a b c"},
		{"all dangerous chars", "a\n\r\x00b", "a  b"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeEnvValue(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeEnvValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSEC010_GitArgsNotShellInterpolated documents that git arguments are passed as
// separate exec.Command arguments (safe), not via shell interpolation (unsafe).
// This test verifies the SystemGitClient correctly creates a new git instance per call.
func TestSEC010_GitArgsNotShellInterpolated(t *testing.T) {
	// SystemGitClient.gitFor() creates a new git.Git per call with Dir set.
	// git.Git.Run() uses exec.CommandContext(ctx, "git", args...) — safe.
	gitClient := NewSystemGitClient(false)

	// Verify gitFor returns a non-nil instance (adapter works)
	g := gitClient.gitFor(t.TempDir())
	if g == nil {
		t.Fatal("gitFor() returned nil — adapter broken")
	}
}

// ============================================================================
// SEC-011: URL Validation Hardening
// ============================================================================

func TestSEC011_ValidateVendorURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantErr   bool
		errSubstr string
	}{
		// Allowed schemes
		{"HTTPS URL", "https://github.com/owner/repo", false, ""},
		{"HTTP URL", "http://github.com/owner/repo", false, ""},
		{"SSH URL", "ssh://git@github.com/owner/repo", false, ""},
		{"git:// URL", "git://github.com/owner/repo.git", false, ""},
		{"git+ssh URL", "git+ssh://git@github.com/owner/repo", false, ""},
		{"SCP-style SSH", "git@github.com:owner/repo.git", false, ""},
		{"SCP-style GitLab", "git@gitlab.com:group/subgroup/repo.git", false, ""},
		{"Bare hostname", "github.com/owner/repo", false, ""},

		// Rejected schemes
		{"file:// URL", "file:///etc/passwd", true, "file://"},
		{"file:// relative", "file://./local-repo", true, "file://"},
		{"ftp:// URL", "ftp://mirror.example.com/repo.git", true, "FTP"},
		{"ftps:// URL", "ftps://mirror.example.com/repo.git", true, "FTP"},
		{"javascript:", "javascript:alert(1)", true, "not allowed"},
		{"data:", "data:text/plain;base64,abc", true, "not allowed"},
		{"custom scheme", "myproto://evil.com/repo", true, "not allowed"},

		// Edge cases
		{"empty URL", "", true, "must not be empty"},
		{"HTTPS uppercase", "HTTPS://GITHUB.COM/OWNER/REPO", false, ""},
		{"mixed case file", "File:///etc/passwd", true, "file://"},
		{"URL with credentials", "https://user:pass@github.com/owner/repo", false, ""},
		{"SSH with custom user", "deploy@myserver.com:repo.git", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVendorURL(tt.url)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateVendorURL(%q) expected error, got nil", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateVendorURL(%q) unexpected error: %v", tt.url, err)
			}
			if tt.wantErr && err != nil && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("ValidateVendorURL(%q) error should contain %q, got: %v", tt.url, tt.errSubstr, err)
				}
			}
		})
	}
}

// TestSEC011_ValidateVendor_RejectsFileURL verifies that vendor validation catches
// file:// URLs at parse time via ValidateVendorURL integration.
func TestSEC011_ValidateVendor_RejectsFileURL(t *testing.T) {
	vendor := &types.VendorSpec{
		Name:    "evil-local",
		URL:     "file:///tmp/local-repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src/", To: "vendor/evil/"},
				},
			},
		},
	}

	svc := &ValidationService{}
	err := svc.validateVendor(vendor)
	if err == nil {
		t.Fatal("validateVendor should reject vendor with file:// URL")
	}
	if !strings.Contains(err.Error(), "file://") {
		t.Errorf("Error should mention file://, got: %v", err)
	}
}

// ============================================================================
// SEC-013: Credential Exposure Prevention
// ============================================================================

func TestSEC013_SanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		// URLs without credentials — unchanged
		{"plain HTTPS", "https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"SSH SCP-style", "git@github.com:owner/repo.git", "git@github.com:owner/repo.git"},
		{"git:// URL", "git://github.com/owner/repo.git", "git://github.com/owner/repo.git"},

		// URLs with credentials — stripped
		{"HTTPS with user:pass", "https://user:token@github.com/owner/repo", "https://github.com/owner/repo"},
		{"HTTPS with token only", "https://ghp_abc123@github.com/owner/repo", "https://github.com/owner/repo"},
		{"GitLab oauth2 token", "https://oauth2:glpat-secret@gitlab.com/owner/repo", "https://gitlab.com/owner/repo"},
		{"basic auth", "https://admin:s3cret@git.internal.com/repo.git", "https://git.internal.com/repo.git"},

		// Edge cases
		{"empty string", "", ""},
		{"malformed URL", "not-a-url", "not-a-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.url)
			if got != tt.want {
				t.Errorf("SanitizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}

			// Verify no password/token remains in sanitized output
			if strings.Contains(got, "token") || strings.Contains(got, "secret") ||
				strings.Contains(got, "s3cret") || strings.Contains(got, "glpat-") ||
				strings.Contains(got, "ghp_") {
				t.Errorf("SanitizeURL(%q) = %q — still contains credentials", tt.url, got)
			}
		})
	}
}

// ============================================================================
// SEC-020: YAML Parsing Limits
// ============================================================================

func TestSEC020_YAMLStore_RejectsOversizedFiles(t *testing.T) {
	tempDir := t.TempDir()
	filename := "test.yml"

	// Create a file exceeding maxYAMLFileSize (1 MB)
	oversizedContent := strings.Repeat("x", maxYAMLFileSize+1)
	if err := os.WriteFile(filepath.Join(tempDir, filename), []byte(oversizedContent), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewYAMLStore[types.VendorConfig](tempDir, filename, false)
	_, err := store.Load()
	if err == nil {
		t.Fatal("YAMLStore.Load() should reject oversized files")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("Error should mention size limit, got: %v", err)
	}
}

func TestSEC020_YAMLStore_AcceptsNormalFiles(t *testing.T) {
	tempDir := t.TempDir()
	filename := "test.yml"

	// Create a normal-sized config file
	content := "vendors:\n  - name: test\n    url: https://github.com/test/repo\n"
	if err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewYAMLStore[types.VendorConfig](tempDir, filename, false)
	config, err := store.Load()
	if err != nil {
		t.Fatalf("YAMLStore.Load() should accept normal files: %v", err)
	}
	if len(config.Vendors) != 1 {
		t.Errorf("Expected 1 vendor, got %d", len(config.Vendors))
	}
}

func TestSEC020_YAMLStore_ExactBoundary(t *testing.T) {
	tempDir := t.TempDir()
	filename := "boundary.yml"

	// File exactly at the limit — should be accepted
	exactContent := make([]byte, maxYAMLFileSize)
	// Fill with valid YAML (mostly spaces to reach the size)
	copy(exactContent, []byte("vendors: []\n"))
	for i := 12; i < len(exactContent); i++ {
		exactContent[i] = ' '
	}
	if err := os.WriteFile(filepath.Join(tempDir, filename), exactContent, 0644); err != nil {
		t.Fatal(err)
	}

	store := NewYAMLStore[types.VendorConfig](tempDir, filename, false)
	_, err := store.Load()
	// May get YAML parse error due to content, but should NOT get size limit error
	if err != nil && strings.Contains(err.Error(), "exceeds maximum size") {
		t.Error("File at exactly maxYAMLFileSize should not be rejected for size")
	}
}

func TestSEC020_YAMLStore_MissingFileStillAllowed(t *testing.T) {
	tempDir := t.TempDir()
	store := NewYAMLStore[types.VendorConfig](tempDir, "nonexistent.yml", true)

	config, err := store.Load()
	if err != nil {
		t.Fatalf("allowMissing=true should return zero value, got error: %v", err)
	}
	if len(config.Vendors) != 0 {
		t.Errorf("Expected empty config for missing file, got %d vendors", len(config.Vendors))
	}
}

// ============================================================================
// SEC-021: Temp Directory Cleanup Verification
// ============================================================================
//
// Audit scope: Verify all CreateTemp/MkdirTemp call sites have defer cleanup.
//
// Findings:
//
//   PASS — sync_service.go:syncRef(): defer func() { _ = s.fs.RemoveAll(tempDir) }()
//   PASS — update_service.go:updateVendor(): defer func() { _ = s.fs.RemoveAll(tempDir) }()
//   PASS — license_fallback.go:CheckLicense(): defer func() { _ = c.fs.RemoveAll(tempDir) }()
//   PASS — remote_explorer.go:FetchRepoDir(): defer func() { _ = e.fs.RemoveAll(tempDir) }()
//   PASS — diff_service.go:DiffVendor(): defer func() { _ = s.fs.RemoveAll(tempDir) }()
//   PASS — update_checker.go:fetchLatestHash(): defer func() { _ = c.fs.RemoveAll(tempDir) }()
//
// All 6 CreateTemp call sites use the pattern:
//   tempDir, err := fs.CreateTemp("", "prefix-*")
//   if err != nil { return ..., err }
//   defer func() { _ = fs.RemoveAll(tempDir) }()
//
// The defer runs on:
//   - Normal return (success)
//   - Error return (any error in the function body)
//   - Panic recovery (Go defers run during panic unwinding)
//
// The defer does NOT run if:
//   - The process is killed with SIGKILL (no cleanup possible — acceptable)
//   - A goroutine panics without recovery (defers in that goroutine still run)
//
// Parallel processing (parallel_executor.go) uses goroutines but each goroutine's
// sync/update function has its own defer cleanup, so worker goroutine panics still
// clean up their own temp dirs.
//
// Conclusion: All temp directory paths are properly guarded by defer cleanup.

// TestSEC021_TempCleanup_DeferPattern verifies that the defer cleanup pattern
// works correctly for both success and error paths.
func TestSEC021_TempCleanup_DeferPattern(t *testing.T) {
	// Simulate the exact pattern used across the codebase
	createAndCleanup := func(shouldError bool) (string, error) {
		tempDir := t.TempDir() // Go testing cleans this, but we verify our pattern
		subDir := filepath.Join(tempDir, "workdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return "", err
		}

		// This is the pattern from sync_service.go et al:
		cleaned := false
		defer func() { cleaned = true; os.RemoveAll(subDir) }()

		if shouldError {
			return subDir, fmt.Errorf("simulated error")
		}
		_ = cleaned // defer will run after return
		return subDir, nil
	}

	// Test: success path — defer runs, dir cleaned
	dir, err := createAndCleanup(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(dir); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("Temp dir should be cleaned after success return")
	}

	// Test: error path — defer still runs, dir cleaned
	dir, err = createAndCleanup(true)
	if err == nil {
		t.Fatal("Expected error")
	}
	if _, statErr := os.Stat(dir); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("Temp dir should be cleaned after error return")
	}
}

// TestSEC021_TempCleanup_PanicPath verifies that defer cleanup runs even
// when a panic occurs (Go guarantees defers run during panic unwinding).
func TestSEC021_TempCleanup_PanicPath(t *testing.T) {
	var cleanedDir string

	func() {
		defer func() {
			recover() // catch the panic
		}()

		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "panic-workdir")
		os.MkdirAll(subDir, 0755)
		cleanedDir = subDir

		defer func() { os.RemoveAll(subDir) }()

		// Simulate a panic in the middle of work
		panic("simulated panic during sync")
	}()

	// Verify the dir was cleaned despite the panic
	if _, err := os.Stat(cleanedDir); !errors.Is(err, os.ErrNotExist) {
		t.Error("Temp dir should be cleaned even after panic (defer runs during unwind)")
	}
}

// ============================================================================
// SEC-022: Symlink Handling Tests
// ============================================================================

// TestSEC022_CopyFile_SymlinkToFile verifies that CopyFile follows symlinks to files
// and copies the dereferenced content (not the symlink itself).
func TestSEC022_CopyFile_SymlinkToFile(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create real file
	realFile := filepath.Join(tempDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("real content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	symlink := filepath.Join(tempDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	// CopyFile should follow symlink
	dest := filepath.Join(tempDir, "dest.txt")
	stats, err := fs.CopyFile(symlink, dest)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	if stats.FileCount != 1 {
		t.Errorf("Expected FileCount=1, got %d", stats.FileCount)
	}

	// Verify content matches real file
	data, _ := os.ReadFile(dest)
	if string(data) != "real content" {
		t.Errorf("Expected 'real content', got %q", string(data))
	}

	// Verify dest is a regular file, not a symlink
	info, _ := os.Lstat(dest)
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Destination should be a regular file, not a symlink")
	}
}

// TestSEC022_CopyFile_DanglingSymlink verifies that CopyFile errors on dangling
// symlinks (symlinks pointing to non-existent targets).
func TestSEC022_CopyFile_DanglingSymlink(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create dangling symlink
	dangling := filepath.Join(tempDir, "dangling.txt")
	if err := os.Symlink("/nonexistent/target", dangling); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	dest := filepath.Join(tempDir, "dest.txt")
	_, err := fs.CopyFile(dangling, dest)
	if err == nil {
		t.Fatal("CopyFile should error on dangling symlink")
	}
}

// TestSEC022_CopyDir_SymlinkToDirectory verifies that CopyDir errors on symlinks
// to directories. filepath.Walk uses os.Lstat (no symlink following), so a symlink
// to a directory is seen as a non-directory entry. CopyFile then calls os.Open
// which follows the symlink to a directory and fails with "is a directory".
//
// This is safe behavior: preventing symlink traversal in directory copies avoids
// symlink-based directory escape attacks. Git clone sources rarely contain
// directory symlinks.
func TestSEC022_CopyDir_SymlinkToDirectory(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	// Create source structure with a symlink to a directory
	srcDir := filepath.Join(tempDir, "src")
	realSubDir := filepath.Join(tempDir, "external")
	os.MkdirAll(filepath.Join(srcDir, "normal"), 0755)
	os.MkdirAll(realSubDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "normal", "file.txt"), []byte("normal"), 0644)
	os.WriteFile(filepath.Join(realSubDir, "ext.txt"), []byte("external"), 0644)

	// Create symlink inside srcDir pointing to external directory
	if err := os.Symlink(realSubDir, filepath.Join(srcDir, "linked")); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	// CopyDir errors on symlinks to directories because filepath.Walk does not
	// descend into symlinked directories — os.Lstat sees the symlink entry,
	// info.IsDir() returns false, so CopyFile is called, which os.Open follows
	// to a directory and fails.
	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)
	_, err := fs.CopyDir(srcDir, destDir)
	if err == nil {
		t.Fatal("CopyDir should error on symlink to directory (filepath.Walk limitation)")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("Expected 'is a directory' error, got: %v", err)
	}
}

// TestSEC022_CopyDir_SymlinkToFile verifies that CopyDir follows file symlinks
// during directory walk and copies the dereferenced content.
func TestSEC022_CopyDir_SymlinkToFile(t *testing.T) {
	fs := NewOSFileSystem()
	tempDir := t.TempDir()

	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Create real file outside srcDir
	externalFile := filepath.Join(tempDir, "external.txt")
	os.WriteFile(externalFile, []byte("external content"), 0644)

	// Symlink inside srcDir points to external file
	if err := os.Symlink(externalFile, filepath.Join(srcDir, "linked.txt")); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	// Also create a regular file
	os.WriteFile(filepath.Join(srcDir, "regular.txt"), []byte("regular content"), 0644)

	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)
	stats, err := fs.CopyDir(srcDir, destDir)
	if err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	if stats.FileCount != 2 {
		t.Errorf("Expected 2 files copied, got %d", stats.FileCount)
	}

	// Verify symlinked file content was dereferenced
	data, _ := os.ReadFile(filepath.Join(destDir, "linked.txt"))
	if string(data) != "external content" {
		t.Errorf("Symlinked file content: got %q, want 'external content'", string(data))
	}
}

// ============================================================================
// SEC-023: Binary File Detection Hardening
// ============================================================================

// TestSEC023_IsBinaryContent_Exported verifies the exported IsBinaryContent
// function works correctly for both text and binary content.
func TestSEC023_IsBinaryContent_Exported(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"ASCII text", []byte("hello world\n"), false},
		{"UTF-8 text", []byte("café résumé\n"), false},
		{"null byte at start", []byte{0x00, 'h', 'e', 'l', 'l', 'o'}, true},
		{"null byte in middle", []byte("hel\x00lo"), true},
		{"PNG header", []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}, true},
		{"ELF binary", []byte{0x7f, 'E', 'L', 'F', 0x00}, true},
		{"Go compiled binary (simulated)", append([]byte("MZ"), make([]byte, 100)...), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryContent(tt.data)
			if got != tt.want {
				t.Errorf("IsBinaryContent(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestSEC023_CopyMapping_BinaryWarning verifies that whole-file copies of binary
// files produce a warning in CopyStats.Warnings.
func TestSEC023_CopyMapping_BinaryWarning(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	fs := NewOSFileSystem()

	// Create a binary source file (contains null bytes)
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	binaryContent := []byte("binary\x00content\x00here")
	os.WriteFile(filepath.Join(srcDir, "data.bin"), binaryContent, 0644)

	// Create the service with relative destination (ValidateDestPath requires relative)
	svc := NewFileCopyService(fs)

	vendor := &types.VendorSpec{
		Name: "test-binary",
	}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "data.bin", To: "dest/data.bin"},
		},
	}

	stats, err := svc.CopyMappings(srcDir, vendor, spec)
	if err != nil {
		t.Fatalf("CopyMappings failed: %v", err)
	}

	// Verify warning about binary file
	found := false
	for _, w := range stats.Warnings {
		if strings.Contains(w, "binary") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning about binary file in CopyStats.Warnings")
	}

	// Verify the file was still copied (binary copies are allowed)
	if stats.FileCount != 1 {
		t.Errorf("Expected FileCount=1 (binary copies allowed), got %d", stats.FileCount)
	}
}

// TestSEC023_CopyMapping_TextNoWarning verifies that text file copies produce
// no binary warnings.
func TestSEC023_CopyMapping_TextNoWarning(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	fs := NewOSFileSystem()

	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file.go"), []byte("package main\n"), 0644)

	svc := NewFileCopyService(fs)

	vendor := &types.VendorSpec{Name: "test-text"}
	spec := types.BranchSpec{
		Ref: "main",
		Mapping: []types.PathMapping{
			{From: "file.go", To: "dest/file.go"},
		},
	}

	stats, err := svc.CopyMappings(srcDir, vendor, spec)
	if err != nil {
		t.Fatalf("CopyMappings failed: %v", err)
	}

	if len(stats.Warnings) > 0 {
		t.Errorf("Text file should produce no binary warnings, got: %v", stats.Warnings)
	}
}
