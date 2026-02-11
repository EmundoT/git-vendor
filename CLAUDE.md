# CLAUDE.md

**CRITICAL: ALWAYS USE THE `private` REMOTE** (e.g. `git pull private main`, `git push private main`).

## What is git-vendor?

Go CLI for deterministic, file-level source code vendoring from any Git repository. Granular path mapping — vendor specific files/directories from remote repos to specific local paths with full provenance tracking.

## Build / Test / Lint

```bash
# Build (optimized)
make build

# Build (debug)
make build-dev

# Generate mocks (required before first test run or after interface changes)
make mocks

# Tests
go test ./...

# Tests with coverage
go test -cover ./internal/core

# Tests with race detector
go test -race ./...

# Vet
go vet ./...
```

## Project Layout

```text
main.go                          # CLI entry point, command routing
internal/
  core/                          # Business logic (clean architecture, DI)
    engine.go                    # Manager facade (public API)
    vendor_syncer.go             # Top-level sync orchestrator
    sync_service.go              # Sync logic (fetch, cache, skip)
    update_service.go            # Update lockfile, compute hashes
    file_copy_service.go         # Position-aware file copy
    verify_service.go            # Verification against lockfile hashes
    validation_service.go        # Config validation, conflict detection
    position_extract.go          # Line/column extraction and placement
    git_operations.go            # GitClient interface + SystemGitClient
    filesystem.go                # FileSystem interface (I/O, path validation)
    config_store.go / lock_store.go  # YAML I/O interfaces
    hook_service.go              # Pre/post sync shell hooks
    cache_store.go               # Incremental sync cache
    parallel_executor.go         # Worker pool for concurrent ops
    diff_service.go / drift_service.go  # Diff and drift detection
    commit_service.go            # COMMIT-SCHEMA v1 trailers + git notes
    config_commands.go           # LLM-friendly CLI (Spec 072)
    cli_response.go              # JSON output types for Spec 072
    internal_sync_service.go     # Internal vendor sync (same-repo file copy, Spec 070)
    compliance_service.go        # Drift detection + propagation for internal vendors (Spec 070)
    errors.go                    # Sentinel errors + structured types
    constants.go                 # Path constants, git refs, license lists
  tui/wizard.go                  # Interactive TUI (charmbracelet/huh + lipgloss)
  types/                         # Data models (VendorConfig, VendorLock, etc.)
  version/                       # Build version injection via ldflags
docs/                            # Human-facing documentation
.claude/
  commands/                      # Workflow skills (code review, security audit, etc.)
  rules/                         # Contextual rules (loaded by file path)
```

## Key Interfaces (Dependency Injection)

All in `internal/core/`. Mock with gomock for tests.

| Interface | Impl | File |
|-----------|------|------|
| `GitClient` | `SystemGitClient` | git_operations.go |
| `FileSystem` | `OSFileSystem` | filesystem.go |
| `LicenseChecker` | `GitHubLicenseChecker` | github_client.go |
| `ConfigStore` | `YAMLConfigStore` | config_store.go |
| `LockStore` | `YAMLLockStore` | lock_store.go |
| `HookExecutor` | `ShellHookExecutor` | hook_service.go |
| `CacheStore` | `FileCacheStore` | cache_store.go |
| `InternalSyncServiceInterface` | `InternalSyncService` | internal_sync_service.go |
| `ComplianceServiceInterface` | `ComplianceService` | compliance_service.go |

## File System Structure

```text
.git-vendor/
  vendor.yml          # Config: what to vendor and where
  vendor.lock         # Lock: exact commit SHAs + file hashes
  licenses/           # Cached license files per dependency
  .cache/             # Incremental sync cache
.git-vendor-policy.yml  # Optional license policy (project root)
```

## Internal Vendors (Spec 070)

Internal vendors track files **within the same repository** for consistency enforcement. Configured via `source: internal` on `VendorSpec`.

| VendorSpec Field | Purpose |
|-----------------|---------|
| `Source` | `""` (external, default) or `"internal"` |
| `Compliance` | `""` (source-canonical, default) or `"bidirectional"` |

| LockDetails Field | Purpose |
|------------------|---------|
| `Source` | `"internal"` for internal vendors |
| `SourceFileHashes` | `map[string]string` — source path to SHA-256 |

Internal vendors MUST use `Ref: "local"` (sentinel, not a git ref). `--internal` flag on `sync` runs only internal vendors. `--reverse` propagates dest changes back to source (requires `--internal`).

