# Todo fulfillment experiment

This experiment sends the **same complete diff** to one judgment per todo item.
It is intentionally simple so we can discover where the approach stops working.

```sh
cat > /tmp/todos.json <<'JSON'
[
  {"id":"AUTH-1","priority":"high","description":"Reject expired authentication tokens."},
  {"id":"AUTH-2","priority":"medium","description":"Add tests for expired-token rejection."}
]
JSON

git diff main...HEAD > /tmp/change.patch
./examples/todo-fulfillment/review.sh /tmp/todos.json /tmp/change.patch
```

The output is one JSON object per todo item. `probability` is JEQ's probability
for the literal fulfillment condition. It is not automatically calibrated
confidence. `confidence` is preserved when the model returns that separate
field.

The default policy is:

- `>= 0.85`: `fulfilled`
- `<= 0.40`: `not_fulfilled`
- between them: `uncertain`

Change thresholds explicitly when experimenting:

```sh
PASS_MIN=0.90 REJECT_MAX=0.30 \
  ./examples/todo-fulfillment/review.sh /tmp/todos.json /tmp/change.patch
```

The script returns exit 0 when all items are fulfilled, 10 when at least one is
not fulfilled, and 11 when no item is rejected but at least one is uncertain.
It makes one request per item, so a large todo list repeats the large diff and
costs more. That repetition is useful for the first experiment; a later version
can judge the whole list in one request or map compact per-item context.

## Stretching the experiment

Use a real large diff, not a fixture. Compare results while varying:

1. the number of todo items;
2. diff size and unrelated-file noise;
3. whether tests are included separately;
4. whether acceptance criteria replace short descriptions;
5. thresholds and human-verified outcomes.

Record false positives, false negatives, and uncertain cases. Those labels are
needed before treating probabilities as confidence.

Do not include secrets or unapproved private source in the diff.

## Establishing accuracy

The labeled benchmark separates the mechanism from the claim that the model is
reliable:

```sh
./examples/todo-fulfillment/benchmark.sh
```

Each case contains `todos.json`, `change.patch`, and an `expected` label. Add
real, human-reviewed cases before interpreting `accuracy`, false positives,
false negatives, or abstentions. Include fulfilled, rejected, and partial cases.
Run the benchmark repeatedly because model judgments can vary.
