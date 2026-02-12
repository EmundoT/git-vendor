package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/EmundoT/git-vendor/internal/core"
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

// resolveVendorData resolves initial vendor data from a parsed smart URL.
// resolveVendorData returns the cleaned URL, resolved ref, inferred name, and smart path.
func resolveVendorData(rawURL string, parser func(string) (string, string, string)) (url, ref, name, smartPath string) {
	baseURL, smartRef, sp := parser(rawURL)
	url = baseURL
	ref = resolveRef(smartRef, core.DefaultRef)
	name = inferVendorName(url)
	smartPath = sp
	return
}

// classifyEditSelection interprets the user's selection in the edit vendor wizard.
// classifyEditSelection returns an action string ("cancel", "save", "new", or "manage")
// and a branch index (valid only for "manage").
func classifyEditSelection(selection string) (string, int) {
	switch selection {
	case "cancel":
		return "cancel", -1
	case "save":
		return "save", -1
	case "new":
		return "new", -1
	default:
		var idx int
		fmt.Sscanf(selection, "%d", &idx)
		return "manage", idx
	}
}

// appendNewBranch appends a new BranchSpec with the given ref and returns the updated
// slice along with the selection string for the new branch index.
func appendNewBranch(specs []types.BranchSpec, ref string) ([]types.BranchSpec, string) {
	specs = append(specs, types.BranchSpec{Ref: ref})
	return specs, fmt.Sprintf("%d", len(specs)-1)
}

// classifyMappingSelection interprets the user's selection in the mapping manager.
// classifyMappingSelection returns an action string ("back", "add", or "manage")
// and a mapping index (valid only for "manage").
func classifyMappingSelection(selection string) (string, int) {
	switch selection {
	case "back":
		return "back", -1
	case "add":
		return "add", -1
	default:
		var idx int
		fmt.Sscanf(selection, "%d", &idx)
		return "manage", idx
	}
}

// processRemoteBrowserSelection handles the user's selection in the remote file browser.
// processRemoteBrowserSelection returns (result, newCurrentDir, done).
// When done is true, result is the final selected path (empty for cancel).
// When done is false, newCurrentDir is the updated directory for the next iteration.
func processRemoteBrowserSelection(selection, currentDir string) (string, string, bool) {
	if selection == "CANCEL" || selection == "" {
		return "", "", true
	}
	if selection == "SELECT_CURRENT" {
		return currentDir, "", true
	}
	if selection == ".." {
		return "", navigateUp(currentDir), false
	}
	newDir, file, isFile := resolveRemoteSelection(selection, currentDir)
	if isFile {
		return file, "", true
	}
	return "", newDir, false
}

// processLocalBrowserSelection handles the user's selection in the local file browser.
// processLocalBrowserSelection returns (result, newCurrentDir, done).
// When done is true, result is the final selected path (empty for cancel).
// When done is false, newCurrentDir is the updated directory for the next iteration.
func processLocalBrowserSelection(selection, currentDir string) (string, string, bool) {
	if selection == "CANCEL" || selection == "" {
		return "", "", true
	}
	if selection == "SELECT_CURRENT" {
		return currentDir, "", true
	}
	if selection == ".." {
		return "", navigateLocalUp(currentDir), false
	}
	newDir, file, isFile := resolveLocalSelection(selection, currentDir)
	if isFile {
		return file, "", true
	}
	return "", newDir, false
}

// buildRemoteBrowserOptionData builds option labels and values for the remote file browser.
// buildRemoteBrowserOptionData includes navigation options (.., select current, cancel)
// alongside the file/directory items.
func buildRemoteBrowserOptionData(currentDir string, items []string) (labels, values []string) {
	if currentDir != "" {
		labels = append(labels, ".. (Go Up)")
		values = append(values, "..")
	}
	labels = append(labels, selectCurrentLabel(currentDir))
	values = append(values, "SELECT_CURRENT")
	for _, item := range items {
		labels = append(labels, itemLabel(item))
		values = append(values, item)
	}
	labels = append(labels, "❌ Cancel")
	values = append(values, "CANCEL")
	return
}

