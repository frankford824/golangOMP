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

mkdir -p \
  "$REMOTE_BASE_DIR/incoming" \
  "$REMOTE_BASE_DIR/releases" \
  "$REMOTE_BASE_DIR/shared" \
  "$REMOTE_BASE_DIR/logs" \
  "$REMOTE_BASE_DIR/run" \
  "$REMOTE_BASE_DIR/scripts"

rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"
cp -R "$PACKAGE_ROOT"/. "$RELEASE_DIR/"
chmod +x "$RELEASE_DIR"/deploy/*.sh "$RELEASE_DIR/ecommerce-api" "$RELEASE_DIR/erp_bridge"

if [ "$PARALLEL" != "true" ]; then
  cp "$RELEASE_DIR"/deploy/*.sh "$REMOTE_BASE_DIR/scripts/"
  chmod +x "$REMOTE_BASE_DIR/scripts/"*.sh

  ln -sfn "$RELEASE_DIR" "$REMOTE_BASE_DIR/current"
  ln -sfn "$RELEASE_DIR/ecommerce-api" "$REMOTE_BASE_DIR/ecommerce-api"
  ln -sfn "$RELEASE_DIR/erp_bridge" "$REMOTE_BASE_DIR/erp_bridge"
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
  CANDIDATE_PID_FILE="$REMOTE_BASE_DIR/run/ecommerce-api-${VERSION}-parallel.pid"
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
  if main_env_uses_db_field_model "$BRIDGE_ENV_PATH"; then
    remove_env_key "$BRIDGE_ENV_PATH" "MYSQL_DSN"
  fi

  if [ "$START_SERVICES" = "true" ]; then
    if [ "$MAIN_ENV_CREATED" = "true" ] || [ "$BRIDGE_ENV_CREATED" = "true" ]; then
      RESULT_STATUS="deployed_waiting_for_env"
    else
      "$REMOTE_BASE_DIR/scripts/stop-main.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null || true
      "$REMOTE_BASE_DIR/scripts/stop-bridge.sh" --base-dir "$REMOTE_BASE_DIR" >/dev/null || true
      "$REMOTE_BASE_DIR/scripts/start-main.sh" --base-dir "$REMOTE_BASE_DIR" --env-file "$RUNTIME_ENV_PATH" >/dev/null
      "$REMOTE_BASE_DIR/scripts/start-bridge.sh" --base-dir "$REMOTE_BASE_DIR" --env-file "$BRIDGE_ENV_PATH" >/dev/null
    fi
  fi
fi

if [ "${KEEP_RELEASES:-0}" -gt 0 ] 2>/dev/null; then
  mapfile -t RELEASE_DIRS < <(find "$REMOTE_BASE_DIR/releases" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V)
  if [ "${#RELEASE_DIRS[@]}" -gt "$KEEP_RELEASES" ]; then
    REMOVE_COUNT=$((${#RELEASE_DIRS[@]} - KEEP_RELEASES))
    for old_version in "${RELEASE_DIRS[@]:0:$REMOVE_COUNT}"; do
      if [ "$old_version" != "$VERSION" ]; then
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
