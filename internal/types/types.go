// Package types defines data structures for git-vendor configuration and state management.
//
//nolint:revive // Package name "types" is standard and appropriate
package types

// VendorConfig represents the root configuration file (vendor.yml) structure.
type VendorConfig struct {
	Vendors []VendorSpec `yaml:"vendors"`
}

// VendorSpec defines a single vendored dependency with its source repository and mappings.
type VendorSpec struct {
	Name    string       `yaml:"name"`
	URL     string       `yaml:"url"`
	License string       `yaml:"license"`
	Groups  []string     `yaml:"groups,omitempty"` // Optional groups for batch operations
	Specs   []BranchSpec `yaml:"specs"`
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
type VendorLock struct {
	Vendors []LockDetails `yaml:"vendors"`
}

// LockDetails contains the locked state for a specific vendor and ref.
type LockDetails struct {
	Name        string `yaml:"name"`
	Ref         string `yaml:"ref"`
	CommitHash  string `yaml:"commit_hash"`
	LicensePath string `yaml:"license_path"` // Automatically managed
	Updated     string `yaml:"updated"`
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

// VendorStatus represents the sync status of a vendor
type VendorStatus struct {
	Name         string
	Ref          string
	IsSynced     bool
	MissingPaths []string // Paths that should exist but don't
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
	VendorName  string
	Ref         string
	CurrentHash string
	LatestHash  string
	LastUpdated string
	UpToDate    bool
}
