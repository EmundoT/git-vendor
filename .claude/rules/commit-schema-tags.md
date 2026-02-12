# Commit Schema & Tag System Rules

These rules apply to all git-ecosystem projects (git-plumbing, git-agent, git-vendor). Follow them when writing code, writing commits, or reviewing changes.

Full protocol spec: `../COMMIT-SCHEMA.md`
Query cookbook: `../COMMIT-COOKBOOK.md`

---

## 1. Tag Grammar

Tags use lowercase segments joined by dots. Hyphens join words within a segment.

```text
tag     := segment ('.' segment)*
segment := [a-z][a-z0-9-]*
```

Valid: `auth`, `security.auth.oauth`, `session-mgmt`, `pci.compliance`
Invalid: `Auth` (uppercase), `#123` (starts with digit), `sec urity` (space), `.auth` (leading dot)

Max length: 128 characters.

## 2. Canonical Extraction Regex (PINNED)

```text
(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)
```

This regex is PINNED. It MUST NOT change without a major version bump to the commit schema. All tools in the ecosystem use this exact pattern for extracting `#tag` from source files.

Known v1 acceptance: trailing dots are consumed; hex color codes starting with `a-f` false-positive (e.g., `#abcdef`). These are acceptable tradeoffs.

## 3. Prefix Matching

Matching is segment-wise, NOT substring. Segments are split on `.` and compared with exact string equality left-to-right.

```text
Query: "security"        Candidate: "security.auth.oauth"  -> MATCH (1 segment prefix)
Query: "security.auth"   Candidate: "security.auth.oauth"  -> MATCH (2 segment prefix)
Query: "sec"             Candidate: "security.auth.oauth"  -> NO MATCH (partial segment)
Query: "security.mfa"    Candidate: "security.auth.oauth"  -> NO MATCH (second segment differs)
```

## 4. Writing Code Tags

When writing or modifying code, add `#tag` comments to functions, types, and declarations to build Touch coverage.

```go
// ProcessPayment handles payment processing.
// #payments #critical #pci.compliance
func ProcessPayment(order *Order) error {
```

Rules:
- `#` MUST be immediately adjacent to the tag name (no space between `#` and tag)
- Place tags in the doc comment block preceding the declaration
- Tags before the package declaration or first declaration have file scope
- Use hierarchical tags for specificity: `#auth.mfa` not just `#auth`
- Add tags as you write code. This is the primary mechanism for building Touch coverage over time.

## 5. Commit Subject Format

```text
<type>(<scope>)[!]: <description>
```

- **type**: Lowercase. One of: feat fix refactor test docs chore perf style ci
- **scope**: Lowercase `[a-z][a-z0-9-]*`. The module or component.
- **!**: After `)`, before `:`. Marks a breaking change.
- **description**: Imperative mood. No period. No initial capital.
- **Total length**: MUST NOT exceed 72 characters.

The subject is the source of truth for type and scope. Do NOT write `Commit-Type` or `Commit-Scope` trailers — they do not exist in COMMIT-SCHEMA v1.

## 6. Trailer Protocol

Every ecosystem commit carries `Commit-Schema: <namespace>/v1`. Three namespaces:

### Shared trailers (any namespace)

| Trailer | Auto | Format | Notes |
|---------|------|--------|-------|
| Commit-Schema | YES (hooks) | `<namespace>/v1` | Discriminator. Absent = legacy commit. |
| Tags | NO (author) | Comma-separated tags | REQUIRED for agent/v1. OPTIONAL otherwise. |
| Touch | YES (hooks) | Comma-separated tags | Extracted from `#tag` comments in staged files. Do NOT set manually. |
| Diff-Additions | YES (hooks) | Integer | Lines added. Do NOT set manually. |
| Diff-Deletions | YES (hooks) | Integer | Lines removed. Do NOT set manually. |
| Diff-Files | YES (hooks) | Integer | Files changed. Do NOT set manually. |
| Diff-Surface | YES (hooks) | api\|internal\|config\|data\|docs\|test | Primary impact surface. Do NOT set manually. |

### Agent namespace (agent/v1) — LLM-authored commits

| Trailer | Required | Auto | Format |
|---------|----------|------|--------|
| Agent-Id | REQUIRED | YES (resolved) | `<provider>/<identifier>` |
| Model | REQUIRED | YES (from env) | Model identifier string |
| Intent | REQUIRED | NO | Free-form. WHY this commit exists. Not a restatement of the subject. |
| Tags | REQUIRED | NO | Comma-separated semantic intent tags |
| Confidence | OPTIONAL | NO | low / medium / high |
| Refs | OPTIONAL | NO | Comma-separated issue/ticket refs |

### Vendor namespace (vendor/v1) — dependency updates

| Trailer | Required | Auto | Format |
|---------|----------|------|--------|
| Vendor-Name | REQUIRED | YES | Dependency name |
| Vendor-Ref | REQUIRED | YES | Version/tag/branch |
| Vendor-Commit | REQUIRED | YES | Full SHA (40 chars) |
| Vendor-License | OPTIONAL | NO | SPDX identifier |

Multi-vendor commits use duplicate trailer keys with positional association: the Nth `Vendor-Name` corresponds to the Nth `Vendor-Ref`/`Vendor-Commit`.

### Manual namespace (manual/v1) — human commits

No namespace-specific trailers required. Auto-added by hooks when no schema is present. Humans get Touch, Diff metrics, and Diff-Surface enrichment for free.

## 7. Tags vs Touch

Tags is what you SAID you were doing. Touch is what you ACTUALLY modified.

| Situation | Signal |
|-----------|--------|
| Tags mentions `docs` but Touch shows `auth, security` | Changed security code — really just docs? Review. |
| Tags mentions `auth.mfa` and Touch shows `auth` | Focused work, intent matches evidence. |
| Touch shows `security` but Tags is empty | Touched security code with no semantic tag. Review. |

Divergence between Tags and Touch is a review signal, not an error. Touch starts empty and grows as `#tag` comments are added to source files over time.

## 8. Enrichment Flow

Hooks run automatically on commit if configured:

1. **prepare-commit-msg hook**: Adds `Commit-Schema: manual/v1` (if no schema present), extracts Touch from staged `#tag` comments, computes Diff-Additions/Deletions/Files, classifies Diff-Surface from file paths.
2. **commit-msg hook**: Validates subject length (warns >72 chars), normalizes Tags/Touch to lowercase, validates tag syntax.

Enrichment is idempotent. Running hooks twice produces identical output. `AppendTrailer` checks for existing keys before adding.

Auto-computed trailers (Touch, Diff-*, Diff-Surface) MUST NOT be set manually in agent or vendor commits. Let the hooks compute them.

## 9. Pinned Decisions

These MUST NOT change without a major version bump to the commit schema:

1. Tag grammar: `segment ('.' segment)*` where `segment := [a-z][a-z0-9-]*`
2. Code tag sigil: `#` immediately adjacent, no space
3. Extraction regex: `(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)`
4. Trailer format: standard git `Key: Value`
5. Namespace ownership: prefix -> tool mapping
6. Schema discriminator: `Commit-Schema: <namespace>/v<N>`
7. Tags and Touch trailers: comma-separated, shared across namespaces
8. Prefix matching: segment-wise, not substring
9. Notes ref isolation: each tool owns its ref
10. Conventional commits subject line format
11. Agent-Id format: `<provider>/<identifier>`
12. Multi-valued trailers: duplicate keys with positional association
