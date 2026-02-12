package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Compliance Check Tests
// ============================================================================

// complianceMocks bundles the mocks used by ComplianceService tests.
type complianceMocks struct {
	ctrl   *gomock.Controller
	config *MockConfigStore
	lock   *MockLockStore
	fs     *MockFileSystem
	cache  *mockCacheStore
}

func setupComplianceMocks(t *testing.T) complianceMocks {
	ctrl := gomock.NewController(t)
	return complianceMocks{
		ctrl:   ctrl,
		config: NewMockConfigStore(ctrl),
		lock:   NewMockLockStore(ctrl),
		fs:     NewMockFileSystem(ctrl),
		cache:  newMockCacheStore(),
	}
}

func newComplianceService(m complianceMocks) *ComplianceService {
	return NewComplianceService(m.config, m.lock, m.cache, m.fs, "/test")
}

// makeInternalConfig creates a VendorConfig with one internal vendor.
func makeInternalConfig(name, compliance, srcPath, destPath string) types.VendorConfig {
	return types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       name,
				Source:     SourceInternal,
				Compliance: compliance,
				Specs: []types.BranchSpec{
					{
						Ref: RefLocal,
						Mapping: []types.PathMapping{
							{From: srcPath, To: destPath},
						},
					},
				},
			},
		},
	}
}

// makeInternalLock creates a VendorLock with one internal vendor entry.
func makeInternalLock(name, srcPath, srcHash, destPath, destHash string) types.VendorLock {
	return types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:   name,
				Ref:    RefLocal,
				Source: SourceInternal,
				SourceFileHashes: map[string]string{
					srcPath: srcHash,
				},
				FileHashes: map[string]string{
					destPath: destHash,
				},
			},
		},
	}
}

func TestComplianceCheck_SourceCanonical_SourceDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Source changed, dest unchanged
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "locked_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	entry := result.Entries[0]
	if entry.Direction != types.DriftSourceDrift {
		t.Errorf("expected direction source_drifted, got %s", entry.Direction)
	}
	if entry.Action != "propagate source → dest" {
		t.Errorf("expected action 'propagate source → dest', got %q", entry.Action)
	}
	if result.Summary.Result != "DRIFTED" {
		t.Errorf("expected summary result DRIFTED, got %s", result.Summary.Result)
	}
}

func TestComplianceCheck_SourceCanonical_DestDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Source unchanged, dest changed
	m.cache.files["pkg/types.go"] = "locked_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftDestDrift {
		t.Errorf("expected direction dest_drifted, got %s", entry.Direction)
	}
	if entry.Action != "warning: dest modified (source-canonical)" {
		t.Errorf("expected warning action, got %q", entry.Action)
	}
}

func TestComplianceCheck_SourceCanonical_BothDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Both changed
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftBothDrift {
		t.Errorf("expected direction both_drifted, got %s", entry.Direction)
	}
	if entry.Action != "conflict: manual resolution required" {
		t.Errorf("expected conflict action, got %q", entry.Action)
	}
	if result.Summary.Result != "CONFLICT" {
		t.Errorf("expected summary result CONFLICT, got %s", result.Summary.Result)
	}
}

func TestComplianceCheck_Bidirectional_SourceDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceBidirectional, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Source changed, dest unchanged
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "locked_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftSourceDrift {
		t.Errorf("expected direction source_drifted, got %s", entry.Direction)
	}
	// Bidirectional source drift should propagate source → dest (same as source-canonical)
	if entry.Action != "propagate source → dest" {
		t.Errorf("expected 'propagate source → dest', got %q", entry.Action)
	}
}

func TestComplianceCheck_Bidirectional_BothDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceBidirectional, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Both changed
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftBothDrift {
		t.Errorf("expected direction both_drifted, got %s", entry.Direction)
	}
	if entry.Action != "conflict: manual resolution required" {
		t.Errorf("expected conflict action, got %q", entry.Action)
	}
	if result.Summary.Result != "CONFLICT" {
		t.Errorf("expected summary result CONFLICT, got %s", result.Summary.Result)
	}
}

