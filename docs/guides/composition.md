# Composition

jeq commands compose through JSON or NDJSON. Keep one record per line for streams.
In Bash, use `set -o pipefail` so a failed producer or policy cannot be hidden by a later
successful command. It returns the rightmost failing stage's status, not every
stage's exact error. Inspect `PIPESTATUS` immediately only when you need each
stage's status. `jq` owns deterministic selection and reshaping; jeq owns
judgment and typed evidence; the shell owns sequencing and final actions.

Native `ask` sends one complete request. Composed commands construct requests
from flags and input records. Request shape and cost are bounded as follows:

- `map` and `rate`: one request per input record.
- `rank` and `reduce`: one bounded request for the candidate/aggregate set.
- `gate` and `validate`: offline; no TypeSafe request.

Streaming commands consume stdin when their input source is selected. `ask`
consumes stdin only when its selected request/state/questions file is `-`.
Make sources explicit and see the [worked examples](../../examples/README.md).


```bash
# Requires Bash, jeq, and TYPESAFE_API_KEY; this sends synthetic input to TypeSafe.
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

Use `map` for independent records, `rate` for a score, `reduce` for one
aggregate judgment, and `rank` to order candidates. Use `gate` for a local
policy decision after evidence exists. Pointers select fields; inspect the
shape before choosing them.

Evaluation commands emit JSON for one document and NDJSON for streams; human
errors are plain text on stderr. Keep stdout for data and stderr for diagnostics/traces. A model choice is evidence,
not permission to execute a command. The caller owns deduplication, persistence,
redaction, retries, and final action.
