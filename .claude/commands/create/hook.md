# Create a Hook

You are a Claude Code hooks expert. Help the user create a hook for their use case.

## What hooks are

Claude Code hooks are shell commands that run automatically in response to lifecycle events. They are the **only** way to execute code at specific points in a Claude session — there is no plugin system, no middleware, no other extension point.

Hooks run as child processes. Claude Code pipes JSON to their stdin, reads their stdout, and uses the exit code to decide what happens next:

- **Exit 0** — proceed. Stdout is injected into Claude's context (for applicable events).
- **Exit 2** — block the action (for guard hooks like PreToolUse, UserPromptSubmit).
- **Any other exit** — proceed, but stdout is discarded. Stderr shows in `--verbose` mode only.

## Where hooks live

There are two places to define hooks:

### 1. Project-wide hooks in `settings.json`

These fire for every session in this project.

**File:** `.claude/settings.json`

```json
{
  "hooks": {
    "EventName": [
      {
        "matcher": "optional-filter",
        "hooks": [
          {
            "type": "command",
            "command": "your shell command here",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### 2. Command-scoped hooks in frontmatter

These fire only when a specific slash command is active.

**File:** `.claude/commands/your-command.md`

```yaml
---
hooks:
  EventName:
    - hooks:
        - type: command
          command: "your shell command here"
          once: true
          timeout: 15
---

# Your Command

Rest of the command content...
```

The `once: true` flag means the hook runs once when the command starts, not on every event cycle.

## Available events

| Event | When it fires | Matcher values | Stdout injected? |
|-------|--------------|----------------|-------------------|
| **SessionStart** | Session begins | `startup`, `resume`, `compact`, `clear` | Yes |
| **SessionEnd** | Session ends | `prompt_input_exit`, `stop_tool`, etc. | No |
| **PreCompact** | Before context compaction | `auto`, `manual` | Yes (into compaction summary) |
| **Stop** | Claude stops generating | — | No |
| **PreToolUse** | Before a tool runs | Tool name (e.g., `Bash`, `Write`, `Edit`) | Yes |
| **PostToolUse** | After a tool runs | Tool name | Yes |
| **UserPromptSubmit** | User sends a message | — | Yes |
| **Notification** | Notification sent | — | No |
| **SubagentStart** | Subagent spawns | — | Yes |
| **SubagentStop** | Subagent finishes | — | No |
| **PermissionRequest** | Permission prompt shown | — | No |
| **PostToolUseFailure** | After a tool fails | Tool name | Yes |

## What hooks receive on stdin

Every hook gets a JSON object on stdin. Common fields:

```json
{
  "session_id": "abc-123",
  "transcript_path": "/path/to/session.jsonl",
  "cwd": "/project/root",
  "permission_mode": "default",
  "hook_event_name": "SessionStart"
}
```

Event-specific fields:

- **SessionStart**: `source` (`startup`, `resume`, `compact`, `clear`)
- **SessionEnd**: `reason` (`prompt_input_exit`, `stop_tool`, etc.)
- **PreCompact**: `trigger` (`auto`, `manual`), `transcript_path`
- **PreToolUse / PostToolUse**: `tool_name`, `tool_input`
- **UserPromptSubmit**: `prompt`

## Practical examples

### Inject git context on session start

The most common and useful hook. Gives Claude awareness of your branch and recent work.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume",
        "hooks": [
          {
            "type": "command",
            "command": "echo \"Branch: $(git branch --show-current)\" && git log --oneline -5 && git status --short",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### Block dangerous bash commands

Prevent Claude from running destructive commands:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "node -e \"const input = JSON.parse(require('fs').readFileSync(0, 'utf-8')); const cmd = input.tool_input?.command || ''; if (/rm\\s+-rf|git\\s+push\\s+--force|drop\\s+table/i.test(cmd)) { process.exit(2); }\"",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

### Auto-run tests after file edits

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "go test ./... 2>&1 | tail -5",
            "timeout": 60
          }
        ]
      }
    ]
  }
}
```

### Command-scoped pre-scan

Add to a command file's frontmatter so it only runs when that command is invoked:

```yaml
---
hooks:
  SessionStart:
    - hooks:
        - type: command
          command: "echo '## Scan Results' && gofmt -l internal/ && go vet ./... 2>&1 | tail -5"
          once: true
          timeout: 30
---
```

### Re-inject context after compaction

When Claude's context gets compacted, it generates a summary but loses exact details. Re-inject critical state:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "echo '## Post-Compaction Context' && echo \"Branch: $(git branch --show-current)\" && git log --oneline -5 && git diff --stat origin/main...HEAD 2>/dev/null",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

## Bad usage cases

Hooks are powerful but easy to misuse. Avoid these patterns:

### Over-engineering with scripts

**Bad:** Writing a 300-line Node.js orchestrator to do what 3 bash commands can do.

```json
// Don't do this unless you genuinely need the complexity
"command": "node elaborate-orchestrator.mjs --mode=init --format=structured --log-level=debug"

// Do this instead
"command": "echo \"Branch: $(git branch --show-current)\" && git log --oneline -5"
```

If your hook needs a script file, it's probably doing too much.

### Reinventing native features

**Bad:** Building custom compaction recovery that parses transcripts and reconstructs context.

Claude Code already generates a summary during compaction. Your hook should supplement it with hard facts (branch name, dirty files) — not try to replace the summarization.

### Slow hooks that block the session

**Bad:** Running a full test suite on every session start.

```json
// This blocks Claude for 30+ seconds every time
"command": "go test -race -coverprofile=coverage.out ./..."

// Fast alternative: just count tests, run the suite in the workflow
"command": "echo \"Test files: $(find . -name '*_test.go' | wc -l)\""
```

Hooks should be fast (under 10 seconds). Heavy operations belong in the workflow itself, where Claude can manage timing and react to failures.

### Guard hooks that are too broad

**Bad:** Blocking all Bash commands that mention `rm`.

```json
// This will block legitimate operations like "rm coverage.out"
"command": "if echo \"$TOOL_INPUT\" | grep -q 'rm'; then exit 2; fi"
```

Guard hooks (exit 2) should be surgical. Overly broad guards create friction and teach users to disable hooks.

### Hooks that modify files silently

**Bad:** A hook that auto-formats code or modifies files without Claude knowing.

```json
// Claude doesn't know this happened — it will read stale file content
"command": "gofmt -w internal/"
```

If a hook changes files, Claude's cached view becomes stale. Use hooks for reading state, not modifying it.

## Where to learn more

- **Hook system reference:** https://docs.anthropic.com/en/docs/claude-code/hooks
- **Hook guide with examples:** https://docs.anthropic.com/en/docs/claude-code/hooks-guide
- **This project's hooks:** `.claude/settings.json` (SessionStart hooks for git context injection)
- **Command-scoped hook examples:** `.claude/commands/CODE_REVIEW.md`, `SECURITY_AUDIT.md`, `TEST_COVERAGE.md`, `RELEASE_READY.md`

## Your task

Now help the user create a hook. Ask them:

1. **What event** should trigger the hook? (session start, before a tool runs, after edits, etc.)
2. **What should it do?** (inject context, block dangerous commands, run checks, etc.)
3. **Where should it live?** (project-wide in settings.json, or scoped to a specific command?)

Then generate the configuration and explain where to put it.
