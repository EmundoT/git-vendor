# Multi-Remote Support — Implementation Plan

## Problem Statement

Currently, each `VendorSpec` has a single `URL` field. If that remote is unreachable (GitHub outage, corporate firewall, geo-restriction), sync/update fails entirely. Multi-remote support enables:

1. **Mirror failover**: Try `github.com/org/repo`, fall back to `gitlab.com/org/repo-mirror`.
2. **Air-gapped environments**: Corporate mirror as primary, public URL as secondary (or vice versa).
3. **Migration**: Transition from one hosting provider to another without editing every consumer's config simultaneously.
4. **Read performance**: Prefer a geographically-closer mirror for faster clones.

## Current State

### Hardcoded "origin" in Production Code

Every git operation that touches a remote uses the literal string `"origin"` as the remote name. There is no abstraction layer for remote selection.

| File | Line | Usage |
|------|------|-------|
| `internal/core/git_operations.go` | 68 | `Fetch()` hardcodes `g.gitFor(dir).Fetch(ctx, "origin", ref, depth)` |
| `internal/core/git_operations.go` | 73 | `FetchAll()` hardcodes `g.gitFor(dir).FetchAll(ctx, "origin")` |
| `internal/core/sync_service.go` | 526 | `SyncVendor()` calls `AddRemote(ctx, tempDir, "origin", cloneURL)` |
| `internal/core/diff_service.go` | 73 | `DiffVendor()` calls `AddRemote(ctx, tempDir, "origin", vendor.URL)` |
| `internal/core/drift_service.go` | 177 | `driftForSpec()` calls `AddRemote(ctx, tempDir, "origin", vendor.URL)` |
| `internal/core/drift_service.go` | 211 | Checkout uses `"origin/"+spec.Ref` as fallback |
| `internal/core/update_checker.go` | 125 | `fetchLatestHash()` calls `AddRemote(ctx, tempDir, "origin", url)` |
| `main.go` | 125 | `GetRemoteURL(ctx, "origin")` for TUI auto-fill |

### Current Remote Lifecycle

Every sync/update/diff/drift operation follows the same pattern:

```text
1. Create temp dir
2. git init
3. git remote add origin <vendor.URL>
4. git fetch origin <ref> --depth=1   (or full fetch as fallback)
5. git checkout <hash or FETCH_HEAD>
6. Copy files
7. Remove temp dir
```

The remote is ephemeral — created per-operation in a temp directory, never persisted. This is actually favorable for multi-remote: we can try remote-1, and on failure, try remote-2 in the same temp dir (just `git remote set-url origin <url2>` or add a second named remote).

### GitClient Interface (Current)

```go
type GitClient interface {
    Init(ctx context.Context, dir string) error
    AddRemote(ctx context.Context, dir, name, url string) error
    Fetch(ctx context.Context, dir string, depth int, ref string) error
    FetchAll(ctx context.Context, dir string) error
    Checkout(ctx context.Context, dir, ref string) error
    GetHeadHash(ctx context.Context, dir string) (string, error)
    Clone(ctx context.Context, dir, url string, opts *types.CloneOptions) error
    ListTree(ctx context.Context, dir, ref, subdir string) ([]string, error)
    GetCommitLog(ctx context.Context, dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error)
    GetTagForCommit(ctx context.Context, dir, commitHash string) (string, error)
    Add(ctx context.Context, dir string, paths ...string) error
    Commit(ctx context.Context, dir string, opts types.CommitOptions) error
    AddNote(ctx context.Context, dir, noteRef, commitHash, content string) error
    GetNote(ctx context.Context, dir, noteRef, commitHash string) (string, error)
    ConfigSet(ctx context.Context, dir, key, value string) error
    ConfigGet(ctx context.Context, dir, key string) (string, error)
    LsRemote(ctx context.Context, url, ref string) (string, error)
}
```

Key observations:
- `Fetch` and `FetchAll` hardcode `"origin"` inside `SystemGitClient` — callers have no control.
- `AddRemote` already accepts a `name` parameter — callers just always pass `"origin"`.
- `LsRemote` takes a URL directly (no remote name), so it already works for any URL.
- `Clone` takes a URL directly — no remote name involvement.

## Proposed Design

### Data Model Changes

#### VendorSpec — `internal/types/types.go`

