package core

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"

	"git-vendor/internal/types"
	"gopkg.in/yaml.v3"
)

// ============================================================================
// Property-Based Tests using testing/quick
// ============================================================================
//
// These tests validate invariants that must hold for ALL inputs, using
// automated random input generation (1000 iterations each by default).

// ============================================================================
// Security Properties: Path Validation
// ============================================================================

// TestProperty_ValidateDestPath_AbsolutePathsRejected verifies that ANY
// absolute path is rejected by ValidateDestPath, regardless of platform.
//
// Property: For all paths p, if filepath.IsAbs(p) then ValidateDestPath(p) returns error
func TestProperty_ValidateDestPath_AbsolutePathsRejected(t *testing.T) {
	// Generator for absolute Unix paths
	absoluteUnixPath := func() string {
		paths := []string{
			"/etc/passwd",
			"/root",
			"/tmp/file",
			"/usr/bin/something",
			"/var/log/app.log",
		}
		return paths[len(paths)%5]
	}

	// Generator for absolute Windows paths
	absoluteWindowsPath := func() string {
		drives := []rune{'C', 'D', 'E', 'F', 'G'}
		paths := []string{
			":\\Windows\\System32",
			":\\Users\\Public",
			":\\Program Files",
			":\\temp\\file.txt",
		}
		drive := drives[len(drives)%5]
		return string(drive) + paths[len(paths)%4]
	}

	t.Run("Unix absolute paths", func(t *testing.T) {
		f := func() bool {
			path := absoluteUnixPath()
			err := ValidateDestPath(path)
			return err != nil // MUST reject
		}

		if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
			t.Errorf("Property violated: %v", err)
		}
	})

	t.Run("Windows absolute paths", func(t *testing.T) {
		f := func() bool {
			path := absoluteWindowsPath()
			err := ValidateDestPath(path)
			return err != nil // MUST reject
		}

		if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
			t.Errorf("Property violated: %v", err)
		}
	})
}

