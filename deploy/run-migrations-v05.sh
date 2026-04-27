#!/usr/bin/env bash
# Run v0.5 migrations (038, 039, 040) on target DB. Order is mandatory: 038 is v0.5 startup prerequisite.
# Usage: bash deploy/run-migrations-v05.sh [--base-dir /root/ecommerce_ai] [--dry-run]
# Reads DB connection from shared/main.env (DB_HOST, DB_USER, DB_NAME, DB_PASS, DB_PORT).
set -euo pipefail

BASE_DIR=""
DRY_RUN="false"
while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    --dry-run)  DRY_RUN="true"; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BASE_DIR="${BASE_DIR:-/root/ecommerce_ai}"
ENV_FILE="$BASE_DIR/shared/main.env"
MIGRATIONS_DIR="$ROOT/db/migrations"

if [ ! -f "$ENV_FILE" ]; then
  echo "Env file not found: $ENV_FILE" >&2
  echo "Set BASE_DIR if different, e.g. --base-dir /path/to/ecommerce_ai" >&2
  exit 1
fi

set -a
# shellcheck source=/dev/null
. "$ENV_FILE" 2>/dev/null || true
set +a

if [ -z "${DB_HOST:-}" ] || [ -z "${DB_USER:-}" ] || [ -z "${DB_NAME:-}" ]; then
  echo "Set DB_HOST, DB_USER, DB_NAME in $ENV_FILE" >&2
  exit 1
fi
DB_PORT="${DB_PORT:-3306}"
export MYSQL_PWD="${DB_PASS:-}"

run_mysql() {
  mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" -N -e "$1" 2>/dev/null || true
}

has_table() {
  run_mysql "SHOW TABLES LIKE '$1'" | grep -q "$1"
}

has_column() {
  run_mysql "SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='${DB_NAME}' AND TABLE_NAME='$1' AND COLUMN_NAME='$2' LIMIT 1" | grep -q 1
}

echo "=== v0.5 Migration Runner (038, 039, 040) ==="
echo "DB: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo ""

# 038: users jst_u_id / jst_raw_snapshot_json (v0.5 startup prerequisite; must run first)
if has_column "users" "jst_u_id"; then
  echo "  [038] users.jst_u_id already exists, skipping"
else
  echo "  [038] Applying users JST prewire..."
  if [ "$DRY_RUN" = "true" ]; then
    echo "  [038] (dry-run) would run: mysql < $MIGRATIONS_DIR/038_v7_jst_user_prewire.sql"
  else
    mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" < "$MIGRATIONS_DIR/038_v7_jst_user_prewire.sql"
    echo "  [038] OK"
  fi
fi

# Pre-check: 039 rule_templates
if has_table "rule_templates"; then
  echo "  [039] rule_templates already exists, skipping"
else
  echo "  [039] Applying rule_templates..."
  if [ "$DRY_RUN" = "true" ]; then
    echo "  [039] (dry-run) would run: mysql < $MIGRATIONS_DIR/039_v7_rule_templates.sql"
  else
    mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" < "$MIGRATIONS_DIR/039_v7_rule_templates.sql"
    echo "  [039] OK"
  fi
fi

# Pre-check: 040 server_logs
if has_table "server_logs"; then
  echo "  [040] server_logs already exists, skipping"
else
  echo "  [040] Applying server_logs..."
  if [ "$DRY_RUN" = "true" ]; then
    echo "  [040] (dry-run) would run: mysql < $MIGRATIONS_DIR/040_v7_server_logs.sql"
  else
    mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" < "$MIGRATIONS_DIR/040_v7_server_logs.sql"
    echo "  [040] OK"
  fi
fi

# Verification
echo ""
echo "=== Verification ==="
if has_column "users" "jst_u_id"; then
  echo "  users.jst_u_id (038): OK"
else
  echo "  users.jst_u_id (038): MISSING"
fi
if has_table "rule_templates"; then
  echo "  rule_templates: OK ($(run_mysql "SELECT COUNT(*) FROM rule_templates" | tr -d ' ') rows)"
else
  echo "  rule_templates: MISSING"
fi
if has_table "server_logs"; then
  echo "  server_logs: OK ($(run_mysql "SELECT COUNT(*) FROM server_logs" | tr -d ' ') rows)"
else
  echo "  server_logs: MISSING"
fi

unset MYSQL_PWD
echo ""
echo "=== Done ==="
