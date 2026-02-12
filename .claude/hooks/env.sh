#!/bin/bash
# Shared environment setup for Claude Code hooks.
# Source this at the top of any hook that needs Go or file path conversion.
# Handles Windows (Git Bash/MSYS2/MINGW64), WSL2, macOS, and Linux.

# --- Path conversion utility ---
# Convert a Windows path (C:\foo\bar) to a unix path.
# Git Bash/MSYS2: /c/foo/bar. WSL: /mnt/c/foo/bar.
# On non-Windows systems or already-unix paths, returns the input unchanged.
win_to_unix_path() {
  local p="$1"
  case "$p" in
    [A-Za-z]:\\*)
      p="${p//\\//}"
      local drive="${p%%:*}"
      drive="${drive,,}"
      if [ -d /mnt/c/ ]; then
        echo "/mnt/${drive}${p#?:}"
      else
        echo "/${drive}${p#?:}"
      fi
      ;;
    *)
      echo "$p"
      ;;
  esac
}

# --- Go PATH discovery ---
if ! command -v go &>/dev/null; then
  for dir in \
    "/c/Program Files/Go/bin" \
    "/mnt/c/Program Files/Go/bin" \
    "/usr/local/go/bin" \
    "$HOME/go/bin" \
    "$HOME/.go/bin" \
    "/usr/lib/go/bin" \
    "/snap/go/current/bin"
  do
    if [ -x "$dir/go" ] || [ -x "$dir/go.exe" ]; then
      export PATH="$dir:$PATH"
      break
    fi
  done
fi

if command -v go &>/dev/null; then
  export GO_AVAILABLE=1
else
  export GO_AVAILABLE=0
fi
