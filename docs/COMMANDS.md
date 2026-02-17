# Command Reference

Complete reference for all git-vendor commands.

## Git Subcommand Support

git-vendor can be invoked as either `git-vendor` or `git vendor` - both work identically.

**Examples:**

```bash
# Standalone command
git-vendor init
git-vendor sync

# Git subcommand (same result)
git vendor init
git vendor sync
```

All examples in this document use `git-vendor`, but you can substitute `git vendor` anywhere.

---

## Table of Contents

- [Global Flags](#global-flags)
- [init](#init)
- [add](#add)
- [edit](#edit)
- [remove](#remove)
- [list](#list)
- [sync](#sync)
- [update](#update)
- [validate](#validate)
- [status](#status)
- [check-updates](#check-updates)
- [diff](#diff)
- [watch](#watch)
- [completion](#completion)

---

## Global Flags

| Flag            | Description                          |
| --------------- | ------------------------------------ |
| `--version`     | Show version information             |
| `--help, -h`    | Show help for command                |
| `--verbose, -v` | Show git commands (sync/update only) |

---

## Commands

### init

Initialize vendor directory structure in your project.

**Usage:**

```bash
git-vendor init
```

**Creates:**

- `vendor/vendor.yml` - Configuration file
- `vendor/vendor.lock` - Lock file with commit hashes
- `vendor/licenses/` - Directory for cached license files

**When to use:**

- First time setting up git-vendor in a project
- After cloning a repository that uses git-vendor

---

### add

Add a new vendor dependency using an interactive wizard.

**Usage:**

```bash
git-vendor add
```

**The wizard guides you through:**

1. Entering the repository URL (or paste a GitHub file link)
2. Selecting the git ref (branch/tag)
3. Choosing specific paths to vendor
4. Configuring local destination paths

**Smart URL Parsing**: You can paste any GitHub URL:

- `https://github.com/owner/repo` - Base repository
- `https://github.com/owner/repo/blob/main/src/file.go` - Specific file
- `https://github.com/owner/repo/tree/main/src/` - Specific directory

The tool automatically extracts the repository, ref, and path.

**⚠️ Limitation**: Branch names containing slashes (e.g., `feature/new-feature`) are not supported in URL parsing due to ambiguity. For such branches, use the base repository URL and manually enter the ref during the wizard.

---

### edit

Modify existing vendor configurations through an interactive wizard.

**Usage:**

```bash
git-vendor edit
```

**Allows you to:**

- Add/remove path mappings
- Change tracked branches/tags
- Update configuration settings

---

### remove

Remove a vendor dependency.

**Usage:**

```bash
git-vendor remove <name>
```

**Arguments:**

- `<name>` - Name of the vendor to remove

**Requires confirmation before deletion.**

**Removes:**

- Configuration entry from `vendor.yml`
- Cached license file from `vendor/licenses/`

---

### list

Display all configured vendor dependencies.

**Usage:**

```bash
git-vendor list
```

**Shows:**

- Repository URL
- Tracked refs
- Path mappings
- Sync status

---

### sync

Download vendored dependencies to their configured local paths.

**Usage:**

```bash
git-vendor sync [options] [vendor-name]
```

**Options:**

| Flag             | Description                                                           |
| ---------------- | --------------------------------------------------------------------- |
| `--dry-run`      | Preview what will be synced without making changes                    |
| `--force`        | Re-download files even if already synced                              |
| `--no-cache`     | Disable incremental sync cache (re-download and revalidate all files) |
| `--group <name>` | Sync only vendors in the specified group                              |
| `--parallel`     | Enable parallel processing (3-5x faster)                              |
| `--workers <N>`  | Number of parallel workers (default: NumCPU, max 8)                   |
| `--internal`     | Sync only internal vendors (no network access required)               |
| `--reverse`      | Propagate destination changes back to source (requires `--internal`)  |
| `--commit`       | Auto-commit after sync with vendor/v1 trailers                       |
| `--local`        | Allow file:// and local filesystem paths in vendor URLs               |
| `--verbose, -v`  | Show git commands as they run (useful for debugging)                  |
| `<vendor-name>`  | Sync only the specified vendor                                        |

**Examples:**

```bash
# Sync all vendors
git-vendor sync

# Preview sync without changes
git-vendor sync --dry-run

# Sync only specific vendor
git-vendor sync my-vendor

# Sync all vendors in a group
git-vendor sync --group frontend

# Force re-download
git-vendor sync --force

# Disable cache
git-vendor sync --no-cache

# Parallel sync with default workers
git-vendor sync --parallel

# Parallel sync with custom worker count
git-vendor sync --parallel --workers 4
```

**How it works:**

1. Reads locked commit hashes from `vendor.lock`
2. **Checks incremental sync cache** - if files exist and match cached checksums, skip git operations entirely (⚡ fast!)
3. Clones repositories to temporary directories (only if cache miss)
4. Checks out exact commit hashes for reproducibility
5. Copies specified paths to local destinations
6. Updates cache with SHA-256 file checksums for next sync
7. Caches license files to `vendor/licenses/`

**Note:** The incremental sync cache is stored in `vendor/.cache/` and automatically invalidates when commit hashes change. Use `--force` or `--no-cache` to bypass the cache.

---

### update

Fetch the latest commits for all configured refs and update the lock file.

**Usage:**

```bash
git-vendor update [vendor-name] [options]
```

**Arguments:**

- `[vendor-name]` - Optional. Update only the specified vendor (others retain existing lock entries)

**Options:**

| Flag             | Description                                             |
| ---------------- | ------------------------------------------------------- |
| `--group <name>` | Update only vendors in the specified group               |
| `--parallel`     | Enable parallel processing (3-5x faster)                |
| `--workers <N>`  | Number of parallel workers (default: NumCPU, max 8)     |
| `--local`        | Allow file:// and local filesystem paths in vendor URLs  |
| `--verbose, -v`  | Show git commands as they run (useful for debugging)     |

**This command:**

1. Fetches the latest commit for each tracked ref
2. Updates `vendor.lock` with new commit hashes
3. Downloads and caches license files
4. Shows which vendors were updated

**Run this when you want to:**

- Update to the latest version of dependencies
- Regenerate the lock file after manual config changes
- Refresh cached license files

**Examples:**

```bash
# Update all vendors
git-vendor update

# Update with verbose output
git-vendor update --verbose

# Parallel update
git-vendor update --parallel

# Parallel update with custom workers
git-vendor update --parallel --workers 4
```

---

### validate

Check configuration integrity and detect path conflicts between vendors.

**Usage:**

```bash
git-vendor validate
```

**This command validates:**

- Configuration file syntax and structure
- All vendors have required fields (name, URL, refs)
- All specs have at least one path mapping
- No duplicate vendor names
- **Path conflicts** - detects if multiple vendors map to the same destination

**Example output:**

```bash
# Successful validation
$ git-vendor validate
✔ Validation passed. No issues found.

# With conflicts detected
$ git-vendor validate
! Path Conflicts Detected
Found 2 conflict(s)

⚠ Conflict: lib/utils
  • vendor-a: src/utils → lib/utils
  • vendor-b: pkg/utils → lib/utils
```

**When to use:**

- After editing `vendor.yml` manually
- Before committing vendoring changes
- To diagnose sync issues
- As part of CI/CD validation

---

### status

Check if local files are in sync with the lock file.

**Usage:**

```bash
git-vendor status [options]
```

**Options:**

| Flag          | Description                          |
| ------------- | ------------------------------------ |
| `--json`      | Output in JSON format for automation |
| `--quiet, -q` | No output, exit code only            |

**This command verifies:**

- All vendored files exist at their configured paths
- Files haven't been manually modified since sync
- Lock file entries have corresponding local files

**Examples:**

```bash
# Check sync status (normal output)
git-vendor status

# JSON output for automation
git-vendor status --json

# Quiet mode (exit code only)
git-vendor status --quiet
```

**Example output:**

```text
✔ All vendors synced

# Or if out of sync:
! Vendors Need Syncing
2 vendors out of sync

⚠ my-vendor @ main
  • Missing: lib/utils.go
  • Missing: lib/types.go

Run 'git-vendor sync' to fix.
```

**Exit codes:**

- `0` - All vendors are synced
- `1` - Some vendors need syncing

---

### check-updates

Check if newer commits are available for your vendored dependencies without updating them.

**Usage:**

```bash
git-vendor check-updates [options]
```

**Options:**

| Flag     | Description                          |
| -------- | ------------------------------------ |
| `--json` | Output in JSON format for automation |

**This command:**

1. Compares locked commit hashes with the latest commits on tracked refs
2. Shows which vendors have updates available
3. Displays current and latest commit hashes
4. Supports JSON output for automation

**Examples:**

```bash
# Check for updates (normal output)
git-vendor check-updates

# JSON output for scripting
git-vendor check-updates --json
```

**Example output:**

```text
Found 2 updates:

📦 charmbracelet/lipgloss @ v0.10.0
   Current: abc123f
   Latest:  def456g
   Updated: 2024-11-15T10:30:00Z

📦 golang/mock @ main
   Current: ghi789h
   Latest:  jkl012i
   Updated: 2024-12-01T14:20:00Z

Run 'git-vendor update' to fetch latest versions
```

**Exit codes:**

- `0` - All vendors are up to date
- `1` - Updates available

---

### diff

Show commit differences between the locked version and the latest available version.

**Usage:**

```bash
git-vendor diff [vendor-name] [options]
```

**Arguments:**

- `[vendor-name]` - Optional. Show diff for a specific vendor only

**Options:**

| Flag             | Description                                             |
| ---------------- | ------------------------------------------------------- |
| `--ref <ref>`    | Compare against a specific ref instead of latest         |
| `--group <name>` | Show diffs only for vendors in the specified group        |

**This command displays:**

- Current locked commit hash and date
- Latest available commit hash and date
- Commit history (up to 10 commits) with messages
- Author and date for each commit

**Examples:**

```bash
# Show diff for a specific vendor
git-vendor diff my-vendor

# Check what changed since last update
git-vendor diff charmbracelet/lipgloss
```

**Example output:**

```text
📦 charmbracelet/lipgloss @ v0.10.0
   Old: abc123f (Nov 15)
   New: def456g (Dec 20)

   Commits (+5):
   • def456g - Fix: color rendering bug (Dec 20)
   • ghi789h - Feat: add gradient support (Dec 18)
   • jkl012i - Docs: update examples (Dec 15)
   • mno345j - Refactor: optimize styles (Dec 12)
   • pqr678k - Fix: border rendering (Nov 28)
```

**When to use:**

- Before running `git-vendor update` to see what will change
- To review changes in dependencies
- To track when dependencies were last updated
- To generate changelog information

---

### watch

Watch for changes to `vendor.yml` and automatically sync when the file is modified.

**Usage:**

```bash
git-vendor watch
```

**This command:**

1. Monitors `vendor/vendor.yml` for file system changes
2. Automatically runs `git-vendor sync` when changes are detected
3. Debounces rapid changes (1 second delay)
4. Runs until you press Ctrl+C

**Example:**

```bash
git-vendor watch
# 👁 Watching for changes to vendor/vendor.yml...
# Press Ctrl+C to stop
#
# 📝 Detected change to vendor.yml
# [Sync output...]
# ✓ Sync completed
#
# 👁 Still watching for changes...
```

**When to use:**

- During development when frequently modifying vendor configuration
- To automatically sync after manual edits to `vendor.yml`
- For live-reloading vendored dependencies

**Note:** This command requires write access to the vendor directory and will sync all vendors on each detected change.

---

### completion

Generate shell completion scripts for command-line auto-completion.

**Usage:**

```bash
git-vendor completion <shell>
```

**Arguments:**

- `<shell>` - Shell type: `bash`, `zsh`, `fish`, or `powershell`

**Supported shells:**

- `bash` - Bash completion
- `zsh` - Zsh completion
- `fish` - Fish shell completion
- `powershell` - PowerShell completion

**Installation:**

```bash
# Bash (Linux)
git-vendor completion bash | sudo tee /etc/bash_completion.d/git-vendor

# Bash (macOS with Homebrew)
git-vendor completion bash > $(brew --prefix)/etc/bash_completion.d/git-vendor

# Zsh
git-vendor completion zsh > ~/.zsh/completions/_git-vendor

# Fish
git-vendor completion fish > ~/.config/fish/completions/git-vendor.fish

# PowerShell (add to profile)
git-vendor completion powershell >> $PROFILE
```

**Features:**

- Command name completion
- Flag completion for each command
- Context-aware suggestions (e.g., flags only shown for relevant commands)

---

### config add-mirror

Add a fallback mirror URL to a vendor.

**Usage:**

```bash
git-vendor config add-mirror <vendor-name> <mirror-url>
```

The mirror URL is tried after the primary URL fails during sync, update, diff, and outdated operations. Multiple mirrors are tried in declaration order.

---

### config remove-mirror

Remove a mirror URL from a vendor.

**Usage:**

```bash
git-vendor config remove-mirror <vendor-name> <mirror-url>
```

---

### config list-mirrors

List all mirror URLs configured for a vendor.

**Usage:**

```bash
git-vendor config list-mirrors <vendor-name>
```

---

## See Also

- [Configuration Reference](./CONFIGURATION.md) - vendor.yml and vendor.lock format
- [Advanced Usage](./ADVANCED.md) - Hooks, groups, parallel processing
- [Platform Support](./PLATFORMS.md) - GitHub, GitLab, Bitbucket details
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
