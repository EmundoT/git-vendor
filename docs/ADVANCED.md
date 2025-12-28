# Advanced Usage

Advanced features and workflows for power users.

**Note:** All examples use `git-vendor`, but `git vendor` works identically as a git subcommand.

---

## Table of Contents

- [Multi-Ref Tracking](#multi-ref-tracking)
- [Auto-Naming Paths](#auto-naming-paths)
- [Custom Hooks](#custom-hooks)
- [Vendor Groups](#vendor-groups)
- [Incremental Sync Cache](#incremental-sync-cache)
- [Parallel Processing](#parallel-processing)
- [Watch Mode](#watch-mode)
- [CI/CD Integration](#cicd-integration)
- [Performance Tuning](#performance-tuning)

---

## Multi-Ref Tracking

Track multiple branches or tags from the same repository simultaneously.

**Use Cases:**
- Maintain multiple versions of a library (v1.x and v2.x)
- Track both stable and development branches
- Compare implementations across versions

**Configuration:**

```yaml
vendors:
  - name: multi-version-lib
    url: https://github.com/owner/repo
    license: MIT
    specs:
      - ref: v1.0
        mapping:
          - from: src
            to: vendor/lib-v1
      - ref: v2.0
        mapping:
          - from: src
            to: vendor/lib-v2
      - ref: main
        mapping:
          - from: src
            to: vendor/lib-dev
```

**How it works:**
- Each `spec` tracks a different git ref independently
- Separate lock file entries for each ref
- Independent sync and update per ref

**Example workflow:**
```bash
# Update all refs (v1.0, v2.0, and main)
git-vendor update

# Sync all versions
git-vendor sync

# Files now in:
# - vendor/lib-v1/ (from v1.0)
# - vendor/lib-v2/ (from v2.0)
# - vendor/lib-dev/ (from main)
```

---

## Auto-Naming Paths

Leave the destination path empty to use automatic naming based on the source path.

**Configuration:**

```yaml
vendors:
  - name: utils-lib
    url: https://github.com/owner/repo
    specs:
      - ref: main
        default_target: "lib/"  # Optional default directory
        mapping:
          - from: src/utils
            to: ""  # Auto-named as "lib/utils"
          - from: src/helpers.go
            to: ""  # Auto-named as "lib/helpers.go"
```

**Auto-naming logic:**

1. **With `default_target`**: Uses base name of source + default target
   - `from: src/utils` + `default_target: lib/` → `lib/utils`

2. **Without `default_target`**: Uses base name of source in current directory
   - `from: src/utils` → `utils`

3. **For root paths**: Falls back to vendor name
   - `from: .` → `{vendor-name}/`

**Examples:**

```yaml
# Example 1: Simple auto-naming
mapping:
  - from: src/utils
    to: ""  # → utils/

# Example 2: With default target
default_target: "vendor/"
mapping:
  - from: lib/helpers
    to: ""  # → vendor/helpers/

# Example 3: Single file
mapping:
  - from: src/config.go
    to: ""  # → config.go
```

---

## Custom Hooks

Run shell commands before and after vendor sync operations for workflow automation.

### Configuration

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

### Hook Types

| Hook | When | Failure Behavior |
|------|------|------------------|
| `pre_sync` | Before git clone/sync | **Stops sync** (vendor skipped) |
| `post_sync` | After successful sync | **Marks sync as failed** (files already copied) |

### Features

- **Full shell support**: Execute via `sh -c` (pipes, multiline scripts, etc.)
- **Multiline commands**: Use YAML `|` syntax for multi-line scripts
- **Cache-aware**: Hooks run even for cache hits (when git clone is skipped)
- **Environment variables**: Access sync context via env vars

### Environment Variables

Hooks have access to these variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `GIT_VENDOR_NAME` | Vendor name | `"frontend-lib"` |
| `GIT_VENDOR_URL` | Repository URL | `"https://github.com/..."` |
| `GIT_VENDOR_REF` | Git ref being synced | `"main"` |
| `GIT_VENDOR_COMMIT` | Resolved commit hash | `"abc123..."` |
| `GIT_VENDOR_ROOT` | Project root directory | `"/path/to/project"` |
| `GIT_VENDOR_FILES_COPIED` | Number of files copied | `"42"` |

### Example - Build Automation

```yaml
vendors:
  - name: ui-components
    url: https://github.com/company/ui
    hooks:
      post_sync: |
        cd vendor/ui-components
        npm install
        npm run build
        echo "Built $GIT_VENDOR_FILES_COPIED component files"
```

### Example - Notifications

```yaml
vendors:
  - name: api-client
    url: https://github.com/company/api
    hooks:
      pre_sync: echo "📦 Syncing $GIT_VENDOR_NAME from $GIT_VENDOR_URL"
      post_sync: |
        echo "✅ Synced $GIT_VENDOR_FILES_COPIED files at commit $GIT_VENDOR_COMMIT"
        curl -X POST https://slack.com/api/chat.postMessage \
          -d "text=Synced $GIT_VENDOR_NAME to $GIT_VENDOR_COMMIT"
```

### Example - Code Generation

```yaml
vendors:
  - name: protobuf-definitions
    url: https://github.com/company/protos
    hooks:
      post_sync: |
        cd vendor/protos
        protoc --go_out=. *.proto
        echo "Generated Go code from protos"
```

### Security Considerations

**Important:** Hooks execute arbitrary shell commands with your user permissions.

- ✅ **No sandboxing or privilege restrictions**
- ✅ **Same trust model** as npm scripts, git hooks, or Makefile targets
- ✅ **Runs in project root directory**
- ⚠️ **Only use hooks with vendor configurations you trust**

**Security best practices:**
1. Review hook commands before running `git-vendor sync`
2. Avoid untrusted vendor configurations
3. Use version control for `vendor.yml` to track hook changes
4. Consider using `--dry-run` to preview sync before executing hooks

---

## Vendor Groups

Organize vendors into logical groups for batch operations.

### Configuration

```yaml
vendors:
  - name: react-components
    url: https://github.com/company/react
    groups: ["frontend", "ui"]
    # ... rest of config ...

  - name: api-client
    url: https://github.com/company/api
    groups: ["backend", "api"]
    # ... rest of config ...

  - name: shared-utils
    url: https://github.com/company/utils
    groups: ["frontend", "backend", "shared"]
    # ... rest of config ...
```

### Usage

```bash
# Sync only frontend group
git-vendor sync --group frontend

# Sync only backend group
git-vendor sync --group backend

# Sync shared utilities
git-vendor sync --group shared
```

### Use Cases

**Environment-specific vendors:**
```yaml
vendors:
  - name: dev-tools
    groups: ["development"]
  - name: production-config
    groups: ["production"]
```

**Feature-based grouping:**
```yaml
vendors:
  - name: auth-service
    groups: ["authentication", "backend"]
  - name: payment-gateway
    groups: ["payments", "backend"]
  - name: ui-components
    groups: ["ui", "frontend"]
```

**Platform-specific:**
```yaml
vendors:
  - name: ios-sdk
    groups: ["mobile", "ios"]
  - name: android-sdk
    groups: ["mobile", "android"]
```

### Multiple Groups

Vendors can belong to multiple groups:

```yaml
vendors:
  - name: shared-utils
    groups: ["frontend", "backend", "mobile"]
```

This allows flexible batch operations:
```bash
# Sync all frontend (includes shared-utils)
git-vendor sync --group frontend

# Sync all backend (also includes shared-utils)
git-vendor sync --group backend
```

---

## Incremental Sync Cache

git-vendor caches file checksums to skip re-downloading unchanged files.

### How It Works

1. **After sync**: SHA-256 checksums calculated for all vendored files
2. **Stored in**: `vendor/.cache/<vendor-name>.json`
3. **Next sync**: Validates files exist and match cached checksums
4. **Cache hit**: Skips git clone entirely (⚡ 80% faster!)
5. **Cache miss**: Falls back to normal git clone and sync

### Cache Invalidation

Cache automatically invalidates when:
- Commit hash changes in `vendor.lock` (after `git-vendor update`)
- Files manually modified or deleted
- Vendor configuration changes (new mappings added)
- Using `--force` or `--no-cache` flags

### Performance

| Scenario | Speed | Notes |
|----------|-------|-------|
| **Cache hit** | ⚡⚡⚡ 80% faster | No git operations needed |
| **Cache miss** | Standard | Normal git clone + sync |
| **Partial cache** | ⚡ 30-50% faster | Some files cached |

### Cache Management

**Disable cache for single sync:**
```bash
git-vendor sync --no-cache
```

**Force re-download (bypasses cache):**
```bash
git-vendor sync --force
```

**Clear cache manually:**
```bash
rm -rf vendor/.cache/
```

**Inspect cache:**
```bash
cat vendor/.cache/my-vendor.json
```

Example cache file:
```json
{
  "vendor_name": "my-vendor",
  "ref": "main",
  "commit_hash": "abc123...",
  "cached_at": "2024-12-27T10:30:00Z",
  "files": [
    {
      "path": "lib/utils.go",
      "checksum": "sha256:def456..."
    }
  ]
}
```

### Limitations

- **File limit**: 1,000 files per vendor (prevents excessive memory usage)
- **No directory checksums**: Each file checksummed individually
- **Graceful degradation**: Cache failures don't fail sync

---

## Parallel Processing

Speed up multi-vendor operations with worker pools.

### Enable Parallel Mode

```bash
# Use default worker count (NumCPU, max 8)
git-vendor sync --parallel

# Custom worker count
git-vendor sync --parallel --workers 4

# Parallel update
git-vendor update --parallel
```

### How It Works

1. **Worker pool** created with N workers (default: NumCPU, max 8)
2. **Vendors dispatched** to available workers via job channel
3. **Each worker** processes vendors independently with unique temp directories
4. **Results collected** and lockfile written once at end
5. **Fail-fast** behavior: first error stops execution

### Performance

| Vendors | Sequential | Parallel (4 workers) | Speedup |
|---------|-----------|---------------------|---------|
| 1 | 10s | 10s | 1x |
| 3 | 30s | 12s | 2.5x |
| 5 | 50s | 15s | 3.3x |
| 10 | 100s | 25s | 4x |

**Best for:**
- ✅ Projects with 3+ vendors
- ✅ Large repositories
- ✅ Network-bound operations

**Not beneficial for:**
- ❌ Single vendor (no parallelization possible)
- ❌ 1-2 vendors (overhead exceeds benefit)
- ❌ CPU-bound operations (rare)

### Thread Safety

git-vendor ensures thread-safe parallel operations:

| Aspect | Thread Safety Approach |
|--------|----------------------|
| **Git operations** | Unique temp directories per vendor |
| **File writes** | Each vendor writes to different paths (no conflicts) |
| **Lockfile** | Results collected in goroutines, written once at end |
| **Progress tracking** | Thread-safe progress tracker |

**Tested with:**
- `go test -race` (no race conditions detected)
- All 55 existing tests pass in parallel mode

### Worker Count Tuning

**Default (NumCPU, max 8):**
```bash
git-vendor sync --parallel
```

**Custom worker count:**
```bash
# 2 workers (conservative)
git-vendor sync --parallel --workers 2

# 8 workers (aggressive)
git-vendor sync --parallel --workers 8
```

**Recommendations:**
- **Fast network**: Use more workers (4-8)
- **Slow network**: Use fewer workers (2-4)
- **Mixed vendor sizes**: Default is optimal
- **Large repos**: Reduce workers to avoid memory pressure

### Automatic Disabling

Parallel mode auto-disables for:
- Dry-run mode (`--dry-run`)
- Single vendor sync (`git-vendor sync my-vendor`)

---

## Watch Mode

Automatically sync vendors when `vendor.yml` is modified.

### Usage

```bash
git-vendor watch
```

**Output:**
```text
👁 Watching for changes to vendor/vendor.yml...
Press Ctrl+C to stop

📝 Detected change to vendor.yml
⠿ Syncing vendors...
✓ Sync completed

👁 Still watching for changes...
```

### How It Works

1. **Monitors** `vendor/vendor.yml` for file system changes
2. **Debounces** rapid changes (1 second delay)
3. **Automatically runs** `git-vendor sync` when changes detected
4. **Runs indefinitely** until Ctrl+C

### Use Cases

**Development workflow:**
```bash
# Terminal 1: Watch mode
git-vendor watch

# Terminal 2: Edit vendor.yml
vim vendor/vendor.yml
# Save → auto-sync triggers
```

**Live reloading:**
- Edit vendor configuration
- Automatic sync
- See changes immediately

### Editor Compatibility

| Editor | Compatibility | Notes |
|--------|--------------|-------|
| **VSCode** | ✅ Excellent | Direct writes trigger watch |
| **Sublime** | ✅ Good | Triggers correctly |
| **vim** | ⚠️ Varies | May use atomic writes (temp files) |
| **nano** | ✅ Good | Usually triggers |

**Tip:** If your editor doesn't trigger watch, save changes and manually touch the file:
```bash
touch vendor/vendor.yml
```

### Limitations

- Watches `vendor.yml` only (not other vendor files)
- Syncs **all vendors** on each change
- No selective sync (use manual `git-vendor sync <vendor>` instead)

---

## CI/CD Integration

git-vendor is designed for reproducible builds and works great in automated pipelines.

### Basic GitHub Actions Workflow

```yaml
name: Build
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install git-vendor
        run: |
          curl -L https://github.com/EmundoT/git-vendor/releases/latest/download/git-vendor-linux-amd64 -o /usr/local/bin/git-vendor
          chmod +x /usr/local/bin/git-vendor

      - name: Sync vendored dependencies
        run: git-vendor sync
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Build project
        run: make build
```

### Configuration Strategies

**Option 1: Pre-commit lockfile (Recommended)**

Commit both `vendor/vendor.yml` and `vendor/vendor.lock`:

```bash
git add vendor/vendor.yml vendor/vendor.lock
git commit -m "Add vendor dependencies"
```

**CI workflow:**
```bash
git-vendor sync  # Uses committed lockfile
```

**Benefits:**
- ✅ Deterministic builds (exact commit hashes)
- ✅ Fast CI (no update step needed)
- ✅ Reviewable in PRs

**Option 2: Generate lockfile in CI**

Commit only `vendor/vendor.yml`:

**CI workflow:**
```bash
git-vendor update  # Generate lockfile from latest
git-vendor sync    # Download dependencies
```

**Drawbacks:**
- ❌ Non-deterministic (latest commits at CI runtime)
- ❌ Slower (update + sync)
- ⚠️ May break if upstream changes

### Environment Variables

**GitHub Actions:**
```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  GITLAB_TOKEN: ${{ secrets.GITLAB_TOKEN }}
```

**GitLab CI:**
```yaml
variables:
  GITHUB_TOKEN: $CI_JOB_TOKEN
```

**CircleCI:**
```yaml
environment:
  GITHUB_TOKEN: ${GITHUB_TOKEN}
```

### Validation Step

Add validation to catch config errors:

```yaml
- name: Validate vendor config
  run: git-vendor validate
```

**Checks:**
- Duplicate vendor names
- Missing URLs/refs
- Empty path mappings
- Path conflicts

### Caching

Speed up builds by caching vendored files:

```yaml
- uses: actions/cache@v3
  with:
    path: |
      vendor/
      lib/
    key: vendor-${{ hashFiles('vendor/vendor.lock') }}
```

**Benefits:**
- Skip re-downloading on cache hit
- Faster CI runs
- Reduced network usage

### Best Practices

1. **Use verbose mode for debugging:**
   ```bash
   git-vendor sync --verbose
   ```

2. **Dry-run in pull requests:**
   ```yaml
   - name: Preview vendor changes
     run: git-vendor sync --dry-run
   ```

3. **Parallel sync for speed:**
   ```bash
   git-vendor sync --parallel
   ```

4. **Fail fast on validation:**
   ```bash
   git-vendor validate || exit 1
   ```

---

## Performance Tuning

Optimize git-vendor for your workflow.

### Sync Performance

| Technique | Speedup | When to Use |
|-----------|---------|-------------|
| **Incremental cache** | 80% | Re-syncing unchanged vendors |
| **Parallel processing** | 3-5x | Multiple vendors (3+) |
| **Vendor groups** | Variable | Syncing subsets |
| **Shallow clones** | 20-40% | Large repos (automatic) |

### Best Practices

**1. Use incremental cache:**
```bash
# Normal sync (uses cache)
git-vendor sync

# Only bypass when needed
git-vendor sync --no-cache
```

**2. Enable parallel for multi-vendor:**
```bash
# 5+ vendors? Use parallel
git-vendor sync --parallel
```

**3. Organize with groups:**
```yaml
vendors:
  - name: frontend-lib
    groups: ["frontend"]
  - name: backend-lib
    groups: ["backend"]
```

```bash
# Sync only what you need
git-vendor sync --group frontend
```

**4. Lock to stable refs:**
```yaml
# Prefer tags over branches
specs:
  - ref: v1.2.3  # ✅ Stable
  # - ref: main    # ❌ Changes frequently
```

**5. Minimize path mappings:**
```yaml
# ❌ Avoid many small mappings
mapping:
  - from: file1.go
    to: lib/file1.go
  - from: file2.go
    to: lib/file2.go

# ✅ Use directory mappings
mapping:
  - from: src/
    to: lib/
```

### Troubleshooting Slow Syncs

**Symptom:** Sync takes too long

**Check:**
1. **Network speed:** `ping github.com`
2. **Repository size:** `git clone --depth 1 <url> test`
3. **Cache status:** Check `vendor/.cache/`
4. **Parallel mode:** Try `--parallel` flag

**Solutions:**
- Use `--parallel` for multiple vendors
- Ensure cache is enabled (no `--no-cache`)
- Use `--group` to sync subsets
- Consider vendor-specific optimization (contact repo owner)

---

## See Also

- [Commands Reference](./COMMANDS.md) - All available commands
- [Configuration Reference](./CONFIGURATION.md) - vendor.yml format
- [Platform Support](./PLATFORMS.md) - GitHub, GitLab, Bitbucket
- [Troubleshooting](../TROUBLESHOOTING.md) - Common issues
