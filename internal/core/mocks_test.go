package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git-vendor/internal/types"
)

// ============================================================================
// MockGitClient
// ============================================================================

// MockGitClient implements GitClient interface for testing
type MockGitClient struct {
	InitFunc       func(dir string) error
	AddRemoteFunc  func(dir, name, url string) error
	FetchFunc      func(dir string, depth int, ref string) error
	FetchAllFunc   func(dir string) error
	CheckoutFunc   func(dir, ref string) error
	GetHeadHashFunc func(dir string) (string, error)
	CloneFunc      func(dir, url string, opts *CloneOptions) error
	ListTreeFunc   func(dir, ref, subdir string) ([]string, error)

	// Call tracking
	InitCalls       []string
	AddRemoteCalls  [][]string
	FetchCalls      [][]interface{}
	FetchAllCalls   []string
	CheckoutCalls   [][]string
	GetHeadHashCalls []string
	CloneCalls      [][]interface{}
	ListTreeCalls   [][]string
}

// Init implements GitClient
func (m *MockGitClient) Init(dir string) error {
	m.InitCalls = append(m.InitCalls, dir)
	if m.InitFunc != nil {
		return m.InitFunc(dir)
	}
	return nil
}

// AddRemote implements GitClient
func (m *MockGitClient) AddRemote(dir, name, url string) error {
	m.AddRemoteCalls = append(m.AddRemoteCalls, []string{dir, name, url})
	if m.AddRemoteFunc != nil {
		return m.AddRemoteFunc(dir, name, url)
	}
	return nil
}

// Fetch implements GitClient
func (m *MockGitClient) Fetch(dir string, depth int, ref string) error {
	m.FetchCalls = append(m.FetchCalls, []interface{}{dir, depth, ref})
	if m.FetchFunc != nil {
		return m.FetchFunc(dir, depth, ref)
	}
	return nil
}

// FetchAll implements GitClient
func (m *MockGitClient) FetchAll(dir string) error {
	m.FetchAllCalls = append(m.FetchAllCalls, dir)
	if m.FetchAllFunc != nil {
		return m.FetchAllFunc(dir)
	}
	return nil
}

// Checkout implements GitClient
func (m *MockGitClient) Checkout(dir, ref string) error {
	m.CheckoutCalls = append(m.CheckoutCalls, []string{dir, ref})
	if m.CheckoutFunc != nil {
		return m.CheckoutFunc(dir, ref)
	}
	return nil
}

// GetHeadHash implements GitClient
func (m *MockGitClient) GetHeadHash(dir string) (string, error) {
	m.GetHeadHashCalls = append(m.GetHeadHashCalls, dir)
	if m.GetHeadHashFunc != nil {
		return m.GetHeadHashFunc(dir)
	}
	return "abc123def456", nil
}

// Clone implements GitClient
func (m *MockGitClient) Clone(dir, url string, opts *CloneOptions) error {
	m.CloneCalls = append(m.CloneCalls, []interface{}{dir, url, opts})
	if m.CloneFunc != nil {
		return m.CloneFunc(dir, url, opts)
	}
	return nil
}

// ListTree implements GitClient
func (m *MockGitClient) ListTree(dir, ref, subdir string) ([]string, error) {
	m.ListTreeCalls = append(m.ListTreeCalls, []string{dir, ref, subdir})
	if m.ListTreeFunc != nil {
		return m.ListTreeFunc(dir, ref, subdir)
	}
	return []string{"README.md", "src/"}, nil
}

// ============================================================================
// MockFileSystem
// ============================================================================

// MockFileSystem implements FileSystem interface for testing
type MockFileSystem struct {
	CopyFileFunc   func(src, dst string) error
	CopyDirFunc    func(src, dst string) error
	MkdirAllFunc   func(path string, perm os.FileMode) error
	ReadDirFunc    func(path string) ([]string, error)
	StatFunc       func(path string) (os.FileInfo, error)
	RemoveFunc     func(path string) error
	CreateTempFunc func(dir, pattern string) (string, error)
	RemoveAllFunc  func(path string) error

	// Call tracking
	CopyFileCalls   [][]string
	CopyDirCalls    [][]string
	MkdirAllCalls   [][]interface{}
	ReadDirCalls    []string
	StatCalls       []string
	RemoveCalls     []string
	CreateTempCalls [][]string
	RemoveAllCalls  []string

	// Virtual filesystem for tracking
	Files map[string]string
	Dirs  map[string]bool
}

// NewMockFileSystem creates a new MockFileSystem with empty state
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files: make(map[string]string),
		Dirs:  make(map[string]bool),
	}
}

