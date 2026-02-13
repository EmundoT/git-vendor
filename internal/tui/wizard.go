package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/EmundoT/git-vendor/internal/core"
	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/EmundoT/git-vendor/internal/version"
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCard    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("238"))
)

// VendorManager defines the interface for vendor management operations used by the wizard.
type VendorManager interface {
	ParseSmartURL(string) (string, string, string)
	FetchRepoDir(context.Context, string, string, string) ([]string, error)
	ListLocalDir(string) ([]string, error)
	GetLockHash(vendorName, ref string) string
	DetectConflicts() ([]types.PathConflict, error)
}

func check(err error) {
	if err != nil {
		fmt.Println("Aborted.")
		os.Exit(1)
	}
}

// --- ADD WIZARD ---

// RunAddWizard launches the interactive wizard for adding a new vendor dependency.
func RunAddWizard(mgr interface{}, existingVendors map[string]types.VendorSpec) *types.VendorSpec {
	manager := mgr.(VendorManager)

	// Temporary flat struct for wizard input
	var name, url, ref string

	var rawURL string
	err := huh.NewInput().
		Title("Remote URL").
		Placeholder("https://github.com/owner/repo or https://gitlab.com/group/project").
		Description("Paste a full repo URL or a specific file link (GitHub, GitLab, Bitbucket, or any git URL)").
		Value(&rawURL).
		Validate(validateURL).
		Run()
	check(err)

	url, ref, name, smartPath := resolveVendorData(rawURL, manager.ParseSmartURL)

	existing, exists := isExistingVendor(url, existingVendors)
	isAppending := false

	if exists {
		addToExisting := true
		err = huh.NewConfirm().
			Title(buildExistingVendorPrompt(existing.Name)).
			Description("Add to existing vendor?").
			Value(&addToExisting).
			Run()
		check(err)

		if addToExisting {
			return RunEditVendorWizard(mgr, &existing)
		}
	}

	if !isAppending {
		err = huh.NewInput().Title("Vendor Name").Value(&name).Run()
		check(err)

		err = huh.NewInput().Title("Git Ref (Branch/Tag)").Value(&ref).Run()
		check(err)
	}

	spec := newBaseSpec(name, url, ref)

	// Handle deep link path if present
	if isRootSmartPath(smartPath) {
		useDeep := true
		err = huh.NewConfirm().Title("Track specific path?").Description(smartPath).Value(&useDeep).Run()
		check(err)
		if useDeep {
			var dest string
			description := deepLinkDescription(smartPath)
			_ = huh.NewInput().Title("Local Target").Description(description).Value(&dest).Run()
			addMappingToFirstSpec(&spec, smartPath, dest)
		}
	}

	// Enter Edit Loop immediately for the new vendor
	return RunEditVendorWizard(mgr, &spec)
}

// --- EDIT WIZARD (The Core Loop) ---

// RunEditVendorWizard launches the interactive wizard for editing an existing vendor.
func RunEditVendorWizard(mgr interface{}, vendor *types.VendorSpec) *types.VendorSpec {
	manager := mgr.(VendorManager)

	for {
		// 1. Select Ref (Branch) to Edit
		optLabels, optValues := buildBranchOptionData(vendor.Specs, vendor.Name, manager.GetLockHash)
		var branchOpts []huh.Option[string]
		for i := range optLabels {
			branchOpts = append(branchOpts, huh.NewOption(optLabels[i], optValues[i]))
		}

		var selection string
		fmt.Println(styleCard.Render(buildEditVendorTitle(vendor.Name)))

		err := huh.NewSelect[string]().
			Title("Select Branch to Manage").
			Description("Use arrow keys to navigate, Enter to select, Ctrl+C to cancel").
			Options(branchOpts...).
			Value(&selection).
			Height(10).
			Run()
		check(err)

		result := processEditWizardAction(selection, vendor, manager)
		if result.ShouldExit {
			return result.ReturnVendor
		}
		idx := result.ManageIdx
		if result.NeedNewBranch {
			var newRef string
			_ = huh.NewInput().Title("New Branch/Tag Name").Value(&newRef).Run()
			vendor.Specs, _ = appendNewBranch(vendor.Specs, newRef)
			idx = len(vendor.Specs) - 1
		}
		updatedBranch := runMappingManager(manager, vendor.URL, vendor.Specs[idx])
		vendor.Specs[idx] = updatedBranch
	}
}

