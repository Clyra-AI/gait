#!/usr/bin/env python3

from __future__ import annotations

import os
import subprocess
import sys


def write_packages(packages: list[str]) -> None:
    payload = ("\n".join(packages) + "\n").encode("utf-8")
    if hasattr(sys.stdout, "buffer"):
        sys.stdout.buffer.write(payload)
        return
    sys.stdout.write(payload.decode("utf-8"))


def main() -> int:
    go_bin = os.environ.get("GO", "go")
    result = subprocess.run(
        [go_bin, "list", "./..."],
        check=True,
        capture_output=True,
        text=True,
    )

    packages = [
        line.strip()
        for line in result.stdout.splitlines()
        if line.strip() and "/node_modules/" not in line
    ]
    if not packages:
        print("no Go packages matched after filtering", file=sys.stderr)
        return 1

    write_packages(packages)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
