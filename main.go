// Package main implements the git-vendor CLI tool for managing vendored dependencies from Git repositories.
package main

import (
	"fmt"
	"git-vendor/cmd"
	"git-vendor/internal/core"
	"git-vendor/internal/tui"
	"git-vendor/internal/types"
	"git-vendor/internal/version"
	"os"
	"strings"
)

// Version information is managed in internal/version package
// GoReleaser injects version info directly via ldflags

// parseCommonFlags extracts common non-interactive flags from args
// Returns: flags, remainingArgs
func parseCommonFlags(args []string) (core.NonInteractiveFlags, []string) {
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

	return flags, remaining
}

// countSynced counts how many vendors are synced
func countSynced(statuses []types.VendorStatus) int {
	count := 0
	for _, s := range statuses {
		if s.IsSynced {
			count++
		}
	}
	return count
}

func main() {
	if len(os.Args) < 2 {
		tui.PrintHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	// Handle help flags
	if command == "--help" || command == "-h" || command == "help" {
		tui.PrintHelp()
		os.Exit(0)
	}

	// Handle version flag
	if command == "--version" {
		fmt.Printf("git-vendor %s\n", version.Version)
		fmt.Printf("  commit: %s\n", version.Commit)
		fmt.Printf("  built:  %s\n", version.Date)
		os.Exit(0)
	}

	if !core.IsGitInstalled() {
		tui.PrintError("Error", "git not found.")
		os.Exit(1)
	}

	manager := core.NewManager()
	manager.SetUICallback(tui.NewTUICallback()) // Set TUI for user interaction

	switch command {
	case "init":
		if err := manager.Init(); err != nil {
			tui.PrintError("Initialization Failed", err.Error())
			os.Exit(1)
		}
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
		for _, v := range cfg.Vendors {
			existing[v.URL] = v
		}

		spec := tui.RunAddWizard(manager, existing)
		if spec == nil {
			return
		}

		if err := manager.AddVendor(spec); err != nil {
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
		fmt.Println("  git vendor update    # Fetch latest commits (also works)")

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
		for _, v := range cfg.Vendors {
			names = append(names, v.Name)
		}

		if len(names) == 0 {
			tui.PrintWarning("Empty", "No vendors found.")
			return
		}

		targetName := tui.RunEditWizardName(names)

		var targetVendor types.VendorSpec
		for _, v := range cfg.Vendors {
			if v.Name == targetName {
				targetVendor = v
				break
			}
		}

		updatedSpec := tui.RunEditVendorWizard(manager, &targetVendor)
		if updatedSpec == nil {
			return
		}

		if err := manager.SaveVendor(updatedSpec); err != nil {
			tui.PrintError("Error", err.Error())
		} else {
			tui.PrintSuccess("Saved " + updatedSpec.Name)
			// Show conflict warnings after editing vendor (conflicts already shown in wizard, but show again for clarity)
		}

	case "remove":
		// Parse common flags
		flags, args := parseCommonFlags(os.Args[2:])

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
		flags, _ := parseCommonFlags(os.Args[2:])

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

		// Check for conflicts (best-effort, don't fail list command if detection fails)
		conflicts, _ := manager.DetectConflicts() //nolint:errcheck
		conflictMap := make(map[string]bool)
		if len(conflicts) > 0 {
			for _, c := range conflicts {
				conflictMap[c.Vendor1] = true
				conflictMap[c.Vendor2] = true
			}
		}

		switch {
		case flags.Mode == core.OutputJSON:
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

			_ = callback.FormatJSON(core.JSONOutput{
				Status: "success",
				Data: map[string]interface{}{
					"vendors":        vendorData,
					"vendor_count":   len(cfg.Vendors),
					"conflict_count": len(conflicts),
				},
			})
		case len(cfg.Vendors) == 0:
			if flags.Mode != core.OutputQuiet {
				fmt.Println("No vendors configured.")
			}
		default:
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
		flags, args := parseCommonFlags(os.Args[2:])

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
		noCache := false
		vendorName := ""
		groupName := ""
		parallel := false
		workers := 0

		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case arg == "--dry-run":
				dryRun = true
			case arg == "--force":
				force = true
			case arg == "--no-cache":
				noCache = true
			case arg == "--parallel":
				parallel = true
			case arg == "--workers":
				if i+1 < len(args) {
					if _, err := fmt.Sscanf(args[i+1], "%d", &workers); err != nil {
						callback.ShowError("Invalid Flag", fmt.Sprintf("--workers requires a valid number, got: %s", args[i+1]))
						os.Exit(1)
					}
					i++ // Skip next arg
				} else {
					callback.ShowError("Invalid Flag", "--workers requires a number")
					os.Exit(1)
				}
			case arg == "--group":
				if i+1 < len(args) {
					groupName = args[i+1]
					i++ // Skip next arg
				} else {
					callback.ShowError("Invalid Flag", "--group requires a group name")
					os.Exit(1)
				}
			case arg == "--verbose" || arg == "-v":
				core.Verbose = true
				manager.UpdateVerboseMode(true)
			case !strings.HasPrefix(arg, "--"):
				vendorName = arg
			}
		}

		// Validate that vendor name and group are not both specified
		if vendorName != "" && groupName != "" {
			callback.ShowError("Invalid Options", "Cannot specify both vendor name and --group")
			os.Exit(1)
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
			// Choose sync method based on flags
			switch {
			case parallel:
				parallelOpts := types.ParallelOptions{
					Enabled:    true,
					MaxWorkers: workers,
				}
				if err := manager.SyncWithParallel(vendorName, force, noCache, parallelOpts); err != nil {
					callback.ShowError("Sync Failed", err.Error())
					os.Exit(1)
				}
			case groupName != "":
				// Use group sync if group is specified
				if err := manager.SyncWithGroup(groupName, force, noCache); err != nil {
					callback.ShowError("Sync Failed", err.Error())
					os.Exit(1)
				}
			default:
				// Regular sync
				if err := manager.SyncWithOptions(vendorName, force, noCache); err != nil {
					callback.ShowError("Sync Failed", err.Error())
					os.Exit(1)
				}
			}
			callback.ShowSuccess("Synced.")
		}

	case "update":
		// Parse common flags
		flags, args := parseCommonFlags(os.Args[2:])

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Parse command-specific flags
		parallel := false
		workers := 0

		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "--verbose", "-v":
				core.Verbose = true
				manager.UpdateVerboseMode(true)
			case "--parallel":
				parallel = true
			case "--workers":
				if i+1 < len(args) {
					if _, err := fmt.Sscanf(args[i+1], "%d", &workers); err != nil {
						callback.ShowError("Invalid Flag", fmt.Sprintf("--workers requires a valid number, got: %s", args[i+1]))
						os.Exit(1)
					}
					i++ // Skip next arg
				} else {
					callback.ShowError("Invalid Flag", "--workers requires a number")
					os.Exit(1)
				}
			}
		}

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		// Use parallel update if requested
		if parallel {
			parallelOpts := types.ParallelOptions{
				Enabled:    true,
				MaxWorkers: workers,
			}
			if err := manager.UpdateAllWithParallel(parallelOpts); err != nil {
				callback.ShowError("Update Failed", err.Error())
				os.Exit(1)
			}
		} else {
			if err := manager.UpdateAll(); err != nil {
				callback.ShowError("Update Failed", err.Error())
				os.Exit(1)
			}
		}
		callback.ShowSuccess("Updated all vendors.")

	case "validate":
		// Parse common flags
		flags, _ := parseCommonFlags(os.Args[2:])

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
				_ = callback.FormatJSON(core.JSONOutput{
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
			}

			_ = callback.FormatJSON(core.JSONOutput{
				Status:  "success",
				Message: "Validation passed",
				Data: map[string]interface{}{
					"config_valid":   true,
					"conflicts":      []map[string]interface{}{},
					"conflict_count": 0,
					"vendor_count":   len(cfg.Vendors),
				},
			})
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

	case "status":
		// Parse common flags
		flags, _ := parseCommonFlags(os.Args[2:])

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

		// Check sync status
		status, err := manager.CheckSyncStatus()
		if err != nil {
			callback.ShowError("Status Check Failed", err.Error())
			os.Exit(1)
		}

		if flags.Mode == core.OutputJSON {
			// JSON output mode
			vendorStatusData := make([]map[string]interface{}, 0, len(status.VendorStatuses))
			for _, vs := range status.VendorStatuses {
				vendorStatusData = append(vendorStatusData, map[string]interface{}{
					"name":          vs.Name,
					"ref":           vs.Ref,
					"is_synced":     vs.IsSynced,
					"missing_paths": vs.MissingPaths,
				})
			}

			_ = callback.FormatJSON(core.JSONOutput{
				Status: func() string {
					if status.AllSynced {
						return "success"
					}
					return "warning"
				}(),
				Message: func() string {
					if status.AllSynced {
						return "All vendors synced"
					}
					return "Some vendors need syncing"
				}(),
				Data: map[string]interface{}{
					"all_synced":      status.AllSynced,
					"vendor_statuses": vendorStatusData,
				},
			})

			if !status.AllSynced {
				os.Exit(1)
			}
		} else {
			// Normal output mode
			if status.AllSynced {
				callback.ShowSuccess("All vendors synced")
			} else {
				// Show which vendors need syncing
				callback.ShowWarning("Vendors Need Syncing", fmt.Sprintf("%s out of sync",
					core.Pluralize(len(status.VendorStatuses)-countSynced(status.VendorStatuses), "vendor", "vendors")))
				fmt.Println()

				for _, vs := range status.VendorStatuses {
					if !vs.IsSynced {
						fmt.Printf("⚠ %s @ %s\n", vs.Name, vs.Ref)
						for _, path := range vs.MissingPaths {
							fmt.Printf("  • Missing: %s\n", path)
						}
					}
				}
				fmt.Println()
				fmt.Println("Run 'git-vendor sync' to fix.")
				os.Exit(1)
			}
		}

	case "check-updates":
		// Parse common flags
		flags, _ := parseCommonFlags(os.Args[2:])

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

		// Check for updates
		updates, err := manager.CheckUpdates()
		if err != nil {
			callback.ShowError("Update Check Failed", err.Error())
			os.Exit(1)
		}

		// Count updates available
		updatesAvailable := 0
		for _, u := range updates {
			if !u.UpToDate {
				updatesAvailable++
			}
		}

		if flags.Mode == core.OutputJSON {
			// JSON output mode
			updateData := make([]map[string]interface{}, 0, len(updates))
			for _, u := range updates {
				updateData = append(updateData, map[string]interface{}{
					"vendor_name":  u.VendorName,
					"ref":          u.Ref,
					"current_hash": u.CurrentHash,
					"latest_hash":  u.LatestHash,
					"last_updated": u.LastUpdated,
					"up_to_date":   u.UpToDate,
				})
			}

			_ = callback.FormatJSON(core.JSONOutput{
				Status: func() string {
					if updatesAvailable == 0 {
						return "success"
					}
					return "warning"
				}(),
				Message: func() string {
					if updatesAvailable == 0 {
						return "All vendors up to date"
					}
					return fmt.Sprintf("%s available", core.Pluralize(updatesAvailable, "update", "updates"))
				}(),
				Data: map[string]interface{}{
					"updates":           updateData,
					"updates_available": updatesAvailable,
					"total_checked":     len(updates),
				},
			})

			if updatesAvailable > 0 {
				os.Exit(1)
			}
		} else {
			// Normal output mode
			if updatesAvailable == 0 {
				callback.ShowSuccess("All vendors up to date")
			} else {
				fmt.Printf("Found %s:\n\n", core.Pluralize(updatesAvailable, "update", "updates"))

				for _, u := range updates {
					if !u.UpToDate {
						fmt.Printf("📦 %s @ %s\n", u.VendorName, u.Ref)
						fmt.Printf("   Current: %s\n", u.CurrentHash[:7])
						fmt.Printf("   Latest:  %s\n", u.LatestHash[:7])
						if u.LastUpdated != "" {
							fmt.Printf("   Updated: %s\n", u.LastUpdated)
						}
						fmt.Println()
					}
				}

				fmt.Println("Run 'git-vendor update' to fetch latest versions")
			}
		}

	case "completion":
		// Generate shell completion script
		if len(os.Args) < 3 {
			tui.PrintError("Usage", "git-vendor completion <shell>\nSupported shells: bash, zsh, fish, powershell")
			os.Exit(1)
		}

		shell := os.Args[2]
		var script string

		switch shell {
		case "bash":
			script = cmd.GenerateBashCompletion()
		case "zsh":
			script = cmd.GenerateZshCompletion()
		case "fish":
			script = cmd.GenerateFishCompletion()
		case "powershell":
			script = cmd.GeneratePowerShellCompletion()
		default:
			tui.PrintError("Invalid Shell", fmt.Sprintf("'%s' is not supported. Use: bash, zsh, fish, or powershell", shell))
			os.Exit(1)
		}

		fmt.Println(script)

	case "diff":
		// Parse common flags
		flags, args := parseCommonFlags(os.Args[2:])

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Get vendor name from args
		if len(args) < 1 {
			callback.ShowError("Usage", "git-vendor diff <vendor>")
			os.Exit(1)
		}
		vendorName := args[0]

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized)
			os.Exit(1)
		}

		// Get diff for vendor
		diffs, err := manager.DiffVendor(vendorName)
		if err != nil {
			callback.ShowError("Diff Failed", err.Error())
			os.Exit(1)
		}

		if len(diffs) == 0 {
			callback.ShowWarning("No Diffs", fmt.Sprintf("No locked versions found for vendor '%s'", vendorName))
			os.Exit(0)
		}

		// Display diffs
		for i := range diffs {
			fmt.Print(core.FormatDiffOutput(&diffs[i]))
			fmt.Println()
		}

	case "watch":
		// Parse common flags
		flags, _ := parseCommonFlags(os.Args[2:])

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

		// Watch for config changes and auto-sync
		err := manager.WatchConfig(func() error {
			return manager.Sync()
		})

		if err != nil {
			callback.ShowError("Watch Failed", err.Error())
			os.Exit(1)
		}

	default:
		tui.PrintError("Unknown Command", fmt.Sprintf("'%s' is not a valid git-vendor command", command))
		fmt.Println()
		tui.PrintHelp()
		os.Exit(1)
	}
}
