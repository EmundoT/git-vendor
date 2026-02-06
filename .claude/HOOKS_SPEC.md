# Chat Init Hooks -- System Spec v2

## Problem

When a Claude Code session starts, Claude has no idea what state the project is
in. When context compaction fires, Claude forgets everything. When a slash
command is invoked, there's no way to run setup commands specific to that
command's workflow. When a session ends, there's no record of final state.

## What Claude Code Gives Us Natively

Claude Code has a built-in hook system. Every hook receives JSON on stdin with
context about what just happened. What matters is **which fields each event
provides**, because that determines what we can actually detect and act on.

### The events and their data

**SessionStart** -- fires when a session begins, resumes, or recovers from
compaction.

```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/home/user/project",
  "permission_mode": "default",
  "hook_event_name": "SessionStart",
  "source": "startup",
  "model": "claude-sonnet-4-5-20250929"
}
```

`source` is one of: `startup`, `resume`, `compact`, `clear`. This tells us
HOW the session started but NOT what the user intends to do.

**UserPromptSubmit** -- fires when the user presses enter, before Claude
processes the prompt.

```json
{
  "session_id": "abc123",
  "cwd": "/home/user/project",
  "hook_event_name": "UserPromptSubmit",
  "prompt": "the full text of what the user submitted"
}
```

The `prompt` field contains the user's text. If they invoked a slash command,
this contains the **expanded content** of that command file (Claude Code expands
slash commands before the hook fires). No matcher support -- fires on every
prompt.

**PreCompact** -- fires before context compaction destroys the conversation.

```json
{
  "session_id": "abc123",
  "hook_event_name": "PreCompact",
  "trigger": "auto",
  "custom_instructions": ""
}
```

This is the last chance to snapshot state before Claude forgets everything.

**Stop** -- fires every time Claude finishes a response.

```json
{
  "session_id": "abc123",
  "hook_event_name": "Stop",
  "stop_hook_active": false,
  "transcript_path": "/path/to/transcript.jsonl"
}
```

`stop_hook_active` is `true` if Claude is already continuing from a previous
Stop hook. Check this to prevent infinite loops.

**SessionEnd** -- fires when the session terminates.

```json
{
  "session_id": "abc123",
  "hook_event_name": "SessionEnd",
  "reason": "prompt_input_exit"
}
```

`reason` is one of: `clear`, `logout`, `prompt_input_exit`,
`bypass_permissions_disabled`, `other`.

### What hooks can return

| Exit code | Effect |
|-----------|--------|
| 0 | Action proceeds. **stdout is injected into Claude's context** (for SessionStart, UserPromptSubmit). |
| 2 | Action is **blocked**. stderr is fed back to Claude as an error. |
| Other | Action proceeds. stdout is **discarded**. stderr logged in verbose mode only. |

For richer control, exit 0 and print JSON to stdout:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "This text goes into Claude's context"
  }
}
```

### Hook types

| Type | What it does |
|------|-------------|
| `command` | Runs a shell command. Stdin JSON in, stdout/exit code out. |
| `prompt` | Sends a prompt to a fast Claude model. Returns `{"ok": true/false, "reason": "..."}`. |
| `agent` | Spawns a subagent with tool access (Read, Grep, Glob, Bash). Multi-turn verification. |

### Skill/agent frontmatter hooks (the key insight)

**Slash command files can define their own hooks in YAML frontmatter.** These
hooks are scoped to that command's lifecycle -- they only run while the command
is active.

```yaml
---
name: idea-workflow
description: Implement features from queues
hooks:
  SessionStart:
    - matcher: ""
      hooks:
        - type: command
          command: "git fetch origin main && git log --oneline -5"
          once: true
---
# IDEA_WORKFLOW

Rest of the command file...
```

This means: **Claude Code already knows which slash command is active.** We don't
need to detect it ourselves. The frontmatter hooks fire automatically when the
command is invoked.

## Architecture

There are three layers, each solving a different problem:

### Layer 1: Session-level hooks (settings.json)

These run on every session, regardless of what command is used. They handle
environment/state awareness.

```
.claude/settings.json
    │
    ├── SessionStart (matcher: "startup|resume")
    │     → Runs the init script
    │     → Writes .claude/agent_init.md
    │     → Injects summary into Claude's context
    │
    ├── SessionStart (matcher: "compact")
    │     → Runs the init script with richer output
    │     → Re-injects project conventions, current branch, task context
    │     → This is the "amnesia recovery" path
    │
    ├── PreCompact
    │     → Snapshots current state BEFORE context is destroyed
    │     → Writes .claude/pre_compact_state.md
    │     → The compact SessionStart hook reads this file back
    │
    └── SessionEnd
          → Runs the post script
          → Writes .claude/agent_end.md
