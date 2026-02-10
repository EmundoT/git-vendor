# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in git-vendor, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

### How to Report

1. Email: Send details to the repository maintainers via GitHub's private vulnerability reporting feature
2. Go to **Security** > **Advisories** > **Report a vulnerability** on the [GitHub repository](https://github.com/EmundoT/git-vendor/security/advisories/new)

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 5 business days
- **Fix release**: Within 30 days for critical issues

## Security Model

### Trust Boundaries

git-vendor operates within the following trust model:

- **vendor.yml is trusted input**: Like `package.json` or `Makefile`, the configuration file is authored by project maintainers. Hook commands execute with the user's permissions.
- **Remote repositories are partially trusted**: git-vendor clones and copies files from configured URLs. It does NOT execute code from remote repositories (except via explicit hooks).
- **License files are cached locally**: License detection queries public APIs (GitHub, GitLab) or reads LICENSE files from cloned repos.

### Hook Execution

Pre/post-sync hooks (`hooks.pre_sync`, `hooks.post_sync` in vendor.yml) execute arbitrary shell commands:

- Commands run with the current user's permissions
- Commands run in the project root directory
- No sandboxing or privilege escalation restrictions
- 5-minute timeout prevents hanging hooks
- Environment variable values are sanitized (newlines/null bytes stripped)

This follows the same security model as npm scripts, git hooks, and Makefile targets.

### Path Traversal Protection

All file copy operations validate destination paths via `ValidateDestPath`:

- Rejects absolute paths (e.g., `/etc/passwd`)
- Rejects parent directory traversal (e.g., `../../../etc/passwd`)
- Only allows relative paths within the project directory

### Network Access

git-vendor makes network requests for:

- Git clone/fetch operations (via system `git`)
- GitHub API license detection (when `GITHUB_TOKEN` is set)
- GitLab API license detection (when `GITLAB_TOKEN` is set)
- OSV.dev API for vulnerability scanning (`git-vendor scan`)

### Environment Variables

- `GITHUB_TOKEN` - Used for GitHub API access (license detection, private repos)
- `GITLAB_TOKEN` - Used for GitLab API access (license detection, private repos)
- `GIT_VENDOR_CACHE_TTL` - Controls vulnerability scan cache duration

These tokens are passed to git operations and API requests. They are NOT logged or stored on disk.

## Dependencies

git-vendor minimizes its dependency surface:

- **Runtime**: git-plumbing (git CLI wrapper), charmbracelet/huh (TUI), yaml.v3, fsnotify
- **No CGO**: Pure Go binary, no native dependencies
- **Vendored git-plumbing**: The git-plumbing dependency is self-vendored to `pkg/git-plumbing/`

## Binary Verification

Release binaries are built via GoReleaser with checksums. Verify downloads against the published checksums in each release.
