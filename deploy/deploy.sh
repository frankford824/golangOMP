#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

RELEASE_NOTE="bash release workflow standardization"
RELEASE_HISTORY_PATH="deploy/release-history.log"
OUTPUT_ROOT="dist"
LOCAL_ONLY="false"
SKIP_TESTS="false"
SKIP_RUNTIME_VERIFY="false"
FIXED_VERSION=""

RUNTIME_ENV_FILE=""
BRIDGE_ENV_FILE=""
PARALLEL_DEPLOY="false"
PARALLEL_PORT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --release-note)
      RELEASE_NOTE="$2"
      shift 2
      ;;
    --release-history-path)
      RELEASE_HISTORY_PATH="$2"
      shift 2
      ;;
    --output-root)
      OUTPUT_ROOT="$2"
      shift 2
      ;;
    --runtime-env-file)
      RUNTIME_ENV_FILE="$2"
      shift 2
      ;;
    --bridge-env-file)
      BRIDGE_ENV_FILE="$2"
      shift 2
      ;;
    --parallel)
      PARALLEL_DEPLOY="true"
      shift
      ;;
    --parallel-port)
      PARALLEL_PORT="$2"
      shift 2
      ;;
    --local-only)
      LOCAL_ONLY="true"
      shift
      ;;
    --skip-tests)
      SKIP_TESTS="true"
      shift
      ;;
    --skip-runtime-verify)
      SKIP_RUNTIME_VERIFY="true"
      shift
      ;;
    --version)
      FIXED_VERSION="$2"
      shift 2
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

ROOT="$(repo_root)"
load_local_deploy_env "$ROOT"

RUNTIME_ENV_FILE="${RUNTIME_ENV_FILE:-${DEPLOY_RUNTIME_ENV_FILE:-}}"
BRIDGE_ENV_FILE="${BRIDGE_ENV_FILE:-${DEPLOY_BRIDGE_ENV_FILE:-}}"
PARALLEL_PORT="${PARALLEL_PORT:-${DEPLOY_PARALLEL_PORT:-18080}}"

HISTORY_PATH="$(resolve_path "$ROOT" "$RELEASE_HISTORY_PATH")"
ensure_release_history_file "$HISTORY_PATH"

ARTIFACT_PREFIX="$(history_value "$HISTORY_PATH" artifact_prefix)"
BRIDGE_BASE_URL_DEFAULT="$(history_value "$HISTORY_PATH" bridge_runtime_base_url)"
if [ -n "${FIXED_VERSION:-}" ]; then
  VERSION="$FIXED_VERSION"
elif [ "$LOCAL_ONLY" = "true" ]; then
  VERSION="$(next_managed_release_version "$HISTORY_PATH")"
else
  fail "Remote deploy requires explicit --version to prevent unintended managed release drift."
fi
CREATED_AT="$(utc_now)"
DEPLOY_HOST_VALUE="local-only"
DEPLOY_BASE_DIR_VALUE="/root/ecommerce_ai"

if [ "$LOCAL_ONLY" != "true" ]; then
  : "${DEPLOY_HOST:?DEPLOY_HOST is required for remote deployment.}"
  : "${DEPLOY_USER:?DEPLOY_USER is required for remote deployment.}"
  : "${DEPLOY_PORT:?DEPLOY_PORT is required for remote deployment.}"
  : "${DEPLOY_BASE_DIR:?DEPLOY_BASE_DIR is required for remote deployment.}"
  # SSH key deploy is the default path. Password auth is compatibility-only and
  # requires DEPLOY_AUTH_MODE=password plus DEPLOY_PASSWORD.
  DEPLOY_HOST_VALUE="$DEPLOY_HOST"
  DEPLOY_BASE_DIR_VALUE="$DEPLOY_BASE_DIR"
fi

append_release_record \
  "$HISTORY_PATH" \
  "$VERSION" \
  "packaging" \
  "$RELEASE_NOTE" \
  "" \
  "" \
  "$DEPLOY_HOST_VALUE" \
  "$DEPLOY_BASE_DIR_VALUE" \
  "release workflow started" \
  "$CREATED_AT" \
  "$CREATED_AT"

