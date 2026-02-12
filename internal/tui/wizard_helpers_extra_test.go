package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/EmundoT/git-vendor/internal/types"
)

// --- resolveVendorData ---

func TestResolveVendorData(t *testing.T) {
	parser := func(raw string) (string, string, string) {
		return "https://github.com/owner/repo", "v1.0", "src/lib.go"
	}
	url, ref, name, smartPath := resolveVendorData("anything", parser)
	if url != "https://github.com/owner/repo" {
		t.Errorf("url = %q, want %q", url, "https://github.com/owner/repo")
	}
	if ref != "v1.0" {
		t.Errorf("ref = %q, want %q", ref, "v1.0")
	}
	if name != "repo" {
		t.Errorf("name = %q, want %q", name, "repo")
	}
	if smartPath != "src/lib.go" {
		t.Errorf("smartPath = %q, want %q", smartPath, "src/lib.go")
	}
}

func TestResolveVendorData_NoSmartRef(t *testing.T) {
	parser := func(raw string) (string, string, string) {
		return "https://github.com/owner/my-lib.git", "", ""
	}
	url, ref, name, smartPath := resolveVendorData("anything", parser)
	if url != "https://github.com/owner/my-lib.git" {
		t.Errorf("url = %q", url)
	}
	if ref != "main" { // core.DefaultRef
		t.Errorf("ref = %q, want %q", ref, "main")
	}
	if name != "my-lib" {
		t.Errorf("name = %q, want %q", name, "my-lib")
	}
	if smartPath != "" {
		t.Errorf("smartPath = %q, want empty", smartPath)
	}
}

// --- classifyEditSelection ---

func TestClassifyEditSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		wantAction string
		wantIdx    int
	}{
		{"cancel", "cancel", "cancel", -1},
		{"save", "save", "save", -1},
		{"new", "new", "new", -1},
		{"index 0", "0", "manage", 0},
		{"index 3", "3", "manage", 3},
		{"index 10", "10", "manage", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, idx := classifyEditSelection(tt.selection)
			if action != tt.wantAction {
				t.Errorf("classifyEditSelection(%q) action = %q, want %q", tt.selection, action, tt.wantAction)
			}
			if idx != tt.wantIdx {
				t.Errorf("classifyEditSelection(%q) idx = %d, want %d", tt.selection, idx, tt.wantIdx)
			}
		})
	}
}

// --- appendNewBranch ---

func TestAppendNewBranch(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main"},
	}
	result, sel := appendNewBranch(specs, "develop")
	if len(result) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(result))
	}
	if result[1].Ref != "develop" {
		t.Errorf("new branch ref = %q, want %q", result[1].Ref, "develop")
	}
	if sel != "1" {
		t.Errorf("selection = %q, want %q", sel, "1")
	}
}

func TestAppendNewBranch_Empty(t *testing.T) {
	result, sel := appendNewBranch(nil, "v1.0")
	if len(result) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(result))
	}
	if sel != "0" {
		t.Errorf("selection = %q, want %q", sel, "0")
	}
}

// --- classifyMappingSelection ---

func TestClassifyMappingSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		wantAction string
		wantIdx    int
	}{
		{"back", "back", "back", -1},
		{"add", "add", "add", -1},
		{"index 0", "0", "manage", 0},
		{"index 5", "5", "manage", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, idx := classifyMappingSelection(tt.selection)
			if action != tt.wantAction {
				t.Errorf("classifyMappingSelection(%q) action = %q, want %q", tt.selection, action, tt.wantAction)
			}
			if idx != tt.wantIdx {
				t.Errorf("classifyMappingSelection(%q) idx = %d, want %d", tt.selection, idx, tt.wantIdx)
			}
		})
	}
}

// --- processRemoteBrowserSelection ---

func TestProcessRemoteBrowserSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		currentDir string
		wantResult string
		wantNewDir string
		wantDone   bool
	}{
		{"cancel", "CANCEL", "", "", "", true},
		{"cancel from subdir", "CANCEL", "src", "", "", true},
		{"empty selection", "", "src", "", "", true},
		{"select current root", "SELECT_CURRENT", "", "", "", true},
		{"select current subdir", "SELECT_CURRENT", "src/components", "src/components", "", true},
		{"go up from subdir", "..", "src/components", "", "src", false},
		{"go up from single dir", "..", "src", "", "", false},
		{"select file from root", "README.md", "", "README.md", "", true},
		{"select file from subdir", "main.go", "src", "src/main.go", "", true},
		{"select dir from root", "pkg/", "", "", "pkg", false},
		{"select dir from subdir", "utils/", "src", "", "src/utils", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, newDir, done := processRemoteBrowserSelection(tt.selection, tt.currentDir)
			if result != tt.wantResult {
				t.Errorf("result = %q, want %q", result, tt.wantResult)
			}
			if newDir != tt.wantNewDir {
				t.Errorf("newDir = %q, want %q", newDir, tt.wantNewDir)
			}
			if done != tt.wantDone {
				t.Errorf("done = %v, want %v", done, tt.wantDone)
			}
		})
	}
}

