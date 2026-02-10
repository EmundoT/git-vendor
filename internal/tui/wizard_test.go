package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/core"
	"github.com/EmundoT/git-vendor/internal/types"
)

// mockVendorManager implements VendorManager for testing ShowConflictWarnings.
type mockVendorManager struct {
	conflicts   []types.PathConflict
	conflictErr error
	lockHashes  map[string]string
}

func (m *mockVendorManager) ParseSmartURL(_ string) (string, string, string) {
	return "", "", ""
}

func (m *mockVendorManager) FetchRepoDir(_ context.Context, _, _, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockVendorManager) ListLocalDir(_ string) ([]string, error) {
	return nil, nil
}

func (m *mockVendorManager) GetLockHash(name, ref string) string {
	key := name + ":" + ref
	if m.lockHashes != nil {
		return m.lockHashes[key]
	}
	return ""
}

func (m *mockVendorManager) DetectConflicts() ([]types.PathConflict, error) {
	return m.conflicts, m.conflictErr
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world", 8, "hello..."},
		{"very short max", "abcdef", 4, "a..."},
		{"single char over", "abcdef", 5, "ab..."},
		{"empty string", "", 5, ""},
		{"min truncation length", "abcd", 3, "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// --- isValidGitURL ---

func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"https github", "https://github.com/owner/repo", true},
		{"http gitlab", "http://gitlab.com/owner/repo", true},
		{"git protocol", "git://github.com/owner/repo.git", true},
		{"git@ SSH", "git@github.com:owner/repo.git", true},
		{"ssh protocol", "ssh://git@github.com/owner/repo.git", true},
		{"bitbucket https", "https://bitbucket.org/owner/repo", true},
		{"self-hosted https", "https://git.example.com/org/project", true},
		{"bare domain with path", "github.com/owner/repo", true},
		{"trimmed spaces", "  https://github.com/owner/repo  ", true},
		{"empty string", "", false},
		{"no slash bare", "github.com", false},
		{"random word", "notaurl", false},
		{"no dot", "localhost/repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGitURL(tt.url)
			if got != tt.want {
				t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// --- validateURL ---

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"empty", "", true, "URL cannot be empty"},
		{"valid https", "https://github.com/owner/repo", false, ""},
		{"valid with whitespace", "  https://github.com/owner/repo  ", false, ""},
		{"invalid format", "notaurl", true, "invalid git URL format"},
		{"valid git@", "git@github.com:owner/repo.git", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateURL(%q) expected error, got nil", tt.input)
				} else if err.Error() != tt.errMsg {
					t.Errorf("validateURL(%q) error = %q, want %q", tt.input, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("validateURL(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

// --- formatBranchLabel ---

func TestFormatBranchLabel(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		count    int
		lockHash string
		want     string
	}{
		{"no paths not synced", "main", 0, "", "main (no paths, not synced)"},
		{"one path not synced", "main", 1, "", "main (1 path, not synced)"},
		{"multiple paths not synced", "v1.0", 3, "", "v1.0 (3 paths, not synced)"},
		{"locked with hash", "main", 2, "abc1234def5678", "main (2 paths, locked: abc1234)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBranchLabel(tt.ref, tt.count, tt.lockHash)
			if got != tt.want {
				t.Errorf("formatBranchLabel(%q, %d, %q) = %q, want %q",
					tt.ref, tt.count, tt.lockHash, got, tt.want)
			}
		})
	}
}

// --- formatPathCount ---

func TestFormatPathCount(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "no paths"},
		{1, "1 path"},
		{2, "2 paths"},
		{10, "10 paths"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("count_%d", tt.count), func(t *testing.T) {
			got := formatPathCount(tt.count)
			if got != tt.want {
				t.Errorf("formatPathCount(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

// --- formatMappingLabel ---

func TestFormatMappingLabel(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want string
	}{
		{"with destination", "src/main.go", "vendor/main.go",
			fmt.Sprintf("%-20s → %s", "src/main.go", "vendor/main.go")},
		{"empty destination shows auto", "src/main.go", "",
			fmt.Sprintf("%-20s → %s", "src/main.go", "(auto)")},
		{"long from truncated", "very/long/path/to/some/deeply/nested/file.go", "out.go",
			fmt.Sprintf("%-20s → %s", "very/long/path/to...", "out.go")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMappingLabel(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("formatMappingLabel(%q, %q) = %q, want %q",
					tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// --- buildBreadcrumb ---

func TestBuildBreadcrumb(t *testing.T) {
	tests := []struct {
		name       string
		repoName   string
		ref        string
		currentDir string
		want       string
	}{
		{"root dir", "my-repo", "main", "", "my-repo @ main"},
		{"one level deep", "my-repo", "main", "src", "my-repo @ main / src"},
		{"nested dir", "my-repo", "v1.0", "src/components/ui",
			"my-repo @ v1.0 / src / components / ui"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBreadcrumb(tt.repoName, tt.ref, tt.currentDir)
			if got != tt.want {
				t.Errorf("buildBreadcrumb(%q, %q, %q) = %q, want %q",
					tt.repoName, tt.ref, tt.currentDir, got, tt.want)
			}
		})
	}
}

// --- navigateUp ---

func TestNavigateUp(t *testing.T) {
	tests := []struct {
		name       string
		currentDir string
		want       string
	}{
		{"single dir to root", "src", ""},
		{"nested to parent", "src/components", "src"},
		{"deep to parent", "a/b/c/d", "a/b/c"},
		{"trailing slash stripped", "src/components/", "src"},
		{"root stays root", ".", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := navigateUp(tt.currentDir)
			if got != tt.want {
				t.Errorf("navigateUp(%q) = %q, want %q", tt.currentDir, got, tt.want)
			}
		})
	}
}

// --- resolveRemoteSelection ---

func TestResolveRemoteSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		currentDir string
		wantDir    string
		wantFile   string
		wantIsFile bool
	}{
		{"dir from root", "src/", "", "src", "", false},
		{"dir from subdir", "components/", "src", "src/components", "", false},
		{"file from root", "README.md", "", "", "README.md", true},
		{"file from subdir", "main.go", "src", "", "src/main.go", true},
		{"nested dir from root", "pkg/", "", "pkg", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotFile, gotIsFile := resolveRemoteSelection(tt.selection, tt.currentDir)
			if gotDir != tt.wantDir {
				t.Errorf("resolveRemoteSelection(%q, %q) dir = %q, want %q",
					tt.selection, tt.currentDir, gotDir, tt.wantDir)
			}
			if gotFile != tt.wantFile {
				t.Errorf("resolveRemoteSelection(%q, %q) file = %q, want %q",
					tt.selection, tt.currentDir, gotFile, tt.wantFile)
			}
			if gotIsFile != tt.wantIsFile {
				t.Errorf("resolveRemoteSelection(%q, %q) isFile = %v, want %v",
					tt.selection, tt.currentDir, gotIsFile, tt.wantIsFile)
			}
		})
	}
}

// --- autoNameFromPath ---

func TestAutoNameFromPath(t *testing.T) {
	tests := []struct {
		name     string
		fromPath string
		want     string
	}{
		{"simple file", "src/main.go", "main.go"},
		{"directory path", "src/components", "components"},
		{"root path dot", ".", "(repository root)"},
		{"root path slash", "/", "(repository root)"},
		{"empty path", "", "(repository root)"},
		{"with position specifier", "src/config.go:L5-L20", "config.go"},
		{"with column position", "api/types.go:L1C5:L10C30", "types.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoNameFromPath(tt.fromPath)
			if got != tt.want {
				t.Errorf("autoNameFromPath(%q) = %q, want %q", tt.fromPath, got, tt.want)
			}
		})
	}
}

// --- itemLabel ---

func TestItemLabel(t *testing.T) {
	tests := []struct {
		name string
		item string
		want string
	}{
		{"directory", "src/", "📂 src/"},
		{"file", "main.go", "📄 main.go"},
		{"nested dir", "components/", "📂 components/"},
		{"dotfile", ".gitignore", "📄 .gitignore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemLabel(tt.item)
			if got != tt.want {
				t.Errorf("itemLabel(%q) = %q, want %q", tt.item, got, tt.want)
			}
		})
	}
}

// --- repoNameFromURL ---

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https github", "https://github.com/owner/repo", "repo"},
		{"with .git suffix", "https://github.com/owner/repo.git", "repo"},
		{"gitlab", "https://gitlab.com/group/project", "project"},
		{"plain name", "my-repo", "my-repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoNameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --- Print functions (capture stdout) ---

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintError(t *testing.T) {
	output := captureStdout(func() {
		PrintError("Failed", "something went wrong")
	})
	if !strings.Contains(output, "Failed") {
		t.Errorf("PrintError output missing title, got: %q", output)
	}
	if !strings.Contains(output, "something went wrong") {
		t.Errorf("PrintError output missing message, got: %q", output)
	}
}

func TestPrintSuccess(t *testing.T) {
	output := captureStdout(func() {
		PrintSuccess("operation completed")
	})
	if !strings.Contains(output, "operation completed") {
		t.Errorf("PrintSuccess output missing message, got: %q", output)
	}
}

func TestPrintInfo(t *testing.T) {
	output := captureStdout(func() {
		PrintInfo("informational message")
	})
	if !strings.Contains(output, "informational message") {
		t.Errorf("PrintInfo output missing message, got: %q", output)
	}
}

func TestPrintWarning(t *testing.T) {
	output := captureStdout(func() {
		PrintWarning("Caution", "be careful")
	})
	if !strings.Contains(output, "Caution") {
		t.Errorf("PrintWarning output missing title, got: %q", output)
	}
	if !strings.Contains(output, "be careful") {
		t.Errorf("PrintWarning output missing message, got: %q", output)
	}
}

func TestStyleTitle(t *testing.T) {
	result := StyleTitle("My Title")
	if !strings.Contains(result, "My Title") {
		t.Errorf("StyleTitle result missing text, got: %q", result)
	}
}

func TestPrintComplianceSuccess(t *testing.T) {
	output := captureStdout(func() {
		PrintComplianceSuccess("MIT")
	})
	if !strings.Contains(output, "MIT") {
		t.Errorf("PrintComplianceSuccess output missing license, got: %q", output)
	}
	if !strings.Contains(output, "License Verified") {
		t.Errorf("PrintComplianceSuccess output missing verification text, got: %q", output)
	}
}

func TestPrintHelp(t *testing.T) {
	output := captureStdout(func() {
		PrintHelp()
	})
	// Verify key sections are present
	checks := []string{
		"git-vendor",
		"Commands:",
		"init",
		"add",
		"edit",
		"remove",
		"list",
		"sync",
		"update",
		"validate",
		"verify",
		"scan",
		"status",
		"check-updates",
		"diff",
		"watch",
		"completion",
		"Examples:",
		"Navigation:",
		"--dry-run",
		"--parallel",
		"--verbose",
		"--format=",
		"--fail-on",
	}
	for _, keyword := range checks {
		if !strings.Contains(output, keyword) {
			t.Errorf("PrintHelp output missing %q", keyword)
		}
	}
}

// --- ShowConflictWarnings ---

func TestShowConflictWarnings_NoConflicts(t *testing.T) {
	mgr := &mockVendorManager{conflicts: nil}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if output != "" {
		t.Errorf("ShowConflictWarnings with no conflicts should produce no output, got: %q", output)
	}
}

func TestShowConflictWarnings_WithConflicts(t *testing.T) {
	mgr := &mockVendorManager{
		conflicts: []types.PathConflict{
			{
				Path:    "shared/utils.go",
				Vendor1: "my-vendor",
				Vendor2: "other-vendor",
			},
		},
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if !strings.Contains(output, "shared/utils.go") {
		t.Errorf("ShowConflictWarnings output missing conflict path, got: %q", output)
	}
	if !strings.Contains(output, "other-vendor") {
		t.Errorf("ShowConflictWarnings output missing other vendor name, got: %q", output)
	}
	if !strings.Contains(output, "Conflicts Detected") {
		t.Errorf("ShowConflictWarnings output missing warning title, got: %q", output)
	}
}

func TestShowConflictWarnings_VendorIsVendor2(t *testing.T) {
	mgr := &mockVendorManager{
		conflicts: []types.PathConflict{
			{
				Path:    "shared/config.go",
				Vendor1: "other-vendor",
				Vendor2: "my-vendor",
			},
		},
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if !strings.Contains(output, "other-vendor") {
		t.Errorf("ShowConflictWarnings should show Vendor1 when target is Vendor2, got: %q", output)
	}
}

func TestShowConflictWarnings_ErrorSilent(t *testing.T) {
	mgr := &mockVendorManager{
		conflictErr: fmt.Errorf("detection failed"),
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if output != "" {
		t.Errorf("ShowConflictWarnings should silently skip on error, got: %q", output)
	}
}

func TestShowConflictWarnings_UnrelatedConflictsIgnored(t *testing.T) {
	mgr := &mockVendorManager{
		conflicts: []types.PathConflict{
			{
				Path:    "unrelated/file.go",
				Vendor1: "vendor-a",
				Vendor2: "vendor-b",
			},
		},
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if output != "" {
		t.Errorf("ShowConflictWarnings should not show unrelated conflicts, got: %q", output)
	}
}

func TestShowConflictWarnings_MultipleConflicts(t *testing.T) {
	mgr := &mockVendorManager{
		conflicts: []types.PathConflict{
			{Path: "a.go", Vendor1: "my-vendor", Vendor2: "other1"},
			{Path: "b.go", Vendor1: "other2", Vendor2: "my-vendor"},
			{Path: "c.go", Vendor1: "unrelated1", Vendor2: "unrelated2"},
		},
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	if !strings.Contains(output, "a.go") {
		t.Errorf("ShowConflictWarnings should show a.go conflict, got: %q", output)
	}
	if !strings.Contains(output, "b.go") {
		t.Errorf("ShowConflictWarnings should show b.go conflict, got: %q", output)
	}
	if strings.Contains(output, "c.go") {
		t.Errorf("ShowConflictWarnings should not show c.go (unrelated), got: %q", output)
	}
	// Should mention "2 conflicts"
	if !strings.Contains(output, "2 conflicts") {
		t.Errorf("ShowConflictWarnings should say '2 conflicts', got: %q", output)
	}
}

// --- resolveLocalSelection ---

func TestResolveLocalSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		currentDir string
		wantDir    string
		wantFile   string
		wantIsFile bool
	}{
		{"dir from current", "src/", ".", "src", "", false},
		{"file from current", "main.go", ".", "", "main.go", true},
		{"dir from subdir", "components/", "src", "src/components", "", false},
		{"file from subdir", "index.ts", "src", "", "src/index.ts", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotFile, gotIsFile := resolveLocalSelection(tt.selection, tt.currentDir)
			if gotDir != tt.wantDir {
				t.Errorf("resolveLocalSelection(%q, %q) dir = %q, want %q",
					tt.selection, tt.currentDir, gotDir, tt.wantDir)
			}
			if gotFile != tt.wantFile {
				t.Errorf("resolveLocalSelection(%q, %q) file = %q, want %q",
					tt.selection, tt.currentDir, gotFile, tt.wantFile)
			}
			if gotIsFile != tt.wantIsFile {
				t.Errorf("resolveLocalSelection(%q, %q) isFile = %v, want %v",
					tt.selection, tt.currentDir, gotIsFile, tt.wantIsFile)
			}
		})
	}
}

// --- deleteMapping ---

func TestDeleteMapping(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "a.go", To: "out/a.go"},
		{From: "b.go", To: "out/b.go"},
		{From: "c.go", To: "out/c.go"},
	}

	result := deleteMapping(mappings, 1)
	if len(result) != 2 {
		t.Fatalf("deleteMapping: expected 2 mappings, got %d", len(result))
	}
	if result[0].From != "a.go" {
		t.Errorf("deleteMapping: first element = %q, want %q", result[0].From, "a.go")
	}
	if result[1].From != "c.go" {
		t.Errorf("deleteMapping: second element = %q, want %q", result[1].From, "c.go")
	}
}

func TestDeleteMapping_First(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "a.go", To: "out/a.go"},
		{From: "b.go", To: "out/b.go"},
	}
	result := deleteMapping(mappings, 0)
	if len(result) != 1 || result[0].From != "b.go" {
		t.Errorf("deleteMapping first: got %v", result)
	}
}

func TestDeleteMapping_Last(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "a.go", To: "out/a.go"},
		{From: "b.go", To: "out/b.go"},
	}
	result := deleteMapping(mappings, 1)
	if len(result) != 1 || result[0].From != "a.go" {
		t.Errorf("deleteMapping last: got %v", result)
	}
}

// --- inferVendorName ---

func TestInferVendorName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"github url", "https://github.com/owner/my-lib", "my-lib"},
		{"with .git", "https://github.com/owner/my-lib.git", "my-lib"},
		{"gitlab nested", "https://gitlab.com/group/sub/project", "project"},
		{"plain name", "repo-name", "repo-name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferVendorName(tt.url)
			if got != tt.want {
				t.Errorf("inferVendorName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --- resolveRef ---

func TestResolveRef(t *testing.T) {
	tests := []struct {
		name       string
		smartRef   string
		defaultRef string
		want       string
	}{
		{"smart ref present", "v1.0", "main", "v1.0"},
		{"smart ref empty uses default", "", "main", "main"},
		{"smart ref empty uses develop", "", "develop", "develop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRef(tt.smartRef, tt.defaultRef)
			if got != tt.want {
				t.Errorf("resolveRef(%q, %q) = %q, want %q",
					tt.smartRef, tt.defaultRef, got, tt.want)
			}
		})
	}
}

// --- selectCurrentLabel ---

func TestSelectCurrentLabel(t *testing.T) {
	tests := []struct {
		name       string
		currentDir string
		want       string
	}{
		{"root empty", "", "✔ Select Root"},
		{"root dot", ".", "✔ Select Root"},
		{"subdir", "src", "✔ Select '/" + "src'"},
		{"nested", "src/components", "✔ Select '/" + "src/components'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectCurrentLabel(tt.currentDir)
			if got != tt.want {
				t.Errorf("selectCurrentLabel(%q) = %q, want %q", tt.currentDir, got, tt.want)
			}
		})
	}
}

// --- autoTargetDescription ---

func TestAutoTargetDescription(t *testing.T) {
	tests := []struct {
		name     string
		fromPath string
		wantSub  string
	}{
		{"simple file", "src/main.go", "main.go"},
		{"with position", "api/types.go:L5-L20", "types.go"},
		{"root path", ".", "(repository root)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoTargetDescription(tt.fromPath)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("autoTargetDescription(%q) missing %q, got: %q", tt.fromPath, tt.wantSub, got)
			}
			if !strings.Contains(got, "Leave empty for automatic naming") {
				t.Errorf("autoTargetDescription(%q) missing boilerplate, got: %q", tt.fromPath, got)
			}
		})
	}
}

