#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

BASE_DIR="/root/ecommerce_ai"
ENV_FILE=""
PORT=""
BINARY_PATH=""
PID_FILE=""
LOG_FILE=""

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --binary-path)
      BINARY_PATH="$2"
      shift 2
      ;;
    --pid-file)
      PID_FILE="$2"
      shift 2
      ;;
    --log-file)
      LOG_FILE="$2"
      shift 2
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

ENV_FILE="${ENV_FILE:-$BASE_DIR/shared/main.env}"
BINARY_PATH="${BINARY_PATH:-$BASE_DIR/ecommerce-api}"
PID_FILE="${PID_FILE:-$BASE_DIR/run/ecommerce-api.pid}"
RUN_WITH_ENV_SCRIPT="$DEPLOY_SCRIPT_DIR/run-with-env.sh"
[ -f "$ENV_FILE" ] || fail "Runtime env file not found: $ENV_FILE"
[ -x "$BINARY_PATH" ] || fail "Main binary not found: $BINARY_PATH"
[ -x "$RUN_WITH_ENV_SCRIPT" ] || fail "Runner helper not found: $RUN_WITH_ENV_SCRIPT"

mkdir -p "$BASE_DIR/logs" "$(dirname "$PID_FILE")"
PORT="${PORT:-$(read_main_port_from_env "$ENV_FILE" 8080)}"
LOG_FILE="${LOG_FILE:-$BASE_DIR/logs/ecommerce-api-$(date -u +%Y%m%dT%H%M%SZ).log}"
mkdir -p "$(dirname "$LOG_FILE")"

[[ "$PORT" =~ ^[0-9]+$ ]] && [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ] || fail "Invalid MAIN port: $PORT"
STARTUP_TIMEOUT_SECONDS="${START_MAIN_TIMEOUT_SECONDS:-30}"
[[ "$STARTUP_TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]] || fail "START_MAIN_TIMEOUT_SECONDS must be a positive integer."
command -v curl >/dev/null 2>&1 || fail "curl is required for the MAIN startup health gate."
if ! command -v ss >/dev/null 2>&1 && ! command -v lsof >/dev/null 2>&1; then
  fail "ss or lsof is required to verify MAIN listener ownership."
fi

listener_pids_for_port() {
  local port="$1"
  local pids=""

  if command -v ss >/dev/null 2>&1; then
    pids="$(ss -H -ltnp "sport = :$port" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u || true)"
    if [ -n "$pids" ]; then
      printf '%s\n' "$pids"
      return 0
    fi
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null | sort -u || true
  fi
}

port_is_listening() {
  local port="$1"

  if command -v ss >/dev/null 2>&1; then
    ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null | grep -q .
    return
  fi
  tcp_ready 127.0.0.1 "$port"
}

health_code() {
  local port="$1"
  local code="000"

  if command -v curl >/dev/null 2>&1; then
    code="$(curl --max-time 2 -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/health" 2>/dev/null || true)"
  fi
  [[ "$code" =~ ^[0-9]{3}$ ]] || code="000"
  printf '%s\n' "$code"
}

pid_file_value() {
  [ -f "$PID_FILE" ] || return 0
  tr -d '[:space:]' <"$PID_FILE"
}

cleanup_started_process() {
  local attempt
  local recorded_pid=""

  if kill -0 "$PID" >/dev/null 2>&1; then
    kill "$PID" >/dev/null 2>&1 || true
    for attempt in $(seq 1 20); do
      kill -0 "$PID" >/dev/null 2>&1 || break
      sleep 0.25
    done
    if kill -0 "$PID" >/dev/null 2>&1; then
      kill -9 "$PID" >/dev/null 2>&1 || true
    fi
  fi
  recorded_pid="$(pid_file_value || true)"
  if [ "$recorded_pid" = "$PID" ]; then
    rm -f "$PID_FILE"
  fi
}

startup_fail() {
  local message="$1"
  cleanup_started_process
  fail "$message Check $LOG_FILE"
}

if port_is_listening "$PORT"; then
  EXISTING_LISTENER_PIDS="$(listener_pids_for_port "$PORT" | xargs 2>/dev/null || true)"
  fail "MAIN port $PORT is already listening before start (listener_pids=${EXISTING_LISTENER_PIDS:-unknown})."
fi

if [ -f "$PID_FILE" ]; then
  EXISTING_PID="$(pid_file_value || true)"
  if [[ "$EXISTING_PID" =~ ^[1-9][0-9]*$ ]] && kill -0 "$EXISTING_PID" >/dev/null 2>&1; then
    fail "PID file $PID_FILE already references running pid $EXISTING_PID."
  fi
  rm -f "$PID_FILE"
fi

nohup "$RUN_WITH_ENV_SCRIPT" "$ENV_FILE" "$BINARY_PATH" >"$LOG_FILE" 2>&1 &
PID=$!
echo "$PID" >"$PID_FILE"
DEADLINE=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
LISTENER_PIDS=""
HEALTH_CODE="000"
LISTENER_OWNED="false"

while [ "$SECONDS" -lt "$DEADLINE" ]; do
  kill -0 "$PID" >/dev/null 2>&1 || startup_fail "ecommerce-api exited before becoming ready."

  if port_is_listening "$PORT"; then
    LISTENER_PIDS="$(listener_pids_for_port "$PORT")"
    if printf '%s\n' "$LISTENER_PIDS" | grep -Fxq "$PID"; then
      LISTENER_OWNED="true"
      HEALTH_CODE="$(health_code "$PORT")"
      if [ "$HEALTH_CODE" = "200" ]; then
        printf 'STARTED=true\nPID=%s\nPID_FILE=%s\nLOG_FILE=%s\nPORT=%s\nBINARY_PATH=%s\nTCP_READY=true\nLISTENER_OWNED=true\nHEALTH_CODE=%s\n' \
          "$PID" "$PID_FILE" "$LOG_FILE" "$PORT" "$BINARY_PATH" "$HEALTH_CODE"
        exit 0
      fi
    else
      startup_fail "MAIN port $PORT is listening, but it is not owned by launched pid $PID (listener_pids=$(printf '%s' "$LISTENER_PIDS" | xargs 2>/dev/null || printf unknown))."
    fi
  fi
  sleep 1
done

startup_fail "ecommerce-api did not become ready within ${STARTUP_TIMEOUT_SECONDS}s (listener_owned=$LISTENER_OWNED health_code=$HEALTH_CODE)."
