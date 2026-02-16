---
name: commit-schema
description: COMMIT-SCHEMA v1 protocol reference for writing structured commits with trailers, tags, and notes in the git-plumbing ecosystem
user-invocable: false
---

# Commit Schema v1 — Protocol Reference

Use this skill when writing git commits in the git-plumbing ecosystem. This skill provides the complete rules for structuring commit messages with the correct trailers, tags, and format.

For the full protocol spec: `COMMIT-SCHEMA.md`
For query examples: `COMMIT-COOKBOOK.md`

---

## Subject Line Format

```
<type>(<scope>)[!]: <description>
```

- **type**: Lowercase. One of: feat fix refactor test docs chore perf style ci
- **scope**: Lowercase `[a-z][a-z0-9-]*`. Module or component name.
- **!**: After `)`, before `:`. Marks a breaking change.
- **description**: Imperative mood. No period. No initial capital.
- **Max 72 characters total.**

Subject line parsing is defined in COMMIT-SCHEMA.md (Section: Subject Line). In git-plumbing, `ParseSubject` in `subject.go` implements this.

## Commit-Schema Trailer (REQUIRED)

Every ecosystem commit MUST have a Commit-Schema trailer:

```
Commit-Schema: <namespace>/v1
```

| Namespace | When |
|-----------|------|
| `agent/v1` | LLM/agent-authored commits (git-agent sets this) |
| `vendor/v1` | Dependency vendor/update commits (git-vendor sets this) |
| `manual/v1` | Human commits (hooks auto-add when no schema present) |

If Commit-Schema is already present, do NOT add another one. Hook enrichment enforces this idempotency (see COMMIT-SCHEMA.md, Section: Enrichment Flow).

## Tags Trailer (author-declared intent)

```
Tags: auth.mfa, security, user-management
```

- REQUIRED for agent/v1 commits
- OPTIONAL for vendor/v1 and manual/v1
- Comma-separated, lowercase only
- Hierarchical with dots: `security.auth.oauth`
- Hyphens for word joining: `session-mgmt`
- Must start with letter, max 128 characters per tag
- Describes WHAT the commit is about (semantic intent)

## Touch Trailer (auto-extracted evidence)

```
Touch: auth, auth.session, security
```

- Auto-extracted from `#tag` comments in staged source files
- Same tag format as Tags trailer
- Describes WHAT code areas were actually modified
- Hooks compute this automatically at commit time
- DO NOT manually set Touch — let the hooks handle it

## Tags vs Touch

Tags = what you SAID you were doing. Touch = what you ACTUALLY modified. Divergence is a review signal:

| Tags | Touch | Signal |
|------|-------|--------|
| docs | auth, security | Changed security code — really just docs? |
| auth.mfa | auth | Focused work, intent matches |
| (empty) | security | Touched security code, no semantic tag — review |

## Diff Metrics (auto-computed)

These are computed by hooks automatically:

```
Diff-Additions: 142
Diff-Deletions: 17
Diff-Files: 3
Diff-Surface: api
```

DO NOT manually set these — hooks compute them from staged diff state.

## Agent-Specific Trailers (agent/v1 only)

When writing commits as an agent (Commit-Schema: agent/v1):

| Trailer | Required | Source |
|---------|----------|--------|
| Agent-Id | REQUIRED | `<provider>/<identifier>`, e.g., `claude-code/kyle-desktop` |
| Model | REQUIRED | LLM model identifier, e.g., `claude-opus-4-6` |
| Intent | REQUIRED | WHY this commit was made (not a restatement of subject) |
| Tags | REQUIRED | Semantic domain tags |
| Confidence | OPTIONAL | `low`, `medium`, or `high` |
| Refs | OPTIONAL | Issue/ticket references |

Intent MUST explain the reasoning or trigger, NOT restate the subject line.

- Good: `implement 2FA per security roadmap SEC-042`
- Bad: `add TOTP authentication` (restates subject)

## Vendor-Specific Trailers (vendor/v1 only)

| Trailer | Required | Format |
|---------|----------|--------|
| Vendor-Name | REQUIRED | Dependency name |
| Vendor-Ref | REQUIRED | Version/tag/branch |
| Vendor-Commit | REQUIRED | Full SHA (40 chars) |
| Vendor-License | OPTIONAL | SPDX identifier |

Multi-vendor commits use duplicate keys with positional association. The Nth `Vendor-Name` corresponds to the Nth `Vendor-Ref`/`Vendor-Commit`.

## Code Tags (for building Touch coverage)

When writing or modifying source code, add `#tag` comments to functions and types:

```go
// ProcessPayment handles payment processing.
// #payments #critical #pci.compliance
func ProcessPayment(order *Order) error {
```

Rules:

- `#` immediately adjacent to tag name (no space)
- Same grammar as trailer tags: lowercase, dots for hierarchy, hyphens for words
- Tags preceding a declaration scope to that declaration
- Tags before package/first declaration scope to the file
- Hooks extract these into Touch trailers at commit time

## Enrichment Flow

1. Human/agent writes subject + body + author trailers (Tags, Intent, etc.)
2. `prepare-commit-msg` hook adds:
   - Commit-Schema: manual/v1 (if no schema present)
   - Touch: (extracted from staged files' #tag comments)
   - Diff-Additions, Diff-Deletions, Diff-Files (from staged diff)
   - Diff-Surface (classified from file paths)
3. `commit-msg` hook validates:
   - Subject line length (warns if > 72 chars)
   - Tag syntax (warns on invalid tags)
   - Normalizes Tags/Touch values to lowercase

See COMMIT-SCHEMA.md for the full enrichment specification and hook implementation details.

## Tag Prefix Matching

Querying a tag matches all descendants. Segments are atomic:

```
Commit tags: security.auth.oauth, payments.tax

Query "security"       -> MATCH  (prefix)
Query "security.auth"  -> MATCH  (prefix)
Query "sec"            -> NO MATCH (partial segment)
```

Prefix matching is segment-wise, not substring. See COMMIT-SCHEMA.md for the complete tag grammar and matching rules.

## Trailer API Reference

`Commit.Trailers` is `[]Trailer` (ordered, supports duplicate keys).

| Method | Returns | Description |
|--------|---------|-------------|
| `TrailerValue(key)` | `string` | First value for key, or "" |
| `TrailerValues(key)` | `[]string` | All values for key, in order |
| `HasTrailer(key)` | `bool` | Whether any trailer with key exists |

Multi-vendor commits use positional association: the Nth `Vendor-Name` corresponds to the Nth `Vendor-Ref`/`Vendor-Commit`.

### Multi-vendor template

```
chore(vendor): update <lib-a> and <lib-b>

Commit-Schema: vendor/v1
Vendor-Name: <lib-a>
Vendor-Ref: <version-a>
Vendor-Commit: <sha-a>
Vendor-Name: <lib-b>
Vendor-Ref: <version-b>
Vendor-Commit: <sha-b>
Tags: vendor.update
```

## Quick Reference: Commit Templates

### Agent commit

```
<type>(<scope>): <description>

<body explaining why>

Commit-Schema: agent/v1
Agent-Id: <provider>/<identifier>
Model: <model-id>
Intent: <why this commit exists>
Tags: <domain1>, <domain2>
Confidence: <low|medium|high>
```

### Human commit (hooks auto-enrich)

```
<type>(<scope>): <description>
```

Hooks add: Commit-Schema, Touch, Diff-Additions, Diff-Deletions, Diff-Files, Diff-Surface.

## Pinned Decisions (MUST NOT change without major version)

1. Tag grammar: `segment ('.' segment)*` where `segment := [a-z][a-z0-9-]*`
2. Code tag sigil: `#` immediately adjacent
3. Extraction regex: `(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)`
4. Trailer format: standard git `Key: Value`
5. Prefix matching: segment-wise, not substring
6. Schema discriminator: `Commit-Schema: <namespace>/v<N>`
