package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateBashCompletion(t *testing.T) {
	script := GenerateBashCompletion()

	// Verify bash header
	if !strings.Contains(script, "# bash completion for git-vendor") {
		t.Error("Expected bash completion header")
	}

	// Verify function name
	if !strings.Contains(script, "_git_vendor_completions()") {
		t.Error("Expected bash completion function")
	}

	// Verify complete command
	if !strings.Contains(script, "complete -F _git_vendor_completions git-vendor") {
		t.Error("Expected bash complete registration")
	}

	// Verify all commands are included
	for _, cmd := range commands {
		if !strings.Contains(script, cmd) {
			t.Errorf("Expected command '%s' in bash completion", cmd)
		}
	}

	// Verify sync flags
	if !strings.Contains(script, "--dry-run") {
		t.Error("Expected --dry-run flag for sync command")
	}
	if !strings.Contains(script, "--force") {
		t.Error("Expected --force flag for sync command")
	}
	if !strings.Contains(script, "--parallel") {
		t.Error("Expected --parallel flag for sync command")
	}

	// Verify update flags
	if !strings.Contains(script, "update)") {
		t.Error("Expected update command case")
	}

	// Verify completion shells
	if !strings.Contains(script, "bash zsh fish powershell") {
		t.Error("Expected completion shell options")
	}
}

func TestGenerateZshCompletion(t *testing.T) {
	script := GenerateZshCompletion()

	// Verify zsh header
	if !strings.Contains(script, "#compdef git-vendor") {
		t.Error("Expected zsh compdef header")
	}

	// Verify function name
	if !strings.Contains(script, "_git_vendor()") {
		t.Error("Expected zsh completion function")
	}

	// Verify _describe command
	if !strings.Contains(script, "_describe 'command' commands") {
		t.Error("Expected zsh _describe command")
	}

	// Verify all commands with descriptions are included
	for _, cmd := range commands {
		desc := getCommandDescription(cmd)
		if desc == "" {
			continue
		}
		expected := cmd + ":" + desc
		if !strings.Contains(script, expected) {
			t.Errorf("Expected command '%s' with description '%s' in zsh completion", cmd, desc)
		}
	}

	// Verify sync command flags
	if !strings.Contains(script, "--dry-run[Preview without changes]") {
		t.Error("Expected --dry-run flag with description")
	}
	if !strings.Contains(script, "--force[Re-download even if synced]") {
		t.Error("Expected --force flag with description")
	}
	if !strings.Contains(script, "--no-cache[Skip incremental cache]") {
		t.Error("Expected --no-cache flag with description")
	}
	if !strings.Contains(script, "--parallel[Enable parallel processing]") {
		t.Error("Expected --parallel flag with description")
	}

	// Verify update command flags
	if !strings.Contains(script, "update)") {
		t.Error("Expected update command case")
	}

	// Verify completion shell options
	if !strings.Contains(script, "1:shell:(bash zsh fish powershell)") {
		t.Error("Expected completion shell options")
	}
}

func TestGenerateFishCompletion(t *testing.T) {
	script := GenerateFishCompletion()

	// Verify fish completion syntax
	if !strings.Contains(script, "complete -c git-vendor") {
		t.Error("Expected fish completion syntax")
	}

	// Verify subcommand check
	if !strings.Contains(script, "__fish_use_subcommand") {
		t.Error("Expected fish subcommand check")
	}

	// Verify all commands with descriptions are included
	for _, cmd := range commands {
		desc := getCommandDescription(cmd)
		if desc == "" {
			continue
		}
		// Fish format: complete -c git-vendor -f -n '__fish_use_subcommand' -a 'cmd' -d 'description'
		if !strings.Contains(script, fmt.Sprintf("-a '%s'", cmd)) {
			t.Errorf("Expected command '%s' in fish completion", cmd)
		}
		if !strings.Contains(script, desc) {
			t.Errorf("Expected description '%s' in fish completion", desc)
		}
	}

	// Verify sync command flags
	if !strings.Contains(script, "__fish_seen_subcommand_from sync") {
		t.Error("Expected sync subcommand check")
	}
	if !strings.Contains(script, "-l dry-run -d 'Preview without changes'") {
		t.Error("Expected --dry-run flag with description")
	}
	if !strings.Contains(script, "-l force -d 'Re-download even if synced'") {
		t.Error("Expected --force flag with description")
	}
	if !strings.Contains(script, "-l parallel -d 'Enable parallel processing'") {
		t.Error("Expected --parallel flag with description")
	}

	// Verify update command flags
	if !strings.Contains(script, "__fish_seen_subcommand_from update") {
		t.Error("Expected update subcommand check")
	}

	// Verify completion shells
	if !strings.Contains(script, "__fish_seen_subcommand_from completion") {
		t.Error("Expected completion subcommand check")
	}
	if !strings.Contains(script, "-a 'bash zsh fish powershell'") {
		t.Error("Expected completion shell options")
	}
}

