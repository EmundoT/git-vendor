# Krillin Counters

Every time an error is committed and audited, it gets tallied here. Named after the character who dies the most but always comes back — these are the errors that keep happening until we stop letting them.

Two boards track different failure modes: **agent misbehavior** (the agent did something wrong independent of instructions) and **project compliance failures** (instructions existed but were not followed). Counters are sorted lowest-first. If no existing error fits a new finding, add a new category. Keep categories general — no one-off entries.

---

## Board 1: Agent Misbehavior

Errors caused by the agent acting outside its role, misusing tools, or producing incorrect output independent of user instructions.

| # | Count | Name | Description | Last Changed |
|---|-------|------|-------------|--------------|
| 1 | 0 | Context Amnesia | Agent lost track of details established earlier in the conversation | — |
| 2 | 0 | Phantom Completion | Agent marked a task complete without finishing it | — |
| 3 | 0 | Unsolicited Refactor | Agent modified code or structure beyond what was requested | — |
| 4 | 0 | Silent Assumption | Agent made a design decision without surfacing it to the user | — |
| 5 | 0 | Tool Misfire | Agent used the wrong tool or used a tool incorrectly for the task | — |
| 6 | 0 | Hallucinated Reference | Agent referenced a file, function, or concept that does not exist | — |
| 7 | 0 | Overcorrection | Agent applied a fix that was more invasive than necessary | — |

---

## Board 2: Project Compliance Failures

Errors where instructions (explicit or implicit) existed but were not followed during execution.

| # | Count | Name | Description | Last Changed |
|---|-------|------|-------------|--------------|
| 1 | 0 | Silent Drop | A requirement was accepted but never addressed or acknowledged again | — |
| 2 | 0 | Scope Drift | Implementation expanded or contracted beyond the stated requirement | — |
| 3 | 0 | Plan Divergence | A plan was agreed upon but execution deviated without discussion | — |
| 4 | 0 | Convention Violation | Project conventions (CLAUDE.md, commit style, etc.) were not followed | — |
| 5 | 0 | Tracking Skip | Required updates to tracking files (ideas/, queue, etc.) were skipped | — |
| 6 | 0 | Test Gap | Implementation was delivered without required test coverage | — |
| 7 | 0 | Doc Drift | Documentation was not updated to reflect changes made | — |
