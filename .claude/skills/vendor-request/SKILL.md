---
name: vendor-request
description: Open an upstream issue or PR when you need to change a vendored file — routes change requests to the source repo instead of editing locally
---

# Vendor Request — Upstream Change Proposals

Use this skill when you need to modify a vendored file but cannot edit it locally (vendor-guard blocks it). Instead of bypassing the guard, this skill opens an issue or PR on the upstream source repo.

## When to Use

- You discovered a bug in a vendored file (hook, rule, skill, library)
- You need a feature or behavior change in vendored content
- A vendored file's behavior doesn't work in the downstream project's context
- You want to propose a pattern change that should propagate to all ecosystem projects

## Workflow

### Step 1: Identify the Vendor Source

Read `.git-vendor/vendor.lock` and find the vendor entry that owns the file:

```bash
# Find which vendor owns a file
grep -B 20 "path/to/file" .git-vendor/vendor.lock | grep "name:"
```

Then read `.git-vendor/vendor.yml` to get the source URL:

```yaml
# vendor.yml entry tells you the upstream repo
- name: git-ecosystem
  url: https://github.com/EmundoT/git-ecosystem.git
```

### Step 2: Choose Action

**Option A: Open an issue** (default — request a change, let upstream decide)

```bash
gh issue create --repo <owner>/<repo> \
  --title "<concise title>" \
  --body "$(cat <<'EOF'
## Context

- **Downstream project**: <project-name>
- **Vendored file**: <path in downstream>
- **Source file**: <path in upstream> (from vendor.yml mapping)

## Problem

<What's wrong or what needs to change. Be specific.>

## Proposed Fix

<How it should be fixed upstream. Include code if possible.>

## Workaround Applied

<Any temporary local fix applied to non-vendored files (e.g., settings.json).>
EOF
)"
```

**Option B: Open a PR** (you have a concrete fix ready)

1. Clone or navigate to the upstream repo
2. Create a branch, make the fix
3. Open a PR referencing the downstream context

### Step 3: Document Locally

After opening the issue/PR:

1. If a local workaround was applied, add a code comment referencing the upstream issue:
   ```bash
   # WORKAROUND: upstream issue <owner>/<repo>#<number>
   # Remove after next git-vendor sync once fix is merged
   ```

2. If the change is blocking, use `/nominate` to flag it for the project owner.

## Rules

1. MUST NOT edit vendored files locally. The vendor-guard hook enforces this.
2. MUST identify the correct upstream repo from `vendor.yml` before opening issues.
3. MUST include downstream project context in the issue body — upstream maintainers need to understand the impact.
4. SHOULD include a proposed fix, not just a problem description.
5. SHOULD reference the upstream issue in any local workaround comments for traceability.
6. MAY apply workarounds to non-vendored files (e.g., `settings.json`, project-local scripts) while waiting for the upstream fix.

## File Discovery Reference

```text
vendor.yml    → mapping.from = source path in upstream repo
              → mapping.to   = destination path in downstream repo
vendor.lock   → file_hashes  = SHA-256 of each vendored file
              → name         = vendor name (for git-vendor sync <name>)
              → commit_hash  = exact upstream commit
```
