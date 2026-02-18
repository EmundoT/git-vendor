package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// PushOptions configures the push operation behavior.
// PushOptions controls which vendor's local modifications are proposed upstream.
type PushOptions struct {
	VendorName string // Required: which vendor to push changes for
	FilePath   string // Optional: push only a specific file (empty = all modified files)
	DryRun     bool   // Show what would be pushed without actually doing it
}

// PushResult summarizes what a push operation did or would do.
type PushResult struct {
	FilesModified      []string          `json:"files_modified,omitempty"`      // Local paths with modifications relative to lockfile
	ReverseMapping     map[string]string `json:"reverse_mapping,omitempty"`     // local path -> source repo path
	BranchName         string            `json:"branch_name,omitempty"`         // Branch created in source repo
	PRUrl              string            `json:"pr_url,omitempty"`              // URL of created PR (empty if manual fallback)
	ManualInstructions string            `json:"manual_instructions,omitempty"` // Manual git instructions when gh CLI unavailable
	DryRun             bool              `json:"dry_run"`                       // Whether this was a dry-run
}

// PushVendor detects locally modified vendored files, clones the source repo,
// applies the reverse-mapped diffs, and creates a PR (or prints manual instructions).
//
// PushVendor requires the vendor to be an external vendor (not internal).
// Internal vendors use compliance propagation instead.
func (s *VendorSyncer) PushVendor(ctx context.Context, opts PushOptions) (*PushResult, error) {
	if opts.VendorName == "" {
		return nil, fmt.Errorf("vendor name is required")
	}

	// Load config and lock
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Find the vendor spec
	var vendor *types.VendorSpec
	for i := range config.Vendors {
		if config.Vendors[i].Name == opts.VendorName {
			vendor = &config.Vendors[i]
			break
		}
	}
	if vendor == nil {
		return nil, fmt.Errorf("vendor %q not found in config", opts.VendorName)
	}

	// Internal vendors cannot be pushed — they use compliance propagation
	if vendor.Source == SourceInternal {
		return nil, fmt.Errorf("vendor %q is internal; use 'git vendor sync --internal --reverse' for propagation", opts.VendorName)
	}

	// Find the lock entry
	var lockEntry *types.LockDetails
	for i := range lock.Vendors {
		if lock.Vendors[i].Name == opts.VendorName {
			lockEntry = &lock.Vendors[i]
			break
		}
	}
	if lockEntry == nil {
		return nil, fmt.Errorf("vendor %q has no lock entry; run 'git vendor sync' first", opts.VendorName)
	}
	if len(lockEntry.FileHashes) == 0 {
		return nil, fmt.Errorf("vendor %q lock entry has no file hashes; run 'git vendor sync' to populate", opts.VendorName)
	}

	// Build reverse mapping: local "to" path -> source "from" path
	reverseMap := buildReverseMapping(vendor)

	// Detect modified files by comparing local file checksums against lock hashes
	cache := NewFileCacheStore(s.fs, s.rootDir)
	modifiedFiles, err := detectModifiedFiles(cache, lockEntry, reverseMap, opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("detect modified files: %w", err)
	}

	if len(modifiedFiles) == 0 {
		return &PushResult{
			FilesModified: nil,
			DryRun:        opts.DryRun,
		}, nil
	}

	// Build result with reverse mappings for modified files only
	modifiedReverseMap := make(map[string]string, len(modifiedFiles))
	for _, localPath := range modifiedFiles {
		if srcPath, ok := reverseMap[localPath]; ok {
			modifiedReverseMap[localPath] = srcPath
		}
	}

	result := &PushResult{
		FilesModified:  modifiedFiles,
		ReverseMapping: modifiedReverseMap,
		DryRun:         opts.DryRun,
	}

	if opts.DryRun {
		return result, nil
	}

	// Determine downstream project name for branch naming
	projectName := detectProjectName(ctx, s.gitClient)

	// Create temp directory for source repo clone
	tempDir, err := os.MkdirTemp("", "git-vendor-push-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Clone source repo (shallow)
	if err := s.gitClient.Clone(ctx, tempDir, vendor.URL, &types.CloneOptions{Depth: 1}); err != nil {
		return nil, fmt.Errorf("clone source repo %s: %w", SanitizeURL(vendor.URL), err)
	}

	// Create branch: vendor-push/<downstream-project>/<YYYY-MM-DD>
	branchName := fmt.Sprintf("vendor-push/%s/%s", projectName, time.Now().Format("2006-01-02"))
	if err := s.gitClient.CreateBranch(ctx, tempDir, branchName, ""); err != nil {
		return nil, fmt.Errorf("create branch %s: %w", branchName, err)
	}
	if err := s.gitClient.Checkout(ctx, tempDir, branchName); err != nil {
		return nil, fmt.Errorf("checkout branch %s: %w", branchName, err)
	}

	// Copy modified files from local to source repo (reverse-mapped paths)
	var stagedPaths []string
	for _, localPath := range modifiedFiles {
		srcPath, ok := modifiedReverseMap[localPath]
		if !ok {
			continue
		}
		destInClone := filepath.Join(tempDir, filepath.FromSlash(srcPath))

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destInClone), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir for %s: %w", srcPath, err)
		}

		// Copy local file to cloned source repo
		if _, copyErr := s.fs.CopyFile(localPath, destInClone); copyErr != nil {
			return nil, fmt.Errorf("copy %s -> %s: %w", localPath, srcPath, copyErr)
		}
		stagedPaths = append(stagedPaths, srcPath)
	}

	// Stage and commit
	if err := s.gitClient.Add(ctx, tempDir, stagedPaths...); err != nil {
		return nil, fmt.Errorf("git add: %w", err)
	}

	commitMsg := fmt.Sprintf("chore(vendor): apply changes from %s\n\nPushed via git-vendor push from downstream project %q.", projectName, projectName)
	commitOpts := types.CommitOptions{Message: commitMsg}
	if err := s.gitClient.Commit(ctx, tempDir, commitOpts); err != nil {
		return nil, fmt.Errorf("git commit: %w", err)
	}

	// Push branch to origin
	if err := s.gitClient.Push(ctx, tempDir, "origin", branchName); err != nil {
		return nil, fmt.Errorf("git push: %w", err)
	}

	result.BranchName = branchName

	// Try to create PR via gh CLI, fall back to manual instructions
	if isGhInstalled() {
		prURL, ghErr := createPRViaGh(ctx, tempDir, branchName, projectName, modifiedFiles)
		if ghErr != nil {
			// gh is installed but PR creation failed — provide manual instructions as fallback
			result.ManualInstructions = buildManualInstructions(vendor.URL, branchName, projectName, modifiedFiles)
			result.ManualInstructions += fmt.Sprintf("\n\nNote: gh pr create failed: %s", ghErr)
		} else {
			result.PRUrl = prURL
		}
	} else {
		result.ManualInstructions = buildManualInstructions(vendor.URL, branchName, projectName, modifiedFiles)
	}

	return result, nil
}

