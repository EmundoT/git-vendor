# Completed Ideas

> Archive of completed feature ideas for git-vendor. Items with specs have implementation docs in `ideas/specs/complete/`.

## Feature Ideas

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| 001 | P0 | 2026-02-04 | Lockfile Schema Versioning | Added `schema_version` field with backward compatibility | [spec](specs/complete/001-lockfile-schema-versioning.md) |
| 002 | P0 | 2026-02-04 | Verify Command Hardening | Bulletproof integrity check: detect modified, added, deleted files with JSON output | [spec](specs/complete/002-verify-command-hardening.md) |
| 003 | P0 | 2026-02-05 | Lockfile Metadata Enrichment | Added license_spdx, source_version_tag, vendored_at, vendored_by, last_synced_at to lock entries | [spec](specs/complete/003-lockfile-metadata-enrichment.md) |
| 011 | P0 | 2026-02-05 | CVE/Vulnerability Scanning | `git vendor scan` - Query OSV.dev for known CVEs with caching, JSON output, --fail-on threshold | [spec](specs/complete/011-cve-vulnerability-scanning.md) |

---

## Security Issues

> Completed security vulnerabilities and hardening tasks. See `ideas/security.md` for pending items.

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| - | - | - | *No completed security issues yet* | - | - |

---

## Code Quality Issues

> Completed code quality improvements and technical debt items. See `ideas/code_quality.md` for pending items.

| ID | Priority | Completed | Title | Brief | Spec |
|----|----------|-----------|-------|-------|------|
| - | - | - | *No completed code quality issues yet* | - | - |

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
