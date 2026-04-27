# ITERATION_104

## Phase
High-risk destructive test reset (keep Admin) on live `v0.8`

## Goal
- Reset the environment to a clean re-test baseline.
- Keep Admin login capability and base org/role/config runtime.
- Clear task-centered business test data, upload/asset metadata, operation traces, and cache/tmp/log artifacts across server/NAS/local.

## Mandatory Boundary (enforced)
- Kept:
  - Admin/SuperAdmin users and role bindings
  - base org/options/roles contract behavior
  - release/deploy/migration/repo/docs skeleton
- Did not delete:
  - `/root/ecommerce_ai/releases/v0.8` binaries and deploy scripts
  - migration files
  - source repository
- Cleared:
  - task/procurement/asset/upload/export/integration test business data
  - runtime logs/caches/tmp artifacts in scoped directories

## Scripts Added
- SQL reset script:
  - `scripts/test_env_destructive_reset_keep_admin.sql`
- Shell orchestration script:
  - `scripts/test_env_destructive_reset_keep_admin.sh`

## Backup (before delete)
- Server backup:
  - `/root/ecommerce_ai/backups/20260402T022844Z_pre_test_reset_keep_admin`
  - includes:
    - `full_db.sql` (~1.1G)
    - `key_tables.sql` (~8.4M)
    - `service_status_pre.txt`
    - `server_logs_pre.list`
    - `server_tmp_pre.list`
    - reset SQL/result + post-reset verification files
- NAS backup:
  - `/volume1/homes/yongbo/asset-upload-service/backups/20260402T022844Z_pre_test_reset_keep_admin`
  - includes:
    - `nas_dirs_pre.list`
    - `nas_files_pre.list`
    - `nas_du_pre.txt`
    - `upload.db.pre`

## Actual Execution
- Service freeze:
  - stopped 8080/8081/8082 before DB and file cleanup
- SQL cleanup executed with explicit Admin guard:
  - guard requires non-empty keep-admin set
  - keep-admin result:
    - `admin`
    - `testuser_fix`
    - `candidate_test`
    - `test_01`
- Cleared DB tables (data-only):
  - task chain:
    - `tasks`, `task_details`, `task_sku_items`, `task_event_logs`, `task_event_sequences`
  - asset/upload chain:
    - `task_assets`, `design_assets`, `upload_requests`, `asset_storage_refs`
  - procurement/audit/warehouse/outsource chain:
    - `procurement_records`, `procurement_record_items`, `audit_records`, `audit_handovers`, `warehouse_receipts`, `outsource_orders`
  - cost override trace chain:
    - `cost_override_events`, `cost_override_event_sequences`, `cost_override_reviews`, `cost_override_finance_flags`
  - export/integration/log chain:
    - `export_jobs`, `export_job_events`, `export_job_event_sequences`, `export_job_attempts`, `export_job_dispatches`
    - `integration_call_logs`, `integration_call_executions`
    - `permission_logs`, `server_logs`, `erp_sync_runs`, `erp_sync_log` (optional live runtime log table)
    - `distribution_jobs`, `job_attempts`, `event_logs`, `sku_sequences`, `workbench_preferences`
  - auth sessions:
    - `user_sessions`
  - non-admin test users removed; keep-admin users preserved

## File/Cache/Log Cleanup
- Server:
  - `/root/ecommerce_ai/logs/*`
  - `/root/ecommerce_ai/tmp/*`
  - `/root/ecommerce_ai/server.log` truncated
- NAS upload service:
  - cleared:
    - `/volume1/docker/asset-upload/data/uploads/tasks/*`
    - `/volume1/docker/asset-upload/data/uploads/nas/design-assets/*`
    - `/volume1/docker/asset-upload/data/uploads/nas/file/*`
    - `/volume1/docker/asset-upload/data/uploads/.sessions/*`
  - sqlite metadata cleared:
    - `upload_sessions = 0`
    - `file_meta = 0`
  - preserved root skeleton and service root
- Local:
  - cleared cache/tmp dirs:
    - `.gocache`, `.gomodcache`, `.gotmp`, `.tmp`, `.tmp-go`, `.tmp_go`, `.tmp_gotest`, `tmp`
  - removed top-level temp files matching `tmp_*` / `.tmp_*`

## Restart and Verification
- Service health after restart:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
- Admin/base checks:
  - admin login success
  - `GET /v1/auth/me` success
  - `GET /v1/org/options` success
  - `GET /v1/roles` success
- Data-empty checks:
  - `GET /v1/tasks?page=1&page_size=20` => `{"data":[],"pagination":{"total":0,...}}`
  - `GET /v1/operation-logs?page=1&page_size=20` => empty
  - `GET /v1/export-jobs?page=1&page_size=20` => empty
  - `GET /v1/integration/call-logs?page=1&page_size=20` => empty
- Evidence files:
  - server:
    - `post_reset_core_counts.txt`
    - `post_reset_users.txt`
    - `post_reset_health.txt`
    - `api_verify_post_reset.txt`
  - NAS:
    - `nas_post_reset_status.txt`

## Notes and Residual Risk
- Post-reset verification itself creates fresh session/permission records.
  - This is expected new activity after reset.
- Base config/rule tables were intentionally preserved.
- This round is data cleanup only; no schema/migration edits on live DB structure.