// --- ShowConflictWarnings single conflict label ---

func TestShowConflictWarnings_SingleConflict(t *testing.T) {
	mgr := &mockVendorManager{
		conflicts: []types.PathConflict{
			{Path: "a.go", Vendor1: "my-vendor", Vendor2: "other"},
		},
	}
	output := captureStdout(func() {
		ShowConflictWarnings(mgr, "my-vendor")
	})
	// Should say "1 conflict" (singular)
	if !strings.Contains(output, "1 conflict") {
		t.Errorf("ShowConflictWarnings singular: missing '1 conflict', got: %q", output)
	}
}

// --- newBaseSpec ---

func TestNewBaseSpec(t *testing.T) {
	spec := newBaseSpec("my-lib", "https://github.com/owner/lib", "v1.0")
	if spec.Name != "my-lib" {
		t.Errorf("Name = %q, want %q", spec.Name, "my-lib")
	}
	if spec.URL != "https://github.com/owner/lib" {
		t.Errorf("URL = %q, want %q", spec.URL, "https://github.com/owner/lib")
	}
	if len(spec.Specs) != 1 {
		t.Fatalf("Specs len = %d, want 1", len(spec.Specs))
	}
	if spec.Specs[0].Ref != "v1.0" {
		t.Errorf("Ref = %q, want %q", spec.Specs[0].Ref, "v1.0")
	}
	if len(spec.Specs[0].Mapping) != 0 {
		t.Errorf("Mapping should be empty, got %d", len(spec.Specs[0].Mapping))
	}
}

