package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// DiffOptions configures filtering for DiffVendorWithOptions.
// Empty fields mean "no filter" (match all).
type DiffOptions struct {
	VendorName string // Filter to a single vendor by name
	Ref        string // Filter to a specific ref within matched vendors
	Group      string // Filter vendors by group membership
}

// DiffVendor compares the locked version with the latest available version
// for a single vendor. DiffVendor is a backward-compatible wrapper around
// DiffVendorWithOptions that filters by vendor name only.
func (s *VendorSyncer) DiffVendor(vendorName string) ([]types.VendorDiff, error) {
	return s.DiffVendorWithOptions(DiffOptions{VendorName: vendorName})
}

// DiffVendorWithOptions compares locked versions with the latest available versions,
// applying optional filters. When all filter fields are empty, DiffVendorWithOptions
// diffs every vendor. When VendorName is set, only that vendor is diffed. When Group
// is set, only vendors in that group are diffed. When Ref is set, only specs matching
// that ref are diffed within matched vendors.
func (s *VendorSyncer) DiffVendorWithOptions(opts DiffOptions) ([]types.VendorDiff, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Build list of vendors to diff based on filters
	var vendors []types.VendorSpec
	for i := range config.Vendors {
		v := &config.Vendors[i]
		if opts.VendorName != "" && v.Name != opts.VendorName {
			continue
		}
		if opts.Group != "" && !containsGroup(v.Groups, opts.Group) {
			continue
		}
		vendors = append(vendors, *v)
	}

	// If a specific vendor was requested but not found, return an error
	if opts.VendorName != "" && len(vendors) == 0 {
		return nil, fmt.Errorf("vendor '%s' not found", opts.VendorName)
	}
	// If a group was requested but no vendors matched, return an error
	if opts.Group != "" && len(vendors) == 0 {
		return nil, fmt.Errorf("no vendors found in group '%s'", opts.Group)
	}

	var diffs []types.VendorDiff

	for vi := range vendors {
		vendor := &vendors[vi]
		for _, spec := range vendor.Specs {
			// Filter by ref if specified
			if opts.Ref != "" && spec.Ref != opts.Ref {
				continue
			}

			// Find locked commit
			var lockedHash string
			var lockedDate string
			for i := range lock.Vendors {
				entry := &lock.Vendors[i]
				if entry.Name == vendor.Name && entry.Ref == spec.Ref {
					lockedHash = entry.CommitHash
					lockedDate = entry.Updated
					break
				}
			}

			if lockedHash == "" {
				// Not locked yet, can't diff
				continue
			}

			// Process each spec in a function to ensure cleanup happens after each iteration
			err := func() error {
				// Fetch latest commit on the ref
				tempDir, err := s.fs.CreateTemp("", "diff-check-*")
				if err != nil {
					return fmt.Errorf("failed to create temp dir: %w", err)
				}
				defer func() { _ = s.fs.RemoveAll(tempDir) }() //nolint:errcheck // cleanup in defer

				ctx := context.Background()

				// Initialize repo
				if err := s.gitClient.Init(ctx, tempDir); err != nil {
					return fmt.Errorf("failed to init temp repo: %w", err)
				}

				// Add remote
				if err := s.gitClient.AddRemote(ctx, tempDir, "origin", vendor.URL); err != nil {
					return fmt.Errorf("failed to add remote: %w", err)
				}

				// Shallow fetch the target ref first (depth 20 covers the 10-commit
				// log limit plus margin). Only deepen if the locked commit is not
				// reachable in the shallow history.
				const shallowDepth = 20
				if err := s.gitClient.Fetch(ctx, tempDir, "origin", shallowDepth, spec.Ref); err != nil {
					return fmt.Errorf("failed to fetch ref '%s': %w", spec.Ref, err)
				}

				// Get latest commit hash
				if err := s.gitClient.Checkout(ctx, tempDir, "FETCH_HEAD"); err != nil {
					return fmt.Errorf("failed to checkout FETCH_HEAD: %w", err)
				}

				latestHash, err := s.gitClient.GetHeadHash(ctx, tempDir)
				if err != nil {
					return fmt.Errorf("failed to get HEAD hash: %w", err)
				}

				// Get commit log between locked and latest
				commits, err := s.gitClient.GetCommitLog(ctx, tempDir, lockedHash, latestHash, 10)
				if err != nil {
					// Locked commit not in shallow history — deepen and retry
					if deepErr := s.gitClient.Fetch(ctx, tempDir, "origin", 0, spec.Ref); deepErr != nil {
						// Full fetch also failed; return empty log (diverged or force-pushed)
						commits = []types.CommitInfo{}
					} else {
						commits, _ = s.gitClient.GetCommitLog(ctx, tempDir, lockedHash, latestHash, 10)
					}
				}

				// Get timestamp for latest commit if different
				latestDate := ""
				if latestHash != lockedHash && len(commits) > 0 {
					latestDate = commits[0].Date
				}

				diffs = append(diffs, types.VendorDiff{
					VendorName:  vendor.Name,
					Ref:         spec.Ref,
					OldHash:     lockedHash,
					NewHash:     latestHash,
					OldDate:     lockedDate,
					NewDate:     latestDate,
					Commits:     commits,
					CommitCount: len(commits),
				})

				return nil
			}()
			if err != nil {
				return nil, err
			}
		}
	}

	return diffs, nil
}

// containsGroup checks if a group name is present in a groups slice.
func containsGroup(groups []string, target string) bool {
	for _, g := range groups {
		if g == target {
			return true
		}
	}
	return false
}

// FormatDiffOutput formats a VendorDiff for display
func FormatDiffOutput(diff *types.VendorDiff) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📦 %s @ %s\n", diff.VendorName, diff.Ref))

	if diff.OldHash == diff.NewHash {
		sb.WriteString(fmt.Sprintf("   ✓ Up to date (%s)\n", diff.OldHash[:7]))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("   Old: %s", diff.OldHash[:7]))
	if diff.OldDate != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", formatDate(diff.OldDate)))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("   New: %s", diff.NewHash[:7]))
	if diff.NewDate != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", formatDate(diff.NewDate)))
	}
	sb.WriteString("\n")

	if diff.CommitCount > 0 {
		sb.WriteString(fmt.Sprintf("\n   Commits (+%d):\n", diff.CommitCount))
		for _, commit := range diff.Commits {
			date := formatDate(commit.Date)
			sb.WriteString(fmt.Sprintf("   • %s - %s (%s)\n", commit.ShortHash, commit.Subject, date))
		}
	} else if diff.OldHash != diff.NewHash {
		sb.WriteString("\n   ⚠ Commits diverged or ahead\n")
	}

	return sb.String()
}

// formatDate extracts date from ISO8601 timestamp
func formatDate(isoDate string) string {
	// Input format: "2024-12-20 15:30:45 -0800"
	// Output format: "Dec 20"
	parts := strings.Fields(isoDate)
	if len(parts) < 1 {
		return isoDate
	}

	datePart := parts[0] // "2024-12-20"
	dateFields := strings.Split(datePart, "-")
	if len(dateFields) != 3 {
		return isoDate
	}

	month := dateFields[1]
	day := dateFields[2]

	monthNames := map[string]string{
		"01": "Jan", "02": "Feb", "03": "Mar", "04": "Apr",
		"05": "May", "06": "Jun", "07": "Jul", "08": "Aug",
		"09": "Sep", "10": "Oct", "11": "Nov", "12": "Dec",
	}

	monthName, ok := monthNames[month]
	if !ok {
		return isoDate
	}

	return fmt.Sprintf("%s %s", monthName, day)
}
