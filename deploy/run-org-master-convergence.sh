#!/usr/bin/env bash
# Apply the v1.0 org-master-data convergence release flow on a target DB.
# Order is intentional:
#   1. backup focused org/identity tables
#   2. apply 058 to drop global org_teams.name uniqueness
#   3. seed the official v1.0 org baseline from config/auth_identity.json
#   4. apply 057 to migrate users + disable legacy org rows
# Usage:
#   bash deploy/run-org-master-convergence.sh [--base-dir /root/ecommerce_ai] [--backup-dir /root/ecommerce_ai/backups/<ts>] [--dry-run]
set -euo pipefail

BASE_DIR="/root/ecommerce_ai"
BACKUP_DIR=""
DRY_RUN="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    --backup-dir)
      BACKUP_DIR="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$BASE_DIR/shared/main.env"
CONFIG_FILE="$ROOT/config/auth_identity.json"
MIGRATION_057="$ROOT/db/migrations/057_v1_0_org_master_convergence.sql"
MIGRATION_058="$ROOT/db/migrations/058_v1_0_org_team_department_scoped_uniqueness.sql"

[ -f "$ENV_FILE" ] || { echo "Env file not found: $ENV_FILE" >&2; exit 1; }
[ -f "$CONFIG_FILE" ] || { echo "Config file not found: $CONFIG_FILE" >&2; exit 1; }
[ -f "$MIGRATION_057" ] || { echo "Migration file not found: $MIGRATION_057" >&2; exit 1; }
[ -f "$MIGRATION_058" ] || { echo "Migration file not found: $MIGRATION_058" >&2; exit 1; }

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

command -v mysql >/dev/null 2>&1 || { echo "mysql client is required" >&2; exit 1; }
command -v mysqldump >/dev/null 2>&1 || { echo "mysqldump is required" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "python3 is required" >&2; exit 1; }

run_mysql() {
  mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" -N -e "$1"
}

run_sql_file() {
  mysql -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" < "$1"
}

timestamp_utc() {
  date -u +"%Y%m%dT%H%M%SZ"
}

if [ -z "$BACKUP_DIR" ]; then
  BACKUP_DIR="$BASE_DIR/backups/$(timestamp_utc)_org_master_convergence"
fi

SEED_SQL="$(mktemp)"
PRECHECK_SQL="$(mktemp)"
POSTCHECK_SQL="$(mktemp)"
cleanup() {
  rm -f "$SEED_SQL" "$PRECHECK_SQL" "$POSTCHECK_SQL"
  unset MYSQL_PWD
}
trap cleanup EXIT

CONFIG_FILE="$CONFIG_FILE" PRECHECK_SQL="$PRECHECK_SQL" POSTCHECK_SQL="$POSTCHECK_SQL" SEED_SQL="$SEED_SQL" python3 <<'PY'
import json
import os

config_file = os.environ["CONFIG_FILE"]
seed_sql = os.environ["SEED_SQL"]
precheck_sql = os.environ["PRECHECK_SQL"]
postcheck_sql = os.environ["POSTCHECK_SQL"]

with open(config_file, "r", encoding="utf-8") as f:
    config = json.load(f)

departments = config["departments"]
department_teams = config["department_teams"]
legacy_departments = ["设计部", "采购部", "仓储部", "烘焙仓储部"]
legacy_teams = [
    "运营一组", "运营二组", "运营三组", "运营四组", "运营五组", "运营六组", "运营七组",
    "研发默认组", "定制默认组", "云仓默认组",
    "定制美工审核组", "常规审核组",
    "设计组", "定制美工组", "设计审核组",
    "采购组", "仓储组", "烘焙仓储组",
]

def q(value: str) -> str:
    return "'" + value.replace("\\", "\\\\").replace("'", "''") + "'"

seed_lines = []
for department in departments:
    seed_lines.append(
        f"INSERT INTO org_departments (name, enabled) "
        f"SELECT {q(department)}, 1 FROM DUAL "
        f"WHERE NOT EXISTS (SELECT 1 FROM org_departments WHERE name = {q(department)});"
    )
    seed_lines.append(f"UPDATE org_departments SET enabled = 1 WHERE name = {q(department)};")

for department, teams in department_teams.items():
    for team in teams:
        seed_lines.append(
            "INSERT INTO org_teams (department_id, name, enabled) "
            f"SELECT d.id, {q(team)}, 1 "
            "FROM org_departments d "
            f"WHERE d.name = {q(department)} "
            "AND NOT EXISTS ("
            "  SELECT 1 FROM org_teams t "
            f"  WHERE t.department_id = d.id AND t.name = {q(team)}"
            ");"
        )
        seed_lines.append(
            "UPDATE org_teams t "
            "INNER JOIN org_departments d ON d.id = t.department_id "
            "SET t.enabled = 1 "
            f"WHERE d.name = {q(department)} AND t.name = {q(team)};"
        )

with open(seed_sql, "w", encoding="utf-8", newline="\n") as f:
    f.write("\n".join(seed_lines) + "\n")

