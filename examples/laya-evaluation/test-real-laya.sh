#!/usr/bin/env bash
# Opt-in acceptance test against the real Laya Agent, checkpoint, Router, and HTTP server.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
LAYA_SOURCE_REV=${LAYA_SOURCE_REV:-1addbb9ab8ffcd5b72a82d5158875f981384b7b8}
LAYA_CHECKPOINT_REV=${LAYA_CHECKPOINT_REV:-5e7b2b1b8ca2ecdd3f2322d94069c9b6ce7e844b}
LAYA_PORT=${LAYA_PORT:-18787}
JEQ_BIN=${JEQ_BIN:-$REPO_ROOT/.tmp/laya-real-acceptance/jeq}

if [[ ${JEQ_REAL_LAYA_CONFIRM:-} != 1 ]]; then
  printf 'set JEQ_REAL_LAYA_CONFIRM=1 to load the real local checkpoint and run 19 judgments\n' >&2
  exit 2
fi
: "${LAYA_CHECKOUT:?set LAYA_CHECKOUT to the pinned Laya source checkout}"
: "${LAYA_MODEL_PATH:?set LAYA_MODEL_PATH to the pinned English checkpoint snapshot}"

if [[ $(git -C "$LAYA_CHECKOUT" rev-parse HEAD) != "$LAYA_SOURCE_REV" ]]; then
  printf 'Laya source is not pinned at %s\n' "$LAYA_SOURCE_REV" >&2
  exit 2
fi
if [[ $(basename "$LAYA_MODEL_PATH") != "$LAYA_CHECKPOINT_REV" ]] || [[ ! -f "$LAYA_MODEL_PATH/model.safetensors" ]]; then
  printf 'Laya checkpoint is not the pinned English snapshot %s\n' "$LAYA_CHECKPOINT_REV" >&2
  exit 2
fi
if [[ ! "$LAYA_PORT" =~ ^[0-9]+$ ]] || (( LAYA_PORT < 1024 || LAYA_PORT > 65535 )); then
  printf 'LAYA_PORT must be an unprivileged TCP port\n' >&2
  exit 2
fi
if lsof -t -iTCP:"$LAYA_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  printf 'loopback test port %s is already in use\n' "$LAYA_PORT" >&2
  exit 2
fi

mkdir -p "$(dirname "$JEQ_BIN")"
if [[ ! -x "$JEQ_BIN" ]]; then
  (cd "$REPO_ROOT" && go build -o "$JEQ_BIN" ./cmd/jeq)
fi

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-real-laya.XXXXXX")
launcher_pid=''
server_pid=''
cleanup() {
  [[ -n "$server_pid" ]] && kill "$server_pid" 2>/dev/null || true
  [[ -n "$launcher_pid" ]] && kill "$launcher_pid" 2>/dev/null || true
  [[ -n "$launcher_pid" ]] && wait "$launcher_pid" 2>/dev/null || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

(
  cd "$LAYA_CHECKOUT"
  exec env \
    LAYA_MODEL_PATH="$LAYA_MODEL_PATH" \
    LAYA_PORT="$LAYA_PORT" \
    LAYA_DEVICE=cpu \
    LAYA_THREADS=4 \
    HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    HTTP_PROXY=http://127.0.0.1:9 \
    HTTPS_PROXY=http://127.0.0.1:9 \
    ALL_PROXY=http://127.0.0.1:9 \
    http_proxy=http://127.0.0.1:9 \
    https_proxy=http://127.0.0.1:9 \
    all_proxy=http://127.0.0.1:9 \
    NO_PROXY=127.0.0.1,localhost \
    no_proxy=127.0.0.1,localhost \
    PYTHONDONTWRITEBYTECODE=1 \
    nix develop -c python "$SCRIPT_DIR/real_laya_server.py"
) >"$TMP_DIR/server.log" 2>&1 &
launcher_pid=$!

for _ in $(seq 1 180); do
  server_pid=$(lsof -t -iTCP:"$LAYA_PORT" -sTCP:LISTEN 2>/dev/null | head -1 || true)
  if [[ -n "$server_pid" ]] && curl -fsS "http://127.0.0.1:$LAYA_PORT/health" >"$TMP_DIR/health.json"; then
    break
  fi
  if ! kill -0 "$launcher_pid" 2>/dev/null; then
    cat "$TMP_DIR/server.log" >&2
    printf 'real Laya server exited before readiness\n' >&2
    exit 1
  fi
  sleep 1
done

if [[ -z "$server_pid" ]] || ! jq -e '.status == "ok" and .device == "cpu" and (.loaded | index("english") != null)' "$TMP_DIR/health.json" >/dev/null; then
  cat "$TMP_DIR/server.log" >&2
  printf 'real Laya server did not become ready with the English checkpoint\n' >&2
  exit 1
fi

JEQ_PROVIDER=custom \
JEQ_BASE_URL="http://127.0.0.1:$LAYA_PORT" \
JEQ_AUTH=none \
JEQ_EVAL_MODEL=english \
JEQ_EVAL_CONFIRM=1 \
JEQ_BIN="$JEQ_BIN" \
  "$SCRIPT_DIR/run.sh" >"$TMP_DIR/result.json" 2>"$TMP_DIR/evaluation.log"

jq -e '
  .provider == "custom" and .requested_model == "english" and
  .cases == 19 and .quality_cases == 19 and .transport_failures == 0 and
  .invalid_responses == 0 and .unscorable_responses == 0 and
  .probability_metrics_unscorable == 0 and .usage.complete == true and
  (.items | all(.model == "laya-rl-agent")) and
  ([.items[].primitive] | unique) == ["choice","noul","rank","score"]
' "$TMP_DIR/result.json" >/dev/null

jq '{provider,requested_model,cases,quality_cases,transport_failures,overall_acceptance_rate,useful_answers,useful_answers_per_input_token,latency_ms,usage}' "$TMP_DIR/result.json"
printf 'PASS real Laya acceptance at source %s checkpoint %s\n' "$LAYA_SOURCE_REV" "$LAYA_CHECKPOINT_REV" >&2
