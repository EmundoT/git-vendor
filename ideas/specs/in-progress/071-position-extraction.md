# 071: Position Extraction

## Summary

Extend git-vendor path mappings to support extracting specific line/column ranges from source files, not just whole files or directories.

## Dependencies

None (foundation spec)

## Problem

Current `from` paths only support file or directory granularity:

```yaml
mapping:
  - from: src/schema.json
    to: vendor/schema.json
```

This forces users to vendor entire files when they only need a portion - a single function, a constant block, or a specific section.

## Solution

Add position specifiers to the `from` path syntax:

```yaml
mapping:
  - from: src/config.go:L15-L30        # Lines 15-30
  - from: src/types.ts:L42             # Single line
  - from: src/constants.py:L10-EOF     # Line 10 to end of file
  - from: src/api.rs:L5C20:L5C45       # Column-precise (single line)
  - from: src/schema.json:L8C3:L15C20  # Column-precise (multi-line)
```

## Syntax

```
file_path:position_specifier

position_specifier:
  L{n}                  # Single line
  L{n}-L{m}             # Line range
  L{n}:L{m}             # Line range (alt syntax)
  L{n}-EOF              # Line to end of file
  L{n}C{c}:L{m}C{d}     # Column-precise range
```

Where:
- `L` = line (1-indexed)
- `C` = column (1-indexed)
- `EOF` = end of file

## Behavior

### Extraction

When syncing, git-vendor extracts only the specified range from the source file:

```go
// Source: api/constants.go
package api

const (
    Version = "1.0.0"    // L4
    MaxRetries = 3       // L5
    Timeout = 30         // L6
)

func init() { ... }
```

With mapping `from: api/constants.go:L4-L6`, the vendored content is:

```go
    Version = "1.0.0"
    MaxRetries = 3
    Timeout = 30
```

### Placement

The `to` path can also include position specifiers for replacement targets:

```yaml
mapping:
  - from: api/constants.go:L4-L6
    to: lib/config.ts:L10-L12
```

This replaces lines 10-12 in the target with lines 4-6 from the source.

If `to` has no position specifier, the extracted content replaces the entire file (current behavior).

### Position Tracking in Lockfile

The lockfile records exact positions at sync time:

```yaml
locks:
  - name: api-constants
    ref: main
    commit_hash: abc123
    positions:
      - from: api/constants.go:L4-L6
        to: lib/config.ts:L10-L12
        source_hash: sha256:def456  # Hash of extracted content
```

This enables exact-match verification even when source file changes outside the extracted range.

## Validation

### On Sync

1. Validate position specifiers parse correctly
2. Validate ranges exist in source file (L30 in a 20-line file = error)
3. Validate target ranges exist if specified
4. Warn if extracted content differs from what's at target position

### On Verify

Compare extracted content hash against lockfile. Position drift in source file (e.g., lines inserted above) causes verification failure - user must re-sync.

## Edge Cases

| Case | Behavior |
|------|----------|
| Source file shorter than specified range | Error: "Line 30 does not exist in file (25 lines)" |
| Column exceeds line length | Error: "Column 80 exceeds line length (45 chars)" |
| Target position doesn't match content length | Replace range regardless of length mismatch |
| Empty range (L5-L5 or L5:L5) | Extract single line |
| Overlapping ranges in same file | Allowed (user's responsibility) |
| Binary files | Not supported, error on detection |

## CLI Impact

Existing commands work unchanged. Position specifiers are part of the path string:

```bash
# Via LLM-friendly CLI (spec 072)
git-vendor add-mapping constants api/constants.go:L4-L6 --to lib/config.ts:L10-L12

# Sync works as normal
git-vendor sync constants
```

## Implementation Notes

1. **Parsing**: Extend path parsing to detect `:L` suffix and extract position info
2. **Extraction**: Read file, slice to range, compute hash of slice
3. **Placement**: If target has position, read file, replace range, write file
4. **Lockfile**: Store positions alongside existing fields

## Out of Scope

- Semantic extraction (e.g., "extract function `foo`") - too language-specific
- Regex-based extraction - complex and fragile
- Named anchors/markers - covered in spec 073 (vendor variables)
