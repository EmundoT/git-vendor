#!/bin/bash
# PostToolUse hook: auto-format Go files after Edit/Write.
# Exit 0 = no-op, Exit 2 = block and report to Claude.
set -e

INPUT=$(cat)

# Graceful fallback if jq is not installed
if ! command -v jq &>/dev/null; then
  exit 0
fi

FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Skip non-Go files
if [[ "$FILE_PATH" != *.go ]]; then
  exit 0
fi

# Skip files that may not exist yet during creation
if [[ ! -f "$FILE_PATH" ]]; then
  exit 0
fi

if ! gofmt -w "$FILE_PATH" 2>&1; then
  echo "gofmt failed on $FILE_PATH" >&2
  exit 2
fi

exit 0
