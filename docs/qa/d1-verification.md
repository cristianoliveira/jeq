# D1 verification report (TASK-0003/0004/0005 vs acceptance plan)

Status: independent QA verification — evidence gathered from the tree, not from delivery claims. No code or plans changed; nothing committed.
Commits under test: **980dc1e** (strict lossless decode), **680326c** (closed local validation gate), **8c16115** (ask modes, composition, model precedence). HEAD at verification: b5bad98; code under `internal/` is identical to 8c16115 for these packages (later commits touch plans/ and D0-8 only — `git diff 8c16115 HEAD -- internal/ cmd/` is empty).
Scope: local contract/composition correctness (D1-1..D1-14). Live paid evidence already captured separately (docs/qa/live-api-baseline.md).

## Result summary

| Check | Result |
| --- | --- |
| D1-1 malformed JSON fails locally | **PASS** |
| D1-2 duplicate keys rejected, not last-wins | **PASS** |
| D1-3 unknown fields pass through, both modes | **PASS** |
| D1-4 type mismatches name the field path | **PASS** |
| D1-5 response decode is tolerant/lossless | **PASS** |
| D1-6 unknown primitive fails locally | **PASS** |
| D1-7 closed local rule set (OQ-2 list) | **PASS** |
| D1-8 empty state / empty questions fail locally | **PASS** |
| D1-9 full source-conflict matrix, zero I/O | **PASS** |
| D1-10 exactly one state source accepted | **PASS** |
| D1-11 stdin never read implicitly | **DEFERRED** (needs the ask command, TASK-0011/D2) |
| D1-12 model precedence chain | **PASS** (in `internal/cli` — plan corrected) |
| D1-13 base-url override honored | **DEFERRED** (HTTP client is TASK-0006/D2) |
| D1-14 fixture↔code meta-test | **PASS** |

**12 pass · 0 fail · 2 deferred · 2 findings (1 low, 1 observation)**

## Evidence

### D1-1 / D1-2 / D1-4 — strict decode: malformed, duplicates, types — PASS

```
$ go test ./internal/domain/contract/... -run TestDecodeRequestStrict -v   → 10/10 subtests PASS
$ go test ./internal/domain/contract/... -run TestDecodeRequestTypeMismatchNamesFieldPath -v → PASS
```

Table covers truncated, trailing comma, trailing data after document, top-level AND nested duplicate keys (`JEQ_REQUEST_INVALID`, not last-wins), UTF-8 BOM, invalid UTF-8, and type mismatches (`model: 7`, `questions: []`, `criteria: "Calm"` for score) with the field path named (`questions.f.criteria`). Fixture `contract/dup_key.json` matches the inline case; `contract/malformed_truncated.json`, `malformed_trailing_comma.json`, `type_mismatch.json`, `bom.json` present in the corpus.

### D1-3 — unknown fields pass through in both modes — PASS

```
$ go test ./internal/domain/contract/... -run TestUnknownFieldPassthrough -v   → PASS
$ go test ./internal/domain/jeq/... -run TestComposeQuestionsDocUnknownFieldsPassThrough -v → PASS
```

Implementation reads: `DecodeRequest`/`decodeQuestion` park unrecognized keys in `Extra` (raw preserved via `json.Compact`); `DecodeQuestionsDoc` returns document-level extras that `Compose` rides onto the outgoing request. Fixture `contract/unknown_field.json` carries `vendor_meta` (question-level) and `x_extra` (request-level). No unknown field is rejected anywhere (OQ-1 honored).

### D1-5 — tolerant response decode — PASS

```
$ go test ./internal/domain/contract/... -run 'TestLosslessResponse|TestDecodeResponseMissingKnownFieldIsResponseInvalid|TestDecodeResponseHappyPath' -v → PASS
```

Unknown server fields survive in `Response.Extra`/`Answer.Extra`/`Usage.unknown`; missing `model`/`answers`/`usage`, missing primitive-required fields, or unknown answer `type` classify `JEQ_RESPONSE_INVALID` (exit 1 semantics, never local-input). Matches the live-API schema captured in docs/qa/live-api-baseline.md.

### D1-6 / D1-7 / D1-8 — closed local validation gate — PASS

```
$ go test ./internal/domain/contract/... -run 'TestValidateLocalRules|TestValidateEveryRuleHasRecoveryText|TestValidateAcceptsValidDocuments' -v → PASS (rule subtests incl. unknown primitive, choice/score/noul criteria shapes, missing instructions)
$ go test ./internal/domain/contract/... -run TestValidatePreNetwork -v → PASS (validates the whole fixture corpus with a live httptest server counting requests: 0)
```

