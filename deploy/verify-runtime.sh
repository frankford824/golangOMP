#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

BASE_DIR="/root/ecommerce_ai"
BASE_URL="http://127.0.0.1:8080"
BRIDGE_URL="http://127.0.0.1:8081"
SYNC_URL="http://127.0.0.1:8082"
USERNAME=""
PASSWORD=""
BEARER_TOKEN=""
AUTO_RECOVER_8082="false"
CHECK_THREE_SERVICE="true"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    --base-url)
      BASE_URL="$2"
      shift 2
      ;;
    --bridge-url)
      BRIDGE_URL="$2"
      shift 2
      ;;
    --sync-url)
      SYNC_URL="$2"
      shift 2
      ;;
    --username)
      USERNAME="$2"
      shift 2
      ;;
    --password)
      PASSWORD="$2"
      shift 2
      ;;
    --bearer-token)
      BEARER_TOKEN="$2"
      shift 2
      ;;
    --auto-recover-8082)
      AUTO_RECOVER_8082="true"
      shift
      ;;
    --skip-three-service-check)
      CHECK_THREE_SERVICE="false"
      shift
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

parse_host() {
  local url="$1"
  local without_scheme="${url#*://}"
  local host_port="${without_scheme%%/*}"
  printf '%s\n' "${host_port%%:*}"
}

parse_port() {
  local url="$1"
  local without_scheme="${url#*://}"
  local host_port="${without_scheme%%/*}"
  if [[ "$host_port" == *:* ]]; then
    printf '%s\n' "${host_port##*:}"
    return
  fi
  if [[ "$url" == https://* ]]; then
    printf '443\n'
    return
  fi
  printf '80\n'
}

BASE_HOST="$(parse_host "$BASE_URL")"
BASE_PORT="$(parse_port "$BASE_URL")"
if tcp_ready "$BASE_HOST" "$BASE_PORT"; then
  printf 'TCP_READY=true\n'
else
  printf 'TCP_READY=false\n'
fi

if [ -n "$BRIDGE_URL" ]; then
  BRIDGE_HOST="$(parse_host "$BRIDGE_URL")"
  BRIDGE_PORT="$(parse_port "$BRIDGE_URL")"
  if tcp_ready "$BRIDGE_HOST" "$BRIDGE_PORT"; then
    printf 'BRIDGE_TCP_READY=true\n'
  else
    printf 'BRIDGE_TCP_READY=false\n'
  fi
fi

if command -v curl >/dev/null 2>&1; then
  HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' "$BASE_URL/v1/auth/me" || true)"
  printf 'AUTH_ME_HTTP_CODE=%s\n' "${HTTP_CODE:-000}"
  if [ -n "$BEARER_TOKEN" ]; then
    HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $BEARER_TOKEN" "$BASE_URL/v1/tasks?page=1&page_size=1" || true)"
    printf 'TASK_LIST_HTTP_CODE=%s\n' "${HTTP_CODE:-000}"
  elif [ -n "$USERNAME" ] && [ -n "$PASSWORD" ]; then
    HTTP_CODE="$(curl -sS -o /dev/null -w '%{http_code}' -H "Content-Type: application/json" -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" "$BASE_URL/v1/auth/login" || true)"
    printf 'LOGIN_HTTP_CODE=%s\n' "${HTTP_CODE:-000}"
  fi
else
  printf 'HTTP_CHECKS=skipped_no_curl\n'
fi

if [ "$CHECK_THREE_SERVICE" = "true" ] && [ -x "$DEPLOY_SCRIPT_DIR/check-three-services.sh" ]; then
  THREE_SERVICE_ARGS=(
    --base-dir "$BASE_DIR"
    --main-url "$BASE_URL"
    --bridge-url "$BRIDGE_URL"
    --sync-url "$SYNC_URL"
  )
  if [ "$AUTO_RECOVER_8082" = "true" ]; then
    THREE_SERVICE_ARGS+=(--auto-recover-8082)
  else
    THREE_SERVICE_ARGS+=(--no-auto-recover-8082)
  fi
  bash "$DEPLOY_SCRIPT_DIR/check-three-services.sh" "${THREE_SERVICE_ARGS[@]}"
fi
