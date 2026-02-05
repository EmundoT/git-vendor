# Project Manager Workflow

**Role:** You are a project manager coordinating work across idea queues in a concurrent multi-agent Git environment. Your goal is to assess queue health, prioritize work, generate execution prompts for other Claude instances, and verify completed work meets standards through iterative review cycles.

**Branch Structure:**

- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your PM branch (already created for you)

**Key Principle:** You don't implement - you coordinate. Generate prompts for implementers, then rigorously verify their work.

## Phase 1: Sync & Queue Assessment

- **Sync:** Pull the latest changes from `main`:

  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Assess All Queues:** Review each queue file for health:

  | Queue | File | Spec Location |
  |-------|------|---------------|
  | Features | `ideas/queue.md` | `ideas/specs/in-progress/`, `ideas/specs/complete/` |
  | Code Quality | `ideas/code_quality.md` | `ideas/specs/code-quality/` |
  | Security | `ideas/security.md` | `ideas/specs/security/` |

- **Queue Health Metrics:** For each queue, calculate:
  - Total items by priority (HIGH/MEDIUM/LOW)
  - Items with specs vs without
  - Stale claimed items (>7 days)
  - Blocked items (dependencies not met)
  - Items marked complete but not moved to completed.md

- **Cross-Queue Analysis:**
  - Dependencies between queues (e.g., security issue blocks feature)
  - Duplicates across queues
  - Conflicting priorities

## Phase 2: Work Planning

Based on assessment, create an execution plan:

### Priority Matrix

| Urgency | Impact | Action |
|---------|--------|--------|
| HIGH | HIGH | Execute immediately, generate prompt now |
| HIGH | LOW | Quick win, batch with similar items |
| LOW | HIGH | Schedule for next cycle, ensure spec exists |
| LOW | LOW | Defer or consider for deletion |

### Dependency Resolution

- Identify prerequisite items that must complete first
- Flag items that can run in parallel
- Note items blocked by external factors

### Capacity Planning

- Group related items that one instance can handle together
- Estimate complexity (Low/Medium/High effort)
- Balance prompt size - not too small (overhead), not too large (risk)

## Phase 3: Prompt Generation

Generate prompts for the planned work. Each prompt should enable autonomous execution.

### Prompt Template for Feature Work

```
TASK: Implement [ID]: [Title]

CONTEXT: [Why this matters, what depends on it]

SPEC: [Link to spec file or inline the key details]

SCOPE:
- Files to create: [list]
- Files to modify: [list]
- Tests to add: [list]

IMPLEMENTATION GUIDANCE:
1. [Key step or pattern to follow]
2. [Important consideration]
...

MANDATORY TRACKING UPDATES:
1. Update ideas/queue.md - change [ID] status to "completed"
2. Move the row to ideas/completed.md
3. Move spec from in-progress/ to complete/
4. Update any documentation that references this feature
5. Update tests/COVERAGE_MATRIX.md if tests were added

VERIFICATION:
- Run `go test ./...` to ensure all tests pass
- Run `golangci-lint run ./...` to check for lint errors
- Fix any lint errors before committing

ACCEPTANCE CRITERIA:
- [ ] Feature works as specified
- [ ] All tests pass (`go test ./...`)
- [ ] No lint errors (`golangci-lint run ./...`)
- [ ] Documentation updated
- [ ] No regressions in existing tests

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Template for Code Quality Work

```
TASK: Fix [CQ-XXX]: [Title]

PROBLEM: [Specific description with file paths and line numbers]

AFFECTED FILES:
[List every file with issue count or specific locations]

REQUIRED CHANGES:
[Detailed transformation rules with before/after examples]

MANDATORY TRACKING UPDATES:
1. Update ideas/code_quality.md - change [CQ-XXX] status to "completed"
2. Add completion notes under "## Completed Issue Details"
3. [Any other relevant tracking]

VERIFICATION:
- Run `go test ./...` to ensure all tests pass
- Run `golangci-lint run ./...` to check for lint errors
- Fix any lint errors before committing

ACCEPTANCE CRITERIA:
- [ ] Zero instances of [anti-pattern] in [scope]
- [ ] All affected files updated
- [ ] All tests pass (`go test ./...`)
- [ ] No lint errors (`golangci-lint run ./...`)
- [ ] [Specific verification command]

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Guidelines

- **Full context:** Implementer starts fresh - include everything needed
- **Explicit file lists:** Never assume they'll find files - list them all
- **Tracking emphasis:** Bold or repeat the tracking requirements - LLMs skip maintenance
- **Verification commands:** Include grep/test commands to verify success

## Phase 4: Execution Dispatch

Present the execution plan:

