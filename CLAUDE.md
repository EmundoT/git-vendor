# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**CRITICAL: ALWAYS USE THE PRIVATE REMOTE**
ALWAYS USE THE `private` remote ex. git pull private main or git push private main

## Global Claude Code Protocol

### Definition of Done (Non-Negotiable)

Work is INCOMPLETE until ALL of the following pass with exit code zero:
1. Build succeeds
2. All tests pass
3. Linter is green (aligned with local git hooks if configured)
4. Inline documentation updated for any changed logic
5. Relevant `CLAUDE.md` entries updated if patterns/commands changed

**Self-Correction:** If build/test/lint fails, analyze the output, fix, and re-run immediately. Do NOT wait for prompting. Do NOT cheese tests — a passing test MUST be scrutinized for correctness, not tailored to pass.

### Documentation

- **Inline-first:** Documentation MUST live in the source file (language-idiomatic doc comments). Standalone docs only for staples like `README.md`.
- **RAG-ready:** MUST NOT use pronouns ("this", "it") in doc blocks. Use explicit function/module names for isolation clarity.
- **No gratuitous docs:** Every doc block MUST serve a purpose for a human or LLM. No obvious restating.
- **DRY:** Document a concept once, reference elsewhere. No duplication across files.
- **Staleness tax:** Every logic change MUST simultaneously update: inline docs, `CLAUDE.md` (if patterns changed), relevant guides.

### Architectural Guardrails

- **Negative documentation:** When an approach is rejected, document it as a "Legacy Trap" or "Non-Goal" in the project `CLAUDE.md` to prevent future regressions.
- **RFC 2119:** Use MUST, MUST NOT, SHOULD in all planning and technical feedback.
- **DRY/SOLID:** Propose a refactor before adding new features on top of messy or duplicated logic.

### Communication Style

- High-context, low-prose. I'm a dev — no "codebrain translating."
- No hedging. No "Based on my analysis..." — just the plan or the code.
- ALWAYS label code blocks with a language/format tag, even if just ` ```text `.
- Present options with numbered lists. When multiple approaches exist, present them with tradeoffs.
- Label response sections by topic (Section A, Point B) for easy reference and quick replies.
- Each actionable content block (code, commands) gets its own labeled code block.
- When a topic veers significantly off-task, recommend spinning it off to a new context with a ready-to-paste prompt that carries forward the relevant prior context.

### Context Conservation

- **Pseudoscript (preference: 6/10):** In long conversations, progressively shorthand repeated concepts, terms, and patterns using programmer-friendly abbreviations. Build a running glossary. Adapt density to topic — technical problem-solving ramps up; straightforward tasks stay clean. Full spec: `/pseudoscript`
- **Evolving instructions:** For complex multi-step tasks, consolidate the full spec into an idempotent instruction set before executing. Present it as a contract. Full protocol: `/instructions-improve`

### CLAUDE.md Stewardship

This file is a living document. Update it immediately (without prompting) when:
- A new project-level pattern, command, or convention is established
- A "Legacy Trap" is discovered
- A pseudoscript convention proves durable enough to persist across projects

Proactively prune stale or superseded entries during task completion.

### Tooling Preferences

- You MAY suggest installing CLI tools. MUST explain: what it is, what you'll use it for, security considerations.
- After completing a task, assess if the workflow should be scripted for reuse. If so, propose it. Track project scripts in the project `CLAUDE.md`.
- MCP servers SHOULD provide capabilities beyond what CLI tools offer. Prefer CLI for git, GitHub, Docker, etc.

## Project Overview

`git-vendor` is a CLI tool for managing vendored dependencies from Git repositories. It provides an interactive TUI for selecting specific files/directories from remote repos and syncing them to your local project with deterministic locking.

**Key Concept**: Unlike traditional vendoring tools, git-vendor allows granular path mapping - you can vendor specific files or subdirectories from a remote repository to specific locations in your project.

## Building and Running

```bash
# Build optimized binary (recommended - 34% smaller)
make build

# Build development binary (with debug symbols)
make build-dev