if ! {
  require_cmd tar
  go_cmd >/dev/null
  package_release "$ROOT" "$VERSION" "$OUTPUT_ROOT" "$SKIP_TESTS" "$ARTIFACT_PREFIX" "${DEPLOY_BRIDGE_BASE_URL:-$BRIDGE_BASE_URL_DEFAULT}"

  append_release_record \
    "$HISTORY_PATH" \
    "$VERSION" \
    "$([ "$LOCAL_ONLY" = "true" ] && printf 'packaged_local_only' || printf 'packaged')" \
    "$RELEASE_NOTE" \
    "$PACKAGE_ARTIFACT_NAME" \
    "$PACKAGE_ARTIFACT_SHA256" \
    "$DEPLOY_HOST_VALUE" \
    "$DEPLOY_BASE_DIR_VALUE" \
    "entrypoint=$PACKAGE_ENTRYPOINT artifact_dir=$PACKAGE_ARTIFACT_DIR_NAME" \
    "$CREATED_AT" \
    "$(utc_now)"

  if [ "$LOCAL_ONLY" = "true" ]; then
    printf 'Packaged %s at %s\n' "$VERSION" "$PACKAGE_ARTIFACT_PATH"
    exit 0
  fi

  require_cmd ssh scp bash

  SSH_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"
  REMOTE_MAIN_ENV="${DEPLOY_BASE_DIR}/shared/main.env"
  REMOTE_BRIDGE_ENV="${DEPLOY_BASE_DIR}/shared/bridge.env"
  REMOTE_PARALLEL_MAIN_ENV="${DEPLOY_BASE_DIR}/incoming/${PACKAGE_ARTIFACT_DIR_NAME}.main.parallel.env"

  ssh_runner "$SSH_TARGET" \
    "DEPLOY_BASE_DIR=$(printf '%q' "$DEPLOY_BASE_DIR") bash -s" <<'REMOTE_PREP'
set -euo pipefail
mkdir -p \
  "$DEPLOY_BASE_DIR/incoming" \
  "$DEPLOY_BASE_DIR/releases" \
  "$DEPLOY_BASE_DIR/shared" \
  "$DEPLOY_BASE_DIR/logs" \
  "$DEPLOY_BASE_DIR/run" \
  "$DEPLOY_BASE_DIR/scripts"
REMOTE_PREP

  scp_runner "$PACKAGE_ARTIFACT_PATH" "$SSH_TARGET:$DEPLOY_BASE_DIR/incoming/$PACKAGE_ARTIFACT_NAME"
  if [ "$PARALLEL_DEPLOY" = "true" ] && [ -n "$RUNTIME_ENV_FILE" ]; then
    scp_runner "$(resolve_path "$ROOT" "$RUNTIME_ENV_FILE")" "$SSH_TARGET:$REMOTE_PARALLEL_MAIN_ENV"
  elif [ -n "$RUNTIME_ENV_FILE" ]; then
    scp_runner "$(resolve_path "$ROOT" "$RUNTIME_ENV_FILE")" "$SSH_TARGET:$REMOTE_MAIN_ENV"
  fi
  if [ "$PARALLEL_DEPLOY" != "true" ] && [ -n "$BRIDGE_ENV_FILE" ]; then
    scp_runner "$(resolve_path "$ROOT" "$BRIDGE_ENV_FILE")" "$SSH_TARGET:$REMOTE_BRIDGE_ENV"
  fi

  append_release_record \
    "$HISTORY_PATH" \
    "$VERSION" \
    "uploaded" \
    "$RELEASE_NOTE" \
    "$PACKAGE_ARTIFACT_NAME" \
    "$PACKAGE_ARTIFACT_SHA256" \
    "$DEPLOY_HOST_VALUE" \
    "$DEPLOY_BASE_DIR_VALUE" \
    "uploaded via scp" \
    "$CREATED_AT" \
    "$(utc_now)"

  REMOTE_OUTPUT="$(
    ssh_runner "$SSH_TARGET" \
      "DEPLOY_BASE_DIR=$(printf '%q' "$DEPLOY_BASE_DIR") ARTIFACT_NAME=$(printf '%q' "$PACKAGE_ARTIFACT_NAME") ARTIFACT_DIR_NAME=$(printf '%q' "$PACKAGE_ARTIFACT_DIR_NAME") VERSION=$(printf '%q' "$VERSION") KEEP_RELEASES=$(printf '%q' "${DEPLOY_KEEP_RELEASES:-5}") MAIN_ENV=$(printf '%q' "$([ "$PARALLEL_DEPLOY" = "true" ] && [ -n "$RUNTIME_ENV_FILE" ] && printf '%s' "$REMOTE_PARALLEL_MAIN_ENV" || printf '%s' "$REMOTE_MAIN_ENV")") BRIDGE_ENV=$(printf '%q' "$REMOTE_BRIDGE_ENV") PARALLEL_DEPLOY=$(printf '%q' "$PARALLEL_DEPLOY") PARALLEL_PORT=$(printf '%q' "$PARALLEL_PORT") bash -s" <<'REMOTE_DEPLOY'
set -euo pipefail
tar -xzf "$DEPLOY_BASE_DIR/incoming/$ARTIFACT_NAME" -C "$DEPLOY_BASE_DIR/incoming"
if [ "$PARALLEL_DEPLOY" = "true" ]; then
  bash "$DEPLOY_BASE_DIR/incoming/$ARTIFACT_DIR_NAME/deploy/remote-deploy.sh" \
    --package-root "$DEPLOY_BASE_DIR/incoming/$ARTIFACT_DIR_NAME" \
    --version "$VERSION" \
    --remote-base-dir "$DEPLOY_BASE_DIR" \
    --runtime-env-path "$MAIN_ENV" \
    --bridge-env-path "$BRIDGE_ENV" \
    --keep-releases "$KEEP_RELEASES" \
    --parallel \
    --parallel-port "$PARALLEL_PORT" \
    --start-services
else
  bash "$DEPLOY_BASE_DIR/incoming/$ARTIFACT_DIR_NAME/deploy/remote-deploy.sh" \
    --package-root "$DEPLOY_BASE_DIR/incoming/$ARTIFACT_DIR_NAME" \
    --version "$VERSION" \
    --remote-base-dir "$DEPLOY_BASE_DIR" \
    --runtime-env-path "$MAIN_ENV" \
    --bridge-env-path "$BRIDGE_ENV" \
    --keep-releases "$KEEP_RELEASES" \
    --start-services
fi
REMOTE_DEPLOY
  )"

  REMOTE_STATUS="$(printf '%s\n' "$REMOTE_OUTPUT" | awk -F= '/^RESULT_STATUS=/{print $2; exit}')"
  [ -n "$REMOTE_STATUS" ] || fail "Remote deploy did not return RESULT_STATUS."
  REMOTE_RELEASE_DIR="$(printf '%s\n' "$REMOTE_OUTPUT" | awk -F= '/^RESULT_RELEASE_DIR=/{print $2; exit}')"

  append_release_record \
    "$HISTORY_PATH" \
    "$VERSION" \
    "$REMOTE_STATUS" \
    "$RELEASE_NOTE" \
    "$PACKAGE_ARTIFACT_NAME" \
    "$PACKAGE_ARTIFACT_SHA256" \
    "$DEPLOY_HOST_VALUE" \
    "$DEPLOY_BASE_DIR_VALUE" \
    "release_dir=$REMOTE_RELEASE_DIR" \
    "$CREATED_AT" \
    "$(utc_now)"

  if [ "$SKIP_RUNTIME_VERIFY" != "true" ] && { [ "$REMOTE_STATUS" = "deployed" ] || [ "$REMOTE_STATUS" = "deployed_parallel" ]; }; then
    ssh_runner "$SSH_TARGET" \
      "DEPLOY_BASE_DIR=$(printf '%q' "$DEPLOY_BASE_DIR") RELEASE_DIR=$(printf '%q' "$REMOTE_RELEASE_DIR") MAIN_PORT=$(printf '%q' "$([ "$REMOTE_STATUS" = "deployed_parallel" ] && printf '%s' "$PARALLEL_PORT" || printf '%s' "${DEPLOY_MAIN_PORT:-8080}")") VERIFY_SCRIPT=$(printf '%q' "$([ "$REMOTE_STATUS" = "deployed_parallel" ] && printf '%s' "$REMOTE_RELEASE_DIR/deploy/verify-runtime.sh" || printf '%s' "$DEPLOY_BASE_DIR/scripts/verify-runtime.sh")") bash -s" <<'REMOTE_VERIFY'
set -euo pipefail
if [ "$MAIN_PORT" != "8080" ]; then
  bash "$VERIFY_SCRIPT" --base-url "http://127.0.0.1:${MAIN_PORT}" --skip-three-service-check
else
  bash "$VERIFY_SCRIPT" --base-dir "$DEPLOY_BASE_DIR" --base-url "http://127.0.0.1:${MAIN_PORT}" --bridge-url "http://127.0.0.1:8081" --sync-url "http://127.0.0.1:8082" --auto-recover-8082
fi
REMOTE_VERIFY
  fi
}; then
  append_release_record \
    "$HISTORY_PATH" \
    "$VERSION" \
    "failed" \
    "$RELEASE_NOTE" \
    "${PACKAGE_ARTIFACT_NAME:-}" \
    "${PACKAGE_ARTIFACT_SHA256:-}" \
    "$DEPLOY_HOST_VALUE" \
    "$DEPLOY_BASE_DIR_VALUE" \
    "command_failed" \
    "$CREATED_AT" \
    "$(utc_now)"
  exit 1
fi