// runMappingManager presents a menu loop for viewing, adding, editing, and deleting
// path mappings within a single BranchSpec. Returns the updated BranchSpec on exit.
func runMappingManager(mgr VendorManager, url string, branch types.BranchSpec) types.BranchSpec {
	for {
		optLabels, optValues := buildMappingOptionData(branch.Mapping)
		var opts []huh.Option[string]
		for i := range optLabels {
			opts = append(opts, huh.NewOption(optLabels[i], optValues[i]))
		}

		var selection string
		fmt.Println(styleDim.Render(buildMappingManagerTitle(branch.Ref)))
		_ = huh.NewSelect[string]().
			Title("Paths").
			Description("Use arrow keys to navigate, Enter to select").
			Options(opts...).
			Value(&selection).
			Height(10).
			Run()

		action, idx := classifyMappingSelection(selection)

		switch action {
		case "back":
			return branch
		case "add":
			newMap := runMappingCreator(mgr, url, branch.Ref)
			if newMap != nil {
				branch.Mapping = append(branch.Mapping, *newMap)
			}
			continue
		}

		// Edit/Delete selected mapping
		var subAction string
		_ = huh.NewSelect[string]().
			Title(buildMappingActionTitle(branch.Mapping[idx].From)).
			Options(
				huh.NewOption("Edit Paths", "edit"),
				huh.NewOption("Delete", "delete"),
				huh.NewOption("← Back", "back"),
			).Value(&subAction).Run()

		switch subAction {
		case "delete":
			var confirmDelete bool
			_ = huh.NewConfirm().
				Title(buildDeleteMappingTitle(branch.Mapping[idx].From)).
				Description("This will remove the path mapping.").
				Value(&confirmDelete).
				Run()
			if confirmDelete {
				branch.Mapping = deleteMapping(branch.Mapping, idx)
			}
		case "edit":
			_ = huh.NewInput().
				Title("Remote Path").
				Description("Append :L5-L20 for line range or :L5C10:L10C30 for column-precise extraction").
				Value(&branch.Mapping[idx].From).
				Validate(validateFromPath).
				Run()

			description := autoTargetDescription(branch.Mapping[idx].From)
			_ = huh.NewInput().
				Title("Local Target").
				Description(description).
				Value(&branch.Mapping[idx].To).
				Validate(validateToPath).
				Run()
		}
	}
}

// runMappingCreator prompts the user to create a new PathMapping by selecting a remote
// path (via browser or manual entry) and a local target. Supports position specifiers
// like :L5-L20. Returns nil if the user cancels.
func runMappingCreator(mgr VendorManager, url, ref string) *types.PathMapping {
	var m types.PathMapping

	// Remote Path
	var mode string
	_ = huh.NewSelect[string]().
		Title("Remote Path").
		Description("Browse: interactively select files/dirs | Manual: type path (e.g. src/components)").
		Options(
			huh.NewOption("Browse Remote Files", "browse"),
			huh.NewOption("Enter Manually", "manual"),
		).Value(&mode).Run()

	if mode == "browse" {
		m.From = runRemoteBrowser(mgr, url, ref)
		if m.From == "" {
			return nil
		}
	} else {
		_ = huh.NewInput().
			Title("Remote Path").
			Description("Append :L5-L20 for line range or :L5C10:L10C30 for column-precise extraction").
			Value(&m.From).
			Validate(validateFromPath).
			Run()
	}

	// Local Target
	_ = huh.NewSelect[string]().
		Title("Local Target").
		Options(
			huh.NewOption("Browse Local Files", "browse"),
			huh.NewOption("Enter Manually", "manual"),
		).Value(&mode).Run()

	if mode == "browse" {
		m.To = runLocalBrowser(mgr)
		if m.To == "" {
			return nil
		} // User cancelled
	} else {
		description := autoTargetDescription(m.From)
		_ = huh.NewInput().
			Title("Local Target").
			Description(description).
			Value(&m.To).
			Validate(validateToPath).
			Run()
	}

	return &m
}

