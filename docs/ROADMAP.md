# git-vendor Product Engineering Roadmap

> **Document Type:** Product Engineering Specification & Development Roadmap
> **Version:** 1.0.0
> **Date:** February 4, 2026
> **Scope:** Months 1–6 foundation build, with forward references to post-foundation work
> **Purpose:** This is the canonical development plan for git-vendor. Every feature, every priority, every architectural decision flows from this document. If it's not in here, it's not planned. If it contradicts something said in a conversation, this document wins.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Assessment](#2-current-state-assessment)
3. [Definition of "Definitive"](#3-definition-of-definitive)
4. [Strategic Principles](#4-strategic-principles)
5. [Phase 1: Foundation Hardening (Months 1–2)](#5-phase-1-foundation-hardening-months-12)
6. [Phase 2: Supply Chain Intelligence (Months 2–4)](#6-phase-2-supply-chain-intelligence-months-24)
7. [Phase 3: Ecosystem Integration (Months 4–6)](#7-phase-3-ecosystem-integration-months-46)
8. [Phase 4: Visibility & Adoption (Ongoing)](#8-phase-4-visibility--adoption-ongoing)
9. [Architecture Guidelines](#9-architecture-guidelines)
10. [Testing Strategy](#10-testing-strategy)
11. [Success Metrics](#11-success-metrics)
12. [Dependency Map](#12-dependency-map)
13. [Risk Register](#13-risk-register)
14. [Post-Foundation Forward References](#14-post-foundation-forward-references)

---

## 1. Executive Summary

git-vendor is a Go CLI tool for deterministic, file-level source code vendoring from any Git repository. It already solves the core mechanical problem — pulling specific files from remote repos into a local project with cryptographic integrity tracking via SHA-256 lockfiles.

**The gap:** git-vendor is currently a good vendoring tool. To become the *definitive* vendoring tool — the one that gets cited in blog posts, recommended in security audits, and required in enterprise procurement — it needs to graduate from "moves files reliably" to "provides complete supply chain provenance for vendored source code."

**The plan:** Over 6 months, extend git-vendor with SBOM generation, license compliance, CVE monitoring, dependency visualization, drift detection, and compliance evidence reporting. All of this ships free and open-source. The goal is not revenue. The goal is adoption, credibility, and an unassailable position as the standard tool for source-level vendoring.

**The outcome:** By month 6, any developer who vendors source code from Git repositories and does NOT use git-vendor is accepting unnecessary risk. That's the bar.

---

## 2. Current State Assessment

### 2.1 What Exists Today

| Capability | Status | Notes |
|---|---|---|
| File-level vendoring from any Git repo | ✅ Shipped | Core differentiator — not whole-repo, granular file selection |
| Cryptographic lockfile (`vendor.lock`) | ✅ Shipped | SHA-256 hashes, source repo URL, commit SHA, file paths, timestamps |
| Interactive TUI | ✅ Shipped | File selection, dependency browsing |
| Parallel processing | ✅ Shipped | 3–5x sync acceleration over sequential |
| Incremental caching | ✅ Shipped | Only re-fetches changed content |
| CI/CD automation hooks | ✅ Shipped | Non-interactive mode for pipelines |
| Multi-platform Git support | ✅ Shipped | GitHub, GitLab, Bitbucket |
| License detection | ✅ Shipped | Basic detection during `add` |
| Provenance tracking | ✅ Shipped | Source repo, commit, timestamp per file |
| Cross-language support | ✅ Shipped | Not Go-specific; works with any source files |

### 2.2 What Does NOT Exist Yet

| Capability | Status | Priority |
|---|---|---|
| SBOM generation (CycloneDX/SPDX) | ❌ Not started | P0 — regulatory requirement |
| CVE/vulnerability scanning | ❌ Not started | P0 — security requirement |
| Drift detection (origin vs. vendored copy) | ❌ Not started | P0 — integrity requirement |
| Dependency graph visualization | ❌ Not started | P1 — adoption driver |
| Compliance evidence reports | ❌ Not started | P1 — enterprise requirement |
| `git vendor audit` command | ❌ Not started | P1 — combines multiple checks |
| CI/CD GitHub Action | ❌ Not started | P1 — adoption driver |
| Migration/extraction metrics | ❌ Not started | P2 — monolith use case |
| Plugin/hook system | ❌ Not started | P2 — extensibility |
| Sigstore/cosign integration | ❌ Not started | P2 — advanced provenance |

### 2.3 Known Technical Debt

Before building new features, audit and address:

- **Test coverage:** Ensure all existing commands have integration tests that exercise real Git operations, not just mocks. New features will compound any gaps.
- **Error handling consistency:** All user-facing errors should follow a consistent format with actionable remediation steps.
- **Lockfile schema versioning:** The `vendor.lock` format must be forward-compatible. Add a schema version field if not present. Every new feature that adds lockfile data must not break older parsers.
- **Documentation:** README should be comprehensive enough that a new user can go from zero to vendored dependency in under 5 minutes. Man pages for every command.

---

## 3. Definition of "Definitive"

A vendoring tool becomes "definitive" when it satisfies all four stakeholder groups simultaneously:

### 3.1 Individual Developer
> "I use git-vendor because it's the fastest way to pull specific files from a repo and keep them in sync, and it warns me if I'm pulling in something with a bad license or a known CVE."

**Requirements:** Fast, ergonomic CLI. Clear output. Sensible defaults. No unnecessary ceremony.

### 3.2 Security Team
> "We require git-vendor because it's the only vendoring tool that produces a machine-readable SBOM of vendored source code and continuously monitors for vulnerabilities in vendored dependencies."

**Requirements:** SBOM output in CycloneDX and SPDX. CVE scanning. Integrity verification. Audit trail.

### 3.3 Compliance/Legal Team
> "git-vendor satisfies our evidence requirements for EO 14028, NIST SP 800-161, and DORA Article 28 because it produces signed provenance records and license compliance reports."

**Requirements:** Compliance evidence generation. License detection and policy enforcement. Provenance chain documentation.

### 3.4 Engineering Leadership
> "I can see what vendored code exists across our services, where it came from, whether it's drifted from origin, and whether we're exposed to any known vulnerabilities."

**Requirements:** Aggregate reporting. Drift detection. Dependency graph. Migration metrics.

**The tool is "definitive" when all four groups prefer it over the alternative (which is usually copy-paste with no tracking at all).**

---

## 4. Strategic Principles

These principles govern every design and implementation decision in this roadmap.

### 4.1 Free and Open-Source, No Exceptions
Every feature in this roadmap ships in the open-source CLI. No feature gates. No "upgrade to unlock." The entire point of this phase is adoption, and adoption requires zero friction. Revenue comes later, from different products built on top of this foundation (see `OPPORTUNITIES.md`).

### 4.2 Lockfile Is the Source of Truth
Every new capability derives its data from the `vendor.lock` file and the vendored files themselves. The lockfile is git-vendor's database. If information isn't in the lockfile or derivable from the vendored files, we either add it to the lockfile or we don't claim to have it. No external state. No databases. No accounts. No network calls except to Git remotes and public vulnerability APIs.

### 4.3 Offline-First, Network-Optional
Every command except explicit sync/fetch operations must work completely offline. CVE scanning and SBOM enrichment may optionally use network APIs, but the tool must degrade gracefully without network access. This is a hard requirement for air-gapped environments (DoD, financial services, critical infrastructure).

### 4.4 Output Is the Product
The primary adoption driver is not the vendoring mechanism (developers already know how to copy files). It's the *output* — the SBOM, the dependency graph, the compliance report, the CVE alert. Every new feature should produce output that someone wants to screenshot, paste into a Slack channel, attach to a Jira ticket, or present in a meeting. If the output isn't compelling enough to share, the feature isn't done.

### 4.5 Incremental, Not Big-Bang
Every feature ships as a standalone `git vendor <subcommand>` that is useful immediately, without requiring any other new feature. No feature depends on another unshipped feature. A user who only cares about SBOMs never has to touch the CVE scanner. A user who only cares about drift detection never has to generate a compliance report.

### 4.6 Don't Break Anything
The lockfile format must remain backward-compatible. New fields are additive. Old lockfiles must work with new CLI versions. New CLI features that need data not present in old lockfiles must either compute it on-the-fly or prompt the user to run a migration command. Zero surprise breakage.

---

## 5. Phase 1: Foundation Hardening (Months 1–2)

**Goal:** Ensure the existing tool is production-grade, well-documented, well-tested, and has the lockfile schema extensibility needed for everything that follows.

---

### Feature 1.1: Lockfile Schema Versioning

**What:** Add a `schema_version` field to `vendor.lock`. Current format becomes `v1`. All new features that add lockfile fields increment the minor version (e.g., `v1.1`, `v1.2`). Major version changes are reserved for breaking changes (which should never happen).

**Why:** Every subsequent feature adds metadata to the lockfile. Without versioning, older CLI versions will encounter unknown fields and may behave unpredictably. This is the single most important foundation work.

**Implementation Details:**
- Add `schema_version: "1.0"` as the first field in `vendor.lock`
- When reading a lockfile, if `schema_version` is absent, treat as `v1.0` (backward compat)
- When reading a lockfile with a higher minor version than the CLI understands, warn but proceed (unknown fields are ignored)
- When reading a lockfile with a higher major version, error with message: "This lockfile requires git-vendor vX.Y.Z or newer. Run `git vendor self-update` or visit [release page]."
- Document the schema in a `LOCKFILE_SCHEMA.md` file in the repo

**Difficulty:** 1–2
**Estimated Effort:** 2–3 days
**Dependencies:** None
**Acceptance Criteria:**
- [ ] `vendor.lock` includes `schema_version` on new `git vendor add`
- [ ] Old lockfiles without `schema_version` still parse correctly
- [ ] CLI warns on unknown minor version, errors on unknown major version
- [ ] `LOCKFILE_SCHEMA.md` documents every field, its type, when it was added, and what it means

---

### Feature 1.2: `git vendor verify` Hardening

**What:** Ensure `git vendor verify` is bulletproof — it must detect any tampering, corruption, or drift between what the lockfile says and what's actually on disk.

**Why:** This is the trust anchor. If `verify` says "all good," everything downstream (SBOMs, compliance reports, CVE scans) can be trusted. If `verify` is unreliable, nothing else matters.

**Implementation Details:**
- Re-hash every vendored file and compare against lockfile SHA-256
- Detect files present on disk but not in lockfile (unauthorized additions)
- Detect files in lockfile but missing from disk (deletions)
- Detect files with matching hash but different permissions (if tracked)
- Exit code 0 = all verified, exit code 1 = discrepancies found
- Machine-readable output mode (`--format json`) for CI integration
- Human-readable output with clear pass/fail per file

**Difficulty:** 2
**Estimated Effort:** 3–5 days
**Dependencies:** None (builds on existing verify)
**Acceptance Criteria:**
- [ ] Detects modified files (hash mismatch)
- [ ] Detects added files (in vendor dir but not in lockfile)
- [ ] Detects deleted files (in lockfile but not on disk)
- [ ] `--format json` produces machine-parseable output
- [ ] Exit codes are documented and consistent
- [ ] Integration test covers all three discrepancy types

---

### Feature 1.3: Lockfile Metadata Enrichment

**What:** Extend `vendor.lock` entries with additional metadata fields that subsequent features will consume: `license_spdx`, `source_version_tag`, `vendored_at` (ISO 8601 timestamp), `vendored_by` (git user identity).

**Why:** SBOM generation, compliance reports, and license scanning all need this metadata. Collecting it at `add`/`sync` time (when we already have the source repo cloned) is trivial. Trying to reconstruct it later is painful.

**Implementation Details:**
- During `git vendor add`:
  - Detect license file in source repo, classify using SPDX identifier, store as `license_spdx`
  - If the source commit matches a tag, store as `source_version_tag`
  - Record current ISO 8601 timestamp as `vendored_at`
  - Record `git config user.name` and `git config user.email` as `vendored_by`
- During `git vendor sync`:
  - Update all metadata fields for synced entries
  - Preserve `vendored_at` of the original add; add `last_synced_at` for updates
- All new fields are optional (missing = "not available")
- License detection: Use the `go-license-detector` library or equivalent. Detect from LICENSE, LICENSE.md, LICENSE.txt, COPYING files. Store SPDX short identifier (e.g., "MIT", "Apache-2.0", "GPL-3.0-only").

**Difficulty:** 2–3
**Estimated Effort:** 5–7 days
**Dependencies:** Feature 1.1 (schema versioning)
**Acceptance Criteria:**
- [ ] `git vendor add` populates all new metadata fields
- [ ] `git vendor sync` updates metadata for synced entries
- [ ] License detection correctly identifies MIT, Apache-2.0, GPL-2.0, GPL-3.0, BSD-2-Clause, BSD-3-Clause, ISC, MPL-2.0, LGPL-2.1, Unlicense
- [ ] `git vendor list` displays license and version tag when available
- [ ] Lockfile migration command updates existing lockfiles with computable metadata

---

### Feature 1.4: Comprehensive Test Suite

**What:** Achieve ≥80% test coverage across all commands with integration tests that exercise real Git operations against local test repositories.

**Why:** Every new feature will be built on top of existing commands. If the foundation is untested, bugs in new features may actually be bugs in old code, making debugging hell. This is also a quality signal to potential adopters reviewing the repo.

**Implementation Details:**
- Create a `testdata/` directory with small Git repos for testing
- Integration tests for every command: `add`, `sync`, `verify`, `list`, `remove`
- Edge case tests: empty repos, repos with no license, binary files, symlinks, large files, repos requiring authentication (should fail gracefully)
- CI pipeline: GitHub Actions workflow that runs full test suite on every PR
- Coverage reporting: Badge in README showing coverage percentage

**Difficulty:** 3
**Estimated Effort:** 7–10 days
**Dependencies:** None
**Acceptance Criteria:**
- [ ] ≥80% line coverage
- [ ] GitHub Actions CI runs on every push and PR
- [ ] Coverage badge in README
- [ ] All edge cases listed above have explicit tests
- [ ] Tests run in under 60 seconds

---

### Feature 1.5: Documentation Overhaul

**What:** Complete rewrite of README and creation of comprehensive documentation.

**Why:** The README is the landing page. A developer evaluating git-vendor will spend 30 seconds on the README before deciding whether to try it. If they can't understand what it does, how to install it, and see a compelling example in 30 seconds, they're gone.

**Implementation Details:**
- **README.md:**
  - One-sentence description
  - Animated GIF or asciicast of basic workflow (add → sync → verify)
  - "Why git-vendor?" section: 3 concrete scenarios where git-vendor is better than copy-paste, git submodules, or language-specific package managers
  - Quick start: Install → first vendored dependency in under 5 commands
  - Feature matrix comparing git-vendor to alternatives (git submodules, git subtree, manual copy-paste)
  - Link to full documentation
- **docs/ directory:**
  - `COMMANDS.md` — Complete reference for every command, every flag, every option
  - `LOCKFILE_SCHEMA.md` — Complete lockfile specification (from Feature 1.1)
  - `CI_CD.md` — Guide for integrating git-vendor into GitHub Actions, GitLab CI, Jenkins, CircleCI
  - `SECURITY.md` — How git-vendor handles integrity, what it does and doesn't guarantee
  - `MIGRATION.md` — How to adopt git-vendor in an existing project that already has manually vendored code
  - `FAQ.md` — Common questions and gotchas
- **Man pages:** `git-vendor(1)` with full command reference

**Difficulty:** 3 (writing, not coding)
**Estimated Effort:** 5–7 days
**Dependencies:** None
**Acceptance Criteria:**
- [ ] New user can go from zero to vendored dependency in under 5 minutes following README alone
- [ ] Every command has complete documentation with examples
- [ ] CI/CD guide covers at least GitHub Actions and GitLab CI
- [ ] Feature comparison matrix is accurate and fair to alternatives

---

## 6. Phase 2: Supply Chain Intelligence (Months 2–4)

**Goal:** Transform git-vendor from a file-moving tool into a supply chain intelligence tool. Every feature in this phase produces output that security teams, compliance teams, or engineering leadership would find valuable.

---

### Feature 2.1: SBOM Generation (`git vendor sbom`)

**What:** Generate a Software Bill of Materials in CycloneDX and SPDX formats from the `vendor.lock` and vendored file metadata.

**Why:** SBOM requirements are proliferating across regulations (EO 14028, DORA, EU CRA, NIST SP 800-161). Every team that needs an SBOM and discovers git-vendor produces one for free becomes a user. This is table stakes for being taken seriously in the supply chain security space.

**Implementation Details:**
- New command: `git vendor sbom [--format cyclonedx|spdx] [--output file]`
- Default format: CycloneDX JSON (more widely adopted in security tooling)
- Mapping from lockfile to SBOM:
  - Each vendored dependency (grouped by source repo) → one SBOM component
  - Component name: source repo name
  - Component version: `source_version_tag` if available, otherwise commit SHA
  - Component type: "library" (SBOM spec term)
  - Supplier: extracted from repo URL (e.g., "github.com/org")
  - Hashes: SHA-256 from lockfile
  - License: `license_spdx` from lockfile
  - External references: source repo URL, commit URL
  - Provenance: `vendored_at` timestamp, `vendored_by` identity
- Use CycloneDX Go library (`github.com/CycloneDX/cyclonedx-go`) for CycloneDX output
- Use `tools-go` (`github.com/spdx/tools-golang`) for SPDX output
- SBOM metadata:
  - Tool: "git-vendor" with version
  - Timestamp: generation time
  - Component: the project containing vendored code

**Difficulty:** 2–3
**Estimated Effort:** 5–7 days
**Dependencies:** Feature 1.3 (metadata enrichment — needs license and version data)
**Acceptance Criteria:**
- [ ] CycloneDX JSON output validates against CycloneDX schema
- [ ] SPDX JSON output validates against SPDX 2.3 schema
- [ ] Every vendored dependency appears as a component with name, version, license, and hashes
- [ ] SBOM can be ingested by Dependency-Track, Grype, and OWASP tools without errors
- [ ] `--output` flag writes to file; without it, writes to stdout (pipeable)

---

### Feature 2.2: CVE/Vulnerability Scanning (`git vendor scan`)

**What:** Check vendored dependencies against the OSV.dev vulnerability database and report known CVEs.

**Why:** This is the feature that makes security teams care. Snyk, Grype, and Trivy scan `package.json`, `go.mod`, `requirements.txt` — but they cannot scan vendored source code that doesn't appear in any manifest. git-vendor fills this blind spot.

**Implementation Details:**
- New command: `git vendor scan [--format table|json] [--fail-on critical|high|medium|low]`
- For each vendored dependency:
  1. Extract ecosystem and package identity from source repo URL + version tag
  2. Query OSV.dev API (`https://api.osv.dev/v1/query`) with package info
  3. If no version tag, query by commit SHA (OSV supports this)
  4. If neither resolves (internal/private repos), skip with informational message
- Output:
  - Table format: dependency name, CVE ID, severity, affected versions, fixed version, description
  - JSON format: full OSV response per vulnerability
  - Summary line: "X dependencies scanned, Y vulnerabilities found (Z critical, W high)"
- `--fail-on` flag: exit code 1 if any vulnerability at or above specified severity. Designed for CI gating.
- Cache responses locally (`.git-vendor-cache/`) for 24 hours to avoid hammering the API
- Rate limiting: respect OSV.dev rate limits, batch queries where possible

**Limitations to document clearly:**
- Only works for vendored code from public open-source packages that OSV.dev tracks
- Code vendored from internal/private repos won't have CVE data (no vulnerability database covers them)
- Commit-level granularity may miss vulnerabilities announced against version ranges
- This is a best-effort scan, not a guarantee of security

**Difficulty:** 3–4
**Estimated Effort:** 7–10 days
**Dependencies:** Feature 1.3 (metadata enrichment — needs version tag for accurate lookups)
**Acceptance Criteria:**
- [ ] Correctly identifies known CVEs for vendored dependencies from well-known packages (test against a dependency with a known historical CVE)
- [ ] `--fail-on` exits with code 1 when threshold is exceeded
- [ ] JSON output includes full CVE details for each finding
- [ ] Gracefully handles: no network, API rate limits, unresolvable packages
- [ ] Clearly communicates when a dependency cannot be scanned (internal repo, no version mapping)

---

### Feature 2.3: Drift Detection (`git vendor drift`)

**What:** Compare each vendored file against its source origin and report how much the local copy has diverged.

**Why:** This is git-vendor's most unique capability — no other tool does this. When someone vendors code, they often modify it locally (bug fixes, customizations, removals). Over time, the vendored copy drifts from the original. When the original gets a security patch, nobody knows the vendored copy needs updating too. Drift detection makes this visible.

**Implementation Details:**
- New command: `git vendor drift [--dependency name] [--format table|json|detail]`
- For each vendored dependency:
  1. Fetch the current state of the source file(s) at the originally-vendored commit SHA (from lockfile)
  2. Fetch the current HEAD (or latest tag) of the source repo
  3. Compute three-way comparison:
     - **Local drift:** diff between vendored lockfile state and current local files (local modifications)
     - **Upstream drift:** diff between vendored lockfile state and upstream HEAD (upstream changes since vendoring)
     - **Combined divergence:** are local changes and upstream changes in the same files? (merge conflict risk)
  4. Report:
     - Files with local modifications (and percentage changed)
     - Files with upstream changes available (and percentage changed)
     - Files with both (conflict risk)
     - Overall drift score per dependency (0% = identical, 100% = completely rewritten)
- `--detail` flag shows actual diff output per file
- Cache fetched upstream state in `.git-vendor-cache/` to avoid repeated network calls
- `--offline` flag skips upstream fetch and only reports local drift (vs. lockfile state)

**Difficulty:** 4–5
**Estimated Effort:** 10–14 days
**Dependencies:** Feature 1.2 (verify hardening — needs reliable hash comparison)
**Acceptance Criteria:**
- [ ] Detects local modifications to vendored files
- [ ] Detects upstream changes since vendoring
- [ ] Identifies files with both local and upstream changes (conflict risk)
- [ ] Drift percentage calculation is meaningful (line-level, not byte-level)
- [ ] `--offline` mode works without network access
- [ ] `--format json` produces machine-parseable output for CI integration

---

### Feature 2.4: License Policy Enforcement (`git vendor license`)

**What:** Scan all vendored dependencies for license compliance and enforce configurable policies (e.g., "no GPL in this proprietary project").

**Why:** FOSSA charges $5k+/year for license compliance. git-vendor's version is narrower (only vendored code) but covers this specific use case perfectly. The developer who gets a "WARNING: this file is GPL-licensed and your repo is MIT" during `git vendor add` will remember that.

**Implementation Details:**
- New command: `git vendor license [--policy file] [--format table|json]`
- Policy file (`.git-vendor-policy.yml`):
  ```yaml
  license_policy:
    allow:
      - MIT
      - Apache-2.0
      - BSD-2-Clause
      - BSD-3-Clause
      - ISC
    deny:
      - GPL-2.0-only
      - GPL-3.0-only
      - AGPL-3.0-only
    warn:
      - LGPL-2.1-only
      - MPL-2.0
    unknown: warn  # what to do when license can't be detected: allow | warn | deny
  ```
- Behavior:
  - During `git vendor add`: Check license against policy, warn or block depending on config
  - `git vendor license` standalone: Report license status for all vendored dependencies
  - `--fail-on deny` for CI gating
- License detection improvement:
  - Check LICENSE, LICENSE.md, LICENSE.txt, COPYING, COPYING.md at repo root
  - Check license headers in individual vendored source files
  - Use SPDX license list for classification
  - Store detected license per-file, not just per-repo (a repo may contain files under different licenses)

**Difficulty:** 2–3
**Estimated Effort:** 5–7 days
**Dependencies:** Feature 1.3 (metadata enrichment — needs license detection)
**Acceptance Criteria:**
- [ ] Detects licenses for ≥95% of vendored dependencies from public repos with standard LICENSE files
- [ ] Policy file correctly blocks denied licenses during `add`
- [ ] `--fail-on deny` exits with code 1 for CI
- [ ] Correctly identifies dual-licensed projects
- [ ] Handles "no license detected" case according to policy config

---

## 7. Phase 3: Ecosystem Integration (Months 4–6)

**Goal:** Make git-vendor easy to adopt in existing workflows and produce output that non-developers (managers, auditors, security teams) can use directly.

---

### Feature 3.1: Unified Audit Command (`git vendor audit`)

**What:** A single command that runs verify + scan + license + drift and produces a comprehensive report.

**Why:** Nobody wants to run four commands. The security team wants one command that tells them "is our vendored code safe?" with a single exit code for CI.

**Implementation Details:**
- New command: `git vendor audit [--format table|json|html] [--output file] [--fail-on critical|high|medium|low]`
- Runs in sequence:
  1. `verify` — integrity check
  2. `license` — policy compliance
  3. `scan` — vulnerability check
  4. `drift` — drift detection (upstream only, with cache)
- Aggregates results into a single report:
  - Summary: X dependencies, Y files, Z vulnerabilities, W license issues, V drifted
  - Per-dependency detail: status badge (PASS/WARN/FAIL), findings
  - Overall status: PASS (all clear) / WARN (issues found, none blocking) / FAIL (blocking issues)
- HTML format produces a self-contained single-page report with styling (for attachment to tickets/emails)
- Exit code: 0 = PASS, 1 = FAIL, 2 = WARN (configurable via `--fail-on`)

**Difficulty:** 3
**Estimated Effort:** 5–7 days (most logic is calling existing commands and formatting)
**Dependencies:** Features 2.1, 2.2, 2.3, 2.4
**Acceptance Criteria:**
- [ ] Single command produces comprehensive report
- [ ] HTML report is self-contained (no external CSS/JS) and looks professional
- [ ] Exit codes are reliable for CI gating
- [ ] Report clearly separates blocking vs. informational findings
- [ ] Runs in under 30 seconds for a project with 20 vendored dependencies (cached)

---

### Feature 3.2: Dependency Graph Visualization (`git vendor graph`)

**What:** Generate a visual dependency graph showing vendored dependencies, their source repos, and relationships.

**Why:** This is the highest-value adoption driver because it's *visible*. A pretty dependency graph gets shared in Slack, shown in architecture reviews, and included in migration planning docs. Every time someone shares it, git-vendor's name is on it.

**Implementation Details:**
- New command: `git vendor graph [--format mermaid|dot|html|json]`
- Graph data:
  - Nodes: the current project, each vendored dependency (source repo)
  - Edges: "vendors from" relationship
  - Node metadata: version/commit, license, vulnerability count, drift percentage
  - Color coding: green (clean), yellow (warnings), red (vulnerabilities or policy violations)
- Output formats:
  - **Mermaid:** Embeddable in GitHub/GitLab markdown, renders in browser
  - **DOT (Graphviz):** For teams with existing graphviz tooling
  - **HTML:** Self-contained interactive graph using D3.js force-directed layout
  - **JSON:** Raw graph data for custom visualization
- Import analysis (stretch goal):
  - Parse vendored source files for import/include/require statements
  - Identify cross-dependency relationships (vendored file A imports vendored file B)
  - Show these as internal edges in the graph

**Difficulty:** 4–5
**Estimated Effort:** 10–14 days (data is easy, visualization takes iteration)
**Dependencies:** Feature 1.3 (metadata for node annotations)
**Acceptance Criteria:**
- [ ] Mermaid output renders correctly in GitHub markdown
- [ ] HTML output is a single self-contained file that opens in any browser
- [ ] Nodes are color-coded by status
- [ ] Graph is readable with up to 30 dependencies (layout doesn't collapse)
- [ ] `--format json` provides complete graph data for custom tooling

---

### Feature 3.3: GitHub Action (`git-vendor-action`)

**What:** A GitHub Actions action that runs `git vendor audit` on every PR and posts results as a PR comment and/or check status.

**Why:** GitHub Actions is where adoption happens. If a developer can add git-vendor to their CI in 5 lines of YAML, they will. If they have to write a custom script, they won't.

**Implementation Details:**
- Separate repository: `EmundoT/git-vendor-action`
- Action YAML:
  ```yaml
  - uses: EmundoT/git-vendor-action@v1
    with:
      fail-on: high           # vulnerability severity threshold
      license-policy: .git-vendor-policy.yml
      post-comment: true      # post results as PR comment
      sbom-output: sbom.json  # optionally produce SBOM artifact
  ```
- Features:
  - Installs git-vendor binary (cached for speed)
  - Runs `git vendor audit`
  - Posts formatted results as PR comment (collapsible sections for detail)
  - Sets check status (pass/fail)
  - Optionally uploads SBOM as build artifact
- Also provide GitLab CI template (`.gitlab-ci.yml` snippet in docs)

**Difficulty:** 3
**Estimated Effort:** 5–7 days
**Dependencies:** Feature 3.1 (audit command)
**Acceptance Criteria:**
- [ ] Action installs and runs in under 60 seconds on a standard GitHub runner
- [ ] PR comment is well-formatted and collapsible
- [ ] Check status correctly reflects audit results
- [ ] SBOM artifact is uploadable and downloadable
- [ ] Works with both public and private repositories

---

### Feature 3.4: Compliance Evidence Reports (`git vendor compliance`)

**What:** Generate formatted compliance evidence documents for specific regulatory frameworks.

**Why:** The evidence itself is trivial to produce (it's just formatted lockfile data). But presenting it in the format an auditor expects saves compliance teams hours of manual work. This is a cherry-on-top feature, not a major driver, but it signals that git-vendor understands enterprise compliance workflows.

**Implementation Details:**
- New command: `git vendor compliance [--framework eo14028|nist-800-161|dora|cra|soc2] [--format json|pdf|html]`
- Each framework template includes:
  - **EO 14028:** Component inventory, provenance chain, SBOM reference, integrity verification results
  - **NIST SP 800-161:** Supply chain risk assessment, component origins, vulnerability status
  - **DORA Article 28:** Third-party ICT library tracking, version monitoring, risk assessment
  - **EU CRA:** SBOM, vulnerability handling process documentation, update tracking
  - **SOC 2:** Change management evidence, integrity controls, monitoring documentation
- JSON output for machine ingestion by GRC platforms (RegScale, Drata, Vanta, etc.)
- HTML output for human review
- PDF output via HTML-to-PDF conversion

**Difficulty:** 2–3 per framework template
**Estimated Effort:** 7–10 days (for all five frameworks)
**Dependencies:** Features 2.1, 2.2, 2.3 (needs SBOM, CVE, and drift data)
**Acceptance Criteria:**
- [ ] Each framework template produces a complete evidence document
- [ ] JSON output can be ingested by at least one GRC platform (test with RegScale API if possible)
- [ ] HTML output is professional enough to attach to an audit response
- [ ] Framework templates are externalized (YAML/JSON config) so new frameworks can be added without code changes

---

### Feature 3.5: Migration Metrics (`git vendor metrics`)

**What:** For projects using git-vendor during monolith decomposition, report extraction progress, completeness, and remaining work.

**Why:** This feature specifically serves the monolith decomposition use case. A developer extracting code from a monolith can show their engineering manager "we've extracted 73% of the payments module, 12 files remain, 3 have been modified locally." That number goes into the weekly report.

**Implementation Details:**
- New command: `git vendor metrics [--source repo-url] [--format table|json]`
- Metrics:
  - Total files vendored from source repo
  - Total files available in source repo (at vendored commit)
  - Extraction completeness percentage
  - Files vendored but locally modified (drift count)
  - Files vendored but not imported/used anywhere (dead vendored code)
  - Timeline: when each file was vendored (from `vendored_at` metadata)
- Requires `--source` to scope metrics to a specific source repository
- Use case: `git vendor metrics --source https://github.com/company/monolith` → shows extraction progress from that specific monolith

**Difficulty:** 3
**Estimated Effort:** 5–7 days
**Dependencies:** Feature 1.3 (metadata), Feature 2.3 (drift detection)
**Acceptance Criteria:**
- [ ] Correctly calculates extraction completeness percentage
- [ ] Identifies locally modified vendored files
- [ ] JSON output is consumable by dashboards
- [ ] Works when source repo is not accessible (uses lockfile data only, reduced metrics)

---

## 8. Phase 4: Visibility & Adoption (Ongoing)

This phase runs in parallel with all technical work. Building features nobody knows about is pointless.

### 8.1 GitHub Presence

- **Stars goal:** 500 by month 3, 2,000 by month 6
- **README quality:** Must be top-1% of Go CLI tool READMEs
- **GitHub Topics:** Tag with `vendoring`, `supply-chain-security`, `sbom`, `dependency-management`, `compliance`, `devsecops`
- **Social proof:** Add "Used by" section as adoption grows
- **Release cadence:** Monthly releases with clear changelogs

### 8.2 Content Marketing

Write and publish (on blog, Dev.to, Hashnode, or personal site):

1. **"The Vendoring Blind Spot: Why SCA Tools Miss Your Most Dangerous Dependencies"** — Problem framing. Published when CVE scanning ships.
2. **"Generating SBOMs for Vendored Source Code"** — Tutorial. Published when SBOM generation ships.
3. **"How We Track Code Drift in Monolith Decompositions"** — Case study format. Published when drift detection ships.
4. **"git-vendor vs. git submodules vs. copy-paste: A Practical Comparison"** — Evergreen comparison. Published with documentation overhaul.

### 8.3 Community Engagement

- Answer vendoring-related questions on Stack Overflow (link to git-vendor where relevant)
- Submit to Awesome Go list
- Submit to Hacker News "Show HN" when a major feature ships (SBOM or CVE scanning — these have the most news value)
- Engage with InnerSource Commons community (git-vendor is relevant to their mission)
- Present at local meetups or record a short demo video

### 8.4 Integration Partnerships (Soft)

- Ensure git-vendor SBOM output works seamlessly with:
  - OWASP Dependency-Track
  - Grype
  - Trivy
  - Snyk (import SBOM feature)
- Document integration with GRC platforms:
  - RegScale (compliance evidence ingestion via API)
  - Drata
  - Vanta
- These are documentation exercises, not code partnerships

---

## 9. Architecture Guidelines

### 9.1 Command Structure

```
git vendor <command> [flags]

Commands:
  add          Add a vendored dependency
  remove       Remove a vendored dependency
  sync         Sync vendored dependencies to latest
  verify       Verify integrity of vendored files
  list         List vendored dependencies
  sbom         Generate SBOM (CycloneDX/SPDX)
  scan         Scan for known vulnerabilities
  drift        Detect drift from origin
  license      Check license compliance
  audit        Run all checks (verify + scan + license + drift)
  graph        Generate dependency graph
  metrics      Show extraction/migration metrics
  compliance   Generate compliance evidence reports
  version      Show version information
```

### 9.2 Output Contract

Every command that produces data must support:
- `--format table` (default, human-readable)
- `--format json` (machine-readable, for piping and CI)
- `--output <file>` (write to file instead of stdout)
- Consistent exit codes: 0 = success, 1 = failure/findings, 2 = warnings

### 9.3 Configuration Hierarchy

```
1. Command-line flags (highest priority)
2. Environment variables (GIT_VENDOR_*)
3. Project config file (.git-vendor.yml in repo root)
4. User config file (~/.config/git-vendor/config.yml)
5. Built-in defaults (lowest priority)
```

### 9.4 Caching Strategy

- Cache directory: `.git-vendor-cache/` (gitignored)
- CVE scan results: cached 24 hours
- Upstream repo state (for drift): cached 1 hour
- SBOM generation: no cache (always reflects current lockfile)
- Cache is always optional — every command works without it (just slower)

### 9.5 Error Handling

**User-facing errors** should follow this display format:
```
Error: <what went wrong>
  Context: <relevant details>
  Fix: <what the user should do>
```

Example:
```
Error: Cannot scan dependency "utils" for vulnerabilities
  Context: No version tag found; commit abc123 does not map to a known package version
  Fix: Tag the source repository with a semver version, then run `git vendor sync utils`
```

**Implementation follows idiomatic Go conventions:**

1. **Use `fmt.Errorf`** for most errors (the default):
   - Informational errors that are logged or displayed
   - Wrapping underlying errors with context (`fmt.Errorf("failed to sync: %w", err)`)
   - Internal errors where callers don't need to branch on error type

2. **Use sentinel errors** (`errors.New`) when callers need `errors.Is()`:
   - `ErrNotInitialized` — vendor directory doesn't exist
   - `ErrComplianceFailed` — license compliance check failed

3. **Use custom error types** when callers need `errors.As()` or structured data:
   - `VendorNotFoundError` — vendor name doesn't exist in config
   - `GroupNotFoundError` — group tag doesn't exist
   - `PathNotFoundError` — source path doesn't exist in repo
   - `StaleCommitError` — locked commit was deleted/force-pushed
   - `CheckoutError` — git checkout failed (wraps underlying cause)
   - `ValidationError` — configuration validation failed

Custom types implement the Error/Context/Fix format in their `Error()` method. Helper functions (`IsVendorNotFound()`, etc.) provide convenient type checking.

**Location:** `internal/core/errors.go` and `internal/core/constants.go`

---

## 10. Testing Strategy

### 10.1 Unit Tests
- Every function that transforms data has unit tests
- Lockfile parsing/serialization is exhaustively tested
- License detection has tests for every SPDX license we claim to detect

### 10.2 Integration Tests
- Test fixtures: local Git repos in `testdata/` created by test setup
- Every command has at least one integration test that exercises the full flow
- "Golden file" tests for output formats (JSON, Mermaid, HTML) — compare against known-good output

### 10.3 End-to-End Tests
- Script that installs git-vendor from scratch, vendors a real public dependency, runs all commands, and verifies output
- Runs in CI on every release

### 10.4 Compatibility Tests
- Test against Go 1.21+ (minimum supported version)
- Test on Linux, macOS, Windows
- Test with Git 2.30+ (minimum supported version)

---

## 11. Success Metrics

### Month 2 Checkpoint
- [ ] All Phase 1 features shipped
- [ ] ≥80% test coverage
- [ ] README rewritten and compelling
- [ ] CI/CD pipeline running on all PRs
- [ ] 0 known bugs in existing commands

### Month 4 Checkpoint
- [ ] SBOM generation shipped and validated
- [ ] CVE scanning shipped and working against OSV.dev
- [ ] Drift detection shipped
- [ ] License policy enforcement shipped
- [ ] ≥200 GitHub stars

### Month 6 Checkpoint
- [ ] All Phase 3 features shipped
- [ ] GitHub Action published
- [ ] At least 2 blog posts published
- [ ] ≥500 GitHub stars
- [ ] At least 1 "Show HN" submission
- [ ] SBOM output validated against Dependency-Track and Grype
- [ ] Compliance evidence tested against at least one GRC platform

### Stretch Goals (Month 6+)
- [ ] 2,000+ GitHub stars
- [ ] Inclusion in Awesome Go list
- [ ] At least 1 external contributor (non-maintainer PR merged)
- [ ] Mentioned in a supply chain security blog post or conference talk by someone other than the maintainer

---

## 12. Dependency Map

This shows which features depend on which. Build order must respect these dependencies.

```
Feature 1.1 (Schema Versioning)
  └── Feature 1.3 (Metadata Enrichment)
       ├── Feature 2.1 (SBOM Generation)
       ├── Feature 2.2 (CVE Scanning)
       ├── Feature 2.4 (License Policy)
       ├── Feature 3.2 (Dependency Graph)
       └── Feature 3.5 (Migration Metrics)

Feature 1.2 (Verify Hardening)
  └── Feature 2.3 (Drift Detection)
       └── Feature 3.5 (Migration Metrics)

Feature 1.4 (Test Suite)
  └── [all subsequent features benefit]

Feature 1.5 (Documentation)
  └── Feature 3.3 (GitHub Action — needs docs for usage)

Features 2.1 + 2.2 + 2.3 + 2.4
  └── Feature 3.1 (Audit Command — aggregates all four)
       └── Feature 3.3 (GitHub Action — runs audit)
       └── Feature 3.4 (Compliance Reports — needs all data)
```

**Critical path:** 1.1 → 1.3 → 2.1 → 2.2 → 3.1 → 3.3

---

## 13. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| OSV.dev API changes or becomes unreliable | Low | Medium | Abstract API client behind interface; support fallback to NVD or GitHub Advisory DB |
| CycloneDX/SPDX spec changes significantly | Low | Low | Pin to specific spec version; update on next major release |
| Lockfile migration breaks existing users | Medium | High | Automated migration command; never change existing field semantics; only add fields |
| GitHub Action marketplace competition | Medium | Low | Differentiation is git-vendor itself, not the action; action is just distribution |
| Low adoption despite quality | Medium | High | Double down on content marketing; target specific communities (InnerSource, monolith decomposition); consider conference talks |
| Feature scope creep beyond solo maintainer capacity | High | High | Strict adherence to this roadmap; say no to feature requests that aren't in here; defer to OPPORTUNITIES.md for revenue features |
| License detection false positives/negatives | Medium | Medium | Conservative defaults (unknown = warn); let users override via policy file; document limitations clearly |

---

## 14. Post-Foundation Forward References

These are NOT part of the 6-month plan. They are referenced here so that architectural decisions made during months 1–6 don't accidentally preclude them.

### 14.1 Revenue Features (See OPPORTUNITIES.md)
- Org-wide vendoring dashboard (SaaS)
- Vendored code vulnerability propagation scanning (SaaS)
- Monolith decomposition operations tracking (SaaS)
- Internal code reuse intelligence (SaaS)

### 14.2 Advanced Provenance
- Sigstore/cosign integration for cryptographic signing of lockfiles
- SLSA provenance attestation for vendored code
- Transparency log for vendoring operations

### 14.3 Extensibility
- Plugin system for custom scanners, formatters, and policy engines
- Hook system for pre-add, post-sync, pre-verify events
- API mode (`git vendor serve`) for integration with other tools

### 14.4 Enterprise Features
- Multi-repo lockfile aggregation
- Role-based access control for vendoring operations
- Approval workflows for adding new dependencies
- Integration with LDAP/SSO for identity in provenance records

**Architectural implication:** Design all lockfile extensions, output formats, and command interfaces so that a future SaaS product can consume them via API without requiring changes to the CLI. The CLI should be a perfect offline-first client that a SaaS product wraps, not a reduced-functionality version of a SaaS product.

---

## Appendix A: Effort Summary

| Phase | Features | Total Estimated Days | Calendar Months |
|---|---|---|---|
| Phase 1 | 5 features | 22–32 days | ~1.5–2 months |
| Phase 2 | 4 features | 27–38 days | ~2–2.5 months |
| Phase 3 | 5 features | 32–45 days | ~2–3 months |
| Phase 4 | Ongoing | Parallel | Continuous |
| **Total** | **14 features** | **81–115 days** | **~5.5–7.5 months** |

Assumes solo developer working ~75% capacity on git-vendor (accounting for other obligations). With AI-assisted development (as demonstrated by git-vendor's current codebase), implementation time can be compressed by 30–50%.

---

## Appendix B: Version Milestones

| Version | Contains | Approximate Date |
|---|---|---|
| v1.1.0 | Schema versioning, verify hardening, metadata enrichment | Month 2 |
| v1.2.0 | SBOM generation, CVE scanning | Month 3 |
| v1.3.0 | Drift detection, license policy | Month 4 |
| v1.4.0 | Audit command, dependency graph | Month 5 |
| v1.5.0 | GitHub Action, compliance reports, migration metrics | Month 6 |
| v2.0.0 | Reserved for any future breaking changes | TBD |

---

*This document is the single source of truth for git-vendor development priorities. When in doubt, check this document. When this document is silent, make the decision that best serves the four stakeholder groups defined in Section 3.*
