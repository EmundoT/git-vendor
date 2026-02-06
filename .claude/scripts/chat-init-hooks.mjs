#!/usr/bin/env node

/**
 * chat-init-hooks.mjs
 *
 * Cross-platform Claude Code chat initialization hook executor.
 * Parses pre/post hook blocks from markdown files (slash commands, hooks.md)
 * and executes embedded shell commands with structured logging.
 *
 * HOOK BLOCK SYNTAX (embed in any .md file):
 *
 *   <!-- @hook:pre
 *   command1
 *   command2
 *   -->
 *
 *   ...markdown content...
 *
 *   <!-- @hook:post
 *   command3
 *   command4
 *   -->
 *
 * Rules:
 *   - @hook:pre block goes at the TOP of the file
 *   - @hook:post block goes at the BOTTOM of the file
 *   - Both blocks are optional
 *   - Each line is a shell command, executed sequentially
 *   - Empty lines are ignored
 *   - Lines starting with # are comments (ignored)
 *   - Commands run via sh -c (Unix) or cmd /c (Windows)
 *
 * USAGE:
 *   node chat-init-hooks.mjs --phase pre  [--file path] [--scan] [--fail-fast]
 *   node chat-init-hooks.mjs --phase post [--file path] [--scan] [--fail-fast]
 *
 * OPTIONS:
 *   --phase <pre|post>   Required. Which hook phase to execute.
 *   --file <path>        Parse hooks from a specific command file.
 *   --scan               Also scan all .claude/commands/*.md files.
 *   --fail-fast          Stop execution on first command failure.
 *   --quiet              Suppress command output to stderr during execution.
 *
 * OUTPUT:
 *   Pre-phase log  -> .claude/agent_init.md (replaces previous content)
 *   Post-phase log -> .claude/agent_end.md  (replaces previous content)
 *   Summary + errors -> stdout (added to Claude's context by hook system)
 *
 * ENVIRONMENT:
 *   CLAUDE_PROJECT_DIR   Project root directory (set by Claude Code)
 *   CLAUDE_CODE_REMOTE   "true" when running in cloud (Anthropic VMs)
 *   CLAUDE_SESSION_ID    Current session ID (if available)
 *
 * EXIT CODES:
 *   0  All commands succeeded (or no commands found)
 *   1  One or more commands failed
 *   2  Configuration or parse error
 *
 * NODE SERVER ACTION PATTERN:
 *   All core functions are exported for use as a module:
 *
 *   import { parseHookBlock, executeHookPhase, renderMarkdownLog } from './chat-init-hooks.mjs';
 *
 *   app.post('/hooks/:phase', async (req, res) => {
 *     const result = executeHookPhase({
 *       phase: req.params.phase,
 *       commands: parseHookBlock(fileContent, req.params.phase),
 *       workingDir: projectRoot,
 *     });
 *     const log = renderMarkdownLog(result);
 *     res.json({ log, result });
 *   });
 */

