# git-plumbing Commit Protocol v1

Structured commit format for the git-plumbing ecosystem. Defines
trailers, tags, notes, and enrichment responsibilities shared by
git-plumbing, git-agent, and git-vendor.

**Status:** Ratified v1 — 2026-02-11. Implemented in git-plumbing, git-agent, git-vendor.

---

## Principles

1. **Git-native.** Subjects, bodies, trailers, notes. No custom
   storage.
2. **Conventional-commits compatible.** Standard tooling (commitlint,
   semantic-release) works unchanged. Our trailers are invisible to
   tools that don't look for them.
3. **One discriminator.** `Commit-Schema` is the single entry point
   for machine parsing. Absent = legacy commit.
4. **Namespace isolation.** Each tool owns a trailer prefix. Tools
   write their own prefix, read any, ignore unknown.
5. **Forward-compatible.** New OPTIONAL trailers: no version bump.
   New REQUIRED trailers or changed semantics: version bump.
   Unknown trailers: preserved, never stripped.
6. **Tags are the backbone.** The hierarchical tag system — in code
   comments, in trailers, in queries — is the semantic index over
   the entire git history. Every design decision serves tag
   richness and queryability.
7. **LLM-first, human-compatible.** LLMs maintain code tags, author
   commit tags, and populate intent. Hooks auto-extract Touch from
   code. Humans benefit without extra effort.
8. **Protocol over mechanism.** This spec defines WHAT trailers exist
   and WHAT they mean. HOW they get populated (hooks, CLI, skills)
   is each tool's implementation concern, documented separately.

---

## 1. Commit Structure

### 1.1 Subject Line

```
<type>(<scope>)[!]: <description>
```

| Field | Rules |
| ----- | ----- |
| type | Lowercase. Configurable set (see Configurable Values). |
| scope | Lowercase `[a-z][a-z0-9-]*`. Module/component. |
| `!` | After `)`, before `:`. Marks breaking change. |
| description | Imperative mood. No period. No initial capital. |
| Total | MUST NOT exceed 72 characters. |

The subject is the source of truth for type and scope. It is
self-sufficient in `git log --oneline`. Tools that need the type
or scope MUST parse the subject line; there are no redundant
trailers duplicating this data.

### 1.2 Body

Optional. One blank line after subject. Free-form prose.
Explains why, not what. MAY contain inline #tags for
human readability and `git log --grep` (see Tag System, In Commit Body).

### 1.3 Trailers

One blank line after body (or subject). Standard git format:
`Key: Value`, one per line. Parsed by `git interpret-trailers`
and `git log --format='%(trailers)'`.

---

## 2. Commit-Schema: The Discriminator

```
Commit-Schema: <namespace>/v<N>
```

| Rule | Detail |
| ------ | -------- |
| REQUIRED on all ecosystem commits | Hooks auto-add manual/v1 |
| MAY appear multiple times | Multi-namespace commits |
| Absent = legacy commit | No trailer expectations |
| Value = `<namespace>/v<N>` | e.g., agent/v1 |

### Registered Namespaces

| Namespace | Owner | Description |
| ----------- | ------- | ------------- |
| agent | git-agent | LLM/agent-authored commits |
| vendor | git-vendor | Dependency vendor/update |
| manual | (shared) | Human commits opting into tags |

Future tools register new namespaces. Conflicts resolved by the
git-plumbing maintainer.

### Parser Behavior

1. Collect all Commit-Schema values.
2. For each namespace, check version against known versions.
3. If version > known: parse recognized trailers, skip unknown
   (graceful degradation).
4. If no Commit-Schema: treat as unstructured legacy commit.

---

## 3. Shared Trailers

These are tool-agnostic. ANY commit in ANY namespace may include
them. The tag system (Tags + Touch) is the semantic backbone of
the entire ecosystem.

### 3.1 Tags (author-declared)

```
Tags: auth.mfa, security, user-management
```

Semantic intent tags. What the commit IS ABOUT. Declared by the
author (human or LLM). See Tag System for tag syntax.

| Field | Value |
| ------- | ------- |
| Key | Tags |
| Required | REQUIRED (agent), OPTIONAL (vendor, manual) |
| Auto | No — requires author judgment |
| Format | Comma-separated tag list |

### 3.2 Touch (auto-extracted)

```
Touch: auth, auth.session, security, config
```

Structural impact tags. What code AREAS WERE ACTUALLY MODIFIED.
Auto-extracted from `#tag` comments in staged files by hooks.

