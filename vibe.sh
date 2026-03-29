#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./vibe.sh [max-duration]

Launch a paired vibe session for ARM64 container porting work.

The first side advances one recipe in ARM64_PORT_PROGRESS.md to `built`.
The second side advances one `built` recipe in ARM64_PORT_PROGRESS.md to `completed`.

Examples:
  ./vibe.sh
  ./vibe.sh 8h
EOF
}

retry_failure_notify() {
  local exit_code="$1"
  local message="vibe pair failed in $SCRIPT_DIR with exit code $exit_code; retrying notify until it succeeds"

  while true; do
    if notify -message "$message"; then
      return 0
    fi
    sleep 30
  done
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -gt 1 ]]; then
  usage >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAX_DURATION="${1:-8h}"
VIBE_BIN="${VIBE_BIN:-$HOME/vibe}"
MODEL="${VIBE_MODEL:-}"
CPU_QUOTA="${VIBE_CPU_QUOTA:-300%}"
MEMORY_MAX="${VIBE_MEMORY_MAX:-12G}"

if [[ ! -x "$VIBE_BIN" ]]; then
  echo "vibe executable not found or not executable: $VIBE_BIN" >&2
  exit 1
fi

BUILD_PROMPT='read one recipe from ARM64_PORT_PROGRESS.md and progress it from not-started or build-attempted to built then update ARM64_PORT_PROGRESS.md'
TEST_PROMPT='read one recipe from ARM64_PORT_PROGRESS.md and progress it from built or tested to completed then update ARM64_PORT_PROGRESS.md'

cmd=("$VIBE_BIN" pair --cwd "$SCRIPT_DIR" --git --notify)
if [[ -n "$MODEL" ]]; then
  cmd+=(--model "$MODEL")
fi
if [[ -n "$CPU_QUOTA" ]]; then
  cmd+=(--cpu-quota "$CPU_QUOTA")
fi
if [[ -n "$MEMORY_MAX" ]]; then
  cmd+=(--memory-max "$MEMORY_MAX")
fi
cmd+=("$BUILD_PROMPT" "$TEST_PROMPT" "$MAX_DURATION")

status=0
if "${cmd[@]}"; then
  :
else
  status=$?
  retry_failure_notify "$status"
fi

exit "$status"