```go
// VendorSpec defines a single vendored dependency with source repository URL and path mappings.
type VendorSpec struct {
    Name       string       `yaml:"name"`
    URL        string       `yaml:"url"`                    // Primary URL (unchanged)
    Mirrors    []string     `yaml:"mirrors,omitempty"`      // Fallback URLs, tried in order
    License    string       `yaml:"license"`
    Groups     []string     `yaml:"groups,omitempty"`
    Hooks      *HookConfig  `yaml:"hooks,omitempty"`
    Source     string       `yaml:"source,omitempty"`
    Compliance string       `yaml:"compliance,omitempty"`
    Specs      []BranchSpec `yaml:"specs"`
}
```

**Rationale for `Mirrors []string` over `Remotes []RemoteSpec`:**
- Keeps vendor.yml simple. Users add 1-2 mirror URLs, not named remote objects.
- The primary URL stays in `URL` (backward compatible — zero-mirror configs work identically).
- All mirrors share the same ref/mapping config (mirrors MUST host the same repository content).
- If a future use case needs per-mirror auth hints or priority weights, `Mirrors` can be promoted to `[]MirrorSpec` in a minor schema bump.

**vendor.yml example (new format):**

```yaml
vendors:
  - name: my-lib
    url: https://github.com/org/my-lib
    mirrors:
      - https://gitlab.com/org/my-lib-mirror
      - https://internal-git.corp.com/mirrors/my-lib
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: src/utils.go
            to: pkg/vendored/utils.go
```

**vendor.yml example (unchanged, backward compatible):**

```yaml
vendors:
  - name: my-lib
    url: https://github.com/org/my-lib
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: src/utils.go
            to: pkg/vendored/utils.go
```

#### LockDetails — `internal/types/types.go`

```go
type LockDetails struct {
    Name        string            `yaml:"name"`
    Ref         string            `yaml:"ref"`
    CommitHash  string            `yaml:"commit_hash"`
    SourceURL   string            `yaml:"source_url,omitempty"`  // NEW: which URL was actually used
    LicensePath string            `yaml:"license_path"`
    Updated     string            `yaml:"updated"`
    FileHashes  map[string]string `yaml:"file_hashes,omitempty"`
    // ... remaining fields unchanged
}
```

**Rationale for `SourceURL`:**
- Provenance tracking: know which mirror actually served the content.
- Debugging: if a mirror serves stale data, the lockfile reveals which mirror was used.
- Optional: empty means the primary `URL` was used (backward compatible).

**Schema version bump:** `1.2` -> `1.3` (minor bump, backward compatible — new optional field).

#### Helper: Resolve URLs for a Vendor

A new utility function consolidates URL resolution across all call sites:

```go
// ResolveVendorURLs returns the ordered list of URLs to try for a vendor.
// Primary URL first, then mirrors. All URLs are validated.
// For internal vendors (Source == "internal"), returns nil.
func ResolveVendorURLs(v *types.VendorSpec) []string {
    if v.Source == SourceInternal {
        return nil
    }
    urls := make([]string, 0, 1+len(v.Mirrors))
    urls = append(urls, v.URL)
    urls = append(urls, v.Mirrors...)
    return urls
}
```

### GitClient Interface Changes

The `GitClient` interface itself requires **two changes**:

```go
type GitClient interface {
    // Existing — unchanged
    Init(ctx context.Context, dir string) error
    AddRemote(ctx context.Context, dir, name, url string) error
    Checkout(ctx context.Context, dir, ref string) error
    GetHeadHash(ctx context.Context, dir string) (string, error)
    Clone(ctx context.Context, dir, url string, opts *types.CloneOptions) error
    ListTree(ctx context.Context, dir, ref, subdir string) ([]string, error)
    GetCommitLog(ctx context.Context, dir, oldHash, newHash string, maxCount int) ([]types.CommitInfo, error)
    GetTagForCommit(ctx context.Context, dir, commitHash string) (string, error)
    Add(ctx context.Context, dir string, paths ...string) error
    Commit(ctx context.Context, dir string, opts types.CommitOptions) error
    AddNote(ctx context.Context, dir, noteRef, commitHash, content string) error
    GetNote(ctx context.Context, dir, noteRef, commitHash string) (string, error)
    ConfigSet(ctx context.Context, dir, key, value string) error
    ConfigGet(ctx context.Context, dir, key string) (string, error)
    LsRemote(ctx context.Context, url, ref string) (string, error)

    // CHANGED: Accept remote name parameter instead of hardcoding "origin"
    Fetch(ctx context.Context, dir, remote string, depth int, ref string) error
    FetchAll(ctx context.Context, dir, remote string) error

    // NEW: Update an existing remote's URL (for failover retry)
    SetRemoteURL(ctx context.Context, dir, name, url string) error
}
```

**`SystemGitClient` implementation changes:**

