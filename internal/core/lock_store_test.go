package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// ============================================================================
// Path Tests
// ============================================================================

func TestFileLockStore_Path(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Test: Path() should return vendor.lock path
	expectedPath := filepath.Join(vendorDir, "vendor.lock")
	actualPath := store.Path()

	if actualPath != expectedPath {
		t.Errorf("Path() = %q, want %q", actualPath, expectedPath)
	}
}

// ============================================================================
// GetHash Tests
// ============================================================================

func TestFileLockStore_GetHash(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create test lockfile
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "vendor1",
				Ref:        "main",
				CommitHash: "abc123def456",
			},
			{
				Name:       "vendor1",
				Ref:        "develop",
				CommitHash: "xyz789ghi012",
			},
			{
				Name:       "vendor2",
				Ref:        "v1.0",
				CommitHash: "111222333444",
			},
		},
	}

	// Save lockfile
	if err := store.Save(lock); err != nil {
		t.Fatalf("Failed to save lockfile: %v", err)
	}

	tests := []struct {
		name         string
		vendorName   string
		ref          string
		expectedHash string
	}{
		{
			name:         "vendor1 @ main",
			vendorName:   "vendor1",
			ref:          "main",
			expectedHash: "abc123def456",
		},
		{
			name:         "vendor1 @ develop",
			vendorName:   "vendor1",
			ref:          "develop",
			expectedHash: "xyz789ghi012",
		},
		{
			name:         "vendor2 @ v1.0",
			vendorName:   "vendor2",
			ref:          "v1.0",
			expectedHash: "111222333444",
		},
		{
			name:         "nonexistent vendor",
			vendorName:   "vendor3",
			ref:          "main",
			expectedHash: "",
		},
		{
			name:         "nonexistent ref",
			vendorName:   "vendor1",
			ref:          "nonexistent",
			expectedHash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := store.GetHash(tt.vendorName, tt.ref)
			if hash != tt.expectedHash {
				t.Errorf("GetHash(%q, %q) = %q, want %q", tt.vendorName, tt.ref, hash, tt.expectedHash)
			}
		})
	}
}

func TestFileLockStore_GetHash_EmptyLockfile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create empty lockfile
	lock := types.VendorLock{
		Vendors: []types.LockDetails{},
	}

	if err := store.Save(lock); err != nil {
		t.Fatalf("Failed to save lockfile: %v", err)
	}

	// Test: GetHash on empty lockfile should return empty string
	hash := store.GetHash("vendor1", "main")
	if hash != "" {
		t.Errorf("GetHash() on empty lockfile = %q, want empty string", hash)
	}
}

func TestFileLockStore_GetHash_MissingLockfile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Don't create lockfile - lockfile doesn't exist on disk

	// Test: GetHash on missing lockfile should return empty string (no error)
	hash := store.GetHash("vendor1", "main")
	if hash != "" {
		t.Errorf("GetHash() on missing lockfile = %q, want empty string", hash)
	}
}

// ============================================================================
// Load and Save Tests (additional coverage)
// ============================================================================

func TestFileLockStore_LoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Create test lock
	originalLock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:        "test-vendor",
				Ref:         "main",
				CommitHash:  "abc123",
				Updated:     "2024-01-01T00:00:00Z",
				LicensePath: VendorDir + "/licenses/test-vendor.txt",
			},
		},
	}

	// Test: Save
	if err := store.Save(originalLock); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Test: Load
	loadedLock, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded lock matches original
	if len(loadedLock.Vendors) != 1 {
		t.Fatalf("Expected 1 vendor, got %d", len(loadedLock.Vendors))
	}

	vendor := loadedLock.Vendors[0]
	if vendor.Name != "test-vendor" {
		t.Errorf("Vendor name = %q, want %q", vendor.Name, "test-vendor")
	}
	if vendor.Ref != "main" {
		t.Errorf("Vendor ref = %q, want %q", vendor.Ref, "main")
	}
	if vendor.CommitHash != "abc123" {
		t.Errorf("Vendor commit hash = %q, want %q", vendor.CommitHash, "abc123")
	}
}

