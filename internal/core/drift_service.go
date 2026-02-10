package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// DriftOptions configures drift detection behavior.
type DriftOptions struct {
	Dependency string // Scope to specific vendor (empty = all)
	Offline    bool   // Skip upstream fetch, report only local drift
	Detail     bool   // Include unified diff output per file
}

// DriftServiceInterface defines the contract for drift detection.
// DriftServiceInterface enables mocking in tests and alternative drift strategies.
// ctx is accepted for cancellation of network operations (git fetch/clone).
type DriftServiceInterface interface {
	Drift(ctx context.Context, opts DriftOptions) (*types.DriftResult, error)
}

// Compile-time interface satisfaction check.
var _ DriftServiceInterface = (*DriftService)(nil)

// DriftService detects drift between vendored files, their locked origin state,
// and (optionally) the latest upstream state.
type DriftService struct {
	configStore ConfigStore
	lockStore   LockStore
	gitClient   GitClient
	fs          FileSystem
	rootDir     string
}

// NewDriftService creates a new DriftService with the given dependencies.
func NewDriftService(
	configStore ConfigStore,
	lockStore LockStore,
	gitClient GitClient,
	fs FileSystem,
	rootDir string,
) *DriftService {
	return &DriftService{
		configStore: configStore,
		lockStore:   lockStore,
		gitClient:   gitClient,
		fs:          fs,
		rootDir:     rootDir,
	}
}

// Drift runs drift detection across all (or a specific) vendored dependency.
// For each dependency, Drift compares local files against the locked commit state
// and (unless offline) against the latest upstream commit.
// ctx controls cancellation of git operations (clone, fetch, checkout).
func (s *DriftService) Drift(ctx context.Context, opts DriftOptions) (*types.DriftResult, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	if len(lock.Vendors) == 0 {
		return nil, fmt.Errorf("no vendors in lockfile — run 'git-vendor update' first")
	}

	// Build lock lookup: "name@ref" → LockDetails
	lockMap := make(map[string]*types.LockDetails)
	for i := range lock.Vendors {
		entry := &lock.Vendors[i]
		lockMap[entry.Name+"@"+entry.Ref] = entry
	}

	result := &types.DriftResult{
		SchemaVersion: "1.0",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Dependencies:  make([]types.DriftDependency, 0),
	}

	for i := range config.Vendors {
		vendor := &config.Vendors[i]

		// Filter to specific dependency if requested
		if opts.Dependency != "" && vendor.Name != opts.Dependency {
			continue
		}

		for _, spec := range vendor.Specs {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			lockEntry, ok := lockMap[vendor.Name+"@"+spec.Ref]
			if !ok {
				continue // Not locked, skip
			}

			dep, err := s.driftForVendorRef(ctx, vendor, &spec, lockEntry, opts)
			if err != nil {
				return nil, fmt.Errorf("drift analysis for %s@%s: %w", vendor.Name, spec.Ref, err)
			}

			result.Dependencies = append(result.Dependencies, *dep)
		}
	}

	// If dependency filter was set but produced no results, report error
	if opts.Dependency != "" && len(result.Dependencies) == 0 {
		return nil, &VendorNotFoundError{Name: opts.Dependency}
	}

	// Compute summary
	result.Summary = computeDriftSummary(result.Dependencies)

	return result, nil
}