```go
// Fetch fetches from the named remote with optional depth.
func (g *SystemGitClient) Fetch(ctx context.Context, dir, remote string, depth int, ref string) error {
    return g.gitFor(dir).Fetch(ctx, remote, ref, depth)
}

// FetchAll fetches all refs from the named remote.
func (g *SystemGitClient) FetchAll(ctx context.Context, dir, remote string) error {
    return g.gitFor(dir).FetchAll(ctx, remote)
}

// SetRemoteURL updates the URL of an existing remote.
func (g *SystemGitClient) SetRemoteURL(ctx context.Context, dir, name, url string) error {
    return g.gitFor(dir).SetRemoteURL(ctx, name, url)
}
```

**Note:** `git-plumbing` may need a `SetRemoteURL` method added (delegates to `git remote set-url <name> <url>`). If not available, the fallback is `git config remote.<name>.url <url>` via `ConfigSet`.

### Service Layer Changes

#### New: `remote_fallback.go` — Centralized Failover Logic

```go
// FetchWithFallback tries fetching from each URL in order until one succeeds.
// On success, returns the URL that worked and nil error.
// On total failure, returns empty string and the last error encountered.
//
// Strategy: init + add remote "origin" with first URL, fetch. On failure,
// set-url to next mirror and retry. This avoids multiple named remotes
// (which would complicate ref resolution like "origin/main" vs "mirror1/main").
func FetchWithFallback(
    ctx context.Context,
    gitClient GitClient,
    tempDir string,
    urls []string,
    ref string,
    depth int,
) (usedURL string, err error) {
    if len(urls) == 0 {
        return "", fmt.Errorf("no URLs provided")
    }

    // Add first URL as "origin"
    if err := gitClient.AddRemote(ctx, tempDir, "origin", urls[0]); err != nil {
        return "", fmt.Errorf("add remote: %w", err)
    }

    // Try each URL
    var lastErr error
    for i, url := range urls {
        if i > 0 {
            // Switch to next mirror
            if err := gitClient.SetRemoteURL(ctx, tempDir, "origin", url); err != nil {
                lastErr = fmt.Errorf("set remote URL to %s: %w", SanitizeURL(url), err)
                continue
            }
        }

        // Attempt fetch
        fetchErr := gitClient.Fetch(ctx, tempDir, "origin", depth, ref)
        if fetchErr == nil {
            return url, nil
        }
        lastErr = fetchErr

        // Log mirror failure (not fatal yet)
        fmt.Printf("  ⚠ Mirror %s failed: %v\n", SanitizeURL(url), fetchErr)
    }

    return "", fmt.Errorf("all remotes failed for ref %s (last error: %w)", ref, lastErr)
}
```

#### `sync_service.go` Modifications

**`SyncVendor` method:**

Current code (lines 522-528):
```go
if err := s.gitClient.Init(ctx, tempDir); err != nil { ... }
if err := s.gitClient.AddRemote(ctx, tempDir, "origin", cloneURL); err != nil { ... }
```

Proposed replacement:
```go
if err := s.gitClient.Init(ctx, tempDir); err != nil { ... }

// Resolve all URLs (primary + mirrors), applying --local gating to each
urls, err := s.resolveAllURLs(v, opts)
if err != nil {
    return nil, CopyStats{}, err
}
```

**`syncRef` / `fetchWithFallback` method:**

The existing `fetchWithFallback` (shallow-then-full for a single remote) gets wrapped by the new multi-URL fallback:

```go
func (s *SyncService) fetchWithRemoteFallback(ctx context.Context, tempDir string, urls []string, ref string) (string, error) {
    // Try shallow fetch across all mirrors first
    usedURL, err := FetchWithFallback(ctx, s.gitClient, tempDir, urls, ref, 1)
    if err == nil {
        return usedURL, nil
    }
    // Shallow failed on all mirrors; try full fetch on all mirrors
    usedURL, err = FetchWithFallback(ctx, s.gitClient, tempDir, urls, ref, 0)
    if err != nil {
        return "", fmt.Errorf("fetch ref %s: %w", ref, err)
    }
    return usedURL, nil
}
```

The `usedURL` is threaded back to the caller so it can be recorded in `LockDetails.SourceURL`.

**`RefMetadata` struct change:**

```go
type RefMetadata struct {
    CommitHash string
    VersionTag string
    Positions  []positionRecord
    SourceURL  string  // NEW: which mirror URL succeeded
}
```

#### `diff_service.go` / `drift_service.go` / `update_checker.go` Modifications

All three follow the same `init + AddRemote("origin", vendor.URL) + Fetch` pattern. Each gets the same treatment:

