# Dependency Audit Workflow

**Role:** You are a dependency analyst working in a concurrent multi-agent Git environment. Your goal is to map Go package dependencies, identify problematic patterns (circular imports, heavy deps), generate refactoring prompts for other Claude instances, and verify dependency health through iterative review cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your audit branch (already created for you)

**Key Principle:** Dependencies should form a clean DAG (directed acyclic graph). Circular imports cause build failures. Heavy dependencies slow builds and increase attack surface.

## Phase 1: Sync & Dependency Discovery

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **List Direct Dependencies:**
  ```bash
  # View go.mod dependencies
  cat go.mod

  # List all module dependencies
  go list -m all

  # Show dependency graph
  go mod graph
  ```

- **Analyze Import Structure:**
  ```bash
  # Find all imports in each package
  for dir in internal/core internal/tui internal/types internal/version; do
    echo "=== $dir ==="
    grep -rh "^import" $dir/*.go 2>/dev/null | sort -u
  done

  # Find external package imports
  grep -rh "github.com\|golang.org" internal/ | sort -u
  ```

- **Build Dependency Matrix:**

  | Package | Imports From | Imported By |
  |---------|-------------|-------------|
  | internal/core | types, tui | main |
  | internal/tui | types | core |
  | internal/types | (none) | core, tui |

- **Identify Entry Points:**
  Packages that are imported from main.go (public API):
  ```bash
  grep "internal/" main.go | sed 's/.*"\(.*\)"/\1/'
  ```

## Phase 2: Dependency Analysis

### 2.1 Build Import Graph

Create package dependency visualization:

```
main.go
├── internal/core
│   ├── internal/types
│   └── (external deps)
├── internal/tui
│   └── internal/types
├── internal/types
│   └── (none)
└── internal/version
    └── (none)
```

### 2.2 Detect Circular Imports

```bash
# Go will fail to build with circular imports
go build ./...

# Check for potential cycles
go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/...
```

### 2.3 Analyze External Dependencies

```bash
# List all external dependencies with versions
go list -m -versions all | head -20

# Check for outdated dependencies
go list -u -m all

# Show why a dependency is included
go mod why -m [module-name]
```

### 2.4 Identify Problematic Patterns

| Pattern | Problem | Detection |
|---------|---------|-----------|
| **Circular Import** | Build failure | `go build` fails |
| **Heavy Dependency** | Slow builds, large binary | Check transitive deps |
| **Unused Dependency** | Dead code, attack surface | `go mod tidy` removes it |
| **Version Conflict** | Build issues | `go mod graph` shows multiple versions |
| **Deprecated Package** | Security/support risk | Check module status |

## Phase 3: Categorize Issues

| Severity | Issue Type | Impact |
|----------|------------|--------|
| **HIGH** | Circular import | Build failure |
| **HIGH** | Known vulnerable dependency | Security risk |
| **MEDIUM** | Heavy transitive dependencies | Build time, binary size |
| **MEDIUM** | Outdated major version | Missing features, bugs |
| **LOW** | Unused dependency | Clutter |
| **LOW** | Minor version behind | Low risk |

## Phase 4: Refactoring Prompt Generation

### Prompt Template for Circular Import

```
TASK: Break circular import between internal/[A] and internal/[B]

CYCLE DETECTED:
- internal/[A] imports internal/[B]
- internal/[B] imports internal/[A]

ANALYSIS:
[Why this cycle exists - what types/functions are shared]

RESOLUTION OPTIONS:

Option 1: Extract Shared Types
- Create internal/shared or internal/common package
- Move shared types there
- Both A and B import from shared

Option 2: Interface Inversion
- Define interface in the dependent package
- Pass implementation via dependency injection

Option 3: Merge Packages
- If packages are tightly coupled, consider merging

RECOMMENDED: Option [N] because [reasoning]

IMPLEMENTATION:
1. [Specific steps]
2. [More steps]

VERIFICATION:
- [ ] `go build ./...` succeeds
- [ ] No import cycles: `go list -f '{{.ImportPath}}' ./internal/...`
- [ ] All tests pass
- [ ] Functionality preserved

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Template for Dependency Update

```
TASK: Update [dependency] from v[X] to v[Y]

CURRENT STATE:
- Current version: v[X]
- Latest version: v[Y]
- Breaking changes: [Yes/No]

CHANGELOG REVIEW:
[Key changes between versions]

