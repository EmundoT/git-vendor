// Package types defines data structures for git-vendor configuration and state management.
//
//nolint:revive // Package name "types" is standard and appropriate
package types

// VendorPolicy defines commit guard and status policy for vendor drift and staleness.
// Pointer types distinguish "not set" (nil) from "explicitly false/0" so that
// per-vendor overrides only replace fields they explicitly declare.
//
// Defaults (when nil): BlockOnDrift=true, BlockOnStale=false, MaxStalenessDays=0 (no limit).
type VendorPolicy struct {
	BlockOnDrift     *bool `yaml:"block_on_drift,omitempty" json:"block_on_drift,omitempty"`
	BlockOnStale     *bool `yaml:"block_on_stale,omitempty" json:"block_on_stale,omitempty"`
	MaxStalenessDays *int  `yaml:"max_staleness_days,omitempty" json:"max_staleness_days,omitempty"`
}

// PolicyViolation represents a single policy rule that a vendor has violated.
// PolicyViolation is produced by policy evaluation and surfaced in StatusResult
// and commit guard output.
type PolicyViolation struct {
	VendorName string `json:"vendor_name"`
	Type       string `json:"type"`     // "drift" or "stale"
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error" (blocks commit) or "warning" (report only)
}

// ResolvedPolicy merges a per-vendor VendorPolicy into a global VendorPolicy,
// with per-vendor fields winning when non-nil. Returns a fully-populated
// VendorPolicy with no nil pointers.
//
// Defaults: BlockOnDrift=true, BlockOnStale=false, MaxStalenessDays=0.
func ResolvedPolicy(global, perVendor *VendorPolicy) VendorPolicy {
	defaultTrue := true
	defaultFalse := false
	defaultZero := 0

	resolved := VendorPolicy{
		BlockOnDrift:     &defaultTrue,
		BlockOnStale:     &defaultFalse,
		MaxStalenessDays: &defaultZero,
	}

	// Apply global overrides (copy values to avoid aliasing the caller's pointers).
	if global != nil {
		if global.BlockOnDrift != nil {
			v := *global.BlockOnDrift
			resolved.BlockOnDrift = &v
		}
		if global.BlockOnStale != nil {
			v := *global.BlockOnStale
			resolved.BlockOnStale = &v
		}
		if global.MaxStalenessDays != nil {
			v := *global.MaxStalenessDays
			resolved.MaxStalenessDays = &v
		}
	}

	// Apply per-vendor overrides (wins over global). Same copy-value pattern.
	if perVendor != nil {
		if perVendor.BlockOnDrift != nil {
			v := *perVendor.BlockOnDrift
			resolved.BlockOnDrift = &v
		}
		if perVendor.BlockOnStale != nil {
			v := *perVendor.BlockOnStale
			resolved.BlockOnStale = &v
		}
		if perVendor.MaxStalenessDays != nil {
			v := *perVendor.MaxStalenessDays
			resolved.MaxStalenessDays = &v
		}
	}

	return resolved
}

