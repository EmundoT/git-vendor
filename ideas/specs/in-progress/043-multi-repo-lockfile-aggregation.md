# Spec 043: Multi-Repo Lockfile Aggregation

> **Status:** Draft
> **Priority:** P2 - Extensibility (Tier 1), Enterprise (Tiers 2-3)
> **Effort:** Tier 1: 1 week, Tiers 2-3: TBD (enterprise roadmap)
> **Dependencies:** 002 (Verify Hardening), 075 (Vendor Compliance Modes)
> **Blocks:** None (leaf feature)

---

## Problem Statement

Organizations with multiple projects vendoring from the same upstream repositories have no mechanism to detect or enforce consistency across projects. When `security-lib@v2.1.0` is vendored by five projects, a vulnerability fix in v2.2.0 may be adopted by three and missed by two — with no visibility.

**Key framing:** This is NOT a peer-to-peer problem. Project A has no business knowing about Project B. This is an **organizational concern** — a registry owned by the org that tracks which projects vendor what, applies policy top-down, and flags drift.

The commit carrying a vendor change is the natural place to attach compliance metadata. Git commits are immutable, auditable, and already the unit of change tracking. Building on commits (not config files or external databases) keeps the system grounded in git's existing infrastructure.

---

## Non-Goals

- **Runtime enforcement**: Build/commit gates are 075's domain. 043 provides the cross-project visibility layer.
- **Automatic remediation**: 043 detects drift, it does not auto-update projects.
- **Replacing package managers**: This is vendoring compliance, not dependency resolution.

---

## Architecture: The Org Model

```text
                    ┌──────────────────────┐
                    │    Org Registry       │
                    │  (shared git repo)    │
                    │                       │
                    │  - vendor manifest    │
                    │  - default policy     │
                    │  - project list       │
                    └──────────┬───────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                 │
         ┌────▼────┐     ┌────▼────┐      ┌────▼────┐
         │Project A │     │Project B │      │Project C │
         │          │     │          │      │          │
         │ vendors: │     │ vendors: │      │ vendors: │
         │  lib-x   │     │  lib-x   │      │  lib-y   │
         │  lib-y   │     │  lib-z   │      │  lib-z   │
         └──────────┘     └──────────┘      └──────────┘

Projects don't know about each other.
The registry knows about all of them.
Policy flows downward.
```

**Vendor change lifecycle:**

