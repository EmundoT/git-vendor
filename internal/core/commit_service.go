package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	git "github.com/EmundoT/git-plumbing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// VendorNoteRef is the git notes namespace for vendor metadata.
// Notes stored under refs/notes/vendor contain JSON with rich per-vendor
// provenance data that would be too verbose for inline trailers.
const VendorNoteRef = "refs/notes/vendor"

// VendorNoteData is the JSON structure stored in git notes under VendorNoteRef.
// VendorNoteData provides rich per-vendor provenance metadata that supplements
// the compact trailer representation in the commit message.
type VendorNoteData struct {
	Schema  string            `json:"schema"`            // "vendor/v1"
	Vendors []VendorNoteEntry `json:"vendors"`           // One entry per lock entry in this commit
	Created string            `json:"created,omitempty"` // ISO 8601 timestamp
}

// VendorNoteEntry contains per-vendor metadata for a single lock entry in a note.
type VendorNoteEntry struct {
	Name             string            `json:"name"`
	URL              string            `json:"url,omitempty"`
	Ref              string            `json:"ref"`
	CommitHash       string            `json:"commit_hash"`
	LicenseSPDX      string            `json:"license_spdx,omitempty"`
	SourceVersionTag string            `json:"source_version_tag,omitempty"`
	FileHashes       map[string]string `json:"file_hashes,omitempty"`
	Paths            []string          `json:"paths,omitempty"` // Destination paths affected
}

// VendorTrailers builds COMMIT-SCHEMA v1 vendor/v1 trailers for one or more lock entries.
// VendorTrailers returns an ordered []types.Trailer suitable for multi-valued trailer composition.
//
// For N vendors in a single commit, trailer keys repeat N times in order:
//
//	Commit-Schema: vendor/v1   (once)
//	Vendor-Name: lib-a         (Nth vendor)
//	Vendor-Ref: main
//	Vendor-Commit: abc123...
//	Vendor-Name: lib-b         (N+1th vendor)
//	Vendor-Ref: v2
//	Vendor-Commit: def456...
//
// Optional trailers (Vendor-License, Vendor-Source-Tag) are included per-vendor
// only when the corresponding LockDetails field is non-empty.
func VendorTrailers(locks []types.LockDetails) []types.Trailer {
	trailers := []types.Trailer{
		{Key: "Commit-Schema", Value: "vendor/v1"},
		{Key: "Tags", Value: "vendor.update"},
	}

	for _, lock := range locks {
		trailers = append(trailers,
			types.Trailer{Key: "Vendor-Name", Value: lock.Name},
			types.Trailer{Key: "Vendor-Ref", Value: lock.Ref},
			types.Trailer{Key: "Vendor-Commit", Value: lock.CommitHash},
		)

		if lock.LicenseSPDX != "" {
			trailers = append(trailers, types.Trailer{Key: "Vendor-License", Value: lock.LicenseSPDX})
		}

		if lock.SourceVersionTag != "" {
			trailers = append(trailers, types.Trailer{Key: "Vendor-Source-Tag", Value: lock.SourceVersionTag})
		}
	}

	return trailers
}

// VendorCommitSubject formats a conventional-commits subject line for vendor operations.
// VendorCommitSubject produces:
//   - Single vendor: "chore(vendor): <operation> <name> to <ref>"
//   - Multiple vendors: "chore(vendor): <operation> <N> vendors"
func VendorCommitSubject(locks []types.LockDetails, operation string) string {
	if len(locks) == 1 {
		return fmt.Sprintf("chore(vendor): %s %s to %s", operation, locks[0].Name, locks[0].Ref)
	}
	return fmt.Sprintf("chore(vendor): %s %d vendors", operation, len(locks))
}

// VendorNoteJSON builds the JSON note content for a vendor commit.
// VendorNoteJSON includes rich metadata (file hashes, URLs, paths) that would
// be too verbose for inline trailers.
func VendorNoteJSON(locks []types.LockDetails, specMap map[string]*types.VendorSpec) (string, error) {
	note := VendorNoteData{
		Schema:  "vendor/v1",
		Created: time.Now().UTC().Format(time.RFC3339),
	}

	for _, lock := range locks {
		entry := VendorNoteEntry{
			Name:             lock.Name,
			Ref:              lock.Ref,
			CommitHash:       lock.CommitHash,
			LicenseSPDX:      lock.LicenseSPDX,
			SourceVersionTag: lock.SourceVersionTag,
			FileHashes:       lock.FileHashes,
		}

		if spec, ok := specMap[lock.Name]; ok {
			entry.URL = spec.URL
			entry.Paths = collectDestPaths(spec)
		}

		note.Vendors = append(note.Vendors, entry)
	}

	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal vendor note: %w", err)
	}
	return string(data), nil
}

