# Spec 070: Internal Project Compliance

> **Status:** Draft
> **Priority:** P1 - Agent/LLM Workflow Enabler
> **Effort:** 2-3 weeks
> **Dependencies:** 002 (Verify Hardening), 075 (Vendor Compliance Modes — provides enforcement model)
> **Blocks:** Agent handoff workflows, documentation compliance

---

## Problem Statement

Humans rarely update documentation directly. The burden increasingly falls on LLMs, which are prone to context rot and have no enforcement mechanism for keeping files in sync. This leads to:

1. **Documentation Staleness**: README sections duplicated in multiple files drift apart
2. **Schema Divergence**: API schemas defined in one place but referenced in docs/tests elsewhere
3. **Configuration Sprawl**: The same config structure defined in multiple locations
4. **No Compliance Signal**: Builds succeed even when documentation is outdated

git-vendor solves external dependency vendoring. **Internal compliance** extends this to enforce consistency between files *within the same project*.

---

## Use Cases

### 1. Documentation Sync
A CLI tool's command reference lives in:
- `docs/COMMANDS.md` (full documentation)
- `README.md` (quick reference section)
- `--help` output (generated from same source)

**Problem**: Three places to update, three chances to forget.

### 2. Schema Bodies
An API schema defined in:
- `api/schema.json` (canonical source)
- `docs/API_REFERENCE.md` (embedded in markdown)
- `tests/fixtures/valid_request.json` (test examples)

**Problem**: Schema changes but docs/tests use stale version.

### 3. Configuration Templates
A config structure in:
- `config.example.yaml` (user template)
- `docs/CONFIGURATION.md` (documented fields)
- `internal/config/defaults.go` (default values)

**Problem**: New config options added to code but not documented.

### 4. CLAUDE.md Sections
Agent context files that need sections kept in sync:
- `CLAUDE.md` (project root)
- `packages/*/CLAUDE.md` (package-specific, may duplicate core sections)

---

## Solution Overview

Extend git-vendor with an `internal` source type that tracks files within the same repository, with transforms and CI enforcement.

### Configuration

```yaml
# vendor/vendor.yml
vendors:
  # External vendor (existing behavior)
  - name: external-lib
    url: https://github.com/org/lib
    specs:
      - ref: main
        mapping:
          - from: src/
            to: vendor/lib/

  # Internal compliance (new behavior)
  - name: readme-commands
    source: internal
    specs:
      - mapping:
          - from: docs/COMMANDS.md
            to: README.md
            transform: extract-section
            section: "## Quick Reference"

  - name: api-schema
    source: internal
    compliance: strict  # fail on any drift (see Spec 075)
    specs:
      - mapping:
          - from: api/schema.json
            to: docs/API_SCHEMA.md
            transform: embed-json
            wrapper: "```json\n{content}\n```"
          - from: api/schema.json
            to: tests/fixtures/schema.json
            # No transform = exact copy
```

### New Fields

| Field | Type | Description |
|-------|------|-------------|
| `source` | `internal` | Marks this as an internal compliance vendor |
| `strictness` | `strict\|lenient` | `strict`: any drift fails, `lenient`: warn only |
| `transform` | string | Transform to apply (see Transforms section) |

### Compliance Levels (defined by Spec 075)

Internal vendors use the compliance model from [Spec 075: Vendor Compliance Modes](075-vendor-compliance-modes.md). The per-vendor `compliance` field and global `compliance.default` / `compliance.mode` settings apply uniformly to both external and internal vendors.

| Level | Behavior | Use Case |
|-------|----------|----------|
| `strict` | `verify` fails with exit 1 on any drift | Schema compliance |
| `lenient` | `verify` warns (exit 2) but doesn't fail | Documentation |
| `info` | `verify` reports drift, exit 0 | Experimental sync |

> **Note:** Earlier drafts of this spec used a `strictness` field. This has been unified as `compliance` across all vendor types per Spec 075. See 075 for enforcement hooks, CLI flags, global defaults, and override mode.

---

## Transforms

Transforms allow partial file sync and format conversion.

### Built-in Transforms

| Transform | Description | Example |
|-----------|-------------|---------|
| `exact` | Byte-for-byte copy (default) | Config files |
| `extract-section` | Extract markdown section by heading | README sections |
| `embed-json` | Embed JSON in a wrapper | Schema in docs |
| `embed-yaml` | Embed YAML in a wrapper | Config in docs |
| `strip-frontmatter` | Remove YAML frontmatter | Blog to README |
| `normalize-whitespace` | Ignore whitespace differences | Forgiving compare |

### Section Extraction

```yaml
transform: extract-section
section: "## Quick Reference"  # H2 heading to extract
include_heading: true          # Include the heading itself
depth: 2                       # Also include H3, H4 under it
```

### Embedding

```yaml
transform: embed-json
wrapper: |
  ## API Schema

  ```json
  {content}
  ```

  See [full documentation](./API.md) for details.
```

### Custom Transforms (Future)

```yaml
transform: custom
command: "jq '.components | keys' < {input}"
```

---

## Commands

### `git vendor sync` (extended)

For internal vendors:
- Reads source file
- Applies transform
- Writes to destination
- Updates lockfile with source hash

```bash
# Sync all (external + internal)
git vendor sync

# Sync only internal compliance
git vendor sync --internal

# Sync specific internal vendor
git vendor sync readme-commands
```

### `git vendor verify` (extended)

For internal vendors:
- Computes source hash
- Applies transform to temp file
- Compares with destination
- Reports drift

```bash
git vendor verify
# Output:
# Verifying vendored dependencies...
#
# External:
#   ✓ external-lib (v1.2.3)
#
# Internal:
#   ✓ readme-commands (synced)
#   ✗ api-schema (DRIFT DETECTED)
#     Source: api/schema.json (hash: abc123)
#     Dest:   docs/API_SCHEMA.md (hash: def456)
#     Run 'git vendor sync api-schema' to update
#
# Result: FAIL (1 internal drift)
```

### `git vendor diff` (extended)

For internal vendors:
- Shows actual content diff between source (transformed) and destination

```bash
git vendor diff api-schema
# --- api/schema.json (transformed)
# +++ docs/API_SCHEMA.md
# @@ -5,7 +5,7 @@
#    "type": "object",
#    "properties": {
# -    "name": {"type": "string"},
# +    "name": {"type": "string", "maxLength": 100},
#    }
```

---

## Lockfile Changes

```yaml
schema_version: "1.2"
vendors:
  # External vendor
  - name: external-lib
    ref: main
    commit_hash: abc123...
    # ... existing fields

  # Internal vendor (new)
  - name: api-schema
    source: internal
    source_hash: def456...      # Hash of source file(s)
    dest_hashes:                 # Hash of each destination
      docs/API_SCHEMA.md: ghi789...
      tests/fixtures/schema.json: jkl012...
    last_synced_at: "2026-02-07T..."
```

---

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/compliance.yml
name: Compliance Check

on: [push, pull_request]

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install git-vendor
        run: go install github.com/EmundoT/git-vendor@latest
      - name: Verify compliance
        run: git vendor verify --strict
        # Exit 1 = external drift or strict internal drift
        # Exit 2 = lenient internal drift (warning only)
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Check for internal drift
if ! git vendor verify --internal --quiet; then
    echo "ERROR: Internal compliance drift detected"
    echo "Run 'git vendor sync --internal' to update derived files"
    exit 1
fi
```

---

## Agent Workflow Integration

### The Core Value for LLMs

When a Claude instance modifies a canonical source file, the pre-commit hook or CI catches if derived files weren't updated. This creates a forcing function:

1. Agent edits `api/schema.json`
2. Commits
3. Pre-commit hook runs `git vendor verify`
4. **FAIL**: `docs/API_SCHEMA.md` is stale
5. Agent runs `git vendor sync api-schema`
6. Commits again
7. **PASS**: All files in sync

### Auto-Sync Mode (Optional)

For trusted workflows, auto-sync on source change:

```yaml
vendors:
  - name: api-schema
    source: internal
    auto_sync: true  # Auto-run sync when source changes