| Field | Value |
| ------- | ------- |
| Key | Touch |
| Required | OPTIONAL |
| Auto | YES — hook extracts from staged diff |
| Format | Comma-separated tag list |
| Rollout | Starts empty. Grows as LLMs add #tags to source |

#### Tags vs Touch: Intent vs Evidence

Tags is what you SAID you were doing. Touch is what you
ACTUALLY modified. Divergence is signal.

| Tags | Touch | Signal |
| ------ | ------- | -------- |
| docs | auth, security | Changed security code — really just docs? |
| dx, refactor | auth, billing | Wide blast radius on refactor |
| auth.mfa | auth | Focused work, intent matches |
| (empty) | security | Touched security code, no semantic tag — review |

Touch is empty on day 1. This is by design. As LLMs add `#tag`
comments to source files over time, Touch becomes richer. Tags
carries the full weight alone until Touch catches up. The two
layers converge gradually — divergence detection becomes more
valuable as Touch coverage increases.

### 3.3 Diff Metrics (auto-computed)

```
Diff-Additions: 142
Diff-Deletions: 17
Diff-Files: 3
```

Pre-computed change summary. Enables aggregate queries across
hundreds of commits (heatmaps, energy trendlines) without
running `git diff --stat` per commit.

| Trailer | Auto | Format |
| --------- | ------ | -------- |
| Diff-Additions | YES | Integer (lines) |
| Diff-Deletions | YES | Integer (lines) |
| Diff-Files | YES | Integer (files) |

### 3.4 Diff-Surface (auto-classified)

```
Diff-Surface: api
```

The PRIMARY impact surface of the change. Classified from file
paths in the diff using configurable directory-to-surface
mapping with sensible defaults.

| Value | Meaning |
| ------- | --------- |
| api | Public interface changed (exported symbols) |
| internal | Implementation detail only |
| config | Configuration, schemas, build files |
| data | Data models, migrations, schemas |
| docs | Documentation only |
| test | Tests only |

Priority rule: If multiple surfaces are touched, the highest-
impact surface wins: api > data > config > internal > test > docs.

Default heuristic mapping (overridable via project config):

| Path Pattern | Surface |
| ------------- | --------- |
| `docs/`, `*.md`, `LICENSE` | docs |
| `*_test.go`, `**/__tests__/` | test |
| `internal/`, `**/internal/` | internal |
| `*.yml`, `*.yaml`, `*.toml`, `*.json` | config |
| `**/migrations/`, `**/schema*` | data |
| `cmd/`, `api/`, `**/handler*` | api |

### 3.5 Standard Git Trailers

```
Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
BREAKING CHANGE: removed password-only authentication
```

Standard git/conventional-commits trailers. Not namespaced.
Always valid. Breaking changes are indicated by `!` in the
subject line (conventional-commits standard). `BREAKING CHANGE`
footer is optional supplementary detail.

---

## 4. Namespace Trailers

### 4.1 Agent Namespace (Commit-Schema: agent/v1)

Written by git-agent. Present when an LLM/agent authors a commit.

| Trailer | Required | Auto | Format |
| --------- | ---------- | ------ | -------- |
| Agent-Id | REQUIRED | Yes (resolved) | `<provider>/<identifier>` |
| Model | REQUIRED | Yes (from env) | Model identifier string |
| Intent | REQUIRED | No (LLM-authored) | Free-form string |
| Tags | REQUIRED | No (LLM-authored) | Comma-separated |
| Confidence | OPTIONAL | No | low / medium / high |
| Refs | OPTIONAL | No | Comma-separated refs |

#### 4.1.1 Agent-Id

```
Agent-Id: claude-code/kyle-desktop
```

Format: `<provider>/<identifier>`

- **provider**: Tool name. Lowercase alphanumeric + hyphens.
  Examples: `claude-code`, `cursor`, `aider`, `github-copilot`
- **identifier**: Instance discriminator. Same character constraints.
  Resolved by git-agent: flag > env > git user.name (slugified)
  > hostname-pid.
  Examples: `kyle-desktop`, `ci-runner-3`, `project-alpha`

Queryable with native git:

```bash
git log --format='%(trailers:key=Agent-Id,valueonly)' | grep '^claude-code/'
```

#### 4.1.2 Model

```
Model: claude-opus-4-6
```

