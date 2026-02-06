package core

import (
	"fmt"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// DiffVendor compares the locked version with the latest available version
func (s *VendorSyncer) DiffVendor(vendorName string) ([]types.VendorDiff, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile: %w", err)
	}

	// Find vendor in config
	var vendor *types.VendorSpec
	for i := range config.Vendors {
		if config.Vendors[i].Name == vendorName {
			vendor = &config.Vendors[i]
			break
		}
	}

	if vendor == nil {
		return nil, fmt.Errorf("vendor '%s' not found", vendorName)
	}

	var diffs []types.VendorDiff

	for _, spec := range vendor.Specs {
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

			// Initialize repo
			if err := s.gitClient.Init(tempDir); err != nil {
				return fmt.Errorf("failed to init temp repo: %w", err)
			}

			// Add remote
			if err := s.gitClient.AddRemote(tempDir, "origin", vendor.URL); err != nil {
				return fmt.Errorf("failed to add remote: %w", err)
			}

			// Fetch the ref (full fetch to get all commits for diff)
			if err := s.gitClient.FetchAll(tempDir); err != nil {
				// Try fetching just the ref
				if err := s.gitClient.Fetch(tempDir, 0, spec.Ref); err != nil {
					return fmt.Errorf("failed to fetch ref '%s': %w", spec.Ref, err)
				}
			}

			// Get latest commit hash
			if err := s.gitClient.Checkout(tempDir, "FETCH_HEAD"); err != nil {
				return fmt.Errorf("failed to checkout FETCH_HEAD: %w", err)
			}

			latestHash, err := s.gitClient.GetHeadHash(tempDir)
			if err != nil {
				return fmt.Errorf("failed to get HEAD hash: %w", err)
			}

			// Get commit log between locked and latest
			commits, err := s.gitClient.GetCommitLog(tempDir, lockedHash, latestHash, 10) // Limit to 10 commits
			if err != nil {
				// If the locked commit is not in history, it might be ahead or diverged
				commits = []types.CommitInfo{}
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

	return diffs, nil
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
