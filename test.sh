#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./test.sh <recipe>

Require an existing local Docker image for the recipe, convert it to
`sifs/<name>_<version>.simg` from the Docker daemon, and run the
NeuroContainers tester against it.
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
RECIPE_ROOT_DEFAULT="$SCRIPT_DIR/neurocontainers/recipes"
RESULTS_DIR="${TEST_RESULTS_DIR:-$SCRIPT_DIR/local/test-results}"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Config file not found: $CONFIG_FILE" >&2
  exit 1
fi

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

NAME="$(sed -n 's/^name:[[:space:]]*//p' "$BUILD_FILE" | head -n 1 | tr -d '"' | tr -d "'")"
VERSION="$(sed -n 's/^version:[[:space:]]*//p' "$BUILD_FILE" | head -n 1 | sed 's/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'")"

if [[ -z "$NAME" || -z "$VERSION" ]]; then
  echo "Unable to determine name/version from $BUILD_FILE" >&2
  exit 1
fi

mkdir -p "$SCRIPT_DIR/sifs" "$RESULTS_DIR"

IMAGE_TAG="${NAME}:${VERSION}"
SIMG_PATH="$SCRIPT_DIR/sifs/${NAME}_${VERSION}.simg"

if ! docker image inspect "$IMAGE_TAG" >/dev/null 2>&1; then
  echo "Docker image $IMAGE_TAG is not built. Run ./build.sh $RECIPE first." >&2
  exit 1
fi

"$APPTAINER_BIN" build --force "$SIMG_PATH" "docker-daemon://$IMAGE_TAG"

PYTHONPATH="$SCRIPT_DIR/neurocontainers${PYTHONPATH:+:$PYTHONPATH}" \
python3 - "$NAME" "$VERSION" "$RESULTS_DIR" <<'PY'
import json
import sys
from pathlib import Path

from workflows.test_runner import ContainerTestRunner, TestRequest

recipe = sys.argv[1]
version = sys.argv[2]
output_dir = Path(sys.argv[3])

runner = ContainerTestRunner(repo_root=Path.cwd() / "neurocontainers")
outcome = runner.run(
    TestRequest(
        recipe=recipe,
        version=version,
        runtime="apptainer",
        location="local",
        output_dir=output_dir,
        verbose=True,
    )
)

print(json.dumps(
    {
        "recipe": outcome.recipe,
        "version": outcome.version,
        "status": outcome.status,
        "results_path": str(outcome.results_path),
        "report_path": str(outcome.report_path) if outcome.report_path else None,
    },
    indent=2,
))

if outcome.status != "passed":
    raise SystemExit(1)
PY