// buildLocalBrowserOptionData builds option labels and values for the local file browser.
// buildLocalBrowserOptionData includes navigation options (.., select current, cancel)
// alongside the file/directory items.
func buildLocalBrowserOptionData(currentDir string, items []string) (labels, values []string) {
	if hasLocalParent(currentDir) {
		labels = append(labels, ".. (Go Up)")
		values = append(values, "..")
	}
	labels = append(labels, selectCurrentLabel(currentDir))
	values = append(values, "SELECT_CURRENT")
	for _, item := range items {
		labels = append(labels, itemLabel(item))
		values = append(values, item)
	}
	labels = append(labels, "❌ Cancel")
	values = append(values, "CANCEL")
	return
}

// buildMappingOptionData builds option labels and values for the mapping manager menu.
// buildMappingOptionData includes each mapping's formatted label plus "add" and "back" actions.
func buildMappingOptionData(mappings []types.PathMapping) (labels, values []string) {
	for i, m := range mappings {
		labels = append(labels, formatMappingLabel(m.From, m.To))
		values = append(values, fmt.Sprintf("%d", i))
	}
	labels = append(labels, "+ Add Path")
	values = append(values, "add")
	labels = append(labels, "← Back")
	values = append(values, "back")
	return
}

// buildBranchOptionData builds option labels and values for the edit vendor branch menu.
// buildBranchOptionData includes each branch's formatted label plus "new", "save", and "cancel" actions.
func buildBranchOptionData(specs []types.BranchSpec, vendorName string, getLockHash func(string, string) string) (labels, values []string) {
	for i, s := range specs {
		hash := getLockHash(vendorName, s.Ref)
		label := formatBranchLabel(s.Ref, len(s.Mapping), hash)
		labels = append(labels, label)
		values = append(values, fmt.Sprintf("%d", i))
	}
	labels = append(labels, "+ Add New Branch")
	values = append(values, "new")
	labels = append(labels, "💾 Save & Exit")
	values = append(values, "save")
	labels = append(labels, "❌ Cancel")
	values = append(values, "cancel")
	return
}

// classifyMappingAction interprets a sub-action from the edit/delete menu for a mapping.
// classifyMappingAction returns "delete", "edit", or "back".
func classifyMappingAction(action string) string {
	switch action {
	case "delete", "edit", "back":
		return action
	default:
		return "back"
	}
}

// buildCreatorModeResult resolves the remote path for a mapping creator given the mode.
// buildCreatorModeResult returns the resolved From path, and whether the operation was cancelled.
// For "browse" mode, the browsedPath is used; for "manual" mode, the manualPath is used.
func buildCreatorModeResult(mode, browsedPath, manualPath string) (from string, cancelled bool) {
	if mode == "browse" {
		if browsedPath == "" {
			return "", true
		}
		return browsedPath, false
	}
	return manualPath, false
}

// buildCreatorLocalResult resolves the local target for a mapping creator given the mode.
// buildCreatorLocalResult returns the resolved To path, and whether the operation was cancelled.
func buildCreatorLocalResult(mode, browsedPath, manualPath string) (to string, cancelled bool) {
	if mode == "browse" {
		if browsedPath == "" {
			return "", true
		}
		return browsedPath, false
	}
	return manualPath, false
}

// buildEditVendorTitle builds the styled title for the edit vendor card display.
func buildEditVendorTitle(name string) string {
	return fmt.Sprintf("Editing Vendor: %s", name)
}

// buildMappingManagerTitle builds the title for the mapping manager section display.
func buildMappingManagerTitle(ref string) string {
	return fmt.Sprintf("Managing paths for %s", ref)
}

// buildExistingVendorPrompt builds the confirmation title for tracking an already-tracked repo.
func buildExistingVendorPrompt(name string) string {
	return fmt.Sprintf("Repo '%s' is already tracked.", name)
}

// buildDeleteMappingTitle builds the confirmation title for deleting a path mapping.
func buildDeleteMappingTitle(from string) string {
	return fmt.Sprintf("Delete mapping for '%s'?", from)
}

// buildMappingActionTitle builds the title for the edit/delete action menu.
func buildMappingActionTitle(from string) string {
	return fmt.Sprintf("Path: %s", from)
}

// buildAcceptLicenseTitle builds the title for the license override prompt.
func buildAcceptLicenseTitle(license string) string {
	return fmt.Sprintf("Accept %s License?", license)
}

// prepareRemoteBrowserOptions fetches remote directory contents and builds
// option labels/values for the remote file browser display.
// prepareRemoteBrowserOptions returns an error if the remote fetch fails.
func prepareRemoteBrowserOptions(mgr VendorManager, url, ref, currentDir string) (labels, values []string, breadcrumb string, err error) {
	items, fetchErr := mgr.FetchRepoDir(url, ref, currentDir)
	if fetchErr != nil {
		return nil, nil, "", fetchErr
	}
	labels, values = buildRemoteBrowserOptionData(currentDir, items)
	breadcrumb = buildBreadcrumb(repoNameFromURL(url), ref, currentDir)
	return labels, values, breadcrumb, nil
}

// prepareLocalBrowserOptions lists local directory contents and builds
// option labels/values for the local file browser display.
// prepareLocalBrowserOptions returns an error if listing the directory fails.
func prepareLocalBrowserOptions(mgr VendorManager, currentDir string) (labels, values []string, title string, err error) {
	items, listErr := mgr.ListLocalDir(currentDir)
	if listErr != nil {
		return nil, nil, "", listErr
	}
	labels, values = buildLocalBrowserOptionData(currentDir, items)
	title = selectLocalLabel(currentDir)
	return labels, values, title, nil
}

// editActionResult describes the result of processing an edit wizard selection.
type editActionResult struct {
	ShouldExit    bool
	ReturnVendor  *types.VendorSpec
	NeedNewBranch bool
	ManageIdx     int
}

// processEditWizardAction processes a selection from the edit vendor wizard
// and returns the corresponding action result.
// processEditWizardAction handles cancel/save/new/manage dispatch.
func processEditWizardAction(selection string, vendor *types.VendorSpec, mgr VendorManager) editActionResult {
	action, idx := classifyEditSelection(selection)
	switch action {
	case "cancel":
		return editActionResult{ShouldExit: true}
	case "save":
		ShowConflictWarnings(mgr, vendor.Name)
		return editActionResult{ShouldExit: true, ReturnVendor: vendor}
	case "new":
		return editActionResult{NeedNewBranch: true}
	case "manage":
		return editActionResult{ManageIdx: idx}
	}
	return editActionResult{ShouldExit: true}
}

// validateVendorInput validates the essential fields of a vendor spec entry.
// validateVendorInput checks that URL, name, and ref are present and well-formed.
func validateVendorInput(url, name, ref string) error {
	if url == "" {
		return fmt.Errorf("URL is required")
	}
	url = strings.TrimSpace(url)
	if !isValidGitURL(url) {
		return fmt.Errorf("invalid git URL: %s", url)
	}
	if name == "" {
		return fmt.Errorf("vendor name is required")
	}
	if strings.ContainsAny(name, " \t\n/\\") {
		return fmt.Errorf("vendor name contains invalid characters: %s", name)
	}
	if ref == "" {
		return fmt.Errorf("git ref is required")
	}
	return nil
}

// validateMappingPair validates a single from/to path mapping pair.
// validateMappingPair allows empty To path (triggers auto-naming).
func validateMappingPair(from, to string) error {
	if err := validateFromPath(from); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if to != "" {
		if err := validateToPath(to); err != nil {
			return fmt.Errorf("invalid target path: %w", err)
		}
	}
	return nil
}

// summarizeMappings builds a human-readable summary of path mappings.
// summarizeMappings returns "(no mappings)" for empty slices.
func summarizeMappings(mappings []types.PathMapping) string {
	if len(mappings) == 0 {
		return "  (no mappings)"
	}
	var lines []string
	for _, m := range mappings {
		lines = append(lines, "  "+formatMappingLabel(m.From, m.To))
	}
	return strings.Join(lines, "\n")
}

// formatVendorSummary builds a complete text summary of a vendor spec
// including all branches and their mappings.
func formatVendorSummary(spec types.VendorSpec, getLockHash func(string, string) string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Name: %s", spec.Name))
	lines = append(lines, fmt.Sprintf("URL:  %s", spec.URL))
	for _, s := range spec.Specs {
		hash := ""
		if getLockHash != nil {
			hash = getLockHash(spec.Name, s.Ref)
		}
		label := formatBranchLabel(s.Ref, len(s.Mapping), hash)
		lines = append(lines, fmt.Sprintf("  %s", label))
		lines = append(lines, summarizeMappings(s.Mapping))
	}
	return strings.Join(lines, "\n")
}

// detectDuplicateMappings scans all branch specs for duplicate source paths.
// detectDuplicateMappings returns a description for each duplicate found.
func detectDuplicateMappings(specs []types.BranchSpec) []string {
	seen := make(map[string]string)
	var dupes []string
	for _, s := range specs {
		for _, m := range s.Mapping {
			if prevRef, ok := seen[m.From]; ok {
				dupes = append(dupes, fmt.Sprintf("%s (in %s and %s)", m.From, prevRef, s.Ref))
			} else {
				seen[m.From] = s.Ref
			}
		}
	}
	return dupes
}

