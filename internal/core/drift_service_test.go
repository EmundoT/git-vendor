package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Pure function tests (no mocks needed)
// ============================================================================

func TestCountLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 2},
		{"two lines", "hello\nworld", 2},
		{"three lines trailing newline", "a\nb\nc\n", 4},
		{"only newlines", "\n\n\n", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.content)
			if got != tt.want {
				t.Errorf("countLines(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestLongestCommonSubsequence(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int
	}{
		{"both empty", nil, nil, 0},
		{"a empty", nil, []string{"x"}, 0},
		{"b empty", []string{"x"}, nil, 0},
		{"identical single", []string{"a"}, []string{"a"}, 1},
		{"identical multi", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 3},
		{"completely different", []string{"a", "b"}, []string{"x", "y"}, 0},
		{"one insertion", []string{"a", "b", "c"}, []string{"a", "x", "b", "c"}, 3},
		{"one deletion", []string{"a", "b", "c"}, []string{"a", "c"}, 2},
		{"one modification", []string{"a", "b", "c"}, []string{"a", "x", "c"}, 2},
		{"prefix match", []string{"a", "b"}, []string{"a", "b", "c"}, 2},
		{"suffix match", []string{"b", "c"}, []string{"a", "b", "c"}, 2},
		{"interleaved", []string{"a", "b", "c", "d"}, []string{"a", "c", "b", "d"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := longestCommonSubsequence(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("longestCommonSubsequence(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLineDiff(t *testing.T) {
	tests := []struct {
		name        string
		original    string
		current     string
		wantAdded   int
		wantRemoved int
	}{
		{
			"identical",
			"line1\nline2\nline3",
			"line1\nline2\nline3",
			0, 0,
		},
		{
			"one line added",
			"line1\nline2",
			"line1\nline2\nline3",
			1, 0,
		},
		{
			"one line removed",
			"line1\nline2\nline3",
			"line1\nline3",
			0, 1,
		},
		{
			"one line modified",
			"line1\nline2\nline3",
			"line1\nmodified\nline3",
			1, 1,
		},
		{
			"completely different",
			"aaa\nbbb",
			"xxx\nyyy\nzzz",
			3, 2,
		},
		{
			"empty to content",
			"",
			"line1\nline2",
			// strings.Split("", "\n") = [""] (1 element)
			// LCS([""], ["line1","line2"]) = 0, so added=2, removed=1
			2, 1,
		},
		{
			"content to empty",
			"line1\nline2",
			"",
			// LCS(["line1","line2"], [""]) = 0, so added=1, removed=2
			1, 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := lineDiff(tt.original, tt.current)
			if added != tt.wantAdded || removed != tt.wantRemoved {
				t.Errorf("lineDiff() = (added=%d, removed=%d), want (added=%d, removed=%d)",
					added, removed, tt.wantAdded, tt.wantRemoved)
			}
		})
	}
}

func TestDriftPercentage(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		removed  int
		original string
		current  string
		wantPct  float64
	}{
		{"no changes", 0, 0, "a\nb", "a\nb", 0},
		{"all changed small", 2, 2, "a\nb", "x\ny", 100},
		{"half changed", 1, 1, "a\nb\nc\nd", "a\nx\nc\nd", 50},
		{"empty both", 0, 0, "", "", 0},
		{"100 percent cap", 10, 10, "a", "b", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driftPercentage(tt.added, tt.removed, tt.original, tt.current)
			if got != tt.wantPct {
				t.Errorf("driftPercentage(%d, %d, ...) = %.1f, want %.1f",
					tt.added, tt.removed, got, tt.wantPct)
			}
		})
	}
}

func TestAggregateDriftPct(t *testing.T) {
	tests := []struct {
		name           string
		totalAdded     int
		totalRemoved   int
		totalOrigLines int
		wantPct        float64
	}{
		{"no changes", 0, 0, 100, 0},
		{"all changed", 100, 100, 100, 100},
		{"50 percent", 25, 25, 100, 50},
		{"zero orig lines no additions", 0, 0, 0, 0},
		{"zero orig lines with additions", 10, 0, 0, 100},
		{"over 100 capped", 200, 200, 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateDriftPct(tt.totalAdded, tt.totalRemoved, tt.totalOrigLines)
			if got != tt.wantPct {
				t.Errorf("aggregateDriftPct(%d, %d, %d) = %.1f, want %.1f",
					tt.totalAdded, tt.totalRemoved, tt.totalOrigLines, got, tt.wantPct)
			}
		})
	}
}

func TestComputeDriftSummary(t *testing.T) {
	tests := []struct {
		name string
		deps []types.DriftDependency
		want types.DriftSummary
	}{
		{
			"empty",
			nil,
			types.DriftSummary{Result: types.DriftResultClean},
		},
		{
			"all clean",
			[]types.DriftDependency{
				{DriftScore: 0, LocalDrift: types.DriftStats{FilesChanged: 0}, UpstreamDrift: types.DriftStats{FilesChanged: 0}},
				{DriftScore: 0, LocalDrift: types.DriftStats{FilesChanged: 0}, UpstreamDrift: types.DriftStats{FilesChanged: 0}},
			},
			types.DriftSummary{
				TotalDependencies: 2,
				Clean:             2,
				Result:            types.DriftResultClean,
			},
		},
		{
			"local drift only",
			[]types.DriftDependency{
				{DriftScore: 50, LocalDrift: types.DriftStats{FilesChanged: 1}, UpstreamDrift: types.DriftStats{FilesChanged: 0}},
			},
			types.DriftSummary{
				TotalDependencies: 1,
				DriftedLocal:      1,
				OverallDriftScore: 50,
				Result:            types.DriftResultDrifted,
			},
		},
		{
			"upstream drift only",
			[]types.DriftDependency{
				{DriftScore: 0, LocalDrift: types.DriftStats{FilesChanged: 0}, UpstreamDrift: types.DriftStats{FilesChanged: 2}},
			},
			types.DriftSummary{
				TotalDependencies: 1,
				DriftedUpstream:   1,
				Result:            types.DriftResultDrifted,
			},
		},
		{
			"conflict risk — both local and upstream changed",
			[]types.DriftDependency{
				{DriftScore: 75, LocalDrift: types.DriftStats{FilesChanged: 2}, UpstreamDrift: types.DriftStats{FilesChanged: 1}},
			},
			types.DriftSummary{
				TotalDependencies: 1,
				DriftedLocal:      1,
				DriftedUpstream:   1,
				ConflictRisk:      1,
				OverallDriftScore: 75,
				Result:            types.DriftResultConflict,
			},
		},
		{
			"mixed dependencies",
			[]types.DriftDependency{
				{DriftScore: 0, LocalDrift: types.DriftStats{FilesChanged: 0}, UpstreamDrift: types.DriftStats{FilesChanged: 0}},
				{DriftScore: 30, LocalDrift: types.DriftStats{FilesChanged: 1}, UpstreamDrift: types.DriftStats{FilesChanged: 0}},
				{DriftScore: 60, LocalDrift: types.DriftStats{FilesChanged: 2}, UpstreamDrift: types.DriftStats{FilesChanged: 1}},
			},
			types.DriftSummary{
				TotalDependencies: 3,
				Clean:             1,
				DriftedLocal:      2,
				DriftedUpstream:   1,
				ConflictRisk:      1,
				OverallDriftScore: 30,
				Result:            types.DriftResultConflict,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDriftSummary(tt.deps)

			if got.TotalDependencies != tt.want.TotalDependencies {
				t.Errorf("TotalDependencies = %d, want %d", got.TotalDependencies, tt.want.TotalDependencies)
			}
			if got.Clean != tt.want.Clean {
				t.Errorf("Clean = %d, want %d", got.Clean, tt.want.Clean)
			}
			if got.DriftedLocal != tt.want.DriftedLocal {
				t.Errorf("DriftedLocal = %d, want %d", got.DriftedLocal, tt.want.DriftedLocal)
			}
			if got.DriftedUpstream != tt.want.DriftedUpstream {
				t.Errorf("DriftedUpstream = %d, want %d", got.DriftedUpstream, tt.want.DriftedUpstream)
			}
			if got.ConflictRisk != tt.want.ConflictRisk {
				t.Errorf("ConflictRisk = %d, want %d", got.ConflictRisk, tt.want.ConflictRisk)
			}
			if got.Result != tt.want.Result {
				t.Errorf("Result = %q, want %q", got.Result, tt.want.Result)
			}
			if got.OverallDriftScore != tt.want.OverallDriftScore {
				t.Errorf("OverallDriftScore = %.1f, want %.1f", got.OverallDriftScore, tt.want.OverallDriftScore)
			}
		})
	}
}

func TestGenerateSimpleDiff(t *testing.T) {
	original := "line1\nline2\nline3"
	current := "line1\nmodified\nline3"

	output := generateSimpleDiff("test.go", original, current, "locked", "local")

	if output == "" {
		t.Fatal("generateSimpleDiff returned empty string")
	}

	// Verify header
	if !contains(output, "--- a/test.go (locked)") {
		t.Error("diff output missing original header")
	}
	if !contains(output, "+++ b/test.go (local)") {
		t.Error("diff output missing current header")
	}

	// Verify diff markers
	if !contains(output, "-line2") {
		t.Error("diff output missing removed line marker")
	}
	if !contains(output, "+modified") {
		t.Error("diff output missing added line marker")
	}

	// Verify unchanged lines
	if !contains(output, " line1") {
		t.Error("diff output missing unchanged line marker")
	}
}

func TestFormatDriftOutput(t *testing.T) {
	dep := &types.DriftDependency{
		Name:         "test-lib",
		Ref:          "main",
		LockedCommit: "abc1234567890",
		LatestCommit: "def9876543210",
		DriftScore:   25,
		Files: []types.DriftFile{
			{
				Path:              "lib/test.go",
				LocalStatus:       types.DriftStatusModified,
				UpstreamStatus:    types.DriftStatusUnchanged,
				LocalLinesAdded:   5,
				LocalLinesRemoved: 2,
				LocalDriftPct:     25,
			},
			{
				Path:        "lib/util.go",
				LocalStatus: types.DriftStatusUnchanged,
			},
		},
		LocalDrift: types.DriftStats{
			FilesChanged:   1,
			FilesUnchanged: 1,
			DriftPercentage: 25,
		},
		UpstreamDrift: types.DriftStats{
			FilesChanged:   0,
			FilesUnchanged: 2,
			DriftPercentage: 0,
		},
	}

	output := FormatDriftOutput(dep, false)

	if output == "" {
		t.Fatal("FormatDriftOutput returned empty string")
	}

	// Verify vendor header
	if !contains(output, "test-lib") {
		t.Error("output missing vendor name")
	}
	if !contains(output, "abc1234") {
		t.Error("output missing short commit hash")
	}

	// Verify file listing
	if !contains(output, "lib/test.go") {
		t.Error("output missing modified file path")
	}
	if !contains(output, "+5/-2") {
		t.Error("output missing line change stats")
	}

	// Verify summary
	if !contains(output, "Local drift: 25%") {
		t.Error("output missing local drift summary")
	}
	if !contains(output, "Overall drift score: 25%") {
		t.Error("output missing overall drift score")
	}
}

func TestFormatDriftOutput_Offline(t *testing.T) {
	dep := &types.DriftDependency{
		Name:         "test-lib",
		Ref:          "main",
		LockedCommit: "abc1234567890",
		DriftScore:   0,
		Files: []types.DriftFile{
			{
				Path:        "lib/test.go",
				LocalStatus: types.DriftStatusUnchanged,
			},
		},
		LocalDrift: types.DriftStats{
			FilesUnchanged: 1,
		},
	}

	output := FormatDriftOutput(dep, true)

	// In offline mode, upstream info should not appear
	if contains(output, "Upstream drift") {
		t.Error("offline output should not contain upstream drift info")
	}
}

func TestFormatDriftOutput_ConflictRisk(t *testing.T) {
	dep := &types.DriftDependency{
		Name:            "risky-lib",
		Ref:             "main",
		LockedCommit:    "abc1234567890",
		LatestCommit:    "def9876543210",
		DriftScore:      50,
		HasConflictRisk: true,
		Files: []types.DriftFile{
			{
				Path:              "lib/shared.go",
				LocalStatus:       types.DriftStatusModified,
				UpstreamStatus:    types.DriftStatusModified,
				HasConflictRisk:   true,
				LocalLinesAdded:   3,
				LocalLinesRemoved: 1,
				LocalDriftPct:     40,
			},
		},
		LocalDrift: types.DriftStats{
			FilesChanged:    1,
			DriftPercentage: 50,
		},
		UpstreamDrift: types.DriftStats{
			FilesChanged:    1,
			DriftPercentage: 30,
		},
	}

	output := FormatDriftOutput(dep, false)

	if !contains(output, "CONFLICT RISK") {
		t.Error("output missing conflict risk indicator for file")
	}
	if !contains(output, "Merge conflict risk detected") {
		t.Error("output missing conflict risk summary")
	}
}

// ============================================================================
// Service-level tests with gomock
// ============================================================================

func TestDrift_NoLockfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "test", URL: "https://github.com/owner/repo", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{}, nil)

	svc := NewDriftService(configStore, lockStore, nil, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})

	if err == nil {
		t.Fatal("expected error for empty lockfile")
	}
	if !contains(err.Error(), "no vendors in lockfile") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_VendorNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "other-vendor", URL: "https://github.com/owner/repo", Specs: []types.BranchSpec{{Ref: "main"}}},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "other-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	svc := NewDriftService(configStore, lockStore, nil, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{Dependency: "nonexistent"})

	if err == nil {
		t.Fatal("expected error for nonexistent vendor")
	}
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestDrift_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{}, fmt.Errorf("config broken"))

	svc := NewDriftService(configStore, lockStore, nil, nil, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})

	if err == nil {
		t.Fatal("expected error for config load failure")
	}
	if !contains(err.Error(), "load config") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_LockLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{}, fmt.Errorf("lock broken"))

	svc := NewDriftService(configStore, lockStore, nil, nil, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})

	if err == nil {
		t.Fatal("expected error for lock load failure")
	}
	if !contains(err.Error(), "load lockfile") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_SkipsPositionMappings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Vendor with only position-extracted mappings should produce zero files
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "pos-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "file.go:L5-L10", To: "local/snippet.go"},
						},
					},
				},
			},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "pos-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, "/root")
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Dependencies))
	}

	// Position-only vendor should have zero tracked files and zero drift
	if len(result.Dependencies[0].Files) != 0 {
		t.Errorf("expected 0 files for position-only vendor, got %d", len(result.Dependencies[0].Files))
	}
	if result.Dependencies[0].DriftScore != 0 {
		t.Errorf("expected 0 drift score for position-only vendor, got %.1f", result.Dependencies[0].DriftScore)
	}
}