# Or build manually
go build -ldflags="-s -w" -o git-vendor  # Optimized
go build -o git-vendor                    # Debug build

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

   - **engine.go**: `Manager` facade - public API that delegates to service layer
   - **vendor_syncer.go**: `VendorSyncer` - top-level orchestrator, delegates to services below
   - **sync_service.go**: `SyncService` - sync logic (fetchWithFallback, canSkipSync, updateCache)
   - **update_service.go**: `UpdateService` - update lockfile, compute file hashes, parallel updates
   - **file_copy_service.go**: `FileCopyService` - position-aware file copy, local modification detection
   - **verify_service.go**: `VerifyService` - verification against lockfile hashes + position-level checks
   - **validation_service.go**: `ValidationService` - config validation, conflict detection
   - **position_extract.go**: Position extraction and placement (ExtractPosition, PlaceContent)
   - **git_operations.go**: `GitClient` interface - Git command operations
   - **filesystem.go**: `FileSystem` interface - File I/O operations, CopyStats, ValidateDestPath
   - **github_client.go**: `LicenseChecker` interface - GitHub API license detection
   - **config_store.go**: `ConfigStore` interface - vendor.yml I/O
   - **lock_store.go**: `LockStore` interface - vendor.lock I/O
   - **hook_service.go**: `HookExecutor` interface - Pre/post sync shell hooks
   - **cache_store.go**: `CacheStore` interface - Incremental sync cache
   - **parallel_executor.go**: `ParallelExecutor` - Worker pool for concurrent vendor processing
   - **diff_service.go**: Diff service - Commit comparison between locked and latest versions
   - **watch_service.go**: Watch service - File monitoring for auto-sync on config changes
   - **update_checker.go**: Update checker - Check for available updates without modifying files
   - **constants.go**: Path constants (`ConfigPath`, `LockPath`), git refs, license lists
   - **errors.go**: Sentinel errors and structured error types (see Error Handling)
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
   - Version information injected directly via ldflags during builds
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
      ├─ Updated: timestamp
      └─ Positions: []PositionLock (omitempty)
          └─ PositionLock
              ├─ From: source path with position (e.g., "api/constants.go:L4-L6")
              ├─ To: destination path with optional position
              └─ SourceHash: SHA-256 of extracted content

PositionSpec (internal/types/position.go)
  ├─ StartLine: int (1-indexed)
  ├─ EndLine: int (1-indexed, 0 = same as StartLine)
  ├─ StartCol: int (1-indexed, 0 = no column)
  ├─ EndCol: int (1-indexed inclusive, 0 = no column)
  └─ ToEOF: bool (true = extract to end of file)
```

### File System Structure

All vendor-related files live in `./.git-vendor/`:

```text
.git-vendor/
├── vendor.yml       # Configuration file
├── vendor.lock      # Lock file with commit hashes
└── licenses/        # Cached license files
    └── {name}.txt
