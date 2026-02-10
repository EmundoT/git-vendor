package core

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// CreateVendorEntry Tests
// ============================================================================

func TestCreateVendorEntry_HappyPath(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	// repository.Exists calls config.Load
	config.EXPECT().Load().Return(types.VendorConfig{}, nil)
	// repository.Save calls config.Load then config.Save
	config.EXPECT().Load().Return(types.VendorConfig{}, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if len(cfg.Vendors) != 1 {
			t.Errorf("expected 1 vendor, got %d", len(cfg.Vendors))
		}
		v := cfg.Vendors[0]
		if v.Name != "api-types" {
			t.Errorf("expected name 'api-types', got %q", v.Name)
		}
		if v.URL != "https://github.com/org/api" {
			t.Errorf("expected URL 'https://github.com/org/api', got %q", v.URL)
		}
		if v.License != "MIT" {
			t.Errorf("expected license 'MIT', got %q", v.License)
		}
		if len(v.Specs) != 1 || v.Specs[0].Ref != "v2.0.0" {
			t.Errorf("expected ref 'v2.0.0', got %v", v.Specs)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.CreateVendorEntry("api-types", "https://github.com/org/api", "v2.0.0", "MIT")
	assertNoError(t, err, "CreateVendorEntry")
}

func TestCreateVendorEntry_DefaultRef(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)
	config.EXPECT().Load().Return(types.VendorConfig{}, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors[0].Specs[0].Ref != "main" {
			t.Errorf("expected default ref 'main', got %q", cfg.Vendors[0].Specs[0].Ref)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.CreateVendorEntry("mylib", "https://github.com/org/lib", "", "")
	assertNoError(t, err, "CreateVendorEntry with default ref")
}

func TestCreateVendorEntry_AlreadyExists(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	existing := createTestVendorSpec("api-types", "https://github.com/org/api", "main")
	config.EXPECT().Load().Return(createTestConfig(existing), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.CreateVendorEntry("api-types", "https://github.com/org/other", "main", "")
	assertError(t, err, "CreateVendorEntry duplicate")
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestCreateVendorEntry_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.CreateVendorEntry("", "https://github.com/org/api", "main", "MIT")
	assertError(t, err, "CreateVendorEntry empty name")
}

func TestCreateVendorEntry_EmptyURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.CreateVendorEntry("mylib", "", "main", "MIT")
	assertError(t, err, "CreateVendorEntry empty URL")
}

// ============================================================================
// RenameVendor Tests
// ============================================================================

func TestRenameVendor_HappyPath(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	existing := createTestVendorSpec("old-name", "https://github.com/org/lib", "main")
	cfg := createTestConfig(existing)

	// Exists check for new name
	config.EXPECT().Load().Return(cfg, nil)
	// Load config for rename
	config.EXPECT().Load().Return(cfg, nil)
	// Save config
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors[0].Name != "new-name" {
			t.Errorf("expected renamed vendor 'new-name', got %q", cfg.Vendors[0].Name)
		}
		return nil
	})
	// Load lockfile
	lockData := types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("old-name", "main", "abc123"),
		},
	}
	lock.EXPECT().Load().Return(lockData, nil)
	// Save lockfile with new name
	lock.EXPECT().Save(gomock.Any()).DoAndReturn(func(l types.VendorLock) error {
		if l.Vendors[0].Name != "new-name" {
			t.Errorf("expected lock entry 'new-name', got %q", l.Vendors[0].Name)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	err := syncer.RenameVendor("old-name", "new-name")
	assertNoError(t, err, "RenameVendor")
}

func TestRenameVendor_NotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil) // Exists check
	config.EXPECT().Load().Return(types.VendorConfig{}, nil) // Load for rename

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RenameVendor("nonexistent", "new-name")
	assertError(t, err, "RenameVendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestRenameVendor_NewNameExists(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	v1 := createTestVendorSpec("old-name", "https://github.com/org/a", "main")
	v2 := createTestVendorSpec("new-name", "https://github.com/org/b", "main")

	config.EXPECT().Load().Return(createTestConfig(v1, v2), nil) // Exists check for new name

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RenameVendor("old-name", "new-name")
	assertError(t, err, "RenameVendor new name exists")
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRenameVendor_SameNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RenameVendor("same", "same")
	assertError(t, err, "RenameVendor same names")
}

func TestRenameVendor_NoLockfile(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	existing := createTestVendorSpec("old-name", "https://github.com/org/lib", "main")
	cfg := createTestConfig(existing)

	config.EXPECT().Load().Return(cfg, nil) // Exists check
	config.EXPECT().Load().Return(cfg, nil) // Load for rename
	config.EXPECT().Save(gomock.Any()).Return(nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, os.ErrNotExist) // No lockfile

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	err := syncer.RenameVendor("old-name", "new-name")
	assertNoError(t, err, "RenameVendor with no lockfile")
}

// ============================================================================
// AddMappingToVendor Tests
// ============================================================================

func TestAddMappingToVendor_HappyPath(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		mappings := cfg.Vendors[0].Specs[0].Mapping
		if len(mappings) != 2 {
			t.Errorf("expected 2 mappings, got %d", len(mappings))
		}
		newMapping := mappings[1]
		if newMapping.From != "src/new.go" || newMapping.To != "lib/new.go" {
			t.Errorf("unexpected mapping: %+v", newMapping)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("mylib", "src/new.go", "lib/new.go", "")
	assertNoError(t, err, "AddMappingToVendor")
}

func TestAddMappingToVendor_WithRef(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "mylib",
		URL:     "https://github.com/org/lib",
		License: "MIT",
		Specs: []types.BranchSpec{
			{Ref: "main", Mapping: []types.PathMapping{{From: "a.go", To: "b.go"}}},
			{Ref: "v2", Mapping: []types.PathMapping{}},
		},
	}
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		// The mapping should be added to the v2 spec, not main
		if len(cfg.Vendors[0].Specs[1].Mapping) != 1 {
			t.Errorf("expected 1 mapping on v2 spec, got %d", len(cfg.Vendors[0].Specs[1].Mapping))
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("mylib", "src/new.go", "lib/new.go", "v2")
	assertNoError(t, err, "AddMappingToVendor with ref")
}

func TestAddMappingToVendor_Duplicate(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	// "src/file.go" already exists in createTestVendorSpec
	err := syncer.AddMappingToVendor("mylib", "src/file.go", "other.go", "")
	assertError(t, err, "AddMappingToVendor duplicate")
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddMappingToVendor_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("nonexistent", "a.go", "b.go", "")
	assertError(t, err, "AddMappingToVendor vendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestAddMappingToVendor_RefNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("mylib", "a.go", "b.go", "nonexistent-ref")
	assertError(t, err, "AddMappingToVendor ref not found")
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestAddMappingToVendor_EmptySpecs(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := types.VendorSpec{
		Name:    "mylib",
		URL:     "https://github.com/org/lib",
		License: "MIT",
		Specs:   []types.BranchSpec{}, // no specs
	}
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("mylib", "a.go", "b.go", "")
	assertError(t, err, "AddMappingToVendor empty specs")
	if !strings.Contains(err.Error(), "no specs") {
		t.Errorf("expected 'no specs' error, got: %v", err)
	}
}

func TestAddMappingToVendor_EmptyArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.AddMappingToVendor("", "a.go", "b.go", "")
	assertError(t, err, "AddMappingToVendor empty vendor name")

	err = syncer.AddMappingToVendor("mylib", "", "b.go", "")
	assertError(t, err, "AddMappingToVendor empty from path")
}

// ============================================================================
// RemoveMappingFromVendor Tests
// ============================================================================

func TestRemoveMappingFromVendor_HappyPath(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		mappings := cfg.Vendors[0].Specs[0].Mapping
		if len(mappings) != 0 {
			t.Errorf("expected 0 mappings after remove, got %d", len(mappings))
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RemoveMappingFromVendor("mylib", "src/file.go")
	assertNoError(t, err, "RemoveMappingFromVendor")
}

func TestRemoveMappingFromVendor_NotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RemoveMappingFromVendor("mylib", "nonexistent.go")
	assertError(t, err, "RemoveMappingFromVendor not found")
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRemoveMappingFromVendor_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RemoveMappingFromVendor("nonexistent", "src/file.go")
	assertError(t, err, "RemoveMappingFromVendor vendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestRemoveMappingFromVendor_EmptyArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.RemoveMappingFromVendor("", "src/file.go")
	assertError(t, err, "RemoveMappingFromVendor empty vendor name")

	err = syncer.RemoveMappingFromVendor("mylib", "")
	assertError(t, err, "RemoveMappingFromVendor empty from path")
}

// ============================================================================
// UpdateMappingInVendor Tests
// ============================================================================

func TestUpdateMappingInVendor_HappyPath(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		mapping := cfg.Vendors[0].Specs[0].Mapping[0]
		if mapping.To != "new/target.go" {
			t.Errorf("expected new target 'new/target.go', got %q", mapping.To)
		}
		if mapping.From != "src/file.go" {
			t.Errorf("expected from unchanged 'src/file.go', got %q", mapping.From)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.UpdateMappingInVendor("mylib", "src/file.go", "new/target.go")
	assertNoError(t, err, "UpdateMappingInVendor")
}

func TestUpdateMappingInVendor_NotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.UpdateMappingInVendor("mylib", "nonexistent.go", "new.go")
	assertError(t, err, "UpdateMappingInVendor not found")
}

func TestUpdateMappingInVendor_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.UpdateMappingInVendor("nonexistent", "src/file.go", "new.go")
	assertError(t, err, "UpdateMappingInVendor vendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestUpdateMappingInVendor_EmptyArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), NewMockConfigStore(ctrl), NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.UpdateMappingInVendor("", "src/file.go", "new.go")
	assertError(t, err, "UpdateMappingInVendor empty vendor name")

	err = syncer.UpdateMappingInVendor("mylib", "", "new.go")
	assertError(t, err, "UpdateMappingInVendor empty from path")
}

// ============================================================================
// ShowVendor Tests
// ============================================================================

func TestShowVendor_HappyPath(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	// repository.Find calls config.Load
	config.EXPECT().Load().Return(cfg, nil)
	// lockStore.Load for metadata
	lockData := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:             "mylib",
				Ref:              "main",
				CommitHash:       "abc123def456",
				LicenseSPDX:      "MIT",
				SourceVersionTag: "v1.0.0",
			},
		},
	}
	lock.EXPECT().Load().Return(lockData, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	data, err := syncer.ShowVendor("mylib")
	assertNoError(t, err, "ShowVendor")

	if data["name"] != "mylib" {
		t.Errorf("expected name 'mylib', got %v", data["name"])
	}
	if data["url"] != "https://github.com/org/lib" {
		t.Errorf("expected URL, got %v", data["url"])
	}
	specs, ok := data["specs"].([]map[string]interface{})
	if !ok || len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %v", data["specs"])
	}
	if specs[0]["commit_hash"] != "abc123def456" {
		t.Errorf("expected commit hash, got %v", specs[0]["commit_hash"])
	}
}

func TestShowVendor_NotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	_, err := syncer.ShowVendor("nonexistent")
	assertError(t, err, "ShowVendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestShowVendor_NoLockfile(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, os.ErrNotExist)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	data, err := syncer.ShowVendor("mylib")
	assertNoError(t, err, "ShowVendor no lockfile")

	if data["name"] != "mylib" {
		t.Errorf("expected name 'mylib', got %v", data["name"])
	}
	specs, ok := data["specs"].([]map[string]interface{})
	if !ok || len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %v", data["specs"])
	}
	// No commit_hash when lockfile is absent
	if _, hasHash := specs[0]["commit_hash"]; hasHash {
		t.Errorf("expected no commit_hash without lockfile, got %v", specs[0]["commit_hash"])
	}
}