// ComplianceConfig defines the global compliance enforcement block in vendor.yml.
// ComplianceConfig controls how per-vendor compliance levels are resolved:
//   - Default: the fallback enforcement level for vendors without explicit compliance
//   - Mode: "default" lets per-vendor override global; "override" forces global for all
type ComplianceConfig struct {
	Default string `yaml:"default,omitempty" json:"default,omitempty"` // "strict", "lenient", or "info" (default: "lenient")
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`      // "default" or "override" (default: "default")
}

// VendorConfig represents the root configuration file (vendor.yml) structure.
type VendorConfig struct {
	Policy     *VendorPolicy    `yaml:"policy,omitempty" json:"policy,omitempty"`     // Global policy defaults
	Compliance *ComplianceConfig `yaml:"compliance,omitempty" json:"compliance,omitempty"` // Global compliance enforcement (Spec 075)
	Vendors    []VendorSpec     `yaml:"vendors"`
}

// VendorSpec defines a single vendored dependency with source repository URL and path mappings.
type VendorSpec struct {
	Name       string        `yaml:"name"`
	URL        string        `yaml:"url"`
	Mirrors    []string      `yaml:"mirrors,omitempty"`    // Fallback URLs, tried in declaration order after URL
	License    string        `yaml:"license"`
	Groups     []string      `yaml:"groups,omitempty"`     // Optional groups for batch operations
	Hooks      *HookConfig   `yaml:"hooks,omitempty"`      // Optional pre/post sync hooks
	Policy     *VendorPolicy `yaml:"policy,omitempty"`     // Per-vendor policy overrides
	Source      string        `yaml:"source,omitempty"`      // "" (external, default) or "internal"
	Direction   string        `yaml:"direction,omitempty"`   // "" (source-canonical) or "bidirectional" (Spec 070 sync direction)
	Enforcement string        `yaml:"compliance,omitempty"`  // "" (inherits global) or "strict"/"lenient"/"info" (Spec 075)
	Specs       []BranchSpec  `yaml:"specs"`
}

// BranchSpec defines mappings for a specific Git ref (branch, tag, or commit).
type BranchSpec struct {
	Ref           string        `yaml:"ref"`
	DefaultTarget string        `yaml:"default_target,omitempty"`
	Mapping       []PathMapping `yaml:"mapping"`
}

// PathMapping defines a source-to-destination path mapping for vendoring.
// When From is a directory, Exclude patterns (gitignore-style globs) skip
// matching files during sync. Exclude has no effect on file-level mappings.
type PathMapping struct {
	From    string   `yaml:"from"`
	To      string   `yaml:"to"`
	Exclude []string `yaml:"exclude,omitempty"`
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

	// Multi-remote provenance (schema v1.3)
	SourceURL string `yaml:"source_url,omitempty"` // Which URL actually served the content (empty = primary URL)

	// Accepted drift metadata (CLI-003)
	AcceptedDrift map[string]string `yaml:"accepted_drift,omitempty"` // path -> SHA-256 of accepted local content

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

// LockConflict represents a merge conflict detected in a vendor.lock file.
// LockConflict is returned when git merge markers are found, providing
// structured context for error reporting instead of a cryptic YAML parse failure.
type LockConflict struct {
	LineNumber int    // Line where the conflict marker starts
	OursRaw    string // Content from the "ours" side (between <<<<<<< and =======)
	TheirsRaw  string // Content from the "theirs" side (between ======= and >>>>>>>)
}

// LockMergeConflict represents a vendor entry that could not be auto-merged
// because both sides modified the same vendor with no clear resolution strategy.
type LockMergeConflict struct {
	VendorName string      // Vendor name in conflict
	Ref        string      // Ref in conflict
	Ours       LockDetails // Lock entry from "ours" side
	Theirs     LockDetails // Lock entry from "theirs" side
}

// LockMergeResult holds the outcome of merging two VendorLock structs.
type LockMergeResult struct {
	Merged    VendorLock          // Successfully merged lock
	Conflicts []LockMergeConflict // Entries requiring manual resolution
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

// VerifySummary contains aggregate statistics for verification.
// Stale and Orphaned track config/lock coherence issues (VFY-001):
//   - Stale: config mapping destinations with no corresponding lock FileHashes entry
//   - Orphaned: lock FileHashes entries with no corresponding config mapping destination
type VerifySummary struct {
	TotalFiles int    `json:"total_files"`
	Verified   int    `json:"verified"`
	Modified   int    `json:"modified"`
	Added      int    `json:"added"`
	Deleted    int    `json:"deleted"`
	Accepted   int    `json:"accepted"` // Files with accepted drift (CLI-003)
	Stale      int    `json:"stale"`    // Config mappings not present in lock FileHashes
	Orphaned   int    `json:"orphaned"` // Lock FileHashes entries not present in config mappings
	Result     string `json:"result"`   // PASS, FAIL, WARN
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
	Status       string          `json:"status"`             // verified, modified, added, deleted, accepted, stale, orphaned
	Type         string          `json:"type"`               // "file", "position", or "coherence"
	ExpectedHash *string         `json:"expected_hash,omitempty"`
	ActualHash   *string         `json:"actual_hash,omitempty"`
	Position     *PositionDetail `json:"position,omitempty"` // Present only for type="position"
}

// DriftDetail provides per-file hash comparison for drift detection (GRD-001).
// DriftDetail is included in VendorStatusDetail JSON output so pre-commit hooks
// can display lock vs disk hash mismatches without re-computing hashes.
type DriftDetail struct {
	Path     string `json:"path"`
	LockHash string `json:"lock_hash"`
	DiskHash string `json:"disk_hash"`
	Accepted bool   `json:"accepted"`
}

// VendorStatusDetail holds combined verify + outdated information for a single vendor/ref pair.
// VendorStatusDetail is produced by the status command to merge offline (disk) and remote checks.
type VendorStatusDetail struct {
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	CommitHash  string `json:"commit_hash"`
	Enforcement string `json:"enforcement,omitempty"` // Resolved compliance level: "strict", "lenient", or "info" (Spec 075)

	// Offline (verify) results
	FilesVerified int      `json:"files_verified"`
	FilesModified int      `json:"files_modified"`
	FilesAdded    int      `json:"files_added"`
	FilesDeleted  int      `json:"files_deleted"`
	FilesAccepted int      `json:"files_accepted"` // Files with accepted drift (CLI-003)
	ModifiedPaths []string `json:"modified_paths,omitempty"`
	AddedPaths    []string `json:"added_paths,omitempty"`
	DeletedPaths  []string `json:"deleted_paths,omitempty"`
	AcceptedPaths []string `json:"accepted_paths,omitempty"`

	// Per-file drift details with hash comparison (GRD-001).
	// Populated for modified and accepted files when offline checks run.
	DriftDetails []DriftDetail `json:"drift_details,omitempty"`

	// Lock age metadata for staleness policy evaluation (GRD-003).
	// LastUpdated is the RFC3339 timestamp from LockDetails.Updated, recording when
	// the lock entry was last written. Used by PolicyService to compare against
	// MaxStalenessDays when the vendor is behind upstream.
	LastUpdated string `json:"last_updated,omitempty"`

	// Remote (outdated) results — nil when --offline
	UpstreamHash    string `json:"upstream_hash,omitempty"`
	UpstreamStale   *bool  `json:"upstream_stale,omitempty"`   // nil = not checked
	UpstreamSkipped bool   `json:"upstream_skipped,omitempty"` // true = ls-remote failed

	// Policy violations for this vendor (GRD-002).
	// Populated when policy section is present in vendor.yml.
	PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`
}

