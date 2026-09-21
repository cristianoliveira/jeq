# Purpose

Contract owns the TypeSafe System One document language: strict local decoding, typed values, deterministic encoding, and lossless preservation of unknown fields.

# Boundaries

It validates and represents documents without performing I/O, invoking the API, or deciding shell source precedence. Stable error codes are provided by [jeq rules](../jeq/AGENTS.md).

# Connections

- [jeq rules](../jeq/AGENTS.md): supplies the stable error-code registry used by contract validation.
- [Domain](../AGENTS.md): consumes contract values for composition and pipelines.
- [Infrastructure](../../infra/AGENTS.md): consumes encoded responses and typed requests at adapter boundaries.
- [CLI](../../cli/AGENTS.md): consumes validation and document types through domain orchestration.

# Landmarks

- `internal/domain/contract/types.go:Request`: native request representation.
- `internal/domain/contract/decode.go:DecodeRequest`: strict request decoding entry point.
- `internal/domain/contract/validate.go:ValidateRequest`: client-owned request invariant boundary.
- `internal/domain/contract/encode.go:Response.Encode`: deterministic lossless response encoding.

# Placement

Put stable wire/document shape and local structural invariants here. Keep source selection, policy decisions, and transport classification outside this package.
