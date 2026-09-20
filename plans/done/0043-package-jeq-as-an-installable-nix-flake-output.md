---
id: TASK-0043
title: Package JEQ as an installable Nix flake output
status: done
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
The current flake declares `aarch64-darwin`, `aarch64-linux`, `x86_64-darwin`, and `x86_64-linux`, but exposes only `devShells`. PXP's `buildGoModule` package and app outputs are a useful local reference. Do not copy PXP's hard-coded version.

Pinned nixpkgs 26.11 explicitly dropped `x86_64-darwin`; `nix flake check --all-systems` cannot evaluate that output. JEQ's Nix system list must therefore match current nixpkgs support. GoReleaser may still cross-build a Darwin amd64 archive because that uses the Go toolchain rather than nixpkgs packages.

## Acceptance criteria
- [ ] `packages.<system>.jeq` and `packages.<system>.default` build `cmd/jeq` with `buildGoModule` on the three systems supported by pinned nixpkgs: `aarch64-darwin`, `aarch64-linux`, and `x86_64-linux`.
- [ ] `apps.<system>.jeq` and `apps.<system>.default` run the packaged `jeq` binary on the same supported systems.
- [ ] Go dependencies use a real pinned `vendorHash`; packaging does not fetch unpinned dependencies or use a fake hash.
- [ ] The package is CGO-free and injects deterministic values into `internal/cli.Version` and `internal/cli.Commit`; the packaged `jeq version` does not report `dev` or `unknown`.
- [ ] `nix build`, `nix run . -- version`, `nix flake check`, and evaluation with `nix flake check --all-systems --no-build` pass from a clean checkout.
- [ ] A focused check proves the packaged binary, not a PATH binary, reports the injected version and commit.
- [ ] README installation instructions include direct flake build, run, and profile-install examples without removing the source-build path.
- [ ] Existing development-shell behavior and normal verification gate remain unchanged.

## Constraints and non-goals
- Do not publish to nixpkgs or another registry in this task.
- Do not hard-code a release version that must be updated in two places.
- Do not add network calls to tests beyond normal pinned Nix dependency resolution.
- Do not change JEQ runtime behavior or supported CLI commands.