func TestComplianceCheck_Bidirectional_DestDrifted(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceBidirectional, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Source unchanged, dest changed
	m.cache.files["pkg/types.go"] = "locked_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftDestDrift {
		t.Errorf("expected direction dest_drifted, got %s", entry.Direction)
	}
	if entry.Action != "propagate dest → source" {
		t.Errorf("expected 'propagate dest → source', got %q", entry.Action)
	}
}

func TestComplianceCheck_AllSynced(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Nothing changed
	m.cache.files["pkg/types.go"] = "locked_hash"
	m.cache.files["internal/types.go"] = "locked_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Direction != types.DriftSynced {
		t.Errorf("expected direction synced, got %s", entry.Direction)
	}
	if result.Summary.Result != "SYNCED" {
		t.Errorf("expected summary result SYNCED, got %s", result.Summary.Result)
	}
	if result.Summary.Synced != 1 {
		t.Errorf("expected 1 synced, got %d", result.Summary.Synced)
	}
}

// ============================================================================
// Compliance Propagation Tests
// ============================================================================

func TestCompliancePropagate_DryRun_ReportsWouldPropagate(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Source drifted
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "locked_dest_hash"

	// No fs.MkdirAll or os.WriteFile expectations → will fail if called

	svc := newComplianceService(m)
	result, err := svc.Propagate(ComplianceOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Propagate(DryRun) error: %v", err)
	}

	entry := result.Entries[0]
	if !contains(entry.Action, "would") {
		t.Errorf("expected 'would' prefix in dry-run action, got %q", entry.Action)
	}
}

func TestCompliancePropagate_SourceCanonical_SourceDrifted_CopiesToDest(t *testing.T) {
	// Source-canonical mode: source changed → propagate source to dest.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")
	if err := os.WriteFile(srcFile, []byte("new source content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFile, []byte("old dest content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Check call
	configStore.EXPECT().Load().Return(
		makeInternalConfig("test-vendor", ComplianceSourceCanonical, srcFile, destFile), nil)
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "old_src_hash", destFile, "locked_dest_hash"), nil)

	// Source drifted, dest unchanged
	cache.files[srcFile] = "new_src_hash"
	cache.files[destFile] = "locked_dest_hash"

	// Propagation: will write to destFile
	fs.EXPECT().MkdirAll(filepath.Dir(destFile), os.FileMode(0755)).Return(nil)

	// updateLockfileHashes: load + save
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "old_src_hash", destFile, "locked_dest_hash"), nil)
	lockStore.EXPECT().Save(gomock.Any()).Return(nil)

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	result, err := svc.Propagate(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Propagate() error: %v", err)
	}

	if result.Entries[0].Direction != types.DriftSourceDrift {
		t.Errorf("expected source_drifted direction, got %s", result.Entries[0].Direction)
	}

	// Verify the file was actually copied (os.ReadFile/WriteFile in copyFile)
	destContent, readErr := os.ReadFile(destFile)
	if readErr != nil {
		t.Fatalf("read dest file: %v", readErr)
	}
	if string(destContent) != "new source content\n" {
		t.Errorf("expected dest to contain source content, got %q", string(destContent))
	}
}

