# Providers

`jeq` selects one explicit provider for each command. It does not infer a provider from a credential or silently fall back to another endpoint.

## Direct TypeSafe

The default provider is `typesafe`. Set its credential in the environment:

```sh
export TYPESAFE_API_KEY='...'
jeq models
```

For a local loopback service, override the endpoint with `TYPESAFE_BASE_URL`. Remote endpoints must use `https`; `http` is accepted only for `localhost`, `127.0.0.1`, or `::1`.

## Vercel AI Gateway

Select the built-in Vercel profile explicitly:

```sh
export AI_GATEWAY_API_KEY='...'
jeq --provider vercel models
```

`VERCEL_OIDC_TOKEN` is accepted when `AI_GATEWAY_API_KEY` is absent. The profile uses the TypeSafe gateway path and its default model. Check the endpoint and model with `jeq models` before a composed command.

## One-off custom provider

Use environment settings for one command without writing a credential to disk:

```sh
JEQ_PROVIDER=custom JEQ_BASE_URL=https://example.invalid/typesafe \
JEQ_API_KEY="$MY_PROVIDER_KEY" jeq models
```

Set `JEQ_AUTH=none` only for a loopback HTTP service. A remote unauthenticated endpoint is rejected. HTTPS is required for every remote provider. Redirects are refused by the HTTP adapter.

## Named profiles

Keep endpoint metadata and credential names in a JSON config. Store credential values only in the environment:

```json
{
  "default_provider": "staging",
  "providers": {
    "staging": {
      "base_url": "https://api.example.invalid/typesafe",
      "default_model": "jev-latest",
      "auth": "bearer",
      "api_key_env": "STAGING_TYPESAFE_API_KEY"
    }
  }
}
```

Select it with `JEQ_CONFIG=/path/to/config.json JEQ_PROVIDER=staging`. The default config is `$XDG_CONFIG_HOME/jeq/config.json`, or `$HOME/.config/jeq/config.json`. `JEQ_CONFIG` takes precedence over that optional default file. Unknown fields, malformed JSON, unsupported auth, credentials embedded in URLs, and invalid schemes are rejected.

## Model precedence

For composed commands, model selection is:

1. `--model`
2. `JEQ_DEFAULT_MODEL`
3. the selected provider profile's `default_model`
4. legacy `TYPESAFE_DEFAULT_MODEL` or the top-level `default_model` in config
5. the provider default

A native `ask` request's embedded model is authoritative. Provider selection and model selection are separate decisions.

## Verify and troubleshoot

Run `jeq models` after selecting a provider. It verifies the endpoint, authentication, and model listing without changing config. Common errors:

- `JEQ_AUTH_MISSING`: export the environment variable named by `api_key_env`.
- invalid provider or scheme: use an explicit provider and HTTPS for remote endpoints.
- unknown config field or oversized config: simplify the JSON and keep it below the config size limit.
- model rejected: check the provider's `models` output, then pass an exact `--model`.

Do not put API keys in config files, command arguments, examples, logs, or trace output.
