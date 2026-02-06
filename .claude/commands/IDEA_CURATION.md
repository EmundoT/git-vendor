---
disable-model-invocation: true
---

# Ideas Curation Workflow

**Role:** You are an ideas curator working in a concurrent multi-agent Git environment. Each chat instance operates in its own dedicated worktree branched from `main`. Your goal is to maintain idea quality, remove cruft, flesh out promising ideas, and keep the queue healthy.

**Branch Structure:**
- `main` - Parent branch for all agent work (claim locks and final merges)
- `{your-current-branch}` - Your worktree branch (where curation happens), already created for you

**Queue File:** This workflow applies to any idea queue file. If not specified, defaults to `ideas/queue.md`.

Available queue files:
| Queue | ID Prefix | Spec Location |
|-------|-----------|---------------|
| `ideas/queue.md` | 3-digit (001-054) | `ideas/specs/in-progress/`, `ideas/specs/complete/` |
| `ideas/code_quality.md` | CQ-NNN | `ideas/specs/code-quality/` |
| `ideas/security.md` | SEC-NNN | `ideas/specs/security/` |
| `ideas/research.md` | R-NNN | `ideas/research/` |

**Scope Guidance:** Balance breadth vs depth based on idea complexity. For simple cleanup/deletion, cover more ground. For detailed spec creation or priority rebalancing, focus on fewer items with thorough analysis.

