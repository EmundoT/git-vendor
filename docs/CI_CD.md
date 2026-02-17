# CI/CD Integration

git-vendor is designed for reproducible builds and works in automated pipelines.

## GitHub Actions

### Basic Workflow

```yaml
name: Build
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

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

### With Validation and Verification

```yaml
name: Vendor Check
on: [pull_request]

jobs:
  vendor-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install git-vendor
        run: |
          curl -L https://github.com/EmundoT/git-vendor/releases/latest/download/git-vendor-linux-amd64 -o /usr/local/bin/git-vendor
          chmod +x /usr/local/bin/git-vendor

      - name: Validate config
        run: git-vendor validate

      - name: Verify vendored files
        run: git-vendor verify

      - name: Scan for vulnerabilities
        run: git-vendor scan --fail-on high

      - name: Preview sync (dry run)
        run: git-vendor sync --dry-run
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## GitLab CI

```yaml
vendor-sync:
  stage: build
  script:
    - curl -L https://github.com/EmundoT/git-vendor/releases/latest/download/git-vendor-linux-amd64 -o /usr/local/bin/git-vendor
    - chmod +x /usr/local/bin/git-vendor
    - git-vendor sync
  variables:
    GITHUB_TOKEN: $CI_JOB_TOKEN
    GITLAB_TOKEN: $CI_JOB_TOKEN
```

## CircleCI

```yaml
jobs:
  vendor-sync:
    docker:
      - image: golang:1.23
    steps:
      - checkout
      - run:
          name: Install git-vendor
          command: go install github.com/EmundoT/git-vendor@latest
      - run:
          name: Sync dependencies
          command: git-vendor sync
          environment:
            GITHUB_TOKEN: ${GITHUB_TOKEN}
```

## Configuration Strategies

### Option 1: Pre-commit Lockfile (Recommended)

Commit both `.git-vendor/vendor.yml` and `.git-vendor/vendor.lock`:

```bash
git add .git-vendor/vendor.yml .git-vendor/vendor.lock
git commit -m "chore: update vendor dependencies"
```

CI workflow uses `git-vendor sync` which reads the committed lockfile.

**Benefits:**
- Deterministic builds (exact commit hashes)
- Fast CI (no update step needed)
- Changes are reviewable in PRs

### Option 2: Generate Lockfile in CI

Commit only `.git-vendor/vendor.yml`:

```bash
git-vendor update  # Generate lockfile from latest
git-vendor sync    # Download dependencies
```

**Drawbacks:**
- Non-deterministic (latest commits at CI runtime)
- Slower (update + sync)
- May break if upstream changes

## Caching

### GitHub Actions

```yaml
- uses: actions/cache@v4
  with:
    path: |
      .git-vendor/.cache/
    key: vendor-${{ hashFiles('.git-vendor/vendor.lock') }}
    restore-keys: vendor-
```

### GitLab CI

```yaml
cache:
  key: vendor-${CI_COMMIT_REF_SLUG}
  paths:
    - .git-vendor/.cache/
```

## Environment Variables

| Variable | Purpose | Platforms |
| --- | --- | --- |
| `GITHUB_TOKEN` | GitHub API access for license detection and private repos | All |
| `GITLAB_TOKEN` | GitLab API access for license detection and private repos | All |
| `GIT_VENDOR_CACHE_TTL` | Vulnerability scan cache duration (default: 24h) | All |

## Best Practices

1. **Commit the lockfile** for deterministic builds
2. **Use `git-vendor validate`** as a CI gate to catch config errors early
3. **Use `git-vendor verify`** to detect unauthorized modifications to vendored files
4. **Use `--parallel`** for projects with many vendors (3-5x speedup)
5. **Use `--verbose`** when debugging CI failures
6. **Cache `.git-vendor/.cache/`** to speed up incremental syncs
7. **Run `git-vendor scan`** to check for known CVEs in vendored dependencies

## Non-Interactive Mode

For CI environments, git-vendor auto-detects non-interactive terminals and skips prompts. For explicit control:

```bash
git-vendor sync --yes --quiet    # No prompts, minimal output
git-vendor verify --format=json  # Machine-readable output
git-vendor scan --format=json    # Machine-readable vulnerability report
```

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | Success / PASS |
| 1 | Failure / FAIL (modified/deleted files, vulnerabilities found) |
| 2 | Warning (added files, scan incomplete) |
