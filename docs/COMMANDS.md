# Command Reference

> **Authoritative source:** [`CLAUDE.md`](../CLAUDE.md) — the project primer contains the complete, maintained command documentation with flags, implementation details, and usage patterns.
>
> This file is a thin index. See CLAUDE.md for full details.

## Invocation

`git-vendor` or `git vendor` — both work identically.

## Core Commands

| Command | Purpose |
|---------|---------|
| `pull [name]` | Fetch latest from upstream, update lock, copy files. Replaces `update` + `sync`. |
| `push [name]` | Propose local vendored file changes upstream via PR. |
| `status` | Unified inspection: lock vs disk (offline) + lock vs upstream (remote). |
| `accept [name]` | Acknowledge intentional local drift to vendored files. |
| `cascade` | Transitive graph pull across sibling projects in topological order. |

## Supporting Commands

| Command | Purpose |
|---------|---------|
| `init` | Create `.git-vendor/` directory structure. |
| `add` | Interactive wizard to register a new vendor. |
| `edit` | Edit an existing vendor spec. |
| `remove` | Remove vendor + lock + files. |
| `list` | List all vendors. |
| `validate` | Validate vendor.yml config. |
| `compliance` | Show effective enforcement levels per vendor (Spec 075). |
| `hook install` | Generate pre-commit guard or Makefile target. |
| `config` | Mirror management + LLM-friendly CRUD (Spec 072). |
| `completion` | Shell completions (bash, zsh, fish, powershell). |

## LLM-Friendly Commands (Spec 072)

| Command | Purpose |
|---------|---------|
| `create` / `delete` / `rename` | Vendor CRUD without interactive TUI. |
| `show` | Show details for a single vendor. |
| `add-mapping` / `remove-mapping` / `list-mappings` / `update-mapping` | Path mapping CRUD. |
| `check` | Staleness check (synced/stale). |
| `preview` | Preview what a pull would do. |

## Deprecated Aliases

These print a deprecation warning to stderr. Will be removed after 2 minor versions.

| Old | New |
|-----|-----|
| `sync` | `pull --locked` |
| `update` | `pull` |
| `verify` | `status --offline` |
| `diff` | `status` |
| `outdated` | `status --remote-only` |

## Other Commands

| Command | Purpose |
|---------|---------|
| `sbom` | Generate CycloneDX or SPDX SBOM. |
| `license` | License compliance reporting. |
| `audit` | Audit vendored dependencies. |
| `scan` | Security/license scan. |
| `drift` | Drift detection reporting. |
| `annotate` | Annotate commits with git notes. |
| `migrate` | Migrate vendor.lock schema version. |
| `watch` | File-watch vendor.yml and auto-sync (experimental). |