// CopyFile implements FileSystem
func (m *MockFileSystem) CopyFile(src, dst string) error {
	m.CopyFileCalls = append(m.CopyFileCalls, []string{src, dst})
	if m.CopyFileFunc != nil {
		return m.CopyFileFunc(src, dst)
	}
	// Simulate copy in virtual fs
	if content, ok := m.Files[src]; ok {
		m.Files[dst] = content
	} else {
		m.Files[dst] = "mock content"
	}
	return nil
}

// CopyDir implements FileSystem
func (m *MockFileSystem) CopyDir(src, dst string) error {
	m.CopyDirCalls = append(m.CopyDirCalls, []string{src, dst})
	if m.CopyDirFunc != nil {
		return m.CopyDirFunc(src, dst)
	}
	m.Dirs[dst] = true
	return nil
}

// MkdirAll implements FileSystem
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	m.MkdirAllCalls = append(m.MkdirAllCalls, []interface{}{path, perm})
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	m.Dirs[path] = true
	return nil
}

// ReadDir implements FileSystem
func (m *MockFileSystem) ReadDir(path string) ([]string, error) {
	m.ReadDirCalls = append(m.ReadDirCalls, path)
	if m.ReadDirFunc != nil {
		return m.ReadDirFunc(path)
	}
	return []string{"file1.txt", "dir1/"}, nil
}

// Stat implements FileSystem
func (m *MockFileSystem) Stat(path string) (os.FileInfo, error) {
	m.StatCalls = append(m.StatCalls, path)
	if m.StatFunc != nil {
		return m.StatFunc(path)
	}
	return &mockFileInfo{name: filepath.Base(path), isDir: m.Dirs[path]}, nil
}

// Remove implements FileSystem
func (m *MockFileSystem) Remove(path string) error {
	m.RemoveCalls = append(m.RemoveCalls, path)
	if m.RemoveFunc != nil {
		return m.RemoveFunc(path)
	}
	delete(m.Files, path)
	delete(m.Dirs, path)
	return nil
}

// CreateTemp implements FileSystem
func (m *MockFileSystem) CreateTemp(dir, pattern string) (string, error) {
	m.CreateTempCalls = append(m.CreateTempCalls, []string{dir, pattern})
	if m.CreateTempFunc != nil {
		return m.CreateTempFunc(dir, pattern)
	}
	tempDir := filepath.Join(dir, pattern+"-12345")
	m.Dirs[tempDir] = true
	return tempDir, nil
}

// RemoveAll implements FileSystem
func (m *MockFileSystem) RemoveAll(path string) error {
	m.RemoveAllCalls = append(m.RemoveAllCalls, path)
	if m.RemoveAllFunc != nil {
		return m.RemoveAllFunc(path)
	}
	// Remove all files/dirs with prefix
	for k := range m.Files {
		if k == path || len(k) > len(path) && k[:len(path)+1] == path+string(filepath.Separator) {
			delete(m.Files, k)
		}
	}
	for k := range m.Dirs {
		if k == path || len(k) > len(path) && k[:len(path)+1] == path+string(filepath.Separator) {
			delete(m.Dirs, k)
		}
	}
	return nil
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 1024 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// ============================================================================
// MockConfigStore
// ============================================================================

// MockConfigStore implements ConfigStore interface for testing
type MockConfigStore struct {
	LoadFunc func() (types.VendorConfig, error)
	SaveFunc func(config types.VendorConfig) error
	PathFunc func() string

	// State
	Config types.VendorConfig

	// Call tracking
	LoadCalls int
	SaveCalls []types.VendorConfig
	PathCalls int
}

// Load implements ConfigStore
func (m *MockConfigStore) Load() (types.VendorConfig, error) {
	m.LoadCalls++
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}
	return m.Config, nil
}

// Save implements ConfigStore
func (m *MockConfigStore) Save(config types.VendorConfig) error {
	m.SaveCalls = append(m.SaveCalls, config)
	if m.SaveFunc != nil {
		return m.SaveFunc(config)
	}
	m.Config = config
	return nil
}

// Path implements ConfigStore
func (m *MockConfigStore) Path() string {
	m.PathCalls++
	if m.PathFunc != nil {
		return m.PathFunc()
	}
	return "/mock/vendor/vendor.yml"
}

// ============================================================================
// MockLockStore
// ============================================================================

// MockLockStore implements LockStore interface for testing
type MockLockStore struct {
	LoadFunc    func() (types.VendorLock, error)
	SaveFunc    func(lock types.VendorLock) error
	PathFunc    func() string
	GetHashFunc func(vendorName, ref string) string

	// State
	Lock types.VendorLock

	// Call tracking
	LoadCalls    int
	SaveCalls    []types.VendorLock
	PathCalls    int
	GetHashCalls [][]string
}

// Load implements LockStore
func (m *MockLockStore) Load() (types.VendorLock, error) {
	m.LoadCalls++
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}
	return m.Lock, nil
}

