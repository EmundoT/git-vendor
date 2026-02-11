#!/bin/bash
# PostToolUse hook: auto-format Go files after Edit/Write.
# Exit 0 = no-op, Exit 2 = block and report to Claude.
set -e

HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$HOOK_DIR/env.sh"

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Skip non-Go files
if [[ "$FILE_PATH" != *.go ]]; then
  exit 0
fi

# Skip if gofmt isn't available
if ! command -v gofmt &>/dev/null; then
  exit 0
fi

# Convert Windows path if needed
FILE_PATH_UNIX=$(win_to_unix_path "$FILE_PATH")

# Skip files that don't exist yet during creation
if [[ ! -f "$FILE_PATH_UNIX" ]]; then
  exit 0
fi

if ! gofmt -w "$FILE_PATH_UNIX" 2>&1; then
  echo "gofmt failed on $FILE_PATH" >&2
  exit 2
fi

exit 0
