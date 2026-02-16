---
name: nominate
description: Leave a structured nomination for the project owner — ideas, findings, or cross-project observations discovered during a session
---

# Nominate — Agent-to-Owner Idea Inbox

Use this skill when you discover something worth surfacing to the project owner but it's outside the scope of your current task. Nominations persist across sessions in `.claude/nominations.md`.

## When to Nominate

- A refactoring opportunity that would benefit the project but isn't your current task
- A cross-project pattern that should be shared (e.g., "pattern X in git-vendor would benefit git-plumbing")
- A bug or inconsistency you noticed but couldn't fix without scope creep
- A tooling improvement, missing test, or documentation gap
- A dependency that looks outdated or risky

## How to Nominate

Append a nomination entry to `.claude/nominations.md` in the project root. Create the file if it doesn't exist.

### Entry Format

```markdown
## <short title>

- **Date**: YYYY-MM-DD
- **Source**: <project>/<session-context>
- **Priority**: low | medium | high
- **Category**: refactor | bug | dx | cross-project | tooling | docs | security | test

<1-3 sentences describing what you found and why it matters.>
```

### Rules

1. MUST use the exact format above — the `/manager-review` skill parses these entries.
2. MUST NOT duplicate an existing nomination. Read the file first and check.
3. MUST NOT remove or edit existing nominations — only append new ones.
4. Keep descriptions factual and actionable. No hedging.
5. Priority guide:
   - **high**: Blocks other work or is a correctness issue
   - **medium**: Improves DX, quality, or performance noticeably
   - **low**: Nice-to-have, cosmetic, or speculative
6. Cross-project nominations go in the SOURCE project's file with `cross-project` category.

### Example

```markdown
## Go vendor cache can mask broken builds

- **Date**: 2026-02-16
- **Source**: git-vendor/pre-req-fix
- **Priority**: high
- **Category**: bug

When `pkg/git-plumbing/` is updated via git-vendor but `go mod vendor` isn't re-run, Go builds from the stale `vendor/` cache. Builds pass but use old code. Need a pre-commit check comparing the two directories.
```

## Nomination Lifecycle

1. Agent appends nomination during or at end of session
2. `/manager-review` reads nominations during ecosystem health check
3. Owner reviews, triages into `PROJECT_TASK.md` batches or dismisses
4. Owner deletes processed nominations (or marks with `<!-- reviewed -->`)

## File Location

Always `.claude/nominations.md` in the project root. This file is NOT vendored — each project maintains its own nomination inbox.