// CommitVendorChanges stages and commits all vendor changes in a single commit
// with multi-valued COMMIT-SCHEMA v1 trailers, then attaches a git note with
// rich per-vendor metadata under refs/notes/vendor.
//
// Flow:
//  1. Load config + lock
//  2. Filter lock entries by vendorFilter (empty = all)
//  3. Collect all affected paths across all matching vendors
//  4. Stage all paths in one git add
//  5. Build multi-valued trailers (one Vendor-Name/Ref/Commit group per vendor)
//  6. Create single commit
//  7. Read back HEAD hash
//  8. Attach JSON note with rich metadata
//
// CommitVendorChanges creates exactly one commit regardless of vendor count.
// For per-vendor provenance, consumers read the structured trailers or note.
func CommitVendorChanges(ctx context.Context, gitClient GitClient, configStore ConfigStore,
	lockStore LockStore, rootDir, operation, vendorFilter string) error {

	config, err := configStore.Load()
	if err != nil {
		return fmt.Errorf("load config for commit: %w", err)
	}

	lock, err := lockStore.Load()
	if err != nil {
		return fmt.Errorf("load lockfile for commit: %w", err)
	}

	// Build vendor name → spec map
	specMap := make(map[string]*types.VendorSpec, len(config.Vendors))
	for i := range config.Vendors {
		specMap[config.Vendors[i].Name] = &config.Vendors[i]
	}

	// Collect matching lock entries and all paths
	var matchedLocks []types.LockDetails
	var allPaths []string

	for _, lockEntry := range lock.Vendors {
		if vendorFilter != "" && lockEntry.Name != vendorFilter {
			continue
		}

		spec, ok := specMap[lockEntry.Name]
		if !ok {
			continue // Orphaned lock entry — skip
		}

		matchedLocks = append(matchedLocks, lockEntry)
		allPaths = append(allPaths, collectVendorPaths(spec, lockEntry, rootDir)...)
	}

	if len(matchedLocks) == 0 {
		return nil // Nothing to commit
	}

	// Deduplicate paths (lockfile/config appear for every vendor)
	allPaths = dedup(allPaths)

	// Stage all files in one operation
	if err := gitClient.Add(ctx, rootDir, allPaths...); err != nil {
		return fmt.Errorf("stage vendor files: %w", err)
	}

	// Build single commit with multi-valued trailers
	subject := VendorCommitSubject(matchedLocks, operation)
	trailers := VendorTrailers(matchedLocks)

	// Compute shared trailers (Touch, Diff-*, Diff-Surface).
	// The programmatic commit path bypasses git hooks, so enrichment
	// that hooks would normally provide must be computed inline.
	// Failures are non-fatal — missing enrichment does not block the commit.
	names, namesErr := gitClient.DiffCachedNames(ctx, rootDir)
	if namesErr == nil && len(names) > 0 {
		absPaths := make([]string, len(names))
		for i, n := range names {
			absPaths[i] = filepath.Join(rootDir, n)
		}
		scanResult, _ := git.TagScan(absPaths)
		tags := git.MergeTags(scanResult)
		if len(tags) > 0 {
			trailers = append(trailers, types.Trailer{Key: "Touch", Value: strings.Join(tags, ", ")})
		}
	}

	metrics, metricsErr := gitClient.DiffCachedStat(ctx, rootDir)
	if metricsErr == nil {
		trailers = append(trailers,
			types.Trailer{Key: "Diff-Additions", Value: strconv.Itoa(metrics.Added)},
			types.Trailer{Key: "Diff-Deletions", Value: strconv.Itoa(metrics.Removed)},
			types.Trailer{Key: "Diff-Files", Value: strconv.Itoa(metrics.FileCount)},
		)
	}

	if namesErr == nil {
		surface := git.ClassifySurface(names, git.DefaultSurfaceRules())
		trailers = append(trailers, types.Trailer{Key: "Diff-Surface", Value: string(surface)})
	}

	if err := gitClient.Commit(ctx, rootDir, types.CommitOptions{
		Message:  subject,
		Trailers: trailers,
	}); err != nil {
		return fmt.Errorf("commit vendor changes: %w", err)
	}

	// Attach note with rich metadata
	headHash, err := gitClient.GetHeadHash(ctx, rootDir)
	if err != nil {
		// Non-fatal: commit succeeded, note attachment is best-effort
		return nil
	}

	noteJSON, err := VendorNoteJSON(matchedLocks, specMap)
	if err != nil {
		return nil // Non-fatal
	}

	// Best-effort note attachment — failure does not fail the commit
	_ = gitClient.AddNote(ctx, rootDir, VendorNoteRef, headHash, noteJSON)

	return nil
}