// ============================================================================
// driftForVendorRef service tests (require real temp files + mocked git)
// ============================================================================

// setupDriftTestFiles creates temp directories with pre-written source and local
// files, returning (cloneDir, workDir, cleanup). The cloneDir simulates the git
// clone working tree; the workDir simulates the project root with local files.
func setupDriftTestFiles(t *testing.T, sourceFiles, localFiles map[string]string) (string, string, func()) {
	t.Helper()

	cloneDir := t.TempDir()
	workDir := t.TempDir()

	// Write "original" source files (simulating checked-out locked commit)
	for path, content := range sourceFiles {
		full := filepath.Join(cloneDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write "local" files (simulating vendored files on disk)
	for path, content := range localFiles {
		full := filepath.Join(workDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return cloneDir, workDir, func() {}
}

// expectGitOpsForDrift sets up gomock expectations for the git operations
// that driftForVendorRef performs: CreateTemp→Init→FetchWithFallback(AddRemote+Fetch)→Checkout.
// Returns the cloneDir that CreateTemp returns.
func expectGitOpsForDrift(
	t *testing.T,
	fs *MockFileSystem,
	git *MockGitClient,
	cloneDir string,
	offline bool,
) {
	t.Helper()

	fs.EXPECT().CreateTemp("", "drift-*").Return(cloneDir, nil)
	fs.EXPECT().RemoveAll(cloneDir).Return(nil)

	git.EXPECT().Init(gomock.Any(), cloneDir).Return(nil)
	// FetchWithFallback: AddRemote + Fetch(depth=0) for primary URL
	git.EXPECT().AddRemote(gomock.Any(), cloneDir, "origin", gomock.Any()).Return(nil)
	git.EXPECT().Fetch(gomock.Any(), cloneDir, "origin", 0, gomock.Any()).Return(nil)
	// Checkout locked commit
	git.EXPECT().Checkout(gomock.Any(), cloneDir, gomock.Any()).Return(nil)

	if !offline {
		// Checkout upstream (FETCH_HEAD)
		git.EXPECT().Checkout(gomock.Any(), cloneDir, "FETCH_HEAD").Return(nil)
		git.EXPECT().GetHeadHash(gomock.Any(), cloneDir).Return("latest123456", nil)
	}
}

func TestDrift_HappyPath_NoChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	content := "line1\nline2\nline3"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": content},
		map[string]string{"lib/file.go": content},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-lib",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
				}},
			},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"},
		},
	}, nil)

	expectGitOpsForDrift(t, fs, gitClient, cloneDir, false)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Dependencies))
	}

	dep := result.Dependencies[0]
	if dep.DriftScore != 0 {
		t.Errorf("expected 0 drift score, got %.1f", dep.DriftScore)
	}
	if len(dep.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(dep.Files))
	}
	if dep.Files[0].LocalStatus != types.DriftStatusUnchanged {
		t.Errorf("expected unchanged local status, got %q", dep.Files[0].LocalStatus)
	}
	if dep.Files[0].UpstreamStatus != types.DriftStatusUnchanged {
		t.Errorf("expected unchanged upstream status, got %q", dep.Files[0].UpstreamStatus)
	}
	if result.Summary.Result != types.DriftResultClean {
		t.Errorf("expected CLEAN result, got %q", result.Summary.Result)
	}
}

