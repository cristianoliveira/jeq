#!/usr/bin/env bash
set -euo pipefail

JEQ_BIN=${JEQ_BIN:-jeq}
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
questions=$(<"$root/questions.json")
exec "$JEQ_BIN" reduce --as release_ready --input ndjson --questions-json "$questions" <"$root/findings.ndjson"
