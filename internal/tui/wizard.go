package tui

import (
	"fmt"
	"git-vendor/internal/core"
	"git-vendor/internal/types"
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
	FetchRepoDir(string, string, string) ([]string, error)
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
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("URL cannot be empty")
			}
			s = strings.TrimSpace(s)
			// Allow any git URL - provider registry handles platform detection
			if !isValidGitURL(s) {
				return fmt.Errorf("invalid git URL format")
			}
			return nil
		}).
		Run()
	check(err)

	baseURL, smartRef, smartPath := manager.ParseSmartURL(rawURL)
	url = baseURL
	ref = "main"
	if smartRef != "" {
		ref = smartRef
	}

	baseName := path.Base(url)
	name = strings.TrimSuffix(baseName, ".git")

	existing, exists := existingVendors[url]
	isAppending := false

	if exists {
		addToExisting := true
		err = huh.NewConfirm().
			Title(fmt.Sprintf("Repo '%s' is already tracked.", existing.Name)).
			Description("Add to existing vendor?").
			Value(&addToExisting).
			Run()
		check(err)

		if addToExisting {
			return RunEditVendorWizard(mgr, existing)
		}
	}

	if !isAppending {
		err = huh.NewInput().Title("Vendor Name").Value(&name).Run()
		check(err)

		err = huh.NewInput().Title("Git Ref (Branch/Tag)").Value(&ref).Run()
		check(err)
	}

	// Create base spec
	spec := types.VendorSpec{
		Name: name,
		URL:  url,
		Specs: []types.BranchSpec{
			{Ref: ref, Mapping: []types.PathMapping{}},
		},
	}

	// Handle deep link path if present
	if smartPath != "" {
		useDeep := true
		err = huh.NewConfirm().Title("Track specific path?").Description(smartPath).Value(&useDeep).Run()
		check(err)
		if useDeep {
			var dest string
			autoName := path.Base(smartPath)
			if autoName == "" || autoName == "." || autoName == "/" {
				autoName = "(repository root)"
			}
			description := fmt.Sprintf("Leave empty for automatic naming (will use: %s)", autoName)
			_ = huh.NewInput().Title("Local Target").Description(description).Value(&dest).Run()
			spec.Specs[0].Mapping = append(spec.Specs[0].Mapping, types.PathMapping{From: smartPath, To: dest})
		}
	}

	// Enter Edit Loop immediately for the new vendor
	return RunEditVendorWizard(mgr, spec)
}

// --- EDIT WIZARD (The Core Loop) ---

// RunEditVendorWizard launches the interactive wizard for editing an existing vendor.
func RunEditVendorWizard(mgr interface{}, vendor types.VendorSpec) *types.VendorSpec {
	manager := mgr.(VendorManager)

	for {
		// 1. Select Ref (Branch) to Edit
		var branchOpts []huh.Option[string]
		for i, s := range vendor.Specs {
			// Get lock status
			status := "not synced"
			if hash := manager.GetLockHash(vendor.Name, s.Ref); hash != "" {
				status = fmt.Sprintf("locked: %s", hash[:7])
			}

			// Show number of paths instead of "mappings"
			pathCount := "no paths"
			if len(s.Mapping) == 1 {
				pathCount = "1 path"
			} else if len(s.Mapping) > 1 {
				pathCount = fmt.Sprintf("%d paths", len(s.Mapping))
			}

			label := fmt.Sprintf("%s (%s, %s)", s.Ref, pathCount, status)
			branchOpts = append(branchOpts, huh.NewOption(label, fmt.Sprintf("%d", i)))
		}
		branchOpts = append(branchOpts, huh.NewOption("+ Add New Branch", "new"))
		branchOpts = append(branchOpts, huh.NewOption("💾 Save & Exit", "save"))
		branchOpts = append(branchOpts, huh.NewOption("❌ Cancel", "cancel"))

		var selection string
		fmt.Println(styleCard.Render(fmt.Sprintf("Editing Vendor: %s", vendor.Name)))

		err := huh.NewSelect[string]().
			Title("Select Branch to Manage").
			Description("Use arrow keys to navigate, Enter to select, Ctrl+C to cancel").
			Options(branchOpts...).
			Value(&selection).
			Height(10).
			Run()
		check(err)

		if selection == "cancel" {
			return nil
		}
		if selection == "save" {
			// Show conflict warnings before saving
			ShowConflictWarnings(manager, vendor.Name)
			return &vendor
		}

		if selection == "new" {
			var newRef string
			_ = huh.NewInput().Title("New Branch/Tag Name").Value(&newRef).Run()
			vendor.Specs = append(vendor.Specs, types.BranchSpec{Ref: newRef})
			selection = fmt.Sprintf("%d", len(vendor.Specs)-1)
		}

		// 2. Manage Mappings for Selected Branch
		var idx int
		_, _ = fmt.Sscanf(selection, "%d", &idx)

		updatedBranch := runMappingManager(manager, vendor.URL, vendor.Specs[idx])
		vendor.Specs[idx] = updatedBranch
	}
}