```

Vendored files are copied to paths specified in the configuration (outside .git-vendor/ directory).

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
- License files are automatically copied to `.git-vendor/licenses/{name}.txt`

### Position Extraction (Spec 071)

Fine-grained file vendoring — extract specific line/column ranges from source files and place them at specific positions in destination files.

**Syntax** (appended to path with `:`):

```text
file.go:L5          # Single line 5
file.go:L5-L20      # Lines 5 through 20
file.go:L5-EOF      # Line 5 to end of file
file.go:L5C10:L10C30  # Line 5 col 10 through line 10 col 30 (1-indexed inclusive)
```

**Pipeline:**

1. `ParsePathPosition()` splits `path:Lspec` into file path + `PositionSpec`
2. `ExtractPosition()` reads file, normalizes CRLF→LF, extracts content, returns content + SHA-256 hash
3. `PlaceContent()` normalizes existing content CRLF→LF, writes extracted content at specified position
4. `CopyStats.Positions` carries `positionRecord` (From, To, SourceHash) back to caller
5. `toPositionLocks()` converts to `PositionLock` for lockfile persistence

**Sync-time behavior:**

- `checkLocalModifications()` warns (not errors) if destination content differs before overwrite
- Warnings printed as `⚠ <path> has local modifications at target position that will be overwritten`

**Verify-time behavior:**

- `verifyPositions()` reads destination file locally, extracts target range, hashes, compares to stored `SourceHash`
- No network access required — purely local verification
- Position entries produce separate verification results from whole-file entries

**Key files:** `internal/types/position.go` (parser), `internal/core/position_extract.go` (extract/place), `internal/core/file_copy_service.go` (integration)

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
- **Cross-platform shell execution:**
  - Unix/Linux/macOS: `sh -c command`
  - Windows: `cmd /c command`
- Full shell support for pipes, redirections, and multiline commands
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
- `GIT_VENDOR_DIRS_CREATED`: Number of directories created

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

### Legacy Traps (Non-Goals)

- **`os.IsNotExist()` for wrapped errors**: MUST NOT use `os.IsNotExist(err)` when the error may have been wrapped with `fmt.Errorf("%w")`. MUST use `errors.Is(err, os.ErrNotExist)` instead. Go's `os.IsNotExist` does not unwrap.
- **`net/http.DetectContentType` for binary detection**: Rejected in favor of git's null-byte heuristic (scan first 8000 bytes for `\x00`). `DetectContentType` only inspects 512 bytes and can misclassify source code as `application/octet-stream`. The null-byte approach matches git's own `xdl_mmfile_istext` and has no false positives on valid text files including multi-byte UTF-8.

### Error Handling

Error handling follows Go conventions (see ROADMAP.md section 9.5 for details):

- **`fmt.Errorf`**: Default for most errors (informational, wrapping with `%w`)
- **Sentinel errors**: `ErrNotInitialized`, `ErrComplianceFailed` — use with `errors.Is()`
- **Custom types**: `VendorNotFoundError`, `StaleCommitError`, `HookError`, `OSVAPIError`, etc. — use with `errors.As()` or `Is*()` helpers

Service-specific error handling:
- **Hooks**: `HookError` wraps hook failures with vendor name, phase (pre/post-sync), and command context. Hooks have a 5-minute timeout (configurable via `hookService.timeout` field for testing) via `context.WithTimeout` to prevent hangs. Environment variable values are sanitized to strip newlines/null bytes.
- **Sync cache**: `canSkipSync()` logs a warning and forces re-sync on cache corruption. Uses `errors.Is(err, os.ErrNotExist)` for file existence checks (not `os.IsNotExist`).
- **Vuln scanner**: `OSVAPIError` wraps HTTP error responses with status code and truncated body. Response bodies are size-limited to 10 MB via `io.LimitReader`. Rate limit (HTTP 429), server error (5xx), and client error (4xx) produce distinct error messages.

Display patterns:
- TUI functions use `check(err)` helper that prints "Aborted." and exits
- Core functions return errors for caller handling
- CLI prints styled errors via `tui.PrintError(title, message)` — note: pass `err.Error()` not `err`

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

**Test Infrastructure:**

- Auto-generated mocks using MockGen (gomock framework)
- Test files organized by service
- `testhelpers_gomock_test.go`: Gomock setup helpers
- `testhelpers.go`: Common test utilities

**Coverage:**

Critical paths (sync, update, validation) have high test coverage, while uncovered areas are primarily I/O wrappers and OS-dependent operations.

```bash
# Check current coverage
go test -cover ./internal/core
```

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
- `github.com/CycloneDX/cyclonedx-go` - CycloneDX SBOM generation
- `github.com/spdx/tools-golang` - SPDX SBOM generation
- `github.com/google/uuid` - UUID generation for SBOM serial numbers

**Testing:**

- `github.com/golang/mock` - Mock generation (gomock/mockgen)

**Environment Variables (Optional):**

- `GITHUB_TOKEN` - GitHub personal access token (significantly increases API rate limit and enables private repo access)
- `GITLAB_TOKEN` - GitLab personal access token (enables private repos and increases API rate limits)

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
12. **Incremental sync cache**: Stored in .git-vendor/.cache/, auto-invalidates on commit hash changes, 1000 file limit per vendor
13. **Hook execution**: Hooks run in project root with full shell support (sh -c), runs even for cache hits, same security model as npm scripts. 5-minute timeout kills hanging hooks (override `hookService.timeout` in tests). Environment variable values are sanitized (newlines/null bytes stripped). Timeout tests MUST use `exec sleep` (not bare `sleep`) to prevent orphaned child processes when `sh -c` is killed
14. **Parallel processing**: Auto-disabled for dry-run mode, worker count defaults to NumCPU (max 8), thread-safe operations
15. **Watch mode**: 1-second debounce for rapid changes, watches vendor.yml only, re-runs full sync on changes
16. **Sentinel errors with tui.PrintError**: Sentinel errors like `ErrNotInitialized` are `error` types, not strings. Call `.Error()` when passing to `tui.PrintError(title, err.Error())`
17. **Position syntax and Windows paths**: Position parser uses first `:L<digit>` occurrence to split, avoiding false matches on Windows drive letters like `C:\path`
18. **EndCol is 1-indexed inclusive byte offset**: `L1C5:L1C10` extracts bytes 5-10 (6 bytes). Maps to Go slice `line[StartCol-1 : EndCol]` because Go's exclusive upper bound equals the 1-indexed inclusive bound. See gotcha #22 for multi-byte character implications
19. **errors.Is vs os.IsNotExist**: `os.IsNotExist()` does NOT unwrap `fmt.Errorf("%w")`-wrapped errors. MUST use `errors.Is(err, os.ErrNotExist)` when checking errors from functions that wrap (e.g., `ExtractPosition`)
20. **Binary file detection**: `ExtractPosition` and `PlaceContent` (with position) reject binary files by scanning the first 8000 bytes for null bytes (git's heuristic). Null byte beyond 8000 bytes is NOT detected. Whole-file replacement (`PlaceContent` with nil pos) bypasses the check
21. **Verify produces separate position-level and whole-file results**: A file with both types of lockfile entries gets two verification results; position-level can fail independently of whole-file
22. **Position column semantics are byte-offset, not rune-offset**: Column numbers in `L1C5:L1C10` refer to byte positions in the Go string, not Unicode codepoints. For ASCII this is identical, but multi-byte characters (emoji=4 bytes, CJK=3 bytes, accented=2 bytes) require counting bytes. Extracting a partial multi-byte character produces invalid UTF-8.
23. **CRLF normalized to LF in position extraction**: `extractFromContent` and `placeInContent` normalize `\r\n` → `\n` before processing. Extracted content always uses LF. Files with CRLF will have their line endings changed to LF after `PlaceContent`. Standalone `\r` (classic Mac) is NOT normalized.
24. **Trailing newline creates phantom empty line**: A file ending with `\n` has one more "line" than visible content lines (e.g., `"a\nb\n"` = 3 lines: `"a"`, `"b"`, `""`). L5-EOF on a 5-line file with trailing newline captures the trailing newline; without trailing newline it does not.
25. **Empty file has 1 line**: A 0-byte file splits to `[""]` (1 empty line). `L1` extracts empty string. `L2+` errors.
26. **Sequential PlaceContent calls operate on modified content**: When two vendors write to different positions in the same file, the second call sees the file as modified by the first. If the first call changes line count, the second call's position targets shifted lines.
27. **L1-EOF hash equals whole-file hash**: `L1-EOF` extraction produces content byte-identical to the raw file (after CRLF normalization), so the hash matches `sha256(file_content)`.

## Quick Reference

### Available Commands

```bash
git-vendor init                      # Initialize .git-vendor directory
git-vendor add                       # Add vendor (interactive)
git-vendor edit                      # Edit vendor (interactive)
git-vendor remove <name>             # Remove vendor
git-vendor list                      # List all vendors
git-vendor sync [options] [vendor]   # Sync dependencies
git-vendor update [options]          # Update lockfile
git-vendor validate                  # Validate config and detect conflicts
git-vendor verify [options]          # Verify files against lockfile hashes
git-vendor scan [options]            # Scan for CVE vulnerabilities (via OSV.dev)
git-vendor status                    # Check if local files match lockfile
git-vendor check-updates             # Preview available updates
git-vendor diff <vendor>             # Show commit history between locked and latest
git-vendor watch                     # Auto-sync on config changes
git-vendor sbom [options]            # Generate SBOM (CycloneDX/SPDX)
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

