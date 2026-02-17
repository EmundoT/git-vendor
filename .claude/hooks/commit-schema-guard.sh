#!/usr/bin/env bash
# commit-schema-guard.sh — PreToolUse hook for Bash(git commit*)
# Prints protocol reminder + vendor freshness warning before commits.
# Exit 0 = allow (informational only). Git hooks handle enforcement.

TOOL_INPUT="$CLAUDE_TOOL_INPUT"

# Only act on git commit commands
if ! echo "$TOOL_INPUT" | grep -q '"command"'; then
  exit 0
fi

COMMAND=$(echo "$TOOL_INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\(.*\)".*/\1/p' | head -1)

case "$COMMAND" in
  *"git commit"*)
    echo "COMMIT-SCHEMA REMINDER: This project uses the COMMIT-SCHEMA v1 protocol."
    echo "- Hooks auto-add: Commit-Schema: manual/v1, Touch, Diff metrics, Diff-Surface"
    echo "- Vendor commits SHOULD use: git-vendor sync --commit (auto-adds vendor/v1 trailers)"
    echo "- Subject format: <type>(<scope>)[!]: <description> (max 72 chars)"
    echo ""

    # Vendor freshness check
    PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
    LOCK_FILE="$PROJECT_DIR/.git-vendor/vendor.lock"
    if [ -f "$LOCK_FILE" ]; then
      STALE_COUNT=0
      while IFS= read -r line; do
        if echo "$line" | grep -q "commit_hash:"; then
          HASH=$(echo "$line" | sed 's/.*commit_hash:[[:space:]]*//' | tr -d '"')
          # Check if vendor name precedes this hash
          VENDOR_NAME=$(echo "$PREV_LINE" | sed -n 's/.*name:[[:space:]]*\(.*\)/\1/p' | tr -d '"')
          if [ -n "$VENDOR_NAME" ] && [ -n "$HASH" ]; then
            # Check if vendored files have been modified locally (drift detection)
            VENDOR_DIR="$PROJECT_DIR/pkg/$VENDOR_NAME"
            if [ -d "$VENDOR_DIR" ]; then
              MODIFIED=$(git -C "$PROJECT_DIR" diff --name-only -- "$VENDOR_DIR" 2>/dev/null | wc -l | tr -d ' ')
              if [ "$MODIFIED" -gt 0 ]; then
                echo "WARNING: $VENDOR_NAME has $MODIFIED locally modified vendored files."
                echo "  Consider running: git-vendor update && git-vendor sync"
                STALE_COUNT=$((STALE_COUNT + 1))
              fi
            fi
          fi
          PREV_LINE=""
        else
          PREV_LINE="$line"
        fi
      done < "$LOCK_FILE"

      if [ "$STALE_COUNT" -gt 0 ]; then
        echo ""
        echo "VENDOR DRIFT: $STALE_COUNT vendor(s) have local modifications to vendored files."
      fi
    fi
    ;;
esac

exit 0
