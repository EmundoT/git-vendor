package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// Schema version constants
const (
	// CurrentSchemaVersion is the version written to new lockfiles.
	// Bumped to 1.2 for Spec 070 (internal vendor Source/SourceFileHashes fields).
	CurrentSchemaVersion = "1.3"
	// MaxSupportedMajor is the maximum major version this CLI can handle
	MaxSupportedMajor = 1
	// MaxSupportedMinor is the maximum minor version this CLI fully understands
	MaxSupportedMinor = 3
)

// parseSchemaVersion parses a schema version string into major and minor components.
// Empty string defaults to version 1.0 for backward compatibility.
func parseSchemaVersion(version string) (major, minor int, err error) {
	if version == "" {
		return 1, 0, nil // Default to 1.0 for backward compatibility
	}

	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid schema version format: %q (expected major.minor)", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	if major < 0 || minor < 0 {
		return 0, 0, fmt.Errorf("negative version numbers not allowed: %q", version)
	}

	return major, minor, nil
}

// validateSchemaVersion checks if the lockfile version is compatible with this CLI.
// Returns an error if the major version is unsupported.
// Writes a warning to the provided writer if minor version is newer than expected.
func validateSchemaVersion(version string, warnWriter io.Writer) error {
	major, minor, err := parseSchemaVersion(version)
	if err != nil {
		return fmt.Errorf("parse schema version: %w", err)
	}

	if major > MaxSupportedMajor {
		return fmt.Errorf(
			"lockfile schema version %q requires a newer git-vendor version\n"+
				"  Your CLI supports schema v%d.x, but lockfile is v%d.x\n"+
				"  Run 'git vendor version' to check your version, then update git-vendor",
			version, MaxSupportedMajor, major)
	}

	if major == MaxSupportedMajor && minor > MaxSupportedMinor {
		if warnWriter != nil {
			//nolint:errcheck // Warning output - error is non-actionable
			fmt.Fprintf(warnWriter,
				"Warning: Lockfile schema version %q is newer than expected (%d.%d)\n"+
					"  Some metadata fields may be ignored. Consider updating git-vendor.\n",
				version, MaxSupportedMajor, MaxSupportedMinor)
		}
	}

	return nil
}

// LockStore handles vendor.lock I/O operations
type LockStore interface {
	Load() (types.VendorLock, error)
	Save(lock types.VendorLock) error
	Path() string
	GetHash(vendorName, ref string) string
}

// FileLockStore implements LockStore using YAMLStore
type FileLockStore struct {
	store *YAMLStore[types.VendorLock]
}

// NewFileLockStore creates a new FileLockStore
func NewFileLockStore(rootDir string) *FileLockStore {
	return &FileLockStore{
		store: NewYAMLStore[types.VendorLock](rootDir, LockFile, false), // allowMissing=false
	}
}

// Path returns the lock file path
func (s *FileLockStore) Path() string {
	return s.store.Path()
}

// ErrLockConflict indicates git merge conflict markers were detected in vendor.lock.
var ErrLockConflict = errors.New("git merge conflict detected in vendor.lock")

// LockConflictError provides structured details about merge conflicts in vendor.lock.
// LockConflictError wraps ErrLockConflict so callers can use errors.Is for detection.
type LockConflictError struct {
	Conflicts []types.LockConflict
}

func (e *LockConflictError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: Git merge conflict detected in vendor.lock (%d conflict region(s))", len(e.Conflicts)))
	for i, c := range e.Conflicts {
		b.WriteString(fmt.Sprintf("\n  Conflict %d at line %d", i+1, c.LineNumber))
	}
	b.WriteString("\n  Fix: Resolve merge conflicts in vendor.lock, then run 'git-vendor update' or manually edit the file")
	return b.String()
}

func (e *LockConflictError) Unwrap() error {
	return ErrLockConflict
}

// IsLockConflictError returns true if err is a LockConflictError.
func IsLockConflictError(err error) bool {
	var e *LockConflictError
	return errors.As(err, &e)
}

// DetectConflicts scans the raw lock file for git merge conflict markers
// (<<<<<<<, =======, >>>>>>>) and returns a LockConflictError if any are found.
// DetectConflicts is called before YAML parsing to provide a clear error
// instead of a cryptic parse failure.
func (s *FileLockStore) DetectConflicts() error {
	data, err := os.ReadFile(s.store.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // No file = no conflicts
		}
		return err
	}

	return detectConflictsInData(data)
}

// detectConflictsInData scans raw bytes for git merge conflict markers.
// Extracted as a standalone function for testability without filesystem.
func detectConflictsInData(data []byte) error {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var conflicts []types.LockConflict
	lineNum := 0
	inConflict := false
	var oursLines []string
	var theirsLines []string
	conflictStart := 0
	pastSeparator := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimRight(scanner.Text(), "\r")

		switch {
		case strings.HasPrefix(line, "<<<<<<<"):
			inConflict = true
			conflictStart = lineNum
			oursLines = nil
			theirsLines = nil
			pastSeparator = false
		case strings.HasPrefix(line, "=======") && inConflict:
			pastSeparator = true
		case strings.HasPrefix(line, ">>>>>>>") && inConflict:
			conflicts = append(conflicts, types.LockConflict{
				LineNumber: conflictStart,
				OursRaw:    strings.Join(oursLines, "\n"),
				TheirsRaw:  strings.Join(theirsLines, "\n"),
			})
			inConflict = false
		default:
			if inConflict {
				if pastSeparator {
					theirsLines = append(theirsLines, line)
				} else {
					oursLines = append(oursLines, line)
				}
			}
		}
	}

	if len(conflicts) > 0 {
		return &LockConflictError{Conflicts: conflicts}
	}
	return nil
}