// --- processLocalBrowserSelection ---

func TestProcessLocalBrowserSelection(t *testing.T) {
	tests := []struct {
		name       string
		selection  string
		currentDir string
		wantResult string
		wantNewDir string
		wantDone   bool
	}{
		{"cancel", "CANCEL", ".", "", "", true},
		{"empty selection", "", ".", "", "", true},
		{"select current root", "SELECT_CURRENT", ".", ".", "", true},
		{"select current subdir", "SELECT_CURRENT", "src", "src", "", true},
		{"go up from subdir", "..", "src/components", "", "src", false},
		{"go up from root", "..", ".", "", ".", false},
		{"select file", "main.go", ".", "main.go", "", true},
		{"select file from subdir", "index.ts", "src", "src/index.ts", "", true},
		{"select dir", "pkg/", ".", "", "pkg", false},
		{"select dir from subdir", "lib/", "src", "", "src/lib", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, newDir, done := processLocalBrowserSelection(tt.selection, tt.currentDir)
			if result != tt.wantResult {
				t.Errorf("result = %q, want %q", result, tt.wantResult)
			}
			if newDir != tt.wantNewDir {
				t.Errorf("newDir = %q, want %q", newDir, tt.wantNewDir)
			}
			if done != tt.wantDone {
				t.Errorf("done = %v, want %v", done, tt.wantDone)
			}
		})
	}
}

// --- buildRemoteBrowserOptionData ---

func TestBuildRemoteBrowserOptionData_Root(t *testing.T) {
	items := []string{"src/", "README.md"}
	labels, values := buildRemoteBrowserOptionData("", items)

	// Root: no ".." option
	// Expected: SELECT_CURRENT, src/, README.md, CANCEL
	if len(labels) != 4 {
		t.Fatalf("expected 4 options, got %d: %v", len(labels), labels)
	}
	if values[0] != "SELECT_CURRENT" {
		t.Errorf("first value = %q, want SELECT_CURRENT", values[0])
	}
	if values[1] != "src/" {
		t.Errorf("second value = %q, want src/", values[1])
	}
	if values[len(values)-1] != "CANCEL" {
		t.Errorf("last value = %q, want CANCEL", values[len(values)-1])
	}
	// Labels should have icons
	if !strings.Contains(labels[1], "📂") {
		t.Errorf("dir label missing folder icon: %q", labels[1])
	}
	if !strings.Contains(labels[2], "📄") {
		t.Errorf("file label missing file icon: %q", labels[2])
	}
}

func TestBuildRemoteBrowserOptionData_Subdir(t *testing.T) {
	items := []string{"utils.go"}
	labels, values := buildRemoteBrowserOptionData("src", items)

	// Subdir: has ".." option
	// Expected: .., SELECT_CURRENT, utils.go, CANCEL
	if len(labels) != 4 {
		t.Fatalf("expected 4 options, got %d: %v", len(labels), labels)
	}
	if values[0] != ".." {
		t.Errorf("first value = %q, want ..", values[0])
	}
	if labels[0] != ".. (Go Up)" {
		t.Errorf("first label = %q, want '.. (Go Up)'", labels[0])
	}
}

