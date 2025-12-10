package main

import (
	"fmt"
	"git-vendor/internal/core"
	"git-vendor/internal/tui"
	"git-vendor/internal/types"
	"os"
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
		cfg, _ := manager.GetConfig()
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
		cfg, _ := manager.GetConfig()
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
		if err := manager.RemoveVendor(name); err != nil {
			tui.PrintError("Error", err.Error())
		} else {
			tui.PrintSuccess("Removed " + name)
		}

	case "list":
		cfg, _ := manager.GetConfig()
		if len(cfg.Vendors) == 0 {
			fmt.Println("No vendors configured.")
		} else {
			fmt.Println("Configured Vendors:")
			for _, v := range cfg.Vendors {
				fmt.Printf("- %s (%s)\n", v.Name, v.URL)
				for _, s := range v.Specs {
					fmt.Printf("  @ %s\n", s.Ref)
					for _, m := range s.Mapping {
						fmt.Printf("    • %s -> %s\n", m.From, m.To)
					}
				}
			}
		}

	case "sync":
		if err := manager.Sync(); err != nil {
			tui.PrintError("Sync Failed", err.Error())
			os.Exit(1)
		}
		tui.PrintSuccess("Synced.")

	case "update":
		manager.UpdateAll()

	default:
		tui.PrintHelp()
	}
}