// buildReverseMapping constructs a map from local destination path to source path
// across all specs in the vendor. For directory mappings, buildReverseMapping stores
// the directory-level mapping — file resolution happens at detection time.
func buildReverseMapping(vendor *types.VendorSpec) map[string]string {
	reverseMap := make(map[string]string)
	for _, spec := range vendor.Specs {
		for _, m := range spec.Mapping {
			// Store the mapping: to -> from
			// For file hashes in the lock, the key is the "to" path
			reverseMap[m.To] = m.From
		}
	}
	return reverseMap
}

// detectModifiedFiles compares local file checksums against lock hashes
// and returns the list of locally modified file paths.
// If filePath is non-empty, only that specific file is checked.
func detectModifiedFiles(
	cache CacheStore,
	lockEntry *types.LockDetails,
	reverseMap map[string]string,
	filePath string,
) ([]string, error) {
	var modified []string

	for localPath, expectedHash := range lockEntry.FileHashes {
		// If a specific file filter is set, skip non-matching paths
		if filePath != "" && localPath != filePath {
			continue
		}

		// Only consider files that have a reverse mapping (are vendored from source)
		if _, hasSrcPath := reverseMap[localPath]; !hasSrcPath {
			continue
		}

		actualHash, err := cache.ComputeFileChecksum(localPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// File was deleted locally — skip (not a modification to push)
				continue
			}
			return nil, fmt.Errorf("checksum %s: %w", localPath, err)
		}

		if actualHash != expectedHash {
			modified = append(modified, localPath)
		}
	}

	return modified, nil
}

