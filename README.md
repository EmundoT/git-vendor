# git-vendor

[![Tests](https://github.com/EmundoT/git-vendor/workflows/Tests/badge.svg)](https://github.com/EmundoT/git-vendor/actions)
[![codecov](https://codecov.io/gh/EmundoT/git-vendor/branch/main/graph/badge.svg)](https://codecov.io/gh/EmundoT/git-vendor)
[![Go Report Card](https://goreportcard.com/badge/github.com/EmundoT/git-vendor)](https://goreportcard.com/report/github.com/EmundoT/git-vendor)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A CLI tool for vendoring specific files/directories from Git repositories with deterministic locking.

## Table of Contents

- [The Problem](#the-problem)
- [The Solution](#the-solution)
- [Key Features](#key-features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Core Concepts](#core-concepts)
- [Common Commands](#common-commands)
- [Example Use Cases](#example-use-cases)
- [Documentation](#documentation)
- [Comparison](#comparison)
- [FAQ](#faq)
- [Contributing](#contributing)
- [License](#license)

---

## The Problem

**Vendoring entire repositories wastes space.** You often need just a handful of files from a large codebase, but git submodules force you to checkout the entire repo.

**Manual copying is error-prone and hard to track.** Copy-pasting code loses provenance - where did this file come from? What version? How do you update it?

**Package managers don't work for cross-language projects.** npm, pip, and cargo are great for their ecosystems, but what if you need utility functions from a Python repo in your Go project? Or want to vendor config files and scripts?

**Git submodules are heavyweight.** Nested `.git` directories, complex commands (`git submodule update --init --recursive`), and entire repository checkouts make submodules cumbersome for simple vendoring needs.

---

## The Solution

**git-vendor lets you vendor exactly what you need** - specific files or directories - from any Git repository, with:

- ✅ **Granular path control** - Vendor `src/utils.go`, not the entire repo
- ✅ **Deterministic locking** - `vendor.lock` ensures reproducible builds with exact commit SHAs
- ✅ **Interactive file browser** - TUI for easy path selection (no complex CLI flags)
- ✅ **Multi-platform support** - Works with GitHub, GitLab, Bitbucket, and any Git server
- ✅ **License compliance** - Automatic detection and tracking
- ✅ **Smart caching** - 80% faster re-syncs with incremental cache

**Think of it like package managers for Git files** - but language-agnostic and file-level granular.

---

## Key Features

- **Granular Path Vendoring** - Vendor specific files/directories, not entire repositories
- **Deterministic Locking** - vendor.lock ensures reproducible builds across teams and environments
- **Interactive TUI** - File browser for selecting paths (built with charmbracelet/huh)
- **Multi-Platform** - GitHub, GitLab, Bitbucket, and generic Git servers
- **Smart URL Parsing** - Paste file links directly - auto-extracts repo, branch, and path
- **License Compliance** - Automatic detection with caching for audit
- **Parallel Processing** - 3-5x speedup for multi-vendor projects with `--parallel` flag
- **Custom Hooks** - Pre/post sync automation for build workflows
- **Incremental Cache** - Skip unchanged files for faster re-syncs
- **CI/CD Ready** - Non-interactive mode with `--yes`, `--quiet`, `--json` flags

---

## Quick Start

```bash
# 1. Install
go install github.com/EmundoT/git-vendor@latest

# Works as both 'git-vendor' and 'git vendor'

# 2. Initialize vendor directory
git-vendor init

# 3. Add a dependency (interactive wizard)
git-vendor add
# Paste URL: https://github.com/owner/repo/blob/main/src/utils.go

# 4. Sync files
git-vendor sync

# Done! Files vendored to configured paths with exact commit tracking.
```

**What just happened?**

- `init` created `vendor/vendor.yml`, `vendor/vendor.lock`, and `vendor/licenses/`
- `add` launched an interactive wizard to configure path mappings
- `sync` downloaded files and locked to exact commit SHAs
- Your files are now vendored with full provenance tracking!

---

## Installation

### Build from Source

```bash
git clone https://github.com/EmundoT/git-vendor
cd git-vendor
go build -o git-vendor
sudo mv git-vendor /usr/local/bin/  # macOS/Linux
```

### As a Git Subcommand

Once installed in your PATH, git-vendor automatically works as a git subcommand:

```bash
# Both of these work identically:
git-vendor sync
git vendor sync

# All commands work with git prefix:
git vendor init
git vendor add
git vendor update
```

To install only as a git subcommand (without standalone `git-vendor`):

**Linux/macOS:**

```bash
go build -o git-vendor
sudo mv git-vendor $(git --exec-path)/git-vendor
```

**Windows:**

```powershell
go build -o git-vendor.exe
move git-vendor.exe "$(git --exec-path)\git-vendor.exe"
```

### Requirements

- Go 1.23+ (for building from source)
- Git (must be in PATH)

---

## Core Concepts

### Configuration (vendor.yml)

Defines what to vendor and where:

```yaml
vendors:
  - name: example-lib
    url: https://github.com/owner/repo
    license: MIT # Auto-detected
    specs:
      - ref: main # Branch, tag, or commit
        mapping:
          - from: src/utils.go # Remote path
            to: internal/utils.go # Local path
```

**Key fields:**

- `name` - Display name
- `url` - Git repository URL (any platform)
- `ref` - Branch, tag, or commit to track
- `mapping` - Remote → local path mappings

[Full configuration reference →](./docs/CONFIGURATION.md)

### Lock File (vendor.lock)

Stores exact commit hashes for reproducibility:

```yaml
vendors:
  - name: example-lib
    ref: main
    commit_hash: abc123def456... # Exact SHA
    updated: "2024-12-27T10:30:00Z"
```

**Auto-generated** by `update` and `sync` - never edit manually.

### Path Mapping

Map remote paths to local destinations:

```yaml
from: src/utils/ # Remote: owner/repo/src/utils/
to: lib/ # Local: your-project/lib/
```

Leave `to` empty for automatic naming based on source filename.

---

## Common Commands

**Note:** All commands can be invoked as `git vendor <command>` or `git-vendor <command>`.

| Command         | Description                                       |
| --------------- | ------------------------------------------------- |
| `init`          | Initialize vendor directory                       |
| `add`           | Add vendor dependency (interactive)               |
| `sync`          | Download files to locked versions                 |
| `update`        | Fetch latest commits, update lockfile             |
| `list`          | Show all configured vendors                       |
| `validate`      | Check for configuration errors and path conflicts |
| `status`        | Check if local files match lockfile               |
| `check-updates` | Preview available updates                         |
| `diff <vendor>` | Show commit history between locked and latest     |
| `watch`         | Auto-sync on config changes                       |

[Complete command reference →](./docs/COMMANDS.md)

### Common Options

**Sync options:**

```bash
git-vendor sync --dry-run      # Preview without changes
git-vendor sync --force        # Re-download all files
git-vendor sync --no-cache     # Disable incremental cache
git-vendor sync --parallel     # 3-5x faster for multiple vendors
git-vendor sync --group frontend  # Sync only specific group
```

**Global options:**

```bash
--verbose, -v    # Show git commands (debugging)
--version        # Show version information
--help, -h       # Show help
```

---

## Example Use Cases

### 1. Vendor Utility Functions

Copy specific utility files from another project:

```bash
git-vendor add
# URL: https://github.com/golang/go/blob/master/src/crypto/rand/util.go
# Maps to: internal/crypto/util.go
```

**Result:** Single file vendored with full provenance and license tracking.

### 2. Multi-Language Monorepo

Vendor frontend components into your backend project:

```yaml
vendors:
  - name: frontend-components
    url: https://github.com/company/monorepo
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: packages/ui/Button
            to: web/components/Button
          - from: packages/ui/Form
            to: web/components/Form
```

**Result:** Language-agnostic vendoring across your stack.

### 3. Track Multiple Versions

Maintain compatibility with old and new APIs:

```yaml
vendors:
  - name: api-client
    url: https://github.com/company/api
    specs:
      - ref: v1.0 # Stable API
        mapping:
          - from: client
            to: vendor/api-v1
      - ref: v2.0 # New API
        mapping:
          - from: client
            to: vendor/api-v2
```

**Result:** Side-by-side version comparison and gradual migration.

### 4. Automation with Hooks

Build vendored code after sync:

```yaml
vendors:
  - name: ui-library
    url: https://github.com/company/ui
    hooks:
      post_sync: |
        cd vendor/ui
        npm install
        npm run build
```

**Result:** Zero-manual-step workflow - sync triggers build automatically.

[More examples →](./examples/)

---

## Screenshot

![Interactive TUI](./docs/images/tui-placeholder.png)

_Interactive file browser for selecting paths to vendor_

> Note: TUI screenshot will be added in a future release

---

## Documentation

- **[Commands Reference](./docs/COMMANDS.md)** - All 13 commands with examples and options
- **[Platform Support](./docs/PLATFORMS.md)** - GitHub, GitLab, Bitbucket, generic Git details
- **[Advanced Usage](./docs/ADVANCED.md)** - Hooks, groups, parallel processing, caching, watch mode
- **[Configuration Reference](./docs/CONFIGURATION.md)** - vendor.yml and vendor.lock format
- **[Architecture](./docs/ARCHITECTURE.md)** - Technical design and extension points
- **[Troubleshooting](./docs/TROUBLESHOOTING.md)** - Common issues and solutions
- **[Contributing](./CONTRIBUTING.md)** - Development setup and guidelines
- **[Changelog](./CHANGELOG.md)** - Release history

---

## Comparison

### vs. Git Submodules

| Feature            | git-vendor                    | Git Submodules                    |
| ------------------ | ----------------------------- | --------------------------------- |
| **Granular paths** | ✅ Specific files/directories | ❌ Entire repository              |
| **Nested repos**   | ✅ Plain copies (no `.git`)   | ❌ Nested `.git` dirs             |
| **Easy updates**   | ✅ `git-vendor update`        | ⚠️ Complex `git submodule update` |
| **Installation**   | ⚠️ Separate tool              | ✅ Built into Git                 |

**Best for:** Vendoring specific files from larger repos

### vs. Package Managers (npm, pip, cargo, go modules)

| Feature                   | git-vendor                    | Package Managers               |
| ------------------------- | ----------------------------- | ------------------------------ |
| **Language-agnostic**     | ✅ Any Git repo               | ❌ Language-specific           |
| **Non-packaged code**     | ✅ Snippets, configs, scripts | ❌ Requires published packages |
| **Granular control**      | ✅ File-level                 | ⚠️ Package-level               |
| **Dependency resolution** | ❌ No automatic resolution    | ✅ Automatic                   |

**Best for:** Cross-language projects, non-packaged code

### vs. Manual Copying

| Feature              | git-vendor                   | Manual Copy            |
| -------------------- | ---------------------------- | ---------------------- |
| **Reproducibility**  | ✅ Lockfile with commit SHAs | ❌ No version tracking |
| **Easy updates**     | ✅ One command               | ❌ Manual re-copy      |
| **Provenance**       | ✅ Tracks source and version | ❌ Loses origin        |
| **License tracking** | ✅ Automatic caching         | ❌ Manual              |
| **Setup**            | ⚠️ Requires tool             | ✅ None                |

**Best for:** Anyone copying files manually who wants reproducibility

---

## FAQ

**Q: How is this different from git submodules?**
A: Submodules vendor entire repos; git-vendor vendors specific files/directories with granular control. Plus, no nested `.git` directories - vendored files are plain copies.

**Q: Can I vendor from private repositories?**
A: Yes! Set `GITHUB_TOKEN` or `GITLAB_TOKEN` environment variables for private GitHub/GitLab repos. For generic Git, use SSH keys.

**Q: How do I update dependencies?**
A: Run `git-vendor update` to fetch latest commits and update `vendor.lock`, then `git-vendor sync` to download the new versions.

**Q: Does it work with non-GitHub repositories?**
A: Yes! Supports GitHub, GitLab, Bitbucket, and any Git server (HTTPS/SSH/Git protocol).

**Q: How does license compliance work?**
A: git-vendor auto-detects licenses via API (GitHub/GitLab) or LICENSE file (others) and caches them in `vendor/licenses/` for audit. Pre-approved licenses (MIT, Apache-2.0, BSD, etc.) are automatically accepted.

**Q: Can I automate vendoring in CI/CD?**
A: Yes! Use `--yes --quiet` flags for non-interactive mode. Commit both `vendor.yml` and `vendor.lock` for deterministic CI builds. [See CI/CD guide →](./docs/ADVANCED.md#cicd-integration)

**Q: Why is re-syncing so much faster after the first time?**
A: git-vendor uses incremental caching with SHA-256 file checksums. If files haven't changed, it skips git operations entirely (80% faster). Use `--force` to bypass the cache.

**Q: Can I vendor multiple versions of the same library?**
A: Yes! Use multi-ref tracking to vendor v1.0 and v2.0 to different local paths. [See example →](#3-track-multiple-versions)

**Q: What if I need to vendor from a repository with a branch name containing slashes?**
A: Use the base repository URL in the wizard and manually enter the ref name (e.g., `feature/new-api`). Smart URL parsing doesn't support slash-containing branch names due to ambiguity.

---

## Who Should Use This?

**✅ Perfect for:**

- Projects vendoring utility functions from OSS libraries
- Cross-language teams needing dependencies from multiple ecosystems
- Teams wanting deterministic, reproducible builds
- Projects needing specific files/directories, not entire repos
- Anyone currently copying files manually
- CI/CD pipelines requiring locked dependency versions

**⚠️ Maybe Not For:**

- Large-scale package management (use language-specific package managers)
- Projects requiring automatic dependency resolution
- Teams uncomfortable with CLI tools

---

## Contributing

Contributions welcome! See [CONTRIBUTING.md](./CONTRIBUTING.md) for:

- Development setup
- Testing requirements and guidelines
- Code style guidelines
- Pull request process

**Quick start:**

```bash
git clone https://github.com/EmundoT/git-vendor
cd git-vendor
make mocks  # Generate test mocks
make test   # Run tests
make lint   # Run linter
```

---

## License

MIT License - see [LICENSE](./LICENSE) for details.

---

## Credits

Built with:

- [charmbracelet/huh](https://github.com/charmbracelet/huh) - TUI forms
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) - File watching

---

**Need help?** Check [Troubleshooting](./docs/TROUBLESHOOTING.md) or [open an issue](https://github.com/EmundoT/git-vendor/issues).