```

### Layer 2: Command-specific hooks (YAML frontmatter)

These run only when a specific slash command is invoked. They handle
command-specific setup.

```
.claude/commands/IDEA_WORKFLOW.md
    │
    └── YAML frontmatter:
          hooks:
            SessionStart:
              - hooks:
                  - type: command
                    command: "git fetch origin main && git stash list"
                    once: true
```

The `once: true` field means this runs once when the command is first invoked,
then removes itself. Claude Code manages this natively.

### Layer 3: The orchestrator script (chat-init-hooks.mjs)

This exists because Claude Code hooks support **one command per hook entry**. If
you want to run 8 commands, capture all their output, and render a structured
log, you need an orchestrator. The script:

1. Reads the stdin JSON from Claude Code (session_id, source, model, etc.)
2. Reads shell commands from a configuration source (hooks.md or inline)
3. Runs each command, capturing stdout, stderr, exit code, and duration
4. Renders a structured markdown log to a file (agent_init.md / agent_end.md)
5. Writes a summary to stdout (injected into Claude's context)
6. **Always exits 0** so the output reaches Claude even when commands fail

The script also exports all functions for import as a Node.js module.

## Data Flow

### Normal session start (startup/resume)

```
Session starts
    │
    ▼
Claude Code fires SessionStart {source: "startup"}
    │
    ▼
settings.json: matcher "startup|resume" matches
    │
    ▼
Runs: node chat-init-hooks.mjs --phase pre
    │
    ├── Reads stdin JSON (session_id, source, model)
    ├── Reads .claude/hooks.md for <!-- @hook:pre --> commands
    ├── Runs each command, captures output
    ├── Writes full log to .claude/agent_init.md
    ├── Writes summary to stdout
    └── Exits 0
    │
    ▼
Claude Code injects stdout into context:
  "SessionStart:startup hook success:
   [chat-init-hooks] pre phase: SUCCESS (8/8)"
    │
    ▼
Claude sees the summary. Full log is in agent_init.md
if Claude or the user needs details.
```

### Compaction recovery

```
Context window fills up
    │
    ▼
Claude Code fires PreCompact {trigger: "auto"}
    │
    ▼
PreCompact hook snapshots current state:
  - Current branch + dirty files
  - What Claude was working on (from transcript)
  - Writes to .claude/pre_compact_state.md
    │
    ▼
Claude Code compacts conversation (context is lost)
    │
    ▼
Claude Code fires SessionStart {source: "compact"}
    │
    ▼
settings.json: matcher "compact" matches
    │
    ▼
Runs: node chat-init-hooks.mjs --phase pre --compact
    │
    ├── Normal env/git state collection
    ├── ALSO reads .claude/pre_compact_state.md
    ├── Outputs richer context: project conventions, branch state,
    │   what was being worked on, recent changes
    └── Exits 0
    │
    ▼
Claude Code injects the richer context into Claude's
now-empty conversation. Claude recovers awareness.
```

### Slash command invocation

```
User types /IDEA_WORKFLOW
    │
    ▼
Claude Code expands the command file
    │
    ▼
If the file has YAML frontmatter hooks, they activate:

    ---
    hooks:
      SessionStart:
        - hooks:
            - type: command
              command: "git log --oneline -10 origin/main..HEAD"
              once: true
    ---

    │
    ▼
The frontmatter hook runs automatically (managed by Claude Code)
Its stdout is added to context alongside the command content
    │
    ▼
Claude processes the expanded command with the hook's
output as additional context
```

### Session end

```
User exits or session terminates
    │
    ▼
Claude Code fires SessionEnd {reason: "prompt_input_exit"}
    │
    ▼
settings.json: SessionEnd hook matches
    │
    ▼