### Verify Command Flags

```bash
--format=<fmt>    # Output format: table (default) or json
# Exit codes: 0=PASS, 1=FAIL (modified/deleted), 2=WARN (added)
```

**Behavior:** The verify command checks all vendored files against the SHA-256 hashes stored in the lockfile. It detects:
- **Modified files**: Hash mismatch between expected and actual
- **Deleted files**: Files in lockfile but missing from disk
- **Added files**: Files in vendor directories but not in lockfile
- **Position-level drift**: For position-extracted mappings, verifies the target range hash matches the stored `source_hash` (local-only, no cloning)

### Scan Command Flags

```bash
--format=<fmt>    # Output format: table (default) or json
--fail-on <sev>   # Fail if vulnerabilities at or above severity (critical|high|medium|low)
# Exit codes: 0=PASS, 1=FAIL (vulns found), 2=WARN (scan incomplete)
```

**Behavior:** The scan command queries OSV.dev for known CVEs affecting vendored dependencies. It uses:
- **PURL query**: Package URL with version tag (preferred when available)
- **Commit query**: Git commit hash fallback for untagged dependencies

**Limitations:**
- Only scans packages tracked by OSV.dev vulnerability database
- Private/internal repos cannot have CVE data
- Results cached for 24 hours (configurable via `GIT_VENDOR_CACHE_TTL` env var)
- Commit-level queries may miss vulnerabilities announced against version ranges

