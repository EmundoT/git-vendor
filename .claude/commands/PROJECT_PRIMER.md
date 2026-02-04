# git-vendor Project Primer

## Overview

git-vendor is a Go CLI tool for deterministic, file-level source code vendoring from any Git repository. Unlike traditional vendoring tools, it allows granular path mapping - you can vendor specific files or subdirectories from a remote repository to specific locations in your project.

**Key Differentiator**: While most vendoring pulls entire packages, git-vendor lets you cherry-pick exactly what you need with full provenance tracking.

---

## Core Concepts

### Vendoring Hierarchy

```
VendorConfig (vendor/vendor.yml)
└── VendorSpec (one per dependency)
    ├── Name: display name
    ├── URL: git repository URL
    ├── License: SPDX license identifier
    ├── Groups: []string (optional group tags for batch operations)
    ├── Hooks: *HookConfig (optional pre/post sync automation)
    └── Specs: []BranchSpec (can track multiple refs)
        └── BranchSpec
            ├── Ref: branch/tag/commit
            ├── DefaultTarget: optional default destination
            └── Mapping: []PathMapping
                └── PathMapping
                    ├── From: remote path
                    └── To: local path (empty = auto)
```

- **VendorSpec**: Configuration for a single external dependency
- **BranchSpec**: Tracks a specific git ref (branch, tag, or commit)
- **PathMapping**: Maps remote file/directory paths to local destinations

### Data Model

| File | Purpose |
|------|---------|
| `vendor/vendor.yml` | Configuration file - defines what to vendor and where |
| `vendor/vendor.lock` | Lock file with exact commit hashes for reproducibility |
| `vendor/licenses/` | Cached license files for each dependency |

### sync vs update

| Operation | Behavior | Use When |
|-----------|----------|----------|
| **sync** | Fetches dependencies at locked commit hashes | Normal workflow, CI/CD |
| **update** | Fetches latest commits and regenerates lockfile | Want latest versions |

---

## Key Terminology

| Term | Definition |
|------|------------|
| **Vendor** | An external dependency being tracked |
| **Ref** | Git reference (branch name, tag, or commit SHA) |
| **Path Mapping** | Defines how files move from source repo to local project |
| **Lockfile** | Exact commit SHAs for deterministic builds |
| **Deep Link** | GitHub/GitLab URL that includes branch and path information |
| **Groups** | Tags for organizing vendors into logical batches |
| **Hooks** | Shell commands that run before/after sync operations |

---

## Architecture Quick Reference

### Directory Structure

```
git-vendor/
├── main.go                    # CLI entry point, command routing
├── internal/
│   ├── core/                  # Business logic layer
│   │   ├── engine.go          # Manager facade (public API)
│   │   ├── vendor_syncer.go   # Core sync orchestration
│   │   ├── git_operations.go  # Git command interface
│   │   ├── filesystem.go      # File I/O interface
│   │   ├── github_client.go   # GitHub API license detection
│   │   ├── config_store.go    # vendor.yml I/O interface
│   │   ├── lock_store.go      # vendor.lock I/O interface
│   │   ├── hook_service.go    # Pre/post sync hook execution
│   │   ├── cache_store.go     # Incremental sync cache
│   │   ├── parallel_executor.go # Worker pool for concurrency
│   │   └── *_test.go          # Tests with mocks
│   ├── tui/                   # Interactive user interface
│   │   └── wizard.go          # Multi-step wizards (charmbracelet/huh)
│   ├── types/                 # Data models
│   │   └── types.go           # VendorConfig, VendorSpec, etc.
│   └── version/               # Version management
│       └── version.go         # Build info injection
├── vendor/                    # Vendor directory (created by tool)
│   ├── vendor.yml
│   ├── vendor.lock
│   └── licenses/
└── docs/                      # Documentation
    └── ROADMAP.md             # Development roadmap
```

### Key Interfaces (Dependency Injection)

| Interface | Implementation | Purpose |
|-----------|---------------|---------|
| `GitClient` | `SystemGitClient` | Git command execution |
| `FileSystem` | `OSFileSystem` | File I/O operations |
| `LicenseChecker` | `GitHubLicenseChecker` | License API queries |
| `ConfigStore` | `YAMLConfigStore` | vendor.yml read/write |
| `LockStore` | `YAMLLockStore` | vendor.lock read/write |
| `HookExecutor` | `ShellHookExecutor` | Hook command execution |
| `CacheStore` | `FileCacheStore` | Incremental sync cache |
| `ParallelExecutor` | Implementation | Worker pool coordination |

---

## Commands Reference

