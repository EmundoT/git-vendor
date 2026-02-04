# Git-Controlled Execution Concurrency

**Role:** You are an autonomous software engineer working in a concurrent multi-agent Git environment. Each chat instance operates in its own dedicated worktree branched from `main`. Your goal is to claim, execute, and integrate tasks while maintaining sync with the parent branch.

**Branch Structure:**
- `main` - Parent branch for all agent work (claim locks and final merges)
- `claude/{feature}-{id}` - Your worktree branch (where implementation happens)

**Queue File:** This workflow applies to any idea queue file. If not specified, defaults to `ideas/queue.md`.

Available queue files:
| Queue | ID Prefix | Spec Location |
|-------|-----------|---------------|
| `ideas/queue.md` | 3-digit (001-054) | `ideas/specs/in-progress/`, `ideas/specs/complete/` |
| `ideas/code_quality.md` | CQ-NNN | `ideas/specs/code-quality/` |
| `ideas/security.md` | SEC-NNN | `ideas/specs/security/` |
| `ideas/research.md` | R-NNN | `ideas/research/` |

## Phase 1: Resource Claiming & Upstream Sync

- **Sync:** Pull the latest changes from `main` to ensure your local state is current.
  ```bash
  git fetch origin main
  git merge origin/main
  ```
- **Select Tasks:** Open the specified queue file (default: `ideas/queue.md`). Identify tasks to claim based on the following priority logic:
  - *Default:* Select the top 3 highest-priority unclaimed ideas.
  - *Override:* If specific instructions are provided at the end of this prompt (e.g., "5 randoms", "4 lows"), follow those criteria instead.
- **Claim:** Update the status column of your selected ideas to `claimed:{branch-name}`.
- **Collision Handling:** If a task is already claimed by another agent, immediately select the next available tasks that fit your criteria.
- **Lock:** Commit and push the queue file directly to `main` to notify other concurrent agents:
  ```bash
  git add ideas/{queue-file}.md
  git commit -m "chore: claim tasks [IDs] for {branch-name}"
  git push origin HEAD:main
  ```

## Phase 2: Execution & Implementation

For each claimed idea:

- **Research:** Check `ideas/research.md` and `ideas/research/` folder for related research. Check `docs/ROADMAP.md` for architectural context and guidelines.
- **Spec:** Read the detailed specification from the appropriate spec folder (see table above). If no spec exists, create one from the brief in the queue file.
- **Standard:** Write clean, modular Go code following project conventions:
  - Use dependency injection via interfaces
  - Follow existing patterns in `internal/core/`
  - Handle errors by returning them, not panicking
  - Use gomock for test dependencies
- **Verification:** Include comprehensive tests appropriate for the scope:
  - Unit tests with gomock for interface dependencies
  - Table-driven tests for multiple cases
  - Run `go test -race ./...` to check for races
- **Documentation:** Update relevant docs:
  - Add godoc comments to exported functions
  - Update CLAUDE.md if adding features
  - Update README if user-facing
- **Commit:** Create one Conventional Commit per idea (e.g., `feat: implementation of [idea]`).

## Phase 3: Expansion & Maintenance

- **Brainstorm:** Generate 6 new ideas based on the current project state. Consider roadmap alignment. You may elevate one to high priority if it is a critical next step.
- **Update Registry:**
  - Mark your completed tasks as `completed` in the queue file, then move those rows to `ideas/completed.md`.
  - Move finished specs from `in-progress/` to `complete/` (for queue.md items).
  - Append your 6 new ideas to the queue file with brief descriptions.
  - Create specs in the appropriate spec folder for any high-priority additions.
- **Final Local Commit:** `docs: update ideas queue`

## Phase 4: Integration & Upstream Merge

- **Sync Before Merge:** Pull latest `main` to catch concurrent changes:
  ```bash
  git fetch origin main
  git merge origin/main
  ```
- **Conflict Resolution:**
  - Resolve standard merge conflicts locally (especially queue files which may have concurrent edits).
  - If a major architectural conflict occurs, STOP, abort the merge, and report the conflict analysis to the user.
- **Run Tests:**
  ```bash
  go test ./...
  go test -race ./...
  ```
- **Push Your Branch:** Push your worktree branch with all changes:
  ```bash
  git push -u origin {your-branch-name}
  ```
- **Create PR:** Create a pull request from your branch to `main`.
- **Final Report:** Summarize work completed and nominate the next 3 priority ideas for future agents.

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

---

## git-vendor Implementation Guidelines

When implementing features for git-vendor:

### Code Structure
- Business logic goes in `internal/core/`
- Data types in `internal/types/types.go`
- TUI components in `internal/tui/`
- CLI routing in `main.go`

### Interface Pattern
```go
// Define interface for testability
type SomeService interface {
    DoSomething(ctx context.Context, params Params) error
}

// Implementation
type realSomeService struct {
    git GitClient
    fs  FileSystem
}
```

### Testing Pattern
```go
func TestSomething(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockGit := NewMockGitClient(ctrl)
    mockFS := NewMockFileSystem(ctrl)

    // Set expectations
    mockGit.EXPECT().Clone(...).Return(nil)

    // Test
    svc := NewSomeService(mockGit, mockFS)
    err := svc.DoSomething(...)

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

### Roadmap Alignment
Always check `docs/ROADMAP.md` before implementing:
- Feature dependencies (what must be built first)
- Architectural guidelines (how features should integrate)
- Testing requirements
- Success metrics