// detectProjectName returns the basename of the current git repository
// for use in branch naming and commit messages. Falls back to "downstream"
// if the repository name cannot be determined.
func detectProjectName(ctx context.Context, gitClient GitClient) string {
	url, err := gitClient.ConfigGet(ctx, ".", "remote.origin.url")
	if err != nil || url == "" {
		return "downstream"
	}
	// Extract repo name from URL: git@github.com:user/repo.git -> repo
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name != "" {
			return name
		}
	}
	return "downstream"
}

// isGhInstalled checks whether the GitHub CLI (gh) is available on PATH.
func isGhInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// createPRViaGh creates a pull request using the gh CLI tool.
// createPRViaGh runs "gh pr create" in the cloned source repo directory.
func createPRViaGh(ctx context.Context, repoDir, branchName, projectName string, files []string) (string, error) {
	title := fmt.Sprintf("vendor-push: changes from %s", projectName)
	body := fmt.Sprintf("Automated vendor push from downstream project `%s`.\n\n**Modified files:**\n", projectName)
	for _, f := range files {
		body += fmt.Sprintf("- `%s`\n", f)
	}
	body += "\n_Created by `git vendor push`._"

	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--head", branchName,
	)
	cmd.Dir = repoDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}

	// gh pr create outputs the PR URL on success
	return strings.TrimSpace(string(out)), nil
}

// buildManualInstructions generates step-by-step git instructions
// for creating a PR manually when the gh CLI is not available.
func buildManualInstructions(sourceURL, branchName, projectName string, files []string) string {
	var sb strings.Builder
	sb.WriteString("Manual PR creation instructions:\n\n")
	sb.WriteString(fmt.Sprintf("  Branch '%s' has been pushed to %s\n\n", branchName, SanitizeURL(sourceURL)))
	sb.WriteString("  To create a PR, visit:\n")
	sb.WriteString(fmt.Sprintf("    %s\n\n", buildPRURL(sourceURL, branchName)))
	sb.WriteString(fmt.Sprintf("  Or use: gh pr create --title 'vendor-push: changes from %s' --head %s\n\n", projectName, branchName))
	sb.WriteString("  Modified files:\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("    - %s\n", f))
	}
	return sb.String()
}

// buildPRURL constructs a GitHub/GitLab "new PR" URL from the repo URL and branch name.
// buildPRURL handles HTTPS and SSH URL formats for GitHub. Returns a generic
// hint for non-GitHub repos.
func buildPRURL(repoURL, branchName string) string {
	// Normalize SSH URL: git@github.com:user/repo.git -> https://github.com/user/repo
	normalized := repoURL
	if strings.HasPrefix(normalized, "git@github.com:") {
		normalized = strings.Replace(normalized, "git@github.com:", "https://github.com/", 1)
	}
	normalized = strings.TrimSuffix(normalized, ".git")

	if strings.Contains(normalized, "github.com") {
		return fmt.Sprintf("%s/compare/%s?expand=1", normalized, branchName)
	}
	if strings.Contains(normalized, "gitlab.com") {
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", normalized, branchName)
	}
	return fmt.Sprintf("%s (branch: %s)", normalized, branchName)
}
