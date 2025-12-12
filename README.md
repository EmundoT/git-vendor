# git-vendor

A lightweight CLI tool for managing vendored dependencies from Git repositories with granular path control.

## Features

- **Granular Path Vendoring**: Vendor specific files or directories, not entire repositories
- **Interactive TUI**: User-friendly terminal interface with file browser
- **Deterministic Locking**: Lock dependencies to specific commits for reproducibility
- **Multi-Ref Support**: Track multiple branches/tags from the same repository
- **Smart URL Parsing**: Paste GitHub file links directly - auto-extracts repo, branch, and path
- **License Compliance**: Automatic license detection and verification
- **Dry-Run Mode**: Preview changes before applying them

## Installation

### Build from Source

```bash
git clone https://github.com/yourusername/git-vendor
cd git-vendor
go build -o git-vendor
```

### Install Binary

Copy the built binary to your PATH:

```bash
# macOS/Linux
sudo mv git-vendor /usr/local/bin/

# Windows
# Move git-vendor.exe to a directory in your PATH
```

### Requirements

- Go 1.23 or later (for building from source)
- Git installed and accessible in PATH

## Quick Start

```bash
# Initialize vendor directory
git-vendor init

# Add a vendor dependency (interactive wizard)
git-vendor add

# Sync all dependencies
git-vendor sync

# List configured vendors
git-vendor list
```

## Usage

### Commands

#### `git-vendor init`

Initialize the vendor directory structure in your project.

Creates:

- `vendor/vendor.yml` - Configuration file
- `vendor/vendor.lock` - Lock file with commit hashes
- `vendor/licenses/` - Directory for cached license files

#### `git-vendor add`

Add a new vendor dependency using an interactive wizard.

The wizard will guide you through:

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

#### `git-vendor edit`

Modify existing vendor configurations through an interactive wizard.

Allows you to:

- Add/remove path mappings
- Change tracked branches/tags
- Update configuration settings

#### `git-vendor remove <name>`

Remove a vendor dependency.

Requires confirmation before deletion. Removes:

- Configuration entry from `vendor.yml`
- Cached license file from `vendor/licenses/`

#### `git-vendor list`

Display all configured vendor dependencies with their:

- Repository URL
- Tracked refs
- Path mappings
- Sync status

#### `git-vendor sync [options] [vendor-name]`

Download vendored dependencies to their configured local paths.

**Options:**

- `--dry-run` - Preview what will be synced without making changes
- `--force` - Re-download files even if already synced
- `--verbose` / `-v` - Show git commands as they run (useful for debugging)
- `<vendor-name>` - Sync only the specified vendor

**Examples:**

```bash
# Sync all vendors
git-vendor sync

# Preview sync without changes
git-vendor sync --dry-run

# Sync only specific vendor
git-vendor sync my-vendor

# Force re-download
git-vendor sync --force
```

**How it works:**

1. Reads locked commit hashes from `vendor.lock`
2. Clones repositories to temporary directories
3. Checks out exact commit hashes for reproducibility
4. Copies specified paths to local destinations
5. Caches license files to `vendor/licenses/`

#### `git-vendor update [options]`

Fetch the latest commits for all configured refs and update the lock file.

**Options:**

- `--verbose` / `-v` - Show git commands as they run (useful for debugging)

This command:

1. Fetches the latest commit for each tracked ref
2. Updates `vendor.lock` with new commit hashes
3. Downloads and caches license files
4. Shows which vendors were updated

Run this when you want to:

- Update to the latest version of dependencies
- Regenerate the lock file after manual config changes
- Refresh cached license files

#### `git-vendor validate`

Check configuration integrity and detect path conflicts between vendors.

This command validates:

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

### Configuration Files

#### vendor.yml

Main configuration file defining all vendor dependencies.

```yaml
vendors:
  - name: example-lib
    url: https://github.com/owner/repo
    license: MIT
    specs:
      - ref: main
        default_target: ""
        mapping:
          - from: src/utils
            to: internal/vendor/example-utils
          - from: src/types.go
            to: internal/vendor/types.go
```

**Fields:**

- `name`: Display name for the vendor
- `url`: Git repository URL
- `license`: SPDX license identifier (auto-detected)
- `specs`: Array of refs to track (can track multiple branches/tags)
  - `ref`: Branch, tag, or commit hash
  - `default_target`: Optional default destination directory
  - `mapping`: Array of path mappings
    - `from`: Remote path in the repository
    - `to`: Local destination path (empty = auto-named from source)

