#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

BASE_DIR="/root/ecommerce_ai"
ENV_FILE=""
PORT=""

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
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

ENV_FILE="${ENV_FILE:-$BASE_DIR/shared/bridge.env}"
[ -f "$ENV_FILE" ] || fail "Bridge env file not found: $ENV_FILE"
[ -x "$BASE_DIR/erp_bridge" ] || fail "Bridge binary not found: $BASE_DIR/erp_bridge"

mkdir -p "$BASE_DIR/logs" "$BASE_DIR/run"
PORT="${PORT:-$(read_env_value "$ENV_FILE" SERVER_PORT 8081)}"
LOG_FILE="$BASE_DIR/logs/erp_bridge-$(date -u +%Y%m%dT%H%M%SZ).log"

nohup "$BASE_DIR/scripts/run-with-env.sh" "$ENV_FILE" "$BASE_DIR/erp_bridge" >"$LOG_FILE" 2>&1 &
PID=$!
echo "$PID" >"$BASE_DIR/run/erp_bridge.pid"
sleep 3
kill -0 "$PID" >/dev/null 2>&1 || fail "erp_bridge exited immediately. Check $LOG_FILE"

if tcp_ready 127.0.0.1 "$PORT"; then
  TCP_READY="true"
else
  TCP_READY="false"
fi

printf 'STARTED=true\nPID=%s\nLOG_FILE=%s\nPORT=%s\nTCP_READY=%s\n' "$PID" "$LOG_FILE" "$PORT" "$TCP_READY"