// --- deepLinkDescription ---

func TestDeepLinkDescription(t *testing.T) {
	desc := deepLinkDescription("src/utils.go")
	if !strings.Contains(desc, "utils.go") {
		t.Errorf("deepLinkDescription missing file name, got: %q", desc)
	}
	if !strings.Contains(desc, "Leave empty") {
		t.Errorf("deepLinkDescription missing prompt text, got: %q", desc)
	}
	// Should NOT contain position syntax hint (unlike autoTargetDescription)
	if strings.Contains(desc, ":L5-L10") {
		t.Errorf("deepLinkDescription should not mention position syntax, got: %q", desc)
	}
}

func TestDeepLinkDescription_Root(t *testing.T) {
	desc := deepLinkDescription(".")
	if !strings.Contains(desc, "(repository root)") {
		t.Errorf("deepLinkDescription root missing '(repository root)', got: %q", desc)
	}
}

// --- addMappingToFirstSpec ---

func TestAddMappingToFirstSpec(t *testing.T) {
	spec := newBaseSpec("test", "https://example.com/repo", "main")
	addMappingToFirstSpec(&spec, "src/file.go", "vendor/file.go")

	if len(spec.Specs[0].Mapping) != 1 {
		t.Fatalf("Mapping len = %d, want 1", len(spec.Specs[0].Mapping))
	}
	if spec.Specs[0].Mapping[0].From != "src/file.go" {
		t.Errorf("From = %q, want %q", spec.Specs[0].Mapping[0].From, "src/file.go")
	}
	if spec.Specs[0].Mapping[0].To != "vendor/file.go" {
		t.Errorf("To = %q, want %q", spec.Specs[0].Mapping[0].To, "vendor/file.go")
	}
}

