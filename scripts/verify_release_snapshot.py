#!/usr/bin/env python3
"""Verify the bounded, local GoReleaser snapshot contract."""
import hashlib
import platform
import subprocess
import sys
import tarfile
from pathlib import Path


def main(directory="dist"):
    root = Path(directory)
    archives = sorted(root.glob("jeq_*.tar.gz"))
    expected = {"jeq_0.0.1-SNAPSHOT-"}  # prefix keeps the check independent of commit
    if len(archives) != 4:
        raise ValueError(f"expected four target archives, found {len(archives)}")
    targets = set()
    for archive in archives:
        with tarfile.open(archive) as bundle:
            names = bundle.getnames()
            if "README.md" not in names or "jeq" not in names:
                raise ValueError(f"{archive.name} lacks jeq or README.md")
        targets.add("_".join(archive.name[:-7].split("_")[-2:]))
    if targets != {"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"}:
        raise ValueError(f"unexpected targets: {sorted(targets)}")
    checksums = root / "checksums.txt"
    lines = checksums.read_text().splitlines()
    if len(lines) != 4:
        raise ValueError("checksums.txt must contain exactly four archives")
    for line in lines:
        digest, name = line.split(maxsplit=1)
        if digest != hashlib.sha256((root / name).read_bytes()).hexdigest():
            raise ValueError(f"checksum mismatch: {name}")
    host = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
    arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine())
    if host and arch:
        archive = next(a for a in archives if f"_{host}_{arch}.tar.gz" in a.name)
        with tarfile.open(archive) as bundle:
            bundle.extract("jeq", root / ".verify")
        output = subprocess.check_output([str(root / ".verify" / "jeq"), "version"], text=True)
        if "SNAPSHOT" not in output or "Commit: " not in output:
            raise ValueError("host binary lacks snapshot metadata")


if __name__ == "__main__":
    try:
        main(sys.argv[1] if len(sys.argv) > 1 else "dist")
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"verify_release_snapshot.py: {error}", file=sys.stderr)
        raise SystemExit(1)
