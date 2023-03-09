#!/usr/bin/env python
"""
Check the version we are building matches the version from `GITHUB_REF` environment variable.
"""
import os
import re
import sys
from pathlib import Path

from packaging.version import Version


def main() -> int:
    version_ref = os.getenv("GITHUB_REF")
    if not version_ref:
        exit('✖ "GITHUB_REF" env variable not found')

    match = re.match(r"^refs/tags/(?P<app>[^/]+)/v(?P<ver>.*)$", version_ref)

    if not match or match["app"] not in {"buildkit", "provider"}:
        exit("✖ Tag doesn't match expected pattern")

    version = match["ver"]
    ver = Version(version)

    if match["app"] == "provider":
        filename = next((Path(__file__).parents[1] / "dist").glob("*.whl"))
        # Wheel file spec guarantees that this will be the version. So says TP Chung
        project_version = filename.name.split("-")[1]

        if project_version == version:
            print(f"✓ GITHUB_REF matches version {project_version!r}")
        else:
            exit(f"✖ GITHUB_REF version {version!r} does not match project version {project_version!r}")

    with open(os.environ["GITHUB_OUTPUT"], "a") as fh:
        ver = Version(version)
        is_prerelease = ver.is_prerelease or ver.is_devrelease
        print(f"is_prerelease={'true' if is_prerelease else 'false'}", file=fh)
        print(f"major_ver={ver.major}", file=fh)

    return 0


if __name__ == "__main__":
    sys.exit(main())