// StatusResult holds the combined output of the status command (verify + outdated).
// StatusResult is the top-level return type for Manager.Status / VendorSyncer.Status.
type StatusResult struct {
	Vendors          []VendorStatusDetail `json:"vendors"`
	Summary          StatusSummary        `json:"summary"`
	PolicyViolations []PolicyViolation    `json:"policy_violations,omitempty"` // All violations across vendors (GRD-002)
	ComplianceConfig *ComplianceConfig    `json:"compliance_config,omitempty"` // Global compliance config (Spec 075)
}

// StatusSummary contains aggregate statistics across all vendors for the status command.
type StatusSummary struct {
	TotalVendors   int    `json:"total_vendors"`
	TotalFiles     int    `json:"total_files"`
	Verified       int    `json:"verified"`
	Modified       int    `json:"modified"`
	Added          int    `json:"added"`
	Deleted        int    `json:"deleted"`
	Accepted       int    `json:"accepted"`        // Files with accepted drift (CLI-003)
	Stale          int    `json:"stale"`            // Vendors behind upstream
	UpstreamErrors int    `json:"upstream_errors"`  // Vendors where ls-remote failed
	StaleConfigs   int    `json:"stale_configs"`    // Config mapping dests with no lock FileHashes entry (VFY-001)
	OrphanedLock   int    `json:"orphaned_lock"`    // Lock FileHashes entries with no config mapping dest (VFY-001)
	Result         string `json:"result"`           // PASS, FAIL, WARN
}