#### vendor.lock

Lock file storing exact commit hashes for reproducibility.

```yaml
vendors:
  - name: example-lib
    ref: main
    commit_hash: abc123def456...
    license_path: vendor/licenses/example-lib.txt
    updated: "2024-01-15T14:30:45Z"
```

**Never edit this file manually** - it's automatically generated by `update` and `sync` commands.

## Common Workflows

### Adding Your First Dependency

1. **Initialize the vendor directory:**

   ```bash
   git-vendor init
   ```

2. **Run the add wizard:**

   ```bash
   git-vendor add
   ```

3. **Enter the repository URL:**
   - Paste any GitHub URL (repo, file, or directory)
   - Smart URL parsing will extract repo, branch, and path
   - Example: `https://github.com/owner/repo/blob/main/src/utils/helper.go`

4. **Configure the vendor:**
   - Confirm or edit the vendor name
   - Confirm or edit the git ref (branch/tag)
   - Select specific paths to vendor using the file browser

5. **Map paths to your project:**
   - Choose remote paths using the interactive browser
   - Set local destination paths
   - Leave destination empty for automatic naming

6. **Save and sync:**
   - The wizard automatically runs `update` to lock commits
   - Run `git-vendor sync` to download the files

### Vendoring Specific Files or Directories

**From a GitHub URL:**

```bash
git-vendor add
# Paste: https://github.com/owner/repo/blob/main/src/utils/helper.go
# The tool auto-detects you want just that one file
```

**Using the file browser:**

```bash
git-vendor add
# Enter base repo URL
# Navigate through the remote file browser
# Select the exact file or directory you need
```

### Updating Dependencies

**Update to latest versions:**

```bash
git-vendor update  # Fetch latest commits and update lockfile
git-vendor sync    # Download updated files
```

**Preview before updating:**

```bash
git-vendor sync --dry-run  # See what will change without modifying files
```

**Update specific vendor only:**

```bash
git-vendor sync my-vendor  # Update just one dependency
```

## Advanced Usage

### Multi-Ref Tracking

Track multiple branches or tags from the same repository:

```yaml
vendors:
  - name: multi-branch-lib
    url: https://github.com/owner/repo
    specs:
      - ref: v1.0
        mapping:
          - from: src
            to: vendor/lib-v1
      - ref: v2.0
        mapping:
          - from: src
            to: vendor/lib-v2
```

### Auto-Naming Paths

Leave the destination path empty to use automatic naming:

```yaml
mapping:
  - from: src/utils
    to: ""  # Will be named "utils" in current directory
```

The auto-naming logic:

1. Uses the base name of the source path
2. Respects `default_target` if set
3. Falls back to vendor name for root paths

## Security

Git-vendor includes several security protections to ensure safe vendoring operations.

### Path Traversal Protection

Destination paths are validated to prevent path traversal attacks:

- **Absolute paths are rejected**: Paths like `/etc/passwd` or `C:\Windows\System32` are not allowed
- **Parent directory references are rejected**: Paths containing `..` like `../../../etc/passwd` are blocked
- **Only relative paths within the project are allowed**: All vendored files must be copied to paths relative to your project root

If you encounter an "invalid destination path" error, ensure your path mappings use relative paths without `..` segments.

### License Compliance

Git-vendor automatically detects and validates licenses to help ensure compliance:

**Automatic Detection:**

- Queries GitHub API to detect repository licenses
- Supports SPDX license identifiers
- Caches license files locally in `vendor/licenses/` for audit purposes

**Allowed Licenses:**
The following licenses are pre-approved and will be accepted automatically:

- MIT
- Apache-2.0
- BSD-3-Clause
- BSD-2-Clause
- ISC
- Unlicense
- CC0-1.0

**Non-Standard Licenses:**

- Other licenses will trigger a confirmation prompt
- You can review the license details before accepting
- Accepting is your responsibility for license compliance

**Override License Check:**
When prompted, you can accept any license by confirming the dialog. The license file will be cached in `vendor/licenses/<vendor-name>.txt` for your records.

## Comparison with Alternatives

### vs. Git Submodules

**Advantages:**

- ✅ Vendor specific files/directories, not entire repositories
- ✅ No `.gitmodules` file to manage
- ✅ Simpler workflow for partial vendoring
- ✅ Vendored files are plain copies, not nested git repos
- ✅ No need to `git submodule update --init --recursive`

**Disadvantages:**

- ❌ Requires separate tool installation
- ❌ Not natively supported by git

**Best for:** Vendoring specific files or directories from larger repos