Runs: node chat-init-hooks.mjs --phase post
    │
    ├── Reads .claude/hooks.md for <!-- @hook:post --> commands
    ├── Runs each (git status, git log, timestamp)
    ├── Writes full log to .claude/agent_end.md
    └── Exits 0
    │
    ▼
Log persists on disk for next session or external systems.
SessionEnd stdout is NOT injected into context (session is over).
```

## File Map

```
.claude/
├── settings.json                  # Hook registration (SessionStart, PreCompact, SessionEnd)
├── hooks.md                       # Default shell commands for session-level hooks
├── scripts/
│   └── chat-init-hooks.mjs        # Orchestrator: parses, runs, logs, always exits 0
├── agent_init.md                  # Generated: log from last SessionStart (gitignored)
├── agent_end.md                   # Generated: log from last SessionEnd (gitignored)
├── pre_compact_state.md           # Generated: state snapshot before compaction (gitignored)
└── commands/
    ├── IDEA_WORKFLOW.md            # Has YAML frontmatter hooks (command-specific)
    ├── CODE_REVIEW.md              # Can also have frontmatter hooks
    └── ...
```

## The Markdown Syntax (hooks.md)

Shell commands are embedded in HTML comment blocks. The markdown between them is
documentation that the script ignores.

```markdown
<!-- @hook:pre
git branch --show-current
git log --oneline -5 --decorate
git status --short
echo "Go: $(go version 2>/dev/null | cut -d' ' -f3 || echo 'not found')"
-->

# Documentation

This file's pre-hooks run on every session start.
This file's post-hooks run on every session end.

<!-- @hook:post
git status --short
git log --oneline -1 --decorate
-->
```

**Parsing rules:**
- `<!-- @hook:pre ... -->` must be the first thing in the file (anchored to `^`)
- `<!-- @hook:post ... -->` must be the last thing in the file (anchored to `$`)
- One shell command per line
- Empty lines and `#`-prefixed lines are skipped
- Commands run via `sh -c` (Unix) or `cmd /c` (Windows)

## The Node Server Action Pattern

The script exports all core functions so a node server can call them directly
without spawning a child process:

```javascript
import {
  parseHookBlock,
  executeHookPhase,
  renderMarkdownLog,
  collectHooks,
} from './.claude/scripts/chat-init-hooks.mjs';

// As an event listener
server.on('session:start', (session) => {
  const { commands, sourceFiles } = collectHooks('pre', projectDir);
  const result = executeHookPhase({
    phase: 'pre',
    commands,
    workingDir: projectDir,
    sessionInput: { session_id: session.id, source: 'startup' },
  });
  const log = renderMarkdownLog(result);
  fs.writeFileSync('.claude/agent_init.md', log);
  dashboard.push({ sessionId: session.id, hookResult: result });
});
```

## Key Design Decisions

### Why the script always exits 0

Claude Code only injects stdout into context on exit 0. If we exit 1 when a
command fails, Claude never sees the failure details. So the script always exits
0 and puts error information in the stdout summary. Errors are information, not
crashes.

### Why hooks.md exists alongside settings.json

settings.json tells Claude Code WHEN to run the script. hooks.md tells the
script WHAT commands to run. Separating these means you can change the commands
(hooks.md) without touching the hook registration (settings.json), and
non-developers can edit a markdown file instead of JSON.

### Why YAML frontmatter for command-specific hooks

Claude Code natively supports hooks in skill/agent frontmatter. These hooks are
scoped to the command's lifecycle and managed entirely by Claude Code. We use
this instead of our `<!-- @hook:pre -->` syntax for command-specific hooks
because:

1. Claude Code knows which command is active (we don't have to detect it)
2. The hooks activate/deactivate automatically with the command
3. The `once: true` field handles one-shot setup without custom logic
4. It's a supported, documented feature of the platform

The `<!-- @hook:pre -->` syntax remains useful for session-level hooks in
hooks.md, where there's no YAML frontmatter and we want a clean way to embed
commands in a documentation file.

### Why compaction gets special treatment

When compaction fires, Claude loses ALL context -- it doesn't know what it was
doing, what branch it's on, or what the project conventions are. The PreCompact
hook snapshots this state before it's lost. The compact-triggered SessionStart
hook reads the snapshot and re-injects it. This is the most important hook path
in the system.