func TestCompliancePropagate_Bidirectional_DestDrifted_CopiesToSource(t *testing.T) {
	// Bidirectional mode: dest changed → propagate dest back to source.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")
	if err := os.WriteFile(srcFile, []byte("old source content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFile, []byte("new dest content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Check call
	configStore.EXPECT().Load().Return(
		makeInternalConfig("test-vendor", ComplianceBidirectional, srcFile, destFile), nil)
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "locked_src_hash", destFile, "old_dest_hash"), nil)

	// Source unchanged, dest drifted
	cache.files[srcFile] = "locked_src_hash"
	cache.files[destFile] = "new_dest_hash"

	// Propagation: dest → source. Will write to srcFile
	fs.EXPECT().MkdirAll(filepath.Dir(srcFile), os.FileMode(0755)).Return(nil)

	// updateLockfileHashes: load + save
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "locked_src_hash", destFile, "old_dest_hash"), nil)
	lockStore.EXPECT().Save(gomock.Any()).Return(nil)

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	result, err := svc.Propagate(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Propagate() error: %v", err)
	}

	if result.Entries[0].Direction != types.DriftDestDrift {
		t.Errorf("expected dest_drifted, got %s", result.Entries[0].Direction)
	}

	// Verify the source file was overwritten with dest content
	srcContent, readErr := os.ReadFile(srcFile)
	if readErr != nil {
		t.Fatalf("read src file: %v", readErr)
	}
	if string(srcContent) != "new dest content\n" {
		t.Errorf("expected source to contain dest content, got %q", string(srcContent))
	}
}

func TestCompliancePropagate_BothDrifted_ReturnsConflictError(t *testing.T) {
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", ComplianceSourceCanonical, "pkg/types.go", "internal/types.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Both drifted
	m.cache.files["pkg/types.go"] = "new_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	_, err := svc.Propagate(ComplianceOptions{})
	if err == nil {
		t.Fatal("expected error for both-drifted conflict, got nil")
	}
	if !contains(err.Error(), "propagation errors") {
		t.Errorf("expected propagation errors message, got: %v", err)
	}
	if !contains(err.Error(), "Compliance conflict") {
		t.Errorf("expected ComplianceConflictError in message, got: %v", err)
	}
}

// ============================================================================
// Position Spec Auto-Update Tests
// ============================================================================

func TestCompliancePositionUpdate_LineRangeExpandsWithDelta(t *testing.T) {
	// Source has L5-L20 position spec within the SAME vendor.
	// Propagation adds 5 lines to dest → EndLine adjusts from L20 to L25.
	// updatePositionSpecs only looks within vendorName=entry.VendorName.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")

	// Old dest: 20 lines
	oldContent := makeNLines(20)
	if err := os.WriteFile(destFile, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// New source: 25 lines (will replace dest, adding 5 lines)
	newContent := makeNLines(25)
	if err := os.WriteFile(srcFile, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// The vendor has two mappings:
	// 1. srcFile → destFile (standard copy, triggers propagation)
	// 2. destFile:L5-L20 → other.go (position spec that should be auto-updated)
	// Both must be in the SAME vendor for updatePositionSpecs to find the position spec.
	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "pos-vendor",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{
					{
						Ref: RefLocal,
						Mapping: []types.PathMapping{
							{From: srcFile, To: destFile},
							{From: destFile + ":L5-L20", To: "other.go"},
						},
					},
				},
			},
		},
	}

	lock := makeInternalLock("pos-vendor", srcFile, "old_src_hash", destFile, "old_dest_hash")

	// Check call
	configStore.EXPECT().Load().Return(config, nil)
	lockStore.EXPECT().Load().Return(lock, nil)

	// Source drifted, dest unchanged
	cache.files[srcFile] = "new_src_hash"
	cache.files[destFile] = "old_dest_hash"

	// Propagation writes to destFile
	fs.EXPECT().MkdirAll(filepath.Dir(destFile), os.FileMode(0755)).Return(nil)

	// updatePositionSpecs: configStore.Load + configStore.Save
	configStore.EXPECT().Load().Return(config, nil)
	var savedConfig types.VendorConfig
	configStore.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		savedConfig = cfg
		return nil
	})

	// updateLockfileHashes: lockStore.Load + lockStore.Save
	lockStore.EXPECT().Load().Return(lock, nil)
	lockStore.EXPECT().Save(gomock.Any()).Return(nil)

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	_, err := svc.Propagate(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Propagate() error: %v", err)
	}

	// Check the position spec was updated from L5-L20 to L5-L25
	if len(savedConfig.Vendors) != 1 {
		t.Fatalf("expected 1 vendor in saved config, got %d", len(savedConfig.Vendors))
	}
	mapping := savedConfig.Vendors[0].Specs[0].Mapping[1] // second mapping has the position
	expected := fmt.Sprintf("%s:L5-L25", destFile)
	if mapping.From != expected {
		t.Errorf("expected position updated to %q, got %q", expected, mapping.From)
	}
}

