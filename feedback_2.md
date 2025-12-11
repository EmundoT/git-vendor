# git-vendor UI/UX Feedback

**Test Date:** 2025-12-10
**Tester:** Claude Code Assistant
**Version:** git-vendor v5.0

## Executive Summary

I tested the git-vendor CLI tool by exploring its commands, reading the codebase, and analyzing the user workflows. The tool demonstrates a well-thought-out interactive TUI using the charmbracelet/huh library with a clean multi-step wizard approach. Below are my observations organized by component.

---

## Positive Highlights

### 1. **Clear Command Structure**

- The help output is concise and well-formatted
- Commands follow intuitive naming conventions (`init`, `add`, `edit`, `remove`, `list`, `sync`, `update`)
- Navigation instructions are provided upfront ("Use arrow keys to navigate, Enter to select, Ctrl+C to cancel")

### 2. **Smart URL Parsing**

- The `ParseSmartURL` function elegantly handles GitHub deep links (blob/tree URLs)
- Automatically extracts repository, ref, and path from complex URLs
- Reduces user friction when adding dependencies from browser

### 3. **Dry-Run Support**

- `sync --dry-run` provides preview of what will be downloaded
- Shows locked commit hashes for transparency
- Clear messaging: "This is a dry-run. No files were modified"

### 4. **Visual Feedback**

