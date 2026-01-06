# Release Process

Guide for releasing new versions of git-vendor.

## Overview

**We use GoReleaser for fully automated releases.** This handles building binaries, creating archives, generating checksums, creating the GitHub release, and auto-generating release notes from conventional commit messages.

## Prerequisites

- GoReleaser installed (`goreleaser`)
- GitHub CLI (`gh`) installed and authenticated
- Push access to repository
- **Repository must be public for releases** (see below)

### Making Repository Public (First-Time Setup)

**IMPORTANT:** GoReleaser requires a public repository. If your repository is private, make it public before creating releases.

```bash
# Option 1: GitHub CLI (recommended)
gh repo edit EmundoT/git-vendor --visibility public

# Verify it worked
gh repo view EmundoT/git-vendor --json isPrivate -q .isPrivate
# Should output: false

# Option 2: GitHub Web UI
# Navigate to Settings > Danger Zone > Change visibility > Change to public
```

**Note:** This is a one-time action. Once public, the repository remains public for all future releases.

## Standard Release Workflow

### Pre-Release Checklist

```bash
# 1. Run full test suite
make ci
make bench
go test -race ./...

# 2. Verify documentation examples
# Check all examples in README.md, CONFIGURATION.md, etc.

# 3. Update CHANGELOG.md for major releases (1.0.0, 2.0.0)
# Add comprehensive narrative for major milestones

# 4. Ensure working tree is clean
git status  # Should be clean
```

### Release Steps

```bash
# 1. Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0

Production-ready CLI tool for vendoring specific files/directories from Git
repositories with deterministic locking.

See CHANGELOG.md for details."

# 2. Push tag to remote
git push origin v1.0.0

# 3. Run GoReleaser (fully automated)
# Extract token from gh auth status (gh auth token is not a valid command)
export GITHUB_TOKEN=$(gh auth status --show-token 2>&1 | grep "Token:" | awk '{print $3}')
goreleaser release --clean

# 4. Verify release on GitHub
gh release view v1.0.0
```

### What GoReleaser Does

- ✅ Builds binaries for 6 platforms (Linux, macOS, Windows × amd64, arm64)
- ✅ Creates archives (.tar.gz for Unix, .zip for Windows)
- ✅ Generates SHA-256 checksums
- ✅ Creates GitHub release with tag
- ✅ Uploads all artifacts to GitHub
- ✅ Auto-generates changelog from git commits (conventional commits format)
- ✅ Adds installation instructions footer

**Configuration:** See `.goreleaser.yml` in repository root

**Output:** GitHub release at `github.com/EmundoT/git-vendor/releases/tag/v1.0.0`

### Post-Release Verification

```bash
# 1. Verify installation works
go install github.com/EmundoT/git-vendor@latest
git-vendor --version  # Should show the released version

# 2. Test binary downloads
gh release download <version> -p "*linux_amd64*"
tar -xzf git-vendor_*_linux_amd64.tar.gz
./git-vendor --version

# 3. Announce release (optional)
# - GitHub Discussions
# - Social media
# - Reddit (r/golang, etc.)
# - Dev.to/Hashnode blog post
```

---

## GoReleaser Configuration

The `.goreleaser.yml` file controls the release process:

**Key sections:**
- `before.hooks`: Commands to run before build (e.g., `go mod tidy`)
- `builds`: Platform targets and build flags
- `archives`: Archive format and included files (README.md, TROUBLESHOOTING.md)
- `checksum`: Checksum file generation
- `changelog`: Automatic changelog from conventional commits
- `release`: GitHub release settings

**Changelog auto-generation:**
```yaml
changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
```

GoReleaser scans commit messages following conventional commits format:
- `feat:` commits appear under "Features"
- `fix:` commits appear under "Bug Fixes"
- `docs:`, `test:`, `chore:` commits are excluded

**GitHub release configuration:**
```yaml
release:
  github:
    owner: EmundoT
    name: git-vendor
  draft: false           # Publish immediately
  prerelease: auto       # Auto-detect pre-release from tag (v1.0.0-beta.1)
  mode: append           # Append to existing release if found
  footer: |              # Added to all release notes
    ## Installation
    ...
```

---

## Version Bumping Strategy

### Semantic Versioning

- **MAJOR (1.0.0 → 2.0.0)**: Breaking changes, incompatible API changes
- **MINOR (1.0.0 → 1.1.0)**: New features, backwards-compatible
- **PATCH (1.0.0 → 1.0.1)**: Bug fixes, backwards-compatible

### Pre-release versions

- **Alpha**: `v1.0.0-alpha.1` - Early testing, unstable
- **Beta**: `v1.0.0-beta.1` - Feature complete, testing
- **RC**: `v1.0.0-rc.1` - Release candidate, final testing

---

## Troubleshooting

### "HTTP 404: Not Found (https://api.github.com/repos/...)"

**Cause:** Repository is private or doesn't exist

**Fix:**
- Make repository public before releasing
- Verify `GITHUB_TOKEN` has correct permissions

### "tag v1.0.0 does not exist"

