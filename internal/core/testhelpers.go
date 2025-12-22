package core

import (
	"os"
	"time"

	"git-vendor/internal/types"
)

// ============================================================================
// Common Test Helpers
// ============================================================================

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

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 1024 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// ============================================================================
// String Helpers
// ============================================================================

// contains checks if string s contains substring substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring finds a substring within a string
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
