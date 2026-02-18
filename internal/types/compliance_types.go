// Package types defines compliance-related data structures for internal vendor tracking (Spec 070).
//
//nolint:revive // Package name "types" is standard and appropriate
package types

// ComplianceDriftDirection represents the drift state between source and destination.
type ComplianceDriftDirection string

// ComplianceDriftDirection values for internal vendor compliance.
const (
	DriftSynced       ComplianceDriftDirection = "synced"
	DriftSourceDrift  ComplianceDriftDirection = "source_drifted"
	DriftDestDrift    ComplianceDriftDirection = "dest_drifted"
	DriftBothDrift    ComplianceDriftDirection = "both_drifted"
)

// ComplianceResult holds the full compliance check output for internal vendors.
type ComplianceResult struct {
	SchemaVersion string            `json:"schema_version"`
	Timestamp     string            `json:"timestamp"`
	Entries       []ComplianceEntry `json:"entries"`
	Summary       ComplianceSummary `json:"summary"`
}

// ComplianceEntry represents the compliance state of a single source-to-destination mapping.
type ComplianceEntry struct {
	VendorName        string                   `json:"vendor_name"`
	FromPath          string                   `json:"from_path"`
	ToPath            string                   `json:"to_path"`
	Direction         ComplianceDriftDirection `json:"direction"`
	SyncDirection     string                   `json:"sync_direction"`       // "source-canonical" or "bidirectional" (renamed from Compliance for Spec 075)
	SourceHashLocked  string                   `json:"source_hash_locked"`
	SourceHashCurrent string                   `json:"source_hash_current"`
	DestHashLocked    string                   `json:"dest_hash_locked"`
	DestHashCurrent   string                   `json:"dest_hash_current"`
	Action            string                   `json:"action"` // suggested action
}

// ComplianceSummary aggregates compliance statistics.
type ComplianceSummary struct {
	Total       int    `json:"total"`
	Synced      int    `json:"synced"`
	SourceDrift int    `json:"source_drift"`
	DestDrift   int    `json:"dest_drift"`
	BothDrift   int    `json:"both_drift"`
	Result      string `json:"result"` // "SYNCED" | "DRIFTED" | "CONFLICT"
}