// countMappings counts the total number of path mappings across all branch specs.
func countMappings(specs []types.BranchSpec) int {
	total := 0
	for _, s := range specs {
		total += len(s.Mapping)
	}
	return total
}

// findMappingByFrom searches all branch specs for a mapping with the given source path.
// findMappingByFrom returns the branch index, mapping index, and whether a match was found.
func findMappingByFrom(specs []types.BranchSpec, from string) (branchIdx, mappingIdx int, found bool) {
	for bi, s := range specs {
		for mi, m := range s.Mapping {
			if m.From == from {
				return bi, mi, true
			}
		}
	}
	return -1, -1, false
}

// isEmptySpec checks whether a VendorSpec has any configured path mappings.
func isEmptySpec(spec types.VendorSpec) bool {
	if len(spec.Specs) == 0 {
		return true
	}
	for _, s := range spec.Specs {
		if len(s.Mapping) > 0 {
			return false
		}
	}
	return true
}

// buildVendorSpecWithDefaults builds a VendorSpec from a raw URL, using the parser
// to extract components, and applying name/ref overrides if non-empty.
func buildVendorSpecWithDefaults(rawURL string, parser func(string) (string, string, string), nameOverride, refOverride string) types.VendorSpec {
	url, ref, name, _ := resolveVendorData(rawURL, parser)
	if nameOverride != "" {
		name = nameOverride
	}
	if refOverride != "" {
		ref = refOverride
	}
	return newBaseSpec(name, url, ref)
}

// specHasMapping checks whether any branch spec in the vendor has a mapping with
// the given source path.
func specHasMapping(spec types.VendorSpec, from string) bool {
	for _, s := range spec.Specs {
		for _, m := range s.Mapping {
			if m.From == from {
				return true
			}
		}
	}
	return false
}

// formatVendorListEntry builds a single-line summary of a vendor for list display,
// showing name, URL, branch count, and total mapping count.
func formatVendorListEntry(spec types.VendorSpec) string {
	branches := len(spec.Specs)
	mappings := countMappings(spec.Specs)
	branchWord := "branch"
	if branches != 1 {
		branchWord = "branches"
	}
	return fmt.Sprintf("%s (%s) — %d %s, %s", spec.Name, spec.URL, branches, branchWord, formatPathCount(mappings))
}

// validateAllMappings validates every path mapping across all branch specs.
// validateAllMappings returns the first validation error encountered, annotated
// with the branch ref.
func validateAllMappings(specs []types.BranchSpec) error {
	for _, s := range specs {
		for _, m := range s.Mapping {
			if err := validateMappingPair(m.From, m.To); err != nil {
				return fmt.Errorf("branch %s: %w", s.Ref, err)
			}
		}
	}
	return nil
}

// hasMappings checks whether a VendorSpec has at least one path mapping configured.
func hasMappings(spec types.VendorSpec) bool {
	return countMappings(spec.Specs) > 0
}

// validateVendorSpec validates that a VendorSpec has all required fields:
// non-empty name, valid URL, and at least one branch spec with a non-empty ref.
func validateVendorSpec(spec types.VendorSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("vendor name is required")
	}
	if spec.URL == "" {
		return fmt.Errorf("vendor URL is required")
	}
	if !isValidGitURL(spec.URL) {
		return fmt.Errorf("invalid vendor URL: %s", spec.URL)
	}
	if len(spec.Specs) == 0 {
		return fmt.Errorf("at least one branch spec is required")
	}
	for i, s := range spec.Specs {
		if s.Ref == "" {
			return fmt.Errorf("branch spec %d has empty ref", i)
		}
	}
	return nil
}

// truncateHash returns a truncated commit hash suitable for display.
// truncateHash returns the full hash if it is shorter than the requested length.
func truncateHash(hash string, length int) string {
	if len(hash) <= length {
		return hash
	}
	return hash[:length]
}

// collectMappingFromPaths returns all source (From) paths across all branch specs.
func collectMappingFromPaths(specs []types.BranchSpec) []string {
	var paths []string
	for _, s := range specs {
		for _, m := range s.Mapping {
			paths = append(paths, m.From)
		}
	}
	return paths
}