func TestDrift_HappyPath_LocalModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2\nline3"
	modified := "line1\nchanged\nline3"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{"lib/file.go": modified},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	expectGitOpsForDrift(t, fs, gitClient, cloneDir, false)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if dep.Files[0].LocalStatus != types.DriftStatusModified {
		t.Errorf("expected modified, got %q", dep.Files[0].LocalStatus)
	}
	if dep.Files[0].LocalLinesAdded != 1 {
		t.Errorf("expected 1 line added, got %d", dep.Files[0].LocalLinesAdded)
	}
	if dep.Files[0].LocalLinesRemoved != 1 {
		t.Errorf("expected 1 line removed, got %d", dep.Files[0].LocalLinesRemoved)
	}
	if dep.DriftScore == 0 {
		t.Error("expected non-zero drift score for modified file")
	}
	if result.Summary.Result != types.DriftResultDrifted {
		t.Errorf("expected DRIFTED, got %q", result.Summary.Result)
	}
}

func TestDrift_HappyPath_LocalDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2"
	// No local file — simulates deletion
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{}, // empty = no local files
	)
	defer cleanup()

	localPath := filepath.Join(workDir, "lib/file.go")

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: localPath}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	expectGitOpsForDrift(t, fs, gitClient, cloneDir, false)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if dep.Files[0].LocalStatus != types.DriftStatusDeleted {
		t.Errorf("expected deleted, got %q", dep.Files[0].LocalStatus)
	}
	if dep.Files[0].LocalDriftPct != 100 {
		t.Errorf("expected 100%% drift for deleted file, got %.1f", dep.Files[0].LocalDriftPct)
	}
}