1. Replace `vendor.URL` with `ResolveVendorURLs(vendor)`.
2. Replace manual `AddRemote` + `Fetch` with `FetchWithFallback(...)`.
3. Update `Fetch(ctx, dir, depth, ref)` calls to `Fetch(ctx, dir, "origin", depth, ref)` (new signature).

#### `outdated_service.go` Modifications

`OutdatedService.Outdated()` uses `LsRemote(ctx, vendor.URL, spec.Ref)` which takes a URL directly — no temp dir. Multi-remote support here means trying each URL in order:

```go
func (s *OutdatedService) lsRemoteWithFallback(ctx context.Context, urls []string, ref string) (string, error) {
    var lastErr error
    for _, url := range urls {
        hash, err := s.gitClient.LsRemote(ctx, url, ref)
        if err == nil {
            return hash, nil
        }
        lastErr = err
    }
    return "", lastErr
}
```

#### `remote_explorer.go` Modifications

`FetchRepoDir` uses `Clone(ctx, tempDir, url, opts)` which takes a URL directly. For multi-remote, try each URL until clone succeeds:

```go
for _, url := range urls {
    if err := e.gitClient.Clone(ctx, tempDir, url, opts); err == nil {
        break
    }
    lastErr = err
    // Clean temp dir for retry
    _ = e.fs.RemoveAll(tempDir)
    tempDir, _ = e.fs.CreateTemp("", "git-vendor-index-*")
}
```

### CLI Changes

#### New Flags

No new flags needed for basic multi-remote. Configuration happens in `vendor.yml` via the `mirrors` field. The existing `--local` flag applies to all URLs (primary + mirrors).

#### Config Management (TUI Wizard)

The `tui.RunAddVendorWizard` and `tui.RunEditVendorWizard` should gain an optional "Mirror URLs" input (comma-separated or multi-line). This is a TUI-only change, not a new CLI flag.

#### Config Commands (Spec 072)

The `config add-mirror` and `config remove-mirror` subcommands provide non-interactive mirror management:

```text
git-vendor config add-mirror <vendor-name> <mirror-url>
git-vendor config remove-mirror <vendor-name> <mirror-url>
git-vendor config list-mirrors <vendor-name>
```

These are additive subcommands under the existing `config` command namespace.

### Validation Changes

#### `validation_service.go`

- Validate all mirror URLs with `ValidateVendorURL()` (same rules as primary URL).
- Validate that mirrors are not duplicates of the primary URL.
- Validate that internal vendors (`Source: "internal"`) have no mirrors (mirrors are meaningless for same-repo vendors).

## Migration / Backward Compatibility

### Zero-Change for Existing Configs

- `Mirrors` field is `omitempty` — existing YAML without `mirrors:` deserializes to `nil`.
- `ResolveVendorURLs()` with nil mirrors returns `[]string{v.URL}` — single-URL behavior.
- `LockDetails.SourceURL` is `omitempty` — existing lockfiles are unaffected.
- Schema version bumps from `1.2` to `1.3` (minor) — old CLIs read `1.3` with a warning, new CLIs read `1.2` without issue.

### Interface Change Migration

The `Fetch(ctx, dir, depth, ref)` -> `Fetch(ctx, dir, remote, depth, ref)` signature change is **breaking for mock consumers**. All test files with `mockGit.EXPECT().Fetch(...)` need updating. This is a one-time mechanical change (add `"origin"` parameter to every existing call).

**Affected test files (from grep):**
- `sync_service_test.go` (many calls)
- `diff_service_test.go`
- `drift_service_test.go`
- `update_checker_test.go`
- `file_copy_service_test.go`
- `vendor_repository_test.go`
- `integration_test.go`

After updating the interface, `make mocks` regenerates the mock, and all test call sites get the extra `"origin"` argument.

## Implementation Phases

### Phase 1: GitClient Interface Refactor (No Behavioral Change)

**Goal:** Remove hardcoded "origin" from `SystemGitClient`, add `remote` parameter to `Fetch`/`FetchAll`.

1. Add `SetRemoteURL` to `GitClient` interface and `SystemGitClient`.
2. Change `Fetch(ctx, dir, depth, ref)` to `Fetch(ctx, dir, remote, depth, ref)`.
3. Change `FetchAll(ctx, dir)` to `FetchAll(ctx, dir, remote)`.
4. Update all callers to pass `"origin"` explicitly — pure mechanical change, zero behavioral difference.
5. `make mocks` + update all test expectations.
6. Verify: `go test ./...` passes, `go vet ./...` clean.