func TestAddMappingToFirstSpec_Multiple(t *testing.T) {
	spec := newBaseSpec("test", "https://example.com/repo", "main")
	addMappingToFirstSpec(&spec, "a.go", "out/a.go")
	addMappingToFirstSpec(&spec, "b.go", "out/b.go")

	if len(spec.Specs[0].Mapping) != 2 {
		t.Fatalf("Mapping len = %d, want 2", len(spec.Specs[0].Mapping))
	}
}

// --- isExistingVendor ---

func TestIsExistingVendor_Found(t *testing.T) {
	vendors := map[string]types.VendorSpec{
		"https://github.com/owner/repo": {Name: "repo", URL: "https://github.com/owner/repo"},
	}
	spec, exists := isExistingVendor("https://github.com/owner/repo", vendors)
	if !exists {
		t.Error("expected vendor to be found")
	}
	if spec.Name != "repo" {
		t.Errorf("Name = %q, want %q", spec.Name, "repo")
	}
}

func TestIsExistingVendor_NotFound(t *testing.T) {
	vendors := map[string]types.VendorSpec{
		"https://github.com/owner/repo": {Name: "repo"},
	}
	_, exists := isExistingVendor("https://github.com/owner/other", vendors)
	if exists {
		t.Error("expected vendor not to be found")
	}
}