// runRemoteBrowser presents an interactive directory browser for the remote repository.
// runRemoteBrowser uses VendorManager.FetchRepoDir to list contents via git ls-tree.
// Returns the selected file/directory path, or empty string if cancelled.
func runRemoteBrowser(mgr VendorManager, url, ref string) string {
	currentDir := ""
	for {
		optLabels, optValues, breadcrumb, fetchErr := prepareRemoteBrowserOptions(context.Background(), mgr, url, ref, currentDir)
		if fetchErr != nil {
			PrintError("Error", fetchErr.Error())
			return ""
		}

		var opts []huh.Option[string]
		for i := range optLabels {
			opts = append(opts, huh.NewOption(optLabels[i], optValues[i]))
		}

		var selection string
		_ = huh.NewSelect[string]().
			Title(breadcrumb).
			Description("Navigate: ↑↓ | Select file/folder: Enter | Cancel: Ctrl+C").
			Options(opts...).
			Value(&selection).
			Height(15).
			Run()

		result, newDir, done := processRemoteBrowserSelection(selection, currentDir)
		if done {
			return result
		}
		currentDir = newDir
	}
}

// runLocalBrowser presents an interactive directory browser for the local filesystem.
// runLocalBrowser uses VendorManager.ListLocalDir to list directory contents.
// Returns the selected file/directory path, or empty string if cancelled.
func runLocalBrowser(mgr VendorManager) string {
	currentDir := "."
	for {
		optLabels, optValues, localTitle, listErr := prepareLocalBrowserOptions(mgr, currentDir)
		if listErr != nil {
			PrintError("Error", listErr.Error())
			return ""
		}

		var opts []huh.Option[string]
		for i := range optLabels {
			opts = append(opts, huh.NewOption(optLabels[i], optValues[i]))
		}

		var selection string
		_ = huh.NewSelect[string]().
			Title(localTitle).
			Description("Navigate: ↑↓ | Select file/folder: Enter | Cancel: Ctrl+C").
			Options(opts...).
			Value(&selection).
			Height(15).
			Run()

		result, newDir, done := processLocalBrowserSelection(selection, currentDir)
		if done {
			return result
		}
		currentDir = newDir
	}
}

// RunEditWizardName prompts the user to select a vendor to edit from the list.
func RunEditWizardName(vendorNames []string) string {
	var selected string
	_ = huh.NewSelect[string]().Title("Select Vendor to Edit").Options(huh.NewOptions(vendorNames...)...).Value(&selected).Run()
	return selected
}

// validateFromPath validates a remote path, accepting optional position specifiers.
func validateFromPath(s string) error {
	if s == "" {
		return fmt.Errorf("path cannot be empty")
	}
	_, _, err := types.ParsePathPosition(s)
	return err
}

// validateToPath validates a local target path, accepting optional position specifiers.
// Empty is allowed (triggers auto-naming).
func validateToPath(s string) error {
	if s == "" {
		return nil // Empty triggers auto-naming
	}
	_, _, err := types.ParsePathPosition(s)
	return err
}

// truncate shortens a string to maxLen characters, adding "..." suffix if truncated.
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// isValidGitURL checks if the string looks like a valid git repository URL
func isValidGitURL(s string) bool {
	s = strings.TrimSpace(s)
	// Accept http://, https://, git://, or git@ prefixes
	validPrefixes := []string{"http://", "https://", "git://", "git@", "ssh://"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	// Also accept URLs without protocol (will be normalized by provider)
	// Must contain at least one slash and a dot (domain.com/path)
	return strings.Contains(s, "/") && strings.Contains(s, ".")
}

// PrintError displays an error message with styling to the terminal.
func PrintError(title, msg string) { fmt.Println(styleErr.Render("✖ " + title)); fmt.Println(msg) }

// PrintSuccess displays a success message with styling to the terminal.
func PrintSuccess(msg string) { fmt.Println(styleSuccess.Render("✔ " + msg)) }

// PrintInfo displays an informational message to the terminal.
func PrintInfo(msg string) {
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(msg))
}

// PrintWarning displays a warning message with styling to the terminal.
func PrintWarning(title, msg string) { fmt.Println(styleWarn.Render("! " + title)); fmt.Println(msg) }

// StyleTitle applies title styling to the given text string.
func StyleTitle(text string) string { return styleTitle.Render(text) }

// PrintComplianceSuccess displays a license compliance success message.
func PrintComplianceSuccess(license string) {
	fmt.Println(styleSuccess.Render(fmt.Sprintf("✔ License Verified: %s", license)))
}

// AskToOverrideCompliance prompts the user to override license compliance check.
func AskToOverrideCompliance(license string) bool {
	var confirm bool
	_ = huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(buildAcceptLicenseTitle(license)).Value(&confirm))).Run()
	return confirm
}

