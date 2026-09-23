#!/usr/bin/env bash
# Verify the evaluation harness with controlled local responses. This test makes no network call.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-laya-evaluation-test.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

jq -e -s 'length == 19 and ([.[].id] | unique | length) == 19' "$SCRIPT_DIR/cases.ndjson" >/dev/null

cat >"$TMP_DIR/cases.ndjson" <<'CASES'
{"id":"choice","family":"test","primitive":"choice","state":"fake-choice","question":{"type":"choice","instructions":"choose","criteria":{"a":null,"b":null}},"expected":{"kind":"choice","value":"a"}}
{"id":"noul","family":"test","primitive":"noul","state":"fake-noul","question":{"type":"noul","instructions":"true?"},"expected":{"kind":"noul","target":1,"min":0.8,"max":1.0}}
{"id":"score","family":"test","primitive":"score","state":"fake-score","question":{"type":"score","instructions":"score","criteria":["0","1"]},"expected":{"kind":"score","point":0.5,"min":0.4,"max":0.6}}
{"id":"rank","family":"test","primitive":"rank","state":"fake-rank","question":{"type":"choice","instructions":"rank","criteria":{"a":null,"b":null}},"expected":{"kind":"rank","order":["b","a"]}}
{"id":"invalid","family":"test","primitive":"noul","state":"fake-invalid","question":{"type":"noul","instructions":"true?"},"expected":{"kind":"noul","target":1,"min":0.8,"max":1.0}}
{"id":"incomplete-probabilities","family":"test","primitive":"choice","state":"fake-incomplete","question":{"type":"choice","instructions":"choose","criteria":{"a":null,"b":null}},"expected":{"kind":"choice","value":"a"}}
{"id":"invalid-probability-sum","family":"test","primitive":"choice","state":"fake-invalid-sum","question":{"type":"choice","instructions":"choose","criteria":{"a":null,"b":null}},"expected":{"kind":"choice","value":"a"}}
{"id":"transport","family":"test","primitive":"choice","state":"fake-transport","question":{"type":"choice","instructions":"choose","criteria":{"a":null,"b":null}},"expected":{"kind":"choice","value":"a"}}
CASES

cat >"$TMP_DIR/jeq" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
request=${3:?request path}
state=$(jq -r .state "$request")
case "$state" in
  fake-choice) answer='{"type":"choice","choice":"a","probabilities":{"a":0.9,"b":0.1},"confidence":0.8}' ;;
  fake-noul) answer='{"type":"noul","noul":0.9,"confidence":0.9}' ;;
  fake-score) answer='{"type":"score","score":0.5,"legend":{"0":"0","1":"1"},"probabilities":{"0":0.5,"1":0.5},"confidence":0}' ;;
  fake-rank) answer='{"type":"choice","choice":"b","probabilities":{"a":0.3,"b":0.7},"confidence":0.4}' ;;
  fake-invalid)
    jq -cn '{model:"controlled",answers:{},usage:{input_tokens:1,output_tokens:0}}'
    exit 0
    ;;
  fake-incomplete) answer='{"type":"choice","choice":"a","probabilities":{"a":1},"confidence":1}' ;;
  fake-invalid-sum) answer='{"type":"choice","choice":"a","probabilities":{"a":0.4,"b":0.1},"confidence":0.3}' ;;
  *) exit 64 ;;
esac
jq -cn --argjson answer "$answer" '{model:"controlled",answers:{answer:$answer},usage:{input_tokens:1,output_tokens:0}}'
FAKE
chmod +x "$TMP_DIR/jeq"

set +e
JEQ_PROVIDER=custom JEQ_EVAL_CASES="$TMP_DIR/cases.ndjson" JEQ_BIN="$TMP_DIR/jeq" "$SCRIPT_DIR/run.sh" >/dev/null 2>"$TMP_DIR/unconfirmed.stderr"
unconfirmed=$?
set -e
[[ "$unconfirmed" == 2 ]]

head -n 7 "$TMP_DIR/cases.ndjson" >"$TMP_DIR/complete-cases.ndjson"
JEQ_PROVIDER=custom JEQ_EVAL_MODEL=test JEQ_EVAL_CONFIRM=1 JEQ_EVAL_CASES="$TMP_DIR/complete-cases.ndjson" JEQ_BIN="$TMP_DIR/jeq" \
  "$SCRIPT_DIR/run.sh" >"$TMP_DIR/complete-result.json" 2>"$TMP_DIR/complete-stderr"
jq -e '
  .cases == 7 and .transport_failures == 0 and .invalid_responses == 1 and
  .probability_metrics_unscorable == 2 and .useful_answers == 6 and
  .useful_answers_per_input_token == (6 / 7) and
  .usage == {complete:true,input_tokens:7,output_tokens:0}
' "$TMP_DIR/complete-result.json" >/dev/null

JEQ_PROVIDER=custom JEQ_EVAL_MODEL=test JEQ_EVAL_CONFIRM=1 JEQ_EVAL_CASES="$TMP_DIR/cases.ndjson" JEQ_BIN="$TMP_DIR/jeq" \
  "$SCRIPT_DIR/run.sh" >"$TMP_DIR/result.json" 2>"$TMP_DIR/stderr"

jq -e '
  .provider == "custom" and .requested_model == "test" and
  .cases == 8 and .quality_cases == 6 and .transport_failures == 1 and
  .invalid_responses == 1 and .unscorable_responses == 0 and
  .probability_metrics_unscorable == 2 and .overall_acceptance_rate == 1 and
  .useful_answers == 6 and .useful_answers_per_input_token == null and
  .choice_accuracy == 1 and .noul_sign_accuracy == 1 and .noul_band_rate == 1 and
  .score_band_rate == 1 and .score_mae == 0 and .rank_top1_accuracy == 1 and
  .rank_exact_order_accuracy == 1 and
  .usage == {complete:false,input_tokens:7,output_tokens:0} and .failures == ["invalid","transport"]
' "$TMP_DIR/result.json" >/dev/null

printf 'PASS laya evaluation harness\n'
