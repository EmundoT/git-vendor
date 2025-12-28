# Changelog

All notable changes to git-vendor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-beta.1] - 2024-12-27

First beta release with comprehensive feature set for granular Git vendoring.

### Added

**Core Vendoring Features:**
- Granular path vendoring - vendor specific files/directories, not entire repositories
- Deterministic locking with vendor.lock (exact commit SHA tracking)
- Interactive TUI with file browser for path selection
- Multi-ref support - track multiple branches/tags from the same repository
- Smart URL parsing - paste GitHub/GitLab/Bitbucket URLs, auto-extract repo/ref/path
- License compliance with automatic detection and validation (GitHub API, GitLab API, LICENSE file fallback)
- Path conflict detection and warnings
- Dry-run mode (`--dry-run`) for previewing changes
- Verbose mode (`--verbose`) for debugging git operations

**Multi-Platform Git Support:**
- GitHub (github.com and GitHub Enterprise) with API-based license detection
- GitLab (gitlab.com and self-hosted) with API-based license detection and nested group support
- Bitbucket (bitbucket.org) with file-based license detection
- Generic Git (any server via HTTPS/SSH/Git protocol)
- Platform auto-detection from URLs

**Advanced Features:**
- **Parallel Processing** - Worker pool for 3-5x speedup on multi-vendor operations
  - `--parallel` flag enables parallel mode
  - `--workers <N>` flag sets custom worker count (default: NumCPU, max 8)
  - Thread-safe with unique temp directories per vendor
  - Atomic lockfile writes (collect results, write once at end)

- **Custom Hooks** - Pre/post sync automation
  - `pre_sync` hook runs before git clone/sync operations
  - `post_sync` hook runs after successful sync
  - Full shell support via `sh -c` (pipes, multiline scripts, etc.)
  - Environment variables: `GIT_VENDOR_NAME`, `GIT_VENDOR_URL`, `GIT_VENDOR_REF`, `GIT_VENDOR_COMMIT`, `GIT_VENDOR_ROOT`, `GIT_VENDOR_FILES_COPIED`
  - Hooks run even for cache hits

- **Incremental Sync Cache** - 80% faster re-syncs
  - SHA-256 file checksums cached in `vendor/.cache/`
  - Skip re-downloading unchanged files
  - Auto-invalidates when commit hashes change
  - `--no-cache` flag to bypass cache
  - `--force` flag to re-download all files

- **Vendor Groups** - Organize vendors for batch operations
  - `groups` field in vendor.yml for logical grouping
  - `--group <name>` flag to sync specific groups
  - Vendors can belong to multiple groups

- **Advanced CLI Commands:**
  - `status` - Check if local files match lockfile
  - `check-updates` - Preview available updates without modifying files
  - `diff <vendor>` - View commit history between locked and latest versions
  - `watch` - Auto-sync on vendor.yml changes (file watching with 1-second debounce)
  - `completion <shell>` - Generate shell completion for bash/zsh/fish/powershell

- **Progress Indicators:**
  - Real-time progress during sync and update operations
  - TTY auto-detection (animated for interactive, text for CI/CD)
  - Vendor-level progress granularity

- **Non-Interactive Mode:**
  - `--yes` flag to auto-confirm prompts (CI/CD)
  - `--quiet` flag to suppress output
  - `--json` flag for JSON output (automation)

**Commands:**
- `init` - Initialize vendor directory structure
- `add` - Add vendor dependency (interactive wizard)
- `edit` - Modify vendor configuration (interactive wizard)
- `remove <name>` - Remove vendor
- `list` - Display all configured vendors
- `sync [options] [vendor]` - Download vendored dependencies
- `update [options]` - Fetch latest commits and update lockfile
- `validate` - Check configuration integrity
- `status` - Check sync state
- `check-updates` - Check for available updates
- `diff <vendor>` - Show commit differences
- `watch` - Watch for config changes
- `completion <shell>` - Generate shell completion

**Documentation:**
- Comprehensive README with quick start
- TROUBLESHOOTING.md with 20+ common issues
- CONTRIBUTING.md with development workflow
- CLAUDE.md for AI-assisted development
- examples/ directory with 7 configuration examples

### Security

- Path traversal protection - rejects absolute paths and `..` references
- License compliance checking with pre-approved OSS licenses
- License files cached in `vendor/licenses/` for audit
- Security validations in vendor.yml parsing

### Performance

- Shallow git clones (`--depth 1`) for faster operations
- Incremental sync cache with SHA-256 checksums
- Parallel processing with worker pools
- File-level caching (1000 file limit per vendor)

### Testing

- 65% test coverage across core packages
- Gomock-based unit tests with auto-generated mocks
- 55+ test cases for sync, update, validation
- Race condition testing (`go test -race` passes)
- Integration tests with real git operations

### Developer Experience

- Clean architecture with dependency injection
- Interface-driven design for testability
- Comprehensive error messages
- Verbose mode for debugging
- Dry-run mode for preview

---

## [0.1.0-alpha] - 2024-12-01

Initial alpha release for internal testing.

### Added

- Basic vendoring functionality with add/sync/update commands
- Interactive add/edit wizards
- vendor.yml and vendor.lock files
- GitHub URL parsing
- License detection (GitHub only)

---

[Unreleased]: https://github.com/EmundoT/git-vendor/compare/v0.1.0-beta.1...HEAD
[0.1.0-beta.1]: https://github.com/EmundoT/git-vendor/releases/tag/v0.1.0-beta.1
[0.1.0-alpha]: https://github.com/EmundoT/git-vendor/releases/tag/v0.1.0-alpha
