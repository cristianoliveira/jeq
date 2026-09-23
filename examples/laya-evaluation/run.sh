#!/usr/bin/env bash
# Run the bounded provider comparison corpus. This command makes one provider request per case.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
JEQ_BIN=${JEQ_BIN:-jeq}
CASES=${JEQ_EVAL_CASES:-$SCRIPT_DIR/cases.ndjson}
MODEL=${JEQ_EVAL_MODEL:-english}
PROVIDER=${JEQ_PROVIDER:-}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-laya-evaluation.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ ${JEQ_EVAL_CONFIRM:-} != 1 ]]; then
  printf 'set JEQ_EVAL_CONFIRM=1 after reviewing the provider and request count\n' >&2
  exit 2
fi
if [[ -z "$PROVIDER" ]]; then
  printf 'set JEQ_PROVIDER explicitly; this benchmark never selects a provider for you\n' >&2
  exit 2
fi
if [[ ! -f "$CASES" ]] || ! jq -e -s 'length > 0 and all(.[]; (.id|type)=="string" and (.state|type)=="string" and (.question|type)=="object" and (.expected.kind|IN("choice","noul","score","rank")))' "$CASES" >/dev/null; then
  printf 'invalid evaluation corpus: %s\n' "$CASES" >&2
  exit 2
fi

case_count=$(jq -s 'length' "$CASES")
printf 'provider=%s model=%s requests=%s corpus=%s\n' "$PROVIDER" "$MODEL" "$case_count" "$CASES" >&2
results=$TMP_DIR/results.ndjson
: >"$results"

while IFS= read -r case_doc; do
  [[ -n "$case_doc" ]] || continue
  printf '%s\n' "$case_doc" >"$TMP_DIR/case.json"
  jq -c --arg model "$MODEL" '{model:$model,state,questions:{answer:.question}}' "$TMP_DIR/case.json" >"$TMP_DIR/request.json"

  started=$(date +%s%3N)
  set +e
  "$JEQ_BIN" ask --request "$TMP_DIR/request.json" >"$TMP_DIR/response.json" 2>"$TMP_DIR/stderr"
  status=$?
  set -e
  elapsed_ms=$(($(date +%s%3N) - started))

  if (( status != 0 )); then
    jq -cn --slurpfile case "$TMP_DIR/case.json" --argjson exit "$status" --argjson latency_ms "$elapsed_ms" \
      '{id:$case[0].id,family:$case[0].family,primitive:$case[0].primitive,status:"transport_failed",exit:$exit,latency_ms:$latency_ms}' >>"$results"
    continue
  fi

  jq -cn --slurpfile case "$TMP_DIR/case.json" --slurpfile response "$TMP_DIR/response.json" --argjson latency_ms "$elapsed_ms" '
    def square($x): $x * $x;
    $case[0] as $c | $response[0] as $r | $c.expected as $e | $r.answers.answer as $a |
    if $e.kind == "choice" then
      ([($a.probabilities // {}) | to_entries[] | square(.value - (if .key == $e.value then 1 else 0 end))] | add // null) as $brier |
      {actual:$a.choice,correct:($a.choice == $e.value),in_band:null,brier:$brier,absolute_error:null,expected:$e.value}
    elif $e.kind == "noul" then
      {actual:$a.noul,correct:(($a.noul >= 0.5) == ($e.target == 1)),in_band:($a.noul >= $e.min and $a.noul <= $e.max),brier:square($a.noul - $e.target),absolute_error:null,expected:$e.target}
    elif $e.kind == "score" then
      {actual:$a.score,correct:($a.score >= $e.min and $a.score <= $e.max),in_band:($a.score >= $e.min and $a.score <= $e.max),brier:null,absolute_error:((($a.score - $e.point) | fabs)),expected:$e.point}
    else
      ([$a.probabilities | to_entries | sort_by([-(.value), .key])[] | .key]) as $order |
      ([($a.probabilities // {}) | to_entries[] | square(.value - (if .key == $e.order[0] then 1 else 0 end))] | add // null) as $brier |
      {actual:$order,correct:($order == $e.order),top1_correct:($order[0] == $e.order[0]),in_band:null,brier:$brier,absolute_error:null,expected:$e.order}
    end |
    . + {id:$c.id,family:$c.family,primitive:$c.primitive,status:"ok",latency_ms:$latency_ms,model:$r.model,usage:$r.usage}
  ' >>"$results"
done <"$CASES"

jq -s --arg provider "$PROVIDER" --arg requested_model "$MODEL" '
  def average($values): if ($values | length) == 0 then null else (($values | add) / ($values | length)) end;
  . as $items |
  [$items[] | select(.status == "ok")] as $quality |
  [$quality[] | select(.primitive == "choice")] as $choice |
  [$quality[] | select(.primitive == "noul")] as $noul |
  [$quality[] | select(.primitive == "score")] as $score |
  [$quality[] | select(.primitive == "rank")] as $rank |
  {
    provider:$provider,
    requested_model:$requested_model,
    cases:($items | length),
    quality_cases:($quality | length),
    transport_failures:([$items[] | select(.status != "ok")] | length),
    exact_accuracy:average([$quality[] | if .correct then 1 else 0 end]),
    choice_accuracy:average([$choice[] | if .correct then 1 else 0 end]),
    choice_brier:average([$choice[].brier | select(. != null)]),
    noul_sign_accuracy:average([$noul[] | if .correct then 1 else 0 end]),
    noul_band_rate:average([$noul[] | if .in_band then 1 else 0 end]),
    noul_brier:average([$noul[].brier]),
    score_band_rate:average([$score[] | if .in_band then 1 else 0 end]),
    score_mae:average([$score[].absolute_error]),
    rank_top1_accuracy:average([$rank[] | if .top1_correct then 1 else 0 end]),
    rank_exact_order_accuracy:average([$rank[] | if .correct then 1 else 0 end]),
    latency_ms:{total:([$quality[].latency_ms] | add // 0),mean:average([$quality[].latency_ms])},
    usage:{input_tokens:([$quality[].usage.input_tokens] | add // 0),output_tokens:([$quality[].usage.output_tokens] | add // 0)},
    failures:[$items[] | select(.status != "ok" or .correct != true) | .id],
    items:$items
  }
' "$results"
