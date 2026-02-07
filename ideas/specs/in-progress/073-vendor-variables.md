# 073: Vendor Variables + File Watcher

## Summary

Enable inline vendor variable syntax in any file that gets automatically expanded by a file watcher, creating a symlink-like experience for vendored content.

## Dependencies

- 071: Position Extraction (for position specifier syntax)

## Problem

Current git-vendor workflow requires:

1. Define mapping in vendor.yml
2. Run `git-vendor sync`
3. Content appears in target file

This is manual overhead that breaks flow, especially for LLMs generating code. The experience should be as seamless as a symlink - content just appears where you reference it.

## Solution

### Vendor Variable Syntax

Place vendor variables directly in any file:

```typescript
// Any file
const API_VERSION = $v{api-config:L5};

interface User {
  $v{user-schema:L10-L25}
}
```

### File Watcher

`git-vendor watch` monitors files and expands vendor variables on save:

1. Detect file save
2. Scan for `$v{...}` patterns
3. Look up vendor in config
4. Extract content from source at specified position
5. Replace variable with content
6. Re-save file
7. Record mapping in vendor.yml

## Syntax

```
$v{vendor_name:position}

Examples:
$v{schema:L5}              # Line 5 from 'schema' vendor
$v{schema:L5-L20}          # Lines 5-20
$v{schema:L4C10:L4C30}     # Column-precise range
$v{config:L10-EOF}         # Line 10 to end of file
```

The vendor name must exist in vendor.yml with a configured source URL.

## Workflow

### Initial Setup

```yaml
# vendor.yml
vendors:
  - name: api-schema
    url: https://github.com/org/api
    license: MIT
    specs:
      - ref: main
        # No mappings yet - they're created by watch
```

### Developer/LLM Writes Code

```typescript
// src/types.ts
export interface User {
  $v{api-schema:L15-L30}
}
```

### On Save

```bash
$ git-vendor watch
Watching for changes...
[src/types.ts] Expanded $v{api-schema:L15-L30} -> 15 lines
```

File becomes:

```typescript
// src/types.ts
export interface User {
  id: string;
  name: string;
  email: string;
  // ... lines 15-30 from api-schema source
}
```

Config updated:

```yaml
vendors:
  - name: api-schema
    url: https://github.com/org/api
    specs:
      - ref: main
        mapping:
          - from: src/schema.ts:L15-L30
            to: src/types.ts:L2-L17
```

## Bidirectional Sync

Sync direction is configurable at three levels:

### 1. Install Default

```bash
git-vendor config set sync.direction source-to-derived  # default
git-vendor config set sync.direction bidirectional
```

### 2. Project Override

```yaml
# vendor.yml
settings:
  sync_direction: bidirectional
```

### 3. Per-Mapping Override

```yaml
mapping:
  - from: src/schema.ts:L15-L30
    to: src/types.ts:L2-L17
    sync_direction: source-to-derived  # Override project setting
```

### Bidirectional Behavior

When `bidirectional`:

1. On target file save: check if content differs from source
2. If different: propagate changes back to source file
3. Update lockfile with new content hash
4. If source is remote: stage change locally, warn user to push

```bash
[src/types.ts] Content changed, propagating to api-schema:src/schema.ts:L15-L30
```

## Watch Command

```bash
# Start watcher
git-vendor watch

# Watch specific directories
git-vendor watch src/ lib/

# Watch with verbose output
git-vendor watch --verbose

# Dry run (show what would be expanded)
git-vendor watch --dry-run
```

### Watch Behavior

| Event | Action |
|-------|--------|
| File saved with `$v{...}` | Expand variable, update config |
| File saved with vendored content | Check sync (if bidirectional, propagate) |
| vendor.yml changed | Re-validate, re-sync if needed |
| Source file changed (bidirectional) | Update derived files |

## Error Handling

| Error | Behavior |
|-------|----------|
| Unknown vendor name | Leave `$v{...}` unexpanded, log warning |
| Position out of range | Leave unexpanded, log error |
| Network unavailable | Use cached content if available, warn |
| Circular reference | Error: "Circular vendor reference detected" |

## IDE Interaction

The `$v{...}` syntax is intentionally not valid in any language. This means:

1. IDE shows syntax error
2. Developer saves file
3. Watch expands variable (milliseconds)
4. IDE re-reads file, syntax error gone

This is acceptable because expansion is near-instant. For better IDE experience, see spec 074 (VS Code Extension).

## Lockfile Integration

Expanded vendor variables are tracked in lockfile:

```yaml
locks:
  - name: api-schema
    ref: main
    commit_hash: abc123
    positions:
      - from: src/schema.ts:L15-L30
        to: src/types.ts:L2-L17
        content_hash: sha256:def456
```

`git-vendor verify` validates these positions still match.

## Security

- Watch only expands content from configured vendors
- Cannot reference arbitrary files (must be in vendor config)
- Cannot escape project directory (path validation per spec 071)
- Remote content fetched via existing git clone (respects auth)

## Performance

- File watcher uses fsnotify (already a dependency)
- Debounce rapid saves (100ms default)
- Cache expanded content in memory during session
- Only scan files matching configurable patterns (default: all non-binary)

## Configuration

```yaml
# vendor.yml
settings:
  watch:
    debounce_ms: 100
    patterns:
      - "*.ts"
      - "*.go"
      - "*.py"
    ignore:
      - "node_modules/**"
      - "vendor/**"
```

## Out of Scope

- Running as system daemon (user starts/stops watch)
- IDE integration (spec 074)
- Transforms/preprocessing (content copied as-is)
