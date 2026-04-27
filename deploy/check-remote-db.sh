#!/usr/bin/env bash
# Remote database integration readiness check.
# Run on the cloud server; reads DB connection from shared/main.env.
# Usage: bash deploy/check-remote-db.sh [--base-dir /root/ecommerce_ai]
set -euo pipefail

BASE_DIR="/root/ecommerce_ai"
while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

ENV_FILE="$BASE_DIR/shared/main.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

# Load DB_* vars (avoid exporting secrets)
source_env() {
  set -a
  # shellcheck source=/dev/null
  . "$ENV_FILE" 2>/dev/null || true
  set +a
}
source_env

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
  run_mysql "DESCRIBE $1" | grep -qE "^\s*$2\s"
}

echo "=== Database Integration Readiness Check ==="
echo ""

# A. User & org
echo "--- A. User & Org Tables ---"
for t in users user_roles user_sessions permission_logs org_departments org_teams; do
  if has_table "$t"; then echo "  OK: $t"; else echo "  MISSING: $t"; fi
done
if has_table "users"; then
  for col in department mobile email team is_config_super_admin; do
    if has_column users "$col"; then echo "    OK users.$col"; else echo "    MISSING users.$col"; fi
  done
fi
if has_table "org_departments"; then
  enabled_legacy_departments="$(run_mysql "SELECT COUNT(*) FROM org_departments WHERE enabled = 1 AND name IN ('设计部','采购部','仓储部','烘焙仓储部')" | tr -d ' ')"
  echo "    enabled legacy departments: ${enabled_legacy_departments:-0}"
fi
if has_table "org_teams" && has_table "org_departments"; then
  enabled_legacy_teams="$(run_mysql "SELECT COUNT(*) FROM org_teams t INNER JOIN org_departments d ON d.id = t.department_id WHERE d.enabled = 1 AND t.enabled = 1 AND t.name IN ('运营一组','运营二组','运营三组','运营四组','运营五组','运营六组','运营七组','研发默认组','定制默认组','云仓默认组','定制美工审核组','常规审核组','设计组','定制美工组','设计审核组','采购组','仓储组','烘焙仓储组')" | tr -d ' ')"
  echo "    enabled legacy teams under enabled departments: ${enabled_legacy_teams:-0}"
fi
echo ""

# B. Task flow
echo "--- B. Task Flow Tables ---"
for t in tasks task_details task_event_logs task_event_sequences products procurement_records export_jobs; do
  if has_table "$t"; then echo "  OK: $t"; else echo "  MISSING: $t"; fi
done
echo ""

# C. ERP / Bridge
echo "--- C. ERP / Bridge Related ---"
for t in products integration_call_logs; do
  if has_table "$t"; then echo "  OK: $t"; else echo "  MISSING: $t"; fi
done
echo ""

# D. Logs / audit
echo "--- D. Logs & Audit ---"
for t in permission_logs integration_call_logs; do
  if has_table "$t"; then echo "  OK: $t"; else echo "  MISSING: $t"; fi
done
echo ""

echo "=== Check Complete ==="
unset MYSQL_PWD
