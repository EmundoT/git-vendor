# Prompt: Set 4 — Output Format Standardization (Spec 030)

## Concurrency
CONFLICTS with Sets 2 and 3 (all touch main.go).
SAFE with Sets 1 and 5.

## Branch
Create and work on a branch named `claude/set4-output-format-<session-suffix>`.

## Context
Some commands already support `--format table|json`: `verify`, `scan`, `drift`, `license`, `sbom`.
Other commands only output TUI-formatted text: `list`, `status`, `check-updates`, `diff`, `validate`.

Spec 072 added JSON output for config subcommands (create, show, check, etc.) via `CLIResponse` in `cli_response.go`. The older commands predate this and need updating.

## Task
Add `--format table|json` support to all commands that currently lack it.

### Commands to update

1. **`list`** (main.go, currently uses tui.PrintVendorList):
   JSON output should be the VendorConfig struct serialized directly:

        {
          "vendors": [
            {"name": "foo", "url": "...", "license": "MIT", "specs": [...]}
          ]
        }

2. **`status`** (main.go, currently uses tui.PrintSyncStatus):
   JSON output should be the SyncStatus struct:

        {
          "in_sync": true,
          "details": [{"vendor": "foo", "status": "synced", ...}]
        }

3. **`check-updates`** (main.go, currently uses tui.PrintUpdateResults):
   JSON output should be the []UpdateCheckResult slice:

        {
          "updates": [
            {"name": "foo", "current_ref": "v1.0", "latest_commit": "abc123", "has_update": true}
          ]
        }

4. **`diff`** (main.go, currently uses tui.PrintVendorDiff):
   JSON output should be the []VendorDiff slice:

        {
          "vendor": "foo",
          "diffs": [
            {"ref": "main", "from_commit": "abc", "to_commit": "def", "commits": [...]}
          ]
        }

5. **`validate`** (main.go, currently prints text):
   JSON output:

        {
          "valid": true,
          "conflicts": [],
          "errors": []
        }

### Implementation pattern

Follow the existing `verify` command as the reference pattern (main.go ~line 683):

    format := "table" // default
    for _, arg := range os.Args[2:] {
        if strings.HasPrefix(arg, "--format=") {
            format = strings.TrimPrefix(arg, "--format=")
        } else if arg == "--format" {
            // next arg is the format value
        }
    }

    // ... execute command ...

    if format == "json" {
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        enc.Encode(result)
    } else {
        tui.PrintResult(result)
    }

### Also add --output flag

For commands with potentially large output, add `--output <file>` / `-o <file>` to write to a file instead of stdout. Follow the `sbom` command's pattern.

### Testing

No new test files needed for this — the format switching is in main.go which is intentionally untested. But DO verify:
- `go build` succeeds
- `go test ./...` passes (no broken interfaces)
- `go vet ./...` clean

If any types need new methods (e.g., `types.SyncStatus` needs a JSON tag), update the type definition and ensure existing tests still pass.

### What NOT to change
- Commands that already have --format: `verify`, `scan`, `drift`, `license`, `sbom`
- Spec 072 subcommands (already have JSON via CLIResponse)
- `init`, `add`, `edit`, `remove`, `watch`, `completion` — interactive/side-effect commands

### Definition of Done
1. `go build` succeeds
2. `go test ./...` passes
3. `go vet ./...` clean
4. All display-oriented commands support `--format table|json`
5. JSON output uses proper struct serialization (not string formatting)
6. CLAUDE.md updated: note --format flag availability in Quick Reference
7. Commit and push
