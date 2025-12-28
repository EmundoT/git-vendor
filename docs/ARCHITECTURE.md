# Architecture

Technical architecture and design decisions for git-vendor.

## Table of Contents

- [Overview](#overview)
- [Design Philosophy](#design-philosophy)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [File System Layout](#file-system-layout)
- [Key Design Decisions](#key-design-decisions)
- [Extension Points](#extension-points)

---

## Overview

git-vendor is a CLI tool built in Go that enables granular vendoring of files/directories from Git repositories with deterministic locking.

**Key characteristics:**
- Written in Go (single binary, no runtime dependencies)
- Clean architecture with dependency injection
- Interactive TUI with charmbracelet/huh
- Multi-platform Git support (GitHub, GitLab, Bitbucket, generic)
- Deterministic builds via lockfile

---

## Design Philosophy

### 1. Granular Path Control

Unlike git submodules (entire repos) or package managers (fixed module boundaries), git-vendor allows vendoring specific files or directories.

**Why:** Reduces bloat, vendors only what you need.

### 2. Deterministic Locking

Every vendor is locked to an exact commit hash in `vendor.lock`.

**Why:** Reproducible builds across environments, teams, and time.

### 3. No Git History

Vendored files are plain copies, not nested git repositories.

**Why:** Avoids submodule complexity, simpler mental model.

### 4. Interactive First

Primary workflow uses interactive TUI for path selection.

**Why:** Path mapping is complex - TUI simplifies UX.

### 5. CI/CD Compatible

Non-interactive mode (`--yes`, `--quiet`, `--json`) for automation.

**Why:** Works in both dev (interactive) and CI (automated) environments.

---

## Core Components

### Command Dispatcher (main.go)

**Responsibility:** Routes commands to appropriate handlers

**Entry points:**
- `init` - Initialize vendor directory
- `add`/`edit` - Interactive wizards
- `remove` - Delete vendor
- `list` - Show vendors
- `sync` - Download files
- `update` - Fetch latest commits
- `validate` - Check config
- `status`/`check-updates`/`diff`/`watch` - Utility commands
- `completion` - Shell completion

### Manager (engine.go)

**Responsibility:** Public API facade

Delegates to VendorSyncer for business logic. Provides clean, simple API for CLI.

### VendorSyncer (vendor_syncer.go)

**Responsibility:** Orchestrates vendor operations

Coordinates between:
- ConfigStore (vendor.yml)
- LockStore (vendor.lock)
- GitClient (git operations)
- FileSystem (file I/O)
- LicenseChecker (compliance)
- CacheStore (incremental sync)
- HookExecutor (automation)

### GitClient Interface

**Responsibility:** Git operations abstraction

**Methods:**
- `Clone()` - Clone repository
- `Fetch()` - Fetch specific ref
- `Checkout()` - Checkout commit
- `ListTree()` - Browse remote directory
- `GetHeadHash()` - Get commit hash
- `GetCommitLog()` - Get commit history

**Implementations:**
- Production: Executes git binary via `exec.Command`
- Testing: Mock via gomock

### FileSystem Interface

**Responsibility:** File I/O abstraction

**Methods:**
- `CopyFile()` / `CopyDir()` - File operations
- `CreateTemp()` - Temp directory
- `RemoveAll()` - Cleanup
- `Stat()` - File info
- `ValidateDestPath()` - Security check

**Security:** Rejects absolute paths and `..` references.

### ConfigStore/LockStore Interfaces

**Responsibility:** Configuration persistence

- `Load()` - Read YAML
- `Save()` - Write YAML

Abstracts vendor.yml and vendor.lock I/O.

### LicenseChecker Interface

**Responsibility:** License detection and compliance

**Implementations:**
- GitHub: API-based
- GitLab: API-based
- Bitbucket/Generic: File-based

### CacheStore Interface

**Responsibility:** Incremental sync cache

Stores SHA-256 checksums to skip re-downloading unchanged files.

**Location:** `vendor/.cache/<vendor-name>.json`

### HookExecutor Interface

**Responsibility:** Pre/post sync automation

Executes shell commands via `sh -c` with environment variables.

### ParallelExecutor

**Responsibility:** Concurrent vendor processing

Worker pool pattern for multi-vendor operations (opt-in via `--parallel`).

### TUI Layer (internal/tui/)

**Responsibility:** Interactive user interface

Built with charmbracelet/huh (forms) and lipgloss (styling).

**Wizards:**
- Add vendor wizard
- Edit vendor wizard
- File browser (remote and local)
- Path mapping editor

---

## Data Flow

### Sync Operation

```
User runs: git-vendor sync

1. main.go
   ↓ Dispatch command
2. Manager.Sync()
   ↓ Delegate
3. VendorSyncer.Sync()
   ↓ Coordinate
4. Load vendor.yml (ConfigStore)
5. Load vendor.lock (LockStore)
6. For each vendor:
   ├─ Check cache (CacheStore)
   │  ├─ Cache hit? → Skip git operations
   │  └─ Cache miss? → Continue
   ├─ Run pre-sync hook (HookExecutor)
   ├─ Clone repo (GitClient)
   ├─ Checkout commit (GitClient)
   ├─ Copy files (FileSystem)
   ├─ Update cache (CacheStore)
   ├─ Cache license (LicenseChecker)
   └─ Run post-sync hook (HookExecutor)
7. Display results (TUI)
```

### Update Operation

```
User runs: git-vendor update

1. main.go
   ↓ Dispatch command
2. Manager.UpdateAll()
   ↓ Delegate
3. VendorSyncer.UpdateAll()
   ↓ Coordinate
4. Load vendor.yml (ConfigStore)
5. For each vendor:
   ├─ Clone repo (GitClient)
   ├─ Fetch ref (GitClient)
   ├─ Get commit hash (GitClient)
   ├─ Check license (LicenseChecker)
   └─ Build lock entry
6. Save vendor.lock (LockStore)
7. Display results (TUI)
```

### Add Vendor Wizard

```
User runs: git-vendor add

1. main.go
   ↓ Launch wizard
2. TUI.RunAddWizard()
   ├─ Prompt for URL
   ├─ Parse smart URL (extract repo, ref, path)
   ├─ Check if exists → offer edit
   ├─ Prompt for name
   ├─ Prompt for ref
   ├─ Browse remote directory (GitClient.ListTree)
   ├─ Select paths
   ├─ Configure mappings
   └─ Save
3. Manager.SaveVendor()
   ↓ Delegate
4. VendorSyncer.SaveVendor()
   ├─ Update vendor.yml (ConfigStore)
   └─ Run update (UpdateAll)
5. Display success
```

---

## File System Layout

```
project/
├── vendor/
│   ├── vendor.yml       # Configuration (user-managed)
│   ├── vendor.lock      # Lock file (auto-generated)
│   ├── licenses/        # Cached licenses
│   │   └── {name}.txt
│   └── .cache/          # Incremental sync cache
│       └── {name}.json
├── internal/            # Vendored code (example)
│   └── vendor/
│       ├── lib-v1/
│       └── lib-v2/
└── lib/                 # More vendored code (example)
    └── utils/
```

**Key directories:**
- `vendor/` - git-vendor metadata
- User-specified paths - vendored files (outside vendor/)

---

## Key Design Decisions

### Why Granular Path Mapping?

**Problem:** Git submodules vendor entire repositories.

**Solution:** Allow selecting specific files/directories to reduce bloat.

**Trade-off:** More configuration complexity, but better control.

### Why Deterministic Locking?

**Problem:** `git clone main` fetches different commits over time.

**Solution:** Lock to exact commit hashes in vendor.lock.

**Trade-off:** Manual updates required, but reproducible builds.

### Why No Git History?

**Problem:** Git submodules create nested repos with complex interactions.

**Solution:** Copy files as plain files (no .git directory).

**Trade-off:** Lose git history, but simpler mental model.

### Why Interactive TUI?

**Problem:** Path mapping is complex to express via CLI flags.

**Solution:** File browser for selecting paths interactively.

**Trade-off:** Requires TTY, but much better UX.

### Why Shallow Clones?

**Implementation:** Uses `--depth 1` for git clone when possible.

**Benefit:** Faster clones, less disk space.

**Limitation:** May fail for old locked commits (fallback to full fetch).

### Why Incremental Cache?

**Problem:** Re-syncing unchanged files wastes time and bandwidth.

**Solution:** Cache SHA-256 checksums, skip git operations on cache hit.

**Benefit:** 80% faster re-syncs for unchanged vendors.

### Why Parallel Processing (Opt-in)?

**Problem:** Sequential syncing slow for many vendors.

**Solution:** Worker pool for concurrent processing.

**Design:** Opt-in via `--parallel` flag (safe default: sequential).

### Why Custom Hooks?

**Problem:** Users need post-sync automation (build, codegen, etc.).

**Solution:** Pre/post sync shell commands.

**Security:** Same trust model as npm scripts or Makefiles.

---

## Extension Points

### Adding New Git Platforms

**Steps:**
1. Update smart URL parsing in `ParseSmartURL()` (git_operations.go)
2. Implement platform-specific license detection
3. Add tests

**Example:** Adding Codeberg support would require:
- Pattern matching for `codeberg.org` URLs
- License file detection (no API)

### Adding New Commands

**Steps:**
1. Add command case in main.go switch
2. Implement logic in Manager (engine.go)
3. Add TUI wizard if interactive (tui/wizard.go)
4. Add tests

**Example:** Adding `git-vendor search <query>` command:
```go
case "search":
    query := os.Args[2]
    results := manager.SearchVendors(query)
    tui.DisplaySearchResults(results)
```

### Custom License Validators

**Steps:**
1. Implement `LicenseChecker` interface
2. Inject into VendorSyncer
3. Override default checker

**Example:** Custom SPDX validation:
```go
type CustomLicenseChecker struct {}

func (c *CustomLicenseChecker) CheckLicense(url string) (string, error) {
    // Custom validation logic
}
```

### Plugin System (Future)

**Potential design:**
```yaml
plugins:
  - name: "custom-provider"
    path: "/path/to/plugin.so"
```

**Would enable:**
- Custom Git providers
- Custom license validators
- Custom hook executors

---

## Performance Characteristics

### Sync Performance

| Operation | Time | Bottleneck |
|-----------|------|------------|
| **First sync** | ~5-10s/vendor | Network + Git |
| **Cache hit** | ~1-2s/vendor | File validation |
| **Parallel (4 vendors)** | ~3x faster | Worker pool |

### Memory Usage

| Scenario | Memory |
|----------|--------|
| **Single vendor** | ~50MB |
| **10 vendors** | ~200MB |
| **Parallel (8 workers)** | ~400MB |

### Disk Usage

| Item | Size |
|------|------|
| **Binary** | ~10MB |
| **Temp dirs** | Deleted after sync |
| **Cache** | ~1KB/file |

---

## Testing Strategy

### Unit Tests

- Mock-based via gomock
- Interface-driven (easy to mock)
- 65% code coverage

### Integration Tests

- Real git operations in temp repos
- Property-based testing
- Race condition testing (`go test -race`)

### Manual Tests

- TUI components (interactive)
- Platform compatibility
- Performance benchmarks

---

## Dependencies

**Runtime:**
- `github.com/charmbracelet/huh` - TUI forms
- `github.com/charmbracelet/lipgloss` - Styling
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/fsnotify/fsnotify` - File watching

**Testing:**
- `github.com/golang/mock` - Mock generation (gomock)

**Build:**
- GoReleaser - Release automation with ldflags version injection

---

## Security Considerations

### Path Traversal Protection

**Validation:** `ValidateDestPath()` in filesystem.go

**Rejects:**
- Absolute paths: `/etc/passwd`, `C:\Windows\System32`
- Parent references: `../../../etc/passwd`

**Allows:**
- Relative paths within project: `internal/vendor/utils`

### License Compliance

**Approach:** Auto-detect and validate licenses

**Pre-approved:** MIT, Apache-2.0, BSD-3-Clause, etc.

**User confirmation:** Required for non-standard licenses

### Hook Execution

**Security model:** Same as npm scripts or Makefiles

**Runs with:** User permissions (no elevation)

**Sandboxing:** None (user is responsible for hook safety)

---

## Future Enhancements

**Potential features:**
- Checksum verification for vendored files
- Diff visualization in TUI
- Vendor search/discovery
- Plugin system for custom providers
- Git LFS support
- Monorepo-specific optimizations

---

## See Also

- [Commands Reference](./COMMANDS.md) - All available commands
- [Configuration Reference](./CONFIGURATION.md) - vendor.yml format
- [Advanced Usage](./ADVANCED.md) - Power user features
- [Contributing](../CONTRIBUTING.md) - Development guide
