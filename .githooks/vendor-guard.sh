#!/bin/sh
# vendor-guard.sh — Pre-commit guard for git-vendor (GRD-001/GRD-002/GRD-003).
# Two-pass approach:
#   Pass 1: Offline check (fast, no network) — detects drift
#   Pass 2: Full check (with remote) — runs only when block_on_stale is enabled
#            in vendor.yml, to detect staleness beyond max_staleness_days.
#
# Exit codes:
#   0 — No policy violations (or git-vendor not applicable)
#   1 — Policy violation detected, commit blocked
#
# Dependencies:
#   - git-vendor binary (skips silently if not found)
#   - jq (optional; falls back to grep/awk parsing)
#
# Usage:
#   Called from .githooks/pre-commit or installed standalone.
#   POSIX-compatible (#!/bin/sh).
# #guard #drift-detection #staleness #pre-commit

# --- Guard: skip if git-vendor is not installed ---
if ! command -v git-vendor >/dev/null 2>&1; then
    exit 0
fi

# --- Guard: skip if not a vendored project ---
if [ ! -f ".git-vendor/vendor.yml" ]; then
    exit 0
fi

# --- Pass 1: Offline check (fast, no network) ---
STATUS_JSON=$(git-vendor status --offline --format=json 2>/dev/null)

# If status command produced no output, skip guard (fatal error).
if [ -z "$STATUS_JSON" ]; then
    exit 0
fi

# --- GRD-003: Check if block_on_stale is enabled anywhere in vendor.yml ---
# If so, re-run status WITHOUT --offline to get remote staleness data.
# This avoids network latency on every commit when staleness checking is off.
NEEDS_REMOTE=0
if command -v jq >/dev/null 2>&1; then
    # Precise YAML check not possible in pure shell; grep vendor.yml for the key.
    # False positives (commented-out lines) are acceptable — worst case is an
    # extra network round-trip.
    :
fi
# Both jq and non-jq paths use the same grep heuristic on vendor.yml.
# Filter out YAML comment lines before matching to avoid false positives (I5).
if grep -v '^[[:space:]]*#' ".git-vendor/vendor.yml" 2>/dev/null | \
   grep -q 'block_on_stale:[[:space:]]*true'; then
    NEEDS_REMOTE=1
fi

if [ "$NEEDS_REMOTE" -eq 1 ]; then
    FULL_JSON=$(git-vendor status --format=json 2>/dev/null)
    if [ -n "$FULL_JSON" ]; then
        STATUS_JSON="$FULL_JSON"
    fi
    # If full check failed, fall back to offline results (already in STATUS_JSON).
fi

# --- Parse JSON for policy violations and drift ---
BLOCK=0

if command -v jq >/dev/null 2>&1; then
    # --- jq path: precise JSON parsing ---

    # Check policy violations first (GRD-002/GRD-003). Only "error" severity blocks.
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
    # Check for policy error violations (GRD-002/GRD-003)
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
            # Extract drift_details entries and display path + hash info (GAP-G1).
            # drift_details objects have "path", "lock_hash", "disk_hash", "accepted" fields.
            # Parse each path/lock_hash/disk_hash from the JSON without jq.
            DRIFT_PATHS=$(printf '%s' "$STATUS_JSON" | grep -o '"drift_details"[[:space:]]*:[[:space:]]*\[[^]]*\]' 2>/dev/null)
            if [ -n "$DRIFT_PATHS" ]; then
                printf '%s' "$DRIFT_PATHS" | grep -o '"path":"[^"]*"' | sed 's/"path":"//;s/"$//' | while read -r dpath; do
                    echo "  $dpath" >&2
                done
                # Show lock/disk hashes for each drift entry
                printf '%s' "$DRIFT_PATHS" | grep -o '"lock_hash":"[^"]*"' | sed 's/"lock_hash":"//;s/"$//' | while read -r lh; do
                    lh_short=$(printf '%s' "$lh" | cut -c1-16)
                    echo "    lock:  ${lh_short}..." >&2
                done
                printf '%s' "$DRIFT_PATHS" | grep -o '"disk_hash":"[^"]*"' | sed 's/"disk_hash":"//;s/"$//' | while read -r dh; do
                    dh_short=$(printf '%s' "$dh" | cut -c1-16)
                    echo "    disk:  ${dh_short}..." >&2
                done
            else
                # Fallback: extract modified_paths if drift_details not available
                printf '%s' "$STATUS_JSON" | grep -o '"modified_paths"[[:space:]]*:[[:space:]]*\[[^]]*\]' | \
                    grep -o '"[^"]*"' | sed 's/"//g' | while read -r path; do
                    echo "  $path (lock hash mismatch)" >&2
                done
            fi
            BLOCK=1
        fi
    fi
fi

if [ "$BLOCK" -eq 0 ]; then
    exit 0
fi

echo "" >&2
echo "Resolve with:" >&2

# Check if there are deleted files (I6). Deleted files cannot be accepted or pushed;
# they can only be restored via pull or removed from vendor.yml.
HAS_DELETED=0
if command -v jq >/dev/null 2>&1; then
    DELETED_COUNT=$(printf '%s' "$STATUS_JSON" | jq -r '.summary.deleted // 0' 2>/dev/null)
    DELETED_COUNT=${DELETED_COUNT:-0}
    if [ "$DELETED_COUNT" -gt 0 ] 2>/dev/null; then
        HAS_DELETED=1
    fi
else
    DELETED_COUNT=$(printf '%s' "$STATUS_JSON" | grep -o '"deleted":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*$')
    DELETED_COUNT=${DELETED_COUNT:-0}
    if [ "$DELETED_COUNT" -gt 0 ] 2>/dev/null; then
        HAS_DELETED=1
    fi
fi

echo "  git vendor pull     # restore upstream files and get latest" >&2
if [ "$HAS_DELETED" -eq 0 ]; then
    echo "  git vendor push     # propose changes upstream" >&2
    echo "  git vendor accept   # acknowledge drift, update lock" >&2
else
    echo "  git vendor push     # propose modified files upstream (not applicable to deletions)" >&2
    echo "  git vendor accept   # acknowledge modified files (not applicable to deletions)" >&2
    echo "" >&2
    echo "For deleted vendored files, use 'git vendor pull' to restore or remove" >&2
    echo "the mapping from vendor.yml if the file is no longer needed." >&2
fi
echo "" >&2
echo "Commit blocked." >&2
exit 1
