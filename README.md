# git-vendor

[![Tests](https://github.com/EmundoT/git-vendor/workflows/Tests/badge.svg)](https://github.com/EmundoT/git-vendor/actions)
[![codecov](https://codecov.io/gh/EmundoT/git-vendor/branch/main/graph/badge.svg)](https://codecov.io/gh/EmundoT/git-vendor)
[![Go Report Card](https://goreportcard.com/badge/github.com/EmundoT/git-vendor)](https://goreportcard.com/report/github.com/EmundoT/git-vendor)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A CLI tool for vendoring specific files and directories from Git repositories with deterministic locking, license compliance, and vulnerability scanning.

## Table of Contents

- [Why git-vendor?](#why-git-vendor)
- [Feature Matrix](#feature-matrix)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Core Concepts](#core-concepts)
- [Common Commands](#common-commands)
- [Example Use Cases](#example-use-cases)
- [Documentation](#documentation)
- [Comparison](#comparison)
- [FAQ](#faq)
- [Who Should Use This?](#who-should-use-this)
- [Contributing](#contributing)
- [License](#license)
- [Credits](#credits)

---

## Why git-vendor?

You need a few files from another repository. Your options today:

| Approach | Problem |
| --- | --- |
| **Git submodules** | Checks out the entire repo, nested `.git` dirs, complex update commands |
| **Package managers** | Language-specific, requires published packages, no file-level granularity |
| **Manual copy-paste** | Loses provenance, no version tracking, no reproducibility |

**git-vendor gives you file-level vendoring with full provenance.** Vendor `src/utils.go` instead of the entire repo. Lock to exact commit SHAs for reproducible builds. Auto-detect licenses for compliance. Scan for CVEs. All language-agnostic.

```bash
git-vendor add    # Interactive wizard - paste a GitHub URL, select files
git-vendor sync   # Download files at locked versions
git-vendor verify # Confirm nothing has been tampered with
```

## Feature Matrix

| Capability | Details |
| --- | --- |
| **Granular path vendoring** | Vendor specific files, directories, or even line ranges from source files |
| **Deterministic locking** | `vendor.lock` with exact commit SHAs, file hashes (SHA-256), timestamps |
| **Multi-platform** | GitHub, GitLab (self-hosted), Bitbucket, any Git server (HTTPS/SSH) |
| **Interactive TUI** | File browser for remote repos, path mapping wizard (charmbracelet/huh) |
| **License compliance** | Auto-detection via API, caching in `.git-vendor/licenses/`, SPDX identifiers |
| **Vulnerability scanning** | CVE detection via OSV.dev, PURL-based queries, severity filtering |
| **SBOM generation** | CycloneDX 1.5 and SPDX 2.3 output for supply chain compliance |
| **Parallel processing** | 3-5x speedup with `--parallel` flag, configurable worker pool |
| **Custom hooks** | Pre/post sync shell commands with environment variable injection |
| **Incremental cache** | SHA-256 checksums skip unchanged files (~80% faster re-syncs) |
| **Position extraction** | Vendor specific line/column ranges (`file.go:L5-L20`, `file.go:L5-EOF`) |
| **CI/CD ready** | Non-interactive mode, JSON output, exit codes, deterministic builds |
| **Git subcommand** | Works as both `git-vendor` and `git vendor` |

---

## Quick Start

```bash
# 1. Install
go install github.com/EmundoT/git-vendor@latest

# Works as both 'git-vendor' and 'git vendor'

# 2. Initialize .git-vendor directory
git-vendor init

# 3. Add a dependency (interactive wizard)
git-vendor add
# Paste URL: https://github.com/owner/repo/blob/main/src/utils.go

# 4. Sync files
git-vendor sync

# Done! Files vendored to configured paths with exact commit tracking.
```

**What just happened?**

- `init` created `.git-vendor/vendor.yml`, `.git-vendor/vendor.lock`, and `.git-vendor/licenses/`
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
| `init`          | Initialize .git-vendor directory                  |
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

## Documentation

- **[Commands Reference](./docs/COMMANDS.md)** - All commands with examples and options
- **[Configuration Reference](./docs/CONFIGURATION.md)** - vendor.yml and vendor.lock format
- **[Platform Support](./docs/PLATFORMS.md)** - GitHub, GitLab, Bitbucket, generic Git details
- **[Advanced Usage](./docs/ADVANCED.md)** - Hooks, groups, parallel processing, caching, watch mode
- **[CI/CD Integration](./docs/CI_CD.md)** - GitHub Actions, GitLab CI, CircleCI pipelines
- **[FAQ](./docs/FAQ.md)** - Common questions and answers
- **[Security Policy](./SECURITY.md)** - Vulnerability reporting and security model
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
A: Submodules vendor entire repos; git-vendor vendors specific files/directories. No nested `.git` directories.

**Q: Can I vendor from private repositories?**
A: Yes. Set `GITHUB_TOKEN` or `GITLAB_TOKEN` for private repos. For generic Git, use SSH keys.

**Q: Does it work with non-GitHub repositories?**
A: Yes. Supports GitHub, GitLab, Bitbucket, and any Git server (HTTPS/SSH).

**Q: Can I automate vendoring in CI/CD?**
A: Yes. Use `--yes --quiet` for non-interactive mode. See [CI/CD Guide](./docs/CI_CD.md).

[Full FAQ →](./docs/FAQ.md)

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