func TestDrift_HappyPath_UpstreamModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2\nline3"
	upstream := "line1\nupstream-change\nline3"

	// cloneDir starts with original files (locked commit)
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{"lib/file.go": original}, // local unchanged
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	fs.EXPECT().CreateTemp("", "drift-*").Return(cloneDir, nil)
	fs.EXPECT().RemoveAll(cloneDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), cloneDir).Return(nil)
	gitClient.EXPECT().AddRemote(gomock.Any(), cloneDir, "origin", gomock.Any()).Return(nil)
	gitClient.EXPECT().Fetch(gomock.Any(), cloneDir, "origin", 0, gomock.Any()).Return(nil)

	// First checkout: locked commit (files already written as original)
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "abc1234567890").Return(nil)

	// Second checkout: FETCH_HEAD (overwrite source file with upstream content)
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "FETCH_HEAD").
		DoAndReturn(func(_ context.Context, dir, _ string) error {
			// Simulate upstream checkout by overwriting the source file
			return os.WriteFile(filepath.Join(dir, "src/file.go"), []byte(upstream), 0644)
		})
	gitClient.EXPECT().GetHeadHash(gomock.Any(), cloneDir).Return("latest999999", nil)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if dep.Files[0].LocalStatus != types.DriftStatusUnchanged {
		t.Errorf("expected local unchanged, got %q", dep.Files[0].LocalStatus)
	}
	if dep.Files[0].UpstreamStatus != types.DriftStatusModified {
		t.Errorf("expected upstream modified, got %q", dep.Files[0].UpstreamStatus)
	}
	if dep.UpstreamDrift.FilesChanged != 1 {
		t.Errorf("expected 1 upstream file changed, got %d", dep.UpstreamDrift.FilesChanged)
	}
	if result.Summary.DriftedUpstream != 1 {
		t.Errorf("expected 1 upstream-drifted dep, got %d", result.Summary.DriftedUpstream)
	}
}

