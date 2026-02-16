package core

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// Service stubs for VendorSyncer orchestration tests
// ============================================================================

// stubRepositoryService implements VendorRepositoryInterface for testing.
type stubRepositoryService struct {
	existsResult bool
	existsErr    error
	saveErr      error
	deleteErr    error
	config       types.VendorConfig
	configErr    error
}

func (s *stubRepositoryService) Find(_ string) (*types.VendorSpec, error) {
	return nil, nil
}

func (s *stubRepositoryService) FindAll() ([]types.VendorSpec, error) {
	return s.config.Vendors, s.configErr
}

func (s *stubRepositoryService) Exists(_ string) (bool, error) {
	return s.existsResult, s.existsErr
}

func (s *stubRepositoryService) Save(_ *types.VendorSpec) error {
	return s.saveErr
}

func (s *stubRepositoryService) Delete(_ string) error {
	return s.deleteErr
}

func (s *stubRepositoryService) GetConfig() (types.VendorConfig, error) {
	return s.config, s.configErr
}

// stubSyncService implements SyncServiceInterface for testing.
type stubSyncService struct {
	syncErr       error
	syncVendorErr error
}

func (s *stubSyncService) Sync(_ context.Context, _ SyncOptions) error {
	return s.syncErr
}

func (s *stubSyncService) SyncVendor(_ context.Context, _ *types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
	return nil, CopyStats{}, s.syncVendorErr
}

// stubUpdateService implements UpdateServiceInterface for testing.
type stubUpdateService struct {
	updateErr error
	callCount int
	lastOpts  UpdateOptions // Captures last UpdateOptions passed to UpdateAllWithOptions
}

func (s *stubUpdateService) UpdateAll(_ context.Context) error {
	s.callCount++
	return s.updateErr
}

func (s *stubUpdateService) UpdateAllWithOptions(_ context.Context, opts UpdateOptions) error {
	s.callCount++
	s.lastOpts = opts
	return s.updateErr
}

// stubValidationService implements ValidationServiceInterface for testing.
type stubValidationService struct {
	validateErr error
	conflicts   []types.PathConflict
	conflictErr error
}

func (s *stubValidationService) ValidateConfig() error {
	return s.validateErr
}

func (s *stubValidationService) DetectConflicts() ([]types.PathConflict, error) {
	return s.conflicts, s.conflictErr
}

// stubUpdateCheckerService implements UpdateCheckerInterface for testing.
type stubUpdateCheckerService struct {
	results []types.UpdateCheckResult
	err     error
}

func (s *stubUpdateCheckerService) CheckUpdates(_ context.Context) ([]types.UpdateCheckResult, error) {
	return s.results, s.err
}

// stubVerifyService implements VerifyServiceInterface for testing.
type stubVerifyService struct {
	result *types.VerifyResult
	err    error
}

func (s *stubVerifyService) Verify(_ context.Context) (*types.VerifyResult, error) {
	return s.result, s.err
}

// stubVulnScanner implements VulnScannerInterface for testing.
type stubVulnScanner struct {
	result *types.ScanResult
	err    error
}

func (s *stubVulnScanner) Scan(_ context.Context, _ string) (*types.ScanResult, error) {
	return s.result, s.err
}

func (s *stubVulnScanner) ClearCache() error {
	return nil
}

// newTestSyncer creates a VendorSyncer with all services stubbed via overrides.
func newTestSyncer(
	configStore ConfigStore,
	lockStore LockStore,
	fs FileSystem,
	overrides *ServiceOverrides,
) *VendorSyncer {
	return NewVendorSyncer(
		configStore,
		lockStore,
		nil, // gitClient not needed when services are overridden
		fs,
		nil, // licenseChecker not needed when services are overridden
		"/test/root",
		&SilentUICallback{},
		overrides,
	)
}

// ============================================================================
// VendorSyncer.Init tests
// ============================================================================

func TestVendorSyncer_Init_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockConfig := NewMockConfigStore(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(nil).Times(2)
	mockConfig.EXPECT().Save(types.VendorConfig{Vendors: []types.VendorSpec{}}).Return(nil)

	syncer := newTestSyncer(mockConfig, nil, mockFS, &ServiceOverrides{})

	err := syncer.Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
}

