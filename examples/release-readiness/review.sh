#!/usr/bin/env bash
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
questions=$(<"$root/questions.json")
exec jeq reduce --as release_ready --input ndjson --questions-json "$questions" <"$root/findings.ndjson"
