# Hook Execution Threat Model (SEC-012)

This document describes the threat model for `git-vendor`'s hook execution system.

## Overview

git-vendor allows users to configure pre-sync and post-sync shell hooks in `vendor.yml`. These hooks execute arbitrary shell commands with the user's permissions.

```yaml
vendors:
  - name: frontend-lib
    hooks:
      pre_sync: echo "preparing..."
      post_sync: npm install && npm run build
```

## Trust Model

git-vendor's hook execution follows the **same trust model as npm scripts, git hooks, and Makefile targets**: the user who controls the config file controls the commands. This is explicitly NOT sandboxed.

| Property | Value |
|----------|-------|
| Execution context | Project root directory |
| Shell | `sh -c` (Unix), `cmd /c` (Windows) |
| Permissions | Current user's full permissions |
| Timeout | 5 minutes (kills via context cancellation) |
| Environment | Inherits parent env + GIT_VENDOR_* vars |

## Threat Vectors and Mitigations

### T1: Malicious vendor.yml in Shared Repository

**Threat:** An attacker commits a malicious `vendor.yml` with hooks that exfiltrate data, install backdoors, or modify the filesystem.

**Mitigation:** Same as `package.json` `postinstall` scripts — the user MUST review `vendor.yml` changes before running `git-vendor sync`. This is the accepted trust model for configuration-driven build tools.

**Accepted Risk:** Users who run `git-vendor sync` on untrusted repos execute attacker-controlled hooks.

### T2: Environment Variable Injection

**Threat:** An attacker crafts vendor names/URLs containing newlines or null bytes to inject additional environment variables when hooks run.

**Mitigation:** `sanitizeEnvValue()` strips `\n`, `\r`, and `\x00` from all `GIT_VENDOR_*` environment variable values before injection. This prevents environment variable boundary manipulation.

**Implementation:** `hook_service.go:sanitizeEnvValue()`

### T3: Hook Timeout Exhaustion

**Threat:** A hook command hangs indefinitely, blocking the sync pipeline.

**Mitigation:** `context.WithTimeout` with a 5-minute deadline. When the deadline exceeds, the process group is killed via `exec.CommandContext`. Tests MUST use `exec sleep` (not bare `sleep`) to ensure the child process is killed when `sh -c` is terminated.

**Implementation:** `hook_service.go:executeHook()` timeout via `hookTimeout` constant (overridable in tests via `hookService.timeout` field).

### T4: Hook Output Injection

**Threat:** Hook output (stdout/stderr) contains ANSI escape sequences or terminal control characters that could manipulate the user's terminal.

**Mitigation:** Hook output is passed directly to the parent process's stdout/stderr. This is the same behavior as `make`, `npm run`, and `git hooks`. Terminal manipulation via output is a general terminal emulator concern, not specific to git-vendor.

**Accepted Risk:** Hook output is unfiltered. Users MUST trust the commands they configure.

### T5: Credential Leakage via Hook Environment

**Threat:** Hooks inherit the parent environment, which may contain `GITHUB_TOKEN`, `GITLAB_TOKEN`, or other secrets. A malicious hook could exfiltrate these.

**Mitigation:** Environment inheritance is intentional — hooks need access to tokens for operations like `npm install` from private registries. The trust model requires that the user controls the hook commands.

**Accepted Risk:** Hooks have access to all parent environment variables.

### T6: Pre-sync Hook Modifies Checkout Before Copy

**Threat:** A pre-sync hook modifies files in the cloned repository before git-vendor copies them, injecting malicious content into vendored files.

**Mitigation:** Pre-sync hooks run before the clone/checkout occurs. The hook `RootDir` is the project root, not the temp clone directory. The temp clone directory path is not exposed to hooks.

**Status:** Not exploitable — hooks cannot access the temp clone directory.

### T7: Post-sync Hook Failure Leaves Partial State

**Threat:** A post-sync hook fails after files have been copied, leaving the project in a partially-synced state.

**Mitigation:** This is documented behavior. Post-sync hook failure marks the sync operation as failed, but files already copied remain on disk. Users can re-run `git-vendor sync` to retry.

**Accepted Risk:** Post-sync hook failure does not roll back copied files.

## Security Properties

| Property | Guaranteed | Notes |
|----------|-----------|-------|
| No shell injection in git args | Yes | All git operations use `exec.Command` with separate args (no shell) |
| Hook env vars sanitized | Yes | `sanitizeEnvValue()` strips newlines/nulls |
| Hook timeout enforced | Yes | 5-minute deadline via `context.WithTimeout` |
| Hook commands sandboxed | No | By design — same model as npm/make |
| Credentials hidden from hooks | No | By design — hooks need env vars |
| Hook output filtered | No | Same as npm/make/git — direct pass-through |

## Recommendations for Users

1. **Review vendor.yml changes** before running `git-vendor sync` on repos you don't control
2. **Use CI/CD isolation** when running `git-vendor sync` in automated pipelines
3. **Audit hook commands** the same way you audit `Makefile` targets or `package.json` scripts
4. **Use `--dry-run`** to preview sync operations without executing hooks
