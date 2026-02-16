---
description: Prevents editing files vendored from upstream repos via git-vendor
globs: [".claude/**/*", ".githooks/**/*", "docs/COMMIT-SCHEMA.md", "docs/COMMIT-COOKBOOK.md"]
alwaysApply: false
---

# Vendored File Protection

Files matching this rule's globs are likely vendored from git-ecosystem via git-vendor. Check `.git-vendor/vendor.lock` before editing.

## How to Check

Read `.git-vendor/vendor.lock` and look at `file_hashes` keys under each vendor entry. If the file you're about to edit appears there, it is vendored.

## If a File is Vendored

1. **DO NOT edit it locally.** Local edits will be overwritten on next `git-vendor sync`.
2. **Edit the source repo instead.** The vendor name in `vendor.lock` tells you where to make the change.
3. **Then sync downstream:** `git-vendor sync <vendor-name>` pulls the updated version.

## Exceptions

- `CLAUDE.md` is never vendored — always safe to edit locally.
- `PROJECT_TASK.md` is never vendored — always safe to edit locally.
- `.claude/settings.json` is never vendored — project-specific configuration.
- If `.git-vendor/vendor.lock` does not exist, this project doesn't use git-vendor. Ignore this rule.
