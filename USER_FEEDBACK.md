# User Feedback Report - git-vendor

**Test Date:** December 25, 2025
**Version Tested:** v5.0
**Test Methodology:** Multiple user personas testing various workflows via interactive sessions

---

## Implementation Status

**Last Updated:** December 25, 2025 (after Phase 2 implementation)

### ✅ Completed (Quick Wins)
1. ✅ Fixed "no such file or directory" error → Shows "Run 'git-vendor init' first"
2. ✅ Added proper pluralization (1 vendor vs. 2 vendors)
3. ✅ Show unknown command error before help text
4. ✅ Make remove default to "No" instead of "Yes"
5. ✅ Improved git error messages with context

**Commit:** `99b1dfd` - "fix: polish UX with better error messages and proper pluralization (Quick Wins)"

### ✅ Completed (Phase 1: Polish & Messaging)
1. ✅ Add tool description to help text
2. ✅ Show "Next steps" after add command
3. ✅ Add helper text for browse vs manual
4. ✅ Improve "No lockfile found" messaging
5. ✅ Add "(remote) → (local)" to path notation
6. ✅ Expand validation success message

**Commit:** `3d9345f` - "feat: Phase 1 - polish messaging and UX clarity improvements"

### ✅ Completed (Phase 2: Enhanced Output)
1. ✅ Show sync summary with file details - track files copied in sync operations
2. ✅ Show file details in sync output - display "(X files)" after each path mapping

**Output Examples:**
- Per-vendor: `✓ claude-code-tools @ main (synced 1 path: 32 files)`
- Summary: `Summary: Synced 32 files across all vendors`

**Files Modified:**
- `internal/core/filesystem.go` - Added CopyStats type, updated interface
- `internal/core/file_copy_service.go` - Aggregate stats through mappings
- `internal/core/sync_service.go` - Display stats and summary
- `internal/core/license_service.go` - Handle new return value
- `internal/core/update_service.go` - Handle new return value
- `internal/core/vendor_syncer.go` - Update delegate signature
- All test files - Updated mock expectations

**Commit:** (pending)

### 🔄 Remaining Work

**Phase 3: New Features**
- [ ] Add status/check command
- [ ] Add `--yes`, `--quiet`, `--json` flags
- [ ] Add progress indication during clone
- [ ] Show diff/summary before update

**Long Term:**
- [ ] Interactive path preview in wizard
- [ ] Shell completions
- [ ] Better conflict resolution workflow
- [ ] Performance optimization for large repos

---

## 🎯 Implementation Plan

### Phase 1: Polish & Messaging ✅ COMPLETED
*Low-hanging fruit that significantly improves UX*

**Complexity:** LOW | **Impact:** HIGH | **Time:** 1-2 hours

**Status:** ✅ All 6 tasks completed (Commit: `3d9345f`)

### Phase 2: Enhanced Output ✅ COMPLETED
*Better feedback about what happened*

**Complexity:** MEDIUM | **Impact:** MEDIUM-HIGH | **Time:** 2-3 hours

**Status:** ✅ All 2 tasks completed (Commit: pending)

**Implementation Details:**
- Added `CopyStats` type to track file counts and bytes
- Updated `FileSystem` interface to return statistics
- Propagated stats through `FileCopyService` → `SyncService`
- Display format: `✓ vendor @ ref (synced X paths: Y files)`
- Summary: `Summary: Synced Y files across all vendors`

### Phase 3: New Features ⭐ NEXT
*Significant new functionality*

**Complexity:** HIGH | **Impact:** HIGH | **Time:** 6-8 hours

**Priority Order:**
- **3a.** Non-interactive flags (`--yes`, `--quiet`, `--json`)
- **3b.** Status command (check sync state)
- **3c.** Progress during clone (UX improvement)
- **3d.** Update confirmation with diff preview

**Recommended Approach:** Implement Phase 1 first for immediate UX gains, then reassess priorities based on user needs (DevOps automation vs. general UX).

---

## Executive Summary

