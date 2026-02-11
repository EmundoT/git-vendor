#!/bin/bash
# Stop hook: full build/test/vet before Claude declares work done.
# Exit 0 = all clear, Exit 2 = block stop and feed errors to Claude.

INPUT=$(cat)

# Graceful fallback if jq is not installed
if ! command -v jq &>/dev/null; then
  cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0
else
  # Prevent infinite Stop hook loops
  if [ "$(echo "$INPUT" | jq -r '.stop_hook_active // empty')" = "true" ]; then
    exit 0
  fi
  CWD="$(echo "$INPUT" | jq -r '.cwd // empty')"
  cd "${CWD:-$CLAUDE_PROJECT_DIR}"
fi

echo "Running go build..." >&2
if ! go build ./... 2>&1; then
  echo "Build failed. Fix errors before completing." >&2
  exit 2
fi

echo "Running go vet..." >&2
if ! go vet ./... 2>&1; then
  echo "go vet found issues. Fix before completing." >&2
  exit 2
fi

echo "Running go test..." >&2
if ! go test ./... 2>&1; then
  echo "Tests failed. Fix failing tests before completing." >&2
  exit 2
fi

echo "All checks passed." >&2
exit 0
