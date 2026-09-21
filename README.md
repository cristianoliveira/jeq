# JEQ


<div align="center">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/jeq-logo.svg">
  <img src="docs/assets/jeq-logo-mono.svg" alt="JEQ logo" width="160" height="160">
  
<strong>Intelligence you can pipe.</strong>
</picture>

</div>


JEQ is a typed judgment CLI for scripts, agents, and systems. It turns a question
and JSON state into evidence that can move through a Unix pipeline.

```sh
printf '%s\n' '{"id":"a","text":"billing is urgent"}' \
  | jeq map --as urgency --input ndjson --questions-json \
    '{"questions":{"urgency":{"type":"noul","instructions":"Is this urgent?"}}}' \
  | jeq gate --as policy --input ndjson --value-pointer /_jeq/urgency/answers/urgency/noul \
    --pass-min 0.8 --reject-max 0.2
```

## Quick start

On Linux, install the verified release archive with the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/cristianoliveira/jeq/main/scripts/install-jeq-linux.sh \
  -o /tmp/install-jeq-linux.sh
bash /tmp/install-jeq-linux.sh
```

On macOS, install through the Homebrew tap:

```sh
brew tap cristianoliveira/tap
brew install cristianoliveira/tap/jeq
jeq version
```

Alternatively, install from the pinned release candidate with Nix:

```sh
nix run github:cristianoliveira/jeq/v0.1.0-rc.1 -- version
nix profile install github:cristianoliveira/jeq/v0.1.0-rc.1
```

Then set `TYPESAFE_API_KEY` and run an explicit judgment. Start with the focused
guides:

- [Installation](docs/guides/installation.md)
- [Getting started](docs/guides/getting-started.md)
- [Composition](docs/guides/composition.md)
- [Reduce a bounded collection](docs/guides/reduce.md)
- [CLI reference](docs/guides/cli-reference.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Examples](examples/README.md)

## Boundaries

- Evaluation commands make network requests. `validate` validates locally.
- Input is untrusted data. Do not treat model output, IDs, or excerpts as commands.
- Review prompts, state, and output before sending. Do not send credentials or
  private data unintentionally.
- API usage costs money. Retries, batch size, and model choice affect cost.
- Output and exit codes are designed for pipelines; policy `reject` and
  `uncertain` are non-zero. Verbose traces go to stderr.
- JEQ does not own actions, memory, sessions, or release tags. The caller owns
  state, credentials, retries, and automation policy.

## More documentation

- [Development](docs/DEVELOPMENT.md)
- [Release candidate notes](docs/releases/v0.1.0-rc.1.md)
- [MIT License](LICENSE)
