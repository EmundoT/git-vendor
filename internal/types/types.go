// Package types defines data structures for git-vendor configuration and state management.
//
//nolint:revive // Package name "types" is standard and appropriate
package types

// VendorConfig represents the root configuration file (vendor.yml) structure.
type VendorConfig struct {
	Vendors []VendorSpec `yaml:"vendors"`
}

// VendorSpec defines a single vendored dependency with source repository URL and path mappings.
type VendorSpec struct {
	Name       string       `yaml:"name"`
	URL        string       `yaml:"url"`
	License    string       `yaml:"license"`
	Groups     []string     `yaml:"groups,omitempty"`     // Optional groups for batch operations
	Hooks      *HookConfig  `yaml:"hooks,omitempty"`      // Optional pre/post sync hooks
	Source     string       `yaml:"source,omitempty"`     // "" (external, default) or "internal"
	Compliance string       `yaml:"compliance,omitempty"` // "" (source-canonical) or "bidirectional"
	Specs      []BranchSpec `yaml:"specs"`
}

// BranchSpec defines mappings for a specific Git ref (branch, tag, or commit).
type BranchSpec struct {
	Ref           string        `yaml:"ref"`
	DefaultTarget string        `yaml:"default_target,omitempty"`
	Mapping       []PathMapping `yaml:"mapping"`
}

// PathMapping defines a source-to-destination path mapping for vendoring.
type PathMapping struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// VendorLock represents the lock file (vendor.lock) storing resolved commit hashes.
//
// Schema version uses major.minor format:
//   - Minor bump: new optional fields added (backward compatible)
//   - Major bump: breaking changes requiring CLI upgrade
//
// Version compatibility:
//   - Missing schema_version is treated as "1.0"
//   - Unknown minor versions: warning, operation proceeds, unknown fields preserved
//   - Unknown major versions: error, operation aborts to prevent data corruption
//
// Current version: 1.1 (adds LicenseSPDX, SourceVersionTag, VendoredAt,
// VendoredBy, LastSyncedAt). Migrate via "git-vendor migrate".
type VendorLock struct {
	SchemaVersion string        `yaml:"schema_version,omitempty"`
	Vendors       []LockDetails `yaml:"vendors"`
}

// LockDetails contains the locked state for a specific vendor and ref.
type LockDetails struct {
	Name        string            `yaml:"name"`
	Ref         string            `yaml:"ref"`
	CommitHash  string            `yaml:"commit_hash"`
	LicensePath string            `yaml:"license_path"`          // Automatically managed
	Updated     string            `yaml:"updated"`
	FileHashes  map[string]string `yaml:"file_hashes,omitempty"` // path -> SHA-256 hash

	// Metadata fields (schema v1.1)
	LicenseSPDX      string `yaml:"license_spdx,omitempty"`       // SPDX license identifier
	SourceVersionTag string `yaml:"source_version_tag,omitempty"` // Git tag matching commit (if any)
	VendoredAt       string `yaml:"vendored_at,omitempty"`        // ISO 8601 timestamp of initial vendoring
	VendoredBy       string `yaml:"vendored_by,omitempty"`        // Git user identity who performed the vendoring
	LastSyncedAt     string `yaml:"last_synced_at,omitempty"`     // ISO 8601 timestamp of most recent sync

	// Position extraction metadata (spec 071)
	Positions []PositionLock `yaml:"positions,omitempty"` // Position-extracted mappings with source hashes

	// Internal vendor metadata (spec 070)
	Source           string            `yaml:"source,omitempty"`              // "internal" for internal vendors
	SourceFileHashes map[string]string `yaml:"source_file_hashes,omitempty"` // source path -> SHA-256
}

// PositionLock records a position-extracted mapping in the lockfile for auditing and verification.
type PositionLock struct {
	From       string `yaml:"from"`        // Source path with position (e.g., "api/constants.go:L4-L6")
	To         string `yaml:"to"`          // Destination path with optional position
	SourceHash string `yaml:"source_hash"` // SHA-256 of extracted content
}

// PathConflict represents a conflict between two vendors mapping to overlapping paths
type PathConflict struct {
	Path     string
	Vendor1  string
	Vendor2  string
	Mapping1 PathMapping
	Mapping2 PathMapping
}

// CloneOptions holds options for git clone operations
type CloneOptions struct {
	Filter     string // e.g., "blob:none"
	NoCheckout bool
	Depth      int
}

// Trailer represents a single key-value git trailer.
// Multiple Trailers with the same Key are valid for multi-valued trailers
// (e.g., multiple Vendor-Name entries in a multi-vendor commit).
type Trailer struct {
	Key   string
	Value string
}

// CommitOptions holds options for creating a git commit with structured trailers.
// CommitOptions is used by CommitVendorChanges to pass message and trailer data
// to the GitClient.Commit adapter. Trailers are ordered and support duplicate keys.
type CommitOptions struct {
	Message  string
	Trailers []Trailer
}

