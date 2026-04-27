#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="/root/ecommerce_ai"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      exit 1
      ;;
  esac
done

PID_FILE="$BASE_DIR/run/erp_sync.pid"
STOPPED=""

if [ -f "$PID_FILE" ]; then
  PID="$(tr -d '[:space:]' <"$PID_FILE")"
  if [ -n "$PID" ] && kill -0 "$PID" >/dev/null 2>&1; then
    kill "$PID" >/dev/null 2>&1 || true
    sleep 2
    if kill -0 "$PID" >/dev/null 2>&1; then
      kill -9 "$PID" >/dev/null 2>&1 || true
    fi
    STOPPED="$PID"
  fi
  rm -f "$PID_FILE"
fi

if command -v pgrep >/dev/null 2>&1; then
  while IFS= read -r pid; do
    [ -n "$pid" ] || continue
    kill "$pid" >/dev/null 2>&1 || true
    STOPPED="$STOPPED $pid"
  done < <(pgrep -f "$BASE_DIR/erp_bridge_sync" || true)
fi

printf 'STOPPED_PIDS=%s\n' "$(printf '%s' "$STOPPED" | xargs 2>/dev/null || true)"