func TestDrift_ConflictRisk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2\nline3"
	localMod := "line1\nlocal-change\nline3"
	upstream := "line1\nupstream-change\nline3"

	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{"lib/file.go": localMod},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	fs.EXPECT().CreateTemp("", "drift-*").Return(cloneDir, nil)
	fs.EXPECT().RemoveAll(cloneDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), cloneDir).Return(nil)
	gitClient.EXPECT().AddRemote(gomock.Any(), cloneDir, "origin", gomock.Any()).Return(nil)
	gitClient.EXPECT().Fetch(gomock.Any(), cloneDir, "origin", 0, gomock.Any()).Return(nil)
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "abc1234567890").Return(nil)
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "FETCH_HEAD").
		DoAndReturn(func(_ context.Context, dir, _ string) error {
			return os.WriteFile(filepath.Join(dir, "src/file.go"), []byte(upstream), 0644)
		})
	gitClient.EXPECT().GetHeadHash(gomock.Any(), cloneDir).Return("latest999999", nil)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if !dep.HasConflictRisk {
		t.Error("expected conflict risk for dep with both local and upstream changes")
	}
	if !dep.Files[0].HasConflictRisk {
		t.Error("expected conflict risk on file with both local and upstream changes")
	}
	if result.Summary.Result != types.DriftResultConflict {
		t.Errorf("expected CONFLICT, got %q", result.Summary.Result)
	}
	if result.Summary.ConflictRisk != 1 {
		t.Errorf("expected 1 conflict risk, got %d", result.Summary.ConflictRisk)
	}
}