func runMappingManager(mgr VendorManager, url string, branch types.BranchSpec) types.BranchSpec {
	for {
		var opts []huh.Option[string]
		for i, m := range branch.Mapping {
			dest := m.To
			if dest == "" {
				dest = "(auto)"
			}
			label := fmt.Sprintf("%-20s → %s", truncate(m.From, 20), dest)
			opts = append(opts, huh.NewOption(label, fmt.Sprintf("%d", i)))
		}
		opts = append(opts, huh.NewOption("+ Add Path", "add"))
		opts = append(opts, huh.NewOption("← Back", "back"))

		var selection string
		fmt.Println(styleDim.Render(fmt.Sprintf("Managing paths for %s", branch.Ref)))
		_ = huh.NewSelect[string]().
			Title("Paths").
			Description("Use arrow keys to navigate, Enter to select").
			Options(opts...).
			Value(&selection).
			Height(10).
			Run()

		if selection == "back" {
			return branch
		}

		if selection == "add" {
			newMap := runMappingCreator(mgr, url, branch.Ref)
			if newMap != nil {
				branch.Mapping = append(branch.Mapping, *newMap)
			}
			continue
		}

		// Edit/Delete
		var idx int
		_, _ = fmt.Sscanf(selection, "%d", &idx)

		var action string
		_ = huh.NewSelect[string]().
			Title(fmt.Sprintf("Path: %s", branch.Mapping[idx].From)).
			Options(
				huh.NewOption("Edit Paths", "edit"),
				huh.NewOption("Delete", "delete"),
				huh.NewOption("← Back", "back"),
			).Value(&action).Run()

		switch action {
		case "delete":
			var confirmDelete bool
			_ = huh.NewConfirm().
				Title(fmt.Sprintf("Delete mapping for '%s'?", branch.Mapping[idx].From)).
				Description("This will remove the path mapping.").
				Value(&confirmDelete).
				Run()
			if confirmDelete {
				branch.Mapping = append(branch.Mapping[:idx], branch.Mapping[idx+1:]...)
			}
		case "edit":
			// Reuse creator for editing
			// Ideally pre-fill, but for now simple edit inputs
			_ = huh.NewInput().Title("Remote Path").Value(&branch.Mapping[idx].From).Run()

			// Show auto-naming preview
			autoName := path.Base(branch.Mapping[idx].From)
			if autoName == "" || autoName == "." || autoName == "/" {
				autoName = "(repository root)"
			}
			description := fmt.Sprintf("Leave empty for automatic naming (will use: %s)", autoName)
			_ = huh.NewInput().Title("Local Target").Description(description).Value(&branch.Mapping[idx].To).Run()
		}
	}
}

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
		_ = huh.NewInput().Title("Remote Path").Value(&m.From).Run()
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
		// Show preview of auto-generated name
		autoName := path.Base(m.From)
		if autoName == "" || autoName == "." || autoName == "/" {
			autoName = "(repository root)"
		}
		description := fmt.Sprintf("Leave empty for automatic naming (will use: %s)", autoName)
		_ = huh.NewInput().Title("Local Target").Description(description).Value(&m.To).Run()
	}

	return &m
}

func runRemoteBrowser(mgr VendorManager, url, ref string) string {
	// Extract repo name from URL for breadcrumb
	repoName := path.Base(url)
	repoName = strings.TrimSuffix(repoName, ".git")

	currentDir := ""
	for {
		items, err := mgr.FetchRepoDir(url, ref, currentDir)
		if err != nil {
			PrintError("Error", err.Error())
			return ""
		}

		var opts []huh.Option[string]
		if currentDir != "" {
			opts = append(opts, huh.NewOption(".. (Go Up)", ".."))
			opts = append(opts, huh.NewOption(fmt.Sprintf("✔ Select '/%s'", currentDir), "SELECT_CURRENT"))
		} else {
			opts = append(opts, huh.NewOption("✔ Select Root", "SELECT_CURRENT"))
		}

		for _, item := range items {
			var label string
			if strings.HasSuffix(item, "/") {
				label = "📂 " + item
			} else {
				label = "📄 " + item
			}
			opts = append(opts, huh.NewOption(label, item))
		}
		opts = append(opts, huh.NewOption("❌ Cancel", "CANCEL"))

		// Build breadcrumb trail
		breadcrumb := repoName + " @ " + ref
		if currentDir != "" {
			breadcrumb += " / " + strings.ReplaceAll(currentDir, "/", " / ")
		}

		var selection string
		_ = huh.NewSelect[string]().
			Title(breadcrumb).
			Description("Navigate: ↑↓ | Select file/folder: Enter | Cancel: Ctrl+C").
			Options(opts...).
			Value(&selection).
			Height(15).
			Run()

		if selection == "CANCEL" {
			return ""
		}
		if selection == "SELECT_CURRENT" {
			return currentDir
		}
		if selection == ".." {
			currentDir = path.Dir(strings.TrimSuffix(currentDir, "/"))
			if currentDir == "." {
				currentDir = ""
			}
			continue
		}
		if strings.HasSuffix(selection, "/") {
			if currentDir == "" {
				currentDir = strings.TrimSuffix(selection, "/")
			} else {
				currentDir = currentDir + "/" + strings.TrimSuffix(selection, "/")
			}
		} else {
			full := selection
			if currentDir != "" {
				full = currentDir + "/" + selection
			}
			return full
		}
	}
}

