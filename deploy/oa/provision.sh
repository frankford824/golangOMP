#!/usr/bin/env bash
# Run once as root on the existing ECS. Source is read-only during this snapshot.
set -euo pipefail
umask 077
oa_db=yongbo_oa
oa_user=yongbo_oa_app
env_dir=/etc/yongbo-oa
install -d -m 0700 "$env_dir"
if [[ -f "$env_dir/provisioned" ]]; then
  echo "OA snapshot already provisioned; preserving validation data."
  exit 0
fi
. /root/ecommerce_ai/shared/main.env
[[ "$DB_NAME" =~ ^[A-Za-z0-9_]+$ && "$DB_NAME" != "$oa_db" ]]
export MYSQL_PWD="$DB_PASS"
mysql=(mysql --no-defaults --protocol=tcp -h "$DB_HOST" -P "${DB_PORT:-3306}" -u "$DB_USER" --default-character-set=utf8mb4 --batch --skip-column-names)
if [[ -z "$("${mysql[@]}" -e "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$oa_db'")" ]]; then
  "${mysql[@]}" -e "CREATE DATABASE $oa_db CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
  touch "$env_dir/owned-schema"
elif [[ ! -f "$env_dir/owned-schema" ]]; then
  echo "Refusing to adopt an existing database" >&2; exit 1
fi
if [[ ! -f "$env_dir/db-password" ]]; then openssl rand -hex 32 > "$env_dir/db-password"; fi
pw=$(cat "$env_dir/db-password")
peer=$("${mysql[@]}" -e "SELECT SUBSTRING_INDEX(USER(),'@',-1)")
[[ "$peer" =~ ^[A-Za-z0-9.:-]+$ ]]
"${mysql[@]}" -e "CREATE USER IF NOT EXISTS '$oa_user'@'$peer' IDENTIFIED BY '$pw'; GRANT ALL PRIVILEGES ON $oa_db.* TO '$oa_user'@'$peer';"
# Schema only: no triggers, routines, events, credentials, sessions, outboxes or cash transactions.
mapfile -t tables < <("${mysql[@]}" information_schema -e "SELECT TABLE_NAME FROM TABLES WHERE TABLE_SCHEMA='$DB_NAME' AND TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME")
(( ${#tables[@]} > 100 ))
mysqldump --no-defaults --protocol=tcp -h "$DB_HOST" -P "${DB_PORT:-3306}" -u "$DB_USER" --no-data --skip-triggers --skip-add-drop-table --skip-lock-tables --no-tablespaces --set-gtid-purged=OFF "$DB_NAME" "${tables[@]}" | sed 's/^CREATE TABLE /CREATE TABLE IF NOT EXISTS /' | "${mysql[@]}" "$oa_db"
# Data phase is a single consistent transaction. Never re-seed a live OA database.
{
  echo 'SET FOREIGN_KEY_CHECKS=0; START TRANSACTION WITH CONSISTENT SNAPSHOT;'
  for tab in org_departments org_teams auth_roles auth_permissions auth_role_permissions auth_policy_state auth_user_role_assignments auth_assignment_scope_subjects auth_org_role_policies code_rules code_rule_revisions code_rule_sequences code_rule_revision_sequences; do
    echo "INSERT INTO $oa_db.$tab SELECT * FROM $DB_NAME.$tab;"
  done
  cat <<SQL
INSERT INTO $oa_db.users (id,username,employee_no,display_name,department,department_id,team,team_id,password_hash,status,employment_type,created_at,updated_at)
SELECT id,CONCAT('oa-u',id),employee_no,display_name,department,department_id,team,team_id,'!no-test-password!',status,employment_type,created_at,updated_at FROM $DB_NAME.users;
INSERT INTO $oa_db.app_memberships (id,app_code,user_id,status,identity_type,source,created_at,updated_at)
SELECT id,app_code,user_id,status,identity_type,source,created_at,updated_at FROM $DB_NAME.app_memberships;
INSERT INTO $oa_db.asset_workbench_profiles (id,user_id,worker_type,job_grade,real_name,status,grade_hidden,settlement_visible,created_at,updated_at)
SELECT id,user_id,worker_type,job_grade,real_name,status,grade_hidden,settlement_visible,created_at,updated_at FROM $DB_NAME.asset_workbench_profiles;
INSERT INTO $oa_db.tasks (id,task_no,source_mode,sku_code,product_name_snapshot,task_type,creator_id,requester_id,designer_id,current_handler_id,task_status,workflow_revision,priority,deadline_at,created_at,updated_at,owner_department,owner_department_id,owner_org_team,owner_team_id,business_lane,batch_item_count,primary_sku_code)
SELECT id,task_no,source_mode,sku_code,product_name_snapshot,task_type,creator_id,requester_id,designer_id,current_handler_id,task_status,workflow_revision,priority,deadline_at,created_at,updated_at,owner_department,owner_department_id,owner_org_team,owner_team_id,business_lane,batch_item_count,primary_sku_code FROM $DB_NAME.tasks;
INSERT INTO $oa_db.task_sku_items (id,task_id,sequence_no,sku_code,sku_status,sku_origin,product_name_snapshot,product_i_id,cost_price,quantity,estimated_cost,manual_cost_override,requires_manual_review,design_requirement,reference_file_refs_json,dedupe_key,created_at,updated_at)
SELECT id,task_id,sequence_no,sku_code,sku_status,sku_origin,product_name_snapshot,product_i_id,cost_price,quantity,estimated_cost,manual_cost_override,requires_manual_review,'','[]',CONCAT('oa-seed-',id),created_at,updated_at FROM $DB_NAME.task_sku_items;
COMMIT;
SQL
} | "${mysql[@]}" "$oa_db"
getent passwd yongbo-oa >/dev/null || useradd --system --user-group --home-dir /var/lib/yongbo-oa --shell /usr/sbin/nologin yongbo-oa
install -d -m 700 -o yongbo-oa -g yongbo-oa /var/lib/yongbo-oa /var/lib/yongbo-oa/attachments
install -d -m 755 /srv/yongbo-oa /srv/yongbo-oa/releases
if [[ ! -s "$env_dir/admin-password" ]]; then openssl rand -hex 20 > "$env_dir/admin-password"; fi
python3 - "$env_dir" <<'PY'
import json,sys
from pathlib import Path
d=Path(sys.argv[1])
src=Path('/root/ecommerce_ai/current/config/auth_identity.example.json')
c=json.loads(src.read_text())
c['super_admins']=[dict(username='oa-review-admin',display_name='OA 验证管理员',department='未分配',team='未分配池',mobile='13900009991',password=(d/'admin-password').read_text().strip())]
c['department_admin_keys']={}
c['configured_user_assignments']=[]
(d/'identity.json').write_text(json.dumps(c,ensure_ascii=False))
PY
# Only this file contains the sandbox bootstrap credential and must be readable by the service user.
chown root:yongbo-oa "$env_dir/identity.json"
chmod 0640 "$env_dir/identity.json"
# Keep the secret parent root-only; deliver identity via systemd credentials.
cat > "$env_dir/core.env" <<EOF
MYSQL_DSN=$oa_user:$pw@unix(/run/yongbo-oa-db/mysql.sock)/$oa_db?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4
SERVER_PORT=18086
AUTH_SETTINGS_FILE=/run/credentials/yongbo-oa-core.service/identity.json
FRONTEND_ACCESS_SETTINGS_FILE=/srv/yongbo-oa/current/config/frontend_access.json
ERP_SYNC_ENABLED=false
COST_SYNC_ENABLED=false
ERP_BRIDGE_BASE_URL=http://127.0.0.1:9
ERP_IMAGE_PROXY_PUBLIC_BASE_URL=https://oa.yongbo.cloud
UPLOAD_SERVICE_ENABLED=false
WEB_PUSH_ENABLED=false
AI_AGENT_ENABLED=false
WECHAT_ENABLED=false
ASSET_MEDIA_JOBS_ENABLED=false
ASSET_WORKBENCH_BATCH_JOB_WORKER_ENABLED=false
ASSET_WORKBENCH_PREVIEW_WORKER_ENABLED=false
ASSET_WORKBENCH_UPLOAD_EXPIRY_WORKER_ENABLED=false
MYSQL_MAX_OPEN_CONNS=8
TZ=Asia/Shanghai
EOF
cat > "$env_dir/oa.env" <<EOF
OA_DB_DSN=$oa_user:$pw@tcp(127.0.0.1:${DB_PORT:-3306})/$oa_db?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4
OA_CORE_URL=http://127.0.0.1:18087
OA_FILES_DIR=/var/lib/yongbo-oa/attachments
TZ=Asia/Shanghai
EOF
printf 'Seeded from %s at %s\n' "$DB_NAME" "$(date -Is)" > "$env_dir/provisioned"
echo "Created independent OA storage and isolated reference snapshot."
