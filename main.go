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
			fmt.Println("Configured Vendors:")
			for _, v := range cfg.Vendors {
				fmt.Printf("- %s (%s)\n", v.Name, v.URL)
				for _, s := range v.Specs {
					fmt.Printf("  @ %s\n", s.Ref)
					for _, m := range s.Mapping {
						dest := m.To
						if dest == "" {
							dest = "(auto)"
						}
						fmt.Printf("    • %s -> %s\n", m.From, dest)
					}
				}
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
		if err := manager.UpdateAll(); err != nil {
			tui.PrintError("Update Failed", err.Error())
			os.Exit(1)
		}
		tui.PrintSuccess("Updated all vendors.")

	default:
		tui.PrintHelp()
	}
}