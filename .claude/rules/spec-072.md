---
paths:
  - "internal/core/config_commands.go"
  - "internal/core/config_commands_test.go"
  - "internal/core/cli_response.go"
---

# LLM-Friendly CLI (Spec 072)

Non-interactive CLI commands for programmatic vendor management. All support --json for structured output.

## JSON Output Schema

Success:
```json
{"success": true, "data": { ... }}
```

Error:
```json
{"success": false, "error": {"code": "VENDOR_NOT_FOUND", "message": "..."}}
```

## Commands

| Command | Purpose |
|---------|---------|
| create <name> <url> [--ref] [--license] | Add vendor non-interactively (config-only, no sync) |
| delete <name> | Remove vendor (alias for remove) |
| rename <old> <new> | Rename across config/lock/license |
| add-mapping <vendor> <from> --to <to> [--ref] | Add path mapping |
| remove-mapping <vendor> <from> | Remove path mapping by source |
| list-mappings <vendor> | List mappings for vendor |
| update-mapping <vendor> <from> --to <new-to> | Change mapping destination |
| show <vendor> | Detailed vendor info (config + lockfile) |
| check <vendor> | Per-vendor sync status |
| preview <vendor> | Preview what would be synced |
| config list | List all config key-value pairs |
| config get <key> | Get config value |
| config set <key> <value> | Set config value |

## Exit Codes

0=Success, 1=General error, 2=Vendor not found, 3=Invalid arguments, 4=Config validation failed, 5=Network error

## Error Codes

VENDOR_NOT_FOUND, VENDOR_EXISTS, MAPPING_NOT_FOUND, MAPPING_EXISTS, INVALID_ARGUMENTS, NOT_INITIALIZED, CONFIG_ERROR, VALIDATION_FAILED, NETWORK_ERROR, INTERNAL_ERROR, REF_NOT_FOUND, INVALID_KEY

## Config Key Format

`vendors.<name>.<field>` where field is: url, license, ref, groups, name (read-only)

## Key Functions

- CreateVendorEntry() -- non-interactive vendor creation
- RenameVendor() -- rename across config, lockfile, license file
- AddMappingToVendor() / RemoveMappingFromVendor() / UpdateMappingInVendor()
- ShowVendor() -- combined config + lockfile info
- GetConfigValue() / SetConfigValue() -- dotted key-path config access
- CheckVendorStatus() -- per-vendor sync status
- EmitCLISuccess() / EmitCLIError() -- JSON output helpers
- CLIExitCodeForError() / CLIErrorCodeForError() -- error-to-code mappers