// driftForVendorRef analyzes drift for a single vendor at a specific ref.
// ctx controls cancellation of git operations (clone, fetch, checkout).
func (s *DriftService) driftForVendorRef(
	ctx context.Context,
	vendor *types.VendorSpec,
	spec *types.BranchSpec,
	lockEntry *types.LockDetails,
	opts DriftOptions,
) (*types.DriftDependency, error) {

	dep := &types.DriftDependency{
		Name:         vendor.Name,
		URL:          vendor.URL,
		Ref:          spec.Ref,
		LockedCommit: lockEntry.CommitHash,
		Files:        make([]types.DriftFile, 0),
	}

	// Skip whole-file mappings that have position specifiers — those are handled
	// by the verify command's position-level checks instead.
	var wholeMappings []types.PathMapping
	for _, m := range spec.Mapping {
		_, srcPos, parseErr := types.ParsePathPosition(m.From)
		if parseErr == nil && srcPos != nil {
			continue // Position-extracted mapping, skip for drift
		}
		wholeMappings = append(wholeMappings, m)
	}

	if len(wholeMappings) == 0 {
		dep.DriftScore = 0
		return dep, nil
	}

	// Clone and checkout locked commit to read original files
	tempDir, err := s.fs.CreateTemp("", "drift-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = s.fs.RemoveAll(tempDir) }() //nolint:errcheck // cleanup in defer

	if err := s.gitClient.Init(ctx, tempDir); err != nil {
		return nil, fmt.Errorf("init temp repo: %w", err)
	}
	if err := s.gitClient.AddRemote(ctx, tempDir, "origin", vendor.URL); err != nil {
		return nil, fmt.Errorf("add remote: %w", err)
	}

	// Fetch full history (need both locked commit and potentially HEAD)
	if err := s.gitClient.FetchAll(ctx, tempDir); err != nil {
		// Fallback to specific ref fetch
		if err := s.gitClient.Fetch(ctx, tempDir, 0, spec.Ref); err != nil {
			return nil, fmt.Errorf("fetch ref '%s': %w", spec.Ref, err)
		}
	}

	// Phase 1: Read original files at locked commit
	if err := s.gitClient.Checkout(ctx, tempDir, lockEntry.CommitHash); err != nil {
		return nil, fmt.Errorf("checkout locked commit %s: %w", lockEntry.CommitHash[:7], err)
	}

	originals := make(map[string]string) // mapping.From → content
	for _, m := range wholeMappings {
		srcPath := filepath.Join(tempDir, m.From)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			// Source file doesn't exist at locked commit (shouldn't happen for valid lockfile)
			originals[m.From] = ""
			continue
		}
		originals[m.From] = string(data)
	}

	// Phase 2: If not offline, read upstream files at latest commit
	upstreams := make(map[string]string)
	if !opts.Offline {
		if err := s.gitClient.Checkout(ctx, tempDir, "FETCH_HEAD"); err != nil {
			// FETCH_HEAD may not exist if we fetched a specific commit; try the ref
			if err := s.gitClient.Checkout(ctx, tempDir, "origin/"+spec.Ref); err != nil {
				// Cannot determine latest — proceed with local-only drift
				opts.Offline = true
			}
		}

		if !opts.Offline {
			latestHash, err := s.gitClient.GetHeadHash(ctx, tempDir)
			if err == nil {
				dep.LatestCommit = latestHash
			}

			for _, m := range wholeMappings {
				srcPath := filepath.Join(tempDir, m.From)
				data, err := os.ReadFile(srcPath)
				if err != nil {
					// File deleted upstream
					upstreams[m.From] = ""
					continue
				}
				upstreams[m.From] = string(data)
			}
		}
	}

	// Phase 3: Read local files and compute drift
	var totalLocalAdded, totalLocalRemoved int
	var totalUpstreamAdded, totalUpstreamRemoved int
	var localChanged, localUnchanged int
	var upstreamChanged, upstreamUnchanged int
	var totalOrigLines int

	for _, m := range wholeMappings {
		original := originals[m.From]

		// Compute destination path
		destPath := m.To
		if destPath == "" || destPath == "." {
			srcClean := filepath.Clean(m.From)
			destPath = ComputeAutoPath(srcClean, spec.DefaultTarget, vendor.Name)
		}

		df := types.DriftFile{
			Path: destPath,
		}

		// Read local file
		localData, localErr := os.ReadFile(destPath)

		// --- Local drift ---
		switch {
		case localErr != nil:
			// Local file missing (deleted)
			df.LocalStatus = types.DriftStatusDeleted
			df.LocalDriftPct = 100
			df.LocalLinesRemoved = countLines(original)
			localChanged++
		case original == "":
			// Original didn't exist but local does (added locally — unusual)
			df.LocalStatus = types.DriftStatusAdded
			df.LocalDriftPct = 100
			df.LocalLinesAdded = countLines(string(localData))
			localChanged++
		default:
			local := string(localData)
			if local == original {
				df.LocalStatus = types.DriftStatusUnchanged
				df.LocalDriftPct = 0
				localUnchanged++
			} else {
				added, removed := lineDiff(original, local)
				df.LocalStatus = types.DriftStatusModified
				df.LocalLinesAdded = added
				df.LocalLinesRemoved = removed
				df.LocalDriftPct = driftPercentage(added, removed, original, local)
				localChanged++

				if opts.Detail {
					df.DiffOutput = generateSimpleDiff(destPath, original, local, "locked", "local")
				}
			}
		}
		totalLocalAdded += df.LocalLinesAdded
		totalLocalRemoved += df.LocalLinesRemoved

		// --- Upstream drift ---
		if !opts.Offline {
			upstream, hasUpstream := upstreams[m.From]
			switch {
			case !hasUpstream || (upstream == "" && original != ""):
				// File deleted upstream
				df.UpstreamStatus = types.DriftStatusDeleted
				df.UpstreamDriftPct = 100
				df.UpstreamLinesRemoved = countLines(original)
				upstreamChanged++
			case original == "" && upstream != "":
				// File added upstream
				df.UpstreamStatus = types.DriftStatusAdded
				df.UpstreamDriftPct = 100
				df.UpstreamLinesAdded = countLines(upstream)
				upstreamChanged++
			default:
				if upstream == original {
					df.UpstreamStatus = types.DriftStatusUnchanged
					df.UpstreamDriftPct = 0
					upstreamUnchanged++
				} else {
					added, removed := lineDiff(original, upstream)
					df.UpstreamStatus = types.DriftStatusModified
					df.UpstreamLinesAdded = added
					df.UpstreamLinesRemoved = removed
					df.UpstreamDriftPct = driftPercentage(added, removed, original, upstream)
					upstreamChanged++
				}
			}
			totalUpstreamAdded += df.UpstreamLinesAdded
			totalUpstreamRemoved += df.UpstreamLinesRemoved

			// Conflict risk: both local and upstream modified the same file
			if df.LocalStatus != types.DriftStatusUnchanged &&
				df.UpstreamStatus != types.DriftStatusUnchanged {
				df.HasConflictRisk = true
			}
		}

		// Accumulate for overall drift score
		totalOrigLines += countLines(original)

		dep.Files = append(dep.Files, df)
	}

	// Aggregate stats
	dep.LocalDrift = types.DriftStats{
		FilesChanged:      localChanged,
		FilesUnchanged:    localUnchanged,
		TotalLinesAdded:   totalLocalAdded,
		TotalLinesRemoved: totalLocalRemoved,
		DriftPercentage:   aggregateDriftPct(totalLocalAdded, totalLocalRemoved, totalOrigLines),
	}

	if !opts.Offline {
		dep.UpstreamDrift = types.DriftStats{
			FilesChanged:      upstreamChanged,
			FilesUnchanged:    upstreamUnchanged,
			TotalLinesAdded:   totalUpstreamAdded,
			TotalLinesRemoved: totalUpstreamRemoved,
			DriftPercentage:   aggregateDriftPct(totalUpstreamAdded, totalUpstreamRemoved, totalOrigLines),
		}
	}

	// Overall drift score (based on local drift)
	dep.DriftScore = dep.LocalDrift.DriftPercentage
	dep.HasConflictRisk = false
	for _, f := range dep.Files {
		if f.HasConflictRisk {
			dep.HasConflictRisk = true
			break
		}
	}

	return dep, nil
}

