#!/usr/bin/env bash
# vendor-guard.sh — Pre-commit hook: blocks rogue edits to vendored files.
# Reads .git-vendor/vendor.lock to identify vendored file paths and their
# expected SHA-256 hashes. For each staged vendored file, compares the staged
# content hash against the lock hash:
#   - Hash match   → allow (legitimate git-vendor sync output)
#   - Hash mismatch → block (rogue edit — must edit upstream instead)
#   - Not in lock  → block (unrecognized vendored path)
#
# CRLF is normalized (stripped) before hashing to avoid false mismatches on
# Windows, consistent with hook-parity.sh.
#
# Install: symlink or copy to .githooks/pre-commit (or chain from existing pre-commit).
# Also usable as a Claude Code PreToolUse hook (checks CLAUDE_TOOL_INPUT for git commit).
#
# Exit codes:
#   0  no vendored files staged, or all staged vendored files match lock hashes
#   1  one or more vendored files have hash mismatches — commit blocked

LOCK_FILE=".git-vendor/vendor.lock"

# If no vendor.lock, nothing to guard
if [[ ! -f "$LOCK_FILE" ]]; then
  exit 0
fi

# Portable SHA-256: prefer sha256sum, fall back to shasum -a 256.
# Normalizes CRLF → LF before hashing (consistent with hook-parity.sh).
if command -v sha256sum >/dev/null 2>&1; then
  sha256_stdin() { tr -d '\r' | sha256sum | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256_stdin() { tr -d '\r' | shasum -a 256 | awk '{print $1}'; }
else
  echo "VENDOR GUARD: ERROR — neither sha256sum nor shasum found" >&2
  exit 1
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

# Check for matches — compare staged content hash against vendor.lock hash
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

      # Extract expected hash from vendor.lock for this file path.
      # Matches "        path/to/file: <64-char-hex>" and extracts the hex.
      LOCK_HASH=$(grep -E "^[[:space:]]+${vendored}: [0-9a-f]{64}$" "$LOCK_FILE" \
        | sed 's/^[[:space:]]*//' | awk -F': ' '{print $NF}')

      if [[ -z "$LOCK_HASH" ]]; then
        # Path matched as vendored but no hash in lock — still block
        BLOCKED="${BLOCKED}  ${staged} (vendored from ${VENDOR_NAME:-unknown}, no hash in vendor.lock)\n"
        continue
      fi

      # Hash staged content (from index, not working tree) with CRLF normalization
      STAGED_HASH=$(git show ":${staged}" 2>/dev/null | sha256_stdin)

      if [[ "$STAGED_HASH" != "$LOCK_HASH" ]]; then
        BLOCKED="${BLOCKED}  ${staged} (vendored from ${VENDOR_NAME:-unknown}, hash mismatch)\n"
        BLOCKED="${BLOCKED}    staged:   ${STAGED_HASH}\n"
        BLOCKED="${BLOCKED}    expected: ${LOCK_HASH}\n"
      fi
      # Hash match → legitimate sync commit, allow through silently
    fi
  done <<< "$STAGED"
done <<< "$VENDORED_PATHS"

if [[ -n "$BLOCKED" ]]; then
  echo "VENDOR GUARD: Staged vendored files have hash mismatches — edit upstream instead:"
  printf "$BLOCKED"
  echo ""
  echo "If these files were synced with git-vendor, re-run: git-vendor sync <vendor-name>"
  echo "To bypass this check (NOT recommended): git commit --no-verify"
  exit 1
fi

exit 0