func TestCompliancePositionUpdate_ToEOFNoChange(t *testing.T) {
	// ToEOF specs auto-expand; no position update needed.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")

	oldContent := makeNLines(10)
	if err := os.WriteFile(destFile, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}
	newContent := makeNLines(15)
	if err := os.WriteFile(srcFile, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "pos-vendor",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{
					{Ref: RefLocal, Mapping: []types.PathMapping{
						{From: srcFile, To: destFile},
						{From: destFile + ":L5-EOF", To: "other.go"},
					}},
				},
			},
		},
	}

	lock := makeInternalLock("pos-vendor", srcFile, "old_src_hash", destFile, "old_dest_hash")

	configStore.EXPECT().Load().Return(config, nil)
	lockStore.EXPECT().Load().Return(lock, nil)

	cache.files[srcFile] = "new_src_hash"
	cache.files[destFile] = "old_dest_hash"

	fs.EXPECT().MkdirAll(filepath.Dir(destFile), os.FileMode(0755)).Return(nil)

	// updatePositionSpecs loads config. ToEOF should NOT trigger a Save.
	configStore.EXPECT().Load().Return(config, nil)
	// No configStore.Save expected for ToEOF

	lockStore.EXPECT().Load().Return(lock, nil)
	lockStore.EXPECT().Save(gomock.Any()).Return(nil)

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	_, err := svc.Propagate(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Propagate() error: %v", err)
	}
	// Success: no config save means ToEOF was correctly skipped
}

func TestCompliancePositionUpdate_NegativeDeltaShrinksPastStartLine(t *testing.T) {
	// If negative delta makes EndLine < StartLine, expect error.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")

	// Old dest: 20 lines → new source: 3 lines. Delta = negative.
	// Position destFile:L5-L20 → EndLine 20 + delta < 5 → error.
	oldContent := makeNLines(20)
	if err := os.WriteFile(destFile, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}
	newContent := makeNLines(3)
	if err := os.WriteFile(srcFile, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "shrink-vendor",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{
					{Ref: RefLocal, Mapping: []types.PathMapping{
						{From: srcFile, To: destFile},
						{From: destFile + ":L5-L20", To: "other.go"},
					}},
				},
			},
		},
	}

	lock := makeInternalLock("shrink-vendor", srcFile, "old_src_hash", destFile, "old_dest_hash")

	// Check() call
	configStore.EXPECT().Load().Return(config, nil)
	lockStore.EXPECT().Load().Return(lock, nil)

	cache.files[srcFile] = "new_src_hash"
	cache.files[destFile] = "old_dest_hash"

	// propagateEntry: copyFile writes to destFile
	fs.EXPECT().MkdirAll(filepath.Dir(destFile), os.FileMode(0755)).Return(nil)

	// updatePositionSpecs loads config, finds the error, returns without Save
	configStore.EXPECT().Load().Return(config, nil)
	// No configStore.Save — should error before save
	// No lockStore.Load for updateLockfileHashes — early return due to propagation error

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	_, err := svc.Propagate(ComplianceOptions{})
	if err == nil {
		t.Fatal("expected error for negative delta shrinking past StartLine, got nil")
	}
	if !contains(err.Error(), "EndLine") || !contains(err.Error(), "StartLine") {
		t.Errorf("expected EndLine/StartLine error, got: %v", err)
	}
}