## sync vs update

- **sync**: Fetch dependencies at locked commit hashes (deterministic). Uses `--depth 1` for shallow clones. Falls back to full fetch for stale commits. With `--internal`: syncs only internal vendors (no network).
- **update**: Fetch latest commits and regenerate entire lockfile.

## Design Principles

1. **Offline-First**: Every command except sync/fetch works without network
2. **Lockfile Is Truth**: All state derives from vendor.lock
3. **Incremental, Not Big-Bang**: Features ship as standalone subcommands
4. **Don't Break Anything**: Lockfile format remains backward-compatible
5. **Output Is the Product**: Features produce shareable, actionable output

## Dependencies

**Runtime**: git-plumbing (self-vendored to `pkg/git-plumbing/`), charmbracelet/huh, lipgloss, yaml.v3, fsnotify, cyclonedx-go, spdx/tools-golang, google/uuid

**Test**: golang/mock (gomock/mockgen)

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `GITHUB_TOKEN` | GitHub API rate limits + private repo access |
| `GITLAB_TOKEN` | GitLab private repos + rate limits |
| `GIT_VENDOR_OSV_ENDPOINT` | Override OSV.dev base URL (air-gapped proxies) |
| `GIT_VENDOR_CACHE_TTL` | Override 24h scan cache TTL (Go duration format) |

## Essential Gotchas

1. **`errors.Is` not `os.IsNotExist`**: `os.IsNotExist()` does NOT unwrap `fmt.Errorf("%w")`-wrapped errors. MUST use `errors.Is(err, os.ErrNotExist)`.
2. **Smart URL branch ambiguity**: Branch names with slashes (e.g., `feature/foo`) cannot be parsed from URLs. Use base URL + manual ref entry.
3. **Position hash prefix**: `ComputeFileChecksum` returns bare hex; `ExtractPosition` returns `"sha256:<hex>"`. MUST normalize before comparing.
4. **tui.PrintError takes string**: Sentinel errors like `ErrNotInitialized` are `error` types. Call `.Error()` when passing to `tui.PrintError(title, err.Error())`.
5. **Git operations via git-plumbing**: No direct `exec.Command` calls. All git ops delegate through `gitFor(dir)` which creates `*git.Git` instances.
6. **Context propagation**: All long-running operations accept `context.Context`. CLI creates `signal.NotifyContext` for Ctrl+C.
7. **RefLocal is a sentinel, not a git ref**: `RefLocal` ("local") is used for internal vendors. MUST NOT pass to git operations (checkout, fetch). All internal vendor ops use `os.Stat`/`os.ReadFile`, not git commands.
8. **SourceFileHashes population**: Only populated during internal sync. Keyed by source file path (file-level granularity, position specs stripped before keying).
9. **Position auto-update scope**: `updatePositionSpecs` only adjusts line-range specs (`L5-L20`). ToEOF specs auto-expand (no update). Column specs NOT auto-updated (documented limitation).

## Contextual Rules

Detailed rules load automatically based on which files you're editing. See `.claude/rules/`:

| Rule file | Loaded when editing | Contains |
|-----------|-------------------|----------|
| `architecture.md` | `internal/**/*.go` | Data model, DI pattern, service layer, context propagation |
| `legacy-traps.md` | `internal/**/*.go` | Rejected approaches, format mismatches, known pitfalls |
| `testing.md` | `**/*_test.go` | Testing boundaries, mock gen, what's untested and why |
| `security.md` | security-critical files | Path traversal, URL validation, hook threat model |
| `position-extraction.md` | position/file_copy files | Syntax spec, pipeline, CRLF, column semantics |
| `spec-072.md` | config_commands/cli_response | LLM-friendly CLI, JSON schema, error codes |
| `commit-schema.md` | commit_service.go | Trailers, git notes, atomic commit design |

## Deeper Documentation

- `docs/COMMANDS.md` — Full command reference with all flags
- `docs/ARCHITECTURE.md` — Architecture deep-dive
- `docs/CONFIGURATION.md` — vendor.yml and policy file format
- `docs/ROADMAP.md` — Development roadmap and phases
- `docs/TROUBLESHOOTING.md` — Common issues and solutions
- `.claude/commands/PROJECT_PRIMER.md` — Onboarding skill (`/PROJECT_PRIMER`)