MIGRATION STEPS:
1. Update go.mod: `go get [module]@v[Y]`
2. Run `go mod tidy`
3. [Any API changes to handle]
4. Run tests: `go test ./...`

VERIFICATION:
- [ ] `go build ./...` succeeds
- [ ] All tests pass
- [ ] No deprecation warnings
- [ ] Functionality preserved

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Template for Unused Dependency

```
TASK: Remove unused dependency [module]

ANALYSIS:
- Module: [name]
- Why it was added: [if known]
- Current usage: None detected

VERIFICATION BEFORE REMOVAL:
```bash
go mod why -m [module]
grep -rn "[package-name]" internal/
```

REMOVAL STEPS:
1. Run `go mod tidy`
2. Verify it's removed from go.mod
3. Run `go build ./...`
4. Run `go test ./...`

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

## Phase 5: Dependency Report

```markdown
## Dependency Audit Report

### Dependency Summary

| Metric | Value |
|--------|-------|
| Direct Dependencies | X |
| Transitive Dependencies | Y |
| Internal Packages | Z |
| Circular Imports | N |
| Outdated Dependencies | M |

### Dependency Visualization

```
go-vendor
├── internal/core (5 imports)
├── internal/tui (3 imports)
├── internal/types (0 imports)
└── internal/version (0 imports)

External:
├── github.com/charmbracelet/huh (TUI forms)
├── github.com/charmbracelet/lipgloss (styling)
├── gopkg.in/yaml.v3 (config parsing)
└── github.com/fsnotify/fsnotify (file watching)
```

### Issues Found

#### Circular Imports (HIGH)
| Cycle | Packages |
|-------|----------|
| None detected | - |

#### Outdated Dependencies (MEDIUM)
| Module | Current | Latest | Behind |
|--------|---------|--------|--------|
| [name] | v1.0.0 | v1.2.0 | 2 minor |

#### Unused Dependencies (LOW)
| Module | Action |
|--------|--------|
| [name] | Remove with `go mod tidy` |

### Prompts Generated

---

## PROMPT 1: Update dependency
[Full prompt]

---
```

## Phase 6: Verification Cycle

After refactoring prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Re-Run Dependency Analysis:**
  ```bash
  # Rebuild
  go build ./...

  # Run tests
  go test ./...

  # Check for cycles
  go list -f '{{.ImportPath}}' ./internal/...

  # Verify mod is tidy
  go mod tidy
  git diff go.mod go.sum
  ```

- **Grade Each Refactoring:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Issue resolved, builds clean | Complete |
  | **PARTIAL** | Issue improved but not fully resolved | Follow-up |
  | **REGRESSION** | Refactoring broke functionality | Urgent fix |
  | **FAIL** | No change made | Redo prompt |

## Phase 7: Dependency Health Report

When audit is complete:

- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```

- **Final Report:**

  ```markdown
  ## Dependency Audit Complete

  ### Health Improvement

  | Metric | Before | After |
  |--------|--------|-------|
  | Circular Imports | X | 0 |
  | Outdated Deps | Y | Z |
  | Unused Deps | W | 0 |

  ### Changes Made

  | Issue | Resolution |
  |-------|------------|
  | Outdated huh | Updated to v0.5.0 |
  | Unused module | Removed |

  ### Dependency Graph (Clean)
  [Updated visualization]

  ### Recommendations
  - [Preventive measures]
  - [Monitoring suggestions]
  ```

---

## Dependency Analysis Quick Reference

### Quick Health Check

```bash
# Full dependency check
go mod verify

# Check for updates
go list -u -m all

# Remove unused
go mod tidy

# Why is this included?
go mod why -m [module]

# Dependency graph
go mod graph | grep "git-vendor"
```

### Security Check

```bash
# If govulncheck is installed
govulncheck ./...

# Check for known vulnerabilities
go list -m -json all | jq -r '.Path + "@" + .Version' | head -20
```

### Size Analysis

```bash
# Build and check binary size
go build -o git-vendor
ls -lh git-vendor

# See what's contributing to size
go build -ldflags="-s -w" -o git-vendor-stripped
ls -lh git-vendor-stripped
```

---

## Integration Points

- **go.mod** - Module dependencies
- **go.sum** - Dependency checksums
- **internal/** - Package structure
- **CLAUDE.md** - Dependency documentation
- **Makefile** - Build targets