// Load reads and parses vendor.lock, validating schema version compatibility.
// Load first checks for git merge conflict markers — returns a LockConflictError
// if found, providing a clear error instead of a cryptic YAML parse failure.
// Returns an error if the major version is unsupported.
// Writes a warning to stderr if minor version is newer than expected.
func (s *FileLockStore) Load() (types.VendorLock, error) {
	// Check for merge conflicts before attempting YAML parse
	if err := s.DetectConflicts(); err != nil {
		return types.VendorLock{}, err
	}

	lock, err := s.store.Load()
	if err != nil {
		return lock, err
	}

	// Validate schema version compatibility
	if err := validateSchemaVersion(lock.SchemaVersion, os.Stderr); err != nil {
		return types.VendorLock{}, err
	}

	return lock, nil
}

// Save writes vendor.lock, always setting the current schema version.
func (s *FileLockStore) Save(lock types.VendorLock) error {
	lock.SchemaVersion = CurrentSchemaVersion
	return s.store.Save(lock)
}

// MergeLockEntries merges two VendorLock structs into one.
// Non-overlapping vendors are combined directly. For overlapping entries
// (same vendor name + ref), the entry with the later Updated timestamp wins.
// If timestamps are equal, the higher CommitHash (lexicographic) wins.
// If neither heuristic resolves the conflict, it is flagged in
// LockMergeResult.Conflicts for manual resolution.
func MergeLockEntries(ours, theirs *types.VendorLock) types.LockMergeResult {
	if ours == nil {
		ours = &types.VendorLock{}
	}
	if theirs == nil {
		theirs = &types.VendorLock{}
	}

	type key struct {
		Name string
		Ref  string
	}

	oursMap := make(map[key]types.LockDetails, len(ours.Vendors))
	for _, e := range ours.Vendors {
		oursMap[key{e.Name, e.Ref}] = e
	}

	theirsMap := make(map[key]types.LockDetails, len(theirs.Vendors))
	for _, e := range theirs.Vendors {
		theirsMap[key{e.Name, e.Ref}] = e
	}

	result := types.LockMergeResult{}
	// Use the newer schema version
	result.Merged.SchemaVersion = ours.SchemaVersion
	if theirs.SchemaVersion > ours.SchemaVersion {
		result.Merged.SchemaVersion = theirs.SchemaVersion
	}

	seen := make(map[key]bool)

	// Process all entries from ours
	for _, e := range ours.Vendors {
		k := key{e.Name, e.Ref}
		if seen[k] {
			continue
		}
		seen[k] = true

		if theirs, ok := theirsMap[k]; ok {
			// Both sides have this entry — pick winner or flag conflict
			winner, conflict := resolveLockEntry(e, theirs)
			if conflict != nil {
				result.Conflicts = append(result.Conflicts, *conflict)
				// Still include ours as the default in merged output
				result.Merged.Vendors = append(result.Merged.Vendors, e)
			} else {
				result.Merged.Vendors = append(result.Merged.Vendors, winner)
			}
		} else {
			result.Merged.Vendors = append(result.Merged.Vendors, e)
		}
	}

	// Add entries only in theirs
	for _, e := range theirs.Vendors {
		k := key{e.Name, e.Ref}
		if seen[k] {
			continue
		}
		seen[k] = true
		result.Merged.Vendors = append(result.Merged.Vendors, e)
	}

	return result
}

// resolveLockEntry picks a winner between two LockDetails for the same vendor+ref.
// Returns the winner and nil conflict if resolution succeeds, or zero-value and
// a LockMergeConflict if manual resolution is required.
func resolveLockEntry(ours, theirs types.LockDetails) (types.LockDetails, *types.LockMergeConflict) {
	// Same commit = no conflict
	if ours.CommitHash == theirs.CommitHash {
		// Pick the one with the later timestamp for metadata freshness
		if theirs.Updated > ours.Updated {
			return theirs, nil
		}
		return ours, nil
	}

	// Different commits: later timestamp wins
	if ours.Updated != theirs.Updated {
		if theirs.Updated > ours.Updated {
			return theirs, nil
		}
		return ours, nil
	}

	// Same timestamp, different commits: lexicographic tiebreaker
	if ours.CommitHash != theirs.CommitHash {
		if theirs.CommitHash > ours.CommitHash {
			return theirs, nil
		}
		if ours.CommitHash > theirs.CommitHash {
			return ours, nil
		}
	}

	// Truly unresolvable (should not reach here with different commit hashes,
	// but guard against edge cases)
	return types.LockDetails{}, &types.LockMergeConflict{
		VendorName: ours.Name,
		Ref:        ours.Ref,
		Ours:       ours,
		Theirs:     theirs,
	}
}

// GetHash retrieves the locked commit hash for a vendor@ref
func (s *FileLockStore) GetHash(vendorName, ref string) string {
	lock, err := s.Load()
	if err != nil {
		return ""
	}

	for i := range lock.Vendors {
		l := &lock.Vendors[i]
		if l.Name == vendorName && l.Ref == ref {
			return l.CommitHash
		}
	}

	return ""
}