func TestFileLockStore_Load_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Test: Load on missing file should error
	_, err := store.Load()
	if err == nil {
		t.Error("Expected error when loading missing lockfile, got nil")
	}
}

// ============================================================================
// Schema Version Tests
// ============================================================================

func TestParseSchemaVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{
			name:      "empty defaults to 1.0",
			input:     "",
			wantMajor: 1,
			wantMinor: 0,
			wantErr:   false,
		},
		{
			name:      "valid 1.0",
			input:     "1.0",
			wantMajor: 1,
			wantMinor: 0,
			wantErr:   false,
		},
		{
			name:      "valid 1.1",
			input:     "1.1",
			wantMajor: 1,
			wantMinor: 1,
			wantErr:   false,
		},
		{
			name:      "valid 1.5",
			input:     "1.5",
			wantMajor: 1,
			wantMinor: 5,
			wantErr:   false,
		},
		{
			name:      "valid 2.0",
			input:     "2.0",
			wantMajor: 2,
			wantMinor: 0,
			wantErr:   false,
		},
		{
			name:      "valid 10.20",
			input:     "10.20",
			wantMajor: 10,
			wantMinor: 20,
			wantErr:   false,
		},
		{
			name:      "invalid single number",
			input:     "1",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid three parts",
			input:     "1.0.0",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid non-numeric major",
			input:     "a.0",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid non-numeric minor",
			input:     "1.b",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid both non-numeric",
			input:     "a.b",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid negative major",
			input:     "-1.0",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
		{
			name:      "invalid negative minor",
			input:     "1.-1",
			wantMajor: 0,
			wantMinor: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, err := parseSchemaVersion(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseSchemaVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if major != tt.wantMajor {
					t.Errorf("parseSchemaVersion(%q) major = %d, want %d", tt.input, major, tt.wantMajor)
				}
				if minor != tt.wantMinor {
					t.Errorf("parseSchemaVersion(%q) minor = %d, want %d", tt.input, minor, tt.wantMinor)
				}
			}
		})
	}
}

func TestValidateSchemaVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		wantErr     bool
		wantWarning bool
	}{
		{
			name:        "missing version defaults to 1.0",
			version:     "",
			wantErr:     false,
			wantWarning: false,
		},
		{
			name:        "current version 1.0",
			version:     "1.0",
			wantErr:     false,
			wantWarning: false,
		},
		{
			name:        "current version 1.1",
			version:     "1.1",
			wantErr:     false,
			wantWarning: false,
		},
		{
			name:        "current version 1.2",
			version:     "1.2",
			wantErr:     false,
			wantWarning: false,
		},
		{
			name:        "newer minor version 1.5 warns",
			version:     "1.5",
			wantErr:     false,
			wantWarning: true,
		},
		{
			name:        "newer major version 2.0 errors",
			version:     "2.0",
			wantErr:     true,
			wantWarning: false,
		},
		{
			name:        "newer major version 3.0 errors",
			version:     "3.0",
			wantErr:     true,
			wantWarning: false,
		},
		{
			name:        "invalid format errors",
			version:     "invalid",
			wantErr:     true,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warnBuf strings.Builder
			err := validateSchemaVersion(tt.version, &warnBuf)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchemaVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
				return
			}

			hasWarning := warnBuf.Len() > 0
			if hasWarning != tt.wantWarning {
				t.Errorf("validateSchemaVersion(%q) warning = %v, wantWarning %v", tt.version, hasWarning, tt.wantWarning)
			}

			// Verify warning message content when expected
			if tt.wantWarning {
				warning := warnBuf.String()
				if !strings.Contains(warning, "Warning") {
					t.Errorf("warning should contain 'Warning', got: %s", warning)
				}
				if !strings.Contains(warning, tt.version) {
					t.Errorf("warning should contain version %q, got: %s", tt.version, warning)
				}
			}

			// Verify error message content when expected (for major version errors)
			if tt.wantErr && !strings.Contains(tt.version, "invalid") {
				if err != nil && !strings.Contains(err.Error(), "newer git-vendor version") {
					t.Errorf("error should mention needing newer git-vendor, got: %v", err)
				}
			}
		})
	}
}

