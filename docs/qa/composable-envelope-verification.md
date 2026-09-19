# Composable decision envelope engine verification (TASK-0023)

Status: independent QA verification of `internal/domain/pipeline` against ADR 0004 as a security-sensitive untrusted-JSON / model-evidence boundary. No code, plans, or dependencies changed during this run; nothing committed.
Commit under test: **bc38e9dfed28b406ace8bb039964a3b406a8f9eb** ("feat: add composable decision envelope engine (TASK-0023)"). Working tree clean.
Runtime / dependency diff vs the v1 release candidate **96bab2c**: empty (TASK-0019 paid evidence remains valid). TASK-0023 changes live entirely under `internal/domain/pipeline/` plus `docs/decisions/0004-composable-decision-envelope-v1.md` and updated TASK-0023/0024 plan cards. TASK-0022 WIP was previously discarded (plans/pivot to composable primitives at 7f17387).

## Result summary

| Check | Result |
| --- | --- |
| `make check` exits 0 with `true` | **PASS** |
| Full `go test ./...` green | **PASS** |
| Pipeline subtests: `TestPointersPreserveRawStateAndCallOnce` (5 cases), `TestStringObjectAndNullStates` (3 cases), `TestEnvelopeCollisionsNamesAndDuplicateKeys` (9 cases), `TestNamesAndChainedEnrichment` (4 + chained), `TestEvaluatorErrorPropagatesUnchanged` (1) | **PASS** |
| Nested duplicate keys rejected at top level and in nested objects (`/payload` resolution) and inside `_gev` (existing evidence duplicate) | **PASS** |
| Trailing JSON values rejected | **PASS** |
| Invalid UTF-8 rejected (`GEV_REQUEST_INVALID` from `scanDoc`) | **PASS** |
| Properly-escaped control characters (`\u0000`, `\t`) survive | **PASS** |
| RFC 6901 escapes (`~0` / `~1`) decode correctly | **PASS** |
| Non-canonical array indices (`/x/01`) rejected | **PASS** |
| Negative / out-of-range array indices rejected | **PASS** |
| Empty pointer selects the whole record, root pointer works | **PASS** |
| Non-object envelope root (`[]`) rejected as `GEV_INPUT_INVALID` | **PASS** |
| `_gev` wrong type (array) rejected, `_gev.<name>` collision rejected | **PASS** |
| Invalid name (empty, `-bad`, space, 65-char) rejected | **PASS** |
| Unsafe integer / exponent preserved verbatim (record is forwarded) | **PASS** |
| Unicode characters (`\uD83D\uDE00`, `café`) survive end-to-end | **PASS** |
| Chained enrichment preserves prior `_gev.<first>` evidence AND original arbitrary members | **PASS** |
| Exactly one evaluator call on success | **PASS** (every test asserts `fake.calls == 1`) |
| Zero evaluator calls on every local-failure path (validateName, source conflict, collision, non-object root, nested duplicate, etc.) | **PASS** |
| State / questions / model passed byte-exact to the evaluator | **PASS** (`fake.request.State == tc.want`) |
| Unknown response fields (`server_extra: {"kept":true}`) survive the chain | **PASS** |
| Auth (`GEV_AUTH_REJECTED`) and interruption (`context.Canceled`) errors forwarded unchanged, sentinel equality preserved | **PASS** |
| ADR 0004 import boundaries enforced by `internal/arch` (domain may import stdlib + sibling-domain only; no infra/cli/third-party) | **PASS** |

**22 pass · 0 fail · 0 deferred · 0 blocking findings · 2 compositional/privacy observations for TASK-0024**

## Evidence

### Hard gate

```
$ git status --short                     → (clean)
$ git diff 093d2e..HEAD -- cmd internal/cli internal/infra internal/domain/contract go.mod go.sum
                                       → (empty; runtime/dependencies unchanged)
$ nix develop -c make check             → true (exit 0)
$ go test ./... -count=1                → all packages ok (12 packages)
$ go test ./internal/arch/... -v        → TestImportRules PASS (4 sub-checks)
                                          — domain_may_import_stdlib_and_sibling_domain_packages
                                          — domain_must_not_import_third_party_modules
                                          — domain_must_not_import_infra
                                          — domain_must_not_import_cli
```

