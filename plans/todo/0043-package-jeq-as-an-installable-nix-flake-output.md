---
id: TASK-0043
title: Package JEQ as an installable Nix flake output
status: doing
depends_on: []
priority: high
tags: [nix, packaging, release]
---

# Package JEQ as an installable Nix flake output

## Problem
The flake provides only a development shell, so Nix users cannot build, run, or install JEQ directly and packaged binaries do not prove the existing version contract.

## Desired outcome
A Nix user can build, run, or install JEQ directly from the repository flake. The packaged binary reports meaningful, reproducible build metadata.

## Context
The current flake already declares `aarch64-darwin`, `aarch64-linux`, `x86_64-darwin`, and `x86_64-linux`, but exposes only `devShells`. PXP's `buildGoModule` package and app outputs are a useful local reference. Keep JEQ's existing system matrix and version variables rather than copying PXP's hard-coded version.

## Acceptance criteria
- [ ] `packages.<system>.jeq` and `packages.<system>.default` build `cmd/jeq` with `buildGoModule` on all four declared systems.
- [ ] `apps.<system>.jeq` and `apps.<system>.default` run the packaged `jeq` binary.
- [ ] Go dependencies use a real pinned `vendorHash`; packaging does not fetch unpinned dependencies or use a fake hash.
- [ ] The package is CGO-free and injects deterministic values into `internal/cli.Version` and `internal/cli.Commit`; the packaged `jeq version` does not report `dev` or `unknown`.
- [ ] `nix build`, `nix run . -- version`, and `nix flake check` pass from a clean checkout on the host system.
- [ ] A focused check proves the packaged binary, not a PATH binary, reports the injected version and commit.
- [ ] README installation instructions include direct flake build, run, and profile-install examples without removing the source-build path.
- [ ] Existing development-shell behavior and normal verification gate remain unchanged.

## Constraints and non-goals
- Do not publish to nixpkgs or another registry in this task.
- Do not hard-code a release version that must be updated in two places.
- Do not add network calls to tests beyond normal pinned Nix dependency resolution.
- Do not change JEQ runtime behavior or supported CLI commands.

