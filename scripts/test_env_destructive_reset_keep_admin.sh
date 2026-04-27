#!/usr/bin/env bash
set -euo pipefail

# High-risk destructive test reset orchestration.
# This script is meant to be run from the local MAIN control workspace.

SERVER_HOST="${SERVER_HOST:-jst_ecs}"
NAS_HOST="${NAS_HOST:-synology-dsm}"
SERVER_BASE_DIR="${SERVER_BASE_DIR:-/root/ecommerce_ai}"
NAS_UPLOAD_ROOT="${NAS_UPLOAD_ROOT:-/volume1/docker/asset-upload/data/uploads}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SQL_FILE_LOCAL="$REPO_ROOT/scripts/test_env_destructive_reset_keep_admin.sql"
UTC_TS="$(date -u +%Y%m%dT%H%M%SZ)"
SERVER_BACKUP_DIR="${SERVER_BASE_DIR}/backups/${UTC_TS}_pre_reset_keep_admin"
NAS_BACKUP_DIR="/volume1/homes/yongbo/asset-upload-service/backups/${UTC_TS}_pre_reset_keep_admin"

echo "[INFO] UTC_TS=$UTC_TS"
echo "[INFO] SERVER_BACKUP_DIR=$SERVER_BACKUP_DIR"
echo "[INFO] NAS_BACKUP_DIR=$NAS_BACKUP_DIR"

if [[ ! -f "$SQL_FILE_LOCAL" ]]; then
  echo "[ERROR] SQL file not found: $SQL_FILE_LOCAL" >&2
  exit 1
fi

echo "[STEP] 1/8 Create backup directories and baseline snapshots"
ssh "$SERVER_HOST" "
  set -euo pipefail
  mkdir -p '$SERVER_BACKUP_DIR'
  bash '$SERVER_BASE_DIR/current/deploy/check-three-services.sh' --base-dir '$SERVER_BASE_DIR' --no-auto-recover-8082 > '$SERVER_BACKUP_DIR/service_status_pre.txt' || true
  find '$SERVER_BASE_DIR/logs' -maxdepth 1 -type f | sort > '$SERVER_BACKUP_DIR/server_logs_pre.list' || true
  find '$SERVER_BASE_DIR/tmp' -maxdepth 3 -type f | sort > '$SERVER_BACKUP_DIR/server_tmp_pre.list' || true
"

