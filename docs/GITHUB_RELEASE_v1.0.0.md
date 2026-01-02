# 🎉 git-vendor v1.0.0 - First Stable Release

**Production-ready CLI tool for vendoring specific files/directories from Git repositories with deterministic locking.**

Unlike traditional vendoring tools that copy entire repositories, git-vendor gives you granular control over exactly what gets vendored and where it goes.

## ✨ Key Features

- 🎯 **Granular Path Control** - Vendor specific files, not entire repos
- 🔒 **Deterministic Locking** - Exact commit SHA tracking in `vendor.lock`
- 🎨 **Interactive TUI** - Wizard-based interface with file browser
- ⚡ **High Performance** - 80% faster re-syncs with incremental cache, 3-5x speedup with parallel processing
- 🔧 **CI/CD Ready** - Non-interactive flags, custom hooks, vendor groups
- 🌐 **Multi-Platform** - GitHub, GitLab, Bitbucket, and generic Git support
- 🔐 **Secure** - Automatic license detection and path traversal protection

## 🚀 Quick Start

```bash
# Install
go install github.com/EmundoT/git-vendor@v1.0.0

# Initialize and add a dependency
git-vendor init
git-vendor add  # Interactive wizard

# Sync dependencies
git-vendor sync
```

## 📦 What's Included

### Core Commands
- `init`, `add`, `edit`, `remove`, `list` - Vendor management
- `sync`, `update` - Download and update dependencies
- `validate`, `status`, `check-updates`, `diff` - Validation and inspection
- `watch` - Auto-sync on config changes
- `completion` - Shell completion (bash/zsh/fish/powershell)

### Advanced Features
- Incremental sync cache (80% faster re-syncs)
- Parallel processing (3-5x speedup with `--parallel`)
- Custom pre/post sync hooks for automation
- Vendor groups for batch operations
- Multi-ref support (track multiple versions)
- Dry-run mode (`--dry-run`)

## 📊 Project Statistics

- **Production Code:** ~7,000 LOC
- **Test Code:** ~6,700 LOC (1:1 ratio)
- **Test Coverage:** 48.0% (critical paths 84-100%)
- **Binary Size:** 7.2 MB (optimized)
- **Platforms:** Linux, macOS, Windows (amd64, arm64)
- **Code Quality:** A- (91/100)

## 📚 Documentation

- [README.md](https://github.com/EmundoT/git-vendor/blob/main/README.md) - Getting started
- [Configuration Reference](https://github.com/EmundoT/git-vendor/blob/main/docs/CONFIGURATION.md)
- [Command Reference](https://github.com/EmundoT/git-vendor/blob/main/docs/COMMANDS.md)
- [Advanced Usage](https://github.com/EmundoT/git-vendor/blob/main/docs/ADVANCED.md)
- [Troubleshooting](https://github.com/EmundoT/git-vendor/blob/main/docs/TROUBLESHOOTING.md)
- [Full Release Notes](https://github.com/EmundoT/git-vendor/blob/main/docs/RELEASE_NOTES_v1.0.0.md)

## 🔄 Installation Methods

### Using Go
```bash
go install github.com/EmundoT/git-vendor@v1.0.0
```

### Binary Downloads
Download pre-built binaries for your platform from the assets below.

### Verify Installation
```bash
git-vendor --version
# git-vendor 1.0.0
```

## ⚖️ vs Other Tools

| Feature | git-vendor | git submodule | Copy-paste |
|---------|------------|---------------|------------|
| Granular path control | ✅ | ❌ | ✅ |
| Custom destinations | ✅ | ❌ | ✅ |
| Deterministic locking | ✅ | ⚠️ | ❌ |
| Multi-ref support | ✅ | ❌ | ❌ |
| Interactive UX | ✅ | ❌ | ❌ |
| Provenance tracking | ✅ | ⚠️ | ❌ |

## 🙏 Acknowledgments

Built with [charmbracelet/huh](https://github.com/charmbracelet/huh), [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss), and [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify).

Special thanks to all testers and contributors!

## 📝 Full Changelog

See [CHANGELOG.md](https://github.com/EmundoT/git-vendor/blob/main/CHANGELOG.md) for complete details.

---

**Get Started:** [Quick Start Guide](https://github.com/EmundoT/git-vendor/blob/main/README.md#quick-start) | **Questions?** [Discussions](https://github.com/EmundoT/git-vendor/discussions) | **Issues?** [Bug Reports](https://github.com/EmundoT/git-vendor/issues)