1. Upstream `lib-x` releases v2.2.0
2. Project A updates, commits with vendor trailers (see Commit Syntax below)
3. Project A's CI publishes lockfile to registry (Tier 2-3, enterprise)
4. Registry detects Projects B also vendors `lib-x` but is still on v2.1.0
5. Registry flags drift per org policy (strict → block B's CI, lenient → notify, AI auto → open PR on B)

**Tier 1 doesn't need the registry.** Tier 1 is the `compare` command that answers: "given two lockfiles, what's shared and what's diverged?"

---

## Tier 1: `git-vendor compare` (Full Spec)

### Command

```bash
# Compare against another lockfile (local path)
git-vendor compare ./path/to/other/vendor.lock

# Compare against a remote lockfile (fetched via URL)
git-vendor compare https://raw.githubusercontent.com/org/project-b/main/.git-vendor/vendor.lock

# Compare against a GitHub repo's lockfile (shorthand)
git-vendor compare gh:org/project-b

# Output formats
git-vendor compare --format table ./other.lock    # default
git-vendor compare --format json ./other.lock     # machine-readable
```

### Comparison Logic

For each vendor present in BOTH lockfiles (matched by normalized URL):

| Field | Comparison | Result |
|-------|-----------|--------|
| `commit_hash` | Exact match | `match` / `diverged` |
| `ref` | Exact match | `same-ref` / `different-ref` |
| File hashes (intersection) | Per shared file | `identical` / `modified` |
| Compliance level | Informational | Reported, not compared |

**Matching vendors:** Two lockfile entries refer to the same vendor when their repository URLs normalize to the same value (strip `.git` suffix, normalize case, resolve `http` → `https`).

### Output

#### Table (default)

```text
Comparing lockfiles...
  Local:  .git-vendor/vendor.lock
  Remote: ../project-b/.git-vendor/vendor.lock

Shared vendors: 2 of 5

  Vendor          Local Ref    Remote Ref   Commit Status   Files
  security-lib    v2.2.0       v2.1.0       diverged        3 shared (1 modified)
  util-helpers    main         main         match           7 shared (0 modified)

Local only: auth-sdk, metrics-lib
Remote only: frontend-kit

Summary: 1 diverged, 1 matched
```

#### JSON

```json
{
  "schema_version": "1.0",
  "local_lockfile": ".git-vendor/vendor.lock",
  "remote_lockfile": "../project-b/.git-vendor/vendor.lock",
  "shared": [
    {
      "url": "https://github.com/org/security-lib",
      "local_name": "security-lib",
      "remote_name": "security-lib",
      "local_ref": "v2.2.0",
      "remote_ref": "v2.1.0",
      "local_commit": "abc123...",
      "remote_commit": "def456...",
      "commit_status": "diverged",
      "shared_files": 3,
      "modified_files": 1,
      "file_details": [
        {
          "path": "src/auth.go",
          "local_hash": "sha256:...",
          "remote_hash": "sha256:...",
          "status": "modified"
        }
      ]
    }
  ],
  "local_only": ["auth-sdk", "metrics-lib"],
  "remote_only": ["frontend-kit"]
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All shared vendors match |
| 1 | One or more shared vendors diverged |
| 2 | No shared vendors found (informational) |

### Flags

| Flag | Behavior |
|------|----------|
| `--format table\|json` | Output format (default: table) |
| `--vendor <name>` | Compare only a specific vendor |
| `--commits-only` | Only compare commit hashes, skip file-level comparison |
| `--quiet` | Exit code only, no output |

---

## Commit Syntax Specification

> **STABILITY: This section defines the wire format for vendor change metadata in git commits. Changes to this format are breaking. Treat this as a versioned protocol.**

### Design Principles

1. **Git-native**: Use git trailers (RFC 822-style key-value pairs at the end of commit messages). Parsed by `git interpret-trailers`, queryable via `git log --format='%(trailers:key=X)'`.
2. **Forward-compatible**: Unknown trailers are ignored. New trailers can be added without breaking existing parsers. Trailer values are always strings.
3. **Machine-first, human-readable**: Structured enough for automation, clean enough to read in `git log`.
4. **Shared vocabulary**: The same tag syntax (`#word`) works in code comments, commit messages, and vendor config. Trailers are the structured overlay for commits.

### Trailer Schema (v1)

```text
feat: update security-lib to v2.2.0

Updated logging format for consistency with org standards.
Minor text fix, no behavioral change.

Vendor-Name: security-lib
Vendor-Ref: v2.2.0
Vendor-Commit: abc123def456...
Vendor-Compliance: lenient
Vendor-Syntax: v1
Tags: vendor.update, logging, minor
```

#### Required Trailers (for vendor change commits)

| Trailer | Type | Description | Stable? |
|---------|------|-------------|---------|
| `Vendor-Name` | string | Vendor name as defined in vendor.yml | Yes |
| `Vendor-Ref` | string | Git ref being synced to | Yes |
| `Vendor-Commit` | string | Full commit SHA resolved | Yes |
| `Vendor-Syntax` | `v1` | Trailer schema version. MUST be present. Parsers MUST check this before processing. | Yes |

#### Optional Trailers

| Trailer | Type | Default | Description | Stable? |
|---------|------|---------|-------------|---------|
| `Vendor-Compliance` | `strict\|lenient\|info` | (inherits from config) | Override compliance level for THIS change | Yes |
| `Vendor-Reason` | string | (none) | Human/LLM explanation of compliance override | Yes |
| `Tags` | comma-separated | (none) | Dot-hierarchical tags (see Tag System below) | Yes |
| `Vendor-Previous-Commit` | string | (none) | Previous commit SHA (for diff tracking) | Experimental |
| `Vendor-Files-Changed` | integer | (none) | Number of vendored files changed | Experimental |

#### Multi-Vendor Commits

When a single commit updates multiple vendors, repeat the trailer block with numeric suffix:

```text
feat: update security-lib and util-helpers

Vendor-Name: security-lib
Vendor-Ref: v2.2.0
Vendor-Commit: abc123...
Vendor-Compliance: lenient
Vendor-Syntax: v1

Vendor-Name: util-helpers
Vendor-Ref: main
Vendor-Commit: def456...
Vendor-Syntax: v1
```

**Parsing rule:** Group trailers by sequential `Vendor-Name` occurrences. Each `Vendor-Name` starts a new vendor block. All subsequent `Vendor-*` trailers belong to that block until the next `Vendor-Name`.

#### Versioning Contract

- `Vendor-Syntax: v1` — current and only version
- Parsers MUST check `Vendor-Syntax` before processing
- New OPTIONAL trailers can be added without version bump
- Changing the meaning of an existing trailer or adding new REQUIRED trailers bumps the version
- Parsers SHOULD ignore unknown trailers (forward-compat)
- Parsers MUST reject commits where `Vendor-Syntax` is higher than their supported version (fail-safe)

### Why Trailers, Not Comments or Tags

| Alternative | Rejection reason |
|---|---|
| Git notes | Mutable, not part of commit SHA, easily lost on clone |
| Commit body parsing | Fragile, no standard format, conflicts with prose |
| Separate metadata file | Extra file per commit, not atomic with the change |
| Git tags | Per-commit overhead, pollutes tag namespace |
| Custom headers | Not supported by git |

Git trailers are the only metadata mechanism that is:
- Part of the commit object (immutable, included in SHA)
- Parseable by standard git tooling (`git interpret-trailers`, `git log --format`)
- Extensible without breaking existing parsers
- Already in widespread use (`Signed-off-by`, `Co-Authored-By`, `Reviewed-by`)

---

## Tag System Integration

> **This section defines the shared tag vocabulary used across git-vendor, git-agent, and git-plumbing. The tag syntax is infrastructure — it is not owned by any single tool.**

### Syntax: `#tag` in Code, `Tags:` Trailer in Commits

#### In code comments (all languages)

```go
// ProcessPayment handles payment processing for customer orders.
// #payments #critical #pci.compliance
func ProcessPayment(order *Order) error {
```

```python
# normalize_query normalizes user input for search indexing.
# #search #normalization #i18n
def normalize_query(raw: str) -> str:
```

#### Rules

| Rule | Spec |
|------|------|
| Format | `#` followed by `[a-zA-Z][a-zA-Z0-9._-]*` |
| Space tolerance | Both `#tag` and `# tag` are valid (Elixir convention) |
| Hierarchy | Dot-separated: `#security.auth.oauth` |
| Location | In comment lines only (not strings, not code) |
| Scope | Tied to the next declaration (function, type, const, var, struct, class) |
| File-level | Tags in a comment block before any declaration = file-level tags |
| Aggregation | File tags = union of all declaration tags + file-level tags |
| Folder tags | Union of all file tags in the directory (computed, not stored) |

#### In git commit messages

```text
Tags: vendor.update, security, minor
```

The `Tags:` trailer carries the same vocabulary as code tags, in comma-separated form. Tags in trailers omit the `#` prefix (it's implied by the `Tags:` key).

Tags MAY also appear inline in the commit body as `#tag` for readability:

```text
fix: patch XSS vulnerability in input sanitizer

Addresses #security.xss finding from audit.
Only affects HTML rendering path.

Tags: security.xss, bugfix, critical
Vendor-Syntax: v1
```

**Parsing priority:** The `Tags:` trailer is the machine-readable source of truth. Inline `#tags` in the commit body are supplementary (for human readability and git log searches via `--grep`).

### Tag Hierarchy

```text
#vendor                      # Top-level domain
#vendor.update               # Vendor was updated
#vendor.update.minor         # Minor update (no breaking changes)
#vendor.update.major         # Major update (breaking changes possible)
#vendor.compliance           # Compliance-related change
#vendor.compliance.override  # Compliance level was overridden

#security                    # Security domain
#security.auth               # Authentication subsystem
#security.auth.oauth         # OAuth specifically

#infra                       # Infrastructure
#infra.ci                    # CI/CD changes
#infra.deploy                # Deployment changes
```

**Querying:** `#security` matches `#security`, `#security.auth`, `#security.auth.oauth`. Parent tags are prefixes. Tools query with prefix matching.

### Reserved Tag Prefixes

| Prefix | Owner | Purpose |
|--------|-------|---------|
| `#vendor.*` | git-vendor | Vendor change classification |
| `#agent.*` | git-agent | Agent workflow metadata |
| `#review.*` | git-agent | Code review tags |
| `#plumbing.*` | git-plumbing | Git operation metadata |

All other tags are user-defined. No enforcement on user tags — they're emergent.

---

## Future: Enterprise Tiers (Design Intent)

> **These tiers are NOT in scope for implementation. They document the design direction so Tier 1 infrastructure (commit syntax, compare command) is built with forward compatibility.**

### Tier 2: Org Registry

A shared git repository containing:

```yaml
# .git-vendor-registry.yml
registry:
  version: 1
  org: my-org

policy:
  default: lenient                    # org-wide default compliance
  mode: default                       # default | override
  update_window: 30d                  # max allowed drift age
  notification: slack                 # how to notify on drift
  ai_auto:                            # AI-assisted resolution
    enabled: false
    strategy: open-pr                 # open-pr | auto-merge (with approval)

# Projects are auto-discovered via CI publish, or manually listed
projects:
  - name: project-a
    repo: org/project-a
    lockfile_ref: main                # branch to read lockfile from
  - name: project-b
    repo: org/project-b
    lockfile_ref: main

# Per-vendor policy overrides (org-level)
vendor_policies:
  - url: https://github.com/org/security-lib
    compliance: strict                # this vendor is always strict, org-wide
    max_drift: 7d                     # tighter window for security deps
  - url: https://github.com/org/docs-helpers
    compliance: info                  # nobody cares if docs-helpers drifts
```

**CI integration:** Each project's CI publishes its lockfile to the registry on merge to main. A central job runs `git-vendor compare --registry` across all projects and reports org-wide drift.

### Tier 3: Policy Enforcement + AI Auto-Resolution

- **Strict org vendors**: Block project CI if project is behind on a strict vendor
- **AI auto**: When drift is detected, git-agent opens a PR on the lagging project with the update
- **Human case-by-case**: Flag in a dashboard, require manual approval to proceed
- **Override at project level**: A project can flag a specific vendor change as `lenient` via the `Vendor-Compliance` trailer, even if the org says `strict`. The registry records the override with `Vendor-Reason` for audit.

### Why This Works

The commit trailers carry all the metadata. The registry just aggregates and compares. Policy is config, not code. The `Vendor-Compliance` override on the commit is the escape valve — a project can always say "this specific change is minor, treat it as lenient" and the org can audit that decision via the trailer.

No magic. No hidden state. Everything is in git.

---

## Implementation Plan (Tier 1 Only)

### Phase 1: Lockfile Parsing + Comparison (3 days)
1. Add `compare` command to CLI dispatcher (main.go)
2. Implement lockfile loading from local path
3. Implement URL normalization for vendor matching
4. Implement field-by-field comparison logic
5. Table and JSON output formatters

### Phase 2: Remote Lockfile Fetching (2 days)
1. HTTP(S) URL fetching with timeout
2. `gh:org/repo` shorthand resolution (GitHub API)
3. Auth token passthrough (GITHUB_TOKEN, GITLAB_TOKEN)

### Phase 3: Commit Trailer Support (2 days)
1. `git-vendor sync` and `git-vendor update` generate trailer-formatted output
2. `--trailer` flag: output vendor trailers for pasting into commit message
3. Document trailer schema in help text and docs

### Phase 4: Tests + Docs (1 day)
1. Unit tests for comparison logic
2. Unit tests for URL normalization edge cases
3. Documentation for `compare` command
4. Document commit trailer schema

---

## Success Criteria

### Tier 1

- [ ] `git-vendor compare <path>` compares two lockfiles and reports shared vendor status
- [ ] `git-vendor compare <url>` fetches remote lockfile via HTTP(S)
- [ ] `git-vendor compare gh:org/repo` resolves GitHub lockfile
- [ ] URL normalization correctly matches equivalent repo URLs
- [ ] Table output clearly shows shared/diverged/local-only/remote-only vendors
- [ ] JSON output includes all comparison fields
- [ ] Exit codes: 0 (match), 1 (diverged), 2 (no shared vendors)
- [ ] `--vendor` flag filters to specific vendor
- [ ] `--commits-only` flag skips file-level comparison
- [ ] Commit trailer schema documented with versioning contract
- [ ] `git-vendor sync --trailer` outputs trailer block for commit message
- [ ] All existing tests pass
- [ ] New tests for comparison logic and URL normalization

### Commit Syntax (Tier 1 infrastructure, used by all tiers)

- [ ] `Vendor-Syntax: v1` trailer defined and documented
- [ ] Multi-vendor commit parsing logic defined
- [ ] Versioning contract documented (breaking vs non-breaking changes)
- [ ] Reserved tag prefixes documented

---

## References

- Spec 002: Verify Command Hardening (exit code alignment)
- Spec 075: Vendor Compliance Modes (compliance levels, enforcement model)
- Spec 070: Internal Project Compliance (internal vendor compliance)
- Queue item 022: GitHub Action (CI integration for registry publish)
- Queue item 023: Compliance Evidence Reports (consumes cross-project drift data)
