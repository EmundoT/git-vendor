// Package version provides version information for the application.
package version

import "fmt"

// Version information - injected by GoReleaser via ldflags during builds
// Default values are used for development builds (go run, go build)
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// GetVersion returns the version string
// Returns "dev" for development builds, or the actual version (e.g., "v0.1.0-beta.1") for releases
func GetVersion() string {
	if Version == "dev" {
		return "dev"
	}
	return Version
}

// GetFullVersion returns version with build information
// Format: "v0.1.0-beta.1 (commit: abc123, built: 2024-12-27T10:30:00Z)"
func GetFullVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}
