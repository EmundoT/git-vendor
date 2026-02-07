# 074: VS Code Extension

## Summary

VS Code extension that integrates git-vendor functionality directly into the editor, eliminating the syntax error flash from vendor variables and providing inline vendor management.

## Dependencies

- 073: Vendor Variables + File Watcher (core functionality to wrap)

## Problem

With spec 073, vendor variables like `$v{schema:L5-L20}` cause momentary syntax errors in the IDE:

1. Developer types `$v{...}`
2. IDE highlights syntax error (red squiggles)
3. Developer saves
4. Watch expands variable
5. IDE re-reads file, error clears

This works but creates friction. A proper IDE integration can:
- Suppress/style vendor variable "errors"
- Show preview of vendored content on hover
- Provide autocomplete for vendor names
- Manage watch lifecycle

## Features

### 1. Vendor Variable Recognition

The extension recognizes `$v{...}` patterns and:

- Styles them distinctly (not as errors)
- Shows as collapsed/foldable region
- Displays inline hint with vendor name

```typescript
// Before extension: red squiggles
interface User {
  $v{api-schema:L15-L30}  // <-- IDE shows error
}

// With extension: styled as special syntax
interface User {
  $v{api-schema:L15-L30}  // <-- Dim purple, no error, hover shows preview
}
```

### 2. Hover Preview

Hovering over a vendor variable shows:

```
┌─────────────────────────────────────────┐
│ api-schema:L15-L30                      │
│ Source: github.com/org/api/schema.ts    │
│ ─────────────────────────────────────── │
│   id: string;                           │
│   name: string;                         │
│   email: string;                        │
│   ...                                   │
│ ─────────────────────────────────────── │
│ [Expand] [Go to Source] [Edit Mapping]  │
└─────────────────────────────────────────┘
```

### 3. Autocomplete

Typing `$v{` triggers autocomplete:

```
$v{|
    ├── api-schema
    ├── utils
    └── config
```

Selecting a vendor shows position autocomplete:

```
$v{api-schema:|
    ├── L1-L50 (full file)
    ├── L15-L30 (interface User)
    └── L35-L40 (interface Product)
```

Position suggestions derived from analyzing source file structure.

### 4. Integrated Watch

Extension manages `git-vendor watch` lifecycle:

- Auto-start watch when opening project with vendor.yml
- Status bar indicator: `$(sync) git-vendor`
- Click to show watch output panel
- Auto-restart on crash

```
┌─────────────────────────────────────────────────┐
│ STATUS BAR                                      │
│ ... $(sync) git-vendor watching · 3 vendors ... │
└─────────────────────────────────────────────────┘
```

### 5. Inline Expansion Toggle

Command palette / keyboard shortcut to toggle between:

- **Collapsed**: Show `$v{...}` syntax
- **Expanded**: Show actual vendored content (read-only overlay)

```typescript
// Collapsed view
interface User {
  $v{api-schema:L15-L30}
}

// Expanded view (toggle with Cmd+Shift+V)
interface User {
  id: string;           // ┐
  name: string;         // │ vendored from api-schema:L15-L30
  email: string;        // │ (read-only)
  createdAt: Date;      // ┘
}
```

### 6. CodeLens Actions

Above each vendor variable:

```typescript
// [Sync] [Diff] [Go to Source]
$v{api-schema:L15-L30}
```

- **Sync**: Force re-sync this mapping
- **Diff**: Show diff between local and remote
- **Go to Source**: Open source file at position (clones if needed)

### 7. Problems Panel Integration

Vendor-related issues appear in Problems panel:

```
PROBLEMS
├── api-schema: Content out of sync (3 lines differ)
├── utils: Source position invalid (file shortened)
└── config: Vendor not found in vendor.yml
```

### 8. Command Palette

```
> Git Vendor: Add Vendor
> Git Vendor: Add Mapping
> Git Vendor: Sync All
> Git Vendor: Sync Current File
> Git Vendor: Check for Updates
> Git Vendor: Show Watch Output
> Git Vendor: Toggle Variable Expansion
> Git Vendor: Go to Vendor Config
```

## Settings

```json
{
  "gitVendor.autoStartWatch": true,
  "gitVendor.expandOnHover": true,
  "gitVendor.showCodeLens": true,
  "gitVendor.variableStyle": "dimmed",  // or "highlighted", "underlined"
  "gitVendor.statusBar.show": true
}
```

## Extension Architecture

```
┌─────────────────────────────────────────────┐
│              VS Code Extension              │
├─────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────────────┐  │
│  │  Language   │  │   Watch Manager      │  │
│  │  Features   │  │   (spawns CLI)       │  │
│  │  - Hover    │  └──────────────────────┘  │
│  │  - Complete │                            │
│  │  - CodeLens │  ┌──────────────────────┐  │
│  └─────────────┘  │   Config Watcher     │  │
│                   │   (vendor.yml)       │  │
│  ┌─────────────┐  └──────────────────────┘  │
│  │  Decorator  │                            │
│  │  Provider   │  ┌──────────────────────┐  │
│  │  ($v style) │  │   CLI Interface      │  │
│  └─────────────┘  │   (git-vendor cmds)  │  │
│                   └──────────────────────┘  │
└─────────────────────────────────────────────┘
                      │
                      ▼
              ┌───────────────┐
              │ git-vendor    │
              │ CLI binary    │
              └───────────────┘
```

The extension wraps the CLI - it doesn't reimplement git-vendor logic. This ensures consistency and reduces maintenance burden.

## Installation

```bash
# Extension depends on git-vendor CLI being installed
code --install-extension git-vendor.git-vendor

# Or via marketplace
ext install git-vendor
```

Extension checks for CLI on activation:

```
git-vendor not found. [Install] [Configure Path] [Dismiss]
```

## File Associations

Extension activates for any file type in a project with `vendor/vendor.yml`. No new file extensions needed.

## Telemetry

Optional, anonymized usage metrics:
- Extension activation count
- Feature usage (hover, autocomplete, codelens)
- Error rates

All telemetry respects VS Code's global telemetry setting.

## Future Considerations

- **JetBrains Plugin**: Similar functionality for IntelliJ/WebStorm/GoLand
- **Neovim Plugin**: Lua plugin with LSP integration
- **LSP Server**: Language-agnostic server that any editor can use

## Out of Scope

- Implementing git-vendor logic in TypeScript (use CLI)
- Supporting editors other than VS Code (separate specs)
- Vendor creation wizard (use CLI or existing TUI)