func TestBuildRemoteBrowserOptionData_Empty(t *testing.T) {
	labels, values := buildRemoteBrowserOptionData("", nil)
	// Root, no items: SELECT_CURRENT, CANCEL
	if len(labels) != 2 {
		t.Fatalf("expected 2 options, got %d", len(labels))
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
}

// --- buildLocalBrowserOptionData ---

func TestBuildLocalBrowserOptionData_Root(t *testing.T) {
	items := []string{"src/", "main.go"}
	labels, values := buildLocalBrowserOptionData(".", items)

	// Root (.): no ".." option (hasLocalParent returns false)
	// Expected: SELECT_CURRENT, src/, main.go, CANCEL
	if len(labels) != 4 {
		t.Fatalf("expected 4 options, got %d: %v", len(labels), labels)
	}
	if values[0] != "SELECT_CURRENT" {
		t.Errorf("first value = %q, want SELECT_CURRENT", values[0])
	}
	if values[len(values)-1] != "CANCEL" {
		t.Errorf("last value = %q, want CANCEL", values[len(values)-1])
	}
}

func TestBuildLocalBrowserOptionData_Subdir(t *testing.T) {
	items := []string{"index.ts"}
	labels, values := buildLocalBrowserOptionData("src", items)

	// Subdir: has ".." option
	// Expected: .., SELECT_CURRENT, index.ts, CANCEL
	if len(labels) != 4 {
		t.Fatalf("expected 4 options, got %d: %v", len(labels), labels)
	}
	if values[0] != ".." {
		t.Errorf("first value = %q, want ..", values[0])
	}
}

// --- check() error path via subprocess ---

func TestCheck_WithError(t *testing.T) {
	if os.Getenv("TEST_CHECK_EXIT") == "1" {
		check(errors.New("test error"))
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCheck_WithError$")
	cmd.Env = append(os.Environ(), "TEST_CHECK_EXIT=1")
	var stdout strings.Builder
	cmd.Stdout = &stdout
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit error, got: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}
	if !strings.Contains(stdout.String(), "Aborted") {
		t.Errorf("expected 'Aborted.' in output, got: %q", stdout.String())
	}
}

// --- BubbletaeProgressTracker ---

func TestBubbletaeProgressTracker_Lifecycle(t *testing.T) {
	m := &progressModel{total: 5, label: "test", width: 80}
	r, w := io.Pipe()
	defer r.Close()
	var devnull devNull
	p := tea.NewProgram(m, tea.WithInput(r), tea.WithOutput(&devnull))
	tracker := &BubbletaeProgressTracker{program: p}

	go func() {
		_, _ = p.Run()
	}()
	time.Sleep(50 * time.Millisecond)

	tracker.Increment("step 1")
	tracker.SetTotal(10)
	tracker.Complete()

	w.Close()
	time.Sleep(200 * time.Millisecond)
}

func TestBubbletaeProgressTracker_Fail(t *testing.T) {
	m := &progressModel{total: 3, label: "failing", width: 80}
	r, w := io.Pipe()
	defer r.Close()
	var devnull devNull
	p := tea.NewProgram(m, tea.WithInput(r), tea.WithOutput(&devnull))
	tracker := &BubbletaeProgressTracker{program: p}

	go func() {
		_, _ = p.Run()
	}()
	time.Sleep(50 * time.Millisecond)

	tracker.Increment("step 1")
	tracker.Fail(fmt.Errorf("something broke"))

	w.Close()
	time.Sleep(200 * time.Millisecond)
}

func TestNewBubbletaeProgressTracker(t *testing.T) {
	// NewBubbletaeProgressTracker uses os.Stdin/Stdout for the program.
	// In non-TTY test environments, Run() may return immediately.
	// Capture stdout to avoid polluting test output.
	output := captureStdout(func() {
		tracker := NewBubbletaeProgressTracker(3, "new tracker test")
		if tracker == nil {
			t.Fatal("NewBubbletaeProgressTracker returned nil")
		}
		if tracker.program == nil {
			t.Fatal("program is nil")
		}
		tracker.Complete()
		time.Sleep(200 * time.Millisecond)
	})
	_ = output
}

// devNull is an io.Writer that discards all writes (used for bubbletea output in tests).
type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

// --- buildMappingOptionData ---

func TestBuildMappingOptionData(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "src/lib.go", To: "vendor/lib.go"},
		{From: "src/utils.go", To: ""},
	}
	labels, values := buildMappingOptionData(mappings)
	// Expected: 2 mappings + "add" + "back" = 4
	if len(labels) != 4 {
		t.Fatalf("expected 4 options, got %d: %v", len(labels), labels)
	}
	if values[0] != "0" || values[1] != "1" {
		t.Errorf("mapping values = %v, want [0 1 ...]", values)
	}
	if values[2] != "add" {
		t.Errorf("add value = %q, want %q", values[2], "add")
	}
	if values[3] != "back" {
		t.Errorf("back value = %q, want %q", values[3], "back")
	}
	if !strings.Contains(labels[0], "→") {
		t.Errorf("mapping label missing arrow: %q", labels[0])
	}
}

func TestBuildMappingOptionData_Empty(t *testing.T) {
	labels, values := buildMappingOptionData(nil)
	if len(labels) != 2 {
		t.Fatalf("expected 2 options (add+back), got %d", len(labels))
	}
	if values[0] != "add" || values[1] != "back" {
		t.Errorf("values = %v, want [add back]", values)
	}
}

// --- buildBranchOptionData ---

