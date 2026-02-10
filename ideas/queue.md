# Ideas Queue

> Feature ideas for git-vendor organized by roadmap phase. Items with `[spec]` have detailed implementation specs in `ideas/specs/in-progress/`.

## Phase 1: Foundation Hardening (P0)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 001 | completed | Lockfile Schema Versioning | Add `schema_version` field to vendor.lock, handle version compatibility | [spec](specs/complete/001-lockfile-schema-versioning.md) |
| 002 | completed | Verify Command Hardening | Bulletproof integrity check: detect modified, added, deleted files with JSON output | [spec](specs/complete/002-verify-command-hardening.md) |
| 003 | completed | Lockfile Metadata Enrichment | Add license_spdx, source_version_tag, vendored_at, vendored_by to lock entries | [spec](specs/complete/003-lockfile-metadata-enrichment.md) |
| 004 | pending | Comprehensive Test Suite | Achieve ≥80% coverage with integration tests for all commands | - |
| 005 | pending | Documentation Overhaul | Rewrite README, create docs/ with COMMANDS.md, CI_CD.md, SECURITY.md | - |

## Phase 2: Supply Chain Intelligence (P0)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 010 | completed | SBOM Generation | `git vendor sbom` - Generate CycloneDX and SPDX format SBOMs from lockfile | [spec](specs/complete/010-sbom-generation.md) |
| 011 | completed | CVE/Vulnerability Scanning | `git vendor scan` - Query OSV.dev API for known vulnerabilities | [spec](specs/complete/011-cve-vulnerability-scanning.md) |
| 012 | pending | Drift Detection | `git vendor drift` - Compare vendored files against origin, detect local and upstream changes | - |
| 013 | pending | License Policy Enforcement | `git vendor license` - Configurable policy file with allow/deny/warn lists | - |

## Phase 3: Ecosystem Integration (P1)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 020 | pending | Unified Audit Command | `git vendor audit` - Run verify + scan + license + drift, produce HTML/JSON report | - |
| 021 | pending | Dependency Graph Visualization | `git vendor graph` - Generate Mermaid, DOT, HTML, JSON dependency graphs | - |
| 022 | pending | GitHub Action | git-vendor-action for CI/CD with PR comments, check status, SBOM artifacts | - |
| 023 | pending | Compliance Evidence Reports | `git vendor compliance` - Generate EO 14028, NIST, DORA, CRA, SOC 2 evidence docs | - |
| 024 | pending | Migration Metrics | `git vendor metrics` - Track extraction progress for monolith decomposition | - |
| 070 | pending | Internal Project Compliance | `source: internal` vendors for intra-repo file sync with transforms, CI enforcement | [spec](specs/in-progress/070-internal-compliance.md) |
| 075 | pending | Vendor Compliance Modes | Global + per-vendor compliance levels (strict/lenient/info), enforcement hooks, override mode | [spec](specs/in-progress/075-vendor-compliance-modes.md) |

## Phase 4: Developer & LLM Experience (P1)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 071 | pending | Position Extraction | Line/column range extraction in path mappings (from: file:L5-L20) | [spec](specs/in-progress/071-position-extraction.md) |
| 072 | pending | LLM-Friendly CLI | CLI commands for vendor management without editing YAML, JSON output | [spec](specs/in-progress/072-llm-friendly-cli.md) |
| 073 | pending | Vendor Variables + File Watcher | $v{name:L5-L20} inline syntax with auto-expansion on save | [spec](specs/in-progress/073-vendor-variables.md) |
| 074 | pending | VS Code Extension | IDE integration: variable styling, hover preview, autocomplete, integrated watch | [spec](specs/in-progress/074-vscode-extension.md) |

## MEDIUM Priority (P1)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 030 | pending | Output Format Standardization | Ensure all commands support --format table|json and --output file flags | - |
| 031 | pending | Configuration Hierarchy | Implement CLI flags > env vars > project config > user config > defaults | - |
| 032 | pending | Caching Strategy | Implement .git-vendor-cache/ for CVE scan results and upstream repo state | - |
| 033 | pending | Error Message Standardization | Implement Error/Context/Fix format for all user-facing errors | - |
| 034 | pending | Man Pages | Generate man pages for all commands using go-md2man or similar | - |

## LOW Priority (P2)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 040 | pending | Sigstore/Cosign Integration | Cryptographic signing of lockfiles for advanced provenance | - |
| 041 | pending | Plugin/Hook System | Pre-add, post-sync, pre-verify event hooks for extensibility | - |
| 042 | pending | API Mode | `git vendor serve` for integration with other tools | - |
| 043 | pending | Multi-Repo Lockfile Aggregation | `git-vendor compare` + commit trailer syntax for cross-project vendor consistency | [spec](specs/in-progress/043-multi-repo-lockfile-aggregation.md) |
| 044 | pending | GitLab CI Template | Provide .gitlab-ci.yml snippet for GitLab users | - |

## Backlog (Unprioritized)

| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| 050 | pending | Dependency-Track Integration | Ensure SBOM output works seamlessly with Dependency-Track | - |
| 051 | pending | Grype/Trivy Integration | Test and document integration with popular vulnerability scanners | - |
| 052 | pending | GRC Platform Integration | Document RegScale, Drata, Vanta compliance evidence ingestion | - |
| 053 | pending | Awesome Go Submission | Prepare and submit to Awesome Go list once features mature | - |
| 054 | pending | Show HN Post | Prepare Show HN submission for SBOM or CVE scanning release | - |
| 055 | pending | Lockfile Migration Command | `git vendor migrate` - Add missing metadata to existing lockfiles | - |
| 056 | pending | Golden File Tests | Output format tests comparing against known-good JSON/Mermaid/HTML | - |
| 057 | pending | E2E Test Script | Install from scratch, vendor real dep, run all commands, verify output | - |
| 058 | pending | Compatibility Test Matrix | Test against Go 1.21+, Git 2.30+, Linux/macOS/Windows | - |
| 059 | pending | Supplier Extraction Enhancement | Improve ExtractSupplier heuristics to better match organizational structure from URLs | - |
| 060 | pending | Configurable OSV Batch Size | Make maxBatchSize configurable or dynamically detected for OSV.dev API changes | - |

---

## Priority Reference

| Priority | Meaning | Source |
|----------|---------|--------|
| **P0** | Must have - regulatory/security requirement | ROADMAP Section 2.2 |
| **P1** | Should have - adoption driver | ROADMAP Section 2.2 |
| **P2** | Nice to have - extensibility | ROADMAP Section 14 |

## Dependency Map

Per ROADMAP.md Section 12:

```
001 (Schema Versioning)
  └── 003 (Metadata Enrichment)
       ├── 010 (SBOM Generation)
       ├── 011 (CVE Scanning)
       ├── 013 (License Policy)
       ├── 021 (Dependency Graph)
       └── 024 (Migration Metrics)

002 (Verify Hardening)
  ├── 012 (Drift Detection)
  │    └── 024 (Migration Metrics)
  └── 075 (Vendor Compliance Modes)
       ├── 070 (Internal Compliance)
       ├── 022 (GitHub Action)
       └── 023 (Compliance Reports)

010 + 011 + 012 + 013
  └── 020 (Audit Command)
       ├── 022 (GitHub Action)
       └── 023 (Compliance Reports)

071 (Position Extraction)
  ├── 072 (LLM-Friendly CLI)
  └── 073 (Vendor Variables + File Watcher)
       └── 074 (VS Code Extension)
```

**Critical Path:** 001 → 003 → 010 → 011 → 020 → 022

**LLM Experience Path:** 071 → 073 → 074