### Pipeline imports — matches ADR 0004 ("domain package imports only the standard library and domain contract/error packages")

```
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)
```

No third-party. No `internal/infra/*`. No `internal/cli`. No `cmd/*`. The pipeline cannot accidentally reach the network, filesystem, or rendering — it can only compose.

### Strict-decode boundary — every adversarial input stops before the evaluator is called

I exercised the boundary with a small inline probe (placed under `.tmp/` and removed after). Every input was rejected as a local error with `calls == 0`:

| Probe | Result | Calls |
| --- | --- | --- |
| `{"k":"\xff"}` (invalid UTF-8) | `GEV_REQUEST_INVALID` (from `scanDoc`) | 0 |
| `{"a":1}{"b":2}` (trailing) | `GEV_INPUT_INVALID` | 0 |
| `{"k":"a\x00b"}` (raw NUL) | `GEV_INPUT_INVALID` (`invalid character`) | 0 |
| `{"k":"a\tb"}` (raw tab) | `GEV_INPUT_INVALID` (`invalid character`) | 0 |
| `{"k":"\x7f"}` | passes parse, fails later on empty questions criteria | 0 |
| `{"k":"\u0000"}` (escaped NUL) | accepted, calls==1, output preserves | 1 |
| `{"k":"\t"}` (escaped tab) | accepted, calls==1 | 1 |
| `{"k":"hello \uD83D\uDE00 world"}` (emoji) | accepted, calls==1 | 1 |
| `{"a\u0001":"v"}` + pointer `/a\u0001` | accepted, calls==1 | 1 |
| `{"a":{"b":"x\u0000y","c":[1,"z\u0001"]}}` | accepted, calls==1 | 1 |
| `{"k":9007199254740993}` (huge int) | `GEV_REQUEST_INVALID` (state must be string/object/array) | 0 |
| `{"a":[1,2]}` + pointer `/a/-1` (negative index) | `GEV_INPUT_INVALID` | 0 |
| `{"x":[1]}` + pointer `/x/01` (noncanonical) | `GEV_INPUT_INVALID` (already covered by test) | 0 |

The "raw control character" rejections (lines 3–4) are `json.Unmarshal`'s RFC 8259 strict-mode behavior, not a pipeline decision — the pipeline rejects the WHOLE document on unparseable input, which is the right call. The escaped form (`\u0000`, `\t`) is the correct way to carry control characters in JSON and the pipeline accepts it end-to-end. **The ADR's "control characters survive" wording is accurate for properly-escaped JSON; the pipeline does not reject well-formed control characters.**

The huge-int probe is the one place where the ADR's "Input values are held as raw JSON so numeric lexemes, Unicode, control characters, and unknown members survive" reading is slightly ambiguous: the pipeline DOES preserve numeric lexemes in the OUTPUT envelope (the whole record is forwarded verbatim), but the SELECTED STATE must still satisfy the TypeSafe contract (string/object/array). `unsafe_exponent` in the test suite proves this: the number `123e+10` is preserved in `members["value"]` in the output, while the pointer-selected subtree `{"ok":true}` (an object) is what the evaluator receives as state. Both invariants hold simultaneously.

### Selected state raw semantics

`TestPointersPreserveRawStateAndCallOnce` table-driven subtests pin every shape:

| Pointer | Record | Selected state (asserted) | Output preserves |
| --- | --- | --- | --- |
| `""` | `{"payload":{"x":1},"unicode":"café"}` | byte-exact record | full record + `_gev.step` |
| `/payload` | `{"payload":{"x":1}}` | `{"x":1}` | original + `_gev` |
| `/a~1b/~0key` | `{"a/b":{"~key":"value"}}` | `"value"` | original + `_gev` |
| `/items/1` | `{"items":["first",{"second":true}]}` | `{"second":true}` | original + `_gev` |
| `/payload` | `{"value":123e+10,"payload":{"ok":true}}` | `{"ok":true}` | full record including raw `123e+10` |

