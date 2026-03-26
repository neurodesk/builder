#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./build.sh <recipe>

Build a recipe locally with the repo's builder configuration.
EOF
}

if [[ $# -ne 1 ]]; then
  usage >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RECIPE="$1"
CONFIG_FILE="${BUILDER_CONFIG:-$SCRIPT_DIR/builder.config.yaml}"
BUILDER_BIN="${BUILDER_BIN:-$SCRIPT_DIR/local/builder}"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Config file not found: $CONFIG_FILE" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required to build the local builder binary" >&2
  exit 1
fi

mkdir -p "$SCRIPT_DIR/local"

go build -o "$BUILDER_BIN" ./cmd/builder

exec "$BUILDER_BIN" --config "$CONFIG_FILE" build "$RECIPE"
