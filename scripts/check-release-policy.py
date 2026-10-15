#!/usr/bin/env python3
"""Check a limited textual subset of the release workflow policy.

This is a structural regression check, not a full YAML or GitHub Actions
security parser. Native actionlint remains the syntax and expression check.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


SCANNER = "v1.9.0"
SCANNER_RE = re.compile(
    r"go run\s+golang\.org/x/vuln/cmd/govulncheck@(v[0-9][^\s]*)\s+\./\.\.\."
)


def job_blocks(text: str) -> dict[str, str]:
    lines = text.splitlines(keepends=True)
    jobs_index = next(
        (i for i, line in enumerate(lines) if line.strip() == "jobs:"), None
    )
    if jobs_index is None:
        raise ValueError("workflow has no jobs section")

    blocks: dict[str, str] = {}
    current: str | None = None
    body: list[str] = []
    for line in lines[jobs_index + 1 :]:
        if line and not line[0].isspace() and line.strip():
            break
        match = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", line)
        if match:
            if current is not None:
                blocks[current] = "".join(body)
            current = match.group(1)
            body = []
        elif current is not None:
            body.append(line)
    if current is not None:
        blocks[current] = "".join(body)
    return blocks


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def run_values(text: str) -> list[str]:
    """Return values from actual, uncommented single-line `run:` entries.

    Block scalars are intentionally excluded from required policy commands so
    a gate cannot hide failure handling inside a shell script.
    """
    lines = text.splitlines()
    values: list[str] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        if line.lstrip().startswith("#"):
            index += 1
            continue
        match = re.match(r"^(?P<indent>\s*)run:\s*(?P<value>.*)$", line)
        if match is None:
            index += 1
            continue
        value = match.group("value")
        if value in {"|", ">", "|-", ">-", "|+", ">+"}:
            base_indent = len(match.group("indent"))
            index += 1
            while index < len(lines):
                nested = lines[index]
                if nested.strip() and len(nested) - len(nested.lstrip()) <= base_indent:
                    break
                index += 1
            continue
        values.append(value)
        index += 1
    return values


def scanner_versions(text: str) -> list[str]:
    return SCANNER_RE.findall("\n".join(run_values(text)))


def check(release_path: Path, ci_path: Path) -> None:
    release = release_path.read_text()
    ci = ci_path.read_text()
    jobs = job_blocks(release)
    require("quality" in jobs, "release workflow must define quality job")
    require("publish" in jobs, "release workflow must define publish job")

    quality = jobs["quality"]
    publish = jobs["publish"]
    require(
        re.search(r"^defaults:", release, re.MULTILINE) is None,
        "release workflow must retain default shell semantics",
    )
    require(
        re.search(r"^\s*(?:defaults|shell):", quality, re.MULTILINE) is None,
        "quality job must retain default shell semantics",
    )
    require(
        re.search(r"^    needs:\s*(?:quality|\[quality\])\s*$", publish, re.MULTILINE)
        is not None,
        "publish job must need quality",
    )
    require(
        re.search(r"^\s*if:\s*", publish, re.MULTILINE) is None,
        "publish job must use default success semantics",
    )
    require(
        re.search(r"^\s*if:\s*", quality, re.MULTILINE) is None,
        "quality job must not override success semantics",
    )
    require(
        re.search(r"^\s*continue-on-error:\s*", quality, re.MULTILINE) is None,
        "quality job must not continue on error",
    )

    required_quality_commands = (
        "go test ./... -race -count=1",
        "go vet ./...",
        f"go run golang.org/x/vuln/cmd/govulncheck@{SCANNER} ./...",
    )
    quality_runs = run_values(quality)
    for command in required_quality_commands:
        require(
            command in quality_runs,
            f"quality job missing actual run command: {command}",
        )

    release_versions = scanner_versions(quality)
    ci_versions = scanner_versions(ci)
    require(release_versions == [SCANNER], "release scanner must use the pinned version")
    require(ci_versions == [SCANNER], "Go CI scanner must use the pinned version")

    workflow_permissions = re.search(
        r"(?m)^permissions:\s*\n((?:^ {2}\S[^\n]*\n)*)", release
    )
    require(workflow_permissions is not None, "release workflow needs read permissions")
    workflow_block = workflow_permissions.group(1)
    require("contents: read" in workflow_block, "workflow contents permission must be read")
    require("write" not in workflow_block, "workflow-level permissions must not write")

    require("write" not in quality, "quality job must remain read-only")
    require("contents: write" in publish, "publish job needs contents write")
    require("packages: write" in publish, "publish job needs packages write")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--release", type=Path)
    parser.add_argument("--ci", type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    release_path = args.release or root / ".github/workflows/release.yml"
    ci_path = args.ci or root / ".github/workflows/go-ci.yml"
    try:
        check(release_path, ci_path)
    except (OSError, ValueError) as error:
        print(f"release policy check failed: {error}")
        return 1
    print("release policy check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