// ============================================================================
// GetConfigValue Tests
// ============================================================================

func TestGetConfigValue_VendorCount(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	v1 := createTestVendorSpec("a", "https://a.com", "main")
	v2 := createTestVendorSpec("b", "https://b.com", "main")
	config.EXPECT().Load().Return(createTestConfig(v1, v2), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendor_count")
	assertNoError(t, err, "GetConfigValue vendor_count")
	if val != 2 {
		t.Errorf("expected 2, got %v", val)
	}
}

func TestGetConfigValue_VendorField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors.mylib.url")
	assertNoError(t, err, "GetConfigValue vendor URL")
	if val != "https://github.com/org/lib" {
		t.Errorf("expected URL, got %v", val)
	}
}

func TestGetConfigValue_UnknownKey(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	_, err := syncer.GetConfigValue("unknown_key")
	assertError(t, err, "GetConfigValue unknown key")
}

func TestGetConfigValue_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	_, err := syncer.GetConfigValue("vendors.nonexistent.url")
	assertError(t, err, "GetConfigValue vendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestGetConfigValue_VendorsList(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	v1 := createTestVendorSpec("alpha", "https://a.com", "main")
	v2 := createTestVendorSpec("beta", "https://b.com", "main")
	config.EXPECT().Load().Return(createTestConfig(v1, v2), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors")
	assertNoError(t, err, "GetConfigValue vendors list")

	names, ok := val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", val)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", names)
	}
}

