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

  set +e
  jq -n --slurpfile case "$TMP_DIR/case.json" --slurpfile response "$TMP_DIR/response.json" \
    --argjson latency_ms "$elapsed_ms" -f "$SCRIPT_DIR/score.jq" >"$TMP_DIR/scored.json"
  score_status=$?
  set -e
  if (( score_status != 0 )); then
    jq -cn --slurpfile case "$TMP_DIR/case.json" --argjson latency_ms "$elapsed_ms" \
      '{id:$case[0].id,family:$case[0].family,primitive:$case[0].primitive,status:"invalid_response",correct:false,useful:false,latency_ms:$latency_ms}' >>"$results"
    continue
  fi
  cat "$TMP_DIR/scored.json" >>"$results"
done <"$CASES"

jq -s --arg provider "$PROVIDER" --arg requested_model "$MODEL" '
  def average($values): if ($values | length) == 0 then null else (($values | add) / ($values | length)) end;
  . as $items |
  [$items[] | select(.status == "ok")] as $quality |
  [$quality[] | select(.primitive == "choice")] as $choice |
  [$quality[] | select(.primitive == "noul")] as $noul |
  [$quality[] | select(.primitive == "score")] as $score |
  [$quality[] | select(.primitive == "rank")] as $rank |
  [$items[] | select((.usage.input_tokens | type) == "number" and (.usage.output_tokens | type) == "number")] as $reported_usage |
  ([$quality[] | if .useful then 1 else 0 end] | add // 0) as $useful_answers |
  ([$reported_usage[].usage.input_tokens] | add // 0) as $input_tokens |
  ($reported_usage | length == ($items | length)) as $usage_complete |
  {
    provider:$provider,
    requested_model:$requested_model,
    cases:($items | length),
    quality_cases:($quality | length),
    transport_failures:([$items[] | select(.status == "transport_failed")] | length),
    invalid_responses:([$items[] | select(.status == "invalid_response")] | length),
    unscorable_responses:([$items[] | select(.status == "unscorable")] | length),
    probability_metrics_unscorable:([$quality[] | select(.probability_metric_status == "unscorable")] | length),
    overall_acceptance_rate:average([$quality[] | if .correct then 1 else 0 end]),
    useful_answers:$useful_answers,
    useful_answers_per_input_token:(if $usage_complete and $input_tokens > 0 then $useful_answers / $input_tokens else null end),
    choice_accuracy:average([$choice[] | if .correct then 1 else 0 end]),
    choice_brier:average([$choice[].brier | select(. != null)]),
    noul_sign_accuracy:average([$noul[] | if .correct then 1 else 0 end]),
    noul_band_rate:average([$noul[] | if .in_band then 1 else 0 end]),
    noul_brier:average([$noul[].brier]),
    score_band_rate:average([$score[] | if .in_band then 1 else 0 end]),
    score_mae:average([$score[].absolute_error]),
    rank_top1_accuracy:average([$rank[] | if .top1_correct then 1 else 0 end]),
    rank_exact_order_accuracy:average([$rank[] | if .correct then 1 else 0 end]),
    latency_ms:{total:([$items[].latency_ms] | add // 0),mean:average([$items[].latency_ms])},
    usage:{complete:$usage_complete,input_tokens:$input_tokens,output_tokens:([$reported_usage[].usage.output_tokens] | add // 0)},
    failures:[$items[] | select(.status != "ok" or .correct != true) | .id],
    items:$items
  }
' "$results"