func TestComplianceCheck_DefaultsToSourceCanonical(t *testing.T) {
	// Empty compliance field defaults to source-canonical.
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("api-types", "", "pkg/types.go", "internal/types.go"), nil) // empty compliance
	m.lock.EXPECT().Load().Return(
		makeInternalLock("api-types", "pkg/types.go", "locked_src_hash", "internal/types.go", "locked_dest_hash"), nil)

	// Dest drifted → should get source-canonical behavior (warning, not propagate)
	m.cache.files["pkg/types.go"] = "locked_src_hash"
	m.cache.files["internal/types.go"] = "new_dest_hash"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	entry := result.Entries[0]
	if entry.Compliance != ComplianceSourceCanonical {
		t.Errorf("expected compliance source-canonical, got %q", entry.Compliance)
	}
	if entry.Action != "warning: dest modified (source-canonical)" {
		t.Errorf("expected warning action, got %q", entry.Action)
	}
}

func TestComplianceCheck_VendorNameFilter(t *testing.T) {
	// VendorName filter scopes to a specific vendor.
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "vendor-a", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: "a.go", To: "dest-a.go"}}},
			}},
			{Name: "vendor-b", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: "b.go", To: "dest-b.go"}}},
			}},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: RefLocal, Source: SourceInternal,
				SourceFileHashes: map[string]string{"a.go": "ah"},
				FileHashes:       map[string]string{"dest-a.go": "adh"}},
			{Name: "vendor-b", Ref: RefLocal, Source: SourceInternal,
				SourceFileHashes: map[string]string{"b.go": "bh"},
				FileHashes:       map[string]string{"dest-b.go": "bdh"}},
		},
	}

	m.config.EXPECT().Load().Return(config, nil)
	m.lock.EXPECT().Load().Return(lock, nil)

	m.cache.files["a.go"] = "ah"
	m.cache.files["dest-a.go"] = "adh"
	m.cache.files["b.go"] = "new_bh"
	m.cache.files["dest-b.go"] = "bdh"

	svc := newComplianceService(m)
	result, err := svc.Check(ComplianceOptions{VendorName: "vendor-a"})
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}

	// Only vendor-a should be in results (which is synced)
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (vendor-a only), got %d", len(result.Entries))
	}
	if result.Entries[0].VendorName != "vendor-a" {
		t.Errorf("expected vendor-a, got %s", result.Entries[0].VendorName)
	}
	if result.Entries[0].Direction != types.DriftSynced {
		t.Errorf("expected synced, got %s", result.Entries[0].Direction)
	}
}

// ============================================================================
// Validation Tests for Internal Vendors
// ============================================================================

func TestValidateInternalVendor_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file for os.Stat check
	srcFile := filepath.Join(tmpDir, "src.go")
	if err := os.WriteFile(srcFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-lib",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{
					{
						Ref: RefLocal,
						Mapping: []types.PathMapping{
							{From: srcFile, To: "lib/dest.go"},
						},
					},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err != nil {
		t.Errorf("expected valid internal config, got error: %v", err)
	}
}

func TestValidateInternalVendor_RejectsURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:   "bad-internal",
				Source: SourceInternal,
				URL:    "https://github.com/org/repo",
				Specs:  []types.BranchSpec{{Ref: RefLocal, Mapping: []types.PathMapping{{From: "src.go", To: "dest.go"}}}},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for internal vendor with URL")
	}
	if !contains(err.Error(), "MUST NOT have a URL") {
		t.Errorf("expected URL validation error, got: %v", err)
	}
}

func TestValidateInternalVendor_RejectsNonLocalRef(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.go")
	if err := os.WriteFile(srcFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:   "bad-ref",
				Source: SourceInternal,
				Specs:  []types.BranchSpec{{Ref: "main", Mapping: []types.PathMapping{{From: srcFile, To: "dest.go"}}}},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for internal vendor with non-local ref")
	}
	if !contains(err.Error(), "MUST use ref") {
		t.Errorf("expected ref validation error, got: %v", err)
	}
}

