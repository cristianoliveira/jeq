# Installation

## Linux installer

The installer supports Linux amd64 and arm64. It verifies the release archive
against the published checksums before installing to `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/cristianoliveira/jeq/main/scripts/install-jeq-linux.sh \
  -o /tmp/install-jeq-linux.sh
less /tmp/install-jeq-linux.sh
bash /tmp/install-jeq-linux.sh
```

Pass a release and destination directory when needed:

```sh
bash /tmp/install-jeq-linux.sh v0.1.0-rc.1 /usr/local/bin
```

## Homebrew

On macOS, install the release from the Homebrew tap:

```sh
brew tap cristianoliveira/tap
brew install cristianoliveira/tap/jeq
```

See the [provider guide](providers.md) before making a network request.

Verify the installation:

```sh
jeq version
```

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
rm -rf "$tmp_gobin"
```

Tagged `go install` builds use the source defaults (`dev` and `unknown`) for
metadata. Use the Nix package or release archives when release metadata matters.

For a checkout development install, use `nix develop -c go install ./cmd/jeq`.

Set `TYPESAFE_API_KEY` only in the environment that needs evaluation. Provider
selection may use `JEQ_PROVIDER=vercel` with `AI_GATEWAY_API_KEY`, or
`JEQ_PROVIDER=custom` with `JEQ_BASE_URL`, `JEQ_DEFAULT_MODEL`, and either
`JEQ_API_KEY` or `JEQ_AUTH=none` for loopback HTTP. JSON configuration is read
from `JEQ_CONFIG` when set; otherwise the optional
`$XDG_CONFIG_HOME/jeq/config.json` or `$HOME/.config/jeq/config.json`.
Credentials are environment names in config, never values. Do not put
credentials in prompts, state, shell history, or committed files.