func TestFileLockStore_Load_VersionCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		schemaVersion string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "missing version loads successfully",
			schemaVersion: "",
			wantErr:       false,
		},
		{
			name:          "current version 1.0 loads",
			schemaVersion: "1.0",
			wantErr:       false,
		},
		{
			name:          "current version 1.1 loads",
			schemaVersion: "1.1",
			wantErr:       false,
		},
		{
			name:          "newer minor 1.2 loads with warning",
			schemaVersion: "1.2",
			wantErr:       false,
		},
		{
			name:          "newer major 2.0 fails",
			schemaVersion: "2.0",
			wantErr:       true,
			errContains:   "newer git-vendor version",
		},
		{
			name:          "invalid format fails",
			schemaVersion: "invalid",
			wantErr:       true,
			errContains:   "invalid schema version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			vendorDir := filepath.Join(tempDir, VendorDir)
			_ = os.MkdirAll(vendorDir, 0755)

			// Write lockfile with specific schema version
			lockContent := "vendors:\n  - name: test\n    ref: main\n    commit_hash: abc123\n    updated: '2024-01-01T00:00:00Z'\n"
			if tt.schemaVersion != "" {
				lockContent = "schema_version: \"" + tt.schemaVersion + "\"\n" + lockContent
			}

			lockPath := filepath.Join(vendorDir, "vendor.lock")
			err := os.WriteFile(lockPath, []byte(lockContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test lockfile: %v", err)
			}

			store := NewFileLockStore(vendorDir)
			_, err = store.Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Load() error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestFileLockStore_Save_SetsSchemaVersion(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Save lock without schema version set
	lock := types.VendorLock{
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
			},
		},
	}

	err := store.Save(lock)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read the file directly to verify schema_version is written
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("Failed to read lockfile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "schema_version") {
		t.Error("Saved lockfile should contain schema_version field")
	}
	if !strings.Contains(content, CurrentSchemaVersion) {
		t.Errorf("Saved lockfile should contain version %q, got:\n%s", CurrentSchemaVersion, content)
	}

	// Load and verify
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("Loaded SchemaVersion = %q, want %q", loaded.SchemaVersion, CurrentSchemaVersion)
	}
}

// ============================================================================
// Merge Conflict Detection Tests
// ============================================================================

func TestDetectConflictsInData_NoConflicts(t *testing.T) {
	data := []byte(`schema_version: "1.2"
vendors:
  - name: test
    ref: main
    commit_hash: abc123
`)
	err := detectConflictsInData(data)
	if err != nil {
		t.Errorf("detectConflictsInData() = %v, want nil", err)
	}
}

func TestDetectConflictsInData_WithConflicts(t *testing.T) {
	data := []byte(`schema_version: "1.2"
vendors:
<<<<<<< HEAD
  - name: vendor-a
    ref: main
    commit_hash: abc123
=======
  - name: vendor-a
    ref: main
    commit_hash: def456
>>>>>>> feature-branch
`)
	err := detectConflictsInData(data)
	if err == nil {
		t.Fatal("Expected error for conflict markers, got nil")
	}

	var conflictErr *LockConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("Expected LockConflictError, got %T", err)
	}

	if len(conflictErr.Conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflictErr.Conflicts))
	}

	c := conflictErr.Conflicts[0]
	if c.LineNumber != 3 {
		t.Errorf("Expected conflict at line 3, got %d", c.LineNumber)
	}
	if !strings.Contains(c.OursRaw, "abc123") {
		t.Errorf("Ours should contain 'abc123', got: %s", c.OursRaw)
	}
	if !strings.Contains(c.TheirsRaw, "def456") {
		t.Errorf("Theirs should contain 'def456', got: %s", c.TheirsRaw)
	}

	// Verify errors.Is works
	if !errors.Is(err, ErrLockConflict) {
		t.Error("Expected errors.Is(err, ErrLockConflict) to be true")
	}
}

