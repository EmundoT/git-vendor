# Chat Init Hooks -- System Spec

## Problem

When a Claude Code session starts, Claude has no idea what state the project is
in. It doesn't know the current branch, whether there are uncommitted changes,
what toolchain versions are installed, or what happened in the last session. When
a session ends, there's no record of what state things were left in. When context
compaction fires mid-session, Claude forgets everything.

This system solves that by running shell commands at session boundaries and
feeding the results back into Claude's context automatically.

## How Claude Code Hooks Work (the platform)

Claude Code has a built-in hook system. You configure it in
`.claude/settings.json`. It works like this:

1. A **lifecycle event** fires (session starts, a tool runs, Claude stops, etc.)
2. Claude Code looks in settings.json for hooks registered to that event
3. For each matching hook, Claude Code **pipes JSON to the hook's stdin** and
   runs the command
4. The hook runs, does whatever it wants, and exits
5. **If the hook exits 0**, Claude Code takes its **stdout** and injects it into
   Claude's conversation context (Claude can see it)
6. **If the hook exits 2**, the action is **blocked** (e.g., prevent a tool from
   running)
7. **Any other exit code**, the action proceeds but stdout is discarded -- only
   stderr is logged (Claude never sees it)

The JSON piped to stdin looks like this (for a SessionStart event):

```json
{
  "session_id": "0a8aec4b-bdfb-471d-885b-6aff3739d5fe",
  "cwd": "/home/user/git-vendor-private",
  "hook_event_name": "SessionStart",
  "source": "startup"
}
```

The `source` field for SessionStart can be:
- `startup` -- brand new session
- `resume` -- continuing a previous session
- `compact` -- context window was full, conversation was summarized to free space
- `clear` -- user ran /clear

### Available events

| Event             | When it fires                              | Matcher filters on |
|-------------------|--------------------------------------------|--------------------|
| SessionStart      | Session begins, resumes, or compacts       | source (startup, resume, compact, clear) |
| SessionEnd        | Session terminates                         | reason (clear, logout, prompt_input_exit, other) |
| Stop              | Claude finishes a response (every time)    | no matcher         |
| PreToolUse        | Before a tool runs (can block it)          | tool name          |
| PostToolUse       | After a tool runs successfully             | tool name          |
| UserPromptSubmit  | User submits a prompt, before processing   | no matcher         |
| PreCompact        | Before context compaction                  | trigger (manual, auto) |
| Notification      | Claude needs attention                     | notification type  |
| SubagentStart     | A subagent is spawned                      | agent type         |
| SubagentStop      | A subagent finishes                        | agent type         |

### Hook types

| Type      | What it does |
|-----------|-------------|
| `command` | Runs a shell command. This is what our system uses. |
| `prompt`  | Sends a prompt to a Claude model (Haiku default). Returns `{"ok": true/false}`. For judgment calls. |
| `agent`   | Spawns a subagent with tool access (can read files, run commands). For verification that needs investigation. |

## What Our System Adds

Our system is a **single Node.js script** that Claude Code invokes via the hook
system. It does three things:

1. **Reads shell commands** from a markdown file (`.claude/hooks.md`)
2. **Runs them** one by one, capturing output
3. **Writes a structured log** to a file and a summary to stdout

That's it. The rest is just Claude Code's built-in hook system doing its job.

### Why a script instead of inline commands?

Claude Code hooks support one command per hook entry. If you want to run 8
commands and log all their results in a structured way, you either need 8
separate hook entries or one script that orchestrates them. The script is the
orchestrator.

### Why embed commands in markdown instead of the script itself?

So you can put project-specific commands in a file that's easy to read and edit
without touching JavaScript. The markdown file is the configuration; the script
is the engine.

## File Map

```
.claude/
├── settings.json                  # Layer 1: Tells Claude Code WHEN to run our script
├── hooks.md                       # Layer 2: Contains the shell commands TO run
├── scripts/
│   └── chat-init-hooks.mjs        # Layer 2: The engine that parses hooks.md and runs commands
├── agent_init.md                  # Output: Log from the last SessionStart run (gitignored)
└── agent_end.md                   # Output: Log from the last SessionEnd run (gitignored)
```

### settings.json (the trigger)

This file tells Claude Code: "when a session starts, run this script."

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "node \"$CLAUDE_PROJECT_DIR/.claude/scripts/chat-init-hooks.mjs\" --phase pre"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "node \"$CLAUDE_PROJECT_DIR/.claude/scripts/chat-init-hooks.mjs\" --phase post"
          }
        ]
      }
    ]
  }
}
```

Empty matcher `""` = fires on every SessionStart source (startup, resume,
compact, clear) and every SessionEnd reason.

### hooks.md (the commands)

Shell commands are embedded in HTML comment blocks. HTML comments are invisible
when the markdown renders, so the file can also contain documentation.

```markdown
<!-- @hook:pre
git branch --show-current
git log --oneline -5
git status --short
-->

# This part is documentation, ignored by the script

Any markdown content here. Explain what the hooks do, etc.

<!-- @hook:post
git status --short
git log --oneline -1
-->
```

**Syntax rules:**
- `<!-- @hook:pre ... -->` must be the very first thing in the file
- `<!-- @hook:post ... -->` must be the very last thing in the file
- One shell command per line
- Empty lines and lines starting with `#` are skipped
- Both blocks are optional

The position anchoring (first/last) is enforced by regex so that example syntax
in the documentation section of the file doesn't accidentally get parsed as real
hooks.

### chat-init-hooks.mjs (the engine)

Invoked with `--phase pre` or `--phase post`. Does this:

```
1. Read stdin (JSON from Claude Code -- session_id, source, cwd)
2. Read .claude/hooks.md
3. Find the <!-- @hook:pre --> or <!-- @hook:post --> block
4. Parse out the shell commands
5. For each command:
   a. Run it via sh -c (Unix) or cmd /c (Windows)
   b. Capture stdout, stderr, exit code, duration
   c. If it fails, record the error and continue (unless --fail-fast)
6. Render all results as a markdown table
7. Write that markdown to .claude/agent_init.md (pre) or .claude/agent_end.md (post)
8. Write a one-line summary to stdout
9. Exit 0 if all commands succeeded, 1 if any failed
```

### agent_init.md / agent_end.md (the output)

Generated files. Overwritten each run. Gitignored. Contain a structured log:

```markdown
# Agent Init Log

| Field | Value |
|-------|-------|
| Timestamp | 2026-02-06T15:58:26.164Z |
| Phase | pre |
| Platform | linux (linux x64) |
| Session ID | 0a8aec4b-... |
| Session Source | resume |

## Command Execution

### 1. `git log --oneline -5`
Status: SUCCESS (exit 0) | Duration: 24ms
(output)

### 2. `git status --short`
Status: SUCCESS (exit 0) | Duration: 18ms
(output)

## Summary
| # | Command | Status | Duration |
| 1 | git log | SUCCESS | 24ms |
| 2 | git status | SUCCESS | 18ms |

Result: SUCCESS (2/2)
```

## Complete Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ User opens Claude Code or session resumes                          │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Claude Code fires SessionStart event                               │
│                                                                     │
│ Reads .claude/settings.json                                         │
│ Finds: SessionStart → matcher "" → run chat-init-hooks.mjs         │
│                                                                     │
│ Pipes to stdin:                                                     │
│   {"session_id":"0a8a...","source":"resume","cwd":"/home/user/..."} │
│                                                                     │
│ Executes: node .claude/scripts/chat-init-hooks.mjs --phase pre     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│ chat-init-hooks.mjs runs                                           │
│                                                                     │
│ 1. Reads stdin → gets session_id, source                           │
│ 2. Reads .claude/hooks.md                                           │
│ 3. Regex finds <!-- @hook:pre ... --> at top of file               │
│ 4. Parses 8 shell commands                                         │
│ 5. Runs each via sh -c, captures everything                       │
│ 6. Writes full log → .claude/agent_init.md                         │
│ 7. Writes to stdout: "[chat-init-hooks] pre phase: SUCCESS (8/8)" │
│ 8. Exits 0                                                         │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Claude Code receives exit 0 + stdout                               │
│                                                                     │
│ Injects stdout into Claude's context as a system-reminder:         │
│   "SessionStart:resume hook success:                                │
│    [chat-init-hooks] pre phase: SUCCESS (8/8)"                     │
│                                                                     │
│ Claude can now see the summary. The full log is in agent_init.md   │
│ if Claude (or the user) needs to read it.                          │
└─────────────────────────────────────────────────────────────────────┘
```

The same flow happens for SessionEnd → `--phase post` → `<!-- @hook:post -->` →
`agent_end.md`.

## The --file and --scan Flags

The script can also parse hook blocks from other markdown files:

```bash
# Parse hooks from a specific slash command file (in addition to hooks.md)
node chat-init-hooks.mjs --phase pre --file .claude/commands/IDEA_WORKFLOW.md

# Parse hooks from ALL files in .claude/commands/
node chat-init-hooks.mjs --phase pre --scan
```

This means any slash command file can have its own `<!-- @hook:pre -->` and
`<!-- @hook:post -->` blocks. But currently nothing in settings.json triggers
these -- they'd need to be invoked manually or via additional hook configuration.

## The Module Export Angle

The script guards its CLI entry point behind a check:

```javascript
if (process.argv[1] === __filename) {
  main();  // only runs when executed directly
}
```

All core functions are exported:

```javascript
import { parseHookBlock, executeHookPhase, renderMarkdownLog, collectHooks } from './chat-init-hooks.mjs';
```

This means a Node.js server can import the functions directly and call them
programmatically without spawning a child process:

```javascript
// Example: an event listener on a server that manages Claude Code sessions
server.on('session:start', (session) => {
  const { commands, sourceFiles } = collectHooks('pre', projectDir);
  const result = executeHookPhase({ phase: 'pre', commands, workingDir: projectDir });
  const log = renderMarkdownLog(result);
  // store log, send to dashboard, etc.
});
```

## Known Issues in Current Implementation

### 1. Exit code 1 silences error output

Per Claude Code docs: only exit 0 stdout gets injected into context. The script
exits 1 when any command fails, which means the failure summary it writes to
stdout never reaches Claude. Claude only sees errors when everything succeeds.

**Fix:** Always exit 0. Put error information in the stdout summary. Use the
structured JSON output format if needed.

### 2. Compaction is not handled distinctly

`compact` is a SessionStart source. When it fires, Claude just lost all context.
This is the most important moment to re-inject state, but the current system
treats it identically to startup/resume. Ideally the script should detect
`source: "compact"` from the stdin JSON and output richer context (project
conventions, current task, branch state).

### 3. The Stop hook references a nonexistent script

settings.json has a Stop hook pointing to `~/.claude/stop-hook-git-check.sh`
which doesn't exist. It fails silently on every Claude response.

### 4. Default hook commands are bash-only

The hooks.md commands use `$(uname -s)`, `$(pwd)`, etc. These fail on Windows
with `cmd /c`. The script itself is cross-platform, but the default commands
are not.

### 5. Two redundant SessionStart entries

Separate entries for `startup` and `resume` that run the same command. One entry
with `"matcher": ""` does the same thing.
