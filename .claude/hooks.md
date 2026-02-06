<!-- @hook:pre
git fetch origin main --quiet 2>/dev/null || true
git log --oneline -5 --decorate
git diff --stat origin/main...HEAD 2>/dev/null || true
echo "--- Environment ---"
echo "Platform: $(uname -s 2>/dev/null || echo unknown) $(uname -m 2>/dev/null || echo unknown)"
echo "Go: $(go version 2>/dev/null | cut -d' ' -f3 || echo 'not found')"
echo "Node: $(node --version 2>/dev/null || echo 'not found')"
echo "PWD: $(pwd)"
-->

# Chat Init Hooks

This file defines default pre/post hooks that run on every Claude Code session.

## Hook Block Syntax

Hooks are embedded in markdown files using HTML comment blocks with a
`@hook:pre` or `@hook:post` directive. The pre block goes at the **top** of the
file and the post block goes at the **bottom**. Both are optional.

```text
<!-- @hook:pre
command1
command2
-->

...file content...

<!-- @hook:post
command3
command4
-->
```

### Rules

- Each non-empty line inside the block is a shell command
- Lines starting with `#` are comments and are skipped
- Commands execute sequentially via `sh -c` (Unix) or `cmd /c` (Windows)
- Errors are caught and logged; by default execution continues to the next command
- Use `--fail-fast` flag to stop on first failure

### Placement

| Block | Location | Output file |
|-------|----------|-------------|
| `@hook:pre` | Top of file | `.claude/agent_init.md` |
| `@hook:post` | Bottom of file | `.claude/agent_end.md` |

### Scope

| File | When it runs |
|------|--------------|
| `.claude/hooks.md` | Every session (SessionStart / SessionEnd) |
| `.claude/commands/*.md` | When invoked with `--file` or `--scan` |

## What the Pre-Hook Above Does

1. Fetches latest main (quiet, ignores errors if offline)
2. Shows the 5 most recent commits for context
3. Shows the diff stat between main and HEAD (how far this branch has diverged)
4. Prints environment info: platform, Go version, Node version, working directory

## What the Post-Hook Below Does

1. Shows current git status (working tree state at session end)
2. Prints the branch and latest commit for provenance

## Node Server Action Usage

The executor script exports all core functions for use as a module:

```javascript
import {
  parseHookBlock,
  executeHookPhase,
  renderMarkdownLog,
  collectHooks,
} from './.claude/scripts/chat-init-hooks.mjs';

// Example: Express route handler
app.post('/hooks/:phase', (req, res) => {
  const result = executeHookPhase({
    phase: req.params.phase,
    commands: req.body.commands,
    workingDir: '/path/to/project',
    sourceFiles: req.body.sources || [],
  });
  res.json({ log: renderMarkdownLog(result), result });
});

// Example: Event listener on a node server
server.on('session:start', (session) => {
  const { commands, sourceFiles } = collectHooks('pre', projectDir);
  const result = executeHookPhase({
    phase: 'pre',
    commands,
    workingDir: projectDir,
    sourceFiles,
    sessionInput: { session_id: session.id },
  });
  writeFileSync('.claude/agent_init.md', renderMarkdownLog(result));
});
```

<!-- @hook:post
git status --short
git log --oneline -1 --decorate
echo "Session ended: $(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date)"
-->
