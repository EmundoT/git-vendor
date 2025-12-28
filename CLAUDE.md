# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`git-vendor` is a CLI tool for managing vendored dependencies from Git repositories. It provides an interactive TUI for selecting specific files/directories from remote repos and syncing them to your local project with deterministic locking.

**Key Concept**: Unlike traditional vendoring tools, git-vendor allows granular path mapping - you can vendor specific files or subdirectories from a remote repository to specific locations in your project.

## Building and Running

```bash
# Build the project
go build -o git-vendor

# Run directly
./git-vendor <command>

# Or use go run
go run main.go <command>
```

## Core Architecture

### Clean Architecture with Dependency Injection

The codebase follows clean architecture principles with proper separation of concerns:

1. **main.go** - Command dispatcher and CLI interface
   - Routes commands (init, add, edit, remove, list, sync, update, validate, status, check-updates, diff, watch, completion)
   - Handles argument parsing and basic validation
   - Entry point for all user interactions
   - Version management (receives ldflags from GoReleaser, injects into version package)

2. **internal/core/** - Business logic layer (dependency injection pattern)
   - **engine.go**: `Manager` facade - public API that delegates to VendorSyncer
   - **vendor_syncer.go**: `VendorSyncer` - orchestrates all business logic
   - **git_operations.go**: `GitClient` interface - Git command operations
   - **filesystem.go**: `FileSystem` interface - File I/O operations
   - **github_client.go**: `LicenseChecker` interface - GitHub API license detection
   - **config_store.go**: `ConfigStore` interface - vendor.yml I/O
   - **lock_store.go**: `LockStore` interface - vendor.lock I/O
   - **hook_service.go**: `HookExecutor` interface - Pre/post sync shell hooks
   - **cache_store.go**: `CacheStore` interface - Incremental sync cache
   - **parallel_executor.go**: `ParallelExecutor` - Worker pool for concurrent vendor processing
   - **diff_service.go**: Diff service - Commit comparison between locked and latest versions
   - **watch_service.go**: Watch service - File monitoring for auto-sync on config changes
   - **update_checker.go**: Update checker - Check for available updates without modifying files
   - **mocks_test.go**: Mock implementations for testing

3. **internal/tui/wizard.go** - Interactive user interface
   - Built with charmbracelet/huh (form library) and lipgloss (styling)
   - Multi-step wizards for add/edit operations
   - File browser for both remote (via git ls-tree) and local directories
   - Path mapping management interface

4. **internal/types/types.go** - Data models
   - VendorConfig, VendorSpec, BranchSpec, PathMapping
   - VendorLock, LockDetails
   - PathConflict

5. **internal/version/version.go** - Version management
   - Receives version information from main.go (injected via ldflags)
   - GetVersion() returns version string
   - GetFullVersion() returns version with build info

### Data Model (internal/types/types.go)

```text
VendorConfig (vendor.yml)
  └─ VendorSpec (one per dependency)
      ├─ Name: display name
      ├─ URL: git repository URL
      ├─ License: SPDX license identifier
      ├─ Groups: []string (optional group tags for batch operations)
      ├─ Hooks: *HookConfig (optional pre/post sync automation)
      │   ├─ PreSync: shell command to run before sync
      │   └─ PostSync: shell command to run after sync
      └─ Specs: []BranchSpec (can track multiple refs)
          └─ BranchSpec
              ├─ Ref: branch/tag/commit
              ├─ DefaultTarget: optional default destination
              └─ Mapping: []PathMapping
                  └─ PathMapping
                      ├─ From: remote path
                      └─ To: local path (empty = auto)

VendorLock (vendor.lock)
  └─ LockDetails (one per ref per vendor)
      ├─ Name: vendor name
      ├─ Ref: branch/tag
      ├─ CommitHash: exact commit SHA
      ├─ LicensePath: path to cached license
      └─ Updated: timestamp
```

### File System Structure

All vendor-related files live in `./vendor/`:

```text
vendor/
├── vendor.yml       # Configuration file
├── vendor.lock      # Lock file with commit hashes
└── licenses/        # Cached license files
    └── {name}.txt
```

Vendored files are copied to paths specified in the configuration (outside vendor/ directory).

## Key Operations

### sync vs update

- **sync**: Fetches dependencies at locked commit hashes (deterministic)
  - If no lockfile exists, runs `update` first
  - Uses `--depth 1` for shallow clones when possible
  - Supports `--dry-run` flag for preview

- **update**: Fetches latest commits and regenerates lockfile
  - Updates all vendors to latest available commit on their configured ref
  - Rewrites entire lockfile
  - Downloads and caches license files

### Smart URL Parsing

The `ParseSmartURL` function (git_operations.go:183) extracts repository, ref, and path from GitHub URLs:

- `github.com/owner/repo` → base URL, no ref, no path
- `github.com/owner/repo/blob/main/path/to/file.go` → base URL, "main", "path/to/file.go"
- `github.com/owner/repo/tree/v1.0/src/` → base URL, "v1.0", "src/"

**Limitation**: Branch names with slashes (e.g., `feature/foo`) cannot be parsed from URLs due to regex ambiguity. Use base URL and manually enter ref in wizard.

### Remote Directory Browsing

The `FetchRepoDir` function (vendor_syncer.go:632) browses remote repository contents without full checkout:

1. Clone with `--filter=blob:none --no-checkout --depth 1`
2. Fetch specific ref if needed
3. Use `GitClient.ListTree()` which runs `git ls-tree` to list directory contents
4. 30-second timeout protection via context

### License Compliance

Automatic license detection via `GitHubLicenseChecker` (github_client.go:33):

- Queries GitHub API `/repos/:owner/:repo/license` endpoint
- Allowed by default: MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC, Unlicense, CC0-1.0
- Other licenses prompt user confirmation via `tui.AskToOverrideCompliance()`
- License files are automatically copied to `vendor/licenses/{name}.txt`

### Path Traversal Protection

Security validation via `ValidateDestPath` (filesystem.go:121):

- Rejects absolute paths (e.g., `/etc/passwd`, `C:\Windows\System32`)
- Rejects parent directory references (e.g., `../../../etc/passwd`)
- Only allows relative paths within project directory
- Called before all file copy operations in `vendor_syncer.go`

### Parallel Processing (Phase 8)

Worker pool-based parallel processing for multi-vendor operations via `ParallelExecutor` (parallel_executor.go):

**Features:**
- Worker pool pattern with configurable worker count
- Default workers: runtime.NumCPU() (limited to max 8)
- Thread-safe git operations (unique temp dirs per vendor)
- Thread-safe lockfile writes (collect all results, write once)
- Progress tracking via channel aggregation
- `--parallel` flag enables parallel mode
- `--workers <N>` flag sets custom worker count

**Usage:**

```bash
# Parallel sync (default workers = NumCPU)
git-vendor sync --parallel

# Parallel sync with 4 workers
git-vendor sync --parallel --workers 4

# Parallel update
git-vendor update --parallel
```

**Implementation:**
- `ParallelExecutor` with `ExecuteParallelSync()` and `ExecuteParallelUpdate()` methods
- Worker goroutines process vendors from job channel
- Results collected via channel and aggregated
- First error returned (fail-fast behavior)

**Performance:**
- 3-5x speedup for multi-vendor operations
- No performance penalty for single vendor
- Automatically disabled for dry-run mode

**Thread Safety:**
- Git operations use unique temp directories per vendor
- Lockfile collected from results and written once at end
- File operations protected by filesystem guarantees
- No shared mutable state between workers

**Testing:**
- Passes `go test -race` with no race conditions
- All 55 existing tests continue to pass
- Backwards compatible (opt-in via `--parallel` flag)

### Custom Hooks (Phase 8)

Pre and post-sync shell command execution via `HookExecutor` (hook_service.go):

**Features:**
- Pre-sync hooks run before git clone/sync operations
- Post-sync hooks run after successful sync completion
- Environment variable injection for hook context
- Executed via `sh -c` for full shell support (pipes, multiline, etc.)
- Runs in project root directory with current user permissions

**Configuration Example:**

```yaml
vendors:
  - name: frontend-lib
    url: https://github.com/owner/lib
    license: MIT
    hooks:
      pre_sync: echo "Preparing to sync frontend-lib..."
      post_sync: |
        npm install
        npm run build
    specs:
      - ref: main
        mapping:
          - from: src/
            to: vendor/frontend-lib/
```

**Environment Variables Provided to Hooks:**
- `GIT_VENDOR_NAME`: Vendor name
- `GIT_VENDOR_URL`: Repository URL
- `GIT_VENDOR_REF`: Git ref being synced
- `GIT_VENDOR_COMMIT`: Resolved commit hash
- `GIT_VENDOR_ROOT`: Project root directory
- `GIT_VENDOR_FILES_COPIED`: Number of files copied

**Behavior:**
- Pre-sync hook failure stops the sync operation (entire vendor skipped)
- Post-sync hook failure fails the sync (files already copied but operation marked failed)
- Hook output is displayed directly to stdout/stderr
- Hooks run even for cache hits (where git clone is skipped)

**Security Considerations:**
- Hooks execute arbitrary shell commands with user's permissions
- No sandboxing or privilege restrictions
- Users control hook commands via vendor.yml (acceptable - same trust model as package.json scripts)
- Commands run in project root, cannot escape to parent directories via cd
- Similar security model to npm scripts, git hooks, or Makefile targets

## Common Patterns

### Error Handling

- TUI functions use `check(err)` helper that prints "Aborted." and exits
- Core functions return errors for caller handling
- CLI prints styled errors via `tui.PrintError(title, message)`

### Wizard Flow

1. User inputs URL (validates GitHub URLs only)
2. ParseSmartURL extracts components
3. Check if repo already tracked → offer to edit existing
4. Collect name and ref
5. If deep link provided, offer to use that path
6. Enter edit loop for path mapping
7. Save triggers `UpdateAll()` which regenerates lockfile

### Git Operations

Git operations use the `GitClient` interface (git_operations.go):

- `SystemGitClient` implements `GitClient` for production
- Methods: `Init`, `AddRemote`, `Fetch`, `FetchAll`, `Checkout`, `GetHeadHash`, `Clone`, `ListTree`
- Internal `run()` method executes git commands via `exec.Command`
- Verbose mode logs commands to stderr when `--verbose` flag is used
- Temp directories cleaned up with `defer fs.RemoveAll(tempDir)`

## Development Notes

### Development Practices

- Follows Go conventions with proper package structure
- Uses Go modules for dependency management
- Comprehensive unit tests with mocks for all interfaces
- Clear separation of concerns for maintainability
- Uses context with timeouts for external operations
- Proper error propagation and handling
- Consistent naming conventions and code style
- Modular functions with single responsibility
- Extensive comments and documentation
- Meaningful commit messages, use Conventional Commits format
- Logical branching strategy for features and fixes

### Test Coverage

The codebase has **63.9% test coverage** with comprehensive tests:

**Test Infrastructure:**

- Auto-generated mocks using MockGen (gomock framework)
- Test files organized by service (9 focused test files)
- `testhelpers_gomock_test.go`: Gomock setup helpers
- `testhelpers.go`: Common test utilities

**Well-Tested Areas:**

- syncVendor: 89.7% coverage (15 test cases)
- UpdateAll: 100% coverage (10 test cases)
- Sync/SyncDryRun/SyncWithOptions: 100% coverage (12 test cases)
- FetchRepoDir: 84.6% coverage
- SaveVendor/RemoveVendor: 100% coverage
- ValidateConfig: 95.7% coverage (11 test cases)
- DetectConflicts: 86.1% coverage
- Config/Lock I/O: 100% coverage (13 test cases)

**Running Tests:**

Tests use auto-generated mocks via MockGen. The mocks are automatically generated and should not be committed to git.

```bash
# Generate mocks (required on first run or after interface changes)
# On Unix/Mac/Linux:
make mocks

# On Windows (or if make is not available):
go install github.com/golang/mock/mockgen@latest
go run -exec "$(go env GOPATH)/bin/mockgen" -source=internal/core/git_operations.go -destination=internal/core/git_client_mock_test.go -package=core
go run -exec "$(go env GOPATH)/bin/mockgen" -source=internal/core/filesystem.go -destination=internal/core/filesystem_mock_test.go -package=core
go run -exec "$(go env GOPATH)/bin/mockgen" -source=internal/core/config_store.go -destination=internal/core/config_store_mock_test.go -package=core
go run -exec "$(go env GOPATH)/bin/mockgen" -source=internal/core/lock_store.go -destination=internal/core/lock_store_mock_test.go -package=core
go run -exec "$(go env GOPATH)/bin/mockgen" -source=internal/core/github_client.go -destination=internal/core/license_checker_mock_test.go -package=core

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./internal/core

# Run tests verbosely
go test -v ./...
```

**Note:** Mock files (`*_mock_test.go`) are auto-generated and git-ignored. Generate them locally before running tests.

### Dependencies

**Runtime:**
- `github.com/charmbracelet/huh` - TUI forms
- `github.com/charmbracelet/lipgloss` - styling
- `gopkg.in/yaml.v3` - config file parsing
- `github.com/fsnotify/fsnotify` - file system watching (watch mode)

**Testing:**
- `github.com/golang/mock` - Mock generation (gomock/mockgen)

**Environment Variables (Optional):**
- `GITHUB_TOKEN` - GitHub personal access token (increases rate limit from 60/hr to 5000/hr)
- `GITLAB_TOKEN` - GitLab personal access token (enables private repos and higher rate limits)

### Concurrency Considerations

- Git operations use 30-second timeout contexts for directory listing
- Optional parallel vendor processing via --parallel flag
  - Worker pool pattern with configurable worker count
  - Default workers: runtime.NumCPU() (max 8)
  - Thread-safe git operations (unique temp dirs per vendor)
  - Thread-safe lockfile writes (collect results, write once at end)
  - Automatically disabled for dry-run mode
- File copying is synchronous

## Gotchas

1. **Multi-platform support**: Smart URL parsing works with GitHub, GitLab, Bitbucket, and generic git
2. **License detection**: GitHub and GitLab use API, others read LICENSE file from repo
3. **Platform auto-detection**: Provider automatically detected from URL
4. **Self-hosted support**: Works with self-hosted GitLab and GitHub Enterprise
5. **GitLab nested groups**: Supports unlimited depth (owner/group/subgroup/repo)
6. **Shallow clones**: Uses `--depth 1` which may fail for old locked commits (falls back to full fetch)
7. **License location**: Fallback checks LICENSE, LICENSE.txt, LICENSE.md, COPYING in repository root
8. **Path mapping**: Empty destination uses auto-naming based on source basename
9. **Edit mode**: Changes aren't saved until user selects "💾 Save & Exit"
10. **.md gotchas**: All ````` blocks must have a language specifier (e.g. ``````yaml) to render correctly, use text for the UI and in lieu of nothing
11. **Branch names with slashes**: Cannot parse from URL due to ambiguity - use base URL and enter ref manually
12. **Incremental sync cache**: Stored in vendor/.cache/, auto-invalidates on commit hash changes, 1000 file limit per vendor
13. **Hook execution**: Hooks run in project root with full shell support (sh -c), runs even for cache hits, same security model as npm scripts
14. **Parallel processing**: Auto-disabled for dry-run mode, worker count defaults to NumCPU (max 8), thread-safe operations
15. **Watch mode**: 1-second debounce for rapid changes, watches vendor.yml only, re-runs full sync on changes

## Quick Reference

### Available Commands

```bash
git-vendor init                      # Initialize vendor directory
git-vendor add                       # Add vendor (interactive)
git-vendor edit                      # Edit vendor (interactive)
git-vendor remove <name>             # Remove vendor
git-vendor list                      # List all vendors
git-vendor sync [options] [vendor]   # Sync dependencies
git-vendor update [options]          # Update lockfile
git-vendor validate                  # Validate config and detect conflicts
git-vendor status                    # Check if local files match lockfile
git-vendor check-updates             # Preview available updates
git-vendor diff <vendor>             # Show commit history between locked and latest
git-vendor watch                     # Auto-sync on config changes
git-vendor completion <shell>        # Generate shell completion (bash/zsh/fish/powershell)
```

### Sync Command Flags

```bash
--dry-run         # Preview without changes
--force           # Re-download even if synced
--no-cache        # Disable incremental sync cache
--group <name>    # Sync only vendors in specified group
--parallel        # Enable parallel processing
--workers <N>     # Set custom worker count (requires --parallel)
--verbose, -v     # Show git commands
<vendor-name>     # Sync only specified vendor
```

### Update Command Flags

```bash
--parallel        # Enable parallel processing
--workers <N>     # Set custom worker count (requires --parallel)
--verbose, -v     # Show git commands
```

### File Paths

- Config: `vendor/vendor.yml`
- Lock: `vendor/vendor.lock`
- Licenses: `vendor/licenses/<name>.txt`
- Vendored files: User-specified paths (outside vendor/)

### Important Functions by File

**vendor_syncer.go:**

- `syncVendor()` - Core sync logic for single vendor
- `UpdateAll()` - Update all vendors, regenerate lockfile
- `DetectConflicts()` - Find path conflicts between vendors
- `ValidateConfig()` - Comprehensive config validation

**git_operations.go:**

- `ParseSmartURL()` - Extract repo/ref/path from GitHub URLs
- `GitClient.ListTree()` - Browse remote directories via git ls-tree

**github_client.go:**

- `CheckLicense()` - Query GitHub API for license
- `IsAllowed()` - Validate against allowed licenses

**filesystem.go:**

- `ValidateDestPath()` - Security check for path traversal
- `CopyFile()` / `CopyDir()` - File operations
