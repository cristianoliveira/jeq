---
id: TASK-0069
title: Publish jeq v0.1.0-rc.2
status: done
depends_on: [TASK-0064]
priority: high
tags: [release, prerelease, goreleaser, github-actions, qa]
---

# Publish jeq v0.1.0-rc.2

## Problem
Substantial CLI, streaming, provider, browser-example, and discovery changes landed after v0.1.0-rc.1. Users needed one verified immutable prerelease with current binaries and practical release notes. TASK-0064 had to pass its offline and paid headed gates before tagging.

## Desired outcome
The exact verified main commit is tagged `v0.1.0-rc.2`, the tag-triggered workflow publishes all four supported archives plus checksums as a GitHub prerelease, and the release page explains user-facing changes and remaining limits.

## Acceptance criteria
- [x] TASK-0064 is complete, including the known item-ID candidate lookup fix, explicit headed-run documentation, repeated offline stability evidence, and its required authorized paid acceptance run.
- [x] Add `docs/releases/v0.1.0-rc.2.md` covering user-facing changes since rc.1, install paths, supported targets, compatibility, known limitations, and upgrade notes without dumping the commit log.
- [x] The exact candidate commit passes the full watcher gate, `nix develop -c govulncheck ./...`, `go mod verify`, dependency/license review, and credential/artifact inspection.
- [x] A clean GoReleaser snapshot passes `scripts/verify_release_snapshot.py`; inspect all four archives, README and LICENSE inclusion, checksums, and host binary version/commit metadata; remove `dist` afterward.
- [x] Any required paid TypeSafe verification uses synthetic bounded inputs only after explicit authorization and records sanitized model, usage, and outcome evidence locally.
- [x] Push the clean candidate commit to `origin/main`, create tag `v0.1.0-rc.2` at that exact commit, and push only that tag.
- [x] The GitHub Release workflow succeeds for the tag and publishes a prerelease containing four target archives and one checksum file.
- [x] Verify the published checksums and one host-compatible downloaded binary reports version `0.1.0-rc.2` and the tagged commit.
- [x] Publish a final GO report naming the commit, workflow, release URL, assets, verification evidence, and residual limitations.

## Completion evidence
- Tagged commit: `a02f974a26b32fed9b4526df2d9804d1b479fe82`.
- Workflow: `https://github.com/cristianoliveira/jeq/actions/runs/35851235490`, success.
- Release: `https://github.com/cristianoliveira/jeq/releases/tag/v0.1.0-rc.2`, published prerelease.
- Published assets: four target archives plus `checksums.txt`; every checksum passed.
- Downloaded macOS arm64 binary: `jeq 0.1.0-rc.2`, commit `a02f974a26b32fed9b4526df2d9804d1b479fe82`.
- Local report: `.tmp/reports/23-09-26/jeq-v0.1.0-rc.2-release.md`.

## Abort conditions
- Any unresolved deterministic or intermittent offline test failure.
- Candidate tree differs from the commit verified by the release checks.
- Secrets or raw provider payloads appear in tracked files, logs, or release assets.
- Workflow or artifact verification fails; do not move or recreate the immutable tag without an explicit recovery decision.

## Constraints
- Publish as a prerelease, not stable `v0.1.0`.
- Do not add signing, provenance, Windows, Homebrew, or nixpkgs work to this release.
- Never commit `.tmp`, `dist`, credentials, or paid raw responses.

