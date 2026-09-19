#!/usr/bin/env bash
set -euo pipefail

GEV_BIN=${GEV_BIN:-gev}
JQ_BIN=${JQ_BIN:-jq}
MAX_FILES=20
MAX_BYTES=$((256 * 1024))
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if ! command -v "$GEV_BIN" >/dev/null 2>&1; then
	printf 'unix-review-pipeline: GEV_BIN is unavailable: %s\n' "$GEV_BIN" >&2
	exit 127
fi
if ! command -v "$JQ_BIN" >/dev/null 2>&1; then
	printf 'unix-review-pipeline: JQ_BIN is unavailable: %s\n' "$JQ_BIN" >&2
	exit 127
fi
if (($# < 1 || $# > MAX_FILES)); then
	printf 'usage: %s <1..20 readable regular files>\n' "$0" >&2
	exit 2
fi

for path in "$@"; do
	if [[ ! -f "$path" || ! -r "$path" ]]; then
		printf 'unix-review-pipeline: not a readable regular file: %s\n' "$path" >&2
		exit 2
	fi
	bytes=$(wc -c <"$path")
	bytes=${bytes//[[:space:]]/}
	if ((bytes > MAX_BYTES)); then
		printf 'unix-review-pipeline: file exceeds 256 KiB: %s\n' "$path" >&2
		exit 2
	fi
done

emit_files() {
	for path in "$@"; do
		"$JQ_BIN" -c -n --arg path "$path" --rawfile content "$path" \
			'{file: {path: $path, content: $content}}'
	done
}

# Keep each stage visible: emit records -> local map -> shape -> aggregate reduce
# -> offline gate -> source-free final projection.
emit_files "$@" |
	"$GEV_BIN" map --as local_focus --questions "$SCRIPT_DIR/local-questions.json" --state-pointer /file --input ndjson |
	"$JQ_BIN" -c '{file: .file, local_focus: ._gev.local_focus.answers.local_focus.noul}' |
	"$GEV_BIN" reduce --as aggregate_focus --questions "$SCRIPT_DIR/change-questions.json" --input ndjson |
	"$GEV_BIN" gate --as focus_policy --value-pointer /_gev/aggregate_focus/answers/aggregate_focus/noul --pass-min 0.80 --reject-max 0.40 |
	"$JQ_BIN" -c 'del(.items)'