// computeDriftSummary aggregates per-dependency stats into a summary.
func computeDriftSummary(deps []types.DriftDependency) types.DriftSummary {
	s := types.DriftSummary{
		TotalDependencies: len(deps),
	}

	var totalScore float64
	for _, dep := range deps {
		totalScore += dep.DriftScore

		hasLocal := dep.LocalDrift.FilesChanged > 0
		hasUpstream := dep.UpstreamDrift.FilesChanged > 0

		switch {
		case hasLocal && hasUpstream:
			s.ConflictRisk++
			s.DriftedLocal++
			s.DriftedUpstream++
		case hasLocal:
			s.DriftedLocal++
		case hasUpstream:
			s.DriftedUpstream++
		default:
			s.Clean++
		}
	}

	if len(deps) > 0 {
		s.OverallDriftScore = totalScore / float64(len(deps))
	}

	switch {
	case s.ConflictRisk > 0:
		s.Result = types.DriftResultConflict
	case s.DriftedLocal > 0 || s.DriftedUpstream > 0:
		s.Result = types.DriftResultDrifted
	default:
		s.Result = types.DriftResultClean
	}

	return s
}

// lineDiff computes the number of added and removed lines between original and current
// content using an LCS-based algorithm. Returns line-level change counts.
func lineDiff(original, current string) (added, removed int) {
	origLines := strings.Split(original, "\n")
	currLines := strings.Split(current, "\n")

	lcsLen := longestCommonSubsequence(origLines, currLines)
	removed = len(origLines) - lcsLen
	added = len(currLines) - lcsLen
	return
}

