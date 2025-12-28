// Package cmd provides CLI utilities for git-vendor
package cmd

import (
	"fmt"
	"strings"
)

// Commands available in git-vendor
var commands = []string{
	"init",
	"add",
	"edit",
	"remove",
	"list",
	"sync",
	"update",
	"validate",
	"status",
	"check-updates",
	"diff",
	"watch",
	"completion",
	"help",
}

// GenerateBashCompletion generates bash completion script
func GenerateBashCompletion() string {
	return fmt.Sprintf(`# bash completion for git-vendor
_git_vendor_completions() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Commands
    opts="%s"

    # Command-specific options
    case "${prev}" in
        sync)
            opts="--dry-run --force --no-cache --group --verbose -v"
            ;;
        update)
            opts="--verbose -v"
            ;;
        remove)
            opts="--yes -y --quiet -q --json"
            ;;
        list|validate|status|check-updates)
            opts="--quiet -q --json"
            ;;
        completion)
            opts="bash zsh fish powershell"
            ;;
        diff|watch)
            opts=""
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _git_vendor_completions git-vendor
`, strings.Join(commands, " "))
}

// GenerateZshCompletion generates zsh completion script
func GenerateZshCompletion() string {
	cmdList := make([]string, len(commands))
	for i, cmd := range commands {
		desc := getCommandDescription(cmd)
		cmdList[i] = fmt.Sprintf("    '%s:%s'", cmd, desc)
	}

	return fmt.Sprintf(`#compdef git-vendor

_git_vendor() {
    local -a commands
    commands=(
%s
    )

    _arguments -C \
        '1: :->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                sync)
                    _arguments \
                        '--dry-run[Preview without changes]' \
                        '--force[Re-download even if synced]' \
                        '--no-cache[Skip incremental cache]' \
                        '--group[Sync vendor group]:group:' \
                        '--verbose[Show git commands]' \
                        '-v[Show git commands]'
                    ;;
                update)
                    _arguments \
                        '--verbose[Show git commands]' \
                        '-v[Show git commands]'
                    ;;
                remove)
                    _arguments \
                        '--yes[Skip confirmation]' \
                        '-y[Skip confirmation]' \
                        '--quiet[Minimal output]' \
                        '-q[Minimal output]' \
                        '--json[JSON output]'
                    ;;
                list|validate|status|check-updates)
                    _arguments \
                        '--quiet[Minimal output]' \
                        '-q[Minimal output]' \
                        '--json[JSON output]'
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish powershell)'
                    ;;
            esac
            ;;
    esac
}

_git_vendor "$@"
`, strings.Join(cmdList, "\n"))
}

// GenerateFishCompletion generates fish completion script
func GenerateFishCompletion() string {
	var completions []string

	// Add command completions
	for _, cmd := range commands {
		desc := getCommandDescription(cmd)
		completions = append(completions, fmt.Sprintf("complete -c git-vendor -f -n '__fish_use_subcommand' -a '%s' -d '%s'", cmd, desc))
	}

	// Add flag completions
	completions = append(completions, "# sync command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l dry-run -d 'Preview without changes'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l force -d 'Re-download even if synced'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l no-cache -d 'Skip incremental cache'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l group -d 'Sync vendor group' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l verbose -s v -d 'Show git commands'")

	completions = append(completions, "# update command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update' -l verbose -s v -d 'Show git commands'")

	completions = append(completions, "# remove command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l yes -s y -d 'Skip confirmation'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l json -d 'JSON output'")

	completions = append(completions, "# list/validate/status/check-updates flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from list validate status check-updates' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from list validate status check-updates' -l json -d 'JSON output'")

	completions = append(completions, "# completion command shells")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from completion' -f -a 'bash zsh fish powershell'")

	return strings.Join(completions, "\n")
}

// GeneratePowerShellCompletion generates PowerShell completion script
func GeneratePowerShellCompletion() string {
	cmdArray := make([]string, len(commands))
	for i, cmd := range commands {
		cmdArray[i] = fmt.Sprintf("'%s'", cmd)
	}

	return fmt.Sprintf(`# PowerShell completion for git-vendor
Register-ArgumentCompleter -Native -CommandName git-vendor -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @(%s)

    $line = $commandAst.ToString()
    $tokens = $line.Split(' ')

    if ($tokens.Count -eq 2) {
        # Complete command
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }
    elseif ($tokens.Count -gt 2) {
        $subcommand = $tokens[1]

        switch ($subcommand) {
            'sync' {
                @('--dry-run', '--force', '--no-cache', '--group', '--verbose', '-v') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'update' {
                @('--verbose', '-v') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'remove' {
                @('--yes', '-y', '--quiet', '-q', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            { $_ -in 'list','validate','status','check-updates' } {
                @('--quiet', '-q', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'completion' {
                @('bash', 'zsh', 'fish', 'powershell') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
        }
    }
}
`, strings.Join(cmdArray, ", "))
}

// getCommandDescription returns a short description for a command
func getCommandDescription(cmd string) string {
	descriptions := map[string]string{
		"init":          "Initialize vendor directory",
		"add":           "Add vendor dependency",
		"edit":          "Edit vendor configuration",
		"remove":        "Remove vendor dependency",
		"list":          "List all vendors",
		"sync":          "Sync dependencies at locked versions",
		"update":        "Update lockfile with latest commits",
		"validate":      "Validate config and check conflicts",
		"status":        "Check sync status",
		"check-updates": "Check for available updates",
		"diff":          "Show commit differences",
		"watch":         "Watch for config changes",
		"completion":    "Generate shell completion script",
		"help":          "Show help information",
	}

	if desc, ok := descriptions[cmd]; ok {
		return desc
	}
	return ""
}
