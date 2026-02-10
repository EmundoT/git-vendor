package core

import (
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"

	"github.com/golang/mock/gomock"
)

// ============================================================================
// Shared Service Stubs
// ============================================================================
// These stubs implement service interfaces with no-op or configurable behavior.
// They are used in tests that construct SyncService directly (bypassing
// VendorSyncer/createMockSyncer) to control specific dependencies like CacheStore.
// Gomock mocks are not generated for HookExecutor, FileCopyServiceInterface,
// or LicenseServiceInterface because these interfaces are only mocked at the
// service-stub level — the existing gomock mocks target lower-level infrastructure (GitClient,
// FileSystem, ConfigStore, LockStore, LicenseChecker).

// stubHookExecutor is a no-op HookExecutor for tests that don't exercise hooks.
type stubHookExecutor struct{}

func (s *stubHookExecutor) ExecutePreSync(_ *types.VendorSpec, _ *types.HookContext) error {
	return nil
}
func (s *stubHookExecutor) ExecutePostSync(_ *types.VendorSpec, _ *types.HookContext) error {
	return nil
}

// stubFileCopyService returns configurable CopyStats from CopyMappings.
type stubFileCopyService struct {
	stats CopyStats
	err   error
}

func (s *stubFileCopyService) CopyMappings(_ string, _ *types.VendorSpec, spec types.BranchSpec) (CopyStats, error) {
	if s.err != nil {
		return CopyStats{}, s.err
	}
	stats := s.stats
	if stats.FileCount == 0 {
		stats.FileCount = len(spec.Mapping)
	}
	return stats, nil
}

// stubLicenseService is a no-op LicenseServiceInterface for tests.
type stubLicenseService struct{}

func (s *stubLicenseService) CheckCompliance(_ string) (string, error) { return "MIT", nil }
func (s *stubLicenseService) CopyLicense(_, _ string) error            { return nil }
func (s *stubLicenseService) GetLicensePath(_ string) string           { return "" }
func (s *stubLicenseService) CheckLicense(_ string) (string, error)    { return "MIT", nil }

// errCacheStore wraps mockCacheStore to inject a Load error.
type errCacheStore struct {
	*mockCacheStore
	loadErr error
	saved   []*types.IncrementalSyncCache // tracks Save calls
}

func (e *errCacheStore) Load(vendorName, ref string) (types.IncrementalSyncCache, error) {
	if e.loadErr != nil {
		return types.IncrementalSyncCache{}, e.loadErr
	}
	return e.mockCacheStore.Load(vendorName, ref)
}

func (e *errCacheStore) Save(cache *types.IncrementalSyncCache) error {
	e.saved = append(e.saved, cache)
	return e.mockCacheStore.Save(cache)
}

// trackingCacheStore wraps mockCacheStore and records whether Load/Save were called.
type trackingCacheStore struct {
	*mockCacheStore
	loadCalled bool
	saveCalled bool
}

func (t *trackingCacheStore) Load(vendorName, ref string) (types.IncrementalSyncCache, error) {
	t.loadCalled = true
	return t.mockCacheStore.Load(vendorName, ref)
}

func (t *trackingCacheStore) Save(cache *types.IncrementalSyncCache) error {
	t.saveCalled = true
	return t.mockCacheStore.Save(cache)
}

// newSyncServiceWithCache creates a SyncService with a custom CacheStore,
// using gomock for GitClient/FileSystem and stubs for other deps.
func newSyncServiceWithCache(
	git GitClient,
	fs FileSystem,
	cache CacheStore,
	rootDir string,
) *SyncService {
	return NewSyncService(
		nil, // configStore (unused in SyncVendor)
		nil, // lockStore (unused in SyncVendor)
		git,
		fs,
		&stubFileCopyService{stats: CopyStats{FileCount: 1, ByteCount: 100}},
		&stubLicenseService{},
		cache,
		&stubHookExecutor{},
		&SilentUICallback{},
		rootDir,
	)
}

// ============================================================================
// Gomock Test Helpers
// ============================================================================

// setupMocks creates all mock dependencies with gomock
func setupMocks(t *testing.T) (
	*gomock.Controller,
	*MockGitClient,
	*MockFileSystem,
	*MockConfigStore,
	*MockLockStore,
	*MockLicenseChecker,
) {
	ctrl := gomock.NewController(t)

	git := NewMockGitClient(ctrl)
	fs := NewMockFileSystem(ctrl)
	config := NewMockConfigStore(ctrl)
	lock := NewMockLockStore(ctrl)
	license := NewMockLicenseChecker(ctrl)

	return ctrl, git, fs, config, lock, license
}

// createMockSyncer creates a VendorSyncer with mock dependencies
func createMockSyncer(
	git GitClient,
	fs FileSystem,
	config ConfigStore,
	lock LockStore,
	license LicenseChecker,
) *VendorSyncer {
	return NewVendorSyncer(config, lock, git, fs, license, "/mock/vendor", &SilentUICallback{}, nil)
}

// capturingUICallback captures UI output for testing
type capturingUICallback struct {
	errorMsg    string
	successMsg  string
	warningMsg  string
	confirmResp bool
	licenseMsg  string
}

func (c *capturingUICallback) ShowError(title, message string) {
	c.errorMsg = title + ": " + message
}

func (c *capturingUICallback) ShowSuccess(message string) {
	c.successMsg = message
}

func (c *capturingUICallback) ShowWarning(title, message string) {
	c.warningMsg = title + ": " + message
}

func (c *capturingUICallback) AskConfirmation(_, _ string) bool {
	return c.confirmResp
}

func (c *capturingUICallback) ShowLicenseCompliance(license string) {
	c.licenseMsg = license
}

func (c *capturingUICallback) StyleTitle(title string) string {
	return title
}

func (c *capturingUICallback) GetOutputMode() OutputMode {
	return OutputNormal
}

func (c *capturingUICallback) IsAutoApprove() bool {
	return false
}

func (c *capturingUICallback) FormatJSON(_ JSONOutput) error {
	return nil
}

func (c *capturingUICallback) StartProgress(_ int, _ string) types.ProgressTracker {
	return &NoOpProgressTracker{}
}
