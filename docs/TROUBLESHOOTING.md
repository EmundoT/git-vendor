# Troubleshooting Guide

Common issues and solutions for git-vendor.

## Table of Contents

- [Sync Errors](#sync-errors)
- [License Issues](#license-issues)
- [Configuration Problems](#configuration-problems)
- [Validation Errors](#validation-errors)
- [Performance Issues](#performance-issues)
- [Git Issues](#git-issues)
- [TUI/Display Issues](#tuidisplay-issues)
- [General Debugging](#general-debugging)

## Sync Errors

### Error: "locked commit no longer exists in the repository"

**Symptoms:**

```text
✖ Sync Failed
locked commit abc123d no longer exists in the repository.

This usually happens when the remote repository has been force-pushed or the commit was deleted.
Run 'git-vendor update' to fetch the latest commit and update the lockfile, then try syncing again
```

**Cause:**
The commit hash stored in `vendor.lock` no longer exists in the remote repository. This typically happens when:

- The remote repository was force-pushed
- The branch was rebased
- The commit was deleted

**Solution:**

```bash
# Update to the latest commit
git-vendor update

# Then try syncing again
git-vendor sync
```

**Prevention:**

- Track stable branches or tags instead of development branches
- Use version tags (v1.0.0) instead of branch names
- Communicate with upstream maintainers about force-push policies

---

### Error: "path 'xyz' not found"

**Symptoms:**

```text
✖ Sync Failed
path 'src/utils' not found
```

**Cause:**
The path specified in your vendor configuration doesn't exist at the locked commit.

**Solution:**

1. **Check if the path exists in the repository:**

   ```bash
   # Visit the repository on GitHub and verify the path
   ```

2. **Update your configuration:**

   ```bash
   git-vendor edit
   # Update the path mapping to the correct path
   ```

3. **Update to latest commit if path was added recently:**

   ```bash
   git-vendor update
   git-vendor sync
   ```

---

### Error: "No lockfile found. Running update..."

**Symptoms:**
Sync command automatically runs update first.

**Cause:**
The `vendor.lock` file is missing or empty.

**Solution:**
This is expected behavior. The tool automatically creates the lock file:

```bash
git-vendor sync  # Will run update first
```

To create lock file without syncing:

```bash
git-vendor update
```

---

### Error: "checkout ref 'xyz' failed"

**Symptoms:**

```text
✖ Sync Failed
checkout ref 'main' failed: error: pathspec 'main' did not match any file(s)
```

**Cause:**

- The specified ref (branch/tag) doesn't exist in the repository
- The ref name was misspelled
- The repository changed its default branch name

**Note:** This error typically occurs during `sync` operations when the lockfile references a ref that no longer exists. During `update` operations, the tool has fallback logic that automatically fetches all refs if the specific ref fetch fails, so invalid refs may not immediately fail during update.

**Solution:**

1. **Verify the ref exists:**
   Visit the repository on GitHub and check available branches/tags

2. **Edit the configuration:**

   ```bash
   git-vendor edit
   # Update the ref to the correct branch/tag name
   ```

3. **Common issue - main vs master:**

   ```bash
   git-vendor edit
   # Change "master" to "main" or vice versa
   ```

4. **If error occurs during sync:**

   ```bash
   # Update to regenerate lockfile with valid refs
   git-vendor update
   git-vendor sync
   ```

---

## License Issues

### Warning: "Accept [LICENSE_NAME] License?"

**Symptoms:**
When completing the `add` command (after selecting "💾 Save & Exit"), you're prompted to accept a license.

**Cause:**
The repository's license is not in the pre-approved list:

- MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC, Unlicense, CC0-1.0

**Solution:**

1. **Review the license:**

   - License file is shown in `vendor/licenses/<vendor-name>.txt`
   - Or visit the repository to read the full license

2. **Accept or reject:**

   - Accept: You take responsibility for compliance
   - Reject: Vendor operation is canceled

3. **Automatic approval for common licenses:**
   If the license should be auto-approved, this is a tool limitation. You can still accept it manually.

---

### Error: "License detection failed"

**Symptoms:**
License shows as "UNKNOWN" or "NONE".

**Cause:**

- Repository doesn't have a license file
- License file is not in the root directory
- GitHub API is unavailable

**Solution:**

1. **Check repository for license:**
   Visit the repository and look for LICENSE or COPYING files

2. **Proceed with caution:**

   - No license = all rights reserved by default
   - Contact repository owner about licensing
   - Consider using an alternative dependency

3. **Override if you know the license:**
   The tool will still let you proceed after warning.

---

## Configuration Problems

### Error: "vendor 'xyz' not found"

**Symptoms:**

```text
✖ Error
vendor 'my-vendor' not found
```

**Cause:**
The vendor name doesn't exist in your configuration.

**Solution:**

1. **List configured vendors:**

   ```bash
   git-vendor list
   ```

2. **Check spelling:**
   Vendor names are case-sensitive

3. **Add the vendor if missing:**

   ```bash
   git-vendor add
   ```

---

### Issue: Empty mapping destination shows as "(auto)"

**Symptoms:**
When running `git-vendor list`, you see:

```text
• src/utils -> (auto)
```

**Cause:**
This is expected behavior when you leave the destination path empty.

**Explanation:**

- "(auto)" means the tool will use automatic naming
- The actual destination will be the base name of the source path
- Example: `src/utils` → `utils` (in current directory)

**To change:**

```bash
git-vendor edit
# Edit the path mapping and specify an explicit destination
```

---

### Issue: Configuration changes not taking effect

**Symptoms:**
You edited `vendor.yml` manually but changes aren't reflected.

**Solution:**

1. **Regenerate lock file:**

   ```bash
   git-vendor update
   ```

2. **Sync to apply changes:**

   ```bash
   git-vendor sync
   ```

**Note:** Always run `update` after manually editing `vendor.yml`.

---

## Validation Errors

### Error: "no vendors configured"

**Symptoms:**

```text
✖ Validation Failed
no vendors configured
```

**Cause:**
The `vendor.yml` file exists but has no vendors defined (empty vendor list).

**Solution:**

This is expected if you just initialized the vendor directory. Add your first vendor:

```bash
git-vendor add
```

**Note:** Validation requires at least one vendor to pass. An empty configuration is technically valid YAML but not useful for the tool.

---

### Error: "duplicate vendor name"

**Symptoms:**

```text
✖ Validation Failed
duplicate vendor name: my-vendor
```

**Cause:**
Two vendors in `vendor.yml` have the same name.

**Solution:**

1. **Edit configuration:**

   ```bash
   git-vendor edit
   ```

2. **Or manually edit `vendor.yml`:**
   - Find the duplicate vendor names
   - Rename one to be unique
   - Run `git-vendor update` to regenerate lockfile

---

### Warning: "Path Conflicts Detected"

**Symptoms:**

```text
! Path Conflicts Detected
Found 2 conflict(s)

⚠ Conflict: lib/utils
  • vendor-a: src/utils → lib/utils
  • vendor-b: pkg/utils → lib/utils
```

**Cause:**
Multiple vendors are configured to copy files to the same destination path.

**Solution:**

1. **Review the conflicts:**

   ```bash
   git-vendor validate  # Shows full conflict details
   ```

2. **Resolve by editing path mappings:**

   ```bash
   git-vendor edit
   # Change one of the conflicting destinations
   ```

3. **Example resolution:**

   ```yaml
   vendors:
     - name: vendor-a
       specs:
         - ref: main
           mapping:
             - from: src/utils
               to: lib/vendor-a-utils # Changed to avoid conflict
     - name: vendor-b
       specs:
         - ref: main
           mapping:
             - from: pkg/utils
               to: lib/vendor-b-utils # Changed to avoid conflict
   ```

**Note:**

- Path conflicts are warnings, not errors. Sync will proceed, but the last vendor synced will overwrite the previous one.
- On Windows, paths in conflict warnings may use backslashes (`lib\utils`) instead of forward slashes. This is normal OS behavior.

---

### Error: "vendor xyz has no specs configured"

**Symptoms:**

```text
✖ Validation Failed
vendor my-vendor has no specs configured
```

**Cause:**
The vendor entry exists but has no `specs` (branches/tags to track).

**Solution:**

Edit the vendor and add at least one spec (branch/tag):

```bash
git-vendor edit
# Select the vendor
# Add a new branch
```

---

### Error: "vendor xyz has no URL"

**Symptoms:**

```text
✖ Validation Failed
vendor my-vendor has no URL
```

**Cause:**
The vendor entry exists but has an empty URL field (usually from manual editing).

**Solution:**

Edit the vendor and add a valid repository URL:

```bash
git-vendor edit
# Select the vendor
# Enter a valid GitHub repository URL
```

---

### Error: "vendor xyz has a spec with no ref"

**Symptoms:**

```text
✖ Validation Failed
vendor my-vendor has a spec with no ref
```

**Cause:**
A branch/tag specification exists but has an empty ref field.

**Solution:**

Edit the vendor configuration and ensure all specs have a ref:

```bash
git-vendor edit
# Select the vendor
# Select the spec with no ref
# Enter a valid branch/tag name
```

---

### Error: "vendor xyz @ ref has no path mappings"

**Symptoms:**

```text
✖ Validation Failed
vendor my-vendor @ main has no path mappings
```

**Cause:**
The vendor spec exists but has no path mappings configured.

**Solution:**

Add at least one path mapping:

```bash
git-vendor edit
# Select the vendor
# Select the branch/tag
# Add a path mapping
```

---

### Error: "vendor xyz @ ref has a mapping with empty 'from' path"

**Symptoms:**

```text
✖ Validation Failed
vendor my-vendor @ main has a mapping with empty 'from' path
```

**Cause:**
A path mapping exists but the source path (from) is empty.

**Solution:**

Edit the vendor and fix the empty mapping:

```bash
git-vendor edit
# Select the vendor
# Select the branch
# Delete the invalid mapping and add a correct one
```

Or manually edit `vendor.yml`:

```yaml
vendors:
  - name: my-vendor
    specs:
      - ref: main
        mapping:
          - from: src/utils # Must not be empty
            to: lib/utils
```

---

## Performance Issues

### Issue: Sync takes a very long time

**Symptoms:**
The `sync` command appears to hang or takes minutes to complete.

**Causes:**

- Large repository being cloned
- Slow network connection
- Locked commit is not in recent history

**Solutions:**

1. **Check progress:**
   Look for "• Processing vendor-name..." message

2. **For large repositories:**

   - Vendor only specific subdirectories, not root
   - Use tags instead of branch tips (faster shallow clone)

3. **Force a fresh sync:**

   ```bash
   git-vendor sync --force
   ```

4. **Update to fetch latest:**

   ```bash
   git-vendor update  # Updates lock to latest commits
   git-vendor sync    # Should be faster now
   ```

---

### Issue: "fatal: fetch-pack: invalid index-pack output" or timeout errors

**Symptoms:**
Git operations fail with network or timeout errors.

**Cause:**

- Network issues
- Repository is very large
- Context timeout (30 seconds for directory listing)

**Solutions:**

1. **Check network connection:**

   ```bash
   git clone https://github.com/owner/repo  # Test manually
   ```

2. **Try again:**
   Network issues are often transient

3. **For persistent issues:**
   - Check if repository is publicly accessible
   - Verify URL is correct
   - Contact repository owner about access

---

## Cache Issues

### Issue: Sync still downloading files after running recently

**Symptoms:**

```text
• Syncing vendor-name...
  Cloning repository...
  (Expected to see "Cache hit, skipping clone")
```

**Cause:**

- Incremental sync cache disabled or cleared
- Commit hash changed (lockfile updated)
- File contents changed at same commit
- Cache corruption

**Solutions:**

1. **Check if cache exists:**

   ```bash
   ls -la vendor/.cache/
   # Should show <vendor-name>.json files
   ```

2. **Verify commit hasn't changed:**

   ```bash
   cat vendor/vendor.lock
   # Check commit_hash for your vendor
   ```

3. **Clear and rebuild cache:**

   ```bash
   rm -rf vendor/.cache/
   git-vendor sync
   # First run will be slow, subsequent runs fast
   ```

4. **Check if --no-cache or --force was used:**
   - `--no-cache` disables incremental cache
   - `--force` re-downloads all files
   - Remove these flags for normal caching

---

### Issue: Cache checksum mismatch

**Symptoms:**

```text
Warning: Cache checksum mismatch for file.go
Re-downloading...
```

**Cause:**

- File was manually edited in vendored location
- Cache corruption
- Filesystem race condition

**Solutions:**

1. **Don't edit vendored files:**
   Vendored files should be read-only (git-vendor will overwrite changes)

2. **Clear cache for vendor:**

   ```bash
   rm vendor/.cache/<vendor-name>.json
   git-vendor sync <vendor-name>
   ```

3. **If persistent, report as bug:**
   Cache corruption should not happen - please file an issue with:
   - Cache file content (vendor/.cache/<vendor-name>.json)
   - Operating system
   - File system type

---

### Issue: How to clear cache completely

**Purpose:**
Force complete re-download of all vendors (useful for debugging or disk space issues)

**Solution:**

```bash
# Clear all cache
rm -rf vendor/.cache/

# Or clear specific vendor
rm vendor/.cache/<vendor-name>.json

# Re-sync (will be slower first time)
git-vendor sync
```

**Note:** Cache auto-invalidates when commit hashes change, so manual clearing is rarely needed.

---

### Issue: When is cache bypassed?

**Cache is disabled for:**

1. **--force flag:**

   ```bash
   git-vendor sync --force  # Always re-downloads
   ```

2. **--no-cache flag:**

   ```bash
   git-vendor sync --no-cache  # Disables cache
   ```

3. **First sync (no cache file exists)**

4. **Commit hash changed in lockfile**

5. **File limit exceeded (>1000 files per vendor)**

**Cache is enabled for:**

- Normal sync operations
- Re-syncing unchanged vendors
- Dry-run mode (validates cache, doesn't write)

---

## Hook Debugging

### Issue: pre_sync hook failed

**Symptoms:**

```text
✖ Error syncing vendor-name
  pre_sync hook failed: exit status 1
  Output: <hook error message>
```

**Cause:**

- Hook command not found
- Hook script has bugs
- Missing dependencies
- Wrong working directory

**Solutions:**

1. **Test hook manually:**

   ```bash
   # Hooks run from project root
   cd /path/to/project

   # Set environment variables
   export GIT_VENDOR_NAME="vendor-name"
   export GIT_VENDOR_URL="https://github.com/owner/repo"
   export GIT_VENDOR_REF="main"
   export GIT_VENDOR_COMMIT="abc123..."
   export GIT_VENDOR_ROOT="$(pwd)"

   # Run hook command
   sh -c "echo 'Testing pre_sync hook...'"
   ```

2. **Check if command exists:**

   ```bash
   which <command>
   # Example: which npm, which make
   ```

3. **Fix vendor.yml syntax:**

   ```yaml
   # ✅ Correct
   hooks:
     pre_sync: echo "Starting sync"

   # ✅ Multi-line correct
   hooks:
     pre_sync: |
       npm install
       npm run build

   # ❌ Wrong (missing pipe for multiline)
   hooks:
     pre_sync:
       npm install
   ```

4. **Check hook output:**
   Hook stdout/stderr is displayed - look for error messages

5. **Use absolute paths:**

   ```yaml
   # ✅ Better
   hooks:
     pre_sync: /usr/local/bin/npm install

   # ⚠️ May fail if npm not in PATH
   hooks:
     pre_sync: npm install
   ```

---

### Issue: post_sync hook failed

**Symptoms:**

```text
✖ Error syncing vendor-name
  post_sync hook failed: exit status 1
  (Files already copied to destination)
```

**Cause:**

- Same as pre_sync issues
- Hook depends on files that weren't copied
- Build command failed

**Solutions:**

1. **Files are already copied:**
   Even if post_sync fails, vendored files are in place

2. **Test hook manually:**

   ```bash
   # Navigate to project root
   cd /path/to/project

   # Set environment variables
   export GIT_VENDOR_NAME="vendor-name"
   export GIT_VENDOR_FILES_COPIED="5"

   # Run hook
   sh -c "cd vendor/ui && npm run build"
   ```

3. **Common build failures:**

   - Missing dependencies: `npm install`, `go mod download`
   - Wrong directory: Use `cd` in hook
   - Missing tools: Install required tools

4. **Example working hooks:**

   ```yaml
   # Example 1: npm build
   hooks:
     post_sync: |
       cd vendor/frontend
       npm ci
       npm run build

   # Example 2: Go codegen
   hooks:
     post_sync: go generate ./vendor/...

   # Example 3: Cleanup
   hooks:
     post_sync: rm vendor/lib/test_*.go
   ```

---

### Issue: Hook environment variables not set

**Symptoms:**

Hook references `$GIT_VENDOR_NAME` but it's empty

**Cause:**

- Hook runs in subshell via `sh -c`
- Environment variables are set by git-vendor before execution

**Solution:**

**Available environment variables:**

- `GIT_VENDOR_NAME` - Vendor name
- `GIT_VENDOR_URL` - Repository URL
- `GIT_VENDOR_REF` - Git ref (branch/tag)
- `GIT_VENDOR_COMMIT` - Resolved commit hash
- `GIT_VENDOR_ROOT` - Project root directory
- `GIT_VENDOR_FILES_COPIED` - Number of files copied (post_sync only)

**Example usage:**

```yaml
hooks:
  pre_sync: echo "Syncing $GIT_VENDOR_NAME from $GIT_VENDOR_URL"
  post_sync: |
    echo "Copied $GIT_VENDOR_FILES_COPIED files"
    echo "Commit: $GIT_VENDOR_COMMIT"
```

---

### Issue: Hook runs even when cache hits

**Symptoms:**

```text
Cache hit, skipping clone
Running post_sync hook...
```

**Expected behavior:**
This is intentional - hooks run even for cache hits

**Rationale:**

- Build artifacts may be missing even if source files are cached
- Allows rebuilding without full re-sync
- Use `--no-cache` to force full sync if needed

**Solutions:**

1. **If you want hook to run only on actual downloads:**

   Check `$GIT_VENDOR_FILES_COPIED`:

   ```yaml
   hooks:
     post_sync: |
       if [ "$GIT_VENDOR_FILES_COPIED" -gt 0 ]; then
         npm run build
       else
         echo "Cache hit, skipping build"
       fi
   ```

2. **If you want to disable hooks temporarily:**

   Comment out in vendor.yml:

   ```yaml
   hooks:
     # pre_sync: echo "disabled"
     # post_sync: npm run build
   ```

---

### Issue: Testing hooks without syncing

**Purpose:**
Debug hook commands without running full sync

**Solution:**

```bash
# 1. Extract hook command from vendor.yml
cat vendor/vendor.yml

# 2. Set environment variables
export GIT_VENDOR_NAME="my-vendor"
export GIT_VENDOR_URL="https://github.com/owner/repo"
export GIT_VENDOR_REF="main"
export GIT_VENDOR_COMMIT="abc123"
export GIT_VENDOR_ROOT="$(pwd)"
export GIT_VENDOR_FILES_COPIED="10"

# 3. Run hook manually
sh -c "npm install && npm run build"

# 4. Check exit code
echo $?  # 0 = success, non-zero = failure
```

---

## Parallel Processing

### Issue: Parallel sync slower than sequential

**Symptoms:**

```bash
git-vendor sync --parallel
# Takes 20 seconds

git-vendor sync
# Takes 15 seconds
```

**Cause:**

- Only one vendor (no benefit from parallelization)
- Disk I/O bottleneck (SSD helps)
- Worker overhead exceeds benefit for small vendors
- Network is the bottleneck, not CPU

**Solutions:**

1. **Use parallel only for multiple vendors:**

   ```bash
   # ✅ Beneficial (4+ vendors)
   git-vendor sync --parallel

   # ❌ No benefit (1-2 vendors)
   git-vendor sync  # Sequential is fine
   ```

2. **Adjust worker count:**

   ```bash
   # Default: NumCPU (max 8)
   git-vendor sync --parallel

   # Custom worker count
   git-vendor sync --parallel --workers 4
   ```

3. **Profile with verbose mode:**

   ```bash
   git-vendor sync --parallel --verbose
   # Look for bottlenecks in git operations
   ```

**When parallel helps:**

- 4+ vendors
- Fast network
- CPU-bound operations (not network-bound)
- SSD storage

**When sequential is better:**

- 1-2 vendors
- Slow network
- HDD storage
- Limited bandwidth

---

### Issue: Worker pool timeout

**Symptoms:**

```text
✖ Error: context deadline exceeded
```

**Cause:**

- Vendor taking too long to sync (>30 seconds for directory listing)
- Network timeout
- Very large repository

**Solutions:**

1. **This is a git timeout, not a worker timeout**

   - Git operations have 30-second context timeout
   - Parallel mode doesn't change timeout behavior

2. **Try sequential mode:**

   ```bash
   git-vendor sync  # No --parallel flag
   ```

3. **See "Performance Issues" section for timeout solutions**

---

### Issue: When to use --parallel flag

**Use parallel when:**

✅ You have 4+ vendors
✅ Fast network connection
✅ Multi-core CPU
✅ SSD storage

**Don't use parallel when:**

❌ 1-3 vendors (minimal benefit)
❌ Slow network (network is bottleneck)
❌ Single-core CPU
❌ Using --dry-run (auto-disabled)

**Example:**

```bash
# Project with 10 vendors on fast network
git-vendor sync --parallel
# ✅ 3-5x faster

# Project with 2 vendors
git-vendor sync
# ✅ Simpler, minimal difference
```

---

## Watch Mode

### Issue: Watch not detecting changes

**Symptoms:**

```bash
git-vendor watch
# Edit vendor/vendor.yml
# No sync triggered
```

**Cause:**

- File not saved to disk
- Editor using atomic saves (creates temp file)
- Wrong file being edited
- Watch crashed silently

**Solutions:**

1. **Verify watch is running:**

   ```text
   👀 Watching vendor/vendor.yml for changes...
   Press Ctrl+C to stop.
   ```

2. **Save file properly:**

   - Ensure file is saved (not just edited in buffer)
   - Wait 1 second (debounce period)

3. **Check which file watch monitors:**

   ```bash
   # Watches this file only:
   ls -la vendor/vendor.yml
   ```

4. **Test with manual edit:**

   ```bash
   # Terminal 1
   git-vendor watch

   # Terminal 2
   echo "# test" >> vendor/vendor.yml
   # Should trigger sync after 1 second
   ```

5. **Restart watch:**

   ```bash
   # Stop with Ctrl+C
   # Start again
   git-vendor watch
   ```

---

### Issue: Watch triggering too frequently

**Symptoms:**

```text
Change detected, syncing...
Change detected, syncing...
Change detected, syncing...
```

**Cause:**

- Editor creating multiple file events
- Backup files being created
- Auto-save enabled

**Solutions:**

1. **This is normal for rapid edits:**

   - Watch has 1-second debounce
   - Multiple edits within 1 second = one sync

2. **Disable auto-save in editor:**

   - VSCode: `"files.autoSave": "off"`
   - Vim: No special config needed
   - Emacs: `(setq auto-save-default nil)`

3. **Use manual sync during heavy editing:**

   ```bash
   # Stop watch (Ctrl+C)
   # Edit vendor.yml
   # Manual sync when done
   git-vendor sync
   ```

---

### Issue: Watch exited after error

**Symptoms:**

```text
👀 Watching vendor/vendor.yml for changes...
Change detected, syncing...
✖ Error syncing vendor-name
  [Watch exits]
```

**Cause:**

- Watch exits on first sync error
- Invalid vendor.yml syntax
- Vendor sync failed

**Solutions:**

1. **Fix the error:**

   ```bash
   # Check what failed
   git-vendor sync
   # Fix issue in vendor.yml
   ```

2. **Restart watch:**

   ```bash
   git-vendor watch
   ```

3. **Validate before watching:**

   ```bash
   git-vendor validate  # Check for errors
   git-vendor watch     # Start watching
   ```

**Future improvement:**
Watch mode could continue running after errors (not implemented yet)

---

### Issue: High CPU usage in watch mode

**Symptoms:**

```bash
git-vendor watch
# CPU usage 10-20% continuously
```

**Cause:**

- File watcher polling (depends on OS)
- Multiple watch events
- This is generally expected behavior

**Solutions:**

1. **This is normal:**

   - File watching has some overhead
   - Should be <5% CPU when idle
   - Spikes during sync are expected

2. **If CPU >20% when idle:**

   - Check if other processes are watching same file
   - Restart watch mode
   - Report as bug with OS details

3. **Alternative: Don't use watch:**

   ```bash
   # Manual sync when needed
   git-vendor sync

   # Or use git hooks
   # Create .git/hooks/post-checkout
   ```

---

## Git Issues

### Error: "git not found"

**Symptoms:**

```text
✖ Error
git not found.
```

**Cause:**
Git is not installed or not in system PATH.

**Solution:**

1. **Install Git:**

   - macOS: `brew install git`
   - Ubuntu/Debian: `sudo apt-get install git`
   - Windows: Download from <https://git-scm.com/>

2. **Verify installation:**

   ```bash
   git --version
   ```

3. **Add Git to PATH:**
   If installed but not found, add Git's bin directory to your system PATH.

---

### Error: "fatal: remote error: upload-pack not permitted"

**Symptoms:**
Git clone fails with permission error.

**Cause:**

- Private repository requires authentication
- Repository doesn't exist or is not accessible

**Solution:**

1. **Verify repository exists:**
   Try accessing it in a web browser

2. **For private repositories, configure authentication:**

   **GitHub/GitLab:**
   ```bash
   export GITHUB_TOKEN=your_token_here
   # or
   export GITLAB_TOKEN=your_token_here
   ```

   **Other Git servers (SSH):**
   ```bash
   # Ensure SSH key is loaded
   ssh-add ~/.ssh/id_rsa

   # Use SSH URL
   git-vendor add git@server.com:org/repo.git
   ```

   See [PLATFORMS.md](./PLATFORMS.md) for detailed authentication guide.

3. **Check URL spelling:**
   Ensure the URL is correct

---

## TUI/Display Issues

### Issue: Colors not showing / garbled output

**Symptoms:**

- Colors appear as escape codes
- Text is garbled or unreadable
- Emoji don't display correctly

**Causes:**

- Terminal doesn't support 256 colors
- Running in CI/CD environment
- Using an older terminal emulator

**Solutions:**

1. **For CI/CD environments:**

   ```bash
   # Use with automated scripts to disable interactive prompts
   # (Future: --no-color flag)
   ```

2. **Update terminal:**

   - Use a modern terminal emulator
   - iTerm2 (macOS), Windows Terminal (Windows), or any terminal with 256-color support

3. **Check terminal environment:**

   ```bash
   echo $TERM  # Should show something like "xterm-256color"
   ```

---

### Issue: Wizard exits immediately / Ctrl+C during wizard

**Symptoms:**

- Pressed Ctrl+C during wizard and it exited
- Want to cancel wizard without exiting

**Explanation:**
This is expected behavior - Ctrl+C cancels the operation.

**Solution:**

- To cancel: Press Ctrl+C (operation will be aborted)
- To go back: Use the "← Back" or "❌ Cancel" options in the wizard
- No changes are made until you select "💾 Save & Exit"

---

### Issue: Changes made in wizard not persisting

**Symptoms:**

- Made changes in the `add` or `edit` wizard
- Exited without seeing changes saved
- Configuration appears unchanged

**Explanation:**
The wizard does not save changes until you explicitly select "💾 Save & Exit" from the main menu.

**How the wizard works:**

1. All edits (adding vendors, branches, path mappings) are made in memory
2. You can navigate freely using "← Back" to make changes
3. Changes are discarded if you:
   - Press Ctrl+C
   - Select "❌ Cancel"
   - Exit the terminal
4. Changes are only persisted when you select "💾 Save & Exit"

**Solution:**

Always complete your workflow by selecting "💾 Save & Exit":

```text
┃ Select Branch to Manage
┃ > main (2 paths, synced)
┃   + Add New Branch
┃   💾 Save & Exit  ← Select this to persist changes
┃   ❌ Cancel
```

After selecting "💾 Save & Exit", the wizard will:

- Save changes to `vendor.yml`
- Run `update` to regenerate `vendor.lock`
- Check for path conflicts and display warnings if any

---

## General Debugging

### Enable verbose output

git-vendor supports verbose mode with the `--verbose` or `-v` flag:

```bash
# Show git commands during sync
git-vendor sync --verbose

# Show git commands during update
git-vendor update -v
```

**What verbose mode shows:**

- All git commands being executed
- Working directories for git operations
- Helpful for debugging network issues or git errors

**Additional debugging steps:**

1. **Check configuration:**

   ```bash
   cat vendor/vendor.yml
   cat vendor/vendor.lock
   ```

2. **Test Git access:**

   ```bash
   git clone --depth 1 <repository-url> test-dir
   cd test-dir
   git log -1
   ```

3. **Verify paths:**

   ```bash
   git clone <repository-url> test-dir
   cd test-dir
   git checkout <ref>
   ls -la <path>  # Check if path exists
   ```

---

## Getting More Help

If you encounter an issue not covered here:

1. **Check existing issues:**
   <https://github.com/yourusername/git-vendor/issues>

2. **Create a new issue with:**

   - Command you ran
   - Full error message
   - Contents of `vendor.yml` (sanitized)
   - Git and git-vendor versions
   - Operating system

3. **Include debug info:**

   ```bash
   git --version
   git-vendor --help  # Version shown in title
   cat vendor/vendor.yml
   ```
