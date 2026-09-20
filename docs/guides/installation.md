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
  [GitHub release](https://github.com/cristianoliveira/jeq/releases) and verify
  `checksums.txt` before extracting.

Moving-main profile installs can be upgraded with `nix profile upgrade jeq`.
A pinned release does not advance: review a new tag, remove the old profile
entry with `nix profile remove jeq`, then install the new pinned reference.
Verify a tagged Go install without changing your normal binary:

```sh
tmp_gobin=$(mktemp -d)
GOBIN="$tmp_gobin" go install github.com/cristianoliveira/jeq/cmd/jeq@v0.1.0-rc.1
"$tmp_gobin/jeq" version
rm -rf "$tmp_gobin" # tagged Go installs use source defaults: dev/unknown metadata
```

For a checkout development install, use `nix develop -c go install ./cmd/jeq`.

Set `TYPESAFE_API_KEY` only in the environment that needs evaluation. Do not put
credentials in prompts, state, shell history, or committed files.
