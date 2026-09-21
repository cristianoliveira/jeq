#!/usr/bin/env bash
set -euo pipefail

MAX_FILES=20
MAX_BYTES=$((256 * 1024))
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if ! command -v jeq >/dev/null 2>&1; then
	printf 'unix-review-pipeline: jeq is unavailable on PATH: %s\n' jeq >&2
	exit 127
fi
if ! command -v jq >/dev/null 2>&1; then
	printf 'unix-review-pipeline: jq is unavailable on PATH: %s\n' jq >&2
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
		jq -c -n --arg path "$path" --rawfile content "$path" \
			'{file: {path: $path, content: $content}}'
	done
}

# Keep each stage visible: emit records -> local map -> shape -> aggregate reduce
# -> offline gate -> source-free final projection.
emit_files "$@" |
	jeq map --as local_focus --questions "$SCRIPT_DIR/local-questions.json" --state-pointer /file --input ndjson |
	jq -c '{file: .file, local_focus: ._jeq.local_focus.answers.local_focus.noul}' |
	jeq reduce --as aggregate_focus --questions "$SCRIPT_DIR/change-questions.json" --input ndjson |
	jeq gate --as focus_policy --value-pointer /_jeq/aggregate_focus/answers/aggregate_focus/noul --pass-min 0.80 --reject-max 0.40 |
	jq -c 'del(.items)'