func runLocalBrowser(mgr VendorManager) string {
	currentDir := "."
	for {
		items, err := mgr.ListLocalDir(currentDir)
		if err != nil {
			PrintError("Error", err.Error())
			return ""
		}

		var opts []huh.Option[string]
		if currentDir != "." {
			opts = append(opts, huh.NewOption(".. (Go Up)", ".."))
		}
		opts = append(opts, huh.NewOption(fmt.Sprintf("✔ Select '%s'", currentDir), "SELECT_CURRENT"))

		for _, item := range items {
			var label string
			if strings.HasSuffix(item, "/") {
				label = "📂 " + item
			} else {
				label = "📄 " + item
			}
			opts = append(opts, huh.NewOption(label, item))
		}
		opts = append(opts, huh.NewOption("❌ Cancel", "CANCEL"))

		var selection string
		_ = huh.NewSelect[string]().
			Title(fmt.Sprintf("Local: %s", currentDir)).
			Description("Navigate: ↑↓ | Select file/folder: Enter | Cancel: Ctrl+C").
			Options(opts...).
			Value(&selection).
			Height(15).
			Run()

		if selection == "CANCEL" {
			return ""
		}
		if selection == "SELECT_CURRENT" {
			return currentDir
		}
		if selection == ".." {
			currentDir = path.Dir(currentDir)
			continue
		}
		if strings.HasSuffix(selection, "/") {
			currentDir = path.Join(currentDir, selection)
		} else {
			return path.Join(currentDir, selection)
		}
	}
}

// RunEditWizardName prompts the user to select a vendor to edit from the list.
func RunEditWizardName(vendorNames []string) string {
	var selected string
	_ = huh.NewSelect[string]().Title("Select Vendor to Edit").Options(huh.NewOptions(vendorNames...)...).Value(&selected).Run()
	return selected
}

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
	_ = huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(fmt.Sprintf("Accept %s License?", license)).Value(&confirm))).Run()
	return confirm
}

// PrintHelp displays usage information for git-vendor commands.
func PrintHelp() {
	fmt.Println(styleTitle.Render("git-vendor v5.0"))
	fmt.Println("Vendor specific files/directories from Git repositories with deterministic locking")
	fmt.Println("\nCommands:")
	fmt.Println("  init                Initialize vendor directory")
	fmt.Println("  add                 Add a new vendor dependency (interactive wizard)")
	fmt.Println("  edit                Modify existing vendor configuration")
	fmt.Println("  remove <name>       Remove a vendor by name")
	fmt.Println("  list                Show all configured vendors with dependency tree")
	fmt.Println("  sync [options] [vendor-name]")
	fmt.Println("                      Download dependencies to locked versions")
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
	fmt.Println("  status              Check if local files are in sync with lockfile")
	fmt.Println("  check-updates       Check for available updates to vendors")
	fmt.Println("  diff <vendor>       Show commit differences between locked and latest")
	fmt.Println("  watch               Watch for config changes and auto-sync")
	fmt.Println("  completion <shell>  Generate shell completion script (bash/zsh/fish/powershell)")
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
	fmt.Println("  git-vendor validate")
	fmt.Println("  git-vendor status")
	fmt.Println("  git-vendor check-updates")
	fmt.Println("  git-vendor diff my-vendor")
	fmt.Println("  git-vendor watch")
	fmt.Println("  git-vendor completion bash > /etc/bash_completion.d/git-vendor")
	fmt.Println("  git-vendor remove my-vendor")
	fmt.Println("\nNavigation:")
	fmt.Println("  Use arrow keys to navigate, Enter to select")
	fmt.Println("  Press Ctrl+C to cancel at any time")
}

// ShowConflictWarnings displays any path conflicts involving the given vendor
func ShowConflictWarnings(mgr VendorManager, vendorName string) {
	conflicts, err := mgr.DetectConflicts()
	if err != nil {
		return // Silently skip if detection fails
	}

	// Filter conflicts for this vendor
	var vendorConflicts []types.PathConflict
	for _, c := range conflicts {
		if c.Vendor1 == vendorName || c.Vendor2 == vendorName {
			vendorConflicts = append(vendorConflicts, c)
		}
	}

	if len(vendorConflicts) > 0 {
		fmt.Println()
		PrintWarning("Path Conflicts Detected", fmt.Sprintf("Found %s with this vendor", core.Pluralize(len(vendorConflicts), "conflict", "conflicts")))
		for _, c := range vendorConflicts {
			fmt.Printf("  ⚠ %s\n", c.Path)
			other := c.Vendor2
			if c.Vendor2 == vendorName {
				other = c.Vendor1
			}
			fmt.Printf("    Conflicts with vendor: %s\n", other)
		}
		fmt.Println()
		fmt.Println("  Run 'git-vendor validate' for full details")
		fmt.Println()
	}
}
