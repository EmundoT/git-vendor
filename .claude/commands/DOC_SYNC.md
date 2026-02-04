# Documentation Sync Workflow

**Role:** You are a documentation analyst working in a concurrent multi-agent Git environment. Your goal is to identify documentation drift (where docs don't match code), generate sync prompts for other Claude instances, and verify documentation accuracy through iterative review cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your doc-sync branch (already created for you)

**Key Principle:** Documentation that's wrong is worse than no documentation. Verify docs match actual behavior, not intended behavior.

## Phase 1: Sync & Documentation Inventory

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Inventory Documentation Sources:**

  | Source | Location | Purpose |
  |--------|----------|---------|
  | CLAUDE.md | Root `CLAUDE.md` | AI context, development guide |
  | README.md | Root `README.md` | Project overview, quick start |
  | ROADMAP.md | `docs/ROADMAP.md` | Development roadmap |
  | Command Docs | `.claude/commands/*.md` | Workflow commands |
  | Code Comments | `internal/**/*.go` | Godoc documentation |
  | CLI Help | `main.go` help text | User-facing help |

- **Extract Command Signatures:**
  ```bash
  # Get actual CLI commands and flags from main.go
  grep -n "case\|flag\." main.go | head -30

  # Get flag definitions
  grep -rn "flag\." main.go
  ```

- **Extract Documented Commands:**
  ```bash
  # Get commands from CLAUDE.md
  grep -E "git-vendor|git vendor" CLAUDE.md | head -20

  # Get commands from README
  grep -E "git-vendor|git vendor" README.md | head -20
  ```

## Phase 2: Drift Detection

Run these drift detection checks:

### 2.1 Command & Flag Drift

| Check | Method |
|-------|--------|
| Missing command docs | Commands in main.go not in CLAUDE.md |
| Orphan command docs | Documented commands that don't exist |
| Flag mismatches | Documented flags differ from actual |
| Default value drift | Documented defaults differ from actual |

```bash
# Extract actual commands
grep -E "case \"" main.go | sed 's/.*case "\([^"]*\)".*/\1/' | sort

# Compare to documented commands
grep -E "^\| `git-vendor" CLAUDE.md | sed 's/.*`git-vendor \([^`]*\)`.*/\1/' | sort
```

### 2.2 Interface Drift

Compare documented interfaces to actual code:

```bash
# Get actual interfaces
grep -rn "type.*interface" internal/core/*.go

# Get documented interfaces
grep -A 5 "Interface" CLAUDE.md
```

### 2.3 Configuration Drift

Compare documented config options to actual:

```bash
# Check vendor.yml structure in types
grep -A 20 "type VendorConfig" internal/types/types.go

# Compare to documented structure
grep -A 20 "vendor.yml" CLAUDE.md
```

### 2.4 Example Code Validity

Check that example code in documentation actually works:
- README.md examples
- CLAUDE.md examples
- Command help examples

```bash
# Test documented commands
./git-vendor --help
./git-vendor list --help
```

## Phase 3: Categorize Drift

| Severity | Criteria | Examples |
|----------|----------|----------|
| **CRITICAL** | Docs describe non-existent functionality | Command removed but still documented |
| **HIGH** | Docs have wrong information | Wrong flags, wrong behavior described |
| **MEDIUM** | Docs incomplete | Missing commands, missing flags |
| **LOW** | Docs could be clearer | Style issues, outdated examples |

## Phase 4: Sync Prompt Generation

### Prompt Template for CLAUDE.md Updates

```
TASK: Sync CLAUDE.md documentation for [feature/section]

DRIFT DETECTED:
- [Specific mismatch description]

ACTUAL CODE STATE:
- File: [path]
- Code: [actual behavior]

DOCUMENTATION STATE:
- Section: [CLAUDE.md section]
- Current text: [current incorrect text]

REQUIRED SYNC:
1. Update section [X] to reflect: [correct info]
2. Add/remove: [specific changes]
3. Update examples to: [working examples]

VERIFICATION:
- Example commands work when executed
- Documentation matches code behavior
- No outdated references

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Template for README Updates

```
TASK: Update README.md - sync with current code state

DRIFT DETECTED:
- [List of inaccuracies]

CURRENT DOC STATE:
- Section: [section name]
- Content: [current incorrect content]

ACTUAL CODE STATE:
- [What the code actually does]
- [Evidence: file paths, grep output]

REQUIRED CHANGES:
1. [Specific change with before/after]
2. [Next change]

VERIFICATION:
- [ ] All code examples work when executed
- [ ] All command names match actual CLI
- [ ] All flags match actual flags
- [ ] Installation instructions work

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

## Phase 5: Documentation Report

```markdown
## Documentation Sync Report

### Drift Summary

| Category | Critical | High | Medium | Low |
|----------|----------|------|--------|-----|
| CLAUDE.md | X | X | X | X |
| README.md | X | X | X | X |
| Command Help | X | X | X | X |
| Code Comments | X | X | X | X |

### Critical Drift (Wrong Information)

| Location | Issue | Actual State |
|----------|-------|--------------|
| [File:Section] | [What's wrong] | [What's correct] |

### Missing Documentation

| Feature | Documented In | Missing From |
|---------|---------------|--------------|
| [Feature] | main.go | CLAUDE.md, README |

### Prompts Generated

---

## PROMPT 1: Sync docs for [feature]
[Full prompt]

---
```

## Phase 6: Verification Cycle

After sync prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Re-Run Drift Detection:**
  - Execute same checks from Phase 2
  - Verify each drift item is resolved

- **Verify Documentation Accuracy:**

  | Check | Method |
  |-------|--------|
  | CLI help | Run `./git-vendor --help` |
  | Example code | Execute documented examples |
  | File references | Check paths exist |
  | Command names | Match actual commands |

- **Grade Each Sync Prompt:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Docs now match code | Complete |
  | **PARTIAL** | Some drift fixed, some remains | Follow-up for remaining |
  | **STYLE-ONLY** | Fixed docs but not accurate | Redo with correct info |
  | **FAIL** | Docs still wrong | Redo prompt |

- **Iterate:** Repeat until all documentation matches code

## Phase 7: Documentation Sign-Off

When all drift is resolved:

- **Final Verification:**
  - Run all drift checks, expect clean
  - Test all examples
  - Verify CLI help matches docs

- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```

- **Sync Report:**

  ```markdown
  ## Documentation Sync Complete

  ### Drift Resolved

  | Category | Items Fixed |
  |----------|-------------|
  | CLAUDE.md | X |
  | README.md | X |
  | Command Help | X |

  ### Verification
  - CLI help: Accurate
  - Examples: All execute successfully
  - References: All paths valid

  ### Recommendations
  - [Process improvements to prevent drift]
  - [Documentation that needs expansion]
  ```

---

## Drift Detection Quick Reference

### Command Documentation Check

```bash
# Extract commands from main.go
grep -E 'case "[a-z]' main.go | sed 's/.*"\([^"]*\)".*/\1/' | sort > /tmp/actual.txt

# Extract documented commands
grep -oE 'git-vendor [a-z]+' README.md | sed 's/git-vendor //' | sort -u > /tmp/docs.txt

# Find differences
diff /tmp/actual.txt /tmp/docs.txt
```

### Flag Documentation Check

```bash
# Extract actual flags
grep -E "flag\.(Bool|String|Int)" main.go | head -20

# Compare to documented flags in CLAUDE.md
grep -E "\-\-[a-z]" CLAUDE.md | head -20
```

### Interface Documentation Check

```bash
# Actual interfaces
grep -rn "type.*interface" internal/core/*.go | wc -l

# Documented interfaces
grep -c "Interface" CLAUDE.md
```

---

## Integration Points

- **CLAUDE.md** - Primary development documentation
- **README.md** - User-facing documentation
- **docs/ROADMAP.md** - Development roadmap
- **main.go** - CLI help text
- **internal/**/*.go** - Code comments / godoc
- **.claude/commands/*.md** - Workflow documentation
