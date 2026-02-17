# Frequently Asked Questions

## General

### How is git-vendor different from git submodules?

Submodules vendor entire repositories and create nested `.git` directories. git-vendor vendors specific files or directories as plain copies with deterministic locking. No nested repos, no complex `git submodule update --init --recursive` commands.

### Can I vendor from private repositories?

Yes. Set `GITHUB_TOKEN` or `GITLAB_TOKEN` environment variables for private GitHub/GitLab repos. For generic Git servers, configure SSH keys as you normally would for git operations.

### Does it work with non-GitHub repositories?

Yes. git-vendor supports GitHub, GitLab (including self-hosted), Bitbucket, and any Git server accessible via HTTPS, SSH, or git protocol.

### What languages/ecosystems does git-vendor support?

git-vendor is language-agnostic. It operates at the file level, so it works with any language, framework, or file type. Vendor Go files into a Python project, config files into a Node project, or protobuf definitions across your monorepo.

## Usage

### How do I update dependencies?

```bash
git-vendor update   # Fetch latest commits, update vendor.lock
git-vendor sync     # Download files at new locked versions
```

### How do I vendor multiple versions of the same library?

Use multi-ref tracking with different destination paths:

```yaml
vendors:
  - name: api-client
    url: https://github.com/company/api
    specs:
      - ref: v1.0
        mapping:
          - from: client
            to: vendor/api-v1
      - ref: v2.0
        mapping:
          - from: client
            to: vendor/api-v2
```

### Can I vendor specific lines from a file?

Yes, using position extraction syntax:

```yaml
mapping:
  - from: "api/constants.go:L4-L6"    # Lines 4-6
    to: "internal/constants.go"
  - from: "config.go:L10-EOF"          # Line 10 to end of file
    to: "internal/config_snippet.go"
```

See [Position Extraction](./CONFIGURATION.md) for full syntax.

### What if a branch name contains slashes?

Smart URL parsing cannot disambiguate slashes in branch names from path separators. Use the base repository URL in the wizard and manually enter the ref name (e.g., `feature/new-api`).

### Why is re-syncing so fast after the first time?

git-vendor uses incremental caching with SHA-256 file checksums. If files haven't changed since the last sync, git clone is skipped entirely (~80% faster). Use `--force` to bypass the cache, or `--no-cache` to disable caching.

## CI/CD

### Can I automate vendoring in CI/CD?

Yes. Use `--yes --quiet` flags for non-interactive mode. Commit both `vendor.yml` and `vendor.lock` for deterministic CI builds.

```bash
git-vendor sync --yes --quiet
```

See [CI/CD Guide](./CI_CD.md) for detailed pipeline configurations.

### Which files should I commit?

**Always commit:**
- `.git-vendor/vendor.yml` (configuration)
- `.git-vendor/vendor.lock` (deterministic lock)

**Optionally commit:**
- `.git-vendor/licenses/` (cached license files)
- Vendored files themselves (depends on your workflow)

**Never commit:**
- `.git-vendor/.cache/` (incremental sync cache)

## Troubleshooting

### "stale commit" error during sync

The locked commit hash no longer exists in the remote repository (force-pushed or rebased). Run `git-vendor update` to fetch the latest commit, then `git-vendor sync`.

### Sync is slow for large repositories

1. Use `--parallel` for multi-vendor projects (3-5x speedup)
2. Ensure incremental cache is enabled (default; disable with `--no-cache`)
3. git-vendor uses shallow clones (`--depth 1`) by default; full clones only happen as fallback

### License detection returns "NONE"

The repository may not have a LICENSE file, or the API token may lack permissions. Check:
1. Does the repo have a LICENSE/COPYING file?
2. Is `GITHUB_TOKEN`/`GITLAB_TOKEN` set correctly?
3. For private repos, does the token have `repo` scope?

### Files are out of sync after manual edits

Run `git-vendor verify` to check which files differ from the lockfile. Then either:
- `git-vendor sync --force` to restore vendored files
- `git-vendor update && git-vendor sync` to update to latest

For more issues, see [Troubleshooting](./TROUBLESHOOTING.md).