// Save implements LockStore
func (m *MockLockStore) Save(lock types.VendorLock) error {
	m.SaveCalls = append(m.SaveCalls, lock)
	if m.SaveFunc != nil {
		return m.SaveFunc(lock)
	}
	m.Lock = lock
	return nil
}

// Path implements LockStore
func (m *MockLockStore) Path() string {
	m.PathCalls++
	if m.PathFunc != nil {
		return m.PathFunc()
	}
	return "/mock/vendor/vendor.lock"
}

// GetHash implements LockStore
func (m *MockLockStore) GetHash(vendorName, ref string) string {
	m.GetHashCalls = append(m.GetHashCalls, []string{vendorName, ref})
	if m.GetHashFunc != nil {
		return m.GetHashFunc(vendorName, ref)
	}
	// Look in mock lock state
	for _, l := range m.Lock.Vendors {
		if l.Name == vendorName && l.Ref == ref {
			return l.CommitHash
		}
	}
	return ""
}

// ============================================================================
// MockLicenseChecker
// ============================================================================

// MockLicenseChecker implements LicenseChecker interface for testing
type MockLicenseChecker struct {
	CheckLicenseFunc func(url string) (string, error)
	IsAllowedFunc    func(license string) bool

	// Call tracking
	CheckLicenseCalls []string
	IsAllowedCalls    []string
}

// CheckLicense implements LicenseChecker
func (m *MockLicenseChecker) CheckLicense(url string) (string, error) {
	m.CheckLicenseCalls = append(m.CheckLicenseCalls, url)
	if m.CheckLicenseFunc != nil {
		return m.CheckLicenseFunc(url)
	}
	return "MIT", nil
}

// IsAllowed implements LicenseChecker
func (m *MockLicenseChecker) IsAllowed(license string) bool {
	m.IsAllowedCalls = append(m.IsAllowedCalls, license)
	if m.IsAllowedFunc != nil {
		return m.IsAllowedFunc(license)
	}
	// Default allowed licenses
	allowed := []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC"}
	for _, l := range allowed {
		if license == l {
			return true
		}
	}
	return false
}

// ============================================================================
// Helper Functions
// ============================================================================

// setupMocks creates default mock implementations for testing
func setupMocks() (*MockGitClient, *MockFileSystem, *MockConfigStore, *MockLockStore, *MockLicenseChecker) {
	git := &MockGitClient{}
	fs := NewMockFileSystem()
	config := &MockConfigStore{
		Config: types.VendorConfig{Vendors: []types.VendorSpec{}},
	}
	lock := &MockLockStore{
		Lock: types.VendorLock{Vendors: []types.LockDetails{}},
	}
	license := &MockLicenseChecker{}

	return git, fs, config, lock, license
}

// createMockSyncer creates a VendorSyncer with mock dependencies
func createMockSyncer(git GitClient, fs FileSystem, config ConfigStore, lock LockStore, license LicenseChecker) *VendorSyncer {
	return NewVendorSyncer(config, lock, git, fs, license, "/mock/vendor")
}

// createTestVendorSpec creates a basic vendor spec for testing
func createTestVendorSpec(name, url, ref string) types.VendorSpec {
	return types.VendorSpec{
		Name:    name,
		URL:     url,
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: ref,
				Mapping: []types.PathMapping{
					{From: "src/file.go", To: "lib/file.go"},
				},
			},
		},
	}
}

// createTestConfig creates a config with test vendors
func createTestConfig(vendors ...types.VendorSpec) types.VendorConfig {
	return types.VendorConfig{Vendors: vendors}
}

// createTestLock creates a lock with test entries
func createTestLock(entries ...types.LockDetails) types.VendorLock {
	return types.VendorLock{Vendors: entries}
}

// createTestLockEntry creates a lock entry for testing
func createTestLockEntry(name, ref, hash string) types.LockDetails {
	return types.LockDetails{
		Name:       name,
		Ref:        ref,
		CommitHash: hash,
		Updated:    "2025-12-11T00:00:00Z",
	}
}

// assertNoError is a test helper for checking errors
func assertNoError(t interface{ Fatalf(string, ...interface{}) }, err error, msg string) {
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// assertError is a test helper for expecting errors
func assertError(t interface{ Errorf(string, ...interface{}) }, err error, msg string) {
	if err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
}

// assertContains checks if a string contains a substring
func assertContains(t interface{ Errorf(string, ...interface{}) }, s, substr, msg string) {
	if !contains(s, substr) {
		t.Errorf("%s: expected %q to contain %q", msg, s, substr)
	}
}

// assertEqual checks if two values are equal
func assertEqual(t interface{ Errorf(string, ...interface{}) }, got, want interface{}, msg string) {
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}
