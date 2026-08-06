#!/usr/bin/env bash
# V8 production-schema readiness check. Read-only; uses shared/main.env.
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

set -a
# shellcheck source=/dev/null
. "$ENV_FILE"
set +a

if [ -z "${DB_HOST:-}" ] || [ -z "${DB_USER:-}" ] || [ -z "${DB_NAME:-}" ]; then
  echo "Set DB_HOST, DB_USER, DB_NAME in $ENV_FILE" >&2
  exit 1
fi

DB_PORT="${DB_PORT:-3306}"
MYSQL_BIN="${MYSQL_BIN:-mysql}"
export MYSQL_PWD="${DB_PASS:-}"
trap 'unset MYSQL_PWD' EXIT

run_mysql() {
  "$MYSQL_BIN" -h"${DB_HOST}" -P"${DB_PORT}" -u"${DB_USER}" "${DB_NAME}" -N -e "$1"
}

require_table() {
  local table="$1"
  if [ "$(run_mysql "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '${table}'")" != "1" ]; then
    echo "MISSING TABLE: $table" >&2
    return 1
  fi
  echo "  OK: $table"
}

require_column() {
  local table="$1"
  local column="$2"
  if [ "$(run_mysql "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '${table}' AND column_name = '${column}'")" != "1" ]; then
    echo "MISSING COLUMN: ${table}.${column}" >&2
    return 1
  fi
  echo "  OK: ${table}.${column}"
}

echo "=== V8 Database Readiness Check ==="
echo "database: ${DB_NAME}"

echo "--- Explicit access ---"
for table in auth_permissions auth_roles auth_role_permissions auth_user_role_assignments auth_assignment_scope_subjects auth_org_role_policies auth_policy_events auth_policy_state; do
  require_table "$table"
done

echo "--- Task, planning SKU and resource groups ---"
for table in tasks task_details task_sku_items task_planning_settings task_planning_sku_details task_planning_sku_revisions task_asset_groups task_asset_group_revisions task_asset_group_revision_items task_asset_group_revision_references task_asset_staging_drafts; do
  require_table "$table"
done
require_column tasks workflow_revision
require_column task_assets binding_state

echo "--- Async delivery ---"
for table in task_erp_outbox search_reindex_outbox asset_object_deletion_outbox; do
  require_table "$table"
done

legacy_task_types="$(run_mysql "SELECT COUNT(*) FROM tasks WHERE task_type = 'purchase_task'")"
legacy_states="$(run_mysql "SELECT COUNT(*) FROM tasks WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')")"
incomplete_groups="$(run_mysql "SELECT COUNT(*) FROM task_asset_groups WHERE migration_incomplete = 1")"
resource_group_mismatches="$(run_mysql "
  SELECT COUNT(*)
  FROM (
    SELECT
      t.id,
      CASE
        WHEN t.task_type = 'purchase_task' THEN 0
        WHEN t.task_type = 'sku_planning' THEN (
          SELECT COUNT(*) FROM task_sku_items tsi WHERE tsi.task_id = t.id
        )
        WHEN t.task_type = 'retouch_task' THEN (
          SELECT COUNT(*) FROM task_retouch_requirements trr WHERE trr.task_id = t.id
        )
        ELSE GREATEST(1, (
          SELECT COUNT(*) FROM task_sku_items tsi WHERE tsi.task_id = t.id
        ))
      END AS expected_groups,
      (SELECT COUNT(*) FROM task_asset_groups tag WHERE tag.task_id = t.id) AS actual_groups
    FROM tasks t
  ) resource_counts
  WHERE expected_groups <> actual_groups
")"

if [ "$legacy_task_types" != "0" ] || [ "$legacy_states" != "0" ] || [ "$incomplete_groups" != "0" ] || [ "$resource_group_mismatches" != "0" ]; then
  echo "V8 cutover blockers: purchase_task=${legacy_task_types}, legacy_states=${legacy_states}, migration_incomplete_groups=${incomplete_groups}, resource_group_mismatches=${resource_group_mismatches}" >&2
  exit 1
fi

echo "=== V8 database is ready ==="
