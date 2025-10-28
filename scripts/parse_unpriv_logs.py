#!/usr/bin/env python3
"""Parse unprivileged build logs and summarize outcomes as JSON."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence


TIMESTAMP_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\s*")
TEST_MARKERS = ("✓", "✗", "⊝")
SECTION_PREFIXES = (
    "Test Results for",
    "Detailed Results:",
    "##[",
    "[command]",
    "Post job cleanup",
    "shell:",
    "env:",
    "Running test:",
    "Running builtin test:",
    "Using container runtime",
    "Found container",
)
FAILURE_KEYWORDS = ("error", "failed", "exception", "traceback", "abort", "denied")
LOG_ROOT = Path("local/unpriv_logs")
OUTPUT_PATH = Path("unpriv_build_summary.json")
MAX_OUTPUT_CHARS = 4000


def strip_timestamp(line: str) -> str:
    """Remove the leading GitHub Actions timestamp if present."""
    line = line.replace("\ufeff", "")
    return TIMESTAMP_RE.sub("", line, count=1).rstrip()


def clamp_text(text: str, limit: int = MAX_OUTPUT_CHARS) -> str:
    """Clamp large output blobs to a manageable length."""
    if len(text) <= limit:
        return text
    truncated = text[:limit].rstrip()
    remainder = len(text) - limit
    return f"{truncated}\n... (truncated, {remainder} more characters)"


def iter_log_files(root: Path) -> List[Path]:
    """Return a sorted list of log files to process."""
    if not root.exists():
        return []
    files: List[Path] = []
    for path in root.rglob("*.txt"):
        if path.name.lower() == "system.txt":
            continue
        if path.is_file():
            files.append(path)
    return sorted(files)


def extract_env_value(messages: Sequence[str], prefix: str) -> Optional[str]:
    target = f"{prefix}:"
    for msg in messages:
        if msg.startswith(target):
            return msg.split(":", 1)[1].strip()
    return None


def parse_test_blocks(messages: Sequence[str]) -> Dict[str, Any]:
    last_idx: Optional[int] = None
    for idx, raw_msg in enumerate(messages):
        if raw_msg.lstrip().startswith("Test Results for"):
            last_idx = idx
    if last_idx is None:
        return {}

    container_line = messages[last_idx].lstrip()
    container = container_line[len("Test Results for") :].strip().rstrip(":")
    summary: Dict[str, Any] = {}
    tests: List[Dict[str, Any]] = []
    failures: List[Dict[str, Any]] = []

    idx = last_idx + 1
    while idx < len(messages):
        raw_msg = messages[idx]
        msg = raw_msg.lstrip()
        if not msg:
            idx += 1
            continue
        if msg.startswith("Detailed Results:"):
            idx += 1
            break
        if ":" in msg:
            key, value = msg.split(":", 1)
            key = key.strip().lower()
            value = value.strip()
            try:
                summary[key] = int(value)
            except ValueError:
                summary[key] = value
        else:
            break
        idx += 1

    while idx < len(messages):
        raw_msg = messages[idx]
        msg = raw_msg.lstrip()
        if not msg:
            idx += 1
            continue
        if msg.startswith(SECTION_PREFIXES):
            break
        first = msg[0]
        if first in TEST_MARKERS:
            rest = msg[1:].strip()
            name = rest
            note = None
            if ": " in rest:
                name_part, status_part = rest.split(": ", 1)
                name = name_part.strip()
                note = status_part.strip()
            status_map = {"✓": "passed", "✗": "failed", "⊝": "skipped"}
            status = status_map.get(first, "unknown")
            record: Dict[str, Any] = {"name": name, "status": status}
            if note and note.lower() != status:
                record["note"] = note

            if status == "failed":
                output_lines: List[str] = []
                lookahead = idx + 1
                while lookahead < len(messages):
                    nxt_raw = messages[lookahead]
                    nxt = nxt_raw.lstrip()
                    if not nxt:
                        output_lines.append("")
                        lookahead += 1
                        continue
                    if nxt.startswith(SECTION_PREFIXES) or (nxt and nxt[0] in TEST_MARKERS):
                        break
                    output_lines.append(nxt)
                    lookahead += 1
                if output_lines:
                    record["output"] = clamp_text("\n".join(output_lines).strip())
                idx = lookahead - 1
                failures.append(record)
            tests.append(record)
        idx += 1

    result: Dict[str, Any] = {}
    if container:
        result["test_target"] = container
    if summary:
        result["test_summary"] = summary
    if tests:
        result["tests"] = tests
    if failures:
        result["failures"] = failures
    return result


def collect_failure_context(messages: Sequence[str], error_indices: Sequence[int]) -> Dict[str, str]:
    if not error_indices:
        return {}
    idx = error_indices[0]
    start = max(0, idx - 30)
    context = [
        msg
        for msg in messages[start:idx]
        if msg
        and not msg.startswith(("[command]", "Post job cleanup", "Temporary overriding", "Adding repository directory", "Cleaning up", "shell:", "env:"))
    ]
    if not context:
        return {}

    trimmed_context = context[-20:]
    reason = next(
        (line for line in reversed(trimmed_context) if any(keyword in line.lower() for keyword in FAILURE_KEYWORDS)),
        trimmed_context[-1],
    )
    output = clamp_text("\n".join(trimmed_context).strip())
    return {"reason": reason, "output": output}


def parse_log(path: Path) -> Dict[str, Any]:
    text = path.read_text(encoding="utf-8-sig", errors="replace")
    lines = text.splitlines()
    messages = [strip_timestamp(line) for line in lines]

    entry: Dict[str, Any] = {
        "name": path.stem,
        "path": str(path),
    }

    recipe = extract_env_value(messages, "RECIPE")
    if recipe:
        entry["recipe"] = recipe
    version = None
    for msg in messages:
        if msg.startswith("Detected version:"):
            version = msg.split(":", 1)[1].strip()
            break
    if not version:
        version = extract_env_value(messages, "VERSION")
    if version:
        entry["version"] = version

    error_indices = [idx for idx, msg in enumerate(messages) if msg.startswith("##[error]")]
    status = "failed" if error_indices else "succeeded"
    entry["status"] = status

    test_info = parse_test_blocks(messages)
    if test_info:
        entry.update(test_info)

    if status == "succeeded":
        summary = test_info.get("test_summary", {})
        if summary and summary.get("failed") == 0:
            total = summary.get("total")
            passed = summary.get("passed")
            if total is not None and passed is not None:
                entry["reason"] = f"All tests passed ({passed} of {total})."
            elif passed is not None:
                entry["reason"] = f"All tests passed ({passed})."
            else:
                entry["reason"] = "All tests passed."
        elif test_info.get("tests"):
            entry["reason"] = "All tests completed without failures."
        else:
            entry["reason"] = "Completed without reported failures."
    else:
        failures: List[Dict[str, Any]] = test_info.get("failures", []) if test_info else []
        if failures:
            entry["reason"] = "Tests failed: " + ", ".join(failure["name"] for failure in failures)
            outputs = [failure.get("output") for failure in failures if failure.get("output")]
            if outputs:
                entry["failure_output"] = clamp_text("\n\n".join(outputs))
        else:
            context = collect_failure_context(messages, error_indices)
            if context:
                entry["reason"] = context["reason"]
                entry["failure_output"] = context["output"]
            else:
                entry["reason"] = "Build failed for an unknown reason."

    return entry


def main() -> None:
    log_files = iter_log_files(LOG_ROOT)
    entries = [parse_log(path) for path in log_files]
    entries.sort(key=lambda item: item.get("name", ""))

    summary = {
        "log_directory": str(LOG_ROOT),
        "total_builds": len(entries),
        "summary": {
            "succeeded": sum(1 for item in entries if item.get("status") == "succeeded"),
            "failed": sum(1 for item in entries if item.get("status") == "failed"),
        },
        "entries": entries,
    }

    OUTPUT_PATH.write_text(json.dumps(summary, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
