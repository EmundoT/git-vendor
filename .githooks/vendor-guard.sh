#!/bin/sh
# vendor-guard.sh — Pre-commit drift detection for git-vendor (GRD-001).
# Runs "git-vendor status --offline --format json --yes" and blocks the commit
# if any vendored files have unacknowledged drift (modified but not accepted).
#
# Exit codes:
#   0 — No unacknowledged drift (or git-vendor not applicable)
#   1 — Unacknowledged drift detected, commit blocked
#
# Dependencies:
#   - git-vendor binary (skips silently if not found)
#   - jq (optional; falls back to grep/awk parsing)
#
# Usage:
#   Called from .githooks/pre-commit or installed standalone.
#   POSIX-compatible (#!/bin/sh).
# #guard #drift-detection #pre-commit

# --- Guard: skip if git-vendor is not installed ---
if ! command -v git-vendor >/dev/null 2>&1; then
    exit 0
fi

# --- Guard: skip if not a vendored project ---
if [ ! -f ".git-vendor/vendor.yml" ]; then
    exit 0
fi

# --- Run status check ---
STATUS_JSON=$(git-vendor status --offline --format=json --yes 2>/dev/null)
EXIT_CODE=$?

# If status command itself failed (exit 1 = FAIL which is expected for drift),
# we still parse the JSON. Only bail on unexpected errors (no JSON output).
if [ -z "$STATUS_JSON" ]; then
    # No output at all — git-vendor may have errored fatally. Skip guard.
    exit 0
fi

# --- Parse JSON for unacknowledged drift ---
# Strategy: look for drift_details entries where accepted is false.
# Two parsing paths: jq (preferred) and grep/awk (fallback).

if command -v jq >/dev/null 2>&1; then
    # --- jq path: precise JSON parsing ---
    UNACKED=$(printf '%s' "$STATUS_JSON" | jq -r '
        [.vendors[]? | .drift_details[]? | select(.accepted == false)]
        | if length == 0 then empty
          else .[] | "\(.path)\t\(.lock_hash)\t\(.disk_hash)"
          end
    ' 2>/dev/null)

    if [ -z "$UNACKED" ]; then
        exit 0
    fi

    # --- Block commit with actionable message ---
    echo "[git-vendor] Lock mismatch detected:" >&2
    printf '%s\n' "$UNACKED" | while IFS="$(printf '\t')" read -r path lock_hash disk_hash; do
        echo "  $path" >&2
        lock_short=$(printf '%s' "$lock_hash" | cut -c1-10)
        disk_short=$(printf '%s' "$disk_hash" | cut -c1-10)
        echo "    lock:  $lock_short..." >&2
        echo "    disk:  $disk_short..." >&2
    done
else
    # --- grep/awk fallback: pattern-match JSON text ---
    # Look for "modified_paths" with entries that aren't in "accepted_paths".
    # Simpler heuristic: if summary.modified > 0 and summary.accepted < summary.modified,
    # there are unacknowledged modifications.

    MODIFIED=$(printf '%s' "$STATUS_JSON" | grep -o '"modified":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*$')
    ACCEPTED=$(printf '%s' "$STATUS_JSON" | grep -o '"accepted":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*$')

    MODIFIED=${MODIFIED:-0}
    ACCEPTED=${ACCEPTED:-0}

    if [ "$MODIFIED" -eq 0 ] 2>/dev/null; then
        exit 0
    fi

    if [ "$MODIFIED" -le "$ACCEPTED" ] 2>/dev/null; then
        exit 0
    fi

    # --- Block commit ---
    echo "[git-vendor] Lock mismatch detected:" >&2

    # Extract modified paths from JSON (best-effort line-by-line grep)
    printf '%s' "$STATUS_JSON" | grep -o '"modified_paths"[[:space:]]*:[[:space:]]*\[[^]]*\]' | \
        grep -o '"[^"]*"' | sed 's/"//g' | while read -r path; do
        echo "  $path" >&2
    done
fi

echo "" >&2
echo "Resolve with:" >&2
echo "  git vendor pull     # discard local changes, get latest" >&2
echo "  git vendor push     # propose changes upstream" >&2
echo "  git vendor accept   # acknowledge drift, update lock" >&2
echo "" >&2
echo "Commit blocked." >&2
exit 1
