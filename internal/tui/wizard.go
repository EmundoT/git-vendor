package tui

import (
	"fmt"
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
	styleNew     = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	styleCard    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("238"))
)

type VendorManager interface {
	ParseSmartURL(string) (string, string, string) 
	FetchRepoDir(string, string, string) ([]string, error)
	ListLocalDir(string) ([]string, error)
}

func check(err error) {
	if err != nil {
		fmt.Println("Aborted.")
		os.Exit(1)
	}
}

// --- ADD WIZARD ---

func RunAddWizard(mgr interface{}, existingVendors map[string]types.VendorSpec) *types.VendorSpec {
	manager := mgr.(VendorManager)
	
	// Temporary flat struct for wizard input
	var name, url, ref string
	
	var rawURL string
	err := huh.NewInput().
		Title("Remote URL").
		Placeholder("https://github.com/owner/repo").
		Description("Paste a full repo URL or a specific file link").
		Value(&rawURL).
		Run()
	check(err)

	baseURL, smartRef, smartPath := manager.ParseSmartURL(rawURL)
	url = baseURL
	ref = "main"
	if smartRef != "" { ref = smartRef }

	baseName := path.Base(url)
	name = strings.TrimSuffix(baseName, ".git")

	existing, exists := existingVendors[url]
	isAppending := false

	if exists {
		var addToExisting bool = true
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
		var useDeep bool = true
		err = huh.NewConfirm().Title("Track specific path?").Description(smartPath).Value(&useDeep).Run()
		check(err)
		if useDeep {
			var dest string
			huh.NewInput().Title("Local Target").Description("Hit Enter for auto").Value(&dest).Run()
			spec.Specs[0].Mapping = append(spec.Specs[0].Mapping, types.PathMapping{From: smartPath, To: dest})
		}
	}

	// Enter Edit Loop immediately for the new vendor
	return RunEditVendorWizard(mgr, spec)
}

// --- EDIT WIZARD (The Core Loop) ---

func RunEditVendorWizard(mgr interface{}, vendor types.VendorSpec) *types.VendorSpec {
	manager := mgr.(VendorManager)
	
	for {
		// 1. Select Ref (Branch) to Edit
		var branchOpts []huh.Option[string]
		for i, s := range vendor.Specs {
			label := fmt.Sprintf("Branch: %s (%d mappings)", s.Ref, len(s.Mapping))
			branchOpts = append(branchOpts, huh.NewOption(label, fmt.Sprintf("%d", i)))
		}
		branchOpts = append(branchOpts, huh.NewOption("+ Add New Branch", "new"))
		branchOpts = append(branchOpts, huh.NewOption("💾 Save & Exit", "save"))
		branchOpts = append(branchOpts, huh.NewOption("❌ Cancel", "cancel"))

		var selection string
		fmt.Println(styleCard.Render(fmt.Sprintf("Editing Vendor: %s", vendor.Name)))
		
		err := huh.NewSelect[string]().
			Title("Select Branch to Manage").
			Options(branchOpts...).
			Value(&selection).
			Height(10).
			Run()
		check(err)

		if selection == "cancel" { return nil }
		if selection == "save" { return &vendor }
		
		if selection == "new" {
			var newRef string
			huh.NewInput().Title("New Branch/Tag Name").Value(&newRef).Run()
			vendor.Specs = append(vendor.Specs, types.BranchSpec{Ref: newRef})
			selection = fmt.Sprintf("%d", len(vendor.Specs)-1)
		}

		// 2. Manage Mappings for Selected Branch
		var idx int
		fmt.Sscanf(selection, "%d", &idx)
		
		updatedBranch := runMappingManager(manager, vendor.URL, vendor.Specs[idx])
		vendor.Specs[idx] = updatedBranch
	}
}

func runMappingManager(mgr VendorManager, url string, branch types.BranchSpec) types.BranchSpec {
	for {
		var opts []huh.Option[string]
		for i, m := range branch.Mapping {
			label := fmt.Sprintf("%-20s → %s", truncate(m.From, 20), m.To)
			opts = append(opts, huh.NewOption(label, fmt.Sprintf("%d", i)))
		}
		opts = append(opts, huh.NewOption("+ Add Mapping", "add"))
		opts = append(opts, huh.NewOption("← Back", "back"))

		var selection string
		fmt.Println(styleDim.Render(fmt.Sprintf("Managing mappings for %s", branch.Ref)))
		huh.NewSelect[string]().
			Title("Mappings").
			Options(opts...).
			Value(&selection).
			Height(10).
			Run()

		if selection == "back" { return branch }
		
		if selection == "add" {
			newMap := runMappingCreator(mgr, url, branch.Ref)
			if newMap != nil {
				branch.Mapping = append(branch.Mapping, *newMap)
			}
			continue
		}

		// Edit/Delete
		var idx int
		fmt.Sscanf(selection, "%d", &idx)
		
		var action string
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Mapping: %s", branch.Mapping[idx].From)).
			Options(
				huh.NewOption("Edit Paths", "edit"),
				huh.NewOption("Delete", "delete"),
				huh.NewOption("← Back", "back"),
			).Value(&action).Run()

		if action == "delete" {
			branch.Mapping = append(branch.Mapping[:idx], branch.Mapping[idx+1:]...)
		} else if action == "edit" {
			// Reuse creator for editing
			// Ideally pre-fill, but for now simple edit inputs
			huh.NewInput().Title("Remote Path").Value(&branch.Mapping[idx].From).Run()
			huh.NewInput().Title("Local Target").Value(&branch.Mapping[idx].To).Run()
		}
	}
}

