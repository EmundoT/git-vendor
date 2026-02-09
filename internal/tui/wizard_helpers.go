package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// validateURL validates a git repository URL for the add wizard.
// validateURL rejects empty strings and strings that fail isValidGitURL.
func validateURL(s string) error {
	if s == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	s = strings.TrimSpace(s)
	if !isValidGitURL(s) {
		return fmt.Errorf("invalid git URL format")
	}
	return nil
}

// formatBranchLabel builds a display label for a BranchSpec in the edit wizard.
// formatBranchLabel combines ref name, mapping count, and lock status into a single line.
func formatBranchLabel(ref string, mappingCount int, lockHash string) string {
	status := "not synced"
	if lockHash != "" {
		status = fmt.Sprintf("locked: %s", lockHash[:7])
	}

	pathCount := formatPathCount(mappingCount)
	return fmt.Sprintf("%s (%s, %s)", ref, pathCount, status)
}

// formatPathCount returns a human-readable string for the number of path mappings.
func formatPathCount(count int) string {
	if count == 0 {
		return "no paths"
	}
	if count == 1 {
		return "1 path"
	}
	return fmt.Sprintf("%d paths", count)
}

// formatMappingLabel builds a display label for a PathMapping in the mapping manager.
// formatMappingLabel shows truncated source path and destination (or "(auto)" if empty).
func formatMappingLabel(from, to string) string {
	dest := to
	if dest == "" {
		dest = "(auto)"
	}
	return fmt.Sprintf("%-20s → %s", truncate(from, 20), dest)
}

// buildBreadcrumb constructs a breadcrumb trail for the remote file browser.
// buildBreadcrumb combines repo name, ref, and current directory into a navigable path.
func buildBreadcrumb(repoName, ref, currentDir string) string {
	breadcrumb := repoName + " @ " + ref
	if currentDir != "" {
		breadcrumb += " / " + strings.ReplaceAll(currentDir, "/", " / ")
	}
	return breadcrumb
}

// navigateUp returns the parent directory of currentDir, or empty string for root.
func navigateUp(currentDir string) string {
	parent := path.Dir(strings.TrimSuffix(currentDir, "/"))
	if parent == "." {
		return ""
	}
	return parent
}

// resolveRemoteSelection resolves a user selection in the remote browser.
// resolveRemoteSelection returns (newCurrentDir, selectedFile, isFile).
// For directories (trailing "/"), isFile is false and newCurrentDir is updated.
// For files, isFile is true and selectedFile contains the full path.
func resolveRemoteSelection(selection, currentDir string) (string, string, bool) {
	if strings.HasSuffix(selection, "/") {
		dir := strings.TrimSuffix(selection, "/")
		if currentDir == "" {
			return dir, "", false
		}
		return currentDir + "/" + dir, "", false
	}
	full := selection
	if currentDir != "" {
		full = currentDir + "/" + selection
	}
	return "", full, true
}

// autoNameFromPath derives an automatic display name from a source path.
// autoNameFromPath strips any position specifier before extracting the base name.
func autoNameFromPath(fromPath string) string {
	fromFile, _, _ := types.ParsePathPosition(fromPath)
	name := path.Base(fromFile)
	if name == "" || name == "." || name == "/" {
		return "(repository root)"
	}
	return name
}

// itemLabel builds a display label for a file or directory entry.
// itemLabel prefixes directories (trailing "/") with a folder icon and files with a document icon.
func itemLabel(item string) string {
	if strings.HasSuffix(item, "/") {
		return "📂 " + item
	}
	return "📄 " + item
}

// repoNameFromURL extracts a repository display name from a git URL.
// repoNameFromURL strips trailing ".git" suffix from the URL base name.
func repoNameFromURL(url string) string {
	name := path.Base(url)
	return strings.TrimSuffix(name, ".git")
}

// resolveLocalSelection resolves a user selection in the local file browser.
// resolveLocalSelection returns (newCurrentDir, selectedPath, isFile).
// For directories (trailing "/"), isFile is false and currentDir is joined.
// For files, isFile is true and selectedPath is the joined result.
func resolveLocalSelection(selection, currentDir string) (string, string, bool) {
	if strings.HasSuffix(selection, "/") {
		return path.Join(currentDir, selection), "", false
	}
	return "", path.Join(currentDir, selection), true
}

// deleteMapping removes a mapping at the given index from a slice.
// deleteMapping returns the modified slice.
func deleteMapping(mappings []types.PathMapping, idx int) []types.PathMapping {
	return append(mappings[:idx], mappings[idx+1:]...)
}

// inferVendorName derives a default vendor name from a repository URL.
// inferVendorName strips the ".git" suffix from the base of the URL.
func inferVendorName(url string) string {
	baseName := path.Base(url)
	return strings.TrimSuffix(baseName, ".git")
}

