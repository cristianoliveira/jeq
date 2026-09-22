---
id: TASK-0058
title: Document Jev provider selection
status: done
depends_on: []
priority: normal
tags: [documentation, providers, configuration, security, jev]
---

# Document Jev provider selection

## Problem
jeq supports TypeSafe, Vercel AI Gateway, and System One-compatible custom profiles, but the public instructions are fragmented and omit runnable configuration, protocol, verification, and security details. Users cannot confidently switch providers without reading architecture or source code.

## Desired outcome
A focused provider guide lets a user select TypeSafe direct, Vercel AI Gateway, a one-off custom endpoint, or a named custom profile without reading source. It states the compatibility and security boundaries, shows safe credential handling, and gives a non-evaluation verification command.

## Acceptance criteria
- [ ] Add `docs/guides/providers.md` and link it from the root README, installation guide, and CLI reference.
- [ ] Explain that provider selection is process-wide and explicit through `JEQ_PROVIDER`; jeq does not infer a provider from credentials.
- [ ] Document TypeSafe direct as the default with `TYPESAFE_API_KEY` and `jev-latest`.
- [ ] Document Vercel AI Gateway with `JEQ_PROVIDER=vercel`, `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN` fallback, gateway URL, and default `typesafe-ai/jev` model.
- [ ] Document one-off `custom` selection with `JEQ_BASE_URL`, `JEQ_DEFAULT_MODEL`, `JEQ_API_KEY`, and optional `JEQ_AUTH` behavior.
- [ ] Provide a complete named-profile JSON example using `default_provider`, `base_url`, `default_model`, `auth`, and `api_key_env`; no credential value appears in the file.
- [ ] State that custom providers must implement the System One-compatible `POST /v1/systemone` and `GET /v1/models` contract; a generic OpenAI-compatible endpoint does not work without translation.
- [ ] State remote HTTPS, redirect refusal, and loopback-only unauthenticated HTTP constraints.
- [ ] Explain model precedence accurately and distinguish composed commands from native `ask` requests.
- [ ] Use `jeq models` as the verification step and give direct troubleshooting for missing credentials, invalid URLs, and unsupported provider/model routes.
- [ ] Avoid claiming third-party availability beyond jeq's implemented provider contract.
- [ ] Existing documentation link validation, formatting, and final watcher pass.

## Constraints
- Keep credentials in environment or a secret manager, never in config, arguments, examples, or committed files.
- Keep provider infrastructure separate from semantic command examples.
- Do not add provider behavior or flags in this documentation task.

## Evidence
- Cross-check every environment variable, default, URL rule, and precedence statement against `internal/cli/config.go` and `internal/infra/typesafeapi/client.go`.
- Run the existing provider/config tests and documentation validators; do not add tests that only grep prose.
- Independent QA reviews the copy-paste examples and links without making live calls.