func runMappingCreator(mgr VendorManager, url, ref string) *types.PathMapping {
	var m types.PathMapping
	
	// Remote Path
	var mode string
	huh.NewSelect[string]().
		Title("Remote Path").
		Options(
			huh.NewOption("Browse Remote Files", "browse"),
			huh.NewOption("Enter Manually", "manual"),
		).Value(&mode).Run()

	if mode == "browse" {
		m.From = runRemoteBrowser(mgr, url, ref)
		if m.From == "" { return nil }
	} else {
		huh.NewInput().Title("Remote Path").Value(&m.From).Run()
	}

	// Local Target
	huh.NewSelect[string]().
		Title("Local Target").
		Options(
			huh.NewOption("Browse Local Files", "browse"),
			huh.NewOption("Enter Manually", "manual"),
		).Value(&mode).Run()

	if mode == "browse" {
		m.To = runLocalBrowser(mgr)
		// If they picked a folder, we assume they want to put it inside? Or rename?
		// Simple behavior: If browsing, selecting a folder usually means "put it here".
		// But if mapping a file, selecting a folder means "put in this folder".
		// If mapping a folder, it might mean "put content here" or "create subfolder".
		// We'll let them edit the path after picking.
		huh.NewInput().Title("Refine Local Path").Value(&m.To).Run()
	} else {
		huh.NewInput().Title("Local Target").Description("Hit Enter for auto").Value(&m.To).Run()
	}

	return &m
}

func runRemoteBrowser(mgr VendorManager, url, ref string) string {
	currentDir := ""
	for {
		items, err := mgr.FetchRepoDir(url, ref, currentDir)
		if err != nil { PrintError("Error", err.Error()); return "" }

		var opts []huh.Option[string]
		if currentDir != "" {
			opts = append(opts, huh.NewOption(".. (Go Up)", ".."))
			opts = append(opts, huh.NewOption(fmt.Sprintf("✔ Select '/%s'", currentDir), "SELECT_CURRENT"))
		} else {
			opts = append(opts, huh.NewOption("✔ Select Root", "SELECT_CURRENT"))
		}

		for _, item := range items {
			label := item
			if strings.HasSuffix(item, "/") { label = "📂 " + item } else { label = "📄 " + item }
			opts = append(opts, huh.NewOption(label, item))
		}
		opts = append(opts, huh.NewOption("❌ Cancel", "CANCEL"))

		var selection string
		huh.NewSelect[string]().Title(fmt.Sprintf("Remote: /%s", currentDir)).Options(opts...).Value(&selection).Height(15).Run()

		if selection == "CANCEL" { return "" }
		if selection == "SELECT_CURRENT" { return currentDir }
		if selection == ".." {
			currentDir = path.Dir(strings.TrimSuffix(currentDir, "/"))
			if currentDir == "." { currentDir = "" }
			continue
		}
		if strings.HasSuffix(selection, "/") {
			if currentDir == "" { currentDir = strings.TrimSuffix(selection, "/") } else { currentDir = currentDir + "/" + strings.TrimSuffix(selection, "/") }
		} else {
			full := selection
			if currentDir != "" { full = currentDir + "/" + selection }
			return full
		}
	}
}

func runLocalBrowser(mgr VendorManager) string {
	currentDir := "."
	for {
		items, err := mgr.ListLocalDir(currentDir)
		if err != nil { PrintError("Error", err.Error()); return "" }

		var opts []huh.Option[string]
		if currentDir != "." {
			opts = append(opts, huh.NewOption(".. (Go Up)", ".."))
		}
		opts = append(opts, huh.NewOption(fmt.Sprintf("✔ Select '%s'", currentDir), "SELECT_CURRENT"))

		for _, item := range items {
			label := item
			if strings.HasSuffix(item, "/") { label = "📂 " + item } else { label = "📄 " + item }
			opts = append(opts, huh.NewOption(label, item))
		}
		opts = append(opts, huh.NewOption("❌ Cancel", "CANCEL"))

		var selection string
		huh.NewSelect[string]().Title(fmt.Sprintf("Local: %s", currentDir)).Options(opts...).Value(&selection).Height(15).Run()

		if selection == "CANCEL" { return "" }
		if selection == "SELECT_CURRENT" { return currentDir }
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

func RunEditWizardName(vendorNames []string) string {
	var selected string
	huh.NewSelect[string]().Title("Select Vendor to Edit").Options(huh.NewOptions(vendorNames...)...).Value(&selected).Run()
	return selected
}

func truncate(s string, max int) string {
	if len(s) > max { return s[:max-3] + "..." }
	return s
}

func PrintError(title, msg string) { fmt.Println(styleErr.Render("✖ " + title)); fmt.Println(msg) }
func PrintSuccess(msg string) { fmt.Println(styleSuccess.Render("✔ " + msg)) }
func PrintInfo(msg string) { fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(msg)) }
func PrintWarning(title, msg string) { fmt.Println(styleWarn.Render("! " + title)); fmt.Println(msg) }
func PrintComplianceSuccess(license string) { fmt.Println(styleSuccess.Render(fmt.Sprintf("✔ License Verified: %s", license))) }
func AskToOverrideCompliance(license string) bool {
	var confirm bool
	huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(fmt.Sprintf("Accept %s License?", license)).Value(&confirm))).Run()
	return confirm
}
func PrintHelp() {
	fmt.Println(styleTitle.Render("git-vendor v5.0"))
	fmt.Println("Usage: add, edit, remove, sync, update")
}