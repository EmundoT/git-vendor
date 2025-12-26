package main

import (
	"fmt"
	"git-vendor/internal/core"
	"git-vendor/internal/tui"
	"git-vendor/internal/types"
	"os"
	"strings"
)

// parseCommonFlags extracts common non-interactive flags from args
// Returns: flags, remainingArgs, error
func parseCommonFlags(args []string) (core.NonInteractiveFlags, []string, error) {
	flags := core.NonInteractiveFlags{}
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--yes", "-y":
			flags.Yes = true
		case "--quiet", "-q":
			flags.Mode = core.OutputQuiet
		case "--json":
			flags.Mode = core.OutputJSON
		case "--verbose", "-v":
			// Handle verbose separately (backward compat)
			remaining = append(remaining, arg)
		default:
			remaining = append(remaining, arg)
		}
	}

	return flags, remaining, nil
}

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
	manager.SetUICallback(tui.NewTUICallback()) // Set TUI for user interaction
	command := os.Args[1]

	switch command {
	case "init":
    manager.Init()
    tui.PrintSuccess("Initialized in ./vendor/")

	case "add":
		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

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
		tui.PrintSuccess(fmt.Sprintf("Added %s", spec.Name))

		// Show conflict warnings after adding vendor
		tui.ShowConflictWarnings(manager, spec.Name)

		// Show next steps
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  git-vendor sync      # Download files at locked versions")
		fmt.Println("  git-vendor update    # Fetch latest commits and update lockfile")

	case "edit":
		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

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
		// Parse common flags
		flags, args, err := parseCommonFlags(os.Args[2:])
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}

		// Get vendor name from remaining args
		if len(args) < 1 {
			tui.PrintError("Usage", "git-vendor remove <name>")
			os.Exit(1)
		}
		name := args[0]

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Check if vendor exists BEFORE showing confirmation
		cfg, err := manager.GetConfig()
		if err != nil {
			callback.ShowError("Error", err.Error())
			os.Exit(1)
		}

		found := false
		for _, v := range cfg.Vendors {
			if v.Name == name {
				found = true
				break
			}
		}

		if !found {
			callback.ShowError("Error", fmt.Sprintf("vendor '%s' not found", name))
			os.Exit(1)
		}

		// Show confirmation via callback
		confirmed := callback.AskConfirmation(
			fmt.Sprintf("Remove vendor '%s'?", name),
			"This will delete the config entry and license file.",
		)

		if !confirmed {
			if flags.Mode != core.OutputQuiet {
				fmt.Println("Cancelled.")
			}
			os.Exit(1)
		}

		if err := manager.RemoveVendor(name); err != nil {
			callback.ShowError("Error", err.Error())
			os.Exit(1)
		}
		callback.ShowSuccess("Removed " + name)

	case "list":
		// Parse common flags
		flags, _, err := parseCommonFlags(os.Args[2:])
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		cfg, err := manager.GetConfig()
		if err != nil {
			callback.ShowError("Error", err.Error())
			os.Exit(1)
		}

		// Check for conflicts
		conflicts, _ := manager.DetectConflicts()
		conflictMap := make(map[string]bool)
		if len(conflicts) > 0 {
			for _, c := range conflicts {
				conflictMap[c.Vendor1] = true
				conflictMap[c.Vendor2] = true
			}
		}

		if flags.Mode == core.OutputJSON {
			// JSON output mode
			vendorData := make([]map[string]interface{}, 0, len(cfg.Vendors))
			for _, v := range cfg.Vendors {
				specsData := make([]map[string]interface{}, 0, len(v.Specs))
				for _, s := range v.Specs {
					mappingsData := make([]map[string]interface{}, 0, len(s.Mapping))
					for _, m := range s.Mapping {
						mappingsData = append(mappingsData, map[string]interface{}{
							"from": m.From,
							"to":   m.To,
						})
					}
					specsData = append(specsData, map[string]interface{}{
						"ref":      s.Ref,
						"mappings": mappingsData,
					})
				}
				vendorData = append(vendorData, map[string]interface{}{
					"name":         v.Name,
					"url":          v.URL,
					"license":      v.License,
					"specs":        specsData,
					"has_conflict": conflictMap[v.Name],
				})
			}

			callback.FormatJSON(core.JSONOutput{
				Status: "success",
				Data: map[string]interface{}{
					"vendors":        vendorData,
					"vendor_count":   len(cfg.Vendors),
					"conflict_count": len(conflicts),
				},
			})
		} else if len(cfg.Vendors) == 0 {
			if flags.Mode != core.OutputQuiet {
				fmt.Println("No vendors configured.")
			}
		} else {
			// Normal output mode
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

						fmt.Printf("%s %s (remote) → %s (local)\n", prefix, m.From, dest)
					}
				}
				fmt.Println()
			}

			// Show conflict summary
			if len(conflicts) > 0 {
				tui.PrintWarning("Conflicts Detected", fmt.Sprintf("%s found. Run 'git-vendor validate' for details.", core.Pluralize(len(conflicts), "path conflict", "path conflicts")))
			}
		}

	case "sync":
		// Parse common flags
		flags, args, err := parseCommonFlags(os.Args[2:])
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Parse command-specific flags
		dryRun := false
		force := false
		vendorName := ""

		for _, arg := range args {
			if arg == "--dry-run" {
				dryRun = true
			} else if arg == "--force" {
				force = true
			} else if arg == "--verbose" || arg == "-v" {
				core.Verbose = true
				manager.UpdateVerboseMode(true)
			} else if !strings.HasPrefix(arg, "--") {
				vendorName = arg
			}
		}

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		if dryRun {
			if err := manager.SyncDryRun(); err != nil {
				callback.ShowError("Preview Failed", err.Error())
				os.Exit(1)
			}
			if flags.Mode != core.OutputQuiet {
				fmt.Println("This is a dry-run. No files were modified.")
				fmt.Println("Run 'git-vendor sync' to apply changes.")
			}
		} else {
			if err := manager.SyncWithOptions(vendorName, force); err != nil {
				callback.ShowError("Sync Failed", err.Error())
				os.Exit(1)
			}
			callback.ShowSuccess("Synced.")
		}

	case "update":
		// Parse common flags
		flags, args, err := parseCommonFlags(os.Args[2:])
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Parse command-specific flags
		for _, arg := range args {
			if arg == "--verbose" || arg == "-v" {
				core.Verbose = true
				manager.UpdateVerboseMode(true)
			}
		}

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		if err := manager.UpdateAll(); err != nil {
			callback.ShowError("Update Failed", err.Error())
			os.Exit(1)
		}
		callback.ShowSuccess("Updated all vendors.")

	case "validate":
		// Parse common flags
		flags, _, err := parseCommonFlags(os.Args[2:])
		if err != nil {
			tui.PrintError("Error", err.Error())
			os.Exit(1)
		}

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		// Get config for summary
		cfg, err := manager.GetConfig()
		if err != nil {
			callback.ShowError("Error", err.Error())
			os.Exit(1)
		}

		// Perform config validation
		if err := manager.ValidateConfig(); err != nil {
			callback.ShowError("Validation Failed", err.Error())
			os.Exit(1)
		}

		// Check for conflicts
		conflicts, err := manager.DetectConflicts()
		if err != nil {
			callback.ShowError("Conflict Detection Failed", err.Error())
			os.Exit(1)
		}

		if flags.Mode == core.OutputJSON {
			// JSON output mode
			conflictsData := make([]map[string]interface{}, 0, len(conflicts))
			for _, conflict := range conflicts {
				conflictsData = append(conflictsData, map[string]interface{}{
					"path":    conflict.Path,
					"vendor1": conflict.Vendor1,
					"vendor2": conflict.Vendor2,
					"mapping1": map[string]interface{}{
						"from": conflict.Mapping1.From,
						"to":   conflict.Mapping1.To,
					},
					"mapping2": map[string]interface{}{
						"from": conflict.Mapping2.From,
						"to":   conflict.Mapping2.To,
					},
				})
			}

			if len(conflicts) > 0 {
				callback.FormatJSON(core.JSONOutput{
					Status:  "error",
					Message: fmt.Sprintf("Found %s", core.Pluralize(len(conflicts), "conflict", "conflicts")),
					Data: map[string]interface{}{
						"config_valid":   true,
						"conflicts":      conflictsData,
						"conflict_count": len(conflicts),
						"vendor_count":   len(cfg.Vendors),
					},
				})
				os.Exit(1)
			} else {
				callback.FormatJSON(core.JSONOutput{
					Status:  "success",
					Message: "Validation passed",
					Data: map[string]interface{}{
						"config_valid":   true,
						"conflicts":      []map[string]interface{}{},
						"conflict_count": 0,
						"vendor_count":   len(cfg.Vendors),
					},
				})
			}
		} else {
			// Normal output mode
			if len(conflicts) > 0 {
				tui.PrintWarning("Path Conflicts Detected", fmt.Sprintf("Found %s", core.Pluralize(len(conflicts), "conflict", "conflicts")))
				fmt.Println()
				for _, conflict := range conflicts {
					fmt.Printf("⚠ Conflict: %s\n", conflict.Path)
					fmt.Printf("  • %s: %s (remote) → %s (local)\n", conflict.Vendor1, conflict.Mapping1.From, conflict.Mapping1.To)
					fmt.Printf("  • %s: %s (remote) → %s (local)\n", conflict.Vendor2, conflict.Mapping2.From, conflict.Mapping2.To)
					fmt.Println()
				}
				os.Exit(1)
			}

			tui.PrintSuccess("Validation passed")
			fmt.Println("• Config syntax: OK")
			fmt.Println("• Path conflicts: None")
			fmt.Printf("• Vendors: %s\n", core.Pluralize(len(cfg.Vendors), "vendor", "vendors"))
		}

	default:
		tui.PrintError("Unknown Command", fmt.Sprintf("'%s' is not a valid git-vendor command", command))
		fmt.Println()
		tui.PrintHelp()
		os.Exit(1)
	}
}