func TestVendorSyncer_Init_MkdirFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(errors.New("permission denied"))

	syncer := newTestSyncer(nil, nil, mockFS, &ServiceOverrides{})

	err := syncer.Init()
	if err == nil {
		t.Fatal("Init() expected error, got nil")
	}
	if !contains(err.Error(), "create vendor directory") {
		t.Errorf("Init() error = %q, want containing 'create vendor directory'", err.Error())
	}
}

func TestVendorSyncer_Init_ConfigSaveFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockConfig := NewMockConfigStore(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(nil).Times(2)
	mockConfig.EXPECT().Save(gomock.Any()).Return(errors.New("disk full"))

	syncer := newTestSyncer(mockConfig, nil, mockFS, &ServiceOverrides{})

	err := syncer.Init()
	if err == nil {
		t.Fatal("Init() expected error, got nil")
	}
	if !contains(err.Error(), "save initial config") {
		t.Errorf("Init() error = %q, want containing 'save initial config'", err.Error())
	}
}

func TestVendorSyncer_Init_SetsHooksPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockGit := NewMockGitClient(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(nil).Times(2)
	mockConfig.EXPECT().Save(types.VendorConfig{Vendors: []types.VendorSpec{}}).Return(nil)

	// .githooks/ exists in project root → ConfigSet should be called
	mockFS.EXPECT().Stat(".githooks").Return(nil, nil)
	mockGit.EXPECT().ConfigSet(gomock.Any(), ".", "core.hooksPath", ".githooks").Return(nil)

	syncer := NewVendorSyncer(mockConfig, nil, mockGit, mockFS, nil, ".git-vendor", &SilentUICallback{}, nil)

	err := syncer.Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
}

func TestVendorSyncer_Init_SkipsHooksWhenNoDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockGit := NewMockGitClient(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(nil).Times(2)
	mockConfig.EXPECT().Save(types.VendorConfig{Vendors: []types.VendorSpec{}}).Return(nil)

	// .githooks/ does NOT exist → ConfigSet should NOT be called
	mockFS.EXPECT().Stat(".githooks").Return(nil, os.ErrNotExist)

	syncer := NewVendorSyncer(mockConfig, nil, mockGit, mockFS, nil, ".git-vendor", &SilentUICallback{}, nil)

	err := syncer.Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
}

func TestVendorSyncer_Init_HookSetupFailureNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)
	mockConfig := NewMockConfigStore(ctrl)
	mockGit := NewMockGitClient(ctrl)

	mockFS.EXPECT().MkdirAll(gomock.Any(), os.FileMode(0755)).Return(nil).Times(2)
	mockConfig.EXPECT().Save(types.VendorConfig{Vendors: []types.VendorSpec{}}).Return(nil)

	// .githooks/ exists but ConfigSet fails → should NOT fail Init
	mockFS.EXPECT().Stat(".githooks").Return(nil, nil)
	mockGit.EXPECT().ConfigSet(gomock.Any(), ".", "core.hooksPath", ".githooks").Return(errors.New("git config failed"))

	syncer := NewVendorSyncer(mockConfig, nil, mockGit, mockFS, nil, ".git-vendor", &SilentUICallback{}, nil)

	err := syncer.Init()
	if err != nil {
		t.Fatalf("Init() should succeed even if hook setup fails, got: %v", err)
	}
}

// ============================================================================
// VendorSyncer.AddVendor tests
// ============================================================================

func TestVendorSyncer_AddVendor_NewVendor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	repo := &stubRepositoryService{existsResult: false}
	lic := &stubLicenseService{}
	update := &stubUpdateService{}

	// Save should be called on repository
	repo.saveErr = nil

	syncer := newTestSyncer(mockConfig, mockLock, nil, &ServiceOverrides{
		Repository: repo,
		License:    lic,
		Update:     update,
	})

	spec := &types.VendorSpec{
		Name: "new-vendor",
		URL:  "https://github.com/owner/repo",
	}

	err := syncer.AddVendor(spec)
	if err != nil {
		t.Fatalf("AddVendor() error = %v", err)
	}

	// License should have been set from CheckCompliance
	if spec.License != "MIT" {
		t.Errorf("Expected license 'MIT', got '%s'", spec.License)
	}
}

