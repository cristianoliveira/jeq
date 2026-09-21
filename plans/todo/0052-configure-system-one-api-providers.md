---
id: TASK-0052
title: Configure System One API providers
status: doing
depends_on: []
priority: high
tags: [cli, configuration, providers, security, typesafe, vercel]
---

# Configure System One API providers

## Problem
jeq binds every network command to TypeSafe-specific environment names and one global bearer credential, so users cannot select Vercel's TypeSafe-compatible AI Gateway endpoint or safely target another compatible online or local server without misleading overrides and inflexible authentication.

## Desired outcome
A caller explicitly selects one System One-compatible provider profile. jeq then resolves one endpoint, one credential source, and one model without discovery or fallback. Existing TypeSafe users continue to work unchanged. Vercel AI Gateway and compatible local or online servers become first-class configuration choices.

## Confirmed compatibility
- TypeSafe documents Vercel AI Gateway as a drop-in TypeSafe-compatible endpoint at `https://ai-gateway.vercel.sh/typesafe`.
- Vercel uses the same `POST /v1/systemone` and `GET /v1/models` document shapes already implemented by jeq.
- Vercel's model identifier is `typesafe-ai/jev`; credentials come from `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN`.
- Therefore, do not introduce another domain contract or provider adapter. Generalize construction of the existing HTTP adapter around one resolved connection profile.

## Proposed configuration contract
Built-in profile names are `typesafe` and `vercel`. `typesafe` remains the default. User-defined names under `providers` are System One-compatible profiles and cannot shadow built-ins.

```json
{
  "default_provider": "vercel",
  "providers": {
    "local": {
      "base_url": "http://127.0.0.1:8787",
      "default_model": "jev-local",
      "auth": "none"
    },
    "hosted": {
      "base_url": "https://jev.example.com/api",
      "default_model": "typesafe-ai/jev",
      "auth": "bearer",
      "api_key_env": "HOSTED_JEV_API_KEY"
    }
  }
}
```

Keep the existing top-level `default_model` valid for backward compatibility. Reject unknown fields, duplicate keys, blank names, built-in name overrides, malformed environment-variable names, and unsupported auth modes. Config stores only the name of a credential environment variable, never its value.

### Deterministic selection
1. Provider: `JEQ_PROVIDER` > `default_provider` in config > `typesafe`.
2. Model for composed commands: `--model` > `JEQ_DEFAULT_MODEL` > selected profile `default_model` > legacy top-level/config or `TYPESAFE_DEFAULT_MODEL` compatibility > selected built-in default.
3. Native `ask` keeps its embedded model authoritative.
4. Built-in TypeSafe connection: `https://api.typesafe.ai`, `TYPESAFE_API_KEY`, default `jev-latest`; preserve `TYPESAFE_BASE_URL` as a legacy override.
5. Built-in Vercel connection: `https://ai-gateway.vercel.sh/typesafe`; credential precedence is `AI_GATEWAY_API_KEY` then `VERCEL_OIDC_TOKEN`; default model is `typesafe-ai/jev`.
6. Environment-only custom connection: `JEQ_PROVIDER=custom`, `JEQ_BASE_URL`, `JEQ_DEFAULT_MODEL`, and either `JEQ_API_KEY` or explicit `JEQ_AUTH=none`.

The implementation must document and test the final ordering precisely. It must never infer a provider from present credentials, silently switch providers, or retry against a second provider.

## Acceptance criteria

### Resolution and compatibility
- [ ] Existing TypeSafe invocations using only `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`, and `TYPESAFE_DEFAULT_MODEL` keep their current behavior and error classes.
- [ ] `JEQ_PROVIDER=vercel` and JSON `default_provider: "vercel"` select the Vercel base URL, Vercel credential source, and `typesafe-ai/jev` default.
- [ ] Named JSON profiles and the environment-only `custom` profile resolve one compatible endpoint, authentication mode, and default model.
- [ ] `ask`, `map`, `reduce`, `rank`, `rate`, and `models` all use the same resolved profile; offline commands remain network- and credential-independent.
- [ ] `models` requests `<selected-base>/v1/models`; evaluation requests `<selected-base>/v1/systemone`.
- [ ] A native `ask` request's model is not rewritten. Every composed command uses the documented model precedence.
- [ ] Explicit config is strictly validated before credential lookup or HTTP-client construction. An absent optional default config remains harmless.
- [ ] Missing provider, model, endpoint, or credential errors name the selected setting and give actionable recovery without exposing secret values.

