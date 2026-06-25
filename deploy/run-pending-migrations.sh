#!/usr/bin/env bash
# Apply repository DB migrations that were added after the deployment-side
# baseline was created. Rollback blocks are intentionally stripped.
set -euo pipefail

BASE_DIR="/root/ecommerce_ai"
DRY_RUN="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    --dry-run) DRY_RUN="true"; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$BASE_DIR/shared/main.env"
MIGRATIONS_DIR="$ROOT/db/migrations"
MIGRATION_TABLE="schema_migrations"

die() { echo "[migrations] ERROR: $*" >&2; exit 1; }
step() { echo "[migrations] $*"; }

[ -d "$MIGRATIONS_DIR" ] || die "Missing migrations dir: $MIGRATIONS_DIR"
[ -f "$ENV_FILE" ] || die "Missing env file: $ENV_FILE"

set -a
# shellcheck source=/dev/null
. "$ENV_FILE" 2>/dev/null || true
set +a

[ -n "${DB_HOST:-}" ] || die "DB_HOST is required in $ENV_FILE"
[ -n "${DB_USER:-}" ] || die "DB_USER is required in $ENV_FILE"
[ -n "${DB_NAME:-}" ] || die "DB_NAME is required in $ENV_FILE"
DB_PORT="${DB_PORT:-3306}"
export MYSQL_PWD="${DB_PASS:-}"

mysql_args=(-h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}")

mysql_exec() {
  mysql "${mysql_args[@]}" "$@"
}

mysql_query() {
  mysql "${mysql_args[@]}" -N -B -e "$1"
}

checksum_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

sql_quote() {
  printf "%s" "$1" | sed "s/'/''/g"
}

forward_sql_to() {
  local src="$1"
  local dst="$2"
  sed '/^[[:space:]]*--[[:space:]]*ROLLBACK-BEGIN/,$d' "$src" >"$dst"
}

step "DB: ${DB_HOST}:${DB_PORT}/${DB_NAME}"

if [ "$DRY_RUN" != "true" ]; then
  mysql_exec <<SQL
CREATE TABLE IF NOT EXISTS ${MIGRATION_TABLE} (
  file_name VARCHAR(255) NOT NULL PRIMARY KEY,
  checksum_sha256 VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  error_message TEXT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Repository deployment migration ledger';
SQL
fi

mapfile -t migration_files < <(find "$MIGRATIONS_DIR" -maxdepth 1 -type f -name '*.sql' -printf '%f\n' | sort -V)
[ "${#migration_files[@]}" -gt 0 ] || die "No migration files found in $MIGRATIONS_DIR"

existing_count="0"
if [ "$DRY_RUN" != "true" ]; then
  existing_count="$(mysql_query "SELECT COUNT(*) FROM ${MIGRATION_TABLE}" | tr -d '[:space:]')"
fi

if [ "$DRY_RUN" != "true" ] && [ "${existing_count:-0}" = "0" ]; then
  step "No migration ledger rows found; baselining existing ${#migration_files[@]} files without replay."
  for file_name in "${migration_files[@]}"; do
    path="$MIGRATIONS_DIR/$file_name"
    checksum="$(checksum_file "$path")"
    mysql_exec <<SQL
INSERT INTO ${MIGRATION_TABLE} (file_name, checksum_sha256, status)
VALUES ('$(sql_quote "$file_name")', '$(sql_quote "$checksum")', 'baseline')
ON DUPLICATE KEY UPDATE checksum_sha256 = VALUES(checksum_sha256), status = status;
SQL
  done
  step "Baseline recorded."
  unset MYSQL_PWD
  exit 0
fi

applied=0
for file_name in "${migration_files[@]}"; do
  path="$MIGRATIONS_DIR/$file_name"
  checksum="$(checksum_file "$path")"
  if [ "$DRY_RUN" != "true" ]; then
    already="$(mysql_query "SELECT COUNT(*) FROM ${MIGRATION_TABLE} WHERE file_name='$(sql_quote "$file_name")' AND status IN ('baseline','applied')" | tr -d '[:space:]')"
    if [ "${already:-0}" != "0" ]; then
      continue
    fi
  fi

  step "Applying $file_name"
  tmp_sql="$(mktemp)"
  forward_sql_to "$path" "$tmp_sql"
  if [ ! -s "$tmp_sql" ]; then
    rm -f "$tmp_sql"
    step "Skipping empty forward SQL: $file_name"
    continue
  fi

  if [ "$DRY_RUN" = "true" ]; then
    rm -f "$tmp_sql"
    continue
  fi

  if mysql_exec <"$tmp_sql"; then
    mysql_exec <<SQL
INSERT INTO ${MIGRATION_TABLE} (file_name, checksum_sha256, status, error_message)
VALUES ('$(sql_quote "$file_name")', '$(sql_quote "$checksum")', 'applied', NULL)
ON DUPLICATE KEY UPDATE checksum_sha256 = VALUES(checksum_sha256), status = 'applied', error_message = NULL, applied_at = CURRENT_TIMESTAMP;
SQL
    applied=$((applied + 1))
  else
    mysql_exec <<SQL
INSERT INTO ${MIGRATION_TABLE} (file_name, checksum_sha256, status, error_message)
VALUES ('$(sql_quote "$file_name")', '$(sql_quote "$checksum")', 'failed', 'migration failed')
ON DUPLICATE KEY UPDATE checksum_sha256 = VALUES(checksum_sha256), status = 'failed', error_message = 'migration failed', applied_at = CURRENT_TIMESTAMP;
SQL
    rm -f "$tmp_sql"
    die "Migration failed: $file_name"
  fi
  rm -f "$tmp_sql"
done

step "Done. applied=$applied"
unset MYSQL_PWD
