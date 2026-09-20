---
id: TASK-0040
title: Add short and long version flags
status: doing
depends_on: []
priority: normal
tags: [cli, discovery]
---

# Add short and long version flags

## Problem

JEQ exposes build information only through `jeq version`. Standard CLI probes
use `jeq --version` or `jeq -v`, so they currently fail even when JEQ is
installed and usable.

## Desired outcome

All three forms print the same build information:

```sh
jeq version
jeq --version
jeq -v
```

## Acceptance criteria

- [ ] `jeq --version` exits 0 and writes the same plain-text bytes as
      `jeq version` to stdout.
- [ ] `jeq -v` is an exact alias of `jeq --version`.
- [ ] Successful version probes write nothing to stderr and never require
      credentials, configuration, stdin, or network access.
- [ ] The root help lists `-v, --version` with a concise description.
- [ ] The existing `jeq version` command remains supported without output or
      exit-code changes.
- [ ] The implementation reuses the existing build-information rendering path
      rather than creating a second output format.
- [ ] Unit and built-binary black-box tests cover all three equivalent forms,
      injected version/commit values, stdout, stderr, and exit status.
- [ ] README discovery documentation shows the standard flag and retained
      subcommand.
- [ ] `nix develop -c make check` passes.

## Constraints

- Keep version output plain text and stable for shell probes.
- Keep version discovery offline and side-effect free.
- Do not repurpose `-v` for verbosity.

## Non-goals

- Changing build-time version or commit injection.
- Adding update checks or release downloads.
- Removing the `version` subcommand.
