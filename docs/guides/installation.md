# Installation

## Pinned release candidate

Use the verified remote flake first:

```sh
nix run github:cristianoliveira/jeq/v0.1.0-rc.1 -- version
nix profile install github:cristianoliveira/jeq/v0.1.0-rc.1
```

This uses the tagged release candidate. Replace the tag only after reviewing a
new release.

## Other choices

- Moving main: `nix run github:cristianoliveira/jeq -- version` follows the
  repository default branch and can change.
- Source checkout: `nix run . -- version` or `nix build .#jeq` from a checkout.
- Go install: `go install github.com/cristianoliveira/jeq/cmd/jeq@v0.1.0-rc.1`.
- Archives: download the matching Linux or macOS amd64/arm64 archive from the
  GitHub release and verify `checksums.txt` before extracting.

Set `TYPESAFE_API_KEY` only in the environment that needs evaluation. Do not put
credentials in prompts, state, shell history, or committed files.
