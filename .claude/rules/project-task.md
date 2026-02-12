---
description: Fires on every conversation. Reads PROJECT_TASK.md for the current assignment.
globs: "*"
---

# Project Task Assignment

Check `PROJECT_TASK.md` in the project root at session start. If it exists:

1. Read the file — the active task (frontmatter `status: active`) is your current assignment
2. Work toward the acceptance criteria
3. Append timestamped progress notes to `## Notes` as you make meaningful progress
4. When all acceptance criteria are met, set frontmatter `status: complete` and update the `updated` date

If `status: blocked`, read the Notes section for blocker context and attempt to resolve it. If you cannot, document what you tried in Notes.

Do NOT modify `## Pending Tasks` — that backlog is managed by the ecosystem manager.

## PROJECT_TASK.md Format

```yaml
---
status: active | complete | blocked
assigned: YYYY-MM-DD
updated: YYYY-MM-DD
project: <project-name>
---
```

- `# Current Task` — title on the next non-empty line
- `## Objective` — what to accomplish
- `## Acceptance Criteria` — checkboxes (`- [ ]` / `- [x]`)
- `## Notes` — append-only progress log
- `## Pending Tasks` — priority-ordered backlog (manager-managed, do not modify)
