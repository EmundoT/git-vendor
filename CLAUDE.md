# CLAUDE.md

**CRITICAL: ALWAYS USE THE `private` REMOTE** (e.g. `git pull private main`, `git push private main`).

## Ecosystem Rules

This project follows the git-ecosystem shared rules:
- Commit schema and tag conventions: see `../git-ecosystem/rules/commit-schema-tags.md`
- Documentation and quality standards: see `../git-ecosystem/rules/ecosystem-protocol.md`

## What is git-vendor?

Go CLI for deterministic, file-level source code vendoring from any Git repository. Granular path mapping — vendor specific files/directories from remote repos to specific local paths with full provenance tracking.

## Build / Test / Lint

```bash
# Build (optimized)
make build

# Build (debug)
make build-dev

# Sync Go vendor/ dir (required after updating pkg/git-plumbing/ files)
make vendor

# Generate mocks (required before first test run or after interface changes)
# Automatically runs `make vendor` first
make mocks                    # Unix (requires make)
powershell scripts/mocks.ps1  # Windows (no make required)

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
    config_store.go / lock_store.go  # YAML I/O interfaces + lock conflict detection/merge
    hook_service.go              # Pre/post sync shell hooks
    cache_store.go               # Incremental sync cache
    parallel_executor.go         # Worker pool for concurrent ops
    diff_service.go / drift_service.go  # Diff (with DiffOptions filtering) and drift detection
    outdated_service.go              # Lightweight staleness check via git ls-remote
    commit_service.go            # COMMIT-SCHEMA v1 trailers + git notes
    pull_service.go              # Pull command: combined update+sync orchestration
    push_service.go              # Push command: propose local changes to source repo via PR (CLI-005)
    accept_service.go            # Accept command: acknowledge local drift to vendored files (CLI-003)
    cascade_service.go           # Cascade command: transitive graph pull across sibling projects (CLI-006)
    status_service.go            # Status command: unified verify+outdated inspection (CLI-001)
    policy_service.go            # Policy engine: vendor.yml policy evaluation for commit guard (GRD-002)
    enforcement_service.go       # Compliance enforcement: resolve levels + exit codes (Spec 075)
    config_commands.go           # LLM-friendly CLI (Spec 072) + mirror management
    cli_response.go              # JSON output types for Spec 072
    remote_fallback.go           # Multi-remote: ResolveVendorURLs + FetchWithFallback
    internal_sync_service.go     # Internal vendor sync (same-repo file copy, Spec 070)
    compliance_service.go        # Drift detection + propagation for internal vendors (Spec 070)
    errors.go                    # Sentinel errors + structured types
    constants.go                 # Path constants, git refs, license lists
  tui/wizard.go                  # Interactive TUI (charmbracelet/huh + lipgloss)
  types/                         # Data models (VendorConfig, VendorLock, etc.)
  version/                       # Build version injection via ldflags
docs/                            # Human-facing documentation
.claude/
  skills/                        # Project-specific skills (migration-planner, grc-auditor)
  rules/                         # Contextual rules (loaded by file path)
```

## Key Interfaces (Dependency Injection)

All in `internal/core/`. Mock with gomock for tests.

| Interface | Impl | File |
|-----------|------|------|
| `GitClient` | `SystemGitClient` | git_operations.go |
| `FileSystem` | `OSFileSystem` | filesystem.go |
| `LicenseChecker` | `MultiPlatformLicenseChecker` | license_multiplatform.go |
| `ConfigStore` | `YAMLConfigStore` | config_store.go |
| `LockStore` | `YAMLLockStore` | lock_store.go |
| `HookExecutor` | `ShellHookExecutor` | hook_service.go |
| `CacheStore` | `FileCacheStore` | cache_store.go |
| `InternalSyncServiceInterface` | `InternalSyncService` | internal_sync_service.go |
| `ComplianceServiceInterface` | `ComplianceService` | compliance_service.go |
| `OutdatedServiceInterface` | `OutdatedService` | outdated_service.go |

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
| `Direction` | `""` (source-canonical, default) or `"bidirectional"` (YAML: `direction`) |
| `Enforcement` | `""` (inherits global) or `"strict"`/`"lenient"`/`"info"` (YAML: `compliance`) |

| LockDetails Field | Purpose |
|------------------|---------|
| `Source` | `"internal"` for internal vendors |
| `SourceFileHashes` | `map[string]string` — source path to SHA-256 |

Internal vendors MUST use `Ref: "local"` (sentinel, not a git ref). `--internal` flag on `sync` runs only internal vendors. `--reverse` propagates dest changes back to source (requires `--internal`).

## sync vs update vs pull