func TestDetectConflictsInData_MultipleConflicts(t *testing.T) {
	data := []byte(`<<<<<<< HEAD
line1
=======
line2
>>>>>>> branch
normal line
<<<<<<< HEAD
line3
=======
line4
>>>>>>> branch
`)
	err := detectConflictsInData(data)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var conflictErr *LockConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("Expected LockConflictError, got %T", err)
	}
	if len(conflictErr.Conflicts) != 2 {
		t.Fatalf("Expected 2 conflicts, got %d", len(conflictErr.Conflicts))
	}
}

func TestFileLockStore_Load_WithMergeConflicts(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	// Write a lockfile with merge conflict markers
	content := `schema_version: "1.2"
vendors:
<<<<<<< HEAD
  - name: vendor-a
    ref: main
    commit_hash: abc123
    updated: "2024-01-01T00:00:00Z"
=======
  - name: vendor-a
    ref: main
    commit_hash: def456
    updated: "2024-01-02T00:00:00Z"
>>>>>>> feature-branch
`
	lockPath := filepath.Join(vendorDir, "vendor.lock")
	if err := os.WriteFile(lockPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test lockfile: %v", err)
	}

	store := NewFileLockStore(vendorDir)
	_, err := store.Load()
	if err == nil {
		t.Fatal("Expected error when loading lockfile with merge conflicts")
	}

	if !errors.Is(err, ErrLockConflict) {
		t.Errorf("Expected ErrLockConflict, got: %v", err)
	}

	if !strings.Contains(err.Error(), "merge conflict") {
		t.Errorf("Error message should mention merge conflict, got: %s", err.Error())
	}
}

func TestLockConflictError_ErrorMessage(t *testing.T) {
	err := &LockConflictError{
		Conflicts: []types.LockConflict{
			{LineNumber: 5},
			{LineNumber: 20},
		},
	}
	msg := err.Error()
	if !strings.Contains(msg, "2 conflict region(s)") {
		t.Errorf("Error should mention 2 conflict regions, got: %s", msg)
	}
	if !strings.Contains(msg, "line 5") {
		t.Errorf("Error should mention line 5, got: %s", msg)
	}
	if !strings.Contains(msg, "line 20") {
		t.Errorf("Error should mention line 20, got: %s", msg)
	}
}

// ============================================================================
// MergeLockEntries Tests
// ============================================================================

func TestMergeLockEntries_NonOverlapping(t *testing.T) {
	ours := types.VendorLock{
		SchemaVersion: "1.2",
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa111", Updated: "2024-01-01T00:00:00Z"},
		},
	}
	theirs := types.VendorLock{
		SchemaVersion: "1.2",
		Vendors: []types.LockDetails{
			{Name: "vendor-b", Ref: "main", CommitHash: "bbb222", Updated: "2024-01-01T00:00:00Z"},
		},
	}

	result := MergeLockEntries(&ours, &theirs)

	if len(result.Conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(result.Conflicts))
	}
	if len(result.Merged.Vendors) != 2 {
		t.Fatalf("Expected 2 vendors in merged result, got %d", len(result.Merged.Vendors))
	}

	// Verify both vendors are present
	names := map[string]bool{}
	for _, v := range result.Merged.Vendors {
		names[v.Name] = true
	}
	if !names["vendor-a"] || !names["vendor-b"] {
		t.Errorf("Expected both vendor-a and vendor-b, got: %v", names)
	}
}