func TestVendorSyncer_AddVendor_ExistingVendor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := &stubRepositoryService{existsResult: true}
	update := &stubUpdateService{}

	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		Repository: repo,
		Update:     update,
	})

	spec := &types.VendorSpec{
		Name:    "existing-vendor",
		URL:     "https://github.com/owner/repo",
		License: "Apache-2.0",
	}

	err := syncer.AddVendor(spec)
	if err != nil {
		t.Fatalf("AddVendor() error = %v", err)
	}

	// License should remain unchanged for existing vendor (no compliance check)
	if spec.License != "Apache-2.0" {
		t.Errorf("Expected license 'Apache-2.0', got '%s'", spec.License)
	}
}

// ============================================================================
// VendorSyncer.RemoveVendor tests
// ============================================================================

func TestVendorSyncer_RemoveVendor_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := NewMockFileSystem(ctrl)

	repo := &stubRepositoryService{}
	update := &stubUpdateService{}

	mockFS.EXPECT().Remove(gomock.Any()).Return(nil) // license file removal

	syncer := newTestSyncer(nil, nil, mockFS, &ServiceOverrides{
		Repository: repo,
		Update:     update,
	})

	err := syncer.RemoveVendor("test-vendor")
	if err != nil {
		t.Fatalf("RemoveVendor() error = %v", err)
	}
}

func TestVendorSyncer_RemoveVendor_NotFound(t *testing.T) {
	repo := &stubRepositoryService{
		deleteErr: NewVendorNotFoundError("missing"),
	}

	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		Repository: repo,
	})

	err := syncer.RemoveVendor("missing")
	if err == nil {
		t.Fatal("RemoveVendor() expected error, got nil")
	}

	var vnf *VendorNotFoundError
	if !errors.As(err, &vnf) {
		t.Errorf("Expected VendorNotFoundError, got %T: %v", err, err)
	}
}

// ============================================================================
// VendorSyncer.Sync / SyncDryRun / SyncWithOptions tests
// ============================================================================

func TestVendorSyncer_Sync_WithExistingLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "v1", Ref: "main", CommitHash: "abc"},
		},
	}, nil)

	syncSvc := &stubSyncService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync: syncSvc,
	})

	err := syncer.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestVendorSyncer_Sync_NoLockfileRunsUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)

	mockLock.EXPECT().Load().Return(types.VendorLock{}, nil)

	syncSvc := &stubSyncService{}
	updateSvc := &stubUpdateService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync:   syncSvc,
		Update: updateSvc,
	})

	err := syncer.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestVendorSyncer_Sync_UpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)

	mockLock.EXPECT().Load().Return(types.VendorLock{}, nil)

	updateSvc := &stubUpdateService{updateErr: errors.New("network error")}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Update: updateSvc,
	})

	err := syncer.Sync(context.Background())
	if err == nil {
		t.Fatal("Sync() expected error when update fails")
	}
	if !contains(err.Error(), "generate lockfile") {
		t.Errorf("Sync() error = %q, want containing 'generate lockfile'", err.Error())
	}
}

func TestVendorSyncer_SyncDryRun_WithLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "abc"}},
	}, nil)

	syncSvc := &stubSyncService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync: syncSvc,
	})

	err := syncer.SyncDryRun(context.Background())
	if err != nil {
		t.Fatalf("SyncDryRun() error = %v", err)
	}
}

func TestVendorSyncer_SyncDryRun_NoLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{}, nil)

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{})

	err := syncer.SyncDryRun(context.Background())
	if err != nil {
		t.Fatalf("SyncDryRun() error = %v, want nil", err)
	}
}

func TestVendorSyncer_SyncWithOptions_WithLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "abc"}},
	}, nil)

	syncSvc := &stubSyncService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync: syncSvc,
	})

	err := syncer.SyncWithOptions(context.Background(), "v1", true, false)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}
}

func TestVendorSyncer_SyncWithGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "abc"}},
	}, nil)

	syncSvc := &stubSyncService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync: syncSvc,
	})

	err := syncer.SyncWithGroup(context.Background(), "frontend", true, false)
	if err != nil {
		t.Fatalf("SyncWithGroup() error = %v", err)
	}
}

func TestVendorSyncer_SyncWithParallel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "abc"}},
	}, nil)

	syncSvc := &stubSyncService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync: syncSvc,
	})

	err := syncer.SyncWithParallel(context.Background(), "", false, false, types.ParallelOptions{Enabled: true, MaxWorkers: 2})
	if err != nil {
		t.Fatalf("SyncWithParallel() error = %v", err)
	}
}

