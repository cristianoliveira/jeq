# Incident triage: catalog lookup, native request, map, gate

The first map classifies an incident. `jq` validates that the model answer is a
catalog key, looks up a fixed runbook question, and writes a complete native
request into a new record field. The second map evaluates that request. Gate
then checks its numeric confidence offline.

```sh
GEV_BIN=${GEV_BIN:-gev}

"$GEV_BIN" map --as category --questions category-questions.json \
    --state-pointer /incident |
  jq --argfile catalog runbook-catalog.json '
    .request = {
      "model": "jev-latest",
      "state": .incident,
      "questions": {
        "runbook": {
          "type": "choice",
          "instructions": ("Select the approved runbook for category " +
            (.category // "unknown")),
          "criteria": ($catalog[._gev.category.answers.category.choice] // {})
        }
      }
    }
    | select((.request.questions.runbook.criteria | length) > 0)
  ' |
  "$GEV_BIN" map --as runbook --request-pointer /request |
  "$GEV_BIN" gate --as incident_policy \
    --value-pointer /_gev/runbook/answers/runbook/confidence \
    --pass-min 0.80 --reject-max 0.40
```

The catalog lookup is ordinary `jq` data processing. A response is accepted
only as a key in that fixed catalog; it is never evaluated as shell, a path, or
an executable action. The full incident remains in the envelope, so project it
before logs:

```sh
... | jq 'del(.incident, .request, ._gev.category, ._gev.runbook)'
```

Fixture: [`fixtures/incident.json`](fixtures/incident.json), catalog:
[`runbook-catalog.json`](runbook-catalog.json).