import { execSync } from "node:child_process";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  existsSync,
  readdirSync,
} from "node:fs";
import { join, resolve, dirname } from "node:path";
import { platform, hostname, arch } from "node:os";
import { fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";

// ─── Constants ───────────────────────────────────────────────────

// Pre block MUST be at the very start of the file (enforces "top of file" rule).
// Post block MUST be at the very end of the file (enforces "bottom of file" rule).
// This prevents matching example/documentation blocks in the middle of the file.
// The post pattern uses a greedy prefix ([\s\S]*) to skip to the LAST occurrence.
const HOOK_PRE_PATTERN = /^\s*<!--\s*@hook:pre\s*\n([\s\S]*?)-->/;
const HOOK_POST_PATTERN = /[\s\S]*<!--\s*@hook:post\s*\n([\s\S]*?)-->\s*$/;
const DEFAULT_HOOKS_FILE = ".claude/hooks.md";
const COMMANDS_DIR = ".claude/commands";
const OUTPUT_PRE_FILE = ".claude/agent_init.md";
const OUTPUT_POST_FILE = ".claude/agent_end.md";
const DEFAULT_CMD_TIMEOUT = 60_000; // 60 seconds per command

// ─── Core: Parse Hook Block ─────────────────────────────────────

/**
 * Extract hook commands from a markdown file content.
 *
 * @param {string} content - Markdown file content
 * @param {'pre'|'post'} phase - Which hook block to extract
 * @returns {string[]} Array of shell commands (empty if no block found)
 */
export function parseHookBlock(content, phase) {
  const pattern = phase === "pre" ? HOOK_PRE_PATTERN : HOOK_POST_PATTERN;
  const match = content.match(pattern);
  if (!match) return [];

  return match[1]
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#"));
}

// ─── Core: Execute Commands ─────────────────────────────────────

/**
 * Execute a single shell command synchronously, capturing all output.
 * Uses platform-appropriate shell (sh -c on Unix, cmd /c on Windows).
 *
 * @param {string} command - Shell command to execute
 * @param {string} workingDir - Working directory for execution
 * @param {Object} [options]
 * @param {number} [options.timeout=60000] - Timeout in milliseconds
 * @param {boolean} [options.quiet=false] - Suppress output to stderr
 * @returns {{ command: string, status: string, exitCode: number, stdout: string, error: string, durationMs: number }}
 */
export function executeCommand(command, workingDir, options = {}) {
  const { timeout = DEFAULT_CMD_TIMEOUT, quiet = false } = options;
  const start = performance.now();

  try {
    const output = execSync(command, {
      cwd: workingDir,
      timeout,
      encoding: "utf-8",
      shell: process.platform === "win32" ? "cmd.exe" : "/bin/sh",
      env: { ...process.env },
      stdio: ["pipe", "pipe", "pipe"],
    });

    const durationMs = Math.round(performance.now() - start);

    if (!quiet && output) {
      process.stderr.write(output);
    }

    return {
      command,
      status: "SUCCESS",
      exitCode: 0,
      stdout: output || "",
      error: "",
      durationMs,
    };
  } catch (err) {
    const durationMs = Math.round(performance.now() - start);
    const output = (err.stdout || "") + (err.stderr || "");

    if (!quiet && output) {
      process.stderr.write(output);
    }

    return {
      command,
      status: "FAILED",
      exitCode: err.status ?? 1,
      stdout: output,
      error: err.message,
      durationMs,
    };
  }
}

/**
 * Execute a sequence of hook commands for a given phase.
 * Primary entry point for both CLI and node server action usage.
 *
 * @param {Object} params
 * @param {'pre'|'post'} params.phase - Hook phase
 * @param {string[]} params.commands - Shell commands to execute
 * @param {string} params.workingDir - Working directory
 * @param {string[]} [params.sourceFiles=[]] - Files hooks were parsed from
 * @param {boolean} [params.failFast=false] - Stop on first failure
 * @param {boolean} [params.quiet=false] - Suppress command output
 * @param {Object} [params.sessionInput=null] - Claude Code hook stdin JSON
 * @returns {{ phase: string, timestamp: string, platform: string, hostname: string, nodeVersion: string, cwd: string, sourceFiles: string[], results: Object[], overall: string, totalDurationMs: number, sessionInput: Object|null, isRemote: boolean }}
 */
export function executeHookPhase(params) {
  const {
    phase,
    commands,
    workingDir,
    sourceFiles = [],
    failFast = false,
    quiet = false,
    sessionInput = null,
  } = params;

  const timestamp = new Date().toISOString();
  const startTime = performance.now();
  const results = [];

  for (let i = 0; i < commands.length; i++) {
    const cmd = commands[i];

    if (failFast && results.some((r) => r.status === "FAILED")) {
      results.push({
        command: cmd,
        index: i + 1,
        status: "SKIPPED",
        exitCode: -1,
        stdout: "",
        error: "Skipped due to prior failure (--fail-fast)",
        durationMs: 0,
      });
      continue;
    }

    const result = executeCommand(cmd, workingDir, { quiet });
    result.index = i + 1;
    results.push(result);
  }

  const totalDurationMs = Math.round(performance.now() - startTime);
  const succeeded = results.filter((r) => r.status === "SUCCESS").length;
  const total = results.length;

  let overall;
  if (total === 0) overall = "EMPTY";
  else if (succeeded === total) overall = "SUCCESS";
  else if (succeeded === 0) overall = "FAILED";
  else overall = "PARTIAL";

  return {
    phase,
    timestamp,
    platform: `${process.platform} (${platform()} ${arch()})`,
    hostname: hostname(),
    nodeVersion: process.version,
    cwd: workingDir,
    sourceFiles,
    results,
    overall,
    totalDurationMs,
    sessionInput,
    isRemote: process.env.CLAUDE_CODE_REMOTE === "true",
  };
}

// ─── Rendering ───────────────────────────────────────────────────

/**
 * Format milliseconds as a human-readable duration.
 * @param {number} ms
 * @returns {string}
 */
function formatDuration(ms) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

/**
 * Render execution results as structured markdown.
 * Output serves as a provenance log revealing environment state,
 * command results, and session context.
 *
 * @param {{ phase: string, timestamp: string, platform: string, hostname: string, nodeVersion: string, cwd: string, sourceFiles: string[], results: Object[], overall: string, totalDurationMs: number, sessionInput: Object|null, isRemote: boolean }} result
 * @returns {string} Markdown content
 */
export function renderMarkdownLog(result) {
  const lines = [];
  const title = result.phase === "pre" ? "Agent Init Log" : "Agent End Log";

  lines.push(`# ${title}`);
  lines.push("");
  lines.push("| Field | Value |");
  lines.push("|-------|-------|");
  lines.push(`| **Timestamp** | \`${result.timestamp}\` |`);
  lines.push(`| **Phase** | \`${result.phase}\` |`);
  lines.push(`| **Platform** | \`${result.platform}\` |`);
  lines.push(`| **Node** | \`${result.nodeVersion}\` |`);
  lines.push(`| **Hostname** | \`${result.hostname}\` |`);
  lines.push(`| **Working Directory** | \`${result.cwd}\` |`);
  lines.push(
    `| **Environment** | ${result.isRemote ? "Cloud (`CLAUDE_CODE_REMOTE=true`)" : "Local"} |`
  );
  lines.push(
    `| **Source Files** | ${result.sourceFiles.length > 0 ? result.sourceFiles.map((f) => `\`${f}\``).join(", ") : "_none_"} |`
  );

  if (result.sessionInput) {
    if (result.sessionInput.session_id) {
      lines.push(
        `| **Session ID** | \`${result.sessionInput.session_id}\` |`
      );
    }
    if (result.sessionInput.source) {
      lines.push(
        `| **Session Source** | \`${result.sessionInput.source}\` |`
      );
    }
    if (result.sessionInput.model) {
      lines.push(`| **Model** | \`${result.sessionInput.model}\` |`);
    }
  }

  lines.push("");
  lines.push("---");
  lines.push("");

  if (result.results.length === 0) {
    lines.push("_No hook commands found for this phase._");
    lines.push("");
    return lines.join("\n");
  }

  lines.push("## Command Execution");
  lines.push("");

  for (const r of result.results) {
    lines.push(`### ${r.index}. \`${r.command}\``);
    lines.push("");
    lines.push(
      `**Status:** ${r.status} (exit ${r.exitCode}) | **Duration:** ${formatDuration(r.durationMs)}`
    );
    lines.push("");

    if (r.stdout && r.stdout.trim()) {
      lines.push("```text");
      lines.push(r.stdout.trimEnd());
      lines.push("```");
      lines.push("");
    }

    if (r.status === "FAILED" && r.error) {
      lines.push(`> **Error:** ${r.error.split("\n")[0]}`);
      lines.push("");
    }

    if (r.status === "SKIPPED") {
      lines.push(`> ${r.error}`);
      lines.push("");
    }
  }

  lines.push("---");
  lines.push("");
  lines.push("## Summary");
  lines.push("");
  lines.push("| # | Command | Status | Duration |");
  lines.push("|---|---------|--------|----------|");

  for (const r of result.results) {
    const cmdShort =
      r.command.length > 60 ? r.command.substring(0, 57) + "..." : r.command;
    lines.push(
      `| ${r.index} | \`${cmdShort}\` | ${r.status} | ${formatDuration(r.durationMs)} |`
    );
  }

  lines.push("");

  const succeeded = result.results.filter(
    (r) => r.status === "SUCCESS"
  ).length;
  const failed = result.results.filter((r) => r.status === "FAILED").length;
  const skipped = result.results.filter((r) => r.status === "SKIPPED").length;
  const total = result.results.length;

  lines.push(
    `**Result:** ${result.overall} (${succeeded}/${total} succeeded${failed ? `, ${failed} failed` : ""}${skipped ? `, ${skipped} skipped` : ""})`
  );
  lines.push(`**Total Duration:** ${formatDuration(result.totalDurationMs)}`);
  lines.push("");

  return lines.join("\n");
}