func TestBuildBranchOptionData(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{{From: "a", To: "b"}}},
		{Ref: "v2.0", Mapping: []types.PathMapping{}},
	}
	getLockHash := func(name, ref string) string {
		if ref == "main" {
			return "abc1234567890"
		}
		return ""
	}
	labels, values := buildBranchOptionData(specs, "mylib", getLockHash)
	// Expected: 2 branches + new + save + cancel = 5
	if len(labels) != 5 {
		t.Fatalf("expected 5 options, got %d: %v", len(labels), labels)
	}
	if values[0] != "0" || values[1] != "1" {
		t.Errorf("branch values = %v", values[:2])
	}
	if values[2] != "new" || values[3] != "save" || values[4] != "cancel" {
		t.Errorf("action values = %v", values[2:])
	}
	// main branch should show "locked: abc1234"
	if !strings.Contains(labels[0], "locked:") {
		t.Errorf("main label missing lock status: %q", labels[0])
	}
	// v2.0 should show "not synced"
	if !strings.Contains(labels[1], "not synced") {
		t.Errorf("v2.0 label missing 'not synced': %q", labels[1])
	}
}

// --- classifyMappingAction ---

func TestClassifyMappingAction(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{"delete", "delete"},
		{"edit", "edit"},
		{"back", "back"},
		{"unknown", "back"},
		{"", "back"},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := classifyMappingAction(tt.action)
			if got != tt.want {
				t.Errorf("classifyMappingAction(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

// --- buildCreatorModeResult ---

func TestBuildCreatorModeResult(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		browsed    string
		manual     string
		wantFrom   string
		wantCancel bool
	}{
		{"browse with path", "browse", "src/lib.go", "", "src/lib.go", false},
		{"browse cancelled", "browse", "", "anything", "", true},
		{"manual", "manual", "", "pkg/api.go", "pkg/api.go", false},
		{"manual empty", "manual", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, cancelled := buildCreatorModeResult(tt.mode, tt.browsed, tt.manual)
			if from != tt.wantFrom {
				t.Errorf("from = %q, want %q", from, tt.wantFrom)
			}
			if cancelled != tt.wantCancel {
				t.Errorf("cancelled = %v, want %v", cancelled, tt.wantCancel)
			}
		})
	}
}

// --- buildCreatorLocalResult ---

func TestBuildCreatorLocalResult(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		browsed    string
		manual     string
		wantTo     string
		wantCancel bool
	}{
		{"browse with path", "browse", "vendor/lib.go", "", "vendor/lib.go", false},
		{"browse cancelled", "browse", "", "", "", true},
		{"manual", "manual", "", "out/file.go", "out/file.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to, cancelled := buildCreatorLocalResult(tt.mode, tt.browsed, tt.manual)
			if to != tt.wantTo {
				t.Errorf("to = %q, want %q", to, tt.wantTo)
			}
			if cancelled != tt.wantCancel {
				t.Errorf("cancelled = %v, want %v", cancelled, tt.wantCancel)
			}
		})
	}
}

// --- title/label builders ---

func TestBuildEditVendorTitle(t *testing.T) {
	got := buildEditVendorTitle("my-lib")
	if got != "Editing Vendor: my-lib" {
		t.Errorf("got %q", got)
	}
}

func TestBuildMappingManagerTitle(t *testing.T) {
	got := buildMappingManagerTitle("v2.0")
	if got != "Managing paths for v2.0" {
		t.Errorf("got %q", got)
	}
}

func TestBuildExistingVendorPrompt(t *testing.T) {
	got := buildExistingVendorPrompt("api-types")
	if got != "Repo 'api-types' is already tracked." {
		t.Errorf("got %q", got)
	}
}

func TestBuildDeleteMappingTitle(t *testing.T) {
	got := buildDeleteMappingTitle("src/utils.go")
	if got != "Delete mapping for 'src/utils.go'?" {
		t.Errorf("got %q", got)
	}
}

