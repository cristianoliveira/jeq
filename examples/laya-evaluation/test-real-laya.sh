#!/usr/bin/env bash
# Opt-in acceptance test against the real Laya Agent, checkpoint, Router, and HTTP server.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
readonly LAYA_SOURCE_REV=1addbb9ab8ffcd5b72a82d5158875f981384b7b8
readonly LAYA_CHECKPOINT_REV=5e7b2b1b8ca2ecdd3f2322d94069c9b6ce7e844b
readonly LAYA_WEIGHTS_SHA256=891102d372688fc2a094dac56a384bc537b87c63f21f9f3dac0be2b7cbc8d86c
readonly LAYA_CONFIG_SHA256=ae287b56bbcf5f8c4f4541ae9dfd00c914c4c48b940b8398c3058af37ba92bbd
readonly LAYA_ENCODER_SHA256=bf3ab80598fdccf414855a2ce80f22859e4492d06ca8a62ddd1cfb63972f8979
readonly LAYA_TOKENIZER_SHA256=6c8aaa9a542084f2457eab775d4eeb51f92a70c0fd9de28d5edb0ddec3c08d30
LAYA_PORT=${LAYA_PORT:-18787}

if [[ ${JEQ_REAL_LAYA_CONFIRM:-} != 1 ]]; then
  printf 'set JEQ_REAL_LAYA_CONFIRM=1 to load the real local checkpoint and run 19 judgments\n' >&2
  exit 2
fi
: "${LAYA_CHECKOUT:?set LAYA_CHECKOUT to the pinned Laya source checkout}"
: "${LAYA_MODEL_PATH:?set LAYA_MODEL_PATH to the pinned English checkpoint snapshot}"

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/jeq-real-laya.XXXXXX")
JEQ_BIN=$TMP_DIR/jeq
launcher_pid=''
server_pid=''

is_descendant() {
  local current=$1
  local ancestor=$2
  while [[ "$current" =~ ^[0-9]+$ ]] && (( current > 1 )); do
    [[ "$current" == "$ancestor" ]] && return 0
    current=$(ps -o ppid= -p "$current" 2>/dev/null | tr -d ' ')
  done
  return 1
}

kill_tree() {
  local parent=$1
  local child
  for child in $(pgrep -P "$parent" 2>/dev/null || true); do
    kill_tree "$child"
  done
  kill "$parent" 2>/dev/null || true
}

stop_server() {
  [[ -n "$launcher_pid" ]] && kill_tree "$launcher_pid"
  [[ -n "$server_pid" ]] && kill "$server_pid" 2>/dev/null || true

  for _ in $(seq 1 50); do
    if ! lsof -t -iTCP:"$LAYA_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
      [[ -n "$launcher_pid" ]] && wait "$launcher_pid" 2>/dev/null || true
      server_pid=''
      launcher_pid=''
      return 0
    fi
    sleep 0.1
  done

  [[ -n "$server_pid" ]] && kill -KILL "$server_pid" 2>/dev/null || true
  [[ -n "$launcher_pid" ]] && kill -KILL "$launcher_pid" 2>/dev/null || true
  return 1
}

cleanup() {
  local status=$?
  trap - EXIT
  if ! stop_server >/dev/null 2>&1; then
    printf 'real Laya listener remained after cleanup\n' >&2
    status=1
  fi
  rm -rf "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT

assert_sha256() {
  local relative_path=$1
  local expected=$2
  local file=$LAYA_MODEL_PATH/$relative_path
  if [[ ! -f "$file" ]] || [[ $(shasum -a 256 "$file" | awk '{print $1}') != "$expected" ]]; then
    printf 'checkpoint file failed SHA-256 verification: %s\n' "$relative_path" >&2
    exit 2
  fi
}

if [[ $(git -C "$LAYA_CHECKOUT" rev-parse HEAD) != "$LAYA_SOURCE_REV" ]]; then
  printf 'Laya source is not pinned at %s\n' "$LAYA_SOURCE_REV" >&2
  exit 2
fi
if [[ -n $(git -C "$LAYA_CHECKOUT" status --porcelain=v1 --untracked-files=all) ]]; then
  printf 'Laya source checkout must be clean before the real acceptance test\n' >&2
  exit 2
fi
if [[ $(basename "$LAYA_MODEL_PATH") != "$LAYA_CHECKPOINT_REV" ]]; then
  printf 'Laya checkpoint path is not the pinned English snapshot %s\n' "$LAYA_CHECKPOINT_REV" >&2
  exit 2
fi
assert_sha256 model.safetensors "$LAYA_WEIGHTS_SHA256"
assert_sha256 rl_agent_config.json "$LAYA_CONFIG_SHA256"
assert_sha256 encoder/config.json "$LAYA_ENCODER_SHA256"
assert_sha256 tokenizer/tokenizer.json "$LAYA_TOKENIZER_SHA256"

if [[ ! "$LAYA_PORT" =~ ^[0-9]+$ ]] || (( LAYA_PORT < 1024 || LAYA_PORT > 65535 )); then
  printf 'LAYA_PORT must be an unprivileged TCP port\n' >&2
  exit 2
fi
if lsof -t -iTCP:"$LAYA_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  printf 'loopback test port %s is already in use\n' "$LAYA_PORT" >&2
  exit 2
fi

(cd "$REPO_ROOT" && go build -o "$JEQ_BIN" ./cmd/jeq)

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
  candidate=$(lsof -t -iTCP:"$LAYA_PORT" -sTCP:LISTEN 2>/dev/null | head -1 || true)
  if [[ -n "$candidate" ]]; then
    if ! kill -0 "$launcher_pid" 2>/dev/null || ! is_descendant "$candidate" "$launcher_pid"; then
      printf 'loopback listener does not belong to the spawned Laya process\n' >&2
      exit 1
    fi
    server_pid=$candidate
    if curl --connect-timeout 1 --max-time 2 -fsS "http://127.0.0.1:$LAYA_PORT/health" >"$TMP_DIR/health.json"; then
      break
    fi
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

if ! stop_server; then
  printf 'real Laya listener remained after cleanup\n' >&2
  exit 1
fi

jq '{provider,requested_model,cases,quality_cases,transport_failures,overall_acceptance_rate,useful_answers,useful_answers_per_input_token,latency_ms,usage}' "$TMP_DIR/result.json"
printf 'PASS real Laya acceptance at source %s checkpoint %s\n' "$LAYA_SOURCE_REV" "$LAYA_CHECKPOINT_REV" >&2
