---
id: TASK-0045
title: Prepare the first JEQ release candidate
status: doing
depends_on: []
priority: high
tags: [release, qa, documentation, security]
---

# Prepare the first JEQ release candidate

## Problem
JEQ has no published tags and now has automated packaging, but the first release still needs an explicit version and license decision, exact-commit release evidence, useful release notes, and a final no-publish readiness verdict.

## Desired outcome
A named, immutable commit is demonstrably ready for JEQ's first public prerelease or release. The owner receives a concise release summary and exact tag command, while tag creation and publication remain separate explicit actions.

## Context
There are no existing Git tags or GitHub releases. `main` matches `origin/main`. GoReleaser and the tag-only workflow are ready. The public repository currently has no license; GitHub reports `licenseInfo: null`. The last paid live verification predates substantial command and flake changes, so it cannot certify this candidate.

Default recommendation: prepare `v0.1.0-rc.1` because this is the first public artifact and the distribution path has not yet been exercised on GitHub. The owner must confirm the version and project license before tagging.

## Acceptance criteria
- [ ] Owner explicitly selects the release tag and project license; the repository license is clear and any required archive/package metadata is updated before the candidate is frozen.
- [ ] Release notes describe user-facing capabilities, install paths, supported artifact targets, known limitations, and upgrade compatibility without dumping the commit log.
- [ ] `nix develop -c make check`, `nix develop -c govulncheck ./...`, `go mod verify`, dependency/license review, and secret/artifact inspection pass at the exact candidate commit.
- [ ] A clean GoReleaser snapshot produces and verifies all four configured archives, README inclusion, checksums, and exact host binary version/commit; generated artifacts are removed afterward.
- [ ] The compiled candidate passes explicit paid live TypeSafe verification after the final code/config change, including models, happy-path ask, offline validate, invalid-key/redaction, resolved model, and bounded usage evidence.
- [ ] Live evidence uses synthetic inputs, records sanitized facts and approximate cost locally, and never commits credentials, raw provider payloads, or `.tmp` artifacts.
- [ ] Candidate commit is pushed to `origin/main`, working tree is clean, and no board dependency is blocked.
- [ ] Final readiness report names the exact commit/tag, commands and results, supported targets, checksums, residual limitations, rollback/abort condition, and a GO/NO-GO verdict.
- [ ] Preparation does not create or push a tag, GitHub release, or other publication; publishing requires a separate explicit owner instruction.

## Constraints and non-goals
- Do not infer or choose a legal license for the owner.
- Do not use old live evidence as proof for a changed candidate.
- Do not add Windows, package registries, signing, or provenance during release preparation.
- Do not change product behavior merely to make the release checklist pass; open a separate task for discovered defects.