**Cache:** Results cached in `.git-vendor-cache/osv/` with 24-hour TTL. Stale cache used as fallback when network unavailable.

### SBOM Command Flags

```bash
--format=<fmt>    # Output format: cyclonedx (default) or spdx
--output=<file>   # Write to file instead of stdout
-o <file>         # Shorthand for --output
--validate        # Validate generated SBOM against schema
--help, -h        # Show detailed help for sbom command
```

**Behavior:** The sbom command generates a Software Bill of Materials from the lockfile. Supports:
- **CycloneDX 1.5**: Default format, widely supported by vulnerability scanners
- **SPDX 2.3**: Alternative format for compliance requirements (EO 14028, DORA, CRA)
- **PURL generation**: Automatic Package URL generation for GitHub, GitLab, Bitbucket
- **Metadata mapping**: Maps lockfile fields (license, version, hashes) to SBOM components
- **Supplier info**: Extracts supplier/manufacturer from repository URL
- **Unique IDs**: Handles vendors with multiple refs by including commit hash in IDs

### File Paths

- Config: `.git-vendor/vendor.yml`
- Lock: `.git-vendor/vendor.lock`
- Licenses: `.git-vendor/licenses/<name>.txt`
- Vendored files: User-specified paths (outside .git-vendor/)

### Important Functions by File

**vendor_syncer.go:**

- `VendorSyncer` - Top-level orchestrator, delegates to SyncService/UpdateService/etc.
- `FetchRepoDir()` - Browse remote repository contents via git ls-tree

**sync_service.go:**

- `Sync()` - Orchestrate sync for one or all vendors
- `SyncVendor()` - Core sync logic for single vendor
- `syncRef()` - Sync a single ref (clone, checkout, copy, cache)
- `canSkipSync()` - Check cache to skip redundant sync operations
- `updateCache()` - Build and save cache after successful sync

**update_service.go:**

- `UpdateAll()` - Update all vendors, regenerate lockfile
- `UpdateAllWithOptions()` - Parallel update variant with worker pool
- `computeFileHashes()` - Compute SHA-256 hashes for lockfile entries
- `toPositionLocks()` - Convert positionRecord to PositionLock for lockfile persistence

**validation_service.go:**

- `ValidateConfig()` - Comprehensive config validation
- `DetectConflicts()` - Find path conflicts between vendors