func TestGeneratePowerShellCompletion(t *testing.T) {
	script := GeneratePowerShellCompletion()

	// Verify PowerShell header
	if !strings.Contains(script, "# PowerShell completion for git-vendor") {
		t.Error("Expected PowerShell completion header")
	}

	// Verify Register-ArgumentCompleter
	if !strings.Contains(script, "Register-ArgumentCompleter -Native -CommandName git-vendor") {
		t.Error("Expected PowerShell argument completer registration")
	}

	// Verify script block
	if !strings.Contains(script, "ScriptBlock") {
		t.Error("Expected PowerShell script block")
	}

	// Verify all commands are included
	for _, cmd := range commands {
		expected := fmt.Sprintf("'%s'", cmd)
		if !strings.Contains(script, expected) {
			t.Errorf("Expected command '%s' in PowerShell completion", cmd)
		}
	}

	// Verify sync command flags
	if !strings.Contains(script, "'sync'") {
		t.Error("Expected sync command switch case")
	}
	if !strings.Contains(script, "'--dry-run'") {
		t.Error("Expected --dry-run flag")
	}
	if !strings.Contains(script, "'--force'") {
		t.Error("Expected --force flag")
	}
	if !strings.Contains(script, "'--parallel'") {
		t.Error("Expected --parallel flag")
	}

	// Verify update command flags
	if !strings.Contains(script, "'update'") {
		t.Error("Expected update command switch case")
	}

	// Verify completion shells
	if !strings.Contains(script, "'completion'") {
		t.Error("Expected completion command switch case")
	}
	if !strings.Contains(script, "'bash', 'zsh', 'fish', 'powershell'") {
		t.Error("Expected completion shell options")
	}

	// Verify CompletionResult syntax
	if !strings.Contains(script, "CompletionResult") {
		t.Error("Expected PowerShell CompletionResult")
	}
}

func TestGetCommandDescription(t *testing.T) {
	tests := []struct {
		command     string
		expectDesc  bool
		description string
	}{
		{"init", true, "Initialize vendor directory"},
		{"add", true, "Add vendor dependency"},
		{"edit", true, "Edit vendor configuration"},
		{"remove", true, "Remove vendor dependency"},
		{"list", true, "List all vendors"},
		{"sync", true, "Sync at locked versions (DEPRECATED: use pull --locked)"},
		{"update", true, "Update lockfile (DEPRECATED: use pull)"},
		{"pull", true, "Fetch and sync vendor dependencies"},
		{"validate", true, "Validate config and check conflicts"},
		{"status", true, "Show unified verify + outdated status"},
		{"verify", true, "Verify file hashes (DEPRECATED: use status --offline)"},
		{"outdated", true, "Check staleness (DEPRECATED: use status --remote-only)"},
		{"check-updates", true, "Check for available updates"},
		{"diff", true, "Show commit differences (DEPRECATED: use status)"},
		{"watch", true, "Watch for config changes"},
		{"completion", true, "Generate shell completion script"},
		{"help", true, "Show help information"},
		{"create", true, "Create vendor (non-interactive)"},
		{"delete", true, "Delete vendor (alias for remove)"},
		{"rename", true, "Rename a vendor"},
		{"add-mapping", true, "Add path mapping to vendor"},
		{"remove-mapping", true, "Remove path mapping from vendor"},
		{"list-mappings", true, "List path mappings for vendor"},
		{"update-mapping", true, "Update path mapping destination"},
		{"show", true, "Show vendor details"},
		{"check", true, "Check vendor sync status"},
		{"preview", true, "Preview what would be synced"},
		{"config", true, "Get or set configuration values"},
		{"nonexistent", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := getCommandDescription(tt.command)
			if tt.expectDesc {
				if result != tt.description {
					t.Errorf("Expected description '%s', got '%s'", tt.description, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected empty description for unknown command, got '%s'", result)
				}
			}
		})
	}
}