### vs. Package Managers (npm, pip, cargo, go modules)

**Advantages:**

- ✅ Language-agnostic (works with any Git repository)
- ✅ Granular path control (vendor just what you need)
- ✅ Can vendor non-package code (snippets, configs, scripts)
- ✅ Works with repositories that aren't published packages

**Disadvantages:**

- ❌ Not integrated with language toolchains
- ❌ Manual management required
- ❌ No dependency resolution

**Best for:** Cross-language projects, vendoring utilities, non-packaged code

### vs. Manual Copying

**Advantages:**

- ✅ Reproducible with lock file (know exactly what version you have)
- ✅ Easy to update (one command vs. manual copy)
- ✅ Tracks source and provenance (know where code came from)
- ✅ License compliance tracking built-in
- ✅ Dry-run mode to preview changes

**Disadvantages:**

- ❌ Requires tool installation

**Best for:** Anyone currently copying files manually who wants reproducibility

## Who Should Use This Tool?

**✅ Perfect for:**

- Projects vendoring utility functions from OSS libraries
- Language-agnostic codebases needing dependencies from multiple languages
- Teams wanting deterministic, reproducible builds
- Projects that need specific files/directories, not entire repositories
- Teams concerned about license compliance
- Developers comfortable with CLI tools

**⚠️ Maybe Not For:**

- Large-scale package management (use language-specific package managers)
- Projects requiring GitLab or Bitbucket support (GitHub-only currently)
- Teams needing fully automated CI/CD integration (interactive wizards)
- Projects requiring dependency resolution or version constraints

## Architecture

### Directory Structure

```text
your-project/
├── vendor/
│   ├── vendor.yml      # Configuration
│   ├── vendor.lock     # Lock file
│   └── licenses/       # Cached licenses
│       └── vendor-name.txt
└── internal/           # Your vendored code lives here
    └── vendor/         # (or wherever you configure)
```

### How It Works

1. **Configuration**: `vendor.yml` defines what to vendor and where
2. **Locking**: `vendor.lock` stores exact commit hashes
3. **Syncing**: Git operations in temp directories copy files to destinations
4. **Caching**: Licenses cached locally, repos cloned shallowly for speed

### Key Design Decisions

- **Granular Path Mapping**: Unlike submodules, vendor specific paths
- **Deterministic Locking**: Lock files ensure reproducible builds
- **No Git History**: Vendored files are plain copies, not git repos
- **Interactive TUI**: User-friendly wizard for complex operations
- **GitHub-First**: Smart URL parsing optimized for GitHub workflows

## Troubleshooting

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for common issues and solutions.

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./internal/core

# Run with verbose output
go test -v ./...
```

Current test coverage: **63.9%** of statements

### Building from Source

```bash
# Build for your platform
go build -o git-vendor

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o git-vendor-linux
GOOS=darwin GOARCH=arm64 go build -o git-vendor-macos
GOOS=windows GOARCH=amd64 go build -o git-vendor.exe
```

### Project Structure

```text
git-vendor/
├── main.go                    # CLI entry point and command dispatcher
├── internal/
│   ├── core/                  # Business logic
│   │   ├── engine.go          # Manager facade
│   │   ├── vendor_syncer.go   # Core sync logic
│   │   ├── git_operations.go  # Git client
│   │   ├── filesystem.go      # File operations
│   │   ├── github_client.go   # License checking
│   │   ├── config_store.go    # Config I/O
│   │   └── lock_store.go      # Lock I/O
│   ├── tui/                   # Terminal UI
│   │   └── wizard.go          # Interactive wizards
│   └── types/                 # Data models
│       └── types.go
└── vendor/                    # Vendored dependencies
```

## Contributing

Contributions are welcome! Here's how to contribute:

### Reporting Issues

1. Check existing issues first
2. Include reproduction steps
3. Provide your git and git-vendor versions
4. Include relevant config files (sanitized)

### Contributing Code

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for new functionality (maintain >60% coverage)
4. Ensure all tests pass (`go test ./...`)
5. Update documentation (README, TROUBLESHOOTING, etc.)
6. Commit with clear messages
7. Push to your fork
8. Submit a pull request

### Code Guidelines

- Follow Go best practices
- Add tests for new features
- Update documentation
- Keep dependencies minimal
- Maintain backward compatibility

## License

MIT License - see [LICENSE](./LICENSE) for details.

## Credits

Built with:

- [charmbracelet/huh](https://github.com/charmbracelet/huh) - TUI forms
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