// AnnotateVendorCommit retroactively attaches vendor metadata to an existing commit.
// AnnotateVendorCommit is used when a human manually commits vendor changes and
// wants to add structured provenance after the fact via "git vendor annotate".
//
// commitHash is the commit to annotate (empty = HEAD).
// vendorFilter restricts to a single vendor (empty = all from lockfile).
func AnnotateVendorCommit(ctx context.Context, gitClient GitClient, configStore ConfigStore,
	lockStore LockStore, rootDir, commitHash, vendorFilter string) error {

	if commitHash == "" {
		var err error
		commitHash, err = gitClient.GetHeadHash(ctx, rootDir)
		if err != nil {
			return fmt.Errorf("resolve HEAD for annotate: %w", err)
		}
	}

	config, err := configStore.Load()
	if err != nil {
		return fmt.Errorf("load config for annotate: %w", err)
	}

	lock, err := lockStore.Load()
	if err != nil {
		return fmt.Errorf("load lockfile for annotate: %w", err)
	}

	specMap := make(map[string]*types.VendorSpec, len(config.Vendors))
	for i := range config.Vendors {
		specMap[config.Vendors[i].Name] = &config.Vendors[i]
	}

	var matchedLocks []types.LockDetails
	for _, lockEntry := range lock.Vendors {
		if vendorFilter != "" && lockEntry.Name != vendorFilter {
			continue
		}
		matchedLocks = append(matchedLocks, lockEntry)
	}

	if len(matchedLocks) == 0 {
		return fmt.Errorf("no matching vendors found for annotate")
	}

	noteJSON, err := VendorNoteJSON(matchedLocks, specMap)
	if err != nil {
		return fmt.Errorf("build vendor note: %w", err)
	}

	if err := gitClient.AddNote(ctx, rootDir, VendorNoteRef, commitHash, noteJSON); err != nil {
		return fmt.Errorf("attach vendor note to %s: %w", commitHash[:minLen(commitHash, 12)], err)
	}

	return nil
}

// collectVendorPaths gathers all file paths that should be staged for a vendor commit.
// collectVendorPaths returns paths for:
//   - Mapping destination paths (from config)
//   - The vendor.lock file
//   - The vendor.yml config file
//   - The vendor's license file (if LicensePath is set)
func collectVendorPaths(spec *types.VendorSpec, lock types.LockDetails, rootDir string) []string {
	var paths []string

	for _, branchSpec := range spec.Specs {
		for _, mapping := range branchSpec.Mapping {
			dest := mapping.To
			if dest == "" {
				dest = filepath.Base(mapping.From)
			}
			// Strip position specifier for staging
			destFile, _, err := types.ParsePathPosition(dest)
			if err != nil {
				destFile = dest
			}
			paths = append(paths, destFile)
		}
	}

	paths = append(paths, LockPath, ConfigPath)

	if lock.LicensePath != "" {
		paths = append(paths, lock.LicensePath)
	}

	return paths
}

// collectDestPaths extracts all mapping destination paths from a VendorSpec.
func collectDestPaths(spec *types.VendorSpec) []string {
	var paths []string
	for _, bs := range spec.Specs {
		for _, m := range bs.Mapping {
			dest := m.To
			if dest == "" {
				dest = filepath.Base(m.From)
			}
			paths = append(paths, dest)
		}
	}
	return paths
}

// dedup removes duplicate strings from a slice, preserving order.
// dedup normalizes forward/back slashes for Windows compatibility.
func dedup(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		normalized := strings.ReplaceAll(item, "\\", "/")
		if _, ok := seen[normalized]; !ok {
			seen[normalized] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// minLen returns the smaller of len(s) and n.
func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