- **sync**: Fetch dependencies at locked commit hashes (deterministic). Uses `--depth 1` for shallow clones. Falls back to full fetch for stale commits. With `--internal`: syncs only internal vendors (no network). With `--local`: allows `file://` and local filesystem paths in vendor URLs.
- **update**: Fetch latest commits and regenerate lockfile. Supports `<vendor-name>` positional arg and `--group <name>` for selective updates (non-targeted vendors retain existing lock entries). With `--local`: allows `file://` and local filesystem paths in vendor URLs.
- **pull**: Combines update + sync into one operation ("get the latest from upstream"). Default: fetch latest, update lock, copy files. `--locked`: skip fetch, use existing lock (same as sync). `--prune`: remove dead mappings from vendor.yml. `--keep-local`: detect locally modified files. `--force`/`--no-cache`: passed through to sync. Supports `<vendor-name>` positional arg and `--local`. Implementation: `pull_service.go` (PullOptions, PullResult, VendorSyncer.PullVendors).
- **push**: Propose local changes to vendored files back upstream via PR. Detects locally modified files (lock hash mismatch), clones source repo, applies diffs via reverse path mapping (`to -> from`), creates branch `vendor-push/<project>/<YYYY-MM-DD>`, pushes, and creates PR via `gh` CLI (graceful fallback to manual instructions if `gh` unavailable). `--file <path>`: push a single file. `--dry-run`: preview without action. Internal vendors are rejected (use `--reverse`). Implementation: `push_service.go` (PushOptions, PushResult, VendorSyncer.PushVendor).
- **status**: Unified inspection replacing verify+diff+outdated. Offline checks first (lock vs disk), remote checks second (lock vs upstream). `--offline`: skip remote. `--remote-only`: skip disk. `--format json`: machine-readable. Exit codes: 0=PASS, 1=FAIL, 2=WARN. Includes config/lock coherence detection and policy violation reporting. Implementation: `status_service.go` (StatusService, StatusResult).
- **accept**: Acknowledge local drift to vendored files. Writes `accepted_drift` to lock (path → local SHA-256). Accepted files pass commit guard. `--file <path>`: single file. `--clear`: remove drift entries. `--no-commit`: skip auto-commit. Implementation: `accept_service.go` (AcceptService, AcceptOptions, AcceptResult).
- **cascade**: Walk dependency graph across sibling projects. Discovers siblings with vendor.yml, builds DAG, topological sort, pulls in order. `--root <dir>`: parent directory. `--verify`: run build/test after each pull. `--commit`/`--push`: auto-commit/push. `--pr`: create branches+PRs. `--dry-run`: preview order. Implementation: `cascade_service.go` (CascadeService, CascadeOptions, CascadeResult).
- **diff**: Compare locked vs latest commit per vendor. Supports `<vendor-name>`, `--ref <ref>`, `--group <name>` filters. `DiffVendorWithOptions(DiffOptions)` is the primary API; `DiffVendor(name)` is a backward-compatible wrapper.
- **outdated**: Lightweight staleness check via `git ls-remote` (1 command per vendor, no temp dirs). Read-only — does not modify lockfile. Exit code 1 = stale. CI-friendly alternative to `check-updates`.

## Deprecated Commands

Old commands are aliased with deprecation warnings (CLI-004). Remove after 2 minor versions:
- `sync` → `pull --locked`
- `update` → `pull`
- `verify` → `status --offline`
- `diff` → `status`
- `outdated` → `status --remote-only`

## Commit Guard

Pre-commit hook chain in `.githooks/`:
- **vendor-guard.sh** (GRD-001): Runs `status --offline --format json`, blocks commits with unacknowledged vendor drift. Two-pass: offline first, remote only when `block_on_stale` policy is enabled.
- **pre-commit**: Calls vendor-guard.sh after existing go build/vet/test checks.

## Policy Engine (GRD-002/GRD-003)

Policy lives in `vendor.yml` at top level (global defaults) with per-vendor overrides:
- `block_on_drift` (bool, default true): commit guard blocks on unacknowledged drift
- `block_on_stale` (bool, default false): commit guard checks upstream staleness (network)
- `max_staleness_days` (int, optional): grace window before staleness becomes blocking

Per-vendor policy inherits from global and overrides specific fields. Implementation: `policy_service.go` (PolicyService, VendorPolicy, PolicyViolation, ResolvedPolicy).

## Compliance Enforcement (Spec 075)

Three enforcement levels control how drift affects exit codes and commit gates:
- `strict`: drift → exit 1 (FAIL), blocks commits AND builds
- `lenient`: drift → exit 2 (WARN), blocks commits only
- `info`: drift → exit 0 (PASS), reported but blocks nothing

Configuration in vendor.yml via `compliance:` block (global) and per-vendor `compliance:` field. Modes: `default` (per-vendor overrides global) and `override` (global wins for all). Implementation: `enforcement_service.go` (EnforcementService, ResolveVendorEnforcement, ComputeExitCode).

CLI: `status --strict-only` filters to strict vendors. `status --compliance=<level>` overrides all vendors. `compliance` command shows effective levels.

## Lock Conflict Detection