```

With a file watcher:
```bash
git vendor watch --internal
# Watches source files, auto-syncs when they change
```

---

## Holes and Mitigations

### Hole 1: Circular Dependencies

**Problem**: File A sources from B, B sources from A.

**Mitigation**: Detect cycles during config validation.

```go
func validateInternalDependencies(config *VendorConfig) error {
    // Build dependency graph
    // Detect cycles using DFS
    // Error with clear message: "Circular dependency: A → B → A"
}
```

### Hole 2: Merge Conflicts

**Problem**: Both source and dest modified in parallel branches.

**Mitigation**:
- Source always wins (it's canonical)
- `git vendor sync` after merge resolves
- Document this clearly in README

### Hole 3: Transform Determinism

**Problem**: Transforms must be deterministic or verify will always fail.

**Mitigation**:
- Built-in transforms are deterministic by design
- Custom transforms require `--force` to sync (user acknowledges non-determinism)
- Test transforms in CI with golden files

### Hole 4: Large Files

**Problem**: Syncing large binary files is wasteful.

**Mitigation**:
- Warn if source file > 1MB
- Add `max_size: 1MB` config option
- For large files, suggest git LFS instead

### Hole 5: Partial Section Extraction Edge Cases

**Problem**: Section extraction depends on markdown structure; malformed markdown fails.

**Mitigation**:
- Use robust markdown parser (goldmark)
- Fail clearly: "Section '## Foo' not found in source.md"
- Add `--lenient-parse` for best-effort extraction

---

## Implementation Plan

### Phase 1: Core Infrastructure
1. Add `source: internal` vendor type detection
2. Add internal source hash computation
3. Extend verify to check internal vendors
4. Extend lockfile schema to 1.2

### Phase 2: Transforms
1. Implement `exact` transform (default)
2. Implement `extract-section` transform
3. Implement `embed-json`/`embed-yaml` transforms
4. Add transform validation

### Phase 3: Sync & Commands
1. Extend `sync` command for internal vendors
2. Extend `diff` command for internal vendors
3. Add `--internal` flag to relevant commands
4. Add cycle detection to validation

### Phase 4: Integration
1. Add pre-commit hook generator
2. Document CI/CD integration
3. Add auto-sync mode with watch
4. Write migration guide

---

## Example: Full Configuration

```yaml
# vendor/vendor.yml
vendors:
  # External dependency
  - name: charmbracelet-huh
    url: https://github.com/charmbracelet/huh
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: huh.go
            to: vendor/huh/huh.go

  # Internal: README quick reference from full docs
  - name: readme-commands
    source: internal
    compliance: lenient
    specs:
      - mapping:
          - from: docs/COMMANDS.md
            to: README.md
            transform: extract-section
            section: "## Command Reference"
            include_heading: true

  # Internal: API schema in docs
  - name: api-schema-docs
    source: internal
    strictness: strict
    specs:
      - mapping:
          - from: api/openapi.yaml
            to: docs/API.md
            transform: embed-yaml
            wrapper: |
              # API Reference

              ```yaml
              {content}
              ```

  # Internal: Config docs from example file
  - name: config-docs
    source: internal
    compliance: lenient
    specs:
      - mapping:
          - from: config.example.yaml
            to: docs/CONFIGURATION.md
            transform: embed-yaml
            wrapper: |
              # Configuration Reference

              Below is the full configuration with all options:

              ```yaml
              {content}
              ```

              See comments in the file for detailed explanations.
```

---

## Success Criteria

- [ ] `source: internal` vendors load and validate
- [ ] Internal vendors have hash-based drift detection
- [ ] `git vendor verify` reports internal drift with clear output
- [ ] `git vendor sync` updates derived files from sources
- [ ] `git vendor diff` shows content diff for internal vendors
- [ ] `extract-section` transform works for markdown
- [ ] `embed-json`/`embed-yaml` transforms work
- [ ] Cycle detection prevents circular dependencies
- [ ] CI integration documented with examples
- [ ] Pre-commit hook generator works
- [ ] Lockfile schema updated to 1.2
- [ ] All existing tests pass
- [ ] New tests for internal compliance features

---

## References

- ROADMAP.md Section 14 (Extensibility)
- Spec 002: Verify Command Hardening
- Spec 075: Vendor Compliance Modes (enforcement model, global defaults, override mode)
- User feedback on documentation maintenance burden
