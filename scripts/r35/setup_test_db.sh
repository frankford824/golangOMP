#!/usr/bin/env bash
set -euo pipefail

cd /root/ecommerce_ai
. ./shared/main.env
export MYSQL_PWD="$DB_PASS"

TEST_DB="jst_erp_r3_test"
PROD_DUMP="/root/ecommerce_ai/backups/20260424T024501Z_r2_pre_backfill.sql.gz"

if [[ ! -f "$PROD_DUMP" ]]; then
  echo "missing $PROD_DUMP"
  exit 2
fi

echo "== 1. (Re)create $TEST_DB =="
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "DROP DATABASE IF EXISTS \`$TEST_DB\`;"
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e "CREATE DATABASE \`$TEST_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

echo "== 2. Restore dump, rewriting DB name =="
gunzip -c "$PROD_DUMP" | \
  sed 's/`jst_erp`/`jst_erp_r3_test`/g' | \
  mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER"

echo "== 3. Sanity checks =="
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" "$TEST_DB" -N -B -e "
  SELECT 'tasks' tbl, COUNT(*) FROM tasks
  UNION ALL SELECT 'task_details', COUNT(*) FROM task_details
  UNION ALL SELECT 'task_assets', COUNT(*) FROM task_assets
  UNION ALL SELECT 'asset_storage_refs', COUNT(*) FROM asset_storage_refs
  UNION ALL SELECT 'users', COUNT(*) FROM users
  UNION ALL SELECT 'customization_jobs', COUNT(*) FROM customization_jobs;"

echo "== 4. Pre-test backup =="
TS=$(date -u +%Y%m%dT%H%M%SZ)
mysqldump -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" --single-transaction --databases "$TEST_DB" \
  | gzip > "/root/ecommerce_ai/backups/${TS}_r35_pre_test.sql.gz"
echo "backup: /root/ecommerce_ai/backups/${TS}_r35_pre_test.sql.gz"

echo "DONE: $TEST_DB ready"