// ============================================================================
// VendorSyncer stale commit auto-recovery tests
// ============================================================================

func TestVendorSyncer_Sync_StaleCommitAutoUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "stale123"}},
	}, nil)

	syncSvc := &stubSyncService{
		syncErr: NewStaleCommitError("stale123", "v1", "main"),
	}
	updateSvc := &stubUpdateService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync:   syncSvc,
		Update: updateSvc,
	})

	err := syncer.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync() expected nil after auto-update, got: %v", err)
	}
	if updateSvc.callCount != 1 {
		t.Errorf("expected UpdateAll called once, got %d", updateSvc.callCount)
	}
}

func TestVendorSyncer_Sync_StaleCommitAutoUpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "stale123"}},
	}, nil)

	syncSvc := &stubSyncService{
		syncErr: NewStaleCommitError("stale123", "v1", "main"),
	}
	updateSvc := &stubUpdateService{updateErr: errors.New("network error")}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync:   syncSvc,
		Update: updateSvc,
	})

	err := syncer.Sync(context.Background())
	if err == nil {
		t.Fatal("Sync() expected error when auto-update fails")
	}
	if !contains(err.Error(), "auto-update after stale commit") {
		t.Errorf("Sync() error = %q, want containing 'auto-update after stale commit'", err.Error())
	}
}

func TestVendorSyncer_Sync_NonStaleErrorDoesNotAutoUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "abc"}},
	}, nil)

	syncSvc := &stubSyncService{syncErr: errors.New("network timeout")}
	updateSvc := &stubUpdateService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync:   syncSvc,
		Update: updateSvc,
	})

	err := syncer.Sync(context.Background())
	if err == nil {
		t.Fatal("Sync() expected error for non-stale failure")
	}
	if updateSvc.callCount != 0 {
		t.Errorf("expected UpdateAll NOT called for non-stale error, got %d calls", updateSvc.callCount)
	}
}

func TestVendorSyncer_SyncWithOptions_StaleCommitAutoUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{{Name: "v1", Ref: "main", CommitHash: "stale123"}},
	}, nil)

	syncSvc := &stubSyncService{
		syncErr: NewStaleCommitError("stale123", "v1", "main"),
	}
	updateSvc := &stubUpdateService{}

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{
		Sync:   syncSvc,
		Update: updateSvc,
	})

	err := syncer.SyncWithOptions(context.Background(), "v1", false, false)
	if err != nil {
		t.Fatalf("SyncWithOptions() expected nil after auto-update, got: %v", err)
	}
	if updateSvc.callCount != 1 {
		t.Errorf("expected UpdateAll called once, got %d", updateSvc.callCount)
	}
}

// ============================================================================
// VendorSyncer.CheckSyncStatus tests
// ============================================================================

func TestVendorSyncer_CheckSyncStatus_AllSynced(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/utils.go", To: "lib/utils.go"},
						},
					},
				},
			},
		},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	// File exists
	mockFS.EXPECT().Stat("lib/utils.go").Return(&mockFileInfo{name: "utils.go"}, nil)

	syncer := newTestSyncer(mockConfig, mockLock, mockFS, &ServiceOverrides{})

	status, err := syncer.CheckSyncStatus()
	if err != nil {
		t.Fatalf("CheckSyncStatus() error = %v", err)
	}

	if !status.AllSynced {
		t.Error("Expected AllSynced=true")
	}
	if len(status.VendorStatuses) != 1 {
		t.Fatalf("Expected 1 vendor status, got %d", len(status.VendorStatuses))
	}
	if !status.VendorStatuses[0].IsSynced {
		t.Error("Expected vendor to be synced")
	}
}