The LLM model that authored the commit. Auto-populated from
`$CLAUDE_MODEL` or equivalent environment variable. Enables:

- Debugging (which model introduced a bug?)
- Analytics (commit quality by model)
- Trust calibration (reviewer weights effort by model)

#### 4.1.3 Intent

```
Intent: implement 2FA per security roadmap SEC-042
```

REQUIRED. The agent MUST state WHY it made this commit. Not a
restatement of the subject. Explains the reasoning, the trigger,
or the broader goal.

Good: `implement 2FA per security roadmap SEC-042`
Bad:  `add TOTP authentication` (restates subject)

#### 4.1.4 Tags (in agent context)

REQUIRED for agent commits. Agents have the context to tag
semantically — they know what domain they're working in, what
the intent is, and what code areas are affected. Requiring Tags
ensures every agent commit enters the tag graph.

### 4.2 Vendor Namespace (Commit-Schema: vendor/v1)

Written by git-vendor. Present on dependency vendor/update commits.

| Trailer | Required | Auto | Format |
| --------- | ---------- | ------ | -------- |
| Vendor-Name | REQUIRED | Yes | Dependency name |
| Vendor-Ref | REQUIRED | Yes | Version/tag/branch |
| Vendor-Commit | REQUIRED | Yes | Full SHA (40 chars) |
| Vendor-License | OPTIONAL | No | SPDX identifier |

Multi-vendor updates use duplicate trailer keys with positional
association. The Nth `Vendor-Name` corresponds to the Nth
`Vendor-Ref` and `Vendor-Commit`. Parsers use `TrailerValues(key)`
to retrieve all values in order.

### 4.3 Manual Namespace (Commit-Schema: manual/v1)

No namespace-specific trailers required. Exists so human commits
can enter the tag graph with minimal friction. Auto-added by
prepare-commit-msg hook when no schema is present.

Humans benefit from auto-enrichment: Touch, Diff metrics, and
Diff-Surface are computed by hooks even on manual commits. A
human types 6 words; the hooks add structured metadata.

---

## 5. Notes (Extended Metadata)

Notes hold structured data that is either too large for
trailers or mutable (test results, context snapshots).
Each tool owns its own notes ref.

| Ref | Owner | Content |
| ----- | ------- | --------- |
| refs/notes/agent | git-agent | Decisions, context, handoffs |
| refs/notes/vendor | git-vendor | Audit data |
| refs/notes/metrics | git-plumbing hooks | Test results, coverage |

Tools MUST NOT write to another tool's notes ref.

### 5.1 Agent Notes (refs/notes/agent)

```json
{
  "schema": "note/v1",
  "type": "<type>",
  "timestamp": "2026-02-09T15:30:00Z",
  "payload": {}
}
```

| Type | Payload |
| ------ | --------- |
| decision | {question, chosen, alternatives[], reasoning} |
| context | {files_read[], tags_active[], tokens_used} |
| handoff | {completed[], pending[], blocked_on} |

### 5.2 Metrics Notes (refs/notes/metrics)

```json
{
  "schema": "metrics/v1",
  "tests_run": 52,
  "tests_passed": 52,
  "tests_added": 5,
  "coverage_delta": "+2.1%",
  "duration_ms": 3400
}
```

Written by post-commit hook (if test runner configured).
Mutable — tests can be re-run. Notes can be overwritten.

---

## 6. Tag System

The tag system is the semantic index over the git history. Tags
in code comments propagate to files, to folders, to commits, to
queries. The system is designed for LLM maintenance: agents add
`#tag` comments as they write code, hooks extract them into
Touch trailers, and the tag graph grows organically.

### 6.1 Grammar

```
tag     := segment ('.' segment)*
segment := ALPHA (ALPHA | DIGIT | '-')*
ALPHA   := [a-z]
DIGIT   := [0-9]
```