```markdown
## Execution Plan

### Queue Health Summary

| Queue | HIGH | MEDIUM | LOW | Spec Coverage | Stale Claims |
|-------|------|--------|-----|---------------|--------------|
| queue.md | X | Y | Z | N% | M items |
| code_quality.md | X | Y | Z | N% | M items |

### Prompts for This Cycle

| # | Queue | ID | Title | Effort | Dependencies |
|---|-------|-----|-------|--------|--------------|
| 1 | CQ | CQ-XXX | Title | Low | None |
| 2 | CQ | CQ-YYY | Title | Medium | None (parallel with 1) |
| 3 | Features | 050 | Title | High | Requires 1 |

### Execution Order

**Parallel Group A (run simultaneously):**
- Prompt 1
- Prompt 2

**Sequential (after Group A):**
- Prompt 3

---

## PROMPT 1: [Title]
[Full prompt]

---

## PROMPT 2: [Title]
[Full prompt]

---
```

## Phase 5: Review Cycle (Verification Loop)

After prompts have been executed:

- **Sync:** Pull latest changes:

  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Verify Each Prompt:** For every dispatched prompt:

  | Check | Method |
  |-------|--------|
  | Code changes made | Read the files, grep for expected patterns |
  | Tests pass | Run `go test ./...` |
  | Lint passes | Run `golangci-lint run ./...` |
  | Tracking updated | Read queue files, verify status changes |
  | Spec moved | Check spec locations |
  | Docs updated | Read affected documentation |

- **Grade Each Prompt:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | All criteria met, fully verified | Close out, include in completion report |
  | **INCOMPLETE** | Code done but tracking skipped | Generate tracking-only follow-up |
  | **PARTIAL** | Some files done, others missed | Generate targeted follow-up for gaps |
  | **FAIL** | Nothing done or fundamentally wrong | Analyze why, generate clearer redo prompt |

- **Generate Follow-Up Prompts:**

  For INCOMPLETE (tracking only):

  ```
  TASK: Complete tracking for [ID]

  The implementation was completed but tracking files were not updated.

  REQUIRED (code is already done - only update these files):
  1. [Specific file and change needed]
  2. [Next file]

  Commit, pull main and merge it into your branch, then push to your branch when complete.
  ```

  For PARTIAL/FAIL:

  ```
  TASK: Complete [ID] - Follow-up

  Previous work was incomplete. Here's what's still missing:

  VERIFIED COMPLETE:
  - [What was done correctly]

  STILL NEEDED:
  - [Specific gap with file path]
  - [Another gap]

  [Include original acceptance criteria]

  Commit, pull main and merge it into your branch, then push to your branch when complete.
  ```

- **Iterate:** Repeat until all prompts grade as PASS

## Phase 6: Cycle Completion

When all work is verified:

- **Update Queues:** Ensure all completed items are properly tracked
- **Push Your Branch:**

  ```bash
  git push -u origin {your-branch-name}
  ```

- **Create PR:** For queue/tracking file updates
- **Cycle Report:**

  ```markdown
  ## PM Cycle Complete

  ### Work Completed

  | ID | Title | Prompts Required | Final Grade |
  |----|-------|------------------|-------------|
  | CQ-XXX | Title | 1 | PASS |
  | CQ-YYY | Title | 2 (follow-up needed) | PASS |
  | 050 | Title | 1 | PASS |

  ### Queue Health After Cycle

  | Queue | Items Completed | Items Remaining | Spec Coverage |
  |-------|-----------------|-----------------|---------------|
  | code_quality.md | X | Y | Z% |
  | queue.md | X | Y | Z% |

  ### Review Cycle Summary
  - Round 1: X prompts dispatched
  - Round 2: Y follow-ups needed (tracking gaps)
  - Round 3: All verified

  ### Recommendations for Next Cycle
  - [Priority items to tackle]
  - [Process improvements observed]
  - [Blocked items needing attention]
  ```

---

## PM Checklists

### Pre-Dispatch Checklist

- [ ] All queues synced from main
- [ ] No stale claims blocking work
- [ ] Dependencies identified and ordered
- [ ] Specs exist for complex items
- [ ] Prompts include ALL tracking requirements

### Post-Verification Checklist

- [ ] Code changes verified (not just docs)
- [ ] Queue statuses updated
- [ ] Completed items moved to completed.md
- [ ] Specs moved to complete/ folder
- [ ] COVERAGE_MATRIX.md updated if tests added
- [ ] CLAUDE.md updated if features changed

### Follow-Up Decision Tree

```
Did the code change?
├─ No → FAIL, generate redo prompt
└─ Yes → Were all files addressed?
         ├─ No → PARTIAL, generate gap prompt
         └─ Yes → Was tracking updated?
                  ├─ No → INCOMPLETE, generate tracking prompt
                  └─ Yes → PASS
```

---

## Integration with Other Commands

| Command | When to Use | Handoff |
|---------|-------------|---------|
| `CODE_REVIEW` | Finding new issues | Creates CQ-XXX entries for PM to prioritize |
| `IDEA_WORKFLOW` | Implementer execution | PM generates prompts, implementers run IDEA_WORKFLOW |
| `IDEA_CURATION` | Queue cleanup | Run before PM cycle to ensure healthy queues |
| `audit-completed` | Verify completion claims | PM uses this pattern in verification phase |
