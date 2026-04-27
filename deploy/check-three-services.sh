#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

BASE_DIR="/root/ecommerce_ai"
MAIN_URL="http://127.0.0.1:8080"
BRIDGE_URL="http://127.0.0.1:8081"
SYNC_URL="http://127.0.0.1:8082"
AUTO_RECOVER_8082="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    --main-url)
      MAIN_URL="$2"
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
    --auto-recover-8082)
      AUTO_RECOVER_8082="true"
      shift
      ;;
    --no-auto-recover-8082)
      AUTO_RECOVER_8082="false"
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

health_code() {
  local base_url="$1"
  if ! command -v curl >/dev/null 2>&1; then
    printf '000\n'
    return
  fi
  curl -sS -o /dev/null -w '%{http_code}' "${base_url%/}/health" 2>/dev/null || printf '000'
}

pid_from_file() {
  local pid_file="$1"
  [ -f "$pid_file" ] || return 0
  tr -d '[:space:]' <"$pid_file"
}

pid_exists() {
  local pid="$1"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

listener_line_for_port() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp 2>/dev/null | awk -v needle=":$port" '
      NR > 1 && index($4, needle) {
        print
        found = 1
      }
      END { exit found ? 0 : 1 }
    '
    return
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -ltnp 2>/dev/null | awk -v needle=":$port" '
      index($4, needle) {
        print
        found = 1
      }
      END { exit found ? 0 : 1 }
    '
    return
  fi
  return 1
}

gather_service() {
  local prefix="$1"
  local name="$2"
  local base_url="$3"
  local pid_file="$4"
  local port="$5"
  local pid
  local pid_exists_value="false"
  local health
  local listener_line=""
  local tcp_listening="false"
  local exe_path=""
  local exe_deleted="false"
  local status="degraded"

  pid="$(pid_from_file "$pid_file" || true)"
  if pid_exists "$pid"; then
    pid_exists_value="true"
    exe_path="$(readlink "/proc/$pid/exe" 2>/dev/null || true)"
    case "$exe_path" in
      *"(deleted)"*)
        exe_deleted="true"
        ;;
    esac
  fi

  health="$(health_code "$base_url")"
  if listener_line="$(listener_line_for_port "$port" 2>/dev/null)"; then
    tcp_listening="true"
  fi

  if [ "$health" = "200" ] && [ "$pid_exists_value" = "true" ] && [ "$tcp_listening" = "true" ] && [ "$exe_deleted" != "true" ]; then
    status="ok"
  fi

  printf -v "${prefix}_NAME" '%s' "$name"
  printf -v "${prefix}_URL" '%s' "$base_url"
  printf -v "${prefix}_PORT" '%s' "$port"
  printf -v "${prefix}_PID_FILE" '%s' "$pid_file"
  printf -v "${prefix}_PID" '%s' "$pid"
  printf -v "${prefix}_PID_EXISTS" '%s' "$pid_exists_value"
  printf -v "${prefix}_HEALTH" '%s' "$health"
  printf -v "${prefix}_TCP_LISTENING" '%s' "$tcp_listening"
  printf -v "${prefix}_LISTENER_LINE" '%s' "$listener_line"
  printf -v "${prefix}_EXE_PATH" '%s' "$exe_path"
  printf -v "${prefix}_EXE_DELETED" '%s' "$exe_deleted"
  printf -v "${prefix}_STATUS" '%s' "$status"
}

print_service_human() {
  local prefix="$1"
  local name_var="${prefix}_NAME"
  local port_var="${prefix}_PORT"
  local pid_var="${prefix}_PID"
  local pid_exists_var="${prefix}_PID_EXISTS"
  local health_var="${prefix}_HEALTH"
  local tcp_var="${prefix}_TCP_LISTENING"
  local deleted_var="${prefix}_EXE_DELETED"
  local status_var="${prefix}_STATUS"
  printf '[%s:%s] status=%s health=%s pid=%s pid_exists=%s tcp_listening=%s exe_deleted=%s\n' \
    "${!name_var}" "${!port_var}" "${!status_var}" "${!health_var}" "${!pid_var}" "${!pid_exists_var}" "${!tcp_var}" "${!deleted_var}"
}

print_service_kv() {
  local prefix="$1"
  local upper_prefix="${prefix^^}"
  local fields="NAME URL PORT PID_FILE PID PID_EXISTS HEALTH TCP_LISTENING LISTENER_LINE EXE_PATH EXE_DELETED STATUS"
  local field
  for field in $fields; do
    local var_name="${upper_prefix}_${field}"
    printf '%s=%s\n' "$var_name" "${!var_name}"
  done
}