// resolveRef selects the effective git ref, preferring a smart-detected ref over the default.
func resolveRef(smartRef, defaultRef string) string {
	if smartRef != "" {
		return smartRef
	}
	return defaultRef
}

// selectCurrentLabel builds the "Select current directory" option label for browsers.
func selectCurrentLabel(currentDir string) string {
	if currentDir != "" && currentDir != "." {
		return fmt.Sprintf("✔ Select '/%s'", currentDir)
	}
	return "✔ Select Root"
}

// autoTargetDescription builds the description string for a local target input field.
func autoTargetDescription(fromPath string) string {
	autoName := autoNameFromPath(fromPath)
	return fmt.Sprintf("Leave empty for automatic naming (will use: %s). Supports :L5-L10 position syntax", autoName)
}

// newBaseSpec creates a VendorSpec with a single BranchSpec and empty mappings.
func newBaseSpec(name, url, ref string) types.VendorSpec {
	return types.VendorSpec{
		Name: name,
		URL:  url,
		Specs: []types.BranchSpec{
			{Ref: ref, Mapping: []types.PathMapping{}},
		},
	}
}

// deepLinkDescription builds description text for the deep-link local target prompt.
// deepLinkDescription differs from autoTargetDescription by omitting position syntax hints.
func deepLinkDescription(smartPath string) string {
	autoName := autoNameFromPath(smartPath)
	return fmt.Sprintf("Leave empty for automatic naming (will use: %s)", autoName)
}

// addMappingToFirstSpec appends a PathMapping to the first BranchSpec of a VendorSpec.
func addMappingToFirstSpec(spec *types.VendorSpec, from, to string) {
	spec.Specs[0].Mapping = append(spec.Specs[0].Mapping, types.PathMapping{From: from, To: to})
}

// isExistingVendor checks if a URL is already tracked and returns the existing spec.
func isExistingVendor(url string, existing map[string]types.VendorSpec) (types.VendorSpec, bool) {
	spec, exists := existing[url]
	return spec, exists
}

// buildMappingOptionsLabels returns display labels for a list of path mappings.
// buildMappingOptionsLabels formats each mapping as "from → to" for menu display.
func buildMappingOptionsLabels(mappings []types.PathMapping) []string {
	labels := make([]string, len(mappings))
	for i, m := range mappings {
		labels[i] = formatMappingLabel(m.From, m.To)
	}
	return labels
}

// buildBranchOptionsLabels returns display labels for branch specs in the edit wizard.
// buildBranchOptionsLabels shows ref name, path count, and lock status for each branch.
func buildBranchOptionsLabels(specs []types.BranchSpec, vendorName string, getLockHash func(string, string) string) []string {
	labels := make([]string, len(specs))
	for i, s := range specs {
		hash := getLockHash(vendorName, s.Ref)
		labels[i] = formatBranchLabel(s.Ref, len(s.Mapping), hash)
	}
	return labels
}

// buildItemLabels creates display labels for a list of file/directory items.
func buildItemLabels(items []string) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = itemLabel(item)
	}
	return labels
}

// hasLocalParent checks whether the current directory has a parent to navigate up to.
// hasLocalParent returns false for the root directory (".").
func hasLocalParent(currentDir string) bool {
	return currentDir != "."
}

// navigateLocalUp returns the parent of the local browser's current directory.
func navigateLocalUp(currentDir string) string {
	return path.Dir(currentDir)
}

// filterConflictsForVendor returns only the PathConflicts that involve the given vendor.
func filterConflictsForVendor(conflicts []types.PathConflict, vendorName string) []types.PathConflict {
	var result []types.PathConflict
	for _, c := range conflicts {
		if c.Vendor1 == vendorName || c.Vendor2 == vendorName {
			result = append(result, c)
		}
	}
	return result
}

// otherVendorInConflict returns the opposing vendor name in a conflict for the given vendor.
func otherVendorInConflict(c *types.PathConflict, vendorName string) string {
	if c.Vendor2 == vendorName {
		return c.Vendor1
	}
	return c.Vendor2
}

// selectLocalLabel builds the local browser title from the current directory.
func selectLocalLabel(currentDir string) string {
	return fmt.Sprintf("Local: %s", currentDir)
}

// isRootSmartPath checks whether a smart-detected path requires deep link handling.
func isRootSmartPath(smartPath string) bool {
	return smartPath != ""
}

// formatConflictDetail builds the display lines for a single path conflict entry.
func formatConflictDetail(conflictPath, otherVendor string) string {
	return fmt.Sprintf("  ⚠ %s\n    Conflicts with vendor: %s", conflictPath, otherVendor)
}

// formatConflictSummary builds the summary line for conflict warnings.
// formatConflictSummary returns an empty string when there are no conflicts.
func formatConflictSummary(count int) string {
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("Found %d conflict(s) with this vendor", count)
}