`Rules()` is the closed ten-rule list from TASK-0004 (OQ-2): wellformed JSON, no duplicate keys, request schema, non-empty state, non-empty questions, known primitive, non-empty instructions, choice criteria non-empty, score ≥2 levels, noul criteria shape. Every rule carries recovery text (test-enforced). Fixtures `unknown_primitive.json`, `choice_no_options.json`, `score_no_levels.json`, `missing_instructions.json`, `noul_criteria_wrong_shape.json`, `empty_state.json`, `empty_questions.json` all mapped. Violation order is deterministic (sorted question ids).

### D1-9 / D1-10 — source matrix and composition — PASS

```
$ go test ./internal/domain/jeq/... -run 'TestModeConflictMatrix|TestCompose' -v
--- PASS: TestModeConflictMatrix (17 matrix cells)      --- PASS: TestComposeComposedMode
--- PASS: TestComposeStateJSONSource (+RejectsScalars)  --- PASS: TestComposeNativeMode
--- PASS: TestComposeComposedEmptyStateFailsBeforeIO    --- PASS: TestComposeEmptyModelFails
--- PASS: TestComposeInvalidQuestionsDocFailsLocally    --- PASS: TestComposeNativeModeInvalidDocumentFailsLocally
--- PASS: TestComposeNeverTouchesNetwork
```

The matrix is the full ADR 0001 set: native alone ✓; `--request` × any other source → `JEQ_SOURCE_CONFLICT` (5 cells); two/three state sources → conflict (5 cells); nothing/state-alone/questions-without-state → `JEQ_INPUT_INVALID` (3 cells); each single state source ✓. `TestComposeNeverTouchesNetwork` runs `Compose` over happy and failing inputs against a counting server — **0 requests**. Purity is structural: production `internal/domain/**` imports stdlib only (encoding/json, strings, bytes, fmt, sort, math, errors); no `os`, `net/http`, or Cobra (`internal/arch` gate enforces the arrows; green). Environment and filesystem never reach the domain — `ComposeInput` receives resolved values only.

### D1-12 — model precedence — PASS (location corrected in plan)

```
$ go test ./internal/cli/... -run TestModelPrecedence -v → PASS
```

`internal/cli/model.go`: `--model` → `TYPESAFE_DEFAULT_MODEL` → `jev-latest`, env via injected `getenv` for testability. Plan corrected per PO: D1-12 belongs in `internal/cli` (os.Getenv containment per ADR 0002), not `internal/domain/jeq`.

### D1-14 — fixture↔code meta-test — PASS

```
$ go test ./internal/fixtures/... -v → TestFixtureCodeCoverage PASS
```

Every failing fixture in `contract/` declares exactly one stable code from the D0 registry (unknown codes and double-declarations fail), and every undeclared request fixture must validate clean — the corpus cannot lie. Mirrors the D1-14 design in the acceptance plan.

### D1-11 / D1-13 — DEFERRED

- **D1-11** (stdin never read implicitly): the `ask` command does not exist at the CLI layer yet (TASK-0011, D2); the binary probe with a closed/`/dev/null` stdin is only meaningful then. Domain-level preconditions already hold: `CheckSources` requires explicit sources, `Compose` is pure (zero I/O proven), so nothing can block on a TTY today. Re-verify in D2.
- **D1-13** (base-url override): the HTTP client is TASK-0006/D2. `TYPESAFE_BASE_URL`/`--base-url` have no consumption site yet.

## Findings

**F-D1-1 (LOW, consistency): empty-state detection is whitespace-sensitive for object/array states.**
`emptyJSONValue` (internal/domain/contract/validate.go) treats `{}`/`[]` as empty by byte length (`len(trimmed) == 2`). A pretty-printed empty object `state: {\n}` trims to length 3 and **passes** the local gate, while inline `state: {}` is rejected — same semantic value, different verdict, both deterministic. Whitespace-only *string* states are handled correctly (rejected). Impact: a caller can send an empty object/array state to the server where the local gate promised to catch it. Suggested owner: Dave, D2 window (one-line fix: `json.Compact` before the length check). Not fixed per instructions.

**Observation (no action): native-mode model authority.**
In native mode the document's own `model` field wins and the composed-precedence chain deliberately does not apply (`compose.go` comments this; `TestComposeNativeMode` covers it). Matches ADR 0001, which defines precedence for *composed* mode only. Flagging so D2 e2e fixtures don't assume the flag overrides a native document's model.

## Gate

```
$ nix develop -c make check → true (exit 0)
```

## Corrections applied to docs/qa/acceptance-d0-d2.md

- D1-12 location: `internal/domain/jeq` → `internal/cli` (command updated to `go test ./internal/cli/... -run TestModelPrecedence`).
- F-1/D0-8 phase split (bootstrap stderr fallback kept; TASK-0017 replaces it; D2-17 final) — already in place from the prior ruling, unchanged.