- Color-coded output (green checkmarks, red X's, orange warnings)
- Emoji usage in navigation (📂 folders, 📄 files) enhances scannability
- Styled cards and borders provide visual hierarchy

### 5. **Confirmation Prompts**

- Remove command requires explicit confirmation
- Shows consequences: "This will delete the config entry and license file"
- Prevents accidental destructive operations

### 6. **License Tracking**

- Automatically downloads and stores license files in `vendor/licenses/`
- License verification during vendor addition
- Good compliance support for legal requirements

---

## Areas for Improvement

### 1. **Error Messaging & Recovery**

**Issue:** When sync fails with stale lock hashes, the error message is cryptic:
```
✖ Sync Failed
checkout locked hash ab002f0... failed: fatal: reference is not a tree: ab002f0...
```

**Suggestions:**

- Detect this specific error pattern and provide actionable guidance
- Suggest: "The locked commit may no longer exist. Run `git-vendor update` to fetch the latest commit and try again."
- Consider auto-recovery: offer to run update automatically when this error occurs

### 2. **Progress Indicators for Long Operations**

**Observation:** Commands like `update` and `sync` can take time but provide minimal feedback.

**Current behavior:**
```
• Processing claude-quickstarts...
[long pause with no updates]
```

**Suggestions:**

- Add spinner or progress bar during git operations
- Show intermediate steps: "Cloning repository...", "Checking out ref...", "Copying files..."
- Display elapsed time for operations > 3 seconds
- Use the bubbletea framework's spinner component

### 3. **Wizard Exit Behavior**

**Issue:** When users press Ctrl+C during the wizard, the tool calls `check(err)` which prints "Aborted." and exits with code 1. This is functional but abrupt.

**Suggestions:**

- Catch wizard cancellations gracefully
- Display: "Wizard cancelled. No changes were made."
- Return to main prompt instead of hard exit
- Allow users to save partial progress

### 4. **List Command Output Clarity**

**Current format:**
```
- claude-quickstarts (https://github.com/anthropics/claude-quickstarts)
  @ main
    • autonomous-coding/agent.py ->
    • computer-use-demo -> .
```

**Issues:**

- Empty mapping destination (`->` with nothing after) is confusing
- The `.` destination for "current directory" is not immediately clear
- No indication of sync status (is this downloaded? out of date?)

**Suggestions:**

- Display `(auto)` or `(auto-named)` instead of empty string
- Clarify `.` as `(current directory)` or show actual path
- Add sync status column:
  ```
  - claude-quickstarts [synced ✓] [ee3afd9]
    @ main
      • autonomous-coding/agent.py → agent.py
      • computer-use-demo → ./computer-use-demo
  ```

### 5. **Remote Browser Context Loss**

**Observation:** When browsing remote files, navigating deep into a repository can be disorienting.

**Suggestions:**

- Show breadcrumb trail: `owner/repo / src / components / Button.tsx`
- Display current branch/ref being browsed
- Add "Jump to Root" option for quick navigation
- Consider adding search/filter functionality for large directories

### 6. **No Undo for Edit Operations**

**Issue:** During the edit wizard, users can accidentally delete mappings or branches with no way to undo.

**Suggestions:**

- Add "Undo Last Change" option in edit menu
- Show warning before deleting: "This will remove the mapping for X. Continue?"
- Implement a confirmation step before saving changes
- Undo can be used multiple times within the same session, similar to a stack

### 7. **Unclear Auto-Naming Behavior**

**Issue:** When adding paths, users can leave the "Local Target" empty for "automatic naming", but the logic isn't explained.

**Suggestions:**

- Show preview of auto-generated name in the input description
- Show the expected name based on the source path but gray to implicate it's auto-generated
- Example: `Description: "(./agent.py)"`
- Add tooltip explaining the naming convention

### 8. **Timeout Handling**

**Observation:** Context timeouts are set to 30 seconds for directory listing, but there's no user feedback during this time.

**Suggestions:**

- Display timeout countdown for operations approaching limit
- Allow users to cancel and retry with different settings
- Suggest using shallower clones or specific refs for slow connections

---

## Workflow-Specific Feedback

### Adding a Vendor

**Current Flow:**

1. Enter remote URL
2. Confirm/set vendor name
3. Confirm/set git ref
4. (Optional) Track specific path
5. Enter edit wizard
6. Select branch
7. Add/edit mappings
8. Save & exit

**Pain Points:**

- Step 5 (entering edit wizard) feels like a mode switch that wasn't announced
- Users might expect to be "done" after step 4
- The transition message could help: "Now let's configure the exact paths to track..."

**Suggestions:**

- Add progress indicator: "Step 1/3: Repository Info", "Step 2/3: Path Mapping", etc.
- Provide "Skip path configuration" option for simple cases (track entire repo)
- Offer templates: "Track entire repo", "Track single file", "Track multiple paths"

### Editing a Vendor

**Current Flow:**

1. Select vendor from list
2. Select branch to manage
3. Add/edit/delete path mappings
4. Return to branch selection or save

**Strengths:**

- Nested navigation makes sense
- Clear "Back" and "Save & Exit" options

**Suggestions:**

- Add "View Current Config" option to see the full vendor.yml structure
- Show diff preview before saving: "You've added 2 paths and removed 1"
- Offer "Reset to Last Saved" option

### Syncing

**Current Flow:**

1. Run `sync` or `sync --dry-run`
2. Downloads files to configured paths
3. Success or error message

**Suggestions:**

- Add `sync --force` to re-download even if already synced
- Implement `sync <vendor-name>` to sync only specific vendor
- Show what changed: "Downloaded 5 files, updated 2 files, removed 1 file"
- Add `--verbose` flag for detailed operation log

---

## Code Quality Observations

### Strengths:

- Clean separation of concerns (core, tui, types packages)
- Good use of interfaces (VendorManager interface)
- Consistent error handling patterns
- Type safety with strongly-typed vendor specs

### Suggestions:

- Add unit tests for ParseSmartURL edge cases
- Extract magic strings to constants (file paths, error messages)
- Consider adding context cancellation to all git operations
- Document the sync algorithm (how it determines what to download)

---

## Usability Testing Scenarios

I tested the following scenarios:

### ✅ Scenario 1: Initialize and List

- `git-vendor init` → Success
- `git-vendor list` → Shows existing vendor clearly

### ✅ Scenario 2: Sync with Dry-Run

- `git-vendor sync --dry-run` → Clear preview of what will happen

### ⚠️ Scenario 3: Sync with Stale Lock

- `git-vendor sync` → Failed with cryptic error
- `git-vendor update` → Fixed the issue
- `git-vendor sync` → Success
- **Issue:** Two-step recovery process not obvious to users

### 📝 Scenario 4: Interactive Add (Not Fully Tested)

- Could not fully test interactive wizard due to tmux complexity
- Code review suggests solid flow, but needs real-world testing
- Remote browser implementation looks robust

---

## Terminal Compatibility

**Observations:**

- Uses charmbracelet/lipgloss for styling
- Should work well on modern terminals with 256 colors
- Emoji usage may have issues on older terminals

**Suggestions:**

- Add `--no-color` flag for CI/CD environments
- Test on Windows Command Prompt, PowerShell, and WSL
- Provide ASCII-only mode as fallback

---

## Documentation Gaps

Based on my exploration, the following would benefit from documentation:

1. **README or docs/**
   - Architecture overview (how git-vendor works under the hood)
   - Common workflows with screenshots
   - Troubleshooting guide for common errors
   - Comparison with git submodules / go modules

2. **Example Configurations**
   - Show vendor.yml examples for common use cases
   - Multi-branch tracking example
   - Monorepo path mapping example

3. **CLI Reference**
   - Complete command reference with all flags
   - Exit codes and their meanings
   - Environment variables (if any)

---

## Feature Requests (Future Enhancements)

0. **Multi-Vendor Conflict Detection**
   - Warn if multiple vendors map to overlapping paths

1. **Interactive Conflict Resolution**
   - During add/edit, detect path conflicts and offer resolution options

2. **History & Logging**
   - Maintain a log of sync/update operations
   - Allow users to view history of changes per vendor
   - Can use history to restore previous versions of vendor configs or even undo rollbacks/mass undos(think git reflog but for git-vendor)

3. **Custom Scripts Hooks**
   - Pre-sync and post-sync hooks for custom processing
   - Allow users to run scripts after vendor files are updated

4. **Settings Management & Tags**
   - Global config for default behaviors (e.g., default branch, auto-accept licenses)
   - `git-vendor config set <key> <value>` command
   - Custom project-level settings in a `.gitvendorrc` file in vendor folder
   - Support for per-vendor settings overrides in vendor.yml
   - Can access settings in TUI(wizard) to modify behavior during add/edit/sync operations
   - Ability to specify different settings for different environments (dev, staging, production) by using tags
   - Can set vendor.yml settings using tags in vendor.yml ex. `tags: production|{other_tag}|{other_tag}` to apply specific settings when syncing in that environment
   - Tags can be assigned to default categories or user defined categories in settings to default mappings(ex. tags -> system tags -> environment: production, staging, dev | {other system tag : tags} ||| tags -> custom tags -> {user defined tag : tags})
   - Tags can be specified during sync command ex. `git-vendor sync --tags production,staging` to only sync vendors matching those tags
   - Tags integrated into TUI(wizard) to allow users to assign tags during add/edit operations and future filtering based on tags

5. **Dependency Graph Visualization**
   - Show tree view of all vendors and their paths
   - Detect conflicts (multiple vendors mapping to same path)

6. **Version Pinning Modes**
   - Support semantic version tags (fetch latest v1.x.x)
   - Allow branch tracking vs. specific commit tracking

7. **Workspace Support**
   - Multi-project vendor coordination
   - Shared vendor cache to avoid duplicate downloads

8. **Git Hooks Integration**
   - Pre-commit hook to verify all vendors are synced
   - Post-checkout hook to auto-sync

9. **Import from Other Tools**
   - Convert git submodules to git-vendor config
   - Import from go.mod or package.json

10. **Batch Operations**
   - `git-vendor add-bulk config.json` for multiple vendors at once
   - `git-vendor validate` to check config integrity

11. **Undo Stack for Edits**
   - Allow multiple undos in edit wizard
   - Visual history of changes made during session

12. **Enhanced Remote Browsing**
   - Search functionality within remote file browser
   - File previews for common types (README.MD, other .md, source code extensions, configs, jsons, etc.)

---

## Critical Bugs

### Bug 1: Update Command Provides No Feedback
**Severity:** Medium
**Description:** Running `git-vendor update` shows only "• Processing vendor-name..." with no indication of completion or success.
**Expected:** Should show "✔ Updated vendor-name to commit [hash]" or similar confirmation.

### Bug 2: Empty Mapping Destination
**Severity:** Low
**Description:** When mapping destination is empty (auto-named), the list output shows `From -> ` with nothing after the arrow.
**Expected:** Should display `From -> (auto)` or similar placeholder.

---

## Performance Considerations

**Observations:**

- Git operations can be slow for large repositories
- No caching mechanism for repeated clones during browsing
- Each remote browse operation does a fresh clone

**Suggestions:**

- Implement local cache for repository metadata
- Reuse cloned repos during same session
- Offer "Quick Add" mode that skips browsing for known paths

---

## Security Considerations

**Good Practices:**

- License verification step helps with compliance
- Locked commits ensure reproducibility
- Git operations are sandboxed to temp directories

**Suggestions:**

- Add checksum verification for critical files
- Warn when tracking dependencies from non-official sources
- Implement vendor signature verification (optional)

---

## Conclusion

Overall, git-vendor demonstrates a solid foundation with thoughtful UX decisions. The interactive wizard approach using charmbracelet/huh is appropriate for the use case. The main areas for improvement are:

1. **Error handling and recovery guidance**
2. **Progress feedback for long operations**
3. **Clarity in list output and auto-naming behavior**

The tool is functional and ready for use, but would benefit from enhanced user feedback, better error messages, and more comprehensive documentation.

**Rating: 7.5/10**

**Would recommend:** Yes, especially for projects that need lightweight dependency vendoring without the overhead of git submodules.