func TestGetConfigValue_VendorObject(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors.mylib")
	assertNoError(t, err, "GetConfigValue vendor object")

	obj, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", val)
	}
	if obj["name"] != "mylib" {
		t.Errorf("expected name 'mylib', got %v", obj["name"])
	}
	if obj["url"] != "https://github.com/org/lib" {
		t.Errorf("expected URL, got %v", obj["url"])
	}
}

func TestGetConfigValue_RefField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "v2.0.0")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors.mylib.ref")
	assertNoError(t, err, "GetConfigValue ref field")
	if val != "v2.0.0" {
		t.Errorf("expected 'v2.0.0', got %v", val)
	}
}

func TestGetConfigValue_LicenseField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors.mylib.license")
	assertNoError(t, err, "GetConfigValue license field")
	if val != "MIT" {
		t.Errorf("expected 'MIT', got %v", val)
	}
}

func TestGetConfigValue_GroupsField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	vendor.Groups = []string{"frontend", "shared"}
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	val, err := syncer.GetConfigValue("vendors.mylib.groups")
	assertNoError(t, err, "GetConfigValue groups field")

	groups, ok := val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", val)
	}
	if len(groups) != 2 || groups[0] != "frontend" {
		t.Errorf("expected [frontend shared], got %v", groups)
	}
}

