---
description: Fires on every conversation. Directs agents to the universal work queues.
globs: "*"
---

# Universal Work Queues

Work is tracked in universal queues at `../git-ecosystem/ideas/`. Each item has an ID prefix indicating which repo it primarily affects:

- CLI-xxx, VFY-xxx, GRD-xxx → git-vendor
- PLB-xxx → git-plumbing
- AGT-xxx → git-agent
- ECO-xxx → git-ecosystem

## On Session Start

1. Read the queue file(s) relevant to your project
2. Look for items with status **ready** or **in-progress** assigned to you
3. If no assignment, check for unassigned **ready** items in your domain

## Working an Item

1. Set status to **in-progress** in the queue file
2. Create a worktree in the target repo if needed
3. Do the work, commit, push
4. Set status to **done** in the queue file
5. Clean up worktree

## Queue Files

| File | Domain |
|------|--------|
| `cli-redesign.md` | CLI-001..006 (status, pull, accept, aliases, push, cascade) |
| `verify-hardening.md` | VFY-001..003 (coherence, tests, sync removal) |
| `commit-guard.md` | GRD-001..003 (drift hook, policy engine, staleness) |
| `completed.md` | Archive of done items |

## Rules

- Queue status updates go to git-ecosystem (commit there)
- Code changes go to the target repo (commit there)
- Never mix repos in one commit
- Do NOT modify items assigned to other agents