// ─── Hook Collection ─────────────────────────────────────────────

/**
 * Collect hook commands from applicable files.
 *
 * Resolution order:
 *   1. .claude/hooks.md (default global hooks, always checked)
 *   2. --file <path> (specific command file, if provided)
 *   3. --scan (all .claude/commands/*.md files, if requested)
 *
 * @param {'pre'|'post'} phase
 * @param {string} workingDir
 * @param {Object} [options]
 * @param {string} [options.file] - Specific file path to parse
 * @param {boolean} [options.scan] - Scan all command files
 * @returns {{ commands: string[], sourceFiles: string[] }}
 */
export function collectHooks(phase, workingDir, options = {}) {
  const { file, scan } = options;
  const commands = [];
  const sourceFiles = [];

  // 1. Default hooks file (always checked)
  const defaultPath = join(workingDir, DEFAULT_HOOKS_FILE);
  if (existsSync(defaultPath)) {
    const content = readFileSync(defaultPath, "utf-8");
    const cmds = parseHookBlock(content, phase);
    if (cmds.length > 0) {
      commands.push(...cmds);
      sourceFiles.push(DEFAULT_HOOKS_FILE);
    }
  }

  // 2. Specific file (if provided and different from default)
  if (file) {
    const filePath = resolve(workingDir, file);
    const normalDefault = resolve(workingDir, DEFAULT_HOOKS_FILE);
    if (filePath !== normalDefault && existsSync(filePath)) {
      const content = readFileSync(filePath, "utf-8");
      const cmds = parseHookBlock(content, phase);
      if (cmds.length > 0) {
        commands.push(...cmds);
        sourceFiles.push(file);
      }
    }
  }

  // 3. Scan all command files (if requested)
  if (scan) {
    const commandsDir = join(workingDir, COMMANDS_DIR);
    if (existsSync(commandsDir)) {
      const files = readdirSync(commandsDir)
        .filter((f) => f.endsWith(".md"))
        .sort();
      for (const f of files) {
        const filePath = join(commandsDir, f);
        const content = readFileSync(filePath, "utf-8");
        const cmds = parseHookBlock(content, phase);
        if (cmds.length > 0) {
          commands.push(...cmds);
          sourceFiles.push(join(COMMANDS_DIR, f));
        }
      }
    }
  }

  return { commands, sourceFiles };
}