| Constraint | Rule |
| ------------ | ------ |
| Case | Lowercase only. Stored lowercase. |
| Word join | Hyphens: session-mgmt |
| Hierarchy | Dots: security.auth.oauth |
| Boundaries | No leading/trailing hyphens or dots |
| Start | Must start with letter (rejects #123) |
| Max length | 128 characters |
| Spaces | NONE. #tag only. # tag is NOT a tag. |

### 6.2 In Code Comments

```
code_tag := '#' tag
```

`#` MUST be immediately adjacent to the tag name. No space.

```go
// ProcessPayment handles payment processing.
// #payments #critical #pci.compliance
func ProcessPayment(order *Order) error {
```

Extraction regex (canonical — MUST NOT change):

```
(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)
```

Preceding character must be non-alphanumeric (whitespace, start
of comment, punctuation). This prevents matching URL fragments
(`https://x.com#section`) or color codes (`#ff0000`).

Lowercase-only in the regex rejects `#TODO`, `#FIXME`, `#HACK`
— existing uppercase conventions are not tags.

#### Scoping Rules

1. Tags immediately preceding a declaration -> declaration scope.
2. Tags before package/first declaration -> file scope.
3. File tags = file-level union of all declaration tags.
4. Folder tags = union of all file tags (computed, not stored).

v1: regex scan, no AST. Works for Go, Python, Rust, TypeScript,
Java, C#. Mid-function comment false positives acceptable in v1.

#### Tag Propagation Model

Tags propagate upward through the hierarchy:

```
function ProcessPayment()     -> #payments, #pci.compliance
  file payments.go            -> #payments, #pci.compliance, #billing
    folder internal/billing/  -> (union of all file tags)
      commit touching file    -> Touch: payments, pci.compliance, billing
```

LLMs add `#tag` comments as part of their normal coding workflow.
This is the primary mechanism for building tag coverage. Over
time, every significant function/method/class acquires semantic
tags that propagate automatically to commits via Touch.

### 6.3 In Commit Body (supplementary)

```
This fixes the #security.xss vulnerability from the audit.
```

Inline `#tags` in the body are for `git log --grep` and human
readability. Tools MUST parse the `Tags:` trailer, SHOULD NOT
parse the body for structured tag data.

### 6.4 In Trailers

```
Tags: auth, security.mfa, vendor.update.minor
Touch: auth, auth.session, security
```

No `#` prefix. Comma-separated. Optional space after comma.
Same grammar as code tags.

### 6.5 Prefix Matching

Querying a tag matches all descendants. Segments are atomic.

```
Commit tags: security.auth.oauth, payments.tax

Query "security"            -> MATCH  (prefix of security.auth.oauth)
Query "security.auth"       -> MATCH  (prefix of security.auth.oauth)
Query "security.auth.oauth" -> MATCH  (exact)
Query "security.mfa"        -> NO MATCH
Query "sec"                 -> NO MATCH (partial segment)
```

Implementation: split query and candidate on `.`. Query
segments must be a complete prefix of candidate segments.

---

## 7. Configurable Values

Each consuming tool may configure these. git-plumbing does not
enforce them — validation is the writing tool's responsibility.

| Value | Default | Owner |
| ------- | --------- | ------- |
| Commit types | feat fix refactor test docs chore perf style ci | git-agent |
| Max subject length | 72 | git-agent |
| Require scope | true | git-agent |
| Known tags | [] (freeform) | git-agent |
| Tag aliases | {} (none) | git-agent |
| Surface path mapping | (see Diff-Surface defaults) | git-plumbing |

---

## 8. git-plumbing Parser Contract

git-plumbing parses without opinions about which trailers
should exist. It is the dumb pipe.

| Capability | API | Status |
| ------------ | ----- | -------- |
| Ordered trailers (multi-valued) | `Commit.Trailers []Trailer` | Implemented |
| First-value accessor | `Commit.TrailerValue(key) string` | Implemented |
| All-values accessor | `Commit.TrailerValues(key) []string` | Implemented |
| Existence check | `Commit.HasTrailer(key) bool` | Implemented |
| Filter: exact match | `LogOpts.TrailerFilter map[string]string` | Implemented |
| Filter: prefix match (tags) | `LogOpts.TagFilter []TagQuery` via `MatchTagPrefix` | Implemented |
| Filter: contains (tag in list) | `MatchTagInList(query, tagList)` | Implemented |
| Tag extraction from files | `TagScan(paths)`, `TagScanContent(content)`, `MergeTags(result)` | Implemented |
| Subject parsing | `ParseSubject(line)` -> `Subject` struct | Implemented |
| Surface classification | `ClassifySurface(names, rules)` -> `Surface` | Implemented |
| Hook enrichment | `HookPrepareCommitMsg(ctx, dir, msg)` | Implemented |
| Hook validation | `HookCommitMsg(ctx, msg)` | Implemented |
| Diff metrics | `DiffStat`, `DiffCachedStat` | Implemented |
| Notes | `AddNote`, `GetNote`, `ListNotes` | Implemented |
| Custom refs | `UpdateRef`, `ShowRef`, `ForEachRef` | Implemented |

git-plumbing does NOT:

- Validate namespace-specific trailers
- Enforce Commit-Schema presence
- Parse note JSON payloads
- Resolve tag aliases or deprecations

---

## 9. Trailer Summary

13 trailers total. Every one has a concrete consumer.

### By Population Method

| Method | Trailers |
| -------- | ---------- |
| LLM-authored | Tags, Intent, Confidence, Refs |
| Auto-extract | Touch (from code #tags), Diff-Surface (from paths) |
| Auto-compute | Diff-Additions, Diff-Deletions, Diff-Files |
| Auto-resolve | Commit-Schema, Agent-Id, Model |
| Tool-config | Vendor-Name, Vendor-Ref, Vendor-Commit, Vendor-License |

### By Namespace

| Trailer | Required | Namespace |
| --------- | ---------- | ----------- |
| Commit-Schema | REQUIRED | shared (all) |
| Tags | varies | shared (REQUIRED for agent) |
| Touch | OPTIONAL | shared |
| Diff-Additions | OPTIONAL | shared |
| Diff-Deletions | OPTIONAL | shared |
| Diff-Files | OPTIONAL | shared |
| Diff-Surface | OPTIONAL | shared |
| Agent-Id | REQUIRED | agent |
| Model | REQUIRED | agent |
| Intent | REQUIRED | agent |
| Confidence | OPTIONAL | agent |
| Refs | OPTIONAL | agent |
| Vendor-Name | REQUIRED | vendor |
| Vendor-Ref | REQUIRED | vendor |
| Vendor-Commit | REQUIRED | vendor |
| Vendor-License | OPTIONAL | vendor |

---

## 10. Deferred to v2

Explicitly out of scope for v1. Documented here to prevent
re-invention and to preserve the design rationale.

### 10.1 Change-Kind

How the change affects the codebase (addition, extension,
modification, replacement, removal). The auto-heuristic
(additions-only = addition, mixed = modification) collapses
3 of 5 values into the fallback. Deferred until AST-level
analysis or reliable LLM classification is available.

### 10.2 Reversible

Whether `git revert` would apply cleanly. The heuristic
(additions-only = true) is wrong in both directions: an added
migration is not reversible; a modified typo is. Deferred as
a safety concern — tools trusting this for auto-revert could
cause damage.

### 10.3 Bisect-Hint

Guidance for `git bisect skip`. Trivially derivable from
Diff-Surface at bisect time. Pre-computing adds storage
cost for a query that runs once per bug hunt. Compute
on-the-fly when needed.

### 10.4 Continues / Reverts (chain edges)

Explicit edges between commits overlaid on linear history.
Continues heuristic (scope + tag overlap) produces false
edges. Reverts duplicates git's native `Revert "..."` subject
convention. If needed, LLMs can explicitly set these when
they know the chain — but auto-heuristics are unreliable.

### 10.5 Tag Index Cache

Compute-on-the-fly for v1. Add caching (e.g.,
`.git/plumbing/tags.index`) when someone hits a real
performance wall on tag queries.

---

## 11. Pinned Decisions (MUST NOT change without major version)

1. Tag grammar: `segment ('.' segment)*` where
   `segment := [a-z][a-z0-9-]*`
2. Code tag sigil: `#` immediately adjacent, no space
3. Extraction regex:
   `(?:^|[^a-zA-Z0-9])#([a-z][a-z0-9._-]*)`
4. Trailer format: standard git `Key: Value`
5. Namespace ownership: prefix -> tool mapping
6. Schema discriminator: `Commit-Schema: <namespace>/v<N>`
7. Tags and Touch trailers: comma-separated, shared
8. Prefix matching: segment-wise, not substring
9. Notes ref isolation: each tool owns its ref
10. Conventional commits subject line format
11. Agent-Id format: `<provider>/<identifier>`
12. Multi-valued trailers: duplicate keys with positional association

---

## 12. Resolved Design Notes

1. **Surface path mapping config format.** Resolved: this is a tooling
   concern, not a protocol concern (Principle 8: Protocol over mechanism).
   `ClassifySurface(paths, rules)` in git-plumbing already accepts custom
   rules programmatically. A declarative config file (`.git-plumbing/surface.yml`)
   is deferred to a future release. The protocol defines the surface VALUES
   and priority ordering — classification mechanism is each tool's business.
