# Platform Support

git-vendor supports multiple Git hosting platforms with automatic detection and platform-specific features.

## Table of Contents

- [Supported Platforms Overview](#supported-platforms-overview)
- [GitHub](#github)
- [GitLab](#gitlab)
- [Bitbucket](#bitbucket)
- [Generic Git](#generic-git)
- [Authentication](#authentication)
- [Rate Limits](#rate-limits)
- [Platform Auto-Detection](#platform-auto-detection)

---

## Supported Platforms Overview

| Platform | Smart URLs | License API | Authentication | Private Repos |
|----------|-----------|-------------|----------------|---------------|
| **GitHub** | ✅ Yes | ✅ API | `GITHUB_TOKEN` | ✅ Yes |
| **GitLab** | ✅ Yes | ✅ API | `GITLAB_TOKEN` | ✅ Yes |
| **Bitbucket** | ✅ Yes | ❌ File-based | N/A | ❌ No |
| **Generic Git** | ❌ No | ❌ File-based | SSH/HTTPS | ✅ Yes (via SSH) |

---

## GitHub

**Hosts:** github.com and GitHub Enterprise

### Features

- **Smart URL Parsing**: Automatically extracts repository, ref, and path from `/blob/` and `/tree/` links
- **API-based License Detection**: Uses GitHub API to detect licenses (fast and reliable)
- **GitHub Enterprise Support**: Works with self-hosted GitHub Enterprise instances
- **Private Repository Support**: Requires `GITHUB_TOKEN` environment variable

### Examples

```bash
# Public repository
git-vendor add https://github.com/owner/repo

# Specific file with ref
git-vendor add https://github.com/owner/repo/blob/main/src/utils.go

# Directory with ref
git-vendor add https://github.com/owner/repo/tree/v1.0.0/lib/

# GitHub Enterprise
git-vendor add https://github.enterprise.com/team/project
```

### Smart URL Patterns

GitHub URLs are automatically parsed to extract:
- **Repository**: `github.com/owner/repo`
- **Ref**: Branch, tag, or commit from `/blob/{ref}/` or `/tree/{ref}/`
- **Path**: File or directory path after the ref

Example breakdown:
```
https://github.com/golang/go/blob/master/src/fmt/print.go
         └─────┬──────┘ └──┬──┘ └──┬───┘ └────────┬────────┘
            Repo         Ref    Type      Path
```

### Authentication

Set `GITHUB_TOKEN` to access private repositories and increase rate limits:

```bash
export GITHUB_TOKEN=ghp_your_token_here
git-vendor add https://github.com/your-org/private-repo
```

**How to create a token:**
1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate new token (classic)
3. Select scopes: `repo` (for private repos) or `public_repo` (for public only)
4. Copy the token and set as environment variable

---

## GitLab

**Hosts:** gitlab.com and self-hosted instances

### Features

- **Smart URL Parsing**: Automatically extracts repository, ref, and path from `/-/blob/` and `/-/tree/` links
- **API-based License Detection**: Uses GitLab API to detect licenses
- **Self-hosted Support**: Works with self-hosted GitLab instances
- **Nested Group Support**: Handles unlimited depth of nested groups
- **Private Repository Support**: Requires `GITLAB_TOKEN` environment variable

### Examples

```bash
# Public repository
git-vendor add https://gitlab.com/owner/repo

# Specific file with ref
git-vendor add https://gitlab.com/owner/repo/-/blob/main/lib/helper.go

# Directory with nested groups
git-vendor add https://gitlab.com/group/subgroup/project/-/tree/dev/src

# Self-hosted GitLab
git-vendor add https://gitlab.company.com/team/repo
```

### Smart URL Patterns

GitLab URLs use `/-/blob/` and `/-/tree/` delimiters:
```
https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/api/api.rb
        └────────┬────────┘ └─┬─┘ └──┬───┘ └─────┬─────┘
               Repo       Delim  Ref      Path
```

### Nested Groups

GitLab supports unlimited nested groups:
```bash
git-vendor add https://gitlab.com/company/team/subteam/project/-/blob/main/src/
                      └──────────────┬──────────────┘
                                   Group path
```

### Authentication

Set `GITLAB_TOKEN` to access private repositories:

```bash
export GITLAB_TOKEN=glpat_your_token_here
git-vendor add https://gitlab.com/your-org/private-repo
```

**How to create a token:**
1. Go to GitLab User Settings → Access Tokens
2. Create personal access token
3. Select scopes: `read_api` and `read_repository`
4. Copy the token and set as environment variable

---

## Bitbucket

**Hosts:** bitbucket.org

### Features

- **Smart URL Parsing**: Automatically extracts repository, ref, and path from `/src/` links
- **File-based License Detection**: Reads LICENSE file from repository
- **Public repositories only**: No private repository support currently

### Examples

```bash
# Public repository
git-vendor add https://bitbucket.org/owner/repo

# Specific file
git-vendor add https://bitbucket.org/owner/repo/src/main/utils.py

# Directory
git-vendor add https://bitbucket.org/owner/repo/src/develop/lib/
```

### Smart URL Patterns

Bitbucket URLs use `/src/` delimiter:
```
https://bitbucket.org/atlassian/python-bitbucket/src/master/setup.py
         └───────┬───────┘ └─┬─┘ └──┬──┘ └───┬────┘
               Repo       Path  Ref   File
```

### Limitations

- No API-based license detection (reads LICENSE file from repo)
- No authentication support (public repos only)
- License detection slower than GitHub/GitLab

---

## Generic Git

**Any Git server** accessible via git://, ssh://, or https://

### Features

- **Protocol Support**: git://, ssh://, https://
- **File-based License Detection**: Reads LICENSE file from repository
- **Private Repository Support**: Via SSH keys or HTTPS credentials

### Examples

```bash
# HTTPS
git-vendor add https://git.example.com/project/repo.git

# SSH
git-vendor add git@git.company.com:team/project.git

# Git protocol
git-vendor add git://git.kernel.org/pub/scm/git/git.git
```

### Authentication

For private repositories, use:

**SSH keys:**
```bash
# Ensure SSH key is added to ssh-agent
ssh-add ~/.ssh/id_rsa

# Use git@ URL
git-vendor add git@git.company.com:team/repo.git
```

**HTTPS with credentials:**
```bash
# Git will prompt for credentials or use git credential helper
git-vendor add https://git.company.com/team/repo.git
```

### Limitations

- No smart URL parsing (use base repository URL only)
- No API-based license detection
- Manual ref selection in wizard

---

## Authentication

### GitHub Token

**Purpose:**
- Access private repositories
- Increase rate limit from 60/hr to 5000/hr

**Setup:**
```bash
# Linux/macOS (persistent)
echo 'export GITHUB_TOKEN=ghp_your_token_here' >> ~/.bashrc
source ~/.bashrc

# Windows (persistent via System Properties → Environment Variables)
setx GITHUB_TOKEN "ghp_your_token_here"

# Temporary (current session)
export GITHUB_TOKEN=ghp_your_token_here
```

**Verify:**
```bash
echo $GITHUB_TOKEN
git-vendor add https://github.com/your-org/private-repo
```

### GitLab Token

**Purpose:**
- Access private repositories
- Increase rate limits

**Setup:**
```bash
# Linux/macOS (persistent)
echo 'export GITLAB_TOKEN=glpat_your_token_here' >> ~/.bashrc
source ~/.bashrc

# Temporary (current session)
export GITLAB_TOKEN=glpat_your_token_here
```

**Verify:**
```bash
echo $GITLAB_TOKEN
git-vendor add https://gitlab.com/your-org/private-repo
```

---

## Rate Limits

### GitHub

| Auth Type | Rate Limit | Notes |
|-----------|-----------|-------|
| **Unauthenticated** | 60 requests/hour | Shared by IP address |
| **With GITHUB_TOKEN** | 5,000 requests/hour | Per user |
| **GitHub Enterprise** | Varies | Configured by admin |

**Impact on git-vendor:**
- License detection: 1 request per vendor
- Update checking: 1 request per vendor
- Minimal impact for typical usage

### GitLab

| Auth Type | Rate Limit | Notes |
|-----------|-----------|-------|
| **Unauthenticated** | 300 requests/hour | For gitlab.com |
| **With GITLAB_TOKEN** | Varies | Typically higher |
| **Self-hosted** | Configurable | Set by admin |

### Bitbucket

No API rate limits (file-based license detection).

### Generic Git

No API usage (direct Git operations only).

---

## Platform Auto-Detection

git-vendor automatically detects the hosting platform from your URL - no configuration needed!

**Detection Rules:**

1. **GitHub**: URLs containing `github.com` or matching GitHub Enterprise patterns
2. **GitLab**: URLs containing `gitlab.com` or using `/-/blob/` or `/-/tree/` patterns
3. **Bitbucket**: URLs containing `bitbucket.org`
4. **Generic Git**: Everything else

**Examples:**

```bash
# Auto-detected as GitHub
git-vendor add https://github.com/golang/go/blob/master/src/fmt/print.go

# Auto-detected as GitLab
git-vendor add https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/api/api.rb

# Auto-detected as Bitbucket
git-vendor add https://bitbucket.org/atlassian/python-bitbucket/src/master/setup.py

# Auto-detected as Generic Git
git-vendor add https://git.kernel.org/pub/scm/git/git.git
```

**No configuration required** - just paste the URL and git-vendor handles the rest!

---

## Troubleshooting

### License Detection Failures

**GitHub/GitLab:**
- Verify API token is set correctly
- Check token has required scopes (`repo` or `read_repository`)
- Ensure repository is accessible with your credentials

**Bitbucket/Generic:**
- Ensure repository has LICENSE file in root directory
- Check file is named: `LICENSE`, `LICENSE.txt`, `LICENSE.md`, or `COPYING`
- Verify file permissions allow reading

### Private Repository Access

**GitHub:**
```bash
# Verify token
echo $GITHUB_TOKEN
# Should output: ghp_...

# Test access
git clone https://$GITHUB_TOKEN@github.com/your-org/private-repo.git /tmp/test
```

**GitLab:**
```bash
# Verify token
echo $GITLAB_TOKEN
# Should output: glpat_...

# Test access
git clone https://oauth2:$GITLAB_TOKEN@gitlab.com/your-org/private-repo.git /tmp/test
```

**Generic Git (SSH):**
```bash
# Test SSH connection
ssh -T git@git.company.com

# Verify SSH key is loaded
ssh-add -l
```

---

## See Also

- [Commands Reference](./COMMANDS.md) - All available commands
- [Configuration Reference](./CONFIGURATION.md) - vendor.yml format
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues
