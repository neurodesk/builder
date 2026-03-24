#!/usr/bin/env python3
"""Scan recipe base images and test linux/arm64 pulls for non-Ubuntu images."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections import Counter, defaultdict
from dataclasses import dataclass
from pathlib import Path


BASE_IMAGE_RE = re.compile(r"^\s*base-image:\s*(.+?)\s*$")
TEMPLATE_NAME_RE = re.compile(r"^\s*name:\s*([A-Za-z0-9_.-]+)\s*$")
TEMPLATE_METHOD_RE = re.compile(r"^\s*method:\s*([A-Za-z0-9_.-]+)\s*$")


@dataclass
class RecipeInfo:
    recipe: str
    path: Path
    base_image: str
    template_usage: list[str]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default="builder.config.yaml")
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--output-markdown", default="")
    parser.add_argument("--output-json", default="")
    parser.add_argument("--keep-images", action="store_true")
    return parser.parse_args()


def load_recipe_roots(config_path: Path) -> list[Path]:
    roots: list[Path] = []
    in_recipe_roots = False
    for raw_line in config_path.read_text().splitlines():
        line = raw_line.rstrip()
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if re.match(r"^[A-Za-z0-9_-]+:", stripped):
            in_recipe_roots = stripped == "recipe_roots:"
            continue
        if in_recipe_roots and stripped.startswith("- "):
            roots.append(Path(stripped[2:].strip()))
    if not roots:
        raise SystemExit(f"no recipe_roots found in {config_path}")
    return roots


def parse_build_file(path: Path) -> RecipeInfo | None:
    base_image = ""
    template_usage: list[str] = []
    in_template = False
    template_name = ""

    for raw_line in path.read_text().splitlines():
        if not base_image:
            match = BASE_IMAGE_RE.match(raw_line)
            if match:
                base_image = match.group(1).strip().strip("'\"")

        stripped = raw_line.strip()
        if stripped == "template:":
            in_template = True
            template_name = ""
            continue
        if in_template:
            name_match = TEMPLATE_NAME_RE.match(raw_line)
            if name_match:
                template_name = name_match.group(1)
                continue
            method_match = TEMPLATE_METHOD_RE.match(raw_line)
            if method_match and template_name:
                template_usage.append(f"{template_name}/{method_match.group(1)}")
                in_template = False
                template_name = ""
                continue
            if stripped and not raw_line.startswith(" " * 8):
                in_template = False
                template_name = ""

    if not base_image:
        return None

    return RecipeInfo(
        recipe=path.parent.name,
        path=path,
        base_image=base_image,
        template_usage=sorted(set(template_usage)),
    )


def is_ubuntu_base(image: str) -> bool:
    lower = image.lower()
    return lower.startswith("ubuntu:") or lower == "ubuntu"


def pull_image(image: str, timeout: int, keep_images: bool) -> tuple[str, str]:
    if "{{" in image or "}}" in image:
        return "unresolved", "contains template placeholders"

    cmd = ["docker", "pull", "--platform", "linux/arm64", image]
    try:
        completed = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return "timeout", f"timed out after {timeout}s"

    output = (completed.stdout + "\n" + completed.stderr).strip()
    if completed.returncode == 0:
        if not keep_images:
            subprocess.run(
                ["docker", "image", "rm", image],
                capture_output=True,
                text=True,
                check=False,
            )
        return "ok", summarize_output(output)
    return "failed", summarize_output(output)


def summarize_output(output: str, max_lines: int = 4, max_chars: int = 500) -> str:
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    if not lines:
        return ""
    text = " | ".join(lines[:max_lines])
    if len(text) > max_chars:
        return text[: max_chars - 3] + "..."
    return text


def render_markdown(recipes: list[RecipeInfo], results: dict[str, tuple[str, str]]) -> str:
    non_ubuntu = [r for r in recipes if not is_ubuntu_base(r.base_image)]
    counts = Counter(recipe.base_image for recipe in non_ubuntu)

    lines: list[str] = []
    lines.append("## Arm64 Pull Check For Non-Ubuntu Base Images")
    lines.append("")
    lines.append("Statuses:")
    lines.append("")
    lines.append("- `ok`: `docker pull --platform linux/arm64` succeeded")
    lines.append("- `failed`: pull command returned non-zero")
    lines.append("- `timeout`: pull did not complete within the configured timeout")
    lines.append("- `unresolved`: image tag still contains recipe templating and cannot be pulled directly")
    lines.append("")
    lines.append("### Unique Base Image Results")
    lines.append("")
    lines.append("| Status | Count | Base Image | Detail |")
    lines.append("|---|---:|---|---|")
    for image, count in counts.most_common():
        status, detail = results[image]
        lines.append(f"| `{status}` | {count} | `{image}` | {detail.replace('|', '\\|')} |")
    lines.append("")
    lines.append("### Recipe-Level Mapping")
    lines.append("")
    lines.append("| Recipe | Base Image | Pull Status | Template Usage |")
    lines.append("|---|---|---|---|")
    for recipe in sorted(non_ubuntu, key=lambda item: item.recipe):
        status, _ = results[recipe.base_image]
        usage = ", ".join(recipe.template_usage) if recipe.template_usage else "-"
        lines.append(f"| `{recipe.recipe}` | `{recipe.base_image}` | `{status}` | {usage} |")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    args = parse_args()
    roots = load_recipe_roots(Path(args.config))
    recipes: list[RecipeInfo] = []
    for root in roots:
        for path in sorted(root.glob("*/build.yaml")):
            info = parse_build_file(path)
            if info is not None:
                recipes.append(info)

    non_ubuntu_images = sorted({r.base_image for r in recipes if not is_ubuntu_base(r.base_image)})
    results: dict[str, tuple[str, str]] = {}
    for image in non_ubuntu_images:
        print(f"checking {image}", file=sys.stderr)
        results[image] = pull_image(image, timeout=args.timeout, keep_images=args.keep_images)

    markdown = render_markdown(recipes, results)
    if args.output_markdown:
        Path(args.output_markdown).write_text(markdown)
    else:
        print(markdown)

    if args.output_json:
        Path(args.output_json).write_text(
            json.dumps(
                {
                    "results": {
                        image: {"status": status, "detail": detail}
                        for image, (status, detail) in results.items()
                    }
                },
                indent=2,
                sort_keys=True,
            )
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