// longestCommonSubsequence computes the LCS length between two string slices.
// Uses O(n) space with two-row DP approach.
func longestCommonSubsequence(a, b []string) int {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return 0
	}

	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				curr[j] = prev[j-1] + 1
			case prev[j] > curr[j-1]:
				curr[j] = prev[j]
			default:
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, make([]int, n+1)
	}

	return prev[n] // After final swap, prev holds the last computed row
}

// countLines returns the number of lines in content (at least 1 for non-empty content).
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// driftPercentage computes drift as a percentage of total lines.
// Formula: (added + removed) / max(origLines, currLines, 1) * 100, capped at 100.
func driftPercentage(added, removed int, original, current string) float64 {
	origLines := countLines(original)
	currLines := countLines(current)

	base := origLines
	if currLines > base {
		base = currLines
	}
	if base == 0 {
		return 0
	}

	pct := float64(added+removed) / float64(base) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// aggregateDriftPct computes aggregate drift percentage from line totals.
func aggregateDriftPct(totalAdded, totalRemoved, totalOrigLines int) float64 {
	if totalOrigLines == 0 {
		if totalAdded > 0 {
			return 100
		}
		return 0
	}
	pct := float64(totalAdded+totalRemoved) / float64(totalOrigLines) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// generateSimpleDiff generates a simple unified-diff-like output for --detail mode.
func generateSimpleDiff(path, original, current, oldLabel, newLabel string) string {
	origLines := strings.Split(original, "\n")
	currLines := strings.Split(current, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s (%s)\n", path, oldLabel))
	sb.WriteString(fmt.Sprintf("+++ b/%s (%s)\n", path, newLabel))

	// Simple line-by-line comparison (not a true unified diff, but useful for --detail)
	maxLines := len(origLines)
	if len(currLines) > maxLines {
		maxLines = len(currLines)
	}

	for i := 0; i < maxLines; i++ {
		var origLine, currLine string
		hasOrig := i < len(origLines)
		hasCurr := i < len(currLines)

		if hasOrig {
			origLine = origLines[i]
		}
		if hasCurr {
			currLine = currLines[i]
		}

		switch {
		case hasOrig && hasCurr && origLine == currLine:
			sb.WriteString(fmt.Sprintf(" %s\n", origLine))
		case hasOrig && hasCurr:
			sb.WriteString(fmt.Sprintf("-%s\n", origLine))
			sb.WriteString(fmt.Sprintf("+%s\n", currLine))
		case hasOrig:
			sb.WriteString(fmt.Sprintf("-%s\n", origLine))
		case hasCurr:
			sb.WriteString(fmt.Sprintf("+%s\n", currLine))
		}
	}

	return sb.String()
}

// FormatDriftOutput formats a DriftDependency for human-readable display.
func FormatDriftOutput(dep *types.DriftDependency, offline bool) string {
	var sb strings.Builder

	commitInfo := dep.LockedCommit
	if len(commitInfo) > 7 {
		commitInfo = commitInfo[:7]
	}
	sb.WriteString(fmt.Sprintf("  %s @ %s (%s)\n", dep.Name, dep.Ref, commitInfo))

	if len(dep.Files) == 0 {
		sb.WriteString("    No tracked files\n")
		return sb.String()
	}

	// Per-file drift
	for _, f := range dep.Files {
		localSymbol := "\u2713" // checkmark
		localLabel := ""
		switch f.LocalStatus {
		case types.DriftStatusModified:
			localSymbol = "\u0394" // delta
			localLabel = fmt.Sprintf(" (local: +%d/-%d, %.0f%%)", f.LocalLinesAdded, f.LocalLinesRemoved, f.LocalDriftPct)
		case types.DriftStatusDeleted:
			localSymbol = "\u2717" // x mark
			localLabel = " (local: deleted)"
		case types.DriftStatusAdded:
			localSymbol = "+"
			localLabel = " (local: added)"
		}

		upstreamLabel := ""
		if !offline && f.UpstreamStatus != "" {
			switch f.UpstreamStatus {
			case types.DriftStatusModified:
				upstreamLabel = fmt.Sprintf(" (upstream: +%d/-%d, %.0f%%)", f.UpstreamLinesAdded, f.UpstreamLinesRemoved, f.UpstreamDriftPct)
			case types.DriftStatusDeleted:
				upstreamLabel = " (upstream: deleted)"
			case types.DriftStatusAdded:
				upstreamLabel = " (upstream: added)"
			}
		}

		conflictLabel := ""
		if f.HasConflictRisk {
			conflictLabel = " \u26A0 CONFLICT RISK"
		}

		sb.WriteString(fmt.Sprintf("    %s %-50s%s%s%s\n", localSymbol, f.Path, localLabel, upstreamLabel, conflictLabel))

		if f.DiffOutput != "" {
			for _, line := range strings.Split(f.DiffOutput, "\n") {
				sb.WriteString(fmt.Sprintf("      %s\n", line))
			}
		}
	}

	// Summary line
	sb.WriteString(fmt.Sprintf("    Local drift: %.0f%% (%d changed, %d unchanged)\n",
		dep.LocalDrift.DriftPercentage,
		dep.LocalDrift.FilesChanged,
		dep.LocalDrift.FilesUnchanged))

	if !offline && dep.LatestCommit != "" {
		latestShort := dep.LatestCommit
		if len(latestShort) > 7 {
			latestShort = latestShort[:7]
		}
		sb.WriteString(fmt.Sprintf("    Upstream drift: %.0f%% (%d changed, %d unchanged) [latest: %s]\n",
			dep.UpstreamDrift.DriftPercentage,
			dep.UpstreamDrift.FilesChanged,
			dep.UpstreamDrift.FilesUnchanged,
			latestShort))
	}

	if dep.HasConflictRisk {
		sb.WriteString("    \u26A0 Merge conflict risk detected\n")
	}

	sb.WriteString(fmt.Sprintf("    Overall drift score: %.0f%%\n", dep.DriftScore))

	return sb.String()
}
