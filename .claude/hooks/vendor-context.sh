#!/usr/bin/env bash
# vendor-context.sh — SessionStart hook: injects vendored file list into agent context.
# Reads .git-vendor/vendor.lock and outputs a summary of vendored files so agents
# know which files to avoid editing at the start of every session.
#
# Hook type: SessionStart (runs once when a Claude Code session begins)
# Output: Printed to stdout, becomes part of the agent's context.

LOCK_FILE=".git-vendor/vendor.lock"

if [[ ! -f "$LOCK_FILE" ]]; then
  exit 0
fi

# Extract vendor names and their file paths
CURRENT_VENDOR=""
HAS_FILES=false

echo "VENDORED FILES — do not edit locally. Use 'git-vendor sync <name>' to update."

awk '
  /^    - name:/ {
    vendor = $NF
  }
  /^      file_hashes:/ {
    in_hashes = 1
    next
  }
  in_hashes && /^        [^ ]/ {
    split($0, parts, ":")
    gsub(/^[[:space:]]+/, "", parts[1])
    print "  " parts[1] " ← " vendor
    next
  }
  in_hashes && !/^        / {
    in_hashes = 0
  }
' "$LOCK_FILE"
