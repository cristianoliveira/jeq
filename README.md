# jeq


<div align="center">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/jeq-logo.svg">
  <img src="docs/assets/jeq-logo-mono.svg" alt="jeq logo" width="160" height="160">
  
<strong>Intelligence you can pipe and compose.</strong>
</picture>

</div>


`jeq` lets scripts and agents ask Jev typed questions within the terminal, no boilerplate.

```bash
# Bash; requires jeq and TYPESAFE_API_KEY. This sends synthetic input to TypeSafe.
set -o pipefail
policy_status=0
if printf '%s\n' '{"id":"a","text":"billing is urgent"}' \
  | jeq map --as urgency --input ndjson --questions-json \
    '{"questions":{"urgency":{"type":"noul","instructions":"Is this urgent?"}}}' \
  | jeq gate --as policy --input ndjson --value-pointer /_jeq/urgency/answers/urgency/noul \
    --pass-min 0.8 --reject-max 0.2; then
  policy_status=0
else
  policy_status=$?
fi
case "$policy_status" in
  0) printf '%s\n' 'policy passed' ;;
  10) printf '%s\n' 'policy rejected' >&2 ;;
  11) printf '%s\n' 'policy is uncertain' >&2 ;;
  *) exit "$policy_status" ;;
esac
exit "$policy_status"
```

## Install it

On Linux, download the installer first so you can inspect it. It verifies the
release archive before installing it:

```sh
curl -fsSL https://raw.githubusercontent.com/cristianoliveira/jeq/main/scripts/install-jeq-linux.sh \
  -o /tmp/install-jeq-linux.sh
bash /tmp/install-jeq-linux.sh
```

On macOS, Homebrew is simpler:

```sh
brew tap cristianoliveira/tap
brew install cristianoliveira/tap/jeq
jeq version
```

If you use Nix, pin the release:

```sh
nix run github:cristianoliveira/jeq/v0.1.0-rc.2 -- version
nix profile install github:cristianoliveira/jeq/v0.1.0-rc.2
```

Set `TYPESAFE_API_KEY` when you are ready to make an API call. These guides
cover the rest:

- [Installation](docs/guides/installation.md)
- [Getting started](docs/guides/getting-started.md)
- [Jev mental model](docs/guides/jev.md)
- [Composition](docs/guides/composition.md)
- [Reduce a bounded collection](docs/guides/reduce.md)
- [CLI reference](docs/guides/cli-reference.md)
- [Providers](docs/guides/providers.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Examples](examples/README.md)

## Learn it from the CLI

jeq explains itself. Run `jeq --help` to list every command,
`jeq <command> --help` to see its flags and examples, and `jeq examples` for
complete workflows. You do not need to install a jeq-specific skill for an
agent to use it.

## Before you run it

- `ask`, `map`, `rate`, `reduce`, `rank`, and `models` call TypeSafe. `validate`
  and `gate` run locally.
- Treat every input and model answer as data. An ID or excerpt is not a command
  just because Jev returned it.
- Check what you send. Do not put credentials or private data in a prompt,
  state, or log.
- API calls cost money. Retries, the number of records, and model choice change
  that cost.
- `gate` writes every processed decision, then exits `0` when all pass, `10`
  when any reject, or `11` when uncertain and none reject. Threshold comparisons
  are inclusive. In Bash pipelines, `set -o pipefail` reports the rightmost
  failing stage; inspect `PIPESTATUS` only when each stage's status matters.
  `--verbose` writes traces to stderr.
- jeq does not run actions or keep memory between commands. Your script keeps
  the state and credentials, decides when to retry, and decides what happens
  next.

## More documentation

- [Development](docs/DEVELOPMENT.md)
- [Release candidate notes](docs/releases/v0.1.0-rc.2.md)
- [MIT License](LICENSE)