func TestIsExistingVendor_EmptyMap(t *testing.T) {
	_, exists := isExistingVendor("https://github.com/owner/repo", map[string]types.VendorSpec{})
	if exists {
		t.Error("expected vendor not to be found in empty map")
	}
}

// --- buildMappingOptionsLabels ---

func TestBuildMappingOptionsLabels(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "src/main.go", To: "vendor/main.go"},
		{From: "src/utils.go", To: ""},
		{From: "very/long/path/to/some/deeply/nested/file.go", To: "out.go"},
	}
	labels := buildMappingOptionsLabels(mappings)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if !strings.Contains(labels[0], "vendor/main.go") {
		t.Errorf("label[0] missing destination, got: %q", labels[0])
	}
	if !strings.Contains(labels[1], "(auto)") {
		t.Errorf("label[1] missing (auto), got: %q", labels[1])
	}
	if !strings.Contains(labels[2], "...") {
		t.Errorf("label[2] should be truncated, got: %q", labels[2])
	}
}

func TestBuildMappingOptionsLabels_Empty(t *testing.T) {
	labels := buildMappingOptionsLabels(nil)
	if len(labels) != 0 {
		t.Errorf("expected 0 labels for nil, got %d", len(labels))
	}
}

// --- buildBranchOptionsLabels ---

