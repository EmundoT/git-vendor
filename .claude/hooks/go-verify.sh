#!/bin/bash
# Stop hook: full build/test/vet before Claude declares work done.
# Exit 0 = all clear, Exit 2 = block stop and feed errors to Claude.

HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$HOOK_DIR/env.sh"

INPUT=$(cat)

# Prevent infinite Stop hook loops
if [ "$(echo "$INPUT" | jq -r '.stop_hook_active')" = "true" ]; then
  exit 0
fi

# Skip if Go isn't available in this environment
if [ "$GO_AVAILABLE" != "1" ]; then
  echo "go-verify: go not in PATH, skipping verification" >&2
  exit 0
fi

# Resolve working directory from hook input
CWD="$(echo "$INPUT" | jq -r '.cwd // empty')"
if [ -n "$CWD" ]; then
  CWD_UNIX=$(win_to_unix_path "$CWD")
  if [ -d "$CWD_UNIX" ]; then
    cd "$CWD_UNIX"
  fi
fi

echo "Running go build..." >&2
OUTPUT=$(go build ./... 2>&1)
if [ $? -ne 0 ]; then
  echo "$OUTPUT" >&2
  echo "Build failed. Fix errors before completing." >&2
  exit 2
fi

echo "Running go vet..." >&2
OUTPUT=$(go vet ./... 2>&1)
if [ $? -ne 0 ]; then
  echo "$OUTPUT" >&2
  echo "go vet found issues. Fix before completing." >&2
  exit 2
fi

echo "Running go test..." >&2
OUTPUT=$(go test ./... 2>&1)
if [ $? -ne 0 ]; then
  echo "$OUTPUT" >&2
  echo "Tests failed. Fix failing tests before completing." >&2
  exit 2
fi

echo "All checks passed." >&2
exit 0
