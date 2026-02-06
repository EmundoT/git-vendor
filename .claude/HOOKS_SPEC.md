# Session Hooks Spec

## What this is

A Node.js script that Claude Code calls at session boundaries. It adds two
things that Claude Code doesn't provide natively:

1. **Persistent structured logs** — written to `.claude/logs/` so the next
   session, an external tool, or a future server can read them.
2. **Compaction recovery** — when Claude's context window fills up and gets
   compacted, Claude forgets everything. This system snapshots state before
   compaction and re-injects it after.

Everything else (when hooks fire, how stdin/stdout works, how context injection
works) is Claude Code's native hook system. We configure it, we don't replace it.

## Files

```
.claude/
├── settings.json                   # Wires Claude Code events to the script
├── hooks/
│   └── session-hooks.mjs           # The script (orchestrator + module exports)
└── logs/                           # Generated, gitignored
    ├── session-init.md             # Log from last session start
    ├── session-end.md              # Log from last session end
    └── pre-compact-state.md        # Snapshot saved before last compaction
```

## How it works

### settings.json wires four events

```
SessionStart (startup|resume)  →  session-hooks.mjs init
SessionStart (compact)         →  session-hooks.mjs init --compact-recovery
PreCompact   (any)             →  session-hooks.mjs pre-compact
SessionEnd   (any)             →  session-hooks.mjs end
```

Claude Code pipes JSON to stdin for every hook. The script reads it, runs git
diagnostics, writes a log file, and prints output to stdout. Claude Code injects
stdout into Claude's context when the script exits 0.

### Normal session start

```
Claude Code fires SessionStart {source: "startup"}
    → script runs: git branch, git log, git status, git diff
    → writes .claude/logs/session-init.md
    → prints the same content to stdout
    → Claude Code injects it into context
    → Claude knows the branch, recent commits, dirty files
```

### Compaction recovery

```
Context window fills up
    → Claude Code fires PreCompact {trigger: "auto", transcript_path: "..."}
    → script reads transcript, extracts what Claude was working on
    → runs git branch, git log, git diff --name-only
    → writes snapshot to .claude/logs/pre-compact-state.md
    → Claude Code compacts (context is destroyed)

    → Claude Code fires SessionStart {source: "compact"}
    → script runs git diagnostics (same as normal init)
    → ALSO reads .claude/logs/pre-compact-state.md
    → appends the snapshot as "Pre-Compaction Context"
    → prints everything to stdout
    → Claude Code injects it into now-empty context
    → Claude recovers awareness of branch, task, and dirty files
```

### Session end

```
User exits
    → Claude Code fires SessionEnd {reason: "prompt_input_exit"}
    → script runs: git branch, git log, git status
    → writes .claude/logs/session-end.md
    → log persists on disk for next session or external tools
```

### Command-specific hooks

Not handled by this script. Use native YAML frontmatter in command files:

```yaml
---
name: idea-workflow
hooks:
  SessionStart:
    - hooks:
        - type: command
          command: "git stash list && git log --oneline -3 origin/main..HEAD"
          once: true
---
```

Claude Code manages these natively — they activate when the command is invoked
and deactivate when it's done.

## Module exports

All functions are exported for use in a Node.js server:

```javascript
import { run, runAll, renderLog, buildMeta,
         handleInit, handleCompactRecovery,
         handlePreCompact, handleEnd } from './session-hooks.mjs';
```

| Function | What it does |
|----------|-------------|
| `run(cmd, cwd)` | Run one command, return `{ok, exitCode, output, durationMs}` |
| `runAll(cmds, cwd)` | Run N commands sequentially, return array of results |
| `renderLog({title, meta, results, extra})` | Render results as structured markdown |
| `buildMeta(input)` | Build metadata object from Claude Code stdin JSON |
| `handleInit(input, cwd)` | Full init flow: diagnostics + log + return summary |
| `handleCompactRecovery(input, cwd)` | Init + read pre-compact snapshot |
| `handlePreCompact(input, cwd)` | Snapshot branch, commits, dirty files, transcript context |
| `handleEnd(input, cwd)` | Full end flow: final state + log |

## Design decisions

**Always exits 0.** Claude Code only injects stdout into context on exit 0.
Exiting 1 would silently discard everything. Errors are reported in the output
itself.

**Git commands only.** The init diagnostics are four git commands. No language
toolchain checks, no env dumps. Fast, focused, useful.

**Snapshot reads transcript.** The PreCompact handler receives `transcript_path`
(a JSONL file with the full conversation). It parses the last 50 lines looking
for assistant messages to capture what Claude was working on. This is the piece
that makes compaction recovery work beyond just "here's the branch name."

**Logs go to .claude/logs/.** Separate directory, gitignored. Clean separation
from configuration files in .claude/.