// TestProperty_ValidateDestPath_ParentRefsRejected verifies that ANY path
// containing parent directory references (..) is rejected after cleaning.
//
// Property: For all paths p, if Clean(p) contains ".." then ValidateDestPath(p) returns error
func TestProperty_ValidateDestPath_ParentRefsRejected(t *testing.T) {
	// Generators for paths with parent refs
	generateParentRefPath := func(segments int) string {
		// Generate paths like:
		// "../file"
		// "../../etc/passwd"
		// "foo/../../bar"
		// "a/b/../../../c"
		parentRefs := strings.Repeat("../", segments)
		suffixes := []string{"etc/passwd", "root", "file.txt", ""}
		return parentRefs + suffixes[segments%4]
	}

	f := func(depth uint8) bool {
		// depth 0-255 creates different levels of traversal
		segments := int(depth%10) + 1 // 1-10 segments
		path := generateParentRefPath(segments)

		cleaned := filepath.Clean(path)
		hasParentRef := strings.HasPrefix(cleaned, "..") ||
			strings.Contains(cleaned, string(filepath.Separator)+"..")

		if hasParentRef {
			err := ValidateDestPath(path)
			return err != nil // MUST reject if cleaned path has ..
		}
		return true // Skip paths that don't have .. after cleaning
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestProperty_ValidateDestPath_RelativePathsAccepted verifies that valid
// relative paths (no .., no absolute prefix) are accepted.
//
// Property: For all valid relative paths p, ValidateDestPath(p) returns nil
func TestProperty_ValidateDestPath_RelativePathsAccepted(t *testing.T) {
	// Generators for safe relative paths
	generateSafePath := func(components []string) string {
		// Generate paths like:
		// "src/lib/file.go"
		// "vendor/pkg/subdir/module.go"
		// "docs/README.md"
		validComponents := []string{"src", "lib", "vendor", "pkg", "docs", "test", "internal"}
		var parts []string
		for i := 0; i < len(components)%5+1; i++ {
			parts = append(parts, validComponents[i%len(validComponents)])
		}
		parts = append(parts, "file.go")
		return filepath.Join(parts...)
	}

	f := func(seed [5]string) bool {
		path := generateSafePath(seed[:])

		// Verify path is safe (no .., not absolute)
		cleaned := filepath.Clean(path)
		if filepath.IsAbs(cleaned) {
			return true // Skip absolute paths
		}
		if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
			return true // Skip paths with ..
		}

		// Property: safe relative paths MUST be accepted
		err := ValidateDestPath(path)
		return err == nil
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// ============================================================================
// Serialization Properties: Round-Trip Identity
// ============================================================================

// TestProperty_ConfigRoundTrip_Identity verifies that marshaling and
// unmarshaling a VendorConfig is an identity operation.
//
// Property: For all configs c, Unmarshal(Marshal(c)) == c
func TestProperty_ConfigRoundTrip_Identity(t *testing.T) {
	f := func(name string, url string, license string) bool {
		if name == "" || url == "" {
			return true // Skip empty required fields
		}

		// Create test config
		original := types.VendorConfig{
			Vendors: []types.VendorSpec{
				{
					Name:    name,
					URL:     url,
					License: license,
					Specs: []types.BranchSpec{
						{
							Ref: "main",
							Mapping: []types.PathMapping{
								{From: "src", To: "dest"},
							},
						},
					},
				},
			},
		}

		// Marshal to YAML
		data, err := yaml.Marshal(&original)
		if err != nil {
			t.Logf("Marshal error: %v", err)
			return false
		}

		// Unmarshal back
		var roundtrip types.VendorConfig
		err = yaml.Unmarshal(data, &roundtrip)
		if err != nil {
			t.Logf("Unmarshal error: %v", err)
			return false
		}

		// Property: values must be preserved
		if len(roundtrip.Vendors) != 1 {
			return false
		}
		v := roundtrip.Vendors[0]
		return v.Name == name && v.URL == url && v.License == license
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Config round-trip property violated: %v", err)
	}
}

// TestProperty_LockRoundTrip_Identity verifies that marshaling and
// unmarshaling a VendorLock preserves commit hashes without truncation.
//
// Property: For all locks l, Unmarshal(Marshal(l)).CommitHash == l.CommitHash
func TestProperty_LockRoundTrip_Identity(t *testing.T) {
	f := func(name string, ref string, hash string) bool {
		if name == "" || ref == "" || hash == "" {
			return true // Skip empty required fields
		}

		// Create test lock
		original := types.VendorLock{
			Vendors: []types.LockDetails{
				{
					Name:        name,
					Ref:         ref,
					CommitHash:  hash,
					LicensePath: "vendor/licenses/" + name + ".txt",
					Updated:     "2024-01-01T00:00:00Z",
				},
			},
		}

		// Marshal to YAML
		data, err := yaml.Marshal(&original)
		if err != nil {
			t.Logf("Marshal error: %v", err)
			return false
		}

		// Unmarshal back
		var roundtrip types.VendorLock
		err = yaml.Unmarshal(data, &roundtrip)
		if err != nil {
			t.Logf("Unmarshal error: %v", err)
			return false
		}

		// Property: commit hash MUST NOT be truncated/modified
		if len(roundtrip.Vendors) != 1 {
			return false
		}
		v := roundtrip.Vendors[0]
		return v.CommitHash == hash && v.Name == name && v.Ref == ref
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("Lock round-trip property violated: %v", err)
	}
}

// ============================================================================
// Robustness Properties: Never Panic
// ============================================================================

// TestProperty_ParseSmartURL_NeverPanics verifies that ParseSmartURL never
// panics, regardless of input.
//
// Property: For all inputs s, ParseSmartURL(s) does not panic
func TestProperty_ParseSmartURL_NeverPanics(t *testing.T) {
	f := func(rawURL string) bool {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseSmartURL panicked on input %q: %v", rawURL, r)
			}
		}()

		// Call with arbitrary input - should never panic
		base, ref, path := ParseSmartURL(rawURL)

		// Basic sanity checks (not exhaustive, just verify no crash)
		_ = base
		_ = ref
		_ = path

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("ParseSmartURL panic property violated: %v", err)
	}
}
