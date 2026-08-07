#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

PACKAGE_ROOT=""
VERSION=""
REMOTE_BASE_DIR="/root/ecommerce_ai"
RUNTIME_ENV_PATH=""
BRIDGE_ENV_PATH=""
KEEP_RELEASES="5"
START_SERVICES="false"
PARALLEL="false"
PARALLEL_PORT="18080"

while [ $# -gt 0 ]; do
  case "$1" in
    --package-root)
      PACKAGE_ROOT="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    --remote-base-dir)
      REMOTE_BASE_DIR="$2"
      shift 2
      ;;
    --runtime-env-path)
      RUNTIME_ENV_PATH="$2"
      shift 2
      ;;
    --bridge-env-path)
      BRIDGE_ENV_PATH="$2"
      shift 2
      ;;
    --keep-releases)
      KEEP_RELEASES="$2"
      shift 2
      ;;
    --start-services)
      START_SERVICES="true"
      shift
      ;;
    --parallel)
      PARALLEL="true"
      shift
      ;;
    --parallel-port)
      PARALLEL_PORT="$2"
      shift 2
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

[ -n "$PACKAGE_ROOT" ] || fail "--package-root is required."
[ -n "$VERSION" ] || fail "--version is required."
[ -d "$PACKAGE_ROOT" ] || fail "Package root not found: $PACKAGE_ROOT"

RUNTIME_ENV_PATH="${RUNTIME_ENV_PATH:-$REMOTE_BASE_DIR/shared/main.env}"
BRIDGE_ENV_PATH="${BRIDGE_ENV_PATH:-$REMOTE_BASE_DIR/shared/bridge.env}"
RELEASE_DIR="$REMOTE_BASE_DIR/releases/$VERSION"
SHARED_AVATAR_DIR="$REMOTE_BASE_DIR/shared/avatars"
PARALLEL_PID_FILE="$REMOTE_BASE_DIR/run/ecommerce-api-${VERSION}-parallel.pid"
PREVIOUS_CURRENT_TARGET="$(readlink -f "$REMOTE_BASE_DIR/current" 2>/dev/null || true)"
PREVIOUS_MAIN_LINK="$(readlink "$REMOTE_BASE_DIR/ecommerce-api" 2>/dev/null || true)"
PREVIOUS_BRIDGE_LINK="$(readlink "$REMOTE_BASE_DIR/erp_bridge" 2>/dev/null || true)"

