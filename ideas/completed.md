# Completed Ideas

> Archive of completed feature ideas for git-vendor. Items with specs have implementation docs in `ideas/specs/complete/`.

## Feature Ideas

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| 001 | P0 | 2026-02-04 | Lockfile Schema Versioning | Added `schema_version` field with backward compatibility | [spec](specs/complete/001-lockfile-schema-versioning.md) |
| 002 | P0 | 2026-02-04 | Verify Command Hardening | Bulletproof integrity check: detect modified, added, deleted files with JSON output | [spec](specs/complete/002-verify-command-hardening.md) |
| 003 | P0 | 2026-02-05 | Lockfile Metadata Enrichment | Added license_spdx, source_version_tag, vendored_at, vendored_by, last_synced_at to lock entries | [spec](specs/complete/003-lockfile-metadata-enrichment.md) |
| 010 | P0 | 2026-02-05 | SBOM Generation | `git vendor sbom` - Generate CycloneDX 1.5 and SPDX 2.3 format SBOMs from lockfile | [spec](specs/complete/010-sbom-generation.md) |
| 011 | P0 | 2026-02-05 | CVE/Vulnerability Scanning | `git vendor scan` - Query OSV.dev for known CVEs with caching, JSON output, --fail-on threshold | [spec](specs/complete/011-cve-vulnerability-scanning.md) |
| 005 | P0 | 2026-02-09 | Documentation Overhaul | README rewritten, docs/ with COMMANDS.md, CI_CD.md, SECURITY.md, ARCHITECTURE.md, and 12+ guides | - |
| 012 | P0 | 2026-02-09 | Drift Detection | `git vendor drift` - Three-way drift detection (local/upstream/conflict risk) with LCS-based diff | - |
| 013 | P0 | 2026-02-09 | License Policy Enforcement | `git vendor license` - Configurable .git-vendor-policy.yml with allow/deny/warn lists | - |
| 071 | P1 | 2026-02-08 | Position Extraction | Line/column range extraction in path mappings (from: file:L5-L20), 94 tests | [spec](specs/in-progress/071-position-extraction.md) |
| 072 | P1 | 2026-02-09 | LLM-Friendly CLI | Non-interactive CLI commands (create, rename, add-mapping, etc.) with JSON output, 53 tests | [spec](specs/in-progress/072-llm-friendly-cli.md) |

---

## Security Issues

> Completed security vulnerabilities and hardening tasks. See `ideas/security.md` for pending items.

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| SEC-001 | CRITICAL | 2026-02-09 | Path Traversal Audit | RootedFileSystem with ValidateWritePath, comprehensive tests in filesystem_test.go | - |
| SEC-010 | HIGH | 2026-02-09 | Git Command Injection Review | All git commands delegated to git-plumbing, no exec.Command in git-vendor | - |
| SEC-011 | HIGH | 2026-02-09 | URL Validation Hardening | ValidateVendorURL rejects file://, ftp://, javascript:, data: schemes | - |
| SEC-012 | HIGH | 2026-02-09 | Hook Execution Security | sanitizeEnvValue, 5-min timeout, docs/HOOK_THREAT_MODEL.md | - |
| SEC-013 | HIGH | 2026-02-09 | Credential Exposure Prevention | SanitizeURL strips credentials from error messages | - |
| SEC-020 | MEDIUM | 2026-02-09 | YAML Parsing Limits | 1 MB size limit (maxYAMLFileSize) in YAMLStore.Load | - |
| SEC-021 | MEDIUM | 2026-02-09 | Temp Directory Cleanup | defer-based cleanup verified in security_hardening_test.go | - |
| SEC-022 | MEDIUM | 2026-02-09 | Symlink Handling | CopyDir does not follow directory symlinks, symlink detection tests | - |
| SEC-023 | MEDIUM | 2026-02-09 | Binary File Detection | IsBinaryContent exported, null-byte heuristic (first 8000 bytes) | - |
| SEC-030 | LOW | 2026-02-09 | Security Documentation | SECURITY.md at project root, docs/HOOK_THREAT_MODEL.md | - |

---

## Code Quality Issues

> Completed code quality improvements and technical debt items. See `ideas/code_quality.md` for pending items.

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| CQ-002 | HIGH | 2026-02-09 | Error Wrapping Consistency | fmt.Errorf with %w across all services for error chain inspection | - |
| CQ-006 | HIGH | 2026-02-10 | Configurable OSV Endpoint | GIT_VENDOR_OSV_ENDPOINT env var, context-aware Scan signature | - |

---

## Research

> Completed research topics. See `ideas/research.md` for pending items.

| ID | Priority | Completed | Title | Brief | Output |
|----|----------|-----------|-------|-------|--------|
| - | - | - | *No completed research yet* | - | - |

---

## Completion Guidelines

When moving items here from other queues:

1. **Update the source queue** - Change status to `completed`
2. **Add to this file** - Copy row with completion date
3. **Move spec file** - Move from `in-progress/` to `complete/`
4. **Update CLAUDE.md** - If feature affects user-facing behavior
5. **Add completion notes** - Document any important implementation details

### Completion Entry Template

```markdown
| 001 | HIGH | 2026-02-04 | Lockfile Schema Versioning | Added schema_version field with backward compat | [spec](specs/complete/001-lockfile-schema.md) |
```