ssh "$NAS_HOST" "
  set -euo pipefail
  mkdir -p '$NAS_BACKUP_DIR'
  find '$NAS_UPLOAD_ROOT' -maxdepth 3 -type d | sort > '$NAS_BACKUP_DIR/nas_dirs_pre.list'
  find '$NAS_UPLOAD_ROOT' -maxdepth 4 -type f | sort > '$NAS_BACKUP_DIR/nas_files_pre.list'
  du -sh '$NAS_UPLOAD_ROOT'/* > '$NAS_BACKUP_DIR/nas_du_pre.txt' 2>/dev/null || true
  if [[ -f '$NAS_UPLOAD_ROOT/.meta/upload.db' ]]; then
    cp '$NAS_UPLOAD_ROOT/.meta/upload.db' '$NAS_BACKUP_DIR/upload.db.pre'
  fi
"

echo "[STEP] 2/8 Full DB backup and key-table backup"
ssh "$SERVER_HOST" "
  set -euo pipefail
  set -a
  source '$SERVER_BASE_DIR/shared/main.env'
  set +a
  mysqldump -h\"\$DB_HOST\" -P\"\$DB_PORT\" -u\"\$DB_USER\" -p\"\$DB_PASS\" --single-transaction --routines --triggers \"\$DB_NAME\" > '$SERVER_BACKUP_DIR/full_db.sql'
  mysqldump -h\"\$DB_HOST\" -P\"\$DB_PORT\" -u\"\$DB_USER\" -p\"\$DB_PASS\" \"\$DB_NAME\" \
    users user_roles user_sessions tasks task_details task_assets design_assets upload_requests asset_storage_refs \
    procurement_records procurement_record_items task_event_logs task_event_sequences permission_logs integration_call_logs \
    export_jobs export_job_events export_job_attempts export_job_dispatches > '$SERVER_BACKUP_DIR/key_tables.sql'
"

echo "[STEP] 3/8 Stop services 8080/8081/8082"
ssh "$SERVER_HOST" "
  set -euo pipefail
  bash '$SERVER_BASE_DIR/current/deploy/stop-main.sh' --base-dir '$SERVER_BASE_DIR' || true
  bash '$SERVER_BASE_DIR/current/deploy/stop-bridge.sh' --base-dir '$SERVER_BASE_DIR' || true
  bash '$SERVER_BASE_DIR/current/deploy/stop-sync.sh' --base-dir '$SERVER_BASE_DIR' || true
"

echo "[STEP] 4/8 Execute SQL destructive reset (keep Admin/SuperAdmin)"
scp "$SQL_FILE_LOCAL" "$SERVER_HOST:$SERVER_BACKUP_DIR/test_env_destructive_reset_keep_admin.sql"
ssh "$SERVER_HOST" "
  set -euo pipefail
  set -a
  source '$SERVER_BASE_DIR/shared/main.env'
  set +a
  mysql -h\"\$DB_HOST\" -P\"\$DB_PORT\" -u\"\$DB_USER\" -p\"\$DB_PASS\" \"\$DB_NAME\" < '$SERVER_BACKUP_DIR/test_env_destructive_reset_keep_admin.sql' > '$SERVER_BACKUP_DIR/reset_sql_result.txt'
"

echo "[STEP] 5/8 Clean server logs and temporary files"
ssh "$SERVER_HOST" "
  set -euo pipefail
  find '$SERVER_BASE_DIR/logs' -mindepth 1 -maxdepth 1 -type f -delete || true
  find '$SERVER_BASE_DIR/tmp' -mindepth 1 -maxdepth 2 -type f -delete || true
  find '$SERVER_BASE_DIR/tmp' -mindepth 1 -maxdepth 2 -type d -empty -delete || true
  : > '$SERVER_BASE_DIR/server.log'
"

echo "[STEP] 6/8 Clean NAS task/upload test objects (keep root structure)"
ssh "$NAS_HOST" "
  set -euo pipefail
  mkdir -p '$NAS_UPLOAD_ROOT/tasks' '$NAS_UPLOAD_ROOT/nas/design-assets' '$NAS_UPLOAD_ROOT/nas/file' '$NAS_UPLOAD_ROOT/.sessions' '$NAS_UPLOAD_ROOT/.meta'
  find '$NAS_UPLOAD_ROOT/tasks' -mindepth 1 -maxdepth 2 -exec rm -rf {} +
  find '$NAS_UPLOAD_ROOT/nas/design-assets' -mindepth 1 -maxdepth 2 -exec rm -rf {} +
  find '$NAS_UPLOAD_ROOT/nas/file' -mindepth 1 -maxdepth 2 -exec rm -rf {} +
  find '$NAS_UPLOAD_ROOT/.sessions' -mindepth 1 -maxdepth 2 -exec rm -rf {} +
  if command -v sqlite3 >/dev/null 2>&1 && [[ -f '$NAS_UPLOAD_ROOT/.meta/upload.db' ]]; then
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' '.tables' > '$NAS_BACKUP_DIR/upload_db_tables.txt' || true
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' 'DELETE FROM upload_requests;' || true
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' 'DELETE FROM multipart_parts;' || true
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' 'DELETE FROM completed_files;' || true
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' 'DELETE FROM upload_sessions;' || true
    sqlite3 '$NAS_UPLOAD_ROOT/.meta/upload.db' 'DELETE FROM file_meta;' || true
  fi
"

echo "[STEP] 7/8 Restart services"
ssh "$SERVER_HOST" "
  set -euo pipefail
  bash '$SERVER_BASE_DIR/current/deploy/start-main.sh' --base-dir '$SERVER_BASE_DIR' --env-file '$SERVER_BASE_DIR/shared/main.env' --binary-path '$SERVER_BASE_DIR/current/ecommerce-api'
  bash '$SERVER_BASE_DIR/current/deploy/start-bridge.sh' --base-dir '$SERVER_BASE_DIR' --env-file '$SERVER_BASE_DIR/shared/bridge.env'
  bash '$SERVER_BASE_DIR/current/deploy/start-sync.sh' --base-dir '$SERVER_BASE_DIR'
"

echo "[STEP] 8/8 Post-reset health and API verification"
ssh "$SERVER_HOST" "SERVER_BASE_DIR='$SERVER_BASE_DIR' SERVER_BACKUP_DIR='$SERVER_BACKUP_DIR' bash -s" <<'REMOTE_STEP8'
set -euo pipefail
bash "$SERVER_BASE_DIR/current/deploy/check-three-services.sh" --base-dir "$SERVER_BASE_DIR" --main-url http://127.0.0.1:8080 --bridge-url http://127.0.0.1:8081 --sync-url http://127.0.0.1:8082 --no-auto-recover-8082 > "$SERVER_BACKUP_DIR/service_status_post.txt"
python3 - <<'PY'
import json
import os
import urllib.request

ADMIN_PASSWORD = os.environ.get("RESET_ADMIN_PASSWORD", "")
if not ADMIN_PASSWORD:
    raise SystemExit("RESET_ADMIN_PASSWORD env var is required for post-reset verification")

def call(method, url, body=None, headers=None):
    req = urllib.request.Request(url, data=body, method=method)
    if headers:
        for k, v in headers.items():
            req.add_header(k, v)
    with urllib.request.urlopen(req, timeout=10) as resp:
        return resp.status, resp.read()

status, body = call(
    "POST",
    "http://127.0.0.1:8080/v1/auth/login",
    body=json.dumps({"username": "admin", "password": ADMIN_PASSWORD}).encode("utf-8"),
    headers={"Content-Type": "application/json"},
)
payload = json.loads(body.decode("utf-8"))
token = (
    payload.get("data", {}).get("session", {}).get("token")
    or payload.get("data", {}).get("token")
    or payload.get("token")
)
if not token:
    raise SystemExit("admin login returned no token")

auth = {"Authorization": f"Bearer {token}"}
checks = [
    ("GET", "http://127.0.0.1:8080/v1/auth/me", None, auth),
    ("GET", "http://127.0.0.1:8080/v1/org/options", None, auth),
    ("GET", "http://127.0.0.1:8080/v1/roles", None, auth),
    ("GET", "http://127.0.0.1:8080/v1/tasks?page=1&page_size=20", None, auth),
]
for method, url, body, headers in checks:
    s, b = call(method, url, body, headers)
    print(f"{url} -> {s}")
    print(b.decode("utf-8")[:600])
PY
REMOTE_STEP8

echo "[DONE] destructive test reset workflow completed"
echo "[INFO] backup_dir_server=$SERVER_BACKUP_DIR"
echo "[INFO] backup_dir_nas=$NAS_BACKUP_DIR"