// PrintHelp displays usage information for git-vendor commands.
func PrintHelp() {
	fmt.Println(styleTitle.Render(fmt.Sprintf("git-vendor %s", version.GetVersion())))
	fmt.Println("Vendor specific files/directories from Git repositories with deterministic locking")
	fmt.Println("\nWorks as: git-vendor <command> or git vendor <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  init                Initialize vendor directory")
	fmt.Println("  add                 Add a new vendor dependency (interactive wizard)")
	fmt.Println("  edit                Modify existing vendor configuration")
	fmt.Println("  remove <name>       Remove a vendor by name")
	fmt.Println("  list                Show all configured vendors with dependency tree")
	fmt.Println("  sync [options] [vendor-name]")
	fmt.Println("                      Download dependencies to locked versions")
	fmt.Println("                      Supports position extraction (e.g., file.go:L5-L20)")
	fmt.Println("    --dry-run         Preview what will be synced without making changes")
	fmt.Println("    --force           Re-download even if already synced")
	fmt.Println("    --no-cache        Disable incremental sync cache")
	fmt.Println("    --group <name>    Sync only vendors in the specified group")
	fmt.Println("    --parallel        Enable parallel processing (3-5x faster)")
	fmt.Println("    --workers <N>     Number of parallel workers (default: NumCPU)")
	fmt.Println("    --verbose, -v     Show git commands as they run")
	fmt.Println("    <vendor-name>     Sync only the specified vendor")
	fmt.Println("  update [options]    Fetch latest commits and update lockfile")
	fmt.Println("    --parallel        Enable parallel processing (3-5x faster)")
	fmt.Println("    --workers <N>     Number of parallel workers (default: NumCPU)")
	fmt.Println("    --verbose, -v     Show git commands as they run")
	fmt.Println("  validate            Check configuration integrity and detect conflicts")
	fmt.Println("  verify [options]    Verify vendored files against lockfile hashes")
	fmt.Println("                      Checks both whole-file and position-level (L5-L20) hashes")
	fmt.Println("    --format=<fmt>    Output format: table (default) or json")
	fmt.Println("    Exit codes: 0=PASS, 1=FAIL (modified/deleted), 2=WARN (added)")
	fmt.Println("  scan [options]      Scan vendored dependencies for CVE vulnerabilities")
	fmt.Println("    --format=<fmt>    Output format: table (default) or json")
	fmt.Println("    --fail-on <sev>   Fail if vulnerabilities at this severity or above")
	fmt.Println("                      Levels: critical, high, medium, low")
	fmt.Println("    Exit codes: 0=PASS, 1=FAIL (vulns found), 2=WARN (scan incomplete)")
	fmt.Println("    LIMITATIONS:")
	fmt.Println("      • Only scans packages tracked by OSV.dev vulnerability database")
	fmt.Println("      • Private/internal repos cannot have CVE data")
	fmt.Println("      • Results cached for 24 hours (configurable via GIT_VENDOR_CACHE_TTL)")
	fmt.Println("  license [options]   Check license compliance against policy")
	fmt.Println("    --policy <file>   Path to policy file (default: .git-vendor-policy.yml)")
	fmt.Println("    --format=<fmt>    Output format: table (default) or json")
	fmt.Println("    --fail-on <level> Fail threshold: deny (default) or warn")
	fmt.Println("    Exit codes: 0=PASS, 1=FAIL (denied), 2=WARN (warned)")
	fmt.Println("  status              Check if local files are in sync with lockfile")
	fmt.Println("  check-updates       Check for available updates to vendors")
	fmt.Println("  diff <vendor>       Show commit differences between locked and latest")
	fmt.Println("  watch               Watch for config changes and auto-sync")
	fmt.Println("  completion <shell>  Generate shell completion script (bash/zsh/fish/powershell)")
	fmt.Println("\nLLM-Friendly Commands (non-interactive):")
	fmt.Println("  create <name> <url> [--ref <ref>] [--license <license>]")
	fmt.Println("                      Add vendor without interactive wizard")
	fmt.Println("  delete <name>       Remove vendor (alias for remove, same flags)")
	fmt.Println("  rename <old> <new>  Rename a vendor across config, lock, and license")
	fmt.Println("  add-mapping <vendor> <from> --to <to> [--ref <ref>]")
	fmt.Println("                      Add a path mapping to a vendor")
	fmt.Println("  remove-mapping <vendor> <from>")
	fmt.Println("                      Remove a path mapping from a vendor")
	fmt.Println("  list-mappings <vendor>")
	fmt.Println("                      List all path mappings for a vendor")
	fmt.Println("  update-mapping <vendor> <from> --to <new-to>")
	fmt.Println("                      Update a mapping's destination path")
	fmt.Println("  show <vendor>       Show detailed vendor information")
	fmt.Println("  check <vendor>      Check sync status for a single vendor")
	fmt.Println("  preview <vendor>    Preview what files would be synced")
	fmt.Println("  config list         List all configuration key-value pairs")
	fmt.Println("  config get <key>    Get a config value (e.g., vendors.mylib.url)")
	fmt.Println("  config set <key> <value>")
	fmt.Println("                      Set a config value")
	fmt.Println("  All LLM commands support --json for structured JSON output.")
	fmt.Println("\nExamples:")
	fmt.Println("  git-vendor init")
	fmt.Println("  git-vendor add")
	fmt.Println("  git-vendor sync --dry-run")
	fmt.Println("  git-vendor sync --verbose")
	fmt.Println("  git-vendor sync my-vendor")
	fmt.Println("  git-vendor sync --force")
	fmt.Println("  git-vendor sync --parallel --workers 4")
	fmt.Println("  git-vendor update -v")
	fmt.Println("  git-vendor update --parallel")
	fmt.Println("  git-vendor list")
	fmt.Println("  git-vendor list --json")
	fmt.Println("  git-vendor validate")
	fmt.Println("  git-vendor verify")
	fmt.Println("  git-vendor verify --format=json")
	fmt.Println("  git-vendor scan")
	fmt.Println("  git-vendor scan --format=json")
	fmt.Println("  git-vendor scan --fail-on high")
	fmt.Println("  git-vendor license")
	fmt.Println("  git-vendor license --format=json")
	fmt.Println("  git-vendor license --fail-on warn")
	fmt.Println("  git-vendor license --policy custom-policy.yml")
	fmt.Println("  git-vendor status")
	fmt.Println("  git-vendor check-updates")
	fmt.Println("  git-vendor diff my-vendor")
	fmt.Println("  git-vendor watch")
	fmt.Println("  git-vendor completion bash > /etc/bash_completion.d/git-vendor")
	fmt.Println("  git-vendor remove my-vendor")
	fmt.Println("  git-vendor create api-types https://github.com/org/api --ref v2.0.0 --license MIT")
	fmt.Println("  git-vendor add-mapping api-types src/types/user.ts --to lib/types/user.ts")
	fmt.Println("  git-vendor show api-types --json")
	fmt.Println("  git-vendor config get vendors.api-types.url")
	fmt.Println("\nNavigation:")
	fmt.Println("  Use arrow keys to navigate, Enter to select")
	fmt.Println("  Press Ctrl+C to cancel at any time")
}