git-vendor is a solid tool with intuitive core functionality. The interactive wizards are generally well-designed, but several rough edges in error handling, messaging clarity, and edge cases could confuse users. This feedback focuses on making the tool feel polished and professional.

**Overall Rating:** 7.5/10
**Primary Strengths:** Clear command structure, helpful examples, interactive wizards
**Primary Weaknesses:** Cryptic error messages, inconsistent init requirements, unclear progress states

---

## Feedback by User Persona

### Persona 1: "Sarah" - First-Time User (Never used git-vendor before)

**Scenario:** Trying to understand what git-vendor does and how to get started.

#### Positive Feedback ✅

1. **Excellent help text** - Running `git-vendor` with no arguments shows comprehensive help
   - Clear command listing with descriptions
   - Good examples section showing common use cases
   - Navigation tips are helpful

2. **Intuitive command names** - Commands like `init`, `add`, `sync`, `list` are self-explanatory
   - Follows familiar patterns from other tools (npm, git, etc.)

3. **Version display** - Shows "git-vendor v5.0" at top of help
   - Helps with troubleshooting and bug reports

#### Issues & Confusion ❌

1. **CRITICAL: Confusing error when vendor directory doesn't exist**
   ```
   $ git-vendor sync
   No lockfile found. Running update...
   ✖ Sync Failed
   failed to write vendor.lock: open vendor/vendor.lock: no such file or directory
   ```

   **Problem:** The error message talks about file system details ("open vendor/vendor.lock: no such file or directory") instead of telling the user what they need to do.

   **Expected:** Something like:
   ```
   ✖ Not Initialized
   The vendor directory doesn't exist. Run 'git-vendor init' first.
   ```