**Dependency:** May need `git-plumbing` changes for `SetRemoteURL`. If not available, implement via `ConfigSet(ctx, dir, "remote.origin.url", newURL)`.

### Phase 2: Data Model + Resolution

**Goal:** Add `Mirrors` to `VendorSpec`, `SourceURL` to `LockDetails`, implement `ResolveVendorURLs`.

1. Add `Mirrors []string` to `VendorSpec` in `types.go`.
2. Add `SourceURL string` to `LockDetails` in `types.go`.
3. Implement `ResolveVendorURLs()` in a new `remote_fallback.go`.
4. Implement `FetchWithFallback()` in `remote_fallback.go`.
5. Bump `CurrentSchemaVersion` to `"1.3"`, bump `MaxSupportedMinor` to `3`.
6. Add validation rules for mirrors in `validation_service.go`.
7. Unit tests for `ResolveVendorURLs`, `FetchWithFallback`, mirror validation.

### Phase 3: Service Integration

**Goal:** Wire multi-remote into sync, update, diff, drift, outdated, explorer.

1. **sync_service.go**: Replace `AddRemote + fetchWithFallback` with `FetchWithFallback`. Thread `usedURL` into `RefMetadata.SourceURL`.
2. **update_service.go**: Record `SourceURL` in `LockDetails` when building lock entries.
3. **diff_service.go**: Use `FetchWithFallback` instead of single `AddRemote + Fetch`.
4. **drift_service.go**: Use `FetchWithFallback` instead of single `AddRemote + Fetch`.
5. **update_checker.go**: Use `FetchWithFallback` instead of single `AddRemote + Fetch`.
6. **outdated_service.go**: Implement `lsRemoteWithFallback`, iterate mirrors.
7. **remote_explorer.go**: Iterate mirrors in `FetchRepoDir`.
8. Integration tests with a mock that fails on first URL, succeeds on second.

### Phase 4: CLI + TUI

**Goal:** User-facing mirror management.

1. Add `config add-mirror`, `config remove-mirror`, `config list-mirrors` subcommands.
2. Update TUI wizard to show/edit mirrors.
3. Update `--json` output schemas to include mirrors and source_url.
4. Update `docs/COMMANDS.md` and `docs/CONFIGURATION.md`.

## Risks and Open Questions

### Open Questions

1. **Mirror content identity**: Should git-vendor verify that all mirrors serve the same commit hash for a given ref? A mirror could be stale. Options:
   - (a) Trust the first successful fetch (current plan).
   - (b) LsRemote all mirrors first, warn if hashes diverge, use the primary's hash.
   - Recommendation: (a) for v1, (b) as a follow-up `--verify-mirrors` flag.

2. **Per-ref mirror overrides**: Should different refs be fetchable from different mirrors? E.g., `main` from GitHub, `enterprise-patches` from GitLab. Current plan says no — mirrors are repo-level, not ref-level. This keeps the model simple.

3. **Auth per mirror**: Different mirrors may need different credentials (GITHUB_TOKEN vs GITLAB_TOKEN). Git handles this via credential helpers and `.gitconfig`, so no git-vendor changes needed. But should we document this?

4. **Mirror ordering**: Should users be able to set priority? Current plan: `URL` is always first, `Mirrors` are tried in declaration order. A `priority` field adds complexity for minimal gain.

5. **git-plumbing SetRemoteURL**: Does git-plumbing already expose `git remote set-url`? If not, we need to add it or use `ConfigSet("remote.origin.url", ...)` as a workaround.

### Risks

1. **Interface breakage blast radius**: Changing `Fetch`/`FetchAll` signatures touches every test file that mocks `GitClient`. Mitigation: Phase 1 is a pure mechanical refactor — do it in a single commit, verify all tests pass.

2. **Retry latency**: Failing over across 3 mirrors means 3x timeout on network errors. Mitigation: Use short per-mirror timeouts (e.g., 30s), total timeout controlled by parent context.

3. **Mirror staleness**: A mirror returns a stale commit hash — sync succeeds but with older content than expected. Mitigation: Document that mirrors should be kept in sync. Consider `SourceURL` in lockfile for post-hoc auditing.

4. **YAML list ordering stability**: Go's `yaml.v3` preserves slice order, so `mirrors` declaration order is stable. No risk here.

### Non-Goals

- **Write-back to mirrors**: git-vendor is read-only from vendor repos. No push/mirror-sync operations.
- **Mirror health monitoring**: No persistent state about which mirrors are "healthy". Each operation starts fresh.
- **Automatic mirror discovery**: No DNS-based or API-based mirror enumeration. Users declare mirrors explicitly.
