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

nohup "$RUN_WITH_ENV_SCRIPT" "$ENV_FILE" "$BINARY_PATH" >"$LOG_FILE" 2>&1 &
PID=$!
echo "$PID" >"$PID_FILE"
sleep 3
kill -0 "$PID" >/dev/null 2>&1 || fail "ecommerce-api exited immediately. Check $LOG_FILE"

if tcp_ready 127.0.0.1 "$PORT"; then
  TCP_READY="true"
else
  TCP_READY="false"
fi

printf 'STARTED=true\nPID=%s\nPID_FILE=%s\nLOG_FILE=%s\nPORT=%s\nBINARY_PATH=%s\nTCP_READY=%s\n' "$PID" "$PID_FILE" "$LOG_FILE" "$PORT" "$BINARY_PATH" "$TCP_READY"
