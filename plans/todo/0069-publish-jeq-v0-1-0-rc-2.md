---
id: TASK-0069
title: Publish jeq v0.1.0-rc.2
status: todo
depends_on: [TASK-0064]
priority: high
tags: [release, prerelease, goreleaser, github-actions, qa]
---

# Publish jeq v0.1.0-rc.2

## Problem
Substantial CLI, streaming, provider, browser-example, and discovery changes have landed after v0.1.0-rc.1. Users need one verified immutable prerelease with current binaries and release notes, but main contains a known TASK-0064 flaky candidate lookup that must be fixed before tagging.

## Desired outcome
The exact verified main commit is tagged `v0.1.0-rc.2`, the tag-triggered workflow publishes all four supported archives plus checksums as a GitHub prerelease, and the release page explains user-facing changes and remaining limits.

## Acceptance criteria
- [ ] TASK-0064 is complete, including the known item-ID candidate lookup fix, explicit headed-run documentation, repeated offline stability evidence, and its required authorized paid acceptance run.
- [ ] Add `docs/releases/v0.1.0-rc.2.md` covering user-facing changes since rc.1, install paths, supported targets, compatibility, known limitations, and upgrade notes without dumping the commit log.
- [ ] The exact candidate commit passes the full watcher gate, `nix develop -c govulncheck ./...`, `go mod verify`, dependency/license review, and credential/artifact inspection.
- [ ] A clean GoReleaser snapshot passes `scripts/verify_release_snapshot.py`; inspect all four archives, README and LICENSE inclusion, checksums, and host binary version/commit metadata; remove `dist` afterward.
- [ ] Any required paid TypeSafe verification uses synthetic bounded inputs only after explicit authorization and records sanitized model, usage, and outcome evidence locally.
- [ ] Push the clean candidate commit to `origin/main`, create tag `v0.1.0-rc.2` at that exact commit, and push only that tag.
- [ ] The GitHub Release workflow succeeds for the tag and publishes a prerelease containing four target archives and one checksum file.
- [ ] Verify the published checksums and one host-compatible downloaded binary reports version `0.1.0-rc.2` and the tagged commit.
- [ ] Publish a final GO report naming the commit, workflow, release URL, assets, verification evidence, and residual limitations.

## Abort conditions
- Any unresolved deterministic or intermittent offline test failure.
- Candidate tree differs from the commit verified by the release checks.
- Secrets or raw provider payloads appear in tracked files, logs, or release assets.
- Workflow or artifact verification fails; do not move or recreate the immutable tag without an explicit recovery decision.

## Constraints
- Publish as a prerelease, not stable `v0.1.0`.
- Do not add signing, provenance, Windows, Homebrew, or nixpkgs work to this release.
- Never commit `.tmp`, `dist`, credentials, or paid raw responses.