2. **Inconsistent behavior across commands without init**
   - `list` says "No vendors configured" (seems fine, doesn't error)
   - `sync` fails with cryptic file system error (confusing)
   - `add` starts the wizard successfully (works, but then what?)
   - `validate` says "no vendors configured" but calls it a "Validation Failed"

   **Suggestion:** All commands should either:
   - Detect missing vendor directory and show a friendly "Run 'git-vendor init' first" message, OR
   - Auto-initialize if it makes sense for that command

3. **Help flag inconsistency**
   - `git-vendor --help` shows the same output as `git-vendor` with no args
   - Is `-h` supported? Not documented
   - What about `git-vendor help`?

   **Suggestion:** Document all supported help variations

4. **No clear "what is this tool?" introduction**
   - Help shows commands but doesn't explain what git-vendor does
   - New users might not understand the vendoring concept

   **Suggestion:** Add a brief 1-2 line description at the top:
   ```
   git-vendor v5.0
   Vendor specific files/directories from Git repositories with lock file support

   Commands:
   ...
   ```

5. **Invalid command handling is silent**
   - `git-vendor invalid-command` just shows the help menu
   - No error message saying the command is invalid

   **Suggestion:** Add a line like:
   ```
   ✖ Unknown command: 'invalid-command'

   [then show help]
   ```

---

### Persona 2: "Marcus" - Developer Adding First Dependency

**Scenario:** Has initialized vendor directory, wants to add a dependency.

#### Positive Feedback ✅

1. **Smart URL parsing**
   - Recognizes GitHub URLs
   - Auto-extracts repo name for vendor name suggestion
   - Saves typing

2. **Good validation feedback**
   - URL validation shows clear error: "URL must start with http://, https://, or git@"
   - Inline validation is responsive

3. **Wizard flow is logical**
   - URL → Name → Ref → Paths makes sense
   - Each step is clear about what it's asking for

4. **Contextual prompts**
   - "Paste a full repo URL or a specific file link" gives helpful context
   - Placeholder text shows format expectations

#### Issues & Confusion ❌

1. **CRITICAL: Cryptic error when browsing remote files**
   ```
   ✖ Error
   git ls-tree failed: exit status 128
   ```

   **Problem:** "exit status 128" means nothing to users. What went wrong? Network? Auth? Invalid ref?

   **Expected:** More context like:
   ```
   ✖ Failed to browse remote repository
   Could not access 'main' branch. Check that the branch exists and is accessible.

   Git error: exit status 128
   ```

2. **Unclear what happens after adding vendor**
   - After saving, does it automatically sync files?
   - Do I need to run `git-vendor sync` next?
   - The workflow isn't explained

   **Suggestion:** After saving, show:
   ```
   ✔ Added lipgloss

   Next steps:
     git-vendor sync      # Download files to locked version
     git-vendor update    # Update to latest commits
   ```

3. **"Browse Remote Files" vs "Enter Manually" choice**
   - Not clear what "Enter Manually" means
   - Does it mean type a path? Type multiple paths?
   - What format?

   **Suggestion:** Add helper text:
   ```
   ┃ Remote Path
   ┃ > Browse Remote Files      (interactively select files/dirs)
   ┃   Enter Manually            (type path like: src/components)
   ```

4. **Progress indicators during clone**
   - When selecting "Browse Remote Files", there's a long pause
   - No indication that it's cloning the repo
   - User might think it's frozen

   **Suggestion:** Show progress:
   ```
   ⠿ Cloning repository...
   ```

5. **Edit screen shows "(no paths, not synced)"**
   - While accurate, "(not synced)" is confusing before any paths are added
   - Implies something is wrong

   **Suggestion:** Just show "(no paths)" until paths are added

6. **No way to test/preview path selection**
   - After adding paths, no way to see what files will be synced without committing
   - Would be nice to have a preview or validation step

   **Suggestion:** Add a "Preview" option in the edit menu that shows what would be synced

---

### Persona 3: "Priya" - Daily User Managing Multiple Vendors

**Scenario:** Regular user who syncs dependencies frequently.

#### Positive Feedback ✅

1. **Clean list output**
   ```
   📦 example-lib
      https://github.com/golang/example
      License: BSD-3-Clause
      └─ @ master
         └─ hello → lib/example/hello
   ```
   - Tree structure is very clear
   - Emoji makes it scannable
   - Shows all relevant info at a glance

2. **Sync progress indicators**
   ```
   ⠿ example-lib (cloning repository...)
     ✓ example-lib @ master (synced 1 path(s))
   ✔ Synced.
   ```
   - Good use of spinners and checkmarks
   - Clear what's happening at each stage

3. **Dry-run functionality**
   - `--dry-run` is super helpful for checking before syncing
   - Message is clear: "This is a dry-run. No files were modified."

4. **Force flag works as expected**
   - Re-downloads even when already synced
   - Useful for fixing corrupted files

5. **Specific vendor sync**
   - `git-vendor sync my-vendor` works great
   - Saves time when you only changed one dependency

#### Issues & Confusion ❌

1. **Unclear sync vs update distinction**
   - Both commands seem to download files
   - Documentation says "sync" uses locked versions and "update" fetches latest
   - But the difference isn't obvious from the command names
   - New users will be confused about which to use

   **Suggestion:**
   - Rename `sync` to `install` (matches npm/yarn convention), OR
   - Add aliases so both `sync` and `install` work, OR
   - Make the help text more explicit:
     ```
     sync      Download dependencies (uses locked versions from vendor.lock)
     update    Fetch latest commits and update vendor.lock
     ```

2. **Update command is destructive but has no confirmation**
   - `git-vendor update` rewrites vendor.lock with latest commits
   - No warning that this will change locked versions
   - No diff shown of what changed

   **Suggestion:** Show a summary before updating:
   ```
   Updates available:
     • example-lib: abc123 → def456 (2 commits ahead)
     • other-lib: no changes

   Update lockfile? (y/n)
   ```

3. **Validation command doesn't say what it validated**
   ```
   ✔ Validation passed. No issues found.
   ```
   - What did it check? Paths? Conflicts? Config syntax?
   - Users don't know what "passed" means

   **Suggestion:** Be more specific:
   ```
   ✔ Validation passed
   • Config syntax: OK
   • Path conflicts: None
   • All vendors: OK (2 vendors)
   ```

4. **Remove command confirmation is backwards**
   - Default selection is "Yes" but most deletion commands default to "No"
   - Easy to accidentally delete if user just hits Enter

   **Suggestion:** Make "No" the default (or at least make it more obvious which is selected)

5. **No indication of what files were synced**
   - `✓ example-lib @ master (synced 1 path(s))` is good
   - But which files specifically? Where did they go?
   - Hard to verify without checking the file system

   **Suggestion:** Add verbose output or a summary:
   ```
   ✓ example-lib @ master (synced 1 path(s))
     → lib/example/hello/ (5 files)
   ```

6. **Verbose mode output not tested**
   - `--verbose` flag exists but unclear what it shows
   - Does it show git commands? File operations? Both?

   **Suggestion:** Test and document what verbose mode reveals

---

### Persona 4: "James" - DevOps Engineer Automating Builds

**Scenario:** Integrating git-vendor into CI/CD pipelines.

#### Positive Feedback ✅

1. **Clear exit codes** (assumed based on error messages)
   - Error messages show "✖" for failures
   - Success messages show "✔"
   - Likely has proper exit codes for scripting

2. **Flags are well-named**
   - `--dry-run`, `--force`, `--verbose` are standard
   - Easy to remember and script

3. **Lockfile concept is solid**
   - Deterministic builds with vendor.lock
   - Same as package-lock.json, Cargo.lock, etc.

#### Issues & Concerns ❌

1. **No --yes or --assume-yes flag for non-interactive mode**
   - Remove command prompts for confirmation
   - Can't script deletion without manual intervention

   **Suggestion:** Add `-y` or `--yes` flag to skip prompts

2. **No --quiet or --silent flag**
   - Can't suppress output for clean CI logs
   - Every sync shows progress spinners

   **Suggestion:** Add `-q` or `--quiet` flag for minimal output

3. **No JSON output mode**
   - List command outputs pretty text
   - Hard to parse in scripts

   **Suggestion:** Add `--json` flag for structured output:
   ```bash
   git-vendor list --json
   {
     "vendors": [
       {
         "name": "example-lib",
         "url": "https://github.com/golang/example",
         "license": "BSD-3-Clause",
         "specs": [...]
       }
     ]
   }
   ```

4. **Unclear if commands are idempotent**
   - Does running `sync` twice do extra work?
   - Does `init` fail if vendor directory exists?

   **Suggestion:** Document idempotency guarantees

5. **No way to check if sync is needed**
   - Would be nice to have `git-vendor status` or `git-vendor check`
   - Shows if local files are out of sync with lockfile

   **Suggestion:** Add status command:
   ```bash
   git-vendor status
   ✔ All vendors synced

   # or

   ⚠ Vendors need syncing
   • example-lib: local files missing
   • other-lib: lockfile updated but not synced

   Run 'git-vendor sync' to fix.
   ```

---

## UI/UX Polish Issues

### Visual Consistency

1. **Inconsistent error message formats**
   - Some errors: `✖ Sync Failed\nfailed to write vendor.lock: ...`
   - Some errors: `✖ Usage\ngit vendor remove <name>`
   - Some errors: `✖ Error\ngit ls-tree failed: ...`

   **Suggestion:** Standardize to:
   ```
   ✖ [Category]: [Brief message]
   [Detailed explanation]
   [Suggested action]
   ```

2. **Mixed use of symbols**
   - ✖ ✔ ⠿ 📦 ← + are all used
   - Generally good, but no legend explaining what they mean
   - What does ⠿ mean? (appears during cloning)

   **Suggestion:** Consider adding a brief legend in help or use more common symbols

3. **Box drawing characters in TUI**
   - `╭──────────────────────────╮` looks good in modern terminals
   - Might break in older terminals or Windows cmd.exe

   **Suggestion:** Test on Windows, or provide a `--no-unicode` flag

### Messaging Clarity

1. **"No lockfile found. Running update..."**
   - This happens during `sync` when there's no lockfile
   - "Running update" is passive voice and unclear
   - User doesn't know if update succeeded or what happened

   **Suggestion:**
   ```
   No lockfile found. Generating lockfile...
   ⠿ Fetching latest commits...
   ✔ Lockfile created

   Now syncing files...
   ```

2. **"Syncing 1 vendor(s)..."**
   - "1 vendor(s)" is grammatically awkward
   - Should be "1 vendor" or "2 vendors"

   **Suggestion:** Use proper pluralization

3. **Path notation: "hello → lib/example/hello"**
   - Arrow direction suggests "from → to"
   - Is "hello" the source and "lib/example/hello" the destination?
   - Actually unclear which direction the arrow flows

   **Suggestion:** Make it more explicit:
   ```
   From: hello (remote)
   To: lib/example/hello (local)
   ```
   Or:
   ```
   hello (remote) → lib/example/hello (local)
   ```

### Progress & Feedback

1. **Long operations have no progress indication**
   - `update` command can take a while
   - No indication of what's happening
   - Just sits with a spinner

   **Suggestion:** Show more granular progress:
   ```
   ⠿ example-lib
     ⠿ Cloning repository...
     ⠿ Fetching ref 'main'...
     ⠿ Copying files...
     ✓ Synced 1 path(s)
   ```

2. **No summary after update**
   - `✔ Updated all vendors.` is terse
   - Doesn't say what changed

   **Suggestion:** Add a summary:
   ```
   ✔ Updated all vendors

   Changes:
     • example-lib: abc123 → def456 (main)
     • other-lib: no changes

   Lockfile updated.
   ```

3. **Validation only shows failures**
   - `validate` is silent about what passed
   - Only shows what failed

   **Suggestion:** Show what was checked even on success

---

## Feature Requests (from user testing)

### High Priority

1. **Status/Check command**
   - See if local files match lockfile
   - See if lockfile is outdated
   - Essential for CI/CD workflows

2. **Better error context**
   - Git errors should include the actual git error message
   - Network errors should suggest checking connectivity
   - Permission errors should suggest checking auth

3. **Diff support for updates**
   - Show what will change before updating
   - Show commit messages between old and new versions
   - Helps with reviewing dependency updates

4. **Interactive edit improvements**
   - Preview what files will be synced before saving
   - Show file sizes for large directories
   - Warn if path will sync many files

5. **Non-interactive mode**
   - `--yes` flag for confirmations
   - `--quiet` flag for minimal output
   - `--json` flag for structured output

### Medium Priority

6. **Batch operations**
   - `git-vendor update <vendor-name>` to update just one vendor
   - `git-vendor sync --only=vendor1,vendor2` to sync subset

7. **Configuration validation**
   - Check URLs are accessible before saving
   - Validate ref exists in remote
   - Prevent duplicate vendor names

8. **Lockfile diff command**
   - `git-vendor diff` to compare current vs. potential updates
   - Show what `update` would change without changing it

9. **Better remove workflow**
   - Option to also delete synced files
   - Currently only removes config entry
   - User might expect files to be deleted too

10. **Path conflict detection improvements**
    - Currently shows conflicts but workflow to fix isn't clear
    - Suggest which vendor to edit or paths to change

### Low Priority

11. **Shell completions**
    - Bash/Zsh completions for commands
    - Autocomplete vendor names for `sync`, `remove`, etc.

12. **Config file comments**
    - Preserve comments in vendor.yml
    - Currently YAML library might strip them

13. **Template/preset support**
    - Common vendor configurations (e.g., "vendor Go standard library")
    - Quick setup for popular dependencies

14. **Colored output control**
    - `--no-color` flag for CI environments
    - Respect `NO_COLOR` environment variable

15. **License compliance checking**
    - Report all licenses in use
    - Flag incompatible license combinations
    - Export SBOM (Software Bill of Materials)

---

## Testing Gaps Found

During testing, these scenarios were NOT covered and should be tested:

1. ✗ What happens with very large repositories (100MB+)?
2. ✗ How does it handle repositories with submodules?
3. ✗ What if the lockfile is manually edited and becomes invalid?
4. ✗ What if vendor.yml has syntax errors?
5. ✗ What if two vendors try to write to the same destination?
6. ✗ What happens if `git` is not installed?
7. ✗ What if there's no internet connection?
8. ✗ How does it handle private repositories requiring authentication?
9. ✗ What if a file path has special characters or spaces?
10. ✗ What if the destination path doesn't exist vs. destination file exists?
11. ✗ What if vendor.yml is committed to git and conflicts during merge?
12. ✗ Performance with many vendors (10+, 50+, 100+)?

---

## Accessibility & Internationalization

1. **Color-only feedback**
   - Errors/success indicated by ✖/✔ symbols (good)
   - But also relies on red/green colors
   - Color-blind users might struggle

   **Suggestion:** Symbols are good, ensure they're always present

2. **Screen reader compatibility**
   - TUI elements might not work with screen readers
   - Consider alternate modes for accessibility

3. **No internationalization**
   - All messages are English-only
   - Not necessarily a problem for a dev tool
   - But consider if target audience is global

---

## Documentation Gaps

Based on user testing, these should be documented better:

1. **Workflow examples**
   - Step-by-step guide for common tasks
   - "How do I vendor a single file from a repo?"
   - "How do I update one dependency?"
   - "How do I roll back to a previous version?"

2. **Concept explanations**
   - What is vendoring and why use it?
   - How does the lockfile work?
   - When to use sync vs. update?

3. **Troubleshooting guide**
   - Common error messages and solutions
   - What to do when clone fails
   - How to fix path conflicts

4. **Configuration file format**
   - Manual editing reference for vendor.yml
   - What fields are required/optional?
   - Can I edit it by hand or must use CLI?

5. **Integration examples**
   - Using in Makefile
   - Using in Docker builds
   - Using in GitHub Actions

---

## Overall Recommendations

### Quick Wins (Easy to fix, high impact)

1. ✅ Fix "no such file or directory" error → "Run 'git-vendor init' first"
2. ✅ Add proper pluralization (1 vendor vs. 2 vendors)
3. ✅ Show unknown command error before help text
4. ✅ Make remove default to "No" instead of "Yes"
5. ✅ Improve git error messages with context

### Medium Effort (Moderate work, good impact)

6. ✅ Add status/check command
7. ✅ Add `--yes`, `--quiet`, `--json` flags
8. ✅ Show sync summary (which files, where)
9. ✅ Add progress indication during clone
10. ✅ Show diff/summary before update

### Long Term (Bigger features)

11. ✅ Interactive path preview in wizard
12. ✅ Shell completions
13. ✅ Better conflict resolution workflow
14. ✅ Performance optimization for large repos

---

## Final Thoughts

git-vendor is a **well-designed tool** with a solid foundation. The core functionality works well, and the interactive wizards are intuitive once you understand the flow. However, **error messages and edge cases** need polish to make it feel professional and user-friendly.

The biggest pain points are:
1. Cryptic error messages (especially git errors and missing init)
2. Unclear sync vs. update distinction
3. Lack of feedback about what happened (summaries, previews)
4. Missing automation-friendly features (--yes, --quiet, --json)

With these improvements, git-vendor could easily go from "works well" to "delightful to use."

---

## Appendix: Test Session Transcript Summary

**Commands tested:**
- ✅ `git-vendor` (no args)
- ✅ `git-vendor --help`
- ✅ `git-vendor invalid-command`
- ✅ `git-vendor init`
- ✅ `git-vendor list` (empty and with vendors)
- ✅ `git-vendor validate` (empty and valid)
- ✅ `git-vendor sync` (various flags)
- ✅ `git-vendor sync --dry-run`
- ✅ `git-vendor sync --force`
- ✅ `git-vendor sync <vendor-name>`
- ✅ `git-vendor update`
- ✅ `git-vendor remove <name>`
- ✅ `git-vendor add` (partial wizard flow)

**Edge cases tested:**
- ✅ Running commands before init
- ✅ Invalid URL in add wizard
- ✅ Remove command confirmation
- ✅ Browse remote files (error case)
- ✅ Empty vendor list
- ✅ Syncing after deleting files

**Not tested (but should be):**
- Private repositories
- SSH URLs
- Large repositories
- Many vendors
- Concurrent operations
- Windows compatibility
- Network failures
- Invalid YAML in config files