func TestVendorSyncer_CheckSyncStatus_MissingFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "test-vendor",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/utils.go", To: "lib/utils.go"},
						},
					},
				},
			},
		},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "test-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	// File does not exist
	mockFS.EXPECT().Stat("lib/utils.go").Return(nil, os.ErrNotExist)

	syncer := newTestSyncer(mockConfig, mockLock, mockFS, &ServiceOverrides{})

	status, err := syncer.CheckSyncStatus()
	if err != nil {
		t.Fatalf("CheckSyncStatus() error = %v", err)
	}

	if status.AllSynced {
		t.Error("Expected AllSynced=false")
	}
	if len(status.VendorStatuses) != 1 {
		t.Fatalf("Expected 1 vendor status, got %d", len(status.VendorStatuses))
	}
	vs := status.VendorStatuses[0]
	if vs.IsSynced {
		t.Error("Expected vendor to not be synced")
	}
	if len(vs.MissingPaths) != 1 || vs.MissingPaths[0] != "lib/utils.go" {
		t.Errorf("Expected missing path 'lib/utils.go', got %v", vs.MissingPaths)
	}
}

func TestVendorSyncer_CheckSyncStatus_AutoPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{
				Name: "my-vendor",
				Specs: []types.BranchSpec{
					{
						Ref: "main",
						Mapping: []types.PathMapping{
							{From: "src/util.go", To: ""}, // empty To = auto path
						},
					},
				},
			},
		},
	}, nil)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "my-vendor", Ref: "main", CommitHash: "abc123"},
		},
	}, nil)

	// Auto-path would compute destination based on source
	mockFS.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{name: "util.go"}, nil)

	syncer := newTestSyncer(mockConfig, mockLock, mockFS, &ServiceOverrides{})

	status, err := syncer.CheckSyncStatus()
	if err != nil {
		t.Fatalf("CheckSyncStatus() error = %v", err)
	}

	if !status.AllSynced {
		t.Error("Expected AllSynced=true with auto-path")
	}
}

func TestVendorSyncer_CheckSyncStatus_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockConfig.EXPECT().Load().Return(types.VendorConfig{}, errors.New("config not found"))

	syncer := newTestSyncer(mockConfig, nil, nil, &ServiceOverrides{})

	_, err := syncer.CheckSyncStatus()
	if err == nil {
		t.Fatal("CheckSyncStatus() expected error, got nil")
	}
}

func TestVendorSyncer_CheckSyncStatus_LockLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{}, nil)
	mockLock.EXPECT().Load().Return(types.VendorLock{}, errors.New("lock not found"))

	syncer := newTestSyncer(mockConfig, mockLock, nil, &ServiceOverrides{})

	_, err := syncer.CheckSyncStatus()
	if err == nil {
		t.Fatal("CheckSyncStatus() expected error, got nil")
	}
}

// ============================================================================
// VendorSyncer.MigrateLockfile tests
// ============================================================================

func TestVendorSyncer_MigrateLockfile_MigratesOldEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "old-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
				VendoredAt: "", // empty = needs migration
			},
		},
	}, nil)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "old-vendor", License: "MIT"},
		},
	}, nil)

	mockLock.EXPECT().Save(gomock.Any()).DoAndReturn(func(lock types.VendorLock) error {
		if len(lock.Vendors) != 1 {
			t.Errorf("Expected 1 vendor, got %d", len(lock.Vendors))
		}
		entry := lock.Vendors[0]
		if entry.VendoredAt != "2024-01-01T00:00:00Z" {
			t.Errorf("VendoredAt = %q, want '2024-01-01T00:00:00Z'", entry.VendoredAt)
		}
		if entry.VendoredBy != "unknown (migrated)" {
			t.Errorf("VendoredBy = %q, want 'unknown (migrated)'", entry.VendoredBy)
		}
		if entry.LastSyncedAt != "2024-01-01T00:00:00Z" {
			t.Errorf("LastSyncedAt = %q, want '2024-01-01T00:00:00Z'", entry.LastSyncedAt)
		}
		if entry.LicenseSPDX != "MIT" {
			t.Errorf("LicenseSPDX = %q, want 'MIT'", entry.LicenseSPDX)
		}
		return nil
	})

	syncer := newTestSyncer(mockConfig, mockLock, nil, &ServiceOverrides{})

	migrated, err := syncer.MigrateLockfile()
	if err != nil {
		t.Fatalf("MigrateLockfile() error = %v", err)
	}
	if migrated != 1 {
		t.Errorf("Expected 1 migrated, got %d", migrated)
	}
}

