---
id: TASK-0044
title: Publish tagged JEQ releases with GoReleaser
status: todo
depends_on: [TASK-0043]
priority: high
tags: [release, goreleaser, github-actions]
---

# Publish tagged JEQ releases with GoReleaser

## Problem
JEQ documents a manual cross-build loop but has no executable release contract, so archive targets, names, checksums, version metadata, and GitHub publication can drift or require error-prone manual work.

## Desired outcome
An explicitly approved Git tag produces reproducible JEQ archives, checksums, and one GitHub release through GoReleaser. Ordinary branches and pull requests can verify the release contract but cannot publish.

## Context
PXP's tag-triggered workflow is a useful reference for permissions, generated notes, prerelease handling, and explicit tag approval. JEQ should use GoReleaser as the single owner of the build matrix, archive names, checksums, and GitHub publication instead of copying PXP's hand-written archive loop.

## Acceptance criteria
- [ ] A GoReleaser v2 configuration builds `cmd/jeq` with `CGO_ENABLED=0` and `-trimpath` for linux and darwin on amd64 and arm64, matching JEQ's documented release targets.
- [ ] Release ldflags inject GoReleaser's version and commit into `internal/cli.Version` and `internal/cli.Commit`.
- [ ] Archives have deterministic names, include the binary and README, and GoReleaser emits one checksum file.
- [ ] Prerelease tags such as `v0.1.0-rc.1` are published as GitHub prereleases.
- [ ] GoReleaser is available from the pinned Nix development shell used locally and in CI.
- [ ] `goreleaser check` and a clean snapshot release build all configured artifacts without publishing.
- [ ] Snapshot verification checks archive presence, checksums, target coverage, and version metadata in a host-compatible extracted binary.
- [ ] A GitHub Actions workflow triggers publication only for pushed `v*` tags, checks out full history, runs the normal gate before release, and grants `contents: write` only where publication needs it.
- [ ] The workflow delegates builds and publication to GoReleaser; it does not maintain a second shell build matrix.
- [ ] Development documentation replaces the manual cross-build loop with snapshot verification and an explicit approved tag procedure.
- [ ] Automated tests and verification use no live TypeSafe requests, do not create or push tags, and do not publish releases.

## Constraints and non-goals
- Creating and pushing a release tag remains a human action.
- Windows artifacts, Homebrew, nixpkgs submission, signing, provenance attestations, and automatic changelog curation need separate product decisions.
- Do not claim a release succeeded without inspecting the produced targets, checksum file, and injected version metadata.