func TestDrift_OfflineMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2"
	modified := "line1\nchanged"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{"lib/file.go": modified},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	// Offline: no FETCH_HEAD checkout, no GetHeadHash
	expectGitOpsForDrift(t, fs, gitClient, cloneDir, true)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{Offline: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if dep.LatestCommit != "" {
		t.Errorf("expected empty latest commit in offline mode, got %q", dep.LatestCommit)
	}
	if dep.Files[0].UpstreamStatus != "" {
		t.Errorf("expected empty upstream status in offline mode, got %q", dep.Files[0].UpstreamStatus)
	}
	if dep.Files[0].LocalStatus != types.DriftStatusModified {
		t.Errorf("expected modified local status, got %q", dep.Files[0].LocalStatus)
	}
}

func TestDrift_DetailMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := "line1\nline2\nline3"
	modified := "line1\nchanged\nline3"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": original},
		map[string]string{"lib/file.go": modified},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	expectGitOpsForDrift(t, fs, gitClient, cloneDir, true)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{Offline: true, Detail: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	if dep.Files[0].DiffOutput == "" {
		t.Error("expected non-empty diff output in detail mode")
	}
	if !contains(dep.Files[0].DiffOutput, "-line2") {
		t.Error("diff output missing removed line")
	}
	if !contains(dep.Files[0].DiffOutput, "+changed") {
		t.Error("diff output missing added line")
	}
}