func TestVendorSyncer_MigrateLockfile_NothingToMigrate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := NewMockConfigStore(ctrl)
	mockLock := NewMockLockStore(ctrl)

	mockLock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "new-vendor",
				VendoredAt: "2024-06-01T00:00:00Z", // already migrated
			},
		},
	}, nil)

	mockConfig.EXPECT().Load().Return(types.VendorConfig{
		Vendors: []types.VendorSpec{{Name: "new-vendor"}},
	}, nil)

	// Save should NOT be called since nothing was migrated

	syncer := newTestSyncer(mockConfig, mockLock, nil, &ServiceOverrides{})

	migrated, err := syncer.MigrateLockfile()
	if err != nil {
		t.Fatalf("MigrateLockfile() error = %v", err)
	}
	if migrated != 0 {
		t.Errorf("Expected 0 migrated, got %d", migrated)
	}
}

func TestVendorSyncer_MigrateLockfile_LoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLock := NewMockLockStore(ctrl)
	mockLock.EXPECT().Load().Return(types.VendorLock{}, errors.New("corrupt lockfile"))

	syncer := newTestSyncer(nil, mockLock, nil, &ServiceOverrides{})

	_, err := syncer.MigrateLockfile()
	if err == nil {
		t.Fatal("MigrateLockfile() expected error, got nil")
	}
}

// ============================================================================
// VendorSyncer.UpdateAll / UpdateAllWithOptions tests
// ============================================================================

func TestVendorSyncer_UpdateAll(t *testing.T) {
	update := &stubUpdateService{}
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{Update: update})

	err := syncer.UpdateAll(context.Background())
	if err != nil {
		t.Fatalf("UpdateAll() error = %v", err)
	}
}

func TestVendorSyncer_UpdateAllWithOptions(t *testing.T) {
	update := &stubUpdateService{}
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{Update: update})

	err := syncer.UpdateAllWithOptions(context.Background(), UpdateOptions{
		Parallel: types.ParallelOptions{Enabled: true, MaxWorkers: 2},
	})
	if err != nil {
		t.Fatalf("UpdateAllWithOptions() error = %v", err)
	}
}

// TestSyncWithFullOpts_NoLockfile_PassesVendorFilter verifies that when no
// lockfile exists, SyncWithFullOpts passes VendorName and GroupName through
// to UpdateAllWithOptions instead of updating all vendors.
func TestSyncWithFullOpts_NoLockfile_PassesVendorFilter(t *testing.T) {
	update := &stubUpdateService{}
	// stubSyncService that succeeds (sync after lockfile creation should work)
	syncSvc := &stubSyncService{}
	// stubLockStore returns empty lock on first call (no lockfile), then returns
	// a non-empty lock for subsequent calls (after update creates it).
	lock := &countingLockStore{
		loads: []types.VendorLock{
			{},                                                                                      // First call: empty (triggers lockfile generation)
			{Vendors: []types.LockDetails{{Name: "vendor-a", Ref: "main", CommitHash: "abc123"}}},   // Second call: non-empty
		},
	}

	syncer := newTestSyncer(nil, lock, nil, &ServiceOverrides{
		Update: update,
		Sync:   syncSvc,
	})

	err := syncer.SyncWithFullOpts(context.Background(), SyncOptions{
		VendorName: "vendor-a",
		Local:      true,
	})
	if err != nil {
		t.Fatalf("SyncWithFullOpts() error = %v", err)
	}

	// UpdateAllWithOptions should have been called with the vendor filter
	if update.lastOpts.VendorName != "vendor-a" {
		t.Errorf("UpdateAllWithOptions VendorName = %q, want %q", update.lastOpts.VendorName, "vendor-a")
	}
	if !update.lastOpts.Local {
		t.Error("UpdateAllWithOptions Local = false, want true")
	}
}

// TestSyncWithFullOpts_NoLockfile_PassesGroupFilter verifies that when no
// lockfile exists, SyncWithFullOpts passes GroupName through to
// UpdateAllWithOptions.
func TestSyncWithFullOpts_NoLockfile_PassesGroupFilter(t *testing.T) {
	update := &stubUpdateService{}
	syncSvc := &stubSyncService{}
	lock := &countingLockStore{
		loads: []types.VendorLock{
			{},
			{Vendors: []types.LockDetails{{Name: "vendor-a", Ref: "main", CommitHash: "abc123"}}},
		},
	}

	syncer := newTestSyncer(nil, lock, nil, &ServiceOverrides{
		Update: update,
		Sync:   syncSvc,
	})

	err := syncer.SyncWithFullOpts(context.Background(), SyncOptions{
		GroupName: "frontend",
	})
	if err != nil {
		t.Fatalf("SyncWithFullOpts() error = %v", err)
	}

	if update.lastOpts.Group != "frontend" {
		t.Errorf("UpdateAllWithOptions Group = %q, want %q", update.lastOpts.Group, "frontend")
	}
}