with open(precheck_sql, "w", encoding="utf-8", newline="\n") as f:
    f.write(
        "SELECT 'users_by_department_team';\n"
        "SELECT department, COALESCE(team, ''), COUNT(*) FROM users GROUP BY department, team ORDER BY department, team;\n"
        "SELECT 'enabled_departments';\n"
        "SELECT name, enabled FROM org_departments ORDER BY id;\n"
        "SELECT 'enabled_teams';\n"
        "SELECT d.name, t.name, t.enabled FROM org_teams t JOIN org_departments d ON d.id = t.department_id ORDER BY d.id, t.id;\n"
        "SELECT 'legacy_department_user_rows';\n"
        "SELECT department, COUNT(*) FROM users WHERE department IN ("
        + ",".join(q(x) for x in legacy_departments)
        + ") GROUP BY department ORDER BY department;\n"
        "SELECT 'legacy_operations_user_rows';\n"
        "SELECT team, COUNT(*) FROM users WHERE department = '运营部' AND team IN ("
        + ",".join(q(x) for x in ["运营一组", "运营二组", "运营三组", "运营四组", "运营五组", "运营六组", "运营七组"])
        + ") GROUP BY team ORDER BY team;\n"
    )

official_expectations = []
for department, teams in department_teams.items():
    for team in teams:
        official_expectations.append((department, team))

with open(postcheck_sql, "w", encoding="utf-8", newline="\n") as f:
    f.write(
        "SELECT 'enabled_legacy_departments';\n"
        "SELECT name FROM org_departments WHERE enabled = 1 AND name IN ("
        + ",".join(q(x) for x in legacy_departments)
        + ") ORDER BY name;\n"
        "SELECT 'enabled_legacy_teams_under_enabled_departments';\n"
        "SELECT d.name, t.name FROM org_teams t JOIN org_departments d ON d.id = t.department_id "
        "WHERE d.enabled = 1 AND t.enabled = 1 AND t.name IN ("
        + ",".join(q(x) for x in legacy_teams)
        + ") ORDER BY d.name, t.name;\n"
        "SELECT 'official_enabled_baseline';\n"
    )
    for department, team in official_expectations:
        f.write(
            "SELECT "
            + q(f"{department}/{team}")
            + ", COUNT(*) FROM org_teams t JOIN org_departments d ON d.id = t.department_id "
            + f"WHERE d.name = {q(department)} AND d.enabled = 1 AND t.name = {q(team)} AND t.enabled = 1;\n"
        )
    f.write(
        "SELECT 'org_options_departments';\n"
        "SELECT name FROM org_departments WHERE enabled = 1 ORDER BY id;\n"
        "SELECT 'org_options_teams';\n"
        "SELECT d.name, t.name FROM org_teams t JOIN org_departments d ON d.id = t.department_id WHERE d.enabled = 1 AND t.enabled = 1 ORDER BY d.id, t.id;\n"
    )
PY

echo "=== v1.0 Org Master Convergence Runner (058 + seed + 057) ==="
echo "DB: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "Config: $CONFIG_FILE"
echo "Backup dir: $BACKUP_DIR"
echo ""
echo "--- Precheck Snapshot ---"
run_sql_file "$PRECHECK_SQL"
echo ""

if [ "$DRY_RUN" = "true" ]; then
  echo "--- Dry Run ---"
  echo "Would create backup under: $BACKUP_DIR"
  echo "Would apply: $MIGRATION_058"
  echo "Would seed official baseline from: $CONFIG_FILE"
  echo "Would apply: $MIGRATION_057"
  echo ""
  echo "--- Expected Postcheck Query Set ---"
  run_sql_file "$POSTCHECK_SQL"
  exit 0
fi

mkdir -p "$BACKUP_DIR"
mysqldump --single-transaction -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" \
  users user_roles org_departments org_teams > "$BACKUP_DIR/org_master_convergence_tables.sql"
cp "$CONFIG_FILE" "$BACKUP_DIR/auth_identity.json"
cp "$MIGRATION_057" "$BACKUP_DIR/"
cp "$MIGRATION_058" "$BACKUP_DIR/"
cp "$PRECHECK_SQL" "$BACKUP_DIR/precheck.sql"
run_sql_file "$PRECHECK_SQL" > "$BACKUP_DIR/precheck.txt"

echo "--- Applying 058 ---"
run_sql_file "$MIGRATION_058"
echo "058 applied"
echo ""

echo "--- Seeding Official v1.0 Baseline ---"
run_sql_file "$SEED_SQL"
echo "official baseline seeded"
echo ""

echo "--- Applying 057 ---"
run_sql_file "$MIGRATION_057"
echo "057 applied"
echo ""

echo "--- Postcheck Snapshot ---"
run_sql_file "$POSTCHECK_SQL" | tee "$BACKUP_DIR/postcheck.txt"
echo ""
echo "Backup kept at: $BACKUP_DIR"
