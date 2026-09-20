#!/usr/bin/env python3
"""Verify the bounded, local GoReleaser snapshot contract."""
import hashlib
import json
import platform
import subprocess
import sys
import tarfile
from pathlib import Path


def main(directory="dist"):
    root = Path(directory)
    archives = sorted(root.glob("jeq_*.tar.gz"))
    if len(archives) != 4:
        raise ValueError(f"expected four target archives, found {len(archives)}")
    targets = set()
    for archive in archives:
        with tarfile.open(archive) as bundle:
            names = bundle.getnames()
            if "README.md" not in names or "LICENSE" not in names or "jeq" not in names:
                raise ValueError(f"{archive.name} lacks jeq or README.md")
        targets.add("_".join(archive.name[:-7].split("_")[-2:]))
    if targets != {"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"}:
        raise ValueError(f"unexpected targets: {sorted(targets)}")
    checksums = root / "checksums.txt"
    lines = checksums.read_text().splitlines()
    checksum_names = {line.split(maxsplit=1)[1] for line in lines}
    archive_names = {archive.name for archive in archives}
    if checksum_names != archive_names:
        raise ValueError("checksums.txt names must exactly match the four archives")
    for line in lines:
        digest, name = line.split(maxsplit=1)
        if Path(name).name != name or digest != hashlib.sha256((root / name).read_bytes()).hexdigest():
            raise ValueError(f"checksum mismatch: {name}")
    host = {"darwin": "darwin", "linux": "linux"}.get(platform.system().lower())
    arch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine().lower())
    if not host or not arch:
        raise ValueError(f"unsupported host: {platform.system()}/{platform.machine()}")
    metadata = json.loads((root / "metadata.json").read_text())
    version = metadata["version"]
    commit = metadata["commit"]
    archive = next(a for a in archives if f"_{host}_{arch}.tar.gz" in a.name)
    with tarfile.open(archive) as bundle:
        bundle.extract("jeq", root / ".verify")
    output = subprocess.check_output([str(root / ".verify" / "jeq"), "version"], text=True)
    expected = f"jeq {version}\nCommit: {commit}\n"
    if output != expected:
        raise ValueError(f"host binary metadata mismatch: {output!r}")


if __name__ == "__main__":
    try:
        main(sys.argv[1] if len(sys.argv) > 1 else "dist")
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"verify_release_snapshot.py: {error}", file=sys.stderr)
        raise SystemExit(1)