// TestSyncWithAutoUpdate_StaleCommit_PassesVendorFilter verifies that when
// sync encounters a stale commit error, the auto-update fallback passes
// VendorName through to UpdateAllWithOptions.
func TestSyncWithAutoUpdate_StaleCommit_PassesVendorFilter(t *testing.T) {
	update := &stubUpdateService{}
	// stubSyncService that returns a StaleCommitError
	syncSvc := &stubSyncService{
		syncErr: &StaleCommitError{CommitHash: "deadbeef", VendorName: "vendor-a", Ref: "main"},
	}
	lock := &stubLockStore{
		lock: types.VendorLock{
			Vendors: []types.LockDetails{{Name: "vendor-a", Ref: "main", CommitHash: "deadbeef"}},
		},
	}

	syncer := newTestSyncer(nil, lock, nil, &ServiceOverrides{
		Update: update,
		Sync:   syncSvc,
	})

	err := syncer.SyncWithFullOpts(context.Background(), SyncOptions{
		VendorName: "vendor-a",
		Local:      true,
	})
	if err != nil {
		t.Fatalf("SyncWithFullOpts() error = %v", err)
	}

	if update.lastOpts.VendorName != "vendor-a" {
		t.Errorf("auto-update UpdateAllWithOptions VendorName = %q, want %q", update.lastOpts.VendorName, "vendor-a")
	}
	if update.lastOpts.Group != "" {
		t.Errorf("auto-update UpdateAllWithOptions Group = %q, want %q", update.lastOpts.Group, "")
	}
	if !update.lastOpts.Local {
		t.Error("auto-update UpdateAllWithOptions Local = false, want true")
	}
}

// countingLockStore returns successive lock values on each Load call.
// countingLockStore enables testing code paths that check lockfile existence
// before and after lockfile generation.
type countingLockStore struct {
	loads    []types.VendorLock
	loadIdx int
}

func (s *countingLockStore) Load() (types.VendorLock, error) {
	if s.loadIdx < len(s.loads) {
		l := s.loads[s.loadIdx]
		s.loadIdx++
		return l, nil
	}
	return types.VendorLock{}, nil
}

func (s *countingLockStore) Save(_ types.VendorLock) error { return nil }
func (s *countingLockStore) Path() string                  { return ".git-vendor/vendor.lock" }
func (s *countingLockStore) GetHash(_, _ string) string    { return "" }

// ============================================================================
// VendorSyncer delegation tests
// ============================================================================

func TestVendorSyncer_ValidateConfig(t *testing.T) {
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		Validation: &stubValidationService{},
	})

	err := syncer.ValidateConfig()
	if err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestVendorSyncer_DetectConflicts(t *testing.T) {
	expected := []types.PathConflict{{Path: "lib/a.go"}}
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		Validation: &stubValidationService{conflicts: expected},
	})

	conflicts, err := syncer.DetectConflicts()
	if err != nil {
		t.Fatalf("DetectConflicts() error = %v", err)
	}
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}
}

func TestVendorSyncer_CheckUpdates(t *testing.T) {
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		UpdateChecker: &stubUpdateCheckerService{
			results: []types.UpdateCheckResult{{VendorName: "v1"}},
		},
	})

	results, err := syncer.CheckUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdates() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestVendorSyncer_Verify(t *testing.T) {
	expected := &types.VerifyResult{
		Summary: types.VerifySummary{TotalFiles: 1, Verified: 1},
	}
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		VerifyService: &stubVerifyService{result: expected},
	})

	result, err := syncer.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Summary.TotalFiles != 1 {
		t.Errorf("Expected TotalFiles=1, got %d", result.Summary.TotalFiles)
	}
}

func TestVendorSyncer_Scan(t *testing.T) {
	expected := &types.ScanResult{}
	syncer := newTestSyncer(nil, nil, nil, &ServiceOverrides{
		VulnScanner: &stubVulnScanner{result: expected},
	})

	_, err := syncer.Scan(context.Background(), "high")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
}
