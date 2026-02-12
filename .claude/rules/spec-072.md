---
paths:
  - "internal/core/config_commands.go"
  - "internal/core/config_commands_test.go"
  - "internal/core/cli_response.go"
  - "main.go"
---

# LLM-Friendly CLI (Spec 072)

## Why This Exists

git-vendor's primary interface is an interactive TUI (charmbracelet/huh wizards). Spec 072 adds a parallel non-interactive CLI layer for programmatic use: LLM agents, shell scripts, CI/CD pipelines, and any context where stdin is unavailable or structured output is needed.

These commands MUST remain non-interactive — no prompts, no wizards, no confirmation dialogs. They either succeed or return a structured error. The --json flag on any Spec 072 command switches output from human text to machine-parseable JSON.

## When to Use Spec 072 vs TUI

| Scenario | Use |
|----------|-----|
| Human adding a vendor interactively | TUI (`git-vendor add`) |
| LLM agent creating a vendor from parsed requirements | Spec 072 (`git-vendor create`) |
| Script automating vendor setup in CI | Spec 072 |
| Human browsing remote files to pick paths | TUI (`git-vendor edit`) |
| Programmatic path mapping manipulation | Spec 072 (`add-mapping`, `remove-mapping`) |
| Reading vendor state for automation | Spec 072 (`show`, `check`, `config get`) |

## Design Constraints

- **Config-only, no sync**: `create` writes to vendor.yml but does NOT trigger sync/update. The caller decides when to sync.
- **Atomic config writes**: Each command reads config, validates, writes. No partial state.
- **Deterministic exit codes**: Scripts can branch on exit code without parsing output.
- **Error codes for machines, messages for humans**: JSON errors include both a stable code (VENDOR_NOT_FOUND) and a human-readable message.

## JSON Output Contract

Success:
```json
{"success": true, "data": { ... }}
```

Error:
```json
{"success": false, "error": {"code": "VENDOR_NOT_FOUND", "message": "vendor 'foo' not found in config"}}
```

## Commands

| Command | Purpose |
|---------|---------|
| `create <name> <url> [--ref] [--license]` | Add vendor to config (no sync) |
| `delete <name>` | Remove vendor (alias for `remove`) |
| `rename <old> <new>` | Rename across config, lockfile, license file |
| `add-mapping <vendor> <from> --to <to> [--ref]` | Add path mapping to vendor |
| `remove-mapping <vendor> <from>` | Remove path mapping by source path |
| `list-mappings <vendor>` | List all mappings for a vendor |
| `update-mapping <vendor> <from> --to <new-to>` | Change mapping destination |
| `show <vendor>` | Detailed vendor info (config + lockfile combined) |
| `check <vendor>` | Per-vendor sync status |
| `preview <vendor>` | Preview what sync would do |
| `config list` | List all config key-value pairs |
| `config get <key>` | Get config value by dotted path |
| `config set <key> <value>` | Set config value by dotted path |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Vendor not found |
| 3 | Invalid arguments |
| 4 | Config validation failed |
| 5 | Network error |

## Error Codes

VENDOR_NOT_FOUND, VENDOR_EXISTS, MAPPING_NOT_FOUND, MAPPING_EXISTS, INVALID_ARGUMENTS, NOT_INITIALIZED, CONFIG_ERROR, VALIDATION_FAILED, NETWORK_ERROR, INTERNAL_ERROR, REF_NOT_FOUND, INVALID_KEY

## Config Key Format

Dotted path: `vendors.<name>.<field>` where field is: url, license, ref, groups, name (read-only).

## Implementation Notes

- CLIResponse / CLIErrorDetail types in cli_response.go
- EmitCLISuccess() / EmitCLIError() write JSON to stdout
- CLIExitCodeForError() / CLIErrorCodeForError() map Go errors to stable codes
- Config manipulation methods live on VendorSyncer (config_commands.go), not Manager — they're lower-level operations that bypass the facade
- 35 unit tests in config_commands_test.go cover all methods
