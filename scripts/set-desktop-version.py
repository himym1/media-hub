#!/usr/bin/env python3
"""Write the desktop package version so the Tauri binary matches the release tag."""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def require_semver(value: str) -> str:
    if not re.fullmatch(r"\d+\.\d+\.\d+", value):
        raise SystemExit(f"invalid desktop version: {value}")
    return value


def replace_once(path: Path, pattern: str, replacement: str) -> None:
    text = path.read_text()
    updated, count = re.subn(pattern, replacement, text, count=1)
    if count != 1:
        raise SystemExit(f"{path}: expected one version field, found {count}")
    path.write_text(updated)


def main() -> None:
    version = require_semver(sys.argv[1] if len(sys.argv) == 2 else "")
    replace_once(
        ROOT / "desktop/src-tauri/Cargo.toml",
        r'(?m)^version = "[^"]+"',
        f'version = "{version}"',
    )
    replace_once(
        ROOT / "desktop/src-tauri/Cargo.lock",
        r'(name = "media-hub-desktop"\nversion = ")[^"]+(")',
        rf"\g<1>{version}\g<2>",
    )
    replace_once(
        ROOT / "desktop/src-tauri/tauri.conf.json",
        r'("version"\s*:\s*")[^"]+(")',
        rf"\g<1>{version}\g<2>",
    )
    replace_once(
        ROOT / "desktop/package.json",
        r'("version"\s*:\s*")[^"]+(")',
        rf"\g<1>{version}\g<2>",
    )


if __name__ == "__main__":
    main()
