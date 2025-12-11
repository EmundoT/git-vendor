package main

import (
	"fmt"
	"git-vendor/internal/core"
	"git-vendor/internal/tui"
	"git-vendor/internal/types"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
)

func main() {
	if len(os.Args) < 2 {
		tui.PrintHelp()
		os.Exit(0)
	}

	if !core.IsGitInstalled() {
		tui.PrintError("Error", "git not found.")
		os.Exit(1)
	}

	manager := core.NewManager()
	command := os.Args[1]

	switch command {
	case "init":
    manager.Init()
    tui.PrintSuccess("Initialized in ./vendor/")

	case "add":
		cfg, err := manager.GetConfig()
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}
		existing := make(map[string]types.VendorSpec)
		for _, v := range cfg.Vendors { existing[v.URL] = v }

		spec := tui.RunAddWizard(manager, existing)
		if spec == nil { return }

		if err := manager.AddVendor(*spec); err != nil {
			tui.PrintError("Failed", err.Error())
			os.Exit(1)
		}
		tui.PrintSuccess("Done.")

		// Show conflict warnings after adding vendor
		tui.ShowConflictWarnings(manager, spec.Name)

	case "edit":
		cfg, err := manager.GetConfig()
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}
		var names []string
		for _, v := range cfg.Vendors { names = append(names, v.Name) }
		
		if len(names) == 0 {
			tui.PrintWarning("Empty", "No vendors found.")
			return
		}

		targetName := tui.RunEditWizardName(names)
		
		var targetVendor types.VendorSpec
		for _, v := range cfg.Vendors {
			if v.Name == targetName { targetVendor = v; break }
		}
		
		updatedSpec := tui.RunEditVendorWizard(manager, targetVendor)
		if updatedSpec == nil { return }
		
		if err := manager.SaveVendor(*updatedSpec); err != nil {
			tui.PrintError("Error", err.Error())
		} else {
			tui.PrintSuccess("Saved " + updatedSpec.Name)
			// Show conflict warnings after editing vendor (conflicts already shown in wizard, but show again for clarity)
		}

	case "remove":
		if len(os.Args) < 3 {
			tui.PrintError("Usage", "git vendor remove <name>")
			return
		}
		name := os.Args[2]

		// Add confirmation prompt
		confirmed := false
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Remove vendor '%s'?", name)).
			Description("This will delete the config entry and license file.").
			Value(&confirmed).
			Run()

		if err != nil {
			tui.PrintError("Error", err.Error())
			return
		}

		if !confirmed {
			fmt.Println("Cancelled.")
			return
		}

		if err := manager.RemoveVendor(name); err != nil {
			tui.PrintError("Error", err.Error())
		} else {
			tui.PrintSuccess("Removed " + name)
		}

	case "list":
		cfg, err := manager.GetConfig()
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}
		if len(cfg.Vendors) == 0 {
			fmt.Println("No vendors configured.")
		} else {
			// Check for conflicts
			conflicts, _ := manager.DetectConflicts()
			conflictMap := make(map[string]bool)
			if len(conflicts) > 0 {
				for _, c := range conflicts {
					conflictMap[c.Vendor1] = true
					conflictMap[c.Vendor2] = true
				}
			}

			fmt.Println(tui.StyleTitle("Configured Vendors:"))
			fmt.Println()

			for _, v := range cfg.Vendors {
				// Show conflict indicator
				conflictIndicator := ""
				if conflictMap[v.Name] {
					conflictIndicator = " ⚠"
				}

				fmt.Printf("📦 %s%s\n", v.Name, conflictIndicator)
				fmt.Printf("   %s\n", v.URL)
				fmt.Printf("   License: %s\n", v.License)

				for _, s := range v.Specs {
					fmt.Printf("   └─ @ %s\n", s.Ref)
					for i, m := range s.Mapping {
						dest := m.To
						if dest == "" {
							dest = "(auto)"
						}

						prefix := "      ├─"
						if i == len(s.Mapping)-1 {
							prefix = "      └─"
						}

						fmt.Printf("%s %s → %s\n", prefix, m.From, dest)
					}
				}
				fmt.Println()
			}

			// Show conflict summary
			if len(conflicts) > 0 {
				tui.PrintWarning("Conflicts Detected", fmt.Sprintf("%d path conflict(s) found. Run 'git-vendor validate' for details.", len(conflicts)))
			}
		}

	case "sync":
		// Parse flags and arguments
		dryRun := false
		force := false
		vendorName := ""

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "--dry-run" {
				dryRun = true
			} else if arg == "--force" {
				force = true
			} else if arg == "--verbose" || arg == "-v" {
				core.Verbose = true
			} else if !strings.HasPrefix(arg, "--") {
				vendorName = arg
			}
		}

		if dryRun {
			if err := manager.SyncDryRun(); err != nil {
				tui.PrintError("Preview Failed", err.Error())
				os.Exit(1)
			}
			fmt.Println("This is a dry-run. No files were modified.")
			fmt.Println("Run 'git-vendor sync' to apply changes.")
		} else {
			if err := manager.SyncWithOptions(vendorName, force); err != nil {
				tui.PrintError("Sync Failed", err.Error())
				os.Exit(1)
			}
			tui.PrintSuccess("Synced.")
		}

	case "update":
		// Parse flags
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "--verbose" || arg == "-v" {
				core.Verbose = true
			}
		}

		if err := manager.UpdateAll(); err != nil {
			tui.PrintError("Update Failed", err.Error())
			os.Exit(1)
		}
		tui.PrintSuccess("Updated all vendors.")

	case "validate":
		// Perform config validation
		if err := manager.ValidateConfig(); err != nil {
			tui.PrintError("Validation Failed", err.Error())
			os.Exit(1)
		}

		// Check for conflicts
		conflicts, err := manager.DetectConflicts()
		if err != nil {
			tui.PrintError("Conflict Detection Failed", err.Error())
			os.Exit(1)
		}

		if len(conflicts) > 0 {
			tui.PrintWarning("Path Conflicts Detected", fmt.Sprintf("Found %d conflict(s)", len(conflicts)))
			fmt.Println()
			for _, conflict := range conflicts {
				fmt.Printf("⚠ Conflict: %s\n", conflict.Path)
				fmt.Printf("  • %s: %s → %s\n", conflict.Vendor1, conflict.Mapping1.From, conflict.Mapping1.To)
				fmt.Printf("  • %s: %s → %s\n", conflict.Vendor2, conflict.Mapping2.From, conflict.Mapping2.To)
				fmt.Println()
			}
			os.Exit(1)
		}

		tui.PrintSuccess("Validation passed. No issues found.")

	default:
		tui.PrintHelp()
	}
}