func TestAllCommandsHaveDescriptions(t *testing.T) {
	// Verify all commands have descriptions
	for _, cmd := range commands {
		desc := getCommandDescription(cmd)
		if desc == "" {
			t.Errorf("Command '%s' is missing a description", cmd)
		}
	}
}

func TestBashCompletion_ContainsAllSyncFlags(t *testing.T) {
	script := GenerateBashCompletion()
	syncFlags := []string{"--dry-run", "--force", "--no-cache", "--group", "--parallel", "--workers", "--verbose", "-v"}

	for _, flag := range syncFlags {
		if !strings.Contains(script, flag) {
			t.Errorf("Expected sync flag '%s' in bash completion", flag)
		}
	}
}

func TestZshCompletion_ContainsAllSyncFlags(t *testing.T) {
	script := GenerateZshCompletion()
	syncFlags := []string{
		"--dry-run[Preview without changes]",
		"--force[Re-download even if synced]",
		"--no-cache[Skip incremental cache]",
		"--group[Sync vendor group]",
		"--parallel[Enable parallel processing]",
		"--workers[Number of parallel workers]",
		"--verbose[Show git commands]",
		"-v[Show git commands]",
	}

	for _, flag := range syncFlags {
		if !strings.Contains(script, flag) {
			t.Errorf("Expected sync flag '%s' in zsh completion", flag)
		}
	}
}

func TestFishCompletion_ContainsAllSyncFlags(t *testing.T) {
	script := GenerateFishCompletion()
	syncFlags := []string{
		"-l dry-run",
		"-l force",
		"-l no-cache",
		"-l group",
		"-l parallel",
		"-l workers",
		"-l verbose -s v",
	}

	for _, flag := range syncFlags {
		if !strings.Contains(script, flag) {
			t.Errorf("Expected sync flag '%s' in fish completion", flag)
		}
	}
}

func TestDeprecatedCommandDescriptions(t *testing.T) {
	for cmd, notice := range DeprecatedCommands {
		desc := getCommandDescription(cmd)
		if desc == "" {
			t.Errorf("deprecated command %q has no description", cmd)
		}
		if !strings.Contains(desc, "DEPRECATED") {
			t.Errorf("description for deprecated command %q should contain 'DEPRECATED', got: %q", cmd, desc)
		}
		_ = notice // DeprecatedCommands entries used by shell completion
	}
}

func TestPullCommandInCompletions(t *testing.T) {
	bash := GenerateBashCompletion()
	if !strings.Contains(bash, "pull") {
		t.Error("Expected 'pull' in bash completion commands")
	}
	if !strings.Contains(bash, "--locked") {
		t.Error("Expected --locked flag in bash completion")
	}

	zsh := GenerateZshCompletion()
	if !strings.Contains(zsh, "pull") {
		t.Error("Expected 'pull' in zsh completion commands")
	}
	if !strings.Contains(zsh, "--locked[Use existing lock, skip fetch]") {
		t.Error("Expected --locked flag with description in zsh completion")
	}

	fish := GenerateFishCompletion()
	if !strings.Contains(fish, "__fish_seen_subcommand_from pull") {
		t.Error("Expected pull subcommand check in fish completion")
	}
	if !strings.Contains(fish, "-l locked -d 'Use existing lock, skip fetch'") {
		t.Error("Expected --locked flag in fish completion")
	}

	ps := GeneratePowerShellCompletion()
	if !strings.Contains(ps, "'pull'") {
		t.Error("Expected 'pull' in PowerShell completion")
	}
	if !strings.Contains(ps, "'--locked'") {
		t.Error("Expected --locked flag in PowerShell completion")
	}
}

func TestPowerShellCompletion_ContainsAllSyncFlags(t *testing.T) {
	script := GeneratePowerShellCompletion()
	syncFlags := []string{"'--dry-run'", "'--force'", "'--no-cache'", "'--group'", "'--parallel'", "'--workers'", "'--verbose'", "'-v'"}

	for _, flag := range syncFlags {
		if !strings.Contains(script, flag) {
			t.Errorf("Expected sync flag '%s' in PowerShell completion", flag)
		}
	}
}