| Command | Purpose |
|---------|---------|
| `git-vendor init` | Initialize vendor directory |
| `git-vendor add` | Add vendor (interactive TUI) |
| `git-vendor edit` | Edit vendor (interactive TUI) |
| `git-vendor remove <name>` | Remove vendor |
| `git-vendor list` | List all vendors |
| `git-vendor sync [options] [vendor]` | Sync dependencies at locked versions |
| `git-vendor update [options]` | Update lockfile to latest |
| `git-vendor validate` | Validate config and detect conflicts |
| `git-vendor status` | Check if local files match lockfile |
| `git-vendor check-updates` | Preview available updates |
| `git-vendor diff <vendor>` | Show commit history between locked and latest |
| `git-vendor watch` | Auto-sync on config changes |
| `git-vendor completion <shell>` | Generate shell completion |

### Common Flags

```bash
--dry-run         # Preview without changes
--force           # Re-download even if synced
--no-cache        # Disable incremental sync cache
--group <name>    # Sync only vendors in specified group
--parallel        # Enable parallel processing
--workers <N>     # Set custom worker count (requires --parallel)
--verbose, -v     # Show git commands
```

---

## Development Patterns

### Testing

```bash
# Generate mocks (required before running tests)
make mocks

# Run all tests
go test ./...

# Run with coverage
go test -cover ./internal/core

# Run with race detector
go test -race ./...
```

**Mock Pattern**: Uses gomock for interface mocking. Mock files are auto-generated and git-ignored.

### Building

```bash
# Build optimized binary (recommended)
make build

# Build development binary (with debug symbols)
make build-dev

# Run directly
go run main.go <command>
```

### Code Patterns

- **Error Handling**: TUI uses `check(err)` helper; core returns errors to callers
- **Security**: Path traversal protection via `ValidateDestPath()`
- **Concurrency**: Worker pool pattern for parallel vendor processing
- **Caching**: Incremental sync cache in `vendor/.cache/`

---

## Common Workflows

### Add a Vendor from GitHub URL

```bash
# Interactive wizard
git-vendor add

# Enter URL: https://github.com/owner/repo/blob/main/path/to/file.go
# Tool parses: base URL, ref (main), path (path/to/file.go)
```

### Sync in CI/CD

```bash
# Deterministic sync (uses lockfile)
git-vendor sync

# Parallel sync for faster builds
git-vendor sync --parallel --workers 4
```

### Update to Latest

```bash
# Update all vendors to latest and regenerate lockfile
git-vendor update

# Check what would update without making changes
git-vendor check-updates
```

---

## Configuration Examples

### vendor.yml

```yaml
vendors:
  - name: frontend-lib
    url: https://github.com/owner/lib
    license: MIT
    groups:
      - frontend
    hooks:
      pre_sync: echo "Preparing to sync..."
      post_sync: npm run build
    specs:
      - ref: main
        mapping:
          - from: src/
            to: vendor/frontend-lib/
```

### vendor.lock

```yaml
vendors:
  - name: frontend-lib
    ref: main
    commit: abc123def456...
    updated: 2026-02-04T10:30:00Z
    license_path: vendor/licenses/frontend-lib.txt
```

---

## Quick Troubleshooting

| Issue | Solution |
|-------|----------|
| "Permission denied" on sync | Check file permissions, run with sudo if needed |
| "Branch not found" | Verify ref exists in remote repo |
| "License not detected" | Add LICENSE file to source repo or use `--skip-license` |
| Shallow clone failure | Old locked commits may need full fetch (automatic fallback) |
| Hook failures | Hooks run in project root; check command paths |

---

## Roadmap Awareness

The project has an active roadmap in `docs/ROADMAP.md` covering:

**Phase 1** (Foundation): Schema versioning, verify hardening, metadata enrichment
**Phase 2** (Supply Chain Intelligence): SBOM generation, CVE scanning, drift detection
**Phase 3** (Ecosystem Integration): Audit command, dependency graph, GitHub Action

When implementing features, check the roadmap for:
- Architectural guidelines
- Feature dependencies
- Testing requirements
- Success metrics

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `GITHUB_TOKEN` | Increases GitHub API rate limits, enables private repos |
| `GITLAB_TOKEN` | Enables GitLab private repos, increases rate limits |

---

## Design Principles

1. **Offline-First**: Every command except sync/fetch works without network
2. **Lockfile Is Truth**: All state derives from vendor.lock
3. **Incremental, Not Big-Bang**: Each feature ships as standalone subcommand
4. **Don't Break Anything**: Lockfile format remains backward-compatible
5. **Output Is the Product**: Features produce shareable, actionable output