func TestGetConfigValue_UnknownVendorField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	_, err := syncer.GetConfigValue("vendors.mylib.nonexistent_field")
	assertError(t, err, "GetConfigValue unknown vendor field")
	if !strings.Contains(err.Error(), "unknown vendor field") {
		t.Errorf("expected 'unknown vendor field' error, got: %v", err)
	}
}

// ============================================================================
// SetConfigValue Tests
// ============================================================================

func TestSetConfigValue_URL(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors[0].URL != "https://github.com/other/lib" {
			t.Errorf("expected updated URL, got %q", cfg.Vendors[0].URL)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib.url", "https://github.com/other/lib")
	assertNoError(t, err, "SetConfigValue URL")
}

func TestSetConfigValue_Ref(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors[0].Specs[0].Ref != "v2.0.0" {
			t.Errorf("expected ref 'v2.0.0', got %q", cfg.Vendors[0].Specs[0].Ref)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib.ref", "v2.0.0")
	assertNoError(t, err, "SetConfigValue ref")
}

func TestSetConfigValue_NameRejected(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib.name", "newname")
	assertError(t, err, "SetConfigValue name")
	if !strings.Contains(err.Error(), "rename") {
		t.Errorf("expected rename hint, got: %v", err)
	}
}

func TestSetConfigValue_InvalidKey(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("invalid", "value")
	assertError(t, err, "SetConfigValue invalid key")
}

func TestSetConfigValue_License(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)
	config.EXPECT().Save(gomock.Any()).DoAndReturn(func(cfg types.VendorConfig) error {
		if cfg.Vendors[0].License != "Apache-2.0" {
			t.Errorf("expected license 'Apache-2.0', got %q", cfg.Vendors[0].License)
		}
		return nil
	})

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib.license", "Apache-2.0")
	assertNoError(t, err, "SetConfigValue license")
}

func TestSetConfigValue_UnknownField(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	config.EXPECT().Load().Return(createTestConfig(vendor), nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib.nonexistent", "value")
	assertError(t, err, "SetConfigValue unknown field")
	if !strings.Contains(err.Error(), "unknown vendor field") {
		t.Errorf("expected 'unknown vendor field' error, got: %v", err)
	}
}

func TestSetConfigValue_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.nonexistent.url", "https://new.com")
	assertError(t, err, "SetConfigValue vendor not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestSetConfigValue_IncompleteKey(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	err := syncer.SetConfigValue("vendors.mylib", "value")
	assertError(t, err, "SetConfigValue incomplete key")
	if !strings.Contains(err.Error(), "invalid key format") {
		t.Errorf("expected 'invalid key format' error, got: %v", err)
	}
}

// ============================================================================
// CheckVendorStatus Tests
// ============================================================================

func TestCheckVendorStatus_Synced(t *testing.T) {
	ctrl, _, fs, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil) // repository.Find
	lock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("mylib", "main", "abc123"),
		},
	}, nil)
	// File exists check
	fs.EXPECT().Stat("lib/file.go").Return(&mockFileInfo{name: "file.go"}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), fs, config, lock, NewMockLicenseChecker(ctrl))
	result, err := syncer.CheckVendorStatus("mylib")
	assertNoError(t, err, "CheckVendorStatus synced")

	if result["status"] != "synced" {
		t.Errorf("expected status 'synced', got %v", result["status"])
	}
}