func TestValidateInternalVendor_RejectsInvalidCompliance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "bad-compliance",
				Source:     SourceInternal,
				Compliance: "invalid-mode",
				Specs:      []types.BranchSpec{{Ref: RefLocal, Mapping: []types.PathMapping{{From: "src.go", To: "dest.go"}}}},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected error for invalid compliance mode")
	}
	if !contains(err.Error(), "compliance") {
		t.Errorf("expected compliance validation error, got: %v", err)
	}
}

// ============================================================================
// Cycle Detection Tests
// ============================================================================

func TestDetectInternalCycles_NoCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	// Create temp files so validateInternalVendor passes os.Stat checks
	tmpDir := t.TempDir()
	aFile := filepath.Join(tmpDir, "a.go")
	bFile := filepath.Join(tmpDir, "b.go")
	os.WriteFile(aFile, []byte("a"), 0644)
	os.WriteFile(bFile, []byte("b"), 0644)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "v1", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: aFile, To: bFile}}},
			}},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err != nil {
		t.Errorf("expected no cycle error, got: %v", err)
	}
}

func TestDetectInternalCycles_DirectCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	tmpDir := t.TempDir()
	aFile := filepath.Join(tmpDir, "a.go")
	bFile := filepath.Join(tmpDir, "b.go")
	os.WriteFile(aFile, []byte("a"), 0644)
	os.WriteFile(bFile, []byte("b"), 0644)

	// a → b and b → a: direct cycle
	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "v1", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: aFile, To: bFile}}},
			}},
			{Name: "v2", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: bFile, To: aFile}}},
			}},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !IsCycleError(err) && !contains(err.Error(), "Circular dependency") {
		t.Errorf("expected CycleError, got: %v", err)
	}
}

func TestDetectInternalCycles_TransitiveCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	tmpDir := t.TempDir()
	aFile := filepath.Join(tmpDir, "a.go")
	bFile := filepath.Join(tmpDir, "b.go")
	cFile := filepath.Join(tmpDir, "c.go")
	os.WriteFile(aFile, []byte("a"), 0644)
	os.WriteFile(bFile, []byte("b"), 0644)
	os.WriteFile(cFile, []byte("c"), 0644)

	// a → b, b → c, c → a: transitive cycle
	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "v1", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: aFile, To: bFile}}},
			}},
			{Name: "v2", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: bFile, To: cFile}}},
			}},
			{Name: "v3", Source: SourceInternal, Specs: []types.BranchSpec{
				{Ref: RefLocal, Mapping: []types.PathMapping{{From: cFile, To: aFile}}},
			}},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err == nil {
		t.Fatal("expected cycle error for transitive cycle, got nil")
	}
	if !contains(err.Error(), "Circular dependency") {
		t.Errorf("expected 'Circular dependency' in error, got: %v", err)
	}
}

func TestDetectInternalCycles_SkipsExternalVendors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfig := NewMockConfigStore(ctrl)

	tmpDir := t.TempDir()
	aFile := filepath.Join(tmpDir, "a.go")
	os.WriteFile(aFile, []byte("a"), 0644)

	// Mix of internal and external: external vendor should be ignored by cycle detection
	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:   "external-vendor",
				URL:    "https://github.com/org/repo",
				Source: "", // external
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "src.go", To: "lib/src.go"}}},
				},
			},
			{
				Name:   "internal-vendor",
				Source: SourceInternal,
				Specs: []types.BranchSpec{
					{Ref: RefLocal, Mapping: []types.PathMapping{{From: aFile, To: "dest.go"}}},
				},
			},
		},
	}, nil)

	svc := NewValidationService(mockConfig)
	err := svc.ValidateConfig()
	if err != nil {
		t.Errorf("expected no error for mixed internal/external, got: %v", err)
	}
}

// ============================================================================
// Verify Internal Entries Tests
// ============================================================================