**Cause:** Tag not pushed to remote

**Fix:**
```bash
git push origin v1.0.0
```

### "refusing to create a release from a dirty working tree"

**Cause:** Uncommitted changes or untracked files in working tree

**Fix:**
```bash
git status  # Check what's dirty

# Option 1: Commit changes
git add -A && git commit -m "Pre-release cleanup"

# Option 2: Stash changes
git stash

# Option 3: Temporarily move untracked files (e.g., docs not ready to commit)
mv docs/DRAFT.md /tmp/DRAFT.md
# After release: mv /tmp/DRAFT.md docs/DRAFT.md
```

### "net/http: invalid header field value for 'Authorization'"

**Cause:** `GITHUB_TOKEN` contains invalid characters (usually from using `gh auth token` which is not a valid command)

**Fix:**
```bash
# Use the correct method to extract token
export GITHUB_TOKEN=$(gh auth status --show-token 2>&1 | grep "Token:" | awk '{print $3}')
goreleaser release --clean
```

**Note:** `gh auth token` is NOT a valid gh CLI command despite appearing in many tutorials. Always use `gh auth status --show-token` to extract the token.

### "422 Validation Failed - already_exists" for release assets

**Cause:** GoReleaser partially succeeded in a previous run and assets already exist on the release

**What happened:**
- GoReleaser successfully created the GitHub release
- GoReleaser uploaded some or all assets
- An error occurred (network issue, rate limit, etc.)
- Retrying fails because assets can't be overwritten

**Fix:**
```bash
# Check if release already exists with assets
gh release view v1.0.0

# If assets are complete, you're done! Verify and proceed to post-release verification
gh release view v1.0.0

# If assets are incomplete, delete the release and retry
gh release delete v1.0.0 --yes
git push origin :refs/tags/v1.0.0  # Delete the tag from remote
git tag -d v1.0.0                   # Delete local tag

# Then recreate tag and release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
export GITHUB_TOKEN=$(gh auth status --show-token 2>&1 | grep "Token:" | awk '{print $3}')
goreleaser release --clean
```

### Auto-generated changelog looks incomplete

**Cause:** Commits don't follow conventional commits format, or important commits are excluded by filters

**Options:**
1. **Update commit messages** (if not pushed): Use `git rebase -i` to reword commits
2. **Use git tag message**: Add comprehensive release notes in the annotated tag message
3. **Edit release notes post-creation**: Use `gh release edit v1.0.0 --notes "Updated notes"`

For major releases (v1.0.0, v2.0.0), consider adding comprehensive context to CHANGELOG.md manually. GoReleaser's auto-generated notes are excellent for minor/patch releases but may lack narrative context for major milestones.

---

## Alternative Release Methods

### Local Testing (Snapshot Build)

Build and test release artifacts locally without publishing:

```bash
# Build locally (no GitHub release, no tag required)
goreleaser release --snapshot --clean

# Test the built binaries
./dist/git-vendor_linux_amd64_v1/git-vendor --version
```

This is useful for validating the build process before tagging.

### Manual Release with gh CLI

If GoReleaser is unavailable, you can build and release manually:

```bash
# 1. Build for all platforms
for GOOS in linux darwin windows; do
  for GOARCH in amd64 arm64; do
    EXT=""
    [[ $GOOS == "windows" ]] && EXT=".exe"
    GOOS=$GOOS GOARCH=$GOARCH go build \
      -ldflags="-s -w -X github.com/EmundoT/git-vendor/internal/version.Version=1.0.0" \
      -o "dist/git-vendor-$GOOS-$GOARCH$EXT"
  done
done

# 2. Create archives and checksums
# (manual tar/zip commands for each platform)

# 3. Create GitHub release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
gh release create v1.0.0 --generate-notes dist/*.tar.gz dist/*.zip dist/checksums.txt
```

This approach is more error-prone and not recommended for regular releases.

---

## gh CLI Quick Reference

Useful commands for managing releases:

```bash
# View release details
gh release view v1.0.0

# List all releases
gh release list

# Download release assets
gh release download v1.0.0

# Edit release notes after creation
gh release edit v1.0.0 --notes "Updated release notes"

# Create draft release (review before publishing)
gh release create v1.0.0 --draft --generate-notes

# Delete release (keep tag)
gh release delete v1.0.0

# Delete release and tag
gh release delete v1.0.0 --yes && git push origin :refs/tags/v1.0.0
```

---

## Quick Commands Summary

```bash
# Standard release workflow
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
export GITHUB_TOKEN=$(gh auth status --show-token 2>&1 | grep "Token:" | awk '{print $3}')
goreleaser release --clean

# Snapshot build (local testing, no publishing)
goreleaser release --snapshot --clean

# View release
gh release view v1.0.0

# Download and test binary
gh release download v1.0.0 -p "*linux_amd64*"
tar -xzf git-vendor_*_linux_amd64.tar.gz
./git-vendor --version
```

---

## See Also

- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [GitHub CLI Release Documentation](https://cli.github.com/manual/gh_release)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