func TestBuildBranchOptionsLabels(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{{From: "a", To: "b"}}},
		{Ref: "v1.0", Mapping: []types.PathMapping{{From: "a", To: "b"}, {From: "c", To: "d"}}},
		{Ref: "develop", Mapping: nil},
	}
	getLockHash := func(_, ref string) string {
		if ref == "main" {
			return "abc1234567890"
		}
		return ""
	}
	labels := buildBranchOptionsLabels(specs, "my-vendor", getLockHash)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if !strings.Contains(labels[0], "locked:") {
		t.Errorf("label[0] should be locked, got: %q", labels[0])
	}
	if !strings.Contains(labels[0], "1 path") {
		t.Errorf("label[0] missing '1 path', got: %q", labels[0])
	}
	if !strings.Contains(labels[1], "not synced") {
		t.Errorf("label[1] should be not synced, got: %q", labels[1])
	}
	if !strings.Contains(labels[1], "2 paths") {
		t.Errorf("label[1] missing '2 paths', got: %q", labels[1])
	}
	if !strings.Contains(labels[2], "no paths") {
		t.Errorf("label[2] missing 'no paths', got: %q", labels[2])
	}
}

// --- buildItemLabels ---

func TestBuildItemLabels(t *testing.T) {
	items := []string{"src/", "README.md", "pkg/", ".gitignore"}
	labels := buildItemLabels(items)
	if len(labels) != 4 {
		t.Fatalf("expected 4 labels, got %d", len(labels))
	}
	if !strings.HasPrefix(labels[0], "📂") {
		t.Errorf("label[0] for dir should have folder icon, got: %q", labels[0])
	}
	if !strings.HasPrefix(labels[1], "📄") {
		t.Errorf("label[1] for file should have file icon, got: %q", labels[1])
	}
}