func TestMergeLockEntries_SameVendorDifferentCommits_TimestampWins(t *testing.T) {
	ours := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "old111", Updated: "2024-01-01T00:00:00Z"},
		},
	}
	theirs := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "new222", Updated: "2024-01-02T00:00:00Z"},
		},
	}

	result := MergeLockEntries(&ours, &theirs)

	if len(result.Conflicts) != 0 {
		t.Errorf("Expected 0 conflicts (timestamp resolves), got %d", len(result.Conflicts))
	}
	if len(result.Merged.Vendors) != 1 {
		t.Fatalf("Expected 1 vendor, got %d", len(result.Merged.Vendors))
	}
	// Theirs has later timestamp, should win
	if result.Merged.Vendors[0].CommitHash != "new222" {
		t.Errorf("Expected theirs commit 'new222' to win, got '%s'", result.Merged.Vendors[0].CommitHash)
	}
}

func TestMergeLockEntries_SameVendorSameCommit(t *testing.T) {
	ours := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "same123", Updated: "2024-01-01T00:00:00Z"},
		},
	}
	theirs := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "same123", Updated: "2024-01-02T00:00:00Z"},
		},
	}

	result := MergeLockEntries(&ours, &theirs)

	if len(result.Conflicts) != 0 {
		t.Errorf("Expected 0 conflicts (same commit), got %d", len(result.Conflicts))
	}
	if len(result.Merged.Vendors) != 1 {
		t.Fatalf("Expected 1 vendor, got %d", len(result.Merged.Vendors))
	}
	// Theirs has later Updated timestamp, so theirs metadata should win
	if result.Merged.Vendors[0].Updated != "2024-01-02T00:00:00Z" {
		t.Errorf("Expected later timestamp '2024-01-02T00:00:00Z', got '%s'", result.Merged.Vendors[0].Updated)
	}
}

func TestMergeLockEntries_SameTimestampDifferentCommits_LexicographicTiebreaker(t *testing.T) {
	ours := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "aaa111", Updated: "2024-01-01T00:00:00Z"},
		},
	}
	theirs := types.VendorLock{
		Vendors: []types.LockDetails{
			{Name: "vendor-a", Ref: "main", CommitHash: "zzz999", Updated: "2024-01-01T00:00:00Z"},
		},
	}

	result := MergeLockEntries(&ours, &theirs)

	if len(result.Conflicts) != 0 {
		t.Errorf("Expected 0 conflicts (lexicographic resolves), got %d", len(result.Conflicts))
	}
	// Higher hash (zzz999) should win
	if result.Merged.Vendors[0].CommitHash != "zzz999" {
		t.Errorf("Expected 'zzz999' to win lexicographically, got '%s'", result.Merged.Vendors[0].CommitHash)
	}
}

func TestMergeLockEntries_SchemaVersionPicksNewer(t *testing.T) {
	ours := types.VendorLock{SchemaVersion: "1.1"}
	theirs := types.VendorLock{SchemaVersion: "1.2"}

	result := MergeLockEntries(&ours, &theirs)

	if result.Merged.SchemaVersion != "1.2" {
		t.Errorf("Expected schema version '1.2', got '%s'", result.Merged.SchemaVersion)
	}
}

func TestIsLockConflictError(t *testing.T) {
	err := &LockConflictError{Conflicts: []types.LockConflict{{LineNumber: 1}}}
	if !IsLockConflictError(err) {
		t.Error("IsLockConflictError should return true for LockConflictError")
	}
	if IsLockConflictError(errors.New("other error")) {
		t.Error("IsLockConflictError should return false for non-LockConflictError")
	}
}

func TestFileLockStore_Save_OverridesExistingVersion(t *testing.T) {
	tempDir := t.TempDir()
	vendorDir := filepath.Join(tempDir, VendorDir)
	_ = os.MkdirAll(vendorDir, 0755)

	store := NewFileLockStore(vendorDir)

	// Save lock with a different schema version (simulating migration)
	lock := types.VendorLock{
		SchemaVersion: "0.9", // Old version
		Vendors: []types.LockDetails{
			{
				Name:       "test-vendor",
				Ref:        "main",
				CommitHash: "abc123",
				Updated:    "2024-01-01T00:00:00Z",
			},
		},
	}

	err := store.Save(lock)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify version was updated
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion should be %q after save, got %q", CurrentSchemaVersion, loaded.SchemaVersion)
	}
}
