# git-vendor v1.0.0 Release Notes

**🎉 First stable release - Production ready!**

git-vendor is a CLI tool for vendoring specific files and directories from Git repositories with deterministic locking. Unlike traditional vendoring tools that copy entire repositories, git-vendor gives you granular control over exactly what gets vendored and where it goes.

## What is git-vendor?

**The Problem:** Traditional dependency management tools force you to vendor entire repositories, even when you only need a few files. Copy-pasting code loses provenance and makes updates painful.

**The Solution:** git-vendor lets you vendor specific files or directories from any Git repository to custom locations in your project, with deterministic locking for reproducible builds.

## Key Features

### 🎯 Granular Path Control
- Vendor specific files/directories, not entire repositories
- Map remote paths to any local destination
- Track multiple branches/tags from the same repository
- Example: Vendor `src/utils.go` from a large monorepo to `internal/vendor/utils.go`

### 🔒 Deterministic Locking
- `vendor.lock` with exact commit SHA tracking
- Reproducible builds across teams and environments
- Perfect for CI/CD pipelines

### 🎨 Interactive TUI
- Wizard-based interface for adding and editing vendors
- Browse remote repository files without cloning
- Smart URL parsing - paste GitHub/GitLab/Bitbucket URLs

### ⚡ Performance
- **Incremental sync cache** - 80% faster re-syncs with SHA-256 checksumming
- **Parallel processing** - 3-5x speedup for multi-vendor operations
- **Optimized binary** - 34% smaller (7.2 MB) with stripped debug symbols

### 🔧 Automation & CI/CD
- Custom pre/post sync hooks for automation
- Non-interactive flags (`--yes`, `--quiet`, `--json`) for scripting
- Vendor groups for batch operations
- Status command to check sync state

### 🌐 Multi-Platform Git Support
- GitHub (including GitHub Enterprise)
- GitLab (including self-hosted and nested groups)
- Bitbucket (Cloud and Server)
- Generic Git (any HTTPS/SSH/Git server)

### 🔐 Security & Compliance
- Automatic license detection and validation
- Path traversal protection
- Licenses cached for audit trails

## Installation

### Using Go

```bash
go install github.com/EmundoT/git-vendor@v1.0.0
```

### Binary Downloads

Download pre-built binaries from the [Releases](https://github.com/EmundoT/git-vendor/releases/tag/v1.0.0) page.

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Verify Installation

```bash
git-vendor --version
# git-vendor 1.0.0
```

## Quick Start

```bash
# 1. Initialize vendor directory
git-vendor init

# 2. Add a dependency (interactive wizard)
git-vendor add
# Paste: https://github.com/charmbracelet/lipgloss/tree/main/examples

# 3. Sync dependencies
git-vendor sync
# ✓ Cloning charmbracelet/lipgloss...
# ✓ Copied 5 files

# 4. Commit vendor.yml and vendor.lock
git add vendor/
git commit -m "Add lipgloss examples"
```

## Common Use Cases

### 1. Vendor Utility Files
Vendor specific utility functions from a large monorepo without pulling in the entire codebase.

### 2. Lock Third-Party Code
Track exact versions of third-party code in your repository with deterministic locking.

### 3. Side-by-Side Version Comparison
Vendor both v1.0 and v2.0 of a library to different paths for gradual migration.

### 4. CI/CD Pipelines
Use `--yes --quiet` flags for non-interactive automation with reproducible builds.

## What's New in v1.0.0

This is the first stable release with production-ready features:

### Core Vendoring
- Granular path mapping with custom destinations
- Multi-ref support (track multiple branches/tags)
- Deterministic locking with commit SHAs
- Interactive TUI with file browsers
- Path conflict detection

### Advanced Features
- Incremental sync cache (80% faster)
- Parallel processing (3-5x speedup)
- Custom hooks (pre/post sync automation)
- Vendor groups for batch operations
- Non-interactive mode for CI/CD

### CLI Commands
- `init`, `add`, `edit`, `remove`, `list` - Vendor management
- `sync`, `update` - Download and update dependencies
- `validate`, `status`, `check-updates`, `diff` - Validation and inspection
- `watch` - Auto-sync on config changes
- `completion` - Shell completion (bash/zsh/fish/powershell)

### Architecture & Quality
- Clean architecture with dependency injection
- 48.0% test coverage (critical paths 84-100%)
- Multi-OS CI testing (Ubuntu, Windows, macOS)
- 18 performance benchmarks validating all claims
- Comprehensive documentation

## Documentation

- **[README.md](../README.md)** - Getting started and feature overview
- **[docs/CONFIGURATION.md](./CONFIGURATION.md)** - Configuration reference
- **[docs/COMMANDS.md](./COMMANDS.md)** - Command reference
- **[docs/ADVANCED.md](./ADVANCED.md)** - Advanced usage and CI/CD
- **[docs/TROUBLESHOOTING.md](./TROUBLESHOOTING.md)** - Common issues and solutions
- **[docs/BENCHMARKS.md](./BENCHMARKS.md)** - Performance analysis

## Upgrading

This is the first stable release. If you're coming from pre-release versions (v0.1.0-beta.1 or earlier):

1. Update your installation:
   ```bash
   go install github.com/EmundoT/git-vendor@v1.0.0
   ```

2. No configuration changes required - `vendor.yml` and `vendor.lock` formats are unchanged

3. Verify your setup:
   ```bash
   git-vendor validate
   git-vendor status
   ```

## What's Different from Other Tools

| Feature | git-vendor | git submodule | Copy-paste |
|---------|------------|---------------|------------|
| **Granular path control** | ✅ Vendor specific files | ❌ Entire repos only | ✅ Manual selection |
| **Custom destinations** | ✅ Map to any path | ❌ Fixed paths | ✅ Manual copy |
| **Deterministic locking** | ✅ Exact commit SHAs | ⚠️ Branch/tag only | ❌ No tracking |
| **Multi-ref support** | ✅ Multiple versions | ❌ One ref per submodule | ❌ No versioning |
| **Interactive UX** | ✅ TUI wizards | ❌ Manual config | ❌ Manual work |
| **Provenance tracking** | ✅ Tracks source/version | ⚠️ Limited | ❌ Loses origin |
| **Performance** | ✅ Incremental cache | ❌ Full git ops | ✅ One-time copy |

## Known Limitations

- Branch names with slashes (e.g., `feature/foo`) cannot be parsed from URLs - use base URL and manually enter ref
- Watch mode monitors `vendor.yml` only, not remote repositories
- Incremental cache limited to 1000 files per vendor

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for solutions to common issues.

## Contributing

We welcome contributions! See [CONTRIBUTING.md](../CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](../LICENSE) for details.

## Support

- **Issues:** Report bugs at [github.com/EmundoT/git-vendor/issues](https://github.com/EmundoT/git-vendor/issues)
- **Discussions:** Ask questions at [github.com/EmundoT/git-vendor/discussions](https://github.com/EmundoT/git-vendor/discussions)
- **Documentation:** Full docs at [github.com/EmundoT/git-vendor](https://github.com/EmundoT/git-vendor)

## Acknowledgments

Built with:
- [charmbracelet/huh](https://github.com/charmbracelet/huh) - TUI forms
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) - File watching

Special thanks to all testers and contributors who helped shape this release!

---

**Ready to get started?** Check out the [Quick Start Guide](../README.md#quick-start) or run `git-vendor --help`.