func TestBuildMappingActionTitle(t *testing.T) {
	got := buildMappingActionTitle("pkg/api/")
	if got != "Path: pkg/api/" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAcceptLicenseTitle(t *testing.T) {
	got := buildAcceptLicenseTitle("MIT")
	if got != "Accept MIT License?" {
		t.Errorf("got %q", got)
	}
}

// --- stubVendorMgr for testing helpers that need VendorManager ---

type stubVendorMgr struct {
	fetchRepoDirFn    func(url, ref, dir string) ([]string, error)
	listLocalDirFn    func(dir string) ([]string, error)
	getLockHashFn     func(name, ref string) string
	detectConflictsFn func() ([]types.PathConflict, error)
}

func (m *stubVendorMgr) ParseSmartURL(raw string) (string, string, string) {
	return raw, "", ""
}
func (m *stubVendorMgr) FetchRepoDir(_ context.Context, url, ref, dir string) ([]string, error) {
	if m.fetchRepoDirFn != nil {
		return m.fetchRepoDirFn(url, ref, dir)
	}
	return nil, nil
}
func (m *stubVendorMgr) ListLocalDir(dir string) ([]string, error) {
	if m.listLocalDirFn != nil {
		return m.listLocalDirFn(dir)
	}
	return nil, nil
}
func (m *stubVendorMgr) GetLockHash(name, ref string) string {
	if m.getLockHashFn != nil {
		return m.getLockHashFn(name, ref)
	}
	return ""
}
func (m *stubVendorMgr) DetectConflicts() ([]types.PathConflict, error) {
	if m.detectConflictsFn != nil {
		return m.detectConflictsFn()
	}
	return nil, nil
}

// --- prepareRemoteBrowserOptions ---

func TestPrepareRemoteBrowserOptions(t *testing.T) {
	mgr := &stubVendorMgr{
		fetchRepoDirFn: func(url, ref, dir string) ([]string, error) {
			return []string{"src/", "README.md"}, nil
		},
	}
	labels, values, breadcrumb, err := prepareRemoteBrowserOptions(context.Background(), mgr, "https://github.com/owner/repo.git", "main", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 4 {
		t.Errorf("expected 4 labels, got %d", len(labels))
	}
	if len(values) != 4 {
		t.Errorf("expected 4 values, got %d", len(values))
	}
	if !strings.Contains(breadcrumb, "repo") {
		t.Errorf("breadcrumb missing repo name: %q", breadcrumb)
	}
	if !strings.Contains(breadcrumb, "main") {
		t.Errorf("breadcrumb missing ref: %q", breadcrumb)
	}
}

func TestPrepareRemoteBrowserOptions_Error(t *testing.T) {
	mgr := &stubVendorMgr{
		fetchRepoDirFn: func(url, ref, dir string) ([]string, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	_, _, _, err := prepareRemoteBrowserOptions(context.Background(), mgr, "https://github.com/owner/repo", "main", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("error = %q, want network error", err.Error())
	}
}

func TestPrepareRemoteBrowserOptions_Subdir(t *testing.T) {
	mgr := &stubVendorMgr{
		fetchRepoDirFn: func(url, ref, dir string) ([]string, error) {
			return []string{"util.go"}, nil
		},
	}
	labels, values, breadcrumb, err := prepareRemoteBrowserOptions(context.Background(), mgr, "https://github.com/owner/repo", "v1.0", "src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Subdir: has ".." option
	if values[0] != ".." {
		t.Errorf("first value = %q, want ..", values[0])
	}
	if !strings.Contains(breadcrumb, "src") {
		t.Errorf("breadcrumb missing subdir: %q", breadcrumb)
	}
	_ = labels
}

// --- prepareLocalBrowserOptions ---

func TestPrepareLocalBrowserOptions(t *testing.T) {
	mgr := &stubVendorMgr{
		listLocalDirFn: func(dir string) ([]string, error) {
			return []string{"pkg/", "main.go"}, nil
		},
	}
	labels, values, title, err := prepareLocalBrowserOptions(mgr, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 4 {
		t.Errorf("expected 4 labels, got %d", len(labels))
	}
	if len(values) != 4 {
		t.Errorf("expected 4 values, got %d", len(values))
	}
	if !strings.Contains(title, "Local:") {
		t.Errorf("title missing 'Local:': %q", title)
	}
}

func TestPrepareLocalBrowserOptions_Error(t *testing.T) {
	mgr := &stubVendorMgr{
		listLocalDirFn: func(dir string) ([]string, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}
	_, _, _, err := prepareLocalBrowserOptions(mgr, ".")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- processEditWizardAction ---

func TestProcessEditWizardAction_Cancel(t *testing.T) {
	vendor := &types.VendorSpec{Name: "test"}
	mgr := &stubVendorMgr{}
	result := processEditWizardAction("cancel", vendor, mgr)
	if !result.ShouldExit {
		t.Error("expected ShouldExit=true")
	}
	if result.ReturnVendor != nil {
		t.Error("expected ReturnVendor=nil for cancel")
	}
}

func TestProcessEditWizardAction_Save(t *testing.T) {
	vendor := &types.VendorSpec{Name: "test"}
	mgr := &stubVendorMgr{
		detectConflictsFn: func() ([]types.PathConflict, error) { return nil, nil },
	}
	output := captureStdout(func() {
		result := processEditWizardAction("save", vendor, mgr)
		if !result.ShouldExit {
			t.Error("expected ShouldExit=true")
		}
		if result.ReturnVendor != vendor {
			t.Error("expected ReturnVendor to be the vendor")
		}
	})
	_ = output
}

func TestProcessEditWizardAction_New(t *testing.T) {
	vendor := &types.VendorSpec{Name: "test"}
	mgr := &stubVendorMgr{}
	result := processEditWizardAction("new", vendor, mgr)
	if result.ShouldExit {
		t.Error("expected ShouldExit=false")
	}
	if !result.NeedNewBranch {
		t.Error("expected NeedNewBranch=true")
	}
}

func TestProcessEditWizardAction_Manage(t *testing.T) {
	vendor := &types.VendorSpec{Name: "test"}
	mgr := &stubVendorMgr{}
	result := processEditWizardAction("2", vendor, mgr)
	if result.ShouldExit {
		t.Error("expected ShouldExit=false")
	}
	if result.ManageIdx != 2 {
		t.Errorf("ManageIdx = %d, want 2", result.ManageIdx)
	}
}

// --- validateVendorInput ---

func TestValidateVendorInput(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		vname   string
		ref     string
		wantErr string
	}{
		{"valid", "https://github.com/owner/repo", "mylib", "main", ""},
		{"empty url", "", "mylib", "main", "URL is required"},
		{"invalid url", "not-a-url", "mylib", "main", "invalid git URL"},
		{"empty name", "https://github.com/owner/repo", "", "main", "vendor name is required"},
		{"name with space", "https://github.com/owner/repo", "my lib", "main", "invalid characters"},
		{"name with slash", "https://github.com/owner/repo", "my/lib", "main", "invalid characters"},
		{"empty ref", "https://github.com/owner/repo", "mylib", "", "git ref is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVendorInput(tt.url, tt.vname, tt.ref)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- validateMappingPair ---

func TestValidateMappingPair(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"valid", "src/lib.go", "vendor/lib.go", false},
		{"valid with position", "src/lib.go:L5-L10", "vendor/lib.go", false},
		{"empty to is ok", "src/lib.go", "", false},
		{"empty from", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMappingPair(tt.from, tt.to)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// --- summarizeMappings ---

func TestSummarizeMappings(t *testing.T) {
	mappings := []types.PathMapping{
		{From: "src/lib.go", To: "vendor/lib.go"},
		{From: "src/utils.go", To: ""},
	}
	result := summarizeMappings(mappings)
	if !strings.Contains(result, "src/lib.go") {
		t.Errorf("missing src/lib.go in: %q", result)
	}
	if !strings.Contains(result, "→") {
		t.Errorf("missing arrow in: %q", result)
	}
}

func TestSummarizeMappings_Empty(t *testing.T) {
	result := summarizeMappings(nil)
	if result != "  (no mappings)" {
		t.Errorf("got %q, want '  (no mappings)'", result)
	}
}

// --- formatVendorSummary ---

func TestFormatVendorSummary(t *testing.T) {
	spec := types.VendorSpec{
		Name: "api-types",
		URL:  "https://github.com/owner/api",
		Specs: []types.BranchSpec{
			{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/types.go", To: "vendor/types.go"}},
			},
		},
	}
	getLockHash := func(name, ref string) string {
		if ref == "main" {
			return "abc1234567890"
		}
		return ""
	}
	result := formatVendorSummary(spec, getLockHash)
	if !strings.Contains(result, "api-types") {
		t.Errorf("missing vendor name in: %q", result)
	}
	if !strings.Contains(result, "main") {
		t.Errorf("missing branch ref in: %q", result)
	}
	if !strings.Contains(result, "locked:") {
		t.Errorf("missing lock status in: %q", result)
	}
}

func TestFormatVendorSummary_NilGetLockHash(t *testing.T) {
	spec := types.VendorSpec{
		Name:  "test",
		URL:   "https://example.com/repo",
		Specs: []types.BranchSpec{{Ref: "main"}},
	}
	result := formatVendorSummary(spec, nil)
	if !strings.Contains(result, "not synced") {
		t.Errorf("expected 'not synced' in: %q", result)
	}
}

// --- detectDuplicateMappings ---

func TestDetectDuplicateMappings(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{
			{From: "src/lib.go", To: "a/lib.go"},
			{From: "src/utils.go", To: "a/utils.go"},
		}},
		{Ref: "v2", Mapping: []types.PathMapping{
			{From: "src/lib.go", To: "b/lib.go"},
		}},
	}
	dupes := detectDuplicateMappings(specs)
	if len(dupes) != 1 {
		t.Fatalf("expected 1 duplicate, got %d: %v", len(dupes), dupes)
	}
	if !strings.Contains(dupes[0], "src/lib.go") {
		t.Errorf("duplicate desc = %q, expected src/lib.go", dupes[0])
	}
}

func TestDetectDuplicateMappings_None(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}},
	}
	dupes := detectDuplicateMappings(specs)
	if len(dupes) != 0 {
		t.Errorf("expected 0 duplicates, got %d", len(dupes))
	}
}

// --- countMappings ---

func TestCountMappings(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{{From: "a"}, {From: "b"}}},
		{Ref: "v2", Mapping: []types.PathMapping{{From: "c"}}},
	}
	if got := countMappings(specs); got != 3 {
		t.Errorf("countMappings = %d, want 3", got)
	}
}

func TestCountMappings_Empty(t *testing.T) {
	if got := countMappings(nil); got != 0 {
		t.Errorf("countMappings(nil) = %d, want 0", got)
	}
}

// --- findMappingByFrom ---

func TestFindMappingByFrom(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{
			{From: "a.go", To: "x.go"},
			{From: "b.go", To: "y.go"},
		}},
		{Ref: "v2", Mapping: []types.PathMapping{
			{From: "c.go", To: "z.go"},
		}},
	}
	bi, mi, found := findMappingByFrom(specs, "b.go")
	if !found {
		t.Fatal("expected found=true")
	}
	if bi != 0 || mi != 1 {
		t.Errorf("bi=%d mi=%d, want 0,1", bi, mi)
	}

	bi, mi, found = findMappingByFrom(specs, "c.go")
	if !found {
		t.Fatal("expected found=true for c.go")
	}
	if bi != 1 || mi != 0 {
		t.Errorf("bi=%d mi=%d, want 1,0", bi, mi)
	}

	_, _, found = findMappingByFrom(specs, "nonexistent.go")
	if found {
		t.Error("expected found=false for nonexistent")
	}
}

// --- isEmptySpec ---

func TestIsEmptySpec(t *testing.T) {
	tests := []struct {
		name string
		spec types.VendorSpec
		want bool
	}{
		{"no specs", types.VendorSpec{}, true},
		{"empty branch", types.VendorSpec{Specs: []types.BranchSpec{{Ref: "main"}}}, true},
		{"with mapping", types.VendorSpec{Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a"}}},
		}}, false},
		{"mixed", types.VendorSpec{Specs: []types.BranchSpec{
			{Ref: "main"},
			{Ref: "v2", Mapping: []types.PathMapping{{From: "a"}}},
		}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptySpec(tt.spec); got != tt.want {
				t.Errorf("isEmptySpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- buildVendorSpecWithDefaults ---

func TestBuildVendorSpecWithDefaults(t *testing.T) {
	parser := func(raw string) (string, string, string) {
		return "https://github.com/owner/repo", "main", "src/"
	}
	spec := buildVendorSpecWithDefaults("anything", parser, "", "")
	if spec.Name != "repo" {
		t.Errorf("name = %q, want repo", spec.Name)
	}
	if spec.URL != "https://github.com/owner/repo" {
		t.Errorf("url = %q", spec.URL)
	}
	if spec.Specs[0].Ref != "main" {
		t.Errorf("ref = %q, want main", spec.Specs[0].Ref)
	}
}

// --- specHasMapping ---

func TestSpecHasMapping(t *testing.T) {
	spec := types.VendorSpec{
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}},
			{Ref: "v2", Mapping: []types.PathMapping{{From: "c.go", To: "d.go"}}},
		},
	}
	if !specHasMapping(spec, "a.go") {
		t.Error("expected true for a.go")
	}
	if !specHasMapping(spec, "c.go") {
		t.Error("expected true for c.go")
	}
	if specHasMapping(spec, "nonexistent.go") {
		t.Error("expected false for nonexistent.go")
	}
}

func TestSpecHasMapping_Empty(t *testing.T) {
	if specHasMapping(types.VendorSpec{}, "a.go") {
		t.Error("expected false for empty spec")
	}
}

// --- formatVendorListEntry ---

func TestFormatVendorListEntry(t *testing.T) {
	spec := types.VendorSpec{
		Name: "mylib",
		URL:  "https://github.com/owner/repo",
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a"}, {From: "b"}}},
			{Ref: "v2", Mapping: []types.PathMapping{{From: "c"}}},
		},
	}
	result := formatVendorListEntry(spec)
	if !strings.Contains(result, "mylib") {
		t.Errorf("missing name in: %q", result)
	}
	if !strings.Contains(result, "2 branches") {
		t.Errorf("missing branch count in: %q", result)
	}
	if !strings.Contains(result, "3 paths") {
		t.Errorf("missing path count in: %q", result)
	}
}

func TestFormatVendorListEntry_Single(t *testing.T) {
	spec := types.VendorSpec{
		Name:  "tiny",
		URL:   "https://example.com/tiny",
		Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a"}}}},
	}
	result := formatVendorListEntry(spec)
	if !strings.Contains(result, "1 branch,") {
		t.Errorf("expected singular 'branch' in: %q", result)
	}
	if !strings.Contains(result, "1 path") {
		t.Errorf("expected '1 path' in: %q", result)
	}
}

// --- hasMappings ---

func TestHasMappings(t *testing.T) {
	with := types.VendorSpec{Specs: []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: "a"}}}}}
	without := types.VendorSpec{Specs: []types.BranchSpec{{Ref: "main"}}}
	empty := types.VendorSpec{}
	if !hasMappings(with) {
		t.Error("expected true for spec with mappings")
	}
	if hasMappings(without) {
		t.Error("expected false for spec without mappings")
	}
	if hasMappings(empty) {
		t.Error("expected false for empty spec")
	}
}

