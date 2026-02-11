---
paths:
  - "internal/**/*.go"
  - "main.go"
---

# Architecture

## Data Model

VendorConfig (vendor.yml):
- VendorSpec: Name, URL, License, Groups []string, Hooks *HookConfig (PreSync/PostSync), Specs []BranchSpec
- BranchSpec: Ref, DefaultTarget, Mapping []PathMapping
- PathMapping: From (remote), To (local, empty = auto)

VendorLock (vendor.lock):
- LockDetails: Name, Ref, CommitHash, LicensePath, Updated, FileHashes map[string]string, LicenseSPDX, SourceVersionTag, VendoredAt, VendoredBy, LastSyncedAt (schema v1.1), Positions []PositionLock
- PositionLock: From (with :L spec), To, SourceHash

PositionSpec (internal/types/position.go):
- StartLine, EndLine (1-indexed, 0 = same as StartLine), StartCol, EndCol (1-indexed inclusive byte offset), ToEOF bool

LicensePolicy (.git-vendor-policy.yml):
- LicensePolicyRules: Allow, Deny, Warn []string, Unknown string (allow|warn|deny)
- Evaluation order: deny > allow > warn > unknown

## Service Layer (Clean Architecture)

All services injected via interfaces into VendorSyncer. Manager is the public facade.

Flow: main.go -> Manager (engine.go) -> VendorSyncer -> Service interfaces -> git operations (via git-plumbing)

Key services:
- SyncService: fetchWithFallback, canSkipSync, updateCache
- UpdateService: update lockfile, compute file hashes, parallel updates
- FileCopyService: position-aware copy, local modification detection
- VerifyService: verification against lockfile hashes + position-level checks
- ValidationService: config validation, conflict detection
- DriftService: three-way drift detection (local/upstream/conflict risk)
- HookExecutor: pre/post sync shell commands (5-min timeout, env injection)
- ParallelExecutor: worker pool (default NumCPU, max 8)

## Context Propagation

All long-running operations accept context.Context as first parameter. Flow:
main.go (signal.NotifyContext) -> Manager -> VendorSyncer -> Service interfaces -> git operations

ctx.Err() checked at vendor loop boundaries for cooperative cancellation. Exception: interactive wizard paths use context.Background().

## Git Operations Adapter

SystemGitClient wraps git-plumbing via gitFor(dir) helper. Creates *git.Git instances per-call (cheap, no I/O). All git execution delegated to git-plumbing -- no direct exec.Command calls.

Key adapters:
- GetCommitLog: git.Commit -> types.CommitInfo (date formatting: time.Time -> "2006-01-02 15:04:05 -0700")
- GetTagForCommit: calls TagsAt() then applies isSemverTag() preference
- Commit: converts []types.Trailer -> []git.Trailer (distinct Go types, explicit conversion required)

Standalone: GetGitUserIdentity() -> git.Git{}.UserIdentity(), IsGitInstalled() -> git.IsInstalled()

## Parallel Processing

ParallelExecutor with ExecuteParallelSync() and ExecuteParallelUpdate(). Worker goroutines process vendors from job channel. Results collected via channel, first error = fail-fast. Thread safety: unique temp dirs per vendor, lockfile written once at end.

Flags: --parallel enables, --workers N sets count. Auto-disabled for --dry-run.

## Custom Hooks

HookExecutor runs pre/post sync shell commands. Cross-platform: sh -c (Unix), cmd /c (Windows). 5-minute timeout via context.WithTimeout (configurable in tests via hookService.timeout).

Env vars provided: GIT_VENDOR_NAME, GIT_VENDOR_URL, GIT_VENDOR_REF, GIT_VENDOR_COMMIT, GIT_VENDOR_ROOT, GIT_VENDOR_FILES_COPIED, GIT_VENDOR_DIRS_CREATED.

Pre-sync failure stops sync (vendor skipped). Post-sync failure marks operation failed (files already copied). Hooks run even for cache hits.

## Wizard Flow (TUI)

1. User inputs URL -> ParseSmartURL extracts components
2. Check if repo tracked -> offer to edit existing
3. Collect name and ref
4. If deep link: offer to use that path
5. Enter edit loop for path mappings
6. Save triggers UpdateAll() -> regenerates lockfile

## License Compliance

GitHubLicenseChecker queries /repos/:owner/:repo/license. GitLab uses API. Others fall back to reading license files from repo root in order: LICENSE, LICENSE.txt, LICENSE.md, COPYING.

Allowed by default: MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC, Unlicense, CC0-1.0. Others prompt user confirmation.

When .git-vendor-policy.yml exists: deny -> allow -> warn -> unknown evaluation. Denied = hard block (no override). Warned = user prompt. Malformed policy = error (no silent fallback).

## Smart URL Parsing

ParseSmartURL extracts repo, ref, and path from GitHub/GitLab/Bitbucket URLs:
- github.com/owner/repo -> base URL only
- github.com/owner/repo/blob/main/path/file.go -> base URL, "main", "path/file.go"
- github.com/owner/repo/tree/v1.0/src/ -> base URL, "v1.0", "src/"

Limitation: branch names with slashes cannot be parsed (regex ambiguity).
