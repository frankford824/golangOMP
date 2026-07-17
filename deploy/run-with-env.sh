#!/usr/bin/env bash
set -euo pipefail

[ $# -ge 2 ] || {
  printf 'usage: %s <env-file> <command> [args...]\n' "$0" >&2
  exit 1
}

ENV_FILE="$1"
shift
[ -f "$ENV_FILE" ] || {
  printf 'env file not found: %s\n' "$ENV_FILE" >&2
  exit 1
}

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [ -z "${SERVER_PORT:-}" ] && [ -n "${PORT:-}" ]; then
  export SERVER_PORT="$PORT"
fi
if [ -z "${PORT:-}" ] && [ -n "${SERVER_PORT:-}" ]; then
  export PORT="$SERVER_PORT"
fi

if [ -z "${MYSQL_DSN:-}" ] && [ -n "${DB_HOST:-}" ] && [ -n "${DB_USER:-}" ] && [ -n "${DB_NAME:-}" ]; then
  dsn_port="${DB_PORT:-3306}"
  dsn_charset="${DB_CHARSET:-utf8mb4}"
  dsn_parse_time="${DB_PARSE_TIME:-True}"
  dsn_loc="${DB_LOC:-${TZ:-Local}}"
  dsn_loc="${dsn_loc//'%'/'%25'}"
  dsn_loc="${dsn_loc//'/'/'%2F'}"
  dsn_loc="${dsn_loc//' '/'%20'}"
  export MYSQL_DSN="${DB_USER}:${DB_PASS:-}@tcp(${DB_HOST}:${dsn_port})/${DB_NAME}?charset=${dsn_charset}&parseTime=${dsn_parse_time}&loc=${dsn_loc}"
fi

# Run from the resolved binary directory so packaged relative config paths work.
command_path="$1"
if [ -n "${RUN_WORK_DIR:-}" ] && [ -d "${RUN_WORK_DIR:-}" ]; then
  cd "$RUN_WORK_DIR"
elif [ -n "${command_path:-}" ] && [[ "$command_path" == */* ]]; then
  resolved_command="$(readlink -f "$command_path" 2>/dev/null || printf '%s' "$command_path")"
  resolved_dir="$(dirname "$resolved_command")"
  if [ -d "$resolved_dir" ]; then
    cd "$resolved_dir"
  fi
fi

exec "$@"
