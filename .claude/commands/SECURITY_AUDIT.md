# Security Audit Workflow

**Role:** You are a security auditor working in a concurrent multi-agent Git environment. Your goal is to systematically identify security vulnerabilities, generate remediation prompts for other Claude instances, and verify fixes meet security standards through iterative review cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your audit branch (already created for you)

**Key Principle:** Security issues have SLAs. CRITICAL vulnerabilities demand immediate attention. Never mark a security issue complete without verifying the actual fix in code.

## Phase 1: Sync & Security Scan

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Automated Security Scans:**

  | Category | Pattern | Check Command |
  |----------|---------|---------------|
  | **Path Traversal** | Unsanitized file paths | `grep -rn "filepath.Join\|os.Open\|os.Create" internal/` |
  | **Command Injection** | Unescaped shell commands | `grep -rn "exec.Command" internal/` |
  | **Git Injection** | Unsanitized git args | `grep -rn "git.*+" internal/core/git_operations.go` |
  | **Credential Exposure** | Hardcoded secrets | `grep -rni "password\|secret\|token\|key" internal/` |
  | **Unsafe Deserialization** | YAML without limits | `grep -rn "yaml.Unmarshal" internal/` |
  | **Panic in Library** | Unhandled panics | `grep -rn "panic(" internal/` |
  | **Race Conditions** | Unsynchronized access | `go test -race ./...` |

- **Go Vulnerability Check:**
  ```bash
  # If govulncheck is available
  govulncheck ./...

  # Check dependencies for known vulnerabilities
  go list -m -json all | head -50
  ```

- **Manual Security Review:**
  - Check path validation in `filesystem.go` (ValidateDestPath)
  - Review git command construction in `git_operations.go`
  - Audit hook execution in `hook_service.go`
  - Check URL parsing in smart URL functions

- **Check Existing Security Tracking:**
  - Read `ideas/security.md` for known issues
  - Check `ideas/code_quality.md` for security-tagged items
  - Review any SEC-XXX specs in `ideas/specs/security/`

## Phase 2: Severity Classification

Classify all findings using security severity levels:

| Severity | SLA | Criteria | Examples |
|----------|-----|----------|----------|
| **CRITICAL** | 24 hours | Active exploitation possible, data breach risk | Path traversal to arbitrary files, command injection |
| **HIGH** | 72 hours | Significant vulnerability, requires specific conditions | Git command injection, credential in logs |
| **MEDIUM** | 1 week | Limited impact, defense in depth issue | Missing input validation, verbose errors |
| **LOW** | 2 weeks | Best practice violation, hardening opportunity | Panic instead of error return |

Create/update entries in `ideas/security.md`:
- Use next available SEC-NNN ID
- Set severity level and SLA
- Link to spec if remediation is complex

## Phase 3: Remediation Prompt Generation

For each vulnerability, generate a remediation prompt:

### Prompt Template

```
TASK: [SEC-XXX] Fix [Vulnerability Type] in [Location]

SEVERITY: [CRITICAL/HIGH/MEDIUM/LOW] - SLA: [timeframe]

VULNERABILITY: [Detailed description of the security issue]

LOCATION:
- File: internal/core/[file].go
- Line(s): [line numbers]
- Function: [function name]
- Code:
```go
[snippet showing the vulnerable pattern]
```

ATTACK VECTOR: [How this could be exploited]

REMEDIATION:
1. [Specific fix with code example]
2. [Additional hardening if applicable]

SECURE PATTERN:
```go
[Show the correct, secure implementation]
```

MANDATORY TRACKING UPDATES:
1. Update ideas/security.md - change SEC-XXX status to "completed"
2. Add remediation notes under completion details
3. If this created a new pattern, document in CLAUDE.md

VERIFICATION:
- [ ] Grep confirms vulnerable pattern removed
- [ ] Secure pattern implemented correctly
- [ ] No new vulnerabilities introduced
- [ ] Related code paths also checked
- [ ] Tests added for security boundary

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Security-Specific Guidelines

- **Never suggest disabling security features** as a fix
- **Include defense in depth** - don't just fix the immediate issue
- **Check related code** - same vulnerability often exists elsewhere
- **Provide secure alternatives** - don't just say "don't do X", show the right way
- **Add tests** - security fixes should have test coverage

## Phase 4: Security Report

Present findings with appropriate urgency:

```markdown
## Security Audit Report

### Executive Summary

| Severity | Count | Oldest SLA |
|----------|-------|------------|
| CRITICAL | X | [date] |
| HIGH | X | [date] |
| MEDIUM | X | [date] |
| LOW | X | [date] |

**Immediate Action Required:** [Yes/No]

### Critical Findings (Requires Immediate Attention)

