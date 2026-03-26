#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./test.sh [recipe]

Without arguments, run the niimath full test suite as a smoke test.

With a recipe argument, require an existing local Docker image for the
recipe, convert it to `sifs/<name>_<version>.simg` from the Docker daemon,
and run the recipe's `fulltest.yaml` locally against that `.simg`.
EOF
}

if [[ $# -gt 1 ]]; then
  usage >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

CONFIG_FILE="${BUILDER_CONFIG:-$SCRIPT_DIR/builder.config.yaml}"
RECIPE_ROOT_DEFAULT="$SCRIPT_DIR/neurocontainers/recipes"
RESULTS_DIR="${TEST_RESULTS_DIR:-$SCRIPT_DIR/local/test-results}"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Config file not found: $CONFIG_FILE" >&2
  exit 1
fi

RECIPE="${1:-niimath}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required to read the built image from the Docker daemon" >&2
  exit 1
fi

APPTAINER_BIN="$(command -v apptainer || command -v singularity || true)"
if [[ -z "$APPTAINER_BIN" ]]; then
  echo "apptainer or singularity is required to build a .simg file" >&2
  exit 1
fi

if [[ "$RECIPE" == */* || "$RECIPE" == .* ]]; then
  RECIPE_DIR="$RECIPE"
else
  RECIPE_ROOT="$(
    awk '
      $1 == "recipe_roots:" { in_roots = 1; next }
      in_roots && $1 == "-" { print $2; exit }
      in_roots && $1 !~ /^-/ { in_roots = 0 }
    ' "$CONFIG_FILE"
  )"
  if [[ -z "$RECIPE_ROOT" ]]; then
    RECIPE_ROOT="$RECIPE_ROOT_DEFAULT"
  fi
  if [[ "$RECIPE_ROOT" != /* ]]; then
    RECIPE_ROOT="$SCRIPT_DIR/$RECIPE_ROOT"
  fi
  RECIPE_DIR="$RECIPE_ROOT/$RECIPE"
fi

BUILD_FILE="$RECIPE_DIR/build.yaml"
if [[ ! -f "$BUILD_FILE" ]]; then
  echo "Recipe build file not found: $BUILD_FILE" >&2
  exit 1
fi

FULLTEST_FILE="$RECIPE_DIR/fulltest.yaml"
if [[ ! -f "$FULLTEST_FILE" ]]; then
  echo "Recipe full test file not found: $FULLTEST_FILE" >&2
  exit 1
fi

NAME="$(sed -n 's/^name:[[:space:]]*//p' "$BUILD_FILE" | head -n 1 | tr -d '"' | tr -d "'")"
VERSION="$(sed -n 's/^version:[[:space:]]*//p' "$BUILD_FILE" | head -n 1 | sed 's/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'")"

if [[ -z "$NAME" || -z "$VERSION" ]]; then
  echo "Unable to determine name/version from $BUILD_FILE" >&2
  exit 1
fi

FULLTEST_PATH_ARG="recipes/${NAME}/fulltest.yaml"

mkdir -p "$SCRIPT_DIR/sifs" "$RESULTS_DIR"

IMAGE_TAG="${NAME}:${VERSION}"
SIMG_PATH="$SCRIPT_DIR/sifs/${NAME}_${VERSION}.simg"
RESULTS_JSON="$RESULTS_DIR/${NAME}-fulltest.json"
RESULTS_LOG="$RESULTS_DIR/${NAME}-fulltest.log"

if ! docker image inspect "$IMAGE_TAG" >/dev/null 2>&1; then
  echo "Docker image $IMAGE_TAG is not built. Run ./build.sh $RECIPE first." >&2
  exit 1
fi

"$APPTAINER_BIN" build --force "$SIMG_PATH" "docker-daemon://$IMAGE_TAG"
if command -v uv >/dev/null 2>&1; then
  RUNNER=(uv run "$SCRIPT_DIR/neurocontainers/builder/run_tests.py")
else
  RUNNER=(python3 "$SCRIPT_DIR/neurocontainers/builder/run_tests.py")
fi

cd "$SCRIPT_DIR/neurocontainers"
exec "${RUNNER[@]}" \
  "$FULLTEST_PATH_ARG" \
  -c "$SCRIPT_DIR/sifs" \
  -o "$RESULTS_JSON" \
  --log "$RESULTS_LOG"