process_is_running() {
  local pid="$1"
  local state=""

  kill -0 "$pid" >/dev/null 2>&1 || return 1
  if [ -r "/proc/$pid/stat" ]; then
    state="$(awk '{ print $3 }' "/proc/$pid/stat" 2>/dev/null || true)"
    [ "$state" != "Z" ] || return 1
  fi
  return 0
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

wait_for_pid_exit() {
  local pid="$1"
  local attempt

  for attempt in $(seq 1 20); do
    if ! process_is_running "$pid"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

wait_for_port_release() {
  local port="$1"
  local attempt

  for attempt in $(seq 1 20); do
    if ! port_is_listening "$port"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

atomic_symlink() {
  local target="$1"
  local link_path="$2"
  local next_link="${link_path}.next.$$"

  rm -f "$next_link"
  ln -s "$target" "$next_link"
  mv -Tf "$next_link" "$link_path"
}

restore_link() {
  local previous_target="$1"
  local link_path="$2"

  if [ -n "$previous_target" ]; then
    atomic_symlink "$previous_target" "$link_path"
  else
    rm -f "$link_path"
  fi
}

activate_release_links() {
  # Keep compatibility aliases pinned through current. The only release
  # cutover is therefore the final atomic rename of the current symlink.
  atomic_symlink "$REMOTE_BASE_DIR/current/ecommerce-api" "$REMOTE_BASE_DIR/ecommerce-api"
  atomic_symlink "$REMOTE_BASE_DIR/current/erp_bridge" "$REMOTE_BASE_DIR/erp_bridge"
  atomic_symlink "$RELEASE_DIR" "$REMOTE_BASE_DIR/current"
}

restore_previous_links() {
  # Restore current first so operational inspection immediately identifies the
  # rollback target, then restore the two compatibility aliases.
  restore_link "$PREVIOUS_CURRENT_TARGET" "$REMOTE_BASE_DIR/current"
  restore_link "$PREVIOUS_MAIN_LINK" "$REMOTE_BASE_DIR/ecommerce-api"
  restore_link "$PREVIOUS_BRIDGE_LINK" "$REMOTE_BASE_DIR/erp_bridge"
}

restart_previous_release() {
  [ -n "$PREVIOUS_CURRENT_TARGET" ] || return 0
  [ -x "$PREVIOUS_CURRENT_TARGET/deploy/start-main.sh" ] || return 1
  [ -x "$PREVIOUS_CURRENT_TARGET/deploy/start-bridge.sh" ] || return 1

  "$PREVIOUS_CURRENT_TARGET/deploy/start-main.sh" \
    --base-dir "$REMOTE_BASE_DIR" \
    --env-file "$RUNTIME_ENV_PATH" \
    --binary-path "$PREVIOUS_CURRENT_TARGET/ecommerce-api" >/dev/null
  "$PREVIOUS_CURRENT_TARGET/deploy/start-bridge.sh" \
    --base-dir "$REMOTE_BASE_DIR" \
    --env-file "$BRIDGE_ENV_PATH" >/dev/null
}

rollback_failed_cutover() {
  "$RELEASE_DIR/deploy/stop-main.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null 2>&1 || true
  "$RELEASE_DIR/deploy/stop-bridge.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null 2>&1 || true
  restore_previous_links
  restart_previous_release
}

stop_existing_parallel_candidate() {
  local candidate_binary="$RELEASE_DIR/ecommerce-api"
  local candidate_env="$RELEASE_DIR/runtime/main.parallel.env"
  local candidate_state="$RELEASE_DIR/runtime/deploy-state.parallel.env"
  local candidate_port=""
  local current_target=""
  local release_target=""
  local pid=""
  local expected_exe=""
  local actual_exe=""

  [ -d "$RELEASE_DIR" ] || return 0

  current_target="$(readlink -f "$REMOTE_BASE_DIR/current" 2>/dev/null || true)"
  release_target="$(readlink -f "$RELEASE_DIR" 2>/dev/null || true)"
  if [ -n "$current_target" ] && [ "$current_target" = "$release_target" ]; then
    fail "Refusing to replace currently live release: $RELEASE_DIR"
  fi

  if [ -f "$candidate_env" ]; then
    candidate_port="$(read_main_port_from_env "$candidate_env" "$PARALLEL_PORT")"
  elif [ -f "$candidate_state" ]; then
    candidate_port="$(read_env_value "$candidate_state" MAIN_PORT "$PARALLEL_PORT")"
  fi

  if [ -f "$PARALLEL_PID_FILE" ]; then
    pid="$(tr -d '[:space:]' <"$PARALLEL_PID_FILE")"
    [[ "$pid" =~ ^[1-9][0-9]*$ ]] || fail "Invalid parallel pidfile: $PARALLEL_PID_FILE"

    if process_is_running "$pid"; then
      [ -x "$candidate_binary" ] || fail "Parallel candidate binary is missing while pid $pid is running: $candidate_binary"
      expected_exe="$(readlink -f "$candidate_binary" 2>/dev/null || true)"
      actual_exe="$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)"
      [ -n "$expected_exe" ] && [ "$actual_exe" = "$expected_exe" ] ||
        fail "Parallel pidfile $PARALLEL_PID_FILE points to pid $pid owned by $actual_exe, not $expected_exe"

      kill "$pid"
      if ! wait_for_pid_exit "$pid"; then
        kill -9 "$pid" >/dev/null 2>&1 || true
        wait_for_pid_exit "$pid" || fail "Parallel candidate pid $pid did not stop."
      fi
    fi
    rm -f "$PARALLEL_PID_FILE"
  fi

  if [ -n "$candidate_port" ] && ! wait_for_port_release "$candidate_port"; then
    fail "Parallel candidate port $candidate_port is still listening; release replacement aborted."
  fi
}

mkdir -p \
  "$REMOTE_BASE_DIR/incoming" \
  "$REMOTE_BASE_DIR/releases" \
  "$REMOTE_BASE_DIR/shared" \
  "$SHARED_AVATAR_DIR" \
  "$REMOTE_BASE_DIR/logs" \
  "$REMOTE_BASE_DIR/run" \
  "$REMOTE_BASE_DIR/scripts"

stop_existing_parallel_candidate
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"
cp -R "$PACKAGE_ROOT"/. "$RELEASE_DIR/"
chmod +x "$RELEASE_DIR"/deploy/*.sh "$RELEASE_DIR/ecommerce-api" "$RELEASE_DIR/erp_bridge"

if [ "$PARALLEL" != "true" ]; then
  cp "$RELEASE_DIR"/deploy/*.sh "$REMOTE_BASE_DIR/scripts/"
  chmod +x "$REMOTE_BASE_DIR/scripts/"*.sh
fi

MAIN_ENV_CREATED="false"
BRIDGE_ENV_CREATED="false"
RESULT_STATUS="deployed"
RESULT_RUNTIME_ENV_PATH="$RUNTIME_ENV_PATH"
RESULT_PID_FILE="$REMOTE_BASE_DIR/run/ecommerce-api.pid"
RESULT_LOG_FILE=""

if [ "$PARALLEL" = "true" ]; then
  CANDIDATE_RUNTIME_DIR="$RELEASE_DIR/runtime"
  CANDIDATE_ENV_PATH="$CANDIDATE_RUNTIME_DIR/main.parallel.env"
  CANDIDATE_PID_FILE="$PARALLEL_PID_FILE"
  CANDIDATE_LOG_FILE="$REMOTE_BASE_DIR/logs/ecommerce-api-${VERSION}-parallel.log"

  mkdir -p "$CANDIDATE_RUNTIME_DIR"
  if [ -f "$RUNTIME_ENV_PATH" ]; then
    cp "$RUNTIME_ENV_PATH" "$CANDIDATE_ENV_PATH"
  else
    write_parallel_main_env_template "$CANDIDATE_ENV_PATH" "$PARALLEL_PORT" "http://127.0.0.1:8081"
    MAIN_ENV_CREATED="true"
  fi

  if main_env_uses_db_field_model "$CANDIDATE_ENV_PATH"; then
    remove_env_key "$CANDIDATE_ENV_PATH" "MYSQL_DSN"
  fi
  if ! env_has_key "$CANDIDATE_ENV_PATH" "USER_AVATAR_DIR"; then
    upsert_env_value "$CANDIDATE_ENV_PATH" "USER_AVATAR_DIR" "$SHARED_AVATAR_DIR"
  fi
  if env_has_key "$CANDIDATE_ENV_PATH" "PORT"; then
    upsert_env_value "$CANDIDATE_ENV_PATH" "PORT" "$PARALLEL_PORT"
    remove_env_key "$CANDIDATE_ENV_PATH" "SERVER_PORT"
  elif env_has_key "$CANDIDATE_ENV_PATH" "SERVER_PORT"; then
    upsert_env_value "$CANDIDATE_ENV_PATH" "SERVER_PORT" "$PARALLEL_PORT"
  else
    upsert_env_value "$CANDIDATE_ENV_PATH" "PORT" "$PARALLEL_PORT"
  fi
  upsert_env_value "$CANDIDATE_ENV_PATH" "ERP_BRIDGE_BASE_URL" "http://127.0.0.1:8081"

  RESULT_STATUS="deployed_parallel"
  RESULT_RUNTIME_ENV_PATH="$CANDIDATE_ENV_PATH"
  RESULT_PID_FILE="$CANDIDATE_PID_FILE"
  RESULT_LOG_FILE="$CANDIDATE_LOG_FILE"

  if [ "$START_SERVICES" = "true" ]; then
    if [ "$MAIN_ENV_CREATED" = "true" ]; then
      RESULT_STATUS="deployed_parallel_waiting_for_env"
    else
      "$RELEASE_DIR/deploy/start-main.sh" \
        --base-dir "$REMOTE_BASE_DIR" \
        --env-file "$CANDIDATE_ENV_PATH" \
        --binary-path "$RELEASE_DIR/ecommerce-api" \
        --pid-file "$CANDIDATE_PID_FILE" \
        --log-file "$CANDIDATE_LOG_FILE" \
        --port "$PARALLEL_PORT" >/dev/null
    fi
  fi
else
  if [ ! -f "$RUNTIME_ENV_PATH" ]; then
    cp "$RELEASE_DIR/.env.example" "$RUNTIME_ENV_PATH"
    MAIN_ENV_CREATED="true"
  fi
  if [ ! -f "$BRIDGE_ENV_PATH" ]; then
    cp "$RELEASE_DIR/bridge.env.example" "$BRIDGE_ENV_PATH"
    BRIDGE_ENV_CREATED="true"
  fi
  if main_env_uses_db_field_model "$RUNTIME_ENV_PATH"; then
    remove_env_key "$RUNTIME_ENV_PATH" "MYSQL_DSN"
  fi
  if ! env_has_key "$RUNTIME_ENV_PATH" "USER_AVATAR_DIR"; then
    upsert_env_value "$RUNTIME_ENV_PATH" "USER_AVATAR_DIR" "$SHARED_AVATAR_DIR"
  fi
  if main_env_uses_db_field_model "$BRIDGE_ENV_PATH"; then
    remove_env_key "$BRIDGE_ENV_PATH" "MYSQL_DSN"
  fi

  if [ "$START_SERVICES" = "true" ]; then
    if [ "$MAIN_ENV_CREATED" = "true" ] || [ "$BRIDGE_ENV_CREATED" = "true" ]; then
      RESULT_STATUS="deployed_waiting_for_env"
    else
      "$RELEASE_DIR/deploy/run-pending-migrations.sh" --base-dir "$REMOTE_BASE_DIR"
      PACKAGE_COMMIT="$(
        python3 - "$RELEASE_DIR/PACKAGE_INFO.json" <<'PY'
import json
import sys
from pathlib import Path

value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
print(value.get("git_commit", ""))
PY
      )"
      "$RELEASE_DIR/deploy/check-v8-cutover-readiness.sh" \
        --base-dir "$REMOTE_BASE_DIR" \
        --expected-commit "$PACKAGE_COMMIT"
      "$REMOTE_BASE_DIR/scripts/stop-main.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null || true
      "$REMOTE_BASE_DIR/scripts/stop-bridge.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null || true
      if [ ! -x "$RELEASE_DIR/search_reindex" ]; then
        restart_previous_release ||
          fail "Packaged search_reindex is missing and the previous release could not be restarted."
        fail "Packaged search_reindex tool is required before live cutover."
      fi
      if ! "$RELEASE_DIR/deploy/run-with-env.sh" "$RUNTIME_ENV_PATH" \
        "$RELEASE_DIR/search_reindex" \
        --tasks=false \
        --assets=false \
        --products=false \
        --ensure-product-index \
        --timeout=20m; then
        restart_previous_release ||
          fail "Product search index readiness failed and the previous release could not be restarted."
        fail "Product search index readiness failed before live symlink cutover."
      fi
      activate_release_links
      if ! "$RELEASE_DIR/deploy/start-main.sh" --base-dir "$REMOTE_BASE_DIR" --env-file "$RUNTIME_ENV_PATH" >/dev/null; then
        rollback_failed_cutover ||
          fail "New MAIN failed readiness and automatic rollback restart also failed."
        fail "New MAIN failed readiness; previous release links and services were restored."
      fi
      if ! "$RELEASE_DIR/deploy/start-bridge.sh" --base-dir "$REMOTE_BASE_DIR" --env-file "$BRIDGE_ENV_PATH" >/dev/null; then
        rollback_failed_cutover ||
          fail "New Bridge failed readiness and automatic rollback restart also failed."
        fail "New Bridge failed readiness; previous release links and services were restored."
      fi
    fi
  else
    activate_release_links
  fi
fi

if [ "${KEEP_RELEASES:-0}" -gt 0 ] 2>/dev/null; then
  mapfile -t RELEASE_DIRS < <(find "$REMOTE_BASE_DIR/releases" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V)
  if [ "${#RELEASE_DIRS[@]}" -gt "$KEEP_RELEASES" ]; then
    REMOVE_COUNT=$((${#RELEASE_DIRS[@]} - KEEP_RELEASES))
    for old_version in "${RELEASE_DIRS[@]:0:$REMOVE_COUNT}"; do
      if [ "$old_version" != "$VERSION" ]; then
        old_release_target="$(readlink -f "$REMOTE_BASE_DIR/releases/$old_version" 2>/dev/null || true)"
        current_release_target="$(readlink -f "$REMOTE_BASE_DIR/current" 2>/dev/null || true)"
        if [ -n "$old_release_target" ] && [ "$old_release_target" = "$current_release_target" ]; then
          continue
        fi
        rm -rf "$REMOTE_BASE_DIR/releases/$old_version"
      fi
    done
  fi
fi

STATE_PATH="$REMOTE_BASE_DIR/deploy-state.env"
if [ "$PARALLEL" = "true" ]; then
  STATE_PATH="$RELEASE_DIR/runtime/deploy-state.parallel.env"
fi

cat >"$STATE_PATH" <<EOF
APP_NAME=${DEPLOY_APP_NAME:-ecommerce-ai}
CURRENT_VERSION=$VERSION
RELEASE_DIR=$RELEASE_DIR
MAIN_BINARY=$([ "$PARALLEL" = "true" ] && printf '%s' "$RELEASE_DIR/ecommerce-api" || printf '%s' "$REMOTE_BASE_DIR/ecommerce-api")
BRIDGE_BINARY=$([ "$PARALLEL" = "true" ] && printf '%s' "$REMOTE_BASE_DIR/erp_bridge" || printf '%s' "$REMOTE_BASE_DIR/erp_bridge")
RUNTIME_ENV_PATH=$RESULT_RUNTIME_ENV_PATH
BRIDGE_ENV_PATH=$BRIDGE_ENV_PATH
MAIN_PORT=$(read_main_port_from_env "$RESULT_RUNTIME_ENV_PATH" 8080)
BRIDGE_PORT=$(read_env_value "$BRIDGE_ENV_PATH" SERVER_PORT 8081)
PID_FILE=$RESULT_PID_FILE
LOG_FILE=$RESULT_LOG_FILE
DEPLOY_MODE=$([ "$PARALLEL" = "true" ] && printf 'parallel' || printf 'cutover')
STATUS=$RESULT_STATUS
UPDATED_AT_UTC=$(utc_now)
EOF

printf 'RESULT_VERSION=%s\n' "$VERSION"
printf 'RESULT_RELEASE_DIR=%s\n' "$RELEASE_DIR"
printf 'RESULT_MAIN_ENV_CREATED=%s\n' "$MAIN_ENV_CREATED"
printf 'RESULT_BRIDGE_ENV_CREATED=%s\n' "$BRIDGE_ENV_CREATED"
printf 'RESULT_RUNTIME_ENV_PATH=%s\n' "$RESULT_RUNTIME_ENV_PATH"
printf 'RESULT_PID_FILE=%s\n' "$RESULT_PID_FILE"
printf 'RESULT_LOG_FILE=%s\n' "$RESULT_LOG_FILE"
printf 'RESULT_STATUS=%s\n' "$RESULT_STATUS"