func TestBuildItemLabels_Empty(t *testing.T) {
	labels := buildItemLabels(nil)
	if len(labels) != 0 {
		t.Errorf("expected 0 labels for nil, got %d", len(labels))
	}
}

// --- hasLocalParent ---

func TestHasLocalParent(t *testing.T) {
	if hasLocalParent(".") {
		t.Error("root '.' should not have parent")
	}
	if !hasLocalParent("src") {
		t.Error("'src' should have parent")
	}
	if !hasLocalParent("src/components") {
		t.Error("'src/components' should have parent")
	}
}

// --- navigateLocalUp ---

func TestNavigateLocalUp(t *testing.T) {
	tests := []struct {
		name       string
		currentDir string
		want       string
	}{
		{"one level", "src", "."},
		{"nested", "src/components", "src"},
		{"root", ".", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := navigateLocalUp(tt.currentDir)
			if got != tt.want {
				t.Errorf("navigateLocalUp(%q) = %q, want %q", tt.currentDir, got, tt.want)
			}
		})
	}
}

// --- filterConflictsForVendor ---

func TestFilterConflictsForVendor(t *testing.T) {
	conflicts := []types.PathConflict{
		{Path: "a.go", Vendor1: "v1", Vendor2: "v2"},
		{Path: "b.go", Vendor1: "v2", Vendor2: "v3"},
		{Path: "c.go", Vendor1: "v1", Vendor2: "v3"},
	}
	result := filterConflictsForVendor(conflicts, "v2")
	if len(result) != 2 {
		t.Fatalf("expected 2 conflicts for v2, got %d", len(result))
	}
	if result[0].Path != "a.go" || result[1].Path != "b.go" {
		t.Errorf("unexpected conflict paths: %v", result)
	}
}