## Phase 1: Sync & Audit

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```
- **Audit Queue:** Read the specified queue file and assess each idea for:
  - *Relevance:* Still applicable to current framework state?
  - *Duplication:* Overlaps with other ideas or completed features?
  - *Feasibility:* Technically achievable within reasonable scope?
  - *Value:* Provides meaningful benefit vs implementation cost?
- **Audit Specs:** Verify specs in the appropriate folder match queue entries:
  - Orphan specs (no queue entry)
  - Missing specs (high-priority items without detailed plans)
  - Stale specs (outdated approaches or references)

## Phase 2: Cleanup & Deletion

- **Nominate for Deletion:** Identify ideas that should be removed, present them to the user with reasons:
  - `silly` - Impractical or joke ideas
  - `duplicate` - Covered by another idea or existing feature
  - `deprecated` - Technology or approach no longer relevant
  - `scope-creep` - Far beyond framework's purpose
  - `infeasible` - Technical barriers make it impossible
  - `low-value` - Effort vastly exceeds benefit
- **Delete:** Remove nominated ideas from the queue file.
- **Archive:** If idea has historical value or could be worth revisiting, move to `ideas/specs/archived/` with deletion reason and reasoning at the top of the file.
- **Lock:** Commit deletions with `chore: prune ideas queue - remove [reasons]`.

## Phase 3: Maintenance & Consistency

- **Priority Rebalancing:** Adjust priorities based on:
  - Dependencies (prerequisite ideas should be higher)
  - Recent framework changes (some ideas may be more/less urgent)
  - Community feedback or user requests
- **Status Sync:** Ensure statuses are accurate:
  - Clear stale `claimed:` statuses (older than 7 days with no activity)
  - Mark partially-implemented ideas appropriately
- **Spec Health:** For each spec in the queue's spec folder:
  - Verify "Files to Create/Modify" paths still exist
  - Check "References" are still valid
  - Update code patterns if framework APIs changed
- **Commit:** `chore: ideas queue maintenance`

## Phase 4: Flesh Out Ideas

- **Select:** Choose 2-3 HIGH/MEDIUM priority ideas lacking detailed specs.
- **Research:** Analyze codebase for:
  - Existing patterns to follow
  - Related code that would need modification
  - Potential conflicts with other features
- **Create Specs:** Write detailed specs in the appropriate spec folder following the template:
  ```markdown
  # {ID}: {Title}

  **Status:** Pending
  **Priority:** HIGH/MEDIUM/LOW
  **Effort:** Low/Medium/High

  ## Problem
  [Why this matters]

  ## Solution
  [High-level approach]

  ## Implementation

  ### Files to Create/Modify
  [Specific paths with purposes]

  ### Key Code Patterns
  [SQL/code snippets]

  ### Testing Strategy
  [Validation approach]

  ## Risks & Mitigations
  [Table of risks]

  ## References
  [Relevant DMVs, existing code, docs]
  ```
- **Commit:** `docs: flesh out specs for [idea titles]`

## Phase 5: Integration & Merge

- **Sync Before Merge:** Pull latest `main` to catch concurrent changes:
  ```bash
  git fetch origin main
  git merge origin/main
  ```
- **Verify Claims:** After syncing, re-check the queue file to ensure your claimed tasks weren't also claimed by another agent during your work. If conflicts exist, adjust your work accordingly.
- **Conflict Resolution:**
  - Resolve standard merge conflicts locally (especially queue files which may have concurrent edits).
  - If a major architectural conflict occurs, STOP, abort the merge, and report the conflict analysis to the user.
- **Push Your Branch:** Push your worktree branch with all changes:
  ```bash
  git push -u origin {your-branch-name}
  ```
- **Create PR:** Create a pull request from your branch to `main`:
  - If `gh` CLI is available: `gh pr create --base main --title "..." --body "..."`
  - If `gh` CLI is unavailable: provide the manual PR URL from git push output
- **Report:** Summarize:
  - Ideas deleted and why
  - Priority changes made
  - Specs created or updated
  - Recommendations for next curation cycle

---

## Curation Criteria Reference

### Deletion Thresholds

| Reason | Criteria |
|--------|----------|
| `silly` | No serious use case, joke, or obviously impractical |
| `duplicate` | >70% overlap with another idea or shipped feature |
| `deprecated` | Relies on deprecated Go features or removed APIs |
| `scope-creep` | Would require fundamental redesign or falls outside git-vendor's purpose |
| `infeasible` | Technical limitations make it impossible or impractical |
| `low-value` | HIGH effort + LOW impact quadrant |

### Priority Guidelines

| Priority | Criteria |
|----------|----------|
| HIGH | Core functionality gap, frequently requested, blocks other work |
| MEDIUM | Nice-to-have, improves DX, moderate implementation effort |
| LOW | Edge case, experimental, significant effort for niche benefit |

### Spec Quality Checklist

- [ ] Problem statement is clear and specific
- [ ] Solution is quantifiable or clearly articulated
- [ ] Implementation references actual file paths in codebase
- [ ] Code patterns follow existing framework conventions
- [ ] Testing strategy is concrete and achievable
- [ ] Risks are realistic with practical mitigations

### Report Template

Use this structure for Phase 5 reporting:

```markdown
## Curation Complete - Summary Report

### Queue Health After Curation

| Queue | HIGH | MEDIUM | LOW | With Specs |
|-------|------|--------|-----|------------|
| queue.md | X | X | X | Y% |
| code_quality.md | X | X | X | Y% |
| ... | ... | ... | ... | ... |

### Ideas Pruned

| ID | Title | Reason |
|----|-------|--------|
| XXX-NNN | Title | `reason` |

### Specs Created/Updated

| ID | Title | Action |
|----|-------|--------|
| XXX-NNN | Title | Created/Updated |

### Recommendations for Next Cycle

- [Priority adjustments to consider]
- [Ideas needing specs]
- [Potential deletions to review]
```

---

## Standard Table Format

All queue files use this consistent table format:

```markdown
| ID | Status | Title | Brief | Spec |
|----|--------|-------|-------|------|
| XXX-001 | pending | Short Title | One-line description | [spec](path) or - |
```

**Status Values:**
- `pending` - Not started
- `in_progress` - Being worked on (solo work)
- `claimed:{branch}` - Claimed by agent on branch (concurrent work)
- `completed` - Done, ready to move to completed.md

**ID Conventions:**
- Use the prefix for the queue type (see table above)
- Sequential numbering within each queue
- IDs are permanent once assigned (don't reuse deleted IDs)