// --- validateVendorSpec ---

func TestValidateVendorSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    types.VendorSpec
		wantErr string
	}{
		{"valid", types.VendorSpec{Name: "lib", URL: "https://github.com/o/r", Specs: []types.BranchSpec{{Ref: "main"}}}, ""},
		{"no name", types.VendorSpec{URL: "https://github.com/o/r", Specs: []types.BranchSpec{{Ref: "main"}}}, "vendor name"},
		{"no url", types.VendorSpec{Name: "lib", Specs: []types.BranchSpec{{Ref: "main"}}}, "vendor URL is required"},
		{"invalid url", types.VendorSpec{Name: "lib", URL: "not-a-url", Specs: []types.BranchSpec{{Ref: "main"}}}, "invalid vendor URL"},
		{"no specs", types.VendorSpec{Name: "lib", URL: "https://github.com/o/r"}, "at least one branch"},
		{"empty ref", types.VendorSpec{Name: "lib", URL: "https://github.com/o/r", Specs: []types.BranchSpec{{Ref: ""}}}, "empty ref"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVendorSpec(tt.spec)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- truncateHash ---

func TestTruncateHash(t *testing.T) {
	if got := truncateHash("abc1234567890def", 7); got != "abc1234" {
		t.Errorf("got %q, want %q", got, "abc1234")
	}
	if got := truncateHash("short", 10); got != "short" {
		t.Errorf("got %q, want %q", got, "short")
	}
	if got := truncateHash("", 7); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- validateAllMappings ---

func TestValidateAllMappings_Valid(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{
			{From: "src/lib.go", To: "vendor/lib.go"},
			{From: "src/utils.go", To: ""},
		}},
	}
	if err := validateAllMappings(specs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllMappings_Invalid(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{
			{From: "", To: "vendor/lib.go"},
		}},
	}
	err := validateAllMappings(specs)
	if err == nil {
		t.Fatal("expected error for empty From")
	}
	if !strings.Contains(err.Error(), "branch main") {
		t.Errorf("error = %q, expected branch annotation", err.Error())
	}
}

func TestValidateAllMappings_Empty(t *testing.T) {
	if err := validateAllMappings(nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- collectMappingFromPaths ---

func TestCollectMappingFromPaths(t *testing.T) {
	specs := []types.BranchSpec{
		{Ref: "main", Mapping: []types.PathMapping{{From: "a.go"}, {From: "b.go"}}},
		{Ref: "v2", Mapping: []types.PathMapping{{From: "c.go"}}},
	}
	paths := collectMappingFromPaths(specs)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	if paths[0] != "a.go" || paths[1] != "b.go" || paths[2] != "c.go" {
		t.Errorf("paths = %v", paths)
	}
}

func TestCollectMappingFromPaths_Empty(t *testing.T) {
	paths := collectMappingFromPaths(nil)
	if len(paths) != 0 {
		t.Errorf("expected empty, got %v", paths)
	}
}

func TestBuildVendorSpecWithDefaults_Overrides(t *testing.T) {
	parser := func(raw string) (string, string, string) {
		return "https://github.com/owner/repo", "main", ""
	}
	spec := buildVendorSpecWithDefaults("anything", parser, "custom-name", "v2.0")
	if spec.Name != "custom-name" {
		t.Errorf("name = %q, want custom-name", spec.Name)
	}
	if spec.Specs[0].Ref != "v2.0" {
		t.Errorf("ref = %q, want v2.0", spec.Specs[0].Ref)
	}
}
