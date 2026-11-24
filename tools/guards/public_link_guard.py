#!/usr/bin/env python3
"""Reject private document links in public schemas, SDKs, and documentation."""

from __future__ import annotations

import pathlib
import re
import sys


ROOT = pathlib.Path(__file__).resolve().parents[2]
ASSET_DIRS = (
    "api",
    "docs",
    "examples",
    "expanded_schemas",
    "generated/java/src",
    "internal/umodel/schemaspec/data",
    "schemas",
    "sdk",
    "skills",
)
TEXT_SUFFIXES = {".md", ".mdx", ".rst", ".html", ".yaml", ".yml", ".json", ".go", ".py", ".java", ".xml", ".txt"}
IGNORED_PARTS = {".git", ".venv", "__pycache__", "node_modules", ".cache", "target", "dist"}
PRIVATE_DOCUMENT_LINK = re.compile(
    r"(?:https?:)?//(?:[a-z0-9-]+\.)*alidocs\.dingtalk\.com(?=[:/?#\s<>'\")\]]|$)",
    re.IGNORECASE,
)


def iter_files(root: pathlib.Path) -> list[pathlib.Path]:
    files = set(root.glob("*.md"))
    for directory in ASSET_DIRS:
        files.update((root / directory).rglob("*"))
    # Tool documentation is public; guard fixtures and sanitization code are not
    # schema or documentation output and may deliberately contain test URLs.
    files.update((root / "tools").rglob("*.md"))
    return sorted(
        path for path in files
        if path.is_file() and not path.is_symlink()
        and path.suffix in TEXT_SUFFIXES
        and not IGNORED_PARTS.intersection(path.relative_to(root).parts)
    )


def main(root: pathlib.Path = ROOT) -> int:
    errors: list[str] = []
    for path in iter_files(root):
        for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if PRIVATE_DOCUMENT_LINK.search(line):
                errors.append(f"{path.relative_to(root).as_posix()}:{line_number}")
    if errors:
        print("Public link guard failed: private document links found:")
        for error in errors:
            print(f"  - {error}")
        print("Remove private document links from source files and regenerate affected assets.")
        return 1
    print("Public link guard passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