func TestVerifyInternalEntries_DetectsDrift(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-lib",
				Source:     SourceInternal,
				Compliance: ComplianceSourceCanonical,
				Specs: []types.BranchSpec{
					{Ref: RefLocal, Mapping: []types.PathMapping{{From: "src.go", To: "dest.go"}}},
				},
			},
			{
				Name: "external-lib",
				URL:  "https://github.com/org/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "lib.go", To: "vendor/lib.go"}}},
				},
			},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "internal-lib",
				Ref:              RefLocal,
				Source:           SourceInternal,
				SourceFileHashes: map[string]string{"src.go": "locked_src"},
				FileHashes:       map[string]string{"dest.go": "locked_dest"},
			},
			{
				Name:       "external-lib",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{"vendor/lib.go": "ext_hash"},
			},
		},
	}

	// Source drifted
	cache.files["src.go"] = "new_src"
	cache.files["dest.go"] = "locked_dest"
	cache.files["vendor/lib.go"] = "ext_hash"

	configStore.EXPECT().Load().Return(config, nil)
	lockStore.EXPECT().Load().Return(lock, nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{isDir: false}, nil).AnyTimes()

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(nil)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	// External vendor file should be verified
	if result.Summary.Verified < 1 {
		t.Error("expected at least 1 verified file (external vendor)")
	}

	// Internal status should have source_drifted entry
	if len(result.InternalStatus) != 1 {
		t.Fatalf("expected 1 internal status entry, got %d", len(result.InternalStatus))
	}

	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftSourceDrift {
		t.Errorf("expected source_drifted, got %s", entry.Direction)
	}
	if entry.VendorName != "internal-lib" {
		t.Errorf("expected vendor name 'internal-lib', got %q", entry.VendorName)
	}
}

func TestCompliancePropagate_SourceCanonical_DestDrifted_ReverseCopiesToSource(t *testing.T) {
	// Source-canonical mode with --reverse: dest changed → propagate dest to source.
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.go")
	destFile := filepath.Join(tmpDir, "dest.go")
	if err := os.WriteFile(srcFile, []byte("old source content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destFile, []byte("new dest content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	// Check call
	configStore.EXPECT().Load().Return(
		makeInternalConfig("test-vendor", ComplianceSourceCanonical, srcFile, destFile), nil)
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "locked_src_hash", destFile, "old_dest_hash"), nil)

	// Source unchanged, dest drifted
	cache.files[srcFile] = "locked_src_hash"
	cache.files[destFile] = "new_dest_hash"

	// Propagation: with Reverse=true, should copy dest → source. Will write to srcFile
	fs.EXPECT().MkdirAll(filepath.Dir(srcFile), os.FileMode(0755)).Return(nil)

	// updateLockfileHashes: load + save
	lockStore.EXPECT().Load().Return(
		makeInternalLock("test-vendor", srcFile, "locked_src_hash", destFile, "old_dest_hash"), nil)
	lockStore.EXPECT().Save(gomock.Any()).Return(nil)

	svc := NewComplianceService(configStore, lockStore, cache, fs, tmpDir)
	result, err := svc.Propagate(ComplianceOptions{Reverse: true})
	if err != nil {
		t.Fatalf("Propagate(Reverse) error: %v", err)
	}

	if result.Entries[0].Direction != types.DriftDestDrift {
		t.Errorf("expected dest_drifted, got %s", result.Entries[0].Direction)
	}

	// Verify source file was overwritten with dest content
	srcContent, readErr := os.ReadFile(srcFile)
	if readErr != nil {
		t.Fatalf("read src file: %v", readErr)
	}
	if string(srcContent) != "new dest content\n" {
		t.Errorf("expected source to contain dest content, got %q", string(srcContent))
	}
}