// VendorStatus represents the sync status of a vendor
type VendorStatus struct {
	Name          string
	Ref           string
	IsSynced      bool
	MissingPaths  []string // Paths that should exist but don't
	FileCount     int      // Number of file-level mappings
	PositionCount int      // Number of position-level mappings from lockfile
}

// SyncStatus represents the overall sync status
type SyncStatus struct {
	AllSynced      bool
	VendorStatuses []VendorStatus
}

// FileChecksum represents a cached checksum for incremental sync
type FileChecksum struct {
	Path string `json:"path"`
	Hash string `json:"hash"` // SHA-256 of file content
}

// IncrementalSyncCache tracks file states for skip optimization
type IncrementalSyncCache struct {
	VendorName string         `json:"vendor_name"`
	Ref        string         `json:"ref"`
	CommitHash string         `json:"commit_hash"`
	Files      []FileChecksum `json:"files"`
	CachedAt   string         `json:"cached_at"` // RFC3339 timestamp
}

// ProgressTracker represents a progress indicator for long-running operations
type ProgressTracker interface {
	// Increment advances progress by one unit with an optional status message
	Increment(message string)

	// SetTotal updates the total expected units (for dynamic totals)
	SetTotal(total int)

	// Complete marks the operation as successfully finished
	Complete()

	// Fail marks the operation as failed with an error
	Fail(err error)
}

// UpdateCheckResult represents an available update for a vendor
type UpdateCheckResult struct {
	VendorName  string `json:"vendor_name"`
	Ref         string `json:"ref"`
	CurrentHash string `json:"current_hash"`
	LatestHash  string `json:"latest_hash"`
	LastUpdated string `json:"last_updated"`
	UpToDate    bool   `json:"up_to_date"`
}

// OutdatedResult aggregates the results of checking all vendors for staleness.
// OutdatedResult is returned by OutdatedService.Outdated and consumed by the
// "outdated" CLI command for both human-readable and JSON output.
type OutdatedResult struct {
	Dependencies []UpdateCheckResult `json:"dependencies"`
	TotalChecked int                 `json:"total_checked"`
	Outdated     int                 `json:"outdated"`
	UpToDate     int                 `json:"up_to_date"`
	Skipped      int                 `json:"skipped"`
}

// CommitInfo represents a single git commit
type CommitInfo struct {
	Hash      string
	ShortHash string
	Subject   string
	Author    string
	Date      string
}

// VendorDiff represents the commit history between two refs
type VendorDiff struct {
	VendorName  string
	Ref         string
	OldHash     string
	NewHash     string
	OldDate     string
	NewDate     string
	Commits     []CommitInfo
	CommitCount int
}

// HookConfig defines pre/post sync shell commands for automation
type HookConfig struct {
	PreSync  string `yaml:"pre_sync,omitempty"`  // Shell command to run before sync
	PostSync string `yaml:"post_sync,omitempty"` // Shell command to run after sync
}

// HookContext provides environment context for hook execution
type HookContext struct {
	VendorName  string            // Name of the vendor being synced
	VendorURL   string            // URL of the vendor repository
	Ref         string            // Git ref being synced
	CommitHash  string            // Resolved commit hash
	RootDir     string            // Project root directory
	FilesCopied int               // Number of files copied
	DirsCreated int               // Number of directories created
	Environment map[string]string // Additional environment variables
}

// ParallelOptions configures parallel processing behavior
type ParallelOptions struct {
	Enabled    bool // Whether parallel processing is enabled
	MaxWorkers int  // Maximum concurrent workers (0 = use NumCPU)
}

// VerifyResult represents the outcome of verification
type VerifyResult struct {
	SchemaVersion  string            `json:"schema_version"`
	Timestamp      string            `json:"timestamp"`
	Summary        VerifySummary     `json:"summary"`
	Files          []FileStatus      `json:"files"`
	InternalStatus []ComplianceEntry `json:"internal_status,omitempty"` // Spec 070: internal vendor drift
}

// VerifySummary contains aggregate statistics for verification
type VerifySummary struct {
	TotalFiles int    `json:"total_files"`
	Verified   int    `json:"verified"`
	Modified   int    `json:"modified"`
	Added      int    `json:"added"`
	Deleted    int    `json:"deleted"`
	Result     string `json:"result"` // PASS, FAIL, WARN
}

// PositionDetail provides position-level metadata for FileStatus entries
// that originate from position-extracted mappings.
type PositionDetail struct {
	From       string `json:"from"`        // Source path with position (e.g., "api/constants.go:L4-L6")
	To         string `json:"to"`          // Destination path with optional position
	SourceHash string `json:"source_hash"` // SHA-256 of extracted content at sync time
}

// FileStatus represents the verification status of a single file
type FileStatus struct {
	Path         string          `json:"path"`
	Vendor       *string         `json:"vendor"`
	Status       string          `json:"status"`             // verified, modified, added, deleted
	Type         string          `json:"type"`               // "file" or "position"
	ExpectedHash *string         `json:"expected_hash,omitempty"`
	ActualHash   *string         `json:"actual_hash,omitempty"`
	Position     *PositionDetail `json:"position,omitempty"` // Present only for type="position"
}
