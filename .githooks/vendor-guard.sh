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

BLOCK=0

if command -v jq >/dev/null 2>&1; then
    # --- jq path: precise JSON parsing ---

    # Check policy violations first (GRD-002). Only "error" severity blocks.
    POLICY_ERRORS=$(printf '%s' "$STATUS_JSON" | jq -r '
        [.policy_violations[]? | select(.severity == "error")]
        | if length == 0 then empty
          else .[] | "\(.vendor_name)\t\(.type)\t\(.message)"
          end
    ' 2>/dev/null)

    if [ -n "$POLICY_ERRORS" ]; then
        echo "[git-vendor] Policy violation(s) detected:" >&2
        printf '%s\n' "$POLICY_ERRORS" | while IFS="$(printf '\t')" read -r vendor vtype msg; do
            echo "  [$vtype] $msg" >&2
        done
        BLOCK=1
    fi

    # Show warnings (non-blocking)
    POLICY_WARNINGS=$(printf '%s' "$STATUS_JSON" | jq -r '
        [.policy_violations[]? | select(.severity == "warning")]
        | if length == 0 then empty
          else .[] | "\(.vendor_name)\t\(.type)\t\(.message)"
          end
    ' 2>/dev/null)

    if [ -n "$POLICY_WARNINGS" ]; then
        echo "[git-vendor] Policy warning(s):" >&2
        printf '%s\n' "$POLICY_WARNINGS" | while IFS="$(printf '\t')" read -r vendor vtype msg; do
            echo "  [$vtype] $msg" >&2
        done
    fi

    # If no policy violations block, fall back to legacy unacknowledged drift check.
    # Policy may set block_on_drift=false for some vendors, so only check vendors
    # that have "error" severity drift violations.
    if [ "$BLOCK" -eq 0 ]; then
        # Legacy path: check for any unacknowledged drift (pre-policy behavior).
        # With policy active, drift-type errors are already caught above.
        # This handles the no-policy case where block_on_drift defaults to true.
        HAS_POLICY=$(printf '%s' "$STATUS_JSON" | jq -r '.policy_violations // [] | length' 2>/dev/null)
        if [ "${HAS_POLICY:-0}" -eq 0 ]; then
            UNACKED=$(printf '%s' "$STATUS_JSON" | jq -r '
                [.vendors[]? | .drift_details[]? | select(.accepted == false)]
                | if length == 0 then empty
                  else .[] | "\(.path)\t\(.lock_hash)\t\(.disk_hash)"
                  end
            ' 2>/dev/null)

            if [ -n "$UNACKED" ]; then
                echo "[git-vendor] Lock mismatch detected:" >&2
                printf '%s\n' "$UNACKED" | while IFS="$(printf '\t')" read -r path lock_hash disk_hash; do
                    echo "  $path" >&2
                    lock_short=$(printf '%s' "$lock_hash" | cut -c1-10)
                    disk_short=$(printf '%s' "$disk_hash" | cut -c1-10)
                    echo "    lock:  $lock_short..." >&2
                    echo "    disk:  $disk_short..." >&2
                done
                BLOCK=1
            fi
        fi
    fi
else
    # --- grep/awk fallback: pattern-match JSON text ---
    # Check for policy error violations (GRD-002)
    POLICY_ERROR_COUNT=$(printf '%s' "$STATUS_JSON" | grep -c '"severity"[[:space:]]*:[[:space:]]*"error"' 2>/dev/null)
    POLICY_ERROR_COUNT=${POLICY_ERROR_COUNT:-0}

    if [ "$POLICY_ERROR_COUNT" -gt 0 ] 2>/dev/null; then
        echo "[git-vendor] Policy violation(s) detected (run 'git vendor status' for details)" >&2
        BLOCK=1
    fi

    # Fallback drift check (no-policy case)
    if [ "$BLOCK" -eq 0 ]; then
        MODIFIED=$(printf '%s' "$STATUS_JSON" | grep -o '"modified":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*$')
        ACCEPTED=$(printf '%s' "$STATUS_JSON" | grep -o '"accepted":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*$')

        MODIFIED=${MODIFIED:-0}
        ACCEPTED=${ACCEPTED:-0}

        if [ "$MODIFIED" -gt 0 ] 2>/dev/null && [ "$MODIFIED" -gt "$ACCEPTED" ] 2>/dev/null; then
            echo "[git-vendor] Lock mismatch detected:" >&2
            printf '%s' "$STATUS_JSON" | grep -o '"modified_paths"[[:space:]]*:[[:space:]]*\[[^]]*\]' | \
                grep -o '"[^"]*"' | sed 's/"//g' | while read -r path; do
                echo "  $path" >&2
            done
            BLOCK=1
        fi
    fi
fi

if [ "$BLOCK" -eq 0 ]; then
    exit 0
fi

echo "" >&2
echo "Resolve with:" >&2
echo "  git vendor pull     # discard local changes, get latest" >&2
echo "  git vendor push     # propose changes upstream" >&2
echo "  git vendor accept   # acknowledge drift, update lock" >&2
echo "" >&2
echo "Commit blocked." >&2
exit 1
