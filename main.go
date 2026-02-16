// Package main implements the git-vendor CLI tool for managing vendored dependencies from Git repositories.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/EmundoT/git-vendor/cmd"
	"github.com/EmundoT/git-vendor/internal/core"
	"github.com/EmundoT/git-vendor/internal/tui"
	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/EmundoT/git-vendor/internal/version"
)

// Version information is managed in internal/version package
// GoReleaser injects version info directly via ldflags

// minLen returns the smaller of a and b. Used for safe hash truncation in display output.
func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatShortDate formats an RFC3339 timestamp to just the date portion
func formatShortDate(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}
	return timestamp
}

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
		flags, _ := parseCommonFlags(os.Args[2:])

		if err := manager.Init(); err != nil {
			if flags.Mode == core.OutputJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(core.JSONOutput{
					Status: "error",
					Error:  &core.JSONError{Title: "Initialization Failed", Message: err.Error()},
				})
			} else {
				tui.PrintError("Initialization Failed", err.Error())
			}
			os.Exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		originURL := manager.GetRemoteURL(ctx, "origin")

		// Detect ecosystem state for bootstrap suggestions.
		_, hasHooks := os.Stat(".githooks")
		_, hasPolicy := os.Stat(core.PolicyFile)

		switch flags.Mode {
		case core.OutputJSON:
			data := map[string]interface{}{
				"vendor_dir": core.VendorDir,
				"has_hooks":  hasHooks == nil,
				"has_policy": hasPolicy == nil,
			}
			if originURL != "" {
				data["origin_url"] = originURL
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(core.JSONOutput{
				Status:  "success",
				Message: "Initialized in ./" + core.VendorDir + "/",
				Data:    data,
			})
		case core.OutputQuiet:
			// No output
		default:
			tui.PrintInitSummary(tui.InitSummary{
				VendorDir: core.VendorDir,
				OriginURL: originURL,
				HasHooks:  hasHooks == nil,
				HasPolicy: hasPolicy == nil,
			})
		}

	case "add":
		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
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
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
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
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		cfg, err := manager.GetConfig()
		if err != nil {
			callback.ShowError("Error", err.Error())
			os.Exit(1)
		}

		// Load lockfile to get metadata (best effort)
		lock, _ := manager.GetLock() //nolint:errcheck
		lockMap := make(map[string]types.LockDetails)
		for i := range lock.Vendors {
			entry := &lock.Vendors[i]
			key := entry.Name + "@" + entry.Ref
			lockMap[key] = *entry
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
					specData := map[string]interface{}{
						"ref":      s.Ref,
						"mappings": mappingsData,
					}
					// Add lockfile metadata if available
					if entry, ok := lockMap[v.Name+"@"+s.Ref]; ok {
						specData["commit_hash"] = entry.CommitHash
						specData["license_spdx"] = entry.LicenseSPDX
						specData["source_version_tag"] = entry.SourceVersionTag
						specData["vendored_at"] = entry.VendoredAt
						specData["vendored_by"] = entry.VendoredBy
						specData["last_synced_at"] = entry.LastSyncedAt
					}
					specsData = append(specsData, specData)
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
			fmt.Println(tui.StyleTitle("Vendored Dependencies:"))
			fmt.Println()

			for _, v := range cfg.Vendors {
				// Show conflict indicator
				conflictIndicator := ""
				if conflictMap[v.Name] {
					conflictIndicator = " ⚠"
				}

				fmt.Printf("  %s%s\n", v.Name, conflictIndicator)
				fmt.Printf("    URL:      %s\n", v.URL)

				for _, s := range v.Specs {
					// Get lock entry for this ref
					key := v.Name + "@" + s.Ref
					entry, hasLock := lockMap[key]

					if hasLock {
						fmt.Printf("    Ref:      %s @ %s\n", s.Ref, entry.CommitHash[:7])
						if entry.SourceVersionTag != "" {
							fmt.Printf("    Version:  %s\n", entry.SourceVersionTag)
						}
						if entry.LicenseSPDX != "" {
							fmt.Printf("    License:  %s\n", entry.LicenseSPDX)
						} else if v.License != "" {
							fmt.Printf("    License:  %s\n", v.License)
						}
						if entry.VendoredAt != "" {
							vendoredInfo := formatShortDate(entry.VendoredAt)
							if entry.VendoredBy != "" {
								vendoredInfo += " by " + entry.VendoredBy
							}
							fmt.Printf("    Vendored: %s\n", vendoredInfo)
						}
						if entry.LastSyncedAt != "" {
							fmt.Printf("    Synced:   %s\n", formatShortDate(entry.LastSyncedAt))
						}
					} else {
						fmt.Printf("    Ref:      %s (not synced)\n", s.Ref)
						if v.License != "" {
							fmt.Printf("    License:  %s\n", v.License)
						}
					}

					// Show mappings
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
		commit := false
		internalOnly := false
		reverse := false
		local := false

		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case arg == "--dry-run":
				dryRun = true
			case arg == "--force":
				force = true
			case arg == "--no-cache":
				noCache = true
			case arg == "--commit":
				commit = true
			case arg == "--parallel":
				parallel = true
			case arg == "--internal":
				internalOnly = true
			case arg == "--reverse":
				reverse = true
			case arg == "--local":
				local = true
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

		// --reverse requires --internal
		if reverse && !internalOnly {
			callback.ShowError("Invalid Options", "--reverse requires --internal")
			os.Exit(1)
		}

		// Validate that vendor name and group are not both specified
		if vendorName != "" && groupName != "" {
			callback.ShowError("Invalid Options", "Cannot specify both vendor name and --group")
			os.Exit(1)
		}

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Guard: --commit is incompatible with --dry-run
		if commit && dryRun {
			fmt.Println("Warning: --commit ignored during --dry-run")
			commit = false
		}

		// Create signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		if dryRun {
			if err := manager.SyncDryRun(ctx); err != nil {
				callback.ShowError("Preview Failed", err.Error())
				os.Exit(1)
			}
			if flags.Mode != core.OutputQuiet {
				fmt.Println("This is a dry-run. No files were modified.")
				fmt.Println("Run 'git-vendor sync' to apply changes.")
			}
		} else {
			opts := core.SyncOptions{
				VendorName:   vendorName,
				GroupName:    groupName,
				Force:        force,
				NoCache:      noCache,
				InternalOnly: internalOnly,
				Reverse:      reverse,
				Local:        local,
			}
			if parallel {
				opts.Parallel = types.ParallelOptions{
					Enabled:    true,
					MaxWorkers: workers,
				}
			}
			if err := manager.SyncWithFullOptions(ctx, opts); err != nil {
				callback.ShowError("Sync Failed", err.Error())
				os.Exit(1)
			}
			callback.ShowSuccess("Synced.")

			// Auto-commit if --commit flag is set
			if commit {
				if err := manager.CommitVendorChanges("sync", vendorName); err != nil {
					callback.ShowError("Commit Failed", err.Error())
					os.Exit(1)
				}
				callback.ShowSuccess("Committed vendor changes.")
			}
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
		commit := false
		local := false
		vendorName := ""
		groupName := ""

		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case arg == "--verbose" || arg == "-v":
				core.Verbose = true
				manager.UpdateVerboseMode(true)
			case arg == "--commit":
				commit = true
			case arg == "--parallel":
				parallel = true
			case arg == "--local":
				local = true
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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Create signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		opts := core.UpdateOptions{
			Local:      local,
			VendorName: vendorName,
			Group:      groupName,
		}
		if parallel {
			opts.Parallel = types.ParallelOptions{
				Enabled:    true,
				MaxWorkers: workers,
			}
		}
		if err := manager.UpdateAllWithOptions(ctx, opts); err != nil {
			callback.ShowError("Update Failed", err.Error())
			os.Exit(1)
		}

		// Tailor success message based on filter
		switch {
		case vendorName != "":
			callback.ShowSuccess(fmt.Sprintf("Updated vendor '%s'.", vendorName))
		case groupName != "":
			callback.ShowSuccess(fmt.Sprintf("Updated group '%s'.", groupName))
		default:
			callback.ShowSuccess("Updated all vendors.")
		}

		// Auto-commit if --commit flag is set
		if commit {
			if err := manager.CommitVendorChanges("update", vendorName); err != nil {
				callback.ShowError("Commit Failed", err.Error())
				os.Exit(1)
			}
			callback.ShowSuccess("Committed vendor changes.")
		}

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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
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

	case "verify":
		// Parse command-specific flags
		format := "table" // default format
		for _, arg := range os.Args[2:] {
			switch {
			case arg == "--format=json" || arg == "--json":
				format = "json"
			case arg == "--format=table":
				format = "table"
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
			}
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Run verification with signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		result, err := manager.Verify(ctx)
		if err != nil {
			tui.PrintError("Verification Failed", err.Error())
			os.Exit(1)
		}

		// Output results based on format
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			// Table format
			fmt.Println("Verifying vendored dependencies...")
			fmt.Println()

			fileCount := 0
			posCount := 0
			for _, f := range result.Files {
				var symbol, status string
				switch f.Status {
				case "verified":
					symbol = "\u2713" // checkmark
					status = "[OK]"
				case "modified":
					symbol = "\u2717" // x mark
					status = "[MODIFIED]"
				case "added":
					symbol = "?"
					status = "[ADDED]"
				case "deleted":
					symbol = "\u2717" // x mark
					status = "[DELETED]"
				}
				typeLabel := "file"
				if f.Type == "position" {
					typeLabel = "pos "
					posCount++
				} else {
					fileCount++
				}
				fmt.Printf("%s %-8s %-50s %s\n", symbol, typeLabel, f.Path, status)
			}

			fmt.Println()
			fmt.Printf("Summary: %d checked (%s, %s)\n",
				result.Summary.TotalFiles,
				core.Pluralize(fileCount, "file", "files"),
				core.Pluralize(posCount, "position", "positions"))
			fmt.Printf("  \u2713 %d verified\n", result.Summary.Verified)
			if result.Summary.Modified > 0 || result.Summary.Deleted > 0 {
				fmt.Printf("  \u2717 %d errors (%d modified, %d deleted)\n",
					result.Summary.Modified+result.Summary.Deleted,
					result.Summary.Modified, result.Summary.Deleted)
			}
			if result.Summary.Added > 0 {
				fmt.Printf("  ? %d warnings (%d added)\n", result.Summary.Added, result.Summary.Added)
			}
			fmt.Println()
			fmt.Printf("Result: %s\n", result.Summary.Result)
		}

		// Exit code based on result
		// 0=PASS, 1=FAIL, 2=WARN
		switch result.Summary.Result {
		case "PASS":
			os.Exit(0)
		case "WARN":
			os.Exit(2)
		default: // FAIL
			os.Exit(1)
		}

	case "scan":
		// Parse command-specific flags
		format := "table" // default format
		failOn := ""
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--format=json" || arg == "--json":
				format = "json"
			case arg == "--format=table":
				format = "table"
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
			case strings.HasPrefix(arg, "--fail-on="):
				failOn = strings.TrimPrefix(arg, "--fail-on=")
			case arg == "--fail-on":
				if i+1 < len(os.Args) {
					failOn = os.Args[i+1]
					i++
				}
			}
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Run vulnerability scan with signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		result, err := manager.Scan(ctx, failOn)
		if err != nil {
			tui.PrintError("Scan Failed", err.Error())
			os.Exit(1)
		}

		// Output results based on format
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			// Table format
			fmt.Println("Scanning vendored dependencies for vulnerabilities...")
			fmt.Println()

			for _, dep := range result.Dependencies {
				// Show dependency header
				version := dep.Commit
				if len(version) > 7 {
					version = version[:7]
				}
				if dep.Version != nil {
					version = *dep.Version
				}
				fmt.Printf("  %s (%s)\n", dep.Name, version)

				if dep.ScanStatus == "not_scanned" || dep.ScanStatus == "error" {
					fmt.Printf("    ⚠ Unable to scan: %s\n", dep.ScanReason)
					fmt.Println()
					continue
				}

				if len(dep.Vulnerabilities) == 0 {
					fmt.Println("    ✓ No vulnerabilities found")
					fmt.Println()
					continue
				}

				// Show vulnerabilities
				for _, vuln := range dep.Vulnerabilities {
					// Determine symbol based on severity
					symbol := "✗"
					fmt.Printf("    %s %s [%s] %s\n", symbol, vuln.ID, vuln.Severity, vuln.Summary)
					if vuln.FixedVersion != "" {
						fmt.Printf("      Fixed in: %s\n", vuln.FixedVersion)
					}
					// Show first reference URL
					if len(vuln.References) > 0 {
						fmt.Printf("      %s\n", vuln.References[0])
					}
					fmt.Println()
				}
			}

			// Print summary
			fmt.Printf("Summary: %d dependencies scanned\n", result.Summary.TotalDependencies)
			if result.Summary.Vulnerabilities.Total > 0 {
				fmt.Printf("  ✗ %d vulnerabilities found", result.Summary.Vulnerabilities.Total)
				parts := []string{}
				if result.Summary.Vulnerabilities.Critical > 0 {
					parts = append(parts, fmt.Sprintf("%d critical", result.Summary.Vulnerabilities.Critical))
				}
				if result.Summary.Vulnerabilities.High > 0 {
					parts = append(parts, fmt.Sprintf("%d high", result.Summary.Vulnerabilities.High))
				}
				if result.Summary.Vulnerabilities.Medium > 0 {
					parts = append(parts, fmt.Sprintf("%d medium", result.Summary.Vulnerabilities.Medium))
				}
				if result.Summary.Vulnerabilities.Low > 0 {
					parts = append(parts, fmt.Sprintf("%d low", result.Summary.Vulnerabilities.Low))
				}
				if len(parts) > 0 {
					fmt.Printf(" (%s)", strings.Join(parts, ", "))
				}
				fmt.Println()
			} else {
				fmt.Println("  ✓ No vulnerabilities found")
			}
			if result.Summary.NotScanned > 0 {
				fmt.Printf("  ⚠ %d dependencies could not be scanned\n", result.Summary.NotScanned)
			}
			fmt.Println()
			fmt.Printf("Result: %s", result.Summary.Result)
			if result.Summary.ThresholdExceeded {
				fmt.Printf(" (%s vulnerabilities found)", strings.ToLower(failOn))
			}
			fmt.Println()
		}

		// Exit code logic:
		// - Exit 1 if vulnerabilities found AND (no threshold OR threshold exceeded)
		// - Exit 2 if some dependencies couldn't be scanned (WARN)
		// - Exit 0 otherwise (PASS, or vulns below threshold)
		hasVulns := result.Summary.Vulnerabilities.Total > 0
		shouldFailOnVulns := failOn == "" || result.Summary.ThresholdExceeded

		if hasVulns && shouldFailOnVulns {
			os.Exit(1)
		}
		if result.Summary.NotScanned > 0 && !hasVulns {
			os.Exit(2) // WARN only if no vulns (vulns take precedence)
		}
		os.Exit(0)

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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Check sync status
		status, err := manager.CheckSyncStatus()
		if err != nil {
			callback.ShowError("Status Check Failed", err.Error())
			os.Exit(1)
		}

		// Compute aggregate file/position counts
		totalFiles := 0
		totalPositions := 0
		for _, vs := range status.VendorStatuses {
			totalFiles += vs.FileCount
			totalPositions += vs.PositionCount
		}

		if flags.Mode == core.OutputJSON {
			// JSON output mode
			vendorStatusData := make([]map[string]interface{}, 0, len(status.VendorStatuses))
			for _, vs := range status.VendorStatuses {
				vendorStatusData = append(vendorStatusData, map[string]interface{}{
					"name":           vs.Name,
					"ref":            vs.Ref,
					"is_synced":      vs.IsSynced,
					"missing_paths":  vs.MissingPaths,
					"file_count":     vs.FileCount,
					"position_count": vs.PositionCount,
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
					"total_files":     totalFiles,
					"total_positions": totalPositions,
				},
			})

			if !status.AllSynced {
				os.Exit(1)
			}
		} else {
			// Normal output mode
			if status.AllSynced {
				detail := core.Pluralize(totalFiles, "file", "files")
				if totalPositions > 0 {
					detail += ", " + core.Pluralize(totalPositions, "position", "positions")
				}
				callback.ShowSuccess(fmt.Sprintf("All vendors synced (%s)", detail))
			} else {
				// Show which vendors need syncing
				callback.ShowWarning("Vendors Need Syncing", fmt.Sprintf("%s out of sync",
					core.Pluralize(len(status.VendorStatuses)-countSynced(status.VendorStatuses), "vendor", "vendors")))
				fmt.Println()

				for _, vs := range status.VendorStatuses {
					if !vs.IsSynced {
						detail := core.Pluralize(vs.FileCount, "file", "files")
						if vs.PositionCount > 0 {
							detail += ", " + core.Pluralize(vs.PositionCount, "position", "positions")
						}
						fmt.Printf("⚠ %s @ %s (%s)\n", vs.Name, vs.Ref, detail)
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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Check for updates with signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		updates, err := manager.CheckUpdates(ctx)
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
				os.Exit(1)
			}
		}

	case "outdated":
		// Parse flags: --json, --quiet, [vendor-name]
		format := "table"
		vendor := ""
		quiet := false

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--json":
				format = "json"
			case arg == "--quiet" || arg == "-q":
				quiet = true
			case !strings.HasPrefix(arg, "--"):
				vendor = arg
			}
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		result, err := manager.Outdated(ctx, core.OutdatedOptions{Vendor: vendor})
		if err != nil {
			tui.PrintError("Outdated Check Failed", err.Error())
			os.Exit(1)
		}

		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			if !quiet {
				if result.Outdated == 0 && result.Skipped == 0 {
					fmt.Printf("All %s up to date.\n", core.Pluralize(result.TotalChecked, "vendor", "vendors"))
				} else {
					if result.Outdated > 0 {
						fmt.Printf("%s outdated:\n\n", core.Pluralize(result.Outdated, "vendor", "vendors"))
						for _, dep := range result.Dependencies {
							if !dep.UpToDate {
								fmt.Printf("  %s @ %s\n", dep.VendorName, dep.Ref)
								fmt.Printf("    locked:   %s\n", dep.CurrentHash[:minLen(len(dep.CurrentHash), 12)])
								fmt.Printf("    upstream: %s\n", dep.LatestHash[:minLen(len(dep.LatestHash), 12)])
								fmt.Println()
							}
						}
					}
					if result.Skipped > 0 {
						fmt.Printf("%s skipped (network error or not yet synced)\n", core.Pluralize(result.Skipped, "vendor", "vendors"))
					}
					fmt.Println("\nRun 'git-vendor update' to fetch latest versions.")
				}
			}
		}

		if result.Outdated > 0 {
			os.Exit(1)
		}

	case "annotate":
		// Retroactively attach vendor metadata as a git note to an existing commit
		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		commitHash := ""
		vendorFilter := ""
		for i := 0; i < len(os.Args[2:]); i++ {
			arg := os.Args[2+i]
			switch {
			case arg == "--commit":
				if i+1 < len(os.Args[2:]) {
					commitHash = os.Args[2+i+1]
					i++
				} else {
					tui.PrintError("Invalid Flag", "--commit requires a commit hash")
					os.Exit(1)
				}
			case arg == "--vendor":
				if i+1 < len(os.Args[2:]) {
					vendorFilter = os.Args[2+i+1]
					i++
				} else {
					tui.PrintError("Invalid Flag", "--vendor requires a vendor name")
					os.Exit(1)
				}
			default:
				// First positional arg is commit hash
				if commitHash == "" {
					commitHash = arg
				}
			}
		}

		if err := manager.AnnotateVendorCommit(commitHash, vendorFilter); err != nil {
			tui.PrintError("Annotate Failed", err.Error())
			os.Exit(1)
		}
		tui.PrintSuccess("Vendor metadata attached as git note.")

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
		// Parse common flags and diff-specific flags
		flags, args := parseCommonFlags(os.Args[2:])

		// Create appropriate callback
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Parse diff-specific flags from remaining args
		diffOpts := core.DiffOptions{}
		var filteredArgs []string
		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch {
			case strings.HasPrefix(arg, "--ref="):
				diffOpts.Ref = strings.TrimPrefix(arg, "--ref=")
			case arg == "--ref" && i+1 < len(args):
				i++
				diffOpts.Ref = args[i]
			case strings.HasPrefix(arg, "--group="):
				diffOpts.Group = strings.TrimPrefix(arg, "--group=")
			case arg == "--group" && i+1 < len(args):
				i++
				diffOpts.Group = args[i]
			default:
				filteredArgs = append(filteredArgs, arg)
			}
		}

		// Vendor name is optional positional arg (required when no --group)
		if len(filteredArgs) > 0 {
			diffOpts.VendorName = filteredArgs[0]
		} else if diffOpts.Group == "" {
			callback.ShowError("Usage", "git-vendor diff <vendor> [--ref <ref>] [--group <group>]")
			os.Exit(1)
		}

		if !core.IsVendorInitialized() {
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Get diff with options
		diffs, err := manager.DiffVendorWithOptions(diffOpts)
		if err != nil {
			callback.ShowError("Diff Failed", err.Error())
			os.Exit(1)
		}

		if len(diffs) == 0 {
			label := diffOpts.VendorName
			if label == "" {
				label = "group '" + diffOpts.Group + "'"
			} else {
				label = "vendor '" + label + "'"
			}
			callback.ShowWarning("No Diffs", fmt.Sprintf("No locked versions found for %s", label))
			os.Exit(0)
		}

		// Display diffs
		for i := range diffs {
			fmt.Print(core.FormatDiffOutput(&diffs[i]))
			fmt.Println()
		}

	case "drift":
		// Parse command-specific flags
		format := "table"
		dependency := ""
		offline := false
		detail := false

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--format=json" || arg == "--json":
				format = "json"
			case arg == "--format=table":
				format = "table"
			case arg == "--format=detail":
				format = "detail"
				detail = true
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
				if format == "detail" {
					detail = true
				}
			case arg == "--detail":
				detail = true
			case arg == "--offline":
				offline = true
			case strings.HasPrefix(arg, "--dependency="):
				dependency = strings.TrimPrefix(arg, "--dependency=")
			case arg == "--dependency":
				if i+1 < len(os.Args) {
					dependency = os.Args[i+1]
					i++
				} else {
					tui.PrintError("Invalid Flag", "--dependency requires a vendor name")
					os.Exit(1)
				}
			case !strings.HasPrefix(arg, "--"):
				dependency = arg
			}
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Run drift detection with signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		result, err := manager.Drift(ctx, core.DriftOptions{
			Dependency: dependency,
			Offline:    offline,
			Detail:     detail,
		})
		if err != nil {
			tui.PrintError("Drift Detection Failed", err.Error())
			os.Exit(1)
		}

		// Output results based on format
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			// Table/detail format
			fmt.Println("Analyzing drift for vendored dependencies...")
			fmt.Println()

			for i := range result.Dependencies {
				fmt.Print(core.FormatDriftOutput(&result.Dependencies[i], offline))
				fmt.Println()
			}

			// Summary
			fmt.Printf("Summary: %d dependencies analyzed\n", result.Summary.TotalDependencies)
			if result.Summary.Clean > 0 {
				fmt.Printf("  \u2713 %s with no drift\n", core.Pluralize(result.Summary.Clean, "dependency", "dependencies"))
			}
			if result.Summary.DriftedLocal > 0 {
				fmt.Printf("  \u0394 %s with local drift\n", core.Pluralize(result.Summary.DriftedLocal, "dependency", "dependencies"))
			}
			if result.Summary.DriftedUpstream > 0 {
				fmt.Printf("  \u2191 %s with upstream drift\n", core.Pluralize(result.Summary.DriftedUpstream, "dependency", "dependencies"))
			}
			if result.Summary.ConflictRisk > 0 {
				fmt.Printf("  \u26A0 %s with conflict risk\n", core.Pluralize(result.Summary.ConflictRisk, "dependency", "dependencies"))
			}
			fmt.Printf("  Overall drift score: %.0f%%\n", result.Summary.OverallDriftScore)
			fmt.Println()
			fmt.Printf("Result: %s\n", result.Summary.Result)
		}

		// Exit codes: 0=CLEAN, 1=DRIFTED/CONFLICT
		switch result.Summary.Result {
		case "CLEAN":
			os.Exit(0)
		default:
			os.Exit(1)
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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Watch for config changes and auto-sync.
		// Each sync invocation creates its own context for cancellation.
		err := manager.WatchConfig(func() error {
			return manager.Sync(context.Background())
		})

		if err != nil {
			callback.ShowError("Watch Failed", err.Error())
			os.Exit(1)
		}

	case "migrate":
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
			callback.ShowError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Migrate lockfile to add missing metadata fields
		migrated, err := manager.MigrateLockfile()
		if err != nil {
			callback.ShowError("Migration Failed", err.Error())
			os.Exit(1)
		}

		switch {
		case flags.Mode == core.OutputJSON:
			_ = callback.FormatJSON(core.JSONOutput{
				Status:  "success",
				Message: fmt.Sprintf("Migrated %d entries", migrated),
				Data: map[string]interface{}{
					"migrated_entries": migrated,
				},
			})
		case migrated > 0:
			callback.ShowSuccess(fmt.Sprintf("Migrated %s to schema v1.1", core.Pluralize(migrated, "entry", "entries")))
			fmt.Println()
			fmt.Println("The following metadata was added:")
			fmt.Println("  • license_spdx: from vendor.yml license field")
			fmt.Println("  • vendored_at: approximated from updated timestamp")
			fmt.Println("  • vendored_by: set to 'unknown (migrated)'")
			fmt.Println("  • last_synced_at: from updated timestamp")
			fmt.Println()
			fmt.Println("Run 'git-vendor update' to fetch source_version_tag for tagged releases.")
		default:
			callback.ShowSuccess("Lockfile already up to date - no migration needed")
		}

	case "sbom":
		// Parse command-specific flags
		format := "cyclonedx" // default format
		outputFile := ""
		validate := false
		showHelp := false

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--help" || arg == "-h":
				showHelp = true
			case arg == "--validate":
				validate = true
			case arg == "--format" && i+1 < len(os.Args):
				format = os.Args[i+1]
				i++
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
			case arg == "--output" && i+1 < len(os.Args):
				outputFile = os.Args[i+1]
				i++
			case strings.HasPrefix(arg, "--output="):
				outputFile = strings.TrimPrefix(arg, "--output=")
			case arg == "-o" && i+1 < len(os.Args):
				outputFile = os.Args[i+1]
				i++
			}
		}

		// Show help if requested
		if showHelp {
			fmt.Println("Generate Software Bill of Materials (SBOM) from vendored dependencies")
			fmt.Println()
			fmt.Println("Usage: git-vendor sbom [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --format <fmt>   Output format: cyclonedx (default) or spdx")
			fmt.Println("  --output <file>  Write to file instead of stdout")
			fmt.Println("  -o <file>        Shorthand for --output")
			fmt.Println("  --validate       Validate generated SBOM against schema")
			fmt.Println("  --help, -h       Show this help message")
			fmt.Println()
			fmt.Println("Formats:")
			fmt.Println("  cyclonedx   CycloneDX 1.5 JSON - security-focused, widely supported by scanners")
			fmt.Println("  spdx        SPDX 2.3 JSON - compliance-focused for license analysis")
			fmt.Println()
			fmt.Println("Examples:")
			fmt.Println("  git-vendor sbom                          # Output CycloneDX to stdout")
			fmt.Println("  git-vendor sbom --format spdx            # Output SPDX to stdout")
			fmt.Println("  git-vendor sbom -o sbom.json             # Write CycloneDX to file")
			fmt.Println("  git-vendor sbom --format spdx --validate # Generate and validate SPDX")
			os.Exit(0)
		}

		// Validate format
		var sbomFormat core.SBOMFormat
		switch format {
		case "cyclonedx":
			sbomFormat = core.SBOMFormatCycloneDX
		case "spdx":
			sbomFormat = core.SBOMFormatSPDX
		default:
			tui.PrintError("Invalid Format", fmt.Sprintf("'%s' is not a valid SBOM format. Use 'cyclonedx' or 'spdx'", format))
			os.Exit(1)
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Determine project name from current directory
		projectName := "unknown-project"
		if cwd, err := os.Getwd(); err == nil {
			parts := strings.Split(cwd, string(os.PathSeparator))
			if len(parts) > 0 {
				projectName = parts[len(parts)-1]
			}
		}

		// Generate SBOM with options
		opts := core.SBOMOptions{
			ProjectName: projectName,
			Validate:    validate,
		}
		generator := core.NewSBOMGeneratorWithOptions(
			core.NewFileLockStore(core.VendorDir),
			core.NewFileConfigStore(core.VendorDir),
			opts,
		)
		output, err := generator.Generate(sbomFormat)
		if err != nil {
			tui.PrintError("SBOM Generation Failed", err.Error())
			os.Exit(1)
		}

		// Write output
		if outputFile != "" {
			if err := os.WriteFile(outputFile, output, 0644); err != nil {
				tui.PrintError("Write Failed", err.Error())
				os.Exit(1)
			}
			tui.PrintSuccess(fmt.Sprintf("SBOM written to %s", outputFile))
		} else {
			// Write to stdout
			fmt.Print(string(output))
		}

	case "license":
		// Parse command-specific flags
		format := "table" // default format
		failOn := "deny"  // default: only denied licenses cause FAIL
		policyPath := ""  // empty = default PolicyFile location

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--format=json" || arg == "--json":
				format = "json"
			case arg == "--format=table":
				format = "table"
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
			case strings.HasPrefix(arg, "--fail-on="):
				failOn = strings.TrimPrefix(arg, "--fail-on=")
			case arg == "--fail-on":
				if i+1 < len(os.Args) {
					failOn = os.Args[i+1]
					i++
				}
			case strings.HasPrefix(arg, "--policy="):
				policyPath = strings.TrimPrefix(arg, "--policy=")
			case arg == "--policy":
				if i+1 < len(os.Args) {
					policyPath = os.Args[i+1]
					i++
				}
			}
		}

		// Validate --fail-on value
		switch failOn {
		case "deny", "warn":
			// valid
		default:
			tui.PrintError("Invalid Flag", fmt.Sprintf("--fail-on must be 'deny' or 'warn', got '%s'", failOn))
			os.Exit(1)
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Run license compliance report
		result, err := manager.LicenseReport(policyPath, failOn)
		if err != nil {
			tui.PrintError("License Report Failed", err.Error())
			os.Exit(1)
		}

		// Output results based on format
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			// Table format
			fmt.Println("License compliance report")
			fmt.Printf("Policy: %s\n", result.PolicyFile)
			fmt.Println()

			for _, vendor := range result.Vendors {
				symbol := "✓"
				switch vendor.Decision {
				case "deny":
					symbol = "✗"
				case "warn":
					symbol = "⚠"
				}
				fmt.Printf("  %s %s  %s  [%s]\n", symbol, vendor.Name, vendor.License, vendor.Decision)
				if vendor.Decision != "allow" {
					fmt.Printf("    %s\n", vendor.Reason)
				}
			}

			fmt.Println()
			fmt.Printf("Summary: %d vendors checked\n", result.Summary.TotalVendors)
			if result.Summary.Allowed > 0 {
				fmt.Printf("  ✓ %d allowed\n", result.Summary.Allowed)
			}
			if result.Summary.Warned > 0 {
				fmt.Printf("  ⚠ %d warned\n", result.Summary.Warned)
			}
			if result.Summary.Denied > 0 {
				fmt.Printf("  ✗ %d denied\n", result.Summary.Denied)
			}
			if result.Summary.Unknown > 0 {
				fmt.Printf("  ? %d unknown license\n", result.Summary.Unknown)
			}
			fmt.Println()
			fmt.Printf("Result: %s\n", result.Summary.Result)
		}

		// Exit code logic:
		// - Exit 1 if result is FAIL (denied licenses, or warned when --fail-on=warn)
		// - Exit 2 if result is WARN (warned licenses)
		// - Exit 0 if PASS
		switch result.Summary.Result {
		case "PASS":
			os.Exit(0)
		case "WARN":
			os.Exit(2)
		default: // FAIL
			os.Exit(1)
		}

	case "audit":
		// Parse command-specific flags
		format := "table"
		skipVerify := false
		skipScan := false
		skipLicense := false
		skipDrift := false
		scanFailOn := ""
		licenseFailOn := "deny"
		policyPath := ""
		verbose := false

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--format=json" || arg == "--json":
				format = "json"
			case arg == "--format=table":
				format = "table"
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
			case arg == "--skip-verify":
				skipVerify = true
			case arg == "--skip-scan":
				skipScan = true
			case arg == "--skip-license":
				skipLicense = true
			case arg == "--skip-drift":
				skipDrift = true
			case strings.HasPrefix(arg, "--fail-on="):
				scanFailOn = strings.TrimPrefix(arg, "--fail-on=")
			case arg == "--fail-on":
				if i+1 < len(os.Args) {
					scanFailOn = os.Args[i+1]
					i++
				}
			case strings.HasPrefix(arg, "--license-fail-on="):
				licenseFailOn = strings.TrimPrefix(arg, "--license-fail-on=")
			case arg == "--license-fail-on":
				if i+1 < len(os.Args) {
					licenseFailOn = os.Args[i+1]
					i++
				}
			case strings.HasPrefix(arg, "--policy="):
				policyPath = strings.TrimPrefix(arg, "--policy=")
			case arg == "--policy":
				if i+1 < len(os.Args) {
					policyPath = os.Args[i+1]
					i++
				}
			case arg == "--verbose" || arg == "-v":
				verbose = true
			}
		}

		if verbose {
			core.Verbose = true
			manager.UpdateVerboseMode(true)
		}

		if !core.IsVendorInitialized() {
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(1)
		}

		// Run unified audit with signal-aware context for Ctrl+C cancellation
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		auditResult, err := manager.RunAudit(ctx, core.AuditOptions{
			SkipVerify:        skipVerify,
			SkipScan:          skipScan,
			SkipLicense:       skipLicense,
			SkipDrift:         skipDrift,
			ScanFailOn:        scanFailOn,
			LicenseFailOn:     licenseFailOn,
			LicensePolicyPath: policyPath,
		})
		if err != nil {
			tui.PrintError("Audit Failed", err.Error())
			os.Exit(1)
		}

		// Output results based on format
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(auditResult); err != nil {
				tui.PrintError("JSON Output Failed", err.Error())
				os.Exit(1)
			}
		default:
			fmt.Print(core.FormatAuditTable(auditResult))
		}

		// Exit codes: 0=PASS, 1=FAIL, 2=WARN
		switch auditResult.Summary.Result {
		case "PASS":
			os.Exit(0)
		case "WARN":
			os.Exit(2)
		default: // FAIL
			os.Exit(1)
		}

	// =====================================================================
	// LLM-Friendly CLI Commands (Spec 072)
	// =====================================================================

	case "create":
		// Non-interactive vendor creation
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		// Parse command-specific flags
		ref := ""
		license := ""
		var positionalArgs []string

		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--ref" && i+1 < len(args):
				ref = args[i+1]
				i++
			case args[i] == "--license" && i+1 < len(args):
				license = args[i+1]
				i++
			case !strings.HasPrefix(args[i], "--"):
				positionalArgs = append(positionalArgs, args[i])
			}
		}

		if len(positionalArgs) < 2 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor create <name> <url> [--ref <ref>] [--license <license>]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor create <name> <url> [--ref <ref>] [--license <license>]")
			os.Exit(core.ExitInvalidArguments)
		}

		name := positionalArgs[0]
		url := positionalArgs[1]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		if err := manager.CreateVendorEntry(name, url, ref, license); err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				if strings.Contains(err.Error(), "already exists") {
					code = core.ErrCodeVendorExists
				}
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Failed", err.Error())
			os.Exit(core.ExitGeneralError)
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"name":    name,
				"url":     url,
				"ref":     ref,
				"license": license,
			})
		} else {
			tui.PrintSuccess(fmt.Sprintf("Created vendor '%s'", name))
		}

	case "delete":
		// Alias for remove — delegates to existing remove logic
		// Re-parse as if "remove" was called
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		if len(args) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor delete <name>", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor delete <name>")
			os.Exit(core.ExitInvalidArguments)
		}
		name := args[0]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		// Create callback for confirmation
		var callback core.UICallback
		if flags.Yes || flags.Mode != core.OutputNormal {
			callback = tui.NewNonInteractiveTUICallback(flags)
		} else {
			callback = tui.NewTUICallback()
		}
		manager.SetUICallback(callback)

		// Verify vendor exists
		cfg, err := manager.GetConfig()
		if err != nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeConfigError, err.Error(), core.ExitGeneralError))
			}
			callback.ShowError("Error", err.Error())
			os.Exit(core.ExitGeneralError)
		}

		found := false
		for _, v := range cfg.Vendors {
			if v.Name == name {
				found = true
				break
			}
		}
		if !found {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeVendorNotFound, fmt.Sprintf("vendor '%s' not found", name), core.ExitVendorNotFound))
			}
			callback.ShowError("Error", fmt.Sprintf("vendor '%s' not found", name))
			os.Exit(core.ExitVendorNotFound)
		}

		// Confirmation (skipped in JSON/quiet/yes mode)
		confirmed := callback.AskConfirmation(
			fmt.Sprintf("Remove vendor '%s'?", name),
			"This will delete the config entry and license file.",
		)
		if !confirmed {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInternalError, "cancelled", core.ExitGeneralError))
			}
			fmt.Println("Cancelled.")
			os.Exit(core.ExitGeneralError)
		}

		if err := manager.RemoveVendor(name); err != nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInternalError, err.Error(), core.ExitGeneralError))
			}
			callback.ShowError("Error", err.Error())
			os.Exit(core.ExitGeneralError)
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{"name": name, "deleted": true})
		} else {
			callback.ShowSuccess("Removed " + name)
		}

	case "rename":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 2 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor rename <old-name> <new-name>", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor rename <old-name> <new-name>")
			os.Exit(core.ExitInvalidArguments)
		}

		oldName := positionalArgs[0]
		newName := positionalArgs[1]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		if err := manager.RenameVendor(oldName, newName); err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				exitCode := core.CLIExitCodeForError(err)
				if strings.Contains(err.Error(), "already exists") {
					code = core.ErrCodeVendorExists
				}
				os.Exit(core.EmitCLIError(code, err.Error(), exitCode))
			}
			tui.PrintError("Failed", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"old_name": oldName,
				"new_name": newName,
			})
		} else {
			tui.PrintSuccess(fmt.Sprintf("Renamed '%s' → '%s'", oldName, newName))
		}

	case "add-mapping":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		// Parse flags
		to := ""
		ref := ""
		var positionalArgs []string

		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--to" && i+1 < len(args):
				to = args[i+1]
				i++
			case args[i] == "--ref" && i+1 < len(args):
				ref = args[i+1]
				i++
			case !strings.HasPrefix(args[i], "--"):
				positionalArgs = append(positionalArgs, args[i])
			}
		}

		if len(positionalArgs) < 2 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor add-mapping <vendor> <from> --to <to> [--ref <ref>]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor add-mapping <vendor> <from> --to <to> [--ref <ref>]")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]
		from := positionalArgs[1]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		if err := manager.AddMappingToVendor(vendorName, from, to, ref); err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				if strings.Contains(err.Error(), "already exists") {
					code = core.ErrCodeMappingExists
				}
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Failed", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		dest := to
		if dest == "" {
			dest = "(auto)"
		}
		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"vendor": vendorName,
				"from":   from,
				"to":     to,
			})
		} else {
			tui.PrintSuccess(fmt.Sprintf("Added mapping: %s → %s", from, dest))
		}

	case "remove-mapping":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 2 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor remove-mapping <vendor> <from>", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor remove-mapping <vendor> <from>")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]
		from := positionalArgs[1]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		if err := manager.RemoveMappingFromVendor(vendorName, from); err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				if strings.Contains(err.Error(), "not found in vendor") {
					code = core.ErrCodeMappingNotFound
				}
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Failed", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"vendor":  vendorName,
				"removed": from,
			})
		} else {
			tui.PrintSuccess(fmt.Sprintf("Removed mapping: %s", from))
		}

	case "list-mappings":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor list-mappings <vendor> [--json]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor list-mappings <vendor> [--json]")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		cfg, err := manager.GetConfig()
		if err != nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeConfigError, err.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Error", err.Error())
			os.Exit(core.ExitGeneralError)
		}

		vendor := core.FindVendor(cfg.Vendors, vendorName)
		if vendor == nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeVendorNotFound, fmt.Sprintf("vendor '%s' not found", vendorName), core.ExitVendorNotFound))
			}
			tui.PrintError("Error", fmt.Sprintf("vendor '%s' not found", vendorName))
			os.Exit(core.ExitVendorNotFound)
		}

		if jsonMode {
			allMappings := make([]map[string]interface{}, 0)
			for _, s := range vendor.Specs {
				for _, m := range s.Mapping {
					allMappings = append(allMappings, map[string]interface{}{
						"from": m.From,
						"to":   m.To,
						"ref":  s.Ref,
					})
				}
			}
			core.EmitCLISuccess(map[string]interface{}{
				"vendor":   vendorName,
				"mappings": allMappings,
			})
		} else {
			fmt.Printf("Mappings for %s:\n\n", vendorName)
			for _, s := range vendor.Specs {
				if len(s.Mapping) == 0 {
					fmt.Printf("  %s: (no mappings)\n", s.Ref)
					continue
				}
				for _, m := range s.Mapping {
					dest := m.To
					if dest == "" {
						dest = "(auto)"
					}
					fmt.Printf("  [%s] %s → %s\n", s.Ref, m.From, dest)
				}
			}
		}

	case "update-mapping":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		// Parse flags
		newTo := ""
		var positionalArgs []string

		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--to" && i+1 < len(args):
				newTo = args[i+1]
				i++
			case !strings.HasPrefix(args[i], "--"):
				positionalArgs = append(positionalArgs, args[i])
			}
		}

		if len(positionalArgs) < 2 || newTo == "" {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor update-mapping <vendor> <from> --to <new-to>", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor update-mapping <vendor> <from> --to <new-to>")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]
		from := positionalArgs[1]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		// Get old target for display
		oldTo := ""
		cfg, _ := manager.GetConfig() //nolint:errcheck
		if v := core.FindVendor(cfg.Vendors, vendorName); v != nil {
			for _, s := range v.Specs {
				for _, m := range s.Mapping {
					if m.From == from {
						oldTo = m.To
						break
					}
				}
			}
		}

		if err := manager.UpdateMappingInVendor(vendorName, from, newTo); err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				if strings.Contains(err.Error(), "not found") {
					code = core.ErrCodeMappingNotFound
				}
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Failed", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"vendor": vendorName,
				"from":   from,
				"old_to": oldTo,
				"new_to": newTo,
			})
		} else {
			if oldTo != "" {
				tui.PrintSuccess(fmt.Sprintf("Updated mapping target: %s → %s", oldTo, newTo))
			} else {
				tui.PrintSuccess(fmt.Sprintf("Updated mapping target for %s to %s", from, newTo))
			}
		}

	case "show":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor show <vendor> [--json]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor show <vendor> [--json]")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		data, err := manager.ShowVendor(vendorName)
		if err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Error", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		if jsonMode {
			core.EmitCLISuccess(data)
		} else {
			// Human-readable output
			fmt.Printf("  %s\n", data["name"])
			fmt.Printf("    URL:      %s\n", data["url"])
			if license, ok := data["license"]; ok && license != "" {
				fmt.Printf("    License:  %s\n", license)
			}
			if groups, ok := data["groups"]; ok {
				if g, ok := groups.([]string); ok && len(g) > 0 {
					fmt.Printf("    Groups:   %s\n", strings.Join(g, ", "))
				}
			}

			if specs, ok := data["specs"].([]map[string]interface{}); ok {
				for _, spec := range specs {
					refStr := fmt.Sprintf("%v", spec["ref"])
					if hash, ok := spec["commit_hash"]; ok {
						hashStr := fmt.Sprintf("%v", hash)
						if len(hashStr) > 7 {
							hashStr = hashStr[:7]
						}
						fmt.Printf("    Ref:      %s @ %s\n", refStr, hashStr)
					} else {
						fmt.Printf("    Ref:      %s (not synced)\n", refStr)
					}
					if tag, ok := spec["source_version_tag"]; ok && tag != "" {
						fmt.Printf("    Version:  %s\n", tag)
					}
					if mappings, ok := spec["mappings"].([]map[string]interface{}); ok {
						for i, m := range mappings {
							dest := fmt.Sprintf("%v", m["to"])
							if dest == "" {
								dest = "(auto)"
							}
							prefix := "      ├─"
							if i == len(mappings)-1 {
								prefix = "      └─"
							}
							fmt.Printf("%s %s → %s\n", prefix, m["from"], dest)
						}
					}
				}
			}
		}

	case "check":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor check <vendor> [--json]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor check <vendor> [--json]")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		result, err := manager.CheckVendorStatus(vendorName)
		if err != nil {
			if jsonMode {
				code := core.CLIErrorCodeForError(err)
				os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
			}
			tui.PrintError("Error", err.Error())
			os.Exit(core.CLIExitCodeForError(err))
		}

		if jsonMode {
			core.EmitCLISuccess(result)
		} else {
			status := fmt.Sprintf("%v", result["status"])
			switch status {
			case "synced":
				tui.PrintSuccess(fmt.Sprintf("%s: synced", vendorName))
			case "stale":
				tui.PrintWarning("Stale", fmt.Sprintf("%s needs syncing", vendorName))
				if specs, ok := result["specs"].([]map[string]interface{}); ok {
					for _, spec := range specs {
						if spec["status"] != "synced" {
							fmt.Printf("  %s: %s\n", spec["ref"], spec["status"])
							if missing, ok := spec["missing_paths"].([]string); ok {
								for _, p := range missing {
									fmt.Printf("    • Missing: %s\n", p)
								}
							}
						}
					}
				}
			default:
				fmt.Printf("%s: %s\n", vendorName, status)
			}
		}

	case "preview":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		var positionalArgs []string
		for _, a := range args {
			if !strings.HasPrefix(a, "--") {
				positionalArgs = append(positionalArgs, a)
			}
		}

		if len(positionalArgs) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor preview <vendor> [--json]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor preview <vendor> [--json]")
			os.Exit(core.ExitInvalidArguments)
		}

		vendorName := positionalArgs[0]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		cfg, err := manager.GetConfig()
		if err != nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeConfigError, err.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Error", err.Error())
			os.Exit(core.ExitGeneralError)
		}

		vendor := core.FindVendor(cfg.Vendors, vendorName)
		if vendor == nil {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeVendorNotFound, fmt.Sprintf("vendor '%s' not found", vendorName), core.ExitVendorNotFound))
			}
			tui.PrintError("Error", fmt.Sprintf("vendor '%s' not found", vendorName))
			os.Exit(core.ExitVendorNotFound)
		}

		// Build preview data from config (what would be synced)
		lock, _ := manager.GetLock() //nolint:errcheck
		lockMap := make(map[string]string)
		for _, entry := range lock.Vendors {
			key := entry.Name + "@" + entry.Ref
			lockMap[key] = entry.CommitHash
		}

		previewSpecs := make([]map[string]interface{}, 0, len(vendor.Specs))
		totalFiles := 0
		for _, spec := range vendor.Specs {
			specPreview := map[string]interface{}{
				"ref": spec.Ref,
			}
			if hash, ok := lockMap[vendorName+"@"+spec.Ref]; ok {
				specPreview["locked_commit"] = hash
			}

			files := make([]map[string]interface{}, 0, len(spec.Mapping))
			for _, m := range spec.Mapping {
				dest := m.To
				if dest == "" {
					dest = "(auto)"
				}
				files = append(files, map[string]interface{}{
					"from": m.From,
					"to":   dest,
				})
			}
			specPreview["files"] = files
			totalFiles += len(spec.Mapping)
			previewSpecs = append(previewSpecs, specPreview)
		}

		if jsonMode {
			core.EmitCLISuccess(map[string]interface{}{
				"vendor":      vendorName,
				"url":         vendor.URL,
				"specs":       previewSpecs,
				"total_files": totalFiles,
			})
		} else {
			fmt.Printf("Preview for %s (%s):\n\n", vendorName, vendor.URL)
			for _, spec := range previewSpecs {
				refStr := fmt.Sprintf("%v", spec["ref"])
				if hash, ok := spec["locked_commit"]; ok {
					hashStr := fmt.Sprintf("%v", hash)
					if len(hashStr) > 7 {
						hashStr = hashStr[:7]
					}
					fmt.Printf("  %s @ %s\n", refStr, hashStr)
				} else {
					fmt.Printf("  %s (not locked — will fetch latest)\n", refStr)
				}
				if files, ok := spec["files"].([]map[string]interface{}); ok {
					for _, f := range files {
						fmt.Printf("    %s → %s\n", f["from"], f["to"])
					}
				}
			}
			fmt.Printf("\n%s would be synced.\n", core.Pluralize(totalFiles, "file", "files"))
		}

	case "config":
		flags, args := parseCommonFlags(os.Args[2:])
		jsonMode := flags.Mode == core.OutputJSON

		if len(args) < 1 {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor config <get|set|list> [args]", core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", "git-vendor config <get|set|list> [args]")
			os.Exit(core.ExitInvalidArguments)
		}

		subCmd := args[0]
		subArgs := args[1:]

		if !core.IsVendorInitialized() {
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeNotInitialized, core.ErrNotInitialized.Error(), core.ExitGeneralError))
			}
			tui.PrintError("Not Initialized", core.ErrNotInitialized.Error())
			os.Exit(core.ExitGeneralError)
		}

		switch subCmd {
		case "list":
			cfg, err := manager.GetConfig()
			if err != nil {
				if jsonMode {
					os.Exit(core.EmitCLIError(core.ErrCodeConfigError, err.Error(), core.ExitGeneralError))
				}
				tui.PrintError("Error", err.Error())
				os.Exit(core.ExitGeneralError)
			}

			if jsonMode {
				vendorData := make([]map[string]interface{}, 0, len(cfg.Vendors))
				for _, v := range cfg.Vendors {
					vd := map[string]interface{}{
						"name":    v.Name,
						"url":     v.URL,
						"license": v.License,
					}
					if len(v.Groups) > 0 {
						vd["groups"] = v.Groups
					}
					refs := make([]string, 0, len(v.Specs))
					for _, s := range v.Specs {
						refs = append(refs, s.Ref)
					}
					vd["refs"] = refs
					mappingCount := 0
					for _, s := range v.Specs {
						mappingCount += len(s.Mapping)
					}
					vd["mapping_count"] = mappingCount
					vendorData = append(vendorData, vd)
				}
				core.EmitCLISuccess(map[string]interface{}{
					"vendors":      vendorData,
					"vendor_count": len(cfg.Vendors),
				})
			} else {
				if len(cfg.Vendors) == 0 {
					fmt.Println("No vendors configured.")
				} else {
					for _, v := range cfg.Vendors {
						fmt.Printf("vendors.%s.url = %s\n", v.Name, v.URL)
						fmt.Printf("vendors.%s.license = %s\n", v.Name, v.License)
						for _, s := range v.Specs {
							fmt.Printf("vendors.%s.ref = %s\n", v.Name, s.Ref)
						}
						if len(v.Groups) > 0 {
							fmt.Printf("vendors.%s.groups = %s\n", v.Name, strings.Join(v.Groups, ","))
						}
					}
				}
			}

		case "get":
			if len(subArgs) < 1 {
				if jsonMode {
					os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor config get <key>", core.ExitInvalidArguments))
				}
				tui.PrintError("Usage", "git-vendor config get <key>")
				os.Exit(core.ExitInvalidArguments)
			}

			key := subArgs[0]
			value, err := manager.GetConfigValue(key)
			if err != nil {
				if jsonMode {
					code := core.ErrCodeInvalidKey
					if core.IsVendorNotFound(err) {
						code = core.ErrCodeVendorNotFound
					}
					os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
				}
				tui.PrintError("Error", err.Error())
				os.Exit(core.ExitGeneralError)
			}

			if jsonMode {
				core.EmitCLISuccess(map[string]interface{}{
					"key":   key,
					"value": value,
				})
			} else {
				fmt.Printf("%v\n", value)
			}

		case "set":
			if len(subArgs) < 2 {
				if jsonMode {
					os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, "usage: git-vendor config set <key> <value>", core.ExitInvalidArguments))
				}
				tui.PrintError("Usage", "git-vendor config set <key> <value>")
				os.Exit(core.ExitInvalidArguments)
			}

			key := subArgs[0]
			value := subArgs[1]
			if err := manager.SetConfigValue(key, value); err != nil {
				if jsonMode {
					code := core.ErrCodeInvalidKey
					if core.IsVendorNotFound(err) {
						code = core.ErrCodeVendorNotFound
					}
					os.Exit(core.EmitCLIError(code, err.Error(), core.CLIExitCodeForError(err)))
				}
				tui.PrintError("Error", err.Error())
				os.Exit(core.ExitGeneralError)
			}

			if jsonMode {
				core.EmitCLISuccess(map[string]interface{}{
					"key":   key,
					"value": value,
				})
			} else {
				tui.PrintSuccess(fmt.Sprintf("Set %s = %s", key, value))
			}

		default:
			if jsonMode {
				os.Exit(core.EmitCLIError(core.ErrCodeInvalidArguments, fmt.Sprintf("unknown config subcommand: %s (use get, set, or list)", subCmd), core.ExitInvalidArguments))
			}
			tui.PrintError("Usage", fmt.Sprintf("unknown config subcommand: %s\nUsage: git-vendor config <get|set|list>", subCmd))
			os.Exit(core.ExitInvalidArguments)
		}

	default:
		tui.PrintError("Unknown Command", fmt.Sprintf("'%s' is not a valid git-vendor command", command))
		fmt.Println()
		tui.PrintHelp()
		os.Exit(1)
	}
}