| ID | Vulnerability | Location | SLA Deadline |
|----|--------------|----------|--------------|
| SEC-XXX | [Type] | [File:Line] | [Date] |

### Remediation Prompts

#### PROMPT 1: [CRITICAL] SEC-XXX - [Title]
[Full prompt]

---

### Execution Priority

1. **Immediate (CRITICAL):** [List]
2. **This Week (HIGH):** [List]
3. **Next Sprint (MEDIUM/LOW):** [List]
```

## Phase 5: Verification Cycle

After remediation prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Re-Run Security Scans:**
  - Execute the same grep patterns from Phase 1
  - Verify each finding is actually fixed (not just documented)
  - Check that fix didn't introduce new vulnerabilities

- **Security-Specific Verification:**

  | Check | Method |
  |-------|--------|
  | Pattern removed | Re-run original grep, expect 0 matches |
  | Secure pattern used | Grep for the secure alternative |
  | No regression | Run `go test -race ./...` |
  | Tracking updated | Check ideas/security.md |

- **Grade Each Remediation:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Vulnerability fixed, secure pattern used | Close, update tracking |
  | **PARTIAL** | Main issue fixed, related issues remain | Follow-up for remaining |
  | **FAIL** | Not fixed, or fix introduced new issue | Escalate, generate urgent redo |
  | **REGRESSION** | Fix created new vulnerability | CRITICAL follow-up |

- **Generate Follow-Up Prompts:** For non-PASS grades with security emphasis

- **Iterate:** Repeat until all security issues grade as PASS

## Phase 6: Security Sign-Off

When all vulnerabilities are remediated:

- **Final Security Scan:** Complete re-scan to confirm clean state
- **Update Security Tracking:**
  - All SEC-XXX items marked completed with dates
  - security.md reflects current state
- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```
- **Security Report:**

  ```markdown
  ## Security Audit Complete

  ### Vulnerabilities Remediated

  | ID | Severity | Issue | Remediation |
  |----|----------|-------|-------------|
  | SEC-XXX | CRITICAL | [Type] | [Fix applied] |

  ### Verification Summary
  - All CRITICAL: Verified fixed
  - All HIGH: Verified fixed
  - Security scans: Clean
  - Race detector: Clean

  ### Residual Risk
  [Any accepted risks or deferred items with justification]

  ### Recommendations
  - [Preventive measures]
  - [Security process improvements]
  ```

---

## Security Scan Quick Reference

### Path Traversal Patterns

```bash
# File path operations
grep -rn "filepath.Join\|filepath.Clean" internal/
grep -rn "os.Open\|os.Create\|os.Remove" internal/

# Check ValidateDestPath usage
grep -rn "ValidateDestPath" internal/

# Look for path operations without validation
grep -B5 "os.Open\|os.Create" internal/core/ | grep -v ValidateDestPath
```

### Command Injection Patterns

```bash
# exec.Command usage
grep -rn "exec.Command" internal/

# Check for string concatenation in commands
grep -A5 "exec.Command" internal/core/git_operations.go

# Shell execution
grep -rn "sh -c\|bash -c\|cmd /c" internal/
```

### Credential Exposure

```bash
# Sensitive keywords
grep -rni "password\|secret\|token\|credential\|apikey" internal/

# Environment variable access
grep -rn "os.Getenv" internal/

# Logging of sensitive data
grep -rn "log\.\|fmt.Print" internal/ | grep -i "token\|password"
```

### Go Security Best Practices

```bash
# Panic usage (should return error in library code)
grep -rn "panic(" internal/

# Race condition detection
go test -race ./...

# Check for unsafe package usage
grep -rn "unsafe\." internal/
```

---

## git-vendor Specific Security Areas

### 1. Path Validation (filesystem.go)

The `ValidateDestPath` function is critical:
- Must reject absolute paths
- Must reject parent directory traversal (`../`)
- Must be called before ALL file operations

### 2. Git Operations (git_operations.go)

Git commands must not be injectable:
- URL validation before use
- Ref/branch validation
- No string concatenation in command args

### 3. Hook Execution (hook_service.go)

Hooks run arbitrary commands:
- Document security model
- Ensure hooks run in correct directory
- No injection through environment variables

### 4. YAML Parsing (config_store.go, lock_store.go)

YAML parsing can be vulnerable:
- Consider size limits
- Validate structure after parsing

---

## Integration Points

- **ideas/security.md** - Primary tracking for security issues
- **ideas/code_quality.md** - Security-related code quality items
- **ideas/specs/security/** - Detailed specs for complex remediations
- **CLAUDE.md** - Security documentation and requirements
- **internal/core/filesystem.go** - Path validation
- **internal/core/git_operations.go** - Git command execution
- **internal/core/hook_service.go** - Hook execution
