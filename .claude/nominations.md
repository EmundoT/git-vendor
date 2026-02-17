# Nominations

## CLAUDE_PROJECT_DIR unreliable in Stop hooks

- **Date**: 2026-02-17
- **Source**: git-vendor/task-update-hook-fix
- **Priority**: high
- **Category**: cross-project

`$CLAUDE_PROJECT_DIR` is not reliably set across all Claude Code hook lifecycle events. Observed empty in Stop hooks on Windows/Git Bash, causing hook commands in `settings.json` to resolve paths as `/.claude/hooks/<script>` (nonexistent). All ecosystem projects using vendored hooks from git-ecosystem are affected. Fix: document `${CLAUDE_PROJECT_DIR:-.}` as the canonical invocation pattern in hook command strings. The vendored scripts themselves are fine (they self-resolve via `BASH_SOURCE`), but the launch commands in each project's `settings.json` are project-local and vulnerable.

## vendor-guard.sh lacks sync-vs-rogue distinction

- **Date**: 2026-02-17
- **Source**: git-vendor/task-update-hook-fix
- **Priority**: medium
- **Category**: tooling

`vendor-guard.sh` blocks ALL commits touching vendored files, including legitimate `git-vendor sync` outputs. It should distinguish between: (a) staged file matches vendor target hash (sync commit, allow), (b) staged file diverges from vendor target (rogue local edit, block with guidance). Currently there's no hash comparison — just path matching. The lock file already has SHA-256 hashes; the guard should compute the staged file's hash and compare.