func TestCompliancePropagate_SourceCanonical_DestDrifted_NoReverse_WarnsOnly(t *testing.T) {
	// Source-canonical without --reverse: dest changed → warn only, no copy.
	m := setupComplianceMocks(t)
	defer m.ctrl.Finish()

	m.config.EXPECT().Load().Return(
		makeInternalConfig("test-vendor", ComplianceSourceCanonical, "pkg/src.go", "internal/dest.go"), nil)
	m.lock.EXPECT().Load().Return(
		makeInternalLock("test-vendor", "pkg/src.go", "locked_src_hash", "internal/dest.go", "old_dest_hash"), nil)

	// Source unchanged, dest drifted
	m.cache.files["pkg/src.go"] = "locked_src_hash"
	m.cache.files["internal/dest.go"] = "new_dest_hash"

	// No fs.MkdirAll expectations → propagation should NOT write any files
	// updateLockfileHashes: still called since no errors
	m.lock.EXPECT().Load().Return(
		makeInternalLock("test-vendor", "pkg/src.go", "locked_src_hash", "internal/dest.go", "old_dest_hash"), nil)
	m.lock.EXPECT().Save(gomock.Any()).Return(nil)

	svc := newComplianceService(m)
	result, err := svc.Propagate(ComplianceOptions{Reverse: false})
	if err != nil {
		t.Fatalf("Propagate() error: %v", err)
	}

	// Entry should still show dest_drifted with warning action
	entry := result.Entries[0]
	if entry.Direction != types.DriftDestDrift {
		t.Errorf("expected dest_drifted, got %s", entry.Direction)
	}
}

func TestVerifyInternalEntries_MixedInternalExternal(t *testing.T) {
	// Verify with both internal and external vendors produces separate result streams.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configStore := NewMockConfigStore(ctrl)
	lockStore := NewMockLockStore(ctrl)
	fs := NewMockFileSystem(ctrl)
	cache := newMockCacheStore()

	config := types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name:       "internal-lib",
				Source:     SourceInternal,
				Compliance: ComplianceBidirectional,
				Specs: []types.BranchSpec{
					{Ref: RefLocal, Mapping: []types.PathMapping{{From: "src.go", To: "dest.go"}}},
				},
			},
			{
				Name: "external-lib",
				URL:  "https://github.com/org/repo",
				Specs: []types.BranchSpec{
					{Ref: "main", Mapping: []types.PathMapping{{From: "lib.go", To: "vendor/lib.go"}}},
				},
			},
		},
	}

	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "internal-lib",
				Ref:              RefLocal,
				Source:           SourceInternal,
				SourceFileHashes: map[string]string{"src.go": "src_locked"},
				FileHashes:       map[string]string{"dest.go": "dest_locked"},
			},
			{
				Name:       "external-lib",
				Ref:        "main",
				CommitHash: "abc123",
				FileHashes: map[string]string{"vendor/lib.go": "ext_hash"},
			},
		},
	}

	// Both synced — no drift
	cache.files["src.go"] = "src_locked"
	cache.files["dest.go"] = "dest_locked"
	cache.files["vendor/lib.go"] = "ext_hash"

	configStore.EXPECT().Load().Return(config, nil)
	lockStore.EXPECT().Load().Return(lock, nil)
	fs.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{isDir: false}, nil).AnyTimes()

	service := NewVerifyService(configStore, lockStore, cache, fs, "/test")
	result, err := service.Verify(nil)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	// External vendor file should be verified
	if result.Summary.Verified < 1 {
		t.Errorf("expected at least 1 verified file (external vendor), got %d", result.Summary.Verified)
	}

	// Internal status should have a synced entry
	if len(result.InternalStatus) != 1 {
		t.Fatalf("expected 1 internal status entry, got %d", len(result.InternalStatus))
	}
	entry := result.InternalStatus[0]
	if entry.Direction != types.DriftSynced {
		t.Errorf("expected synced, got %s", entry.Direction)
	}
	if entry.Compliance != ComplianceBidirectional {
		t.Errorf("expected bidirectional compliance, got %q", entry.Compliance)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// makeNLines generates a string with n lines (each line ends with \n).
func makeNLines(n int) string {
	var sb []byte
	for i := 1; i <= n; i++ {
		sb = append(sb, []byte(fmt.Sprintf("line-%d\n", i))...)
	}
	return string(sb)
}