service_json() {
  local prefix="$1"
  local name_var="${prefix}_NAME"
  local url_var="${prefix}_URL"
  local port_var="${prefix}_PORT"
  local pid_file_var="${prefix}_PID_FILE"
  local pid_var="${prefix}_PID"
  local pid_exists_var="${prefix}_PID_EXISTS"
  local health_var="${prefix}_HEALTH"
  local tcp_var="${prefix}_TCP_LISTENING"
  local listener_var="${prefix}_LISTENER_LINE"
  local exe_var="${prefix}_EXE_PATH"
  local deleted_var="${prefix}_EXE_DELETED"
  local status_var="${prefix}_STATUS"
  printf '{"name":"%s","url":"%s","port":"%s","pid_file":"%s","pid":"%s","pid_exists":%s,"health_code":"%s","tcp_listening":%s,"listener_line":"%s","exe_path":"%s","exe_deleted":%s,"status":"%s"}' \
    "$(json_escape "${!name_var}")" \
    "$(json_escape "${!url_var}")" \
    "$(json_escape "${!port_var}")" \
    "$(json_escape "${!pid_file_var}")" \
    "$(json_escape "${!pid_var}")" \
    "${!pid_exists_var}" \
    "$(json_escape "${!health_var}")" \
    "${!tcp_var}" \
    "$(json_escape "${!listener_var}")" \
    "$(json_escape "${!exe_var}")" \
    "${!deleted_var}" \
    "$(json_escape "${!status_var}")"
}

MAIN_PORT="$(parse_port "$MAIN_URL")"
BRIDGE_PORT="$(parse_port "$BRIDGE_URL")"
SYNC_PORT="$(parse_port "$SYNC_URL")"

gather_service MAIN main "$MAIN_URL" "$BASE_DIR/run/ecommerce-api.pid" "$MAIN_PORT"
gather_service BRIDGE bridge "$BRIDGE_URL" "$BASE_DIR/run/erp_bridge.pid" "$BRIDGE_PORT"
gather_service SYNC sync "$SYNC_URL" "$BASE_DIR/run/erp_sync.pid" "$SYNC_PORT"

RECOVER_TRIGGERED="false"
RECOVER_SUCCESS="false"
RECOVER_OUTPUT=""

if [ "$SYNC_STATUS" != "ok" ] && [ "$AUTO_RECOVER_8082" = "true" ]; then
  RECOVER_TRIGGERED="true"
  if [ -x "$BASE_DIR/scripts/stop-sync.sh" ]; then
    bash "$BASE_DIR/scripts/stop-sync.sh" --base-dir "$BASE_DIR" >/dev/null 2>&1 || true
  fi
  if [ -x "$BASE_DIR/scripts/start-sync.sh" ]; then
    if RECOVER_OUTPUT="$(bash "$BASE_DIR/scripts/start-sync.sh" --base-dir "$BASE_DIR" 2>&1)"; then
      sleep 2
      gather_service SYNC sync "$SYNC_URL" "$BASE_DIR/run/erp_sync.pid" "$SYNC_PORT"
      if [ "$SYNC_STATUS" = "ok" ]; then
        RECOVER_SUCCESS="true"
      fi
    else
      RECOVER_OUTPUT="${RECOVER_OUTPUT}
start-sync exited with failure"
    fi
  else
    RECOVER_OUTPUT="start-sync.sh not found under $BASE_DIR/scripts"
  fi
fi

print_service_human MAIN
print_service_human BRIDGE
print_service_human SYNC

printf 'SYNC_RECOVER_TRIGGERED=%s\n' "$RECOVER_TRIGGERED"
printf 'SYNC_RECOVER_SUCCESS=%s\n' "$RECOVER_SUCCESS"
printf 'SYNC_RECOVER_OUTPUT=%s\n' "$(sanitize_field "$RECOVER_OUTPUT")"

print_service_kv MAIN
print_service_kv BRIDGE
print_service_kv SYNC

OVERALL_OK="false"
if [ "$MAIN_STATUS" = "ok" ] && [ "$BRIDGE_STATUS" = "ok" ] && [ "$SYNC_STATUS" = "ok" ]; then
  OVERALL_OK="true"
fi
printf 'OVERALL_OK=%s\n' "$OVERALL_OK"

printf 'JSON_SUMMARY=%s\n' "$(printf '{"main":%s,"bridge":%s,"sync":%s,"sync_recover_triggered":%s,"sync_recover_success":%s,"sync_recover_output":"%s","overall_ok":%s}' \
  "$(service_json MAIN)" \
  "$(service_json BRIDGE)" \
  "$(service_json SYNC)" \
  "$RECOVER_TRIGGERED" \
  "$RECOVER_SUCCESS" \
  "$(json_escape "$RECOVER_OUTPUT")" \
  "$OVERALL_OK")"

if [ "$OVERALL_OK" != "true" ]; then
  exit 1
fi
