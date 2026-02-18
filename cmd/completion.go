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
	"pull",
	"validate",
	"status",
	"verify",
	"outdated",
	"check-updates",
	"diff",
	"watch",
	"completion",
	"help",
	"compliance",
	// LLM-friendly commands (Spec 072)
	"create",
	"delete",
	"rename",
	"add-mapping",
	"remove-mapping",
	"list-mappings",
	"update-mapping",
	"show",
	"check",
	"preview",
	"config",
}

// DeprecatedCommands maps deprecated command names to their replacement
// descriptions, used by shell completion generators to annotate suggestions.
var DeprecatedCommands = map[string]string{
	"sync":     "DEPRECATED: use 'pull --locked'",
	"update":   "DEPRECATED: use 'pull'",
	"verify":   "DEPRECATED: use 'status --offline'",
	"diff":     "DEPRECATED: use 'status'",
	"outdated": "DEPRECATED: use 'status --remote-only'",
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
        pull)
            opts="--locked --prune --keep-local --interactive --force --no-cache --commit --local --verbose -v"
            ;;
        sync)
            opts="--dry-run --force --no-cache --group --parallel --workers --verbose -v"
            ;;
        update)
            opts="--parallel --workers --verbose -v"
            ;;
        remove)
            opts="--yes -y --quiet -q --json"
            ;;
        list|validate|check-updates)
            opts="--quiet -q --json"
            ;;
        status)
            opts="--quiet -q --json --offline --remote-only --strict-only --compliance= --format"
            ;;
        completion)
            opts="bash zsh fish powershell"
            ;;
        diff|watch)
            opts=""
            ;;
        create)
            opts="--ref --license --json"
            ;;
        delete)
            opts="--yes -y --quiet -q --json"
            ;;
        rename)
            opts="--json"
            ;;
        add-mapping)
            opts="--to --ref --json"
            ;;
        remove-mapping)
            opts="--json"
            ;;
        list-mappings|show|check|preview)
            opts="--json"
            ;;
        update-mapping)
            opts="--to --json"
            ;;
        config)
            opts="get set list --json"
            ;;
        compliance)
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
                pull)
                    _arguments \
                        '--locked[Use existing lock, skip fetch]' \
                        '--prune[Remove dead mappings]' \
                        '--keep-local[Detect locally modified files]' \
                        '--interactive[Interactive mode]' \
                        '--force[Re-download even if synced]' \
                        '--no-cache[Skip incremental cache]' \
                        '--commit[Auto-commit after pull]' \
                        '--local[Allow local paths]' \
                        '--verbose[Show git commands]' \
                        '-v[Show git commands]'
                    ;;
                sync)
                    _arguments \
                        '--dry-run[Preview without changes]' \
                        '--force[Re-download even if synced]' \
                        '--no-cache[Skip incremental cache]' \
                        '--group[Sync vendor group]:group:' \
                        '--parallel[Enable parallel processing]' \
                        '--workers[Number of parallel workers]:workers:' \
                        '--verbose[Show git commands]' \
                        '-v[Show git commands]'
                    ;;
                update)
                    _arguments \
                        '--parallel[Enable parallel processing]' \
                        '--workers[Number of parallel workers]:workers:' \
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
                list|validate|check-updates)
                    _arguments \
                        '--quiet[Minimal output]' \
                        '-q[Minimal output]' \
                        '--json[JSON output]'
                    ;;
                status)
                    _arguments \
                        '--quiet[Minimal output]' \
                        '-q[Minimal output]' \
                        '--json[JSON output]' \
                        '--offline[Skip remote checks]' \
                        '--remote-only[Skip disk checks]' \
                        '--strict-only[Only check strict vendors]' \
                        '--compliance=[Override compliance level]:level:(strict lenient info)' \
                        '--format=[Output format]:format:(table json)'
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish powershell)'
                    ;;
                create)
                    _arguments \
                        '--ref[Git ref to track]:ref:' \
                        '--license[SPDX license identifier]:license:' \
                        '--json[JSON output]'
                    ;;
                delete)
                    _arguments \
                        '--yes[Skip confirmation]' \
                        '-y[Skip confirmation]' \
                        '--quiet[Minimal output]' \
                        '-q[Minimal output]' \
                        '--json[JSON output]'
                    ;;
                rename)
                    _arguments '--json[JSON output]'
                    ;;
                add-mapping)
                    _arguments \
                        '--to[Destination path]:path:' \
                        '--ref[Target ref]:ref:' \
                        '--json[JSON output]'
                    ;;
                remove-mapping)
                    _arguments '--json[JSON output]'
                    ;;
                list-mappings|show|check|preview)
                    _arguments '--json[JSON output]'
                    ;;
                update-mapping)
                    _arguments \
                        '--to[New destination path]:path:' \
                        '--json[JSON output]'
                    ;;
                config)
                    _arguments '1:subcommand:(get set list)'
                    ;;
                compliance)
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
	completions = append(completions, "# pull command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l locked -d 'Use existing lock, skip fetch'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l prune -d 'Remove dead mappings'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l keep-local -d 'Detect locally modified files'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l interactive -d 'Interactive mode'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l force -d 'Re-download even if synced'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l no-cache -d 'Skip incremental cache'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l commit -d 'Auto-commit after pull'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l local -d 'Allow local paths'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from pull' -l verbose -s v -d 'Show git commands'")

	completions = append(completions, "# sync command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l dry-run -d 'Preview without changes'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l force -d 'Re-download even if synced'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l no-cache -d 'Skip incremental cache'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l group -d 'Sync vendor group' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l parallel -d 'Enable parallel processing'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l workers -d 'Number of parallel workers' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from sync' -l verbose -s v -d 'Show git commands'")

	completions = append(completions, "# update command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update' -l parallel -d 'Enable parallel processing'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update' -l workers -d 'Number of parallel workers' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update' -l verbose -s v -d 'Show git commands'")

	completions = append(completions, "# remove command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l yes -s y -d 'Skip confirmation'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove' -l json -d 'JSON output'")

	completions = append(completions, "# list/validate/check-updates flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from list validate check-updates' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from list validate check-updates' -l json -d 'JSON output'")
	completions = append(completions, "# status command flags")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l offline -d 'Skip remote checks'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l remote-only -d 'Skip disk checks'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l strict-only -d 'Only check strict vendors'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from status' -l compliance -d 'Override compliance level' -r")

	completions = append(completions, "# completion command shells")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from completion' -f -a 'bash zsh fish powershell'")

	completions = append(completions, "# LLM-friendly command flags (Spec 072)")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from create' -l ref -d 'Git ref to track' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from create' -l license -d 'SPDX license identifier' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from create' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from delete' -l yes -s y -d 'Skip confirmation'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from delete' -l quiet -s q -d 'Minimal output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from delete' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from rename' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from add-mapping' -l to -d 'Destination path' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from add-mapping' -l ref -d 'Target ref' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from add-mapping' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from remove-mapping' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from list-mappings show check preview' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update-mapping' -l to -d 'New destination path' -r")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from update-mapping' -l json -d 'JSON output'")
	completions = append(completions, "complete -c git-vendor -n '__fish_seen_subcommand_from config' -f -a 'get set list'")

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
            'pull' {
                @('--locked', '--prune', '--keep-local', '--interactive', '--force', '--no-cache', '--commit', '--local', '--verbose', '-v') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'sync' {
                @('--dry-run', '--force', '--no-cache', '--group', '--parallel', '--workers', '--verbose', '-v') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'update' {
                @('--parallel', '--workers', '--verbose', '-v') |
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
            { $_ -in 'list','validate','check-updates' } {
                @('--quiet', '-q', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'status' {
                @('--quiet', '-q', '--json', '--offline', '--remote-only', '--strict-only', '--compliance=', '--format') |
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
            'create' {
                @('--ref', '--license', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'delete' {
                @('--yes', '-y', '--quiet', '-q', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            { $_ -in 'rename','remove-mapping','list-mappings','show','check','preview' } {
                @('--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'add-mapping' {
                @('--to', '--ref', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'update-mapping' {
                @('--to', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
            'config' {
                @('get', 'set', 'list', '--json') |
                    Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                    }
            }
        }
    }
}
`, strings.Join(cmdArray, ", "))
}

// getCommandDescription returns a short description for a command.
// Deprecated commands include a deprecation notice in their description.
func getCommandDescription(cmd string) string {
	descriptions := map[string]string{
		"init":           "Initialize vendor directory",
		"add":            "Add vendor dependency",
		"edit":           "Edit vendor configuration",
		"remove":         "Remove vendor dependency",
		"list":           "List all vendors",
		"sync":           "Sync at locked versions (DEPRECATED: use pull --locked)",
		"update":         "Update lockfile (DEPRECATED: use pull)",
		"pull":           "Fetch and sync vendor dependencies",
		"validate":       "Validate config and check conflicts",
		"status":         "Show unified verify + outdated status",
		"verify":         "Verify file hashes (DEPRECATED: use status --offline)",
		"outdated":       "Check staleness (DEPRECATED: use status --remote-only)",
		"check-updates":  "Check for available updates",
		"diff":           "Show commit differences (DEPRECATED: use status)",
		"watch":          "Watch for config changes",
		"completion":     "Generate shell completion script",
		"help":           "Show help information",
		"create":         "Create vendor (non-interactive)",
		"delete":         "Delete vendor (alias for remove)",
		"rename":         "Rename a vendor",
		"add-mapping":    "Add path mapping to vendor",
		"remove-mapping": "Remove path mapping from vendor",
		"list-mappings":  "List path mappings for vendor",
		"update-mapping": "Update path mapping destination",
		"show":           "Show vendor details",
		"check":          "Check vendor sync status",
		"preview":        "Preview what would be synced",
		"compliance":     "Show effective compliance levels",
		"config":         "Get or set configuration values",
	}

	if desc, ok := descriptions[cmd]; ok {
		return desc
	}
	return ""
}