func TestDrift_AutoPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	content := "hello"
	// Use a unique filename that won't exist in the CWD
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/drift_auto_test_unique_xyzzy.go": content},
		map[string]string{}, // auto-path file won't exist at computed path
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	// Empty To field → should use ComputeAutoPath
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "auto-vendor",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/drift_auto_test_unique_xyzzy.go", To: ""}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "auto-vendor", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	expectGitOpsForDrift(t, fs, gitClient, cloneDir, true)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{Offline: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := result.Dependencies[0]
	// Auto-path for "src/drift_auto_test_unique_xyzzy.go" with empty defaultTarget → "drift_auto_test_unique_xyzzy.go"
	if dep.Files[0].Path != "drift_auto_test_unique_xyzzy.go" {
		t.Errorf("expected auto-path 'drift_auto_test_unique_xyzzy.go', got %q", dep.Files[0].Path)
	}
	// File doesn't exist locally (unique name), so should be "deleted"
	if dep.Files[0].LocalStatus != types.DriftStatusDeleted {
		t.Errorf("expected deleted (file doesn't exist at auto-path), got %q", dep.Files[0].LocalStatus)
	}
}

// ============================================================================
// Git error path tests
// ============================================================================

func TestDrift_CreateTempError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "file.go", To: "dest/file.go"}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test", Ref: "main", CommitHash: "abc123"}},
	}, nil)

	fs.EXPECT().CreateTemp("", "drift-*").Return("", fmt.Errorf("disk full"))

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})
	if err == nil {
		t.Fatal("expected error from CreateTemp failure")
	}
	if !contains(err.Error(), "create temp dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_GitInitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "file.go", To: "dest/file.go"}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test", Ref: "main", CommitHash: "abc123"}},
	}, nil)

	tmpDir := t.TempDir()
	fs.EXPECT().CreateTemp("", "drift-*").Return(tmpDir, nil)
	fs.EXPECT().RemoveAll(tmpDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), tmpDir).Return(fmt.Errorf("init failed"))

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})
	if err == nil {
		t.Fatal("expected error from git init failure")
	}
	if !contains(err.Error(), "init temp repo") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_FetchWithFallbackFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "file.go", To: "dest/file.go"}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test", Ref: "main", CommitHash: "abc123"}},
	}, nil)

	tmpDir := t.TempDir()
	fs.EXPECT().CreateTemp("", "drift-*").Return(tmpDir, nil)
	fs.EXPECT().RemoveAll(tmpDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), tmpDir).Return(nil)
	// FetchWithFallback: AddRemote succeeds but Fetch fails
	gitClient.EXPECT().AddRemote(gomock.Any(), tmpDir, "origin", gomock.Any()).Return(nil)
	gitClient.EXPECT().Fetch(gomock.Any(), tmpDir, "origin", 0, "main").Return(fmt.Errorf("fetch ref failed"))

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})
	if err == nil {
		t.Fatal("expected error when FetchWithFallback fails")
	}
	if !contains(err.Error(), "fetch ref") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_CheckoutLockedCommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "file.go", To: "dest/file.go"}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	tmpDir := t.TempDir()
	fs.EXPECT().CreateTemp("", "drift-*").Return(tmpDir, nil)
	fs.EXPECT().RemoveAll(tmpDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), tmpDir).Return(nil)
	gitClient.EXPECT().AddRemote(gomock.Any(), tmpDir, "origin", gomock.Any()).Return(nil)
	gitClient.EXPECT().Fetch(gomock.Any(), tmpDir, "origin", 0, gomock.Any()).Return(nil)
	gitClient.EXPECT().Checkout(gomock.Any(), tmpDir, "abc1234567890").Return(fmt.Errorf("bad commit"))

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, "/root")
	_, err := svc.Drift(context.Background(), DriftOptions{})
	if err == nil {
		t.Fatal("expected error from checkout failure")
	}
	if !contains(err.Error(), "checkout locked commit") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrift_UpstreamCheckoutFallbackToOffline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	content := "line1\nline2"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": content},
		map[string]string{"lib/file.go": content},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{
			Name: "test-lib",
			URL:  "https://github.com/owner/repo",
			Specs: []types.BranchSpec{{
				Ref:     "main",
				Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
			}},
		}},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "test-lib", Ref: "main", CommitHash: "abc1234567890"}},
	}, nil)

	fs.EXPECT().CreateTemp("", "drift-*").Return(cloneDir, nil)
	fs.EXPECT().RemoveAll(cloneDir).Return(nil)
	gitClient.EXPECT().Init(gomock.Any(), cloneDir).Return(nil)
	gitClient.EXPECT().AddRemote(gomock.Any(), cloneDir, "origin", gomock.Any()).Return(nil)
	gitClient.EXPECT().Fetch(gomock.Any(), cloneDir, "origin", 0, gomock.Any()).Return(nil)
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "abc1234567890").Return(nil)
	// Both upstream checkout attempts fail → graceful fallback to offline
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "FETCH_HEAD").Return(fmt.Errorf("no FETCH_HEAD"))
	gitClient.EXPECT().Checkout(gomock.Any(), cloneDir, "origin/main").Return(fmt.Errorf("no origin/main"))

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}

	dep := result.Dependencies[0]
	// Should have local drift info but no upstream info
	if dep.LatestCommit != "" {
		t.Errorf("expected empty latest commit after fallback, got %q", dep.LatestCommit)
	}
	if dep.Files[0].UpstreamStatus != "" {
		t.Errorf("expected empty upstream status after fallback, got %q", dep.Files[0].UpstreamStatus)
	}
}