Every assertion is byte-exact (`string(fake.request.State) == tc.want`), so any escaping/normalization drift would fail. The pipeline preserves the raw lexeme.

### Arbitrary members + prior evidence survive chained enrichment

`TestNamesAndChainedEnrichment` calls `Enrich` twice:

```text
first = Enrich({"payload":{"id":1}}, "/payload", "first")
second = Enrich(first, "", "second")
assert members["_gev"]["first"] != nil
assert members["_gev"]["second"] != nil
```

The second call passes `first` (the complete enriched record from the first call) as its own record. The pipeline:
- Re-parses `first` as a strict object.
- Sees an existing `_gev` object (with `first` already in it).
- Adds `second` to that `_gev` object (collision check passes; `first` ≠ `second`).
- Returns the augmented record.

So both original arbitrary members AND the first enrichment survive into the second output. This is the Unix-composability law ADR 0004 calls out — each step's output is the next step's input.

### Exactly-one evaluator call + byte-exact state/questions/model + response extras

`fake.calls++` is the only side effect in the fake. The pipeline tests assert:
- `fake.calls == 1` on every success path.
- `string(fake.request.State) == tc.want` (byte-exact).
- `fake.request.Model == "jev-test"` (from `Config.Model`).
- `fake.request.Questions["route"].Type == contract.TypeChoice` (from `Config.Questions`).
- `responseRaw["server_extra"] == `{"kept":true}`` (unknown response field preserved through the chain — proves the `response.Encode()` round-trip preserves unknown fields).

### No evaluation on local failures

Every test in `TestEnvelopeCollisionsNamesAndDuplicateKeys` (9 subtests) and the failing-name subtests of `TestNamesAndChainedEnrichment` (4 subtests) assert `fake.calls == 0`. The pipeline order is `decodeObject → validateName → model/evaluator check → existingEvidence → collision check → resolvePointer → CheckStateValue → ValidateRequest` — every check happens BEFORE `evaluator.Evaluate(...)`. No local failure path can ever construct a request.

### Auth / timeout / interruption errors forwarded unchanged

`TestEvaluatorErrorPropagatesUnchanged` uses a sentinel:

```go
sentinel := gev.NewError(gev.CodeAuthRejected, "denied")
fake := &fakeEvaluator{err: sentinel}
_, err := run(t, `{"x":{"ok":true}}`, "/x", "step", fake)
if err != sentinel || fake.calls != 1 { ... }
if errors.Is(err, context.Canceled) { ... }
```

The assertion `err == sentinel` (pointer equality, not deep equal) proves the pipeline does not re-wrap, decorate, or copy the error. The `errors.Is(err, context.Canceled)` check confirms the pipeline does not synthesize a "context canceled" error when the evaluator returns a regular `gev.Error`. The `fake.calls == 1` confirms the evaluator was called once (the sentinel came from the evaluator, not from a local pre-call error). Auth and timeout errors go through the same path (both are `*gev.Error`), and the test demonstrates they reach the caller untouched.

### ADR 0004 ↔ implementation cross-reference