**vuln_scanner.go:**

- `Scan()` - Core vulnerability scanning against OSV.dev
- `batchQuery()` - Batch queries to OSV.dev (up to 1000 per request, auto-paginated)
- `CVSSToSeverity()` - Convert CVSS score to severity level
- `isRateLimitError()` - Detect rate-limit errors via OSVAPIError or string matching
- `isNetworkError()` - Detect transient network errors for stale-cache fallback

**verify_service.go:**

- `Verify()` - Core verification logic, returns VerifyResult
- `verifyPositions()` - Position-level hash verification (local-only, no cloning)
- `buildExpectedFilesFromCache()` - Cache fallback for lockfiles without hashes
- `findAddedFiles()` - Scan vendor dirs for files not in lockfile

**position_extract.go:**

- `ExtractPosition()` - Read file, extract content at PositionSpec, return content + SHA-256 hash
- `PlaceContent()` - Write extracted content into target file at specified position
- `extractColumns()` / `placeColumns()` - Column-precise extraction and placement

**file_copy_service.go:**

- `CopyMappings()` - Orchestrate all file copy operations for a vendor ref
- `copyWithPosition()` - Position-aware file copy (extract → place → track)
- `checkLocalModifications()` - Detect local changes at target position before overwrite

**hook_service.go:**

- `ExecutePreSync()` / `ExecutePostSync()` - Run pre/post sync shell hooks with timeout and env injection
- `sanitizeEnvValue()` - Strip newlines/null bytes from environment variable values

**errors.go:**

- `NewHookError()` / `IsHookError()` - Structured error for hook failures with phase/command context
- `NewOSVAPIError()` / `IsOSVAPIError()` - Structured error for OSV.dev API HTTP errors

**git_operations.go:**

- `ParseSmartURL()` - Extract repo/ref/path from GitHub URLs
- `GitClient.ListTree()` - Browse remote directories via git ls-tree

**github_client.go:**

- `CheckLicense()` - Query GitHub API for license
- `IsAllowed()` - Validate against allowed licenses

**filesystem.go:**

- `ValidateDestPath()` - Security check for path traversal
- `CopyFile()` / `CopyDir()` - File operations

**sbom_generator.go:**

- `Generate()` - Generate SBOM in specified format (CycloneDX or SPDX)
- `generateCycloneDX()` - Create CycloneDX 1.5 JSON SBOM
- `generateSPDX()` - Create SPDX 2.3 JSON SBOM
- `validateSBOM()` - Validate generated SBOM against schema

**internal/hostdetect/ (shared host detection utilities):**

- `FromURL()` - Parse repository URL and extract provider, owner, repo
- `DetectProvider()` - Determine provider type from hostname (exact match → suffix → contains)
- `IsKnownProvider()` - Check if provider is recognized (GitHub, GitLab, Bitbucket)
- `SupportsCVEScanning()` - Check if provider is supported by vulnerability databases

**internal/purl/ (shared PURL utilities):**

- `FromGitURL()` - Create PURL from git repository URL (uses hostdetect for consistency)
- `FromGitURLWithFallback()` - Create PURL with fallback to generic type
- `String()` - Format PURL as standard string (pkg:type/namespace/name@version)
- `SupportsVulnScanning()` - Check if PURL type is supported by OSV.dev
- `ToOSVPackage()` - Get package identifier for OSV.dev API

**internal/sbom/ (shared SBOM utilities):**

- `GenerateBOMRef()` - Create unique CycloneDX BOM reference
- `GenerateSPDXID()` - Create unique SPDX identifier for packages (includes max length truncation)
- `SanitizeSPDXID()` - Convert string to valid SPDX identifier (pattern [a-zA-Z0-9.-]+, max 128 chars)
- `ExtractSupplier()` - Extract supplier info from repository URL (uses hostdetect for consistency)
- `MetadataComment()` - Build SPDX comment from git-vendor metadata
- `ValidateProjectName()` - Ensure project name is valid for SBOMs
- `BuildSPDXNamespace()` - Construct unique SPDX document namespace
