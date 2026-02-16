#!/usr/bin/env bash
# vendor-guard.sh — Pre-commit hook: blocks commits that modify vendored files.
# Reads .git-vendor/vendor.lock to identify vendored file paths, then checks
# if any staged files match. Exits non-zero with a diagnostic if so.
#
# Install: symlink or copy to .githooks/pre-commit (or chain from existing pre-commit).
# Also usable as a Claude Code PreToolUse hook (checks CLAUDE_TOOL_INPUT for git commit).
#
# Exit codes:
#   0  no vendored files staged (or no vendor.lock found)
#   1  vendored files staged — commit blocked

LOCK_FILE=".git-vendor/vendor.lock"

# If no vendor.lock, nothing to guard
if [[ ! -f "$LOCK_FILE" ]]; then
  exit 0
fi

# Extract vendored file paths from vendor.lock file_hashes keys.
# Format in YAML: "        path/to/file: sha256hash"
# We extract the path portion (everything before the colon after leading whitespace).
VENDORED_PATHS=$(grep -E '^\s+\S+\.\S+: [0-9a-f]{64}$' "$LOCK_FILE" | sed 's/^[[:space:]]*//' | cut -d: -f1)

if [[ -z "$VENDORED_PATHS" ]]; then
  exit 0
fi

# Get staged files
STAGED=$(git diff --cached --name-only 2>/dev/null)
if [[ -z "$STAGED" ]]; then
  exit 0
fi

# Check for matches
BLOCKED=""
while IFS= read -r vendored; do
  [[ -z "$vendored" ]] && continue
  while IFS= read -r staged; do
    [[ -z "$staged" ]] && continue
    if [[ "$staged" == "$vendored" ]]; then
      # Find which vendor owns this file
      VENDOR_NAME=$(awk -v path="$vendored" '
        /^    - name:/ { name=$NF }
        $0 ~ path { print name; exit }
      ' "$LOCK_FILE")
      BLOCKED="${BLOCKED}  ${staged} (vendored from ${VENDOR_NAME:-unknown})\n"
    fi
  done <<< "$STAGED"
done <<< "$VENDORED_PATHS"

if [[ -n "$BLOCKED" ]]; then
  echo "VENDOR GUARD: The following staged files are vendored — edit upstream instead:"
  printf "$BLOCKED"
  echo ""
  echo "To update vendored files, edit the source repo and run: git-vendor sync <vendor-name>"
  echo "To bypass this check (NOT recommended): git commit --no-verify"
  exit 1
fi

exit 0