`FileLockStore.Load()` scans for git merge conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) before YAML parsing. Returns `LockConflictError` (wraps `ErrLockConflict` sentinel) with structured `LockConflict` entries (line number, ours/theirs raw content). `MergeLockEntries(ours, theirs)` performs programmatic three-way merge: later timestamp wins, lexicographic commit hash tiebreaker, unresolvable entries flagged in `LockMergeResult.Conflicts`.

Key types in `internal/types/types.go`: `LockConflict`, `LockMergeConflict`, `LockMergeResult`.

## Multi-Remote / Mirrors (Schema v1.3)

Vendors support mirror URLs for redundancy. Primary URL is tried first, then mirrors in declaration order.

| VendorSpec Field | Purpose |
|-----------------|---------|
| `Mirrors` | `[]string` — fallback URLs tried after primary URL fails |

| LockDetails Field | Purpose |
|------------------|---------|
| `SourceURL` | Which URL actually served the content (empty = primary URL) |

- `ResolveVendorURLs(vendor)` returns `[primary, mirror1, mirror2, ...]` (nil for internal vendors)
- `FetchWithFallback(ctx, gitClient, fs, ui, tempDir, urls, ref, depth)` tries each URL in order, swapping origin's URL on fallback
- All services (sync, update, diff, drift, outdated) use `FetchWithFallback` automatically
- CLI management: `config add-mirror`, `config remove-mirror`, `config list-mirrors`
- Business logic: `remote_fallback.go` (ResolveVendorURLs, FetchWithFallback)
- Mirror config commands: `config_commands.go` (AddMirror, RemoveMirror, ListMirrors)

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
10. **Stale `vendor/` directory**: Go's `vendor/` (gitignored) overrides `replace` directives. After adding/modifying files in `pkg/git-plumbing/`, MUST run `make vendor` (or `go mod vendor`) before build/test. Symptoms: `undefined: git.<NewSymbol>` despite the symbol existing in `pkg/git-plumbing/`.
11. **Local paths require `--local` flag**: `file://`, relative paths (`./`, `../`), and absolute filesystem paths are blocked by default in vendor URLs (SEC-011). Pass `--local` to `sync`/`update` to opt in. `IsLocalPath()` detects local URLs; `ResolveLocalURL()` resolves relative paths against the project root. Without `--local`, SyncVendor returns an error with a hint.

## Common Patterns

- **Multi-ref tracking**: Multiple `specs` entries per vendor target different refs to different local paths
- **Vendor groups**: `groups: ["frontend"]` on vendor specs enables `--group frontend` for batch operations on `sync`, `update`, and `diff`
- **Custom hooks**: `hooks.pre_sync` / `hooks.post_sync` run shell commands; env vars `GIT_VENDOR_NAME`, `GIT_VENDOR_URL`, `GIT_VENDOR_REF`, `GIT_VENDOR_COMMIT`, `GIT_VENDOR_ROOT`, `GIT_VENDOR_FILES_COPIED` are injected
- **Incremental cache**: SHA-256 checksums in `.git-vendor/.cache/` skip re-downloading unchanged files. Bypass with `--no-cache` or `--force`
- **Parallel processing**: `--parallel [--workers N]` uses a worker pool for concurrent vendor operations (default workers: NumCPU, max 8)
- **Watch mode**: `git-vendor watch` monitors `vendor.yml` for changes and auto-syncs (1s debounce)
- **CI/CD**: Commit both `vendor.yml` and `vendor.lock` for deterministic builds. Use `--yes --quiet` for non-interactive mode

## Contextual Rules

Detailed rules load automatically based on which files you're editing. See `.claude/rules/`:

| Rule file | Loaded when editing | Contains |
|-----------|-------------------|----------|
| `architecture.md` | `internal/**/*.go`, `main.go` | Data model, DI pattern, service layer, context propagation |
| `go-atomic-edits.md` | `internal/**/*.go`, `main.go` | Cross-file interface edit safety (gopls revert prevention) |
| `legacy-traps.md` | `internal/**/*.go` | Rejected approaches and deferred decisions only |
| `testing.md` | `**/*_test.go` | Testing boundaries, mock gen, what's untested and why |
| `security.md` | security-critical files | Path traversal, URL validation, hook threat model |
| `position-extraction.md` | position/file_copy files | Syntax spec, pipeline, CRLF, column semantics |
| `spec-072.md` | config_commands/cli_response | LLM-friendly CLI, JSON schema, error codes |
| `vendor-commits.md` | commit_service.go | vendor/v1 delta: trailers, git notes, atomic commit design |

## Deeper Documentation

- `docs/COMMANDS.md` — Full command reference with all flags
- `docs/CONFIGURATION.md` — vendor.yml and policy file format
- `../git-ecosystem/ideas/` — Work queues and completed items
- `docs/TROUBLESHOOTING.md` — Common issues and solutions
