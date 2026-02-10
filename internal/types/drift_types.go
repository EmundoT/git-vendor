// Package types defines data structures for git-vendor configuration and state management.
package types

// DriftResult is the top-level result for the drift detection command.
// DriftResult supports both JSON and table output formats and captures
// local drift, upstream drift, and conflict risk for all vendored dependencies.
type DriftResult struct {
	SchemaVersion string            `json:"schema_version"`
	Timestamp     string            `json:"timestamp"`
	Summary       DriftSummary      `json:"summary"`
	Dependencies  []DriftDependency `json:"dependencies"`
}

// DriftSummary contains aggregate drift statistics across all dependencies.
type DriftSummary struct {
	TotalDependencies int     `json:"total_dependencies"`
	DriftedLocal      int     `json:"drifted_local"`      // Dependencies with local modifications
	DriftedUpstream   int     `json:"drifted_upstream"`    // Dependencies with upstream changes
	ConflictRisk      int     `json:"conflict_risk"`       // Dependencies with both local + upstream changes
	Clean             int     `json:"clean"`               // Dependencies with zero drift
	OverallDriftScore float64 `json:"overall_drift_score"` // Average drift score (0-100)
	Result            string  `json:"result"`              // CLEAN, DRIFTED, CONFLICT
}

// DriftDependency represents drift analysis results for a single vendor.
type DriftDependency struct {
	Name            string     `json:"name"`
	URL             string     `json:"url"`
	Ref             string     `json:"ref"`
	LockedCommit    string     `json:"locked_commit"`
	LatestCommit    string     `json:"latest_commit,omitempty"` // Empty in offline mode
	DriftScore      float64    `json:"drift_score"`             // 0-100
	Files           []DriftFile `json:"files"`
	LocalDrift      DriftStats `json:"local_drift"`
	UpstreamDrift   DriftStats `json:"upstream_drift,omitempty"`
	HasConflictRisk bool       `json:"has_conflict_risk"`
}

// DriftStats aggregates line-level drift statistics for a category (local or upstream).
type DriftStats struct {
	FilesChanged      int     `json:"files_changed"`
	FilesUnchanged    int     `json:"files_unchanged"`
	TotalLinesAdded   int     `json:"total_lines_added"`
	TotalLinesRemoved int     `json:"total_lines_removed"`
	DriftPercentage   float64 `json:"drift_percentage"` // 0-100
}

// DriftFile represents drift analysis for a single vendored file.
type DriftFile struct {
	Path                 string  `json:"path"`
	LocalStatus          string  `json:"local_status"`                     // unchanged, modified, deleted, added
	UpstreamStatus       string  `json:"upstream_status,omitempty"`        // unchanged, modified, deleted, added
	LocalLinesAdded      int     `json:"local_lines_added,omitempty"`
	LocalLinesRemoved    int     `json:"local_lines_removed,omitempty"`
	LocalDriftPct        float64 `json:"local_drift_pct"`
	UpstreamLinesAdded   int     `json:"upstream_lines_added,omitempty"`
	UpstreamLinesRemoved int     `json:"upstream_lines_removed,omitempty"`
	UpstreamDriftPct     float64 `json:"upstream_drift_pct,omitempty"`
	HasConflictRisk      bool    `json:"has_conflict_risk,omitempty"`
	DiffOutput           string  `json:"diff_output,omitempty"` // Populated only with --detail flag
}

// Drift status constants for DriftFile.LocalStatus and DriftFile.UpstreamStatus.
const (
	DriftStatusUnchanged = "unchanged"
	DriftStatusModified  = "modified"
	DriftStatusDeleted   = "deleted"
	DriftStatusAdded     = "added"
)

// Drift result constants for DriftSummary.Result.
const (
	DriftResultClean    = "CLEAN"
	DriftResultDrifted  = "DRIFTED"
	DriftResultConflict = "CONFLICT"
)
