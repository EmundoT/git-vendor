# 072: LLM-Friendly CLI

## Summary

Add CLI commands that allow LLMs (and humans) to manage vendored dependencies without directly editing YAML configuration files.

## Dependencies

- 071: Position Extraction (for position specifier support in commands)

## Problem

LLMs interacting with git-vendor must currently:

1. Read vendor.yml to understand current state
2. Mentally parse nested YAML structure
3. Edit YAML correctly (indentation, structure, escaping)
4. Avoid introducing subtle syntax errors

This is error-prone and context-inefficient. LLMs excel at running commands with flags and reading structured output.

## Solution

Expose all configuration operations as CLI commands with JSON output support.

## Commands

### Vendor Management

```bash
# Add a new vendor
git-vendor create <name> <url> [--ref <ref>] [--license <license>]

# Remove a vendor
git-vendor delete <name>

# Rename a vendor
git-vendor rename <old-name> <new-name>

# List vendors
git-vendor list [--json]
```

### Mapping Management

```bash
# Add a mapping to a vendor
git-vendor add-mapping <vendor> <from> --to <to> [--ref <ref>]

# Remove a mapping
git-vendor remove-mapping <vendor> <from>

# List mappings for a vendor
git-vendor list-mappings <vendor> [--json]

# Update a mapping's target
git-vendor update-mapping <vendor> <from> --to <new-to>
```

### Inspection Commands

```bash
# Show vendor details
git-vendor show <vendor> [--json]

# Check sync status (is this vendor up to date?)
git-vendor check <vendor> [--json]

# Preview what would be synced
git-vendor preview <vendor> [--json]

# Show diff between local and remote
git-vendor diff <vendor>
```

### Configuration Queries

```bash
# Get a config value
git-vendor config get <key>

# Set a config value
git-vendor config set <key> <value>

# List all config
git-vendor config list [--json]
```

## Examples

### LLM Workflow: Adding a New Dependency

```bash
# LLM runs these commands instead of editing YAML:

$ git-vendor create api-types https://github.com/org/api --ref v2.0.0 --license MIT
Created vendor 'api-types'

$ git-vendor add-mapping api-types src/types/user.ts:L10-L50 --to lib/types/user.ts
Added mapping: src/types/user.ts:L10-L50 -> lib/types/user.ts

$ git-vendor add-mapping api-types src/types/product.ts --to lib/types/product.ts
Added mapping: src/types/product.ts -> lib/types/product.ts

$ git-vendor sync api-types
Synced api-types (2 files)
```

### LLM Workflow: Checking State

```bash
$ git-vendor list --json
{
  "vendors": [
    {"name": "api-types", "url": "https://github.com/org/api", "ref": "v2.0.0", "mappings": 2},
    {"name": "utils", "url": "https://github.com/org/utils", "ref": "main", "mappings": 5}
  ]
}

$ git-vendor check api-types --json
{
  "vendor": "api-types",
  "status": "stale",
  "local_commit": "abc123",
  "remote_commit": "def456",
  "files_changed": 1
}
```

### LLM Workflow: Updating a Mapping

```bash
$ git-vendor list-mappings api-types --json
{
  "vendor": "api-types",
  "mappings": [
    {"from": "src/types/user.ts:L10-L50", "to": "lib/types/user.ts"},
    {"from": "src/types/product.ts", "to": "lib/types/product.ts"}
  ]
}

$ git-vendor update-mapping api-types src/types/user.ts:L10-L50 --to lib/models/user.ts
Updated mapping target: lib/types/user.ts -> lib/models/user.ts
```

## JSON Output Schema

All `--json` output follows a consistent structure:

```typescript
interface CLIResponse {
  success: boolean;
  data?: any;           // Command-specific payload
  error?: {
    code: string;       // Machine-readable error code
    message: string;    // Human-readable message
  };
}
```

Example error:

```json
{
  "success": false,
  "error": {
    "code": "VENDOR_NOT_FOUND",
    "message": "Vendor 'api-types' not found in configuration"
  }
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Vendor not found |
| 3 | Invalid arguments |
| 4 | Config validation failed |
| 5 | Network error |

Consistent exit codes help LLMs determine next steps programmatically.

## Backward Compatibility

- All existing commands remain unchanged
- New commands are additive
- YAML config file remains the source of truth (commands modify it)
- Users can still edit YAML directly if preferred

## Implementation Notes

1. Commands modify vendor.yml through the existing ConfigStore interface
2. JSON output uses Go's encoding/json with consistent field naming
3. Error codes map to existing error types in errors.go
4. Tab completion extended for new commands (bash/zsh/fish/powershell)

## Out of Scope

- GraphQL or REST API (CLI is the interface)
- Interactive prompts for these commands (use existing TUI for interactive mode)
- Batch operations in single command (run multiple commands instead)