### Security and failure behavior
- [ ] Remote custom endpoints require HTTPS. Plain HTTP and unauthenticated mode are accepted only for loopback hosts (`localhost`, loopback IPv4, or loopback IPv6).
- [ ] Endpoint validation rejects userinfo, query strings, fragments, missing hosts, and unsupported schemes while permitting a clean base path.
- [ ] HTTP redirects fail closed so bearer credentials cannot be forwarded to another origin.
- [ ] `auth: "bearer"` requires a nonblank value from the configured environment variable. `auth: "none"` sends no `Authorization` header.
- [ ] Existing body limits, timeout bounds, retry bounds, and retryable statuses remain in force. Retries never change provider or endpoint.
- [ ] stdout remains only semantic output. Diagnostics and `jeq.trace.v1` never contain credentials, authorization headers, request bodies, or base URLs. Verbose trace may identify only the provider class (`typesafe`, `vercel`, or `custom`).

### Test evidence
- [ ] Table-driven resolver tests cover provider/config/environment/model precedence, unknown providers, malformed profiles, missing credentials, and legacy behavior.
- [ ] `httptest` contract tests prove exact Vercel/custom paths, headers, model values, response preservation, `/v1/models`, unauthenticated loopback behavior, redirect refusal, and no cross-provider fallback.
- [ ] Happy and unhappy paths are covered without paid or live API calls; any live smoke check remains a separate explicit manual action.
- [ ] Full repository verification and security scan pass through the configured watcher/final gate.

### Documentation
- [ ] CLI reference and architecture docs describe provider selection, all supported environment variables, JSON examples, precedence, security restrictions, and migration from TypeSafe-only configuration.
- [ ] Help and examples call Vercel a compatible System One route rather than a different response contract.
- [ ] AGENTS.md guidance is updated so future changes place shared System One wire behavior in the existing infrastructure adapter and provider selection in CLI/composition configuration.

## Non-goals
- Provider auto-discovery from credentials or endpoint shape.
- Automatic fallback, routing, load balancing, or cross-provider retries.
- A plugin/registry system or separate adapter for each provider that speaks the same contract.
- Model-name translation or capability negotiation.
- Storing secrets, operational state, usage budgets, or provider health in jeq.
- Supporting provider-specific APIs that are not TypeSafe System One wire-compatible.

## Implementation sequence
1. Add failing table-driven tests for a provider-neutral resolved connection value and strict config decoding; preserve legacy fixtures.
2. Implement built-in TypeSafe/Vercel profiles and named custom profiles in CLI configuration. Return one resolved value to the composition root.
3. Make authentication optional/configurable in the existing HTTP adapter and disable redirects; add focused adapter tests first.
4. Wire every network command to the same resolver and add command-level tests proving no command bypasses it.
5. Update CLI reference, architecture, examples, and module guidance. Run the watcher final gate.

## Evidence and notes
- Official TypeSafe Vercel guide: `https://docs.typesafe.ai/guides/vercel-ai-gateway` (checked 2026-09-21).
- Current coupling points: `internal/cli/config.go`, all network command constructors, `cmd/jeq/main.go`, and `internal/infra/typesafeapi/client.go`.
- Current blockers: the client requires a bearer key globally and uses the default Go redirect policy.
- Architecture review: keep domain contracts and pipeline ports unchanged; avoid one adapter per compatible provider.
- QA review: treat redirect refusal, endpoint validation, credential isolation, and loopback-only unauthenticated HTTP as release criteria.

