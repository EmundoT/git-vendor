#!/usr/bin/env node

/**
 * session-hooks.mjs
 *
 * Orchestrator for Claude Code lifecycle hooks. Adds two things on top of
 * Claude Code's native hook system:
 *
 *   1. Structured persistent logs (.claude/logs/session-init.md, session-end.md)
 *   2. Compaction recovery (PreCompact snapshots state → compact re-injects it)
 *
 * Claude Code invokes this via settings.json hook entries. The script reads
 * the hook's stdin JSON, runs project-specific diagnostics, writes a log file,
 * and prints a summary to stdout (which Claude Code injects into context on
 * exit 0).
 *
 * All core functions are exported for use as a Node.js module.
 *
 * USAGE (called by Claude Code, not directly):
 *   node session-hooks.mjs <event> [--compact-recovery]
 *
 * EVENTS:
 *   init               Normal session start (startup/resume)
 *   init --compact-recovery   Post-compaction session start (reads snapshot)
 *   pre-compact         Snapshot state before compaction destroys context
 *   end                 Session end
 *
 * EXIT CODES:
 *   Always 0. Errors are reported in stdout so Claude can see them.
 *   (Exit != 0 causes Claude Code to discard stdout.)
 */

import { execSync } from "node:child_process";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  existsSync,
  unlinkSync,
} from "node:fs";
import { join, dirname } from "node:path";
import { platform, hostname, arch } from "node:os";
import { fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";

// ─── Paths ───────────────────────────────────────────────────────

const LOGS_DIR = ".claude/logs";
const INIT_LOG = ".claude/logs/session-init.md";
const END_LOG = ".claude/logs/session-end.md";
const COMPACT_SNAPSHOT = ".claude/logs/pre-compact-state.md";

// ─── Core: Run a command ─────────────────────────────────────────

/**
 * Run a single shell command. Returns structured result.
 *
 * @param {string} command
 * @param {string} cwd
 * @param {number} [timeout=30000]
 * @returns {{ command: string, ok: boolean, exitCode: number, output: string, durationMs: number }}
 */
export function run(command, cwd, timeout = 30_000) {
  const start = performance.now();
  try {
    const output = execSync(command, {
      cwd,
      timeout,
      encoding: "utf-8",
      shell: process.platform === "win32" ? "cmd.exe" : "/bin/sh",
      stdio: ["pipe", "pipe", "pipe"],
    });
    return {
      command,
      ok: true,
      exitCode: 0,
      output: (output || "").trimEnd(),
      durationMs: Math.round(performance.now() - start),
    };
  } catch (err) {
    return {
      command,
      ok: false,
      exitCode: err.status ?? 1,
      output: ((err.stdout || "") + (err.stderr || "")).trimEnd(),
      durationMs: Math.round(performance.now() - start),
    };
  }
}

/**
 * Run multiple commands sequentially. Returns all results.
 *
 * @param {string[]} commands
 * @param {string} cwd
 * @returns {Array<{ command: string, ok: boolean, exitCode: number, output: string, durationMs: number }>}
 */
export function runAll(commands, cwd) {
  return commands.map((cmd) => run(cmd, cwd));
}

// ─── Core: Render log ────────────────────────────────────────────

/**
 * Render command results + metadata as a structured markdown log.
 *
 * @param {Object} params
 * @param {string} params.title
 * @param {Object} params.meta - Key-value pairs for the metadata table
 * @param {Array} params.results - From runAll()
 * @param {string} [params.extra] - Additional markdown appended after results
 * @returns {string}
 */
export function renderLog({ title, meta, results, extra }) {
  const lines = [`# ${title}`, ""];

  // Metadata table
  lines.push("| Field | Value |");
  lines.push("|-------|-------|");
  for (const [key, value] of Object.entries(meta)) {
    lines.push(`| **${key}** | ${value} |`);
  }
  lines.push("");

  if (results.length === 0 && !extra) {
    lines.push("_No commands executed._");
    return lines.join("\n");
  }

  // Command results
  if (results.length > 0) {
    lines.push("---", "", "## Commands", "");

    for (let i = 0; i < results.length; i++) {
      const r = results[i];
      const status = r.ok ? "OK" : `FAIL (exit ${r.exitCode})`;
      lines.push(`**${i + 1}. \`${r.command}\`** — ${status}, ${fmt(r.durationMs)}`);
      if (r.output) {
        lines.push("```text", r.output, "```");
      }
      lines.push("");
    }

    // Summary line
    const ok = results.filter((r) => r.ok).length;
    const total = results.length;
    const totalMs = results.reduce((s, r) => s + r.durationMs, 0);
    lines.push(`**${ok}/${total} succeeded** in ${fmt(totalMs)}`);
    lines.push("");
  }

  if (extra) {
    lines.push("---", "", extra, "");
  }

  return lines.join("\n");
}

function fmt(ms) {
  return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
}

// ─── Core: Build metadata from stdin JSON ────────────────────────

/**
 * Build a metadata object from the stdin JSON + environment.
 *
 * @param {Object|null} input - Parsed stdin JSON from Claude Code
 * @returns {Object} Key-value pairs
 */
export function buildMeta(input) {
  const meta = {};
  meta["Timestamp"] = `\`${new Date().toISOString()}\``;
  meta["Platform"] = `\`${process.platform} ${arch()}\``;
  meta["Node"] = `\`${process.version}\``;
  meta["Hostname"] = `\`${hostname()}\``;
  meta["Environment"] = process.env.CLAUDE_CODE_REMOTE === "true"
    ? "Cloud"
    : "Local";
  if (input?.cwd) meta["Directory"] = `\`${input.cwd}\``;
  if (input?.session_id) meta["Session"] = `\`${input.session_id}\``;
  if (input?.source) meta["Source"] = `\`${input.source}\``;
  if (input?.model) meta["Model"] = `\`${input.model}\``;
  if (input?.reason) meta["Reason"] = `\`${input.reason}\``;
  return meta;
}

// ─── Events ──────────────────────────────────────────────────────

/**
 * Session init: run diagnostics, write log, return summary for Claude.
 */
export function handleInit(input, cwd) {
  const commands = [
    "git branch --show-current",
    "git log --oneline -5 --decorate",
    "git status --short",
    "git diff --stat origin/main...HEAD 2>/dev/null || echo '(no upstream comparison available)'",
  ];

  const results = runAll(commands, cwd);
  const meta = buildMeta(input);

  const log = renderLog({
    title: "Session Init",
    meta,
    results,
  });

  writeLog(INIT_LOG, log, cwd);
  return log;
}

/**
 * Session init after compaction: re-inject state from snapshot.
 */
export function handleCompactRecovery(input, cwd) {
  const commands = [
    "git branch --show-current",
    "git log --oneline -5 --decorate",
    "git status --short",
  ];

  const results = runAll(commands, cwd);
  const meta = buildMeta(input);

  // Read the pre-compact snapshot if it exists
  let snapshot = null;
  const snapshotPath = join(cwd, COMPACT_SNAPSHOT);
  if (existsSync(snapshotPath)) {
    try {
      snapshot = readFileSync(snapshotPath, "utf-8");
    } catch {
      snapshot = "_Failed to read pre-compact snapshot._";
    }
  }

  const extra = snapshot
    ? `## Pre-Compaction Context\n\nThe following was captured before compaction:\n\n${snapshot}`
    : "## Pre-Compaction Context\n\n_No snapshot available. This is the first compaction or the snapshot was lost._";

  const log = renderLog({
    title: "Session Init (post-compaction recovery)",
    meta,
    results,
    extra,
  });

  writeLog(INIT_LOG, log, cwd);
  return log;
}

/**
 * PreCompact: snapshot current state before context is destroyed.
 * Reads the transcript to extract what Claude was working on.
 */
export function handlePreCompact(input, cwd) {
  // Gather current state
  const commands = [
    "git branch --show-current",
    "git log --oneline -3",
    "git diff --name-only HEAD 2>/dev/null || true",
  ];

  const results = runAll(commands, cwd);

  const branch = results[0]?.output || "unknown";
  const recentCommits = results[1]?.output || "none";
  const dirtyFiles = results[2]?.output || "none";

  // Try to extract recent context from transcript
  let taskContext = "";
  if (input?.transcript_path && existsSync(input.transcript_path)) {
    try {
      taskContext = extractRecentContext(input.transcript_path);
    } catch {
      taskContext = "_Could not parse transcript._";
    }
  }

  const snapshot = [
    `**Branch:** \`${branch}\``,
    `**Trigger:** \`${input?.trigger || "unknown"}\``,
    `**Time:** \`${new Date().toISOString()}\``,
    "",
    "### Recent commits",
    "```text",
    recentCommits,
    "```",
    "",
    "### Uncommitted changes",
    "```text",
    dirtyFiles || "(clean working tree)",
    "```",
  ];

  if (taskContext) {
    snapshot.push("", "### What was being worked on", "", taskContext);
  }

  const content = snapshot.join("\n");
  writeLog(COMPACT_SNAPSHOT, content, cwd);

  // Stdout — Claude Code adds this to context for the compaction summary
  return content;
}

/**
 * Session end: capture final state.
 */
export function handleEnd(input, cwd) {
  const commands = [
    "git branch --show-current",
    "git log --oneline -1 --decorate",
    "git status --short",
  ];

  const results = runAll(commands, cwd);
  const meta = buildMeta(input);

  const log = renderLog({
    title: "Session End",
    meta,
    results,
  });

  writeLog(END_LOG, log, cwd);
  return log;
}

// ─── Helpers ─────────────────────────────────────────────────────

function writeLog(relPath, content, cwd) {
  const fullPath = join(cwd, relPath);
  mkdirSync(dirname(fullPath), { recursive: true });
  writeFileSync(fullPath, content, "utf-8");
}

/**
 * Extract recent assistant context from transcript JSONL.
 * Reads the last N lines looking for the most recent assistant messages
 * to summarize what Claude was working on.
 */
function extractRecentContext(transcriptPath) {
  const content = readFileSync(transcriptPath, "utf-8");
  const lines = content.trim().split("\n");

  // Take last 50 lines, look for assistant messages
  const recent = lines.slice(-50);
  const summaryParts = [];

  for (const line of recent) {
    try {
      const entry = JSON.parse(line);
      if (entry.type === "assistant" && entry.message?.content) {
        // Extract text content blocks
        const textBlocks = Array.isArray(entry.message.content)
          ? entry.message.content
              .filter((b) => b.type === "text")
              .map((b) => b.text)
          : [String(entry.message.content)];

        for (const text of textBlocks) {
          // Take first 200 chars of each text block
          if (text.trim()) {
            summaryParts.push(text.trim().slice(0, 200));
          }
        }
      }
    } catch {
      // Skip malformed lines
    }
  }

  if (summaryParts.length === 0) return "";

  // Return last 3 assistant messages, deduplicated
  const unique = [...new Set(summaryParts)].slice(-3);
  return unique.map((s) => `> ${s}${s.length >= 200 ? "..." : ""}`).join("\n\n");
}

// ─── Stdin reader ────────────────────────────────────────────────

function readStdin() {
  try {
    if (process.stdin.isTTY) return null;
    const data = readFileSync(0, "utf-8");
    return data.trim() ? JSON.parse(data) : null;
  } catch {
    return null;
  }
}

// ─── CLI entry point ─────────────────────────────────────────────

function main() {
  const event = process.argv[2];
  const flags = new Set(process.argv.slice(3));

  if (!event || !["init", "pre-compact", "end"].includes(event)) {
    process.stdout.write(
      "Usage: node session-hooks.mjs <init|pre-compact|end> [--compact-recovery]\n"
    );
    process.exit(0);
  }

  const input = readStdin();
  const cwd = input?.cwd || process.env.CLAUDE_PROJECT_DIR || process.cwd();

  let output;
  switch (event) {
    case "init":
      output = flags.has("--compact-recovery")
        ? handleCompactRecovery(input, cwd)
        : handleInit(input, cwd);
      break;
    case "pre-compact":
      output = handlePreCompact(input, cwd);
      break;
    case "end":
      output = handleEnd(input, cwd);
      break;
  }

  // Always write to stdout, always exit 0
  if (output) process.stdout.write(output);
  process.exit(0);
}

// ─── Module guard ────────────────────────────────────────────────

const __filename = fileURLToPath(import.meta.url);
if (process.argv[1] && process.argv[1] === __filename) {
  main();
}
