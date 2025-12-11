# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`git-vendor` is a CLI tool for managing vendored dependencies from Git repositories. It provides an interactive TUI for selecting specific files/directories from remote repos and syncing them to your local project with deterministic locking.

**Key Concept**: Unlike traditional vendoring tools, git-vendor allows granular path mapping - you can vendor specific files or subdirectories from a remote repository to specific locations in your project.

## Building and Running

```bash
# Build the project
go build -o git-vendor

# Run directly
./git-vendor <command>

# Or use go run
go run main.go <command>
```

## Core Architecture

### Three-Layer Structure

1. **main.go** - Command dispatcher and CLI interface
   - Routes commands (init, add, edit, remove, list, sync, update)
   - Handles argument parsing and basic validation
   - Entry point for all user interactions

2. **internal/core/engine.go** - Business logic and Git operations
   - `Manager` struct manages all vendor operations
   - Handles Git cloning, fetching, and checkout operations
   - Performs file/directory copying from temp clones to local paths
   - Manages configuration and lock file I/O
   - License detection via GitHub API
   - Smart URL parsing for deep links (blob/tree URLs)

3. **internal/tui/wizard.go** - Interactive user interface
   - Built with charmbracelet/huh (form library) and lipgloss (styling)
   - Multi-step wizards for add/edit operations
   - File browser for both remote (via git ls-tree) and local directories
   - Path mapping management interface

### Data Model (internal/types/types.go)

```
VendorConfig (vendor.yml)
  └─ VendorSpec (one per dependency)
      ├─ Name: display name
      ├─ URL: git repository URL
      ├─ License: SPDX license identifier
      └─ Specs: []BranchSpec (can track multiple refs)
          └─ BranchSpec
              ├─ Ref: branch/tag/commit
              ├─ DefaultTarget: optional default destination
              └─ Mapping: []PathMapping
                  └─ PathMapping
                      ├─ From: remote path
                      └─ To: local path (empty = auto)

VendorLock (vendor.lock)
  └─ LockDetails (one per ref per vendor)
      ├─ Name: vendor name
      ├─ Ref: branch/tag
      ├─ CommitHash: exact commit SHA
      ├─ LicensePath: path to cached license
      └─ Updated: timestamp
```

### File System Structure

All vendor-related files live in `./vendor/`:
```
vendor/
├── vendor.yml       # Configuration file
├── vendor.lock      # Lock file with commit hashes
└── licenses/        # Cached license files
    └── {name}.txt
```

Vendored files are copied to paths specified in the configuration (outside vendor/ directory).

## Key Operations

### sync vs update

- **sync**: Fetches dependencies at locked commit hashes (deterministic)
  - If no lockfile exists, runs `update` first
  - Uses `--depth 1` for shallow clones when possible
  - Supports `--dry-run` flag for preview

- **update**: Fetches latest commits and regenerates lockfile
  - Updates all vendors to latest available commit on their configured ref
  - Rewrites entire lockfile
  - Downloads and caches license files

### Smart URL Parsing

The `ParseSmartURL` function (engine.go:57) extracts repository, ref, and path from GitHub URLs:
- `github.com/owner/repo` → base URL, no ref, no path
- `github.com/owner/repo/blob/main/path/to/file.go` → base URL, "main", "path/to/file.go"
- `github.com/owner/repo/tree/v1.0/src/` → base URL, "v1.0", "src/"

### Remote Directory Browsing

Uses `git ls-tree` to browse remote repository contents without full checkout (engine.go:69):
1. Clone with `--filter=blob:none --no-checkout --depth 1`
2. Fetch specific ref if needed
3. Run `git ls-tree` to list directory contents
4. 30-second timeout protection via context

### License Compliance

Automatic license detection via GitHub API (engine.go:431):
- Allowed by default: MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC, Unlicense, CC0-1.0
- Other licenses prompt user confirmation
- License files are automatically copied to `vendor/licenses/{name}.txt`

## Common Patterns

### Error Handling
- TUI functions use `check(err)` helper that prints "Aborted." and exits
- Core functions return errors for caller handling
- CLI prints styled errors via `tui.PrintError(title, message)`

### Wizard Flow
1. User inputs URL (validates GitHub URLs only)
2. ParseSmartURL extracts components
3. Check if repo already tracked → offer to edit existing
4. Collect name and ref
5. If deep link provided, offer to use that path
6. Enter edit loop for path mapping
7. Save triggers `UpdateAll()` which regenerates lockfile

### Git Operations
- Use `runGit(dir, args...)` for standard operations
- Use `runGitWithContext(ctx, dir, args...)` for operations with timeout
- Temp directories cleaned up with `defer os.RemoveAll(tempDir)`

## Development Notes

### No Tests Currently
There are no test files in the codebase. When adding tests:
- Test Manager operations with mock file system
- Test URL parsing with various GitHub URL formats
- Test git operations may require git test fixtures or mocking

### Dependencies
- `github.com/charmbracelet/huh` - TUI forms
- `github.com/charmbracelet/lipgloss` - styling
- `gopkg.in/yaml.v3` - config file parsing

### Concurrency Considerations
- Git operations use 30-second timeout contexts for directory listing
- No parallel vendor processing (sequential in UpdateAll)
- File copying is synchronous

## Gotchas

1. **GitHub-only**: Smart URL parsing and license detection only work with GitHub
2. **Shallow clones**: Uses `--depth 1` which may fail for locked commit hashes not in recent history (falls back to full fetch)
3. **License location**: Checks LICENSE, LICENSE.txt, and COPYING in repository root only
4. **Path mapping**: Empty destination ("To" field) uses auto-naming based on source basename
5. **Edit mode**: When editing existing vendor, changes aren't saved until user selects "💾 Save & Exit"
