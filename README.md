# git-vendor

[![Tests](https://github.com/EmundoT/git-vendor/workflows/Tests/badge.svg)](https://github.com/EmundoT/git-vendor/actions)
[![codecov](https://codecov.io/gh/EmundoT/git-vendor/branch/main/graph/badge.svg)](https://codecov.io/gh/EmundoT/git-vendor)
[![Go Report Card](https://goreportcard.com/badge/github.com/EmundoT/git-vendor)](https://goreportcard.com/report/github.com/EmundoT/git-vendor)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A lightweight CLI tool for managing vendored dependencies from Git repositories with granular path control.

## Features

- **Granular Path Vendoring**: Vendor specific files or directories, not entire repositories
- **Interactive TUI**: User-friendly terminal interface with file browser
- **Deterministic Locking**: Lock dependencies to specific commits for reproducibility
- **Incremental Sync**: Smart caching system skips re-downloading unchanged files (80% faster re-syncs)
- **Update Checker**: Check for available updates without modifying files
- **Vendor Groups**: Organize vendors into groups for batch operations
- **Multi-Ref Support**: Track multiple branches/tags from the same repository
- **Multi-Platform Support**: Works with GitHub, GitLab, Bitbucket, and any git server
- **Smart URL Parsing**: Paste file links directly - auto-extracts repo, branch, and path
- **License Compliance**: Automatic license detection via API or LICENSE file
- **Dry-Run Mode**: Preview changes before applying them

## Supported Platforms

git-vendor supports multiple git hosting platforms with automatic detection:

### Full API Support (Automatic License Detection)

**GitHub** - github.com and GitHub Enterprise
- Smart URL parsing for `/blob/` and `/tree/` links
- API-based license detection via GitHub API
- Authentication: `GITHUB_TOKEN` environment variable
```bash
git-vendor add https://github.com/owner/repo/blob/main/src/utils.go
git-vendor add https://github.enterprise.com/team/project
```

**GitLab** - gitlab.com and self-hosted instances
- Smart URL parsing for `/-/blob/` and `/-/tree/` links
- API-based license detection via GitLab API
- Authentication: `GITLAB_TOKEN` environment variable
- Supports nested groups (unlimited depth)
```bash
git-vendor add https://gitlab.com/owner/repo/-/blob/main/lib/helper.go
git-vendor add https://gitlab.com/group/subgroup/project/-/tree/dev/src
git-vendor add https://gitlab.company.com/team/repo
```

### Partial Support (Manual License Detection)

**Bitbucket** - bitbucket.org
- Smart URL parsing for `/src/` links
- License detected from LICENSE file in repository
```bash
git-vendor add https://bitbucket.org/owner/repo/src/main/utils.py
```

**Generic Git** - Any git server
- Supports git://, ssh://, https:// protocols
- License detected from LICENSE file in repository
```bash
git-vendor add https://git.example.com/project/repo.git
git-vendor add git@git.company.com:team/project.git
```

### Authentication

For private repositories or to avoid API rate limits:

```bash
# GitHub (increases rate limit from 60/hr to 5000/hr)
export GITHUB_TOKEN=ghp_your_token_here

# GitLab (enables private repos and increases rate limit)
export GITLAB_TOKEN=glpat_your_token_here

git-vendor add <url>
```

### Platform Auto-Detection

git-vendor automatically detects the hosting platform from your URL - no configuration needed!

```bash
# Detected as GitHub
git-vendor add https://github.com/golang/go/blob/master/src/fmt/print.go

# Detected as GitLab
git-vendor add https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/api/api.rb

# Detected as Bitbucket
git-vendor add https://bitbucket.org/atlassian/python-bitbucket/src/master/setup.py

# Detected as Generic
git-vendor add https://git.kernel.org/pub/scm/git/git.git
```

## Installation

### Build from Source

```bash
git clone https://github.com/EmundoT/git-vendor
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
- `--no-cache` - Disable incremental sync cache (re-download and revalidate all files)
- `--group <name>` - Sync only vendors in the specified group
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

# Sync all vendors in a group
git-vendor sync --group frontend

# Force re-download
git-vendor sync --force
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

#### `git-vendor check-updates`

Check if newer commits are available for your vendored dependencies without updating them.

This command:

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
    groups: ["frontend", "ui"]  # Optional: organize vendors into groups
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
- `groups`: (Optional) Array of group names for batch operations
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

## Using in CI/CD

Git-vendor is designed for reproducible builds and works great in automated pipelines.

### Basic CI/CD Workflow

```yaml
# Example: GitHub Actions
name: Build
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install git-vendor
        run: |
          curl -L https://github.com/EmundoT/git-vendor/releases/latest/download/git-vendor-linux-amd64 -o /usr/local/bin/git-vendor
          chmod +x /usr/local/bin/git-vendor

      - name: Sync vendored dependencies
        run: git-vendor sync
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Build project
        run: make build
```

### Configuration Setup

**Option 1: Pre-commit vendor files (Recommended)**

Commit both `vendor/vendor.yml` and `vendor/vendor.lock` to your repository. In CI, simply run:

```bash
git-vendor sync
```

This ensures deterministic builds - all developers and CI use the exact same locked commits.

**Option 2: Generate lockfile in CI**

Only commit `vendor/vendor.yml`. In CI, run:

```bash
git-vendor update  # Generate lockfile from latest commits
git-vendor sync    # Download dependencies
```

⚠️ **Warning**: This approach fetches the latest commits at CI runtime, which may cause non-deterministic builds.

### Environment Variables

**GITHUB_TOKEN** (Recommended for CI)

Set a GitHub token to increase API rate limits:

- Unauthenticated: 60 requests/hour
- With token: 5,000 requests/hour

```yaml
# GitHub Actions
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

# GitLab CI
variables:
  GITHUB_TOKEN: $CI_JOB_TOKEN

# CircleCI
environment:
  GITHUB_TOKEN: ${GITHUB_TOKEN}
```

### Validation in CI

Add validation to catch configuration errors early:

```yaml
- name: Validate vendor config
  run: git-vendor validate
```

This checks for:

- Duplicate vendor names
- Missing URLs or refs
- Empty path mappings
- Path conflicts between vendors

### Tips for CI/CD

1. **Use verbose mode for debugging**:
   ```bash
   git-vendor sync --verbose
   ```

2. **Cache vendored files** to speed up builds:
   ```yaml
   # GitHub Actions
   - uses: actions/cache@v3
     with:
       path: |
         vendor/
         lib/
       key: vendor-${{ hashFiles('vendor/vendor.lock') }}
   ```

3. **Use dry-run to preview changes**:
   ```bash
   git-vendor sync --dry-run
   ```

4. **Sync specific vendors** for faster iteration:
   ```bash
   git-vendor sync my-vendor
   ```

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

**Quick Start:**
```bash
make mocks    # Generate mocks
make test     # Run tests
make lint     # Run linter
make ci       # Run all CI checks
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
3. If you modify interfaces, regenerate mocks (`make mocks`)
4. Write tests for new functionality (maintain >60% coverage)
5. Ensure all tests pass (`go test ./...`)
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