// InitSummary holds data for the post-init output display.
type InitSummary struct {
	VendorDir string // Relative path to the vendor directory (e.g. ".git-vendor")
	OriginURL string // Detected origin remote URL, empty if unavailable
}

// PrintInitSummary displays a rich post-init summary with detected origin
// and actionable next steps. Omits origin line when OriginURL is empty
// (not a git repo or no origin remote configured).
func PrintInitSummary(summary InitSummary) {
	PrintSuccess("Initialized in ./" + summary.VendorDir + "/")
	fmt.Println()

	if summary.OriginURL != "" {
		PrintInfo("  Detected origin: " + summary.OriginURL)
		fmt.Println()
	}

	PrintInfo("Tip: Create .git-vendor-policy.yml for license compliance enforcement")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  git-vendor add       # Add your first dependency (interactive)")
	fmt.Println("  git-vendor sync      # Download files at locked versions")
	fmt.Println("  git-vendor validate  # Check configuration integrity")
}

// ShowConflictWarnings displays any path conflicts involving the given vendor
func ShowConflictWarnings(mgr VendorManager, vendorName string) {
	conflicts, err := mgr.DetectConflicts()
	if err != nil {
		return // Silently skip if detection fails
	}

	vendorConflicts := filterConflictsForVendor(conflicts, vendorName)

	if len(vendorConflicts) > 0 {
		fmt.Println()
		PrintWarning("Path Conflicts Detected", fmt.Sprintf("Found %s with this vendor", core.Pluralize(len(vendorConflicts), "conflict", "conflicts")))
		for i := range vendorConflicts {
			fmt.Println(formatConflictDetail(vendorConflicts[i].Path, otherVendorInConflict(&vendorConflicts[i], vendorName)))
		}
		fmt.Println()
		fmt.Println("  Run 'git-vendor validate' for full details")
		fmt.Println()
	}
}
