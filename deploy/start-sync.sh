#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

BASE_DIR="/root/ecommerce_ai"
SYNC_PORT=8082

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

BINARY="$BASE_DIR/erp_bridge_sync"
BRIDGE_ENV="$BASE_DIR/shared/bridge.env"
DOT_ENV="$BASE_DIR/.env"
SYNC_ENV="$BASE_DIR/shared/sync.env"
MERGED_ENV="$(mktemp)"
LOG_FILE="$BASE_DIR/logs/erp_sync-$(date -u +%Y%m%dT%H%M%SZ).log"

trap 'rm -f "$MERGED_ENV"' EXIT

[ -x "$BINARY" ] || fail "Sync binary not found: $BINARY"

mkdir -p "$BASE_DIR/logs" "$BASE_DIR/run"

# DB settings come from bridge.env. JST credentials come from .env or sync.env.
if [ -f "$BRIDGE_ENV" ]; then
  awk '/^(DB_|MYSQL_)/ { print }' "$BRIDGE_ENV" >>"$MERGED_ENV"
fi
if [ -f "$DOT_ENV" ]; then
  awk '/^JST_/ { print }' "$DOT_ENV" >>"$MERGED_ENV"
elif [ -f "$SYNC_ENV" ]; then
  awk '/^JST_/ { print }' "$SYNC_ENV" >>"$MERGED_ENV"
fi

{
  printf 'PORT=%s\n' "$SYNC_PORT"
  printf 'SERVER_PORT=%s\n' "$SYNC_PORT"
  printf 'LISTEN_HOST=127.0.0.1\n'
} >>"$MERGED_ENV"

nohup "$BASE_DIR/scripts/run-with-env.sh" "$MERGED_ENV" "$BINARY" >"$LOG_FILE" 2>&1 &
PID=$!
echo "$PID" >"$BASE_DIR/run/erp_sync.pid"
sleep 3
kill -0 "$PID" >/dev/null 2>&1 || fail "erp_bridge_sync exited immediately. Check $LOG_FILE"

if tcp_ready 127.0.0.1 "$SYNC_PORT"; then
  TCP_READY="true"
else
  TCP_READY="false"
fi

printf 'STARTED=true\nPID=%s\nLOG_FILE=%s\nPORT=%s\nTCP_READY=%s\n' "$PID" "$LOG_FILE" "$SYNC_PORT" "$TCP_READY"