func TestCheckVendorStatus_Stale(t *testing.T) {
	ctrl, _, fs, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("mylib", "main", "abc123"),
		},
	}, nil)
	// File missing
	fs.EXPECT().Stat("lib/file.go").Return(nil, os.ErrNotExist)

	syncer := createMockSyncer(NewMockGitClient(ctrl), fs, config, lock, NewMockLicenseChecker(ctrl))
	result, err := syncer.CheckVendorStatus("mylib")
	assertNoError(t, err, "CheckVendorStatus stale")

	if result["status"] != "stale" {
		t.Errorf("expected status 'stale', got %v", result["status"])
	}
}

func TestCheckVendorStatus_NoLockfile(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	lock.EXPECT().Load().Return(types.VendorLock{}, os.ErrNotExist)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	result, err := syncer.CheckVendorStatus("mylib")
	assertNoError(t, err, "CheckVendorStatus no lockfile")

	if result["status"] != "not_synced" {
		t.Errorf("expected status 'not_synced', got %v", result["status"])
	}
}

func TestCheckVendorStatus_VendorNotFound(t *testing.T) {
	ctrl, _, _, config, _, _ := setupMocks(t)
	defer ctrl.Finish()

	config.EXPECT().Load().Return(types.VendorConfig{}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, NewMockLockStore(ctrl), NewMockLicenseChecker(ctrl))
	_, err := syncer.CheckVendorStatus("nonexistent")
	assertError(t, err, "CheckVendorStatus not found")
	if !IsVendorNotFound(err) {
		t.Errorf("expected VendorNotFoundError, got: %v", err)
	}
}

func TestCheckVendorStatus_NotLocked(t *testing.T) {
	ctrl, _, _, config, lock, _ := setupMocks(t)
	defer ctrl.Finish()

	vendor := createTestVendorSpec("mylib", "https://github.com/org/lib", "main")
	cfg := createTestConfig(vendor)

	config.EXPECT().Load().Return(cfg, nil)
	// Lockfile exists but has no entry for mylib@main
	lock.EXPECT().Load().Return(types.VendorLock{
		Vendors: []types.LockDetails{
			createTestLockEntry("other-vendor", "main", "xyz789"),
		},
	}, nil)

	syncer := createMockSyncer(NewMockGitClient(ctrl), NewMockFileSystem(ctrl), config, lock, NewMockLicenseChecker(ctrl))
	result, err := syncer.CheckVendorStatus("mylib")
	assertNoError(t, err, "CheckVendorStatus not locked")

	if result["status"] != "stale" {
		t.Errorf("expected status 'stale', got %v", result["status"])
	}
	specs, ok := result["specs"].([]map[string]interface{})
	if !ok || len(specs) != 1 {
		t.Fatalf("expected 1 spec status, got %v", result["specs"])
	}
	if specs[0]["status"] != "not_locked" {
		t.Errorf("expected spec status 'not_locked', got %v", specs[0]["status"])
	}
}

// ============================================================================
// CLIResponse Tests
// ============================================================================

func TestCLIExitCodeForError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"VendorNotFound", NewVendorNotFoundError("test"), ExitVendorNotFound},
		{"ValidationError", NewValidationError("v", "r", "f", "m"), ExitValidationFailed},
		{"GenericError", errors.New("something"), ExitGeneralError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := CLIExitCodeForError(tt.err)
			if code != tt.expected {
				t.Errorf("expected exit code %d, got %d", tt.expected, code)
			}
		})
	}
}

func TestCLIErrorCodeForError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"VendorNotFound", NewVendorNotFoundError("test"), ErrCodeVendorNotFound},
		{"ValidationError", NewValidationError("v", "r", "f", "m"), ErrCodeValidationFailed},
		{"GenericError", errors.New("something"), ErrCodeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := CLIErrorCodeForError(tt.err)
			if code != tt.expected {
				t.Errorf("expected error code %q, got %q", tt.expected, code)
			}
		})
	}
}
