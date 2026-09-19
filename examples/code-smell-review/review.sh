#!/usr/bin/env bash
set -euo pipefail

GEV_BIN=${GEV_BIN:-gev}
JQ_BIN=${JQ_BIN:-jq}
MAX_FILES=20
MAX_BYTES=$((256 * 1024))

if ! command -v "$GEV_BIN" >/dev/null 2>&1; then
	printf 'code-smell-review: GEV_BIN is unavailable: %s\n' "$GEV_BIN" >&2
	exit 127
fi
if ! command -v "$JQ_BIN" >/dev/null 2>&1; then
	printf 'code-smell-review: JQ_BIN is unavailable: %s\n' "$JQ_BIN" >&2
	exit 127
fi
if (($# < 1 || $# > MAX_FILES)); then
	printf 'usage: %s <1..20 readable regular files>\n' "$0" >&2
	exit 2
fi

tmp=$(mktemp "${TMPDIR:-/tmp}/gev-code-smells.XXXXXX")
trap 'rm -f "$tmp"' EXIT

for path in "$@"; do
	if [[ ! -f "$path" || ! -r "$path" ]]; then
		printf 'code-smell-review: not a readable regular file: %s\n' "$path" >&2
		exit 2
	fi
	bytes=$(wc -c <"$path")
	bytes=${bytes//[[:space:]]/}
	if ((bytes > MAX_BYTES)); then
		printf 'code-smell-review: file exceeds 256 KiB: %s\n' "$path" >&2
		exit 2
	fi
	"$JQ_BIN" -c -n --arg path "$path" --rawfile content "$path" \
		'{path: $path, content: $content}' >>"$tmp"
done

questions_json='{"questions":{"primary_smell":{"type":"choice","instructions":"Which single design smell is most material in this reviewed set?","criteria":{"none":"No material smell beyond routine maintenance","mixed_responsibilities":"One unit owns unrelated reasons to change","duplicated_policy":"The same policy is duplicated across locations","hidden_ambient_state":"Behavior depends on implicit global or ambient state","leaky_abstraction":"An abstraction exposes details callers must know","unnecessary_complexity":"The design is more complex than its stated behavior requires"}},"cohesive":{"type":"noul","instructions":"Is this reviewed set cohesive enough to pass an optional policy gate? Answer high=yes and safe to pass; answer low=no or not safe to pass."}}}'

exec "$GEV_BIN" reduce --as code_smells --input ndjson --questions-json "$questions_json" <"$tmp"