func TestDrift_MultipleVendorsFilterOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	content := "hello"
	cloneDir, workDir, cleanup := setupDriftTestFiles(t,
		map[string]string{"src/file.go": content},
		map[string]string{"lib/file.go": content},
	)
	defer cleanup()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	gitClient := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)

	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "vendor-a",
				URL:  "https://github.com/a/a",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/file.go", To: filepath.Join(workDir, "lib/file.go")}},
				}},
			},
			{
				Name: "vendor-b",
				URL:  "https://github.com/b/b",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "src/other.go", To: "lib/other.go"}},
				}},
			},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa1234567890"},
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb1234567890"},
		},
	}, nil)

	// Only vendor-a should be processed
	expectGitOpsForDrift(t, fs, gitClient, cloneDir, true)

	svc := NewDriftService(configStore, lockStore, gitClient, fs, nil, workDir)
	result, err := svc.Drift(context.Background(), DriftOptions{Dependency: "vendor-a", Offline: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency (filtered), got %d", len(result.Dependencies))
	}
	if result.Dependencies[0].Name != "vendor-a" {
		t.Errorf("expected vendor-a, got %q", result.Dependencies[0].Name)
	}
}

func TestDrift_VendorInConfigNotInLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)

	// Config has vendor, lock has different vendor
	configStore.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "unlocked-vendor",
				URL:  "https://github.com/owner/repo",
				Specs: []types.BranchSpec{{
					Ref:     "main",
					Mapping: []types.PathMapping{{From: "file.go", To: "dest/file.go"}},
				}},
			},
		},
	}, nil)
	lockStore.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "other-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	svc := NewDriftService(configStore, lockStore, nil, nil, nil, "/root")
	result, err := svc.Drift(context.Background(), DriftOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unlocked vendor should be skipped, producing zero dependencies
	if len(result.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies for unlocked vendor, got %d", len(result.Dependencies))
	}
	if result.Summary.Result != types.DriftResultClean {
		t.Errorf("expected CLEAN for zero deps, got %q", result.Summary.Result)
	}
}

// contains and findSubstring helpers are defined in testhelpers.go