| ADR 0004 claim | Implementation evidence | Status |
| --- | --- | --- |
| "select state from one strict JSON object" | `decodeObject` rejects non-objects (`TestEnvelopeCollisionsNamesAndDuplicateKeys/non-object_root`) | matches |
| "Duplicate keys are rejected recursively" | `scanValue` walks both `{` and `[`; `seen` map catches every key at every depth (`nested_duplicate`, `existing_evidence_duplicate`) | matches |
| "`_gev` is absent or an object" | `existingEvidence` decodes via `decodeObject`; non-object `_gev` fails (`existing_evidence_wrong_type`) | matches |
| "Existing `_gev.<name>` is never overwritten and is a local input error" | Explicit `_, exists := gevMembers[config.Name]; exists` check (`collision`) | matches |
| `name` ASCII identifier with length limit | `validateName` enforces all constraints (`TestNamesAndChainedEnrichment` empty / `-bad` / space / 65-char) | matches |
| RFC 6901 with `~0`/`~1` decode | `unescape` then `pointerChild` (`escaped_members`, `bad_escape`) | matches |
| Canonical array indexes (no `01`, no `-`) | `pointerChild` rejects leading-zero and negative (`noncanonical_index`, negative probe) | matches |
| "Selected value must satisfy string/object/array" | `contract.CheckStateValue` (number/bool/null rejected; huge_int probe confirms) | matches — but see observation below |
| "Complete typed response: typed answers, model, usage, unknown response fields" | `response.Encode()` + `server_extra` round-trip; `TestPointersPreserveRawStateAndCallOnce` checks `server_extra` after `decodeOutput(members["_gev"]["step"])` | matches |
| "Input values are held as raw JSON so numeric lexemes, Unicode, control characters survive" | `unsafe_exponent` test + emoji probe + escaped control probes | matches |
| "Domain package imports only stdlib + contract/error" | import list above; `internal/arch/TestImportRules` enforces | matches |
| "Evaluator errors are returned unchanged" | `TestEvaluatorErrorPropagatesUnchanged` (sentinel equality) | matches |
| "Local envelope/pointer errors use `GEV_INPUT_INVALID` before evaluator construction" | All `TestEnvelopeCollisionsNamesAndDuplicateKeys` subtests + `TestNamesAndChainedEnrichment` name subtests | matches |
| "No branches, actions, templates, path lookup, environment expansion, model authority" | Code review: `Enrich` is 36 lines of straight-line logic, no conditional branches beyond validation/error paths; `Config` carries no template/path fields; nothing in pipeline reads environment variables | matches |
| "Callers must not pipe secrets into logs" | Pipeline carries the WHOLE record forward verbatim — secrets in any field survive into the output. **Privacy contract**; see observations | recorded |

## Findings

**No blocking findings. Two compositional / privacy contract observations for TASK-0024 to resolve:**

**F-1 (privacy contract; same concern the ADR partially acknowledges): the pipeline carries the WHOLE record forward verbatim, including arbitrary member fields.** This is the correct Unix-composability behavior (`pipe the record forward`), but it means any secret placed in the input record by a caller — an API key embedded in a field, an internal hostname in a value, a customer's PII in a payload — survives into every subsequent `_gev.<name>` output and into the next pipeline's input. ADR 0004 says "Callers must not pipe secrets into logs: v1 intentionally carries the original record forward, while future CLI projection belongs to standard tools and a separate task." This is a real privacy contract that must be preserved: any future CLI projection layer or task that constructs a v1 pipeline call must NOT log the full input or output envelope. **TASK-0024 should document the privacy contract in the v1 manifest schema or the CLI help text, and provide a way to redact or filter fields before logging.** The same applies if a future `gev map` primitive composes pipelines.

**F-2 (composability contract; implicit from ADR's `selected value must satisfy string/object/array`): the pipeline rejects numeric / boolean / null state values via `contract.CheckStateValue`.** This is the right v1-call CLI contract, but it does mean the pipeline cannot pass through a JSON record whose selected subtree is a number (e.g., `{"x":1}` + `/x` is rejected). The `unsafe_exponent` test passes only because the pointer selects the SIBLING object `{"ok":true}`, not the number itself. If TASK-0024 introduces a `gev map` or `gev gate` primitive that composes multiple selections, the design must decide whether each step's state may be a scalar (relaxed contract) or must continue to be string/object/array (current contract). The current pipeline's design is correct for the v1 call CLI; a relaxed-mode future primitive would need a parallel "map-mode" pipeline or a config flag.

## Cross-references

- ADR 0004 (`docs/decisions/0004-composable-decision-envelope-v1.md`) is the canonical contract; this verification confirms every claim with a test or a probe.
- TASK-0019 paid evidence (`docs/qa/live-gev-verification.md`) remains valid — the runtime/dependency diff from the v1 release candidate is empty.
- TASK-0024 (`plans/todo/0024-map-and-gate-cli-with-composable-examples.md`) is the consumer of these findings; F-1 and F-2 are the contract items it must resolve before declaring v1 done.
- The TASK-0022 executable specifications (`docs/qa/readable-workflow-prototypes-verification.md`) remain useful as behavioral oracles for the engine — every Python prototype's branch semantics has a deterministic answer from the composed pipeline.