// ─── Stdin Reader ────────────────────────────────────────────────

/**
 * Read Claude Code hook input JSON from stdin (synchronous, non-blocking).
 * Claude Code pipes a JSON payload to stdin for all hook commands.
 *
 * @returns {Object|null} Parsed JSON input, or null if unavailable
 */
function readStdinJSON() {
  try {
    if (process.stdin.isTTY) return null;
    const data = readFileSync(0, "utf-8");
    if (data && data.trim()) {
      return JSON.parse(data);
    }
  } catch {
    // stdin not available, empty, or not valid JSON
  }
  return null;
}

// ─── CLI Entry Point ─────────────────────────────────────────────

function main() {
  const args = process.argv.slice(2);

  // Parse arguments
  let phase = null;
  let file = null;
  let scan = false;
  let failFast = false;
  let quiet = false;

  for (let i = 0; i < args.length; i++) {
    switch (args[i]) {
      case "--phase":
        phase = args[++i];
        break;
      case "--file":
        file = args[++i];
        break;
      case "--scan":
        scan = true;
        break;
      case "--fail-fast":
        failFast = true;
        break;
      case "--quiet":
        quiet = true;
        break;
      case "--help":
      case "-h":
        process.stdout.write(
          [
            "Usage: node chat-init-hooks.mjs --phase <pre|post> [options]",
            "",
            "Options:",
            "  --phase <pre|post>  Required. Hook phase to execute.",
            "  --file <path>       Parse hooks from a specific .md file.",
            "  --scan              Also scan all .claude/commands/*.md files.",
            "  --fail-fast         Stop on first command failure.",
            "  --quiet             Suppress command output during execution.",
            "  --help, -h          Show this help message.",
            "",
            "Output files:",
            "  .claude/agent_init.md   Pre-phase execution log",
            "  .claude/agent_end.md    Post-phase execution log",
            "",
          ].join("\n")
        );
        process.exit(0);
        break;
    }
  }

  if (!phase || !["pre", "post"].includes(phase)) {
    process.stderr.write(
      "Error: --phase <pre|post> is required. Use --help for usage.\n"
    );
    process.exit(2);
  }

  const workingDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();

  // Read hook input from stdin (Claude Code pipes JSON here)
  const sessionInput = readStdinJSON();

  // Collect commands from hook files
  const { commands, sourceFiles } = collectHooks(phase, workingDir, {
    file,
    scan,
  });

  // Execute all commands
  const result = executeHookPhase({
    phase,
    commands,
    workingDir,
    sourceFiles,
    failFast,
    quiet,
    sessionInput,
  });

  // Render structured markdown log
  const log = renderMarkdownLog(result);

  // Write log to output file (replace any old content)
  const outputFile = phase === "pre" ? OUTPUT_PRE_FILE : OUTPUT_POST_FILE;
  const outputPath = join(workingDir, outputFile);
  mkdirSync(dirname(outputPath), { recursive: true });
  writeFileSync(outputPath, log, "utf-8");

  // Write summary to stdout (becomes part of Claude's context via hook system)
  if (result.results.length > 0) {
    const succeeded = result.results.filter(
      (r) => r.status === "SUCCESS"
    ).length;
    const total = result.results.length;
    process.stdout.write(
      `[chat-init-hooks] ${phase} phase: ${result.overall} (${succeeded}/${total})\n`
    );

    // On failure, surface error details so Claude can see them
    const failures = result.results.filter((r) => r.status === "FAILED");
    if (failures.length > 0) {
      process.stdout.write("\nFailed commands:\n");
      for (const f of failures) {
        process.stdout.write(`  - ${f.command} (exit ${f.exitCode})\n`);
        if (f.stdout && f.stdout.trim()) {
          const lastLines = f.stdout.trim().split("\n").slice(-5).join("\n  ");
          process.stdout.write(`    ${lastLines}\n`);
        }
      }
      process.stdout.write(`\nFull log written to: ${outputFile}\n`);
    }
  } else {
    process.stdout.write(
      `[chat-init-hooks] ${phase} phase: no hooks configured\n`
    );
  }

  // Exit 0 for success/empty, 1 for any failures
  process.exit(result.overall === "SUCCESS" || result.overall === "EMPTY" ? 0 : 1);
}

// ─── Module Guard ────────────────────────────────────────────────
// Only run CLI when executed directly; skip when imported as a module.

const __filename = fileURLToPath(import.meta.url);
if (process.argv[1] && resolve(process.argv[1]) === resolve(__filename)) {
  main();
}