func TestFilterConflictsForVendor_None(t *testing.T) {
	conflicts := []types.PathConflict{
		{Path: "a.go", Vendor1: "v1", Vendor2: "v2"},
	}
	result := filterConflictsForVendor(conflicts, "v3")
	if len(result) != 0 {
		t.Errorf("expected 0 conflicts for v3, got %d", len(result))
	}
}

// --- otherVendorInConflict ---

func TestOtherVendorInConflict(t *testing.T) {
	c := &types.PathConflict{Path: "x.go", Vendor1: "alpha", Vendor2: "beta"}
	if got := otherVendorInConflict(c, "alpha"); got != "beta" {
		t.Errorf("otherVendorInConflict for alpha = %q, want beta", got)
	}
	if got := otherVendorInConflict(c, "beta"); got != "alpha" {
		t.Errorf("otherVendorInConflict for beta = %q, want alpha", got)
	}
	// When vendorName matches neither (edge case), returns Vendor2
	if got := otherVendorInConflict(c, "gamma"); got != "beta" {
		t.Errorf("otherVendorInConflict for gamma = %q, want beta", got)
	}
}

// --- selectLocalLabel ---

func TestSelectLocalLabel(t *testing.T) {
	if got := selectLocalLabel("."); got != "Local: ." {
		t.Errorf("selectLocalLabel(\".\") = %q, want %q", got, "Local: .")
	}
	if got := selectLocalLabel("src/components"); got != "Local: src/components" {
		t.Errorf("selectLocalLabel = %q, want %q", got, "Local: src/components")
	}
}

// --- isRootSmartPath ---

func TestIsRootSmartPath(t *testing.T) {
	if isRootSmartPath("") {
		t.Error("empty string should not be a smart path")
	}
	if !isRootSmartPath("src/file.go") {
		t.Error("non-empty string should be a smart path")
	}
}

// --- formatConflictSummary ---

func TestFormatConflictSummary(t *testing.T) {
	if got := formatConflictSummary(0); got != "" {
		t.Errorf("formatConflictSummary(0) = %q, want empty", got)
	}
	got := formatConflictSummary(3)
	if !strings.Contains(got, "3") {
		t.Errorf("formatConflictSummary(3) missing count, got: %q", got)
	}
	if !strings.Contains(got, "conflict") {
		t.Errorf("formatConflictSummary(3) missing 'conflict', got: %q", got)
	}
}

// --- formatConflictDetail ---

func TestFormatConflictDetail(t *testing.T) {
	detail := formatConflictDetail("shared/utils.go", "other-vendor")
	if !strings.Contains(detail, "shared/utils.go") {
		t.Errorf("formatConflictDetail missing path, got: %q", detail)
	}
	if !strings.Contains(detail, "other-vendor") {
		t.Errorf("formatConflictDetail missing vendor, got: %q", detail)
	}
	if !strings.Contains(detail, "⚠") {
		t.Errorf("formatConflictDetail missing warning icon, got: %q", detail)
	}
}

// --- check function (nil path only) ---

func TestCheck_NilError(_ *testing.T) {
	// check(nil) should be a no-op — no panic, no exit
	check(nil)
}

// --- NonInteractive ShowError normal mode ---

func TestNonInteractiveTUICallback_ShowError_Normal(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	callback := NewNonInteractiveTUICallback(core.NonInteractiveFlags{
		Mode: core.OutputNormal,
	})

	callback.ShowError("Test Error", "error msg")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if !strings.Contains(buf.String(), "Test Error") {
		t.Errorf("ShowError normal mode missing title in stderr, got: %q", buf.String())
	}
}
